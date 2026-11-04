package subtitles

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/domain"
)

// Service is the native in-house subtitle pipeline (OpenSubtitles).
type Service struct {
	store *db.Store
}

func NewService(store *db.Store) *Service {
	return &Service{store: store}
}

func (s *Service) clientFromConfig(cfg domain.AppConfig) *OpenSubtitlesClient {
	osCfg := cfg.Subtitles.OpenSubtitles
	return NewOpenSubtitlesClient(osCfg.APIKey, osCfg.Username, osCfg.Password)
}

func (s *Service) wantedLanguages(cfg domain.AppConfig) []string {
	langs := cfg.Subtitles.Languages
	if len(langs) == 0 {
		if profiles, err := s.store.ListSubtitleProfiles(); err == nil {
			for _, p := range profiles {
				if p.Enabled && len(p.Languages) > 0 {
					return p.Languages
				}
			}
		}
		return []string{"en"}
	}
	out := make([]string, 0, len(langs))
	for _, l := range langs {
		l = normalizeLang(l)
		if l != "" {
			out = append(out, l)
		}
	}
	if len(out) == 0 {
		return []string{"en"}
	}
	return out
}

// OnMovieImported implements downloadmonitor.SubtitleHook.
func (s *Service) OnMovieImported(ctx context.Context, movieID int64) {
	cfg, err := s.store.GetAppConfig()
	if err != nil || !cfg.Subtitles.Enabled || !cfg.Subtitles.AutoDownload {
		return
	}
	if _, err := s.EnsureMovieSubtitles(ctx, movieID); err != nil {
		log.Printf("subtitles: auto movie %d: %v", movieID, err)
	}
}

// OnEpisodeImported implements downloadmonitor.SubtitleHook.
func (s *Service) OnEpisodeImported(ctx context.Context, episodeID int64) {
	cfg, err := s.store.GetAppConfig()
	if err != nil || !cfg.Subtitles.Enabled || !cfg.Subtitles.AutoDownload {
		return
	}
	if _, err := s.EnsureEpisodeSubtitles(ctx, episodeID); err != nil {
		log.Printf("subtitles: auto episode %d: %v", episodeID, err)
	}
}

// MovieStatus returns tracks + missing languages for a movie.
func (s *Service) MovieStatus(movieID int64) (tracks []domain.SubtitleTrack, missing []string, file *domain.MovieFile, err error) {
	cfg, err := s.store.GetAppConfig()
	if err != nil {
		return nil, nil, nil, err
	}
	file, err = s.store.GetMovieFileByMovieID(movieID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil, err
	}
	tracks, err = s.store.ListSubtitleTracksForMovie(movieID)
	if err != nil {
		return nil, nil, file, err
	}
	tracks = s.mergeDiskSidecarsMovie(movieID, file, tracks)
	missing = missingLanguages(s.wantedLanguages(cfg), tracks)
	return tracks, missing, file, nil
}

// EpisodeStatus returns tracks + missing languages for an episode.
func (s *Service) EpisodeStatus(episodeID int64) (tracks []domain.SubtitleTrack, missing []string, file *domain.EpisodeFile, err error) {
	cfg, err := s.store.GetAppConfig()
	if err != nil {
		return nil, nil, nil, err
	}
	file, err = s.store.GetEpisodeFileByEpisodeID(episodeID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil, err
	}
	tracks, err = s.store.ListSubtitleTracksForEpisode(episodeID)
	if err != nil {
		return nil, nil, file, err
	}
	tracks = s.mergeDiskSidecarsEpisode(episodeID, file, tracks)
	missing = missingLanguages(s.wantedLanguages(cfg), tracks)
	return tracks, missing, file, nil
}

func (s *Service) mergeDiskSidecarsMovie(movieID int64, file *domain.MovieFile, tracks []domain.SubtitleTrack) []domain.SubtitleTrack {
	if file == nil || file.Path == "" {
		return tracks
	}
	langs, err := ListSidecarLanguages(file.Path)
	if err != nil {
		return tracks
	}
	have := map[string]bool{}
	for _, t := range tracks {
		have[normalizeLang(t.Language)] = true
	}
	for _, lang := range langs {
		if have[lang] {
			continue
		}
		tracks = append(tracks, domain.SubtitleTrack{
			MediaType:   "movie",
			MovieID:     movieID,
			MovieFileID: file.ID,
			Language:    lang,
			Path:        SidecarPath(file.Path, lang),
			Provider:    "sidecar",
		})
		have[lang] = true
	}
	return tracks
}

