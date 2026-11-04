package native

import (
	"io"
	"net/http"
	"strings"
)

const webhookSecretHeader = "X-Replacearr-Webhook-Secret"

// handleInboundWebhook serves POST /api/v1/webhooks/{pluginId} and
// POST /api/v1/webhooks/{pluginId}/{slug}. pluginId may also be a PathAlias
// declared on a plugin manifest (e.g. "jellyfin").
func (s *Server) handleInboundWebhook(w http.ResponseWriter, r *http.Request) {
	if s.pluginMgr == nil {
		http.Error(w, "plugins unavailable", http.StatusServiceUnavailable)
		return
	}

	pluginID := strings.TrimSpace(r.PathValue("pluginId"))
	slug := strings.TrimSpace(r.PathValue("slug"))
	if pluginID == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	provided := strings.TrimSpace(r.Header.Get(webhookSecretHeader))
	if provided == "" {
		provided = strings.TrimSpace(r.URL.Query().Get("secret"))
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	headers := map[string]string{}
	for k, vals := range r.Header {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}
	query := map[string]string{}
	for k, vals := range r.URL.Query() {
		if len(vals) > 0 {
			query[k] = vals[0]
		}
	}

	status, err := s.pluginMgr.HandleInboundWebhook(r.Context(), pluginID, slug, provided, headers, query, body)
	if err != nil {
		switch status {
		case http.StatusUnauthorized:
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		case http.StatusNotImplemented:
			http.Error(w, err.Error(), http.StatusNotImplemented)
		case http.StatusBadGateway:
			http.Error(w, err.Error(), http.StatusBadGateway)
		default:
			if status >= 400 {
				http.Error(w, err.Error(), status)
			} else {
				http.Error(w, err.Error(), http.StatusBadGateway)
			}
		}
		return
	}
	if status == 0 {
		status = http.StatusNoContent
	}
	w.WriteHeader(status)
}
