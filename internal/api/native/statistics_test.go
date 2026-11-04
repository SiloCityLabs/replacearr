package native

import "testing"

func TestInferReleaseQuality(t *testing.T) {
	cases := []struct {
		scene, path, want string
	}{
		{"Show.S01E01.1080p.WEB-DL.x264", "", "WEBDL-1080p"},
		{"Movie.2020.2160p.BluRay.Remux", "", "Bluray-2160p Remux"},
		{"", "/data/TV/Show/S01E01.720p.HDTV.mkv", "HDTV-720p"},
		{"Something.1080p.mkv", "", "1080p"},
		{"", "file.mkv", "Unknown"},
	}
	for _, tc := range cases {
		got := inferReleaseQuality(tc.scene, tc.path)
		if got != tc.want {
			t.Fatalf("inferReleaseQuality(%q, %q) = %q, want %q", tc.scene, tc.path, got, tc.want)
		}
	}
}

func TestNormalizeStatuses(t *testing.T) {
	if got := normalizeLibraryStatus("Ended", true); got != "ended" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeMovieStatus("announced"); got != "upcoming" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeMovieStatus("inCinemas"); got != "continuing" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeSeriesType(""); got != "standard" {
		t.Fatalf("got %q", got)
	}
}

func TestInferBookFileType(t *testing.T) {
	if got := inferBookFileType("/books/foo.epub", "ebook"); got != "epub" {
		t.Fatalf("got %q", got)
	}
	if got := inferBookFileType("", "audiobook"); got != "audiobook" {
		t.Fatalf("got %q", got)
	}
}

func TestAverageSize(t *testing.T) {
	if got := averageSize(1000, 4); got != 250 {
		t.Fatalf("got %d", got)
	}
	if got := averageSize(1000, 0); got != 0 {
		t.Fatalf("got %d", got)
	}
}
