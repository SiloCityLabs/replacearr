package prowlarr

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/replacearr/replacearr/internal/api/compat/sonarr"
	"github.com/replacearr/replacearr/internal/config"
	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/domain"
)

type ApplicationProfile struct {
	ID                 int64                `json:"id"`
	Name               string               `json:"name"`
	Implementation     string               `json:"implementation"`
	ImplementationName string               `json:"implementationName"`
	ConfigContract     string               `json:"configContract"`
	SyncLevel          string               `json:"syncLevel"`
	Fields             []domain.ConfigField `json:"fields"`
	Tags               []int64              `json:"tags"`
}

type Handler struct {
	store    *db.Store
	cfg      *config.Config
	searchFn sonarr.SearchFunc
	start    time.Time

	appMu   sync.RWMutex
	apps    []ApplicationProfile
	nextApp int64
}

func NewHandler(store *db.Store, cfg *config.Config, searchFn sonarr.SearchFunc) http.Handler {
	port := 8989
	if cfg != nil && cfg.Port > 0 {
		port = cfg.Port
	}
	prowlarrURL := fmt.Sprintf("http://localhost:%d/prowlarr", port)
	sonarrURL := fmt.Sprintf("http://localhost:%d/sonarr", port)
	radarrURL := fmt.Sprintf("http://localhost:%d/radarr", port)

	defaultApps := []ApplicationProfile{
		{
			ID:                 1,
			Name:               "Replacearr Sonarr (Internal)",
			Implementation:     "Sonarr",
			ImplementationName: "Sonarr",
			ConfigContract:     "SonarrSettings",
			SyncLevel:          "fullSync",
			Fields: []domain.ConfigField{
				{Name: "prowlarrUrl", Label: "Prowlarr Server", Type: "textbox", Value: prowlarrURL},
				{Name: "baseUrl", Label: "Sonarr Server", Type: "textbox", Value: sonarrURL},
				{Name: "apiKey", Label: "API Key", Type: "password", Value: "replacearr-internal-key"},
				{Name: "syncCategories", Label: "Sync Categories", Type: "select", Value: []int{5000, 5030, 5040, 5045}},
			},
			Tags: []int64{},
		},
		{
			ID:                 2,
			Name:               "Replacearr Radarr (Internal)",
			Implementation:     "Radarr",
			ImplementationName: "Radarr",
			ConfigContract:     "RadarrSettings",
			SyncLevel:          "fullSync",
			Fields: []domain.ConfigField{
				{Name: "prowlarrUrl", Label: "Prowlarr Server", Type: "textbox", Value: prowlarrURL},
				{Name: "baseUrl", Label: "Radarr Server", Type: "textbox", Value: radarrURL},
				{Name: "apiKey", Label: "API Key", Type: "password", Value: "replacearr-internal-key"},
				{Name: "syncCategories", Label: "Sync Categories", Type: "select", Value: []int{2000, 2030, 2040, 2045}},
			},
			Tags: []int64{},
		},
	}

	return &Handler{
		store:    store,
		cfg:      cfg,
		searchFn: searchFn,
		start:    time.Now().UTC(),
		apps:     defaultApps,
		nextApp:  3,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSuffix(r.URL.Path, "/")
	if strings.HasPrefix(rawPath, "/prowlarr") {
		rawPath = strings.TrimPrefix(rawPath, "/prowlarr")
	}
	if rawPath == "" {
		rawPath = "/"
	}

	// Check for Torznab/Newznab feed routes first:
	// 1. GET /api?t=...
	// 2. GET /{indexerId}/api?t=...
	// 3. GET /api/v1/indexer/{indexerId}/newznab?t=...
	if r.Method == http.MethodGet {
		if rawPath == "/api" && r.URL.Query().Get("t") != "" {
			h.handleTorznab(w, r, 0)
			return
		}
		if id, ok := matchTorznabPath(rawPath); ok {
			h.handleTorznab(w, r, id)
			return
		}
	}

	path := normalizeProwlarrPath(rawPath)

	switch {
	case (path == "/api" || path == "/system/status") && r.Method == http.MethodGet:
		h.handleSystemStatus(w, r)

	case path == "/health" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, []any{})

	case path == "/indexerstatus" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, []any{})

	case strings.EqualFold(path, "/indexerproxy/schema") && r.Method == http.MethodGet:
		h.handleIndexerProxySchema(w, r)

	case (strings.EqualFold(path, "/indexerproxy/test") || strings.EqualFold(path, "/indexerproxy/testall")) && r.Method == http.MethodPost:
		writeJSON(w, http.StatusOK, map[string]any{"isValid": true})

	case strings.EqualFold(path, "/indexerproxy") && r.Method == http.MethodGet:
		h.handleListIndexerProxies(w, r)

	case strings.EqualFold(path, "/indexerproxy") && r.Method == http.MethodPost:
		h.handleCreateIndexerProxy(w, r)

	case strings.HasPrefix(strings.ToLower(path), "/indexerproxy/"):
		idStr := path[len("/indexerproxy/"):]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid indexerproxy id")
			return
		}
		switch r.Method {
		case http.MethodGet:
			h.handleGetIndexerProxy(w, r, id)
		case http.MethodPut:
			h.handleUpdateIndexerProxy(w, r, id)
		case http.MethodDelete:
			h.handleDeleteIndexerProxy(w, r, id)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}

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

	case path == "/search" && r.Method == http.MethodGet:
		h.handleSearch(w, r)

	case (path == "/search" || path == "/release") && r.Method == http.MethodPost:
		h.handleGrabRelease(w, r)

	case path == "/applications" && r.Method == http.MethodGet:
		h.handleListApplications(w, r)

	case path == "/applications" && r.Method == http.MethodPost:
		h.handleCreateApplication(w, r)

	case strings.HasPrefix(path, "/applications/") && r.Method == http.MethodDelete:
		idStr := strings.TrimPrefix(path, "/applications/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid application id")
			return
		}
		h.handleDeleteApplication(w, r, id)

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

	default:
		writeError(w, http.StatusNotFound, "endpoint not found: "+r.URL.Path)
	}
}

