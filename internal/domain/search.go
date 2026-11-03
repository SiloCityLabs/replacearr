package domain

import (
	"fmt"
	"strings"
)

// PreferSeasonPackQuery returns true when a season-pack search is preferred
// over a per-episode query. Ended series prefer packs; continuing series prefer episodes.
func PreferSeasonPackQuery(sr Series) bool {
	if sr.Ended {
		return true
	}
	return strings.EqualFold(sr.Status, "ended")
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
