// Package metadata provides Sonarr/Radarr-compatible Add New lookup clients.
// Series search uses SkyHook (TVDB-backed); movie search uses api.radarr.video (TMDB-backed).
package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
)

const (
	defaultSkyHookBase = "https://skyhook.sonarr.tv/v2/tvdb"
	defaultRadarrBase  = "https://api.radarr.video/v1"
	defaultTimeout     = 20 * time.Second
)

var (
	imdbURLRe = regexp.MustCompile(`(?i)imdb\.com/title/(tt\d{7,})`)
	tmdbURLRe = regexp.MustCompile(`(?i)themoviedb\.org/movie/(\d+)`)
	imdbIDRe  = regexp.MustCompile(`(?i)^tt\d{7,}$`)
)

// Service looks up series and movies for Add New flows.
type Service struct {
	httpClient  *http.Client
	skyHookBase string
	radarrBase  string
}

// Option configures Service.
type Option func(*Service)

// WithHTTPClient sets the HTTP client used for upstream calls.
func WithHTTPClient(c *http.Client) Option {
	return func(s *Service) {
		if c != nil {
			s.httpClient = c
		}
	}
}

// WithSkyHookBase overrides the SkyHook base URL (tests).
func WithSkyHookBase(base string) Option {
	return func(s *Service) {
		if base != "" {
			s.skyHookBase = strings.TrimRight(base, "/")
		}
	}
}

// WithRadarrBase overrides the Radarr metadata base URL (tests).
func WithRadarrBase(base string) Option {
	return func(s *Service) {
		if base != "" {
			s.radarrBase = strings.TrimRight(base, "/")
		}
	}
}

// New returns a Service pointed at the public SkyHook and Radarr metadata APIs.
func New(opts ...Option) *Service {
	s := &Service{
		httpClient:  &http.Client{Timeout: defaultTimeout},
		skyHookBase: defaultSkyHookBase,
		radarrBase:  defaultRadarrBase,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// LanguageCode maps a preferred-language setting to a SkyHook three-letter code.
func LanguageCode(preferred string) string {
	p := strings.ToLower(strings.TrimSpace(preferred))
	switch {
	case p == "" || strings.HasPrefix(p, "en"):
		return "eng"
	case strings.HasPrefix(p, "de") || p == "german":
		return "deu"
	case strings.HasPrefix(p, "fr") || p == "french":
		return "fra"
	case strings.HasPrefix(p, "es") || p == "spanish":
		return "spa"
	case strings.HasPrefix(p, "it") || p == "italian":
		return "ita"
	case strings.HasPrefix(p, "ja") || p == "japanese":
		return "jpn"
	case strings.HasPrefix(p, "zh") || p == "chinese":
		return "zho"
	case strings.HasPrefix(p, "ru") || p == "russian":
		return "rus"
	case strings.HasPrefix(p, "pt") || p == "portuguese":
		return "por"
	case len(p) == 3:
		return p
	case len(p) >= 2:
		// Best-effort: common ISO-639-1 → SkyHook often wants 639-2/T; default eng.
		return "eng"
	default:
		return "eng"
	}
}

// SearchSeries performs Sonarr-style series lookup (free-text or tvdb:/imdb:/tmdb: prefixes).
func (s *Service) SearchSeries(ctx context.Context, term, language string) ([]domain.Series, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return []domain.Series{}, nil
	}
	lang := LanguageCode(language)
	lower := strings.ToLower(term)

	if id, ok := parsePrefixedInt(lower, "tvdb:", "tvdbid:"); ok {
		series, _, err := s.GetSeriesByTVDB(ctx, id, lang)
		if err != nil {
			if isNotFound(err) {
				return []domain.Series{}, nil
			}
			return nil, err
		}
		return []domain.Series{series}, nil
	}

	// imdb:/tmdb: and free-text all go through SkyHook search (same as Sonarr).
	u := fmt.Sprintf("%s/search?term=%s&language=%s", s.skyHookBase, url.QueryEscape(strings.ToLower(strings.TrimSpace(term))), url.QueryEscape(lang))
	var shows []skyHookShow
	if err := s.getJSON(ctx, u, &shows); err != nil {
		return nil, fmt.Errorf("skyhook search: %w", err)
	}
	out := make([]domain.Series, 0, len(shows))
	for _, show := range shows {
		out = append(out, mapSeries(show))
	}
	return out, nil
}

// GetSeriesByTVDB fetches a single series by TVDB id, including the episode guide.
func (s *Service) GetSeriesByTVDB(ctx context.Context, tvdbID int, language string) (domain.Series, []domain.Episode, error) {
	if tvdbID <= 0 {
		return domain.Series{}, nil, errNotFound
	}
	lang := LanguageCode(language)
	u := fmt.Sprintf("%s/shows/%s/%d/official", s.skyHookBase, url.PathEscape(lang), tvdbID)
	var show skyHookShow
	if err := s.getJSON(ctx, u, &show); err != nil {
		return domain.Series{}, nil, err
	}
	if show.TvdbID == 0 {
		return domain.Series{}, nil, errNotFound
	}
	return mapSeries(show), mapSkyHookEpisodes(show.Episodes), nil
}

// SearchMovies performs Radarr-style movie lookup (free-text, imdb:, tmdb:, or URLs).
func (s *Service) SearchMovies(ctx context.Context, term string) ([]domain.Movie, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return []domain.Movie{}, nil
	}

	if m := imdbURLRe.FindStringSubmatch(term); len(m) == 2 {
		term = "imdb:" + m[1]
	} else if m := tmdbURLRe.FindStringSubmatch(term); len(m) == 2 {
		term = "tmdb:" + m[1]
	}

	lower := strings.ToLower(term)
	if id, ok := parsePrefixedInt(lower, "tmdb:", "tmdbid:"); ok {
		movie, err := s.GetMovieByTMDB(ctx, id)
		if err != nil {
			if isNotFound(err) {
				return []domain.Movie{}, nil
			}
			return nil, err
		}
		return []domain.Movie{movie}, nil
	}
	if imdbID, ok := parsePrefixedString(lower, "imdb:", "imdbid:"); ok {
		movie, err := s.GetMovieByIMDB(ctx, imdbID)
		if err != nil {
			if isNotFound(err) {
				return []domain.Movie{}, nil
			}
			return nil, err
		}
		return []domain.Movie{movie}, nil
	}

	q := strings.ReplaceAll(term, ".", " ")
	q = strings.ReplaceAll(q, "_", " ")
	u := fmt.Sprintf("%s/search?q=%s", s.radarrBase, url.QueryEscape(q))
	var movies []radarrMovie
	if err := s.getJSON(ctx, u, &movies); err != nil {
		return nil, fmt.Errorf("radarr metadata search: %w", err)
	}
	out := make([]domain.Movie, 0, len(movies))
	for _, m := range movies {
		out = append(out, mapMovie(m))
	}
	return out, nil
}

