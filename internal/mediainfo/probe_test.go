package mediainfo

import "testing"

func TestBucketHeight(t *testing.T) {
	cases := map[int]int{
		2160: 2160,
		1440: 1440,
		1080: 1080,
		1040: 1080, // slight letterbox / crop still maps to 1080p
		720:  720,
		480:  480,
		0:    0,
	}
	for h, want := range cases {
		if got := BucketHeight(h); got != want {
			t.Fatalf("BucketHeight(%d)=%d want %d", h, got, want)
		}
	}
}

func TestAlignQualityToResolution(t *testing.T) {
	cases := []struct {
		inferred string
		res      int
		want     string
	}{
		{"WEBRip-1080p", 1080, "WEBRip-1080p"},
		{"WEBRip-2160p", 1080, "WEBRip-1080p"},
		{"Unknown", 1080, "1080p"},
		{"", 2160, "2160p"},
		{"Bluray Remux", 2160, "Bluray Remux 2160p"},
	}
	for _, tc := range cases {
		if got := AlignQualityToResolution(tc.inferred, tc.res); got != tc.want {
			t.Fatalf("AlignQualityToResolution(%q, %d)=%q want %q", tc.inferred, tc.res, got, tc.want)
		}
	}
}

func TestResolutionFromLabel(t *testing.T) {
	if ResolutionFromLabel("WEBDL-1080p") != 1080 {
		t.Fatal("1080")
	}
	if ResolutionFromLabel("4K UHD") != 2160 {
		t.Fatal("4k")
	}
}

func TestNormalizeLanguageTag(t *testing.T) {
	cases := map[string]string{
		"":          "",
		"und":       "",
		"UND":       "",
		"unknown":   "",
		"eng":       "eng",
		" en ":      "en",
	}
	for in, want := range cases {
		if got := normalizeLanguageTag(in); got != want {
			t.Fatalf("normalizeLanguageTag(%q)=%q want %q", in, got, want)
		}
	}
}

func TestFormatHelpers(t *testing.T) {
	if got := formatBitrate(156_800); got != "156.8 kbit/s" {
		t.Fatalf("bitrate: %q", got)
	}
	if got := formatBitrate(4_400_000); got != "4.4 Mbit/s" {
		t.Fatalf("video bitrate: %q", got)
	}
	if got := formatDuration(5931); got != "1:38:51" {
		t.Fatalf("duration: %q", got)
	}
	if got := formatFPS(23.976023976); got != "23.976" {
		t.Fatalf("fps: %q", got)
	}
}
