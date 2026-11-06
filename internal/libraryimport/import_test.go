package libraryimport

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/domain"
)

func openTestStore(t *testing.T) *db.Store {
	t.Helper()
	dir := t.TempDir()
	store, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestWalkSkipsKnownMoviePathsWithoutMediaInfo(t *testing.T) {
	store := openTestStore(t)
	root := t.TempDir()
	knownVideo := filepath.Join(root, "Known (2020)", "Known.mkv")
	newVideo := filepath.Join(root, "New Film (2021)", "New.mkv")
	if err := os.MkdirAll(filepath.Dir(knownVideo), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(newVideo), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(knownVideo, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newVideo, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	mv, err := store.CreateMovie(domain.Movie{
		Title: "Known", Year: 2020, QualityProfileID: 1, RootFolderPath: root, Path: filepath.Dir(knownVideo), Monitored: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertMovieFile(mv.ID, knownVideo, "Known", "", 1); err != nil {
		t.Fatal(err)
	}

	items, err := Walk(ScanOptions{Store: store, MediaType: "movies", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 unmapped file, got %d %+v", len(items), items)
	}
	if items[0].Path != newVideo {
		t.Fatalf("unexpected path %s", items[0].Path)
	}
}

func TestSessionPageAndRemovePaths(t *testing.T) {
	store := NewSessionStore(DefaultSessionTTL)
	items := make([]ScanItem, 250)
	for i := range items {
		name := "film" + strconv.Itoa(i)
		items[i] = ScanItem{
			Path:         filepath.Join("/data/Movies", name, "f.mkv"),
			RelativePath: filepath.Join(name, "f.mkv"),
		}
	}
	id := store.Create("movies", "/data/Movies", items)
	page, total, err := store.Page(id, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if total != 250 || len(page) != 100 {
		t.Fatalf("page0 total=%d len=%d", total, len(page))
	}
	page2, total, err := store.Page(id, 100, 100)
	if err != nil {
		t.Fatal(err)
	}
	if total != 250 || len(page2) != 100 {
		t.Fatalf("page1 total=%d len=%d", total, len(page2))
	}

	remove := make([]string, 0, 100)
	for _, it := range page {
		remove = append(remove, it.Path)
	}
	remaining, err := store.RemovePaths(id, remove)
	if err != nil {
		t.Fatal(err)
	}
	if remaining != 150 {
		t.Fatalf("remaining=%d", remaining)
	}
	pageAfter, totalAfter, err := store.Page(id, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if totalAfter != 150 || len(pageAfter) != 100 {
		t.Fatalf("after remove total=%d len=%d", totalAfter, len(pageAfter))
	}
}

func TestImportSelectedRejectsOverCap(t *testing.T) {
	store := openTestStore(t)
	items := make([]ImportItem, MaxImportItems+1)
	for i := range items {
		items[i] = ImportItem{Path: "/tmp/x.mkv", TmdbID: 1}
	}
	_, err := ImportSelected(context.Background(), ImportOptions{
		Store:     store,
		MediaType: "movies",
	}, items)
	if err == nil || !strings.Contains(err.Error(), "too many items") {
		t.Fatalf("expected too many items error, got %v", err)
	}
}

func TestMatchItemsNoMetaIsNoop(t *testing.T) {
	items := []ScanItem{{Path: "/a/b.mkv", GuessedTitle: "Test"}}
	out := MatchItems(context.Background(), ScanOptions{MediaType: "movies"}, items)
	if len(out) != 1 || out[0].Match != nil {
		t.Fatalf("expected unchanged items, got %+v", out)
	}
}
