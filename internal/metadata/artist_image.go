package metadata

import (
	"context"
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

const (
	defaultWikidataSPARQL = "https://query.wikidata.org/sparql"
	defaultDeezerBase     = "https://api.deezer.com"
	// Thumbnail width for Commons Special:FilePath — Music grid renders ~54px
	// (≈108 @2x); 128 covers that without pulling full-size portraits.
	artistThumbWidth = 128
	artistImageUA    = "Replacearr/1.0 (https://github.com/replacearr/replacearr; music artist thumbs)"

)

// Tunables (vars so tests can shrink delays).
var (
	artistImageMaxAttempts = 4
	artistImageBaseDelay   = time.Second
	artistImageMaxDelay    = 30 * time.Second
	artistImageMinInterval = time.Second // polite spacing between requests
)

// Shared polite limiter for Wikidata / Commons / Deezer (~1 req/s).
var artistImageLimiter = struct {
	mu   sync.Mutex
	last time.Time
}{}

func waitArtistImage(ctx context.Context) error {
	artistImageLimiter.mu.Lock()
	defer artistImageLimiter.mu.Unlock()
	wait := artistImageMinInterval - time.Since(artistImageLimiter.last)
	if wait > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	artistImageLimiter.last = time.Now()
	return nil
}

func artistImageBackoffDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	// attempt 0 → 1s, 1 → 2s, 2 → 4s, …
	d := artistImageBaseDelay << attempt
	if d > artistImageMaxDelay {
		return artistImageMaxDelay
	}
	return d
}

