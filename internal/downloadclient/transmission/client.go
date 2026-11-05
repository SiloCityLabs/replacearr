// Package transmission implements a minimal Transmission RPC client for enqueue and status polling.
package transmission

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
)

// Item is a torrent reported by Transmission.
type Item struct {
	ID          int64
	Hash        string
	Name        string
	Status      int
	Progress    float64
	Size        int64
	AmountLeft  int64
	EtaSeconds  int64 // -1 unknown; -2 not available
	DownloadDir string
}

// Client talks to a Transmission daemon over RPC.
type Client struct {
	rpcURL   string
	username string
	password string
	category string
	http     *http.Client

	mu        sync.Mutex
	sessionID string
}

// NewFromDownloadClient builds a Client from a stored download client row.
func NewFromDownloadClient(dc domain.DownloadClient) *Client {
	baseURL, err := domain.NormalizeDownloadClientBaseURL(dc.BaseURL)
	if err != nil {
		baseURL = "http://127.0.0.1:9091"
	}
	return &Client{
		rpcURL:   rpcEndpoint(baseURL),
		username: dc.Username,
		password: dc.Password,
		category: dc.Category,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// rpcEndpoint appends the standard Transmission RPC path when the stored
// base URL is only scheme+host(+port), matching Sonarr/Radarr defaults.
func rpcEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return strings.TrimRight(baseURL, "/") + "/transmission/rpc"
	}
	path := strings.TrimSuffix(u.EscapedPath(), "/")
	switch {
	case path == "" || path == "/":
		u.Path = "/transmission/rpc"
	case strings.HasSuffix(path, "/rpc"):
		u.Path = path
	case strings.HasSuffix(path, "/transmission"):
		u.Path = path + "/rpc"
	default:
		u.Path = path + "/transmission/rpc"
	}
	return u.String()
}

type rpcRequest struct {
	Method    string `json:"method"`
	Arguments any    `json:"arguments,omitempty"`
	Tag       int    `json:"tag,omitempty"`
}

type rpcResponse struct {
	Result    string          `json:"result"`
	Arguments json.RawMessage `json:"arguments"`
}

// Add enqueues a magnet or torrent URL and returns the infohash when available.
func (c *Client) Add(ctx context.Context, title, downloadURL, _protocol string) (string, error) {
	args := map[string]any{
		"filename": downloadURL,
	}
	hash, err := c.torrentAdd(ctx, args)
	if err != nil {
		return "", err
	}
	if hash != "" {
		return hash, nil
	}
	if h := extractHashFromMagnet(downloadURL); h != "" {
		return strings.ToLower(h), nil
	}
	_ = title
	return "pending", nil
}

// AddFile enqueues a raw .torrent file and returns the infohash when available.
func (c *Client) AddFile(ctx context.Context, title, filename string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty torrent file")
	}
	args := map[string]any{
		"metainfo": base64.StdEncoding.EncodeToString(data),
	}
	hash, err := c.torrentAdd(ctx, args)
	if err != nil {
		return "", err
	}
	if hash != "" {
		return hash, nil
	}
	_ = title
	_ = filename
	return "pending", nil
}

func (c *Client) torrentAdd(ctx context.Context, args map[string]any) (string, error) {
	if c.category != "" {
		// Transmission 3+/4 labels; ignored by older daemons.
		args["labels"] = []string{c.category}
	}
	var out struct {
		Added struct {
			HashString string `json:"hashString"`
			Name       string `json:"name"`
			ID         int64  `json:"id"`
		} `json:"torrent-added"`
		Duplicate struct {
			HashString string `json:"hashString"`
			Name       string `json:"name"`
			ID         int64  `json:"id"`
		} `json:"torrent-duplicate"`
	}
	if err := c.call(ctx, "torrent-add", args, &out); err != nil {
		return "", err
	}
	hash := strings.ToLower(strings.TrimSpace(out.Added.HashString))
	if hash == "" {
		hash = strings.ToLower(strings.TrimSpace(out.Duplicate.HashString))
	}
	return hash, nil
}

