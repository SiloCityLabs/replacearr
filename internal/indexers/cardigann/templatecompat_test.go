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

	cdefinition "github.com/Kcchouette/cardigann-go/definition"
	tmpl "github.com/Kcchouette/cardigann-go/template"
)

func TestFixTemplateString_ProwlarrRegexEscapes(t *testing.T) {
	// Exact string shape after YAML decode of nyaasi.yml path inputs.q
	src := `{{ if .Keywords }}{{ re_replace .Keywords "\b0(\d{1})\b" "$1" }}{{ else }}{{ end }}`
	fixed := fixTemplateString(src)
	out, err := tmpl.Render(fixed, tmpl.Context{Keywords: "Show 01"})
	if err != nil {
		t.Fatalf("Render after fix: %v\nfixed=%q", err, fixed)
	}
	if out != "Show 1" {
		t.Fatalf("got %q, want Show 1", out)
	}
}

func TestFixTemplateString_IdempotentOnAlreadyEscaped(t *testing.T) {
	src := `{{ re_replace .Keywords "\\b0(\\d)\\b" "$1" }}`
	fixed := fixTemplateString(src)
	out, err := tmpl.Render(fixed, tmpl.Context{Keywords: "ep 02"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if out != "ep 2" {
		t.Fatalf("got %q", out)
	}
}

func TestSearch_TemplatedMethodAndRegexInput(t *testing.T) {
	dir := t.TempDir()
	yml := `
id: combo
name: Combo
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
    - path: "{{ if .Keywords }}api/search{{ else }}api/latest{{ end }}"
      method: "{{ if .Keywords }}post{{ else }}get{{ end }}"
      response:
        type: json
      inputs:
        q: "{{ if .Keywords }}{{ re_replace .Keywords \"\\b0(\\d{1})\\b\" \"$1\" }}{{ else }}{{ end }}"
  rows:
    selector: torrents
  fields:
    title:
      selector: title
    details:
      text: /
    infohash:
      selector: infoHash
    size:
      text: "1 GB"
    seeders:
      text: "1"
    leechers:
      text: "0"
    date:
      text: "now"
`
	if err := os.WriteFile(filepath.Join(dir, "combo.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}

	var gotMethod, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if r.Method == http.MethodPost {
			buf := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(buf)
			gotBody = string(buf)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"torrents":[{"title":"Show.S01E01.1080p.WEB-DL.x264","infoHash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rels, err := Search(ctx, SearchRequest{
		DefinitionsDir: dir,
		DefinitionFile: "combo",
		BaseURL:        srv.URL + "/",
		IndexerID:      1,
		IndexerName:    "Combo",
		Query:          "Show 01",
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST (templated method must be rendered)", gotMethod)
	}
	if !strings.Contains(gotPath, "api/search") {
		t.Fatalf("path = %q, want api/search", gotPath)
	}
	if !strings.Contains(gotBody, "Show+1") && !strings.Contains(gotBody, "Show 1") {
		t.Fatalf("body = %q, want stripped episode zero (Show 1)", gotBody)
	}
	if len(rels) != 1 {
		t.Fatalf("releases = %d", len(rels))
	}
	if !strings.HasPrefix(rels[0].DownloadURL, "magnet:?xt=urn:btih:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		t.Fatalf("downloadUrl = %q, want magnet from infohash", rels[0].DownloadURL)
	}
	if rels[0].Quality != "WEBDL-1080p" {
		t.Fatalf("quality = %q, want WEBDL-1080p", rels[0].Quality)
	}
}

func TestResolveDownloadURL(t *testing.T) {
	got := resolveDownloadURL("/dl/1.torrent", "", "T", "https://ex.example/")
	if got != "https://ex.example/dl/1.torrent" {
		t.Fatalf("absolutize: %q", got)
	}
	got = resolveDownloadURL("", "AbCdEf1234567890AbCdEf1234567890AbCdEf12", "My Title", "https://ex.example/")
	if !strings.HasPrefix(got, "magnet:?xt=urn:btih:abcdef1234567890abcdef1234567890abcdef12") {
		t.Fatalf("magnet: %q", got)
	}
	if !strings.Contains(got, "dn=My+Title") {
		t.Fatalf("magnet dn missing: %q", got)
	}
}

func TestPromoteMagnetDownload(t *testing.T) {
	dir := t.TempDir()
	yml := `
id: magonly
name: MagOnly
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
    - path: /
  rows:
    selector: tr
  fields:
    title:
      selector: td.title
    details:
      text: /
    # Prefer empty download (like prefer_magnet_links) so magnet must be promoted.
    download:
      text: "{{ if .Config.prefer_magnet }}{{ else }}{{ .Result.torrent }}{{ end }}"
      optional: true
    torrent:
      selector: a[href$=".torrent"]
      attribute: href
      optional: true
    magnet:
      selector: a[href^="magnet:"]
      attribute: href
    size:
      text: "1 GB"
    seeders:
      text: "1"
    leechers:
      text: "0"
    date:
      text: "now"
`
	if err := os.WriteFile(filepath.Join(dir, "magonly.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	html := `<html><body><table><tr>
<td class="title">Movie.2020.720p.WEBRip</td>
<td><a href="magnet:?xt=urn:btih:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb&dn=x">m</a></td>
</tr></table></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rels, err := Search(ctx, SearchRequest{
		DefinitionsDir: dir,
		DefinitionFile: "magonly",
		BaseURL:        srv.URL + "/",
		IndexerID:      2,
		IndexerName:    "MagOnly",
		Query:          "Movie",
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("releases = %d", len(rels))
	}
	if !strings.HasPrefix(rels[0].DownloadURL, "magnet:?xt=urn:btih:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb") {
		t.Fatalf("downloadUrl = %q, want magnet fallback", rels[0].DownloadURL)
	}
	if rels[0].Quality != "WEBRip-720p" {
		t.Fatalf("quality = %q, want WEBRip-720p", rels[0].Quality)
	}
}

func TestPromoteLeechersToPeers(t *testing.T) {
	s := &cdefinition.Search{
		Fields: map[string]cdefinition.Field{
			"seeders":  {Text: "10"},
			"leechers": {Text: "3"},
		},
	}
	promoteLeechersToPeers(s)
	if _, ok := s.Fields["peers"]; !ok {
		t.Fatal("expected peers field copied from leechers")
	}
	if s.Fields["peers"].Text != "3" {
		t.Fatalf("peers text = %q", s.Fields["peers"].Text)
	}
	// Do not overwrite an explicit peers field.
	s.Fields["peers"] = cdefinition.Field{Text: "99"}
	s.Fields["leechers"] = cdefinition.Field{Text: "3"}
	promoteLeechersToPeers(s)
	if s.Fields["peers"].Text != "99" {
		t.Fatalf("existing peers overwritten: %q", s.Fields["peers"].Text)
	}
}

func TestRenderMethod_KeywordsBranches(t *testing.T) {
	const methodTmpl = `{{ if .Keywords }}post{{ else }}get{{ end }}`
	got, err := renderMethod(methodTmpl, tmpl.Context{Keywords: "foo"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "post" {
		t.Fatalf("with keywords: got %q", got)
	}
	got, err = renderMethod(methodTmpl, tmpl.Context{Keywords: ""})
	if err != nil {
		t.Fatal(err)
	}
	if got != "get" {
		t.Fatalf("without keywords: got %q", got)
	}
}

func TestApplyDefinitionCompat_BundledBangumiAndNyaa(t *testing.T) {
	root := filepath.Join("..", "..", "..", "definitions")
	cases := []struct {
		file   string
		check  func(t *testing.T, def *cdefinition.IndexerDefinition)
	}{
		{
			file: "bangumi-moe.yml",
			check: func(t *testing.T, def *cdefinition.IndexerDefinition) {
				if got := def.Search.Paths[0].Method; !strings.EqualFold(got, "post") {
					t.Fatalf("bangumi method = %q, want post", got)
				}
			},
		},
		{
			file: "nyaasi.yml",
			check: func(t *testing.T, def *cdefinition.IndexerDefinition) {
				q := def.Search.Paths[1].Inputs["q"]
				out, err := tmpl.Render(q, tmpl.Context{Keywords: "Title 01"})
				if err != nil {
					t.Fatalf("nyaa q template: %v\nq=%q", err, q)
				}
				if out != "Title 1" {
					t.Fatalf("nyaa q = %q, want Title 1", out)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(root, tc.file))
			if err != nil {
				t.Fatal(err)
			}
			def, err := cdefinition.Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := applyDefinitionCompat(&def, tmpl.Context{
				Keywords: "Title 01",
				Config:   map[string]string{},
			}); err != nil {
				t.Fatal(err)
			}
			tc.check(t, &def)
		})
	}
}
