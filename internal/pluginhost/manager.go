// Package pluginhost manages built-in in-process Go plugins and external
// standalone plugin binaries communicating over stdin/stdout JSON-RPC.
package pluginhost

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/domain"
	"github.com/replacearr/replacearr/internal/indexers/cardigann"
	"github.com/replacearr/replacearr/internal/metadata"
	"github.com/replacearr/replacearr/pkg/pluginsdk"
)

// DiscoveredPlugin represents the runtime state, manifest, and settings of a plugin.
type DiscoveredPlugin struct {
	Manifest   pluginsdk.Manifest `json:"manifest"`
	BinaryPath string             `json:"binaryPath"`
	Enabled    bool               `json:"enabled"`
	Status     string             `json:"status"` // "running", "disabled", "error", "builtin"
	LastError  string             `json:"lastError,omitempty"`
	Settings   map[string]any     `json:"settings"`
}

type managedEntry struct {
	info    DiscoveredPlugin
	builtin pluginsdk.Plugin
	proc    *pluginProcess
}

// Manager coordinates built-in Go plugins, external subprocess plugins, and Cardigann definitions.
type Manager struct {
	mu              sync.RWMutex
	store           *db.Store
	pluginDir       string
	definitionsDir  string
	entries         map[string]*managedEntry
	order           []string
	definitions     []cardigann.Definition
	browserHub      *notificationHub
	stalledNotified map[int64]struct{}
	meta            *metadata.Service
}

// NewManager creates a new Plugin Host Manager with built-in plugins registered out of the box.
func NewManager(store *db.Store, pluginDir string, definitionsDir string) *Manager {
	defs, _ := cardigann.LoadDefinitions(definitionsDir, 0)
	metaSvc := metadata.New()
	m := &Manager{
		store:           store,
		pluginDir:       pluginDir,
		definitionsDir:  definitionsDir,
		entries:         make(map[string]*managedEntry),
		definitions:     defs,
		browserHub:      newNotificationHub(),
		stalledNotified: make(map[int64]struct{}),
		meta:            metaSvc,
	}

	// Seed compat plugin_configs from legacy AppConfig.compatModules before register.
	m.migrateCompatModulesFromAppConfig()

	// Built-ins kept in-process: browser notifications + *arr compat Gate mounts.
	// Optional proxies/notifiers/indexers install from the marketplace catalog.
	m.registerBuiltin(newBuiltinBrowserNotifications(m.browserHub))
	m.registerBuiltin(&builtinCompatSonarrPlugin{})
	m.registerBuiltin(&builtinCompatRadarrPlugin{})
	m.registerBuiltin(&builtinCompatProwlarrPlugin{})
	m.registerBuiltin(&builtinCompatLidarrPlugin{})
	m.registerBuiltin(&builtinCompatReadarrPlugin{})

	return m
}

// Metadata returns the shared SkyHook / Radarr-video lookup service used by Add New.
func (m *Manager) Metadata() *metadata.Service {
	if m == nil {
		return metadata.New()
	}
	return m.meta
}

// IsEnabled reports whether a discovered/built-in plugin is currently enabled.
func (m *Manager) IsEnabled(pluginID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.entries[pluginID]
	return ok && entry.info.Enabled
}

