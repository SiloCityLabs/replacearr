package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func withFastArtistImageRetries(t *testing.T) {
	t.Helper()
	prevAttempts := artistImageMaxAttempts
	prevBase := artistImageBaseDelay
	prevMax := artistImageMaxDelay
	prevInterval := artistImageMinInterval
	artistImageMaxAttempts = 4
	artistImageBaseDelay = time.Millisecond
	artistImageMaxDelay = 8 * time.Millisecond
	artistImageMinInterval = 0
	t.Cleanup(func() {
		artistImageMaxAttempts = prevAttempts
		artistImageBaseDelay = prevBase
		artistImageMaxDelay = prevMax
		artistImageMinInterval = prevInterval
	})
}

func TestArtistImageBackoffDelay(t *testing.T) {
	prevBase := artistImageBaseDelay
	prevMax := artistImageMaxDelay
	artistImageBaseDelay = time.Second
	artistImageMaxDelay = 30 * time.Second
	t.Cleanup(func() {
		artistImageBaseDelay = prevBase
		artistImageMaxDelay = prevMax
	})
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{10, 30 * time.Second},
	}
	for _, tc := range cases {
		if got := artistImageBackoffDelay(tc.attempt); got != tc.want {
			t.Fatalf("attempt %d: got %v want %v", tc.attempt, got, tc.want)
		}
	}
}

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
	withFastArtistImageRetries(t)
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
	withFastArtistImageRetries(t)
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

func TestResolveArtistImageRetriesThenErrorsOn429(t *testing.T) {
	withFastArtistImageRetries(t)
	var sparqlHits atomic.Int32
	var deezerHits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/sparql", func(w http.ResponseWriter, _ *http.Request) {
		sparqlHits.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	})
	mux.HandleFunc("/search/artist", func(w http.ResponseWriter, _ *http.Request) {
		deezerHits.Add(1)
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
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	if u != "" {
		t.Fatalf("expected empty url, got %q", u)
	}
	if got := sparqlHits.Load(); got != int32(artistImageMaxAttempts) {
		t.Fatalf("sparql hits=%d want %d", got, artistImageMaxAttempts)
	}
	if got := deezerHits.Load(); got != int32(artistImageMaxAttempts) {
		t.Fatalf("deezer hits=%d want %d", got, artistImageMaxAttempts)
	}
}

func TestResolveArtistImageRecoversAfterTransient429(t *testing.T) {
	withFastArtistImageRetries(t)
	var hits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/sparql", func(w http.ResponseWriter, _ *http.Request) {
		n := hits.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/sparql-results+json")
		_, _ = w.Write([]byte(`{"results":{"bindings":[{"img":{"value":"http://commons.wikimedia.org/wiki/Special:FilePath/Recovered.jpg"}}]}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	s := New(WithHTTPClient(srv.Client()))
	s.wikidataSPARQL = srv.URL + "/sparql"
	s.deezerBase = srv.URL

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	u, err := s.ResolveArtistImage(ctx, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u, "Recovered.jpg") {
		t.Fatalf("got %q", u)
	}
	if got := hits.Load(); got != 3 {
		t.Fatalf("hits=%d want 3", got)
	}
}