package mediainfo

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
)

// ProbeDetails returns display properties for a media file (ffprobe when available).
func ProbeDetails(path string) domain.FileDetails {
	out := domain.FileDetails{
		Path:   path,
		Kind:   "media",
		Source: "stat",
	}
	info, err := os.Stat(path)
	if err != nil {
		out.Properties = []domain.FileDetailProperty{
			{Label: "Path", Value: path},
			{Label: "Error", Value: err.Error()},
		}
		return out
	}
	meta := []domain.FileDetailProperty{
		{Label: "Path", Value: path},
		{Label: "Size", Value: formatBytes(info.Size())},
		{Label: "Modified", Value: info.ModTime().UTC().Format(time.RFC3339)},
	}
	if probed, ok := probeFFDetails(path); ok {
		out.Source = "ffprobe"
		out.Properties = append(probed, meta...)
	} else {
		out.Source = "stat"
		out.Properties = append(meta, domain.FileDetailProperty{
			Label: "Note",
			Value: "ffprobe unavailable; showing file metadata only",
		})
	}
	return out
}

// ExtraFileDetails returns basic filesystem metadata for non-video sidecars.
func ExtraFileDetails(path string, fileType, extension string) domain.FileDetails {
	out := domain.FileDetails{
		Path:   path,
		Kind:   "extra",
		Source: "stat",
	}
	props := []domain.FileDetailProperty{
		{Label: "Path", Value: path},
		{Label: "Name", Value: filepath.Base(path)},
		{Label: "Type", Value: firstNonEmpty(fileType, "other")},
		{Label: "Extension", Value: firstNonEmpty(extension, strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."))},
	}
	info, err := os.Stat(path)
	if err != nil {
		props = append(props, domain.FileDetailProperty{Label: "Error", Value: err.Error()})
		out.Properties = props
		return out
	}
	props = append(props,
		domain.FileDetailProperty{Label: "Size", Value: formatBytes(info.Size())},
		domain.FileDetailProperty{Label: "Modified", Value: info.ModTime().UTC().Format(time.RFC3339)},
		domain.FileDetailProperty{Label: "Permissions", Value: info.Mode().String()},
	)
	out.Properties = props
	return out
}

func probeFFDetails(path string) ([]domain.FileDetailProperty, bool) {
	bin, err := exec.LookPath("ffprobe")
	if err != nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin,
		"-v", "error",
		"-probesize", "50M",
		"-analyzeduration", "30M",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	raw, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	var parsed struct {
		Streams []struct {
			CodecType    string `json:"codec_type"`
			CodecName    string `json:"codec_name"`
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			Channels     int    `json:"channels"`
			BitRate      string `json:"bit_rate"`
			BitsPerRaw   string `json:"bits_per_raw_sample"`
			AvgFrameRate string `json:"avg_frame_rate"`
			RFrameRate   string `json:"r_frame_rate"`
			FieldOrder   string `json:"field_order"`
			Tags         struct {
				Language string `json:"language"`
			} `json:"tags"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
			Size     string `json:"size"`
		} `json:"format"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, false
	}

	var (
		videoCodec, audioCodec string
		width, height          int
		audioChannels          int
		audioBitrate           int64
		videoBitrate           int64
		audioStreamCount       int
		bitDepth               string
		fps                    float64
		scanType               string
		audioLangs             []string
	)
	langSeen := map[string]bool{}

	for _, s := range parsed.Streams {
		switch s.CodecType {
		case "video":
			if width == 0 && s.Width > 0 {
				width = s.Width
				height = s.Height
				videoCodec = normalizeCodec(s.CodecName)
				bitDepth = strings.TrimSpace(s.BitsPerRaw)
				fps = parseFrameRate(s.AvgFrameRate)
				if fps == 0 {
					fps = parseFrameRate(s.RFrameRate)
				}
				scanType = scanTypeFromFieldOrder(s.FieldOrder)
				if br, ok := parseInt64(s.BitRate); ok {
					videoBitrate = br
				}
			}
		case "audio":
			audioStreamCount++
			if audioCodec == "" {
				audioCodec = normalizeCodec(s.CodecName)
				audioChannels = s.Channels
				if br, ok := parseInt64(s.BitRate); ok {
					audioBitrate = br
				}
			}
			lang := normalizeLanguageTag(s.Tags.Language)
			if lang != "" && !langSeen[lang] {
				langSeen[lang] = true
				audioLangs = append(audioLangs, lang)
			}
		}
	}

	containerBitrate, _ := parseInt64(parsed.Format.BitRate)
	if videoBitrate == 0 && containerBitrate > 0 && audioBitrate > 0 && containerBitrate > audioBitrate {
		videoBitrate = containerBitrate - audioBitrate
	} else if videoBitrate == 0 && containerBitrate > 0 {
		videoBitrate = containerBitrate
	}

	props := make([]domain.FileDetailProperty, 0, 12)
	if audioBitrate > 0 {
		props = append(props, domain.FileDetailProperty{Label: "Audio Bitrate", Value: formatBitrate(audioBitrate)})
	}
	if audioChannels > 0 {
		props = append(props, domain.FileDetailProperty{Label: "Audio Channels", Value: strconv.Itoa(audioChannels)})
	}
	if audioCodec != "" {
		props = append(props, domain.FileDetailProperty{Label: "Audio Codec", Value: audioCodec})
	}
	if len(audioLangs) > 0 {
		props = append(props, domain.FileDetailProperty{Label: "Audio Languages", Value: strings.Join(audioLangs, ", ")})
	}
	if audioStreamCount > 0 {
		props = append(props, domain.FileDetailProperty{Label: "Audio Stream Count", Value: strconv.Itoa(audioStreamCount)})
	}
	if bitDepth != "" {
		props = append(props, domain.FileDetailProperty{Label: "Video Bit Depth", Value: bitDepth})
	}
	if videoBitrate > 0 {
		props = append(props, domain.FileDetailProperty{Label: "Video Bitrate", Value: formatBitrate(videoBitrate)})
	}
	if videoCodec != "" {
		props = append(props, domain.FileDetailProperty{Label: "Video Codec", Value: videoCodec})
	}
	if fps > 0 {
		props = append(props, domain.FileDetailProperty{Label: "Video Fps", Value: formatFPS(fps)})
	}
	if width > 0 && height > 0 {
		props = append(props, domain.FileDetailProperty{
			Label: "Resolution",
			Value: fmt.Sprintf("%dx%d", width, height),
		})
	}
	if dur, ok := parseFloat(parsed.Format.Duration); ok && dur > 0 {
		props = append(props, domain.FileDetailProperty{Label: "Run Time", Value: formatDuration(dur)})
	}
	if scanType != "" {
		props = append(props, domain.FileDetailProperty{Label: "Scan Type", Value: scanType})
	}
	return props, len(props) > 0
}

func parseFrameRate(v string) float64 {
	v = strings.TrimSpace(v)
	if v == "" || v == "0/0" {
		return 0
	}
	if strings.Contains(v, "/") {
		parts := strings.SplitN(v, "/", 2)
		num, err1 := strconv.ParseFloat(parts[0], 64)
		den, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 != nil || err2 != nil || den == 0 {
			return 0
		}
		return num / den
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return f
}

func parseInt64(v string) (int64, bool) {
	v = strings.TrimSpace(v)
	if v == "" || v == "N/A" {
		return 0, false
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func parseFloat(v string) (float64, bool) {
	v = strings.TrimSpace(v)
	if v == "" || v == "N/A" {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func formatBitrate(bps int64) string {
	if bps <= 0 {
		return ""
	}
	kbps := float64(bps) / 1000
	if kbps >= 1000 {
		mbps := kbps / 1000
		if mbps == float64(int(mbps)) {
			return fmt.Sprintf("%.0f Mbit/s", mbps)
		}
		return fmt.Sprintf("%.1f Mbit/s", mbps)
	}
	if kbps == float64(int(kbps)) {
		return fmt.Sprintf("%.0f kbit/s", kbps)
	}
	return fmt.Sprintf("%.1f kbit/s", kbps)
}

func formatFPS(fps float64) string {
	if fps <= 0 {
		return ""
	}
	// Prefer common NTSC fractions when close.
	if math.Abs(fps-23.976) < 0.01 {
		return "23.976"
	}
	if math.Abs(fps-29.97) < 0.01 {
		return "29.97"
	}
	if math.Abs(fps-59.94) < 0.01 {
		return "59.94"
	}
	if math.Abs(fps-float64(int(fps+0.5))) < 0.01 {
		return strconv.Itoa(int(fps + 0.5))
	}
	return strconv.FormatFloat(fps, 'f', 3, 64)
}

func formatDuration(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	total := int(math.Round(seconds))
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func formatBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	v := float64(n)
	for _, u := range units {
		v /= 1024
		if v < 1024 {
			if v >= 100 {
				return fmt.Sprintf("%.0f %s", v, u)
			}
			return fmt.Sprintf("%.1f %s", v, u)
		}
	}
	return fmt.Sprintf("%.1f PiB", v/1024)
}

func scanTypeFromFieldOrder(order string) string {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "tt", "bb", "tb", "bt":
		return "Interlaced"
	default:
		return "Progressive"
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
