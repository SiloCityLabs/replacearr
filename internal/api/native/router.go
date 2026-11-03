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
	"github.com/replacearr/replacearr/internal/downloadclient/qbittorrent"
	"github.com/replacearr/replacearr/internal/flaresolverr"
	"github.com/replacearr/replacearr/internal/httpclient"
	"github.com/replacearr/replacearr/internal/indexers/cardigann"
	"github.com/replacearr/replacearr/internal/mediacover"
	"github.com/replacearr/replacearr/internal/metadata"
	"github.com/replacearr/replacearr/internal/pluginhost"
	"github.com/replacearr/replacearr/pkg/pluginsdk"
)

type Server struct {
	store       *db.Store
	cfg         *config.Config
	authSvc     *auth.Service
	pluginMgr   *pluginhost.Manager
	meta        *metadata.Service
	flareClient *flaresolverr.Client
	covers      *mediacover.Store
	version     string
	startedAt   time.Time
}

func NewServer(store *db.Store, cfg *config.Config, authSvc *auth.Service, pluginMgr *pluginhost.Manager, version string) *Server {
	flare := flaresolverr.NewClient()
	appCfg, _ := store.GetAppConfig()
	proxyURLs := []string{}
	metaSvc := metadata.New()
	if pluginMgr != nil {
		if u := pluginMgr.ActiveProxyURL(); u != "" {
			proxyURLs = append(proxyURLs, u)
		}
		if m := pluginMgr.Metadata(); m != nil {
			metaSvc = m
		}
	}
	httpCli := httpclient.New(httpclient.Options{
		AppProxy:   appCfg.Proxy,
		PluginURLs: proxyURLs,
		Timeout:    30 * time.Second,
	})
	flare.SetHTTPClient(httpCli)

	configDir := "./config"
	if cfg != nil && cfg.ConfigDir != "" {
		configDir = cfg.ConfigDir
	}
	s := &Server{
		store:       store,
		cfg:         cfg,
		authSvc:     authSvc,
		pluginMgr:   pluginMgr,
		meta:        metaSvc,
		flareClient: flare,
		covers:      mediacover.NewStore(configDir, httpCli),
		version:     version,
		startedAt:   time.Now().UTC(),
	}
	s.seedInitialDemoData()
	return s
}

// Metadata exposes the shared Add New lookup service for compat mounts.
func (s *Server) Metadata() *metadata.Service {
	if s == nil || s.meta == nil {
		return metadata.New()
	}
	return s.meta
}

func (s *Server) preferredMetadataLanguage() string {
	if s.store != nil {
		if cfg, err := s.store.GetAppConfig(); err == nil {
			if lang := strings.TrimSpace(cfg.Metadata.PreferredLanguage); lang != "" {
				return lang
			}
		}
	}
	return "english"
}

// CoverStore exposes the media cover cache for compat MediaCover mounts.
func (s *Server) CoverStore() *mediacover.Store {
	return s.covers
}

