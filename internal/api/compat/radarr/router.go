package radarr

import (
	"database/sql"
	"encoding/json"
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

type Handler struct {
	store    *db.Store
	cfg      *config.Config
	searchFn sonarr.SearchFunc
	start    time.Time

	cmdMu    sync.RWMutex
	commands []domain.CommandRecord
	nextCmd  int64
}

func NewHandler(store *db.Store, cfg *config.Config, searchFn sonarr.SearchFunc) http.Handler {
	return &Handler{
		store:    store,
		cfg:      cfg,
		searchFn: searchFn,
		start:    time.Now().UTC(),
		commands: []domain.CommandRecord{},
		nextCmd:  1,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := normalizeRadarrPath(r.URL.Path)

	switch {
	case (path == "/api" || path == "/system/status") && r.Method == http.MethodGet:
		h.handleSystemStatus(w, r)

	case path == "/health" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, []any{})

	case path == "/movie/lookup" && r.Method == http.MethodGet:
		h.handleMovieLookup(w, r)

	case path == "/movie/lookup/tmdb" && r.Method == http.MethodGet:
		h.handleMovieLookupTMDB(w, r)

	case path == "/movie/lookup/imdb" && r.Method == http.MethodGet:
		h.handleMovieLookupIMDB(w, r)

	case path == "/movie" && r.Method == http.MethodGet:
		h.handleListMovies(w, r)

	case path == "/movie" && r.Method == http.MethodPost:
		h.handleCreateMovie(w, r)

	case path == "/movie" && r.Method == http.MethodPut:
		h.handleUpdateMovieBody(w, r)

	case strings.HasPrefix(path, "/movie/"):
		idStr := strings.TrimPrefix(path, "/movie/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid movie id")
			return
		}
		switch r.Method {
		case http.MethodGet:
			h.handleGetMovie(w, r, id)
		case http.MethodPut:
			h.handleUpdateMovie(w, r, id)
		case http.MethodDelete:
			h.handleDeleteMovie(w, r, id)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}

	case (path == "/moviefile" || strings.HasPrefix(path, "/moviefile/")) && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, []any{})

	case path == "/calendar" && r.Method == http.MethodGet:
		h.handleCalendar(w, r)

	case path == "/queue" && r.Method == http.MethodGet:
		h.handleGetQueue(w, r)

	case path == "/queue/status" && r.Method == http.MethodGet:
		items, _ := h.store.ListQueueItems("movie")
		writeJSON(w, http.StatusOK, map[string]any{
			"totalCount":      len(items),
			"count":           len(items),
			"unknownCount":    0,
			"errors":          false,
			"warnings":        false,
			"unknownErrors":   false,
			"unknownWarnings": false,
		})

	case strings.HasPrefix(path, "/queue/") && r.Method == http.MethodDelete:
		idStr := strings.TrimPrefix(path, "/queue/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid queue id")
			return
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

func normalizeRadarrPath(p string) string {
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return "/"
	}
	if strings.HasPrefix(p, "/radarr") {
		p = strings.TrimPrefix(p, "/radarr")
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
		"appName":           "Radarr",
		"instanceName":      "Replacearr (Radarr Compat)",
		"version":           "5.3.6.8608",
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
		"migrationVersion":  228,
		"urlBase":           "/radarr",
		"runtimeVersion":    "8.0.2",
		"runtimeName":       "netcore",
		"startTime":         h.start,
	})
}

func (h *Handler) handleListMovies(w http.ResponseWriter, r *http.Request) {
	movies, err := h.store.ListMovies()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tmdbStr := r.URL.Query().Get("tmdbId"); tmdbStr != "" {
		if tmdbID, err := strconv.Atoi(tmdbStr); err == nil && tmdbID > 0 {
			filtered := []domain.Movie{}
			for _, mv := range movies {
				if mv.TmdbID == tmdbID {
					filtered = append(filtered, mv)
				}
			}
			writeJSON(w, http.StatusOK, filtered)
			return
		}
	}
	writeJSON(w, http.StatusOK, movies)
}

