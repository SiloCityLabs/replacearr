// Package plugincatalog fetches the Replacearr plugin marketplace registry and
// installs checksummed release binaries into REPLACEARR_PLUGIN_DIR.
package plugincatalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	DefaultCatalogURL = "https://raw.githubusercontent.com/replacearr/replacearr-plugins/main/registry.json"
	SchemaVersion     = 2
	LegacySchemaV1    = 1
	defaultCacheTTL   = 10 * time.Minute
	maxAssetBytes     = 256 << 20 // 256 MiB
	maxJSONBytes      = 8 << 20
)

// Asset is a platform-specific downloadable plugin binary.
type Asset struct {
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Plugin is one marketplace catalog entry (pinned install manifest).
type Plugin struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Version              string   `json:"version"`
	Author               string   `json:"author"`
	Description          string   `json:"description"`
	Types                []string `json:"types"`
	Homepage             string   `json:"homepage"`
	Repository           string   `json:"repository"`
	MinReplacearrVersion string   `json:"minReplacearrVersion"`
	Assets               []Asset  `json:"assets"`
}

// RegistryEntry is one curated row in registry.json (schema v2).
type RegistryEntry struct {
	ID       string `json:"id"`
	Manifest string `json:"manifest"`
}

// Registry is the top-level curated marketplace document (schema v2).
type Registry struct {
	SchemaVersion int             `json:"schemaVersion"`
	Plugins       []RegistryEntry `json:"plugins"`
}

// Index is an assembled in-memory catalog (registry manifests and/or legacy v1).
type Index struct {
	SchemaVersion int      `json:"schemaVersion"`
	UpdatedAt     string   `json:"updatedAt"`
	Plugins       []Plugin `json:"plugins"`
}

// CatalogEntry is an index plugin enriched with install status for the UI.
type CatalogEntry struct {
	Plugin
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installedVersion,omitempty"`
	UpdateAvailable  bool   `json:"updateAvailable"`
	HasPlatformAsset bool   `json:"hasPlatformAsset"`
	Source           string `json:"source,omitempty"`
	RepoURL          string `json:"repoUrl,omitempty"`
	ManifestURL      string `json:"manifestUrl,omitempty"`
}

// InstalledInfo describes a locally discovered external plugin.
type InstalledInfo struct {
	ID         string
	Version    string
	BinaryPath string
	Builtin    bool
}

// Client fetches and installs marketplace plugins.
type Client struct {
	catalogURL string
	pluginDir  string
	httpClient *http.Client
	cacheTTL   time.Duration

	mu           sync.Mutex
	cached       *Index
	cachedAt     time.Time
	cacheErr     error
	manifestURLs map[string]string // plugin id → resolved curated manifest URL
}

// New creates a catalog client. catalogURL empty uses DefaultCatalogURL.
func New(catalogURL, pluginDir string) *Client {
	u := strings.TrimSpace(catalogURL)
	if u == "" {
		u = DefaultCatalogURL
	}
	return &Client{
		catalogURL: u,
		pluginDir:  pluginDir,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		cacheTTL:   defaultCacheTTL,
	}
}

// CatalogURL returns the configured registry URL.
func (c *Client) CatalogURL() string {
	if c == nil {
		return DefaultCatalogURL
	}
	return c.catalogURL
}

// PluginDir returns the install directory.
func (c *Client) PluginDir() string {
	if c == nil {
		return ""
	}
	return c.pluginDir
}

// Fetch returns the marketplace index, using a short in-memory cache unless force is set.
func (c *Client) Fetch(ctx context.Context, force bool) (*Index, error) {
	if c == nil {
		return nil, errors.New("plugincatalog: nil client")
	}
	c.mu.Lock()
	if !force && c.cached != nil && time.Since(c.cachedAt) < c.cacheTTL {
		idx := c.cached
		c.mu.Unlock()
		return idx, nil
	}
	c.mu.Unlock()

	idx, err := c.downloadCatalog(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.cacheErr = err
		if c.cached != nil && !force {
			return c.cached, nil
		}
		return nil, err
	}
	c.cached = idx
	c.cachedAt = time.Now()
	c.cacheErr = nil
	// manifestURLs populated inside downloadCatalog/assembleRegistry under same lock path —
	// downloadCatalog runs unlocked; merge map under lock here if returned.
	return idx, nil
}

