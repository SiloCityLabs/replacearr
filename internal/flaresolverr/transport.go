package flaresolverr

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
)

// SolvingTransport wraps a RoundTripper and retries blocked requests through FlareSolverr.
type SolvingTransport struct {
	Base     http.RoundTripper
	Client   *Client
	Host     string // FlareSolverr base URL (e.g. http://localhost:8191/)
	TimeoutS int    // challenge timeout seconds
}

func (t *SolvingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if t.Client != nil {
		t.Client.InjectClearance(req)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	}

	resp, err := base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}

	host := strings.TrimSpace(t.Host)
	if host == "" || t.Client == nil || !needsSolve(resp.StatusCode, resp.Header, string(body)) {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	timeout := t.TimeoutS
	if timeout <= 0 {
		timeout = 60
	}
	ctx := req.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	solved, solveErr := t.Client.SolveURL(ctx, host, req.URL.String(), req.Method, "", timeout, 15, true)
	if solveErr != nil || solved == nil || solved.Solution == nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	solBody := []byte(solved.Solution.Response)
	out := &http.Response{
		Status:        http.StatusText(http.StatusOK),
		StatusCode:    http.StatusOK,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(solBody)),
		ContentLength: int64(len(solBody)),
		Request:       req,
	}
	out.Header.Set("Content-Type", "text/html; charset=utf-8")
	out.Header.Set("X-Replacearr-FlareSolverr", "1")
	if solved.Solution.UserAgent != "" {
		out.Header.Set("X-Replacearr-User-Agent", solved.Solution.UserAgent)
	}
	return out, nil
}

func needsSolve(status int, headers http.Header, body string) bool {
	if IsCloudflareChallenge(status, headers, body) {
		return true
	}
	// Many CF edges return bare 403/503 without classic markers; still try FlareSolverr.
	if status == http.StatusForbidden || status == http.StatusServiceUnavailable || status == 429 {
		server := strings.ToLower(headers.Get("Server"))
		if strings.Contains(server, "cloudflare") || strings.Contains(server, "ddos-guard") ||
			strings.Contains(strings.ToLower(body), "cloudflare") ||
			strings.Contains(strings.ToLower(body), "just a moment") {
			return true
		}
		// Bare 403 with empty/small body is common from CF; treat as challenge.
		if status == http.StatusForbidden && len(strings.TrimSpace(body)) < 2048 {
			return true
		}
	}
	return false
}
