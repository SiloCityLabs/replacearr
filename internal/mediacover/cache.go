// Package mediacover fetches remote posters and caches them on disk as WebP.
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
	"strconv"
	"strings"
	"time"

	"github.com/gen2brain/vpx/webp"
	_ "golang.org/x/image/webp"

	"github.com/replacearr/replacearr/internal/domain"
)

const webpQuality = 80

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

// LocalURL returns the native API path for a cached poster (keyed by internal entity id).
func LocalURL(mediaType string, id int64) string {
	mt := normalizeMediaType(mediaType)
	return fmt.Sprintf("/api/v1/mediacover/%s/%d/poster.webp", mt, id)
}

// IsLocalURL reports whether u is a Replacearr mediacover path.
func IsLocalURL(u string) bool {
	return strings.HasPrefix(strings.TrimSpace(u), "/api/v1/mediacover/")
}

// CacheKeyTV builds a stable on-disk key for a series (IMDb, else TVDB, else internal id).
func CacheKeyTV(imdbID string, tvdbID int, fallbackID int64) string {
	if id := strings.TrimSpace(imdbID); id != "" {
		return "imdb-" + sanitizeKey(id)
	}
	if tvdbID > 0 {
		return "tvdb-" + strconv.Itoa(tvdbID)
	}
	if fallbackID > 0 {
		return strconv.FormatInt(fallbackID, 10)
	}
	return ""
}

// CacheKeyMovie builds a stable on-disk key for a movie (IMDb, else TMDB, else internal id).
func CacheKeyMovie(imdbID string, tmdbID int, fallbackID int64) string {
	if id := strings.TrimSpace(imdbID); id != "" {
		return "imdb-" + sanitizeKey(id)
	}
	if tmdbID > 0 {
		return "tmdb-" + strconv.Itoa(tmdbID)
	}
	if fallbackID > 0 {
		return strconv.FormatInt(fallbackID, 10)
	}
	return ""
}

// PosterRemoteURL returns the remote URL for coverType=poster only.
// Prefers RemoteURL; falls back to URL when it is an absolute http(s) link.
func PosterRemoteURL(images []domain.MediaImage) string {
	for _, img := range images {
		if !strings.EqualFold(strings.TrimSpace(img.CoverType), "poster") {
			continue
		}
		if u := strings.TrimSpace(img.RemoteURL); isRemoteHTTP(u) {
			return u
		}
		if u := strings.TrimSpace(img.URL); isRemoteHTTP(u) {
			return u
		}
	}
	return ""
}

// SetLocalPoster rewrites the poster entry to prefer a local mediacover URL.
func SetLocalPoster(images []domain.MediaImage, localURL, remoteURL string) []domain.MediaImage {
	if images == nil {
		images = []domain.MediaImage{}
	}
	found := false
	for i := range images {
		if strings.EqualFold(images[i].CoverType, "poster") {
			images[i].CoverType = "poster"
			images[i].URL = localURL
			if images[i].RemoteURL == "" {
				images[i].RemoteURL = remoteURL
			}
			found = true
			break
		}
	}
	if !found && (localURL != "" || remoteURL != "") {
		images = append(images, domain.MediaImage{
			CoverType: "poster",
			URL:       localURL,
			RemoteURL: remoteURL,
		})
	}
	return images
}

func isRemoteHTTP(u string) bool {
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}

// Path returns the on-disk path for a poster under a stable cache key.
// Filename and payload are both WebP.
func (s *Store) Path(mediaType, cacheKey string) string {
	key := sanitizeKey(cacheKey)
	if key == "" {
		key = "unknown"
	}
	return filepath.Join(s.Root, normalizeMediaType(mediaType), key, "poster.webp")
}

// Cached reports whether a valid WebP poster exists for the cache key.
func (s *Store) Cached(mediaType, cacheKey string) bool {
	mt := normalizeMediaType(mediaType)
	return s.validWebPPoster(s.Path(mt, cacheKey), mt)
}

