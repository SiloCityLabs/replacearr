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
	t.Setenv("REPLACEARR_FLARESOLVERR_URL", "http://flaresolverr:8191/")
	t.Setenv("REPLACEARR_JWT_SECRET", "fixed-secret-for-config-test-32b")

	cfg := Load()
	if cfg.Port != 9090 {
		t.Fatalf("port %d", cfg.Port)
	}
	if cfg.AdminUser != "maave" {
		t.Fatalf("admin %s", cfg.AdminUser)
	}
	if cfg.FlareSolverrURL != "http://flaresolverr:8191/" {
		t.Fatalf("flaresolverr url %s", cfg.FlareSolverrURL)
	}
	if cfg.DatabasePath != filepath.Join(dir, "replacearr.db") {
		t.Fatalf("db path %s", cfg.DatabasePath)
	}
	if _, err := os.Stat(cfg.PluginDir); err != nil {
		t.Fatalf("plugin dir: %v", err)
	}
}
