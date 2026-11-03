package db

import (
	"path/filepath"
	"testing"

	"github.com/replacearr/replacearr/internal/domain"
)

func TestStoreOpenAndCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	roles, err := store.ListRoles()
	if err != nil || len(roles) < 5 {
		t.Fatalf("expected seeded roles, got %d (err: %v)", len(roles), err)
	}

	adminRole, err := store.GetRoleByName("admin")
	if err != nil {
		t.Fatalf("get admin role: %v", err)
	}

	u, err := store.CreateUser("maave", "maave@example.com", "hash123", adminRole.ID)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if u.Username != "maave" || u.RoleName != "admin" {
		t.Fatalf("unexpected user: %+v", u)
	}

	sr, err := store.CreateSeries(domain.Series{
		Title:     "Severance",
		Year:      2022,
		TvdbID:    371980,
		Monitored: true,
	})
	if err != nil || sr.ID == 0 {
		t.Fatalf("create series: %v", err)
	}

	eps, err := store.ListEpisodes(sr.ID)
	if err != nil {
		t.Fatalf("list episodes: %v", err)
	}
	// CreateSeries no longer invents placeholder episodes; metadata sync fills them later.
	if len(eps) != 0 {
		t.Fatalf("expected no auto-seeded episodes, got %d", len(eps))
	}
	if err := store.ReplaceEpisodesForSeries(sr.ID, []domain.Episode{
		{TvdbID: 11, SeasonNumber: 1, EpisodeNumber: 1, Title: "Good News About Hell", Monitored: true},
		{TvdbID: 12, SeasonNumber: 1, EpisodeNumber: 2, Title: "Half Loop", Monitored: true},
	}); err != nil {
		t.Fatalf("replace episodes: %v", err)
	}
	eps, err = store.ListEpisodes(sr.ID)
	if err != nil || len(eps) != 2 {
		t.Fatalf("list after replace: %v len=%d", err, len(eps))
	}
	if err := store.MarkEpisodeHasFile(eps[0].ID, true); err != nil {
		t.Fatalf("mark has file: %v", err)
	}
	if err := store.ReplaceEpisodesForSeries(sr.ID, []domain.Episode{
		{TvdbID: 11, SeasonNumber: 1, EpisodeNumber: 1, Title: "Good News About Hell (updated)", Monitored: true},
		{TvdbID: 12, SeasonNumber: 1, EpisodeNumber: 2, Title: "Half Loop", Monitored: true},
		{TvdbID: 13, SeasonNumber: 1, EpisodeNumber: 3, Title: "In Perpetuity", Monitored: true},
	}); err != nil {
		t.Fatalf("replace episodes again: %v", err)
	}
	eps, err = store.ListEpisodes(sr.ID)
	if err != nil || len(eps) != 3 {
		t.Fatalf("list after second replace: %v len=%d", err, len(eps))
	}
	if !eps[0].HasFile || eps[0].Title != "Good News About Hell (updated)" {
		t.Fatalf("expected preserved hasFile + updated title, got %+v", eps[0])
	}

	mv, err := store.CreateMovie(domain.Movie{
		Title:     "Dune: Part Two",
		Year:      2024,
		TmdbID:    693134,
		Monitored: true,
	})
	if err != nil || mv.ID == 0 {
		t.Fatalf("create movie: %v", err)
	}
}
