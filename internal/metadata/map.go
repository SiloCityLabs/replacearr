package metadata

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
)

// skyHookShow matches SkyHook camelCase JSON (search + show detail).
type skyHookShow struct {
	TvdbID     int               `json:"tvdbId"`
	Title      string            `json:"title"`
	Overview   string            `json:"overview"`
	Slug       string            `json:"slug"`
	FirstAired string            `json:"firstAired"`
	TvRageID   *int              `json:"tvRageId"`
	TvMazeID   *int              `json:"tvMazeId"`
	TmdbID     *int              `json:"tmdbId"`
	Status     string            `json:"status"`
	Runtime    *int              `json:"runtime"`
	Network    string            `json:"network"`
	ImdbID     string            `json:"imdbId"`
	Genres     []string          `json:"genres"`
	Images     []skyHookImage    `json:"images"`
	Seasons    []skyHookSeason   `json:"seasons"`
	Episodes   []skyHookEpisode  `json:"episodes"`
	TimeOfDay  *skyHookTimeOfDay `json:"timeOfDay"`
}

type skyHookImage struct {
	CoverType string `json:"coverType"`
	URL       string `json:"url"`
}

type skyHookSeason struct {
	SeasonNumber int  `json:"seasonNumber"`
	Monitored    bool `json:"monitored"`
}

type skyHookEpisode struct {
	TvdbID                 int     `json:"tvdbId"`
	SeasonNumber           int     `json:"seasonNumber"`
	EpisodeNumber          int     `json:"episodeNumber"`
	AbsoluteEpisodeNumber  *int    `json:"absoluteEpisodeNumber"`
	Title                  string  `json:"title"`
	Overview               string  `json:"overview"`
	AirDate                string  `json:"airDate"`
	AirDateUtc             string  `json:"airDateUtc"`
	FinaleType             string  `json:"finaleType"`
}

type skyHookTimeOfDay struct {
	Hours   int `json:"hours"`
	Minutes int `json:"minutes"`
}

// radarrMovie matches api.radarr.video PascalCase (and camelCase) JSON.
type radarrMovie struct {
	TmdbID          int            `json:"TmdbId"`
	ImdbID          string         `json:"ImdbId"`
	Title           string         `json:"Title"`
	OriginalTitle   string         `json:"OriginalTitle"`
	Overview        string         `json:"Overview"`
	TitleSlug       string         `json:"TitleSlug"`
	Runtime         *int           `json:"Runtime"`
	Year            int            `json:"Year"`
	Studio          string         `json:"Studio"`
	Status          string         `json:"Status"`
	Website         string         `json:"Homepage"`
	InCinema        *time.Time     `json:"InCinema"`
	PhysicalRelease *time.Time     `json:"PhysicalRelease"`
	DigitalRelease  *time.Time     `json:"DigitalRelease"`
	Genres          []string       `json:"Genres"`
	Images          []radarrImage  `json:"Images"`
}

type radarrImage struct {
	CoverType string `json:"CoverType"`
	URL       string `json:"Url"`
}

func mapSeries(show skyHookShow) domain.Series {
	year := 0
	if show.FirstAired != "" {
		if t, err := time.Parse("2006-01-02", show.FirstAired); err == nil {
			year = t.Year()
		}
	}
	runtime := 0
	if show.Runtime != nil {
		runtime = *show.Runtime
	}
	status := strings.ToLower(strings.TrimSpace(show.Status))
	if status == "" {
		status = "continuing"
	}
	airTime := ""
	if show.TimeOfDay != nil {
		airTime = formatAirTime(show.TimeOfDay.Hours, show.TimeOfDay.Minutes)
	}

	s := domain.Series{
		Title:      show.Title,
		SortTitle:  strings.ToLower(show.Title),
		CleanTitle: strings.ToLower(show.Title),
		Status:     status,
		Ended:      status == "ended",
		Overview:   show.Overview,
		Network:    show.Network,
		AirTime:    airTime,
		Year:       year,
		Runtime:    runtime,
		TvdbID:     show.TvdbID,
		ImdbID:     show.ImdbID,
		TitleSlug:  show.Slug,
		SeriesType: "standard",
		Monitored:  true,
		SeasonFolder: true,
		Genres:     append([]string(nil), show.Genres...),
		Images:     mapSkyHookImages(show.Images),
		Seasons:    mapSkyHookSeasons(show.Seasons),
	}
	if show.TvRageID != nil {
		s.TvRageID = *show.TvRageID
	}
	if show.TvMazeID != nil {
		s.TvMazeID = *show.TvMazeID
	}
	if s.TitleSlug == "" && show.TvdbID > 0 {
		s.TitleSlug = strconv.Itoa(show.TvdbID)
	}
	return s
}

