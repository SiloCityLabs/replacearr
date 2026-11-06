package sonarr

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/replacearr/replacearr/internal/config"
	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/domain"
	"github.com/replacearr/replacearr/internal/downloadmonitor"
	"github.com/replacearr/replacearr/internal/mediacover"
	"github.com/replacearr/replacearr/internal/mediafile"
	"github.com/replacearr/replacearr/internal/metadata"
)

type SearchFunc func(query string, mediaType string, tvdbID int, tmdbID int, season int, episode int) ([]domain.ReleaseResult, error)

type Handler struct {
	store    *db.Store
	cfg      *config.Config
	searchFn SearchFunc
	meta     *metadata.Service
	covers   *mediacover.Store
	start    time.Time

	cmdMu    sync.RWMutex
	commands []domain.CommandRecord
	nextCmd  int64
}

func NewHandler(store *db.Store, cfg *config.Config, searchFn SearchFunc) *Handler {
	return &Handler{
		store:    store,
		cfg:      cfg,
		searchFn: searchFn,
		meta:     metadata.New(),
		start:    time.Now().UTC(),
		commands: []domain.CommandRecord{},
		nextCmd:  1,
	}
}

// SetCoverStore attaches the shared media cover cache used by /MediaCover routes.
func (h *Handler) SetCoverStore(covers *mediacover.Store) {
	h.covers = covers
}

// SetMetadata attaches the shared SkyHook lookup service.
func (h *Handler) SetMetadata(meta *metadata.Service) {
	if meta != nil {
		h.meta = meta
	}
}

