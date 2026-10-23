package db

import (
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type Store struct {
	db *sql.DB
}

func Open(dbPath string) (*Store, error) {
	dsn := dbPath
	if !strings.Contains(dsn, "?") {
		dsn += "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	}
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB.SetMaxOpenConns(1) // SQLite single-writer safety

	s := &Store{db: sqlDB}
	if err := s.migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	if err := s.seedDefaults(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) migrate() error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		sqlBytes, err := migrationFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		if _, err := s.db.Exec(string(sqlBytes)); err != nil {
			return fmt.Errorf("exec migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func (s *Store) seedDefaults() error {
	// 1. Seed Built-In Roles
	builtInRoles := []struct {
		Name        string
		Description string
		Perms       []string
	}{
		{
			Name:        "admin",
			Description: "Full administrative access to all media, indexers, plugins, and user management",
			Perms:       append([]string{domain.PermAdminAll}, domain.AllPermissions...),
		},
		{
			Name:        "manager",
			Description: "Manage TV series, movies, indexers, and download clients",
			Perms: []string{
				domain.PermTVRead, domain.PermTVWrite, domain.PermTVDelete,
				domain.PermMoviesRead, domain.PermMoviesWrite, domain.PermMoviesDelete,
				domain.PermIndexersRead, domain.PermIndexersWrite, domain.PermIndexersSearch,
				domain.PermDownloadsRead, domain.PermDownloadsWrite,
				domain.PermProfilesRead, domain.PermProfilesWrite,
				domain.PermPluginsRead, domain.PermSystemRead,
			},
		},
		{
			Name:        "requester",
			Description: "Can view and add TV series and movies and search indexers",
			Perms: []string{
				domain.PermTVRead, domain.PermTVWrite,
				domain.PermMoviesRead, domain.PermMoviesWrite,
				domain.PermIndexersRead, domain.PermIndexersSearch,
				domain.PermDownloadsRead, domain.PermProfilesRead, domain.PermSystemRead,
			},
		},
		{
			Name:        "viewer",
			Description: "Read-only access to library, calendar, and queue",
			Perms: []string{
				domain.PermTVRead, domain.PermMoviesRead,
				domain.PermIndexersRead, domain.PermDownloadsRead,
				domain.PermProfilesRead, domain.PermSystemRead,
			},
		},
		{
			Name:        "integration",
			Description: "Service account role for external *arr ecosystem integrations (Overseerr, Bazarr, etc.)",
			Perms: []string{
				domain.PermTVRead, domain.PermTVWrite,
				domain.PermMoviesRead, domain.PermMoviesWrite,
				domain.PermIndexersRead, domain.PermIndexersSearch,
				domain.PermDownloadsRead, domain.PermDownloadsWrite,
				domain.PermProfilesRead, domain.PermSystemRead,
			},
		},
	}

	for _, r := range builtInRoles {
		b, _ := json.Marshal(r.Perms)
		_, err := s.db.Exec(
			`INSERT INTO roles (name, description, permissions_json, built_in) VALUES (?, ?, ?, 1)
			 ON CONFLICT(name) DO UPDATE SET description = excluded.description`,
			r.Name, r.Description, string(b),
		)
		if err != nil {
			return fmt.Errorf("seed role %s: %w", r.Name, err)
		}
	}

	// 2. Seed Default Quality Profiles if empty
	var qpCount int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM quality_profiles`).Scan(&qpCount)
	if qpCount == 0 {
		defaultProfiles := []domain.QualityProfile{
			{
				Name:           "Any",
				UpgradeAllowed: true,
				Cutoff:         7,
				Items: []domain.QualityItem{
					{ID: 1, Name: "SDTV", Source: "television", Resolution: 480, Allowed: true},
					{ID: 4, Name: "HDTV-720p", Source: "television", Resolution: 720, Allowed: true},
					{ID: 7, Name: "WEBDL-1080p", Source: "web", Resolution: 1080, Allowed: true},
					{ID: 9, Name: "Bluray-1080p", Source: "bluray", Resolution: 1080, Allowed: true},
					{ID: 19, Name: "WEBDL-2160p", Source: "web", Resolution: 2160, Allowed: true},
				},
			},
			{
				Name:           "HD - 720p/1080p",
				UpgradeAllowed: true,
				Cutoff:         9,
				Items: []domain.QualityItem{
					{ID: 4, Name: "HDTV-720p", Source: "television", Resolution: 720, Allowed: true},
					{ID: 7, Name: "WEBDL-1080p", Source: "web", Resolution: 1080, Allowed: true},
					{ID: 9, Name: "Bluray-1080p", Source: "bluray", Resolution: 1080, Allowed: true},
				},
			},
			{
				Name:           "Ultra-HD (4K)",
				UpgradeAllowed: true,
				Cutoff:         19,
				Items: []domain.QualityItem{
					{ID: 18, Name: "HDTV-2160p", Source: "television", Resolution: 2160, Allowed: true},
					{ID: 19, Name: "WEBDL-2160p", Source: "web", Resolution: 2160, Allowed: true},
					{ID: 20, Name: "Bluray-2160p", Source: "bluray", Resolution: 2160, Allowed: true},
				},
			},
		}
		for _, qp := range defaultProfiles {
			_, _ = s.CreateQualityProfile(qp)
		}
	}

	// 3. Seed Default Root Folders if empty
	var rfCount int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM root_folders`).Scan(&rfCount)
	if rfCount == 0 {
		_, _ = s.CreateRootFolder(domain.RootFolder{
			Path:       "/data/tv",
			MediaType:  "tv",
			Accessible: true,
			FreeSpace:  536870912000,
			TotalSpace: 1073741824000,
		})
		_, _ = s.CreateRootFolder(domain.RootFolder{
			Path:       "/data/movies",
			MediaType:  "movie",
			Accessible: true,
			FreeSpace:  536870912000,
			TotalSpace: 1073741824000,
		})
	}

	return nil
}

// --- Roles & Users & API Keys ---

func (s *Store) ListRoles() ([]domain.Role, error) {
	rows, err := s.db.Query(`SELECT id, name, description, permissions_json, built_in, created_at FROM roles ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Role
	for rows.Next() {
		var r domain.Role
		var permsJSON string
		var builtIn int
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &permsJSON, &builtIn, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.BuiltIn = builtIn == 1
		_ = json.Unmarshal([]byte(permsJSON), &r.Permissions)
		if r.Permissions == nil {
			r.Permissions = []string{}
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) GetRoleByID(id int64) (*domain.Role, error) {
	var r domain.Role
	var permsJSON string
	var builtIn int
	err := s.db.QueryRow(`SELECT id, name, description, permissions_json, built_in, created_at FROM roles WHERE id = ?`, id).
		Scan(&r.ID, &r.Name, &r.Description, &permsJSON, &builtIn, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	r.BuiltIn = builtIn == 1
	_ = json.Unmarshal([]byte(permsJSON), &r.Permissions)
	return &r, nil
}

func (s *Store) GetRoleByName(name string) (*domain.Role, error) {
	var r domain.Role
	var permsJSON string
	var builtIn int
	err := s.db.QueryRow(`SELECT id, name, description, permissions_json, built_in, created_at FROM roles WHERE name = ?`, name).
		Scan(&r.ID, &r.Name, &r.Description, &permsJSON, &builtIn, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	r.BuiltIn = builtIn == 1
	_ = json.Unmarshal([]byte(permsJSON), &r.Permissions)
	return &r, nil
}

func (s *Store) CreateRole(name, description string, permissions []string) (*domain.Role, error) {
	if permissions == nil {
		permissions = []string{}
	}
	b, _ := json.Marshal(permissions)
	res, err := s.db.Exec(`INSERT INTO roles (name, description, permissions_json, built_in) VALUES (?, ?, ?, 0)`,
		name, description, string(b))
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetRoleByID(id)
}

func (s *Store) UpdateRole(id int64, description string, permissions []string) (*domain.Role, error) {
	if permissions == nil {
		permissions = []string{}
	}
	b, _ := json.Marshal(permissions)
	_, err := s.db.Exec(`UPDATE roles SET description = ?, permissions_json = ? WHERE id = ?`,
		description, string(b), id)
	if err != nil {
		return nil, err
	}
	return s.GetRoleByID(id)
}

func (s *Store) ListUsers() ([]domain.User, error) {
	rows, err := s.db.Query(`
		SELECT u.id, u.username, u.email, u.password_hash, u.role_id, r.name, r.permissions_json,
		       u.is_active, u.last_login_at, u.created_at, u.updated_at
		FROM users u
		JOIN roles r ON u.role_id = r.id
		ORDER BY u.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.User
	for rows.Next() {
		var u domain.User
		var permsJSON string
		var active int
		var lastLogin sql.NullTime
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.RoleID, &u.RoleName, &permsJSON,
			&active, &lastLogin, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		u.IsActive = active == 1
		if lastLogin.Valid {
			t := lastLogin.Time
			u.LastLoginAt = &t
		}
		_ = json.Unmarshal([]byte(permsJSON), &u.Permissions)
		out = append(out, u)
	}
	return out, nil
}

func (s *Store) GetUserByID(id int64) (*domain.User, error) {
	var u domain.User
	var permsJSON string
	var active int
	var lastLogin sql.NullTime
	err := s.db.QueryRow(`
		SELECT u.id, u.username, u.email, u.password_hash, u.role_id, r.name, r.permissions_json,
		       u.is_active, u.last_login_at, u.created_at, u.updated_at
		FROM users u
		JOIN roles r ON u.role_id = r.id
		WHERE u.id = ?`, id).
		Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.RoleID, &u.RoleName, &permsJSON,
			&active, &lastLogin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	u.IsActive = active == 1
	if lastLogin.Valid {
		t := lastLogin.Time
		u.LastLoginAt = &t
	}
	_ = json.Unmarshal([]byte(permsJSON), &u.Permissions)
	return &u, nil
}

func (s *Store) GetUserByUsername(username string) (*domain.User, error) {
	var u domain.User
	var permsJSON string
	var active int
	var lastLogin sql.NullTime
	err := s.db.QueryRow(`
		SELECT u.id, u.username, u.email, u.password_hash, u.role_id, r.name, r.permissions_json,
		       u.is_active, u.last_login_at, u.created_at, u.updated_at
		FROM users u
		JOIN roles r ON u.role_id = r.id
		WHERE u.username = ?`, username).
		Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.RoleID, &u.RoleName, &permsJSON,
			&active, &lastLogin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	u.IsActive = active == 1
	if lastLogin.Valid {
		t := lastLogin.Time
		u.LastLoginAt = &t
	}
	_ = json.Unmarshal([]byte(permsJSON), &u.Permissions)
	return &u, nil
}

func (s *Store) CreateUser(username, email, passwordHash string, roleID int64) (*domain.User, error) {
	res, err := s.db.Exec(`INSERT INTO users (username, email, password_hash, role_id, is_active) VALUES (?, ?, ?, ?, 1)`,
		username, email, passwordHash, roleID)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetUserByID(id)
}

func (s *Store) UpdateUser(id int64, email string, roleID int64, isActive bool, passwordHash string) (*domain.User, error) {
	activeInt := 0
	if isActive {
		activeInt = 1
	}
	if passwordHash != "" {
		_, err := s.db.Exec(`UPDATE users SET email = ?, role_id = ?, is_active = ?, password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			email, roleID, activeInt, passwordHash, id)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := s.db.Exec(`UPDATE users SET email = ?, role_id = ?, is_active = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			email, roleID, activeInt, id)
		if err != nil {
			return nil, err
		}
	}
	return s.GetUserByID(id)
}

func (s *Store) TouchUserLogin(id int64) {
	_, _ = s.db.Exec(`UPDATE users SET last_login_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
}

func (s *Store) DeleteUser(id int64) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (s *Store) CreateAPIKey(name, keyHash, keyPrefix, rawKey string, userID int64, scope string, permissions []string) (*domain.APIKey, error) {
	if scope == "" {
		scope = "all"
	}
	if permissions == nil {
		permissions = []string{}
	}
	b, _ := json.Marshal(permissions)
	res, err := s.db.Exec(`INSERT INTO api_keys (name, key_hash, key_prefix, raw_key, user_id, scope, permissions_json) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		name, keyHash, keyPrefix, rawKey, userID, scope, string(b))
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &domain.APIKey{
		ID:          id,
		Name:        name,
		KeyHash:     keyHash,
		KeyPrefix:   keyPrefix,
		RawKey:      rawKey,
		UserID:      userID,
		Scope:       scope,
		Permissions: permissions,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

func (s *Store) ListAPIKeys() ([]domain.APIKey, error) {
	rows, err := s.db.Query(`SELECT id, name, key_prefix, raw_key, user_id, scope, permissions_json, last_used_at, created_at FROM api_keys ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.APIKey
	for rows.Next() {
		var k domain.APIKey
		var permsJSON string
		var lastUsed sql.NullTime
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.RawKey, &k.UserID, &k.Scope, &permsJSON, &lastUsed, &k.CreatedAt); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			t := lastUsed.Time
			k.LastUsedAt = &t
		}
		_ = json.Unmarshal([]byte(permsJSON), &k.Permissions)
		out = append(out, k)
	}
	return out, nil
}

func (s *Store) GetAPIKeyByHash(keyHash string) (*domain.APIKey, *domain.User, error) {
	var k domain.APIKey
	var permsJSON string
	var lastUsed sql.NullTime
	err := s.db.QueryRow(`SELECT id, name, key_hash, key_prefix, raw_key, user_id, scope, permissions_json, last_used_at, created_at FROM api_keys WHERE key_hash = ?`, keyHash).
		Scan(&k.ID, &k.Name, &k.KeyHash, &k.KeyPrefix, &k.RawKey, &k.UserID, &k.Scope, &permsJSON, &lastUsed, &k.CreatedAt)
	if err != nil {
		return nil, nil, err
	}
	if lastUsed.Valid {
		t := lastUsed.Time
		k.LastUsedAt = &t
	}
	_ = json.Unmarshal([]byte(permsJSON), &k.Permissions)
	_, _ = s.db.Exec(`UPDATE api_keys SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`, k.ID)
	u, err := s.GetUserByID(k.UserID)
	if err != nil {
		return nil, nil, err
	}
	return &k, u, nil
}

func (s *Store) DeleteAPIKey(id int64) error {
	_, err := s.db.Exec(`DELETE FROM api_keys WHERE id = ?`, id)
	return err
}

// --- Tags, RootFolders, QualityProfiles ---

func (s *Store) ListTags() ([]domain.Tag, error) {
	rows, err := s.db.Query(`SELECT id, label FROM tags ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Tag{}
	for rows.Next() {
		var t domain.Tag
		if err := rows.Scan(&t.ID, &t.Label); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func (s *Store) CreateTag(label string) (*domain.Tag, error) {
	res, err := s.db.Exec(`INSERT INTO tags (label) VALUES (?) ON CONFLICT(label) DO UPDATE SET label=excluded.label`, label)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		_ = s.db.QueryRow(`SELECT id FROM tags WHERE label = ?`, label).Scan(&id)
	}
	return &domain.Tag{ID: id, Label: label}, nil
}

func (s *Store) DeleteTag(id int64) error {
	_, err := s.db.Exec(`DELETE FROM tags WHERE id = ?`, id)
	return err
}

func (s *Store) ListRootFolders(mediaType string) ([]domain.RootFolder, error) {
	query := `SELECT id, path, media_type, accessible, free_space, total_space FROM root_folders`
	var args []any
	if mediaType != "" && mediaType != "all" {
		query += ` WHERE media_type = ? OR media_type = 'all'`
		args = append(args, mediaType)
	}
	query += ` ORDER BY id ASC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.RootFolder{}
	for rows.Next() {
		var rf domain.RootFolder
		var acc int
		if err := rows.Scan(&rf.ID, &rf.Path, &rf.MediaType, &acc, &rf.FreeSpace, &rf.TotalSpace); err != nil {
			return nil, err
		}
		rf.Accessible = acc == 1
		rf.UnmappedFolders = []any{}
		out = append(out, rf)
	}
	return out, nil
}

func (s *Store) CreateRootFolder(rf domain.RootFolder) (*domain.RootFolder, error) {
	if rf.MediaType == "" {
		rf.MediaType = "all"
	}
	if rf.FreeSpace == 0 {
		rf.FreeSpace = 536870912000
	}
	if rf.TotalSpace == 0 {
		rf.TotalSpace = 1073741824000
	}
	res, err := s.db.Exec(`INSERT INTO root_folders (path, media_type, accessible, free_space, total_space) VALUES (?, ?, 1, ?, ?)`,
		rf.Path, rf.MediaType, rf.FreeSpace, rf.TotalSpace)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	rf.ID = id
	rf.Accessible = true
	rf.UnmappedFolders = []any{}
	return &rf, nil
}

func (s *Store) DeleteRootFolder(id int64) error {
	_, err := s.db.Exec(`DELETE FROM root_folders WHERE id = ?`, id)
	return err
}

func (s *Store) ListQualityProfiles() ([]domain.QualityProfile, error) {
	rows, err := s.db.Query(`SELECT id, name, upgrade_allowed, cutoff, items_json, min_format_score, cutoff_format_score FROM quality_profiles ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.QualityProfile{}
	for rows.Next() {
		var qp domain.QualityProfile
		var upg int
		var itemsJSON string
		if err := rows.Scan(&qp.ID, &qp.Name, &upg, &qp.Cutoff, &itemsJSON, &qp.MinFormatScore, &qp.CutoffFormatScore); err != nil {
			return nil, err
		}
		qp.UpgradeAllowed = upg == 1
		_ = json.Unmarshal([]byte(itemsJSON), &qp.Items)
		if qp.Items == nil {
			qp.Items = []domain.QualityItem{}
		}
		out = append(out, qp)
	}
	return out, nil
}

func (s *Store) CreateQualityProfile(qp domain.QualityProfile) (*domain.QualityProfile, error) {
	upg := 0
	if qp.UpgradeAllowed {
		upg = 1
	}
	b, _ := json.Marshal(qp.Items)
	res, err := s.db.Exec(`INSERT INTO quality_profiles (name, upgrade_allowed, cutoff, items_json, min_format_score, cutoff_format_score) VALUES (?, ?, ?, ?, ?, ?)`,
		qp.Name, upg, qp.Cutoff, string(b), qp.MinFormatScore, qp.CutoffFormatScore)
	if err != nil {
		return nil, err
	}
	qp.ID, _ = res.LastInsertId()
	return &qp, nil
}

// --- TV Series & Episodes ---

func (s *Store) ListSeries() ([]domain.Series, error) {
	rows, err := s.db.Query(`
		SELECT id, title, sort_title, status, ended, overview, network, air_time, images_json, seasons_json,
		       year, path, quality_profile_id, language_profile_id, season_folder, monitored, use_scene_numbering,
		       runtime, tvdb_id, tv_rage_id, tv_maze_id, imdb_id, title_slug, root_folder_path, series_type,
		       clean_title, genres_json, tags_json, added
		FROM series ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Series{}
	for rows.Next() {
		sr, err := scanSeries(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sr)
	}
	return out, nil
}

func (s *Store) GetSeriesByID(id int64) (*domain.Series, error) {
	rows, err := s.db.Query(`
		SELECT id, title, sort_title, status, ended, overview, network, air_time, images_json, seasons_json,
		       year, path, quality_profile_id, language_profile_id, season_folder, monitored, use_scene_numbering,
		       runtime, tvdb_id, tv_rage_id, tv_maze_id, imdb_id, title_slug, root_folder_path, series_type,
		       clean_title, genres_json, tags_json, added
		FROM series WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	sr, err := scanSeries(rows)
	if err != nil {
		return nil, err
	}
	return &sr, nil
}

func scanSeries(rows *sql.Rows) (domain.Series, error) {
	var sr domain.Series
	var ended, seasonFolder, monitored, sceneNum int
	var imagesJSON, seasonsJSON, genresJSON, tagsJSON string
	err := rows.Scan(
		&sr.ID, &sr.Title, &sr.SortTitle, &sr.Status, &ended, &sr.Overview, &sr.Network, &sr.AirTime,
		&imagesJSON, &seasonsJSON, &sr.Year, &sr.Path, &sr.QualityProfileID, &sr.LanguageProfileID,
		&seasonFolder, &monitored, &sceneNum, &sr.Runtime, &sr.TvdbID, &sr.TvRageID, &sr.TvMazeID,
		&sr.ImdbID, &sr.TitleSlug, &sr.RootFolderPath, &sr.SeriesType, &sr.CleanTitle,
		&genresJSON, &tagsJSON, &sr.Added,
	)
	if err != nil {
		return sr, err
	}
	sr.Ended = ended == 1
	sr.SeasonFolder = seasonFolder == 1
	sr.Monitored = monitored == 1
	sr.UseSceneNumbering = sceneNum == 1
	_ = json.Unmarshal([]byte(imagesJSON), &sr.Images)
	_ = json.Unmarshal([]byte(seasonsJSON), &sr.Seasons)
	_ = json.Unmarshal([]byte(genresJSON), &sr.Genres)
	_ = json.Unmarshal([]byte(tagsJSON), &sr.Tags)
	if sr.Images == nil {
		sr.Images = []domain.MediaImage{}
	}
	if sr.Seasons == nil {
		sr.Seasons = []domain.Season{}
	}
	if sr.Genres == nil {
		sr.Genres = []string{}
	}
	if sr.Tags == nil {
		sr.Tags = []int64{}
	}
	sr.Statistics = map[string]any{
		"seasonCount":       len(sr.Seasons),
		"episodeFileCount":  0,
		"episodeCount":      12,
		"totalEpisodeCount": 12,
		"sizeOnDisk":        0,
		"percentOfEpisodes": 0.0,
	}
	return sr, nil
}

func (s *Store) CreateSeries(sr domain.Series) (*domain.Series, error) {
	if sr.Title == "" {
		return nil, errors.New("title is required")
	}
	if sr.SortTitle == "" {
		sr.SortTitle = strings.ToLower(sr.Title)
	}
	if sr.CleanTitle == "" {
		sr.CleanTitle = strings.ToLower(strings.ReplaceAll(sr.Title, " ", ""))
	}
	if sr.TitleSlug == "" {
		sr.TitleSlug = strings.ToLower(strings.ReplaceAll(sr.Title, " ", "-"))
	}
	if sr.RootFolderPath == "" {
		sr.RootFolderPath = "/data/tv"
	}
	if sr.Path == "" {
		sr.Path = strings.TrimRight(sr.RootFolderPath, "/") + "/" + sr.Title
	}
	if sr.Status == "" {
		sr.Status = "continuing"
	}
	if sr.SeriesType == "" {
		sr.SeriesType = "standard"
	}
	if sr.QualityProfileID == 0 {
		sr.QualityProfileID = 1
	}
	if sr.LanguageProfileID == 0 {
		sr.LanguageProfileID = 1
	}
	if len(sr.Seasons) == 0 {
		sr.Seasons = []domain.Season{
			{SeasonNumber: 1, Monitored: true},
		}
	}
	imgB, _ := json.Marshal(sr.Images)
	seaB, _ := json.Marshal(sr.Seasons)
	genB, _ := json.Marshal(sr.Genres)
	tagB, _ := json.Marshal(sr.Tags)
	res, err := s.db.Exec(`
		INSERT INTO series (
			title, sort_title, status, ended, overview, network, air_time, images_json, seasons_json,
			year, path, quality_profile_id, language_profile_id, season_folder, monitored, use_scene_numbering,
			runtime, tvdb_id, tv_rage_id, tv_maze_id, imdb_id, title_slug, root_folder_path, series_type,
			clean_title, genres_json, tags_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sr.Title, sr.SortTitle, sr.Status, boolToInt(sr.Ended), sr.Overview, sr.Network, sr.AirTime,
		string(imgB), string(seaB), sr.Year, sr.Path, sr.QualityProfileID, sr.LanguageProfileID,
		boolToInt(sr.SeasonFolder), boolToInt(sr.Monitored), boolToInt(sr.UseSceneNumbering),
		sr.Runtime, sr.TvdbID, sr.TvRageID, sr.TvMazeID, sr.ImdbID, sr.TitleSlug, sr.RootFolderPath,
		sr.SeriesType, sr.CleanTitle, string(genB), string(tagB),
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	// Also seed starter episodes for season 1
	for epNum := 1; epNum <= 3; epNum++ {
		_, _ = s.db.Exec(`
			INSERT INTO episodes (series_id, tvdb_id, season_number, episode_number, title, air_date, overview, has_file, monitored, absolute_episode_number)
			VALUES (?, ?, 1, ?, ?, ?, ?, 0, ?, ?)`,
			id, sr.TvdbID*100+epNum, epNum, fmt.Sprintf("Episode %d", epNum),
			time.Now().AddDate(0, 0, epNum*7).Format("2006-01-02"),
			fmt.Sprintf("Episode %d of %s", epNum, sr.Title),
			boolToInt(sr.Monitored), epNum,
		)
	}
	return s.GetSeriesByID(id)
}

func (s *Store) UpdateSeries(sr domain.Series) (*domain.Series, error) {
	imgB, _ := json.Marshal(sr.Images)
	seaB, _ := json.Marshal(sr.Seasons)
	genB, _ := json.Marshal(sr.Genres)
	tagB, _ := json.Marshal(sr.Tags)
	_, err := s.db.Exec(`
		UPDATE series SET
			title = ?, sort_title = ?, status = ?, ended = ?, overview = ?, network = ?, air_time = ?,
			images_json = ?, seasons_json = ?, year = ?, path = ?, quality_profile_id = ?, language_profile_id = ?,
			season_folder = ?, monitored = ?, use_scene_numbering = ?, runtime = ?, root_folder_path = ?,
			series_type = ?, genres_json = ?, tags_json = ?
		WHERE id = ?`,
		sr.Title, sr.SortTitle, sr.Status, boolToInt(sr.Ended), sr.Overview, sr.Network, sr.AirTime,
		string(imgB), string(seaB), sr.Year, sr.Path, sr.QualityProfileID, sr.LanguageProfileID,
		boolToInt(sr.SeasonFolder), boolToInt(sr.Monitored), boolToInt(sr.UseSceneNumbering),
		sr.Runtime, sr.RootFolderPath, sr.SeriesType, string(genB), string(tagB), sr.ID,
	)
	if err != nil {
		return nil, err
	}
	return s.GetSeriesByID(sr.ID)
}

func (s *Store) DeleteSeries(id int64) error {
	_, err := s.db.Exec(`DELETE FROM series WHERE id = ?`, id)
	return err
}

func (s *Store) ListEpisodes(seriesID int64) ([]domain.Episode, error) {
	query := `SELECT id, series_id, tvdb_id, episode_file_id, season_number, episode_number, title, air_date, air_date_utc, overview, has_file, monitored, absolute_episode_number FROM episodes`
	var args []any
	if seriesID > 0 {
		query += ` WHERE series_id = ?`
		args = append(args, seriesID)
	}
	query += ` ORDER BY season_number ASC, episode_number ASC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Episode{}
	for rows.Next() {
		var ep domain.Episode
		var hasFile, mon int
		if err := rows.Scan(&ep.ID, &ep.SeriesID, &ep.TvdbID, &ep.EpisodeFileID, &ep.SeasonNumber, &ep.EpisodeNumber,
			&ep.Title, &ep.AirDate, &ep.AirDateUtc, &ep.Overview, &hasFile, &mon, &ep.AbsoluteEpisodeNum); err != nil {
			return nil, err
		}
		ep.HasFile = hasFile == 1
		ep.Monitored = mon == 1
		out = append(out, ep)
	}
	return out, nil
}

// --- Movies ---

func (s *Store) ListMovies() ([]domain.Movie, error) {
	rows, err := s.db.Query(`
		SELECT id, title, original_title, sort_title, status, overview, in_cinemas, physical_release, digital_release,
		       images_json, website, year, has_file, movie_file_id, path, quality_profile_id, monitored,
		       minimum_availability, is_available, folder_name, runtime, clean_title, imdb_id, tmdb_id,
		       title_slug, root_folder_path, studio, genres_json, tags_json, size_on_disk, added
		FROM movies ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Movie{}
	for rows.Next() {
		mv, err := scanMovie(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func (s *Store) GetMovieByID(id int64) (*domain.Movie, error) {
	rows, err := s.db.Query(`
		SELECT id, title, original_title, sort_title, status, overview, in_cinemas, physical_release, digital_release,
		       images_json, website, year, has_file, movie_file_id, path, quality_profile_id, monitored,
		       minimum_availability, is_available, folder_name, runtime, clean_title, imdb_id, tmdb_id,
		       title_slug, root_folder_path, studio, genres_json, tags_json, size_on_disk, added
		FROM movies WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	mv, err := scanMovie(rows)
	if err != nil {
		return nil, err
	}
	return &mv, nil
}

func scanMovie(rows *sql.Rows) (domain.Movie, error) {
	var mv domain.Movie
	var hasFile, mon, isAvail int
	var imagesJSON, genresJSON, tagsJSON string
	err := rows.Scan(
		&mv.ID, &mv.Title, &mv.OriginalTitle, &mv.SortTitle, &mv.Status, &mv.Overview,
		&mv.InCinemas, &mv.PhysicalRelease, &mv.DigitalRelease, &imagesJSON, &mv.Website,
		&mv.Year, &hasFile, &mv.MovieFileID, &mv.Path, &mv.QualityProfileID, &mon,
		&mv.MinimumAvailability, &isAvail, &mv.FolderName, &mv.Runtime, &mv.CleanTitle,
		&mv.ImdbID, &mv.TmdbID, &mv.TitleSlug, &mv.RootFolderPath, &mv.Studio,
		&genresJSON, &tagsJSON, &mv.SizeOnDisk, &mv.Added,
	)
	if err != nil {
		return mv, err
	}
	mv.HasFile = hasFile == 1
	mv.Monitored = mon == 1
	mv.IsAvailable = isAvail == 1
	_ = json.Unmarshal([]byte(imagesJSON), &mv.Images)
	_ = json.Unmarshal([]byte(genresJSON), &mv.Genres)
	_ = json.Unmarshal([]byte(tagsJSON), &mv.Tags)
	if mv.Images == nil {
		mv.Images = []domain.MediaImage{}
	}
	if mv.Genres == nil {
		mv.Genres = []string{}
	}
	if mv.Tags == nil {
		mv.Tags = []int64{}
	}
	mv.Statistics = map[string]any{
		"movieFileCount": boolToInt(mv.HasFile),
		"sizeOnDisk":     mv.SizeOnDisk,
	}
	return mv, nil
}

func (s *Store) CreateMovie(mv domain.Movie) (*domain.Movie, error) {
	if mv.Title == "" {
		return nil, errors.New("title is required")
	}
	if mv.OriginalTitle == "" {
		mv.OriginalTitle = mv.Title
	}
	if mv.SortTitle == "" {
		mv.SortTitle = strings.ToLower(mv.Title)
	}
	if mv.CleanTitle == "" {
		mv.CleanTitle = strings.ToLower(strings.ReplaceAll(mv.Title, " ", ""))
	}
	if mv.TitleSlug == "" {
		mv.TitleSlug = strings.ToLower(strings.ReplaceAll(mv.Title, " ", "-"))
	}
	if mv.RootFolderPath == "" {
		mv.RootFolderPath = "/data/movies"
	}
	if mv.FolderName == "" {
		mv.FolderName = fmt.Sprintf("%s (%d)", mv.Title, mv.Year)
	}
	if mv.Path == "" {
		mv.Path = strings.TrimRight(mv.RootFolderPath, "/") + "/" + mv.FolderName
	}
	if mv.Status == "" {
		mv.Status = "released"
	}
	if mv.MinimumAvailability == "" {
		mv.MinimumAvailability = "released"
	}
	if mv.QualityProfileID == 0 {
		mv.QualityProfileID = 1
	}
	imgB, _ := json.Marshal(mv.Images)
	genB, _ := json.Marshal(mv.Genres)
	tagB, _ := json.Marshal(mv.Tags)
	res, err := s.db.Exec(`
		INSERT INTO movies (
			title, original_title, sort_title, status, overview, in_cinemas, physical_release, digital_release,
			images_json, website, year, has_file, movie_file_id, path, quality_profile_id, monitored,
			minimum_availability, is_available, folder_name, runtime, clean_title, imdb_id, tmdb_id,
			title_slug, root_folder_path, studio, genres_json, tags_json, size_on_disk
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		mv.Title, mv.OriginalTitle, mv.SortTitle, mv.Status, mv.Overview, mv.InCinemas, mv.PhysicalRelease, mv.DigitalRelease,
		string(imgB), mv.Website, mv.Year, boolToInt(mv.HasFile), mv.MovieFileID, mv.Path, mv.QualityProfileID,
		boolToInt(mv.Monitored), mv.MinimumAvailability, mv.FolderName, mv.Runtime, mv.CleanTitle,
		mv.ImdbID, mv.TmdbID, mv.TitleSlug, mv.RootFolderPath, mv.Studio, string(genB), string(tagB), mv.SizeOnDisk,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetMovieByID(id)
}

func (s *Store) UpdateMovie(mv domain.Movie) (*domain.Movie, error) {
	imgB, _ := json.Marshal(mv.Images)
	genB, _ := json.Marshal(mv.Genres)
	tagB, _ := json.Marshal(mv.Tags)
	_, err := s.db.Exec(`
		UPDATE movies SET
			title = ?, sort_title = ?, status = ?, overview = ?, images_json = ?, year = ?,
			has_file = ?, path = ?, quality_profile_id = ?, monitored = ?, minimum_availability = ?,
			runtime = ?, studio = ?, genres_json = ?, tags_json = ?
		WHERE id = ?`,
		mv.Title, mv.SortTitle, mv.Status, mv.Overview, string(imgB), mv.Year,
		boolToInt(mv.HasFile), mv.Path, mv.QualityProfileID, boolToInt(mv.Monitored),
		mv.MinimumAvailability, mv.Runtime, mv.Studio, string(genB), string(tagB), mv.ID,
	)
	if err != nil {
		return nil, err
	}
	return s.GetMovieByID(mv.ID)
}

func (s *Store) DeleteMovie(id int64) error {
	_, err := s.db.Exec(`DELETE FROM movies WHERE id = ?`, id)
	return err
}

// --- Indexers ---

func (s *Store) ListIndexers() ([]domain.Indexer, error) {
	rows, err := s.db.Query(`
		SELECT id, name, implementation, implementation_name, config_contract, protocol, privacy,
		       enable, enable_rss, enable_automatic_search, enable_interactive_search,
		       supports_rss, supports_search, priority, base_url, api_key, definition_file, plugin_id,
		       categories_json, fields_json, tags_json, added
		FROM indexers ORDER BY priority ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Indexer{}
	for rows.Next() {
		idx, err := scanIndexer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, idx)
	}
	return out, nil
}

func (s *Store) GetIndexerByID(id int64) (*domain.Indexer, error) {
	rows, err := s.db.Query(`
		SELECT id, name, implementation, implementation_name, config_contract, protocol, privacy,
		       enable, enable_rss, enable_automatic_search, enable_interactive_search,
		       supports_rss, supports_search, priority, base_url, api_key, definition_file, plugin_id,
		       categories_json, fields_json, tags_json, added
		FROM indexers WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	idx, err := scanIndexer(rows)
	if err != nil {
		return nil, err
	}
	return &idx, nil
}

func scanIndexer(rows *sql.Rows) (domain.Indexer, error) {
	var idx domain.Indexer
	var en, enRss, enAuto, enInter, supRss, supSearch int
	var catJSON, fieldsJSON, tagsJSON string
	err := rows.Scan(
		&idx.ID, &idx.Name, &idx.Implementation, &idx.ImplementationName, &idx.ConfigContract,
		&idx.Protocol, &idx.Privacy, &en, &enRss, &enAuto, &enInter, &supRss, &supSearch,
		&idx.Priority, &idx.BaseURL, &idx.APIKey, &idx.DefinitionFile, &idx.PluginID,
		&catJSON, &fieldsJSON, &tagsJSON, &idx.Added,
	)
	if err != nil {
		return idx, err
	}
	idx.Enable = en == 1
	idx.EnableRss = enRss == 1
	idx.EnableAutomaticSearch = enAuto == 1
	idx.EnableInteractiveSearch = enInter == 1
	idx.SupportsRss = supRss == 1
	idx.SupportsSearch = supSearch == 1
	_ = json.Unmarshal([]byte(catJSON), &idx.Categories)
	_ = json.Unmarshal([]byte(fieldsJSON), &idx.Fields)
	_ = json.Unmarshal([]byte(tagsJSON), &idx.Tags)
	if idx.Categories == nil {
		idx.Categories = []int{2000, 5000}
	}
	if idx.Fields == nil {
		idx.Fields = []domain.ConfigField{}
	}
	if idx.Tags == nil {
		idx.Tags = []int64{}
	}
	return idx, nil
}

func (s *Store) CreateIndexer(idx domain.Indexer) (*domain.Indexer, error) {
	if idx.Name == "" {
		return nil, errors.New("indexer name is required")
	}
	if idx.Implementation == "" {
		idx.Implementation = "Torznab"
	}
	if idx.ImplementationName == "" {
		idx.ImplementationName = idx.Implementation
	}
	if idx.ConfigContract == "" {
		idx.ConfigContract = idx.Implementation + "Settings"
	}
	if idx.Protocol == "" {
		idx.Protocol = "torrent"
	}
	if idx.Privacy == "" {
		idx.Privacy = "public"
	}
	if idx.Priority == 0 {
		idx.Priority = 25
	}
	if len(idx.Categories) == 0 {
		idx.Categories = []int{2000, 5000}
	}
	idx.SupportsRss = true
	idx.SupportsSearch = true

	catB, _ := json.Marshal(idx.Categories)
	fldB, _ := json.Marshal(idx.Fields)
	tagB, _ := json.Marshal(idx.Tags)
	res, err := s.db.Exec(`
		INSERT INTO indexers (
			name, implementation, implementation_name, config_contract, protocol, privacy,
			enable, enable_rss, enable_automatic_search, enable_interactive_search,
			supports_rss, supports_search, priority, base_url, api_key, definition_file, plugin_id,
			categories_json, fields_json, tags_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 1, ?, ?, ?, ?, ?, ?, ?, ?)`,
		idx.Name, idx.Implementation, idx.ImplementationName, idx.ConfigContract, idx.Protocol, idx.Privacy,
		boolToInt(idx.Enable), boolToInt(idx.EnableRss), boolToInt(idx.EnableAutomaticSearch),
		boolToInt(idx.EnableInteractiveSearch), idx.Priority, idx.BaseURL, idx.APIKey,
		idx.DefinitionFile, idx.PluginID, string(catB), string(fldB), string(tagB),
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetIndexerByID(id)
}

func (s *Store) UpdateIndexer(idx domain.Indexer) (*domain.Indexer, error) {
	catB, _ := json.Marshal(idx.Categories)
	fldB, _ := json.Marshal(idx.Fields)
	tagB, _ := json.Marshal(idx.Tags)
	_, err := s.db.Exec(`
		UPDATE indexers SET
			name = ?, implementation = ?, protocol = ?, privacy = ?, enable = ?,
			enable_rss = ?, enable_automatic_search = ?, enable_interactive_search = ?,
			priority = ?, base_url = ?, api_key = ?, definition_file = ?, plugin_id = ?,
			categories_json = ?, fields_json = ?, tags_json = ?
		WHERE id = ?`,
		idx.Name, idx.Implementation, idx.Protocol, idx.Privacy, boolToInt(idx.Enable),
		boolToInt(idx.EnableRss), boolToInt(idx.EnableAutomaticSearch), boolToInt(idx.EnableInteractiveSearch),
		idx.Priority, idx.BaseURL, idx.APIKey, idx.DefinitionFile, idx.PluginID,
		string(catB), string(fldB), string(tagB), idx.ID,
	)
	if err != nil {
		return nil, err
	}
	return s.GetIndexerByID(idx.ID)
}

func (s *Store) DeleteIndexer(id int64) error {
	_, err := s.db.Exec(`DELETE FROM indexers WHERE id = ?`, id)
	return err
}

// --- Download Clients & Queue & History ---

func (s *Store) ListDownloadClients() ([]domain.DownloadClient, error) {
	rows, err := s.db.Query(`
		SELECT id, name, implementation, implementation_name, config_contract, protocol, enable,
		       priority, host, port, use_ssl, username, password, category, remove_completed,
		       remove_failed, fields_json, tags_json
		FROM download_clients ORDER BY priority ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.DownloadClient{}
	for rows.Next() {
		var dc domain.DownloadClient
		var en, ssl, remComp, remFail int
		var fldJSON, tagJSON string
		if err := rows.Scan(
			&dc.ID, &dc.Name, &dc.Implementation, &dc.ImplementationName, &dc.ConfigContract,
			&dc.Protocol, &en, &dc.Priority, &dc.Host, &dc.Port, &ssl, &dc.Username, &dc.Password,
			&dc.Category, &remComp, &remFail, &fldJSON, &tagJSON,
		); err != nil {
			return nil, err
		}
		dc.Enable = en == 1
		dc.UseSsl = ssl == 1
		dc.RemoveCompletedDownloads = remComp == 1
		dc.RemoveFailedDownloads = remFail == 1
		_ = json.Unmarshal([]byte(fldJSON), &dc.Fields)
		_ = json.Unmarshal([]byte(tagJSON), &dc.Tags)
		if dc.Fields == nil {
			dc.Fields = []domain.ConfigField{}
		}
		if dc.Tags == nil {
			dc.Tags = []int64{}
		}
		out = append(out, dc)
	}
	return out, nil
}

func (s *Store) CreateDownloadClient(dc domain.DownloadClient) (*domain.DownloadClient, error) {
	if dc.Implementation == "" {
		dc.Implementation = "qBittorrent"
	}
	if dc.ImplementationName == "" {
		dc.ImplementationName = dc.Implementation
	}
	if dc.ConfigContract == "" {
		dc.ConfigContract = dc.Implementation + "Settings"
	}
	if dc.Protocol == "" {
		dc.Protocol = "torrent"
	}
	if dc.Priority == 0 {
		dc.Priority = 1
	}
	if dc.Host == "" {
		dc.Host = "localhost"
	}
	if dc.Port == 0 {
		dc.Port = 8080
	}
	fldB, _ := json.Marshal(dc.Fields)
	tagB, _ := json.Marshal(dc.Tags)
	res, err := s.db.Exec(`
		INSERT INTO download_clients (
			name, implementation, implementation_name, config_contract, protocol, enable,
			priority, host, port, use_ssl, username, password, category, remove_completed,
			remove_failed, fields_json, tags_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dc.Name, dc.Implementation, dc.ImplementationName, dc.ConfigContract, dc.Protocol,
		boolToInt(dc.Enable), dc.Priority, dc.Host, dc.Port, boolToInt(dc.UseSsl),
		dc.Username, dc.Password, dc.Category, boolToInt(dc.RemoveCompletedDownloads),
		boolToInt(dc.RemoveFailedDownloads), string(fldB), string(tagB),
	)
	if err != nil {
		return nil, err
	}
	dc.ID, _ = res.LastInsertId()
	return &dc, nil
}

func (s *Store) DeleteDownloadClient(id int64) error {
	_, err := s.db.Exec(`DELETE FROM download_clients WHERE id = ?`, id)
	return err
}

func (s *Store) ListQueueItems(mediaType string) ([]domain.QueueItem, error) {
	query := `
		SELECT id, media_type, series_id, episode_id, movie_id, title, status,
		       tracked_download_status, tracked_download_state, size, sizeleft, timeleft,
		       estimated_completion, protocol, download_client, download_id, indexer, output_path, added
		FROM queue_items`
	var args []any
	if mediaType != "" && mediaType != "all" {
		query += ` WHERE media_type = ?`
		args = append(args, mediaType)
	}
	query += ` ORDER BY id DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.QueueItem{}
	for rows.Next() {
		var q domain.QueueItem
		if err := rows.Scan(
			&q.ID, &q.MediaType, &q.SeriesID, &q.EpisodeID, &q.MovieID, &q.Title, &q.Status,
			&q.TrackedDownloadStatus, &q.TrackedDownloadState, &q.Size, &q.Sizeleft, &q.Timeleft,
			&q.EstimatedCompletion, &q.Protocol, &q.DownloadClient, &q.DownloadID, &q.Indexer,
			&q.OutputPath, &q.Added,
		); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, nil
}

func (s *Store) CreateQueueItem(q domain.QueueItem) (*domain.QueueItem, error) {
	if q.Status == "" {
		q.Status = "downloading"
	}
	if q.TrackedDownloadStatus == "" {
		q.TrackedDownloadStatus = "ok"
	}
	if q.TrackedDownloadState == "" {
		q.TrackedDownloadState = "downloading"
	}
	if q.Timeleft == "" {
		q.Timeleft = "00:08:45"
	}
	if q.EstimatedCompletion.IsZero() {
		q.EstimatedCompletion = time.Now().UTC().Add(9 * time.Minute)
	}
	res, err := s.db.Exec(`
		INSERT INTO queue_items (
			media_type, series_id, episode_id, movie_id, title, status,
			tracked_download_status, tracked_download_state, size, sizeleft, timeleft,
			estimated_completion, protocol, download_client, download_id, indexer, output_path
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		q.MediaType, q.SeriesID, q.EpisodeID, q.MovieID, q.Title, q.Status,
		q.TrackedDownloadStatus, q.TrackedDownloadState, q.Size, q.Sizeleft, q.Timeleft,
		q.EstimatedCompletion, q.Protocol, q.DownloadClient, q.DownloadID, q.Indexer, q.OutputPath,
	)
	if err != nil {
		return nil, err
	}
	q.ID, _ = res.LastInsertId()
	q.Added = time.Now().UTC()
	return &q, nil
}

func (s *Store) DeleteQueueItem(id int64) error {
	_, err := s.db.Exec(`DELETE FROM queue_items WHERE id = ?`, id)
	return err
}

func (s *Store) ListHistory(mediaType string) ([]domain.HistoryRecord, error) {
	query := `SELECT id, media_type, series_id, episode_id, movie_id, source_title, quality_json, date, event_type, data_json FROM history_records`
	var args []any
	if mediaType != "" && mediaType != "all" {
		query += ` WHERE media_type = ?`
		args = append(args, mediaType)
	}
	query += ` ORDER BY date DESC LIMIT 100`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.HistoryRecord{}
	for rows.Next() {
		var h domain.HistoryRecord
		var qJSON, dJSON string
		if err := rows.Scan(&h.ID, &h.MediaType, &h.SeriesID, &h.EpisodeID, &h.MovieID, &h.SourceTitle, &qJSON, &h.Date, &h.EventType, &dJSON); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(qJSON), &h.Quality)
		_ = json.Unmarshal([]byte(dJSON), &h.Data)
		out = append(out, h)
	}
	return out, nil
}

func (s *Store) CreateHistory(h domain.HistoryRecord) error {
	qB, _ := json.Marshal(h.Quality)
	dB, _ := json.Marshal(h.Data)
	_, err := s.db.Exec(`
		INSERT INTO history_records (media_type, series_id, episode_id, movie_id, source_title, quality_json, event_type, data_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		h.MediaType, h.SeriesID, h.EpisodeID, h.MovieID, h.SourceTitle, string(qB), h.EventType, string(dB),
	)
	return err
}

// --- Plugin Configs ---

func (s *Store) GetPluginConfig(pluginID string) (bool, map[string]any, error) {
	var en int
	var settingsJSON string
	err := s.db.QueryRow(`SELECT enabled, settings_json FROM plugin_configs WHERE plugin_id = ?`, pluginID).
		Scan(&en, &settingsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return true, map[string]any{}, nil
	}
	if err != nil {
		return false, nil, err
	}
	out := map[string]any{}
	_ = json.Unmarshal([]byte(settingsJSON), &out)
	return en == 1, out, nil
}

func (s *Store) SavePluginConfig(pluginID string, enabled bool, settings map[string]any) error {
	b, _ := json.Marshal(settings)
	_, err := s.db.Exec(`
		INSERT INTO plugin_configs (plugin_id, enabled, settings_json, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(plugin_id) DO UPDATE SET
			enabled = excluded.enabled,
			settings_json = excluded.settings_json,
			updated_at = CURRENT_TIMESTAMP`,
		pluginID, boolToInt(enabled), string(b),
	)
	return err
}

// --- Indexer Proxies (FlareSolverr / HTTP / SOCKS) ---

func (s *Store) ListIndexerProxies() ([]domain.IndexerProxy, error) {
	rows, err := s.db.Query(`
		SELECT id, name, implementation, implementation_name, config_contract,
		       host, request_timeout, session_ttl_minutes, disable_media, enable,
		       fields_json, tags_json, added
		FROM indexer_proxies ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.IndexerProxy{}
	for rows.Next() {
		ip, err := scanIndexerProxy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ip)
	}
	return out, nil
}

func (s *Store) GetIndexerProxyByID(id int64) (*domain.IndexerProxy, error) {
	rows, err := s.db.Query(`
		SELECT id, name, implementation, implementation_name, config_contract,
		       host, request_timeout, session_ttl_minutes, disable_media, enable,
		       fields_json, tags_json, added
		FROM indexer_proxies WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	ip, err := scanIndexerProxy(rows)
	if err != nil {
		return nil, err
	}
	return &ip, nil
}

func scanIndexerProxy(rows *sql.Rows) (domain.IndexerProxy, error) {
	var ip domain.IndexerProxy
	var disMedia, en int
	var fieldsJSON, tagsJSON string
	err := rows.Scan(
		&ip.ID, &ip.Name, &ip.Implementation, &ip.ImplementationName, &ip.ConfigContract,
		&ip.Host, &ip.RequestTimeout, &ip.SessionTTLMinutes, &disMedia, &en,
		&fieldsJSON, &tagsJSON, &ip.Added,
	)
	if err != nil {
		return ip, err
	}
	ip.DisableMedia = disMedia == 1
	ip.Enable = en == 1
	ip.OnHealthIssue = true
	ip.SupportsOnHealth = true
	_ = json.Unmarshal([]byte(fieldsJSON), &ip.Fields)
	_ = json.Unmarshal([]byte(tagsJSON), &ip.Tags)
	if len(ip.Fields) == 0 {
		ip.Fields = defaultIndexerProxyFields(ip)
	}
	if ip.Tags == nil {
		ip.Tags = []int64{}
	}
	return ip, nil
}

func defaultIndexerProxyFields(ip domain.IndexerProxy) []domain.ConfigField {
	return []domain.ConfigField{
		{Name: "host", Label: "Host", Type: "textbox", Value: ip.Host, HelpText: "FlareSolverr endpoint URL (e.g. http://localhost:8191/)"},
		{Name: "requestTimeout", Label: "Request Timeout", Type: "number", Value: ip.RequestTimeout, HelpText: "FlareSolverr maxTimeout Request Parameter (seconds, 1-180)"},
		{Name: "sessionTtlMinutes", Label: "Session TTL (minutes)", Type: "number", Value: ip.SessionTTLMinutes, HelpText: "Optional FlareSolverr session_ttl_minutes for browser reuse"},
		{Name: "disableMedia", Label: "Disable Media", Type: "checkbox", Value: ip.DisableMedia, HelpText: "Prevent images, CSS, and fonts from loading in headless browser"},
	}
}

func normalizeIndexerProxy(ip *domain.IndexerProxy) {
	// Extract host / requestTimeout from Prowlarr fields array if provided via Prowlarr v1 API
	for _, f := range ip.Fields {
		switch strings.ToLower(f.Name) {
		case "host":
			if s, ok := f.Value.(string); ok && strings.TrimSpace(s) != "" {
				ip.Host = strings.TrimSpace(s)
			}
		case "requesttimeout":
			switch v := f.Value.(type) {
			case float64:
				ip.RequestTimeout = int(v)
			case int:
				ip.RequestTimeout = v
			}
		case "sessionttlminutes", "session_ttl_minutes":
			switch v := f.Value.(type) {
			case float64:
				ip.SessionTTLMinutes = int(v)
			case int:
				ip.SessionTTLMinutes = v
			}
		case "disablemedia", "disable_media":
			if b, ok := f.Value.(bool); ok {
				ip.DisableMedia = b
			}
		}
	}
	if ip.Name == "" {
		ip.Name = "FlareSolverr"
	}
	if ip.Implementation == "" {
		ip.Implementation = "FlareSolverr"
	}
	if ip.ImplementationName == "" {
		ip.ImplementationName = ip.Implementation
	}
	if ip.ConfigContract == "" {
		ip.ConfigContract = ip.Implementation + "Settings"
	}
	if ip.Host == "" {
		ip.Host = "http://localhost:8191/"
	}
	if ip.RequestTimeout <= 0 {
		ip.RequestTimeout = 60
	}
	if ip.RequestTimeout > 180 {
		ip.RequestTimeout = 180
	}
	if ip.SessionTTLMinutes < 0 {
		ip.SessionTTLMinutes = 0
	}
	ip.Fields = defaultIndexerProxyFields(*ip)
	if ip.Tags == nil {
		ip.Tags = []int64{}
	}
}

func (s *Store) CreateIndexerProxy(ip domain.IndexerProxy) (*domain.IndexerProxy, error) {
	normalizeIndexerProxy(&ip)
	fldB, _ := json.Marshal(ip.Fields)
	tagB, _ := json.Marshal(ip.Tags)
	res, err := s.db.Exec(`
		INSERT INTO indexer_proxies (
			name, implementation, implementation_name, config_contract,
			host, request_timeout, session_ttl_minutes, disable_media, enable,
			fields_json, tags_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ip.Name, ip.Implementation, ip.ImplementationName, ip.ConfigContract,
		ip.Host, ip.RequestTimeout, ip.SessionTTLMinutes, boolToInt(ip.DisableMedia),
		boolToInt(ip.Enable), string(fldB), string(tagB),
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetIndexerProxyByID(id)
}

func (s *Store) UpdateIndexerProxy(ip domain.IndexerProxy) (*domain.IndexerProxy, error) {
	normalizeIndexerProxy(&ip)
	fldB, _ := json.Marshal(ip.Fields)
	tagB, _ := json.Marshal(ip.Tags)
	_, err := s.db.Exec(`
		UPDATE indexer_proxies SET
			name = ?, implementation = ?, implementation_name = ?, config_contract = ?,
			host = ?, request_timeout = ?, session_ttl_minutes = ?, disable_media = ?, enable = ?,
			fields_json = ?, tags_json = ?
		WHERE id = ?`,
		ip.Name, ip.Implementation, ip.ImplementationName, ip.ConfigContract,
		ip.Host, ip.RequestTimeout, ip.SessionTTLMinutes, boolToInt(ip.DisableMedia),
		boolToInt(ip.Enable), string(fldB), string(tagB), ip.ID,
	)
	if err != nil {
		return nil, err
	}
	return s.GetIndexerProxyByID(ip.ID)
}

func (s *Store) DeleteIndexerProxy(id int64) error {
	_, err := s.db.Exec(`DELETE FROM indexer_proxies WHERE id = ?`, id)
	return err
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
