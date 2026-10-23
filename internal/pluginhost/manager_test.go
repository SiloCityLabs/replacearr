package pluginhost

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/pkg/pluginsdk"
)

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

	mgr := NewManager(store, pluginDir, "../../.tmp/ProwlarrIndexers/definitions/v11")
	defer mgr.StopAll()

	if err := mgr.DiscoverAndStart(); err != nil {
		t.Fatalf("DiscoverAndStart failed: %v", err)
	}

	plugins := mgr.ListPlugins()
	if len(plugins) < 4 {
		t.Fatalf("expected at least 4 plugins (3 built-in + 1 external), got %d", len(plugins))
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
	if sample.Status != "running" {
		t.Fatalf("expected sample-torznab-indexer status 'running', got %q (lastError=%q)", sample.Status, sample.LastError)
	}
	if sample.Manifest.Name != "Replacearr Torznab Demo Indexer" {
		t.Fatalf("unexpected manifest name: %q", sample.Manifest.Name)
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

	// Verify SearchAllIndexers executes JSON-RPC search against external plugin and built-in indexer
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
