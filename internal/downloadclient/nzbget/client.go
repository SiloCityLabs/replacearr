// Package nzbget implements a minimal NZBGet JSON-RPC client.
package nzbget

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

// Item is a download group from NZBGet.
type Item struct {
	ID         string
	Title      string
	Status     string
	Progress   float64
	Size       int64
	SizeLeft   int64
	OutputPath string
	Completed  bool
}

// Client talks to NZBGet at /jsonrpc.
type Client struct {
	rpcURL   string
	username string
	password string
	category string
	http     *http.Client

	mu    sync.Mutex
	reqID int
}

// NewFromDownloadClient builds a Client from a stored download client row.
func NewFromDownloadClient(dc domain.DownloadClient) *Client {
	baseURL, err := domain.NormalizeDownloadClientBaseURL(dc.BaseURL)
	if err != nil {
		baseURL = "http://127.0.0.1:6789"
	}
	return &Client{
		rpcURL:   rpcEndpoint(baseURL),
		username: dc.Username,
		password: dc.Password,
		category: dc.Category,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

func rpcEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return strings.TrimRight(baseURL, "/") + "/jsonrpc"
	}
	path := strings.TrimSuffix(u.EscapedPath(), "/")
	switch {
	case path == "" || path == "/":
		u.Path = "/jsonrpc"
	case strings.HasSuffix(path, "/jsonrpc"):
		u.Path = path
	default:
		u.Path = path + "/jsonrpc"
	}
	return u.String()
}

