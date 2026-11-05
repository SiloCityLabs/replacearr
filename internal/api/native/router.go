package native

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/auth"
	"github.com/replacearr/replacearr/internal/config"
	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/domain"
	"github.com/replacearr/replacearr/internal/downloadclient/deluge"
	"github.com/replacearr/replacearr/internal/downloadclient/nzbget"
	"github.com/replacearr/replacearr/internal/downloadclient/qbittorrent"
	"github.com/replacearr/replacearr/internal/downloadclient/sabnzbd"
	"github.com/replacearr/replacearr/internal/downloadclient/transmission"
	"github.com/replacearr/replacearr/internal/flaresolverr"
	"github.com/replacearr/replacearr/internal/httpclient"
	"github.com/replacearr/replacearr/internal/indexers/cardigann"
	"github.com/replacearr/replacearr/internal/mediacover"
	"github.com/replacearr/replacearr/internal/metadata"
	"github.com/replacearr/replacearr/internal/plugincatalog"
	"github.com/replacearr/replacearr/internal/pluginhost"
	"github.com/replacearr/replacearr/internal/subtitles"
	"github.com/replacearr/replacearr/internal/tasks"
	"github.com/replacearr/replacearr/pkg/pluginsdk"
)

// QueueProgressSyncer refreshes live download progress/ETA from clients.
type QueueProgressSyncer interface {
	SyncProgress(ctx context.Context)
}

type Server struct {
	store         *db.Store
	cfg           *config.Config
	authSvc       *auth.Service
	pluginMgr     *pluginhost.Manager
	pluginCatalog *plugincatalog.Client
	meta          *metadata.Service
	flareClient   *flaresolverr.Client
	covers        *mediacover.Store
	subtitles     *subtitles.Service
	scheduler     *tasks.Scheduler
	queueSync     QueueProgressSyncer
	version       string
	startedAt     time.Time
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
	pluginDir := ""
	catalogURL := plugincatalog.DefaultCatalogURL
	if cfg != nil {
		if cfg.ConfigDir != "" {
			configDir = cfg.ConfigDir
		}
		pluginDir = cfg.PluginDir
		if cfg.PluginCatalogURL != "" {
			catalogURL = cfg.PluginCatalogURL
		}
	}
	s := &Server{
		store:         store,
		cfg:           cfg,
		authSvc:       authSvc,
		pluginMgr:     pluginMgr,
		pluginCatalog: plugincatalog.New(catalogURL, pluginDir),
		meta:          metaSvc,
		flareClient:   flare,
		covers:        mediacover.NewStore(configDir, httpCli),
		subtitles:     subtitles.NewService(store),
		version:       version,
		startedAt:     time.Now().UTC(),
	}
	s.seedInitialDemoData()
	return s
}

// Subtitles exposes the native subtitle service for download-monitor hooks.
func (s *Server) Subtitles() *subtitles.Service {
	return s.subtitles
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
	return s.SearchReleasesEx(query, mediaType, tvdbID, tmdbID, season, episode, "", "")
}

