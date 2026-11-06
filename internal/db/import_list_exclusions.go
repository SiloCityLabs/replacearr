package db

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
	"github.com/replacearr/replacearr/internal/mediafile"
)

func (s *Store) AddImportListExclusion(item domain.ImportListExclusion) (*domain.ImportListExclusion, error) {
	item.MediaType = strings.TrimSpace(item.MediaType)
	if item.MediaType == "" {
		item.MediaType = "movie"
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	if item.TmdbID > 0 {
		existing, err := s.GetImportListExclusionByTmdb(item.MediaType, item.TmdbID)
		if err == nil && existing != nil {
			return existing, nil
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	res, err := s.db.Exec(`
		INSERT INTO import_list_exclusions (media_type, tmdb_id, tvdb_id, imdb_id, title, year, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		item.MediaType, item.TmdbID, item.TvdbID, item.ImdbID, item.Title, item.Year,
		item.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	item.ID = id
	return &item, nil
}

func (s *Store) GetImportListExclusionByTmdb(mediaType string, tmdbID int) (*domain.ImportListExclusion, error) {
	row := s.db.QueryRow(`
		SELECT id, media_type, tmdb_id, tvdb_id, imdb_id, title, year, created_at
		FROM import_list_exclusions
		WHERE media_type = ? AND tmdb_id = ? AND tmdb_id > 0
		ORDER BY id DESC LIMIT 1`, mediaType, tmdbID)
	return scanImportListExclusion(row)
}

func (s *Store) IsImportListExcluded(mediaType string, tmdbID, tvdbID int, imdbID string) (bool, error) {
	mediaType = strings.TrimSpace(mediaType)
	if mediaType == "" {
		mediaType = "movie"
	}
	imdbID = strings.TrimSpace(imdbID)
	row := s.db.QueryRow(`
		SELECT 1 FROM import_list_exclusions
		WHERE media_type = ?
		  AND (
			(tmdb_id > 0 AND tmdb_id = ?)
			OR (tvdb_id > 0 AND tvdb_id = ?)
			OR (imdb_id != '' AND imdb_id = ?)
		  )
		LIMIT 1`, mediaType, tmdbID, tvdbID, imdbID)
	var one int
	if err := row.Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// DeleteMovieDiskFiles moves the movie folder into the recycle bin when it lives under the configured root.
func (s *Store) DeleteMovieDiskFiles(mv *domain.Movie, recycleBin string) error {
	if mv == nil {
		return nil
	}
	path := strings.TrimSpace(mv.Path)
	if path == "" {
		return nil
	}
	return mediafile.DeleteUnderRoot(path, mv.RootFolderPath, recycleBin)
}

// DeleteSeriesDiskFiles moves the series folder into the recycle bin when it lives under the configured root.
func (s *Store) DeleteSeriesDiskFiles(sr *domain.Series, recycleBin string) error {
	if sr == nil {
		return nil
	}
	path := strings.TrimSpace(sr.Path)
	if path == "" {
		return nil
	}
	return mediafile.DeleteUnderRoot(path, sr.RootFolderPath, recycleBin)
}

// DeleteArtistDiskFiles moves the artist folder into the recycle bin when it lives under the configured root.
func (s *Store) DeleteArtistDiskFiles(a *domain.Artist, recycleBin string) error {
	if a == nil {
		return nil
	}
	path := strings.TrimSpace(a.Path)
	if path == "" {
		return nil
	}
	return mediafile.DeleteUnderRoot(path, a.RootFolderPath, recycleBin)
}

// DeleteBookDiskFiles moves each edition path for the book into the recycle bin.
func (s *Store) DeleteBookDiskFiles(bookID int64, recycleBin string) error {
	editions, err := s.ListEditions(bookID, "")
	if err != nil {
		return err
	}
	for _, e := range editions {
		path := strings.TrimSpace(e.Path)
		if path == "" {
			continue
		}
		if err := mediafile.DeleteUnderRoot(path, e.RootFolderPath, recycleBin); err != nil {
			return err
		}
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanImportListExclusion(row scanner) (*domain.ImportListExclusion, error) {
	var item domain.ImportListExclusion
	var created string
	if err := row.Scan(
		&item.ID, &item.MediaType, &item.TmdbID, &item.TvdbID, &item.ImdbID,
		&item.Title, &item.Year, &created,
	); err != nil {
		return nil, err
	}
	if t, err := time.Parse(time.RFC3339Nano, created); err == nil {
		item.CreatedAt = t
	} else if t, err := time.Parse(time.RFC3339, created); err == nil {
		item.CreatedAt = t
	}
	return &item, nil
}