// Gate returns a handler that serves next only while pluginID is enabled; otherwise 404.
func (m *Manager) Gate(pluginID string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.IsEnabled(pluginID) {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// migrateCompatModulesFromAppConfig one-time seeds builtin-compat-* rows from the
// deprecated AppConfig.compatModules JSON key when no plugin_configs row exists.
func (m *Manager) migrateCompatModulesFromAppConfig() {
	if m.store == nil {
		return
	}
	legacy, found, err := m.store.PeekLegacyCompatModules()
	sonarr, radarr, prowlarr := true, true, true
	if err == nil && found {
		sonarr, radarr, prowlarr = legacy.Sonarr, legacy.Radarr, legacy.Prowlarr
	}
	seed := func(id string, enabled bool) {
		if m.store.HasPluginConfig(id) {
			return
		}
		_ = m.store.SavePluginConfig(id, enabled, map[string]any{})
	}
	seed(pluginsdk.CompatPluginSonarr, sonarr)
	seed(pluginsdk.CompatPluginRadarr, radarr)
	seed(pluginsdk.CompatPluginProwlarr, prowlarr)
	seed(pluginsdk.CompatPluginLidarr, true)
	seed(pluginsdk.CompatPluginReadarr, true)
}

// ActiveProxyURL returns the first enabled network proxy plugin URL, if any.
// Unreachable proxy endpoints are skipped so a dead SOCKS listener cannot
// black-hole indexer HTTP.
func (m *Manager) ActiveProxyURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, id := range m.order {
		entry, ok := m.entries[id]
		if !ok || !entry.info.Enabled {
			continue
		}
		if !hasPluginType(entry.info.Manifest.Types, pluginsdk.PluginTypeNetworkProxy) {
			continue
		}
		var u string
		if entry.builtin != nil {
			if np, ok := entry.builtin.(pluginsdk.NetworkProxyPlugin); ok {
				u = strings.TrimSpace(np.ProxyURL())
			}
		} else if entry.proc != nil {
			var out map[string]string
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			err := entry.proc.call(ctx, "proxy_url", nil, &out)
			cancel()
			if err == nil {
				u = strings.TrimSpace(out["proxyUrl"])
			}
		}
		if u != "" && proxyEndpointReachable(u) {
			return u
		}
	}
	return ""
}

// NotifyAll fans out a lifecycle event to every enabled NotificationPlugin.
// Per-plugin failures are logged and do not fail the caller.
func (m *Manager) NotifyAll(ctx context.Context, eventType string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	m.mu.RLock()
	targets := make([]*managedEntry, 0)
	for _, id := range m.order {
		entry, ok := m.entries[id]
		if !ok || !entry.info.Enabled {
			continue
		}
		if hasPluginType(entry.info.Manifest.Types, pluginsdk.PluginTypeNotification) {
			targets = append(targets, entry)
		}
	}
	m.mu.RUnlock()

	for _, entry := range targets {
		if entry.builtin != nil {
			notifier, ok := entry.builtin.(pluginsdk.NotificationPlugin)
			if !ok {
				continue
			}
			if err := notifier.Notify(ctx, eventType, payload); err != nil {
				log.Printf("notification plugin %s: %v", entry.info.Manifest.ID, err)
			}
			continue
		}
		if entry.proc != nil {
			var out map[string]any
			params := map[string]any{
				"eventType": eventType,
				"payload":   payload,
			}
			if err := entry.proc.call(ctx, "notify", params, &out); err != nil {
				log.Printf("notification plugin %s: %v", entry.info.Manifest.ID, err)
			}
		}
	}
}

// TestNotification sends EventNotificationTest to a single notification plugin.
// Optional settings are applied temporarily (without persisting) so the UI can
// test form values before Save. Built-ins can be tested while disabled; external
// subprocess plugins must be enabled (process running).
func (m *Manager) TestNotification(ctx context.Context, pluginID string, settings map[string]any) error {
	m.mu.Lock()
	entry, ok := m.entries[pluginID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("plugin not found: %s", pluginID)
	}
	if !hasPluginType(entry.info.Manifest.Types, pluginsdk.PluginTypeNotification) {
		m.mu.Unlock()
		return fmt.Errorf("plugin %s is not a notification channel", pluginID)
	}

	prevSettings := cloneSettings(entry.info.Settings)
	merged := defaultSettingsFromSchema(entry.info.Manifest.ConfigSchema)
	for k, v := range prevSettings {
		merged[k] = v
	}
	for k, v := range settings {
		merged[k] = v
	}

	restore := func() {
		if entry.builtin != nil {
			_ = entry.builtin.Configure(prevSettings)
			return
		}
		if entry.proc != nil && entry.info.Enabled {
			entry.proc.setSettings(prevSettings)
			callCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var ack map[string]any
			_ = entry.proc.call(callCtx, "configure", prevSettings, &ack)
		}
	}

	if entry.builtin != nil {
		if err := entry.builtin.Configure(merged); err != nil {
			m.mu.Unlock()
			return err
		}
		notifier, ok := entry.builtin.(pluginsdk.NotificationPlugin)
		m.mu.Unlock()
		if !ok {
			restore()
			return fmt.Errorf("plugin %s does not implement NotificationPlugin", pluginID)
		}
		err := notifier.Notify(ctx, pluginsdk.EventNotificationTest, testNotificationPayload())
		m.mu.Lock()
		restore()
		m.mu.Unlock()
		return err
	}

	if entry.proc == nil || !entry.info.Enabled {
		m.mu.Unlock()
		return fmt.Errorf("enable plugin %s before testing", pluginID)
	}

	entry.proc.setSettings(merged)
	cfgCtx, cfgCancel := context.WithTimeout(ctx, 10*time.Second)
	var ack map[string]any
	cfgErr := entry.proc.call(cfgCtx, "configure", merged, &ack)
	cfgCancel()
	if cfgErr != nil {
		restore()
		m.mu.Unlock()
		return cfgErr
	}

	proc := entry.proc
	m.mu.Unlock()

	notifyCtx, notifyCancel := context.WithTimeout(ctx, 15*time.Second)
	defer notifyCancel()
	params := map[string]any{
		"eventType": pluginsdk.EventNotificationTest,
		"payload":   testNotificationPayload(),
	}
	var out map[string]any
	err := proc.call(notifyCtx, "notify", params, &out)

	m.mu.Lock()
	restore()
	m.mu.Unlock()
	return err
}

func testNotificationPayload() map[string]any {
	return map[string]any{
		"title": "Replacearr test notification",
		"test":  true,
	}
}

func cloneSettings(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// SubscribeBrowserNotifications returns a channel of browser-bound notification events.
// The returned cancel function must be called to unsubscribe.
func (m *Manager) SubscribeBrowserNotifications() (<-chan BrowserNotificationEvent, func()) {
	if m.browserHub == nil {
		ch := make(chan BrowserNotificationEvent)
		close(ch)
		return ch, func() {}
	}
	return m.browserHub.Subscribe()
}

// StalledDownloadDays returns the stalled threshold from the browser notification
// plugin settings when that plugin is enabled, otherwise a default of 3 days.
func (m *Manager) StalledDownloadDays() int {
	const fallback = 3
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.entries[pluginsdk.BuiltinBrowserNotifications]
	if !ok || !entry.info.Enabled {
		return fallback
	}
	switch v := entry.info.Settings["stalledDays"].(type) {
	case float64:
		if int(v) > 0 {
			return int(v)
		}
	case int:
		if v > 0 {
			return v
		}
	case int64:
		if int(v) > 0 {
			return int(v)
		}
	case json.Number:
		if n, err := v.Int64(); err == nil && n > 0 {
			return int(n)
		}
	}
	return fallback
}

// CheckStalledDownloads emits download.stalled once per queue item older than the threshold.
func (m *Manager) CheckStalledDownloads(ctx context.Context, items []domain.QueueItem) {
	days := m.StalledDownloadDays()
	if days <= 0 {
		return
	}
	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	for _, item := range items {
		if item.Status == "completed" || item.TrackedDownloadState == "imported" {
			m.mu.Lock()
			delete(m.stalledNotified, item.ID)
			m.mu.Unlock()
			continue
		}
		added := item.Added
		if added.IsZero() || !added.Before(cutoff) {
			continue
		}
		m.mu.Lock()
		_, already := m.stalledNotified[item.ID]
		if !already {
			m.stalledNotified[item.ID] = struct{}{}
		}
		m.mu.Unlock()
		if already {
			continue
		}
		m.NotifyAll(ctx, pluginsdk.EventDownloadStalled, map[string]any{
			"title":      item.Title,
			"queueId":    item.ID,
			"downloadId": item.DownloadID,
			"mediaType":  item.MediaType,
			"days":       days,
			"added":      added.Format(time.RFC3339),
		})
	}
}

// ListPluginDownloads aggregates ListDownloads from enabled download client plugins that support it.
func (m *Manager) ListPluginDownloads(ctx context.Context) ([]pluginsdk.DownloadItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []pluginsdk.DownloadItem{}
	for _, id := range m.order {
		entry, ok := m.entries[id]
		if !ok || !entry.info.Enabled || entry.builtin == nil {
			continue
		}
		lister, ok := entry.builtin.(pluginsdk.DownloadLister)
		if !ok {
			continue
		}
		items, err := lister.ListDownloads(ctx)
		if err != nil {
			continue
		}
		out = append(out, items...)
	}
	return out, nil
}

// EnqueueDownload sends a download to the first enabled download client plugin that accepts it.
func (m *Manager) EnqueueDownload(ctx context.Context, title, downloadURL, protocol string) (string, error) {
	m.mu.RLock()
	targets := make([]*managedEntry, 0)
	for _, id := range m.order {
		entry, ok := m.entries[id]
		if !ok || !entry.info.Enabled {
			continue
		}
		if hasPluginType(entry.info.Manifest.Types, pluginsdk.PluginTypeDownloadClient) {
			targets = append(targets, entry)
		}
	}
	m.mu.RUnlock()

	if len(targets) == 0 {
		return "", errors.New("no enabled download client plugin")
	}
	var lastErr error
	for _, entry := range targets {
		if entry.builtin != nil {
			if dl, ok := entry.builtin.(pluginsdk.DownloadClientPlugin); ok {
				id, err := dl.EnqueueDownload(ctx, title, downloadURL, protocol)
				if err != nil {
					lastErr = err
					continue
				}
				if id != "" {
					return id, nil
				}
			}
			continue
		}
		if entry.proc != nil {
			var out map[string]string
			params := map[string]string{
				"title":       title,
				"downloadUrl": downloadURL,
				"protocol":    protocol,
			}
			if err := entry.proc.call(ctx, "enqueue_download", params, &out); err != nil {
				lastErr = err
				continue
			}
			if id := out["downloadId"]; id != "" {
				return id, nil
			}
		}
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", errors.New("no download client accepted this release")
}

func defaultPluginEnabled(manifest pluginsdk.Manifest) bool {
	return !manifest.DisabledByDefault
}

// RegisterBuiltinForTest registers an in-process plugin. Intended for unit tests.
func (m *Manager) RegisterBuiltinForTest(p pluginsdk.Plugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registerBuiltinLocked(p)
}

func (m *Manager) registerBuiltin(p pluginsdk.Plugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registerBuiltinLocked(p)
}

func (m *Manager) registerBuiltinLocked(p pluginsdk.Plugin) {
	manifest := p.Manifest()
	settings := defaultSettingsFromSchema(manifest.ConfigSchema)
	enabled := defaultPluginEnabled(manifest)

	if m.store != nil {
		if dbEnabled, dbSettings, err := m.store.GetPluginConfig(manifest.ID); err == nil {
			enabled = dbEnabled
			for k, v := range dbSettings {
				settings[k] = v
			}
		}
	}

	status := "builtin"
	var lastErr string
	if enabled {
		if err := p.Configure(settings); err != nil {
			status = "error"
			lastErr = err.Error()
		}
	} else {
		status = "disabled"
	}

	m.entries[manifest.ID] = &managedEntry{
		info: DiscoveredPlugin{
			Manifest:   manifest,
			BinaryPath: "builtin://" + manifest.ID,
			Enabled:    enabled,
			Status:     status,
			LastError:  lastErr,
			Settings:   settings,
		},
		builtin: p,
	}
	m.order = append(m.order, manifest.ID)
}

// DiscoverAndStart loads Cardigann definitions, configures built-in plugins, and scans
// pluginDir for executable plugin binaries to spawn and configure.
func (m *Manager) DiscoverAndStart() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if defs, err := cardigann.LoadDefinitions(m.definitionsDir, 0); err == nil && len(defs) > 0 {
		m.definitions = defs
	}

	// Refresh built-in plugin configs from store
	for _, id := range m.order {
		entry, ok := m.entries[id]
		if !ok || entry.builtin == nil {
			continue
		}
		settings := defaultSettingsFromSchema(entry.info.Manifest.ConfigSchema)
		enabled := defaultPluginEnabled(entry.info.Manifest)
		if m.store != nil {
			if dbEnabled, dbSettings, err := m.store.GetPluginConfig(id); err == nil {
				enabled = dbEnabled
				for k, v := range dbSettings {
					settings[k] = v
				}
			}
		}
		entry.info.Enabled = enabled
		entry.info.Settings = settings
		if enabled {
			if err := entry.builtin.Configure(settings); err != nil {
				entry.info.Status = "error"
				entry.info.LastError = err.Error()
			} else {
				entry.info.Status = "builtin"
				entry.info.LastError = ""
			}
		} else {
			entry.info.Status = "disabled"
		}
	}

	if strings.TrimSpace(m.pluginDir) == "" {
		return nil
	}

	dirEntries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read plugin dir %s: %w", m.pluginDir, err)
	}

	var binPaths []string
	for _, de := range dirEntries {
		if de.IsDir() || strings.HasPrefix(de.Name(), ".") {
			continue
		}
		fullPath := filepath.Join(m.pluginDir, de.Name())
		fi, err := os.Stat(fullPath)
		if err != nil || fi.IsDir() {
			continue
		}
		if fi.Mode()&0o111 == 0 {
			continue
		}
		binPaths = append(binPaths, fullPath)
	}
	sort.Strings(binPaths)

	// Drop external entries whose binaries disappeared (e.g. marketplace uninstall).
	seenBins := make(map[string]struct{}, len(binPaths))
	for _, p := range binPaths {
		seenBins[p] = struct{}{}
	}
	var keepOrder []string
	for _, id := range m.order {
		entry, ok := m.entries[id]
		if !ok {
			continue
		}
		if entry.builtin != nil {
			keepOrder = append(keepOrder, id)
			continue
		}
		if _, stillThere := seenBins[entry.info.BinaryPath]; stillThere {
			keepOrder = append(keepOrder, id)
			continue
		}
		if entry.proc != nil {
			entry.proc.stop()
		}
		delete(m.entries, id)
	}
	m.order = keepOrder

	for _, binPath := range binPaths {
		proc := newPluginProcess(binPath)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		var manifest pluginsdk.Manifest
		err := proc.call(ctx, "manifest", nil, &manifest)
		if err != nil || manifest.ID == "" {
			cancel()
			proc.stop()
			fallbackID := filepath.Base(binPath)
			errMsg := "manifest returned empty plugin id"
			if err != nil {
				errMsg = err.Error()
			}
			m.upsertEntryLocked(fallbackID, &managedEntry{
				info: DiscoveredPlugin{
					Manifest: pluginsdk.Manifest{
						ID:   fallbackID,
						Name: fallbackID,
					},
					BinaryPath: binPath,
					Enabled:    false,
					Status:     "error",
					LastError:  errMsg,
					Settings:   map[string]any{},
				},
				proc: proc,
			})
			continue
		}

		settings := defaultSettingsFromSchema(manifest.ConfigSchema)
		enabled := defaultPluginEnabled(manifest)
		if m.store != nil {
			if dbEnabled, dbSettings, dbErr := m.store.GetPluginConfig(manifest.ID); dbErr == nil {
				enabled = dbEnabled
				for k, v := range dbSettings {
					settings[k] = v
				}
			}
		}

		proc.setSettings(settings)

		status := "running"
		var lastErr string
		if enabled {
			var cfgAck map[string]any
			if err := proc.call(ctx, "configure", settings, &cfgAck); err != nil {
				status = "error"
				lastErr = err.Error()
			}
		} else {
			status = "disabled"
			proc.stop()
		}
		cancel()

		m.upsertEntryLocked(manifest.ID, &managedEntry{
			info: DiscoveredPlugin{
				Manifest:   manifest,
				BinaryPath: binPath,
				Enabled:    enabled,
				Status:     status,
				LastError:  lastErr,
				Settings:   settings,
			},
			proc: proc,
		})
	}

	return nil
}

func (m *Manager) upsertEntryLocked(id string, entry *managedEntry) {
	if prev, exists := m.entries[id]; exists {
		if prev.proc != nil && prev.proc != entry.proc {
			prev.proc.stop()
		}
		m.entries[id] = entry
		return
	}
	m.entries[id] = entry
	m.order = append(m.order, id)
}

// StopAll cleanly shuts down all running external plugin subprocesses.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, entry := range m.entries {
		if entry.proc != nil {
			entry.proc.stop()
			if entry.info.Status == "running" {
				entry.info.Status = "disabled"
			}
		}
	}
}

