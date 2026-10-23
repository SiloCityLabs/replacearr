package db

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/replacearr/replacearr/internal/domain"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestGetAppConfigDefaults(t *testing.T) {
	store := openTestStore(t)
	cfg, err := store.GetAppConfig()
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if cfg.Host.InstanceName != "Replacearr" {
		t.Fatalf("instance name %q", cfg.Host.InstanceName)
	}
	var found7, found9 bool
	for _, q := range cfg.QualityDefinitions {
		if q.ID == 7 && q.Name == "WEBDL-1080p" {
			found7 = true
		}
		if q.ID == 9 && q.Name == "Bluray-1080p" {
			found9 = true
		}
	}
	if !found7 || !found9 {
		t.Fatalf("missing quality definitions 7/9 (found7=%v found9=%v)", found7, found9)
	}
}

func TestSaveAppConfigRoundTripOmitsSecrets(t *testing.T) {
	store := openTestStore(t)
	cfg, err := store.GetAppConfig()
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	cfg.UI.Theme = "light"
	cfg.MediaManagement.RenameEpisodes = true
	cfg.Security.APIKey = "apiKey-should-not-persist"
	cfg.Host.PendingRestart = true

	saved, err := store.SaveAppConfig(cfg)
	if err != nil {
		t.Fatalf("save config: %v", err)
	}
	if saved.UI.Theme != "light" || !saved.MediaManagement.RenameEpisodes {
		t.Fatalf("saved config did not keep edits: theme=%q rename=%v", saved.UI.Theme, saved.MediaManagement.RenameEpisodes)
	}
	if saved.Security.APIKey != "" {
		t.Fatalf("saved api key %q", saved.Security.APIKey)
	}

	got, err := store.GetAppConfig()
	if err != nil {
		t.Fatalf("get after save: %v", err)
	}
	if got.UI.Theme != "light" || !got.MediaManagement.RenameEpisodes {
		t.Fatalf("round trip theme=%q rename=%v", got.UI.Theme, got.MediaManagement.RenameEpisodes)
	}
	if got.Security.APIKey != "" {
		t.Fatalf("loaded api key %q", got.Security.APIKey)
	}

	var raw string
	if err := store.DB().QueryRow(`SELECT value_json FROM app_config WHERE key = ?`, "app").Scan(&raw); err != nil {
		t.Fatalf("read value_json: %v", err)
	}
	if strings.Contains(raw, "apiKey") {
		t.Fatalf("value_json contains apiKey: %s", raw)
	}
	compact := strings.ReplaceAll(raw, " ", "")
	if strings.Contains(compact, `"pendingRestart":true`) {
		t.Fatalf("value_json has pendingRestart true: %s", raw)
	}
}

func TestQualityProfileUpdateAndInUse(t *testing.T) {
	store := openTestStore(t)
	qp, err := store.CreateQualityProfile(domain.QualityProfile{
		Name:           "Temp HD",
		UpgradeAllowed: true,
		Cutoff:         7,
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	qp.Name = "Temp HD Renamed"
	updated, err := store.UpdateQualityProfile(*qp)
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if updated.Name != "Temp HD Renamed" {
		t.Fatalf("name %q", updated.Name)
	}
	if err := store.DeleteQualityProfile(updated.ID); err != nil {
		t.Fatalf("delete profile: %v", err)
	}

	used, err := store.CreateQualityProfile(domain.QualityProfile{Name: "In Use"})
	if err != nil {
		t.Fatalf("create used profile: %v", err)
	}
	if _, err := store.CreateSeries(domain.Series{
		Title:            "Uses Profile",
		QualityProfileID: used.ID,
	}); err != nil {
		t.Fatalf("create series: %v", err)
	}
	err = store.DeleteQualityProfile(used.ID)
	if err == nil || !strings.Contains(err.Error(), "in use") {
		t.Fatalf("expected in-use error, got %v", err)
	}
}

func TestUpdateDownloadClientKeepsEmptyPassword(t *testing.T) {
	store := openTestStore(t)
	created, err := store.CreateDownloadClient(domain.DownloadClient{
		Name:     "qbit",
		Password: "s3cret",
	})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	created.Password = ""
	updated, err := store.UpdateDownloadClient(*created)
	if err != nil {
		t.Fatalf("update client: %v", err)
	}
	if updated.Password != "s3cret" {
		t.Fatalf("password %q", updated.Password)
	}
}

func TestTagDetailsLabel(t *testing.T) {
	store := openTestStore(t)
	tag, err := store.CreateTag("four-k")
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	details, err := store.TagDetails(tag.ID)
	if err != nil {
		t.Fatalf("tag details: %v", err)
	}
	if details.Label != "four-k" {
		t.Fatalf("label %q", details.Label)
	}
}
