package mediafile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRecycleBin(t *testing.T) {
	if got := ResolveRecycleBin("/custom/bin", "/config"); got != "/custom/bin" {
		t.Fatalf("configured = %q", got)
	}
	got := ResolveRecycleBin("", "/var/lib/replacearr")
	if got != filepath.Join("/var/lib/replacearr", "RecycleBin") {
		t.Fatalf("default = %q", got)
	}
}

func TestChangeRecycleBinPathEmptiesOld(t *testing.T) {
	configDir := t.TempDir()
	oldBin := filepath.Join(configDir, "RecycleBin")
	newBin := filepath.Join(configDir, "RecycleBin-new")
	if err := os.MkdirAll(oldBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldBin, "trashed.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ChangeRecycleBinPath("", newBin, configDir, []string{"/data/Movies"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldBin); !os.IsNotExist(err) {
		t.Fatal("old bin should be removed")
	}
	if st, err := os.Stat(newBin); err != nil || !st.IsDir() {
		t.Fatalf("new bin should exist: %v", err)
	}

	if err := EmptyRecycleBin("/data/Movies", configDir, []string{"/data/Movies"}); err == nil {
		t.Fatal("expected refusal to empty a root folder path")
	}
	if err := EmptyRecycleBin(configDir, configDir, nil); err == nil {
		t.Fatal("expected refusal to empty config dir")
	}
}

func TestValidateUnderRoot(t *testing.T) {
	root := "/data/Movies"
	if _, _, err := ValidateUnderRoot("", root); err == nil {
		t.Fatal("empty path should fail")
	}
	if _, _, err := ValidateUnderRoot(filepath.Join(root, "Film"), ""); err == nil {
		t.Fatal("empty root should fail")
	}
	if _, _, err := ValidateUnderRoot(root, root); err == nil {
		t.Fatal("root itself should fail")
	}
	if _, _, err := ValidateUnderRoot("/tmp/elsewhere", root); err == nil {
		t.Fatal("outside root should fail")
	}
	path, cleanRoot, err := ValidateUnderRoot(filepath.Join(root, "Film (2024)"), root)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Clean(filepath.Join(root, "Film (2024)")) || cleanRoot != filepath.Clean(root) {
		t.Fatalf("got path=%q root=%q", path, cleanRoot)
	}
}

func TestMoveToRecycleBinAndDeleteUnderRoot(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(t.TempDir(), "bin")
	folder := filepath.Join(root, "Show")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(folder, "ep.mkv")
	if err := os.WriteFile(file, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := DeleteUnderRoot(folder, root, bin); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(folder); !os.IsNotExist(err) {
		t.Fatal("source should be gone")
	}
	entries, err := os.ReadDir(bin)
	if err != nil || len(entries) != 1 {
		t.Fatalf("bin entries: %v %#v", err, entries)
	}
	if !strings.HasSuffix(entries[0].Name(), "-Show") {
		t.Fatalf("unexpected bin name %q", entries[0].Name())
	}
	moved := filepath.Join(bin, entries[0].Name(), "ep.mkv")
	if _, err := os.Stat(moved); err != nil {
		t.Fatalf("moved content missing: %v", err)
	}

	if err := DeleteUnderRoot(filepath.Join(root, "missing"), root, bin); err != nil {
		t.Fatalf("missing path should no-op: %v", err)
	}
}

func TestDeleteUnwantedUsesRecycleBin(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.WriteFile(filepath.Join(dir, "keep.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.exe"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := DeleteUnwantedInDir(dir, []string{".exe"}, bin)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "bad.exe")); !os.IsNotExist(err) {
		t.Fatal("exe should leave download dir")
	}
	entries, err := os.ReadDir(bin)
	if err != nil || len(entries) != 1 {
		t.Fatalf("bin: %v %#v", err, entries)
	}
}