// Invalidate clears the cached index.
func (c *Client) Invalidate() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cached = nil
	c.cachedAt = time.Time{}
}

// List merges the catalog with installed plugins (including origin-only URL installs).
func (c *Client) List(ctx context.Context, installed []InstalledInfo, force bool) ([]CatalogEntry, error) {
	idx, err := c.Fetch(ctx, force)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]InstalledInfo, len(installed))
	for _, p := range installed {
		byID[p.ID] = p
	}
	seen := make(map[string]struct{}, len(idx.Plugins))
	out := make([]CatalogEntry, 0, len(idx.Plugins)+len(installed))

	c.mu.Lock()
	manifestByID := make(map[string]string, len(c.manifestURLs))
	for k, v := range c.manifestURLs {
		manifestByID[k] = v
	}
	c.mu.Unlock()

	for _, p := range idx.Plugins {
		seen[p.ID] = struct{}{}
		entry := CatalogEntry{Plugin: p, Source: OriginRegistry}
		_, entry.HasPlatformAsset = SelectAsset(p, runtime.GOOS, runtime.GOARCH)
		if origin, ok := ReadOrigin(c.pluginDir, p.ID); ok {
			entry.RepoURL = origin.RepoURL
			entry.ManifestURL = origin.ManifestURL
			entry.Source = origin.Source
		} else if u, ok := manifestByID[p.ID]; ok {
			entry.ManifestURL = u
		}
		if info, ok := byID[p.ID]; ok && !info.Builtin {
			entry.Installed = true
			entry.InstalledVersion = info.Version
			entry.UpdateAvailable = versionGreater(p.Version, info.Version)
		}
		out = append(out, entry)
	}

	// Origin-only installs (pasted GitHub URL, not in curated registry).
	origins, _ := ListOrigins(c.pluginDir)
	for _, origin := range origins {
		if _, ok := seen[origin.ID]; ok {
			continue
		}
		info, installedOK := byID[origin.ID]
		if !installedOK || info.Builtin {
			continue
		}
		plugin, err := c.fetchPluginManifest(ctx, origin.ManifestURL)
		if err != nil {
			// Still surface a stub so the UI can uninstall / show the repo URL.
			out = append(out, CatalogEntry{
				Plugin: Plugin{
					ID:          origin.ID,
					Name:        origin.ID,
					Version:     origin.InstalledVersion,
					Description: "Installed from " + origin.RepoURL,
				},
				Installed:        true,
				InstalledVersion: info.Version,
				Source:           origin.Source,
				RepoURL:          origin.RepoURL,
				ManifestURL:      origin.ManifestURL,
			})
			continue
		}
		entry := CatalogEntry{
			Plugin:           *plugin,
			Installed:        true,
			InstalledVersion: info.Version,
			Source:           origin.Source,
			RepoURL:          origin.RepoURL,
			ManifestURL:      origin.ManifestURL,
		}
		_, entry.HasPlatformAsset = SelectAsset(*plugin, runtime.GOOS, runtime.GOARCH)
		entry.UpdateAvailable = versionGreater(plugin.Version, info.Version)
		out = append(out, entry)
		seen[origin.ID] = struct{}{}
	}
	return out, nil
}

// Install downloads the platform asset for a curated registry pluginID.
func (c *Client) Install(ctx context.Context, pluginID string) error {
	if c == nil {
		return errors.New("plugincatalog: nil client")
	}
	pluginID = strings.TrimSpace(pluginID)
	if err := validatePluginID(pluginID); err != nil {
		return err
	}
	idx, err := c.Fetch(ctx, false)
	if err != nil {
		return err
	}
	var plugin *Plugin
	for i := range idx.Plugins {
		if idx.Plugins[i].ID == pluginID {
			plugin = &idx.Plugins[i]
			break
		}
	}
	if plugin == nil {
		return fmt.Errorf("plugin %q not found in catalog", pluginID)
	}
	manifestURL := c.resolvedManifestURL(pluginID)
	return c.installPlugin(ctx, *plugin, Origin{
		ID:               plugin.ID,
		Source:           OriginRegistry,
		RepoURL:          strings.TrimSpace(plugin.Repository),
		ManifestURL:      manifestURL,
		InstalledVersion: plugin.Version,
		InstalledAt:      time.Now().UTC().Format(time.RFC3339),
	})
}

