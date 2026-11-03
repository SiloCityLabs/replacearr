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
			}
		}
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	defer f.Close()
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.Copy(w, f)
}

func (s *Server) resolveCover(mediaType string, id int64) (mt, cacheKey, remote string, legacyID int64) {
	legacyID = id
	switch mediaType {
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
	default:
		mt = mediacoverNormalize(mediaType)
	}
	return mt, "", "", legacyID
}

func mediacoverNormalize(mediaType string) string {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "movie", "movies", "film":
		return "movie"
	default:
		return "tv"
	}
}

func (s *Server) cacheCoverAsync(mediaType string, id int64, images []domain.MediaImage) {
	if s.covers == nil || id <= 0 {
		return
	}
	remote := mediacover.PosterRemoteURL(images)
	if remote == "" {
		return
	}
	go func() {
		mt := mediacoverNormalize(mediaType)
		var cacheKey string
		switch mt {
		case "movie":
			mv, err := s.store.GetMovieByID(id)
			if err != nil || mv == nil {
				return
			}
			cacheKey = mediacover.CacheKeyMovie(mv.ImdbID, mv.TmdbID, mv.ID)
			local, err := s.covers.Ensure(mt, cacheKey, id, remote, id)
			if err != nil || local == "" {
				return
			}
			mv.Images = setLocalPoster(mv.Images, local, remote)
			_, _ = s.store.UpdateMovie(*mv)
		default:
			sr, err := s.store.GetSeriesByID(id)
			if err != nil || sr == nil {
				return
			}
			cacheKey = mediacover.CacheKeyTV(sr.ImdbID, sr.TvdbID, sr.ID)
			local, err := s.covers.Ensure(mt, cacheKey, id, remote, id)
			if err != nil || local == "" {
				return
			}
			sr.Images = setLocalPoster(sr.Images, local, remote)
			_, _ = s.store.UpdateSeries(*sr)
		}
	}()
}

func setLocalPoster(images []domain.MediaImage, localURL, remoteURL string) []domain.MediaImage {
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
	if !found {
		images = append(images, domain.MediaImage{
			CoverType: "poster",
			URL:       localURL,
			RemoteURL: remoteURL,
		})
	}
	return images
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
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.Copy(w, f)
}
