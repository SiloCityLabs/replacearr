package subtitles

import (
	"os"
	"path/filepath"
	"strings"
)

// SidecarPath returns the conventional path for a language sidecar next to the video.
// Example: /data/Movies/Film (2020)/Film (2020).en.srt
func SidecarPath(videoPath, language string) string {
	dir := filepath.Dir(videoPath)
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	lang := normalizeLang(language)
	return filepath.Join(dir, base+"."+lang+".srt")
}

// ListSidecarLanguages finds existing .lang.srt (or .lang.ass/.vtt) next to the video.
func ListSidecarLanguages(videoPath string) ([]string, error) {
	dir := filepath.Dir(videoPath)
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var langs []string
	prefix := base + "."
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := strings.TrimPrefix(name, prefix)
		ext := strings.ToLower(filepath.Ext(rest))
		switch ext {
		case ".srt", ".ass", ".ssa", ".vtt", ".sub":
			lang := strings.TrimSuffix(rest, ext)
			if lang != "" {
				langs = append(langs, normalizeLang(lang))
			}
		}
	}
	return langs, nil
}

func normalizeLang(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	lang = strings.ReplaceAll(lang, "_", "-")
	if i := strings.Index(lang, "-"); i > 0 {
		// keep en-us style but prefer primary for matching
		return lang
	}
	return lang
}

func WriteSidecar(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}
