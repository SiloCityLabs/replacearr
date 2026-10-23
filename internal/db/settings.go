package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"syscall"

	"github.com/replacearr/replacearr/internal/domain"
)

const appConfigKey = "app"

func (s *Store) GetAppConfig() (domain.AppConfig, error) {
	cfg := domain.DefaultAppConfig()
	var raw string
	err := s.db.QueryRow(`SELECT value_json FROM app_config WHERE key = ?`, appConfigKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		cfg.Normalize()
		return cfg, nil
	}
	if err != nil {
		return domain.AppConfig{}, err
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return domain.AppConfig{}, err
	}
	cfg.Normalize()
	return cfg, nil
}

func (s *Store) SaveAppConfig(cfg domain.AppConfig) (domain.AppConfig, error) {
	cfg.Normalize()
	cfg.Host.BindAddressEnvLocked = false
	cfg.Host.PortEnvLocked = false
	cfg.Host.URLBaseEnvLocked = false
	cfg.Host.PendingRestart = false
	cfg.Security.APIKey = ""

	b, err := json.Marshal(cfg)
	if err != nil {
		return cfg, err
	}
	b, err = stripStoredAPIKey(b)
	if err != nil {
		return cfg, err
	}
	_, err = s.db.Exec(`
		INSERT INTO app_config (key, value_json, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			value_json = excluded.value_json,
			updated_at = CURRENT_TIMESTAMP`,
		appConfigKey, string(b),
	)
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

// stripStoredAPIKey removes security.apiKey so the master key is never persisted.
func stripStoredAPIKey(b []byte) ([]byte, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	secRaw, ok := doc["security"]
	if !ok {
		return b, nil
	}
	var sec map[string]json.RawMessage
	if err := json.Unmarshal(secRaw, &sec); err != nil {
		return nil, err
	}
	delete(sec, "apiKey")
	secOut, err := json.Marshal(sec)
	if err != nil {
		return nil, err
	}
	doc["security"] = secOut
	return json.Marshal(doc)
}

func (s *Store) GetQualityProfileByID(id int64) (*domain.QualityProfile, error) {
	var qp domain.QualityProfile
	var upg int
	var itemsJSON string
	err := s.db.QueryRow(`SELECT id, name, upgrade_allowed, cutoff, items_json, min_format_score, cutoff_format_score FROM quality_profiles WHERE id = ?`, id).
		Scan(&qp.ID, &qp.Name, &upg, &qp.Cutoff, &itemsJSON, &qp.MinFormatScore, &qp.CutoffFormatScore)
	if err != nil {
		return nil, err
	}
	qp.UpgradeAllowed = upg == 1
	_ = json.Unmarshal([]byte(itemsJSON), &qp.Items)
	if qp.Items == nil {
		qp.Items = []domain.QualityItem{}
	}
	return &qp, nil
}

func (s *Store) UpdateQualityProfile(qp domain.QualityProfile) (*domain.QualityProfile, error) {
	if qp.ID == 0 {
		return nil, errors.New("quality profile id is required")
	}
	if strings.TrimSpace(qp.Name) == "" {
		return nil, errors.New("quality profile name is required")
	}
	upg := 0
	if qp.UpgradeAllowed {
		upg = 1
	}
	b, _ := json.Marshal(qp.Items)
	res, err := s.db.Exec(`UPDATE quality_profiles SET name = ?, upgrade_allowed = ?, cutoff = ?, items_json = ?, min_format_score = ?, cutoff_format_score = ? WHERE id = ?`,
		qp.Name, upg, qp.Cutoff, string(b), qp.MinFormatScore, qp.CutoffFormatScore, qp.ID)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, sql.ErrNoRows
	}
	return s.GetQualityProfileByID(qp.ID)
}

func (s *Store) DeleteQualityProfile(id int64) error {
	var seriesUse int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM series WHERE quality_profile_id = ?`, id).Scan(&seriesUse); err != nil {
		return err
	}
	if seriesUse > 0 {
		return errors.New("quality profile is in use")
	}
	var movieUse int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM movies WHERE quality_profile_id = ?`, id).Scan(&movieUse); err != nil {
		return err
	}
	if movieUse > 0 {
		return errors.New("quality profile is in use")
	}
	_, err := s.db.Exec(`DELETE FROM quality_profiles WHERE id = ?`, id)
	return err
}

func (s *Store) UpdateDownloadClient(dc domain.DownloadClient) (*domain.DownloadClient, error) {
	if strings.TrimSpace(dc.Name) == "" {
		return nil, errors.New("download client name is required")
	}
	var existingPassword string
	err := s.db.QueryRow(`SELECT password FROM download_clients WHERE id = ?`, dc.ID).Scan(&existingPassword)
	if err != nil {
		return nil, err
	}
	if dc.Password == "" {
		dc.Password = existingPassword
	}
	fldB, _ := json.Marshal(dc.Fields)
	tagB, _ := json.Marshal(dc.Tags)
	_, err = s.db.Exec(`
		UPDATE download_clients SET
			name = ?, implementation = ?, implementation_name = ?, config_contract = ?, protocol = ?, enable = ?,
			priority = ?, host = ?, port = ?, use_ssl = ?, username = ?, password = ?, category = ?, remove_completed = ?,
			remove_failed = ?, fields_json = ?, tags_json = ?
		WHERE id = ?`,
		dc.Name, dc.Implementation, dc.ImplementationName, dc.ConfigContract, dc.Protocol,
		boolToInt(dc.Enable), dc.Priority, dc.Host, dc.Port, boolToInt(dc.UseSsl),
		dc.Username, dc.Password, dc.Category, boolToInt(dc.RemoveCompletedDownloads),
		boolToInt(dc.RemoveFailedDownloads), string(fldB), string(tagB), dc.ID,
	)
	if err != nil {
		return nil, err
	}
	return s.getDownloadClientByID(dc.ID)
}

