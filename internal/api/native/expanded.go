package native

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/applog"
	"github.com/replacearr/replacearr/internal/changelog"
	"github.com/replacearr/replacearr/internal/domain"
	"github.com/replacearr/replacearr/internal/libraryimport"
	"github.com/replacearr/replacearr/internal/tasks"
)

func (s *Server) registerExpandedRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/blocklist", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleListBlocklist))
	mux.HandleFunc("POST /api/v1/blocklist", s.authSvc.RequirePermission(domain.PermSystemManage, s.handleCreateBlocklist))
	mux.HandleFunc("DELETE /api/v1/blocklist/{id}", s.authSvc.RequirePermission(domain.PermSystemManage, s.handleDeleteBlocklist))
	mux.HandleFunc("DELETE /api/v1/blocklist/bulk", s.authSvc.RequirePermission(domain.PermSystemManage, s.handleDeleteBlocklistBulk))

	mux.HandleFunc("GET /api/v1/statistics", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleStatistics))
	mux.HandleFunc("GET /api/v1/calendar", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleCalendar))
	mux.HandleFunc("GET /api/v1/history", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleHistoryPage))

	mux.HandleFunc("POST /api/v1/libraryimport/scan", s.authSvc.RequirePermission(domain.PermSystemManage, s.handleLibraryImportScan))
	mux.HandleFunc("POST /api/v1/libraryimport", s.authSvc.RequirePermission(domain.PermSystemManage, s.handleLibraryImport))
	mux.HandleFunc("POST /api/v1/libraryimport/rescan", s.authSvc.RequirePermission(domain.PermSystemManage, s.handleLibraryRescan))

	mux.HandleFunc("GET /api/v1/system/healthchecks", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleHealthChecks))
	mux.HandleFunc("GET /api/v1/system/tasks", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleSystemTasks))
	mux.HandleFunc("POST /api/v1/system/tasks/{name}/run", s.authSvc.RequirePermission(domain.PermSystemManage, s.handleRunSystemTask))
	mux.HandleFunc("GET /api/v1/system/backup", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleListBackups))
	mux.HandleFunc("POST /api/v1/system/backup", s.authSvc.RequirePermission(domain.PermSystemManage, s.handleCreateBackup))
	mux.HandleFunc("GET /api/v1/system/backup/{id}", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleDownloadBackup))
	mux.HandleFunc("DELETE /api/v1/system/backup/{id}", s.authSvc.RequirePermission(domain.PermSystemManage, s.handleDeleteBackup))
	mux.HandleFunc("POST /api/v1/system/backup/{id}/restore", s.authSvc.RequirePermission(domain.PermSystemManage, s.handleRestoreBackup))
	mux.HandleFunc("GET /api/v1/system/updates", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleUpdateStatus))
	mux.HandleFunc("GET /api/v1/system/logs", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleSystemLogs))
	mux.HandleFunc("GET /api/v1/system/changelog", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleChangelog))
}

func (s *Server) SetScheduler(sched *tasks.Scheduler) {
	s.scheduler = sched
}

// SetQueueProgressSyncer wires the download monitor so GET /api/v1/queue
// refreshes live progress and ETA from download clients.
func (s *Server) SetQueueProgressSyncer(syncer QueueProgressSyncer) {
	s.queueSync = syncer
}

