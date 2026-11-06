package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
	"github.com/replacearr/replacearr/internal/mediafile"
	"github.com/replacearr/replacearr/internal/mediainfo"
)

const movieFileSelectCols = `id, movie_id, path, relative_path, size, scene_name, release_group, date_added,
		quality, resolution, video_codec, audio_codec, languages, indexer`

const episodeFileSelectCols = `id, series_id, episode_id, path, relative_path, size, scene_name, release_group, date_added,
		quality, resolution, video_codec, audio_codec, languages, indexer`

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
	info := mediainfo.Probe(path, sceneName)
	_, _ = s.db.Exec(`DELETE FROM episode_files WHERE episode_id = ?`, episodeID)
	res, err := s.db.Exec(`
		INSERT INTO episode_files (
			series_id, episode_id, path, relative_path, size, scene_name, release_group, date_added,
			quality, resolution, video_codec, audio_codec, languages, indexer
		) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, ?, ?, ?, ?, ?)`,
		seriesID, episodeID, path, rel, size, sceneName, releaseGroup,
		info.Quality, info.Resolution, info.VideoCodec, info.AudioCodec, info.Languages, "",
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
	info := mediainfo.Probe(path, sceneName)
	_, _ = s.db.Exec(`DELETE FROM movie_files WHERE movie_id = ?`, movieID)
	res, err := s.db.Exec(`
		INSERT INTO movie_files (
			movie_id, path, relative_path, size, scene_name, release_group, date_added,
			quality, resolution, video_codec, audio_codec, languages, indexer
		) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, ?, ?, ?, ?, ?)`,
		movieID, path, rel, size, sceneName, releaseGroup,
		info.Quality, info.Resolution, info.VideoCodec, info.AudioCodec, info.Languages, "",
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

func (s *Store) updateMovieFileMediaInfo(id int64, info mediainfo.Info) error {
	_, err := s.db.Exec(`
		UPDATE movie_files SET quality = ?, resolution = ?, video_codec = ?, audio_codec = ?,
			languages = CASE WHEN trim(languages) = '' AND ? != '' THEN ? ELSE languages END
		WHERE id = ?`,
		info.Quality, info.Resolution, info.VideoCodec, info.AudioCodec,
		info.Languages, info.Languages, id,
	)
	return err
}

// EnsureMovieFileMediaInfo backfills quality/resolution/codecs when missing.
// Always applies fast filename inference; runs ffprobe only when codecs are unset.
func (s *Store) EnsureMovieFileMediaInfo(f *domain.MovieFile) {
	if f == nil || f.Path == "" {
		return
	}
	needProbe := f.VideoCodec == "" || f.Resolution <= 0 || f.Quality == "" || f.Quality == "Unknown" || f.Languages == ""
	if !needProbe {
		return
	}
	var info mediainfo.Info
	if f.VideoCodec == "" || f.Languages == "" {
		info = mediainfo.Probe(f.Path, f.SceneName)
	} else {
		info = mediainfo.ProbeFilename(f.Path, f.SceneName)
	}
	if info.Quality != "" {
		f.Quality = info.Quality
	}
	if info.Resolution > 0 {
		f.Resolution = info.Resolution
	}
	if info.VideoCodec != "" {
		f.VideoCodec = info.VideoCodec
	}
	if info.AudioCodec != "" {
		f.AudioCodec = info.AudioCodec
	}
	if f.Languages == "" && info.Languages != "" {
		f.Languages = info.Languages
	}
	if f.ID > 0 {
		_ = s.updateMovieFileMediaInfo(f.ID, mediainfo.Info{
			Quality:    f.Quality,
			Resolution: f.Resolution,
			VideoCodec: f.VideoCodec,
			AudioCodec: f.AudioCodec,
			Languages:  f.Languages,
		})
	}
}

// UpdateMovieFile applies editable metadata fields for a movie file.
func (s *Store) UpdateMovieFile(id int64, patch domain.MovieFileUpdate) (*domain.MovieFile, error) {
	existing, err := s.GetMovieFileByID(id)
	if err != nil {
		return nil, err
	}
	quality := strings.TrimSpace(patch.Quality)
	if quality == "" {
		quality = existing.Quality
	}
	languages := strings.TrimSpace(patch.Languages)
	indexer := strings.TrimSpace(patch.Indexer)
	releaseGroup := strings.TrimSpace(patch.ReleaseGroup)
	sceneName := strings.TrimSpace(patch.SceneName)
	videoCodec := strings.TrimSpace(patch.VideoCodec)
	audioCodec := strings.TrimSpace(patch.AudioCodec)
	resolution := patch.Resolution
	if resolution <= 0 {
		resolution = existing.Resolution
		if r := mediainfo.ResolutionFromLabel(quality); r > 0 {
			resolution = r
		}
	}
	if videoCodec == "" {
		videoCodec = existing.VideoCodec
	}
	if audioCodec == "" {
		audioCodec = existing.AudioCodec
	}
	_, err = s.db.Exec(`
		UPDATE movie_files SET
			quality = ?, languages = ?, indexer = ?, release_group = ?, scene_name = ?,
			video_codec = ?, audio_codec = ?, resolution = ?
		WHERE id = ?`,
		quality, languages, indexer, releaseGroup, sceneName,
		videoCodec, audioCodec, resolution, id,
	)
	if err != nil {
		return nil, err
	}
	return s.GetMovieFileByID(id)
}

// DeleteMovieFile removes the movie file DB row, unlinks the movie, and optionally moves the file to the recycle bin.
func (s *Store) DeleteMovieFile(id int64, deleteFromDisk bool, recycleBin string) error {
	f, err := s.GetMovieFileByID(id)
	if err != nil {
		return err
	}
	if deleteFromDisk && strings.TrimSpace(f.Path) != "" {
		mv, err := s.GetMovieByID(f.MovieID)
		if err != nil {
			return err
		}
		root := ""
		if mv != nil {
			root = mv.RootFolderPath
			if root == "" {
				root = mv.Path
			}
		}
		if root == "" {
			root = filepath.Dir(f.Path)
		}
		if err := mediafile.DeleteUnderRoot(f.Path, root, recycleBin); err != nil {
			return fmt.Errorf("delete file from disk: %w", err)
		}
	}
	if _, err := s.db.Exec(`DELETE FROM movie_files WHERE id = ?`, id); err != nil {
		return err
	}
	_, err = s.db.Exec(`
		UPDATE movies SET has_file = 0, movie_file_id = 0, size_on_disk = 0
		WHERE id = ? AND (movie_file_id = ? OR movie_file_id = 0)`,
		f.MovieID, id,
	)
	return err
}

// DeleteMovieExtraFile moves a non-video file under the movie folder into the recycle bin.
func (s *Store) DeleteMovieExtraFile(movieID int64, relativePath, recycleBin string) error {
	full, err := s.resolveMovieExtraPath(movieID, relativePath)
	if err != nil {
		return err
	}
	mv, err := s.GetMovieByID(movieID)
	if err != nil {
		return err
	}
	root := mv.RootFolderPath
	if root == "" {
		root = mv.Path
	}
	if root == "" {
		root = filepath.Dir(full)
	}
	return mediafile.DeleteUnderRoot(full, root, recycleBin)
}

// RenameMovieExtraFile renames a sidecar file within the movie folder.
func (s *Store) RenameMovieExtraFile(movieID int64, relativePath, newName string) (domain.MovieExtraFile, error) {
	src, err := s.resolveMovieExtraPath(movieID, relativePath)
	if err != nil {
		return domain.MovieExtraFile{}, err
	}
	name := filepath.Base(strings.TrimSpace(newName))
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return domain.MovieExtraFile{}, fmt.Errorf("invalid file name")
	}
	dst := filepath.Join(filepath.Dir(src), name)
	if filepath.Clean(dst) == filepath.Clean(src) {
		return s.extraFileInfo(dst)
	}
	if _, err := os.Stat(dst); err == nil {
		return domain.MovieExtraFile{}, fmt.Errorf("target file already exists")
	}
	if err := os.Rename(src, dst); err != nil {
		return domain.MovieExtraFile{}, err
	}
	return s.extraFileInfo(dst)
}

func (s *Store) extraFileInfo(full string) (domain.MovieExtraFile, error) {
	info, err := os.Stat(full)
	if err != nil {
		return domain.MovieExtraFile{}, err
	}
	ext := strings.ToLower(filepath.Ext(full))
	return domain.MovieExtraFile{
		RelativePath: filepath.Base(full),
		Path:         full,
		Extension:    strings.TrimPrefix(ext, "."),
		Type:         classifyExtraFile(ext),
		Size:         info.Size(),
	}, nil
}

// ResolveMovieExtraPath validates that relativePath is a file inside the movie folder.
func (s *Store) ResolveMovieExtraPath(movieID int64, relativePath string) (string, error) {
	return s.resolveMovieExtraPath(movieID, relativePath)
}

func (s *Store) resolveMovieExtraPath(movieID int64, relativePath string) (string, error) {
	rel := filepath.Clean(strings.TrimSpace(relativePath))
	if rel == "." || rel == "" || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid relative path")
	}
	mv, err := s.GetMovieByID(movieID)
	if err != nil {
		return "", err
	}
	dir := strings.TrimSpace(mv.Path)
	if dir == "" {
		return "", fmt.Errorf("movie has no folder path")
	}
	full := filepath.Clean(filepath.Join(dir, rel))
	sep := string(os.PathSeparator)
	cleanDir := filepath.Clean(dir)
	if full == cleanDir || !strings.HasPrefix(full, cleanDir+sep) {
		return "", fmt.Errorf("path escapes movie folder")
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("refusing to delete a directory")
	}
	return full, nil
}