// InstallFromURL resolves a GitHub plugin repo URL and installs it.
func (c *Client) InstallFromURL(ctx context.Context, rawURL string) (*Plugin, error) {
	if c == nil {
		return nil, errors.New("plugincatalog: nil client")
	}
	resolved, err := c.ResolveGitHub(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	if err := c.installPlugin(ctx, resolved.Plugin, Origin{
		ID:               resolved.Plugin.ID,
		Source:           OriginGitHub,
		RepoURL:          resolved.RepoURL,
		ManifestURL:      resolved.ManifestURL,
		InstalledVersion: resolved.Plugin.Version,
		InstalledAt:      time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return nil, err
	}
	return &resolved.Plugin, nil
}

// ResolveGitHub preview-resolves a GitHub repo URL without installing.
func (c *Client) ResolveGitHub(ctx context.Context, rawURL string) (*ResolvedGitHub, error) {
	if c == nil {
		return nil, errors.New("plugincatalog: nil client")
	}
	owner, repo, tag, err := ParseGitHubPluginURL(rawURL)
	if err != nil {
		return nil, err
	}
	repoURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	var manifestURL string
	if tag != "" {
		manifestURL = fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/plugin.json", owner, repo, tag)
	} else {
		manifestURL = fmt.Sprintf("https://github.com/%s/%s/releases/latest/download/plugin.json", owner, repo)
	}
	plugin, finalURL, err := c.fetchPluginManifestPinned(ctx, manifestURL)
	if err != nil {
		return nil, err
	}
	return &ResolvedGitHub{
		Plugin:      *plugin,
		RepoURL:     repoURL,
		ManifestURL: finalURL,
	}, nil
}

// ResolvedGitHub is a preview of a GitHub plugin repo.
type ResolvedGitHub struct {
	Plugin      Plugin
	RepoURL     string
	ManifestURL string
}

// Uninstall removes the installed binary and origin sidecar for pluginID.
func (c *Client) Uninstall(pluginID string) error {
	if c == nil {
		return errors.New("plugincatalog: nil client")
	}
	if err := validatePluginID(pluginID); err != nil {
		return err
	}
	dest := BinaryPath(c.pluginDir, pluginID)
	if err := os.Remove(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_ = RemoveOrigin(c.pluginDir, pluginID)
	return nil
}

// BinaryPath returns the on-disk path for an installed plugin binary.
func BinaryPath(pluginDir, pluginID string) string {
	name := pluginID
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(pluginDir, name)
}

// SelectAsset picks the asset matching goos/goarch.
func SelectAsset(p Plugin, goos, goarch string) (Asset, bool) {
	for _, a := range p.Assets {
		if a.GOOS == goos && a.GOARCH == goarch {
			return a, true
		}
	}
	return Asset{}, false
}

func (c *Client) installPlugin(ctx context.Context, plugin Plugin, origin Origin) error {
	if strings.TrimSpace(c.pluginDir) == "" {
		return errors.New("plugin directory is not configured")
	}
	if err := validatePluginID(plugin.ID); err != nil {
		return err
	}
	asset, ok := SelectAsset(plugin, runtime.GOOS, runtime.GOARCH)
	if !ok {
		return fmt.Errorf("no asset for %s/%s in plugin %q", runtime.GOOS, runtime.GOARCH, plugin.ID)
	}
	if strings.TrimSpace(asset.SHA256) == "" {
		return fmt.Errorf("plugin %q asset is missing sha256", plugin.ID)
	}
	if err := requireHTTPS(asset.URL); err != nil {
		return err
	}

	if err := os.MkdirAll(c.pluginDir, 0o755); err != nil {
		return fmt.Errorf("create plugin dir: %w", err)
	}

	tmp, err := os.CreateTemp(c.pluginDir, "."+plugin.ID+".download-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download asset: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download asset: HTTP %d", resp.StatusCode)
	}

	limit := int64(maxAssetBytes)
	if asset.Size > 0 && asset.Size < int64(maxAssetBytes) {
		limit = asset.Size + (1 << 20)
	}
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, hasher), io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return fmt.Errorf("write asset: %w", err)
	}
	if written > limit {
		return fmt.Errorf("asset exceeds size limit")
	}
	if asset.Size > 0 && written != asset.Size {
		return fmt.Errorf("asset size mismatch: got %d want %d", written, asset.Size)
	}
	sum := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(sum, asset.SHA256) {
		return fmt.Errorf("sha256 mismatch: got %s want %s", sum, asset.SHA256)
	}
	if err := tmp.Chmod(0o755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	dest := BinaryPath(c.pluginDir, plugin.ID)
	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}
	origin.ID = plugin.ID
	origin.InstalledVersion = plugin.Version
	if origin.InstalledAt == "" {
		origin.InstalledAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := WriteOrigin(c.pluginDir, origin); err != nil {
		return err
	}
	return nil
}

func (c *Client) downloadCatalog(ctx context.Context) (*Index, error) {
	if err := requireHTTPS(c.catalogURL); err != nil {
		return nil, err
	}
	body, err := c.getBytes(ctx, c.catalogURL)
	if err != nil {
		return nil, fmt.Errorf("fetch catalog: %w", err)
	}

	var probe struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}

	switch probe.SchemaVersion {
	case LegacySchemaV1:
		var idx Index
		if err := json.Unmarshal(body, &idx); err != nil {
			return nil, fmt.Errorf("parse catalog v1: %w", err)
		}
		for _, p := range idx.Plugins {
			if err := validatePluginID(p.ID); err != nil {
				return nil, fmt.Errorf("catalog entry: %w", err)
			}
		}
		return &idx, nil
	case SchemaVersion:
		var reg Registry
		if err := json.Unmarshal(body, &reg); err != nil {
			return nil, fmt.Errorf("parse registry: %w", err)
		}
		return c.assembleRegistry(ctx, reg)
	default:
		return nil, fmt.Errorf("unsupported catalog schemaVersion %d (want %d or %d)", probe.SchemaVersion, SchemaVersion, LegacySchemaV1)
	}
}