// List returns torrents known to Transmission.
func (c *Client) List(ctx context.Context) ([]Item, error) {
	args := map[string]any{
		"fields": []string{
			"id", "hashString", "name", "status", "totalSize",
			"leftUntilDone", "percentDone", "eta", "downloadDir",
		},
	}
	var out struct {
		Torrents []struct {
			ID            int64   `json:"id"`
			HashString    string  `json:"hashString"`
			Name          string  `json:"name"`
			Status        int     `json:"status"`
			TotalSize     int64   `json:"totalSize"`
			LeftUntilDone int64   `json:"leftUntilDone"`
			PercentDone   float64 `json:"percentDone"`
			Eta           int64   `json:"eta"`
			DownloadDir   string  `json:"downloadDir"`
		} `json:"torrents"`
	}
	if err := c.call(ctx, "torrent-get", args, &out); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(out.Torrents))
	for _, t := range out.Torrents {
		items = append(items, Item{
			ID:          t.ID,
			Hash:        strings.ToLower(t.HashString),
			Name:        t.Name,
			Status:      t.Status,
			Progress:    t.PercentDone,
			Size:        t.TotalSize,
			AmountLeft:  t.LeftUntilDone,
			EtaSeconds:  t.Eta,
			DownloadDir: t.DownloadDir,
		})
	}
	return items, nil
}

// Remove deletes a torrent by infohash (or numeric id string).
func (c *Client) Remove(ctx context.Context, id string, deleteFiles bool) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	args := map[string]any{
		"delete-local-data": deleteFiles,
	}
	if n, err := parseInt64(id); err == nil {
		args["ids"] = []int64{n}
	} else {
		args["ids"] = []string{id}
	}
	return c.call(ctx, "torrent-remove", args, nil)
}

// IsCompleted reports whether a Transmission torrent has finished downloading.
func IsCompleted(status int, progress float64, left int64) bool {
	if left == 0 && progress >= 0.999 {
		return true
	}
	// 5 = seed wait, 6 = seed; 0 = stopped (often complete).
	switch status {
	case 5, 6:
		return true
	case 0:
		return progress >= 0.999 || left == 0
	}
	return false
}

func (c *Client) call(ctx context.Context, method string, arguments any, out any) error {
	body, err := json.Marshal(rpcRequest{Method: method, Arguments: arguments})
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.rpcURL, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		c.mu.Lock()
		sid := c.sessionID
		c.mu.Unlock()
		if sid != "" {
			req.Header.Set("X-Transmission-Session-Id", sid)
		}
		if c.username != "" || c.password != "" {
			req.SetBasicAuth(c.username, c.password)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("transmission rpc: %w", err)
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusConflict {
			next := resp.Header.Get("X-Transmission-Session-Id")
			if next == "" {
				return fmt.Errorf("transmission rpc: 409 without session id")
			}
			c.mu.Lock()
			c.sessionID = next
			c.mu.Unlock()
			continue
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("transmission rpc: unauthorized (check username/password)")
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("transmission rpc: %s (%s)", resp.Status, strings.TrimSpace(string(raw)))
		}
		var rpcResp rpcResponse
		if err := json.Unmarshal(raw, &rpcResp); err != nil {
			return fmt.Errorf("transmission rpc decode: %w", err)
		}
		if !strings.EqualFold(rpcResp.Result, "success") {
			msg := strings.TrimSpace(rpcResp.Result)
			if msg == "" {
				msg = strings.TrimSpace(string(raw))
			}
			return fmt.Errorf("transmission rpc: %s", msg)
		}
		if out == nil || len(rpcResp.Arguments) == 0 || string(rpcResp.Arguments) == "null" {
			return nil
		}
		if err := json.Unmarshal(rpcResp.Arguments, out); err != nil {
			return fmt.Errorf("transmission rpc args: %w", err)
		}
		return nil
	}
	return fmt.Errorf("transmission rpc: session id retry exhausted")
}

func parseInt64(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscan(s, &n)
	return n, err
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
	const prefix = "urn:btih:"
	if strings.HasPrefix(strings.ToLower(xt), prefix) {
		return xt[len(prefix):]
	}
	return ""
}
