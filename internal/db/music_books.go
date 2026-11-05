package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
)

// --- Artists ---

func (s *Store) ListArtists() ([]domain.Artist, error) {
	rows, err := s.db.Query(`
		SELECT id, title, sort_title, status, overview, images_json, path, quality_profile_id, monitored,
		       foreign_artist_id, title_slug, clean_title, root_folder_path, genres_json, tags_json, disambiguation, added
		FROM artists ORDER BY sort_title ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Artist{}
	for rows.Next() {
		a, err := scanArtist(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func (s *Store) GetArtistByID(id int64) (*domain.Artist, error) {
	rows, err := s.db.Query(`
		SELECT id, title, sort_title, status, overview, images_json, path, quality_profile_id, monitored,
		       foreign_artist_id, title_slug, clean_title, root_folder_path, genres_json, tags_json, disambiguation, added
		FROM artists WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	a, err := scanArtist(rows)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) GetArtistByForeignID(foreignID string) (*domain.Artist, error) {
	foreignID = strings.TrimSpace(foreignID)
	if foreignID == "" {
		return nil, sql.ErrNoRows
	}
	rows, err := s.db.Query(`
		SELECT id, title, sort_title, status, overview, images_json, path, quality_profile_id, monitored,
		       foreign_artist_id, title_slug, clean_title, root_folder_path, genres_json, tags_json, disambiguation, added
		FROM artists WHERE foreign_artist_id = ? LIMIT 1`, foreignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	a, err := scanArtist(rows)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func scanArtist(rows *sql.Rows) (domain.Artist, error) {
	var a domain.Artist
	var monitored int
	var imagesJSON, genresJSON, tagsJSON string
	err := rows.Scan(
		&a.ID, &a.Title, &a.SortTitle, &a.Status, &a.Overview, &imagesJSON, &a.Path, &a.QualityProfileID, &monitored,
		&a.ForeignArtistID, &a.TitleSlug, &a.CleanTitle, &a.RootFolderPath, &genresJSON, &tagsJSON, &a.Disambiguation, &a.Added,
	)
	if err != nil {
		return a, err
	}
	a.Monitored = monitored == 1
	_ = json.Unmarshal([]byte(imagesJSON), &a.Images)
	_ = json.Unmarshal([]byte(genresJSON), &a.Genres)
	_ = json.Unmarshal([]byte(tagsJSON), &a.Tags)
	if a.Images == nil {
		a.Images = []domain.MediaImage{}
	}
	if a.Genres == nil {
		a.Genres = []string{}
	}
	if a.Tags == nil {
		a.Tags = []int64{}
	}
	a.Statistics = map[string]any{}
	return a, nil
}

func (s *Store) CreateArtist(a domain.Artist) (*domain.Artist, error) {
	if a.Title == "" {
		return nil, errors.New("title is required")
	}
	fillArtistDefaults(&a)
	if a.RootFolderPath == "" {
		if folders, err := s.ListRootFolders("music"); err == nil {
			for _, f := range folders {
				a.RootFolderPath = f.Path
				break
			}
		}
		if a.RootFolderPath == "" {
			a.RootFolderPath = "/data/Music"
		}
	}
	if a.Path == "" && a.RootFolderPath != "" {
		a.Path = strings.TrimRight(a.RootFolderPath, "/") + "/" + a.Title
	}
	imgB, _ := json.Marshal(a.Images)
	genB, _ := json.Marshal(a.Genres)
	tagB, _ := json.Marshal(a.Tags)
	res, err := s.db.Exec(`
		INSERT INTO artists (
			title, sort_title, status, overview, images_json, path, quality_profile_id, monitored,
			foreign_artist_id, title_slug, clean_title, root_folder_path, genres_json, tags_json, disambiguation
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Title, a.SortTitle, a.Status, a.Overview, string(imgB), a.Path, a.QualityProfileID, boolToInt(a.Monitored),
		a.ForeignArtistID, a.TitleSlug, a.CleanTitle, a.RootFolderPath, string(genB), string(tagB), a.Disambiguation,
	)
	if err != nil {
		return nil, err
	}
	a.ID, _ = res.LastInsertId()
	a.Added = time.Now().UTC()
	if a.Images == nil {
		a.Images = []domain.MediaImage{}
	}
	if a.Genres == nil {
		a.Genres = []string{}
	}
	if a.Tags == nil {
		a.Tags = []int64{}
	}
	a.Statistics = map[string]any{}
	return &a, nil
}

func fillArtistDefaults(a *domain.Artist) {
	if a.SortTitle == "" {
		a.SortTitle = strings.ToLower(a.Title)
	}
	if a.CleanTitle == "" {
		a.CleanTitle = strings.ToLower(strings.ReplaceAll(a.Title, " ", ""))
	}
	if a.TitleSlug == "" {
		a.TitleSlug = strings.ToLower(strings.ReplaceAll(a.Title, " ", "-"))
	}
	if a.Status == "" {
		a.Status = "continuing"
	}
	if a.QualityProfileID == 0 {
		a.QualityProfileID = 1
	}
}

func (s *Store) UpdateArtist(a domain.Artist) (*domain.Artist, error) {
	fillArtistDefaults(&a)
	imgB, _ := json.Marshal(a.Images)
	genB, _ := json.Marshal(a.Genres)
	tagB, _ := json.Marshal(a.Tags)
	_, err := s.db.Exec(`
		UPDATE artists SET title=?, sort_title=?, status=?, overview=?, images_json=?, path=?, quality_profile_id=?,
			monitored=?, foreign_artist_id=?, title_slug=?, clean_title=?, root_folder_path=?, genres_json=?, tags_json=?, disambiguation=?
		WHERE id=?`,
		a.Title, a.SortTitle, a.Status, a.Overview, string(imgB), a.Path, a.QualityProfileID,
		boolToInt(a.Monitored), a.ForeignArtistID, a.TitleSlug, a.CleanTitle, a.RootFolderPath, string(genB), string(tagB), a.Disambiguation, a.ID,
	)
	if err != nil {
		return nil, err
	}
	return s.GetArtistByID(a.ID)
}

func (s *Store) DeleteArtist(id int64) error {
	_, err := s.db.Exec(`DELETE FROM artists WHERE id = ?`, id)
	return err
}

// --- Albums ---

func (s *Store) ListAlbums(artistID int64) ([]domain.Album, error) {
	query := `
		SELECT id, artist_id, title, foreign_album_id, release_date, album_type, overview, images_json,
		       monitored, has_file, path, year, genres_json
		FROM albums`
	var args []any
	if artistID > 0 {
		query += ` WHERE artist_id = ?`
		args = append(args, artistID)
	}
	query += ` ORDER BY year DESC, title ASC, id ASC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Album{}
	for rows.Next() {
		al, err := scanAlbum(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, al)
	}
	return out, nil
}

func (s *Store) GetAlbumByID(id int64) (*domain.Album, error) {
	rows, err := s.db.Query(`
		SELECT id, artist_id, title, foreign_album_id, release_date, album_type, overview, images_json,
		       monitored, has_file, path, year, genres_json
		FROM albums WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	al, err := scanAlbum(rows)
	if err != nil {
		return nil, err
	}
	return &al, nil
}

func scanAlbum(rows *sql.Rows) (domain.Album, error) {
	var al domain.Album
	var monitored, hasFile int
	var imagesJSON, genresJSON string
	err := rows.Scan(
		&al.ID, &al.ArtistID, &al.Title, &al.ForeignAlbumID, &al.ReleaseDate, &al.AlbumType, &al.Overview, &imagesJSON,
		&monitored, &hasFile, &al.Path, &al.Year, &genresJSON,
	)
	if err != nil {
		return al, err
	}
	al.Monitored = monitored == 1
	al.HasFile = hasFile == 1
	_ = json.Unmarshal([]byte(imagesJSON), &al.Images)
	_ = json.Unmarshal([]byte(genresJSON), &al.Genres)
	if al.Images == nil {
		al.Images = []domain.MediaImage{}
	}
	if al.Genres == nil {
		al.Genres = []string{}
	}
	return al, nil
}

func (s *Store) CreateAlbum(al domain.Album) (*domain.Album, error) {
	if al.ArtistID == 0 {
		return nil, errors.New("artistId is required")
	}
	if al.Title == "" {
		return nil, errors.New("title is required")
	}
	if al.AlbumType == "" {
		al.AlbumType = "Album"
	}
	imgB, _ := json.Marshal(al.Images)
	genB, _ := json.Marshal(al.Genres)
	res, err := s.db.Exec(`
		INSERT INTO albums (artist_id, title, foreign_album_id, release_date, album_type, overview, images_json, monitored, has_file, path, year, genres_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		al.ArtistID, al.Title, al.ForeignAlbumID, al.ReleaseDate, al.AlbumType, al.Overview, string(imgB),
		boolToInt(al.Monitored), boolToInt(al.HasFile), al.Path, al.Year, string(genB),
	)
	if err != nil {
		return nil, err
	}
	al.ID, _ = res.LastInsertId()
	if al.Images == nil {
		al.Images = []domain.MediaImage{}
	}
	if al.Genres == nil {
		al.Genres = []string{}
	}
	return &al, nil
}

func (s *Store) ReplaceArtistDiscography(artistID int64, items []domain.AlbumWithTracks) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	existingAlbumHF := map[string]bool{}
	rows, err := tx.Query(`SELECT foreign_album_id, has_file FROM albums WHERE artist_id = ?`, artistID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var fid string
		var hf int
		if err := rows.Scan(&fid, &hf); err != nil {
			rows.Close()
			return err
		}
		if fid != "" {
			existingAlbumHF[fid] = hf == 1
		}
	}
	rows.Close()

	if _, err := tx.Exec(`DELETE FROM tracks WHERE artist_id = ?`, artistID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM albums WHERE artist_id = ?`, artistID); err != nil {
		return err
	}
	for _, item := range items {
		al := item.Album
		al.ArtistID = artistID
		if al.AlbumType == "" {
			al.AlbumType = "Album"
		}
		if existingAlbumHF[al.ForeignAlbumID] {
			al.HasFile = true
		}
		imgB, _ := json.Marshal(al.Images)
		genB, _ := json.Marshal(al.Genres)
		res, err := tx.Exec(`
			INSERT INTO albums (artist_id, title, foreign_album_id, release_date, album_type, overview, images_json, monitored, has_file, path, year, genres_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			artistID, al.Title, al.ForeignAlbumID, al.ReleaseDate, al.AlbumType, al.Overview, string(imgB),
			boolToInt(al.Monitored), boolToInt(al.HasFile), al.Path, al.Year, string(genB),
		)
		if err != nil {
			return err
		}
		albumID, _ := res.LastInsertId()
		for _, tr := range item.Tracks {
			if tr.MediumNumber == 0 {
				tr.MediumNumber = 1
			}
			if tr.AbsoluteTrackNumber == 0 {
				tr.AbsoluteTrackNumber = tr.TrackNumber
			}
			_, err := tx.Exec(`
				INSERT INTO tracks (artist_id, album_id, foreign_track_id, track_number, absolute_track_number, title, duration_ms, explicit, has_file, monitored, medium_number)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
				artistID, albumID, tr.ForeignTrackID, tr.TrackNumber, tr.AbsoluteTrackNumber, tr.Title, tr.DurationMs,
				boolToInt(tr.Explicit), boolToInt(tr.Monitored), tr.MediumNumber,
			)
			if err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) MarkAlbumHasFile(albumID int64, hasFile bool) error {
	_, err := s.db.Exec(`UPDATE albums SET has_file = ? WHERE id = ?`, boolToInt(hasFile), albumID)
	return err
}

// --- Tracks ---

func (s *Store) ListTracks(albumID, artistID int64) ([]domain.Track, error) {
	query := `
		SELECT id, artist_id, album_id, foreign_track_id, track_number, absolute_track_number, title,
		       duration_ms, explicit, has_file, monitored, medium_number
		FROM tracks`
	var conds []string
	var args []any
	if albumID > 0 {
		conds = append(conds, `album_id = ?`)
		args = append(args, albumID)
	}
	if artistID > 0 {
		conds = append(conds, `artist_id = ?`)
		args = append(args, artistID)
	}
	if len(conds) > 0 {
		query += ` WHERE ` + strings.Join(conds, ` AND `)
	}
	query += ` ORDER BY medium_number ASC, track_number ASC, id ASC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Track{}
	for rows.Next() {
		var t domain.Track
		var explicit, hasFile, monitored int
		if err := rows.Scan(
			&t.ID, &t.ArtistID, &t.AlbumID, &t.ForeignTrackID, &t.TrackNumber, &t.AbsoluteTrackNumber, &t.Title,
			&t.DurationMs, &explicit, &hasFile, &monitored, &t.MediumNumber,
		); err != nil {
			return nil, err
		}
		t.Explicit = explicit == 1
		t.HasFile = hasFile == 1
		t.Monitored = monitored == 1
		out = append(out, t)
	}
	return out, nil
}

func (s *Store) MarkTrackHasFile(trackID int64, hasFile bool) error {
	_, err := s.db.Exec(`UPDATE tracks SET has_file = ? WHERE id = ?`, boolToInt(hasFile), trackID)
	return err
}

func (s *Store) MarkAlbumTracksHasFile(albumID int64, hasFile bool) error {
	_, err := s.db.Exec(`UPDATE tracks SET has_file = ? WHERE album_id = ?`, boolToInt(hasFile), albumID)
	if err != nil {
		return err
	}
	return s.MarkAlbumHasFile(albumID, hasFile)
}

// --- Authors ---

func (s *Store) ListAuthors() ([]domain.Author, error) {
	rows, err := s.db.Query(`
		SELECT id, title, sort_title, status, overview, images_json, path, quality_profile_id, monitored,
		       foreign_author_id, title_slug, clean_title, root_folder_path, genres_json, tags_json, added
		FROM authors ORDER BY sort_title ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Author{}
	for rows.Next() {
		a, err := scanAuthor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func (s *Store) GetAuthorByID(id int64) (*domain.Author, error) {
	rows, err := s.db.Query(`
		SELECT id, title, sort_title, status, overview, images_json, path, quality_profile_id, monitored,
		       foreign_author_id, title_slug, clean_title, root_folder_path, genres_json, tags_json, added
		FROM authors WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	a, err := scanAuthor(rows)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) GetAuthorByForeignID(foreignID string) (*domain.Author, error) {
	rows, err := s.db.Query(`
		SELECT id, title, sort_title, status, overview, images_json, path, quality_profile_id, monitored,
		       foreign_author_id, title_slug, clean_title, root_folder_path, genres_json, tags_json, added
		FROM authors WHERE foreign_author_id = ?`, foreignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	a, err := scanAuthor(rows)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func scanAuthor(rows *sql.Rows) (domain.Author, error) {
	var a domain.Author
	var monitored int
	var imagesJSON, genresJSON, tagsJSON string
	err := rows.Scan(
		&a.ID, &a.Title, &a.SortTitle, &a.Status, &a.Overview, &imagesJSON, &a.Path, &a.QualityProfileID, &monitored,
		&a.ForeignAuthorID, &a.TitleSlug, &a.CleanTitle, &a.RootFolderPath, &genresJSON, &tagsJSON, &a.Added,
	)
	if err != nil {
		return a, err
	}
	a.Monitored = monitored == 1
	_ = json.Unmarshal([]byte(imagesJSON), &a.Images)
	_ = json.Unmarshal([]byte(genresJSON), &a.Genres)
	_ = json.Unmarshal([]byte(tagsJSON), &a.Tags)
	if a.Images == nil {
		a.Images = []domain.MediaImage{}
	}
	if a.Genres == nil {
		a.Genres = []string{}
	}
	if a.Tags == nil {
		a.Tags = []int64{}
	}
	a.Statistics = map[string]any{}
	return a, nil
}

func (s *Store) CreateAuthor(a domain.Author) (*domain.Author, error) {
	if a.Title == "" {
		return nil, errors.New("title is required")
	}
	fillAuthorDefaults(&a)
	if a.RootFolderPath == "" {
		if folders, err := s.ListRootFolders("ebook"); err == nil {
			for _, f := range folders {
				a.RootFolderPath = f.Path
				break
			}
		}
		if a.RootFolderPath == "" {
			a.RootFolderPath = "/data/Books"
		}
	}
	if a.Path == "" && a.RootFolderPath != "" {
		a.Path = strings.TrimRight(a.RootFolderPath, "/") + "/" + a.Title
	}
	imgB, _ := json.Marshal(a.Images)
	genB, _ := json.Marshal(a.Genres)
	tagB, _ := json.Marshal(a.Tags)
	res, err := s.db.Exec(`
		INSERT INTO authors (
			title, sort_title, status, overview, images_json, path, quality_profile_id, monitored,
			foreign_author_id, title_slug, clean_title, root_folder_path, genres_json, tags_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Title, a.SortTitle, a.Status, a.Overview, string(imgB), a.Path, a.QualityProfileID, boolToInt(a.Monitored),
		a.ForeignAuthorID, a.TitleSlug, a.CleanTitle, a.RootFolderPath, string(genB), string(tagB),
	)
	if err != nil {
		return nil, err
	}
	a.ID, _ = res.LastInsertId()
	a.Added = time.Now().UTC()
	if a.Images == nil {
		a.Images = []domain.MediaImage{}
	}
	if a.Genres == nil {
		a.Genres = []string{}
	}
	if a.Tags == nil {
		a.Tags = []int64{}
	}
	a.Statistics = map[string]any{}
	return &a, nil
}

func fillAuthorDefaults(a *domain.Author) {
	if a.SortTitle == "" {
		a.SortTitle = strings.ToLower(a.Title)
	}
	if a.CleanTitle == "" {
		a.CleanTitle = strings.ToLower(strings.ReplaceAll(a.Title, " ", ""))
	}
	if a.TitleSlug == "" {
		a.TitleSlug = strings.ToLower(strings.ReplaceAll(a.Title, " ", "-"))
	}
	if a.Status == "" {
		a.Status = "continuing"
	}
	if a.QualityProfileID == 0 {
		a.QualityProfileID = 1
	}
}

func (s *Store) UpdateAuthor(a domain.Author) (*domain.Author, error) {
	fillAuthorDefaults(&a)
	imgB, _ := json.Marshal(a.Images)
	genB, _ := json.Marshal(a.Genres)
	tagB, _ := json.Marshal(a.Tags)
	_, err := s.db.Exec(`
		UPDATE authors SET title=?, sort_title=?, status=?, overview=?, images_json=?, path=?, quality_profile_id=?,
			monitored=?, foreign_author_id=?, title_slug=?, clean_title=?, root_folder_path=?, genres_json=?, tags_json=?
		WHERE id=?`,
		a.Title, a.SortTitle, a.Status, a.Overview, string(imgB), a.Path, a.QualityProfileID,
		boolToInt(a.Monitored), a.ForeignAuthorID, a.TitleSlug, a.CleanTitle, a.RootFolderPath, string(genB), string(tagB), a.ID,
	)
	if err != nil {
		return nil, err
	}
	return s.GetAuthorByID(a.ID)
}

func (s *Store) DeleteAuthor(id int64) error {
	_, err := s.db.Exec(`DELETE FROM authors WHERE id = ?`, id)
	return err
}

// --- Books ---

func (s *Store) ListBooks(authorID int64) ([]domain.Book, error) {
	query := `
		SELECT id, author_id, title, foreign_book_id, title_slug, release_date, overview, images_json,
		       monitored, series_title, series_position, genres_json, page_count
		FROM books`
	var args []any
	if authorID > 0 {
		query += ` WHERE author_id = ?`
		args = append(args, authorID)
	}
	query += ` ORDER BY title ASC, id ASC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Book{}
	for rows.Next() {
		b, err := scanBook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

func (s *Store) GetBookByID(id int64) (*domain.Book, error) {
	rows, err := s.db.Query(`
		SELECT id, author_id, title, foreign_book_id, title_slug, release_date, overview, images_json,
		       monitored, series_title, series_position, genres_json, page_count
		FROM books WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	b, err := scanBook(rows)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Store) GetBookByForeignID(foreignID string) (*domain.Book, error) {
	foreignID = strings.TrimSpace(foreignID)
	if foreignID == "" {
		return nil, sql.ErrNoRows
	}
	rows, err := s.db.Query(`
		SELECT id, author_id, title, foreign_book_id, title_slug, release_date, overview, images_json,
		       monitored, series_title, series_position, genres_json, page_count
		FROM books WHERE foreign_book_id = ? LIMIT 1`, foreignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	b, err := scanBook(rows)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func scanBook(rows *sql.Rows) (domain.Book, error) {
	var b domain.Book
	var monitored int
	var imagesJSON, genresJSON string
	err := rows.Scan(
		&b.ID, &b.AuthorID, &b.Title, &b.ForeignBookID, &b.TitleSlug, &b.ReleaseDate, &b.Overview, &imagesJSON,
		&monitored, &b.SeriesTitle, &b.SeriesPosition, &genresJSON, &b.PageCount,
	)
	if err != nil {
		return b, err
	}
	b.Monitored = monitored == 1
	_ = json.Unmarshal([]byte(imagesJSON), &b.Images)
	_ = json.Unmarshal([]byte(genresJSON), &b.Genres)
	if b.Images == nil {
		b.Images = []domain.MediaImage{}
	}
	if b.Genres == nil {
		b.Genres = []string{}
	}
	return b, nil
}

func (s *Store) CreateBook(b domain.Book) (*domain.Book, error) {
	if b.AuthorID == 0 {
		return nil, errors.New("authorId is required")
	}
	if b.Title == "" {
		return nil, errors.New("title is required")
	}
	if b.TitleSlug == "" {
		b.TitleSlug = strings.ToLower(strings.ReplaceAll(b.Title, " ", "-"))
	}
	imgB, _ := json.Marshal(b.Images)
	genB, _ := json.Marshal(b.Genres)
	res, err := s.db.Exec(`
		INSERT INTO books (author_id, title, foreign_book_id, title_slug, release_date, overview, images_json, monitored, series_title, series_position, genres_json, page_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.AuthorID, b.Title, b.ForeignBookID, b.TitleSlug, b.ReleaseDate, b.Overview, string(imgB),
		boolToInt(b.Monitored), b.SeriesTitle, b.SeriesPosition, string(genB), b.PageCount,
	)
	if err != nil {
		return nil, err
	}
	b.ID, _ = res.LastInsertId()
	if b.Images == nil {
		b.Images = []domain.MediaImage{}
	}
	if b.Genres == nil {
		b.Genres = []string{}
	}
	return &b, nil
}

func (s *Store) UpdateBook(b domain.Book) (*domain.Book, error) {
	if b.ID <= 0 {
		return nil, errors.New("id is required")
	}
	if b.Images == nil {
		b.Images = []domain.MediaImage{}
	}
	if b.Genres == nil {
		b.Genres = []string{}
	}
	imgB, _ := json.Marshal(b.Images)
	genB, _ := json.Marshal(b.Genres)
	_, err := s.db.Exec(`
		UPDATE books SET title=?, foreign_book_id=?, title_slug=?, release_date=?, overview=?, images_json=?,
			monitored=?, series_title=?, series_position=?, genres_json=?, page_count=?
		WHERE id=?`,
		b.Title, b.ForeignBookID, b.TitleSlug, b.ReleaseDate, b.Overview, string(imgB),
		boolToInt(b.Monitored), b.SeriesTitle, b.SeriesPosition, string(genB), b.PageCount, b.ID,
	)
	if err != nil {
		return nil, err
	}
	return s.GetBookByID(b.ID)
}

func (s *Store) DeleteBook(id int64) error {
	_, err := s.db.Exec(`DELETE FROM books WHERE id = ?`, id)
	return err
}

// --- Editions ---

func (s *Store) ListEditions(bookID int64, format string) ([]domain.Edition, error) {
	query := `
		SELECT id, book_id, author_id, foreign_edition_id, title, format, isbn13, isbn10, asin, language,
		       page_count, monitored, has_file, path, quality_profile_id, root_folder_path, images_json, publisher, release_date
		FROM editions`
	var conds []string
	var args []any
	if bookID > 0 {
		conds = append(conds, `book_id = ?`)
		args = append(args, bookID)
	}
	if format != "" {
		conds = append(conds, `format = ?`)
		args = append(args, format)
	}
	if len(conds) > 0 {
		query += ` WHERE ` + strings.Join(conds, ` AND `)
	}
	query += ` ORDER BY id ASC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Edition{}
	for rows.Next() {
		e, err := scanEdition(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *Store) GetEditionByID(id int64) (*domain.Edition, error) {
	rows, err := s.db.Query(`
		SELECT id, book_id, author_id, foreign_edition_id, title, format, isbn13, isbn10, asin, language,
		       page_count, monitored, has_file, path, quality_profile_id, root_folder_path, images_json, publisher, release_date
		FROM editions WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	e, err := scanEdition(rows)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func scanEdition(rows *sql.Rows) (domain.Edition, error) {
	var e domain.Edition
	var monitored, hasFile int
	var imagesJSON string
	err := rows.Scan(
		&e.ID, &e.BookID, &e.AuthorID, &e.ForeignEditionID, &e.Title, &e.Format, &e.Isbn13, &e.Isbn10, &e.Asin, &e.Language,
		&e.PageCount, &monitored, &hasFile, &e.Path, &e.QualityProfileID, &e.RootFolderPath, &imagesJSON, &e.Publisher, &e.ReleaseDate,
	)
	if err != nil {
		return e, err
	}
	e.Monitored = monitored == 1
	e.HasFile = hasFile == 1
	_ = json.Unmarshal([]byte(imagesJSON), &e.Images)
	if e.Images == nil {
		e.Images = []domain.MediaImage{}
	}
	return e, nil
}

func (s *Store) CreateEdition(e domain.Edition) (*domain.Edition, error) {
	if e.BookID == 0 {
		return nil, errors.New("bookId is required")
	}
	if e.Format == "" {
		e.Format = "ebook"
	}
	if e.QualityProfileID == 0 {
		e.QualityProfileID = 1
	}
	if e.Language == "" {
		e.Language = "eng"
	}
	imgB, _ := json.Marshal(e.Images)
	res, err := s.db.Exec(`
		INSERT INTO editions (
			book_id, author_id, foreign_edition_id, title, format, isbn13, isbn10, asin, language,
			page_count, monitored, has_file, path, quality_profile_id, root_folder_path, images_json, publisher, release_date
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.BookID, e.AuthorID, e.ForeignEditionID, e.Title, e.Format, e.Isbn13, e.Isbn10, e.Asin, e.Language,
		e.PageCount, boolToInt(e.Monitored), boolToInt(e.HasFile), e.Path, e.QualityProfileID, e.RootFolderPath,
		string(imgB), e.Publisher, e.ReleaseDate,
	)
	if err != nil {
		return nil, err
	}
	e.ID, _ = res.LastInsertId()
	if e.Images == nil {
		e.Images = []domain.MediaImage{}
	}
	return &e, nil
}

func (s *Store) UpdateEdition(e domain.Edition) (*domain.Edition, error) {
	imgB, _ := json.Marshal(e.Images)
	_, err := s.db.Exec(`
		UPDATE editions SET title=?, format=?, isbn13=?, isbn10=?, asin=?, language=?, page_count=?, monitored=?,
			has_file=?, path=?, quality_profile_id=?, root_folder_path=?, images_json=?, publisher=?, release_date=?, foreign_edition_id=?
		WHERE id=?`,
		e.Title, e.Format, e.Isbn13, e.Isbn10, e.Asin, e.Language, e.PageCount, boolToInt(e.Monitored),
		boolToInt(e.HasFile), e.Path, e.QualityProfileID, e.RootFolderPath, string(imgB), e.Publisher, e.ReleaseDate, e.ForeignEditionID, e.ID,
	)
	if err != nil {
		return nil, err
	}
	return s.GetEditionByID(e.ID)
}

func (s *Store) DeleteEdition(id int64) error {
	_, err := s.db.Exec(`DELETE FROM editions WHERE id = ?`, id)
	return err
}

func (s *Store) MarkEditionHasFile(editionID int64, hasFile bool) error {
	_, err := s.db.Exec(`UPDATE editions SET has_file = ? WHERE id = ?`, boolToInt(hasFile), editionID)
	return err
}
