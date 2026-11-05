package subtitles

import (
	"path/filepath"
	"testing"

	"github.com/replacearr/replacearr/internal/domain"
)

func TestCanonicalLang(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"en", "en"},
		{"EN", "en"},
		{"eng", "en"},
		{"english", "en"},
		{"en-US", "en"},
		{"SDH.eng.HI", "en"},
		{"sdh.eng.hi", "en"},
		{"Latin American.spa", "es"},
		{"latin american.spa", "es"},
		{"spanish", "es"},
		{"fr", "fr"},
		{"french", "fr"},
		{"pt-BR", "pt"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := canonicalLang(tc.in); got != tc.want {
			t.Fatalf("canonicalLang(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestMissingLanguagesEnglishSidecars(t *testing.T) {
	tracks := []domain.SubtitleTrack{
		{Language: "english"},
		{Language: "latin american.spa"},
		{Language: "sdh.eng.hi"},
	}
	missing := missingLanguages([]string{"en"}, tracks)
	if len(missing) != 0 {
		t.Fatalf("expected no missing langs, got %v", missing)
	}
	missing = missingLanguages([]string{"en", "fr"}, tracks)
	if len(missing) != 1 || missing[0] != "fr" {
		t.Fatalf("expected missing [fr], got %v", missing)
	}
}

func TestListSidecarLanguagesReleaseNames(t *testing.T) {
	dir := t.TempDir()
	base := "Avatar.Fire.And.Ash.2025.1080p.WEBRip.x265.10bit.AAC5.1-[YTS.BZ]"
	video := filepath.Join(dir, base+".mp4")
	mustWrite(t, video, []byte("x"))
	mustWrite(t, filepath.Join(dir, base+".English.srt"), []byte("1"))
	mustWrite(t, filepath.Join(dir, base+".Latin American.spa.srt"), []byte("1"))
	mustWrite(t, filepath.Join(dir, base+".SDH.eng.HI.srt"), []byte("1"))
	mustWrite(t, filepath.Join(dir, base+".srt"), []byte("1")) // bare sidecar, no lang tag

	langs, err := ListSidecarLanguages(video)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, l := range langs {
		have[canonicalLang(l)] = true
	}
	if !have["en"] {
		t.Fatalf("expected en from English/SDH sidecars, got %v", langs)
	}
	if !have["es"] {
		t.Fatalf("expected es from Latin American.spa, got %v", langs)
	}
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := WriteSidecar(path, data); err != nil {
		t.Fatal(err)
	}
}