func (c *Client) assembleRegistry(ctx context.Context, reg Registry) (*Index, error) {
	idx := &Index{
		SchemaVersion: SchemaVersion,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
		Plugins:       make([]Plugin, 0, len(reg.Plugins)),
	}
	type result struct {
		plugin      Plugin
		manifestURL string
		err         error
		id          string
	}
	ch := make(chan result, len(reg.Plugins))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, entry := range reg.Plugins {
		entry := entry
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := validatePluginID(entry.ID); err != nil {
				ch <- result{id: entry.ID, err: err}
				return
			}
			manifestURL, err := resolveManifestRef(c.catalogURL, entry.Manifest)
			if err != nil {
				ch <- result{id: entry.ID, err: err}
				return
			}
			plugin, err := c.fetchPluginManifest(ctx, manifestURL)
			if err != nil {
				ch <- result{id: entry.ID, err: fmt.Errorf("%s: %w", entry.ID, err)}
				return
			}
			if plugin.ID != entry.ID {
				ch <- result{id: entry.ID, err: fmt.Errorf("%s: manifest id %q mismatch", entry.ID, plugin.ID)}
				return
			}
			ch <- result{id: entry.ID, plugin: *plugin, manifestURL: manifestURL}
		}()
	}
	wg.Wait()
	close(ch)
	urls := make(map[string]string, len(reg.Plugins))
	var firstErr error
	for r := range ch {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		idx.Plugins = append(idx.Plugins, r.plugin)
		urls[r.id] = r.manifestURL
	}
	c.mu.Lock()
	c.manifestURLs = urls
	c.mu.Unlock()
	if firstErr != nil {
		return nil, firstErr
	}
	return idx, nil
}

func (c *Client) fetchPluginManifest(ctx context.Context, manifestURL string) (*Plugin, error) {
	p, _, err := c.fetchPluginManifestPinned(ctx, manifestURL)
	return p, err
}

