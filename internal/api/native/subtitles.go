package native

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/replacearr/replacearr/internal/domain"
)

func (s *Server) handleMovieSubtitles(w http.ResponseWriter, r *http.Request) {
	if s.subtitles == nil {
		writeErr(w, http.StatusServiceUnavailable, errors.New("subtitles unavailable"))
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	tracks, missing, file, err := s.subtitles.MovieStatus(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tracks":  tracks,
		"missing": missing,
		"file":    file,
	})
}

func (s *Server) handleSearchMovieSubtitles(w http.ResponseWriter, r *http.Request) {
	if s.subtitles == nil {
		writeErr(w, http.StatusServiceUnavailable, errors.New("subtitles unavailable"))
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	lang := r.URL.Query().Get("language")
	cands, err := s.subtitles.SearchMovie(r.Context(), id, lang)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, cands)
}

func (s *Server) handleDownloadMovieSubtitle(w http.ResponseWriter, r *http.Request) {
	if s.subtitles == nil {
		writeErr(w, http.StatusServiceUnavailable, errors.New("subtitles unavailable"))
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var body struct {
		FileID   int64  `json:"fileId"`
		Language string `json:"language"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if body.FileID == 0 {
		writeErr(w, http.StatusBadRequest, errors.New("fileId is required"))
		return
	}
	track, err := s.subtitles.DownloadMovieCandidate(r.Context(), id, body.FileID, body.Language)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, track)
}

func (s *Server) handleEnsureMovieSubtitles(w http.ResponseWriter, r *http.Request) {
	if s.subtitles == nil {
		writeErr(w, http.StatusServiceUnavailable, errors.New("subtitles unavailable"))
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	tracks, err := s.subtitles.EnsureMovieSubtitles(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, tracks)
}

func (s *Server) handleEpisodeSubtitles(w http.ResponseWriter, r *http.Request) {
	if s.subtitles == nil {
		writeErr(w, http.StatusServiceUnavailable, errors.New("subtitles unavailable"))
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	tracks, missing, file, err := s.subtitles.EpisodeStatus(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tracks":  tracks,
		"missing": missing,
		"file":    file,
	})
}

func (s *Server) handleSearchEpisodeSubtitles(w http.ResponseWriter, r *http.Request) {
	if s.subtitles == nil {
		writeErr(w, http.StatusServiceUnavailable, errors.New("subtitles unavailable"))
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	lang := r.URL.Query().Get("language")
	cands, err := s.subtitles.SearchEpisode(r.Context(), id, lang)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, cands)
}

func (s *Server) handleDownloadEpisodeSubtitle(w http.ResponseWriter, r *http.Request) {
	if s.subtitles == nil {
		writeErr(w, http.StatusServiceUnavailable, errors.New("subtitles unavailable"))
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var body struct {
		FileID   int64  `json:"fileId"`
		Language string `json:"language"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if body.FileID == 0 {
		writeErr(w, http.StatusBadRequest, errors.New("fileId is required"))
		return
	}
	track, err := s.subtitles.DownloadEpisodeCandidate(r.Context(), id, body.FileID, body.Language)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, track)
}

func (s *Server) handleEnsureEpisodeSubtitles(w http.ResponseWriter, r *http.Request) {
	if s.subtitles == nil {
		writeErr(w, http.StatusServiceUnavailable, errors.New("subtitles unavailable"))
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	tracks, err := s.subtitles.EnsureEpisodeSubtitles(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, tracks)
}

func (s *Server) handleListSubtitleProfiles(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListSubtitleProfiles()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleUpdateSubtitleProfile(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var p domain.SubtitleProfile
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	p.ID = id
	if strings.TrimSpace(p.Name) == "" {
		writeErr(w, http.StatusBadRequest, errors.New("name is required"))
		return
	}
	updated, err := s.store.UpdateSubtitleProfile(p)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
