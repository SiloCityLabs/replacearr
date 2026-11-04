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

func TestInstallAndUninstall(t *testing.T) {
	payload := []byte("#!/bin/sh\necho replacearr-plugin\n")
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/index.json", func(w http.ResponseWriter, _ *http.Request) {
		idx := Index{
			SchemaVersion: SchemaVersion,
			UpdatedAt:     "2026-11-04T00:00:00Z",
			Plugins: []Plugin{{
				ID:      "demo-plugin",
				Name:    "Demo",
				Version: "1.2.0",
				Author:  "test",
				Types:   []string{"notification"},
				Assets: []Asset{{
					GOOS:   runtime.GOOS,
					GOARCH: runtime.GOARCH,
					URL:    "https://example.invalid/bin", // rewritten via client transport
					SHA256: digest,
					Size:   int64(len(payload)),
				}},
			}},
		}
		_ = json.NewEncoder(w).Encode(idx)
	})
	mux.HandleFunc("/bin", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	pluginDir := t.TempDir()
	client := New(srv.URL+"/index.json", pluginDir)
	client.httpClient = srv.Client()

	// Rewrite asset URL host to the test server.
	idx, err := client.Fetch(context.Background(), true)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
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
	fi, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Fatalf("expected executable bit, mode=%v", fi.Mode())
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
	_ = filepath.Join // keep import used on windows path helpers in coverage
}
