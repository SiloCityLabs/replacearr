// Package sabnzbd implements a minimal SABnzbd HTTP API client.
package sabnzbd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
)

// Item is a queue or history slot from SABnzbd.
type Item struct {
	ID         string
	Title      string
	Status     string
	Progress   float64
	Size       int64
	SizeLeft   int64
	Timeleft   string // SABnzbd "0:12:34" style; empty when unknown
	OutputPath string
	Completed  bool
}

// Client talks to SABnzbd's /api endpoint.
// Password holds the API key (Sonarr/Radarr convention); Username is unused.
type Client struct {
	baseURL  string
	apiKey   string
	category string
	http     *http.Client
}

// NewFromDownloadClient builds a Client from a stored download client row.
func NewFromDownloadClient(dc domain.DownloadClient) *Client {
	baseURL, err := domain.NormalizeDownloadClientBaseURL(dc.BaseURL)
	if err != nil {
		baseURL = "http://127.0.0.1:8080"
	}
	apiKey := strings.TrimSpace(dc.Password)
	if apiKey == "" {
		apiKey = strings.TrimSpace(dc.Username)
	}
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		apiKey:   apiKey,
		category: dc.Category,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// Add enqueues an NZB URL (or magnet rejected) and returns the NZO id.
func (c *Client) Add(ctx context.Context, title, downloadURL, _protocol string) (string, error) {
	if strings.HasPrefix(strings.ToLower(downloadURL), "magnet:") {
		return "", fmt.Errorf("SABnzbd cannot accept magnet links (usenet client)")
	}
	q := url.Values{}
	q.Set("mode", "addurl")
	q.Set("name", downloadURL)
	q.Set("output", "json")
	q.Set("apikey", c.apiKey)
	if title != "" {
		q.Set("nzbname", title)
	}
	if c.category != "" {
		q.Set("cat", c.category)
	}
	var out struct {
		Status bool     `json:"status"`
		Error  string   `json:"error"`
		NzoIDs []string `json:"nzo_ids"`
	}
	if err := c.get(ctx, q, &out); err != nil {
		return "", err
	}
	if !out.Status {
		msg := strings.TrimSpace(out.Error)
		if msg == "" {
			msg = "addurl failed"
		}
		return "", fmt.Errorf("SABnzbd: %s", msg)
	}
	if len(out.NzoIDs) > 0 && out.NzoIDs[0] != "" {
		return out.NzoIDs[0], nil
	}
	return "pending", nil
}

// List returns active queue items plus recent history (for completed import).
func (c *Client) List(ctx context.Context) ([]Item, error) {
	qItems, err := c.listQueue(ctx)
	if err != nil {
		return nil, err
	}
	hItems, err := c.listHistory(ctx)
	if err != nil {
		// Queue alone is still useful.
		return qItems, nil
	}
	return append(qItems, hItems...), nil
}

func (c *Client) listQueue(ctx context.Context) ([]Item, error) {
	q := url.Values{}
	q.Set("mode", "queue")
	q.Set("output", "json")
	q.Set("apikey", c.apiKey)
	var out struct {
		Queue struct {
			Slots []struct {
				NzoID      string `json:"nzo_id"`
				Filename   string `json:"filename"`
				Status     string `json:"status"`
				Percentage string `json:"percentage"`
				MB         string `json:"mb"`
				MBleft     string `json:"mbleft"`
				Timeleft   string `json:"timeleft"`
				Storage    string `json:"storage"`
				Path       string `json:"path"`
			} `json:"slots"`
		} `json:"queue"`
		Error string `json:"error"`
	}
	if err := c.get(ctx, q, &out); err != nil {
		return nil, err
	}
	if out.Error != "" {
		return nil, fmt.Errorf("SABnzbd queue: %s", out.Error)
	}
	items := make([]Item, 0, len(out.Queue.Slots))
	for _, s := range out.Queue.Slots {
		prog := parseFloat(s.Percentage) / 100.0
		size := int64(parseFloat(s.MB) * 1024 * 1024)
		left := int64(parseFloat(s.MBleft) * 1024 * 1024)
		items = append(items, Item{
			ID:         s.NzoID,
			Title:      s.Filename,
			Status:     s.Status,
			Progress:   prog,
			Size:       size,
			SizeLeft:   left,
			Timeleft:   normalizeSABnzbdTimeleft(s.Timeleft),
			OutputPath: firstNonEmpty(s.Storage, s.Path),
			Completed:  IsCompleted(s.Status, prog),
		})
	}
	return items, nil
}

func (c *Client) listHistory(ctx context.Context) ([]Item, error) {
	q := url.Values{}
	q.Set("mode", "history")
	q.Set("output", "json")
	q.Set("apikey", c.apiKey)
	q.Set("limit", "50")
	var out struct {
		History struct {
			Slots []struct {
				NzoID    string  `json:"nzo_id"`
				Name     string  `json:"name"`
				Status   string  `json:"status"`
				Bytes    int64   `json:"bytes"`
				Size     string  `json:"size"`
				Storage  string  `json:"storage"`
				Path     string  `json:"path"`
				Completed float64 `json:"completed"`
			} `json:"slots"`
		} `json:"history"`
		Error string `json:"error"`
	}
	if err := c.get(ctx, q, &out); err != nil {
		return nil, err
	}
	if out.Error != "" {
		return nil, fmt.Errorf("SABnzbd history: %s", out.Error)
	}
	items := make([]Item, 0, len(out.History.Slots))
	for _, s := range out.History.Slots {
		size := s.Bytes
		if size == 0 {
			size = int64(parseFloat(s.Size) * 1024 * 1024)
		}
		completed := IsCompleted(s.Status, 1)
		items = append(items, Item{
			ID:         s.NzoID,
			Title:      s.Name,
			Status:     s.Status,
			Progress:   1,
			Size:       size,
			SizeLeft:   0,
			OutputPath: firstNonEmpty(s.Storage, s.Path),
			Completed:  completed,
		})
	}
	return items, nil
}

// Remove deletes a queue or history item by NZO id.
func (c *Client) Remove(ctx context.Context, id string, deleteFiles bool) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	q := url.Values{}
	q.Set("mode", "queue")
	q.Set("name", "delete")
	q.Set("value", id)
	q.Set("output", "json")
	q.Set("apikey", c.apiKey)
	if deleteFiles {
		q.Set("del_files", "1")
	}
	var out struct {
		Status bool   `json:"status"`
		Error  string `json:"error"`
	}
	if err := c.get(ctx, q, &out); err != nil {
		return err
	}
	if out.Status {
		return nil
	}
	// Fall back to history delete.
	q.Set("mode", "history")
	q.Set("name", "delete")
	if err := c.get(ctx, q, &out); err != nil {
		return err
	}
	if !out.Status {
		msg := strings.TrimSpace(out.Error)
		if msg == "" {
			msg = "delete failed"
		}
		return fmt.Errorf("SABnzbd: %s", msg)
	}
	return nil
}

