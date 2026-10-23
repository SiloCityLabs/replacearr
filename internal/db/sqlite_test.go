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
	if err != nil || len(eps) == 0 {
		t.Fatalf("expected seeded episodes for series, got %d (err: %v)", len(eps), err)
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