// SearchReleasesEx extends SearchReleases with optional artist/album or author/title hints for music/books.
func (s *Server) SearchReleasesEx(query string, mediaType string, tvdbID int, tmdbID int, season int, episode int, artist, albumOrAuthor string) ([]domain.ReleaseResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	now := time.Now().UTC()
	baseTitle := strings.TrimSpace(query)
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if mediaType == "" || mediaType == "search" {
		mediaType = "tv"
	}

	type queryVariant struct {
		Query   string
		Season  int
		Episode int
		Mode    string
	}
	var variants []queryVariant

	switch mediaType {
	case "music":
		for _, q := range domain.BuildMusicSearchQueries(artist, albumOrAuthor, baseTitle) {
			variants = append(variants, queryVariant{Query: q, Mode: "music"})
		}
	case "ebook", "audiobook", "book":
		author := albumOrAuthor
		title := baseTitle
		if title == "" {
			title = artist // allow artist field reuse as title when only one string passed
		}
		for _, q := range domain.BuildBookSearchQueries(author, title) {
			mode := "book"
			if mediaType == "audiobook" {
				mode = "book"
			}
			variants = append(variants, queryVariant{Query: q, Mode: mode})
		}
	default:
		var aliases []domain.SeriesAlias
		var eps []domain.Episode
		var series domain.Series
		if tvdbID > 0 {
			if all, err := s.store.ListSeries(); err == nil {
				for _, sr := range all {
					if sr.TvdbID != tvdbID {
						continue
					}
					series = sr
					if baseTitle == "" {
						baseTitle = sr.Title
					}
					if listed, aErr := s.store.ListSeriesAliases(sr.ID); aErr == nil {
						aliases = listed
					}
					if listedEps, eErr := s.store.ListEpisodes(sr.ID); eErr == nil {
						eps = listedEps
					}
					break
				}
			}
		}
		if baseTitle == "" {
			baseTitle = strings.TrimSpace(query)
		}
		preferPack := false
		if season > 0 {
			preferPack = domain.PreferSeasonPackQuery(series, eps, season, now)
		}
		for _, v := range domain.BuildTVSearchVariants(baseTitle, aliases, eps, season, episode, preferPack, now) {
			variants = append(variants, queryVariant{Query: v.Query, Season: v.Season, Episode: v.Episode, Mode: mediaType})
		}
	}
	if len(variants) == 0 {
		variants = []queryVariant{{Query: baseTitle, Season: season, Episode: episode, Mode: mediaType}}
	}

	proxyURL := ""
	if s.pluginMgr != nil {
		proxyURL = s.pluginMgr.ActiveProxyURL()
	}

	results := []domain.ReleaseResult{}
	var searchErrs []string
	seen := map[string]struct{}{}
	addResults := func(batch []domain.ReleaseResult) {
		for _, r := range batch {
			key := r.Guid
			if key == "" {
				key = r.DownloadURL
			}
			if key == "" {
				key = r.Title + "|" + r.Indexer
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			results = append(results, r)
		}
	}

	for _, v := range variants {
		if ctx.Err() != nil {
			break
		}
		req := pluginsdk.SearchRequest{
			Query:           v.Query,
			Type:            v.Mode,
			TvdbID:          tvdbID,
			TmdbID:          tmdbID,
			Season:          v.Season,
			Episode:         v.Episode,
			Limit:           50,
			FlareSolverrURL: s.activeFlareSolverrHost(),
			ProxyURL:        proxyURL,
		}
		if s.pluginMgr != nil {
			pluginResults, err := s.pluginMgr.SearchAllIndexers(ctx, req)
			if err != nil {
				searchErrs = append(searchErrs, err.Error())
			} else if pluginResults != nil {
				addResults(pluginResults)
			}
		}
		cardigannResults, cardigannErrs := s.searchCardigannIndexers(ctx, v.Query, v.Season, v.Episode, proxyURL, v.Mode)
		addResults(cardigannResults)
		searchErrs = append(searchErrs, cardigannErrs...)
	}

	if len(results) == 0 && len(searchErrs) > 0 {
		return results, fmt.Errorf("search failed: %s", strings.Join(searchErrs, "; "))
	}
	return results, nil
}

func (s *Server) searchCardigannIndexers(ctx context.Context, query string, season, episode int, proxyURL, searchMode string) ([]domain.ReleaseResult, []string) {
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
			SearchMode:     searchMode,
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

	// Inbound plugin webhooks (secret from Manifest.InboundWebhooks; no JWT).
	// pluginId may be a plugin id or a PathAlias (e.g. "jellyfin").
	mux.HandleFunc("POST /api/v1/webhooks/{pluginId}", s.handleInboundWebhook)
	mux.HandleFunc("POST /api/v1/webhooks/{pluginId}/{slug}", s.handleInboundWebhook)

	// Overview
	mux.HandleFunc("GET /api/v1/overview", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleOverview))

	// Series (Sonarr)
	mux.HandleFunc("GET /api/v1/series", s.authSvc.RequirePermission(domain.PermTVRead, s.handleListSeries))
	mux.HandleFunc("POST /api/v1/series", s.authSvc.RequirePermission(domain.PermTVWrite, s.handleCreateSeries))
	mux.HandleFunc("DELETE /api/v1/series/{id}", s.authSvc.RequirePermission(domain.PermTVDelete, s.handleDeleteSeries))
	mux.HandleFunc("GET /api/v1/series/lookup", s.authSvc.RequirePermission(domain.PermTVRead, s.handleLookupSeries))
	mux.HandleFunc("GET /api/v1/episodes", s.authSvc.RequirePermission(domain.PermTVRead, s.handleListEpisodes))
	mux.HandleFunc("GET /api/v1/episodes/{id}/subtitles", s.authSvc.RequirePermission(domain.PermTVRead, s.handleEpisodeSubtitles))
	mux.HandleFunc("GET /api/v1/episodes/{id}/subtitles/search", s.authSvc.RequirePermission(domain.PermTVRead, s.handleSearchEpisodeSubtitles))
	mux.HandleFunc("POST /api/v1/episodes/{id}/subtitles/download", s.authSvc.RequirePermission(domain.PermTVWrite, s.handleDownloadEpisodeSubtitle))
	mux.HandleFunc("POST /api/v1/episodes/{id}/subtitles/ensure", s.authSvc.RequirePermission(domain.PermTVWrite, s.handleEnsureEpisodeSubtitles))

	// Movies (Radarr)
	mux.HandleFunc("GET /api/v1/movies", s.authSvc.RequirePermission(domain.PermMoviesRead, s.handleListMovies))
	mux.HandleFunc("POST /api/v1/movies", s.authSvc.RequirePermission(domain.PermMoviesWrite, s.handleCreateMovie))
	mux.HandleFunc("GET /api/v1/movies/lookup", s.authSvc.RequirePermission(domain.PermMoviesRead, s.handleLookupMovies))
	mux.HandleFunc("GET /api/v1/movies/{id}", s.authSvc.RequirePermission(domain.PermMoviesRead, s.handleGetMovie))
	mux.HandleFunc("PUT /api/v1/movies/{id}", s.authSvc.RequirePermission(domain.PermMoviesWrite, s.handleUpdateMovie))
	mux.HandleFunc("DELETE /api/v1/movies/{id}", s.authSvc.RequirePermission(domain.PermMoviesDelete, s.handleDeleteMovie))
	mux.HandleFunc("POST /api/v1/movies/{id}/refresh", s.authSvc.RequirePermission(domain.PermMoviesWrite, s.handleRefreshMovieMetadata))
	mux.HandleFunc("GET /api/v1/movies/{id}/files", s.authSvc.RequirePermission(domain.PermMoviesRead, s.handleMovieFiles))
	mux.HandleFunc("GET /api/v1/movies/{id}/subtitles", s.authSvc.RequirePermission(domain.PermMoviesRead, s.handleMovieSubtitles))
	mux.HandleFunc("GET /api/v1/movies/{id}/subtitles/search", s.authSvc.RequirePermission(domain.PermMoviesRead, s.handleSearchMovieSubtitles))
	mux.HandleFunc("POST /api/v1/movies/{id}/subtitles/download", s.authSvc.RequirePermission(domain.PermMoviesWrite, s.handleDownloadMovieSubtitle))
	mux.HandleFunc("POST /api/v1/movies/{id}/subtitles/ensure", s.authSvc.RequirePermission(domain.PermMoviesWrite, s.handleEnsureMovieSubtitles))
	mux.HandleFunc("GET /api/v1/subtitleprofiles", s.authSvc.RequirePermission(domain.PermProfilesRead, s.handleListSubtitleProfiles))
	mux.HandleFunc("PUT /api/v1/subtitleprofiles/{id}", s.authSvc.RequirePermission(domain.PermProfilesWrite, s.handleUpdateSubtitleProfile))

	s.registerMusicBooksRoutes(mux)

	// Indexers & Search (Prowlarr)
	mux.HandleFunc("GET /api/v1/indexers", s.authSvc.RequirePermission(domain.PermIndexersRead, s.handleListIndexers))
	mux.HandleFunc("POST /api/v1/indexers", s.authSvc.RequirePermission(domain.PermIndexersWrite, s.handleCreateIndexer))
	mux.HandleFunc("DELETE /api/v1/indexers/{id}", s.authSvc.RequirePermission(domain.PermIndexersWrite, s.handleDeleteIndexer))
	mux.HandleFunc("POST /api/v1/indexers/test", s.authSvc.RequirePermission(domain.PermIndexersRead, s.handleTestIndexer))
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
	mux.HandleFunc("POST /api/v1/downloadclients/discover", s.authSvc.RequirePermission(domain.PermDownloadsRead, s.handleDiscoverDownloadClients))

	// Plugins
	mux.HandleFunc("GET /api/v1/plugins", s.authSvc.RequirePermission(domain.PermPluginsRead, s.handleListPlugins))
	mux.HandleFunc("PUT /api/v1/plugins/{id}", s.authSvc.RequirePermission(domain.PermPluginsManage, s.handleUpdatePlugin))
	mux.HandleFunc("POST /api/v1/plugins/reload", s.authSvc.RequirePermission(domain.PermPluginsManage, s.handleReloadPlugins))
	mux.HandleFunc("POST /api/v1/plugins/{id}/test", s.authSvc.RequirePermission(domain.PermPluginsManage, s.handleTestNotificationPlugin))
	mux.HandleFunc("GET /api/v1/plugins/catalog", s.authSvc.RequirePermission(domain.PermPluginsRead, s.handlePluginCatalog))
	mux.HandleFunc("POST /api/v1/plugins/catalog/refresh", s.authSvc.RequirePermission(domain.PermPluginsManage, s.handlePluginCatalogRefresh))
	mux.HandleFunc("POST /api/v1/plugins/catalog/resolve", s.authSvc.RequirePermission(domain.PermPluginsManage, s.handlePluginCatalogResolve))
	mux.HandleFunc("POST /api/v1/plugins/catalog/install", s.authSvc.RequirePermission(domain.PermPluginsManage, s.handlePluginCatalogInstall))
	mux.HandleFunc("POST /api/v1/plugins/catalog/uninstall", s.authSvc.RequirePermission(domain.PermPluginsManage, s.handlePluginCatalogUninstall))
	mux.HandleFunc("GET /api/v1/notifications/stream", s.authSvc.RequirePermission(domain.PermSystemRead, s.handleNotificationStream))

	s.registerExpandedRoutes(mux)
}