func (s *Store) GetEpisodeFileByID(id int64) (*domain.EpisodeFile, error) {
	row := s.db.QueryRow(`SELECT `+episodeFileSelectCols+` FROM episode_files WHERE id = ?`, id)
	return scanEpisodeFile(row)
}

func (s *Store) GetEpisodeFileByEpisodeID(episodeID int64) (*domain.EpisodeFile, error) {
	row := s.db.QueryRow(`
		SELECT `+episodeFileSelectCols+` FROM episode_files WHERE episode_id = ? ORDER BY id DESC LIMIT 1`, episodeID)
	return scanEpisodeFile(row)
}

func (s *Store) GetMovieFileByID(id int64) (*domain.MovieFile, error) {
	row := s.db.QueryRow(`SELECT `+movieFileSelectCols+` FROM movie_files WHERE id = ?`, id)
	return scanMovieFile(row)
}

func (s *Store) GetMovieFileByMovieID(movieID int64) (*domain.MovieFile, error) {
	row := s.db.QueryRow(`
		SELECT `+movieFileSelectCols+` FROM movie_files WHERE movie_id = ? ORDER BY id DESC LIMIT 1`, movieID)
	return scanMovieFile(row)
}

func (s *Store) ListEpisodeFiles(seriesID int64) ([]domain.EpisodeFile, error) {
	q := `SELECT ` + episodeFileSelectCols + ` FROM episode_files`
	args := []any{}
	if seriesID > 0 {
		q += ` WHERE series_id = ?`
		args = append(args, seriesID)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.EpisodeFile{}
	for rows.Next() {
		f, err := scanEpisodeFileRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// ListEpisodeFilePaths returns on-disk paths for episode files without probing media info.
func (s *Store) ListEpisodeFilePaths(seriesID int64) ([]string, error) {
	q := `SELECT path FROM episode_files`
	args := []any{}
	if seriesID > 0 {
		q += ` WHERE series_id = ?`
		args = append(args, seriesID)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		if p != "" {
			out = append(out, p)
		}
	}
	return out, rows.Err()
}

func (s *Store) ListMovieFiles(movieID int64) ([]domain.MovieFile, error) {
	q := `SELECT ` + movieFileSelectCols + ` FROM movie_files`
	args := []any{}
	if movieID > 0 {
		q += ` WHERE movie_id = ?`
		args = append(args, movieID)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	out := []domain.MovieFile{}
	for rows.Next() {
		f, err := scanMovieFileRows(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, f)
	}
	rows.Close()
	// Probe/persist after the SELECT is closed — SQLite can't UPDATE while rows are open.
	for i := range out {
		s.EnsureMovieFileMediaInfo(&out[i])
	}
	return out, nil
}

// ListMovieFilePaths returns on-disk paths for movie files without probing media info.
func (s *Store) ListMovieFilePaths(movieID int64) ([]string, error) {
	q := `SELECT path FROM movie_files`
	args := []any{}
	if movieID > 0 {
		q += ` WHERE movie_id = ?`
		args = append(args, movieID)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		if p != "" {
			out = append(out, p)
		}
	}
	return out, rows.Err()
}

// ListMovieExtraFiles inventories non-video files in the movie folder.
func (s *Store) ListMovieExtraFiles(movieID int64) ([]domain.MovieExtraFile, error) {
	mv, err := s.GetMovieByID(movieID)
	if err != nil {
		return nil, err
	}
	dir := strings.TrimSpace(mv.Path)
	if dir == "" {
		return []domain.MovieExtraFile{}, nil
	}
	mediaPaths := map[string]struct{}{}
	rows, err := s.db.Query(`SELECT path FROM movie_files WHERE movie_id = ?`, movieID)
	if err == nil {
		for rows.Next() {
			var p string
			if rows.Scan(&p) == nil && p != "" {
				mediaPaths[filepath.Clean(p)] = struct{}{}
			}
		}
		rows.Close()
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.MovieExtraFile{}, nil
		}
		return nil, err
	}
	out := []domain.MovieExtraFile{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		full := filepath.Join(dir, name)
		if _, isMedia := mediaPaths[filepath.Clean(full)]; isMedia {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if isVideoExt(ext) {
			continue
		}
		info, err := e.Info()
		size := int64(0)
		if err == nil {
			size = info.Size()
		}
		out = append(out, domain.MovieExtraFile{
			RelativePath: name,
			Path:         full,
			Extension:    strings.TrimPrefix(ext, "."),
			Type:         classifyExtraFile(ext),
			Size:         size,
		})
	}
	return out, nil
}

func isVideoExt(ext string) bool {
	switch ext {
	case ".mkv", ".mp4", ".avi", ".m4v", ".wmv", ".ts", ".m2ts", ".webm", ".mov":
		return true
	default:
		return false
	}
}

func classifyExtraFile(ext string) string {
	switch ext {
	case ".srt", ".ass", ".ssa", ".vtt", ".sub", ".idx":
		return "subtitle"
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp":
		return "image"
	case ".nfo", ".txt", ".xml":
		return "nfo"
	default:
		return "other"
	}
}

func scanEpisodeFile(row *sql.Row) (*domain.EpisodeFile, error) {
	var f domain.EpisodeFile
	var added string
	err := row.Scan(
		&f.ID, &f.SeriesID, &f.EpisodeID, &f.Path, &f.RelativePath, &f.Size, &f.SceneName, &f.ReleaseGroup, &added,
		&f.Quality, &f.Resolution, &f.VideoCodec, &f.AudioCodec, &f.Languages, &f.Indexer,
	)
	if err != nil {
		return nil, err
	}
	f.DateAdded = parseSQLTime(added)
	return &f, nil
}

func scanMovieFile(row *sql.Row) (*domain.MovieFile, error) {
	var f domain.MovieFile
	var added string
	err := row.Scan(
		&f.ID, &f.MovieID, &f.Path, &f.RelativePath, &f.Size, &f.SceneName, &f.ReleaseGroup, &added,
		&f.Quality, &f.Resolution, &f.VideoCodec, &f.AudioCodec, &f.Languages, &f.Indexer,
	)
	if err != nil {
		return nil, err
	}
	f.DateAdded = parseSQLTime(added)
	return &f, nil
}

func scanEpisodeFileRows(rows *sql.Rows) (domain.EpisodeFile, error) {
	var f domain.EpisodeFile
	var added string
	err := rows.Scan(
		&f.ID, &f.SeriesID, &f.EpisodeID, &f.Path, &f.RelativePath, &f.Size, &f.SceneName, &f.ReleaseGroup, &added,
		&f.Quality, &f.Resolution, &f.VideoCodec, &f.AudioCodec, &f.Languages, &f.Indexer,
	)
	if err != nil {
		return f, err
	}
	f.DateAdded = parseSQLTime(added)
	return f, nil
}

func scanMovieFileRows(rows *sql.Rows) (domain.MovieFile, error) {
	var f domain.MovieFile
	var added string
	err := rows.Scan(
		&f.ID, &f.MovieID, &f.Path, &f.RelativePath, &f.Size, &f.SceneName, &f.ReleaseGroup, &added,
		&f.Quality, &f.Resolution, &f.VideoCodec, &f.AudioCodec, &f.Languages, &f.Indexer,
	)
	if err != nil {
		return f, err
	}
	f.DateAdded = parseSQLTime(added)
	return f, nil
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