func (s *Server) handleListBlocklist(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListBlocklist(r.URL.Query().Get("mediaType"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateBlocklist(w http.ResponseWriter, r *http.Request) {
	var item domain.BlocklistItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(item.SourceTitle) == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("sourceTitle is required"))
		return
	}
	created, err := s.store.CreateBlocklist(item)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleDeleteBlocklist(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := s.store.DeleteBlocklist(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteBlocklistBulk(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.DeleteBlocklistBulk(body.IDs); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStatistics(w http.ResponseWriter, r *http.Request) {
	mediaType := strings.ToLower(r.URL.Query().Get("mediaType"))
	stats, err := s.computeStatistics(mediaType)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	start, _ := time.Parse(time.RFC3339, q.Get("start"))
	end, _ := time.Parse(time.RFC3339, q.Get("end"))
	if start.IsZero() {
		start = time.Now().UTC().AddDate(0, 0, -7)
	}
	if end.IsZero() {
		end = time.Now().UTC().AddDate(0, 1, 0)
	}
	mediaFilter := map[string]bool{}
	for _, m := range strings.Split(q.Get("mediaTypes"), ",") {
		m = strings.TrimSpace(strings.ToLower(m))
		if m != "" {
			mediaFilter[m] = true
		}
	}
	statusFilter := map[string]bool{}
	for _, st := range strings.Split(q.Get("statuses"), ",") {
		st = strings.TrimSpace(strings.ToLower(st))
		if st != "" {
			statusFilter[st] = true
		}
	}
	allowMedia := func(m string) bool {
		if len(mediaFilter) == 0 {
			return true
		}
		return mediaFilter[m]
	}
	allowStatus := func(st string) bool {
		if len(statusFilter) == 0 {
			return true
		}
		return statusFilter[st]
	}

	queue, _ := s.store.ListQueueItems("")
	queuedTitles := map[string]bool{}
	for _, qi := range queue {
		if qi.TrackedDownloadState == "imported" {
			continue
		}
		queuedTitles[strings.ToLower(qi.Title)] = true
		if qi.SeriesID > 0 {
			queuedTitles[fmt.Sprintf("ep:%d", qi.EpisodeID)] = true
		}
		if qi.MovieID > 0 {
			queuedTitles[fmt.Sprintf("movie:%d", qi.MovieID)] = true
		}
	}

	now := time.Now().UTC()
	events := []domain.CalendarEvent{}

	if allowMedia("tv") {
		series, _ := s.store.ListSeries()
		byID := map[int64]domain.Series{}
		for _, sr := range series {
			byID[sr.ID] = sr
		}
		eps, _ := s.store.ListEpisodes(0)
		for _, ep := range eps {
			air := ep.AirDateUtc
			if air.IsZero() {
				air, _ = time.Parse("2006-01-02", ep.AirDate)
			}
			if air.IsZero() || air.Before(start) || air.After(end) {
				continue
			}
			status := "missing"
			if ep.HasFile {
				status = "downloaded"
			} else if air.After(now) {
				status = "unreleased"
			} else if queuedTitles[fmt.Sprintf("ep:%d", ep.ID)] {
				status = "queued"
			}
			if !allowStatus(status) {
				continue
			}
			title := ep.Title
			if sr, ok := byID[ep.SeriesID]; ok {
				title = fmt.Sprintf("%s - S%02dE%02d", sr.Title, ep.SeasonNumber, ep.EpisodeNumber)
			}
			events = append(events, domain.CalendarEvent{
				ID: fmt.Sprintf("ep-%d", ep.ID), Title: title, Date: air, MediaType: "tv",
				Status: status, SeriesID: ep.SeriesID, EpisodeID: ep.ID,
			})
		}
	}

	if allowMedia("movies") || allowMedia("movie") {
		movies, _ := s.store.ListMovies()
		for _, mv := range movies {
			air := parseLooseDate(mv.DigitalRelease)
			if air.IsZero() {
				air = parseLooseDate(mv.PhysicalRelease)
			}
			if air.IsZero() {
				air = parseLooseDate(mv.InCinemas)
			}
			if air.IsZero() || air.Before(start) || air.After(end) {
				continue
			}
			status := "missing"
			if mv.HasFile {
				status = "downloaded"
			} else if air.After(now) {
				status = "unreleased"
			} else if queuedTitles[fmt.Sprintf("movie:%d", mv.ID)] {
				status = "queued"
			}
			if !allowStatus(status) {
				continue
			}
			events = append(events, domain.CalendarEvent{
				ID: fmt.Sprintf("movie-%d", mv.ID), Title: mv.Title, Date: air, MediaType: "movies",
				Status: status, MovieID: mv.ID,
			})
		}
	}

	if allowMedia("music") {
		albums, _ := s.store.ListAlbums(0)
		for _, al := range albums {
			air := parseLooseDate(al.ReleaseDate)
			if air.IsZero() || air.Before(start) || air.After(end) {
				continue
			}
			status := "missing"
			if al.HasFile {
				status = "downloaded"
			} else if air.After(now) {
				status = "unreleased"
			}
			if !allowStatus(status) {
				continue
			}
			events = append(events, domain.CalendarEvent{
				ID: fmt.Sprintf("album-%d", al.ID), Title: al.Title, Date: air, MediaType: "music",
				Status: status, ArtistID: al.ArtistID, AlbumID: al.ID,
			})
		}
	}

	if allowMedia("books") {
		books, _ := s.store.ListBooks(0)
		for _, b := range books {
			air := parseLooseDate(b.ReleaseDate)
			if air.IsZero() || air.Before(start) || air.After(end) {
				continue
			}
			status := "unreleased"
			if air.Before(now) || air.Equal(now) {
				status = "missing"
			}
			eds, _ := s.store.ListEditions(b.ID, "")
			for _, e := range eds {
				if e.HasFile {
					status = "downloaded"
					break
				}
			}
			if !allowStatus(status) {
				continue
			}
			events = append(events, domain.CalendarEvent{
				ID: fmt.Sprintf("book-%d", b.ID), Title: b.Title, Date: air, MediaType: "books",
				Status: status, BookID: b.ID,
			})
		}
	}

	writeJSON(w, http.StatusOK, events)
}

func parseLooseDate(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02", "2006-01-02T15:04:05Z"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func (s *Server) handleHistoryPage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	records, total, err := s.store.ListHistoryPage(q.Get("mediaType"), q.Get("eventType"), page, pageSize)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"page":         page,
		"pageSize":     pageSize,
		"totalRecords": total,
		"records":      records,
	})
}

func (s *Server) handleLibraryImportScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MediaType    string `json:"mediaType"`
		Path         string `json:"path"`
		RootFolderID int64  `json:"rootFolderId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	path := strings.TrimSpace(req.Path)
	if path == "" && req.RootFolderID > 0 {
		folders, _ := s.store.ListRootFolders("")
		for _, f := range folders {
			if f.ID == req.RootFolderID {
				path = f.Path
				break
			}
		}
	}
	if path == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("path is required"))
		return
	}
	result, err := libraryimport.Scan(r.Context(), libraryimport.ScanOptions{
		Store:     s.store,
		Meta:      s.meta,
		MediaType: req.MediaType,
		Root:      path,
		Language:  s.preferredMetadataLanguage(),
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleLibraryImport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MediaType        string                     `json:"mediaType"`
		RootFolderPath   string                     `json:"rootFolderPath"`
		QualityProfileID int64                      `json:"qualityProfileId"`
		Items            []libraryimport.ImportItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := s.store.GetAppConfig()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	n, err := libraryimport.ImportSelected(r.Context(), libraryimport.ImportOptions{
		Store:            s.store,
		Meta:             s.meta,
		AppCfg:           cfg,
		MediaType:        req.MediaType,
		RootFolderPath:   req.RootFolderPath,
		QualityProfileID: req.QualityProfileID,
		Language:         s.preferredMetadataLanguage(),
		SyncSeriesEpisodes: func(ctx context.Context, sr *domain.Series) error {
			return s.syncSeriesEpisodes(ctx, sr)
		},
		SyncArtistAlbums: func(ctx context.Context, artist *domain.Artist) error {
			if artist == nil || artist.ForeignArtistID == "" || s.meta == nil {
				return nil
			}
			_, items, err := s.meta.GetArtistByMBID(ctx, artist.ForeignArtistID)
			if err != nil || len(items) == 0 {
				return err
			}
			return s.store.ReplaceArtistDiscography(artist.ID, items)
		},
	}, req.Items)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": n})
}

func (s *Server) handleLibraryRescan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MediaType string `json:"mediaType"`
		ID        int64  `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	applog.Info("libraryimport", fmt.Sprintf("rescan requested mediaType=%s id=%d", req.MediaType, req.ID))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleHealthChecks(w http.ResponseWriter, _ *http.Request) {
	checks := []domain.HealthCheck{}
	now := time.Now().UTC()
	if err := s.store.DB().Ping(); err != nil {
		checks = append(checks, domain.HealthCheck{Source: "Database", Type: "error", Message: err.Error(), Time: now})
	} else {
		checks = append(checks, domain.HealthCheck{Source: "Database", Type: "ok", Message: "SQLite reachable", Time: now})
	}
	folders, _ := s.store.ListRootFolders("")
	for _, f := range folders {
		fi, err := os.Stat(f.Path)
		if err != nil {
			checks = append(checks, domain.HealthCheck{Source: "RootFolder", Type: "error", Message: f.Path + ": " + err.Error(), Time: now})
			continue
		}
		if !fi.IsDir() {
			checks = append(checks, domain.HealthCheck{Source: "RootFolder", Type: "warning", Message: f.Path + " is not a directory", Time: now})
			continue
		}
		msg := f.Path
		if f.FreeSpace > 0 && f.FreeSpace < 1<<30 {
			checks = append(checks, domain.HealthCheck{Source: "RootFolder", Type: "warning", Message: msg + " low free space", Time: now})
		} else {
			checks = append(checks, domain.HealthCheck{Source: "RootFolder", Type: "ok", Message: msg + " ok", Time: now})
		}
	}
	clients, _ := s.store.ListDownloadClients()
	for _, c := range clients {
		if !c.Enable {
			continue
		}
		checks = append(checks, domain.HealthCheck{
			Source: "DownloadClient", Type: "ok",
			Message: fmt.Sprintf("%s configured (%s)", c.Name, c.Implementation), Time: now,
		})
	}
	if host := s.activeFlareSolverrHost(); host != "" {
		checks = append(checks, domain.HealthCheck{Source: "FlareSolverr", Type: "ok", Message: host, Time: now})
	}
	writeJSON(w, http.StatusOK, checks)
}

func (s *Server) handleSystemTasks(w http.ResponseWriter, _ *http.Request) {
	if s.scheduler == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, s.scheduler.List())
}

func (s *Server) handleRunSystemTask(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		writeErr(w, http.StatusServiceUnavailable, fmt.Errorf("scheduler unavailable"))
		return
	}
	name := r.PathValue("name")
	if err := s.scheduler.RunNow(name); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleListBackups(w http.ResponseWriter, _ *http.Request) {
	if s.scheduler == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	files, err := s.scheduler.ListBackups()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, files)
}

func (s *Server) handleCreateBackup(w http.ResponseWriter, _ *http.Request) {
	if s.scheduler == nil {
		writeErr(w, http.StatusServiceUnavailable, fmt.Errorf("scheduler unavailable"))
		return
	}
	file, err := s.scheduler.CreateBackup()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, file)
}

