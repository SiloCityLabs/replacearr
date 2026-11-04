package plugincatalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	OriginRegistry = "registry"
	OriginGitHub   = "github"
)

// Origin is persisted next to an installed marketplace binary for updates.
type Origin struct {
	ID               string `json:"id"`
	Source           string `json:"source"`
	RepoURL          string `json:"repoURL,omitempty"`
	ManifestURL      string `json:"manifestURL"`
	InstalledVersion string `json:"installedVersion"`
	InstalledAt      string `json:"installedAt"`
}

// OriginPath returns the sidecar path for a plugin id.
func OriginPath(pluginDir, pluginID string) string {
	return filepath.Join(pluginDir, pluginID+".origin.json")
}

// WriteOrigin persists install origin metadata.
func WriteOrigin(pluginDir string, origin Origin) error {
	if strings.TrimSpace(pluginDir) == "" {
		return fmt.Errorf("plugin directory is not configured")
	}
	if err := validatePluginID(origin.ID); err != nil {
		return err
	}
	if strings.TrimSpace(origin.ManifestURL) == "" {
		return fmt.Errorf("origin manifestURL is required")
	}
	if origin.Source == "" {
		origin.Source = OriginRegistry
	}
	raw, err := json.MarshalIndent(origin, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(OriginPath(pluginDir, origin.ID), raw, 0o644)
}

// ReadOrigin loads a sidecar if present.
func ReadOrigin(pluginDir, pluginID string) (Origin, bool) {
	raw, err := os.ReadFile(OriginPath(pluginDir, pluginID))
	if err != nil {
		return Origin{}, false
	}
	var origin Origin
	if err := json.Unmarshal(raw, &origin); err != nil {
		return Origin{}, false
	}
	if origin.ID == "" {
		origin.ID = pluginID
	}
	return origin, true
}

// RemoveOrigin deletes the sidecar if present.
func RemoveOrigin(pluginDir, pluginID string) error {
	err := os.Remove(OriginPath(pluginDir, pluginID))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ListOrigins reads all *.origin.json sidecars in pluginDir.
func ListOrigins(pluginDir string) ([]Origin, error) {
	if strings.TrimSpace(pluginDir) == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Origin
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".origin.json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".origin.json")
		if err := validatePluginID(id); err != nil {
			continue
		}
		origin, ok := ReadOrigin(pluginDir, id)
		if ok {
			out = append(out, origin)
		}
	}
	return out, nil
}
