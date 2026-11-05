package cardigann

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSearch_SeedersAndLeechersMapped(t *testing.T) {
	dir := t.TempDir()
	yml := `
id: mockpeers
name: MockPeers
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
      text: "1 GB"
    seeders:
      text: "42"
    leechers:
      text: "7"
    date:
      text: "now"
`
	if err := os.WriteFile(filepath.Join(dir, "mockpeers.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	html := `<html><body><table><tr><td class="title">Test Release</td></tr></table></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rels, err := Search(ctx, SearchRequest{
		DefinitionsDir: dir,
		DefinitionFile: "mockpeers",
		BaseURL:        srv.URL + "/",
		IndexerID:      1,
		IndexerName:    "MockPeers",
		Query:          "test",
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("len=%d", len(rels))
	}
	r := rels[0]
	t.Logf("got seeders=%d leechers=%d peers=%d", r.Seeders, r.Leechers, r.Peers)
	if r.Seeders != 42 {
		t.Errorf("seeders=%d want 42", r.Seeders)
	}
	if r.Leechers != 7 {
		t.Errorf("leechers=%d want 7", r.Leechers)
	}
	if r.Peers != 49 {
		t.Errorf("peers=%d want 49 (seeders+leechers)", r.Peers)
	}
	_ = strings.Contains
}