func mapMovie(m radarrMovie) domain.Movie {
	runtime := 0
	if m.Runtime != nil {
		runtime = *m.Runtime
	}
	status := strings.ToLower(strings.TrimSpace(m.Status))
	if status == "" {
		status = "released"
	}
	out := domain.Movie{
		Title:               m.Title,
		OriginalTitle:       m.OriginalTitle,
		SortTitle:           strings.ToLower(m.Title),
		CleanTitle:          strings.ToLower(m.Title),
		Status:              status,
		Overview:            m.Overview,
		Website:             m.Website,
		Year:                m.Year,
		Runtime:             runtime,
		ImdbID:              m.ImdbID,
		TmdbID:              m.TmdbID,
		TitleSlug:           m.TitleSlug,
		Studio:              m.Studio,
		Monitored:           true,
		MinimumAvailability: "released",
		IsAvailable:         true,
		Genres:              append([]string(nil), m.Genres...),
		Images:              mapRadarrImages(m.Images),
	}
	if m.InCinema != nil {
		out.InCinemas = m.InCinema.UTC().Format(time.RFC3339)
	}
	if m.PhysicalRelease != nil {
		out.PhysicalRelease = m.PhysicalRelease.UTC().Format(time.RFC3339)
	}
	if m.DigitalRelease != nil {
		out.DigitalRelease = m.DigitalRelease.UTC().Format(time.RFC3339)
	}
	if out.TitleSlug == "" && m.TmdbID > 0 {
		out.TitleSlug = strconv.Itoa(m.TmdbID)
	}
	if out.FolderName == "" {
		out.FolderName = out.Title
	}
	return out
}

func mapSkyHookImages(images []skyHookImage) []domain.MediaImage {
	if len(images) == 0 {
		return nil
	}
	out := make([]domain.MediaImage, 0, len(images))
	for _, img := range images {
		if img.URL == "" {
			continue
		}
		out = append(out, domain.MediaImage{
			CoverType: normalizeCoverType(img.CoverType),
			RemoteURL: img.URL,
		})
	}
	return out
}

func mapRadarrImages(images []radarrImage) []domain.MediaImage {
	if len(images) == 0 {
		return nil
	}
	out := make([]domain.MediaImage, 0, len(images))
	for _, img := range images {
		if img.URL == "" {
			continue
		}
		out = append(out, domain.MediaImage{
			CoverType: normalizeCoverType(img.CoverType),
			RemoteURL: img.URL,
		})
	}
	return out
}

func mapSkyHookSeasons(seasons []skyHookSeason) []domain.Season {
	if len(seasons) == 0 {
		return nil
	}
	out := make([]domain.Season, 0, len(seasons))
	seen := map[int]struct{}{}
	for _, s := range seasons {
		if _, ok := seen[s.SeasonNumber]; ok {
			continue
		}
		seen[s.SeasonNumber] = struct{}{}
		out = append(out, domain.Season{
			SeasonNumber: s.SeasonNumber,
			Monitored:    s.SeasonNumber > 0,
		})
	}
	return out
}

func mapSkyHookEpisodes(episodes []skyHookEpisode) []domain.Episode {
	if len(episodes) == 0 {
		return nil
	}
	out := make([]domain.Episode, 0, len(episodes))
	for _, ep := range episodes {
		mapped := domain.Episode{
			TvdbID:        ep.TvdbID,
			SeasonNumber:  ep.SeasonNumber,
			EpisodeNumber: ep.EpisodeNumber,
			Title:         ep.Title,
			Overview:      ep.Overview,
			AirDate:       ep.AirDate,
			Monitored:     ep.SeasonNumber > 0,
		}
		if ep.AbsoluteEpisodeNumber != nil {
			mapped.AbsoluteEpisodeNum = *ep.AbsoluteEpisodeNumber
		}
		if ep.AirDateUtc != "" {
			if t, err := time.Parse(time.RFC3339, ep.AirDateUtc); err == nil {
				mapped.AirDateUtc = t.UTC()
			}
		}
		out = append(out, mapped)
	}
	return out
}

func normalizeCoverType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "poster":
		return "poster"
	case "banner":
		return "banner"
	case "fanart", "background":
		return "fanart"
	case "clearlogo", "logo":
		return "clearlogo"
	case "headshot":
		return "headshot"
	default:
		if raw == "" {
			return "poster"
		}
		return strings.ToLower(raw)
	}
}

func formatAirTime(hours, minutes int) string {
	if hours < 0 {
		hours = 0
	}
	if minutes < 0 {
		minutes = 0
	}
	return fmt.Sprintf("%02d:%02d", hours, minutes)
}
