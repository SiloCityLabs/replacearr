// Package deluge implements a minimal Deluge WebUI JSON-RPC client.
package deluge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
)

// Item is a torrent reported by Deluge.
type Item struct {
	Hash        string
	Name        string
	State       string
	Progress    float64
	Size        int64
	AmountLeft  int64
	EtaSeconds  int64
	SavePath    string
	ContentPath string
}

// Client talks to Deluge WebUI at /json.
type Client struct {
	jsonURL  string
	password string
	category string
	http     *http.Client

	mu     sync.Mutex
	logged bool
	reqID  int
}

// NewFromDownloadClient builds a Client from a stored download client row.
func NewFromDownloadClient(dc domain.DownloadClient) *Client {
	baseURL, err := domain.NormalizeDownloadClientBaseURL(dc.BaseURL)
	if err != nil {
		baseURL = "http://127.0.0.1:8112"
	}
	jar, _ := cookiejar.New(nil)
	return &Client{
		jsonURL:  jsonEndpoint(baseURL),
		password: firstNonEmpty(dc.Password, "deluge"),
		category: dc.Category,
		http: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
	}
}

func jsonEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return strings.TrimRight(baseURL, "/") + "/json"
	}
	path := strings.TrimSuffix(u.EscapedPath(), "/")
	switch {
	case path == "" || path == "/":
		u.Path = "/json"
	case strings.HasSuffix(path, "/json"):
		u.Path = path
	default:
		u.Path = path + "/json"
	}
	return u.String()
}

type rpcEnvelope struct {
	Method string `json:"method"`
	Params []any  `json:"params"`
	ID     int    `json:"id"`
}

type rpcReply struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
	ID int `json:"id"`
}

// Add enqueues a magnet or HTTP torrent URL and returns the infohash when available.
func (c *Client) Add(ctx context.Context, title, downloadURL, _protocol string) (string, error) {
	if err := c.ensureLogin(ctx); err != nil {
		return "", err
	}
	opts := c.addOpts()
	var method string
	var params []any
	if strings.HasPrefix(strings.ToLower(downloadURL), "magnet:") {
		method = "core.add_torrent_magnet"
		params = []any{downloadURL, opts}
	} else {
		method = "core.add_torrent_url"
		params = []any{downloadURL, opts}
	}
	hash, err := c.addTorrent(ctx, method, params)
	if err != nil {
		return "", err
	}
	if hash == "" {
		hash = strings.ToLower(extractHashFromMagnet(downloadURL))
	}
	if hash != "" {
		return hash, nil
	}
	_ = title
	return "pending", nil
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
	hash, err := c.addTorrent(ctx, "core.add_torrent_file", []any{
		filename,
		base64.StdEncoding.EncodeToString(data),
		c.addOpts(),
	})
	if err != nil {
		return "", err
	}
	if hash != "" {
		return hash, nil
	}
	_ = title
	return "pending", nil
}

func (c *Client) addOpts() map[string]any {
	opts := map[string]any{}
	if c.category != "" {
		opts["label"] = strings.ToLower(c.category)
	}
	return opts
}

func (c *Client) addTorrent(ctx context.Context, method string, params []any) (string, error) {
	var hash string
	if err := c.call(ctx, method, params, &hash); err != nil {
		return "", err
	}
	hash = strings.ToLower(strings.TrimSpace(hash))
	if hash != "" && c.category != "" {
		// Label plugin is optional; ignore failures.
		_ = c.call(ctx, "label.set_torrent", []any{hash, strings.ToLower(c.category)}, nil)
	}
	return hash, nil
}