type rpcRequest struct {
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
	JSONRPC string `json:"jsonrpc"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	ID int `json:"id"`
}

// Add fetches an NZB URL (when http/https), base64-encodes it, and appends to NZBGet.
func (c *Client) Add(ctx context.Context, title, downloadURL, _protocol string) (string, error) {
	if strings.HasPrefix(strings.ToLower(downloadURL), "magnet:") {
		return "", fmt.Errorf("NZBGet cannot accept magnet links (usenet client)")
	}
	filename := strings.TrimSpace(title)
	if filename == "" {
		filename = "release.nzb"
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".nzb") {
		filename += ".nzb"
	}
	contentB64, err := fetchNZBBase64(ctx, c.http, downloadURL)
	if err != nil {
		return "", err
	}
	// append(NZBFilename, Content (base64), Category, Priority, AddToTop)
	var nzbID int64
	if err := c.call(ctx, "append", []any{filename, contentB64, c.category, 0, false}, &nzbID); err != nil {
		// Older daemons: 3-arg form with empty content rejected; retry without priority flags.
		if err2 := c.call(ctx, "append", []any{filename, contentB64, c.category}, &nzbID); err2 != nil {
			return "", err
		}
	}
	if nzbID > 0 {
		return fmt.Sprintf("%d", nzbID), nil
	}
	return "pending", nil
}

// List returns active groups plus recent history.
func (c *Client) List(ctx context.Context) ([]Item, error) {
	active, err := c.listGroups(ctx)
	if err != nil {
		return nil, err
	}
	hist, err := c.listHistory(ctx)
	if err != nil {
		return active, nil
	}
	return append(active, hist...), nil
}

func (c *Client) listGroups(ctx context.Context) ([]Item, error) {
	var groups []struct {
		NZBID          int64   `json:"NZBID"`
		NZBName        string  `json:"NZBName"`
		Status         string  `json:"Status"`
		FileSizeLo     int64   `json:"FileSizeLo"`
		FileSizeHi     int64   `json:"FileSizeHi"`
		FileSizeMB     float64 `json:"FileSizeMB"`
		RemainingSizeLo int64  `json:"RemainingSizeLo"`
		RemainingSizeHi int64  `json:"RemainingSizeHi"`
		RemainingSizeMB float64 `json:"RemainingSizeMB"`
		DownloadedSizeLo int64 `json:"DownloadedSizeLo"`
		DownloadedSizeHi int64 `json:"DownloadedSizeHi"`
		DestDir        string  `json:"DestDir"`
		FinalDir       string  `json:"FinalDir"`
	}
	if err := c.call(ctx, "listgroups", []any{}, &groups); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(groups))
	for _, g := range groups {
		size := combineSize(g.FileSizeHi, g.FileSizeLo, g.FileSizeMB)
		left := combineSize(g.RemainingSizeHi, g.RemainingSizeLo, g.RemainingSizeMB)
		prog := 0.0
		if size > 0 {
			prog = float64(size-left) / float64(size)
		}
		items = append(items, Item{
			ID:         fmt.Sprintf("%d", g.NZBID),
			Title:      g.NZBName,
			Status:     g.Status,
			Progress:   prog,
			Size:       size,
			SizeLeft:   left,
			OutputPath: firstNonEmpty(g.FinalDir, g.DestDir),
			Completed:  IsCompleted(g.Status, prog),
		})
	}
	return items, nil
}

func (c *Client) listHistory(ctx context.Context) ([]Item, error) {
	var hist []struct {
		NZBID    int64   `json:"NZBID"`
		NZBName  string  `json:"NZBName"`
		Status   string  `json:"Status"`
		FileSizeLo int64 `json:"FileSizeLo"`
		FileSizeHi int64 `json:"FileSizeHi"`
		FileSizeMB float64 `json:"FileSizeMB"`
		DestDir  string  `json:"DestDir"`
		FinalDir string  `json:"FinalDir"`
	}
	// list history (false = do not include hidden duplicates extensively)
	if err := c.call(ctx, "history", []any{false}, &hist); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(hist))
	for _, h := range hist {
		size := combineSize(h.FileSizeHi, h.FileSizeLo, h.FileSizeMB)
		completed := IsCompleted(h.Status, 1)
		items = append(items, Item{
			ID:         fmt.Sprintf("%d", h.NZBID),
			Title:      h.NZBName,
			Status:     h.Status,
			Progress:   1,
			Size:       size,
			SizeLeft:   0,
			OutputPath: firstNonEmpty(h.FinalDir, h.DestDir),
			Completed:  completed,
		})
	}
	return items, nil
}

// Remove deletes a group/history item by NZBID.
func (c *Client) Remove(ctx context.Context, id string, deleteFiles bool) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	var nzbID int64
	if _, err := fmt.Sscan(id, &nzbID); err != nil || nzbID == 0 {
		return fmt.Errorf("NZBGet: invalid id %q", id)
	}
	// editqueue(command, offset, size, ids...) — GroupDelete / HistoryDelete
	cmd := "GroupDelete"
	if deleteFiles {
		cmd = "GroupDelete" // NZBGet deletes data based on config; HistoryDelete for history
	}
	var ok bool
	if err := c.call(ctx, "editqueue", []any{cmd, 0, "", []int64{nzbID}}, &ok); err != nil || !ok {
		_ = c.call(ctx, "editqueue", []any{"HistoryDelete", 0, "", []int64{nzbID}}, &ok)
	}
	return nil
}

// IsCompleted reports whether an NZBGet status means download finished.
func IsCompleted(status string, progress float64) bool {
	s := strings.ToUpper(strings.TrimSpace(status))
	switch {
	case strings.HasPrefix(s, "SUCCESS"), s == "SUCCESS":
		return true
	case strings.Contains(s, "SUCCESS/"):
		return true
	case s == "DELETED", strings.HasPrefix(s, "FAILURE"), strings.HasPrefix(s, "WARNING"):
		return false
	}
	return progress >= 0.999
}

func (c *Client) call(ctx context.Context, method string, params []any, out any) error {
	c.mu.Lock()
	c.reqID++
	id := c.reqID
	c.mu.Unlock()
	body, err := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: method, Params: params, ID: id})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.rpcURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("NZBGet rpc: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("NZBGet rpc: unauthorized (check username/password)")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("NZBGet rpc: %s (%s)", resp.Status, strings.TrimSpace(string(raw)))
	}
	var reply rpcResponse
	if err := json.Unmarshal(raw, &reply); err != nil {
		return fmt.Errorf("NZBGet rpc decode: %w", err)
	}
	if reply.Error != nil && reply.Error.Message != "" {
		return fmt.Errorf("NZBGet rpc: %s", reply.Error.Message)
	}
	if out == nil || len(reply.Result) == 0 || string(reply.Result) == "null" {
		return nil
	}
	if err := json.Unmarshal(reply.Result, out); err != nil {
		return fmt.Errorf("NZBGet rpc result: %w", err)
	}
	return nil
}

func combineSize(hi, lo int64, mb float64) int64 {
	if hi != 0 || lo != 0 {
		// NZBGet splits 64-bit into two 32-bit parts.
		return (hi << 32) | (lo & 0xffffffff)
	}
	if mb > 0 {
		return int64(mb * 1024 * 1024)
	}
	return 0
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func fetchNZBBase64(ctx context.Context, httpClient *http.Client, downloadURL string) (string, error) {
	downloadURL = strings.TrimSpace(downloadURL)
	if downloadURL == "" {
		return "", fmt.Errorf("NZBGet: empty download URL")
	}
	// Already looks like raw NZB XML — encode directly.
	if strings.Contains(downloadURL, "<nzb") || strings.Contains(downloadURL, "<?xml") {
		return base64.StdEncoding.EncodeToString([]byte(downloadURL)), nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("NZBGet fetch nzb: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("NZBGet fetch nzb: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return "", fmt.Errorf("NZBGet fetch nzb: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("NZBGet fetch nzb: %s", resp.Status)
	}
	if len(raw) == 0 {
		return "", fmt.Errorf("NZBGet fetch nzb: empty body")
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}
