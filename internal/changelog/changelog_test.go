package changelog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSectionsFromGitHub(t *testing.T) {
	const body = `# Changelog

## [0.2.0] - 2026-11-04

### Added
- Accordion left navigation
- Global Calendar

### Changed
- Settings in left nav

## [0.1.0] - 2026-01-01

### Added
- Single binary
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("expected User-Agent")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	SetSourceURL(srv.URL)
	t.Cleanup(func() { SetSourceURL(DefaultURL) })

	sections, err := Sections(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 2 {
		t.Fatalf("got %d sections, want 2", len(sections))
	}
	if sections[0].Version != "0.2.0" || sections[0].Date != "2026-11-04" {
		t.Fatalf("unexpected first section: %+v", sections[0])
	}
	if len(sections[0].Notes) != 3 {
		t.Fatalf("got %d notes, want 3: %+v", len(sections[0].Notes), sections[0].Notes)
	}
	if sections[1].Version != "0.1.0" {
		t.Fatalf("unexpected second section: %+v", sections[1])
	}

	// Second call should hit cache (server closed would fail otherwise if we refetch immediately after close —
	// keep server up; verify identical content).
	again, err := Sections(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 2 {
		t.Fatalf("cached: got %d sections", len(again))
	}
}

func TestSectionsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	SetSourceURL(srv.URL)
	t.Cleanup(func() { SetSourceURL(DefaultURL) })

	_, err := Sections(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}