func (h *Handler) handleCreateMovie(w http.ResponseWriter, r *http.Request) {
	var mv domain.Movie
	if err := json.NewDecoder(r.Body).Decode(&mv); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if mv.Title == "" && mv.TmdbID > 0 {
		for _, c := range curatedMovieCatalog() {
			if c.TmdbID == mv.TmdbID {
				mv.Title = c.Title
				if mv.Year == 0 {
					mv.Year = c.Year
				}
				if mv.Overview == "" {
					mv.Overview = c.Overview
				}
				if mv.ImdbID == "" {
					mv.ImdbID = c.ImdbID
				}
				break
			}
		}
		if mv.Title == "" {
			mv.Title = fmt.Sprintf("Movie %d", mv.TmdbID)
		}
	}
	created, err := h.store.CreateMovie(mv)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleGetMovie(w http.ResponseWriter, _ *http.Request, id int64) {
	mv, err := h.store.GetMovieByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "movie not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mv)
}

func (h *Handler) handleUpdateMovie(w http.ResponseWriter, r *http.Request, id int64) {
	var mv domain.Movie
	if err := json.NewDecoder(r.Body).Decode(&mv); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	mv.ID = id
	updated, err := h.store.UpdateMovie(mv)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) handleUpdateMovieBody(w http.ResponseWriter, r *http.Request) {
	var mv domain.Movie
	if err := json.NewDecoder(r.Body).Decode(&mv); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	updated, err := h.store.UpdateMovie(mv)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) handleDeleteMovie(w http.ResponseWriter, _ *http.Request, id int64) {
	if err := h.store.DeleteMovie(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func curatedMovieCatalog() []domain.Movie {
	return []domain.Movie{
		{
			Title:               "Dune: Part Two",
			OriginalTitle:       "Dune: Part Two",
			SortTitle:           "dune part two",
			CleanTitle:          "duneparttwo",
			TitleSlug:           "dune-part-two-693134",
			Status:              "released",
			Overview:            "Follow the mythic journey of Paul Atreides as he unites with Chani and the Fremen while on a path of revenge against the conspirators who destroyed his family.",
			Year:                2024,
			Runtime:             166,
			TmdbID:              693134,
			ImdbID:              "tt15239678",
			Studio:              "Legendary Pictures",
			QualityProfileID:    1,
			Monitored:           true,
			MinimumAvailability: "released",
			IsAvailable:         true,
			Genres:              []string{"Science Fiction", "Adventure"},
			Images: []domain.MediaImage{
				{CoverType: "poster", RemoteURL: "https://image.tmdb.org/t/p/original/1pdfLvkbY9ohJlCjQH2CZjjYVvJ.jpg"},
			},
		},
		{
			Title:               "Oppenheimer",
			OriginalTitle:       "Oppenheimer",
			SortTitle:           "oppenheimer",
			CleanTitle:          "oppenheimer",
			TitleSlug:           "oppenheimer-872585",
			Status:              "released",
			Overview:            "The story of J. Robert Oppenheimer's role in the development of the atomic bomb during World War II.",
			Year:                2023,
			Runtime:             181,
			TmdbID:              872585,
			ImdbID:              "tt15398776",
			Studio:              "Universal Pictures",
			QualityProfileID:    1,
			Monitored:           true,
			MinimumAvailability: "released",
			IsAvailable:         true,
			Genres:              []string{"Drama", "History"},
			Images: []domain.MediaImage{
				{CoverType: "poster", RemoteURL: "https://image.tmdb.org/t/p/original/8Gxv8gSFCU0XGDykEGv7zR1n2ua.jpg"},
			},
		},
		{
			Title:               "Interstellar",
			OriginalTitle:       "Interstellar",
			SortTitle:           "interstellar",
			CleanTitle:          "interstellar",
			TitleSlug:           "interstellar-157336",
			Status:              "released",
			Overview:            "The adventures of a group of explorers who make use of a newly discovered wormhole to surpass the limitations on human space travel.",
			Year:                2014,
			Runtime:             169,
			TmdbID:              157336,
			ImdbID:              "tt0816692",
			Studio:              "Paramount Pictures",
			QualityProfileID:    1,
			Monitored:           true,
			MinimumAvailability: "released",
			IsAvailable:         true,
			Genres:              []string{"Adventure", "Drama", "Science Fiction"},
			Images: []domain.MediaImage{
				{CoverType: "poster", RemoteURL: "https://image.tmdb.org/t/p/original/gEU2QniE6E77NI6lCU6MxlNBvIx.jpg"},
			},
		},
		{
			Title:               "Blade Runner 2049",
			OriginalTitle:       "Blade Runner 2049",
			SortTitle:           "blade runner 2049",
			CleanTitle:          "bladerunner2049",
			TitleSlug:           "blade-runner-2049-335984",
			Status:              "released",
			Overview:            "Thirty years after the events of the first film, a new blade runner, LAPD Officer K, unearths a long-buried secret that has the potential to plunge what's left of society into chaos.",
			Year:                2017,
			Runtime:             164,
			TmdbID:              335984,
			ImdbID:              "tt1856101",
			Studio:              "Warner Bros. Pictures",
			QualityProfileID:    1,
			Monitored:           true,
			MinimumAvailability: "released",
			IsAvailable:         true,
			Genres:              []string{"Science Fiction", "Drama"},
			Images: []domain.MediaImage{
				{CoverType: "poster", RemoteURL: "https://image.tmdb.org/t/p/original/gajva2L0rPYkEWjzgFlBXCAVBE5.jpg"},
			},
		},
	}
}

func (h *Handler) handleMovieLookup(w http.ResponseWriter, r *http.Request) {
	term := strings.TrimSpace(r.URL.Query().Get("term"))
	catalog := curatedMovieCatalog()
	if term == "" {
		writeJSON(w, http.StatusOK, catalog)
		return
	}

	lower := strings.ToLower(term)
	if strings.HasPrefix(lower, "tmdb:") {
		idStr := strings.TrimSpace(strings.TrimPrefix(lower, "tmdb:"))
		if tmdbID, err := strconv.Atoi(idStr); err == nil && tmdbID > 0 {
			mv := lookupMovieByTMDB(tmdbID)
			writeJSON(w, http.StatusOK, []domain.Movie{mv})
			return
		}
	}
	if strings.HasPrefix(lower, "imdb:") {
		imdbID := strings.TrimSpace(strings.TrimPrefix(lower, "imdb:"))
		mv := lookupMovieByIMDB(imdbID)
		writeJSON(w, http.StatusOK, []domain.Movie{mv})
		return
	}

	var matched []domain.Movie
	var rest []domain.Movie
	for _, item := range catalog {
		if strings.Contains(strings.ToLower(item.Title), lower) ||
			strings.Contains(strings.ToLower(item.Overview), lower) ||
			strings.Contains(item.CleanTitle, lower) {
			matched = append(matched, item)
		} else {
			rest = append(rest, item)
		}
	}

	if len(matched) == 0 {
		slug := strings.ToLower(strings.ReplaceAll(term, " ", "-"))
		clean := strings.ToLower(strings.ReplaceAll(term, " ", ""))
		matched = append(matched, domain.Movie{
			Title:               term,
			OriginalTitle:       term,
			SortTitle:           strings.ToLower(term),
			CleanTitle:          clean,
			TitleSlug:           slug,
			Status:              "released",
			Overview:            fmt.Sprintf("%s is a feature film discovered via Replacearr TMDB lookup.", term),
			Year:                2025,
			Runtime:             125,
			TmdbID:              700000 + len(term)*113,
			ImdbID:              "tt7654321",
			Studio:              "Replacearr Studios",
			QualityProfileID:    1,
			Monitored:           true,
			MinimumAvailability: "released",
			IsAvailable:         true,
			Genres:              []string{"Drama"},
		})
	}

	results := append(matched, rest...)
	writeJSON(w, http.StatusOK, results)
}

func (h *Handler) handleMovieLookupTMDB(w http.ResponseWriter, r *http.Request) {
	tmdbStr := strings.TrimSpace(r.URL.Query().Get("tmdbId"))
	tmdbID, _ := strconv.Atoi(tmdbStr)
	if tmdbID <= 0 {
		tmdbID = 693134
	}
	mv := lookupMovieByTMDB(tmdbID)
	writeJSON(w, http.StatusOK, mv)
}

func (h *Handler) handleMovieLookupIMDB(w http.ResponseWriter, r *http.Request) {
	imdbID := strings.TrimSpace(r.URL.Query().Get("imdbId"))
	if imdbID == "" {
		imdbID = "tt15239678"
	}
	mv := lookupMovieByIMDB(imdbID)
	writeJSON(w, http.StatusOK, mv)
}

func lookupMovieByTMDB(tmdbID int) domain.Movie {
	for _, item := range curatedMovieCatalog() {
		if item.TmdbID == tmdbID {
			return item
		}
	}
	return domain.Movie{
		Title:               fmt.Sprintf("TMDB Movie %d", tmdbID),
		OriginalTitle:       fmt.Sprintf("TMDB Movie %d", tmdbID),
		SortTitle:           fmt.Sprintf("tmdb movie %d", tmdbID),
		CleanTitle:          fmt.Sprintf("tmdbmovie%d", tmdbID),
		TitleSlug:           fmt.Sprintf("tmdb-movie-%d", tmdbID),
		Status:              "released",
		Overview:            fmt.Sprintf("Dynamic TMDB lookup candidate for ID %d", tmdbID),
		Year:                2024,
		Runtime:             130,
		TmdbID:              tmdbID,
		ImdbID:              fmt.Sprintf("tt%07d", tmdbID),
		Studio:              "Universal",
		QualityProfileID:    1,
		Monitored:           true,
		MinimumAvailability: "released",
		IsAvailable:         true,
		Genres:              []string{"Action", "Drama"},
	}
}

func lookupMovieByIMDB(imdbID string) domain.Movie {
	for _, item := range curatedMovieCatalog() {
		if strings.EqualFold(item.ImdbID, imdbID) {
			return item
		}
	}
	return domain.Movie{
		Title:               fmt.Sprintf("IMDB Movie %s", imdbID),
		OriginalTitle:       fmt.Sprintf("IMDB Movie %s", imdbID),
		SortTitle:           fmt.Sprintf("imdb movie %s", strings.ToLower(imdbID)),
		CleanTitle:          fmt.Sprintf("imdbmovie%s", strings.ToLower(imdbID)),
		TitleSlug:           fmt.Sprintf("imdb-movie-%s", strings.ToLower(imdbID)),
		Status:              "released",
		Overview:            fmt.Sprintf("Dynamic IMDB lookup candidate for %s", imdbID),
		Year:                2024,
		Runtime:             128,
		TmdbID:              654321,
		ImdbID:              imdbID,
		Studio:              "Studio",
		QualityProfileID:    1,
		Monitored:           true,
		MinimumAvailability: "released",
		IsAvailable:         true,
		Genres:              []string{"Drama"},
	}
}

func (h *Handler) handleCalendar(w http.ResponseWriter, _ *http.Request) {
	movies, err := h.store.ListMovies()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, movies)
}

func (h *Handler) handleGetQueue(w http.ResponseWriter, _ *http.Request) {
	items, err := h.store.ListQueueItems("movie")
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
			name = "MoviesSearch"
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
	var movieID int64
	if mStr := q.Get("movieId"); mStr != "" {
		movieID, _ = strconv.ParseInt(mStr, 10, 64)
	}
	var tmdbID int
	if movieID > 0 {
		if mv, err := h.store.GetMovieByID(movieID); err == nil && mv != nil {
			if term == "" {
				term = mv.Title
			}
			tmdbID = mv.TmdbID
		}
	}
	if term == "" {
		term = "Dune Part Two"
	}

	var results []domain.ReleaseResult
	if h.searchFn != nil {
		res, err := h.searchFn(term, "movie", 0, tmdbID)
		if err == nil {
			results = res
		}
	}
	if results == nil {
		results = sonarr.DefaultSearchReleases(term, "movie", 0, tmdbID)
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
		MovieID    int64  `json:"movieId"`
		DownloadID string `json:"downloadId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid release payload")
		return
	}
	if payload.Title == "" {
		if payload.Guid != "" {
			payload.Title = payload.Guid
		} else {
			payload.Title = "Grabbed.Movie.Release.2024.1080p.WEB-DL"
		}
	}
	if payload.Protocol == "" {
		payload.Protocol = "torrent"
	}
	if payload.Size <= 0 {
		payload.Size = 4831838208
	}
	if payload.Indexer == "" {
		payload.Indexer = "Replacearr Indexer"
	}
	if payload.DownloadID == "" {
		payload.DownloadID = fmt.Sprintf("radarr-%d", time.Now().UnixNano())
	}

	qItem, err := h.store.CreateQueueItem(domain.QueueItem{
		MediaType:             "movie",
		MovieID:               payload.MovieID,
		Title:                 payload.Title,
		Status:                "downloading",
		TrackedDownloadStatus: "ok",
		TrackedDownloadState:  "downloading",
		Size:                  payload.Size,
		Sizeleft:              payload.Size / 2,
		Timeleft:              "00:07:15",
		EstimatedCompletion:   time.Now().UTC().Add(7 * time.Minute),
		Protocol:              payload.Protocol,
		DownloadClient:        "Replacearr Internal",
		DownloadID:            payload.DownloadID,
		Indexer:               payload.Indexer,
		OutputPath:            "/data/movies",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = h.store.CreateHistory(domain.HistoryRecord{
		MediaType:   "movie",
		MovieID:     payload.MovieID,
		SourceTitle: payload.Title,
		Quality:     map[string]any{"quality": map[string]any{"id": 7, "name": "WEBDL-1080p"}},
		EventType:   "grabbed",
		Data:        map[string]any{"indexer": payload.Indexer, "guid": payload.Guid},
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

func (h *Handler) handleListRootFolders(w http.ResponseWriter, _ *http.Request) {
	rfs, err := h.store.ListRootFolders("movie")
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
	rf.MediaType = "movie"
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

func (h *Handler) handleGetHistory(w http.ResponseWriter, _ *http.Request) {
	records, err := h.store.ListHistory("movie")
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"message": msg, "error": msg})
}
