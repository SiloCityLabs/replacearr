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

func TestIsSubtitleAndExtraDest(t *testing.T) {
	if !IsSubtitle("a.srt") || !IsSubtitle("b.ASS") || IsSubtitle("c.mkv") {
		t.Fatal("IsSubtitle")
	}
	dir := t.TempDir()
	video := filepath.Join(dir, "Movie.2024.mkv")
	side := filepath.Join(dir, "Movie.2024.srt")
	lang := filepath.Join(dir, "Movie.2024.en.srt")
	subs := filepath.Join(dir, "Subs")
	_ = os.MkdirAll(subs, 0o755)
	nested := filepath.Join(subs, "English.srt")
	for _, p := range []string{video, side, lang, nested} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	extras, err := ListExtraFiles(dir, video, ParseExtensions("srt"))
	if err != nil || len(extras) != 3 {
		t.Fatalf("ListExtraFiles: %v %#v", err, extras)
	}
	destVideo := "/data/Movies/Movie (2024)/Movie (2024).mkv"
	if got := ExtraDestPath(destVideo, side); got != "/data/Movies/Movie (2024)/Movie (2024).srt" {
		t.Fatalf("side = %q", got)
	}
	if got := ExtraDestPath(destVideo, lang); got != "/data/Movies/Movie (2024)/Movie (2024).en.srt" {
		t.Fatalf("lang = %q", got)
	}
	if got := ExtraDestPath(destVideo, nested); got != "/data/Movies/Movie (2024)/Movie (2024).English.srt" {
		t.Fatalf("nested = %q", got)
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