// GetMovieByTMDB fetches a single movie by TMDB id.
func (s *Service) GetMovieByTMDB(ctx context.Context, tmdbID int) (domain.Movie, error) {
	if tmdbID <= 0 {
		return domain.Movie{}, errNotFound
	}
	u := fmt.Sprintf("%s/movie/%d", s.radarrBase, tmdbID)
	var movie radarrMovie
	if err := s.getJSON(ctx, u, &movie); err != nil {
		return domain.Movie{}, err
	}
	if movie.TmdbID == 0 {
		return domain.Movie{}, errNotFound
	}
	return mapMovie(movie), nil
}

// GetMovieByIMDB fetches a movie by IMDb id (tt…).
func (s *Service) GetMovieByIMDB(ctx context.Context, imdbID string) (domain.Movie, error) {
	imdbID = normalizeImdbID(imdbID)
	if imdbID == "" {
		return domain.Movie{}, errNotFound
	}
	u := fmt.Sprintf("%s/movie/imdb/%s", s.radarrBase, url.PathEscape(imdbID))
	var movies []radarrMovie
	if err := s.getJSON(ctx, u, &movies); err != nil {
		return domain.Movie{}, err
	}
	if len(movies) == 0 || movies[0].TmdbID == 0 {
		return domain.Movie{}, errNotFound
	}
	return mapMovie(movies[0]), nil
}

var errNotFound = fmt.Errorf("metadata: not found")

func isNotFound(err error) bool {
	return err != nil && (err == errNotFound || strings.Contains(err.Error(), "not found"))
}

func (s *Service) getJSON(ctx context.Context, rawURL string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Replacearr/0.1 (+https://github.com/replacearr/replacearr)")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("metadata: upstream %s returned %d", rawURL, resp.StatusCode)
	}
	if len(body) == 0 {
		return errNotFound
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("metadata: decode: %w", err)
	}
	return nil
}

func parsePrefixedInt(lower string, prefixes ...string) (int, bool) {
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			slug := strings.TrimSpace(strings.TrimPrefix(lower, p))
			if slug == "" || strings.ContainsAny(slug, " \t") {
				return 0, false
			}
			id, err := strconv.Atoi(slug)
			if err != nil || id <= 0 {
				return 0, false
			}
			return id, true
		}
	}
	return 0, false
}

func parsePrefixedString(lower string, prefixes ...string) (string, bool) {
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			slug := strings.TrimSpace(strings.TrimPrefix(lower, p))
			if slug == "" || strings.ContainsAny(slug, " \t") {
				return "", false
			}
			return normalizeImdbID(slug), true
		}
	}
	return "", false
}

func normalizeImdbID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	lower := strings.ToLower(id)
	if !strings.HasPrefix(lower, "tt") {
		// Numeric-only → pad like Sonarr/Radarr NormalizeImdbId
		if _, err := strconv.Atoi(id); err == nil {
			return "tt" + id
		}
		return ""
	}
	if !imdbIDRe.MatchString(lower) {
		return ""
	}
	return lower
}