func (h *Handler) preferredLanguage() string {
	if h.store != nil {
		if cfg, err := h.store.GetAppConfig(); err == nil {
			if lang := strings.TrimSpace(cfg.Metadata.PreferredLanguage); lang != "" {
				return lang
			}
		}
	}
	return "english"
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := normalizeSonarrPath(r.URL.Path)

	switch {
	case (path == "/api" || path == "/system/status") && r.Method == http.MethodGet:
		h.handleSystemStatus(w, r)

	case path == "/health" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, []any{})

	case strings.HasPrefix(path, "/MediaCover/") && r.Method == http.MethodGet:
		h.handleMediaCover(w, r, path)

	case path == "/series/lookup" && r.Method == http.MethodGet:
		h.handleSeriesLookup(w, r)

	case path == "/series" && r.Method == http.MethodGet:
		h.handleListSeries(w, r)

	case path == "/series" && r.Method == http.MethodPost:
		h.handleCreateSeries(w, r)

	case path == "/series" && r.Method == http.MethodPut:
		h.handleUpdateSeriesBody(w, r)

	case strings.HasPrefix(path, "/series/"):
		idStr := strings.TrimPrefix(path, "/series/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid series id")
			return
		}
		switch r.Method {
		case http.MethodGet:
			h.handleGetSeries(w, r, id)
		case http.MethodPut:
			h.handleUpdateSeries(w, r, id)
		case http.MethodDelete:
			h.handleDeleteSeries(w, r, id)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}

	case path == "/episode" && r.Method == http.MethodGet:
		h.handleListEpisodes(w, r)

	case (path == "/episodefile" || strings.HasPrefix(path, "/episodefile/")) && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, []any{})

	case path == "/calendar" && r.Method == http.MethodGet:
		h.handleCalendar(w, r)

	case path == "/queue" && r.Method == http.MethodGet:
		h.handleGetQueue(w, r)

	case path == "/queue/status" && r.Method == http.MethodGet:
		h.handleGetQueueStatus(w, r)

	case strings.HasPrefix(path, "/queue/") && r.Method == http.MethodDelete:
		idStr := strings.TrimPrefix(path, "/queue/")
		if slash := strings.IndexByte(idStr, '/'); slash >= 0 {
			idStr = idStr[:slash]
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid queue id")
			return
		}
		removeFromClient := queryFlag(r, "removeFromClient")
		deleteData := queryFlag(r, "deleteData")
		if removeFromClient {
			if item, err := h.store.GetQueueItem(id); err == nil && item.DownloadID != "" {
				_ = downloadmonitor.RemoveFromClient(r.Context(), h.store, item.DownloadClient, item.DownloadID, deleteData)
			}
		}
		if err := h.store.DeleteQueueItem(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{})

	case path == "/command" && r.Method == http.MethodGet:
		h.handleListCommands(w, r)

	case path == "/command" && r.Method == http.MethodPost:
		h.handlePostCommand(w, r)

	case strings.HasPrefix(path, "/command/") && r.Method == http.MethodGet:
		idStr := strings.TrimPrefix(path, "/command/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid command id")
			return
		}
		h.handleGetCommand(w, r, id)

	case path == "/release" && r.Method == http.MethodGet:
		h.handleSearchReleases(w, r)

	case path == "/release" && r.Method == http.MethodPost:
		h.handleGrabRelease(w, r)

	case path == "/qualityprofile" && r.Method == http.MethodGet:
		h.handleListQualityProfiles(w, r)

	case path == "/qualityprofile" && r.Method == http.MethodPost:
		h.handleCreateQualityProfile(w, r)

	case strings.HasPrefix(path, "/qualityprofile/") && r.Method == http.MethodGet:
		idStr := strings.TrimPrefix(path, "/qualityprofile/")
		id, _ := strconv.ParseInt(idStr, 10, 64)
		h.handleGetQualityProfile(w, r, id)

	case (path == "/languageprofile" || strings.HasPrefix(path, "/languageprofile/")) && r.Method == http.MethodGet:
		h.handleLanguageProfiles(w, r, path)

	case path == "/customformat" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, []any{})

	case path == "/rootfolder" && r.Method == http.MethodGet:
		h.handleListRootFolders(w, r)

	case path == "/rootfolder" && r.Method == http.MethodPost:
		h.handleCreateRootFolder(w, r)

	case strings.HasPrefix(path, "/rootfolder/") && r.Method == http.MethodDelete:
		idStr := strings.TrimPrefix(path, "/rootfolder/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid rootfolder id")
			return
		}
		if err := h.store.DeleteRootFolder(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{})

	case path == "/tag" && r.Method == http.MethodGet:
		h.handleListTags(w, r)

	case path == "/tag" && r.Method == http.MethodPost:
		h.handleCreateTag(w, r)

	case strings.HasPrefix(path, "/tag/") && r.Method == http.MethodDelete:
		idStr := strings.TrimPrefix(path, "/tag/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid tag id")
			return
		}
		_ = h.store.DeleteTag(id)
		writeJSON(w, http.StatusOK, map[string]any{})

	case path == "/downloadclient" && r.Method == http.MethodGet:
		h.handleListDownloadClients(w, r)

	case path == "/downloadclient" && r.Method == http.MethodPost:
		h.handleCreateDownloadClient(w, r)

	case strings.HasPrefix(path, "/downloadclient/") && r.Method == http.MethodDelete:
		idStr := strings.TrimPrefix(path, "/downloadclient/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid downloadclient id")
			return
		}
		_ = h.store.DeleteDownloadClient(id)
		writeJSON(w, http.StatusOK, map[string]any{})

	case path == "/indexer" && r.Method == http.MethodGet:
		h.handleListIndexers(w, r)

	case path == "/indexer" && r.Method == http.MethodPost:
		h.handleCreateIndexer(w, r)

	case strings.HasPrefix(path, "/indexer/"):
		idStr := strings.TrimPrefix(path, "/indexer/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid indexer id")
			return
		}
		switch r.Method {
		case http.MethodGet:
			h.handleGetIndexer(w, r, id)
		case http.MethodPut:
			h.handleUpdateIndexer(w, r, id)
		case http.MethodDelete:
			h.handleDeleteIndexer(w, r, id)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}

	case path == "/history" && r.Method == http.MethodGet:
		h.handleGetHistory(w, r)

	default:
		writeError(w, http.StatusNotFound, "endpoint not found: "+r.URL.Path)
	}
}

func normalizeSonarrPath(p string) string {
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return "/"
	}
	if strings.HasPrefix(p, "/sonarr") {
		p = strings.TrimPrefix(p, "/sonarr")
	}
	if p == "" || p == "/api" {
		return "/api"
	}
	if strings.HasPrefix(p, "/api/v3") {
		p = strings.TrimPrefix(p, "/api/v3")
	} else if strings.HasPrefix(p, "/v3") {
		p = strings.TrimPrefix(p, "/v3")
	}
	if p == "" {
		return "/"
	}
	return p
}

