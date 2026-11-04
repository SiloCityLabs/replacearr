package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
)

const (
	defaultMusicBrainzBase = "https://musicbrainz.org/ws/2"
	defaultCAABase         = "https://coverartarchive.org"
	musicBrainzUserAgent   = "Replacearr/1.0 (https://github.com/replacearr/replacearr)"
)

// mbRateLimiter enforces ~1 request/second against MusicBrainz.
var mbRateLimiter = struct {
	mu   sync.Mutex
	last time.Time
}{}

func waitMusicBrainz(ctx context.Context) error {
	mbRateLimiter.mu.Lock()
	defer mbRateLimiter.mu.Unlock()
	wait := time.Second - time.Since(mbRateLimiter.last)
	if wait > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	mbRateLimiter.last = time.Now()
	return nil
}

type mbArtistSearch struct {
	Artists []mbArtist `json:"artists"`
}

type mbArtist struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	SortName       string   `json:"sort-name"`
	Disambiguation string   `json:"disambiguation"`
	Type           string   `json:"type"`
	Country        string   `json:"country"`
	Tags           []mbTag  `json:"tags"`
	LifeSpan       *mbSpan  `json:"life-span"`
	ReleaseGroups  []mbRG   `json:"release-groups"`
}

type mbTag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type mbSpan struct {
	Ended bool `json:"ended"`
}

type mbRGSearch struct {
	ReleaseGroups []mbRG `json:"release-groups"`
}

type mbRG struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	PrimaryType  string `json:"primary-type"`
	FirstRelease string `json:"first-release-date"`
	Secondary    []string `json:"secondary-types"`
}

type mbRelease struct {
	ID      string     `json:"id"`
	Title   string     `json:"title"`
	Date    string     `json:"date"`
	Media   []mbMedium `json:"media"`
}

type mbMedium struct {
	Position int       `json:"position"`
	Tracks   []mbTrack `json:"tracks"`
}

type mbTrack struct {
	ID       string `json:"id"`
	Number   string `json:"number"`
	Title    string `json:"title"`
	Length   int    `json:"length"`
	Position int    `json:"position"`
	Recording *struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Disambiguation string `json:"disambiguation"`
	} `json:"recording"`
}

type mbReleaseList struct {
	Releases []mbRelease `json:"releases"`
}

// SearchArtists looks up artists on MusicBrainz.
func (s *Service) SearchArtists(ctx context.Context, term string) ([]domain.Artist, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return []domain.Artist{}, nil
	}
	lower := strings.ToLower(term)
	if id, ok := parsePrefixedUUID(lower, "mbid:", "musicbrainz:"); ok {
		artist, _, err := s.GetArtistByMBID(ctx, id)
		if err != nil {
			if isNotFound(err) {
				return []domain.Artist{}, nil
			}
			return nil, err
		}
		return []domain.Artist{artist}, nil
	}
	if looksLikeUUID(term) {
		artist, _, err := s.GetArtistByMBID(ctx, term)
		if err == nil {
			return []domain.Artist{artist}, nil
		}
		if !isNotFound(err) {
			return nil, err
		}
	}

	u := fmt.Sprintf("%s/artist?query=%s&fmt=json&limit=25",
		s.musicBrainzBase(), url.QueryEscape(term))
	var result mbArtistSearch
	if err := s.getMusicBrainzJSON(ctx, u, &result); err != nil {
		return nil, fmt.Errorf("musicbrainz artist search: %w", err)
	}
	out := make([]domain.Artist, 0, len(result.Artists))
	for _, a := range result.Artists {
		out = append(out, mapMBArtist(a))
	}
	return out, nil
}

// GetArtistByMBID fetches an artist and their release-groups (albums) with tracks from a canonical release.
func (s *Service) GetArtistByMBID(ctx context.Context, mbid string) (domain.Artist, []domain.AlbumWithTracks, error) {
	mbid = strings.TrimSpace(mbid)
	if mbid == "" {
		return domain.Artist{}, nil, errNotFound
	}
	u := fmt.Sprintf("%s/artist/%s?inc=tags+release-groups&fmt=json", s.musicBrainzBase(), url.PathEscape(mbid))
	var a mbArtist
	if err := s.getMusicBrainzJSON(ctx, u, &a); err != nil {
		return domain.Artist{}, nil, err
	}
	if a.ID == "" {
		return domain.Artist{}, nil, errNotFound
	}
	artist := mapMBArtist(a)
	items := make([]domain.AlbumWithTracks, 0, len(a.ReleaseGroups))
	// Cap release-groups to keep Add Artist responsive (full discography can be huge).
	rgs := a.ReleaseGroups
	if len(rgs) > 40 {
		rgs = rgs[:40]
	}
	for _, rg := range rgs {
		if rg.ID == "" {
			continue
		}
		album := mapMBReleaseGroup(rg)
		album.Images = []domain.MediaImage{caaCover(rg.ID)}
		tracks, _ := s.fetchReleaseGroupTracks(ctx, rg.ID)
		items = append(items, domain.AlbumWithTracks{Album: album, Tracks: tracks})
	}
	return artist, items, nil
}

