package native

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// jellyfinSettingsStub mimics the marketplace jellyfin-notifier inbound webhook contract.
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
		InboundWebhooks: []pluginsdk.InboundWebhookSpec{{
			SecretSetting: "inbound_webhook_secret",
			PathAliases:   []string{"jellyfin"},
			Description:   "Jellyfin Webhook ItemAdded callback",
		}},
	}
}

func (s *jellyfinSettingsStub) Configure(settings map[string]any) error {
	s.settings = settings
	return nil
}

func (s *jellyfinSettingsStub) Notify(_ context.Context, _ string, _ map[string]any) error {
	return nil
}

func (s *jellyfinSettingsStub) HandleWebhook(_ context.Context, req pluginsdk.WebhookRequest) (*pluginsdk.WebhookResult, error) {
	raw := map[string]any{}
	if len(bytes.TrimSpace(req.Body)) > 0 {
		if err := json.Unmarshal(req.Body, &raw); err != nil {
			return &pluginsdk.WebhookResult{Status: http.StatusBadRequest}, err
		}
	}
	if !testJellyfinIsItemAdded(raw) {
		return &pluginsdk.WebhookResult{Status: http.StatusNoContent}, nil
	}
	return &pluginsdk.WebhookResult{
		Status: http.StatusNoContent,
		Events: []pluginsdk.WebhookEvent{{
			EventType: pluginsdk.EventJellyfinPickedUp,
			Payload:   testNormalizeJellyfinPayload(raw),
		}},
	}, nil
}

func testJellyfinIsItemAdded(raw map[string]any) bool {
	notifType := ""
	for _, key := range []string{"NotificationType", "notificationType", "Event", "event", "Type", "type"} {
		if v, ok := raw[key].(string); ok && strings.TrimSpace(v) != "" {
			notifType = strings.ToLower(strings.TrimSpace(v))
			break
		}
	}
	if notifType == "" {
		return true
	}
	switch notifType {
	case "itemadded", "item.added", "library.new", "library_new", "item added":
		return true
	}
	if strings.Contains(notifType, "playback") || strings.Contains(notifType, "session") {
		return false
	}
	return false
}

func testNormalizeJellyfinPayload(raw map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"Name", "ItemName", "title", "SeriesName"} {
		if v, ok := raw[key].(string); ok && strings.TrimSpace(v) != "" {
			out["title"] = strings.TrimSpace(v)
			break
		}
	}
	for _, key := range []string{"ItemType", "itemType"} {
		if v, ok := raw[key].(string); ok && strings.TrimSpace(v) != "" {
			out["itemType"] = strings.TrimSpace(v)
			break
		}
	}
	for _, key := range []string{"ItemId", "Id", "itemId"} {
		if v, ok := raw[key].(string); ok && strings.TrimSpace(v) != "" {
			out["itemId"] = strings.TrimSpace(v)
			break
		}
	}
	for _, key := range []string{"Path", "ItemPath", "path"} {
		if v, ok := raw[key].(string); ok && strings.TrimSpace(v) != "" {
			out["path"] = strings.TrimSpace(v)
			break
		}
	}
	return out
}

func newJellyfinWebhookStack(t *testing.T) (*Server, *pluginhost.Manager, *http.ServeMux) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("REPLACEARR_CONFIG_DIR", dir)
	t.Setenv("REPLACEARR_ADMIN_USER", "admin")
	t.Setenv("REPLACEARR_ADMIN_PASS", "replacearr")

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
	if len(channel.events) != 1 || channel.events[0] != pluginsdk.EventJellyfinPickedUp {
		ev := append([]string(nil), channel.events...)
		channel.mu.Unlock()
		t.Fatalf("events = %#v", ev)
	}
	p := channel.payloads[0]
	if p["title"] != "Severance" || p["itemType"] != "Episode" || p["itemId"] != "abc123" {
		channel.mu.Unlock()
		t.Fatalf("payload = %#v", p)
	}
	if p["path"] != "/data/TV/Severance/S01E01.mkv" {
		channel.mu.Unlock()
		t.Fatalf("path = %#v", p["path"])
	}
	channel.mu.Unlock()

	// Also accept the canonical plugin-id path (must not hold channel.mu — Notify locks it).
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/jellyfin-notifier?secret=shared", bytes.NewReader(body))
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("plugin-id path status %d", rr.Code)
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
