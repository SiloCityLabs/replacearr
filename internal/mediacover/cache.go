// Package mediacover fetches remote posters and caches them as WebP on disk.
package mediacover

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/eringen/gowebper"
)

// Store caches media cover art under configDir/media/covers.
type Store struct {
	Root   string
	Client *http.Client
}

// NewStore creates a cover cache rooted at configDir/media/covers.
func NewStore(configDir string, client *http.Client) *Store {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	root := filepath.Join(configDir, "media", "covers")
	_ = os.MkdirAll(root, 0o755)
	return &Store{Root: root, Client: client}
}

// LocalURL returns the native API path for a cached poster.
func LocalURL(mediaType string, id int64) string {
	mt := normalizeMediaType(mediaType)
	return fmt.Sprintf("/api/v1/mediacover/%s/%d/poster.webp", mt, id)
}

// Path returns the on-disk path for a poster.
func (s *Store) Path(mediaType string, id int64) string {
	return filepath.Join(s.Root, normalizeMediaType(mediaType), fmt.Sprintf("%d", id), "poster.webp")
}

// Ensure fetches remoteURL if needed and writes a webp cache file.
func (s *Store) Ensure(mediaType string, id int64, remoteURL string) (string, error) {
	dest := s.Path(mediaType, id)
	if st, err := os.Stat(dest); err == nil && st.Size() > 0 {
		return LocalURL(mediaType, id), nil
	}
	if strings.TrimSpace(remoteURL) == "" {
		return "", fmt.Errorf("no remote cover url")
	}
	if err := s.validateURL(remoteURL); err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodGet, remoteURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Replacearr/1.0")
	res, err := s.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("cover fetch status %d", res.StatusCode)
	}
	limited := io.LimitReader(res.Body, 12<<20)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("decode cover: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := gowebper.Encode(f, img, &gowebper.Options{Level: gowebper.LevelDefault}); err != nil {
		_ = os.Remove(dest)
		return "", err
	}
	return LocalURL(mediaType, id), nil
}

// Open returns a reader for a cached poster if present.
func (s *Store) Open(mediaType string, id int64) (*os.File, error) {
	return os.Open(s.Path(mediaType, id))
}

func normalizeMediaType(mediaType string) string {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "movie", "movies", "film":
		return "movie"
	default:
		return "tv"
	}
}

func (s *Store) validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported cover url scheme")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("empty cover host")
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		if isBlockedHost(host) {
			return fmt.Errorf("blocked cover host")
		}
		return nil
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("blocked cover address")
		}
	}
	return nil
}

func isBlockedHost(host string) bool {
	h := strings.ToLower(host)
	return h == "localhost" || h == "metadata.google.internal" || strings.HasSuffix(h, ".local")
}

func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return true
	}
	return false
}