func (c *Client) fetchPluginManifestPinned(ctx context.Context, manifestURL string) (*Plugin, string, error) {
	if err := requireHTTPS(manifestURL); err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetch manifest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("fetch manifest: HTTP %d", resp.StatusCode)
	}
	finalURL := manifestURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	// Prefer a tagged release URL over /releases/latest/ when redirects resolve.
	finalURL = preferPinnedReleaseURL(finalURL, manifestURL)
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxJSONBytes))
	if err != nil {
		return nil, "", err
	}
	var plugin Plugin
	if err := json.Unmarshal(body, &plugin); err != nil {
		return nil, "", fmt.Errorf("parse manifest: %w", err)
	}
	if err := validatePluginID(plugin.ID); err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(plugin.Version) == "" {
		return nil, "", errors.New("manifest missing version")
	}
	if len(plugin.Assets) == 0 {
		return nil, "", errors.New("manifest missing assets")
	}
	return &plugin, finalURL, nil
}

func (c *Client) getBytes(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxJSONBytes))
}

func (c *Client) resolvedManifestURL(pluginID string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.manifestURLs != nil {
		if u := c.manifestURLs[pluginID]; u != "" {
			return u
		}
	}
	return ""
}

func resolveManifestRef(catalogURL, manifest string) (string, error) {
	manifest = strings.TrimSpace(manifest)
	if manifest == "" {
		return "", errors.New("empty manifest path")
	}
	base, err := url.Parse(catalogURL)
	if err != nil {
		return "", fmt.Errorf("invalid catalog url: %w", err)
	}
	if strings.HasPrefix(manifest, "https://") || strings.HasPrefix(manifest, "http://") {
		abs, err := url.Parse(manifest)
		if err != nil {
			return "", err
		}
		if abs.Scheme != "https" {
			return "", fmt.Errorf("refusing non-HTTPS manifest %q", manifest)
		}
		// Curated registry must stay same-origin (host + first path segment for raw.githubusercontent.com).
		if !sameRegistryOrigin(base, abs) {
			return "", fmt.Errorf("curated manifest must be same-origin as registry (got %q)", manifest)
		}
		return abs.String(), nil
	}
	if strings.Contains(manifest, "..") || strings.HasPrefix(manifest, "/") {
		return "", fmt.Errorf("invalid manifest path %q", manifest)
	}
	ref, err := url.Parse(manifest)
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(ref)
	if resolved.Scheme != "https" {
		return "", fmt.Errorf("refusing non-HTTPS manifest %q", resolved.String())
	}
	return resolved.String(), nil
}

func sameRegistryOrigin(base, abs *url.URL) bool {
	if !strings.EqualFold(base.Host, abs.Host) {
		return false
	}
	// For raw.githubusercontent.com/owner/repo/… keep owner/repo prefix aligned.
	baseParts := strings.Split(strings.Trim(base.Path, "/"), "/")
	absParts := strings.Split(strings.Trim(abs.Path, "/"), "/")
	if strings.EqualFold(base.Host, "raw.githubusercontent.com") {
		if len(baseParts) < 2 || len(absParts) < 2 {
			return false
		}
		return baseParts[0] == absParts[0] && baseParts[1] == absParts[1]
	}
	return true
}

func preferPinnedReleaseURL(finalURL, requested string) string {
	finalURL = strings.TrimSpace(finalURL)
	if finalURL == "" {
		return requested
	}
	// GitHub redirects /releases/latest/download/X → /releases/download/vX.Y.Z/X
	if strings.Contains(requested, "/releases/latest/") && strings.Contains(finalURL, "/releases/download/") {
		return finalURL
	}
	return finalURL
}

func requireHTTPS(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("refusing non-HTTPS url %q", raw)
	}
	if u.Host == "" {
		return fmt.Errorf("invalid url host in %q", raw)
	}
	return nil
}

func validatePluginID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("empty plugin id")
	}
	if strings.HasPrefix(id, "builtin-") {
		return fmt.Errorf("refusing builtin plugin id %q", id)
	}
	if strings.Contains(id, "/") || strings.Contains(id, "\\") || strings.Contains(id, "..") {
		return fmt.Errorf("invalid plugin id %q", id)
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return fmt.Errorf("invalid plugin id %q", id)
	}
	return nil
}

// versionGreater reports whether a is strictly greater than b using dotted numeric segments.
func versionGreater(a, b string) bool {
	as := versionParts(a)
	bs := versionParts(b)
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}
		if av > bv {
			return true
		}
		if av < bv {
			return false
		}
	}
	return false
}

func versionParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n := 0
		for _, r := range p {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
		}
		out = append(out, n)
	}
	return out
}
