package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/replacearr/replacearr/internal/api/compat/prowlarr"
	"github.com/replacearr/replacearr/internal/api/compat/radarr"
	"github.com/replacearr/replacearr/internal/api/compat/sonarr"
	"github.com/replacearr/replacearr/internal/api/native"
	"github.com/replacearr/replacearr/internal/auth"
	"github.com/replacearr/replacearr/internal/config"
	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/downloadmonitor"
	"github.com/replacearr/replacearr/internal/pluginhost"
	"github.com/replacearr/replacearr/internal/webui"
	"github.com/replacearr/replacearr/pkg/pluginsdk"
)

var (
	Version   = "0.1.0"
	Commit    = "dev"
	BuildTime = "unknown"
)

func main() {
	cfg := config.Load()

	store, err := db.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("failed to open sqlite database (%s): %v", cfg.DatabasePath, err)
	}
	defer store.Close()

	authSvc, err := auth.NewService(store, cfg)
	if err != nil {
		log.Fatalf("failed to initialize auth service: %v", err)
	}

	pluginMgr := pluginhost.NewManager(store, cfg.PluginDir, cfg.DefinitionsDir)
	if err := pluginMgr.DiscoverAndStart(); err != nil {
		log.Printf("warning: plugin discovery error: %v", err)
	}
	defer pluginMgr.StopAll()

	nativeSrv := native.NewServer(store, cfg, authSvc, pluginMgr, Version)

	monitorCtx, monitorCancel := context.WithCancel(context.Background())
	defer monitorCancel()
	go downloadmonitor.New(store, pluginMgr, nativeSrv).Run(monitorCtx)

	mux := http.NewServeMux()

	// 1. Auth routes (/api/v1/auth/*, users, roles, apikeys)
	authSvc.RegisterRoutes(mux)

	// 2. Native unified API owns /api/v1/* (do not remount prowlarr over this).
	nativeSrv.RegisterRoutes(mux)

	// 3. *arr compatibility mounts — always registered, gated by builtin-compat-* plugins
	// so enable/disable takes effect without restart.
	sonarrHandler := sonarr.NewHandler(store, cfg, nativeSrv.SearchReleases)
	radarrHandler := radarr.NewHandler(store, cfg, nativeSrv.SearchReleases)
	prowlarrHandler := prowlarr.NewHandler(store, cfg, nativeSrv.SearchReleases)
	sonarrHandler.SetCoverStore(nativeSrv.CoverStore())
	radarrHandler.SetCoverStore(nativeSrv.CoverStore())
	sonarrHandler.SetMetadata(nativeSrv.Metadata())
	radarrHandler.SetMetadata(nativeSrv.Metadata())

	mux.Handle("/sonarr/", authSvc.Middleware("sonarr", "", pluginMgr.Gate(pluginsdk.CompatPluginSonarr, sonarrHandler)))
	mux.Handle("/api/v3/", authSvc.Middleware("sonarr", "", pluginMgr.Gate(pluginsdk.CompatPluginSonarr, sonarrHandler)))
	mux.Handle("/radarr/", authSvc.Middleware("radarr", "", pluginMgr.Gate(pluginsdk.CompatPluginRadarr, radarrHandler)))
	mux.Handle("/prowlarr/", authSvc.Middleware("prowlarr", "", pluginMgr.Gate(pluginsdk.CompatPluginProwlarr, prowlarrHandler)))

	// 4. Embedded React SPA
	mux.Handle("/", webui.NewHandler())

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           corsMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Replacearr v%s (%s) listening on http://%s", Version, Commit, addr)
		log.Printf("  -> Unified Web UI & Native API : http://localhost:%d", cfg.Port)
		if pluginMgr.IsEnabled(pluginsdk.CompatPluginSonarr) {
			log.Printf("  -> Sonarr v3 Compatibility     : http://localhost:%d/sonarr/api/v3", cfg.Port)
		} else {
			log.Printf("  -> Sonarr v3 Compatibility     : disabled (plugin)")
		}
		if pluginMgr.IsEnabled(pluginsdk.CompatPluginRadarr) {
			log.Printf("  -> Radarr v3 Compatibility     : http://localhost:%d/radarr/api/v3", cfg.Port)
		} else {
			log.Printf("  -> Radarr v3 Compatibility     : disabled (plugin)")
		}
		if pluginMgr.IsEnabled(pluginsdk.CompatPluginProwlarr) {
			log.Printf("  -> Prowlarr v1 Compatibility   : http://localhost:%d/prowlarr/api/v1", cfg.Port)
		} else {
			log.Printf("  -> Prowlarr v1 Compatibility   : disabled (plugin)")
		}
		log.Printf("  -> Master API Key              : %s", authSvc.GetMasterAPIKey())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down Replacearr...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Api-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