// List returns torrents known to Deluge.
func (c *Client) List(ctx context.Context) ([]Item, error) {
	if err := c.ensureLogin(ctx); err != nil {
		return nil, err
	}
	keys := []string{
		"name", "state", "progress", "total_size", "total_done",
		"total_remaining", "eta", "save_path", "hash",
	}
	var raw map[string]struct {
		Name           string  `json:"name"`
		State          string  `json:"state"`
		Progress       float64 `json:"progress"` // 0-100 in Deluge
		TotalSize      int64   `json:"total_size"`
		TotalDone      int64   `json:"total_done"`
		TotalRemaining int64   `json:"total_remaining"`
		Eta            int64   `json:"eta"`
		SavePath       string  `json:"save_path"`
		Hash           string  `json:"hash"`
	}
	// Filter empty = all torrents.
	if err := c.call(ctx, "core.get_torrents_status", []any{map[string]any{}, keys}, &raw); err != nil {
		return nil, err
	}
	out := make([]Item, 0, len(raw))
	for id, t := range raw {
		hash := strings.ToLower(firstNonEmpty(t.Hash, id))
		progress := t.Progress
		if progress > 1.0 {
			progress = progress / 100.0
		}
		left := t.TotalRemaining
		if left == 0 && t.TotalSize > t.TotalDone {
			left = t.TotalSize - t.TotalDone
		}
		out = append(out, Item{
			Hash:        hash,
			Name:        t.Name,
			State:       t.State,
			Progress:    progress,
			Size:        t.TotalSize,
			AmountLeft:  left,
			EtaSeconds:  t.Eta,
			SavePath:    t.SavePath,
			ContentPath: t.SavePath,
		})
	}
	return out, nil
}

// Remove deletes a torrent by infohash.
func (c *Client) Remove(ctx context.Context, hash string, deleteFiles bool) error {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return nil
	}
	if err := c.ensureLogin(ctx); err != nil {
		return err
	}
	return c.call(ctx, "core.remove_torrent", []any{hash, deleteFiles}, nil)
}

// IsCompleted reports whether a Deluge torrent has finished downloading.
func IsCompleted(state string, progress float64) bool {
	switch strings.ToLower(state) {
	case "seeding", "paused", "queued", "moving":
		if progress >= 0.999 {
			return true
		}
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
	var ok bool
	if err := c.call(ctx, "auth.login", []any{c.password}, &ok); err != nil {
		return fmt.Errorf("deluge login: %w", err)
	}
	if !ok {
		return fmt.Errorf("deluge login failed (check password)")
	}
	// Ensure WebUI is connected to a daemon when hosts are configured.
	var connected bool
	if err := c.call(ctx, "web.connected", []any{}, &connected); err == nil && !connected {
		var hosts [][]any
		if err := c.call(ctx, "web.get_hosts", []any{}, &hosts); err == nil && len(hosts) > 0 {
			if id, ok := hosts[0][0].(string); ok && id != "" {
				_ = c.call(ctx, "web.connect", []any{id}, nil)
			}
		}
	}
	c.mu.Lock()
	c.logged = true
	c.mu.Unlock()
	return nil
}

func (c *Client) call(ctx context.Context, method string, params []any, out any) error {
	c.mu.Lock()
	c.reqID++
	id := c.reqID
	c.mu.Unlock()
	body, err := json.Marshal(rpcEnvelope{Method: method, Params: params, ID: id})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.jsonURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("deluge rpc: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("deluge rpc: %s (%s)", resp.Status, strings.TrimSpace(string(raw)))
	}
	var reply rpcReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		return fmt.Errorf("deluge rpc decode: %w", err)
	}
	if reply.Error != nil && reply.Error.Message != "" {
		if strings.Contains(strings.ToLower(reply.Error.Message), "not authenticated") {
			c.mu.Lock()
			c.logged = false
			c.mu.Unlock()
		}
		return fmt.Errorf("deluge rpc: %s", reply.Error.Message)
	}
	if out == nil || len(reply.Result) == 0 || string(reply.Result) == "null" {
		return nil
	}
	if err := json.Unmarshal(reply.Result, out); err != nil {
		return fmt.Errorf("deluge rpc result: %w", err)
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
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
	const prefix = "urn:btih:"
	if strings.HasPrefix(strings.ToLower(xt), prefix) {
		return xt[len(prefix):]
	}
	if i := strings.Index(strings.ToLower(u), "btih:"); i >= 0 {
		rest := u[i+5:]
		for j, r := range rest {
			if r == '&' || r == '/' {
				return rest[:j]
			}
		}
		if len(rest) >= 40 {
			return rest[:40]
		}
		return rest
	}
	return ""
}
