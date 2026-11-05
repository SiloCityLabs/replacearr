package discover

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestFingerprintQBittorrent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/app/version" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("v4.6.5"))
	}))
	defer srv.Close()

	client := srv.Client()
	if !fingerprintQBittorrent(context.Background(), client, srv.URL) {
		t.Fatal("expected qBittorrent hit")
	}
}

func TestFingerprintQBittorrentForbiddenNeedsWebUI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/app/version" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`<!doctype html><title>qBittorrent WebUI</title>`))
	}))
	defer srv.Close()
	if !fingerprintQBittorrent(context.Background(), srv.Client(), srv.URL) {
		t.Fatal("expected 403 + WebUI HTML to count as qBittorrent")
	}
}

func TestFingerprintQBittorrentIgnoresBasicAuthChallenge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="NZBGet"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	if fingerprintQBittorrent(context.Background(), srv.Client(), srv.URL) {
		t.Fatal("NZBGet-style basic auth must not match qBittorrent")
	}
}

func TestFingerprintNZBGetBasicAuthNotQBittorrent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="NZBGet"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	client := srv.Client()
	client.Timeout = 2 * time.Second
	if !fingerprintNZBGet(context.Background(), client, srv.URL) {
		t.Fatal("expected NZBGet")
	}
	if fingerprintQBittorrent(context.Background(), client, srv.URL) {
		t.Fatal("qBittorrent must not match NZBGet basic auth")
	}
}

func TestProbesForPortOrdersNZBGetFirstOn6789(t *testing.T) {
	probes := probesForPort(6789)
	if len(probes) == 0 || probes[0].implementation != "NZBGet" {
		t.Fatalf("expected NZBGet first on 6789, got %#v", probes)
	}
	probes = probesForPort(9091)
	if len(probes) == 0 || probes[0].implementation != "Transmission" {
		t.Fatalf("expected Transmission first on 9091, got %#v", probes)
	}
}

func TestFingerprintTransmission(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transmission/rpc" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Transmission-Session-Id", "abc")
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()
	if !fingerprintTransmission(context.Background(), srv.Client(), srv.URL) {
		t.Fatal("expected Transmission hit")
	}
}

func TestFingerprintDeluge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "result": true, "error": nil})
	}))
	defer srv.Close()
	if !fingerprintDeluge(context.Background(), srv.Client(), srv.URL) {
		t.Fatal("expected Deluge hit")
	}
}

func TestFingerprintSABnzbd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "4.3.0"})
	}))
	defer srv.Close()
	if !fingerprintSABnzbd(context.Background(), srv.Client(), srv.URL) {
		t.Fatal("expected SABnzbd hit")
	}
}

func TestFingerprintSABnzbdNeedsKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "API Key Required"})
	}))
	defer srv.Close()
	if !fingerprintSABnzbd(context.Background(), srv.Client(), srv.URL) {
		t.Fatal("expected SABnzbd error JSON to count")
	}
}

func TestFingerprintNZBGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jsonrpc" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": "21.1"})
	}))
	defer srv.Close()
	if !fingerprintNZBGet(context.Background(), srv.Client(), srv.URL) {
		t.Fatal("expected NZBGet hit")
	}
}

func TestFingerprintOpenPortIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/transmission/rpc" {
			w.Header().Set("X-Transmission-Session-Id", "x")
			w.WriteHeader(http.StatusConflict)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	host, portStr, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}
	res, ok := fingerprintOpenPort(context.Background(), openPort{Host: host, Port: port, Via: "local"})
	if !ok {
		t.Fatal("expected hit")
	}
	if res.Implementation != "Transmission" {
		t.Fatalf("got %s", res.Implementation)
	}
}
