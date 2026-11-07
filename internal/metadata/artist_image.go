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
	// Thumbnail width for Commons Special:FilePath — enough for Music grid thumbs.
	artistThumbWidth = 250
	artistImageUA    = "Replacearr/1.0 (https://github.com/replacearr/replacearr; music artist thumbs)"
)

// Shared polite limiter for Wikidata / Commons / Deezer (~1 req/s).
var artistImageLimiter = struct {
	mu   sync.Mutex
	last time.Time
}{}

func waitArtistImage(ctx context.Context) error {
	artistImageLimiter.mu.Lock()
	defer artistImageLimiter.mu.Unlock()
	wait := time.Second - time.Since(artistImageLimiter.last)
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

// ResolveArtistImage returns a remote thumbnail URL for an artist.
// Prefer Wikidata P18 via MusicBrainz ID (P434) → Commons width=250;
// fall back to Deezer picture_medium matched by artist name.
// Returns ("", nil) when no image is found. Rate-limit / 429 errors are soft-failed
// (empty URL) so Add Artist stays responsive.
//
// TheAudioDB is intentionally not used: free-tier ToS restricts publishing apps
// without a paid subscription.
func (s *Service) ResolveArtistImage(ctx context.Context, mbid, name string) (string, error) {
	mbid = strings.TrimSpace(mbid)
	name = strings.TrimSpace(name)
	if mbid != "" {
		if u, err := s.wikidataArtistImage(ctx, mbid); err == nil && u != "" {
			return u, nil
		}
	}
	if name != "" {
		if u, err := s.deezerArtistImage(ctx, name); err == nil && u != "" {
			return u, nil
		}
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
	if err := waitArtistImage(ctx); err != nil {
		return "", err
	}
	// P434 = MusicBrainz artist ID; P18 = image.
	query := fmt.Sprintf(`SELECT ?img WHERE {
  ?item wdt:P434 %q .
  ?item wdt:P18 ?img .
} LIMIT 1`, mbid)
	u := s.wikidataSPARQLBase() + "?format=json&query=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", artistImageUA)
	req.Header.Set("Accept", "application/sparql-results+json")
	res, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode == http.StatusTooManyRequests || res.StatusCode == http.StatusServiceUnavailable {
		return "", nil
	}
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("wikidata sparql status %d", res.StatusCode)
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
	if err := waitArtistImage(ctx); err != nil {
		return "", err
	}
	u := fmt.Sprintf("%s/search/artist?q=%s&limit=5", s.deezerAPIBase(), url.QueryEscape(name))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", artistImageUA)
	res, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode == http.StatusTooManyRequests || res.StatusCode == http.StatusServiceUnavailable {
		return "", nil
	}
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("deezer status %d", res.StatusCode)
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
	// Prefer medium (~250px) for grid thumbs; fall back to small/default.
	for _, candidate := range []string{pick.PictureMedium, pick.PictureSmall, pick.Picture} {
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