func (h *Handler) handleSystemStatus(w http.ResponseWriter, _ *http.Request) {
	appData := "/config"
	if h.cfg != nil && h.cfg.ConfigDir != "" {
		appData = h.cfg.ConfigDir
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"appName":           "Sonarr",
		"instanceName":      "Replacearr (Sonarr Compat)",
		"version":           "3.0.10.1567",
		"buildTime":         "2026-01-01T00:00:00Z",
		"isDebug":           false,
		"isProduction":      true,
		"isAdmin":           true,
		"isUserInteractive": false,
		"startupPath":       "/app",
		"appData":           appData,
		"osName":            "linux",
		"osVersion":         "6.6.0",
		"isMonoRuntime":     false,
		"isMono":            false,
		"isLinux":           true,
		"isOsx":             false,
		"isWindows":         false,
		"isDocker":          true,
		"mode":              "console",
		"branch":            "main",
		"authentication":    "apiKey",
		"sqliteVersion":     "3.45.0",
		"migrationVersion":  195,
		"urlBase":           "/sonarr",
		"runtimeVersion":    "6.0.25",
		"runtimeName":       "netcore",
		"startTime":         h.start,
	})
}

func (h *Handler) handleListSeries(w http.ResponseWriter, r *http.Request) {
	list, err := h.store.ListSeries()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tvdbStr := r.URL.Query().Get("tvdbId"); tvdbStr != "" {
		if tvdbID, err := strconv.Atoi(tvdbStr); err == nil && tvdbID > 0 {
			filtered := []domain.Series{}
			for _, s := range list {
				if s.TvdbID == tvdbID {
					filtered = append(filtered, s)
				}
			}
			writeJSON(w, http.StatusOK, filtered)
			return
		}
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handler) handleCreateSeries(w http.ResponseWriter, r *http.Request) {
	var sr domain.Series
	if err := json.NewDecoder(r.Body).Decode(&sr); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if sr.Title == "" && sr.TvdbID > 0 && h.meta != nil {
		if looked, _, _, err := h.meta.GetSeriesByTVDB(r.Context(), sr.TvdbID, h.preferredLanguage()); err == nil {
			sr.Title = looked.Title
			if sr.Year == 0 {
				sr.Year = looked.Year
			}
			if sr.Overview == "" {
				sr.Overview = looked.Overview
			}
			if sr.ImdbID == "" {
				sr.ImdbID = looked.ImdbID
			}
			if len(sr.Images) == 0 {
				sr.Images = looked.Images
			}
		}
		if sr.Title == "" {
			sr.Title = fmt.Sprintf("Series %d", sr.TvdbID)
		}
	}
	created, err := h.store.CreateSeries(sr)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.syncSeriesEpisodes(r.Context(), created)
	remote := mediacover.PosterRemoteURL(created.Images)
	local := mediacover.LocalURL("tv", created.ID)
	created.Images = mediacover.SetLocalPoster(created.Images, local, remote)
	if updated, uerr := h.store.UpdateSeries(*created); uerr == nil && updated != nil {
		created = updated
	}
	h.cacheCoverAsync(created)
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleGetSeries(w http.ResponseWriter, _ *http.Request, id int64) {
	sr, err := h.store.GetSeriesByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "series not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sr)
}

func (h *Handler) handleUpdateSeries(w http.ResponseWriter, r *http.Request, id int64) {
	var sr domain.Series
	if err := json.NewDecoder(r.Body).Decode(&sr); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	sr.ID = id
	updated, err := h.store.UpdateSeries(sr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) handleUpdateSeriesBody(w http.ResponseWriter, r *http.Request) {
	var sr domain.Series
	if err := json.NewDecoder(r.Body).Decode(&sr); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	updated, err := h.store.UpdateSeries(sr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) handleDeleteSeries(w http.ResponseWriter, r *http.Request, id int64) {
	deleteFiles := queryFlag(r, "deleteFiles")
	addExclusion := queryFlag(r, "addImportListExclusion") || queryFlag(r, "addImportExclusion")

	sr, err := h.store.GetSeriesByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "series not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if addExclusion {
		if _, err := h.store.AddImportListExclusion(domain.ImportListExclusion{
			MediaType: "tv",
			TvdbID:    sr.TvdbID,
			ImdbID:    sr.ImdbID,
			Title:     sr.Title,
			Year:      sr.Year,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "add list exclusion: "+err.Error())
			return
		}
	}

	if deleteFiles {
		if err := h.store.DeleteSeriesDiskFiles(sr, h.recycleBinPath()); err != nil {
			writeError(w, http.StatusInternalServerError, "delete series files: "+err.Error())
			return
		}
	}

	if err := h.store.DeleteSeries(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (h *Handler) recycleBinPath() string {
	configured := ""
	if cfg, err := h.store.GetAppConfig(); err == nil {
		configured = cfg.MediaManagement.RecycleBin
	}
	configDir := "./config"
	if h.cfg != nil && h.cfg.ConfigDir != "" {
		configDir = h.cfg.ConfigDir
	}
	return mediafile.ResolveRecycleBin(configured, configDir)
}

func queryFlag(r *http.Request, name string) bool {
	v := strings.TrimSpace(strings.ToLower(r.URL.Query().Get(name)))
	return v == "true" || v == "1" || v == "yes"
}

func (h *Handler) handleSeriesLookup(w http.ResponseWriter, r *http.Request) {
	term := strings.TrimSpace(r.URL.Query().Get("term"))
	if term == "" {
		writeJSON(w, http.StatusOK, []domain.Series{})
		return
	}
	results, err := h.meta.SearchSeries(r.Context(), term, h.preferredLanguage())
	if err != nil {
		writeError(w, http.StatusBadGateway, "series lookup failed: "+err.Error())
		return
	}
	if results == nil {
		results = []domain.Series{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *Handler) handleListEpisodes(w http.ResponseWriter, r *http.Request) {
	var seriesID int64
	if sStr := r.URL.Query().Get("seriesId"); sStr != "" {
		seriesID, _ = strconv.ParseInt(sStr, 10, 64)
	}
	eps, err := h.store.ListEpisodes(seriesID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(eps) == 0 && seriesID > 0 {
		if sr, gerr := h.store.GetSeriesByID(seriesID); gerr == nil && sr != nil && sr.TvdbID > 0 {
			if serr := h.syncSeriesEpisodes(r.Context(), sr); serr == nil {
				eps, err = h.store.ListEpisodes(seriesID)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, eps)
}

func (h *Handler) syncSeriesEpisodes(ctx context.Context, sr *domain.Series) error {
	if h.meta == nil || sr == nil || sr.TvdbID <= 0 || sr.ID <= 0 {
		return nil
	}
	_, eps, aliases, err := h.meta.GetSeriesByTVDB(ctx, sr.TvdbID, h.preferredLanguage())
	if err != nil {
		return err
	}
	if len(aliases) > 0 {
		_ = h.store.ReplaceSeriesAliases(sr.ID, aliases)
	}
	if len(eps) == 0 {
		return nil
	}
	return h.store.ReplaceEpisodesForSeries(sr.ID, eps)
}

func (h *Handler) handleCalendar(w http.ResponseWriter, r *http.Request) {
	eps, err := h.store.ListEpisodes(0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	start, _ := time.Parse(time.RFC3339, r.URL.Query().Get("start"))
	end, _ := time.Parse(time.RFC3339, r.URL.Query().Get("end"))
	if start.IsZero() && end.IsZero() {
		writeJSON(w, http.StatusOK, eps)
		return
	}
	out := make([]domain.Episode, 0, len(eps))
	for _, ep := range eps {
		air := ep.AirDateUtc
		if air.IsZero() {
			continue
		}
		if !start.IsZero() && air.Before(start) {
			continue
		}
		if !end.IsZero() && air.After(end) {
			continue
		}
		out = append(out, ep)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) handleGetQueue(w http.ResponseWriter, _ *http.Request) {
	items, err := h.store.ListQueueItems("tv")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"page":          1,
		"pageSize":      20,
		"sortKey":       "timeleft",
		"sortDirection": "ascending",
		"totalRecords":  len(items),
		"records":       items,
	})
}

func (h *Handler) handleGetQueueStatus(w http.ResponseWriter, _ *http.Request) {
	items, _ := h.store.ListQueueItems("tv")
	writeJSON(w, http.StatusOK, map[string]any{
		"totalCount":      len(items),
		"count":           len(items),
		"unknownCount":    0,
		"errors":          false,
		"warnings":        false,
		"unknownErrors":   false,
		"unknownWarnings": false,
	})
}

func (h *Handler) handleListCommands(w http.ResponseWriter, _ *http.Request) {
	h.cmdMu.RLock()
	defer h.cmdMu.RUnlock()
	out := make([]domain.CommandRecord, len(h.commands))
	copy(out, h.commands)
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) handleGetCommand(w http.ResponseWriter, _ *http.Request, id int64) {
	h.cmdMu.RLock()
	defer h.cmdMu.RUnlock()
	for _, cmd := range h.commands {
		if cmd.ID == id {
			writeJSON(w, http.StatusOK, cmd)
			return
		}
	}
	writeError(w, http.StatusNotFound, "command not found")
}

func (h *Handler) handlePostCommand(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body = map[string]any{}
	}
	name, _ := body["name"].(string)
	if name == "" {
		if cn, ok := body["commandName"].(string); ok && cn != "" {
			name = cn
		} else {
			name = "SeriesSearch"
		}
	}

	now := time.Now().UTC()
	h.cmdMu.Lock()
	id := h.nextCmd
	h.nextCmd++
	rec := domain.CommandRecord{
		ID:          id,
		Name:        name,
		CommandName: name,
		Message:     fmt.Sprintf("%s completed successfully", name),
		Body:        body,
		Priority:    "normal",
		Status:      "completed",
		Queued:      now,
		Started:     now,
		Ended:       now,
		Trigger:     "manual",
	}
	h.commands = append(h.commands, rec)
	h.cmdMu.Unlock()

	writeJSON(w, http.StatusCreated, rec)
}

func (h *Handler) handleSearchReleases(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	term := strings.TrimSpace(q.Get("term"))
	var seriesID int64
	if sStr := q.Get("seriesId"); sStr != "" {
		seriesID, _ = strconv.ParseInt(sStr, 10, 64)
	}
	season, _ := strconv.Atoi(q.Get("season"))
	episode, _ := strconv.Atoi(q.Get("episode"))
	if episode == 0 {
		episode, _ = strconv.Atoi(q.Get("ep"))
	}
	var tvdbID int
	if seriesID > 0 {
		if sr, err := h.store.GetSeriesByID(seriesID); err == nil && sr != nil {
			if term == "" {
				term = sr.Title
			}
			tvdbID = sr.TvdbID
			// Alias planning / aired-vs-airing season packs are handled in SearchReleases.
		}
	}
	if term == "" {
		term = "Severance"
	}

	var results []domain.ReleaseResult
	if h.searchFn != nil {
		res, err := h.searchFn(term, "tv", tvdbID, 0, season, episode)
		if err == nil {
			results = res
		}
	}
	if results == nil {
		results = DefaultSearchReleases(term, "tv", tvdbID, 0, season, episode)
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *Handler) handleGrabRelease(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Guid        string `json:"guid"`
		Title       string `json:"title"`
		IndexerID   int64  `json:"indexerId"`
		Indexer     string `json:"indexer"`
		Size        int64  `json:"size"`
		Protocol    string `json:"protocol"`
		SeriesID    int64  `json:"seriesId"`
		EpisodeID   int64  `json:"episodeId"`
		DownloadID  string `json:"downloadId"`
		DownloadURL string `json:"downloadUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid release payload")
		return
	}
	if payload.Title == "" {
		if payload.Guid != "" {
			payload.Title = payload.Guid
		} else {
			payload.Title = "Grabbed.TV.Release.S01E01.1080p.WEB-DL"
		}
	}
	if payload.Protocol == "" {
		payload.Protocol = "torrent"
	}
	if payload.Indexer == "" {
		payload.Indexer = "unknown"
	}
	clients, _ := h.store.ListDownloadClients()
	clientName := ""
	for _, c := range clients {
		if c.Enable {
			clientName = c.Name
			break
		}
	}
	if clientName == "" {
		writeError(w, http.StatusUnprocessableEntity, "no enabled download client configured")
		return
	}
	if payload.DownloadID == "" {
		payload.DownloadID = fmt.Sprintf("sonarr-%d", time.Now().UnixNano())
	}

	qItem, err := h.store.CreateQueueItem(domain.QueueItem{
		MediaType:             "tv",
		SeriesID:              payload.SeriesID,
		EpisodeID:             payload.EpisodeID,
		Title:                 payload.Title,
		Status:                "queued",
		TrackedDownloadStatus: "ok",
		TrackedDownloadState:  "downloading",
		Size:                  payload.Size,
		Sizeleft:              payload.Size,
		Timeleft:              "",
		Protocol:              payload.Protocol,
		DownloadClient:        clientName,
		DownloadID:            payload.DownloadID,
		Indexer:               payload.Indexer,
		OutputPath:            "",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = h.store.CreateHistory(domain.HistoryRecord{
		MediaType:   "tv",
		SeriesID:    payload.SeriesID,
		EpisodeID:   payload.EpisodeID,
		SourceTitle: payload.Title,
		Quality:     map[string]any{"quality": map[string]any{"id": 7, "name": "WEBDL-1080p"}},
		EventType:   "grabbed",
		Data: map[string]any{
			"indexer":     payload.Indexer,
			"guid":        payload.Guid,
			"downloadUrl": payload.DownloadURL,
		},
	})

	writeJSON(w, http.StatusCreated, qItem)
}

func (h *Handler) handleListQualityProfiles(w http.ResponseWriter, _ *http.Request) {
	qps, err := h.store.ListQualityProfiles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, qps)
}

func (h *Handler) handleCreateQualityProfile(w http.ResponseWriter, r *http.Request) {
	var qp domain.QualityProfile
	if err := json.NewDecoder(r.Body).Decode(&qp); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	created, err := h.store.CreateQualityProfile(qp)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleGetQualityProfile(w http.ResponseWriter, _ *http.Request, id int64) {
	qps, err := h.store.ListQualityProfiles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, qp := range qps {
		if qp.ID == id {
			writeJSON(w, http.StatusOK, qp)
			return
		}
	}
	writeError(w, http.StatusNotFound, "quality profile not found")
}

func (h *Handler) handleLanguageProfiles(w http.ResponseWriter, _ *http.Request, path string) {
	profile := map[string]any{
		"id":             1,
		"name":           "English",
		"upgradeAllowed": true,
		"cutoff": map[string]any{
			"id":   1,
			"name": "English",
		},
		"languages": []map[string]any{
			{
				"language": map[string]any{
					"id":   1,
					"name": "English",
				},
				"allowed": true,
			},
		},
	}
	if strings.HasPrefix(path, "/languageprofile/") {
		writeJSON(w, http.StatusOK, profile)
		return
	}
	writeJSON(w, http.StatusOK, []any{profile})
}

func (h *Handler) handleListRootFolders(w http.ResponseWriter, _ *http.Request) {
	rfs, err := h.store.ListRootFolders("tv")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rfs)
}

func (h *Handler) handleCreateRootFolder(w http.ResponseWriter, r *http.Request) {
	var rf domain.RootFolder
	if err := json.NewDecoder(r.Body).Decode(&rf); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	rf.MediaType = "tv"
	created, err := h.store.CreateRootFolder(rf)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleListTags(w http.ResponseWriter, _ *http.Request) {
	tags, err := h.store.ListTags()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

func (h *Handler) handleCreateTag(w http.ResponseWriter, r *http.Request) {
	var t domain.Tag
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	created, err := h.store.CreateTag(t.Label)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleListDownloadClients(w http.ResponseWriter, _ *http.Request) {
	dcs, err := h.store.ListDownloadClients()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dcs)
}

func (h *Handler) handleCreateDownloadClient(w http.ResponseWriter, r *http.Request) {
	var dc domain.DownloadClient
	if err := json.NewDecoder(r.Body).Decode(&dc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	created, err := h.store.CreateDownloadClient(dc)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleListIndexers(w http.ResponseWriter, _ *http.Request) {
	idxs, err := h.store.ListIndexers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, idxs)
}

func (h *Handler) handleCreateIndexer(w http.ResponseWriter, r *http.Request) {
	var idx domain.Indexer
	if err := json.NewDecoder(r.Body).Decode(&idx); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	populateIndexerFields(&idx)
	created, err := h.store.CreateIndexer(idx)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleGetIndexer(w http.ResponseWriter, _ *http.Request, id int64) {
	idx, err := h.store.GetIndexerByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "indexer not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, idx)
}

func (h *Handler) handleUpdateIndexer(w http.ResponseWriter, r *http.Request, id int64) {
	var idx domain.Indexer
	if err := json.NewDecoder(r.Body).Decode(&idx); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	idx.ID = id
	populateIndexerFields(&idx)
	updated, err := h.store.UpdateIndexer(idx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) handleDeleteIndexer(w http.ResponseWriter, _ *http.Request, id int64) {
	if err := h.store.DeleteIndexer(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (h *Handler) handleGetHistory(w http.ResponseWriter, _ *http.Request) {
	records, err := h.store.ListHistory("tv")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"page":          1,
		"pageSize":      20,
		"sortKey":       "date",
		"sortDirection": "descending",
		"totalRecords":  len(records),
		"records":       records,
	})
}

func populateIndexerFields(idx *domain.Indexer) {
	for _, f := range idx.Fields {
		switch strings.ToLower(f.Name) {
		case "baseurl":
			if idx.BaseURL == "" {
				if s, ok := f.Value.(string); ok {
					idx.BaseURL = s
				}
			}
		case "apikey":
			if idx.APIKey == "" {
				if s, ok := f.Value.(string); ok {
					idx.APIKey = s
				}
			}
		}
	}
}

func DefaultSearchReleases(query string, mediaType string, tvdbID int, tmdbID int, season int, episode int) []domain.ReleaseResult {
	// No synthetic releases — real indexers/plugins must supply results.
	return []domain.ReleaseResult{}
}

func (h *Handler) cacheCoverAsync(created *domain.Series) {
	if h.covers == nil || created == nil {
		return
	}
	remote := mediacover.PosterRemoteURL(created.Images)
	if remote == "" {
		return
	}
	go func(id int64, remoteURL string) {
		sr, err := h.store.GetSeriesByID(id)
		if err != nil || sr == nil {
			return
		}
		cacheKey := mediacover.CacheKeyTV(sr.ImdbID, sr.TvdbID, sr.ID)
		local, err := h.covers.Ensure("tv", cacheKey, id, remoteURL, id)
		if err != nil || local == "" {
			return
		}
		updated := false
		for i := range sr.Images {
			if strings.EqualFold(sr.Images[i].CoverType, "poster") {
				sr.Images[i].CoverType = "poster"
				sr.Images[i].URL = local
				if sr.Images[i].RemoteURL == "" {
					sr.Images[i].RemoteURL = remoteURL
				}
				updated = true
				break
			}
		}
		if !updated {
			sr.Images = append(sr.Images, domain.MediaImage{CoverType: "poster", URL: local, RemoteURL: remoteURL})
		}
		_, _ = h.store.UpdateSeries(*sr)
	}(created.ID, remote)
}

func (h *Handler) handleMediaCover(w http.ResponseWriter, _ *http.Request, path string) {
	// /MediaCover/{id}/poster.jpg (cached poster bytes; Content-Type sniffed)
	rest := strings.TrimPrefix(path, "/MediaCover/")
	parts := strings.Split(rest, "/")
	if len(parts) < 1 {
		writeError(w, http.StatusNotFound, "media cover not found")
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 || h.covers == nil {
		writeError(w, http.StatusNotFound, "media cover not found")
		return
	}
	sr, gerr := h.store.GetSeriesByID(id)
	if gerr != nil || sr == nil {
		writeError(w, http.StatusNotFound, "media cover not found")
		return
	}
	cacheKey := mediacover.CacheKeyTV(sr.ImdbID, sr.TvdbID, sr.ID)
	remote := mediacover.PosterRemoteURL(sr.Images)
	f, ct, err := h.covers.Open("tv", cacheKey)
	if err != nil && remote != "" {
		if _, eerr := h.covers.Ensure("tv", cacheKey, id, remote, id); eerr == nil {
			f, ct, err = h.covers.Open("tv", cacheKey)
		}
	}
	if err != nil {
		writeError(w, http.StatusNotFound, "media cover not found")
		return
	}
	defer f.Close()
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.Copy(w, f)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"message": msg, "error": msg})
}
