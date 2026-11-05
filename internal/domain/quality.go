package domain

import (
	"path/filepath"
	"strings"
)

// InferReleaseQuality guesses a quality label from a release title / file path.
// Used for search results and library statistics when no explicit quality is stored.
func InferReleaseQuality(sceneName, path string) string {
	hay := strings.ToLower(sceneName + " " + filepath.Base(path))
	switch {
	case strings.Contains(hay, "2160p") && strings.Contains(hay, "remux"):
		return "Bluray-2160p Remux"
	case strings.Contains(hay, "1080p") && strings.Contains(hay, "remux"):
		return "Bluray-1080p Remux"
	case strings.Contains(hay, "2160p") && (strings.Contains(hay, "bluray") || strings.Contains(hay, "blu-ray") || strings.Contains(hay, "bdrip")):
		return "Bluray-2160p"
	case strings.Contains(hay, "1080p") && (strings.Contains(hay, "bluray") || strings.Contains(hay, "blu-ray") || strings.Contains(hay, "bdrip")):
		return "Bluray-1080p"
	case strings.Contains(hay, "720p") && (strings.Contains(hay, "bluray") || strings.Contains(hay, "blu-ray") || strings.Contains(hay, "bdrip")):
		return "Bluray-720p"
	case strings.Contains(hay, "2160p") && (strings.Contains(hay, "webrip") || strings.Contains(hay, "web-rip")):
		return "WEBRip-2160p"
	case strings.Contains(hay, "1080p") && (strings.Contains(hay, "webrip") || strings.Contains(hay, "web-rip")):
		return "WEBRip-1080p"
	case strings.Contains(hay, "720p") && (strings.Contains(hay, "webrip") || strings.Contains(hay, "web-rip")):
		return "WEBRip-720p"
	case strings.Contains(hay, "2160p") && (strings.Contains(hay, "web-dl") || strings.Contains(hay, "webdl") || strings.Contains(hay, "web.dl")):
		return "WEBDL-2160p"
	case strings.Contains(hay, "1080p") && (strings.Contains(hay, "web-dl") || strings.Contains(hay, "webdl") || strings.Contains(hay, "web.dl")):
		return "WEBDL-1080p"
	case strings.Contains(hay, "720p") && (strings.Contains(hay, "web-dl") || strings.Contains(hay, "webdl") || strings.Contains(hay, "web.dl")):
		return "WEBDL-720p"
	case strings.Contains(hay, "1080p") && strings.Contains(hay, "hdtv"):
		return "HDTV-1080p"
	case strings.Contains(hay, "720p") && strings.Contains(hay, "hdtv"):
		return "HDTV-720p"
	case strings.Contains(hay, "2160p") || strings.Contains(hay, "4k") || strings.Contains(hay, "uhd"):
		return "2160p"
	case strings.Contains(hay, "1080p"):
		return "1080p"
	case strings.Contains(hay, "720p"):
		return "720p"
	case strings.Contains(hay, "480p"):
		return "480p"
	default:
		return "Unknown"
	}
}
