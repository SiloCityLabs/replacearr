// Package lidarr provides a thin Lidarr-compatible API mount for migration tools.
package lidarr

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/config"
	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/domain"
)

// SearchFunc matches native SearchReleases signature.
type SearchFunc func(query, mediaType string, tvdbID, tmdbID, season, episode int) ([]domain.ReleaseResult, error)

// Handler serves /lidarr/api/v1/* with a pragmatic Lidarr-shaped surface.
type Handler struct {
	store    *db.Store
	cfg      *config.Config
	searchFn SearchFunc
	start    time.Time
}

func NewHandler(store *db.Store, cfg *config.Config, searchFn SearchFunc) *Handler {
	return &Handler{store: store, cfg: cfg, searchFn: searchFn, start: time.Now().UTC()}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	path = strings.TrimPrefix(path, "/lidarr")
	path = strings.TrimPrefix(path, "/api/v1")
	if path == "" {
		path = "/"
	}

	switch {
	case path == "/system/status" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"appName": "Lidarr", "instanceName": "Replacearr", "version": "2.7.1.4410",
			"buildTime": h.start, "isDebug": false, "isProduction": true, "isAdmin": true,
			"isUserInteractive": false, "startupPath": "/app", "appData": "/config",
			"osName": "linux", "osVersion": "", "isNetCore": true, "isLinux": true,
			"isOsx": false, "isWindows": false, "isDocker": true, "mode": "console",
			"branch": "master", "authentication": "forms", "sqliteVersion": "3",
			"urlBase": "", "runtimeVersion": "go", "runtimeName": "Replacearr",
		})
	case path == "/artist" && r.Method == http.MethodGet:
		items, err := h.store.ListArtists()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	case path == "/artist" && r.Method == http.MethodPost:
		var a domain.Artist
		if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		created, err := h.store.CreateArtist(a)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, created)
	case strings.HasPrefix(path, "/artist/") && r.Method == http.MethodGet:
		id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/artist/"), 10, 64)
		a, err := h.store.GetArtistByID(id)
		if err != nil {
			writeErr(w, http.StatusNotFound, "artist not found")
			return
		}
		writeJSON(w, http.StatusOK, a)
	case strings.HasPrefix(path, "/artist/") && r.Method == http.MethodDelete:
		id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/artist/"), 10, 64)
		_ = h.store.DeleteArtist(id)
		w.WriteHeader(http.StatusOK)
	case path == "/album" && r.Method == http.MethodGet:
		artistID, _ := strconv.ParseInt(r.URL.Query().Get("artistId"), 10, 64)
		items, err := h.store.ListAlbums(artistID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	case path == "/track" && r.Method == http.MethodGet:
		albumID, _ := strconv.ParseInt(r.URL.Query().Get("albumId"), 10, 64)
		artistID, _ := strconv.ParseInt(r.URL.Query().Get("artistId"), 10, 64)
		items, err := h.store.ListTracks(albumID, artistID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	case path == "/qualityprofile" && r.Method == http.MethodGet:
		all, err := h.store.ListQualityProfiles()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		out := []domain.QualityProfile{}
		for _, qp := range all {
			if qp.MediaType == "music" || qp.MediaType == "video" || qp.MediaType == "" {
				out = append(out, qp)
			}
		}
		writeJSON(w, http.StatusOK, out)
	case path == "/rootfolder" && r.Method == http.MethodGet:
		items, err := h.store.ListRootFolders("music")
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	case path == "/tag" && r.Method == http.MethodGet:
		items, err := h.store.ListTags()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	case path == "/queue" && r.Method == http.MethodGet:
		items, err := h.store.ListQueueItems("music")
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"page": 1, "pageSize": 50, "records": items, "totalRecords": len(items)})
	case path == "/history" && r.Method == http.MethodGet:
		items, err := h.store.ListHistory("music")
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"page": 1, "pageSize": 50, "records": items, "totalRecords": len(items)})
	default:
		writeErr(w, http.StatusNotFound, "not found")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"message": msg})
}
