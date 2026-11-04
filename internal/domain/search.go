package domain

import (
	"fmt"
	"strings"
	"time"
)

const seasonAirGrace = 24 * time.Hour

// MaxEpisodeSearchFallbacks caps per-episode searches when a season is still airing.
const MaxEpisodeSearchFallbacks = 20

// TVSearchVariant is one indexer query produced by BuildTVSearchVariants.
type TVSearchVariant struct {
	Query   string
	Season  int
	Episode int
}

// SeasonFullyAired reports whether every dated episode in season has aired
// (or will within 24h), matching Sonarr's SearchAnimeSeason gate. Applies to all TV.
func SeasonFullyAired(eps []Episode, season int, now time.Time) bool {
	if season <= 0 {
		return false
	}
	cutoff := now.UTC().Add(seasonAirGrace)
	dated := 0
	for _, ep := range eps {
		if ep.SeasonNumber != season {
			continue
		}
		air := episodeAirTime(ep)
		if air.IsZero() {
			continue
		}
		dated++
		if air.After(cutoff) {
			return false
		}
	}
	return dated > 0
}

// PreferSeasonPackQuery returns true when season-pack search is preferred.
// Ended series and fully-aired seasons prefer packs (all TV; not anime-only).
func PreferSeasonPackQuery(sr Series, eps []Episode, season int, now time.Time) bool {
	if sr.Ended || strings.EqualFold(sr.Status, "ended") {
		return true
	}
	if season > 0 {
		return SeasonFullyAired(eps, season, now)
	}
	return false
}

// FormatSeasonEpisodeToken builds an Sxx / SxxExx token for release titles.
// When season>0 and episode==0 it returns a season-pack token (e.g. "S02").
func FormatSeasonEpisodeToken(season, episode int) string {
	if season > 0 && episode > 0 {
		return fmt.Sprintf("S%02dE%02d", season, episode)
	}
	if season > 0 {
		return fmt.Sprintf("S%02d", season)
	}
	return ""
}

// SynthesizeSeasonAliases returns season-identifying titles that should be
// searched without appending a season number (e.g. "Show 2nd Season").
func SynthesizeSeasonAliases(title string, season int) []string {
	title = strings.TrimSpace(title)
	if title == "" || season <= 0 {
		return nil
	}
	out := []string{
		fmt.Sprintf("%s Season %d", title, season),
		fmt.Sprintf("%s %s Season", title, ordinal(season)),
		fmt.Sprintf("%s S%02d", title, season),
	}
	return out
}

// BuildTVSearchVariants plans indexer queries for a TV release search.
// Season-scoped aliases are searched with Season/Episode zero so the alias
// alone identifies the season (Sonarr AnimeSeasonSearchCriteria behavior).
// When season>0, episode==0 and the season is not fully aired, also searches
// individual aired episodes (capped by MaxEpisodeSearchFallbacks).
func BuildTVSearchVariants(
	baseTitle string,
	aliases []SeriesAlias,
	eps []Episode,
	season, episode int,
	preferPack bool,
	now time.Time,
) []TVSearchVariant {
	baseTitle = strings.TrimSpace(baseTitle)
	if baseTitle == "" {
		return nil
	}

	seen := map[string]struct{}{}
	var out []TVSearchVariant
	add := func(q string, s, e int) {
		q = strings.TrimSpace(q)
		if q == "" {
			return
		}
		key := strings.ToLower(q) + fmt.Sprintf("|%d|%d", s, e)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, TVSearchVariant{Query: q, Season: s, Episode: e})
	}

	token := FormatSeasonEpisodeToken(season, episode)
	primaryQuery := baseTitle
	if token != "" {
		primaryQuery = baseTitle + " " + token
	}
	add(primaryQuery, season, episode)

	for _, a := range aliases {
		title := strings.TrimSpace(a.Title)
		if title == "" || strings.EqualFold(title, baseTitle) {
			continue
		}
		if a.SeasonNumber != nil && *a.SeasonNumber > 0 {
			// Season-identifying aliases only apply to season-pack searches for that season.
			if season > 0 && episode == 0 && *a.SeasonNumber == season {
				add(title, 0, 0)
			}
			continue
		}
		aq := title
		if token != "" {
			aq = title + " " + token
		}
		add(aq, season, episode)
	}

	// Still-airing season: keep pack search above, also search aired episodes.
	if season > 0 && episode == 0 && !preferPack {
		n := 0
		cutoff := now.UTC().Add(seasonAirGrace)
		for _, ep := range eps {
			if ep.SeasonNumber != season || ep.EpisodeNumber <= 0 {
				continue
			}
			air := episodeAirTime(ep)
			if air.IsZero() || air.After(cutoff) {
				continue
			}
			epToken := FormatSeasonEpisodeToken(ep.SeasonNumber, ep.EpisodeNumber)
			add(baseTitle+" "+epToken, ep.SeasonNumber, ep.EpisodeNumber)
			n++
			if n >= MaxEpisodeSearchFallbacks {
				break
			}
		}
	}

	return out
}

func episodeAirTime(ep Episode) time.Time {
	if !ep.AirDateUtc.IsZero() {
		return ep.AirDateUtc.UTC()
	}
	if ep.AirDate != "" {
		if t, err := time.Parse("2006-01-02", ep.AirDate); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func ordinal(n int) string {
	if n%100 >= 11 && n%100 <= 13 {
		return fmt.Sprintf("%dth", n)
	}
	switch n % 10 {
	case 1:
		return fmt.Sprintf("%dst", n)
	case 2:
		return fmt.Sprintf("%dnd", n)
	case 3:
		return fmt.Sprintf("%drd", n)
	default:
		return fmt.Sprintf("%dth", n)
	}
}
