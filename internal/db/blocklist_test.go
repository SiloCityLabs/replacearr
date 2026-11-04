package db

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
)

func TestBlocklistCRUD(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	item, err := store.CreateBlocklist(domain.BlocklistItem{
		MediaType:   "tv",
		SourceTitle: "Bad.Release.S01E01",
		Indexer:     "test",
		Message:     "manual",
		CreatedAt:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.ID == 0 {
		t.Fatal("expected id")
	}

	list, err := store.ListBlocklist("tv")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d items", len(list))
	}

	blocked, err := store.IsBlocklisted("tv", "Bad.Release.S01E01", "")
	if err != nil || !blocked {
		t.Fatalf("expected blocklisted, err=%v blocked=%v", err, blocked)
	}

	if err := store.DeleteBlocklist(item.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = store.ListBlocklist("tv")
	if len(list) != 0 {
		t.Fatalf("expected empty, got %d", len(list))
	}
}
