package native

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/replacearr/replacearr/internal/auth"
	"github.com/replacearr/replacearr/internal/config"
	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/pluginhost"
	"github.com/replacearr/replacearr/pkg/pluginsdk"
)

type jellyfinStubNotifier struct {
	mu       sync.Mutex
	events   []string
	payloads []map[string]any
}

func (s *jellyfinStubNotifier) Manifest() pluginsdk.Manifest {
	return pluginsdk.Manifest{
		ID:    "test-channel",
		Name:  "Test Channel",
		Types: []pluginsdk.PluginType{pluginsdk.PluginTypeNotification},
	}
}

func (s *jellyfinStubNotifier) Configure(_ map[string]any) error { return nil }

func (s *jellyfinStubNotifier) Notify(_ context.Context, eventType string, payload map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, eventType)
	s.payloads = append(s.payloads, payload)
	return nil
}

type jellyfinSettingsStub struct {
	settings map[string]any
}

func (s *jellyfinSettingsStub) Manifest() pluginsdk.Manifest {
	return pluginsdk.Manifest{
		ID:    pluginsdk.JellyfinNotifierPluginID,
		Name:  "Jellyfin Notifier",
		Types: []pluginsdk.PluginType{pluginsdk.PluginTypeNotification},
		ConfigSchema: []pluginsdk.ConfigFieldSchema{
			{Name: "inbound_webhook_secret", Type: "password"},
		},
	}
}

func (s *jellyfinSettingsStub) Configure(settings map[string]any) error {
	s.settings = settings
	return nil
}

func (s *jellyfinSettingsStub) Notify(_ context.Context, _ string, _ map[string]any) error {
	return nil
}

func newJellyfinWebhookStack(t *testing.T) (*Server, *pluginhost.Manager, *http.ServeMux) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("REPLACEARR_CONFIG_DIR", dir)
	t.Setenv("REPLACEARR_DB_PATH", filepath.Join(dir, "replacearr.db"))
	t.Setenv("REPLACEARR_PLUGIN_DIR", filepath.Join(dir, "plugins"))
	t.Setenv("REPLACEARR_DEFINITIONS_DIR", filepath.Join(dir, "definitions"))
	t.Setenv("REPLACEARR_ADMIN_USER", "admin")
	t.Setenv("REPLACEARR_ADMIN_PASS", "replacearr")
	t.Setenv("REPLACEARR_JWT_SECRET", "test-jwt-secret-at-least-32-bytes-long")
	t.Setenv("REPLACEARR_FLARESOLVERR_URL", "http://127.0.0.1:1/")

	cfg := config.Load()
	store, err := db.Open(cfg.DatabasePath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	authSvc, err := auth.NewService(store, cfg)
	if err != nil {
		t.Fatalf("auth: %v", err)
	}
	mgr := pluginhost.NewManager(store, cfg.PluginDir, cfg.DefinitionsDir)
	t.Cleanup(func() { mgr.StopAll() })

	srv := NewServer(store, cfg, authSvc, mgr, "test")
	mux := http.NewServeMux()
	authSvc.RegisterRoutes(mux)
	srv.RegisterRoutes(mux)
	return srv, mgr, mux
}

func TestJellyfinWebhookUnauthorized(t *testing.T) {
	_, mgr, mux := newJellyfinWebhookStack(t)
	stub := &jellyfinSettingsStub{}
	mgr.RegisterBuiltinForTest(stub)
	if _, err := mgr.ConfigurePlugin(pluginsdk.JellyfinNotifierPluginID, true, map[string]any{
		"inbound_webhook_secret": "correct-secret",
	}); err != nil {
		t.Fatalf("configure: %v", err)
	}

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/jellyfin", bytes.NewReader([]byte(`{"Name":"x"}`))))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("no secret: got %d", rr.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/jellyfin", bytes.NewReader([]byte(`{"Name":"x"}`)))
	req.Header.Set("X-Replacearr-Webhook-Secret", "wrong")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("wrong secret: got %d", rr.Code)
	}
}

