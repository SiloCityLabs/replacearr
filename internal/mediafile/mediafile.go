// Package mediafile provides shared media extension matchers and unwanted-file cleanup.
package mediafile

import (
	"io/fs"
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

// IsSubtitle reports whether path looks like a sidecar subtitle file.
func IsSubtitle(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".srt", ".ass", ".ssa", ".vtt", ".sub", ".idx", ".sup":
		return true
	default:
		return false
	}
}

// HasExtension reports whether path's extension is in the normalized ext list (e.g. ".srt").
func HasExtension(path string, exts []string) bool {
	if len(exts) == 0 {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	return false
}

// ListExtraFiles walks root for files whose extension is in exts (typically ExtraFileExtensions).
// Skips the primary media file itself when primary is under root.
func ListExtraFiles(root, primary string, exts []string) ([]string, error) {
	if len(exts) == 0 {
		return nil, nil
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	primary = filepath.Clean(primary)
	var out []string
	seen := map[string]bool{}
	collect := func(path string) {
		clean := filepath.Clean(path)
		if clean == primary || seen[clean] {
			return
		}
		if !HasExtension(clean, exts) {
			return
		}
		seen[clean] = true
		out = append(out, clean)
	}
	if !info.IsDir() {
		dir := filepath.Dir(root)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			collect(filepath.Join(dir, e.Name()))
		}
		return out, nil
	}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		collect(path)
		return nil
	})
	return out, nil
}

// ExtraDestPath names an imported extra beside the library media file.
// Same-stem companions become "{videoStem}.srt"; others keep a tag, e.g. "{videoStem}.English.srt".
func ExtraDestPath(videoDest, extraSrc string) string {
	dir := filepath.Dir(videoDest)
	videoStem := strings.TrimSuffix(filepath.Base(videoDest), filepath.Ext(videoDest))
	extraBase := filepath.Base(extraSrc)
	extraExt := filepath.Ext(extraBase)
	extraStem := strings.TrimSuffix(extraBase, extraExt)

	// When the extra sits next to a video/audio release, strip that release stem so
	// "Movie.2024.en.srt" → "{videoStem}.en.srt" and "Movie.2024.srt" → "{videoStem}.srt".
	srcVideoStem := companionMediaStem(filepath.Dir(extraSrc))
	if srcVideoStem != "" {
		lowerExtra := strings.ToLower(extraStem)
		lowerVideo := strings.ToLower(srcVideoStem)
		if lowerExtra == lowerVideo {
			return filepath.Join(dir, videoStem+extraExt)
		}
		if strings.HasPrefix(lowerExtra, lowerVideo+".") {
			rest := extraStem[len(srcVideoStem):]
			return filepath.Join(dir, videoStem+rest+extraExt)
		}
	}
	tag := strings.TrimSpace(extraStem)
	if tag == "" {
		return filepath.Join(dir, videoStem+extraExt)
	}
	return filepath.Join(dir, videoStem+"."+tag+extraExt)
}

func companionMediaStem(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var best string
	var bestSize int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !(IsVideo(name) || IsAudio(name) || IsEbook(name) || IsAudiobook(name)) {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		if fi.Size() >= bestSize {
			bestSize = fi.Size()
			best = strings.TrimSuffix(name, filepath.Ext(name))
		}
	}
	return best
}

// MatcherForMediaType returns a path matcher for the given library media type.
func MatcherForMediaType(mediaType string) func(string) bool {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "music":
		return IsAudio
	case "ebook", "book":
		return IsEbook
	case "books":
		// Books library import covers both ebook and audiobook files.
		return func(path string) bool { return IsEbook(path) || IsAudiobook(path) }
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