// ListPlugins returns a snapshot of all built-in and discovered external plugins.
func (m *Manager) ListPlugins() []DiscoveredPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]DiscoveredPlugin, 0, len(m.order))
	for _, id := range m.order {
		if entry, ok := m.entries[id]; ok {
			out = append(out, cloneDiscoveredPlugin(entry.info))
		}
	}
	return out
}

// RemoveExternalPlugin stops an external plugin subprocess, removes it from the
// manager, and deletes its plugin_configs row. Built-ins cannot be removed.
func (m *Manager) RemoveExternalPlugin(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[pluginID]
	if !ok {
		return nil
	}
	if entry.builtin != nil {
		return fmt.Errorf("cannot uninstall built-in plugin %q", pluginID)
	}
	if entry.proc != nil {
		entry.proc.stop()
	}
	delete(m.entries, pluginID)
	filtered := m.order[:0]
	for _, id := range m.order {
		if id != pluginID {
			filtered = append(filtered, id)
		}
	}
	m.order = filtered
	if m.store != nil {
		_ = m.store.DeletePluginConfig(pluginID)
	}
	return nil
}

// PluginSettings returns whether pluginID is known, its enabled flag, and a copy of settings.
func (m *Manager) PluginSettings(pluginID string) (enabled bool, settings map[string]any, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, found := m.entries[pluginID]
	if !found {
		return false, nil, false
	}
	return entry.info.Enabled, cloneSettings(entry.info.Settings), true
}

