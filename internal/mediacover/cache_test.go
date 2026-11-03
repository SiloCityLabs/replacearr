package mediacover

import (
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/replacearr/replacearr/internal/domain"
)

func TestCacheKeyTV(t *testing.T) {
	if got := CacheKeyTV("tt123", 99, 1); got != "imdb-tt123" {
		t.Fatalf("prefer imdb: %q", got)
	}
	if got := CacheKeyTV("", 99, 1); got != "tvdb-99" {
		t.Fatalf("fallback tvdb: %q", got)
	}
	if got := CacheKeyTV("", 0, 7); got != "7" {
		t.Fatalf("fallback id: %q", got)
	}
}

func TestCacheKeyMovie(t *testing.T) {
	if got := CacheKeyMovie("tt9", 42, 1); got != "imdb-tt9" {
		t.Fatalf("prefer imdb: %q", got)
	}
	if got := CacheKeyMovie("", 42, 1); got != "tmdb-42" {
		t.Fatalf("fallback tmdb: %q", got)
	}
}

func TestPosterRemoteURL(t *testing.T) {
	images := []domain.MediaImage{
		{CoverType: "banner", RemoteURL: "https://example.com/banner.jpg"},
		{CoverType: "poster", RemoteURL: "https://example.com/poster.jpg"},
	}
	if got := PosterRemoteURL(images); got != "https://example.com/poster.jpg" {
		t.Fatalf("got %q", got)
	}
}

func TestIsPosterAspect(t *testing.T) {
	if !isPosterAspect(500, 750) {
		t.Fatal("portrait should pass")
	}
	if isPosterAspect(758, 140) {
		t.Fatal("banner should fail")
	}
}

func TestEnsureRejectsBannerAndCachesPoster(t *testing.T) {
	allowPrivateCoverHosts = true
	t.Cleanup(func() { allowPrivateCoverHosts = false })

	root := t.TempDir()
	store := NewStore(root, nil)

	banner := image.NewRGBA(image.Rect(0, 0, 400, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 400; x++ {
			banner.Set(x, y, color.RGBA{R: 200, A: 255})
		}
	}
	poster := image.NewRGBA(image.Rect(0, 0, 200, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 200; x++ {
			poster.Set(x, y, color.RGBA{G: 200, A: 255})
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/banner.jpg", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_ = writePNG(w, banner)
	})
	mux.HandleFunc("/poster.jpg", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_ = writePNG(w, poster)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	store.Client = srv.Client()

	legacyDir := filepath.Join(root, "media", "covers", "tv", "1")
	_ = os.MkdirAll(legacyDir, 0o755)
	legacy := filepath.Join(legacyDir, "poster.webp")
	lf, err := os.Create(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(lf, banner); err != nil {
		t.Fatal(err)
	}
	_ = lf.Close()

	if _, err := store.Ensure("tv", "imdb-tt1", 1, srv.URL+"/banner.jpg", 1); err == nil {
		t.Fatal("expected banner reject")
	}

	local, err := store.Ensure("tv", "imdb-tt1", 1, srv.URL+"/poster.jpg", 1)
	if err != nil {
		t.Fatal(err)
	}
	if local != "/api/v1/mediacover/tv/1/poster.webp" {
		t.Fatalf("local url %q", local)
	}
	dest := store.Path("tv", "imdb-tt1")
	if !store.validPosterFile(dest) {
		t.Fatal("expected valid poster on disk")
	}
	f, ct, err := store.Open("tv", "imdb-tt1")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if ct != "image/png" {
		t.Fatalf("content type %q", ct)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy banner should be removed, err=%v", err)
	}
}

func TestSniffImageContentType(t *testing.T) {
	if got := sniffImageContentType([]byte{0xff, 0xd8, 0xff, 0xe0}); got != "image/jpeg" {
		t.Fatalf("jpeg: %q", got)
	}
	pngMagic := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	if got := sniffImageContentType(pngMagic); got != "image/png" {
		t.Fatalf("png: %q", got)
	}
	webp := append([]byte("RIFF"), make([]byte, 4)...)
	webp = append(webp, []byte("WEBP")...)
	if got := sniffImageContentType(webp); got != "image/webp" {
		t.Fatalf("webp: %q", got)
	}
}

func writePNG(w http.ResponseWriter, img image.Image) error {
	return png.Encode(w, img)
}
