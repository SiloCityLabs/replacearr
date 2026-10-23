// Package flaresolverr implements a full client, session manager, per-domain
// clearance cache, and Cloudflare/DDoS-Guard challenge detector for FlareSolverr
// (https://github.com/FlareSolverr/FlareSolverr).
package flaresolverr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	StatusOK    = "ok"
	StatusError = "error"

	CmdRequestGet      = "request.get"
	CmdRequestPost     = "request.post"
	CmdSessionsCreate  = "sessions.create"
	CmdSessionsList    = "sessions.list"
	CmdSessionsDestroy = "sessions.destroy"
)

// Cookie represents a browser cookie returned or accepted by FlareSolverr.
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain,omitempty"`
	Path     string  `json:"path,omitempty"`
	Expires  float64 `json:"expires,omitempty"`
	Size     int     `json:"size,omitempty"`
	HTTPOnly bool    `json:"httpOnly,omitempty"`
	Secure   bool    `json:"secure,omitempty"`
	Session  bool    `json:"session,omitempty"`
	SameSite string  `json:"sameSite,omitempty"`
}

// ProxyConfig represents an upstream HTTP/SOCKS proxy passed to FlareSolverr.
type ProxyConfig struct {
	URL      string `json:"url,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// V1Request represents the JSON payload sent to FlareSolverr POST /v1.
type V1Request struct {
	Cmd               string       `json:"cmd"`
	URL               string       `json:"url,omitempty"`
	PostData          string       `json:"postData,omitempty"`
	MaxTimeout        int          `json:"maxTimeout,omitempty"` // milliseconds, default 60000
	Cookies           []Cookie     `json:"cookies,omitempty"`
	Session           string       `json:"session,omitempty"`
	SessionTTLMinutes int          `json:"session_ttl_minutes,omitempty"`
	Proxy             *ProxyConfig `json:"proxy,omitempty"`
	ReturnOnlyCookies bool         `json:"returnOnlyCookies,omitempty"`
	ReturnScreenshot  bool         `json:"returnScreenshot,omitempty"`
	WaitInSeconds     int          `json:"waitInSeconds,omitempty"`
	DisableMedia      bool         `json:"disableMedia,omitempty"`
	TabsTillVerify    int          `json:"tabs_till_verify,omitempty"`
}

// Solution contains the solved challenge result from FlareSolverr.
type Solution struct {
	URL            string         `json:"url"`
	Status         int            `json:"status"`
	Headers        map[string]any `json:"headers,omitempty"`
	Response       string         `json:"response"`
	Cookies        []Cookie       `json:"cookies"`
	UserAgent      string         `json:"userAgent"`
	Screenshot     string         `json:"screenshot,omitempty"`
	TurnstileToken string         `json:"turnstile_token,omitempty"`
}

// V1Response represents the JSON response from FlareSolverr POST /v1.
type V1Response struct {
	Status         string    `json:"status"`
	Message        string    `json:"message"`
	Session        string    `json:"session,omitempty"`
	Sessions       []string  `json:"sessions,omitempty"`
	StartTimestamp int64     `json:"startTimestamp"`
	EndTimestamp   int64     `json:"endTimestamp"`
	Version        string    `json:"version"`
	Solution       *Solution `json:"solution,omitempty"`
}

// IndexResponse represents GET / on FlareSolverr.
type IndexResponse struct {
	Msg       string `json:"msg"`
	Version   string `json:"version"`
	UserAgent string `json:"userAgent"`
}

// HealthStatus represents the result of testing/probing a FlareSolverr instance.
type HealthStatus struct {
	Host      string   `json:"host"`
	Healthy   bool     `json:"healthy"`
	Version   string   `json:"version,omitempty"`
	UserAgent string   `json:"userAgent,omitempty"`
	Message   string   `json:"message"`
	LatencyMs int64    `json:"latencyMs"`
	Sessions  []string `json:"sessions,omitempty"`
}

// DomainClearance stores cached cookies and User-Agent solved by FlareSolverr for a domain.
type DomainClearance struct {
	Domain    string    `json:"domain"`
	UserAgent string    `json:"userAgent"`
	Cookies   []Cookie  `json:"cookies"`
	UpdatedAt time.Time `json:"updatedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Client manages communication with FlareSolverr and caches per-domain Cloudflare clearances.
type Client struct {
	httpClient *http.Client
	mu         sync.RWMutex
	cache      map[string]DomainClearance
}

// NewClient creates a new FlareSolverr client with an in-memory clearance cache.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 190 * time.Second,
		},
		cache: make(map[string]DomainClearance),
	}
}

// NormalizeV1Endpoint normalizes a FlareSolverr host URL (e.g. "http://localhost:8191/") to its "/v1" endpoint.
func NormalizeV1Endpoint(host string) string {
	h := strings.TrimSpace(host)
	if h == "" {
		h = "http://localhost:8191"
	}
	h = strings.TrimRight(h, "/")
	if strings.HasSuffix(h, "/v1") {
		return h
	}
	return h + "/v1"
}