// ResolveInboundWebhook maps a path segment (plugin id or PathAlias) plus optional slug
// to an enabled plugin that declared matching Manifest.InboundWebhooks.
func (m *Manager) ResolveInboundWebhook(pathID, slug string) (pluginID string, spec pluginsdk.InboundWebhookSpec, settings map[string]any, ok bool) {
	pathID = strings.TrimSpace(pathID)
	slug = strings.TrimSpace(slug)
	if pathID == "" || m == nil {
		return "", pluginsdk.InboundWebhookSpec{}, nil, false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	match := func(entry *managedEntry) (pluginsdk.InboundWebhookSpec, bool) {
		if entry == nil || !entry.info.Enabled {
			return pluginsdk.InboundWebhookSpec{}, false
		}
		for _, wh := range entry.info.Manifest.InboundWebhooks {
			if strings.TrimSpace(wh.Slug) != slug {
				continue
			}
			if wh.SecretSetting == "" {
				continue
			}
			return wh, true
		}
		return pluginsdk.InboundWebhookSpec{}, false
	}

	if entry, found := m.entries[pathID]; found {
		if spec, ok := match(entry); ok {
			return pathID, spec, cloneSettings(entry.info.Settings), true
		}
	}

	// Path aliases (e.g. "jellyfin" → jellyfin-notifier).
	for _, id := range m.order {
		entry := m.entries[id]
		if entry == nil || !entry.info.Enabled {
			continue
		}
		for _, wh := range entry.info.Manifest.InboundWebhooks {
			if strings.TrimSpace(wh.Slug) != slug {
				continue
			}
			if wh.SecretSetting == "" {
				continue
			}
			for _, alias := range wh.PathAliases {
				if strings.TrimSpace(alias) == pathID {
					return id, wh, cloneSettings(entry.info.Settings), true
				}
			}
		}
	}
	return "", pluginsdk.InboundWebhookSpec{}, nil, false
}

// HandleInboundWebhook authenticates via the plugin's secret setting, invokes
// InboundWebhookPlugin.HandleWebhook (or RPC handle_webhook), and fans out returned events.
func (m *Manager) HandleInboundWebhook(ctx context.Context, pathID, slug, providedSecret string, headers, query map[string]string, body []byte) (status int, err error) {
	pluginID, spec, settings, ok := m.ResolveInboundWebhook(pathID, slug)
	if !ok {
		return http.StatusUnauthorized, errors.New("unauthorized")
	}
	expected := ""
	if settings != nil {
		if v, ok := settings[spec.SecretSetting].(string); ok {
			expected = strings.TrimSpace(v)
		}
	}
	if expected == "" || !secureCompareSecret(providedSecret, expected) {
		return http.StatusUnauthorized, errors.New("unauthorized")
	}

	req := pluginsdk.WebhookRequest{
		Slug:    slug,
		Headers: headers,
		Query:   query,
		Body:    append(json.RawMessage(nil), body...),
	}

	m.mu.RLock()
	entry, found := m.entries[pluginID]
	m.mu.RUnlock()
	if !found {
		return http.StatusUnauthorized, errors.New("unauthorized")
	}

	var result *pluginsdk.WebhookResult
	if entry.builtin != nil {
		wh, ok := entry.builtin.(pluginsdk.InboundWebhookPlugin)
		if !ok {
			return http.StatusNotImplemented, fmt.Errorf("plugin %s does not implement InboundWebhookPlugin", pluginID)
		}
		result, err = wh.HandleWebhook(ctx, req)
	} else if entry.proc != nil {
		callCtx := ctx
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			callCtx, cancel = context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
		}
		var out pluginsdk.WebhookResult
		err = entry.proc.call(callCtx, "handle_webhook", req, &out)
		if err == nil {
			result = &out
		}
	} else {
		return http.StatusServiceUnavailable, fmt.Errorf("plugin %s is not running", pluginID)
	}
	if err != nil {
		return http.StatusBadGateway, err
	}
	if result == nil {
		result = &pluginsdk.WebhookResult{Status: http.StatusNoContent}
	}
	status = result.Status
	if status == 0 {
		status = http.StatusNoContent
	}
	for _, ev := range result.Events {
		if strings.TrimSpace(ev.EventType) == "" {
			continue
		}
		m.NotifyAll(ctx, ev.EventType, ev.Payload)
	}
	return status, nil
}