func matchTorznabPath(rawPath string) (int64, bool) {
	// Matches /{indexerId}/api or /api/v1/indexer/{indexerId}/newznab
	parts := strings.Split(strings.Trim(rawPath, "/"), "/")
	if len(parts) == 2 && parts[1] == "api" {
		if id, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
			return id, true
		}
		if parts[0] == "all" {
			return 0, true
		}
	}
	if len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "indexer" && (parts[4] == "newznab" || parts[4] == "torznab" || parts[4] == "api") {
		if id, err := strconv.ParseInt(parts[3], 10, 64); err == nil {
			return id, true
		}
	}
	return 0, false
}

func normalizeProwlarrPath(rawPath string) string {
	if rawPath == "" || rawPath == "/api" {
		return "/api"
	}
	p := rawPath
	if strings.HasPrefix(p, "/api/v1") {
		p = strings.TrimPrefix(p, "/api/v1")
	} else if strings.HasPrefix(p, "/v1") {
		p = strings.TrimPrefix(p, "/v1")
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
		"appName":           "Prowlarr",
		"instanceName":      "Replacearr (Prowlarr Compat)",
		"version":           "1.14.2.4330",
		"buildTime":         "2026-01-01T00:00:00Z",
		"isDebug":           false,
		"isProduction":      true,
		"isAdmin":           true,
		"isUserInteractive": false,
		"startupPath":       "/app",
		"appData":           appData,
		"osName":            "linux",
		"osVersion":         "6.6.0",
		"isNetCore":         true,
		"isLinux":           true,
		"isOsx":             false,
		"isWindows":         false,
		"isDocker":          true,
		"mode":              "console",
		"branch":            "master",
		"authentication":    "apiKey",
		"sqliteVersion":     "3.45.0",
		"migrationVersion":  52,
		"urlBase":           "/prowlarr",
		"runtimeVersion":    "8.0.2",
		"runtimeName":       "netcore",
		"startTime":         h.start,
	})
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
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
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

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := strings.TrimSpace(q.Get("query"))
	if query == "" {
		query = strings.TrimSpace(q.Get("q"))
	}
	searchType := strings.ToLower(strings.TrimSpace(q.Get("type")))
	mediaType := "tv"
	if searchType == "movie" || searchType == "moviesearch" {
		mediaType = "movie"
	} else if searchType == "tv" || searchType == "tvsearch" {
		mediaType = "tv"
	}
	tvdbID, _ := strconv.Atoi(q.Get("tvdbId"))
	tmdbID, _ := strconv.Atoi(q.Get("tmdbId"))

	var results []domain.ReleaseResult
	if h.searchFn != nil {
		res, err := h.searchFn(query, mediaType, tvdbID, tmdbID)
		if err == nil {
			results = res
		}
	}
	if results == nil {
		results = sonarr.DefaultSearchReleases(query, mediaType, tvdbID, tmdbID)
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *Handler) handleGrabRelease(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Guid       string `json:"guid"`
		Title      string `json:"title"`
		IndexerID  int64  `json:"indexerId"`
		Indexer    string `json:"indexer"`
		Size       int64  `json:"size"`
		Protocol   string `json:"protocol"`
		MediaType  string `json:"mediaType"`
		Categories []int  `json:"categories"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid search grab payload")
		return
	}
	if payload.Title == "" {
		if payload.Guid != "" {
			payload.Title = payload.Guid
		} else {
			payload.Title = "Prowlarr.Grabbed.Release.1080p.WEB-DL"
		}
	}
	if payload.Protocol == "" {
		payload.Protocol = "torrent"
	}
	if payload.Size <= 0 {
		payload.Size = 2147483648
	}
	if payload.Indexer == "" {
		payload.Indexer = "Prowlarr Indexer"
	}
	mediaType := payload.MediaType
	if mediaType == "" {
		mediaType = "tv"
		for _, c := range payload.Categories {
			if c >= 2000 && c < 3000 {
				mediaType = "movie"
				break
			}
		}
	}

	qItem, err := h.store.CreateQueueItem(domain.QueueItem{
		MediaType:             mediaType,
		Title:                 payload.Title,
		Status:                "downloading",
		TrackedDownloadStatus: "ok",
		TrackedDownloadState:  "downloading",
		Size:                  payload.Size,
		Sizeleft:              payload.Size / 2,
		Timeleft:              "00:06:00",
		EstimatedCompletion:   time.Now().UTC().Add(6 * time.Minute),
		Protocol:              payload.Protocol,
		DownloadClient:        "Replacearr Internal",
		DownloadID:            fmt.Sprintf("prowlarr-%d", time.Now().UnixNano()),
		Indexer:               payload.Indexer,
		OutputPath:            "/data/" + mediaType,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, qItem)
}

func (h *Handler) handleListApplications(w http.ResponseWriter, _ *http.Request) {
	h.appMu.RLock()
	defer h.appMu.RUnlock()
	out := make([]ApplicationProfile, len(h.apps))
	copy(out, h.apps)
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) handleCreateApplication(w http.ResponseWriter, r *http.Request) {
	var app ApplicationProfile
	if err := json.NewDecoder(r.Body).Decode(&app); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if app.Implementation == "" {
		app.Implementation = "Sonarr"
	}
	if app.ImplementationName == "" {
		app.ImplementationName = app.Implementation
	}
	if app.ConfigContract == "" {
		app.ConfigContract = app.Implementation + "Settings"
	}
	if app.SyncLevel == "" {
		app.SyncLevel = "fullSync"
	}
	if app.Fields == nil {
		app.Fields = []domain.ConfigField{}
	}
	if app.Tags == nil {
		app.Tags = []int64{}
	}

	h.appMu.Lock()
	app.ID = h.nextApp
	h.nextApp++
	h.apps = append(h.apps, app)
	h.appMu.Unlock()

	writeJSON(w, http.StatusCreated, app)
}

func (h *Handler) handleDeleteApplication(w http.ResponseWriter, _ *http.Request, id int64) {
	h.appMu.Lock()
	filtered := make([]ApplicationProfile, 0, len(h.apps))
	for _, a := range h.apps {
		if a.ID != id {
			filtered = append(filtered, a)
		}
	}
	h.apps = filtered
	h.appMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{})
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

func (h *Handler) handleTorznab(w http.ResponseWriter, r *http.Request, indexerID int64) {
	q := r.URL.Query()
	action := strings.ToLower(strings.TrimSpace(q.Get("t")))
	output := strings.ToLower(strings.TrimSpace(q.Get("o")))
	if output == "" {
		output = strings.ToLower(strings.TrimSpace(q.Get("output")))
	}

	indexerName := "Replacearr Torznab"
	if indexerID > 0 {
		if idx, err := h.store.GetIndexerByID(indexerID); err == nil && idx != nil && idx.Name != "" {
			indexerName = idx.Name
		}
	}

	if action == "caps" {
		if output == "json" {
			writeJSON(w, http.StatusOK, map[string]any{
				"server": map[string]any{
					"version":   "1.1",
					"title":     indexerName,
					"strapline": "Replacearr Unified Torznab Feed",
				},
				"limits": map[string]any{
					"max":     100,
					"default": 100,
				},
				"searching": map[string]any{
					"search":       map[string]any{"available": "yes", "supportedParams": "q"},
					"tv-search":    map[string]any{"available": "yes", "supportedParams": "q,season,ep,tvdbid,rid"},
					"movie-search": map[string]any{"available": "yes", "supportedParams": "q,imdbid,tmdbid"},
				},
				"categories": []map[string]any{
					{"id": 2000, "name": "Movies"},
					{"id": 5000, "name": "TV"},
				},
			})
			return
		}

		capsXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<caps>
  <server version="1.1" title="%s" strapline="Replacearr Unified Torznab Feed" />
  <limits max="100" default="100" />
  <searching>
    <search available="yes" supportedParams="q" />
    <tv-search available="yes" supportedParams="q,season,ep,tvdbid,rid" />
    <movie-search available="yes" supportedParams="q,imdbid,tmdbid" />
  </searching>
  <categories>
    <category id="2000" name="Movies">
      <subcat id="2030" name="Movies/SD" />
      <subcat id="2040" name="Movies/HD" />
      <subcat id="2045" name="Movies/UHD" />
    </category>
    <category id="5000" name="TV">
      <subcat id="5030" name="TV/SD" />
      <subcat id="5040" name="TV/HD" />
      <subcat id="5045" name="TV/UHD" />
    </category>
  </categories>
</caps>`, xmlEscape(indexerName))
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(capsXML))
		return
	}

	mediaType := "tv"
	switch action {
	case "movie", "moviesearch":
		mediaType = "movie"
	case "tvsearch", "tv":
		mediaType = "tv"
	}

	query := strings.TrimSpace(q.Get("q"))
	if query == "" {
		query = strings.TrimSpace(q.Get("query"))
	}
	tvdbID, _ := strconv.Atoi(q.Get("tvdbid"))
	tmdbID, _ := strconv.Atoi(q.Get("tmdbid"))

	var results []domain.ReleaseResult
	if h.searchFn != nil {
		res, err := h.searchFn(query, mediaType, tvdbID, tmdbID)
		if err == nil {
			results = res
		}
	}
	if results == nil {
		results = sonarr.DefaultSearchReleases(query, mediaType, tvdbID, tmdbID)
	}

	if output == "json" {
		writeJSON(w, http.StatusOK, map[string]any{
			"channel": map[string]any{
				"title":       indexerName,
				"description": "Replacearr Torznab Feed",
				"item":        results,
			},
			"releases": results,
		})
		return
	}

	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(`<rss version="2.0" xmlns:torznab="http://torznab.com/schemas/2015/feed">` + "\n")
	buf.WriteString("  <channel>\n")
	buf.WriteString(fmt.Sprintf("    <title>%s</title>\n", xmlEscape(indexerName)))
	buf.WriteString("    <description>Replacearr Torznab Feed</description>\n")

	for _, rel := range results {
		pubDate := rel.PublishDate
		if pubDate.IsZero() {
			pubDate = time.Now().UTC()
		}
		dlURL := rel.DownloadURL
		if dlURL == "" {
			dlURL = fmt.Sprintf("http://localhost:8989/prowlarr/%d/download?guid=%s", indexerID, rel.Guid)
		}
		infoURL := rel.InfoURL
		if infoURL == "" {
			infoURL = dlURL
		}
		cats := rel.Categories
		if len(cats) == 0 {
			if mediaType == "movie" {
				cats = []int{2000, 2040}
			} else {
				cats = []int{5000, 5040}
			}
		}

		buf.WriteString("    <item>\n")
		buf.WriteString(fmt.Sprintf("      <title>%s</title>\n", xmlEscape(rel.Title)))
		buf.WriteString(fmt.Sprintf("      <guid>%s</guid>\n", xmlEscape(rel.Guid)))
		buf.WriteString(fmt.Sprintf("      <link>%s</link>\n", xmlEscape(dlURL)))
		buf.WriteString(fmt.Sprintf("      <comments>%s</comments>\n", xmlEscape(infoURL)))
		buf.WriteString(fmt.Sprintf("      <pubDate>%s</pubDate>\n", pubDate.UTC().Format(time.RFC1123Z)))
		buf.WriteString(fmt.Sprintf("      <size>%d</size>\n", rel.Size))
		buf.WriteString(fmt.Sprintf(`      <enclosure url="%s" length="%d" type="application/x-bittorrent" />`+"\n", xmlEscape(dlURL), rel.Size))
		for _, cat := range cats {
			buf.WriteString(fmt.Sprintf(`      <torznab:attr name="category" value="%d" />`+"\n", cat))
		}
		buf.WriteString(fmt.Sprintf(`      <torznab:attr name="seeders" value="%d" />`+"\n", rel.Seeders))
		buf.WriteString(fmt.Sprintf(`      <torznab:attr name="peers" value="%d" />`+"\n", rel.Peers))
		buf.WriteString(fmt.Sprintf(`      <torznab:attr name="size" value="%d" />`+"\n", rel.Size))
		buf.WriteString(`      <torznab:attr name="downloadvolumefactor" value="0" />` + "\n")
		buf.WriteString(`      <torznab:attr name="uploadvolumefactor" value="1" />` + "\n")
		buf.WriteString("    </item>\n")
	}

	buf.WriteString("  </channel>\n")
	buf.WriteString("</rss>")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"message": msg, "error": msg})
}

