package db

import (
	"database/sql"
	"encoding/json"

	"github.com/replacearr/replacearr/internal/domain"
)

func (s *Store) ListSubtitleProfiles() ([]domain.SubtitleProfile, error) {
	rows, err := s.db.Query(`
		SELECT id, name, languages_json, prefer_hearing_impaired, include_forced, min_score, enabled
		FROM subtitle_profiles ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.SubtitleProfile{}
	for rows.Next() {
		p, err := scanSubtitleProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *Store) GetSubtitleProfile(id int64) (*domain.SubtitleProfile, error) {
	row := s.db.QueryRow(`
		SELECT id, name, languages_json, prefer_hearing_impaired, include_forced, min_score, enabled
		FROM subtitle_profiles WHERE id = ?`, id)
	var p domain.SubtitleProfile
	var langsJSON string
	var preferHI, includeForced, enabled int
	if err := row.Scan(&p.ID, &p.Name, &langsJSON, &preferHI, &includeForced, &p.MinScore, &enabled); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(langsJSON), &p.Languages)
	if p.Languages == nil {
		p.Languages = []string{}
	}
	p.PreferHearingImpaired = preferHI == 1
	p.IncludeForced = includeForced == 1
	p.Enabled = enabled == 1
	return &p, nil
}

func (s *Store) UpdateSubtitleProfile(p domain.SubtitleProfile) (*domain.SubtitleProfile, error) {
	langs, _ := json.Marshal(p.Languages)
	_, err := s.db.Exec(`
		UPDATE subtitle_profiles SET name = ?, languages_json = ?, prefer_hearing_impaired = ?,
			include_forced = ?, min_score = ?, enabled = ? WHERE id = ?`,
		p.Name, string(langs), boolToInt(p.PreferHearingImpaired), boolToInt(p.IncludeForced),
		p.MinScore, boolToInt(p.Enabled), p.ID,
	)
	if err != nil {
		return nil, err
	}
	return s.GetSubtitleProfile(p.ID)
}

func scanSubtitleProfile(rows *sql.Rows) (domain.SubtitleProfile, error) {
	var p domain.SubtitleProfile
	var langsJSON string
	var preferHI, includeForced, enabled int
	err := rows.Scan(&p.ID, &p.Name, &langsJSON, &preferHI, &includeForced, &p.MinScore, &enabled)
	if err != nil {
		return p, err
	}
	_ = json.Unmarshal([]byte(langsJSON), &p.Languages)
	if p.Languages == nil {
		p.Languages = []string{}
	}
	p.PreferHearingImpaired = preferHI == 1
	p.IncludeForced = includeForced == 1
	p.Enabled = enabled == 1
	return p, nil
}

func (s *Store) ListSubtitleTracksForMovie(movieID int64) ([]domain.SubtitleTrack, error) {
	return s.listSubtitleTracks(`movie_id = ?`, movieID)
}

func (s *Store) ListSubtitleTracksForEpisode(episodeID int64) ([]domain.SubtitleTrack, error) {
	return s.listSubtitleTracks(`episode_id = ?`, episodeID)
}

func (s *Store) listSubtitleTracks(where string, arg any) ([]domain.SubtitleTrack, error) {
	rows, err := s.db.Query(`
		SELECT id, media_type, movie_id, episode_id, episode_file_id, movie_file_id,
		       language, path, provider, release_name, score, forced, hearing_impaired, downloaded_at
		FROM subtitle_tracks WHERE `+where+` ORDER BY language ASC, id ASC`, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.SubtitleTrack{}
	for rows.Next() {
		t, err := scanSubtitleTrack(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func (s *Store) CreateSubtitleTrack(t domain.SubtitleTrack) (*domain.SubtitleTrack, error) {
	res, err := s.db.Exec(`
		INSERT INTO subtitle_tracks (
			media_type, movie_id, episode_id, episode_file_id, movie_file_id,
			language, path, provider, release_name, score, forced, hearing_impaired, downloaded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		t.MediaType, t.MovieID, t.EpisodeID, t.EpisodeFileID, t.MovieFileID,
		t.Language, t.Path, t.Provider, t.ReleaseName, t.Score,
		boolToInt(t.Forced), boolToInt(t.HearingImpaired),
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetSubtitleTrackByID(id)
}

func (s *Store) GetSubtitleTrackByID(id int64) (*domain.SubtitleTrack, error) {
	row := s.db.QueryRow(`
		SELECT id, media_type, movie_id, episode_id, episode_file_id, movie_file_id,
		       language, path, provider, release_name, score, forced, hearing_impaired, downloaded_at
		FROM subtitle_tracks WHERE id = ?`, id)
	var t domain.SubtitleTrack
	var added string
	var forced, hi int
	err := row.Scan(
		&t.ID, &t.MediaType, &t.MovieID, &t.EpisodeID, &t.EpisodeFileID, &t.MovieFileID,
		&t.Language, &t.Path, &t.Provider, &t.ReleaseName, &t.Score, &forced, &hi, &added,
	)
	if err != nil {
		return nil, err
	}
	t.Forced = forced == 1
	t.HearingImpaired = hi == 1
	t.DownloadedAt = parseSQLTime(added)
	return &t, nil
}

func (s *Store) DeleteSubtitleTrack(id int64) error {
	_, err := s.db.Exec(`DELETE FROM subtitle_tracks WHERE id = ?`, id)
	return err
}

func (s *Store) HasSubtitleLanguage(mediaType string, entityID int64, language string) (bool, error) {
	var n int
	var err error
	switch mediaType {
	case "movie":
		err = s.db.QueryRow(
			`SELECT COUNT(1) FROM subtitle_tracks WHERE movie_id = ? AND language = ?`,
			entityID, language,
		).Scan(&n)
	case "tv":
		err = s.db.QueryRow(
			`SELECT COUNT(1) FROM subtitle_tracks WHERE episode_id = ? AND language = ?`,
			entityID, language,
		).Scan(&n)
	default:
		return false, nil
	}
	return n > 0, err
}

func scanSubtitleTrack(rows *sql.Rows) (domain.SubtitleTrack, error) {
	var t domain.SubtitleTrack
	var added string
	var forced, hi int
	err := rows.Scan(
		&t.ID, &t.MediaType, &t.MovieID, &t.EpisodeID, &t.EpisodeFileID, &t.MovieFileID,
		&t.Language, &t.Path, &t.Provider, &t.ReleaseName, &t.Score, &forced, &hi, &added,
	)
	if err != nil {
		return t, err
	}
	t.Forced = forced == 1
	t.HearingImpaired = hi == 1
	t.DownloadedAt = parseSQLTime(added)
	return t, nil
}
