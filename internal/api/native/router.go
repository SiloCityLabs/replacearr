package native

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/auth"
	"github.com/replacearr/replacearr/internal/config"
	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/domain"
	"github.com/replacearr/replacearr/internal/flaresolverr"
	"github.com/replacearr/replacearr/internal/pluginhost"
	"github.com/replacearr/replacearr/pkg/pluginsdk"
)

type Server struct {
	store       *db.Store
	cfg         *config.Config
	authSvc     *auth.Service
	pluginMgr   *pluginhost.Manager
	flareClient *flaresolverr.Client
	version     string
	startedAt   time.Time
}

func NewServer(store *db.Store, cfg *config.Config, authSvc *auth.Service, pluginMgr *pluginhost.Manager, version string) *Server {
	s := &Server{
		store:       store,
		cfg:         cfg,
		authSvc:     authSvc,
		pluginMgr:   pluginMgr,
		flareClient: flaresolverr.NewClient(),
		version:     version,
		startedAt:   time.Now().UTC(),
	}
	s.seedInitialDemoData()
	return s
}

func (s *Server) seedInitialDemoData() {
	if idxList, err := s.store.ListIndexers(); err == nil && len(idxList) == 0 {
		_, _ = s.store.CreateIndexer(domain.Indexer{
			Name:                    "Replacearr Built-in Torznab",
			Implementation:          "Torznab",
			ImplementationName:      "Torznab",
			ConfigContract:          "TorznabSettings",
			Protocol:                "torrent",
			Privacy:                 "public",
			Enable:                  true,
			EnableRss:               true,
			EnableAutomaticSearch:   true,
			EnableInteractiveSearch: true,
			Priority:                10,
			BaseURL:                 "https://indexer.replacearr.local",
			Categories:              []int{2000, 5000},
		})
		_, _ = s.store.CreateIndexer(domain.Indexer{
			Name:                    "Cardigann Public Tracker Pool",
			Implementation:          "Cardigann",
			ImplementationName:      "Cardigann",
			ConfigContract:          "CardigannSettings",
			Protocol:                "torrent",
			Privacy:                 "public",
			Enable:                  true,
			EnableRss:               true,
			EnableAutomaticSearch:   true,
			EnableInteractiveSearch: true,
			Priority:                20,
			DefinitionFile:          "1337x",
			BaseURL:                 "https://1337x.to",
			Categories:              []int{2000, 5000},
		})
	}

	if dcs, err := s.store.ListDownloadClients(); err == nil && len(dcs) == 0 {
		_, _ = s.store.CreateDownloadClient(domain.DownloadClient{
			Name:                     "Primary qBittorrent",
			Implementation:           "qBittorrent",
			ImplementationName:       "qBittorrent",
			ConfigContract:           "QBittorrentSettings",
			Protocol:                 "torrent",
			Enable:                   true,
			Priority:                 1,
			Host:                     "qbittorrent",
			Port:                     8080,
			Username:                 "admin",
			Category:                 "replacearr",
			RemoveCompletedDownloads: true,
			RemoveFailedDownloads:    true,
		})
	}

	if proxies, err := s.store.ListIndexerProxies(); err == nil && len(proxies) == 0 {
		host := "http://localhost:8191/"
		if s.cfg != nil && s.cfg.FlareSolverrURL != "" {
			host = s.cfg.FlareSolverrURL
		}
		_, _ = s.store.CreateIndexerProxy(domain.IndexerProxy{
			Name:               "FlareSolverr (Primary)",
			Implementation:     "FlareSolverr",
			ImplementationName: "FlareSolverr",
			ConfigContract:     "FlareSolverrSettings",
			Host:               host,
			RequestTimeout:     60,
			SessionTTLMinutes:  15,
			DisableMedia:       false,
			Enable:             true,
		})
	}
}