// NormalizeBaseURL normalizes a FlareSolverr host URL without trailing slash or /v1 suffix.
func NormalizeBaseURL(host string) string {
	h := strings.TrimSpace(host)
	if h == "" {
		h = "http://localhost:8191"
	}
	h = strings.TrimRight(h, "/")
	h = strings.TrimSuffix(h, "/v1")
	return h
}

// Execute sends a raw V1Request to a FlareSolverr instance at host.
func (c *Client) Execute(ctx context.Context, host string, req V1Request) (*V1Response, error) {
	if req.Cmd == "" {
		req.Cmd = CmdRequestGet
	}
	if (req.Cmd == CmdRequestGet || req.Cmd == CmdRequestPost) && req.MaxTimeout <= 0 {
		req.MaxTimeout = 60000
	}

	endpoint := NormalizeV1Endpoint(host)
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal flaresolverr request: %w", err)
	}

	timeout := time.Duration(req.MaxTimeout)*time.Millisecond + 10*time.Second
	if timeout < 15*time.Second {
		timeout = 15 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create flaresolverr http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("flaresolverr request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read flaresolverr response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		return nil, fmt.Errorf("unexpected flaresolverr HTTP status %d: %s", resp.StatusCode, string(respBytes))
	}

	var v1Resp V1Response
	if err := json.Unmarshal(respBytes, &v1Resp); err != nil {
		return nil, fmt.Errorf("decode flaresolverr response: %w", err)
	}

	if v1Resp.Status != StatusOK {
		msg := v1Resp.Message
		if msg == "" {
			msg = "flaresolverr returned status=" + v1Resp.Status
		}
		return &v1Resp, errors.New(msg)
	}

	// Cache clearance cookies & user-agent whenever a solution is returned
	if v1Resp.Solution != nil {
		targetURL := v1Resp.Solution.URL
		if targetURL == "" {
			targetURL = req.URL
		}
		if parsed, err := url.Parse(targetURL); err == nil && parsed.Hostname() != "" {
			c.CacheSolution(parsed.Hostname(), v1Resp.Solution)
		}
	}

	return &v1Resp, nil
}

// SolveURL executes a request.get or request.post through FlareSolverr and caches the resulting cookies and User-Agent.
func (c *Client) SolveURL(ctx context.Context, host string, targetURL string, method string, postData string, timeoutSec int, sessionTTLMinutes int, disableMedia bool) (*V1Response, error) {
	if timeoutSec <= 0 {
		timeoutSec = 60
	}
	cmd := CmdRequestGet
	if strings.EqualFold(method, http.MethodPost) {
		cmd = CmdRequestPost
	}

	var existingCookies []Cookie
	if u, err := url.Parse(targetURL); err == nil && u.Hostname() != "" {
		if cl, ok := c.GetClearance(u.Hostname()); ok {
			existingCookies = cl.Cookies
		}
	}

	req := V1Request{
		Cmd:               cmd,
		URL:               targetURL,
		PostData:          postData,
		MaxTimeout:        timeoutSec * 1000,
		Cookies:           existingCookies,
		SessionTTLMinutes: sessionTTLMinutes,
		DisableMedia:      disableMedia,
	}
	return c.Execute(ctx, host, req)
}

// CreateSession calls sessions.create on FlareSolverr.
func (c *Client) CreateSession(ctx context.Context, host string, sessionID string, proxy *ProxyConfig) (string, error) {
	resp, err := c.Execute(ctx, host, V1Request{
		Cmd:     CmdSessionsCreate,
		Session: sessionID,
		Proxy:   proxy,
	})
	if err != nil {
		return "", err
	}
	return resp.Session, nil
}

// ListSessions calls sessions.list on FlareSolverr.
func (c *Client) ListSessions(ctx context.Context, host string) ([]string, error) {
	resp, err := c.Execute(ctx, host, V1Request{
		Cmd: CmdSessionsList,
	})
	if err != nil {
		return nil, err
	}
	if resp.Sessions == nil {
		return []string{}, nil
	}
	return resp.Sessions, nil
}

// DestroySession calls sessions.destroy on FlareSolverr.
func (c *Client) DestroySession(ctx context.Context, host string, sessionID string) error {
	_, err := c.Execute(ctx, host, V1Request{
		Cmd:     CmdSessionsDestroy,
		Session: sessionID,
	})
	return err
}

