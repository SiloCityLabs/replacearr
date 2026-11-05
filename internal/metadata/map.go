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
	TvdbID             int                  `json:"tvdbId"`
	Title              string               `json:"title"`
	Overview           string               `json:"overview"`
	Slug               string               `json:"slug"`
	FirstAired         string               `json:"firstAired"`
	TvRageID           *int                 `json:"tvRageId"`
	TvMazeID           *int                 `json:"tvMazeId"`
	TmdbID             *int                 `json:"tmdbId"`
	Status             string               `json:"status"`
	Runtime            *int                 `json:"runtime"`
	Network            string               `json:"network"`
	ImdbID             string               `json:"imdbId"`
	Genres             []string             `json:"genres"`
	Images             []skyHookImage       `json:"images"`
	Seasons            []skyHookSeason      `json:"seasons"`
	Episodes           []skyHookEpisode     `json:"episodes"`
	AlternativeTitles  []skyHookAltTitle    `json:"alternativeTitles"`
	TimeOfDay          *skyHookTimeOfDay    `json:"timeOfDay"`
}

type skyHookAltTitle struct {
	Title string `json:"title"`
}

type skyHookImage struct {
	CoverType string `json:"coverType"`
	URL       string `json:"url"`
}

type skyHookSeason struct {
	SeasonNumber int    `json:"seasonNumber"`
	Title        string `json:"title"`
	Monitored    bool   `json:"monitored"`
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

// radarrMovie matches api.radarr.video PascalCase JSON.
type radarrMovie struct {
	TmdbID             int                     `json:"TmdbId"`
	ImdbID             string                  `json:"ImdbId"`
	Title              string                  `json:"Title"`
	OriginalTitle      string                  `json:"OriginalTitle"`
	OriginalLanguage   string                  `json:"OriginalLanguage"`
	Overview           string                  `json:"Overview"`
	TitleSlug          string                  `json:"TitleSlug"`
	Runtime            *int                    `json:"Runtime"`
	Year               int                     `json:"Year"`
	Studio             string                  `json:"Studio"`
	Status             string                  `json:"Status"`
	Website            string                  `json:"Homepage"`
	YouTubeTrailerID   string                  `json:"YoutubeTrailerId"`
	InCinema           *time.Time              `json:"InCinema"`
	PhysicalRelease    *time.Time              `json:"PhysicalRelease"`
	DigitalRelease     *time.Time              `json:"DigitalRelease"`
	Genres             []string                `json:"Genres"`
	Images             []radarrImage           `json:"Images"`
	Certifications     []radarrCertification   `json:"Certifications"`
	Collection         *radarrCollection       `json:"Collection"`
	MovieRatings       *radarrMovieRatings     `json:"MovieRatings"`
	Credits            *radarrCredits          `json:"Credits"`
	AlternativeTitles  []radarrAltTitle        `json:"AlternativeTitles"`
}

type radarrImage struct {
	CoverType string `json:"CoverType"`
	URL       string `json:"Url"`
}

type radarrCertification struct {
	Country string `json:"Country"`
	Certification string `json:"Certification"`
}

type radarrCollection struct {
	TmdbID int    `json:"TmdbId"`
	Name   string `json:"Name"`
}

type radarrRating struct {
	Count int     `json:"Count"`
	Value float64 `json:"Value"`
	Type  string  `json:"Type"`
}

type radarrMovieRatings struct {
	Tmdb           *radarrRating `json:"Tmdb"`
	Imdb           *radarrRating `json:"Imdb"`
	Metacritic     *radarrRating `json:"Metacritic"`
	RottenTomatoes *radarrRating `json:"RottenTomatoes"`
	Trakt          *radarrRating `json:"Trakt"`
}

type radarrAltTitle struct {
	Title    string `json:"Title"`
	Type     string `json:"Type"`
	Language string `json:"Language"`
}

type radarrCredits struct {
	Cast []radarrPerson `json:"Cast"`
	Crew []radarrPerson `json:"Crew"`
}

type radarrPerson struct {
	Name       string        `json:"Name"`
	Order      int           `json:"Order"`
	Character  string        `json:"Character"`
	Job        string        `json:"Job"`
	Department string        `json:"Department"`
	TmdbID     int           `json:"TmdbId"`
	Images     []radarrImage `json:"Images"`
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
		OriginalLanguage:    strings.ToLower(strings.TrimSpace(m.OriginalLanguage)),
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
		YouTubeTrailerID:    m.YouTubeTrailerID,
		Certification:       pickCertification(m.Certifications),
		Monitored:           true,
		MinimumAvailability: "released",
		IsAvailable:         true,
		Genres:              append([]string(nil), m.Genres...),
		Images:              mapRadarrImages(m.Images),
		Ratings:             mapMovieRatings(m.MovieRatings),
		Credits:             mapMovieCredits(m.Credits),
		AlternateTitles:     mapMovieAltTitles(m.AlternativeTitles),
	}
	if m.Collection != nil && (m.Collection.TmdbID > 0 || m.Collection.Name != "") {
		out.Collection = &domain.MovieCollection{
			TmdbID: m.Collection.TmdbID,
			Name:   m.Collection.Name,
		}
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

func pickCertification(certs []radarrCertification) string {
	if len(certs) == 0 {
		return ""
	}
	prefer := []string{"US", "GB", "CA", "AU"}
	for _, country := range prefer {
		for _, c := range certs {
			if strings.EqualFold(c.Country, country) && strings.TrimSpace(c.Certification) != "" {
				return strings.TrimSpace(c.Certification)
			}
		}
	}
	for _, c := range certs {
		if strings.TrimSpace(c.Certification) != "" {
			return strings.TrimSpace(c.Certification)
		}
	}
	return ""
}

func mapRating(r *radarrRating) *domain.RatingValue {
	if r == nil {
		return nil
	}
	return &domain.RatingValue{Votes: r.Count, Value: r.Value, Type: r.Type}
}

func mapMovieRatings(r *radarrMovieRatings) domain.MovieRatings {
	if r == nil {
		return domain.MovieRatings{}
	}
	return domain.MovieRatings{
		Tmdb:           mapRating(r.Tmdb),
		Imdb:           mapRating(r.Imdb),
		RottenTomatoes: mapRating(r.RottenTomatoes),
		Metacritic:     mapRating(r.Metacritic),
		Trakt:          mapRating(r.Trakt),
	}
}

func mapMovieAltTitles(alts []radarrAltTitle) []domain.MovieAlternateTitle {
	if len(alts) == 0 {
		return []domain.MovieAlternateTitle{}
	}
	out := make([]domain.MovieAlternateTitle, 0, len(alts))
	seen := map[string]struct{}{}
	for _, a := range alts {
		title := strings.TrimSpace(a.Title)
		if title == "" {
			continue
		}
		key := strings.ToLower(title) + "|" + strings.ToLower(a.Language)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, domain.MovieAlternateTitle{
			Title:    title,
			Type:     a.Type,
			Language: a.Language,
		})
	}
	return out
}

func mapMovieCredits(c *radarrCredits) domain.MovieCredits {
	if c == nil {
		return domain.MovieCredits{Cast: []domain.MoviePersonCredit{}, Crew: []domain.MoviePersonCredit{}}
	}
	return domain.MovieCredits{
		Cast: mapMoviePeople(c.Cast),
		Crew: mapMoviePeople(c.Crew),
	}
}

func mapMoviePeople(people []radarrPerson) []domain.MoviePersonCredit {
	if len(people) == 0 {
		return []domain.MoviePersonCredit{}
	}
	out := make([]domain.MoviePersonCredit, 0, len(people))
	for _, p := range people {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		out = append(out, domain.MoviePersonCredit{
			Name:       name,
			Order:      p.Order,
			Character:  p.Character,
			Job:        p.Job,
			Department: p.Department,
			TmdbID:     p.TmdbID,
			Images:     mapRadarrImages(p.Images),
		})
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

// mapSeriesAliases builds series_aliases rows from SkyHook alternative titles,
// named seasons, and synthesized ordinal season titles (Sonarr-style).
func mapSeriesAliases(show skyHookShow) []domain.SeriesAlias {
	title := strings.TrimSpace(show.Title)
	seen := map[string]struct{}{}
	var out []domain.SeriesAlias
	add := func(aliasTitle string, season *int) {
		aliasTitle = strings.TrimSpace(aliasTitle)
		if aliasTitle == "" || strings.EqualFold(aliasTitle, title) {
			return
		}
		key := strings.ToLower(aliasTitle)
		if season != nil {
			key += fmt.Sprintf("|%d", *season)
		} else {
			key += "|*"
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, domain.SeriesAlias{Title: aliasTitle, SeasonNumber: season})
	}

	for _, alt := range show.AlternativeTitles {
		add(alt.Title, nil)
	}
	for _, season := range show.Seasons {
		if season.SeasonNumber <= 0 {
			continue
		}
		sn := season.SeasonNumber
		if strings.TrimSpace(season.Title) != "" {
			add(season.Title, &sn)
		}
		for _, syn := range domain.SynthesizeSeasonAliases(title, sn) {
			add(syn, &sn)
		}
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
