package libraryimport

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	tmdbTagRe   = regexp.MustCompile(`(?i)\{tmdb[-_ ]?(\d+)\}`)
	imdbTagRe   = regexp.MustCompile(`(?i)\{imdb[-_ ]?(tt\d{7,})\}`)
	yearParenRe = regexp.MustCompile(`\((\d{4})\)`)
	yearTokenRe = regexp.MustCompile(`(?:^|[\s._-])((?:19|20)\d{2})(?:$|[\s._-])`)
	junkRe      = regexp.MustCompile(`(?i)\b(1080p|720p|2160p|480p|4k|uhd|bluray|blu-ray|webrip|web-dl|webdl|hdtv|x264|x265|h264|h265|hevc|avc|dts|aac|atmos|truehd|remux|proper|repack|extended|unrated|directors\.?cut)\b`)
)

// ParsedName is a cleaned title guess extracted from a folder or file name.
type ParsedName struct {
	Title  string
	Year   int
	TmdbID int
	ImdbID string
}

func parseMediaName(raw string) ParsedName {
	name := strings.TrimSpace(raw)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	out := ParsedName{}

	if m := tmdbTagRe.FindStringSubmatch(name); len(m) == 2 {
		if id, err := strconv.Atoi(m[1]); err == nil {
			out.TmdbID = id
		}
		name = tmdbTagRe.ReplaceAllString(name, " ")
	}
	if m := imdbTagRe.FindStringSubmatch(name); len(m) == 2 {
		out.ImdbID = strings.ToLower(m[1])
		name = imdbTagRe.ReplaceAllString(name, " ")
	}
	if m := yearParenRe.FindStringSubmatch(name); len(m) == 2 {
		if y, err := strconv.Atoi(m[1]); err == nil {
			out.Year = y
		}
		name = yearParenRe.ReplaceAllString(name, " ")
	} else if m := yearTokenRe.FindStringSubmatch(name); len(m) == 2 {
		if y, err := strconv.Atoi(m[1]); err == nil {
			out.Year = y
		}
		name = yearTokenRe.ReplaceAllString(name, " ")
	}

	name = junkRe.ReplaceAllString(name, " ")
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.Join(strings.Fields(name), " ")
	out.Title = strings.TrimSpace(name)
	return out
}

// guessNameForFile prefers the parent folder name when the file lives in a
// movie/show folder under the scan root; otherwise it uses the file stem.
func guessNameForFile(scanRoot, filePath string) ParsedName {
	rel, err := filepath.Rel(scanRoot, filePath)
	if err != nil {
		return parseMediaName(filepath.Base(filePath))
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) >= 2 {
		return parseMediaName(parts[0])
	}
	return parseMediaName(filepath.Base(filePath))
}

func lookupTerm(p ParsedName) string {
	if p.TmdbID > 0 {
		return "tmdb:" + strconv.Itoa(p.TmdbID)
	}
	if p.ImdbID != "" {
		return "imdb:" + p.ImdbID
	}
	return strings.TrimSpace(p.Title)
}