func (s *Server) handleDownloadBackup(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		writeErr(w, http.StatusServiceUnavailable, fmt.Errorf("scheduler unavailable"))
		return
	}
	path, err := s.scheduler.BackupPath(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+r.PathValue("id")+"\"")
	_, _ = io.Copy(w, f)
}

func (s *Server) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		writeErr(w, http.StatusServiceUnavailable, fmt.Errorf("scheduler unavailable"))
		return
	}
	if err := s.scheduler.DeleteBackup(r.PathValue("id")); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRestoreBackup(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		writeErr(w, http.StatusServiceUnavailable, fmt.Errorf("scheduler unavailable"))
		return
	}
	if err := s.scheduler.RestoreBackup(r.PathValue("id")); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "pendingRestart": true})
}

func (s *Server) handleUpdateStatus(w http.ResponseWriter, _ *http.Request) {
	latest := s.version
	available := false
	resp, err := http.Get("https://hub.docker.com/v2/repositories/ldrrp/replacearr/tags?page_size=5")
	if err == nil {
		defer resp.Body.Close()
		var body struct {
			Results []struct {
				Name string `json:"name"`
			} `json:"results"`
		}
		if json.NewDecoder(resp.Body).Decode(&body) == nil {
			for _, t := range body.Results {
				if t.Name == "latest" {
					continue
				}
				if t.Name != "" {
					latest = t.Name
					if latest != s.version && latest != "v"+s.version {
						available = true
					}
					break
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"currentVersion":  s.version,
		"latestVersion":   latest,
		"updateAvailable": available,
		"mechanism":       "docker",
		"checkedAt":       time.Now().UTC(),
	})
}

func (s *Server) handleSystemLogs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, http.StatusOK, applog.List(r.URL.Query().Get("level"), limit))
}

func (s *Server) handleChangelog(w http.ResponseWriter, r *http.Request) {
	sections, err := changelog.Sections(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, sections)
}
