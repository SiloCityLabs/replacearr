package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadHonorsEnvironment(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REPLACEARR_CONFIG_DIR", dir)
	t.Setenv("REPLACEARR_PORT", "9090")
	t.Setenv("REPLACEARR_ADMIN_USER", "maave")

	cfg := Load()
	if cfg.Port != 9090 {
		t.Fatalf("port %d", cfg.Port)
	}
	if cfg.AdminUser != "maave" {
		t.Fatalf("admin %s", cfg.AdminUser)
	}
	if cfg.DatabasePath != filepath.Join(dir, "replacearr.db") {
		t.Fatalf("db path %s", cfg.DatabasePath)
	}
	if cfg.PluginDir != filepath.Join(dir, "plugins") {
		t.Fatalf("plugin dir %s", cfg.PluginDir)
	}
	if cfg.DefinitionsDir != filepath.Join(dir, "definitions") {
		t.Fatalf("definitions dir %s", cfg.DefinitionsDir)
	}
	if _, err := os.Stat(cfg.PluginDir); err != nil {
		t.Fatalf("plugin dir: %v", err)
	}
	if cfg.JWTSecret == "" {
		t.Fatal("expected auto-generated jwt secret")
	}
	if _, err := os.Stat(filepath.Join(dir, "jwt_secret.key")); err != nil {
		t.Fatalf("jwt secret file: %v", err)
	}
}

func TestLoadIgnoresRemovedPathOverrides(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REPLACEARR_CONFIG_DIR", dir)
	t.Setenv("REPLACEARR_DB_PATH", "/tmp/should-not-use.db")
	t.Setenv("REPLACEARR_PLUGIN_DIR", "/tmp/should-not-use")
	t.Setenv("REPLACEARR_DEFINITIONS_DIR", "/tmp/should-not-use-defs")
	t.Setenv("REPLACEARR_JWT_SECRET", "should-be-ignored-even-if-long-enough")
	t.Setenv("REPLACEARR_API_KEY", "should-be-ignored")
	t.Setenv("REPLACEARR_FLARESOLVERR_URL", "http://ignored/")
	t.Setenv("REPLACEARR_PLUGIN_CATALOG_URL", "https://example.com/registry.json")

	cfg := Load()
	if cfg.DatabasePath != filepath.Join(dir, "replacearr.db") {
		t.Fatalf("db path %s", cfg.DatabasePath)
	}
	if cfg.PluginDir != filepath.Join(dir, "plugins") {
		t.Fatalf("plugin dir %s", cfg.PluginDir)
	}
	if cfg.DefinitionsDir != filepath.Join(dir, "definitions") {
		t.Fatalf("definitions dir %s", cfg.DefinitionsDir)
	}
	if cfg.JWTSecret == "should-be-ignored-even-if-long-enough" {
		t.Fatal("jwt secret must not come from env")
	}
}