func secureCompareSecret(provided, expected string) bool {
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

// ConfigurePlugin updates a plugin's enabled state and settings, persists them to SQLite,
// and applies the configuration to the running plugin (or stops it when disabled).
func (m *Manager) ConfigurePlugin(pluginID string, enabled bool, settings map[string]any) (*DiscoveredPlugin, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[pluginID]
	if !ok {
		return nil, fmt.Errorf("plugin not found: %s", pluginID)
	}

	merged := defaultSettingsFromSchema(entry.info.Manifest.ConfigSchema)
	for k, v := range entry.info.Settings {
		merged[k] = v
	}
	for k, v := range settings {
		merged[k] = v
	}

	if m.store != nil {
		if err := m.store.SavePluginConfig(pluginID, enabled, merged); err != nil {
			return nil, fmt.Errorf("save plugin config: %w", err)
		}
	}

	entry.info.Enabled = enabled
	entry.info.Settings = merged

	if entry.builtin != nil {
		if enabled {
			if err := entry.builtin.Configure(merged); err != nil {
				entry.info.Status = "error"
				entry.info.LastError = err.Error()
				return nil, err
			}
			entry.info.Status = "builtin"
			entry.info.LastError = ""
		} else {
			entry.info.Status = "disabled"
			entry.info.LastError = ""
		}
		cp := cloneDiscoveredPlugin(entry.info)
		return &cp, nil
	}

	if entry.proc != nil {
		entry.proc.setSettings(merged)
		if !enabled {
			entry.proc.stop()
			entry.info.Status = "disabled"
			entry.info.LastError = ""
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var ack map[string]any
			if err := entry.proc.call(ctx, "configure", merged, &ack); err != nil {
				entry.info.Status = "error"
				entry.info.LastError = err.Error()
				return nil, err
			}
			entry.info.Status = "running"
			entry.info.LastError = ""
		}
	}

	cp := cloneDiscoveredPlugin(entry.info)
	return &cp, nil
}

// SearchAllIndexers executes a search across all enabled indexer plugins and returns unified domain.ReleaseResult items.
func (m *Manager) SearchAllIndexers(ctx context.Context, req pluginsdk.SearchRequest) ([]domain.ReleaseResult, error) {
	m.mu.RLock()
	targets := make([]*managedEntry, 0, len(m.order))
	for _, id := range m.order {
		entry, ok := m.entries[id]
		if !ok || !entry.info.Enabled {
			continue
		}
		if hasPluginType(entry.info.Manifest.Types, pluginsdk.PluginTypeIndexer) {
			targets = append(targets, entry)
		}
	}
	m.mu.RUnlock()

	var results []domain.ReleaseResult
	for idx, entry := range targets {
		indexerID := int64(idx + 1)
		indexerName := entry.info.Manifest.Name

		var releases []pluginsdk.Release
		var err error

		if entry.builtin != nil {
			if ip, ok := entry.builtin.(pluginsdk.IndexerPlugin); ok {
				releases, err = ip.Search(ctx, req)
			}
		} else if entry.proc != nil {
			callCtx := ctx
			if _, hasDeadline := ctx.Deadline(); !hasDeadline {
				var cancel context.CancelFunc
				callCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
			}
			err = entry.proc.call(callCtx, "search", req, &releases)
		}

		if err != nil {
			m.mu.Lock()
			entry.info.LastError = err.Error()
			m.mu.Unlock()
			continue
		}

		for _, rel := range releases {
			pubTime := time.Now().UTC()
			if rel.PublishDate != "" {
				if parsed, pErr := time.Parse(time.RFC3339, rel.PublishDate); pErr == nil {
					pubTime = parsed
				}
			}
			ageDays := int(time.Since(pubTime).Hours() / 24)
			if ageDays < 0 {
				ageDays = 0
			}
			leechers := rel.Peers - rel.Seeders
			if leechers < 0 {
				leechers = 0
			}
			protocol := rel.Protocol
			if protocol == "" {
				protocol = "torrent"
			}
			cats := rel.Categories
			if cats == nil {
				cats = []int{2000, 5000}
			}

			results = append(results, domain.ReleaseResult{
				Guid:        rel.Guid,
				Title:       rel.Title,
				IndexerID:   indexerID,
				Indexer:     indexerName,
				Protocol:    protocol,
				Size:        rel.Size,
				Age:         ageDays,
				PublishDate: pubTime,
				DownloadURL: rel.DownloadURL,
				InfoURL:     rel.InfoURL,
				Seeders:     rel.Seeders,
				Leechers:    leechers,
				Peers:       rel.Peers,
				Categories:  cats,
				Quality:     rel.Quality,
				TvdbID:      req.TvdbID,
				TmdbID:      req.TmdbID,
				ImdbID:      req.ImdbID,
			})
		}
	}

	if results == nil {
		results = []domain.ReleaseResult{}
	}
	return results, nil
}

// ListCardigannDefinitions returns the loaded Cardigann YAML definitions.
func (m *Manager) ListCardigannDefinitions() []cardigann.Definition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]cardigann.Definition, len(m.definitions))
	copy(out, m.definitions)
	return out
}

