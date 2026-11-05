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
	return lang
}

// canonicalLang maps sidecar tags and BCP-47 / ISO codes to ISO-639-1 for matching.
// Handles release-style names like "english", "SDH.eng.HI", and "Latin American.spa".
func canonicalLang(lang string) string {
	lang = normalizeLang(lang)
	if lang == "" {
		return ""
	}
	if code := mapToISO6391(lang); code != "" {
		return code
	}
	if i := strings.Index(lang, "-"); i > 0 {
		if code := mapToISO6391(lang[:i]); code != "" {
			return code
		}
	}
	fields := strings.FieldsFunc(lang, func(r rune) bool {
		return r == '.' || r == '-' || r == ' ' || r == '+'
	})
	for _, f := range fields {
		if code := mapToISO6391(f); code != "" {
			return code
		}
	}
	if i := strings.Index(lang, "-"); i > 0 {
		return lang[:i]
	}
	return lang
}

func mapToISO6391(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "en", "eng", "english":
		return "en"
	case "es", "spa", "spanish", "castilian":
		return "es"
	case "fr", "fra", "fre", "french":
		return "fr"
	case "de", "deu", "ger", "german":
		return "de"
	case "it", "ita", "italian":
		return "it"
	case "pt", "por", "portuguese":
		return "pt"
	case "ru", "rus", "russian":
		return "ru"
	case "ja", "jpn", "japanese":
		return "ja"
	case "zh", "zho", "chi", "chinese", "cmn", "mandarin":
		return "zh"
	case "ko", "kor", "korean":
		return "ko"
	case "nl", "nld", "dut", "dutch":
		return "nl"
	case "sv", "swe", "swedish":
		return "sv"
	case "no", "nor", "nob", "nno", "norwegian":
		return "no"
	case "da", "dan", "danish":
		return "da"
	case "fi", "fin", "finnish":
		return "fi"
	case "pl", "pol", "polish":
		return "pl"
	case "cs", "ces", "cze", "czech":
		return "cs"
	case "hu", "hun", "hungarian":
		return "hu"
	case "ro", "ron", "rum", "romanian":
		return "ro"
	case "tr", "tur", "turkish":
		return "tr"
	case "ar", "ara", "arabic":
		return "ar"
	case "he", "heb", "hebrew":
		return "he"
	case "hi", "hin", "hindi":
		return "hi"
	case "th", "tha", "thai":
		return "th"
	case "vi", "vie", "vietnamese":
		return "vi"
	case "uk", "ukr", "ukrainian":
		return "uk"
	case "el", "ell", "gre", "greek":
		return "el"
	case "bg", "bul", "bulgarian":
		return "bg"
	case "hr", "hrv", "croatian":
		return "hr"
	case "sr", "srp", "serbian":
		return "sr"
	case "sk", "slk", "slo", "slovak":
		return "sk"
	case "sl", "slv", "slovenian":
		return "sl"
	case "id", "ind", "indonesian":
		return "id"
	case "ms", "msa", "may", "malay":
		return "ms"
	case "fa", "fas", "per", "persian", "farsi":
		return "fa"
	case "nb":
		return "nb"
	default:
		return ""
	}
}

func WriteSidecar(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}
