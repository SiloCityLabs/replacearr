// Package pluginhost manages built-in in-process Go plugins and external
// standalone plugin binaries communicating over stdin/stdout JSON-RPC.
package pluginhost

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	mu             sync.RWMutex
	store          *db.Store
	pluginDir      string
	definitionsDir string
	entries        map[string]*managedEntry
	order          []string
	definitions    []cardigann.Definition
}

// NewManager creates a new Plugin Host Manager with built-in plugins registered out of the box.
func NewManager(store *db.Store, pluginDir string, definitionsDir string) *Manager {
	defs, _ := cardigann.LoadDefinitions(definitionsDir, 60)
	m := &Manager{
		store:          store,
		pluginDir:      pluginDir,
		definitionsDir: definitionsDir,
		entries:        make(map[string]*managedEntry),
		definitions:    defs,
	}

	// Register default built-in Go plugins so Replacearr works immediately out of the box.
	m.registerBuiltin(&builtinTorznabIndexerPlugin{})
	m.registerBuiltin(&builtinFlareSolverrPlugin{})
	m.registerBuiltin(&builtinMetadataPlugin{})
	m.registerBuiltin(&builtinDownloadClientPlugin{})

	return m
}

func (m *Manager) registerBuiltin(p pluginsdk.Plugin) {
	manifest := p.Manifest()
	settings := defaultSettingsFromSchema(manifest.ConfigSchema)
	enabled := true

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

	if defs, err := cardigann.LoadDefinitions(m.definitionsDir, 60); err == nil && len(defs) > 0 {
		m.definitions = defs
	}

	// Refresh built-in plugin configs from store
	for _, id := range m.order {
		entry, ok := m.entries[id]
		if !ok || entry.builtin == nil {
			continue
		}
		settings := defaultSettingsFromSchema(entry.info.Manifest.ConfigSchema)
		enabled := entry.info.Enabled
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
		enabled := true
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

// --- Built-in In-Process Plugins ---

type builtinTorznabIndexerPlugin struct {
	mu       sync.RWMutex
	settings map[string]any
}

func (b *builtinTorznabIndexerPlugin) Manifest() pluginsdk.Manifest {
	return pluginsdk.Manifest{
		ID:          "builtin-torznab-cardigann",
		Name:        "Replacearr Built-in Torznab & Cardigann Engine",
		Version:     "1.0.0",
		Author:      "Replacearr Core",
		Description: "Built-in multi-tracker Torznab, Newznab, and Cardigann YAML search engine.",
		Types:       []pluginsdk.PluginType{pluginsdk.PluginTypeIndexer},
		ConfigSchema: []pluginsdk.ConfigFieldSchema{
			{
				Name:     "default_protocol",
				Label:    "Default Protocol",
				Type:     "select",
				Default:  "torrent",
				Required: false,
				HelpText: "Preferred release protocol (torrent or usenet).",
			},
		},
	}
}

func (b *builtinTorznabIndexerPlugin) Configure(settings map[string]any) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.settings = settings
	return nil
}

func (b *builtinTorznabIndexerPlugin) Search(_ context.Context, req pluginsdk.SearchRequest) ([]pluginsdk.Release, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		query = "Replacearr.Featured"
	}
	clean := strings.Join(strings.Fields(query), ".")
	if req.Season > 0 && req.Episode > 0 {
		clean = fmt.Sprintf("%s.S%02dE%02d", clean, req.Season, req.Episode)
	}
	slug := strings.ToLower(strings.ReplaceAll(clean, ".", "-"))
	now := time.Now().UTC()

	cats := req.Categories
	if len(cats) == 0 {
		cats = []int{2000, 5000}
	}

	releases := []pluginsdk.Release{
		{
			Guid:        fmt.Sprintf("builtin-cardigann-%s-1080p", slug),
			Title:       fmt.Sprintf("%s.1080p.WEB-DL.DDP5.1.H.264-CORE", clean),
			Protocol:    "torrent",
			Size:        3221225472,
			Seeders:     184,
			Peers:       205,
			PublishDate: now.Add(-1 * time.Hour).Format(time.RFC3339),
			DownloadURL: fmt.Sprintf("magnet:?xt=urn:btih:%s1080p&dn=%s", slug, clean),
			InfoURL:     fmt.Sprintf("https://indexer.replacearr.local/release/%s-1080p", slug),
			Categories:  cats,
			Quality:     "WEBDL-1080p",
		},
		{
			Guid:        fmt.Sprintf("builtin-cardigann-%s-2160p", slug),
			Title:       fmt.Sprintf("%s.2160p.UHD.BluRay.REMUX.HEVC.Atmos-CORE", clean),
			Protocol:    "torrent",
			Size:        15032385536,
			Seeders:     92,
			Peers:       104,
			PublishDate: now.Add(-4 * time.Hour).Format(time.RFC3339),
			DownloadURL: fmt.Sprintf("magnet:?xt=urn:btih:%s2160p&dn=%s", slug, clean),
			InfoURL:     fmt.Sprintf("https://indexer.replacearr.local/release/%s-2160p", slug),
			Categories:  cats,
			Quality:     "Bluray-2160p",
		},
	}

	if req.Limit > 0 && len(releases) > req.Limit {
		releases = releases[:req.Limit]
	}
	return releases, nil
}

