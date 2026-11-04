package db

import (
	"path/filepath"
	"testing"

	"github.com/replacearr/replacearr/internal/domain"
)

func TestMusicBooksCRUD(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	artist, err := store.CreateArtist(domain.Artist{
		Title:           "Test Artist",
		ForeignArtistID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Monitored:       true,
		RootFolderPath:  "/data/Music",
	})
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	if artist.ID == 0 {
		t.Fatal("expected artist id")
	}

	err = store.ReplaceArtistDiscography(artist.ID, []domain.AlbumWithTracks{
		{
			Album: domain.Album{Title: "Debut", ForeignAlbumID: "rg-1", Year: 2000, Monitored: true},
			Tracks: []domain.Track{
				{Title: "Track 1", TrackNumber: 1, AbsoluteTrackNumber: 1, Monitored: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("discography: %v", err)
	}
	albums, err := store.ListAlbums(artist.ID)
	if err != nil || len(albums) != 1 {
		t.Fatalf("albums: %v %#v", err, albums)
	}
	tracks, err := store.ListTracks(albums[0].ID, 0)
	if err != nil || len(tracks) != 1 {
		t.Fatalf("tracks: %v %#v", err, tracks)
	}

	author, err := store.CreateAuthor(domain.Author{
		Title:           "Test Author",
		ForeignAuthorID: "/authors/OL1A",
		Monitored:       true,
		RootFolderPath:  "/data/Books",
	})
	if err != nil {
		t.Fatalf("create author: %v", err)
	}
	book, err := store.CreateBook(domain.Book{
		AuthorID:      author.ID,
		Title:         "Test Book",
		ForeignBookID: "/works/OL1W",
		Monitored:     true,
	})
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	ed, err := store.CreateEdition(domain.Edition{
		BookID:           book.ID,
		AuthorID:         author.ID,
		Title:            book.Title,
		Format:           "ebook",
		Monitored:        true,
		RootFolderPath:   "/data/Books",
		QualityProfileID: 1,
	})
	if err != nil {
		t.Fatalf("create edition: %v", err)
	}
	if ed.Format != "ebook" {
		t.Fatalf("format = %q", ed.Format)
	}

	profiles, err := store.ListQualityProfiles()
	if err != nil {
		t.Fatalf("profiles: %v", err)
	}
	foundMusic, foundEbook, foundAudio := false, false, false
	for _, p := range profiles {
		switch p.MediaType {
		case "music":
			foundMusic = true
		case "ebook":
			foundEbook = true
		case "audiobook":
			foundAudio = true
		}
	}
	if !foundMusic || !foundEbook || !foundAudio {
		t.Fatalf("expected seeded music/ebook/audiobook profiles, got %#v", profiles)
	}
}
