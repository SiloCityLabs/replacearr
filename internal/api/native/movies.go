package native

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/replacearr/replacearr/internal/domain"
)

func (s *Server) handleGetMovie(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	mv, err := s.store.GetMovieByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, errors.New("movie not found"))
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	mv.Images = s.hydrateLibraryPosters("movie", mv.ID, mv.Images)
	writeJSON(w, http.StatusOK, mv)
}

func (s *Server) handleUpdateMovie(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	existing, err := s.store.GetMovieByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, errors.New("movie not found"))
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	var req domain.Movie
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// Preserve library/file state; apply editable + metadata fields from request.
	existing.Title = firstNonEmpty(req.Title, existing.Title)
	existing.OriginalTitle = firstNonEmpty(req.OriginalTitle, existing.OriginalTitle)
	existing.Path = firstNonEmpty(req.Path, existing.Path)
	existing.RootFolderPath = firstNonEmpty(req.RootFolderPath, existing.RootFolderPath)
	existing.FolderName = firstNonEmpty(req.FolderName, existing.FolderName)
	existing.MinimumAvailability = firstNonEmpty(req.MinimumAvailability, existing.MinimumAvailability)
	if req.QualityProfileID > 0 {
		existing.QualityProfileID = req.QualityProfileID
	}
	existing.Monitored = req.Monitored
	if req.Tags != nil {
		existing.Tags = req.Tags
	}
	updated, err := s.store.UpdateMovie(*existing)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	updated.Images = s.hydrateLibraryPosters("movie", updated.ID, updated.Images)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRefreshMovieMetadata(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	existing, err := s.store.GetMovieByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, errors.New("movie not found"))
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if existing.TmdbID <= 0 {
		writeErr(w, http.StatusBadRequest, errors.New("movie has no TMDB id"))
		return
	}
	meta, err := s.meta.GetMovieByTMDB(r.Context(), existing.TmdbID)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	mergeMovieMetadata(existing, meta)
	updated, err := s.store.UpdateMovie(*existing)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	updated.Images = bindLocalPosterURL("movie", updated.ID, updated.Images)
	_, _ = s.store.UpdateMovie(*updated)
	s.cacheCoverAsync("movie", updated.ID, updated.Images)
	updated.Images = s.hydrateLibraryPosters("movie", updated.ID, updated.Images)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleMovieFiles(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if _, err := s.store.GetMovieByID(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, errors.New("movie not found"))
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	media, err := s.store.ListMovieFiles(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if media == nil {
		media = []domain.MovieFile{}
	}
	extra, err := s.store.ListMovieExtraFiles(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if extra == nil {
		extra = []domain.MovieExtraFile{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"media": media,
		"extra": extra,
	})
}

func mergeMovieMetadata(dst *domain.Movie, src domain.Movie) {
	if src.Title != "" {
		dst.Title = src.Title
	}
	if src.OriginalTitle != "" {
		dst.OriginalTitle = src.OriginalTitle
	}
	if src.OriginalLanguage != "" {
		dst.OriginalLanguage = src.OriginalLanguage
	}
	if src.Overview != "" {
		dst.Overview = src.Overview
	}
	if src.Status != "" {
		dst.Status = src.Status
	}
	if src.Website != "" {
		dst.Website = src.Website
	}
	if src.Year > 0 {
		dst.Year = src.Year
	}
	if src.Runtime > 0 {
		dst.Runtime = src.Runtime
	}
	if src.Studio != "" {
		dst.Studio = src.Studio
	}
	if src.Certification != "" {
		dst.Certification = src.Certification
	}
	if src.YouTubeTrailerID != "" {
		dst.YouTubeTrailerID = src.YouTubeTrailerID
	}
	if src.ImdbID != "" {
		dst.ImdbID = src.ImdbID
	}
	if src.InCinemas != "" {
		dst.InCinemas = src.InCinemas
	}
	if src.PhysicalRelease != "" {
		dst.PhysicalRelease = src.PhysicalRelease
	}
	if src.DigitalRelease != "" {
		dst.DigitalRelease = src.DigitalRelease
	}
	if len(src.Genres) > 0 {
		dst.Genres = src.Genres
	}
	if len(src.Images) > 0 {
		dst.Images = src.Images
	}
	if src.Collection != nil {
		dst.Collection = src.Collection
	}
	dst.Ratings = src.Ratings
	dst.Credits = src.Credits
	if len(src.AlternateTitles) > 0 {
		dst.AlternateTitles = src.AlternateTitles
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
