// Package readarr provides a thin Readarr-compatible API mount for migration tools.
package readarr

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

// Handler serves /readarr/api/v1/* with a pragmatic Readarr-shaped surface.
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
	path = strings.TrimPrefix(path, "/readarr")
	path = strings.TrimPrefix(path, "/api/v1")
	if path == "" {
		path = "/"
	}

	switch {
	case path == "/system/status" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"appName": "Readarr", "instanceName": "Replacearr", "version": "0.4.4.2635",
			"buildTime": h.start, "isDebug": false, "isProduction": true, "isAdmin": true,
			"isUserInteractive": false, "startupPath": "/app", "appData": "/config",
			"osName": "linux", "osVersion": "", "isNetCore": true, "isLinux": true,
			"isOsx": false, "isWindows": false, "isDocker": true, "mode": "console",
			"branch": "develop", "authentication": "forms", "sqliteVersion": "3",
			"urlBase": "", "runtimeVersion": "go", "runtimeName": "Replacearr",
		})
	case path == "/author" && r.Method == http.MethodGet:
		items, err := h.store.ListAuthors()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	case path == "/author" && r.Method == http.MethodPost:
		var a domain.Author
		if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		created, err := h.store.CreateAuthor(a)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, created)
	case strings.HasPrefix(path, "/author/") && r.Method == http.MethodGet:
		id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/author/"), 10, 64)
		a, err := h.store.GetAuthorByID(id)
		if err != nil {
			writeErr(w, http.StatusNotFound, "author not found")
			return
		}
		writeJSON(w, http.StatusOK, a)
	case strings.HasPrefix(path, "/author/") && r.Method == http.MethodDelete:
		id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/author/"), 10, 64)
		_ = h.store.DeleteAuthor(id)
		w.WriteHeader(http.StatusOK)
	case path == "/book" && r.Method == http.MethodGet:
		authorID, _ := strconv.ParseInt(r.URL.Query().Get("authorId"), 10, 64)
		items, err := h.store.ListBooks(authorID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	case path == "/edition" && r.Method == http.MethodGet:
		bookID, _ := strconv.ParseInt(r.URL.Query().Get("bookId"), 10, 64)
		items, err := h.store.ListEditions(bookID, "")
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
			if qp.MediaType == "ebook" || qp.MediaType == "audiobook" || qp.MediaType == "video" || qp.MediaType == "" {
				out = append(out, qp)
			}
		}
		writeJSON(w, http.StatusOK, out)
	case path == "/rootfolder" && r.Method == http.MethodGet:
		ebooks, _ := h.store.ListRootFolders("ebook")
		audio, _ := h.store.ListRootFolders("audiobook")
		writeJSON(w, http.StatusOK, append(ebooks, audio...))
	case path == "/tag" && r.Method == http.MethodGet:
		items, err := h.store.ListTags()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	case path == "/queue" && r.Method == http.MethodGet:
		ebooks, _ := h.store.ListQueueItems("ebook")
		audio, _ := h.store.ListQueueItems("audiobook")
		items := append(ebooks, audio...)
		writeJSON(w, http.StatusOK, map[string]any{"page": 1, "pageSize": 50, "records": items, "totalRecords": len(items)})
	case path == "/history" && r.Method == http.MethodGet:
		ebooks, _ := h.store.ListHistory("ebook")
		audio, _ := h.store.ListHistory("audiobook")
		items := append(ebooks, audio...)
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
