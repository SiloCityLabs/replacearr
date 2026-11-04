package plugincatalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallAndUninstallRegistryV2(t *testing.T) {
	payload := []byte("#!/bin/sh\necho replacearr-plugin\n")
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/registry.json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Registry{
			SchemaVersion: SchemaVersion,
			Plugins:       []RegistryEntry{{ID: "demo-plugin", Manifest: "manifests/demo-plugin.json"}},
		})
	})
	mux.HandleFunc("/manifests/demo-plugin.json", func(w http.ResponseWriter, r *http.Request) {
		plugin := Plugin{
			ID:      "demo-plugin",
			Name:    "Demo",
			Version: "1.2.0",
			Author:  "test",
			Types:   []string{"notification"},
			Assets: []Asset{{
				GOOS:   runtime.GOOS,
				GOARCH: runtime.GOARCH,
				URL:    "https://example.invalid/bin",
				SHA256: digest,
				Size:   int64(len(payload)),
			}},
		}
		_ = json.NewEncoder(w).Encode(plugin)
	})
	mux.HandleFunc("/bin", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	pluginDir := t.TempDir()
	client := New(srv.URL+"/registry.json", pluginDir)
	client.httpClient = srv.Client()

	idx, err := client.Fetch(context.Background(), true)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(idx.Plugins) != 1 {
		t.Fatalf("want 1 plugin, got %d", len(idx.Plugins))
	}
	idx.Plugins[0].Assets[0].URL = srv.URL + "/bin"
	client.mu.Lock()
	client.cached = idx
	client.mu.Unlock()

	if err := client.Install(context.Background(), "demo-plugin"); err != nil {
		t.Fatalf("Install: %v", err)
	}
	dest := BinaryPath(pluginDir, "demo-plugin")
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read installed: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("installed bytes mismatch")
	}
	origin, ok := ReadOrigin(pluginDir, "demo-plugin")
	if !ok || origin.Source != OriginRegistry || origin.InstalledVersion != "1.2.0" {
		t.Fatalf("origin sidecar missing/wrong: %+v ok=%v", origin, ok)
	}

	entries, err := client.List(context.Background(), []InstalledInfo{{
		ID: "demo-plugin", Version: "1.0.0",
	}}, false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || !entries[0].Installed || !entries[0].UpdateAvailable {
		t.Fatalf("unexpected catalog entry: %+v", entries)
	}

	if err := client.Uninstall("demo-plugin"); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("expected binary removed, err=%v", err)
	}
	if _, ok := ReadOrigin(pluginDir, "demo-plugin"); ok {
		t.Fatal("expected origin sidecar removed")
	}
}

func TestLegacySchemaV1StillLoads(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/index.json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Index{
			SchemaVersion: LegacySchemaV1,
			Plugins: []Plugin{{
				ID: "legacy-plugin", Name: "Legacy", Version: "0.1.0",
				Assets: []Asset{{GOOS: "linux", GOARCH: "amd64", URL: "https://x/a", SHA256: "aa", Size: 1}},
			}},
		})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	client := New(srv.URL+"/index.json", t.TempDir())
	client.httpClient = srv.Client()
	idx, err := client.Fetch(context.Background(), true)
	if err != nil {
		t.Fatalf("Fetch legacy: %v", err)
	}
	if idx.SchemaVersion != LegacySchemaV1 || len(idx.Plugins) != 1 {
		t.Fatalf("unexpected index: %+v", idx)
	}
}

func TestRejectNonHTTPSAndBadID(t *testing.T) {
	client := New("http://example.com/index.json", t.TempDir())
	if _, err := client.Fetch(context.Background(), true); err == nil {
		t.Fatal("expected non-HTTPS catalog URL to fail")
	}
	if err := validatePluginID("builtin-compat-sonarr"); err == nil {
		t.Fatal("expected builtin id rejection")
	}
	if err := validatePluginID("../evil"); err == nil {
		t.Fatal("expected path traversal rejection")
	}
}

func TestParseGitHubPluginURL(t *testing.T) {
	owner, repo, tag, err := ParseGitHubPluginURL("replacearr/plugin-http-proxy")
	if err != nil || owner != "replacearr" || repo != "plugin-http-proxy" || tag != "" {
		t.Fatalf("got %s/%s tag=%q err=%v", owner, repo, tag, err)
	}
	owner, repo, tag, err = ParseGitHubPluginURL("https://github.com/replacearr/plugin-http-proxy/releases/tag/v0.1.0")
	if err != nil || owner != "replacearr" || repo != "plugin-http-proxy" || tag != "v0.1.0" {
		t.Fatalf("got %s/%s tag=%q err=%v", owner, repo, tag, err)
	}
	if _, _, _, err := ParseGitHubPluginURL("https://gitlab.com/x/y"); err == nil {
		t.Fatal("expected non-github rejection")
	}
}

func TestSelectAssetAndVersionGreater(t *testing.T) {
	p := Plugin{Assets: []Asset{
		{GOOS: "linux", GOARCH: "amd64", URL: "https://x/a", SHA256: "aa"},
		{GOOS: "darwin", GOARCH: "arm64", URL: "https://x/b", SHA256: "bb"},
	}}
	a, ok := SelectAsset(p, "darwin", "arm64")
	if !ok || a.SHA256 != "bb" {
		t.Fatalf("SelectAsset failed: %+v ok=%v", a, ok)
	}
	if !versionGreater("1.2.0", "1.1.9") {
		t.Fatal("expected 1.2.0 > 1.1.9")
	}
	if versionGreater("1.0.0", "1.0.0") {
		t.Fatal("equal versions should not be greater")
	}
	_ = filepath.Join
}

func TestRejectOffOriginManifest(t *testing.T) {
	_, err := resolveManifestRef(
		"https://raw.githubusercontent.com/replacearr/replacearr-plugins/main/registry.json",
		"https://evil.example/manifest.json",
	)
	if err == nil {
		t.Fatal("expected off-origin absolute manifest rejection")
	}
}
