package pluginhost

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/pkg/pluginsdk"
)

func TestCompatBuiltinPluginsAndGate(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	// Seed legacy compatModules with radarr disabled before NewManager migrates.
	_, err = store.DB().Exec(`
		INSERT INTO app_config (key, value_json, updated_at)
		VALUES ('app', '{"compatModules":{"sonarr":true,"radarr":false,"prowlarr":true}}', CURRENT_TIMESTAMP)`)
	if err != nil {
		t.Fatalf("seed legacy compatModules: %v", err)
	}

	mgr := NewManager(store, filepath.Join(tmpDir, "plugins"), "")
	defer mgr.StopAll()

	if !mgr.IsEnabled(pluginsdk.CompatPluginSonarr) {
		t.Fatal("expected sonarr compat enabled after migration")
	}
	if mgr.IsEnabled(pluginsdk.CompatPluginRadarr) {
		t.Fatal("expected radarr compat disabled after migration from legacy compatModules")
	}
	if !mgr.IsEnabled(pluginsdk.CompatPluginProwlarr) {
		t.Fatal("expected prowlarr compat enabled after migration")
	}

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	sonarrGate := mgr.Gate(pluginsdk.CompatPluginSonarr, okHandler)
	radarrGate := mgr.Gate(pluginsdk.CompatPluginRadarr, okHandler)

	rr := httptest.NewRecorder()
	sonarrGate.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/sonarr/api/v3/system/status", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("sonarr gate status = %d, want 200", rr.Code)
	}

	rr = httptest.NewRecorder()
	radarrGate.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/radarr/api/v3/system/status", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("radarr gate status = %d, want 404 while disabled", rr.Code)
	}

	if _, err := mgr.ConfigurePlugin(pluginsdk.CompatPluginRadarr, true, map[string]any{}); err != nil {
		t.Fatalf("enable radarr: %v", err)
	}
	rr = httptest.NewRecorder()
	radarrGate.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/radarr/api/v3/system/status", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("radarr gate status = %d after enable, want 200", rr.Code)
	}

	found := 0
	for _, p := range mgr.ListPlugins() {
		if (p.Manifest.Types != nil) && hasPluginType(p.Manifest.Types, pluginsdk.PluginTypeCompat) {
			found++
		}
	}
	if found != 5 {
		t.Fatalf("expected 5 compat plugins in ListPlugins, got %d", found)
	}
	if !mgr.IsEnabled(pluginsdk.CompatPluginLidarr) || !mgr.IsEnabled(pluginsdk.CompatPluginReadarr) {
		t.Fatal("expected lidarr and readarr compat enabled by default")
	}
}

func TestManagerExternalPluginDiscoveryConfigureAndSearch(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	pluginDir := filepath.Join(tmpDir, "plugins")

	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	defer store.Close()

	// Compile examples/plugins/sample-indexer into pluginDir
	binPath := filepath.Join(pluginDir, "sample-indexer")
	buildCmd := exec.Command("go", "build", "-o", binPath, "../../examples/plugins/sample-indexer")
	buildCmd.Env = append(buildCmd.Environ(), "CGO_ENABLED=0")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("compile sample-indexer plugin failed: %v\n%s", err, string(out))
	}

	mgr := NewManager(store, pluginDir, t.TempDir())
	defer mgr.StopAll()

	if err := mgr.DiscoverAndStart(); err != nil {
		t.Fatalf("DiscoverAndStart failed: %v", err)
	}

	plugins := mgr.ListPlugins()
	// 5 compat + browser notifications + 1 external sample indexer
	if len(plugins) < 7 {
		t.Fatalf("expected at least 7 plugins (6 built-in + 1 external), got %d", len(plugins))
	}

	var sample *DiscoveredPlugin
	for i := range plugins {
		if plugins[i].Manifest.ID == "sample-torznab-indexer" {
			sample = &plugins[i]
			break
		}
	}
	if sample == nil {
		t.Fatalf("sample-torznab-indexer not discovered in %+v", plugins)
	}
	if sample.Enabled || sample.Status != "disabled" {
		t.Fatalf("expected sample-torznab-indexer disabled by default, got enabled=%v status=%q (lastError=%q)", sample.Enabled, sample.Status, sample.LastError)
	}
	if sample.Manifest.Name != "Replacearr Torznab Demo Indexer" {
		t.Fatalf("unexpected manifest name: %q", sample.Manifest.Name)
	}
	if !sample.Manifest.DisabledByDefault {
		t.Fatal("expected sample-torznab-indexer DisabledByDefault=true")
	}

	// Verify ConfigurePlugin updates settings on the external subprocess and persists to SQLite
	customURL := "https://custom-tracker.internal"
	updated, err := mgr.ConfigurePlugin("sample-torznab-indexer", true, map[string]any{
		"base_url":       customURL,
		"freeleech_only": true,
	})
	if err != nil {
		t.Fatalf("ConfigurePlugin failed: %v", err)
	}
	if !updated.Enabled || updated.Status != "running" {
		t.Fatalf("unexpected updated plugin status: %+v", updated)
	}
	if updated.Settings["base_url"] != customURL {
		t.Fatalf("expected base_url=%q, got %v", customURL, updated.Settings["base_url"])
	}

	// Verify SearchAllIndexers executes JSON-RPC search against the external plugin
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	releases, err := mgr.SearchAllIndexers(ctx, pluginsdk.SearchRequest{
		Query:   "Dune Part Two",
		Type:    "movie",
		TmdbID:  693134,
		ImdbID:  "tt15239678",
	})
	if err != nil {
		t.Fatalf("SearchAllIndexers failed: %v", err)
	}
	if len(releases) == 0 {
		t.Fatalf("expected releases from SearchAllIndexers, got 0")
	}

	var foundSampleCustomURL bool
	for _, rel := range releases {
		if rel.Indexer == "Replacearr Torznab Demo Indexer" && strings.HasPrefix(rel.DownloadURL, customURL) {
			foundSampleCustomURL = true
			break
		}
	}
	if !foundSampleCustomURL {
		t.Fatalf("expected release from Replacearr Torznab Demo Indexer using configured base_url %q; got %+v", customURL, releases)
	}

	// Verify Cardigann definitions loaded
	defs := mgr.ListCardigannDefinitions()
	if len(defs) == 0 {
		t.Fatalf("expected non-empty Cardigann definitions")
	}

	// Verify disabling the plugin stops it and excludes it from SearchAllIndexers
	disabled, err := mgr.ConfigurePlugin("sample-torznab-indexer", false, map[string]any{
		"base_url": customURL,
	})
	if err != nil {
		t.Fatalf("ConfigurePlugin disable failed: %v", err)
	}
	if disabled.Enabled || disabled.Status != "disabled" {
		t.Fatalf("expected disabled status, got %+v", disabled)
	}
}
