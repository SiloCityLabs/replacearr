package discover

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
)

const fingerprintTimeout = 2 * time.Second

// Result is a fingerprint hit suitable for pre-filling a download client form.
type Result struct {
	Implementation string `json:"implementation"`
	Protocol       string `json:"protocol"`
	BaseURL        string `json:"baseUrl"`
	NameHint       string `json:"nameHint"`
	Detail         string `json:"detail,omitempty"`
}

type fingerprintFn func(ctx context.Context, client *http.Client, base string) bool

type fingerprintProbe struct {
	implementation string
	nameHint       string
	fn             fingerprintFn
}

var allFingerprints = []fingerprintProbe{
	{"Transmission", "Transmission", fingerprintTransmission},
	{"Deluge", "Deluge", fingerprintDeluge},
	{"SABnzbd", "SABnzbd", fingerprintSABnzbd},
	{"NZBGet", "NZBGet", fingerprintNZBGet},
	{"qBittorrent", "qBittorrent", fingerprintQBittorrent},
}

// Prefer the client that normally owns this port so a generic 401/403 cannot
// steal the match (e.g. NZBGet basic-auth on :6789 must not become qBittorrent).
var fingerprintsByPort = map[int][]fingerprintProbe{
	6789: {
		{"NZBGet", "NZBGet", fingerprintNZBGet},
		{"qBittorrent", "qBittorrent", fingerprintQBittorrent},
		{"Transmission", "Transmission", fingerprintTransmission},
		{"Deluge", "Deluge", fingerprintDeluge},
		{"SABnzbd", "SABnzbd", fingerprintSABnzbd},
	},
	9091: {
		{"Transmission", "Transmission", fingerprintTransmission},
		{"qBittorrent", "qBittorrent", fingerprintQBittorrent},
		{"Deluge", "Deluge", fingerprintDeluge},
		{"SABnzbd", "SABnzbd", fingerprintSABnzbd},
		{"NZBGet", "NZBGet", fingerprintNZBGet},
	},
	8112: {
		{"Deluge", "Deluge", fingerprintDeluge},
		{"qBittorrent", "qBittorrent", fingerprintQBittorrent},
		{"Transmission", "Transmission", fingerprintTransmission},
		{"SABnzbd", "SABnzbd", fingerprintSABnzbd},
		{"NZBGet", "NZBGet", fingerprintNZBGet},
	},
	8085: {
		{"SABnzbd", "SABnzbd", fingerprintSABnzbd},
		{"qBittorrent", "qBittorrent", fingerprintQBittorrent},
		{"Transmission", "Transmission", fingerprintTransmission},
		{"Deluge", "Deluge", fingerprintDeluge},
		{"NZBGet", "NZBGet", fingerprintNZBGet},
	},
	8080: {
		{"qBittorrent", "qBittorrent", fingerprintQBittorrent},
		{"SABnzbd", "SABnzbd", fingerprintSABnzbd},
		{"Transmission", "Transmission", fingerprintTransmission},
		{"Deluge", "Deluge", fingerprintDeluge},
		{"NZBGet", "NZBGet", fingerprintNZBGet},
	},
}

var qbitVersionRe = regexp.MustCompile(`(?i)^v?\d+(\.\d+){1,3}`)

func probesForPort(port int) []fingerprintProbe {
	if ordered, ok := fingerprintsByPort[port]; ok {
		return ordered
	}
	return allFingerprints
}