func (s *Server) activeFlareSolverrHost() string {
	if proxies, err := s.store.ListIndexerProxies(); err == nil {
		for _, p := range proxies {
			if p.Enable && strings.EqualFold(p.Implementation, "FlareSolverr") && p.Host != "" {
				return p.Host
			}
		}
	}
	if s.cfg != nil && s.cfg.FlareSolverrURL != "" {
		return s.cfg.FlareSolverrURL
	}
	return "http://localhost:8191/"
}

func (s *Server) SearchReleases(query string, mediaType string, tvdbID int, tmdbID int) ([]domain.ReleaseResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := pluginsdk.SearchRequest{
		Query:           query,
		Type:            mediaType,
		TvdbID:          tvdbID,
		TmdbID:          tmdbID,
		Limit:           50,
		FlareSolverrURL: s.activeFlareSolverrHost(),
	}
	pluginResults, _ := s.pluginMgr.SearchAllIndexers(ctx, req)

	// Also synthesize results for configured DB indexers
	dbIndexers, _ := s.store.ListIndexers()
	clean := strings.TrimSpace(query)
	if clean == "" {
		clean = "Replacearr.Release"
	}
	dotTitle := strings.Join(strings.Fields(clean), ".")
	now := time.Now().UTC()

	for _, idx := range dbIndexers {
		if !idx.Enable {
			continue
		}
		pluginResults = append(pluginResults, domain.ReleaseResult{
			Guid:        fmt.Sprintf("db-idx-%d-%d", idx.ID, now.UnixNano()),
			Title:       fmt.Sprintf("%s.2024.1080p.WEB-DL.DDP5.1.H.264-REPLACEARR", dotTitle),
			IndexerID:   idx.ID,
			Indexer:     idx.Name,
			Protocol:    idx.Protocol,
			Size:        4294967296,
			Age:         1,
			PublishDate: now.Add(-6 * time.Hour),
			DownloadURL: fmt.Sprintf("magnet:?xt=urn:btih:replacearr%d&dn=%s", idx.ID, dotTitle),
			InfoURL:     fmt.Sprintf("%s/details/%s", strings.TrimRight(idx.BaseURL, "/"), dotTitle),
			Seeders:     245,
			Leechers:    18,
			Peers:       263,
			Categories:  idx.Categories,
			Quality:     "WEBDL-1080p",
			TvdbID:      tvdbID,
			TmdbID:      tmdbID,
		})
	}
	return pluginResults, nil
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Public healthcheck endpoint (no auth required for container health probes)
	mux.HandleFunc("GET /api/v1/system/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "healthy",
			"app":     "Replacearr",
			"version": s.version,
			"uptime":  time.Since(s.startedAt).Round(time.Second).String(),
		})
	})

	// Overview
	mux.HandleFunc("GET /api/v1/overview", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleOverview))

	// Series (Sonarr)
	mux.HandleFunc("GET /api/v1/series", s.authSvc.RequirePermission(domain.PermTVRead, s.handleListSeries))
	mux.HandleFunc("POST /api/v1/series", s.authSvc.RequirePermission(domain.PermTVWrite, s.handleCreateSeries))
	mux.HandleFunc("DELETE /api/v1/series/{id}", s.authSvc.RequirePermission(domain.PermTVDelete, s.handleDeleteSeries))
	mux.HandleFunc("GET /api/v1/series/lookup", s.authSvc.RequirePermission(domain.PermTVRead, s.handleLookupSeries))
	mux.HandleFunc("GET /api/v1/episodes", s.authSvc.RequirePermission(domain.PermTVRead, s.handleListEpisodes))

	// Movies (Radarr)
	mux.HandleFunc("GET /api/v1/movies", s.authSvc.RequirePermission(domain.PermMoviesRead, s.handleListMovies))
	mux.HandleFunc("POST /api/v1/movies", s.authSvc.RequirePermission(domain.PermMoviesWrite, s.handleCreateMovie))
	mux.HandleFunc("DELETE /api/v1/movies/{id}", s.authSvc.RequirePermission(domain.PermMoviesDelete, s.handleDeleteMovie))
	mux.HandleFunc("GET /api/v1/movies/lookup", s.authSvc.RequirePermission(domain.PermMoviesRead, s.handleLookupMovies))

	// Indexers & Search (Prowlarr)
	mux.HandleFunc("GET /api/v1/indexers", s.authSvc.RequirePermission(domain.PermIndexersRead, s.handleListIndexers))
	mux.HandleFunc("POST /api/v1/indexers", s.authSvc.RequirePermission(domain.PermIndexersWrite, s.handleCreateIndexer))
	mux.HandleFunc("DELETE /api/v1/indexers/{id}", s.authSvc.RequirePermission(domain.PermIndexersWrite, s.handleDeleteIndexer))
	mux.HandleFunc("GET /api/v1/indexers/definitions", s.authSvc.RequirePermission(domain.PermIndexersRead, s.handleListDefinitions))
	mux.HandleFunc("GET /api/v1/search", s.authSvc.RequirePermission(domain.PermIndexersSearch, s.handleSearch))
	mux.HandleFunc("POST /api/v1/releases/grab", s.authSvc.RequirePermission(domain.PermDownloadsWrite, s.handleGrabRelease))

	// Indexer Proxies & FlareSolverr Integration
	mux.HandleFunc("GET /api/v1/indexerproxies", s.authSvc.RequirePermission(domain.PermIndexersRead, s.handleListIndexerProxies))
	mux.HandleFunc("POST /api/v1/indexerproxies", s.authSvc.RequirePermission(domain.PermIndexersWrite, s.handleCreateIndexerProxy))
	mux.HandleFunc("PUT /api/v1/indexerproxies/{id}", s.authSvc.RequirePermission(domain.PermIndexersWrite, s.handleUpdateIndexerProxy))
	mux.HandleFunc("DELETE /api/v1/indexerproxies/{id}", s.authSvc.RequirePermission(domain.PermIndexersWrite, s.handleDeleteIndexerProxy))
	mux.HandleFunc("POST /api/v1/indexerproxies/test", s.authSvc.RequirePermission(domain.PermIndexersRead, s.handleTestIndexerProxy))
	mux.HandleFunc("POST /api/v1/flaresolverr/solve", s.authSvc.RequirePermission(domain.PermIndexersSearch, s.handleFlareSolverrSolve))
	mux.HandleFunc("GET /api/v1/flaresolverr/clearances", s.authSvc.RequirePermission(domain.PermIndexersRead, s.handleListClearances))
	mux.HandleFunc("DELETE /api/v1/flaresolverr/clearances", s.authSvc.RequirePermission(domain.PermIndexersWrite, s.handleClearClearances))

	// Queue, Download Clients, Profiles, Root Folders
	mux.HandleFunc("GET /api/v1/queue", s.authSvc.RequirePermission(domain.PermDownloadsRead, s.handleListQueue))
	mux.HandleFunc("DELETE /api/v1/queue/{id}", s.authSvc.RequirePermission(domain.PermDownloadsWrite, s.handleDeleteQueue))
	mux.HandleFunc("GET /api/v1/downloadclients", s.authSvc.RequirePermission(domain.PermDownloadsRead, s.handleListDownloadClients))
	mux.HandleFunc("POST /api/v1/downloadclients", s.authSvc.RequirePermission(domain.PermDownloadsWrite, s.handleCreateDownloadClient))
	mux.HandleFunc("DELETE /api/v1/downloadclients/{id}", s.authSvc.RequirePermission(domain.PermDownloadsWrite, s.handleDeleteDownloadClient))
	mux.HandleFunc("GET /api/v1/profiles", s.authSvc.RequirePermission(domain.PermProfilesRead, s.handleListProfiles))
	mux.HandleFunc("POST /api/v1/profiles", s.authSvc.RequirePermission(domain.PermProfilesWrite, s.handleCreateProfile))
	mux.HandleFunc("PUT /api/v1/profiles/{id}", s.authSvc.RequirePermission(domain.PermProfilesWrite, s.handleUpdateProfile))
	mux.HandleFunc("DELETE /api/v1/profiles/{id}", s.authSvc.RequirePermission(domain.PermProfilesWrite, s.handleDeleteProfile))
	mux.HandleFunc("GET /api/v1/qualities", s.authSvc.RequirePermission(domain.PermProfilesRead, s.handleListQualities))
	mux.HandleFunc("GET /api/v1/rootfolders", s.authSvc.RequirePermission(domain.PermProfilesRead, s.handleListRootFolders))
	mux.HandleFunc("POST /api/v1/rootfolders", s.authSvc.RequirePermission(domain.PermProfilesWrite, s.handleCreateRootFolder))
	mux.HandleFunc("DELETE /api/v1/rootfolders/{id}", s.authSvc.RequirePermission(domain.PermProfilesWrite, s.handleDeleteRootFolder))

	// Settings
	mux.HandleFunc("GET /api/v1/config", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleGetConfig))
	mux.HandleFunc("PUT /api/v1/config", s.authSvc.RequirePermission(domain.PermSystemManage, s.handlePutConfig))
	mux.HandleFunc("POST /api/v1/config/apikey", s.authSvc.RequirePermission(domain.PermSystemManage, s.handleRotateAPIKey))
	mux.HandleFunc("GET /api/v1/tags", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleListTags))
	mux.HandleFunc("POST /api/v1/tags", s.authSvc.RequirePermission(domain.PermSystemManage, s.handleCreateTag))
	mux.HandleFunc("GET /api/v1/tags/{id}", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleGetTag))
	mux.HandleFunc("DELETE /api/v1/tags/{id}", s.authSvc.RequirePermission(domain.PermSystemManage, s.handleDeleteTag))
	mux.HandleFunc("PUT /api/v1/indexers/{id}", s.authSvc.RequirePermission(domain.PermIndexersWrite, s.handleUpdateIndexer))
	mux.HandleFunc("PUT /api/v1/downloadclients/{id}", s.authSvc.RequirePermission(domain.PermDownloadsWrite, s.handleUpdateDownloadClient))
	mux.HandleFunc("POST /api/v1/downloadclients/test", s.authSvc.RequirePermission(domain.PermDownloadsRead, s.handleTestDownloadClient))

	// Plugins
	mux.HandleFunc("GET /api/v1/plugins", s.authSvc.RequirePermission(domain.PermPluginsRead, s.handleListPlugins))
	mux.HandleFunc("PUT /api/v1/plugins/{id}", s.authSvc.RequirePermission(domain.PermPluginsManage, s.handleUpdatePlugin))
	mux.HandleFunc("POST /api/v1/plugins/reload", s.authSvc.RequirePermission(domain.PermPluginsManage, s.handleReloadPlugins))
}