func (s *Service) mergeDiskSidecarsEpisode(episodeID int64, file *domain.EpisodeFile, tracks []domain.SubtitleTrack) []domain.SubtitleTrack {
	if file == nil || file.Path == "" {
		return tracks
	}
	langs, err := ListSidecarLanguages(file.Path)
	if err != nil {
		return tracks
	}
	have := map[string]bool{}
	for _, t := range tracks {
		have[normalizeLang(t.Language)] = true
	}
	for _, lang := range langs {
		if have[lang] {
			continue
		}
		tracks = append(tracks, domain.SubtitleTrack{
			MediaType:     "tv",
			EpisodeID:     episodeID,
			EpisodeFileID: file.ID,
			Language:      lang,
			Path:          SidecarPath(file.Path, lang),
			Provider:      "sidecar",
		})
		have[lang] = true
	}
	return tracks
}

func missingLanguages(wanted []string, tracks []domain.SubtitleTrack) []string {
	have := map[string]bool{}
	for _, t := range tracks {
		have[primaryLang(t.Language)] = true
	}
	var missing []string
	for _, w := range wanted {
		if !have[primaryLang(w)] {
			missing = append(missing, w)
		}
	}
	return missing
}

func primaryLang(lang string) string {
	lang = normalizeLang(lang)
	if i := strings.Index(lang, "-"); i > 0 {
		return lang[:i]
	}
	return lang
}

// SearchMovie queries OpenSubtitles for a movie file.
func (s *Service) SearchMovie(ctx context.Context, movieID int64, language string) ([]domain.SubtitleCandidate, error) {
	cfg, err := s.store.GetAppConfig()
	if err != nil {
		return nil, err
	}
	mv, err := s.store.GetMovieByID(movieID)
	if err != nil {
		return nil, err
	}
	file, err := s.store.GetMovieFileByMovieID(movieID)
	if err != nil {
		return nil, fmt.Errorf("no movie file on disk")
	}
	client := s.clientFromConfig(cfg)
	langs := []string{language}
	if language == "" {
		langs = s.wantedLanguages(cfg)
	}
	hash, _, _ := OSHash(file.Path)
	query := strings.TrimSpace(mv.Title)
	if mv.Year > 0 {
		query = fmt.Sprintf("%s %d", query, mv.Year)
	}
	if file.SceneName != "" {
		query = file.SceneName
	}
	cands, err := client.Search(ctx, osSearchParams{
		MovieHash: hash,
		Query:     query,
		Languages: langs,
		IMDBID:    mv.ImdbID,
		TMDBID:    mv.TmdbID,
		Type:      "movie",
	})
	if err != nil {
		return nil, err
	}
	return rankCandidates(cands, cfg), nil
}

// SearchEpisode queries OpenSubtitles for an episode file.
func (s *Service) SearchEpisode(ctx context.Context, episodeID int64, language string) ([]domain.SubtitleCandidate, error) {
	cfg, err := s.store.GetAppConfig()
	if err != nil {
		return nil, err
	}
	eps, err := s.episodeWithSeries(episodeID)
	if err != nil {
		return nil, err
	}
	file, err := s.store.GetEpisodeFileByEpisodeID(episodeID)
	if err != nil {
		return nil, fmt.Errorf("no episode file on disk")
	}
	client := s.clientFromConfig(cfg)
	langs := []string{language}
	if language == "" {
		langs = s.wantedLanguages(cfg)
	}
	hash, _, _ := OSHash(file.Path)
	query := eps.series.Title
	if file.SceneName != "" {
		query = file.SceneName
	}
	cands, err := client.Search(ctx, osSearchParams{
		MovieHash: hash,
		Query:     query,
		Languages: langs,
		IMDBID:    eps.series.ImdbID,
		Season:    eps.episode.SeasonNumber,
		Episode:   eps.episode.EpisodeNumber,
		Type:      "episode",
	})
	if err != nil {
		return nil, err
	}
	return rankCandidates(cands, cfg), nil
}

type episodeCtx struct {
	episode domain.Episode
	series  domain.Series
}

func (s *Service) episodeWithSeries(episodeID int64) (*episodeCtx, error) {
	ep, err := s.store.GetEpisodeByID(episodeID)
	if err != nil {
		return nil, err
	}
	sr, err := s.store.GetSeriesByID(ep.SeriesID)
	if err != nil {
		return nil, err
	}
	return &episodeCtx{episode: *ep, series: *sr}, nil
}

func rankCandidates(cands []domain.SubtitleCandidate, cfg domain.AppConfig) []domain.SubtitleCandidate {
	preferHI := cfg.Subtitles.PreferHearingImpaired
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.HashMatch != b.HashMatch {
			return a.HashMatch
		}
		if preferHI && a.HearingImpaired != b.HearingImpaired {
			return a.HearingImpaired
		}
		return a.Score > b.Score
	})
	return cands
}

