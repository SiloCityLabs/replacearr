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

func TestUpdateAndDeleteMovieFile(t *testing.T) {
	store := openTestStore(t)
	dir := t.TempDir()
	video := filepath.Join(dir, "Film (2021).mkv")
	sidecar := filepath.Join(dir, "Film (2021).en.srt")
	if err := os.WriteFile(video, []byte("fake-video"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sidecar, []byte("1\n00:00:01,000 --> 00:00:02,000\nhi\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	mv, err := store.CreateMovie(domain.Movie{
		Title:            "Film",
		Year:             2021,
		QualityProfileID: 1,
		RootFolderPath:   dir,
		Path:             dir,
		Monitored:        true,
	})
	if err != nil {
		t.Fatalf("create movie: %v", err)
	}
	mf, err := store.UpsertMovieFile(mv.ID, video, "Film.2021.1080p-GROUP", "GROUP", 10)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	updated, err := store.UpdateMovieFile(mf.ID, domain.MovieFileUpdate{
		Quality:      "Bluray-1080p",
		Languages:    "eng",
		Indexer:      "DemoIndexer",
		ReleaseGroup: "NEWGROUP",
		SceneName:    "Film.2021.Bluray",
		VideoCodec:   "x265",
		AudioCodec:   "TrueHD",
		Resolution:   1080,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Quality != "Bluray-1080p" || updated.Languages != "eng" || updated.Indexer != "DemoIndexer" {
		t.Fatalf("unexpected update: %+v", updated)
	}

	extras, err := store.ListMovieExtraFiles(mv.ID)
	if err != nil || len(extras) == 0 {
		t.Fatalf("extras: %v len=%d", err, len(extras))
	}
	if err := store.DeleteMovieExtraFile(mv.ID, "Film (2021).en.srt"); err != nil {
		t.Fatalf("delete extra: %v", err)
	}
	if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
		t.Fatalf("sidecar still present: %v", err)
	}

	if err := store.DeleteMovieFile(mf.ID, true); err != nil {
		t.Fatalf("delete media: %v", err)
	}
	if _, err := os.Stat(video); !os.IsNotExist(err) {
		t.Fatalf("video still present: %v", err)
	}
	got, err := store.GetMovieByID(mv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.HasFile || got.MovieFileID != 0 {
		t.Fatalf("movie still linked: %+v", got)
	}
}
