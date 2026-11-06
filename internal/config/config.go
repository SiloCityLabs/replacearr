package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	ConfigDir      string `json:"configDir"`
	DatabasePath   string `json:"databasePath"`
	PluginDir      string `json:"pluginDir"`
	DefinitionsDir string `json:"definitionsDir"`
	JWTSecret      string `json:"-"`
	AdminUser      string `json:"adminUser"`
	AdminPass      string `json:"-"`
	Environment    string `json:"environment"`
	URLBase        string `json:"urlBase"`
}

func Load() *Config {
	cfgDir := envOrDefault("REPLACEARR_CONFIG_DIR", "./config")
	_ = os.MkdirAll(cfgDir, 0o755)

	dbPath := filepath.Join(cfgDir, "replacearr.db")
	pluginDir := filepath.Join(cfgDir, "plugins")
	_ = os.MkdirAll(pluginDir, 0o755)

	defDir := filepath.Join(cfgDir, "definitions")
	_ = os.MkdirAll(defDir, 0o755)

	port := 8989
	if pStr := os.Getenv("REPLACEARR_PORT"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			port = p
		}
	}

	secretFile := filepath.Join(cfgDir, "jwt_secret.key")
	jwtSecret := ""
	if b, err := os.ReadFile(secretFile); err == nil && len(b) >= 32 {
		jwtSecret = string(b)
	} else {
		raw := make([]byte, 32)
		_, _ = rand.Read(raw)
		jwtSecret = hex.EncodeToString(raw)
		_ = os.WriteFile(secretFile, []byte(jwtSecret), 0o600)
	}

	return &Config{
		Host:           envOrDefault("REPLACEARR_HOST", "0.0.0.0"),
		Port:           port,
		ConfigDir:      cfgDir,
		DatabasePath:   dbPath,
		PluginDir:      pluginDir,
		DefinitionsDir: defDir,
		JWTSecret:      jwtSecret,
		AdminUser:      envOrDefault("REPLACEARR_ADMIN_USER", "admin"),
		AdminPass:      envOrDefault("REPLACEARR_ADMIN_PASS", "replacearr"),
		Environment:    envOrDefault("REPLACEARR_ENV", "development"),
		URLBase:        envOrDefault("REPLACEARR_URL_BASE", ""),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