func fingerprintOpenPort(ctx context.Context, op openPort) (Result, bool) {
	base := fmt.Sprintf("http://%s:%d", op.Host, op.Port)
	normalized, err := domain.NormalizeDownloadClientBaseURL(base)
	if err != nil {
		return Result{}, false
	}
	client := &http.Client{
		Timeout: fingerprintTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	for _, fp := range probesForPort(op.Port) {
		select {
		case <-ctx.Done():
			return Result{}, false
		default:
		}
		if !fp.fn(ctx, client, normalized) {
			continue
		}
		detail := fmt.Sprintf("open port %d", op.Port)
		if op.Via != "" {
			detail = op.Via + ", " + detail
		}
		return Result{
			Implementation: fp.implementation,
			Protocol:       domain.ProtocolForDownloadClientImplementation(fp.implementation, ""),
			BaseURL:        normalized,
			NameHint:       fp.nameHint,
			Detail:         detail,
		}, true
	}
	return Result{}, false
}

func fingerprintQBittorrent(ctx context.Context, client *http.Client, base string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/api/v2/app/version", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	s := strings.TrimSpace(string(body))

	switch resp.StatusCode {
	case http.StatusOK:
		// Require a real version string — never treat arbitrary OK bodies as qBittorrent.
		return qbitVersionRe.MatchString(s)
	case http.StatusForbidden, http.StatusUnauthorized:
		// NZBGet/Transmission often return 401 Basic on unknown paths. Only treat as
		// qBittorrent when there is no Basic challenge and the WebUI HTML confirms it.
		auth := strings.ToLower(resp.Header.Get("WWW-Authenticate"))
		if strings.Contains(auth, "basic") {
			return false
		}
		return qBittorrentWebUI(ctx, client, base)
	default:
		return false
	}
}

func qBittorrentWebUI(ctx context.Context, client *http.Client, base string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "qbittorrent")
}

func fingerprintTransmission(ctx context.Context, client *http.Client, base string) bool {
	endpoint := transmissionRPC(base)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader([]byte(`{"method":"session-get"}`)))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.Header.Get("X-Transmission-Session-Id") != "" {
		return true
	}
	if resp.StatusCode == http.StatusConflict {
		return true
	}
	// Auth-enabled Transmission answers Basic 401 on the RPC path.
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		auth := strings.ToLower(resp.Header.Get("WWW-Authenticate"))
		if strings.Contains(auth, "basic") || strings.Contains(strings.ToLower(string(body)), "unauthorized") {
			return true
		}
	}
	return false
}

func transmissionRPC(base string) string {
	base = strings.TrimRight(base, "/")
	switch {
	case strings.HasSuffix(base, "/transmission/rpc"):
		return base
	case strings.HasSuffix(base, "/transmission"):
		return base + "/rpc"
	default:
		return base + "/transmission/rpc"
	}
}

func fingerprintDeluge(ctx context.Context, client *http.Client, base string) bool {
	endpoint := delugeJSON(base)
	payload := `{"method":"web.connected","params":[],"id":1}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(payload))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var parsed struct {
		ID     any `json:"id"`
		Result any `json:"result"`
		Error  any `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false
	}
	return parsed.ID != nil || parsed.Result != nil || parsed.Error != nil
}

func delugeJSON(base string) string {
	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(base, "/json") {
		return base
	}
	return base + "/json"
}

func fingerprintSABnzbd(ctx context.Context, client *http.Client, base string) bool {
	u := strings.TrimRight(base, "/") + "/api?mode=version&output=json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var m map[string]any
	if json.Unmarshal(body, &m) != nil {
		return false
	}
	if _, ok := m["version"]; ok {
		return true
	}
	if errMsg, ok := m["error"].(string); ok && errMsg != "" {
		return true
	}
	return false
}

func fingerprintNZBGet(ctx context.Context, client *http.Client, base string) bool {
	endpoint := nzbgetJSONRPC(base)
	payload := `{"jsonrpc":"2.0","method":"version","params":[],"id":1}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(payload))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	// NZBGet uses HTTP basic auth for the whole WebUI/JSON-RPC surface.
	if resp.StatusCode == http.StatusUnauthorized {
		auth := strings.ToLower(resp.Header.Get("WWW-Authenticate"))
		return strings.Contains(auth, "basic") || strings.Contains(auth, "nzbget")
	}
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var parsed struct {
		JSONRPC string `json:"jsonrpc"`
		Result  any    `json:"result"`
		Error   any    `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false
	}
	return parsed.JSONRPC == "2.0" || parsed.Result != nil || parsed.Error != nil
}

func nzbgetJSONRPC(base string) string {
	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(base, "/jsonrpc") {
		return base
	}
	return base + "/jsonrpc"
}
