// Package plugincatalog fetches the Replacearr plugin marketplace index and
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
	DefaultCatalogURL = "https://raw.githubusercontent.com/SiloCityLabs/replacearr-plugins/main/index.json"
	SchemaVersion     = 1
	defaultCacheTTL   = 10 * time.Minute
	maxAssetBytes     = 256 << 20 // 256 MiB
)

// Asset is a platform-specific downloadable plugin binary.
type Asset struct {
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Plugin is one marketplace catalog entry.
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

// Index is the top-level marketplace document.
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

	mu        sync.Mutex
	cached    *Index
	cachedAt  time.Time
	cacheErr  error
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

// CatalogURL returns the configured index URL.
func (c *Client) CatalogURL() string {
	if c == nil {
		return DefaultCatalogURL
	}
	return c.catalogURL
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

	idx, err := c.downloadIndex(ctx)
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

// List merges the catalog with installed plugins for the API/UI.
func (c *Client) List(ctx context.Context, installed []InstalledInfo, force bool) ([]CatalogEntry, error) {
	idx, err := c.Fetch(ctx, force)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]InstalledInfo, len(installed))
	for _, p := range installed {
		byID[p.ID] = p
	}
	out := make([]CatalogEntry, 0, len(idx.Plugins))
	for _, p := range idx.Plugins {
		entry := CatalogEntry{Plugin: p}
		_, entry.HasPlatformAsset = SelectAsset(p, runtime.GOOS, runtime.GOARCH)
		if info, ok := byID[p.ID]; ok && !info.Builtin {
			entry.Installed = true
			entry.InstalledVersion = info.Version
			entry.UpdateAvailable = versionGreater(p.Version, info.Version)
		}
		out = append(out, entry)
	}
	return out, nil
}

// Install downloads the platform asset for pluginID into pluginDir as an executable named after the id.
func (c *Client) Install(ctx context.Context, pluginID string) error {
	if c == nil {
		return errors.New("plugincatalog: nil client")
	}
	pluginID = strings.TrimSpace(pluginID)
	if err := validatePluginID(pluginID); err != nil {
		return err
	}
	if strings.TrimSpace(c.pluginDir) == "" {
		return errors.New("plugin directory is not configured")
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
	asset, ok := SelectAsset(*plugin, runtime.GOOS, runtime.GOARCH)
	if !ok {
		return fmt.Errorf("no asset for %s/%s in plugin %q", runtime.GOOS, runtime.GOARCH, pluginID)
	}
	if strings.TrimSpace(asset.SHA256) == "" {
		return fmt.Errorf("plugin %q asset is missing sha256", pluginID)
	}
	if err := requireHTTPS(asset.URL); err != nil {
		return err
	}

	if err := os.MkdirAll(c.pluginDir, 0o755); err != nil {
		return fmt.Errorf("create plugin dir: %w", err)
	}

	tmp, err := os.CreateTemp(c.pluginDir, "."+pluginID+".download-*")
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
		limit = asset.Size + (1 << 20) // small slack over declared size
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

	dest := BinaryPath(c.pluginDir, pluginID)
	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}
	return nil
}

// Uninstall removes the installed binary for pluginID from pluginDir.
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

func (c *Client) downloadIndex(ctx context.Context) (*Index, error) {
	if err := requireHTTPS(c.catalogURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.catalogURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch catalog: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch catalog: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(body, &idx); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	if idx.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("unsupported catalog schemaVersion %d (want %d)", idx.SchemaVersion, SchemaVersion)
	}
	for _, p := range idx.Plugins {
		if err := validatePluginID(p.ID); err != nil {
			return nil, fmt.Errorf("catalog entry: %w", err)
		}
	}
	return &idx, nil
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
