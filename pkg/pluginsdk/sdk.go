// Package pluginsdk provides the zero-CGO developer SDK for building Replacearr
// plugins as standalone Go binaries communicating over stdin/stdout JSON-RPC.
//
// A complete plugin can be written in a single main.go file:
//
//	func main() {
//	    pluginsdk.Serve(&MyIndexerPlugin{})
//	}
package pluginsdk

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// PluginType identifies the capability domain a plugin provides.
type PluginType string

const (
	PluginTypeIndexer        PluginType = "indexer"
	PluginTypeDownloadClient PluginType = "download_client"
	PluginTypeMetadata       PluginType = "metadata"
	PluginTypeNotification   PluginType = "notification"
	PluginTypeMediaModule    PluginType = "media_module"
	PluginTypeNetworkProxy   PluginType = "network_proxy"
	PluginTypeCompat         PluginType = "compat"
)

// Built-in compatibility shim plugin IDs (in-process; not subprocess binaries).
const (
	CompatPluginSonarr   = "builtin-compat-sonarr"
	CompatPluginRadarr   = "builtin-compat-radarr"
	CompatPluginProwlarr = "builtin-compat-prowlarr"
	CompatPluginLidarr   = "builtin-compat-lidarr"
	CompatPluginReadarr  = "builtin-compat-readarr"
)

// Built-in notification channel that bridges Notify events to the SPA via SSE.
const BuiltinBrowserNotifications = "builtin-browser-notifications"

// JellyfinNotifierPluginID is the marketplace notification plugin id for Jellyfin.
// Prefer Manifest.InboundWebhooks path aliases over hard-coding this in new host code.
const JellyfinNotifierPluginID = "jellyfin-notifier"

// Stable notification event types delivered to NotificationPlugin.Notify.
const (
	EventDownloadStarted  = "download.started"
	EventDownloadComplete = "download.complete"
	EventDownloadStalled  = "download.stalled"
	EventDownloadMoved    = "download.moved"
	EventJellyfinPickedUp = "jellyfin.picked_up"
	// EventNotificationTest is sent by the Plugins UI "Test" action. Channels should
	// always deliver it (ignore per-event enable toggles).
	EventNotificationTest = "notification.test"
)

// InboundWebhookSpec declares an unauthenticated (secret-based) HTTP callback the host
// should expose for this plugin. Secrets are read from ConfigSchema via SecretSetting.
type InboundWebhookSpec struct {
	// Slug is an optional path segment under /api/v1/webhooks/{pluginId}/{slug}.
	// Empty slug means /api/v1/webhooks/{pluginId}.
	Slug string `json:"slug,omitempty"`
	// PathAliases are additional short paths such as "jellyfin" → /api/v1/webhooks/jellyfin.
	PathAliases []string `json:"pathAliases,omitempty"`
	// SecretSetting is the ConfigSchema field name that holds the shared secret.
	SecretSetting string `json:"secretSetting"`
	// Description is optional UI/docs help text for the callback URL.
	Description string `json:"description,omitempty"`
}

// WebhookRequest is the host → plugin payload for an inbound HTTP webhook.
type WebhookRequest struct {
	Slug    string            `json:"slug"`
	Headers map[string]string `json:"headers,omitempty"`
	Query   map[string]string `json:"query,omitempty"`
	Body    json.RawMessage   `json:"body"`
}

// WebhookEvent is an event the plugin wants the host to fan out via NotifyAll.
type WebhookEvent struct {
	EventType string         `json:"eventType"`
	Payload   map[string]any `json:"payload"`
}

// WebhookResult is the plugin → host response for handle_webhook.
type WebhookResult struct {
	// Status is the HTTP status to return to the caller (default 204).
	Status int `json:"status,omitempty"`
	// Events are fanned out to enabled NotificationPlugin channels.
	Events []WebhookEvent `json:"events,omitempty"`
}

// ConfigFieldSchema describes a user-configurable setting exposed in the Replacearr UI.
type ConfigFieldSchema struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Type     string `json:"type"` // text, password, number, checkbox, select
	Default  any    `json:"default,omitempty"`
	Required bool   `json:"required"`
	HelpText string `json:"helpText,omitempty"`
}

// Manifest declares plugin metadata, capabilities, and configuration schema.
type Manifest struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Version      string              `json:"version"`
	Author       string              `json:"author"`
	Description  string              `json:"description"`
	Types        []PluginType        `json:"types"`
	ConfigSchema []ConfigFieldSchema `json:"configSchema"`
	// InboundWebhooks registers secret-authenticated HTTP callbacks on the host.
	// Plugins that declare entries must implement InboundWebhookPlugin.
	InboundWebhooks []InboundWebhookSpec `json:"inboundWebhooks,omitempty"`
	// DisabledByDefault leaves the plugin off on first discovery until the user enables it.
	// Ignored once a plugin_configs row exists.
	DisabledByDefault bool `json:"disabledByDefault,omitempty"`
}

