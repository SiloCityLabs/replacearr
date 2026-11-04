package subtitles

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestOSHashDeterministic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.bin")
	// 128KB+ file with known pattern
	data := make([]byte, osHashChunkSize*2+8)
	for i := range data {
		data[i] = byte(i % 251)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	h1, size1, err := OSHash(path)
	if err != nil {
		t.Fatal(err)
	}
	h2, size2, err := OSHash(path)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 || size1 != size2 {
		t.Fatalf("hash not stable: %s/%d vs %s/%d", h1, size1, h2, size2)
	}
	if len(h1) != 16 {
		t.Fatalf("expected 16 hex chars, got %q", h1)
	}
	// Manual expected: size + first chunk sum + last chunk sum
	var sum uint64 = uint64(len(data))
	sum += sumUint64LE(data[:osHashChunkSize])
	sum += sumUint64LE(data[len(data)-osHashChunkSize:])
	_ = binary.LittleEndian
	want := formatHash(sum)
	if h1 != want {
		t.Fatalf("hash = %s, want %s", h1, want)
	}
}

func formatHash(v uint64) string {
	return sprintf016(v)
}

func sprintf016(v uint64) string {
	const hexdigits = "0123456789abcdef"
	buf := make([]byte, 16)
	for i := 15; i >= 0; i-- {
		buf[i] = hexdigits[v&0xf]
		v >>= 4
	}
	return string(buf)
}

func TestSidecarPath(t *testing.T) {
	got := SidecarPath("/data/Movies/Film/Film.mkv", "en")
	want := "/data/Movies/Film/Film.en.srt"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}