// --- Subprocess JSON-RPC client ---

type pluginProcess struct {
	binaryPath string
	mu         sync.Mutex
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Scanner
	waitCh     chan error
	reqID      atomic.Int64
	settings   map[string]any
}

func newPluginProcess(binaryPath string) *pluginProcess {
	return &pluginProcess{
		binaryPath: binaryPath,
		settings:   map[string]any{},
	}
}

func (p *pluginProcess) setSettings(s map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := make(map[string]any, len(s))
	for k, v := range s {
		cp[k] = v
	}
	p.settings = cp
}

func (p *pluginProcess) startLocked() error {
	if p.cmd != nil && p.stdin != nil && p.stdout != nil {
		select {
		case <-p.waitCh:
			// Process exited; clean up before restarting
			p.cleanupLocked()
		default:
			return nil
		}
	}

	cmd := exec.Command(p.binaryPath)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdoutPipe.Close()
		return fmt.Errorf("start plugin %s: %w", p.binaryPath, err)
	}

	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	p.cmd = cmd
	p.stdin = stdin
	p.stdout = scanner
	p.waitCh = waitCh
	return nil
}

func (p *pluginProcess) cleanupLocked() {
	if p.stdin != nil {
		_ = p.stdin.Close()
		p.stdin = nil
	}
	if p.cmd != nil && p.cmd.Process != nil {
		select {
		case <-p.waitCh:
		case <-time.After(300 * time.Millisecond):
			_ = p.cmd.Process.Kill()
			<-p.waitCh
		}
	}
	p.cmd = nil
	p.stdout = nil
	p.waitCh = nil
}