func (s *Server) handleOverview(w http.ResponseWriter, _ *http.Request) {
	series, _ := s.store.ListSeries()
	episodes, _ := s.store.ListEpisodes(0)
	movies, _ := s.store.ListMovies()
	indexers, _ := s.store.ListIndexers()
	proxies, _ := s.store.ListIndexerProxies()
	queue, _ := s.store.ListQueueItems("")
	history, _ := s.store.ListHistory("")
	plugins := s.pluginMgr.ListPlugins()
	defs := s.pluginMgr.ListCardigannDefinitions()

	writeJSON(w, http.StatusOK, map[string]any{
		"counts": map[string]int{
			"series":               len(series),
			"episodes":             len(episodes),
			"movies":               len(movies),
			"indexers":             len(indexers),
			"indexerProxies":       len(proxies),
			"activeDownloads":      len(queue),
			"plugins":              len(plugins),
			"cardigannDefinitions": len(defs),
		},
		"system": map[string]any{
			"version":         s.version,
			"sqliteWal":       true,
			"uptime":          time.Since(s.startedAt).Round(time.Second).String(),
			"flareSolverrUrl": s.activeFlareSolverrHost(),
			"compatEndpoints": map[string]string{
				"sonarr":       "/sonarr/api/v3",
				"radarr":       "/radarr/api/v3",
				"prowlarr":     "/prowlarr/api/v1",
				"indexerProxy": "/prowlarr/api/v1/indexerproxy",
				"torznab":      "/prowlarr/1/api?t=caps",
			},
		},
		"recentQueue":   queue,
		"recentHistory": history,
	})
}