// DownloadMovieCandidate downloads a specific OpenSubtitles file for a movie.
func (s *Service) DownloadMovieCandidate(ctx context.Context, movieID int64, fileID int64, language string) (*domain.SubtitleTrack, error) {
	cfg, err := s.store.GetAppConfig()
	if err != nil {
		return nil, err
	}
	file, err := s.store.GetMovieFileByMovieID(movieID)
	if err != nil {
		return nil, fmt.Errorf("no movie file on disk")
	}
	client := s.clientFromConfig(cfg)
	content, name, err := client.Download(ctx, fileID)
	if err != nil {
		return nil, err
	}
	lang := language
	if lang == "" {
		lang = "en"
	}
	dest := SidecarPath(file.Path, lang)
	if err := WriteSidecar(dest, content); err != nil {
		return nil, err
	}
	track := domain.SubtitleTrack{
		MediaType:   "movie",
		MovieID:     movieID,
		MovieFileID: file.ID,
		Language:    normalizeLang(lang),
		Path:        dest,
		Provider:    "opensubtitles",
		ReleaseName: name,
		Score:       0,
	}
	return s.store.CreateSubtitleTrack(track)
}

// DownloadEpisodeCandidate downloads a specific OpenSubtitles file for an episode.
func (s *Service) DownloadEpisodeCandidate(ctx context.Context, episodeID int64, fileID int64, language string) (*domain.SubtitleTrack, error) {
	cfg, err := s.store.GetAppConfig()
	if err != nil {
		return nil, err
	}
	file, err := s.store.GetEpisodeFileByEpisodeID(episodeID)
	if err != nil {
		return nil, fmt.Errorf("no episode file on disk")
	}
	client := s.clientFromConfig(cfg)
	content, name, err := client.Download(ctx, fileID)
	if err != nil {
		return nil, err
	}
	lang := language
	if lang == "" {
		lang = "en"
	}
	dest := SidecarPath(file.Path, lang)
	if err := WriteSidecar(dest, content); err != nil {
		return nil, err
	}
	track := domain.SubtitleTrack{
		MediaType:     "tv",
		EpisodeID:     episodeID,
		EpisodeFileID: file.ID,
		Language:      normalizeLang(lang),
		Path:          dest,
		Provider:      "opensubtitles",
		ReleaseName:   name,
	}
	return s.store.CreateSubtitleTrack(track)
}

// EnsureMovieSubtitles downloads missing languages for a movie.
func (s *Service) EnsureMovieSubtitles(ctx context.Context, movieID int64) ([]domain.SubtitleTrack, error) {
	cfg, err := s.store.GetAppConfig()
	if err != nil {
		return nil, err
	}
	if !cfg.Subtitles.Enabled {
		return nil, nil
	}
	tracks, missing, _, err := s.MovieStatus(movieID)
	if err != nil {
		return nil, err
	}
	if len(missing) == 0 {
		return tracks, nil
	}
	client := s.clientFromConfig(cfg)
	if !client.Enabled() {
		return tracks, fmt.Errorf("opensubtitles api key not configured")
	}
	var downloaded []domain.SubtitleTrack
	for _, lang := range missing {
		cands, err := s.SearchMovie(ctx, movieID, lang)
		if err != nil || len(cands) == 0 {
			continue
		}
		best := pickBest(cands, cfg)
		t, err := s.DownloadMovieCandidate(ctx, movieID, best.FileID, lang)
		if err != nil {
			log.Printf("subtitles: download movie %d %s: %v", movieID, lang, err)
			continue
		}
		downloaded = append(downloaded, *t)
	}
	tracks, _, _, _ = s.MovieStatus(movieID)
	return tracks, nil
}

// EnsureEpisodeSubtitles downloads missing languages for an episode.
func (s *Service) EnsureEpisodeSubtitles(ctx context.Context, episodeID int64) ([]domain.SubtitleTrack, error) {
	cfg, err := s.store.GetAppConfig()
	if err != nil {
		return nil, err
	}
	if !cfg.Subtitles.Enabled {
		return nil, nil
	}
	tracks, missing, _, err := s.EpisodeStatus(episodeID)
	if err != nil {
		return nil, err
	}
	if len(missing) == 0 {
		return tracks, nil
	}
	client := s.clientFromConfig(cfg)
	if !client.Enabled() {
		return tracks, fmt.Errorf("opensubtitles api key not configured")
	}
	for _, lang := range missing {
		cands, err := s.SearchEpisode(ctx, episodeID, lang)
		if err != nil || len(cands) == 0 {
			continue
		}
		best := pickBest(cands, cfg)
		if _, err := s.DownloadEpisodeCandidate(ctx, episodeID, best.FileID, lang); err != nil {
			log.Printf("subtitles: download episode %d %s: %v", episodeID, lang, err)
		}
	}
	tracks, _, _, _ = s.EpisodeStatus(episodeID)
	return tracks, nil
}

func pickBest(cands []domain.SubtitleCandidate, cfg domain.AppConfig) domain.SubtitleCandidate {
	ranked := rankCandidates(cands, cfg)
	return ranked[0]
}
