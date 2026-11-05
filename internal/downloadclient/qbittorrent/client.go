// Package qbittorrent implements a minimal qBittorrent WebAPI v2 client for enqueue and status polling.
package qbittorrent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
)

// Item is a torrent reported by qBittorrent.
type Item struct {
	Hash        string
	Name        string
	State       string
	Progress    float64
	Size        int64
	AmountLeft  int64
	EtaSeconds  int64 // 8640000 means unknown/infinite
	SavePath    string
	ContentPath string
	Category    string
}

// Client talks to a qBittorrent instance.
type Client struct {
	baseURL  string
	username string
	password string
	category string
	http     *http.Client

	mu     sync.Mutex
	logged bool
}

// NewFromDownloadClient builds a Client from a stored download client row.
func NewFromDownloadClient(dc domain.DownloadClient) *Client {
	baseURL, err := domain.NormalizeDownloadClientBaseURL(dc.BaseURL)
	if err != nil {
		baseURL = "http://127.0.0.1:8080"
	}
	jar, _ := cookiejar.New(nil)
	return &Client{
		baseURL:  baseURL,
		username: dc.Username,
		password: dc.Password,
		category: dc.Category,
		http: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
	}
}

func (c *Client) endpoint(path string) string {
	return strings.TrimRight(c.baseURL, "/") + path
}

// Add enqueues a magnet or HTTP torrent URL and returns the infohash when available.
func (c *Client) Add(ctx context.Context, title, downloadURL, _protocol string) (string, error) {
	if err := c.ensureLogin(ctx); err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("urls", downloadURL)
	if c.category != "" {
		form.Set("category", c.category)
	}
	if title != "" {
		form.Set("rename", title)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/api/v2/torrents/add"), strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("qBittorrent add failed: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	// Best-effort: find the torrent we just added by name or wait briefly and match URL fragment.
	hash := extractHashFromMagnet(downloadURL)
	if hash != "" {
		return strings.ToLower(hash), nil
	}
	return c.resolveAddedHash(ctx, title)
}

// AddFile enqueues a raw .torrent file and returns the infohash when available.
func (c *Client) AddFile(ctx context.Context, title, filename string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty torrent file")
	}
	if err := c.ensureLogin(ctx); err != nil {
		return "", err
	}
	if strings.TrimSpace(filename) == "" {
		filename = "manual.torrent"
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".torrent") {
		filename += ".torrent"
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("torrents", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if c.category != "" {
		if err := mw.WriteField("category", c.category); err != nil {
			return "", err
		}
	}
	if title != "" {
		if err := mw.WriteField("rename", title); err != nil {
			return "", err
		}
	}
	if err := mw.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/api/v2/torrents/add"), &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("qBittorrent add file failed: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}
	return c.resolveAddedHash(ctx, title)
}

func (c *Client) resolveAddedHash(ctx context.Context, title string) (string, error) {
	time.Sleep(400 * time.Millisecond)
	items, err := c.List(ctx)
	if err != nil {
		return "pending", nil
	}
	want := strings.ToLower(strings.TrimSpace(title))
	for _, it := range items {
		if want != "" && strings.EqualFold(it.Name, title) {
			return it.Hash, nil
		}
	}
	if len(items) > 0 {
		return items[0].Hash, nil
	}
	return "pending", nil
}

// List returns all torrents.
func (c *Client) List(ctx context.Context) ([]Item, error) {
	if err := c.ensureLogin(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint("/api/v2/torrents/info"), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		c.mu.Lock()
		c.logged = false
		c.mu.Unlock()
		if err := c.ensureLogin(ctx); err != nil {
			return nil, err
		}
		return c.List(ctx)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("qBittorrent list failed: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}
	var raw []struct {
		Hash        string  `json:"hash"`
		Name        string  `json:"name"`
		State       string  `json:"state"`
		Progress    float64 `json:"progress"`
		Size        int64   `json:"size"`
		AmountLeft  int64   `json:"amount_left"`
		Eta         int64   `json:"eta"`
		SavePath    string  `json:"save_path"`
		ContentPath string  `json:"content_path"`
		Category    string  `json:"category"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	out := make([]Item, 0, len(raw))
	for _, r := range raw {
		out = append(out, Item{
			Hash:        strings.ToLower(r.Hash),
			Name:        r.Name,
			State:       r.State,
			Progress:    r.Progress,
			Size:        r.Size,
			AmountLeft:  r.AmountLeft,
			EtaSeconds:  r.Eta,
			SavePath:    r.SavePath,
			ContentPath: r.ContentPath,
			Category:    r.Category,
		})
	}
	return out, nil
}

// Remove deletes a torrent from qBittorrent.
func (c *Client) Remove(ctx context.Context, hash string, deleteFiles bool) error {
	if strings.TrimSpace(hash) == "" {
		return nil
	}
	if err := c.ensureLogin(ctx); err != nil {
		return err
	}
	form := url.Values{}
	form.Set("hashes", hash)
	if deleteFiles {
		form.Set("deleteFiles", "true")
	} else {
		form.Set("deleteFiles", "false")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/api/v2/torrents/delete"), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("qBittorrent delete failed: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

// IsCompleted reports whether a torrent state means download finished.
func IsCompleted(state string, progress float64) bool {
	switch strings.ToLower(state) {
	case "uploading", "stalledup", "pausedup", "queuedup", "forcedup", "checkingup", "stoppedup":
		return true
	}
	return progress >= 0.999
}

func (c *Client) ensureLogin(ctx context.Context) error {
	c.mu.Lock()
	logged := c.logged
	c.mu.Unlock()
	if logged {
		return nil
	}
	form := url.Values{}
	form.Set("username", c.username)
	form.Set("password", c.password)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/api/v2/auth/login"), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("qBittorrent login: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !strings.Contains(strings.ToLower(string(body)), "ok") {
		return fmt.Errorf("qBittorrent login failed: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}
	c.mu.Lock()
	c.logged = true
	c.mu.Unlock()
	return nil
}

func extractHashFromMagnet(u string) string {
	if !strings.HasPrefix(strings.ToLower(u), "magnet:") {
		return ""
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return ""
	}
	xt := parsed.Query().Get("xt")
	if xt == "" {
		// Some magnets put xt in the opaque part
		if i := strings.Index(strings.ToLower(u), "btih:"); i >= 0 {
			rest := u[i+5:]
			for j, r := range rest {
				if r == '&' || r == '/' {
					return rest[:j]
				}
				_ = j
			}
			if len(rest) >= 40 {
				return rest[:40]
			}
			return rest
		}
		return ""
	}
	const prefix = "urn:btih:"
	if strings.HasPrefix(strings.ToLower(xt), prefix) {
		return xt[len(prefix):]
	}
	return ""
}
