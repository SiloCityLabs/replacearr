package downloadmonitor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/replacearr/replacearr/internal/domain"
)

func TestFindPrimaryVideoAndImport(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "sample.txt")
	_ = os.WriteFile(small, []byte("nope"), 0o644)
	video := filepath.Join(dir, "Show.S01E01.mkv")
	_ = os.WriteFile(video, []byte("videodata-larger-than-txt-file-here"), 0o644)

	got, err := findPrimaryVideo(dir)
	if err != nil || got != video {
		t.Fatalf("findPrimaryVideo: %v %q", err, got)
	}

	destDir := filepath.Join(t.TempDir(), "library", "Season 01")
	_ = os.MkdirAll(destDir, 0o755)
	dest := filepath.Join(destDir, "Show - S01E01.mkv")
	if err := importFile(got, dest, false); err != nil {
		t.Fatalf("importFile: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil || string(data) != "videodata-larger-than-txt-file-here" {
		t.Fatalf("dest content: %v %q", err, data)
	}
}

func TestParseSeasonEpisode(t *testing.T) {
	s, e, ok := parseSeasonEpisode("Severance.S02E03.1080p")
	if !ok || s != 2 || e != 3 {
		t.Fatalf("got %d %d %v", s, e, ok)
	}
}

func TestApplyRemotePathMapping(t *testing.T) {
	mapped := applyRemotePathMapping(nil, "/downloads/a")
	if mapped != "/downloads/a" {
		t.Fatalf("%q", mapped)
	}
}

func TestApplyMovieFormatAndCleanFileName(t *testing.T) {
	got := applyMovieFormat("{Movie Title} ({Release Year}) {Quality Full}", "Avatar: Fire and Ash", 2025, "")
	if got != "Avatar: Fire and Ash (2025)" {
		t.Fatalf("format = %q", got)
	}
	cleaned := cleanFileName(got, domain.MediaManagementConfig{
		ReplaceIllegalCharacters: true,
		ColonReplacementFormat:   "smart",
	})
	if cleaned != "Avatar - Fire and Ash (2025)" {
		t.Fatalf("clean = %q", cleaned)
	}
}

func TestFormatTimeleft(t *testing.T) {
	cases := []struct {
		eta, left int64
		want      string
	}{
		{125, 100, "00:02:05"},
		{3723, 100, "01:02:03"},
		{90061, 100, "1.01:01:01"},
		{-1, 100, ""},
		{-2, 100, ""},
		{8640000, 100, ""},
		{0, 100, ""},
		{0, 0, "00:00:00"},
	}
	for _, tc := range cases {
		if got := formatTimeleft(tc.eta, tc.left); got != tc.want {
			t.Fatalf("formatTimeleft(%d,%d)=%q want %q", tc.eta, tc.left, got, tc.want)
		}
	}
}