func (s *Server) handleOverview(w http.ResponseWriter, _ *http.Request) {
	series, _ := s.store.ListSeries()
	episodes, _ := s.store.ListEpisodes(0)
	movies, _ := s.store.ListMovies()
	artists, _ := s.store.ListArtists()
	albums, _ := s.store.ListAlbums(0)
	authors, _ := s.store.ListAuthors()
	books, _ := s.store.ListBooks(0)
	editions, _ := s.store.ListEditions(0, "")
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
			"artists":              len(artists),
			"albums":               len(albums),
			"authors":              len(authors),
			"books":                len(books),
			"editions":             len(editions),
			"history":              len(history),
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
				"lidarr":       "/lidarr/api/v1",
				"readarr":      "/readarr/api/v1",
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
	for i := range items {
		items[i].Images = s.hydrateLibraryPosters("tv", items[i].ID, items[i].Images)
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
	created.Images = bindLocalPosterURL("tv", created.ID, created.Images)
	_, _ = s.store.UpdateSeries(*created)
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
	_, eps, aliases, err := s.meta.GetSeriesByTVDB(ctx, sr.TvdbID, s.preferredMetadataLanguage())
	if err != nil {
		return err
	}
	if len(aliases) > 0 {
		_ = s.store.ReplaceSeriesAliases(sr.ID, aliases)
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
	for i := range items {
		items[i].Images = s.hydrateLibraryPosters("movie", items[i].ID, items[i].Images)
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateMovie(w http.ResponseWriter, r *http.Request) {
	var req domain.Movie
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.TmdbID > 0 && s.meta != nil {
		if meta, err := s.meta.GetMovieByTMDB(r.Context(), req.TmdbID); err == nil {
			mergeMovieMetadata(&req, meta)
			if req.Title == "" {
				req.Title = meta.Title
			}
		}
	}
	created, err := s.store.CreateMovie(req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	created.Images = bindLocalPosterURL("movie", created.ID, created.Images)
	_, _ = s.store.UpdateMovie(*created)
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
	artist := r.URL.Query().Get("artist")
	album := r.URL.Query().Get("album")
	author := r.URL.Query().Get("author")
	if author == "" {
		author = album
	}
	// Resolve library IDs into query strings when provided.
	if artistID, _ := strconv.ParseInt(r.URL.Query().Get("artistId"), 10, 64); artistID > 0 {
		if a, err := s.store.GetArtistByID(artistID); err == nil && a != nil {
			artist = a.Title
			if q == "" {
				q = a.Title
			}
		}
	}
	if albumID, _ := strconv.ParseInt(r.URL.Query().Get("albumId"), 10, 64); albumID > 0 {
		if al, err := s.store.GetAlbumByID(albumID); err == nil && al != nil {
			album = al.Title
			if artist == "" {
				if a, err := s.store.GetArtistByID(al.ArtistID); err == nil && a != nil {
					artist = a.Title
				}
			}
			q = strings.TrimSpace(artist + " " + album)
			searchType = "music"
		}
	}
	if authorID, _ := strconv.ParseInt(r.URL.Query().Get("authorId"), 10, 64); authorID > 0 {
		if a, err := s.store.GetAuthorByID(authorID); err == nil && a != nil {
			author = a.Title
			if q == "" {
				q = a.Title
			}
		}
	}
	if bookID, _ := strconv.ParseInt(r.URL.Query().Get("bookId"), 10, 64); bookID > 0 {
		if b, err := s.store.GetBookByID(bookID); err == nil && b != nil {
			q = b.Title
			if author == "" {
				if a, err := s.store.GetAuthorByID(b.AuthorID); err == nil && a != nil {
					author = a.Title
				}
			}
			if searchType == "" || searchType == "search" {
				searchType = "book"
			}
		}
	}
	if editionID, _ := strconv.ParseInt(r.URL.Query().Get("editionId"), 10, 64); editionID > 0 {
		if e, err := s.store.GetEditionByID(editionID); err == nil && e != nil {
			q = e.Title
			searchType = e.Format
			if author == "" {
				if a, err := s.store.GetAuthorByID(e.AuthorID); err == nil && a != nil {
					author = a.Title
				}
			}
		}
	}
	second := album
	if searchType == "book" || searchType == "ebook" || searchType == "audiobook" {
		second = author
	}
	results, err := s.SearchReleasesEx(q, searchType, tvdbID, tmdbID, season, episode, artist, second)
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
		ArtistID     int64  `json:"artistId"`
		AlbumID      int64  `json:"albumId"`
		AuthorID     int64  `json:"authorId"`
		BookID       int64  `json:"bookId"`
		EditionID    int64  `json:"editionId"`
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
	blockMedia := req.MediaType
	if blockMedia == "" {
		switch {
		case req.MovieID > 0:
			blockMedia = "movies"
		case req.ArtistID > 0 || req.AlbumID > 0:
			blockMedia = "music"
		case req.BookID > 0 || req.AuthorID > 0 || req.EditionID > 0:
			blockMedia = "books"
		default:
			blockMedia = "tv"
		}
	}
	if blocked, _ := s.store.IsBlocklisted(blockMedia, req.Title, ""); blocked {
		_ = s.store.CreateHistory(domain.HistoryRecord{
			MediaType: blockMedia, SeriesID: req.SeriesID, MovieID: req.MovieID,
			ArtistID: req.ArtistID, AuthorID: req.AuthorID, BookID: req.BookID,
			SourceTitle: req.Title, EventType: "blocked",
			Quality: map[string]any{}, Data: map[string]any{"reason": "blocklist"},
		})
		writeErr(w, http.StatusConflict, fmt.Errorf("release is blocklisted"))
		return
	}
	if req.Protocol == "" {
		req.Protocol = "torrent"
	}
	if req.MediaType == "" {
		switch {
		case req.EditionID > 0:
			if e, err := s.store.GetEditionByID(req.EditionID); err == nil && e != nil {
				req.MediaType = e.Format
				if req.BookID == 0 {
					req.BookID = e.BookID
				}
				if req.AuthorID == 0 {
					req.AuthorID = e.AuthorID
				}
			}
		case req.AlbumID > 0 || req.ArtistID > 0:
			req.MediaType = "music"
		case req.BookID > 0 || req.AuthorID > 0:
			req.MediaType = "ebook"
		case req.MovieID > 0:
			req.MediaType = "movie"
		default:
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
		ArtistID:              req.ArtistID,
		AlbumID:               req.AlbumID,
		AuthorID:              req.AuthorID,
		BookID:                req.BookID,
		EditionID:             req.EditionID,
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
		ArtistID:    req.ArtistID,
		AlbumID:     req.AlbumID,
		AuthorID:    req.AuthorID,
		BookID:      req.BookID,
		EditionID:   req.EditionID,
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

// enqueueToDownloadClient sends to the first enabled built-in client matching
// the release protocol (torrent → qBittorrent/Transmission/Deluge, usenet →
// SABnzbd/NZBGet), then download-client plugins.
func (s *Server) enqueueToDownloadClient(ctx context.Context, title, downloadURL, protocol string) (clientName, downloadID string, err error) {
	wantProto := strings.ToLower(strings.TrimSpace(protocol))
	if wantProto == "" {
		wantProto = "torrent"
	}
	clients, err := s.store.ListDownloadClients()
	if err != nil {
		return "", "", err
	}
	var matchedEnabled bool
	for _, c := range clients {
		if !c.Enable {
			continue
		}
		if strings.EqualFold(c.Implementation, "Plugin") {
			continue // handled via pluginMgr below
		}
		clientProto := domain.ProtocolForDownloadClientImplementation(c.Implementation, c.Protocol)
		if !strings.EqualFold(clientProto, wantProto) {
			continue
		}
		matchedEnabled = true
		var id string
		var addErr error
		switch {
		case strings.EqualFold(c.Implementation, "qBittorrent"):
			id, addErr = qbittorrent.NewFromDownloadClient(c).Add(ctx, title, downloadURL, protocol)
		case strings.EqualFold(c.Implementation, "Transmission"):
			id, addErr = transmission.NewFromDownloadClient(c).Add(ctx, title, downloadURL, protocol)
		case strings.EqualFold(c.Implementation, "Deluge"):
			id, addErr = deluge.NewFromDownloadClient(c).Add(ctx, title, downloadURL, protocol)
		case strings.EqualFold(c.Implementation, "SABnzbd"):
			id, addErr = sabnzbd.NewFromDownloadClient(c).Add(ctx, title, downloadURL, protocol)
		case strings.EqualFold(c.Implementation, "NZBGet"):
			id, addErr = nzbget.NewFromDownloadClient(c).Add(ctx, title, downloadURL, protocol)
		default:
			continue
		}
		if addErr != nil {
			return "", "", addErr
		}
		return c.Name, id, nil
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
		if qErr != nil && !strings.Contains(qErr.Error(), "no enabled download client plugin") {
			return "", "", qErr
		}
	}
	if matchedEnabled {
		return "", "", fmt.Errorf("enabled download client(s) for %s could not accept the grab", wantProto)
	}
	return "", "", fmt.Errorf("no enabled %s download client configured; add one in Queue & Clients before grabbing", wantProto)
}

// enqueueTorrentFile sends a raw .torrent to the first enabled built-in torrent client.
func (s *Server) enqueueTorrentFile(ctx context.Context, title, filename string, data []byte) (clientName, downloadID string, err error) {
	if len(data) == 0 {
		return "", "", fmt.Errorf("empty torrent file")
	}
	clients, err := s.store.ListDownloadClients()
	if err != nil {
		return "", "", err
	}
	var matchedEnabled bool
	for _, c := range clients {
		if !c.Enable {
			continue
		}
		if strings.EqualFold(c.Implementation, "Plugin") {
			continue
		}
		clientProto := domain.ProtocolForDownloadClientImplementation(c.Implementation, c.Protocol)
		if !strings.EqualFold(clientProto, "torrent") {
			continue
		}
		matchedEnabled = true
		var id string
		var addErr error
		switch {
		case strings.EqualFold(c.Implementation, "qBittorrent"):
			id, addErr = qbittorrent.NewFromDownloadClient(c).AddFile(ctx, title, filename, data)
		case strings.EqualFold(c.Implementation, "Transmission"):
			id, addErr = transmission.NewFromDownloadClient(c).AddFile(ctx, title, filename, data)
		case strings.EqualFold(c.Implementation, "Deluge"):
			id, addErr = deluge.NewFromDownloadClient(c).AddFile(ctx, title, filename, data)
		default:
			continue
		}
		if addErr != nil {
			return "", "", addErr
		}
		return c.Name, id, nil
	}
	if matchedEnabled {
		return "", "", fmt.Errorf("enabled torrent download client(s) could not accept the torrent file")
	}
	return "", "", fmt.Errorf("no enabled torrent download client configured; add qBittorrent, Transmission, or Deluge before uploading a .torrent file")
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

const maxManualTorrentBytes = 16 << 20 // 16 MiB

func decodeTorrentBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("torrentBase64 is empty")
	}
	if i := strings.Index(s, ","); i >= 0 && strings.Contains(strings.ToLower(s[:i]), "base64") {
		s = s[i+1:]
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(s)
	}
	if err != nil {
		return nil, fmt.Errorf("invalid torrentBase64: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("decoded torrent file is empty")
	}
	if len(raw) > maxManualTorrentBytes {
		return nil, fmt.Errorf("torrent file too large (max %d bytes)", maxManualTorrentBytes)
	}
	return raw, nil
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
	var clientName, downloadID string
	var err error
	if url != "" {
		clientName, downloadID, err = s.enqueueToDownloadClient(r.Context(), req.Title, url, protocol)
	} else {
		raw, decErr := decodeTorrentBase64(req.TorrentBase64)
		if decErr != nil {
			writeErr(w, http.StatusBadRequest, decErr)
			return
		}
		filename := req.Title
		if !strings.HasSuffix(strings.ToLower(filename), ".torrent") {
			filename = strings.TrimSpace(filename) + ".torrent"
		}
		clientName, downloadID, err = s.enqueueTorrentFile(r.Context(), req.Title, filename, raw)
	}
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
	if s.queueSync != nil {
		s.queueSync.SyncProgress(r.Context())
	}
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

func (s *Server) installedCatalogInfos() []plugincatalog.InstalledInfo {
	if s.pluginMgr == nil {
		return nil
	}
	plugins := s.pluginMgr.ListPlugins()
	out := make([]plugincatalog.InstalledInfo, 0, len(plugins))
	for _, p := range plugins {
		builtin := strings.HasPrefix(p.BinaryPath, "builtin://") || p.Status == "builtin"
		out = append(out, plugincatalog.InstalledInfo{
			ID:         p.Manifest.ID,
			Version:    p.Manifest.Version,
			BinaryPath: p.BinaryPath,
			Builtin:    builtin,
		})
	}
	return out
}

func (s *Server) handlePluginCatalog(w http.ResponseWriter, r *http.Request) {
	if s.pluginCatalog == nil {
		writeErr(w, http.StatusServiceUnavailable, fmt.Errorf("plugin catalog is not configured"))
		return
	}
	force := r.URL.Query().Get("refresh") == "1" || r.URL.Query().Get("refresh") == "true"
	entries, err := s.pluginCatalog.List(r.Context(), s.installedCatalogInfos(), force)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"catalogUrl": s.pluginCatalog.CatalogURL(),
		"plugins":    entries,
	})
}

func (s *Server) handlePluginCatalogRefresh(w http.ResponseWriter, r *http.Request) {
	if s.pluginCatalog == nil {
		writeErr(w, http.StatusServiceUnavailable, fmt.Errorf("plugin catalog is not configured"))
		return
	}
	s.pluginCatalog.Invalidate()
	entries, err := s.pluginCatalog.List(r.Context(), s.installedCatalogInfos(), true)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"catalogUrl": s.pluginCatalog.CatalogURL(),
		"plugins":    entries,
	})
}

func (s *Server) handlePluginCatalogResolve(w http.ResponseWriter, r *http.Request) {
	if s.pluginCatalog == nil {
		writeErr(w, http.StatusServiceUnavailable, fmt.Errorf("plugin catalog is not configured"))
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	resolved, err := s.pluginCatalog.ResolveGitHub(r.Context(), req.URL)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	_, hasAsset := plugincatalog.SelectAsset(resolved.Plugin, runtime.GOOS, runtime.GOARCH)
	writeJSON(w, http.StatusOK, map[string]any{
		"plugin":           resolved.Plugin,
		"repoUrl":          resolved.RepoURL,
		"manifestUrl":      resolved.ManifestURL,
		"hasPlatformAsset": hasAsset,
	})
}

func (s *Server) handlePluginCatalogInstall(w http.ResponseWriter, r *http.Request) {
	if s.pluginCatalog == nil || s.pluginMgr == nil {
		writeErr(w, http.StatusServiceUnavailable, fmt.Errorf("plugin catalog is not configured"))
		return
	}
	var req struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	req.URL = strings.TrimSpace(req.URL)
	if req.ID == "" && req.URL == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("id or url is required"))
		return
	}

	var installedID string
	if req.URL != "" {
		// Preview id so we can stop a prior copy before replacing the binary.
		resolved, err := s.pluginCatalog.ResolveGitHub(r.Context(), req.URL)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		installedID = resolved.Plugin.ID
		_ = s.pluginMgr.RemoveExternalPlugin(installedID)
		plugin, err := s.pluginCatalog.InstallFromURL(r.Context(), req.URL)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		installedID = plugin.ID
	} else {
		installedID = req.ID
		_ = s.pluginMgr.RemoveExternalPlugin(installedID)
		if err := s.pluginCatalog.Install(r.Context(), installedID); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
	}
	if err := s.pluginMgr.DiscoverAndStart(); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"id":      installedID,
		"plugins": s.pluginMgr.ListPlugins(),
	})
}

func (s *Server) handlePluginCatalogUninstall(w http.ResponseWriter, r *http.Request) {
	if s.pluginCatalog == nil || s.pluginMgr == nil {
		writeErr(w, http.StatusServiceUnavailable, fmt.Errorf("plugin catalog is not configured"))
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("id is required"))
		return
	}
	if err := s.pluginMgr.RemoveExternalPlugin(req.ID); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.pluginCatalog.Uninstall(req.ID); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.pluginMgr.DiscoverAndStart(); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"id":      req.ID,
		"plugins": s.pluginMgr.ListPlugins(),
	})
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
