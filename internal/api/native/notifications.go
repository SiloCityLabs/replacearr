package native

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/replacearr/replacearr/pkg/pluginsdk"
)

// emitNotification fans out a lifecycle event to enabled notification plugins.
func (s *Server) emitNotification(ctx context.Context, eventType string, payload map[string]any) {
	if s.pluginMgr == nil {
		return
	}
	s.pluginMgr.NotifyAll(ctx, eventType, payload)
}

// EmitDownloadComplete notifies plugins that a download finished.
// Call from completed-download / import handling when that path lands.
func (s *Server) EmitDownloadComplete(ctx context.Context, payload map[string]any) {
	s.emitNotification(ctx, pluginsdk.EventDownloadComplete, payload)
}

// EmitDownloadMoved notifies plugins that a download was moved/imported into a library folder.
func (s *Server) EmitDownloadMoved(ctx context.Context, payload map[string]any) {
	s.emitNotification(ctx, pluginsdk.EventDownloadMoved, payload)
}

// EmitJellyfinPickedUp notifies plugins that Jellyfin ingested media.
// Prefer inbound webhooks (Manifest.InboundWebhooks → handle_webhook) for external callbacks;
// this helper remains for in-process emitters.
func (s *Server) EmitJellyfinPickedUp(ctx context.Context, payload map[string]any) {
	s.emitNotification(ctx, pluginsdk.EventJellyfinPickedUp, payload)
}

func (s *Server) handleTestNotificationPlugin(w http.ResponseWriter, r *http.Request) {
	if s.pluginMgr == nil {
		writeErr(w, http.StatusServiceUnavailable, fmt.Errorf("plugins unavailable"))
		return
	}
	id := r.PathValue("id")
	var req struct {
		Settings map[string]any `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	start := time.Now()
	err := s.pluginMgr.TestNotification(r.Context(), id, req.Settings)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":        false,
			"message":   err.Error(),
			"latencyMs": latency,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"message":   "test notification sent",
		"latencyMs": latency,
	})
}

func (s *Server) handleNotificationStream(w http.ResponseWriter, r *http.Request) {
	if s.pluginMgr == nil {
		http.Error(w, "plugins unavailable", http.StatusServiceUnavailable)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	events, cancel := s.pluginMgr.SubscribeBrowserNotifications()
	defer cancel()

	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			if _, err := fmt.Fprintf(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case ev, ok := <-events:
			if !ok {
				return
			}
			raw, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: notification\ndata: %s\n\n", raw); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
