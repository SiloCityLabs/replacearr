//go:build live

package cardigann_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Kcchouette/cardigann-go/definition"
	"github.com/Kcchouette/cardigann-go/engine"
)

func TestLive1337xSearch(t *testing.T) {
	// Optional fixture: drop a Cardigann YAML at this path to exercise a live search.
	path := filepath.Join("testdata", "1337x.yml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("live Cardigann fixture missing (%s): %v", path, err)
	}
	def, err := definition.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	cfg := map[string]string{
		"disablesort":           "true",
		"sort":                  "time",
		"type":                  "desc",
		"primarydownloadlink":   "magnet:",
		"fallbackdownloadlink":  "//itorrents.",
	}
	p := engine.NewProvider(def, "https://1337x.to/", cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rels, err := p.Search(ctx, engine.Query{Keywords: "ubuntu"})
	t.Logf("err=%v count=%d", err, len(rels))
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) == 0 {
		t.Fatal("expected results from 1337x")
	}
	for i, r := range rels {
		if i >= 5 {
			break
		}
		t.Logf("%s seeds=%d size=%d", r.Title, r.Seeders, r.Size)
	}
}
