package native

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
	"github.com/replacearr/replacearr/internal/flaresolverr"
	"github.com/replacearr/replacearr/internal/httpclient"
	"github.com/replacearr/replacearr/internal/indexers/cardigann"
	"github.com/replacearr/replacearr/pkg/pluginsdk"
)

type indexerTestResult struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	LatencyMs int64  `json:"latencyMs"`
	Releases  int    `json:"releases,omitempty"`
}

func (s *Server) handleTestIndexer(w http.ResponseWriter, r *http.Request) {
	var req domain.Indexer
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	idx, err := s.resolveIndexerForTest(req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	start := time.Now()
	result := s.probeIndexer(ctx, idx)
	result.LatencyMs = time.Since(start).Milliseconds()
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) resolveIndexerForTest(req domain.Indexer) (domain.Indexer, error) {
	if req.ID > 0 {
		stored, err := s.store.GetIndexerByID(req.ID)
		if err != nil {
			return domain.Indexer{}, fmt.Errorf("indexer %d not found", req.ID)
		}
		merged := *stored
		if strings.TrimSpace(req.Name) != "" {
			merged.Name = req.Name
		}
		if strings.TrimSpace(req.Implementation) != "" {
			merged.Implementation = req.Implementation
		}
		if strings.TrimSpace(req.BaseURL) != "" {
			merged.BaseURL = req.BaseURL
		}
		if req.APIKey != "" {
			merged.APIKey = req.APIKey
		}
		if strings.TrimSpace(req.DefinitionFile) != "" {
			merged.DefinitionFile = req.DefinitionFile
		}
		if strings.TrimSpace(req.PluginID) != "" {
			merged.PluginID = req.PluginID
		}
		if len(req.Categories) > 0 {
			merged.Categories = req.Categories
		}
		if len(req.Fields) > 0 {
			merged.Fields = req.Fields
		}
		return merged, nil
	}

	if strings.TrimSpace(req.Implementation) == "" {
		return domain.Indexer{}, fmt.Errorf("implementation is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = req.Implementation
	}
	return req, nil
}

func (s *Server) probeIndexer(ctx context.Context, idx domain.Indexer) indexerTestResult {
	impl := strings.ToLower(strings.TrimSpace(idx.Implementation))
	switch impl {
	case "torznab", "newznab":
		return s.probeTorznabNewznab(ctx, idx)
	case "cardigann":
		return s.probeCardigann(ctx, idx)
	case "plugin":
		return s.probePluginIndexer(ctx, idx)
	default:
		return indexerTestResult{
			OK:      false,
			Message: fmt.Sprintf("unsupported implementation %q", idx.Implementation),
		}
	}
}

func (s *Server) probeTorznabNewznab(ctx context.Context, idx domain.Indexer) indexerTestResult {
	base := strings.TrimSpace(idx.BaseURL)
	if base == "" {
		return indexerTestResult{OK: false, Message: "baseUrl is required"}
	}
	capsURL, err := buildIndexerCapsURL(base, idx.APIKey)
	if err != nil {
		return indexerTestResult{OK: false, Message: err.Error()}
	}

	client := s.indexerHTTPClient(60 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, capsURL, nil)
	if err != nil {
		return indexerTestResult{OK: false, Message: err.Error()}
	}
	resp, err := client.Do(req)
	if err != nil {
		return indexerTestResult{OK: false, Message: err.Error()}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := fmt.Sprintf("HTTP %d from caps endpoint", resp.StatusCode)
		if snippet := strings.TrimSpace(string(body)); snippet != "" {
			if len(snippet) > 180 {
				snippet = snippet[:180] + "…"
			}
			msg += ": " + snippet
		}
		return indexerTestResult{OK: false, Message: msg}
	}
	lower := strings.ToLower(string(body))
	if !strings.Contains(lower, "caps") &&
		!strings.Contains(lower, "<server") &&
		!strings.Contains(lower, "categories") &&
		!strings.Contains(lower, "<rss") {
		return indexerTestResult{
			OK:      false,
			Message: "caps response did not look like Newznab/Torznab XML",
		}
	}
	return indexerTestResult{
		OK:      true,
		Message: fmt.Sprintf("caps OK from %s", idx.Name),
	}
}

func buildIndexerCapsURL(baseURL, apiKey string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", fmt.Errorf("invalid baseUrl: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("baseUrl must include scheme and host")
	}
	q := u.Query()
	if q.Get("t") == "" {
		q.Set("t", "caps")
	}
	if key := strings.TrimSpace(apiKey); key != "" && q.Get("apikey") == "" {
		q.Set("apikey", key)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (s *Server) probeCardigann(ctx context.Context, idx domain.Indexer) indexerTestResult {
	defsDir := ""
	if s.cfg != nil {
		defsDir = s.cfg.DefinitionsDir
	}
	if strings.TrimSpace(defsDir) == "" {
		return indexerTestResult{OK: false, Message: "definitions directory is not configured"}
	}
	defFile := strings.TrimSpace(idx.DefinitionFile)
	if defFile == "" {
		return indexerTestResult{OK: false, Message: "definitionFile is required for Cardigann"}
	}

	proxyURL := ""
	if s.pluginMgr != nil {
		proxyURL = s.pluginMgr.ActiveProxyURL()
	}
	client := s.indexerHTTPClient(60 * time.Second)
	rels, err := cardigann.Search(ctx, cardigann.SearchRequest{
		DefinitionsDir: defsDir,
		DefinitionFile: defFile,
		BaseURL:        idx.BaseURL,
		IndexerID:      idx.ID,
		IndexerName:    idx.Name,
		Fields:         idx.Fields,
		Query:          "",
		SearchMode:     "search",
		Categories:     idx.Categories,
		HTTPClient:     client,
		ProxyURL:       proxyURL,
	})
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "403") || strings.Contains(strings.ToLower(msg), "cloudflare") {
			msg += " — site blocked the request; add an enabled FlareSolverr indexer proxy under Indexers"
		}
		return indexerTestResult{OK: false, Message: msg}
	}
	return indexerTestResult{
		OK:       true,
		Message:  fmt.Sprintf("search OK — %d release(s)", len(rels)),
		Releases: len(rels),
	}
}

func (s *Server) probePluginIndexer(ctx context.Context, idx domain.Indexer) indexerTestResult {
	if s.pluginMgr == nil {
		return indexerTestResult{OK: false, Message: "plugin host is unavailable"}
	}
	pluginID := strings.TrimSpace(idx.PluginID)
	if pluginID == "" {
		return indexerTestResult{OK: false, Message: "pluginId is required for Plugin indexers"}
	}
	proxyURL := s.pluginMgr.ActiveProxyURL()
	rels, err := s.pluginMgr.SearchIndexer(ctx, pluginID, pluginsdk.SearchRequest{
		Query:           "",
		Type:            "search",
		Limit:           25,
		FlareSolverrURL: s.activeFlareSolverrHost(),
		ProxyURL:        proxyURL,
	})
	if err != nil {
		return indexerTestResult{OK: false, Message: err.Error()}
	}
	return indexerTestResult{
		OK:       true,
		Message:  fmt.Sprintf("plugin search OK — %d release(s)", len(rels)),
		Releases: len(rels),
	}
}

func (s *Server) indexerHTTPClient(timeout time.Duration) *http.Client {
	proxyURL := ""
	if s.pluginMgr != nil {
		proxyURL = s.pluginMgr.ActiveProxyURL()
	}
	appCfg, _ := s.store.GetAppConfig()
	baseTransport := http.DefaultTransport
	httpCli := httpclient.New(httpclient.Options{
		AppProxy:   appCfg.Proxy,
		PluginURLs: []string{proxyURL},
		Timeout:    timeout,
	})
	if httpCli != nil && httpCli.Transport != nil {
		baseTransport = httpCli.Transport
	}
	transport := baseTransport
	flareHost := strings.TrimSpace(s.activeFlareSolverrHost())
	if flareHost != "" && s.flareClient != nil {
		transport = &flaresolverr.SolvingTransport{
			Base:     baseTransport,
			Client:   s.flareClient,
			Host:     flareHost,
			TimeoutS: int(timeout / time.Second),
		}
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}
