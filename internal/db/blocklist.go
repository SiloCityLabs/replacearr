package db

import (
	"database/sql"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
)

func (s *Store) ListBlocklist(mediaType string) ([]domain.BlocklistItem, error) {
	q := `SELECT id, media_type, source_title, torrent_info_hash, indexer, message,
		series_id, movie_id, artist_id, author_id, size, protocol, created_at
		FROM blocklist`
	args := []any{}
	if mediaType != "" {
		q += ` WHERE media_type = ?`
		args = append(args, normalizeBlockMedia(mediaType))
	}
	q += ` ORDER BY created_at DESC, id DESC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.BlocklistItem{}
	for rows.Next() {
		item, err := scanBlocklist(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Store) CreateBlocklist(item domain.BlocklistItem) (*domain.BlocklistItem, error) {
	item.MediaType = normalizeBlockMedia(item.MediaType)
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	res, err := s.db.Exec(`
		INSERT INTO blocklist (media_type, source_title, torrent_info_hash, indexer, message,
			series_id, movie_id, artist_id, author_id, size, protocol, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.MediaType, item.SourceTitle, item.TorrentInfoHash, item.Indexer, item.Message,
		item.SeriesID, item.MovieID, item.ArtistID, item.AuthorID, item.Size, item.Protocol,
		item.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	item.ID = id
	return &item, nil
}

func (s *Store) DeleteBlocklist(id int64) error {
	_, err := s.db.Exec(`DELETE FROM blocklist WHERE id = ?`, id)
	return err
}

func (s *Store) DeleteBlocklistBulk(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, id := range ids {
		if _, err := tx.Exec(`DELETE FROM blocklist WHERE id = ?`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// IsBlocklisted returns true if title or infoHash matches an entry for the media type.
func (s *Store) IsBlocklisted(mediaType, title, infoHash string) (bool, error) {
	mediaType = normalizeBlockMedia(mediaType)
	title = strings.TrimSpace(strings.ToLower(title))
	infoHash = strings.TrimSpace(strings.ToLower(infoHash))
	if infoHash != "" {
		var id int64
		err := s.db.QueryRow(`
			SELECT id FROM blocklist
			WHERE media_type = ? AND lower(torrent_info_hash) = ?
			LIMIT 1`, mediaType, infoHash).Scan(&id)
		if err == nil {
			return true, nil
		}
		if err != sql.ErrNoRows {
			return false, err
		}
	}
	if title == "" {
		return false, nil
	}
	var id int64
	err := s.db.QueryRow(`
		SELECT id FROM blocklist
		WHERE media_type = ? AND lower(source_title) = ?
		LIMIT 1`, mediaType, title).Scan(&id)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, err
}

func normalizeBlockMedia(mediaType string) string {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "movie", "movies":
		return "movies"
	case "series", "tv":
		return "tv"
	case "music":
		return "music"
	case "ebook", "audiobook", "book", "books":
		return "books"
	default:
		return strings.ToLower(strings.TrimSpace(mediaType))
	}
}

func scanBlocklist(rows *sql.Rows) (domain.BlocklistItem, error) {
	var item domain.BlocklistItem
	var created string
	err := rows.Scan(
		&item.ID, &item.MediaType, &item.SourceTitle, &item.TorrentInfoHash, &item.Indexer, &item.Message,
		&item.SeriesID, &item.MovieID, &item.ArtistID, &item.AuthorID, &item.Size, &item.Protocol, &created,
	)
	if err != nil {
		return item, err
	}
	if t, err := time.Parse(time.RFC3339Nano, created); err == nil {
		item.CreatedAt = t
	} else if t, err := time.Parse(time.RFC3339, created); err == nil {
		item.CreatedAt = t
	} else {
		item.CreatedAt = time.Now().UTC()
	}
	return item, nil
}
