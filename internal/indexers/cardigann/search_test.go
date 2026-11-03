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

func TestSearch_MockHTML(t *testing.T) {
	dir := t.TempDir()
	yml := `
id: mockx
name: MockX
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
    - path: "search/{{ .Keywords }}/1/"
  rows:
    selector: 'tr:has(a[href^="/torrent/"])'
  fields:
    title:
      selector: td a[href^="/torrent/"]
    details:
      selector: td a[href^="/torrent/"]
      attribute: href
    download:
      selector: td a[href^="/torrent/"]
      attribute: href
    size:
      text: "1 GB"
    seeders:
      text: "10"
    leechers:
      text: "2"
    date:
      text: "now"
`
	if err := os.WriteFile(filepath.Join(dir, "mockx.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}

	html := `<html><body><table>
<tr><td><a href="/torrent/1/Ubuntu-ISO/">Ubuntu ISO</a></td></tr>
</table></body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "search/ubuntu") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rels, err := Search(ctx, SearchRequest{
		DefinitionsDir: dir,
		DefinitionFile: "mockx",
		BaseURL:        srv.URL + "/",
		IndexerID:      9,
		IndexerName:    "MockX",
		Query:          "ubuntu",
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(rels) == 0 {
		t.Fatal("expected at least one release")
	}
	if rels[0].Title == "" {
		t.Fatalf("empty title: %+v", rels[0])
	}
	if rels[0].IndexerID != 9 {
		t.Fatalf("indexer id = %d", rels[0].IndexerID)
	}
}