// IsCompleted reports whether a SABnzbd slot has finished downloading.
func IsCompleted(status string, progress float64) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "complete", "finished":
		return true
	case "failed", "deleted":
		return false
	}
	return progress >= 0.999
}

// Test verifies the API key by listing the queue (same auth path as Add/List).
func (c *Client) Test(ctx context.Context) error {
	if strings.TrimSpace(c.apiKey) == "" {
		return fmt.Errorf("SABnzbd: API key is required (store it in the password field)")
	}
	q := url.Values{}
	q.Set("mode", "queue")
	q.Set("output", "json")
	q.Set("apikey", c.apiKey)
	var out struct {
		Status bool   `json:"status"`
		Error  string `json:"error"`
		Queue  any    `json:"queue"`
	}
	if err := c.get(ctx, q, &out); err != nil {
		return err
	}
	if out.Error != "" {
		return fmt.Errorf("SABnzbd: %s", strings.TrimSpace(out.Error))
	}
	if out.Status == false && out.Queue == nil {
		return fmt.Errorf("SABnzbd: authentication failed (check API key)")
	}
	return nil
}

func (c *Client) get(ctx context.Context, q url.Values, out any) error {
	endpoint := c.baseURL + "/api?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("SABnzbd: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("SABnzbd: %s (%s)", resp.Status, strings.TrimSpace(string(raw)))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("SABnzbd decode: %w", err)
	}
	return nil
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// normalizeSABnzbdTimeleft keeps a usable hh:mm:ss-style ETA and drops placeholders.
func normalizeSABnzbdTimeleft(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "?" || strings.EqualFold(s, "unknown") {
		return ""
	}
	return s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
