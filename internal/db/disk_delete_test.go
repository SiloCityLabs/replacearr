package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/replacearr/replacearr/internal/domain"
)

func TestDeleteSeriesArtistBookDiskFiles(t *testing.T) {
	store := openTestStore(t)
	bin := filepath.Join(t.TempDir(), "RecycleBin")

	root := t.TempDir()
	seriesDir := filepath.Join(root, "Show")
	if err := os.MkdirAll(seriesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seriesDir, "ep.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	sr, err := store.CreateSeries(domain.Series{
		Title:            "Show",
		TvdbID:           99,
		QualityProfileID: 1,
		RootFolderPath:   root,
		Path:             seriesDir,
		Monitored:        true,
	})
	if err != nil {
		t.Fatalf("create series: %v", err)
	}
	if err := store.DeleteSeriesDiskFiles(sr, bin); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(seriesDir); !os.IsNotExist(err) {
		t.Fatal("series folder should be moved")
	}

	artistRoot := t.TempDir()
	artistDir := filepath.Join(artistRoot, "Artist")
	if err := os.MkdirAll(artistDir, 0o755); err != nil {
		t.Fatal(err)
	}
	artist, err := store.CreateArtist(domain.Artist{
		Title:           "Artist",
		ForeignArtistID: "mbid-1",
		RootFolderPath:  artistRoot,
		Path:            artistDir,
		Monitored:       true,
	})
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	if err := store.DeleteArtistDiskFiles(artist, bin); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(artistDir); !os.IsNotExist(err) {
		t.Fatal("artist folder should be moved")
	}

	bookRoot := t.TempDir()
	edDir := filepath.Join(bookRoot, "Book Title")
	if err := os.MkdirAll(edDir, 0o755); err != nil {
		t.Fatal(err)
	}
	author, err := store.CreateAuthor(domain.Author{
		Title:           "Author",
		ForeignAuthorID: "/authors/OL9A",
		RootFolderPath:  bookRoot,
		Monitored:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	book, err := store.CreateBook(domain.Book{
		AuthorID:      author.ID,
		Title:         "Book Title",
		ForeignBookID: "/works/OL9W",
		Monitored:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateEdition(domain.Edition{
		BookID:         book.ID,
		AuthorID:       author.ID,
		Title:          book.Title,
		Format:         "ebook",
		Path:           edDir,
		RootFolderPath: bookRoot,
		HasFile:        true,
		Monitored:      true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteBookDiskFiles(book.ID, bin); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(edDir); !os.IsNotExist(err) {
		t.Fatal("edition folder should be moved")
	}

	entries, err := os.ReadDir(bin)
	if err != nil || len(entries) != 3 {
		t.Fatalf("expected 3 recycle entries, got %d err=%v", len(entries), err)
	}
}
