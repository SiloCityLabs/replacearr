package tasks

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/replacearr/replacearr/internal/db"
)

func TestRecycleCleanupScheduledAndManual(t *testing.T) {
	configDir := t.TempDir()
	store, err := db.Open(filepath.Join(configDir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	bin := filepath.Join(configDir, "RecycleBin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(bin, "old-item")
	freshPath := filepath.Join(bin, "fresh-item")
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(freshPath, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	cfg, err := store.GetAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.MediaManagement.RecycleBin = bin
	cfg.MediaManagement.RecycleBinCleanupDays = 1
	if _, err := store.SaveAppConfig(cfg); err != nil {
		t.Fatal(err)
	}

	s := New(store, configDir, filepath.Join(configDir, "test.db"))
	if err := s.runRecycleCleanup(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("old item should be purged by scheduled cleanup")
	}
	if _, err := os.Stat(freshPath); err != nil {
		t.Fatal("fresh item should remain after scheduled cleanup")
	}

	if err := s.runRecycleCleanup(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(freshPath); !os.IsNotExist(err) {
		t.Fatal("manual cleanup should empty the bin")
	}
}