func (h *Handler) handleIndexerProxySchema(w http.ResponseWriter, _ *http.Request) {
	schemas := []domain.IndexerProxy{
		{
			Name:               "FlareSolverr",
			Implementation:     "FlareSolverr",
			ImplementationName: "FlareSolverr",
			ConfigContract:     "FlareSolverrSettings",
			Host:               "http://localhost:8191/",
			RequestTimeout:     60,
			SessionTTLMinutes:  15,
			Enable:             true,
			OnHealthIssue:      true,
			SupportsOnHealth:   true,
			Fields: []domain.ConfigField{
				{Name: "host", Label: "Host", Type: "textbox", Value: "http://localhost:8191/", HelpText: "FlareSolverr server URL"},
				{Name: "requestTimeout", Label: "Request Timeout", Type: "number", Value: 60, HelpText: "FlareSolverr maxTimeout Request Parameter (seconds, 1-180)"},
			},
			Tags: []int64{},
		},
		{
			Name:               "Http",
			Implementation:     "Http",
			ImplementationName: "Http",
			ConfigContract:     "HttpSettings",
			Host:               "http://localhost:8080",
			RequestTimeout:     60,
			Enable:             true,
			Fields: []domain.ConfigField{
				{Name: "host", Label: "Host", Type: "textbox", Value: "localhost"},
				{Name: "port", Label: "Port", Type: "number", Value: 8080},
			},
			Tags: []int64{},
		},
		{
			Name:               "Socks5",
			Implementation:     "Socks5",
			ImplementationName: "Socks5",
			ConfigContract:     "Socks5Settings",
			Host:               "socks5://localhost:1080",
			RequestTimeout:     60,
			Enable:             true,
			Fields: []domain.ConfigField{
				{Name: "host", Label: "Host", Type: "textbox", Value: "localhost"},
				{Name: "port", Label: "Port", Type: "number", Value: 1080},
			},
			Tags: []int64{},
		},
	}
	writeJSON(w, http.StatusOK, schemas)
}

func (h *Handler) handleListIndexerProxies(w http.ResponseWriter, _ *http.Request) {
	items, err := h.store.ListIndexerProxies()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) handleCreateIndexerProxy(w http.ResponseWriter, r *http.Request) {
	var req domain.IndexerProxy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if !req.Enable {
		req.Enable = true
	}
	created, err := h.store.CreateIndexerProxy(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleGetIndexerProxy(w http.ResponseWriter, _ *http.Request, id int64) {
	item, err := h.store.GetIndexerProxyByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "indexer proxy not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) handleUpdateIndexerProxy(w http.ResponseWriter, r *http.Request, id int64) {
	var req domain.IndexerProxy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	req.ID = id
	updated, err := h.store.UpdateIndexerProxy(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) handleDeleteIndexerProxy(w http.ResponseWriter, _ *http.Request, id int64) {
	if err := h.store.DeleteIndexerProxy(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}
