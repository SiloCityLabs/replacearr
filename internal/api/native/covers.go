package native

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/replacearr/replacearr/internal/domain"
	"github.com/replacearr/replacearr/internal/mediacover"
)

func (s *Server) handleMediaCover(w http.ResponseWriter, r *http.Request) {
	if s.covers == nil {
		http.NotFound(w, r)
		return
	}
	mediaType := r.PathValue("mediaType")
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if id <= 0 {
		http.NotFound(w, r)
		return
	}

	mt, cacheKey, remote, legacyID := s.resolveCover(mediaType, id)
	if cacheKey == "" {
		http.NotFound(w, r)
		return
	}

	f, ct, err := s.covers.Open(mt, cacheKey)
	if err != nil {
		if remote != "" {
			if _, eerr := s.covers.Ensure(mt, cacheKey, id, remote, legacyID); eerr == nil {
				f, ct, err = s.covers.Open(mt, cacheKey)
				if err == nil {
					s.persistLocalPoster(mt, id, remote)
				}
			}
		}
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	defer f.Close()
	if ct == "" {
		ct = "image/webp"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.Copy(w, f)
}

func (s *Server) resolveCover(mediaType string, id int64) (mt, cacheKey, remote string, legacyID int64) {
	legacyID = id
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "tv", "series":
		mt = "tv"
		if sr, err := s.store.GetSeriesByID(id); err == nil && sr != nil {
			cacheKey = mediacover.CacheKeyTV(sr.ImdbID, sr.TvdbID, sr.ID)
			remote = mediacover.PosterRemoteURL(sr.Images)
			return mt, cacheKey, remote, legacyID
		}
	case "movie", "movies":
		mt = "movie"
		if mv, err := s.store.GetMovieByID(id); err == nil && mv != nil {
			cacheKey = mediacover.CacheKeyMovie(mv.ImdbID, mv.TmdbID, mv.ID)
			remote = mediacover.PosterRemoteURL(mv.Images)
			return mt, cacheKey, remote, legacyID
		}
	case "music", "artist", "album":
		mt = "music"
		if a, err := s.store.GetArtistByID(id); err == nil && a != nil {
			cacheKey = mediacover.CacheKeyMusic(a.ForeignArtistID, a.ID)
			remote = mediacover.PosterRemoteURL(a.Images)
			return mt, cacheKey, remote, legacyID
		}
	case "book", "books", "ebook", "audiobook":
		mt = "book"
		if b, err := s.store.GetBookByID(id); err == nil && b != nil {
			cacheKey = mediacover.CacheKeyBook(b.ForeignBookID, b.ID)
			remote = mediacover.PosterRemoteURL(b.Images)
			return mt, cacheKey, remote, legacyID
		}
	case "author":
		mt = "author"
		if a, err := s.store.GetAuthorByID(id); err == nil && a != nil {
			cacheKey = mediacover.CacheKeyBook(a.ForeignAuthorID, a.ID)
			remote = mediacover.PosterRemoteURL(a.Images)
			return mt, cacheKey, remote, legacyID
		}
	default:
		mt = mediacoverNormalize(mediaType)
	}
	return mt, "", "", legacyID
}

func mediacoverNormalize(mediaType string) string {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "movie", "movies", "film":
		return "movie"
	case "music", "artist", "album":
		return "music"
	case "book", "books", "ebook", "audiobook":
		return "book"
	case "author":
		return "author"
	default:
		return "tv"
	}
}

// bindLocalPosterURL sets images[].url to the local mediacover path immediately so the SPA
// never falls back to third-party hosts. Fetching happens async / on first MediaCover hit.
func bindLocalPosterURL(mediaType string, id int64, images []domain.MediaImage) []domain.MediaImage {
	if id <= 0 {
		return images
	}
	remote := mediacover.PosterRemoteURL(images)
	local := mediacover.LocalURL(mediaType, id)
	return mediacover.SetLocalPoster(images, local, remote)
}

func (s *Server) cacheCoverAsync(mediaType string, id int64, images []domain.MediaImage) {
	if s.covers == nil || id <= 0 {
		return
	}
	remote := mediacover.PosterRemoteURL(images)
	if remote == "" {
		return
	}
	go s.ensureAndPersistCover(mediaType, id, remote)
}