func sleepArtistImage(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func isTransientArtistImageStatus(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusServiceUnavailable ||
		code == http.StatusBadGateway ||
		code == http.StatusGatewayTimeout
}

// artistImageDoGET performs GET with polite spacing and exponential backoff on
// transient failures (network errors, 429, 502/503/504).
func (s *Service) artistImageDoGET(ctx context.Context, rawURL string, accept string) ([]byte, int, error) {
	var lastErr error
	for attempt := 0; attempt < artistImageMaxAttempts; attempt++ {
		if attempt > 0 {
			if err := sleepArtistImage(ctx, artistImageBackoffDelay(attempt-1)); err != nil {
				return nil, 0, err
			}
		}
		if err := waitArtistImage(ctx); err != nil {
			return nil, 0, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("User-Agent", artistImageUA)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		res, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		_ = res.Body.Close()
		if isTransientArtistImageStatus(res.StatusCode) {
			lastErr = fmt.Errorf("status %d", res.StatusCode)
			continue
		}
		return body, res.StatusCode, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("artist image request failed")
	}
	return nil, 0, lastErr
}

// ResolveArtistImage returns a remote thumbnail URL for an artist.
// Prefer Wikidata P18 via MusicBrainz ID (P434) → Commons width=128;
// fall back to Deezer picture_small matched by artist name.
// Returns ("", nil) when no image is found. Transient upstream errors are
// retried with exponential backoff; if retries are exhausted, the last error
// is returned so callers can back off before trying again.
//
// TheAudioDB is intentionally not used: free-tier ToS restricts publishing apps
// without a paid subscription.
func (s *Service) ResolveArtistImage(ctx context.Context, mbid, name string) (string, error) {
	mbid = strings.TrimSpace(mbid)
	name = strings.TrimSpace(name)
	var lastErr error
	if mbid != "" {
		u, err := s.wikidataArtistImage(ctx, mbid)
		if err == nil && u != "" {
			return u, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if name != "" {
		u, err := s.deezerArtistImage(ctx, name)
		if err == nil && u != "" {
			return u, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", nil
}

// ArtistImagePoster builds a MediaImage poster entry for a remote thumb URL.
func ArtistImagePoster(remote string) domain.MediaImage {
	remote = strings.TrimSpace(remote)
	return domain.MediaImage{
		CoverType: "poster",
		URL:       remote,
		RemoteURL: remote,
	}
}

type wikidataSPARQLResponse struct {
	Results struct {
		Bindings []struct {
			Img struct {
				Value string `json:"value"`
			} `json:"img"`
		} `json:"bindings"`
	} `json:"results"`
}

type deezerArtistHit struct {
	Name          string `json:"name"`
	PictureMedium string `json:"picture_medium"`
	PictureSmall  string `json:"picture_small"`
	Picture       string `json:"picture"`
}

type deezerSearchResponse struct {
	Data []deezerArtistHit `json:"data"`
}

func (s *Service) wikidataArtistImage(ctx context.Context, mbid string) (string, error) {
	// P434 = MusicBrainz artist ID; P18 = image.
	query := fmt.Sprintf(`SELECT ?img WHERE {
  ?item wdt:P434 %q .
  ?item wdt:P18 ?img .
} LIMIT 1`, mbid)
	u := s.wikidataSPARQLBase() + "?format=json&query=" + url.QueryEscape(query)
	body, status, err := s.artistImageDoGET(ctx, u, "application/sparql-results+json")
	if err != nil {
		return "", err
	}
	if status >= 300 {
		return "", fmt.Errorf("wikidata sparql status %d", status)
	}
	var parsed wikidataSPARQLResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Results.Bindings) == 0 {
		return "", nil
	}
	raw := strings.TrimSpace(parsed.Results.Bindings[0].Img.Value)
	if raw == "" {
		return "", nil
	}
	return commonsThumbURL(raw, artistThumbWidth), nil
}

// commonsThumbURL turns a Commons FilePath / file URI into a width-limited thumb URL.
func commonsThumbURL(fileURI string, width int) string {
	if width <= 0 {
		width = artistThumbWidth
	}
	fileURI = strings.TrimSpace(fileURI)
	// SPARQL often returns http://commons.wikimedia.org/wiki/Special:FilePath/Name.jpg
	if strings.Contains(fileURI, "Special:FilePath/") {
		if strings.HasPrefix(fileURI, "http://") {
			fileURI = "https://" + strings.TrimPrefix(fileURI, "http://")
		}
		sep := "?"
		if strings.Contains(fileURI, "?") {
			sep = "&"
		}
		return fmt.Sprintf("%s%swidth=%d", fileURI, sep, width)
	}
	name := strings.TrimPrefix(fileURI, "File:")
	name = strings.ReplaceAll(name, " ", "_")
	return fmt.Sprintf("https://commons.wikimedia.org/wiki/Special:FilePath/%s?width=%d", url.PathEscape(name), width)
}

func (s *Service) deezerArtistImage(ctx context.Context, name string) (string, error) {
	u := fmt.Sprintf("%s/search/artist?q=%s&limit=5", s.deezerAPIBase(), url.QueryEscape(name))
	body, status, err := s.artistImageDoGET(ctx, u, "")
	if err != nil {
		return "", err
	}
	if status >= 300 {
		return "", fmt.Errorf("deezer status %d", status)
	}
	var parsed deezerSearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Data) == 0 {
		return "", nil
	}
	pick := parsed.Data[0]
	for _, hit := range parsed.Data {
		if strings.EqualFold(hit.Name, name) {
			pick = hit
			break
		}
	}
	// Prefer small (~56px) for the ~54px grid; fall back to medium/default.
	for _, candidate := range []string{pick.PictureSmall, pick.PictureMedium, pick.Picture} {
		if strings.TrimSpace(candidate) != "" {
			return strings.TrimSpace(candidate), nil
		}
	}
	return "", nil
}

func (s *Service) wikidataSPARQLBase() string {
	if s != nil && s.wikidataSPARQL != "" {
		return strings.TrimRight(s.wikidataSPARQL, "/")
	}
	return defaultWikidataSPARQL
}

func (s *Service) deezerAPIBase() string {
	if s != nil && s.deezerBase != "" {
		return strings.TrimRight(s.deezerBase, "/")
	}
	return defaultDeezerBase
}