// Ensure fetches remoteURL if needed and writes a WebP cache file under cacheKey.
// entityID is used only for the returned LocalURL (API path). legacyID, when > 0,
// migrates or removes a legacy {legacyID}/poster.webp file.
func (s *Store) Ensure(mediaType, cacheKey string, entityID int64, remoteURL string, legacyID int64) (string, error) {
	mt := normalizeMediaType(mediaType)
	key := sanitizeKey(cacheKey)
	if key == "" {
		return "", fmt.Errorf("empty cover cache key")
	}
	dest := s.Path(mt, key)
	if s.validWebPPoster(dest, mt) {
		return LocalURL(mt, entityID), nil
	}
	// Re-encode legacy jpeg/png stored under poster.webp without re-fetching.
	if s.validPosterFile(dest, mt) {
		if err := s.reencodeFileToWebP(dest, dest, mt); err == nil {
			return LocalURL(mt, entityID), nil
		}
		_ = os.Remove(dest)
	}

	// Migrate a good legacy file; drop a poisoned banner/empty/non-webp file.
	if legacyID > 0 {
		legacy := s.Path(mt, strconv.FormatInt(legacyID, 10))
		if legacy != dest {
			if s.validWebPPoster(legacy, mt) {
				if err := os.MkdirAll(filepath.Dir(dest), 0o755); err == nil {
					if err := os.Rename(legacy, dest); err == nil {
						_ = os.Remove(filepath.Dir(legacy))
						return LocalURL(mt, entityID), nil
					}
				}
			} else if s.validPosterFile(legacy, mt) {
				if err := s.reencodeFileToWebP(legacy, dest, mt); err == nil {
					_ = os.Remove(legacy)
					_ = os.Remove(filepath.Dir(legacy))
					return LocalURL(mt, entityID), nil
				}
				_ = os.Remove(legacy)
			} else {
				_ = os.Remove(legacy)
			}
		}
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
	img, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("decode cover: %w", err)
	}
	if !isCoverAspect(img.Bounds().Dx(), img.Bounds().Dy(), mt) {
		return "", fmt.Errorf("remote cover is not poster aspect")
	}

	payload, err := encodeWebP(raw, format, img)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return "", err
	}
	if !s.validWebPPoster(tmp, mt) {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("cached cover failed validation")
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return LocalURL(mt, entityID), nil
}