func (s *Store) getDownloadClientByID(id int64) (*domain.DownloadClient, error) {
	var dc domain.DownloadClient
	var en, ssl, remComp, remFail int
	var fldJSON, tagJSON string
	err := s.db.QueryRow(`
		SELECT id, name, implementation, implementation_name, config_contract, protocol, enable,
		       priority, host, port, use_ssl, username, password, category, remove_completed,
		       remove_failed, fields_json, tags_json
		FROM download_clients WHERE id = ?`, id).Scan(
		&dc.ID, &dc.Name, &dc.Implementation, &dc.ImplementationName, &dc.ConfigContract,
		&dc.Protocol, &en, &dc.Priority, &dc.Host, &dc.Port, &ssl, &dc.Username, &dc.Password,
		&dc.Category, &remComp, &remFail, &fldJSON, &tagJSON,
	)
	if err != nil {
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
	return &dc, nil
}

func (s *Store) ReplaceAPIKeySecret(id int64, keyHash, keyPrefix, rawKey string) error {
	res, err := s.db.Exec(`UPDATE api_keys SET key_hash = ?, key_prefix = ?, raw_key = ? WHERE id = ?`,
		keyHash, keyPrefix, rawKey, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) GetAPIKeyByName(name string) (*domain.APIKey, error) {
	var k domain.APIKey
	var permsJSON string
	var lastUsed sql.NullTime
	err := s.db.QueryRow(`SELECT id, name, key_prefix, raw_key, user_id, scope, permissions_json, last_used_at, created_at FROM api_keys WHERE name = ?`, name).
		Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.RawKey, &k.UserID, &k.Scope, &permsJSON, &lastUsed, &k.CreatedAt)
	if err != nil {
		return nil, err
	}
	if lastUsed.Valid {
		t := lastUsed.Time
		k.LastUsedAt = &t
	}
	_ = json.Unmarshal([]byte(permsJSON), &k.Permissions)
	if k.Permissions == nil {
		k.Permissions = []string{}
	}
	return &k, nil
}

func (s *Store) TagDetails(id int64) (*domain.TagDetails, error) {
	td := &domain.TagDetails{
		SeriesIDs:         []int64{},
		MovieIDs:          []int64{},
		IndexerIDs:        []int64{},
		DownloadClientIDs: []int64{},
		DelayProfileIDs:   []int64{},
		ReleaseProfileIDs: []int64{},
		NotificationIDs:   []int64{},
		ImportListIDs:     []int64{},
	}
	err := s.db.QueryRow(`SELECT id, label FROM tags WHERE id = ?`, id).Scan(&td.ID, &td.Label)
	if err != nil {
		return nil, err
	}

	series, err := s.ListSeries()
	if err != nil {
		return nil, err
	}
	for _, sr := range series {
		if containsInt64(sr.Tags, id) {
			td.SeriesIDs = append(td.SeriesIDs, sr.ID)
		}
	}
	movies, err := s.ListMovies()
	if err != nil {
		return nil, err
	}
	for _, mv := range movies {
		if containsInt64(mv.Tags, id) {
			td.MovieIDs = append(td.MovieIDs, mv.ID)
		}
	}
	indexers, err := s.ListIndexers()
	if err != nil {
		return nil, err
	}
	for _, idx := range indexers {
		if containsInt64(idx.Tags, id) {
			td.IndexerIDs = append(td.IndexerIDs, idx.ID)
		}
	}
	clients, err := s.ListDownloadClients()
	if err != nil {
		return nil, err
	}
	for _, dc := range clients {
		if containsInt64(dc.Tags, id) {
			td.DownloadClientIDs = append(td.DownloadClientIDs, dc.ID)
		}
	}

	cfg, err := s.GetAppConfig()
	if err != nil {
		return nil, err
	}
	for _, dp := range cfg.DelayProfiles {
		if containsInt64(dp.Tags, id) {
			td.DelayProfileIDs = append(td.DelayProfileIDs, dp.ID)
		}
	}
	for _, rp := range cfg.ReleaseProfiles {
		if containsInt64(rp.Tags, id) {
			td.ReleaseProfileIDs = append(td.ReleaseProfileIDs, rp.ID)
		}
	}
	for _, n := range cfg.Notifications {
		if containsInt64(n.Tags, id) {
			td.NotificationIDs = append(td.NotificationIDs, n.ID)
		}
	}
	for _, list := range cfg.ImportLists {
		if containsInt64(list.Tags, id) {
			td.ImportListIDs = append(td.ImportListIDs, list.ID)
		}
	}
	return td, nil
}

func containsInt64(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func (s *Store) RefreshRootFolderUsage(path string) (free, total int64, accessible bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		_, _ = s.db.Exec(`UPDATE root_folders SET accessible = 0 WHERE path = ?`, path)
		return 0, 0, false
	}
	free = int64(st.Bavail) * int64(st.Bsize)
	total = int64(st.Blocks) * int64(st.Bsize)
	_, _ = s.db.Exec(`UPDATE root_folders SET accessible = 1, free_space = ?, total_space = ? WHERE path = ?`,
		free, total, path)
	return free, total, true
}