func (s *Server) ensureAndPersistCover(mediaType string, id int64, remote string) {
	if s.covers == nil || id <= 0 || remote == "" {
		return
	}
	mt := mediacoverNormalize(mediaType)
	cacheKey, ok := s.coverCacheKey(mt, id)
	if !ok {
		return
	}
	local, err := s.covers.Ensure(mt, cacheKey, id, remote, id)
	if err != nil || local == "" {
		return
	}
	s.persistLocalPoster(mt, id, remote)
}

func (s *Server) coverCacheKey(mt string, id int64) (string, bool) {
	switch mt {
	case "movie":
		mv, err := s.store.GetMovieByID(id)
		if err != nil || mv == nil {
			return "", false
		}
		return mediacover.CacheKeyMovie(mv.ImdbID, mv.TmdbID, mv.ID), true
	case "music":
		a, err := s.store.GetArtistByID(id)
		if err != nil || a == nil {
			return "", false
		}
		return mediacover.CacheKeyMusic(a.ForeignArtistID, a.ID), true
	case "book":
		b, err := s.store.GetBookByID(id)
		if err != nil || b == nil {
			return "", false
		}
		return mediacover.CacheKeyBook(b.ForeignBookID, b.ID), true
	case "author":
		a, err := s.store.GetAuthorByID(id)
		if err != nil || a == nil {
			return "", false
		}
		return mediacover.CacheKeyBook(a.ForeignAuthorID, a.ID), true
	default:
		sr, err := s.store.GetSeriesByID(id)
		if err != nil || sr == nil {
			return "", false
		}
		return mediacover.CacheKeyTV(sr.ImdbID, sr.TvdbID, sr.ID), true
	}
}

func (s *Server) persistLocalPoster(mt string, id int64, remote string) {
	local := mediacover.LocalURL(mt, id)
	switch mt {
	case "movie":
		mv, err := s.store.GetMovieByID(id)
		if err != nil || mv == nil {
			return
		}
		mv.Images = mediacover.SetLocalPoster(mv.Images, local, remote)
		_, _ = s.store.UpdateMovie(*mv)
	case "music":
		a, err := s.store.GetArtistByID(id)
		if err != nil || a == nil {
			return
		}
		a.Images = mediacover.SetLocalPoster(a.Images, local, remote)
		_, _ = s.store.UpdateArtist(*a)
	case "book":
		b, err := s.store.GetBookByID(id)
		if err != nil || b == nil {
			return
		}
		b.Images = mediacover.SetLocalPoster(b.Images, local, remote)
		_, _ = s.store.UpdateBook(*b)
	case "author":
		a, err := s.store.GetAuthorByID(id)
		if err != nil || a == nil {
			return
		}
		a.Images = mediacover.SetLocalPoster(a.Images, local, remote)
		_, _ = s.store.UpdateAuthor(*a)
	default:
		sr, err := s.store.GetSeriesByID(id)
		if err != nil || sr == nil {
			return
		}
		sr.Images = mediacover.SetLocalPoster(sr.Images, local, remote)
		_, _ = s.store.UpdateSeries(*sr)
	}
}

// hydrateLibraryPosters rewrites poster URLs to local mediacover paths and kicks
// async WebP caching for any item still pointing at a third-party host.
func (s *Server) hydrateLibraryPosters(mediaType string, id int64, images []domain.MediaImage) []domain.MediaImage {
	if id <= 0 {
		return images
	}
	mt := mediacoverNormalize(mediaType)
	remote := mediacover.PosterRemoteURL(images)
	local := mediacover.LocalURL(mt, id)
	out := mediacover.SetLocalPoster(images, local, remote)

	needsFetch := remote != ""
	if needsFetch {
		if key, ok := s.coverCacheKey(mt, id); ok && s.covers != nil && s.covers.Cached(mt, key) {
			needsFetch = false
			// File exists but DB may still lack local url — persist quietly.
			if !posterURLIsLocal(images) {
				go s.persistLocalPoster(mt, id, remote)
			}
		}
	}
	if needsFetch {
		s.cacheCoverAsync(mt, id, out)
	}
	return out
}

func posterURLIsLocal(images []domain.MediaImage) bool {
	for _, img := range images {
		if strings.EqualFold(img.CoverType, "poster") && mediacover.IsLocalURL(img.URL) {
			return true
		}
	}
	return false
}

// ServeCoverFile writes a cached poster for compat MediaCover routes.
func ServeCoverFile(covers *mediacover.Store, w http.ResponseWriter, mediaType string, cacheKey string) {
	if covers == nil || cacheKey == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	f, ct, err := covers.Open(mediaType, cacheKey)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	if ct == "" {
		ct = "image/webp"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.Copy(w, f)
}