// encodeWebP keeps remote WebP bytes as-is; other formats are re-encoded as lossy WebP.
func encodeWebP(raw []byte, format string, img image.Image) ([]byte, error) {
	if strings.EqualFold(format, "webp") {
		return raw, nil
	}
	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, webp.EncodeOptions{Quality: webpQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *Store) reencodeFileToWebP(src, dest, mediaType string) error {
	mt := normalizeMediaType(mediaType)
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	img, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	if !isCoverAspect(img.Bounds().Dx(), img.Bounds().Dy(), mt) {
		return fmt.Errorf("not poster aspect")
	}
	payload, err := encodeWebP(raw, format, img)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	if !s.validWebPPoster(tmp, mt) {
		_ = os.Remove(tmp)
		return fmt.Errorf("reencoded cover failed validation")
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// Open returns a reader and content type for a valid cached WebP poster.
func (s *Store) Open(mediaType, cacheKey string) (*os.File, string, error) {
	mt := normalizeMediaType(mediaType)
	dest := s.Path(mt, cacheKey)
	if !s.validWebPPoster(dest, mt) {
		// Attempt one-shot migration of legacy jpeg/png stored as poster.webp.
		if s.validPosterFile(dest, mt) {
			if err := s.reencodeFileToWebP(dest, dest, mt); err != nil {
				_ = os.Remove(dest)
				return nil, "", os.ErrNotExist
			}
		} else {
			_ = os.Remove(dest)
			return nil, "", os.ErrNotExist
		}
	}
	f, err := os.Open(dest)
	if err != nil {
		return nil, "", err
	}
	ct, err := sniffFileContentType(f)
	if err != nil {
		_ = f.Close()
		return nil, "", err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, "", err
	}
	return f, ct, nil
}

func sniffFileContentType(f *os.File) (string, error) {
	var hdr [512]byte
	n, err := f.Read(hdr[:])
	if err != nil && err != io.EOF {
		return "", err
	}
	return sniffImageContentType(hdr[:n]), nil
}

func sniffImageContentType(b []byte) string {
	switch {
	case len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff:
		return "image/jpeg"
	case len(b) >= 8 && bytes.Equal(b[:8], []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case len(b) >= 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "image/webp"
	case len(b) >= 6 && (string(b[0:6]) == "GIF87a" || string(b[0:6]) == "GIF89a"):
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

func (s *Store) validWebPPoster(path, mediaType string) bool {
	st, err := os.Stat(path)
	if err != nil || st.Size() <= 0 {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var hdr [12]byte
	n, err := f.Read(hdr[:])
	if err != nil && err != io.EOF {
		return false
	}
	if sniffImageContentType(hdr[:n]) != "image/webp" {
		return false
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return false
	}
	b := img.Bounds()
	return isCoverAspect(b.Dx(), b.Dy(), mediaType)
}

func (s *Store) validPosterFile(path, mediaType string) bool {
	st, err := os.Stat(path)
	if err != nil || st.Size() <= 0 {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return false
	}
	b := img.Bounds()
	return isCoverAspect(b.Dx(), b.Dy(), mediaType)
}

// isPosterAspect rejects wide banners (e.g. 758x140) that were previously cached as posters.
func isPosterAspect(w, h int) bool {
	return isCoverAspect(w, h, "tv")
}

// isCoverAspect validates aspect by media type. TV/movie posters stay portrait-ish;
// music artist thumbs allow mild landscape (grid photos) but still reject banners.
func isCoverAspect(w, h int, mediaType string) bool {
	if w <= 0 || h <= 0 {
		return false
	}
	maxRatio := 1.2
	if normalizeMediaType(mediaType) == "music" {
		maxRatio = 2.0
	}
	return float64(w) <= float64(h)*maxRatio
}

func sanitizeKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(key))
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

func normalizeMediaType(mediaType string) string {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "movie", "movies", "film":
		return "movie"
	case "music", "artist":
		return "music"
	case "album":
		return "album"
	case "book", "books", "ebook", "audiobook":
		return "book"
	case "author":
		return "author"
	default:
		return "tv"
	}
}

// CacheKeyMusic builds a stable on-disk key for an artist (MBID or internal id).
func CacheKeyMusic(foreignArtistID string, fallbackID int64) string {
	if id := strings.TrimSpace(foreignArtistID); id != "" {
		return "mbid-" + sanitizeKey(id)
	}
	if fallbackID > 0 {
		return strconv.FormatInt(fallbackID, 10)
	}
	return ""
}

// CacheKeyAlbum builds a stable on-disk key for an album (release-group MBID or internal id).
func CacheKeyAlbum(foreignAlbumID string, fallbackID int64) string {
	if id := strings.TrimSpace(foreignAlbumID); id != "" {
		return "mbid-" + sanitizeKey(id)
	}
	if fallbackID > 0 {
		return strconv.FormatInt(fallbackID, 10)
	}
	return ""
}

// CacheKeyBook builds a stable on-disk key for a book/author (OL key or internal id).
func CacheKeyBook(foreignID string, fallbackID int64) string {
	if id := strings.TrimSpace(foreignID); id != "" {
		return "ol-" + sanitizeKey(id)
	}
	if fallbackID > 0 {
		return strconv.FormatInt(fallbackID, 10)
	}
	return ""
}

// allowPrivateCoverHosts is set by tests to exercise Ensure against httptest.
var allowPrivateCoverHosts bool

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
	if allowPrivateCoverHosts {
		return nil
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
