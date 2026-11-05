package cardigann

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseBytes(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"0", 0},
		{"0 B", 0},
		{"1024", 1024},
		{"1 KB", 1024},
		{"1.5 GB", int64(1.5 * 1024 * 1024 * 1024)},
		{"700MiB", 700 * 1024 * 1024},
		{"1,234 MB", 1234 * 1024 * 1024},
		{"2 TB", 2 * 1024 * 1024 * 1024 * 1024},
	}
	for _, tc := range cases {
		got, err := parseBytes(tc.in, nil)
		if err != nil {
			t.Fatalf("parseBytes(%q): %v", tc.in, err)
		}
		n, err := strconv.ParseInt(got, 10, 64)
		if err != nil {
			t.Fatalf("parseBytes(%q) non-int %q", tc.in, got)
		}
		if n != tc.want {
			t.Errorf("parseBytes(%q)=%d want %d", tc.in, n, tc.want)
		}
	}
}

func TestNormalizeDate(t *testing.T) {
	got, err := normalizeDate("1609459200", nil) // 2021-01-01 UTC
	if err != nil {
		t.Fatal(err)
	}
	tm, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("not RFC3339: %q", got)
	}
	if tm.UTC().Year() != 2021 || tm.UTC().Month() != time.January {
		t.Fatalf("got %v", tm)
	}

	got, err = normalizeDate("2021-10-19T06:10:15.772Z", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "2021-10-19T06:10:15") {
		t.Fatalf("fractional: %q", got)
	}

	got, err = normalizeDate("now", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := time.Parse(time.RFC3339, got); err != nil {
		t.Fatalf("now: %q", got)
	}
}

func TestSearch_HumanSizeAndUnixDate(t *testing.T) {
	dir := t.TempDir()
	yml := `
id: mocksize
name: MockSize
type: public
encoding: UTF-8
links:
  - https://example.invalid/
caps:
  modes:
    search: [q]
settings: []
search:
  paths:
    - path: "search"
  rows:
    selector: 'tr'
  fields:
    title:
      selector: td.title
    details:
      text: "/d/1"
    download:
      text: "/dl/1"
    size:
      text: "1.5 GB"
    seeders:
      text: "10"
    leechers:
      text: "2"
    date:
      text: "1609459200"
`
	if err := os.WriteFile(filepath.Join(dir, "mocksize.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	html := `<html><body><table><tr><td class="title">Sized Release</td></tr></table></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rels, err := Search(ctx, SearchRequest{
		DefinitionsDir: dir,
		DefinitionFile: "mocksize",
		BaseURL:        srv.URL + "/",
		IndexerID:      1,
		IndexerName:    "MockSize",
		Query:          "test",
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("len=%d", len(rels))
	}
	wantSize := int64(1.5 * 1024 * 1024 * 1024)
	if rels[0].Size != wantSize {
		t.Fatalf("size=%d want %d", rels[0].Size, wantSize)
	}
	if rels[0].PublishDate.UTC().Year() != 2021 {
		t.Fatalf("publishDate=%v", rels[0].PublishDate)
	}
	if rels[0].Age < 1000 {
		t.Fatalf("age=%d, expected multi-year age from 2021-01-01", rels[0].Age)
	}
}