func TestJellyfinWebhookDisabledPlugin(t *testing.T) {
	_, mgr, mux := newJellyfinWebhookStack(t)
	stub := &jellyfinSettingsStub{}
	mgr.RegisterBuiltinForTest(stub)
	if _, err := mgr.ConfigurePlugin(pluginsdk.JellyfinNotifierPluginID, false, map[string]any{
		"inbound_webhook_secret": "secret",
	}); err != nil {
		t.Fatalf("configure: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/jellyfin", bytes.NewReader([]byte(`{"NotificationType":"ItemAdded","Name":"Film"}`)))
	req.Header.Set("X-Replacearr-Webhook-Secret", "secret")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("disabled plugin: got %d", rr.Code)
	}
}

func TestJellyfinWebhookItemAddedEmitsPickedUp(t *testing.T) {
	_, mgr, mux := newJellyfinWebhookStack(t)
	jf := &jellyfinSettingsStub{}
	mgr.RegisterBuiltinForTest(jf)
	if _, err := mgr.ConfigurePlugin(pluginsdk.JellyfinNotifierPluginID, true, map[string]any{
		"inbound_webhook_secret": "shared",
	}); err != nil {
		t.Fatalf("configure jellyfin: %v", err)
	}

	channel := &jellyfinStubNotifier{}
	mgr.RegisterBuiltinForTest(channel)
	if _, err := mgr.ConfigurePlugin("test-channel", true, map[string]any{}); err != nil {
		t.Fatalf("configure channel: %v", err)
	}

	body := []byte(`{
		"NotificationType":"ItemAdded",
		"Name":"Severance",
		"ItemType":"Episode",
		"ItemId":"abc123",
		"Path":"/data/TV/Severance/S01E01.mkv"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/jellyfin?secret=shared", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		channel.mu.Lock()
		n := len(channel.events)
		channel.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	channel.mu.Lock()
	defer channel.mu.Unlock()
	if len(channel.events) != 1 || channel.events[0] != pluginsdk.EventJellyfinPickedUp {
		t.Fatalf("events = %#v", channel.events)
	}
	p := channel.payloads[0]
	if p["title"] != "Severance" || p["itemType"] != "Episode" || p["itemId"] != "abc123" {
		t.Fatalf("payload = %#v", p)
	}
	if p["path"] != "/data/TV/Severance/S01E01.mkv" {
		t.Fatalf("path = %#v", p["path"])
	}
}

func TestJellyfinWebhookIgnoresPlayback(t *testing.T) {
	_, mgr, mux := newJellyfinWebhookStack(t)
	jf := &jellyfinSettingsStub{}
	mgr.RegisterBuiltinForTest(jf)
	if _, err := mgr.ConfigurePlugin(pluginsdk.JellyfinNotifierPluginID, true, map[string]any{
		"inbound_webhook_secret": "shared",
	}); err != nil {
		t.Fatalf("configure: %v", err)
	}
	channel := &jellyfinStubNotifier{}
	mgr.RegisterBuiltinForTest(channel)
	if _, err := mgr.ConfigurePlugin("test-channel", true, map[string]any{}); err != nil {
		t.Fatalf("configure channel: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/jellyfin", bytes.NewReader([]byte(`{"NotificationType":"PlaybackStart","Name":"x"}`)))
	req.Header.Set("X-Replacearr-Webhook-Secret", "shared")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status %d", rr.Code)
	}
	time.Sleep(50 * time.Millisecond)
	channel.mu.Lock()
	defer channel.mu.Unlock()
	if len(channel.events) != 0 {
		t.Fatalf("expected no events, got %#v", channel.events)
	}
}

func TestNormalizeAndFilterHelpers(t *testing.T) {
	if !jellyfinWebhookIsItemAdded(map[string]any{}) {
		t.Fatal("empty should accept")
	}
	if jellyfinWebhookIsItemAdded(map[string]any{"NotificationType": "PlaybackStop"}) {
		t.Fatal("playback should reject")
	}
	got := normalizeJellyfinWebhookPayload(map[string]any{
		"ItemName": "Movie",
		"item_type": "Movie",
		"Id":        "1",
		"ItemPath":  "/m.mkv",
	})
	if got["title"] != "Movie" || got["itemType"] != "Movie" || got["itemId"] != "1" || got["path"] != "/m.mkv" {
		t.Fatalf("got %#v", got)
	}
}
