package pluginhost

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/domain"
	"github.com/replacearr/replacearr/pkg/pluginsdk"
)

type stubNotifier struct {
	mu     sync.Mutex
	events []string
}

func (s *stubNotifier) Manifest() pluginsdk.Manifest {
	return pluginsdk.Manifest{
		ID:    "test-notifier",
		Name:  "Test Notifier",
		Types: []pluginsdk.PluginType{pluginsdk.PluginTypeNotification},
	}
}

func (s *stubNotifier) Configure(_ map[string]any) error { return nil }

func (s *stubNotifier) Notify(_ context.Context, eventType string, _ map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, eventType)
	return nil
}

func TestNotifyAllBuiltinAndStub(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	mgr := NewManager(store, filepath.Join(tmpDir, "plugins"), "")
	defer mgr.StopAll()

	stub := &stubNotifier{}
	mgr.registerBuiltin(stub)
	if _, err := mgr.ConfigurePlugin("test-notifier", true, map[string]any{}); err != nil {
		t.Fatalf("enable stub: %v", err)
	}
	if _, err := mgr.ConfigurePlugin(pluginsdk.BuiltinBrowserNotifications, true, map[string]any{
		"onDownloadStarted": true,
	}); err != nil {
		t.Fatalf("enable browser: %v", err)
	}

	ch, cancel := mgr.SubscribeBrowserNotifications()
	defer cancel()

	mgr.NotifyAll(context.Background(), pluginsdk.EventDownloadStarted, map[string]any{
		"title": "Severance",
	})

	stub.mu.Lock()
	if len(stub.events) != 1 || stub.events[0] != pluginsdk.EventDownloadStarted {
		t.Fatalf("stub events = %#v", stub.events)
	}
	stub.mu.Unlock()

	select {
	case ev := <-ch:
		if ev.EventType != pluginsdk.EventDownloadStarted {
			t.Fatalf("browser event type = %q", ev.EventType)
		}
		if ev.Payload["title"] != "Severance" {
			t.Fatalf("browser payload = %#v", ev.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for browser notification event")
	}
}

func TestTestNotificationBuiltin(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	mgr := NewManager(store, filepath.Join(tmpDir, "plugins"), "")
	defer mgr.StopAll()

	stub := &stubNotifier{}
	mgr.registerBuiltin(stub)
	// Leave stub disabled; TestNotification should still reach built-ins.
	if _, err := mgr.ConfigurePlugin("test-notifier", false, map[string]any{}); err != nil {
		t.Fatalf("configure stub: %v", err)
	}

	ch, cancel := mgr.SubscribeBrowserNotifications()
	defer cancel()

	if err := mgr.TestNotification(context.Background(), pluginsdk.BuiltinBrowserNotifications, map[string]any{
		"onDownloadStarted": false,
	}); err != nil {
		t.Fatalf("test browser: %v", err)
	}

	select {
	case ev := <-ch:
		if ev.EventType != pluginsdk.EventNotificationTest {
			t.Fatalf("browser event type = %q", ev.EventType)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for browser test event")
	}

	if err := mgr.TestNotification(context.Background(), "test-notifier", nil); err != nil {
		t.Fatalf("test stub: %v", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.events) != 1 || stub.events[0] != pluginsdk.EventNotificationTest {
		t.Fatalf("stub events = %#v", stub.events)
	}
}

func TestCheckStalledDownloadsEmitsOnce(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	mgr := NewManager(store, filepath.Join(tmpDir, "plugins"), "")
	defer mgr.StopAll()

	stub := &stubNotifier{}
	mgr.registerBuiltin(stub)
	if _, err := mgr.ConfigurePlugin("test-notifier", true, map[string]any{}); err != nil {
		t.Fatalf("enable stub: %v", err)
	}
	if _, err := mgr.ConfigurePlugin(pluginsdk.BuiltinBrowserNotifications, true, map[string]any{
		"stalledDays":       float64(2),
		"onDownloadStalled": true,
	}); err != nil {
		t.Fatalf("enable browser: %v", err)
	}

	old := time.Now().UTC().Add(-5 * 24 * time.Hour)
	items := []domain.QueueItem{{
		ID:                   42,
		Title:                "Old.Download",
		Status:               "downloading",
		TrackedDownloadState: "downloading",
		Added:                old,
	}}

	mgr.CheckStalledDownloads(context.Background(), items)
	mgr.CheckStalledDownloads(context.Background(), items)

	stub.mu.Lock()
	defer stub.mu.Unlock()
	count := 0
	for _, e := range stub.events {
		if e == pluginsdk.EventDownloadStalled {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 stalled event, got %d (%#v)", count, stub.events)
	}
}