// SearchRequest represents a Newznab/Torznab-compatible query sent to an IndexerPlugin.
type SearchRequest struct {
	Query           string   `json:"query"`
	Type            string   `json:"type"` // search, tvsearch, movie
	Categories      []int    `json:"categories,omitempty"`
	TvdbID          int      `json:"tvdbId,omitempty"`
	TmdbID          int      `json:"tmdbId,omitempty"`
	ImdbID          string   `json:"imdbId,omitempty"`
	Season          int      `json:"season,omitempty"`
	Episode         int      `json:"episode,omitempty"`
	Aliases         []string `json:"aliases,omitempty"`
	Limit           int      `json:"limit,omitempty"`
	FlareSolverrURL string   `json:"flareSolverrUrl,omitempty"`
	ProxyURL        string   `json:"proxyUrl,omitempty"`
}

// Release represents a single indexer search result returned by an IndexerPlugin.
type Release struct {
	Guid        string `json:"guid"`
	Title       string `json:"title"`
	Protocol    string `json:"protocol"` // torrent, usenet
	Size        int64  `json:"size"`
	Seeders     int    `json:"seeders"`
	Peers       int    `json:"peers"`
	PublishDate string `json:"publishDate"`
	DownloadURL string `json:"downloadUrl"`
	InfoURL     string `json:"infoUrl"`
	Categories  []int  `json:"categories"`
	Quality     string `json:"quality"`
}

// Plugin is the base interface that every Replacearr plugin must implement.
type Plugin interface {
	Manifest() Manifest
	Configure(settings map[string]any) error
}

// IndexerPlugin is an optional capability interface for plugins that search releases.
type IndexerPlugin interface {
	Plugin
	Search(ctx context.Context, req SearchRequest) ([]Release, error)
}

// DownloadItem is a download currently tracked by a download client plugin.
type DownloadItem struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Status      string  `json:"status"` // downloading, completed, paused, error
	Progress    float64 `json:"progress"`
	Size        int64   `json:"size"`
	SizeLeft    int64   `json:"sizeLeft"`
	OutputPath  string  `json:"outputPath"`
	ContentPath string  `json:"contentPath"`
}

// DownloadClientPlugin is an optional capability interface for plugins that enqueue downloads.
type DownloadClientPlugin interface {
	Plugin
	EnqueueDownload(ctx context.Context, title, downloadURL, protocol string) (string, error)
}

// DownloadLister is an optional capability for download clients that can report active items.
type DownloadLister interface {
	ListDownloads(ctx context.Context) ([]DownloadItem, error)
}

// MetadataPlugin is an optional capability interface for plugins that look up TV/movie metadata.
type MetadataPlugin interface {
	Plugin
	LookupMedia(ctx context.Context, mediaType, query string) ([]map[string]any, error)
}

// NotificationPlugin is an optional capability interface for plugins that send notifications.
type NotificationPlugin interface {
	Plugin
	Notify(ctx context.Context, eventType string, payload map[string]any) error
}

// NetworkProxyPlugin is an optional capability for plugins that expose an outbound HTTP/SOCKS proxy URL.
type NetworkProxyPlugin interface {
	Plugin
	ProxyURL() string
}

// InboundWebhookPlugin handles secret-authenticated HTTP callbacks registered via Manifest.InboundWebhooks.
type InboundWebhookPlugin interface {
	Plugin
	HandleWebhook(ctx context.Context, req WebhookRequest) (*WebhookResult, error)
}

// RPCRequest is a newline-delimited JSON-RPC request frame sent over stdin.
type RPCRequest struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// RPCResponse is a newline-delimited JSON-RPC response frame written to stdout.
type RPCResponse struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

type enqueueDownloadParams struct {
	Title       string `json:"title"`
	DownloadURL string `json:"downloadUrl"`
	Protocol    string `json:"protocol"`
}

type lookupMediaParams struct {
	MediaType string `json:"mediaType"`
	Query     string `json:"query"`
}

type notifyParams struct {
	EventType string         `json:"eventType"`
	Payload   map[string]any `json:"payload"`
}

