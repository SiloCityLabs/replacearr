package native

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/replacearr/replacearr/internal/domain"
	"github.com/replacearr/replacearr/internal/mediainfo"
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

func (s *Server) handleUpdateMovieFile(w http.ResponseWriter, r *http.Request) {
	movieID, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	fileID, _ := strconv.ParseInt(r.PathValue("fileId"), 10, 64)
	f, err := s.store.GetMovieFileByID(fileID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, errors.New("movie file not found"))
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if f.MovieID != movieID {
		writeErr(w, http.StatusNotFound, errors.New("movie file not found"))
		return
	}
	var patch domain.MovieFileUpdate
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	updated, err := s.store.UpdateMovieFile(fileID, patch)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteMovieFile(w http.ResponseWriter, r *http.Request) {
	movieID, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	fileID, _ := strconv.ParseInt(r.PathValue("fileId"), 10, 64)
	f, err := s.store.GetMovieFileByID(fileID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, errors.New("movie file not found"))
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if f.MovieID != movieID {
		writeErr(w, http.StatusNotFound, errors.New("movie file not found"))
		return
	}
	deleteFromDisk := queryFlag(r, "deleteFiles")
	if !r.URL.Query().Has("deleteFiles") {
		deleteFromDisk = true
	}
	if err := s.store.DeleteMovieFile(fileID, deleteFromDisk); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMovieFileDetails(w http.ResponseWriter, r *http.Request) {
	movieID, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	fileID, _ := strconv.ParseInt(r.PathValue("fileId"), 10, 64)
	f, err := s.store.GetMovieFileByID(fileID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, errors.New("movie file not found"))
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if f.MovieID != movieID {
		writeErr(w, http.StatusNotFound, errors.New("movie file not found"))
		return
	}
	writeJSON(w, http.StatusOK, mediainfo.ProbeDetails(f.Path))
}

func (s *Server) handleMovieExtraFileDetails(w http.ResponseWriter, r *http.Request) {
	movieID, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	rel := strings.TrimSpace(r.URL.Query().Get("path"))
	if rel == "" {
		writeErr(w, http.StatusBadRequest, errors.New("path query parameter is required"))
		return
	}
	full, err := s.store.ResolveMovieExtraPath(movieID, rel)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, errors.New("movie not found"))
			return
		}
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(full)), ".")
	writeJSON(w, http.StatusOK, mediainfo.ExtraFileDetails(full, classifyExtraType(ext), ext))
}

func (s *Server) handleDeleteMovieExtraFile(w http.ResponseWriter, r *http.Request) {
	movieID, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var body struct {
		RelativePath string `json:"relativePath"`
		Path         string `json:"path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	rel := strings.TrimSpace(body.RelativePath)
	if rel == "" {
		rel = strings.TrimSpace(body.Path)
	}
	if rel == "" {
		rel = strings.TrimSpace(r.URL.Query().Get("path"))
	}
	if rel == "" {
		writeErr(w, http.StatusBadRequest, errors.New("relativePath is required"))
		return
	}
	if err := s.store.DeleteMovieExtraFile(movieID, rel); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, errors.New("movie not found"))
			return
		}
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUpdateMovieExtraFile(w http.ResponseWriter, r *http.Request) {
	movieID, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var body struct {
		RelativePath string `json:"relativePath"`
		NewName      string `json:"newName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(body.RelativePath) == "" || strings.TrimSpace(body.NewName) == "" {
		writeErr(w, http.StatusBadRequest, errors.New("relativePath and newName are required"))
		return
	}
	updated, err := s.store.RenameMovieExtraFile(movieID, body.RelativePath, body.NewName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, errors.New("movie not found"))
			return
		}
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func classifyExtraType(ext string) string {
	switch ext {
	case "srt", "ass", "ssa", "vtt", "sub", "idx":
		return "subtitle"
	case "jpg", "jpeg", "png", "webp", "gif", "bmp":
		return "image"
	case "nfo", "txt", "xml":
		return "nfo"
	default:
		return "other"
	}
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
