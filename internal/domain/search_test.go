package domain

import (
	"testing"
	"time"
)

func TestSeasonFullyAired(t *testing.T) {
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	eps := []Episode{
		{SeasonNumber: 1, EpisodeNumber: 1, AirDateUtc: now.Add(-48 * time.Hour)},
		{SeasonNumber: 1, EpisodeNumber: 2, AirDateUtc: now.Add(-24 * time.Hour)},
		{SeasonNumber: 2, EpisodeNumber: 1, AirDateUtc: now.Add(48 * time.Hour)},
	}
	if !SeasonFullyAired(eps, 1, now) {
		t.Fatal("season 1 should be fully aired")
	}
	if SeasonFullyAired(eps, 2, now) {
		t.Fatal("season 2 should not be fully aired")
	}
	if SeasonFullyAired(eps, 3, now) {
		t.Fatal("empty season should not be fully aired")
	}
}

func TestPreferSeasonPackQuery(t *testing.T) {
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	eps := []Episode{
		{SeasonNumber: 1, EpisodeNumber: 1, AirDateUtc: now.Add(-time.Hour)},
		{SeasonNumber: 2, EpisodeNumber: 1, AirDateUtc: now.Add(72 * time.Hour)},
	}
	ended := Series{Ended: true, Status: "ended"}
	cont := Series{Ended: false, Status: "continuing"}

	if !PreferSeasonPackQuery(ended, eps, 2, now) {
		t.Fatal("ended series should prefer packs")
	}
	if !PreferSeasonPackQuery(cont, eps, 1, now) {
		t.Fatal("aired season on continuing series should prefer packs")
	}
	if PreferSeasonPackQuery(cont, eps, 2, now) {
		t.Fatal("airing season should not prefer packs alone")
	}
}

func TestSynthesizeSeasonAliases(t *testing.T) {
	got := SynthesizeSeasonAliases("Anime", 2)
	want := map[string]bool{
		"Anime Season 2":   true,
		"Anime 2nd Season": true,
		"Anime S02":        true,
	}
	if len(got) != 3 {
		t.Fatalf("got %v", got)
	}
	for _, g := range got {
		if !want[g] {
			t.Fatalf("unexpected alias %q", g)
		}
	}
}

func TestBuildTVSearchVariants_seasonAliasOmitsSeason(t *testing.T) {
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	s2 := 2
	aliases := []SeriesAlias{
		{Title: "Anime 2nd Season", SeasonNumber: &s2},
		{Title: "SNK"}, // series-wide
	}
	eps := []Episode{
		{SeasonNumber: 2, EpisodeNumber: 1, AirDateUtc: now.Add(-time.Hour)},
		{SeasonNumber: 2, EpisodeNumber: 2, AirDateUtc: now.Add(-time.Hour)},
	}
	vars := BuildTVSearchVariants("Anime", aliases, eps, 2, 0, true, now)

	var foundAlias, foundSeriesAlias bool
	for _, v := range vars {
		if v.Query == "Anime 2nd Season" {
			foundAlias = true
			if v.Season != 0 || v.Episode != 0 {
				t.Fatalf("season alias must omit season/episode, got %+v", v)
			}
		}
		if v.Query == "SNK S02" && v.Season == 2 {
			foundSeriesAlias = true
		}
	}
	if !foundAlias {
		t.Fatalf("missing season alias variant: %+v", vars)
	}
	if !foundSeriesAlias {
		t.Fatalf("missing series-wide alias with season token: %+v", vars)
	}
}

func TestBuildTVSearchVariants_airingFallsBackToEpisodes(t *testing.T) {
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	eps := []Episode{
		{SeasonNumber: 1, EpisodeNumber: 1, AirDateUtc: now.Add(-48 * time.Hour)},
		{SeasonNumber: 1, EpisodeNumber: 2, AirDateUtc: now.Add(72 * time.Hour)},
	}
	vars := BuildTVSearchVariants("Show", nil, eps, 1, 0, false, now)
	var pack, ep1, ep2 bool
	for _, v := range vars {
		switch {
		case v.Query == "Show S01" && v.Season == 1 && v.Episode == 0:
			pack = true
		case v.Query == "Show S01E01" && v.Episode == 1:
			ep1 = true
		case v.Query == "Show S01E02":
			ep2 = true
		}
	}
	if !pack || !ep1 {
		t.Fatalf("expected pack + aired ep, got %+v", vars)
	}
	if ep2 {
		t.Fatal("unaired episode should not be searched")
	}
}

func TestOrdinal(t *testing.T) {
	cases := map[int]string{1: "1st", 2: "2nd", 3: "3rd", 4: "4th", 11: "11th", 21: "21st", 22: "22nd"}
	for n, want := range cases {
		if got := ordinal(n); got != want {
			t.Fatalf("ordinal(%d)=%q want %q", n, got, want)
		}
	}
}