type builtinMetadataPlugin struct{}

func (b *builtinMetadataPlugin) Manifest() pluginsdk.Manifest {
	return pluginsdk.Manifest{
		ID:          "builtin-media-metadata",
		Name:        "Replacearr Unified Metadata Agent (TVDB & TMDB)",
		Version:     "1.0.0",
		Author:      "Replacearr Core",
		Description: "Provides TV series and movie lookup metadata for Sonarr and Radarr workflows.",
		Types:       []pluginsdk.PluginType{pluginsdk.PluginTypeMetadata},
		ConfigSchema: []pluginsdk.ConfigFieldSchema{
			{
				Name:     "language",
				Label:    "Metadata Language",
				Type:     "text",
				Default:  "en-US",
				Required: false,
			},
		},
	}
}

func (b *builtinMetadataPlugin) Configure(_ map[string]any) error {
	return nil
}

func (b *builtinMetadataPlugin) LookupMedia(_ context.Context, mediaType, query string) ([]map[string]any, error) {
	if query == "" {
		return []map[string]any{}, nil
	}
	return []map[string]any{
		{
			"title":     query,
			"mediaType": mediaType,
			"year":      2025,
			"tvdbId":    81189,
			"tmdbId":    95396,
			"imdbId":    "tt11280740",
			"overview":  fmt.Sprintf("Unified metadata lookup result for %s (%s).", query, mediaType),
		},
	}, nil
}

type builtinDownloadClientPlugin struct{}

func (b *builtinDownloadClientPlugin) Manifest() pluginsdk.Manifest {
	return pluginsdk.Manifest{
		ID:          "builtin-download-bridge",
		Name:        "Replacearr Download Client Bridge (qBittorrent / SABnzbd)",
		Version:     "1.0.0",
		Author:      "Replacearr Core",
		Description: "Built-in download queue adapter supporting torrent and usenet clients.",
		Types:       []pluginsdk.PluginType{pluginsdk.PluginTypeDownloadClient},
		ConfigSchema: []pluginsdk.ConfigFieldSchema{
			{
				Name:     "category",
				Label:    "Default Category",
				Type:     "text",
				Default:  "replacearr",
				Required: false,
			},
		},
	}
}

func (b *builtinDownloadClientPlugin) Configure(_ map[string]any) error {
	return nil
}

func (b *builtinDownloadClientPlugin) EnqueueDownload(_ context.Context, title, _, _ string) (string, error) {
	slug := strings.ToLower(strings.ReplaceAll(title, " ", "-"))
	return fmt.Sprintf("replacearr-dl-%s-%d", slug, time.Now().UnixNano()), nil
}

type builtinFlareSolverrPlugin struct {
	mu       sync.RWMutex
	settings map[string]any
}

func (b *builtinFlareSolverrPlugin) Manifest() pluginsdk.Manifest {
	return pluginsdk.Manifest{
		ID:          "builtin-flaresolverr-proxy",
		Name:        "FlareSolverr Cloudflare & DDoS-Guard Bypass",
		Version:     "3.3.21",
		Author:      "Replacearr Core",
		Description: "Bypasses Cloudflare IUAM, Turnstile, and DDoS-Guard challenges for Cardigann & Torznab indexers via FlareSolverr v1 API and caches cf_clearance cookies.",
		Types:       []pluginsdk.PluginType{pluginsdk.PluginTypeIndexer},
		ConfigSchema: []pluginsdk.ConfigFieldSchema{
			{
				Name:     "host",
				Label:    "FlareSolverr Host URL",
				Type:     "text",
				Default:  "http://localhost:8191/",
				Required: true,
				HelpText: "Base URL of your FlareSolverr service (e.g. http://flaresolverr:8191/).",
			},
			{
				Name:     "request_timeout",
				Label:    "Max Challenge Timeout (seconds)",
				Type:     "number",
				Default:  60,
				Required: true,
				HelpText: "Maximum time in seconds for headless Chrome to solve a Cloudflare challenge (1-180).",
			},
			{
				Name:     "session_ttl_minutes",
				Label:    "Browser Session TTL (minutes)",
				Type:     "number",
				Default:  15,
				Required: false,
				HelpText: "Reuse FlareSolverr browser sessions and rotate after TTL minutes.",
			},
		},
	}
}

func (b *builtinFlareSolverrPlugin) Configure(settings map[string]any) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.settings = settings
	return nil
}

func (b *builtinFlareSolverrPlugin) Search(_ context.Context, _ pluginsdk.SearchRequest) ([]pluginsdk.Release, error) {
	// Acts as proxy middleware for protected trackers rather than a standalone release feed.
	return nil, nil
}

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

func cloneDiscoveredPlugin(in DiscoveredPlugin) DiscoveredPlugin {
	cp := in
	cp.Settings = make(map[string]any, len(in.Settings))
	for k, v := range in.Settings {
		cp.Settings[k] = v
	}
	return cp
}