func (s *Server) seedInitialDemoData() {
	// Do not seed fake indexers, download clients, or demo media.
	// Users configure these via the wizard / settings. FlareSolverr is optional.
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

func (s *Server) SearchReleases(query string, mediaType string, tvdbID int, tmdbID int, season int, episode int) ([]domain.ReleaseResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	aliases := []string{}
	if tvdbID > 0 {
		if all, err := s.store.ListSeries(); err == nil {
			for _, sr := range all {
				if sr.TvdbID != tvdbID {
					continue
				}
				if listed, aErr := s.store.ListSeriesAliases(sr.ID); aErr == nil {
					for _, a := range listed {
						if a.SeasonNumber == nil || season == 0 || *a.SeasonNumber == season {
							aliases = append(aliases, a.Title)
						}
					}
				}
				if season == 0 && episode == 0 && domain.PreferSeasonPackQuery(sr) {
					season = 1
				}
				break
			}
		}
	}

	searchQuery := strings.TrimSpace(query)
	if len(aliases) > 0 {
		parts := []string{searchQuery}
		for _, a := range aliases {
			if a != "" && !strings.EqualFold(a, searchQuery) {
				parts = append(parts, a)
			}
		}
		searchQuery = strings.Join(parts, " ")
	}

	proxyURL := ""
	if s.pluginMgr != nil {
		proxyURL = s.pluginMgr.ActiveProxyURL()
	}
	req := pluginsdk.SearchRequest{
		Query:           searchQuery,
		Type:            mediaType,
		TvdbID:          tvdbID,
		TmdbID:          tmdbID,
		Season:          season,
		Episode:         episode,
		Aliases:         aliases,
		Limit:           50,
		FlareSolverrURL: s.activeFlareSolverrHost(),
		ProxyURL:        proxyURL,
	}

	results := []domain.ReleaseResult{}
	var searchErrs []string

	if s.pluginMgr != nil {
		pluginResults, err := s.pluginMgr.SearchAllIndexers(ctx, req)
		if err != nil {
			searchErrs = append(searchErrs, err.Error())
		} else if pluginResults != nil {
			results = append(results, pluginResults...)
		}
	}

	cardigannResults, cardigannErrs := s.searchCardigannIndexers(ctx, searchQuery, season, episode, proxyURL)
	results = append(results, cardigannResults...)
	searchErrs = append(searchErrs, cardigannErrs...)

	if len(results) == 0 && len(searchErrs) > 0 {
		return results, fmt.Errorf("search failed: %s", strings.Join(searchErrs, "; "))
	}
	return results, nil
}

func (s *Server) searchCardigannIndexers(ctx context.Context, query string, season, episode int, proxyURL string) ([]domain.ReleaseResult, []string) {
	indexers, err := s.store.ListIndexers()
	if err != nil {
		return nil, []string{err.Error()}
	}
	defsDir := ""
	if s.cfg != nil {
		defsDir = s.cfg.DefinitionsDir
	}
	if defsDir == "" {
		return nil, nil
	}

	flareHost := strings.TrimSpace(s.activeFlareSolverrHost())
	baseTransport := http.DefaultTransport
	appCfg, _ := s.store.GetAppConfig()
	httpCli := httpclient.New(httpclient.Options{
		AppProxy:   appCfg.Proxy,
		PluginURLs: []string{proxyURL},
		Timeout:    60 * time.Second,
	})
	if httpCli != nil && httpCli.Transport != nil {
		baseTransport = httpCli.Transport
	}
	transport := baseTransport
	if flareHost != "" && s.flareClient != nil {
		transport = &flaresolverr.SolvingTransport{
			Base:     baseTransport,
			Client:   s.flareClient,
			Host:     flareHost,
			TimeoutS: 60,
		}
	}
	client := &http.Client{Timeout: 60 * time.Second, Transport: transport}

	var out []domain.ReleaseResult
	var errs []string
	for _, idx := range indexers {
		if !idx.Enable || !idx.SupportsSearch {
			continue
		}
		if !strings.EqualFold(idx.Implementation, "Cardigann") {
			continue
		}
		defFile := strings.TrimSpace(idx.DefinitionFile)
		if defFile == "" {
			defFile = strings.ToLower(strings.ReplaceAll(idx.Name, " ", ""))
		}
		rels, err := cardigann.Search(ctx, cardigann.SearchRequest{
			DefinitionsDir: defsDir,
			DefinitionFile: defFile,
			BaseURL:        idx.BaseURL,
			IndexerID:      idx.ID,
			IndexerName:    idx.Name,
			Fields:         idx.Fields,
			Query:          query,
			Season:         season,
			Episode:        episode,
			Categories:     idx.Categories,
			HTTPClient:     client,
			ProxyURL:       proxyURL,
		})
		if err != nil {
			msg := err.Error()
			if strings.Contains(msg, "403") || strings.Contains(strings.ToLower(msg), "cloudflare") {
				msg += " — site blocked the request; add an enabled FlareSolverr indexer proxy under Indexers"
			}
			errs = append(errs, fmt.Sprintf("%s: %s", idx.Name, msg))
			continue
		}
		out = append(out, rels...)
	}
	return out, errs
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
	mux.HandleFunc("POST /api/v1/releases/manual", s.authSvc.RequirePermission(domain.PermDownloadsWrite, s.handleManualRelease))
	mux.HandleFunc("GET /api/v1/mediacover/{mediaType}/{id}/{name}", s.handleMediaCover)

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
	mux.HandleFunc("POST /api/v1/plugins/{id}/test", s.authSvc.RequirePermission(domain.PermPluginsManage, s.handleTestNotificationPlugin))
	mux.HandleFunc("GET /api/v1/notifications/stream", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleNotificationStream))
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
	_ = s.syncSeriesEpisodes(r.Context(), created)
	s.cacheCoverAsync("tv", created.ID, created.Images)
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
	if term == "" {
		writeJSON(w, http.StatusOK, []domain.Series{})
		return
	}
	results, err := s.meta.SearchSeries(r.Context(), term, s.preferredMetadataLanguage())
	if err != nil {
		writeErr(w, http.StatusBadGateway, fmt.Errorf("series lookup failed: %w", err))
		return
	}
	if results == nil {
		results = []domain.Series{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleListEpisodes(w http.ResponseWriter, r *http.Request) {
	seriesID, _ := strconv.ParseInt(r.URL.Query().Get("seriesId"), 10, 64)
	eps, err := s.store.ListEpisodes(seriesID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if len(eps) == 0 && seriesID > 0 {
		if sr, gerr := s.store.GetSeriesByID(seriesID); gerr == nil && sr != nil && sr.TvdbID > 0 {
			if serr := s.syncSeriesEpisodes(r.Context(), sr); serr == nil {
				eps, err = s.store.ListEpisodes(seriesID)
				if err != nil {
					writeErr(w, http.StatusInternalServerError, err)
					return
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, eps)
}

func (s *Server) syncSeriesEpisodes(ctx context.Context, sr *domain.Series) error {
	if s.meta == nil || sr == nil || sr.TvdbID <= 0 || sr.ID <= 0 {
		return nil
	}
	_, eps, err := s.meta.GetSeriesByTVDB(ctx, sr.TvdbID, s.preferredMetadataLanguage())
	if err != nil {
		return err
	}
	if len(eps) == 0 {
		return nil
	}
	return s.store.ReplaceEpisodesForSeries(sr.ID, eps)
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
	s.cacheCoverAsync("movie", created.ID, created.Images)
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
	if term == "" {
		writeJSON(w, http.StatusOK, []domain.Movie{})
		return
	}
	results, err := s.meta.SearchMovies(r.Context(), term)
	if err != nil {
		writeErr(w, http.StatusBadGateway, fmt.Errorf("movie lookup failed: %w", err))
		return
	}
	if results == nil {
		results = []domain.Movie{}
	}
	writeJSON(w, http.StatusOK, results)
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
	tvdbID, _ := strconv.Atoi(r.URL.Query().Get("tvdbId"))
	tmdbID, _ := strconv.Atoi(r.URL.Query().Get("tmdbId"))
	season, _ := strconv.Atoi(r.URL.Query().Get("season"))
	episode, _ := strconv.Atoi(r.URL.Query().Get("ep"))
	if episode == 0 {
		episode, _ = strconv.Atoi(r.URL.Query().Get("episode"))
	}
	results, err := s.SearchReleases(q, searchType, tvdbID, tmdbID, season, episode)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleGrabRelease(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title        string `json:"title"`
		DownloadURL  string `json:"downloadUrl"`
		Protocol     string `json:"protocol"`
		Indexer      string `json:"indexer"`
		Size         int64  `json:"size"`
		MediaType    string `json:"mediaType"`
		SeriesID     int64  `json:"seriesId"`
		MovieID      int64  `json:"movieId"`
		EpisodeID    int64  `json:"episodeId"`
		SeasonNumber int    `json:"seasonNumber"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.DownloadURL) == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("downloadUrl is required"))
		return
	}
	if req.Title == "" {
		req.Title = req.DownloadURL
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

	clientName, downloadID, err := s.enqueueToDownloadClient(r.Context(), req.Title, req.DownloadURL, req.Protocol)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err)
		return
	}

	item, err := s.store.CreateQueueItem(domain.QueueItem{
		MediaType:             req.MediaType,
		SeriesID:              req.SeriesID,
		EpisodeID:             req.EpisodeID,
		MovieID:               req.MovieID,
		Title:                 req.Title,
		Status:                "queued",
		TrackedDownloadStatus: "ok",
		TrackedDownloadState:  "downloading",
		Size:                  req.Size,
		Sizeleft:              req.Size,
		Timeleft:              "",
		Protocol:              req.Protocol,
		DownloadClient:        clientName,
		DownloadID:            downloadID,
		Indexer:               req.Indexer,
		OutputPath:            "",
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	_ = s.store.CreateHistory(domain.HistoryRecord{
		MediaType:   req.MediaType,
		SeriesID:    req.SeriesID,
		EpisodeID:   req.EpisodeID,
		MovieID:     req.MovieID,
		SourceTitle: req.Title,
		Quality:     map[string]any{"quality": map[string]any{"name": "Unknown"}},
		EventType:   "grabbed",
		Data: map[string]any{
			"indexer":        req.Indexer,
			"downloadUrl":    req.DownloadURL,
			"seasonNumber":   req.SeasonNumber,
			"downloadClient": clientName,
			"downloadId":     downloadID,
		},
	})

	s.emitNotification(r.Context(), pluginsdk.EventDownloadStarted, map[string]any{
		"title":      req.Title,
		"queueId":    item.ID,
		"downloadId": downloadID,
		"mediaType":  req.MediaType,
		"indexer":    req.Indexer,
		"protocol":   req.Protocol,
	})

	writeJSON(w, http.StatusCreated, item)
}

// enqueueToDownloadClient prefers enabled qBittorrent rows, then plugin download clients.
func (s *Server) enqueueToDownloadClient(ctx context.Context, title, downloadURL, protocol string) (clientName, downloadID string, err error) {
	clients, err := s.store.ListDownloadClients()
	if err != nil {
		return "", "", err
	}
	for _, c := range clients {
		if !c.Enable {
			continue
		}
		if strings.EqualFold(c.Implementation, "qBittorrent") {
			cli := qbittorrent.NewFromDownloadClient(c)
			id, addErr := cli.Add(ctx, title, downloadURL, protocol)
			if addErr != nil {
				return "", "", addErr
			}
			return c.Name, id, nil
		}
	}
	if s.pluginMgr != nil {
		id, qErr := s.pluginMgr.EnqueueDownload(ctx, title, downloadURL, protocol)
		if qErr == nil && id != "" {
			name, _ := s.requireEnabledDownloadClient()
			if name == "" {
				name = "plugin"
			}
			return name, id, nil
		}
		if qErr != nil {
			return "", "", qErr
		}
	}
	return "", "", fmt.Errorf("no enabled download client configured; add qBittorrent in Queue & Clients before grabbing")
}

// requireEnabledDownloadClient returns the name of an enabled DB download client, or an error.
func (s *Server) requireEnabledDownloadClient() (string, error) {
	clients, err := s.store.ListDownloadClients()
	if err != nil {
		return "", err
	}
	for _, c := range clients {
		if c.Enable {
			return c.Name, nil
		}
	}
	return "", fmt.Errorf("no enabled download client configured; add one in Queue & Clients or Settings before grabbing")
}

func (s *Server) handleManualRelease(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MagnetURL      string `json:"magnetUrl"`
		DownloadURL    string `json:"downloadUrl"`
		TorrentBase64  string `json:"torrentBase64"`
		Title          string `json:"title"`
		SeriesID       int64  `json:"seriesId"`
		MovieID        int64  `json:"movieId"`
		EpisodeID      int64  `json:"episodeId"`
		SeasonNumber   int    `json:"seasonNumber"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	url := strings.TrimSpace(req.DownloadURL)
	if url == "" {
		url = strings.TrimSpace(req.MagnetURL)
	}
	if url == "" && req.TorrentBase64 == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("magnetUrl, downloadUrl, or torrentBase64 required"))
		return
	}
	if req.Title == "" {
		req.Title = "Manual.Release"
	}
	mediaType := "tv"
	if req.MovieID > 0 {
		mediaType = "movie"
	}
	protocol := "torrent"
	if url == "" && req.TorrentBase64 != "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("torrent file upload is not supported yet; use a magnet or HTTP .torrent URL"))
		return
	}
	clientName, downloadID, err := s.enqueueToDownloadClient(r.Context(), req.Title, url, protocol)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err)
		return
	}
	item, err := s.store.CreateQueueItem(domain.QueueItem{
		MediaType:             mediaType,
		SeriesID:              req.SeriesID,
		EpisodeID:             req.EpisodeID,
		MovieID:               req.MovieID,
		Title:                 req.Title,
		Status:                "queued",
		TrackedDownloadStatus: "ok",
		TrackedDownloadState:  "downloading",
		Size:                  0,
		Sizeleft:              0,
		Timeleft:              "",
		Protocol:              protocol,
		DownloadClient:        clientName,
		DownloadID:            downloadID,
		Indexer:               "manual",
		OutputPath:            "",
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	_ = s.store.CreateHistory(domain.HistoryRecord{
		MediaType:   mediaType,
		SeriesID:    req.SeriesID,
		EpisodeID:   req.EpisodeID,
		MovieID:     req.MovieID,
		SourceTitle: req.Title,
		Quality:     map[string]any{"quality": map[string]any{"name": "Unknown"}},
		EventType:   "grabbed",
		Data: map[string]any{
			"downloadUrl":  url,
			"seasonNumber": req.SeasonNumber,
			"manual":       true,
		},
	})
	s.emitNotification(r.Context(), pluginsdk.EventDownloadStarted, map[string]any{
		"title":      req.Title,
		"queueId":    item.ID,
		"downloadId": downloadID,
		"mediaType":  mediaType,
		"indexer":    "manual",
		"protocol":   protocol,
		"manual":     true,
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
	if s.pluginMgr != nil {
		s.pluginMgr.CheckStalledDownloads(r.Context(), items)
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