func (p *pluginProcess) stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cleanupLocked()
}

func (p *pluginProcess) call(ctx context.Context, method string, params any, out any) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	err := p.callOnceLocked(ctx, method, params, out)
	if err == nil {
		return nil
	}

	// If the subprocess crashed or pipe broke, restart once and replay configuration + request.
	p.cleanupLocked()
	if startErr := p.startLocked(); startErr != nil {
		return err
	}
	if method != "configure" && method != "manifest" && len(p.settings) > 0 {
		var ack map[string]any
		_ = p.callOnceLocked(ctx, "configure", p.settings, &ack)
	}
	return p.callOnceLocked(ctx, method, params, out)
}

func (p *pluginProcess) callOnceLocked(ctx context.Context, method string, params any, out any) error {
	if err := p.startLocked(); err != nil {
		return err
	}

	id := p.reqID.Add(1)
	var rawParams json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal params: %w", err)
		}
		rawParams = b
	}

	req := pluginsdk.RPCRequest{
		ID:     id,
		Method: method,
		Params: rawParams,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	reqBytes = append(reqBytes, '\n')

	type scanResult struct {
		line string
		err  error
	}
	resCh := make(chan scanResult, 1)

	if _, err := p.stdin.Write(reqBytes); err != nil {
		return fmt.Errorf("write request to plugin: %w", err)
	}

	scanner := p.stdout
	go func() {
		if scanner.Scan() {
			resCh <- scanResult{line: scanner.Text()}
			return
		}
		if sErr := scanner.Err(); sErr != nil {
			resCh <- scanResult{err: sErr}
			return
		}
		resCh <- scanResult{err: io.EOF}
	}()

	select {
	case <-ctx.Done():
		p.cleanupLocked()
		return ctx.Err()
	case sr := <-resCh:
		if sr.err != nil {
			return fmt.Errorf("read response from plugin: %w", sr.err)
		}
		var resp pluginsdk.RPCResponse
		if err := json.Unmarshal([]byte(sr.line), &resp); err != nil {
			return fmt.Errorf("invalid plugin rpc response: %w", err)
		}
		if resp.Error != "" {
			return errors.New(resp.Error)
		}
		if out != nil && len(resp.Result) > 0 && string(resp.Result) != "null" {
			if err := json.Unmarshal(resp.Result, out); err != nil {
				return fmt.Errorf("unmarshal plugin result: %w", err)
			}
		}
		return nil
	}
}

// --- Built-in *arr compatibility shims (in-process HTTP mounts) ---

type builtinCompatSonarrPlugin struct{}

func (b *builtinCompatSonarrPlugin) Manifest() pluginsdk.Manifest {
	return pluginsdk.Manifest{
		ID:          pluginsdk.CompatPluginSonarr,
		Name:        "Sonarr v3 Compatibility",
		Version:     "1.0.0",
		Author:      "Replacearr Core",
		Description: "Drop-in Sonarr v3 API at /sonarr and /api/v3 for Overseerr, Bazarr, and nzb360.",
		Types:       []pluginsdk.PluginType{pluginsdk.PluginTypeCompat},
		ConfigSchema: []pluginsdk.ConfigFieldSchema{
			{
				Name:     "basePath",
				Label:    "Base path",
				Type:     "text",
				Default:  "/sonarr",
				Required: false,
				HelpText: "Also mounts root /api/v3 when enabled. Disable to return HTTP 404 on these routes.",
			},
		},
	}
}

