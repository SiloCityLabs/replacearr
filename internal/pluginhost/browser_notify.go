package pluginhost

import (
	"context"
	"sync"
	"time"

	"github.com/replacearr/replacearr/pkg/pluginsdk"
)

// BrowserNotificationEvent is pushed to SPA subscribers over SSE.
type BrowserNotificationEvent struct {
	EventType string         `json:"eventType"`
	Payload   map[string]any `json:"payload"`
	SentAt    time.Time      `json:"sentAt"`
}

type notificationHub struct {
	mu   sync.Mutex
	subs map[chan BrowserNotificationEvent]struct{}
}

func newNotificationHub() *notificationHub {
	return &notificationHub{subs: make(map[chan BrowserNotificationEvent]struct{})}
}

func (h *notificationHub) Subscribe() (<-chan BrowserNotificationEvent, func()) {
	ch := make(chan BrowserNotificationEvent, 16)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	cancel := func() {
		h.mu.Lock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
	return ch, cancel
}

func (h *notificationHub) Publish(ev BrowserNotificationEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- ev:
		default:
			// Drop if subscriber is slow; avoid blocking NotifyAll.
		}
	}
}

type builtinBrowserNotifications struct {
	mu       sync.RWMutex
	hub      *notificationHub
	settings map[string]any
}

func newBuiltinBrowserNotifications(hub *notificationHub) *builtinBrowserNotifications {
	return &builtinBrowserNotifications{
		hub:      hub,
		settings: map[string]any{},
	}
}

func (b *builtinBrowserNotifications) Manifest() pluginsdk.Manifest {
	return pluginsdk.Manifest{
		ID:                pluginsdk.BuiltinBrowserNotifications,
		Name:              "Browser Notifications",
		Version:           "1.0.0",
		Author:            "Replacearr Core",
		Description:       "Pushes lifecycle events to the web UI so the browser Notification API can display them. Configure which events to show here; enable Telegram/Signal/webhook plugins separately for other channels.",
		Types:             []pluginsdk.PluginType{pluginsdk.PluginTypeNotification},
		DisabledByDefault: true,
		ConfigSchema: []pluginsdk.ConfigFieldSchema{
			{
				Name:     "onDownloadStarted",
				Label:    "Download started",
				Type:     "checkbox",
				Default:  true,
				Required: false,
			},
			{
				Name:     "onDownloadComplete",
				Label:    "Download complete",
				Type:     "checkbox",
				Default:  true,
				Required: false,
			},
			{
				Name:     "onDownloadStalled",
				Label:    "Downloading more than X days",
				Type:     "checkbox",
				Default:  true,
				Required: false,
			},
			{
				Name:     "stalledDays",
				Label:    "Stalled after (days)",
				Type:     "number",
				Default:  3,
				Required: false,
				HelpText: "Emit a stalled event when a queue item is older than this many days.",
			},
			{
				Name:     "onDownloadMoved",
				Label:    "Download moved",
				Type:     "checkbox",
				Default:  true,
				Required: false,
			},
			{
				Name:     "onJellyfinPickedUp",
				Label:    "Jellyfin picked up",
				Type:     "checkbox",
				Default:  true,
				Required: false,
			},
		},
	}
}

func (b *builtinBrowserNotifications) Configure(settings map[string]any) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.settings = settings
	return nil
}

func (b *builtinBrowserNotifications) eventEnabled(eventType string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if eventType == pluginsdk.EventNotificationTest {
		return true
	}
	key := ""
	switch eventType {
	case pluginsdk.EventDownloadStarted:
		key = "onDownloadStarted"
	case pluginsdk.EventDownloadComplete:
		key = "onDownloadComplete"
	case pluginsdk.EventDownloadStalled:
		key = "onDownloadStalled"
	case pluginsdk.EventDownloadMoved:
		key = "onDownloadMoved"
	case pluginsdk.EventJellyfinPickedUp:
		key = "onJellyfinPickedUp"
	default:
		return false
	}
	v, ok := b.settings[key]
	if !ok {
		return true
	}
	enabled, ok := v.(bool)
	return ok && enabled
}

func (b *builtinBrowserNotifications) Notify(_ context.Context, eventType string, payload map[string]any) error {
	if !b.eventEnabled(eventType) {
		return nil
	}
	if payload == nil {
		payload = map[string]any{}
	}
	b.hub.Publish(BrowserNotificationEvent{
		EventType: eventType,
		Payload:   payload,
		SentAt:    time.Now().UTC(),
	})
	return nil
}