// TestConnection verifies connectivity to a FlareSolverr server by checking GET / and sessions.list.
func (c *Client) TestConnection(ctx context.Context, host string) HealthStatus {
	start := time.Now()
	baseURL := NormalizeBaseURL(host)

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	status := HealthStatus{
		Host: baseURL,
	}

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodGet, baseURL+"/", nil)
	if err != nil {
		status.Message = err.Error()
		return status
	}

	resp, err := c.httpClient.Do(httpReq)
	status.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		status.Message = fmt.Sprintf("Unable to connect to FlareSolverr at %s: %v", baseURL, err)
		return status
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var idxResp IndexResponse
	if err := json.Unmarshal(body, &idxResp); err == nil && (idxResp.Version != "" || idxResp.Msg != "") {
		status.Healthy = resp.StatusCode == http.StatusOK
		status.Version = idxResp.Version
		status.UserAgent = idxResp.UserAgent
		status.Message = idxResp.Msg
		if sessions, sErr := c.ListSessions(reqCtx, host); sErr == nil {
			status.Sessions = sessions
		}
		return status
	}

	// Fallback: try sessions.list via POST /v1
	if v1Resp, v1Err := c.Execute(reqCtx, host, V1Request{Cmd: CmdSessionsList}); v1Err == nil {
		status.Healthy = true
		status.Version = v1Resp.Version
		status.Sessions = v1Resp.Sessions
		status.Message = "FlareSolverr v1 API online"
		return status
	}

	status.Message = fmt.Sprintf("Unexpected response from %s (HTTP %d)", baseURL, resp.StatusCode)
	return status
}

// CacheSolution stores the solved User-Agent and cookies for a hostname.
func (c *Client) CacheSolution(domainName string, sol *Solution) {
	if sol == nil || domainName == "" {
		return
	}
	domainName = strings.ToLower(strings.TrimSpace(domainName))
	now := time.Now().UTC()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[domainName] = DomainClearance{
		Domain:    domainName,
		UserAgent: sol.UserAgent,
		Cookies:   sol.Cookies,
		UpdatedAt: now,
		ExpiresAt: now.Add(30 * time.Minute),
	}
}

// GetClearance retrieves a non-expired DomainClearance for a hostname.
func (c *Client) GetClearance(domainName string) (DomainClearance, bool) {
	domainName = strings.ToLower(strings.TrimSpace(domainName))
	c.mu.RLock()
	defer c.mu.RUnlock()
	cl, ok := c.cache[domainName]
	if !ok || time.Now().UTC().After(cl.ExpiresAt) {
		return DomainClearance{}, false
	}
	return cl, true
}

// ListClearances returns all active cached domain clearances.
func (c *Client) ListClearances() []DomainClearance {
	c.mu.RLock()
	defer c.mu.RUnlock()
	now := time.Now().UTC()
	out := make([]DomainClearance, 0, len(c.cache))
	for _, cl := range c.cache {
		if now.Before(cl.ExpiresAt) {
			out = append(out, cl)
		}
	}
	return out
}

// ClearCache clears all cached domain clearances.
func (c *Client) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]DomainClearance)
}

// InjectClearance applies any cached FlareSolverr User-Agent and cookies to an outgoing http.Request.
func (c *Client) InjectClearance(req *http.Request) bool {
	if req == nil || req.URL == nil {
		return false
	}
	cl, ok := c.GetClearance(req.URL.Hostname())
	if !ok {
		return false
	}
	if cl.UserAgent != "" {
		req.Header.Set("User-Agent", cl.UserAgent)
	}
	for _, ck := range cl.Cookies {
		req.AddCookie(&http.Cookie{
			Name:  ck.Name,
			Value: ck.Value,
		})
	}
	return true
}

// IsCloudflareChallenge inspects an HTTP status code, headers, and HTML body to detect
// Cloudflare, Turnstile, or DDoS-Guard protection (matching Prowlarr's CloudFlareDetectionService
// and FlareSolverr's challenge selectors).
func IsCloudflareChallenge(statusCode int, headers http.Header, body string) bool {
	server := ""
	vary := ""
	contentEncoding := ""
	if headers != nil {
		server = strings.ToLower(headers.Get("Server"))
		vary = headers.Get("Vary")
		contentEncoding = headers.Get("Content-Encoding")
	}

	isCFServer := strings.Contains(server, "cloudflare") || strings.Contains(server, "ddos-guard")
	lowerBody := strings.ToLower(body)

	if statusCode == http.StatusServiceUnavailable || statusCode == http.StatusForbidden || statusCode == 429 {
		if isCFServer || strings.Contains(lowerBody, "cloudflare") || strings.Contains(lowerBody, "ddos-guard") {
			challengeMarkers := []string{
				"<title>just a moment...</title>",
				"<title>access denied</title>",
				"<title>attention required! | cloudflare</title>",
				"<title>ddos-guard</title>",
				"error code: 1020",
				"cf-challenge-running",
				"cf-please-wait",
				"challenge-spinner",
				"turnstile-wrapper",
				"cf-turnstile-response",
				"jschl_vc",
				"jschl_answer",
			}
			for _, marker := range challengeMarkers {
				if strings.Contains(lowerBody, marker) {
					return true
				}
			}
		}
	}

	// Custom DDoS protection check from Prowlarr's CloudFlareDetectionService
	if vary == "Accept-Encoding,User-Agent" && strings.TrimSpace(contentEncoding) == "" && strings.Contains(lowerBody, "ddos") {
		return true
	}

	return false
}
