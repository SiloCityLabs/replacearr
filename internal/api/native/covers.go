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
	f, err := s.covers.Open(mediaType, id)
	if err != nil {
		remote := ""
		mt := mediaType
		switch mediaType {
		case "tv", "series":
			mt = "tv"
			if sr, gerr := s.store.GetSeriesByID(id); gerr == nil && sr != nil {
				remote = firstRemoteCover(sr.Images)
			}
		case "movie", "movies":
			mt = "movie"
			if mv, gerr := s.store.GetMovieByID(id); gerr == nil && mv != nil {
				remote = firstRemoteCover(mv.Images)
			}
		}
		if remote != "" {
			if _, eerr := s.covers.Ensure(mt, id, remote); eerr == nil {
				f, err = s.covers.Open(mt, id)
			}
		}
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	defer f.Close()
	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.Copy(w, f)
}

func firstRemoteCover(images []domain.MediaImage) string {
	for _, img := range images {
		if strings.TrimSpace(img.RemoteURL) != "" {
			return img.RemoteURL
		}
	}
	return ""
}

func (s *Server) cacheCoverAsync(mediaType string, id int64, images []domain.MediaImage) {
	if s.covers == nil || id <= 0 {
		return
	}
	remote := firstRemoteCover(images)
	if remote == "" {
		return
	}
	go func() {
		local, err := s.covers.Ensure(mediaType, id, remote)
		if err != nil || local == "" {
			return
		}
		switch mediaType {
		case "movie":
			mv, err := s.store.GetMovieByID(id)
			if err != nil || mv == nil {
				return
			}
			mv.Images = setLocalPoster(mv.Images, local, remote)
			_, _ = s.store.UpdateMovie(*mv)
		default:
			sr, err := s.store.GetSeriesByID(id)
			if err != nil || sr == nil {
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
		if images[i].CoverType == "" || strings.EqualFold(images[i].CoverType, "poster") {
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
func ServeCoverFile(covers *mediacover.Store, w http.ResponseWriter, mediaType string, id int64) {
	if covers == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	f, err := covers.Open(mediaType, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.Copy(w, f)
}
