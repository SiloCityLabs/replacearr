package mediafile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/replacearr/replacearr/internal/domain"
)

func TestParseAndUnwanted(t *testing.T) {
	exts := ParseExtensions(".exe, bat, .EXE")
	if len(exts) != 2 {
		t.Fatalf("got %#v", exts)
	}
	mm := domain.MediaManagementConfig{
		UnwantedFileExtensions:   ".exe",
		TVUnwantedFileExtensions: ".nfo",
	}
	got := UnwantedExtensions(mm, "tv")
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
}

func TestDeleteUnwantedInDir(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("video.mkv")
	mustWrite("malware.exe")
	mustWrite("readme.txt")
	n, err := DeleteUnwantedInDir(dir, []string{".exe"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("removed %d", n)
	}
	if _, err := os.Stat(filepath.Join(dir, "malware.exe")); !os.IsNotExist(err) {
		t.Fatal("exe should be gone")
	}
	if _, err := os.Stat(filepath.Join(dir, "video.mkv")); err != nil {
		t.Fatal("video should remain")
	}
}