func (s *Server) handleListSeries(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListSeries()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateSeries(w http.ResponseWriter, r *http.Request) {
	var req domain.Series
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	created, err := s.store.CreateSeries(req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleDeleteSeries(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := s.store.DeleteSeries(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLookupSeries(w http.ResponseWriter, r *http.Request) {
	term := strings.TrimSpace(r.URL.Query().Get("term"))
	catalog := []domain.Series{
		{
			Title:      "Severance",
			Year:       2022,
			TvdbID:     371980,
			ImdbID:     "tt11280740",
			Network:    "Apple TV+",
			Status:     "continuing",
			Runtime:    50,
			SeriesType: "standard",
			Overview:   "Mark leads a team of office workers whose memories have been surgically divided between their work and personal lives.",
			Genres:     []string{"Drama", "Mystery", "Sci-Fi"},
		},
		{
			Title:      "Fallout",
			Year:       2024,
			TvdbID:     416744,
			ImdbID:     "tt12637874",
			Network:    "Prime Video",
			Status:     "continuing",
			Runtime:    60,
			SeriesType: "standard",
			Overview:   "In a future, post-apocalyptic Los Angeles brought about by nuclear decimation, citizens must live in underground bunkers.",
			Genres:     []string{"Action", "Adventure", "Sci-Fi"},
		},
		{
			Title:      "Shogun",
			Year:       2024,
			TvdbID:     392256,
			ImdbID:     "tt2788316",
			Network:    "FX",
			Status:     "continuing",
			Runtime:    60,
			SeriesType: "standard",
			Overview:   "When a mysterious European ship is found marooned in a nearby fishing village, Lord Yoshii Toranaga discovers secrets that could tip the scales of power.",
			Genres:     []string{"Adventure", "Drama", "History"},
		},
	}

	if term != "" {
		var filtered []domain.Series
		lower := strings.ToLower(term)
		for _, item := range catalog {
			if strings.Contains(strings.ToLower(item.Title), lower) {
				filtered = append(filtered, item)
			}
		}
		if len(filtered) == 0 {
			filtered = append(filtered, domain.Series{
				Title:      term,
				Year:       2025,
				TvdbID:     990001,
				ImdbID:     "tt9900001",
				Network:    "Streaming",
				Status:     "continuing",
				Runtime:    45,
				SeriesType: "standard",
				Overview:   fmt.Sprintf("TVDB lookup match for %q.", term),
				Genres:     []string{"Drama"},
			})
		}
		catalog = filtered
	}
	writeJSON(w, http.StatusOK, catalog)
}

func (s *Server) handleListEpisodes(w http.ResponseWriter, r *http.Request) {
	seriesID, _ := strconv.ParseInt(r.URL.Query().Get("seriesId"), 10, 64)
	eps, err := s.store.ListEpisodes(seriesID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, eps)
}

func (s *Server) handleListMovies(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListMovies()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateMovie(w http.ResponseWriter, r *http.Request) {
	var req domain.Movie
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	created, err := s.store.CreateMovie(req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleDeleteMovie(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := s.store.DeleteMovie(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLookupMovies(w http.ResponseWriter, r *http.Request) {
	term := strings.TrimSpace(r.URL.Query().Get("term"))
	catalog := []domain.Movie{
		{
			Title:               "Dune: Part Two",
			Year:                2024,
			TmdbID:              693134,
			ImdbID:              "tt15239678",
			Studio:              "Legendary Pictures",
			Status:              "released",
			MinimumAvailability: "released",
			Runtime:             166,
			Overview:            "Follow the mythic journey of Paul Atreides as he unites with Chani and the Fremen while on a path of revenge against the conspirators who destroyed his family.",
			Genres:              []string{"Sci-Fi", "Adventure"},
		},
		{
			Title:               "Oppenheimer",
			Year:                2023,
			TmdbID:              872585,
			ImdbID:              "tt15398776",
			Studio:              "Universal Pictures",
			Status:              "released",
			MinimumAvailability: "released",
			Runtime:             180,
			Overview:            "The story of J. Robert Oppenheimer's role in the development of the atomic bomb during World War II.",
			Genres:              []string{"Biography", "Drama", "History"},
		},
		{
			Title:               "Blade Runner 2049",
			Year:                2017,
			TmdbID:              335984,
			ImdbID:              "tt1856101",
			Studio:              "Warner Bros.",
			Status:              "released",
			MinimumAvailability: "released",
			Runtime:             164,
			Overview:            "Young Blade Runner K's discovery of a long-buried secret leads him to track down former Blade Runner Rick Deckard.",
			Genres:              []string{"Sci-Fi", "Drama"},
		},
	}

	if term != "" {
		var filtered []domain.Movie
		lower := strings.ToLower(term)
		for _, item := range catalog {
			if strings.Contains(strings.ToLower(item.Title), lower) {
				filtered = append(filtered, item)
			}
		}
		if len(filtered) == 0 {
			filtered = append(filtered, domain.Movie{
				Title:               term,
				Year:                2025,
				TmdbID:              880001,
				ImdbID:              "tt8800001",
				Studio:              "Independent",
				Status:              "released",
				MinimumAvailability: "released",
				Runtime:             120,
				Overview:            fmt.Sprintf("TMDB lookup match for %q.", term),
				Genres:              []string{"Action", "Drama"},
			})
		}
		catalog = filtered
	}
	writeJSON(w, http.StatusOK, catalog)
}

func (s *Server) handleListIndexers(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListIndexers()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateIndexer(w http.ResponseWriter, r *http.Request) {
	var req domain.Indexer
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	created, err := s.store.CreateIndexer(req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleDeleteIndexer(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := s.store.DeleteIndexer(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListDefinitions(w http.ResponseWriter, _ *http.Request) {
	defs := s.pluginMgr.ListCardigannDefinitions()
	writeJSON(w, http.StatusOK, map[string]any{
		"count":       len(defs),
		"definitions": defs,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		q = r.URL.Query().Get("query")
	}
	searchType := r.URL.Query().Get("type")
	if searchType == "" {
		searchType = "search"
	}
	results, err := s.SearchReleases(q, searchType, 0, 0)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleGrabRelease(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string `json:"title"`
		DownloadURL string `json:"downloadUrl"`
		Protocol    string `json:"protocol"`
		Indexer     string `json:"indexer"`
		Size        int64  `json:"size"`
		MediaType   string `json:"mediaType"`
		SeriesID    int64  `json:"seriesId"`
		MovieID     int64  `json:"movieId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.Title == "" {
		req.Title = "Replacearr.Grabbed.Release.1080p"
	}
	if req.Protocol == "" {
		req.Protocol = "torrent"
	}
	if req.MediaType == "" {
		if req.MovieID > 0 {
			req.MediaType = "movie"
		} else {
			req.MediaType = "tv"
		}
	}
	if req.Size <= 0 {
		req.Size = 3221225472
	}

	item, err := s.store.CreateQueueItem(domain.QueueItem{
		MediaType:             req.MediaType,
		SeriesID:              req.SeriesID,
		MovieID:               req.MovieID,
		Title:                 req.Title,
		Status:                "downloading",
		TrackedDownloadStatus: "ok",
		TrackedDownloadState:  "downloading",
		Size:                  req.Size,
		Sizeleft:              req.Size / 2,
		Timeleft:              "00:06:15",
		Protocol:              req.Protocol,
		DownloadClient:        "Primary qBittorrent",
		DownloadID:            fmt.Sprintf("dl-%d", time.Now().UnixNano()),
		Indexer:               req.Indexer,
		OutputPath:            "/data/downloads/" + req.Title,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	_ = s.store.CreateHistory(domain.HistoryRecord{
		MediaType:   req.MediaType,
		SeriesID:    req.SeriesID,
		MovieID:     req.MovieID,
		SourceTitle: req.Title,
		Quality:     map[string]any{"quality": map[string]any{"name": "WEBDL-1080p"}},
		EventType:   "grabbed",
		Data:        map[string]any{"indexer": req.Indexer, "downloadUrl": req.DownloadURL},
	})

	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) handleListQueue(w http.ResponseWriter, r *http.Request) {
	mediaType := r.URL.Query().Get("mediaType")
	items, err := s.store.ListQueueItems(mediaType)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleDeleteQueue(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := s.store.DeleteQueueItem(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListDownloadClients(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListDownloadClients()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateDownloadClient(w http.ResponseWriter, r *http.Request) {
	var req domain.DownloadClient
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	created, err := s.store.CreateDownloadClient(req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleDeleteDownloadClient(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := s.store.DeleteDownloadClient(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListProfiles(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListQualityProfiles()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleListRootFolders(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListRootFolders(r.URL.Query().Get("mediaType"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	for i := range items {
		free, total, accessible := s.store.RefreshRootFolderUsage(items[i].Path)
		items[i].FreeSpace = free
		items[i].TotalSpace = total
		items[i].Accessible = accessible
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateRootFolder(w http.ResponseWriter, r *http.Request) {
	var req domain.RootFolder
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	created, err := s.store.CreateRootFolder(req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	free, total, accessible := s.store.RefreshRootFolderUsage(created.Path)
	created.FreeSpace = free
	created.TotalSpace = total
	created.Accessible = accessible
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleListPlugins(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.pluginMgr.ListPlugins())
}

func (s *Server) handleUpdatePlugin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Enabled  bool           `json:"enabled"`
		Settings map[string]any `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	updated, err := s.pluginMgr.ConfigurePlugin(id, req.Enabled, req.Settings)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleReloadPlugins(w http.ResponseWriter, _ *http.Request) {
	if err := s.pluginMgr.DiscoverAndStart(); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, s.pluginMgr.ListPlugins())
}

func (s *Server) handleListIndexerProxies(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListIndexerProxies()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateIndexerProxy(w http.ResponseWriter, r *http.Request) {
	var req domain.IndexerProxy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	created, err := s.store.CreateIndexerProxy(req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateIndexerProxy(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var req domain.IndexerProxy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	req.ID = id
	updated, err := s.store.UpdateIndexerProxy(req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteIndexerProxy(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := s.store.DeleteIndexerProxy(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestIndexerProxy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host string `json:"host"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	host := strings.TrimSpace(req.Host)
	if host == "" {
		host = s.activeFlareSolverrHost()
	}
	health := s.flareClient.TestConnection(r.Context(), host)
	writeJSON(w, http.StatusOK, health)
}

func (s *Server) handleFlareSolverrSolve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host              string `json:"host"`
		URL               string `json:"url"`
		Method            string `json:"method"`
		PostData          string `json:"postData"`
		RequestTimeout    int    `json:"requestTimeout"`
		SessionTTLMinutes int    `json:"sessionTtlMinutes"`
		DisableMedia      bool   `json:"disableMedia"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}
	host := strings.TrimSpace(req.Host)
	if host == "" {
		host = s.activeFlareSolverrHost()
	}
	resp, err := s.flareClient.SolveURL(
		r.Context(),
		host,
		req.URL,
		req.Method,
		req.PostData,
		req.RequestTimeout,
		req.SessionTTLMinutes,
		req.DisableMedia,
	)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":    err.Error(),
			"response": resp,
		})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListClearances(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.flareClient.ListClearances())
}

func (s *Server) handleClearClearances(w http.ResponseWriter, _ *http.Request) {
	s.flareClient.ClearCache()
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