// SearchAlbums looks up release-groups on MusicBrainz.
func (s *Service) SearchAlbums(ctx context.Context, term string) ([]domain.Album, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return []domain.Album{}, nil
	}
	u := fmt.Sprintf("%s/release-group?query=%s&fmt=json&limit=25",
		s.musicBrainzBase(), url.QueryEscape(term))
	var result mbRGSearch
	if err := s.getMusicBrainzJSON(ctx, u, &result); err != nil {
		return nil, fmt.Errorf("musicbrainz album search: %w", err)
	}
	out := make([]domain.Album, 0, len(result.ReleaseGroups))
	for _, rg := range result.ReleaseGroups {
		al := mapMBReleaseGroup(rg)
		al.Images = []domain.MediaImage{caaCover(rg.ID)}
		out = append(out, al)
	}
	return out, nil
}

func (s *Service) fetchReleaseGroupTracks(ctx context.Context, rgID string) ([]domain.Track, error) {
	u := fmt.Sprintf("%s/release?release-group=%s&inc=recordings&fmt=json&limit=1",
		s.musicBrainzBase(), url.QueryEscape(rgID))
	var list mbReleaseList
	if err := s.getMusicBrainzJSON(ctx, u, &list); err != nil {
		return nil, err
	}
	if len(list.Releases) == 0 {
		return nil, nil
	}
	return mapMBTracks(list.Releases[0]), nil
}

func (s *Service) musicBrainzBase() string {
	if s.mbBase != "" {
		return s.mbBase
	}
	return defaultMusicBrainzBase
}

func (s *Service) getMusicBrainzJSON(ctx context.Context, rawURL string, dest any) error {
	if err := waitMusicBrainz(ctx); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", musicBrainzUserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("musicbrainz http %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return json.Unmarshal(body, dest)
}

func mapMBArtist(a mbArtist) domain.Artist {
	genres := make([]string, 0, len(a.Tags))
	for _, t := range a.Tags {
		if t.Name != "" {
			genres = append(genres, t.Name)
		}
	}
	status := "continuing"
	if a.LifeSpan != nil && a.LifeSpan.Ended {
		status = "ended"
	}
	title := a.Name
	sort := a.SortName
	if sort == "" {
		sort = strings.ToLower(title)
	}
	return domain.Artist{
		Title:           title,
		SortTitle:       sort,
		Status:          status,
		Overview:        a.Disambiguation,
		Disambiguation:  a.Disambiguation,
		ForeignArtistID: a.ID,
		TitleSlug:       slugify(title),
		CleanTitle:      cleanTitle(title),
		Genres:          genres,
		Images:          []domain.MediaImage{},
		Tags:            []int64{},
		Monitored:       true,
		Statistics:      map[string]any{},
	}
}

func mapMBReleaseGroup(rg mbRG) domain.Album {
	albumType := rg.PrimaryType
	if albumType == "" {
		albumType = "Album"
	}
	year := 0
	if len(rg.FirstRelease) >= 4 {
		year, _ = strconv.Atoi(rg.FirstRelease[:4])
	}
	return domain.Album{
		Title:          rg.Title,
		ForeignAlbumID: rg.ID,
		ReleaseDate:    rg.FirstRelease,
		AlbumType:      albumType,
		Year:           year,
		Monitored:      true,
		Images:         []domain.MediaImage{},
		Genres:         []string{},
	}
}

func mapMBTracks(rel mbRelease) []domain.Track {
	out := []domain.Track{}
	abs := 0
	for _, med := range rel.Media {
		medNum := med.Position
		if medNum == 0 {
			medNum = 1
		}
		for _, t := range med.Tracks {
			abs++
			title := t.Title
			fid := t.ID
			if t.Recording != nil {
				if title == "" {
					title = t.Recording.Title
				}
				if fid == "" {
					fid = t.Recording.ID
				}
			}
			num := t.Position
			if num == 0 {
				num, _ = strconv.Atoi(t.Number)
			}
			if num == 0 {
				num = abs
			}
			out = append(out, domain.Track{
				ForeignTrackID:      fid,
				TrackNumber:         num,
				AbsoluteTrackNumber: abs,
				Title:               title,
				DurationMs:          t.Length,
				Monitored:           true,
				MediumNumber:        medNum,
			})
		}
	}
	return out
}

func caaCover(releaseGroupID string) domain.MediaImage {
	remote := fmt.Sprintf("%s/release-group/%s/front-250", defaultCAABase, releaseGroupID)
	return domain.MediaImage{
		CoverType: "poster",
		URL:       remote,
		RemoteURL: remote,
	}
}

func parsePrefixedUUID(lower string, prefixes ...string) (string, bool) {
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			id := strings.TrimSpace(lower[len(p):])
			if looksLikeUUID(id) {
				return id, true
			}
		}
	}
	return "", false
}

func looksLikeUUID(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
	}
	return true
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func cleanTitle(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