// Serve runs the plugin JSON-RPC loop over os.Stdin and os.Stdout.
// Standard logger output is redirected to os.Stderr so log lines never corrupt stdout RPC frames.
// Passing "--manifest" on the command line prints the plugin manifest as JSON and exits immediately.
func Serve(p Plugin) {
	log.SetOutput(os.Stderr)

	for _, arg := range os.Args[1:] {
		if arg == "--manifest" || arg == "-manifest" {
			b, err := json.MarshalIndent(p.Manifest(), "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "marshal manifest: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprintln(os.Stdout, string(b))
			os.Exit(0)
		}
	}

	if err := ServeIO(context.Background(), p, os.Stdin, os.Stdout); err != nil && !errors.Is(err, io.EOF) {
		log.Printf("plugin rpc loop exited with error: %v", err)
		os.Exit(1)
	}
}

// ServeIO runs the newline-delimited JSON-RPC server over the provided reader and writer.
func ServeIO(ctx context.Context, p Plugin, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	enc := json.NewEncoder(out)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req RPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			_ = enc.Encode(RPCResponse{
				ID:    0,
				Error: fmt.Sprintf("invalid json-rpc request: %v", err),
			})
			continue
		}

		res, rpcErr := dispatch(ctx, p, req)
		resp := RPCResponse{ID: req.ID}
		if rpcErr != nil {
			resp.Error = rpcErr.Error()
		} else if res != nil {
			raw, err := json.Marshal(res)
			if err != nil {
				resp.Error = fmt.Sprintf("marshal result: %v", err)
			} else {
				resp.Result = raw
			}
		}

		if err := enc.Encode(resp); err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
	}

	return scanner.Err()
}

func dispatch(ctx context.Context, p Plugin, req RPCRequest) (any, error) {
	switch req.Method {
	case "manifest":
		return p.Manifest(), nil

	case "configure":
		settings := map[string]any{}
		if len(req.Params) > 0 && string(req.Params) != "null" {
			if err := json.Unmarshal(req.Params, &settings); err != nil {
				return nil, fmt.Errorf("invalid configure params: %w", err)
			}
			// Unwrap {"settings": {...}} if passed in a wrapper object
			if nested, ok := settings["settings"].(map[string]any); ok && len(settings) == 1 {
				settings = nested
			}
		}
		if err := p.Configure(settings); err != nil {
			return nil, err
		}
		return map[string]any{"status": "ok"}, nil

	case "search":
		indexer, ok := p.(IndexerPlugin)
		if !ok {
			return nil, errors.New("plugin does not implement IndexerPlugin")
		}
		var sreq SearchRequest
		if len(req.Params) > 0 && string(req.Params) != "null" {
			if err := json.Unmarshal(req.Params, &sreq); err != nil {
				return nil, fmt.Errorf("invalid search params: %w", err)
			}
		}
		releases, err := indexer.Search(ctx, sreq)
		if err != nil {
			return nil, err
		}
		if releases == nil {
			releases = []Release{}
		}
		return releases, nil

	case "enqueue_download":
		dl, ok := p.(DownloadClientPlugin)
		if !ok {
			return nil, errors.New("plugin does not implement DownloadClientPlugin")
		}
		var params enqueueDownloadParams
		if len(req.Params) > 0 && string(req.Params) != "null" {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return nil, fmt.Errorf("invalid enqueue_download params: %w", err)
			}
		}
		downloadID, err := dl.EnqueueDownload(ctx, params.Title, params.DownloadURL, params.Protocol)
		if err != nil {
			return nil, err
		}
		return map[string]string{"downloadId": downloadID}, nil

	case "lookup_media":
		meta, ok := p.(MetadataPlugin)
		if !ok {
			return nil, errors.New("plugin does not implement MetadataPlugin")
		}
		var params lookupMediaParams
		if len(req.Params) > 0 && string(req.Params) != "null" {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return nil, fmt.Errorf("invalid lookup_media params: %w", err)
			}
		}
		items, err := meta.LookupMedia(ctx, params.MediaType, params.Query)
		if err != nil {
			return nil, err
		}
		if items == nil {
			items = []map[string]any{}
		}
		return items, nil

	case "notify":
		notifier, ok := p.(NotificationPlugin)
		if !ok {
			return nil, errors.New("plugin does not implement NotificationPlugin")
		}
		var params notifyParams
		if len(req.Params) > 0 && string(req.Params) != "null" {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return nil, fmt.Errorf("invalid notify params: %w", err)
			}
		}
		if err := notifier.Notify(ctx, params.EventType, params.Payload); err != nil {
			return nil, err
		}
		return map[string]any{"status": "ok"}, nil

	case "proxy_url":
		np, ok := p.(NetworkProxyPlugin)
		if !ok {
			return nil, errors.New("plugin does not implement NetworkProxyPlugin")
		}
		return map[string]string{"proxyUrl": np.ProxyURL()}, nil

	case "handle_webhook":
		wh, ok := p.(InboundWebhookPlugin)
		if !ok {
			return nil, errors.New("plugin does not implement InboundWebhookPlugin")
		}
		var params WebhookRequest
		if len(req.Params) > 0 && string(req.Params) != "null" {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return nil, fmt.Errorf("invalid handle_webhook params: %w", err)
			}
		}
		result, err := wh.HandleWebhook(ctx, params)
		if err != nil {
			return nil, err
		}
		if result == nil {
			result = &WebhookResult{Status: 204}
		}
		return result, nil

	case "health":
		m := p.Manifest()
		return map[string]any{
			"status":  "ok",
			"id":      m.ID,
			"name":    m.Name,
			"version": m.Version,
		}, nil

	default:
		return nil, fmt.Errorf("unknown rpc method: %s", req.Method)
	}
}
