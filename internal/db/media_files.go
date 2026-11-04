package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
)

// UpsertEpisodeFile creates or replaces the video file row for an episode and links episode_file_id.
func (s *Store) UpsertEpisodeFile(seriesID, episodeID int64, path, sceneName, releaseGroup string, size int64) (*domain.EpisodeFile, error) {
	rel := filepath.Base(path)
	if seriesID > 0 {
		if sr, err := s.GetSeriesByID(seriesID); err == nil && sr != nil && sr.Path != "" {
			if r, err := filepath.Rel(sr.Path, path); err == nil && !strings.HasPrefix(r, "..") {
				rel = r
			}
		}
	}
	_, _ = s.db.Exec(`DELETE FROM episode_files WHERE episode_id = ?`, episodeID)
	res, err := s.db.Exec(`
		INSERT INTO episode_files (series_id, episode_id, path, relative_path, size, scene_name, release_group, date_added)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		seriesID, episodeID, path, rel, size, sceneName, releaseGroup,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	_, err = s.db.Exec(`UPDATE episodes SET has_file = 1, episode_file_id = ? WHERE id = ?`, id, episodeID)
	if err != nil {
		return nil, err
	}
	return s.GetEpisodeFileByID(id)
}

// UpsertMovieFile creates or replaces the video file row for a movie and links movie_file_id.
func (s *Store) UpsertMovieFile(movieID int64, path, sceneName, releaseGroup string, size int64) (*domain.MovieFile, error) {
	rel := filepath.Base(path)
	if mv, err := s.GetMovieByID(movieID); err == nil && mv != nil {
		base := mv.Path
		if base == "" {
			base = filepath.Dir(path)
		}
		if r, err := filepath.Rel(base, path); err == nil && !strings.HasPrefix(r, "..") {
			rel = r
		}
	}
	_, _ = s.db.Exec(`DELETE FROM movie_files WHERE movie_id = ?`, movieID)
	res, err := s.db.Exec(`
		INSERT INTO movie_files (movie_id, path, relative_path, size, scene_name, release_group, date_added)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		movieID, path, rel, size, sceneName, releaseGroup,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	_, err = s.db.Exec(`UPDATE movies SET has_file = 1, movie_file_id = ?, size_on_disk = ?, path = ? WHERE id = ?`,
		id, size, filepath.Dir(path), movieID)
	if err != nil {
		return nil, err
	}
	return s.GetMovieFileByID(id)
}

func (s *Store) GetEpisodeFileByID(id int64) (*domain.EpisodeFile, error) {
	row := s.db.QueryRow(`
		SELECT id, series_id, episode_id, path, relative_path, size, scene_name, release_group, date_added
		FROM episode_files WHERE id = ?`, id)
	return scanEpisodeFile(row)
}

func (s *Store) GetEpisodeFileByEpisodeID(episodeID int64) (*domain.EpisodeFile, error) {
	row := s.db.QueryRow(`
		SELECT id, series_id, episode_id, path, relative_path, size, scene_name, release_group, date_added
		FROM episode_files WHERE episode_id = ? ORDER BY id DESC LIMIT 1`, episodeID)
	return scanEpisodeFile(row)
}

func (s *Store) GetMovieFileByID(id int64) (*domain.MovieFile, error) {
	row := s.db.QueryRow(`
		SELECT id, movie_id, path, relative_path, size, scene_name, release_group, date_added
		FROM movie_files WHERE id = ?`, id)
	return scanMovieFile(row)
}

func (s *Store) GetMovieFileByMovieID(movieID int64) (*domain.MovieFile, error) {
	row := s.db.QueryRow(`
		SELECT id, movie_id, path, relative_path, size, scene_name, release_group, date_added
		FROM movie_files WHERE movie_id = ? ORDER BY id DESC LIMIT 1`, movieID)
	return scanMovieFile(row)
}

func scanEpisodeFile(row *sql.Row) (*domain.EpisodeFile, error) {
	var f domain.EpisodeFile
	var added string
	err := row.Scan(&f.ID, &f.SeriesID, &f.EpisodeID, &f.Path, &f.RelativePath, &f.Size, &f.SceneName, &f.ReleaseGroup, &added)
	if err != nil {
		return nil, err
	}
	f.DateAdded = parseSQLTime(added)
	return &f, nil
}

func scanMovieFile(row *sql.Row) (*domain.MovieFile, error) {
	var f domain.MovieFile
	var added string
	err := row.Scan(&f.ID, &f.MovieID, &f.Path, &f.RelativePath, &f.Size, &f.SceneName, &f.ReleaseGroup, &added)
	if err != nil {
		return nil, err
	}
	f.DateAdded = parseSQLTime(added)
	return &f, nil
}

func parseSQLTime(v string) time.Time {
	if v == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
	} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
