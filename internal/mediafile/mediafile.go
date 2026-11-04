// Package mediafile provides shared media extension matchers and unwanted-file cleanup.
package mediafile

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/replacearr/replacearr/internal/domain"
)

func IsVideo(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mkv", ".mp4", ".avi", ".m4v", ".ts", ".m2ts", ".wmv", ".mov":
		return true
	default:
		return false
	}
}

func IsAudio(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".flac", ".mp3", ".aac", ".m4a", ".alac", ".wav", ".ogg", ".opus":
		return true
	default:
		return false
	}
}

func IsEbook(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".epub", ".mobi", ".azw", ".azw3", ".pdf", ".cbz", ".cbr":
		return true
	default:
		return false
	}
}

func IsAudiobook(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".m4b", ".mp3", ".flac", ".m4a", ".aac":
		return true
	default:
		return false
	}
}

// MatcherForMediaType returns a path matcher for the given library media type.
func MatcherForMediaType(mediaType string) func(string) bool {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "music":
		return IsAudio
	case "ebook", "book", "books":
		return IsEbook
	case "audiobook":
		return IsAudiobook
	case "movie", "movies", "tv", "series":
		return IsVideo
	default:
		return IsVideo
	}
}

// ParseExtensions splits a comma-separated extension list into normalized ".ext" values.
func ParseExtensions(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p == "" {
			continue
		}
		if !strings.HasPrefix(p, ".") {
			p = "." + p
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// UnwantedExtensions merges global + per-type unwanted extensions from media management config.
func UnwantedExtensions(mm domain.MediaManagementConfig, mediaType string) []string {
	raw := mm.UnwantedFileExtensions
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "tv", "series":
		raw = joinExt(raw, mm.TVUnwantedFileExtensions)
	case "movie", "movies":
		raw = joinExt(raw, mm.MovieUnwantedFileExtensions)
	case "music":
		raw = joinExt(raw, mm.MusicUnwantedFileExtensions)
	case "ebook", "audiobook", "book", "books":
		raw = joinExt(raw, mm.BookUnwantedFileExtensions)
	}
	return ParseExtensions(raw)
}

func joinExt(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "," + b
}

// DeleteUnwantedInDir removes files under root whose extension is in unwanted.
// If root is a file, only that file's sibling directory is scanned when applicable.
func DeleteUnwantedInDir(root string, unwanted []string, recycleBin string) (int, error) {
	if len(unwanted) == 0 {
		return 0, nil
	}
	set := map[string]bool{}
	for _, e := range unwanted {
		set[e] = true
	}
	info, err := os.Stat(root)
	if err != nil {
		return 0, err
	}
	dir := root
	if !info.IsDir() {
		dir = filepath.Dir(root)
	}
	removed := 0
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !set[ext] {
			continue
		}
		full := filepath.Join(dir, name)
		if recycleBin != "" {
			_ = os.MkdirAll(recycleBin, 0o755)
			dest := filepath.Join(recycleBin, name)
			if err := os.Rename(full, dest); err == nil {
				removed++
				continue
			}
		}
		if err := os.Remove(full); err == nil {
			removed++
		}
	}
	return removed, nil
}