func (b *builtinCompatSonarrPlugin) Configure(_ map[string]any) error { return nil }

type builtinCompatRadarrPlugin struct{}

func (b *builtinCompatRadarrPlugin) Manifest() pluginsdk.Manifest {
	return pluginsdk.Manifest{
		ID:          pluginsdk.CompatPluginRadarr,
		Name:        "Radarr v3 Compatibility",
		Version:     "1.0.0",
		Author:      "Replacearr Core",
		Description: "Drop-in Radarr v3 API at /radarr for Overseerr, Bazarr, and nzb360.",
		Types:       []pluginsdk.PluginType{pluginsdk.PluginTypeCompat},
		ConfigSchema: []pluginsdk.ConfigFieldSchema{
			{
				Name:     "basePath",
				Label:    "Base path",
				Type:     "text",
				Default:  "/radarr",
				Required: false,
				HelpText: "Disable to return HTTP 404 on /radarr routes.",
			},
		},
	}
}

func (b *builtinCompatRadarrPlugin) Configure(_ map[string]any) error { return nil }

type builtinCompatProwlarrPlugin struct{}

func (b *builtinCompatProwlarrPlugin) Manifest() pluginsdk.Manifest {
	return pluginsdk.Manifest{
		ID:          pluginsdk.CompatPluginProwlarr,
		Name:        "Prowlarr v1 Compatibility",
		Version:     "1.0.0",
		Author:      "Replacearr Core",
		Description: "Drop-in Prowlarr v1 API and Torznab feeds at /prowlarr.",
		Types:       []pluginsdk.PluginType{pluginsdk.PluginTypeCompat},
		ConfigSchema: []pluginsdk.ConfigFieldSchema{
			{
				Name:     "basePath",
				Label:    "Base path",
				Type:     "text",
				Default:  "/prowlarr",
				Required: false,
				HelpText: "Disable to return HTTP 404 on /prowlarr routes. Native /api/v1 stays available.",
			},
		},
	}
}

func (b *builtinCompatProwlarrPlugin) Configure(_ map[string]any) error { return nil }

type builtinCompatLidarrPlugin struct{}

func (b *builtinCompatLidarrPlugin) Manifest() pluginsdk.Manifest {
	return pluginsdk.Manifest{
		ID:          pluginsdk.CompatPluginLidarr,
		Name:        "Lidarr v1 Compatibility",
		Version:     "1.0.0",
		Author:      "Replacearr Core",
		Description: "Thin Lidarr-compatible API at /lidarr for migration tools. Native Music UI is preferred.",
		Types:       []pluginsdk.PluginType{pluginsdk.PluginTypeCompat},
		ConfigSchema: []pluginsdk.ConfigFieldSchema{
			{
				Name: "basePath", Label: "Base path", Type: "text", Default: "/lidarr", Required: false,
				HelpText: "Disable to return HTTP 404 on /lidarr routes.",
			},
		},
	}
}

func (b *builtinCompatLidarrPlugin) Configure(_ map[string]any) error { return nil }

type builtinCompatReadarrPlugin struct{}

func (b *builtinCompatReadarrPlugin) Manifest() pluginsdk.Manifest {
	return pluginsdk.Manifest{
		ID:          pluginsdk.CompatPluginReadarr,
		Name:        "Readarr v1 Compatibility",
		Version:     "1.0.0",
		Author:      "Replacearr Core",
		Description: "Thin Readarr-compatible API at /readarr for migration tools. Native Books UI is preferred.",
		Types:       []pluginsdk.PluginType{pluginsdk.PluginTypeCompat},
		ConfigSchema: []pluginsdk.ConfigFieldSchema{
			{
				Name: "basePath", Label: "Base path", Type: "text", Default: "/readarr", Required: false,
				HelpText: "Disable to return HTTP 404 on /readarr routes.",
			},
		},
	}
}

func (b *builtinCompatReadarrPlugin) Configure(_ map[string]any) error { return nil }

// --- Helpers ---

func defaultSettingsFromSchema(schema []pluginsdk.ConfigFieldSchema) map[string]any {
	out := make(map[string]any, len(schema))
	for _, field := range schema {
		if field.Default != nil {
			out[field.Name] = field.Default
		}
	}
	return out
}

func hasPluginType(types []pluginsdk.PluginType, target pluginsdk.PluginType) bool {
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}

func proxyEndpointReachable(proxyURL string) bool {
	u, err := url.Parse(proxyURL)
	if err != nil || u.Host == "" {
		return false
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		switch strings.ToLower(u.Scheme) {
		case "https":
			host += ":443"
		case "socks5", "socks5h", "socks4":
			host += ":1080"
		default:
			host += ":80"
		}
	}
	conn, err := net.DialTimeout("tcp", host, 400*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func cloneDiscoveredPlugin(in DiscoveredPlugin) DiscoveredPlugin {
	cp := in
	cp.Settings = make(map[string]any, len(in.Settings))
	for k, v := range in.Settings {
		cp.Settings[k] = v
	}
	return cp
}
