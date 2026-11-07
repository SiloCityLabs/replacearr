package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCommonsThumbURL(t *testing.T) {
	got := commonsThumbURL("http://commons.wikimedia.org/wiki/Special:FilePath/Radiohead.jpg", 250)
	if !strings.HasPrefix(got, "https://commons.wikimedia.org/wiki/Special:FilePath/Radiohead.jpg") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "width=250") {
		t.Fatalf("missing width: %q", got)
	}
}

func TestResolveArtistImagePrefersWikidata(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sparql", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/sparql-results+json")
		_, _ = w.Write([]byte(`{"results":{"bindings":[{"img":{"value":"http://commons.wikimedia.org/wiki/Special:FilePath/Test_Artist.jpg"}}]}}`))
	})
	mux.HandleFunc("/search/artist", func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("deezer should not be called when wikidata succeeds")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	s := New(WithHTTPClient(srv.Client()))
	s.wikidataSPARQL = srv.URL + "/sparql"
	s.deezerBase = srv.URL

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	u, err := s.ResolveArtistImage(ctx, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "Test Artist")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u, "Special:FilePath/Test_Artist.jpg") || !strings.Contains(u, "width=250") {
		t.Fatalf("got %q", u)
	}
}

func TestResolveArtistImageFallsBackToDeezer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sparql", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/sparql-results+json")
		_, _ = w.Write([]byte(`{"results":{"bindings":[]}}`))
	})
	mux.HandleFunc("/search/artist", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"name":"Test Artist","picture_medium":"https://cdn.example/med.jpg","picture_small":"https://cdn.example/sm.jpg"}]}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	s := New(WithHTTPClient(srv.Client()))
	s.wikidataSPARQL = srv.URL + "/sparql"
	s.deezerBase = srv.URL

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	u, err := s.ResolveArtistImage(ctx, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "Test Artist")
	if err != nil {
		t.Fatal(err)
	}
	if u != "https://cdn.example/med.jpg" {
		t.Fatalf("got %q", u)
	}
}

func TestResolveArtistImageSoftFailsOn429(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sparql", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	mux.HandleFunc("/search/artist", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	s := New(WithHTTPClient(srv.Client()))
	s.wikidataSPARQL = srv.URL + "/sparql"
	s.deezerBase = srv.URL

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	u, err := s.ResolveArtistImage(ctx, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "Test Artist")
	if err != nil {
		t.Fatalf("expected soft fail, got err %v", err)
	}
	if u != "" {
		t.Fatalf("expected empty url, got %q", u)
	}
}
