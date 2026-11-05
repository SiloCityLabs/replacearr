// Package mediainfo detects video quality/resolution from on-disk media.
// Prefers ffprobe when available; falls back to release-name inference.
package mediainfo

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
)

// Info is the detected media attributes for a video file.
type Info struct {
	Quality    string
	Resolution int
	Width      int
	Height     int
	VideoCodec string
	AudioCodec string
	Languages  string // comma-separated audio language tags
	Source     string // ffprobe | filename
}

var resTokenRe = regexp.MustCompile(`(?i)\b(2160|1080|720|480)p\b|\b4k\b|\buhd\b`)

// ProbeFilename infers quality from the release/file name only (no disk probe).
func ProbeFilename(path, sceneName string) Info {
	inferred := domain.InferReleaseQuality(sceneName, path)
	return Info{
		Quality:    inferred,
		Resolution: ResolutionFromLabel(inferred),
		Source:     "filename",
	}
}

// Probe inspects path and returns quality/resolution. sceneName helps filename inference.
func Probe(path, sceneName string) Info {
	info := ProbeFilename(path, sceneName)
	if probed, ok := probeFF(path); ok {
		info.Width = probed.Width
		info.Height = probed.Height
		info.VideoCodec = probed.VideoCodec
		info.AudioCodec = probed.AudioCodec
		info.Languages = probed.Languages
		info.Source = "ffprobe"
		if probed.Height > 0 {
			bucket := BucketHeight(probed.Height)
			info.Resolution = bucket
			info.Quality = AlignQualityToResolution(info.Quality, bucket)
		}
	}
	return info
}

type ffResult struct {
	Width      int
	Height     int
	VideoCodec string
	AudioCodec string
	Languages  string
}

func probeFF(path string) (ffResult, bool) {
	bin, err := exec.LookPath("ffprobe")
	if err != nil {
		return ffResult{}, false
	}
	// Bound probe size so we don't scan multi-GB files end-to-end on every request.
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin,
		"-v", "error",
		"-probesize", "10M",
		"-analyzeduration", "5M",
		"-print_format", "json",
		"-show_entries", "stream=codec_type,codec_name,width,height:stream_tags=language",
		"-show_streams",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return ffResult{}, false
	}
	var parsed struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Tags      struct {
				Language string `json:"language"`
			} `json:"tags"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return ffResult{}, false
	}
	var res ffResult
	var langs []string
	seen := map[string]bool{}
	for _, s := range parsed.Streams {
		switch s.CodecType {
		case "video":
			if res.Width == 0 && s.Width > 0 {
				res.Width = s.Width
				res.Height = s.Height
				res.VideoCodec = normalizeCodec(s.CodecName)
			}
		case "audio":
			if res.AudioCodec == "" {
				res.AudioCodec = normalizeCodec(s.CodecName)
			}
			lang := normalizeLanguageTag(s.Tags.Language)
			if lang != "" && !seen[lang] {
				seen[lang] = true
				langs = append(langs, lang)
			}
		}
	}
	res.Languages = strings.Join(langs, ", ")
	if res.Width == 0 && res.Height == 0 && res.VideoCodec == "" {
		return ffResult{}, false
	}
	return res, true
}

// normalizeLanguageTag returns a usable language code, or empty for unset/undetermined.
func normalizeLanguageTag(tag string) string {
	lang := strings.ToLower(strings.TrimSpace(tag))
	switch lang {
	case "", "und", "undefined", "unknown", "nul", "null":
		return ""
	default:
		return lang
	}
}

func normalizeCodec(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "h264", "avc":
		return "x264"
	case "hevc", "h265":
		return "x265"
	case "av1":
		return "AV1"
	case "vp9":
		return "VP9"
	case "aac":
		return "AAC"
	case "ac3":
		return "AC3"
	case "eac3":
		return "EAC3"
	case "truehd":
		return "TrueHD"
	case "dts":
		return "DTS"
	case "flac":
		return "FLAC"
	case "opus":
		return "Opus"
	default:
		return name
	}
}

// BucketHeight maps pixel height to a common release resolution.
func BucketHeight(height int) int {
	switch {
	case height >= 2000:
		return 2160
	case height >= 1400:
		return 1440
	case height >= 900:
		return 1080
	case height >= 600:
		return 720
	case height >= 400:
		return 480
	case height > 0:
		return height
	default:
		return 0
	}
}

// ResolutionFromLabel extracts 2160/1080/720/480 from a quality string.
func ResolutionFromLabel(label string) int {
	lower := strings.ToLower(label)
	switch {
	case strings.Contains(lower, "2160") || strings.Contains(lower, "4k") || strings.Contains(lower, "uhd"):
		return 2160
	case strings.Contains(lower, "1440"):
		return 1440
	case strings.Contains(lower, "1080"):
		return 1080
	case strings.Contains(lower, "720"):
		return 720
	case strings.Contains(lower, "480"):
		return 480
	default:
		return 0
	}
}

// AlignQualityToResolution keeps source tags from the filename but corrects the resolution token.
func AlignQualityToResolution(inferred string, resolution int) string {
	if resolution <= 0 {
		return inferred
	}
	token := fmt.Sprintf("%dp", resolution)
	if inferred == "" || inferred == "Unknown" {
		return token
	}
	if ResolutionFromLabel(inferred) == resolution {
		return inferred
	}
	if resTokenRe.MatchString(inferred) {
		return resTokenRe.ReplaceAllString(inferred, token)
	}
	return inferred + " " + token
}

// FormatResolution returns a display label like "1080p".
func FormatResolution(resolution int) string {
	if resolution <= 0 {
		return ""
	}
	return strconv.Itoa(resolution) + "p"
}
