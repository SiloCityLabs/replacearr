package native

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/replacearr/replacearr/pkg/pluginsdk"
)

const jellyfinInboundSecretKey = "inbound_webhook_secret"

// handleJellyfinWebhook accepts Jellyfin Webhook plugin callbacks (typically ItemAdded)
// and fans out jellyfin.picked_up to notification channels. Auth is the shared secret
// configured on the jellyfin-notifier plugin (not JWT).
func (s *Server) handleJellyfinWebhook(w http.ResponseWriter, r *http.Request) {
	if s.pluginMgr == nil {
		http.Error(w, "plugins unavailable", http.StatusServiceUnavailable)
		return
	}

	provided := strings.TrimSpace(r.Header.Get("X-Replacearr-Webhook-Secret"))
	if provided == "" {
		provided = strings.TrimSpace(r.URL.Query().Get("secret"))
	}

	enabled, settings, ok := s.pluginMgr.PluginSettings(pluginsdk.JellyfinNotifierPluginID)
	if !ok || !enabled {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	expected := ""
	if settings != nil {
		if v, ok := settings[jellyfinInboundSecretKey].(string); ok {
			expected = strings.TrimSpace(v)
		}
	}
	if expected == "" || !secureSecretEqual(provided, expected) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	raw := map[string]any{}
	if len(bytesTrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &raw); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
	}

	if !jellyfinWebhookIsItemAdded(raw) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	payload := normalizeJellyfinWebhookPayload(raw)
	s.EmitJellyfinPickedUp(r.Context(), payload)
	w.WriteHeader(http.StatusNoContent)
}

func secureSecretEqual(provided, expected string) bool {
	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1 {
		return true
	}
	return false
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func jellyfinWebhookIsItemAdded(raw map[string]any) bool {
	notifType := firstString(raw,
		"NotificationType", "notificationType", "notification_type",
		"Event", "event", "Type", "type",
	)
	if notifType == "" {
		// Untyped payloads are treated as ItemAdded (custom templates).
		return true
	}
	n := strings.ToLower(strings.TrimSpace(notifType))
	switch n {
	case "itemadded", "item.added", "library.new", "library_new", "item added":
		return true
	}
	if strings.Contains(n, "playback") || strings.Contains(n, "session") ||
		strings.Contains(n, "userdatal") || strings.Contains(n, "authentication") {
		return false
	}
	// Unknown typed events: ignore to avoid noise.
	return false
}

func normalizeJellyfinWebhookPayload(raw map[string]any) map[string]any {
	out := map[string]any{}
	if title := firstString(raw, "Name", "ItemName", "title", "SeriesName"); title != "" {
		out["title"] = title
	}
	if itemType := firstString(raw, "ItemType", "item_type", "itemType"); itemType != "" {
		out["itemType"] = itemType
	}
	if itemID := firstString(raw, "ItemId", "Id", "itemId", "item_id"); itemID != "" {
		out["itemId"] = itemID
	}
	if path := firstString(raw, "Path", "ItemPath", "path", "item_path"); path != "" {
		out["path"] = path
	}
	return out
}

func firstString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := raw[key]; ok {
			switch t := v.(type) {
			case string:
				if s := strings.TrimSpace(t); s != "" {
					return s
				}
			}
		}
	}
	return ""
}
