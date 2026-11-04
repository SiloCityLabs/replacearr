package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/replacearr/replacearr/internal/domain"
)

func TestUpsertMovieAndEpisodeFiles(t *testing.T) {
	store := openTestStore(t)
	dir := t.TempDir()
	video := filepath.Join(dir, "Film (2020).mkv")
	if err := os.WriteFile(video, []byte("fake-video"), 0o644); err != nil {
		t.Fatal(err)
	}

	mv, err := store.CreateMovie(domain.Movie{
		Title:            "Film",
		Year:             2020,
		QualityProfileID: 1,
		RootFolderPath:   dir,
		Monitored:        true,
	})
	if err != nil {
		t.Fatalf("create movie: %v", err)
	}
	mf, err := store.UpsertMovieFile(mv.ID, video, "Film.2020.1080p-GROUP", "GROUP", 10)
	if err != nil {
		t.Fatalf("upsert movie file: %v", err)
	}
	if mf.Path != video || mf.SceneName == "" || mf.ReleaseGroup != "GROUP" {
		t.Fatalf("movie file mismatch: %+v", mf)
	}
	got, err := store.GetMovieByID(mv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasFile || got.MovieFileID != mf.ID {
		t.Fatalf("movie not linked: hasFile=%v fileId=%d", got.HasFile, got.MovieFileID)
	}

	sr, err := store.CreateSeries(domain.Series{
		Title:            "Show",
		QualityProfileID: 1,
		RootFolderPath:   dir,
		Path:             dir,
		Monitored:        true,
	})
	if err != nil {
		t.Fatalf("create series: %v", err)
	}
	if err := store.ReplaceEpisodesForSeries(sr.ID, []domain.Episode{{
		SeasonNumber:  1,
		EpisodeNumber: 1,
		Title:         "Pilot",
		Monitored:     true,
	}}); err != nil {
		t.Fatalf("episodes: %v", err)
	}
	eps, err := store.ListEpisodes(sr.ID)
	if err != nil || len(eps) == 0 {
		t.Fatalf("list episodes: %v len=%d", err, len(eps))
	}
	epVideo := filepath.Join(dir, "Show - S01E01.mkv")
	if err := os.WriteFile(epVideo, []byte("ep"), 0o644); err != nil {
		t.Fatal(err)
	}
	ef, err := store.UpsertEpisodeFile(sr.ID, eps[0].ID, epVideo, "Show.S01E01-GROUP", "GROUP", 2)
	if err != nil {
		t.Fatalf("upsert episode file: %v", err)
	}
	ep, err := store.GetEpisodeByID(eps[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ep.HasFile || ep.EpisodeFileID != ef.ID {
		t.Fatalf("episode not linked: %+v", ep)
	}
}
