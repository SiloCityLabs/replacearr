package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/replacearr/replacearr/internal/domain"
)

func TestImportListExclusionAndDiskDelete(t *testing.T) {
	store := openTestStore(t)

	created, err := store.AddImportListExclusion(domain.ImportListExclusion{
		MediaType: "movie",
		TmdbID:    42,
		ImdbID:    "tt0042",
		Title:     "Test Movie",
		Year:      2024,
	})
	if err != nil {
		t.Fatalf("add exclusion: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected exclusion id")
	}

	dup, err := store.AddImportListExclusion(domain.ImportListExclusion{
		MediaType: "movie",
		TmdbID:    42,
		Title:     "Test Movie Again",
		Year:      2024,
	})
	if err != nil {
		t.Fatalf("add duplicate exclusion: %v", err)
	}
	if dup.ID != created.ID {
		t.Fatalf("expected same exclusion id, got %d vs %d", dup.ID, created.ID)
	}

	blocked, err := store.IsImportListExcluded("movie", 42, 0, "")
	if err != nil || !blocked {
		t.Fatalf("expected excluded by tmdb, blocked=%v err=%v", blocked, err)
	}

	dir := t.TempDir()
	movieDir := filepath.Join(dir, "Test Movie (2024)")
	if err := os.MkdirAll(movieDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(movieDir, "movie.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	mv := &domain.Movie{
		Title:          "Test Movie",
		Path:           movieDir,
		RootFolderPath: dir,
	}
	if err := store.DeleteMovieDiskFiles(mv); err != nil {
		t.Fatalf("delete disk files: %v", err)
	}
	if _, err := os.Stat(movieDir); !os.IsNotExist(err) {
		t.Fatalf("expected movie folder removed, err=%v", err)
	}

	outside := &domain.Movie{
		Path:           filepath.Join(os.TempDir(), "not-under-root"),
		RootFolderPath: dir,
	}
	if err := store.DeleteMovieDiskFiles(outside); err == nil {
		t.Fatal("expected refusal for path outside root")
	}
}
