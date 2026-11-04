package subtitles

import (
	"bytes"
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

const (
	openSubtitlesBaseURL = "https://api.opensubtitles.com/api/v1"
	openSubtitlesUA      = "Replacearr v1.0"
)

// OpenSubtitlesClient talks to api.opensubtitles.com REST v1.
type OpenSubtitlesClient struct {
	httpClient *http.Client
	apiKey     string
	username   string
	password   string
	token      string
}

func NewOpenSubtitlesClient(apiKey, username, password string) *OpenSubtitlesClient {
	return &OpenSubtitlesClient{
		httpClient: &http.Client{Timeout: 45 * time.Second},
		apiKey:     strings.TrimSpace(apiKey),
		username:   strings.TrimSpace(username),
		password:   password,
	}
}

func (c *OpenSubtitlesClient) Enabled() bool {
	return c != nil && c.apiKey != ""
}

type osSearchParams struct {
	MovieHash string
	Query     string
	Languages []string
	IMDBID    string
	TMDBID    int
	Season    int
	Episode   int
	Type      string // movie | episode
}

type osSubtitleFile struct {
	FileID   int64  `json:"file_id"`
	FileName string `json:"file_name"`
}

type osSubtitleAttrs struct {
	Language        string           `json:"language"`
	Release         string           `json:"release"`
	HearingImpaired bool             `json:"hearing_impaired"`
	ForeignPartsOnly bool            `json:"foreign_parts_only"`
	MoviehashMatch  bool             `json:"moviehash_match"`
	DownloadCount   int              `json:"download_count"`
	Files           []osSubtitleFile `json:"files"`
	FeatureDetails  struct {
		Title string `json:"title"`
		Year  int    `json:"year"`
	} `json:"feature_details"`
}

type osSearchItem struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Attributes osSubtitleAttrs `json:"attributes"`
}

type osSearchResponse struct {
	Data []osSearchItem `json:"data"`
}

func (c *OpenSubtitlesClient) Search(ctx context.Context, p osSearchParams) ([]domain.SubtitleCandidate, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("opensubtitles api key not configured")
	}
	q := url.Values{}
	if p.MovieHash != "" {
		q.Set("moviehash", p.MovieHash)
	}
	if p.Query != "" {
		q.Set("query", p.Query)
	}
	if len(p.Languages) > 0 {
		q.Set("languages", strings.Join(p.Languages, ","))
	}
	if p.IMDBID != "" {
		imdb := strings.TrimPrefix(strings.ToLower(p.IMDBID), "tt")
		q.Set("imdb_id", imdb)
	}
	if p.TMDBID > 0 {
		q.Set("tmdb_id", strconv.Itoa(p.TMDBID))
	}
	if p.Type != "" {
		q.Set("type", p.Type)
	}
	if p.Season > 0 {
		q.Set("season_number", strconv.Itoa(p.Season))
	}
	if p.Episode > 0 {
		q.Set("episode_number", strconv.Itoa(p.Episode))
	}
	q.Set("order_by", "download_count")
	q.Set("order_direction", "desc")

	var resp osSearchResponse
	if err := c.doJSON(ctx, http.MethodGet, "/subtitles?"+q.Encode(), nil, false, &resp); err != nil {
		return nil, err
	}
	out := make([]domain.SubtitleCandidate, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		if len(attrs.Files) == 0 {
			continue
		}
		file := attrs.Files[0]
		release := attrs.Release
		if release == "" {
			release = file.FileName
		}
		score := attrs.DownloadCount
		if attrs.MoviehashMatch {
			score += 100000
		}
		out = append(out, domain.SubtitleCandidate{
			Provider:        "opensubtitles",
			FileID:          file.FileID,
			Language:        attrs.Language,
			ReleaseName:     release,
			HearingImpaired: attrs.HearingImpaired,
			Forced:          attrs.ForeignPartsOnly,
			HashMatch:       attrs.MoviehashMatch,
			DownloadCount:   attrs.DownloadCount,
			Score:           score,
		})
	}
	return out, nil
}

type osDownloadResponse struct {
	Link         string `json:"link"`
	FileName     string `json:"file_name"`
	Remaining    int    `json:"remaining"`
	Message      string `json:"message"`
	Requests     int    `json:"requests"`
	ResetTime    string `json:"reset_time"`
	ResetTimeUTC string `json:"reset_time_utc"`
}

func (c *OpenSubtitlesClient) Download(ctx context.Context, fileID int64) (content []byte, fileName string, err error) {
	if !c.Enabled() {
		return nil, "", fmt.Errorf("opensubtitles api key not configured")
	}
	if err := c.ensureLogin(ctx); err != nil {
		// Login is optional for some quotas; continue with API key only.
		_ = err
	}
	body := map[string]any{"file_id": fileID, "sub_format": "srt"}
	var resp osDownloadResponse
	if err := c.doJSON(ctx, http.MethodPost, "/download", body, true, &resp); err != nil {
		return nil, "", err
	}
	if resp.Link == "" {
		return nil, "", fmt.Errorf("opensubtitles: empty download link (%s)", resp.Message)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resp.Link, nil)
	if err != nil {
		return nil, "", err
	}
	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(httpResp.Body, 2048))
		return nil, "", fmt.Errorf("opensubtitles download %d: %s", httpResp.StatusCode, string(b))
	}
	data, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, "", err
	}
	name := resp.FileName
	if name == "" {
		name = "subtitle.srt"
	}
	return data, name, nil
}

func (c *OpenSubtitlesClient) ensureLogin(ctx context.Context) error {
	if c.token != "" || c.username == "" || c.password == "" {
		return nil
	}
	var resp struct {
		Token string `json:"token"`
	}
	err := c.doJSON(ctx, http.MethodPost, "/login", map[string]string{
		"username": c.username,
		"password": c.password,
	}, false, &resp)
	if err != nil {
		return err
	}
	c.token = resp.Token
	return nil
}

func (c *OpenSubtitlesClient) doJSON(ctx context.Context, method, path string, body any, auth bool, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, openSubtitlesBaseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Api-Key", c.apiKey)
	req.Header.Set("User-Agent", openSubtitlesUA)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth && c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("opensubtitles %s %s: %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}
