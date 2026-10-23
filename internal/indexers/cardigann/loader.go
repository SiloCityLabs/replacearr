// Package cardigann loads and parses Prowlarr/Jackett Cardigann YAML indexer definitions.
package cardigann

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Definition represents a parsed Cardigann YAML indexer definition.
type Definition struct {
	ID          string           `json:"id" yaml:"id"`
	Name        string           `json:"name" yaml:"name"`
	Description string           `json:"description" yaml:"description"`
	Language    string           `json:"language" yaml:"language"`
	Type        string           `json:"type" yaml:"type"` // public, private, semi-private
	Encoding    string           `json:"encoding" yaml:"encoding"`
	Links       []string         `json:"links" yaml:"links"`
	Caps        map[string]any   `json:"caps,omitempty" yaml:"caps"`
	Settings    []map[string]any `json:"settings,omitempty" yaml:"settings"`
}

// LoadDefinitions loads up to `limit` (when limit > 0) Cardigann `.yml` / `.yaml` definitions
// from `dir`. If `dir` is empty, missing, or contains no valid `.yml` files, built-in fallback
// definitions are returned so Replacearr always provides indexer templates out of the box.
func LoadDefinitions(dir string, limit int) ([]Definition, error) {
	var defs []Definition

	if strings.TrimSpace(dir) != "" {
		entries, err := os.ReadDir(dir)
		if err == nil {
			names := make([]string, 0, len(entries))
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				lower := strings.ToLower(name)
				if strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml") {
					names = append(names, name)
				}
			}
			sort.Strings(names)

			for _, name := range names {
				if limit > 0 && len(defs) >= limit {
					break
				}
				fullPath := filepath.Join(dir, name)
				def, err := ParseDefinitionFile(fullPath)
				if err != nil || def.ID == "" {
					continue
				}
				defs = append(defs, def)
			}
		}
	}

	if len(defs) == 0 {
		fallbacks := BuiltinDefinitions()
		if limit > 0 && len(fallbacks) > limit {
			fallbacks = fallbacks[:limit]
		}
		return fallbacks, nil
	}

	return defs, nil
}

// ParseDefinitionFile reads and parses a single Cardigann YAML file.
func ParseDefinitionFile(path string) (Definition, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, err
	}
	var def Definition
	if err := yaml.Unmarshal(b, &def); err != nil {
		return Definition{}, err
	}
	if def.ID == "" {
		base := filepath.Base(path)
		def.ID = strings.TrimSuffix(strings.TrimSuffix(base, ".yml"), ".yaml")
	}
	if def.Name == "" {
		def.Name = def.ID
	}
	if def.Type == "" {
		def.Type = "public"
	}
	if def.Language == "" {
		def.Language = "en-US"
	}
	if def.Encoding == "" {
		def.Encoding = "UTF-8"
	}
	if def.Links == nil {
		def.Links = []string{}
	}
	if def.Caps == nil {
		def.Caps = map[string]any{}
	}
	if def.Settings == nil {
		def.Settings = []map[string]any{}
	}
	return def, nil
}

// BuiltinDefinitions returns built-in fallback Cardigann indexer definitions.
func BuiltinDefinitions() []Definition {
	return []Definition{
		{
			ID:          "1337x",
			Name:        "1337x",
			Description: "1337x is a Public torrent site that offers verified torrent downloads",
			Language:    "en-US",
			Type:        "public",
			Encoding:    "UTF-8",
			Links:       []string{"https://1337x.to/", "https://1337x.st/"},
			Caps: map[string]any{
				"modes": map[string]any{
					"search":       []string{"q"},
					"tv-search":    []string{"q", "season", "ep"},
					"movie-search": []string{"q"},
				},
			},
			Settings: []map[string]any{
				{"name": "downloadlink", "type": "select", "label": "Download Link", "default": "magnet:"},
			},
		},
		{
			ID:          "yts",
			Name:        "YTS",
			Description: "YTS is a Public torrent site specialising in HD movies of small size",
			Language:    "en-US",
			Type:        "public",
			Encoding:    "UTF-8",
			Links:       []string{"https://yts.mx/", "https://yts.gg/"},
			Caps: map[string]any{
				"modes": map[string]any{
					"search":       []string{"q"},
					"movie-search": []string{"q", "imdbid"},
				},
			},
			Settings: []map[string]any{
				{"name": "apiurl", "type": "text", "label": "API URL", "default": "movies-api.accel.li"},
			},
		},
		{
			ID:          "nyaasi",
			Name:        "Nyaa.si",
			Description: "Nyaa is a Public torrent site focused on East Asian media including anime and manga",
			Language:    "en-US",
			Type:        "public",
			Encoding:    "UTF-8",
			Links:       []string{"https://nyaa.si/"},
			Caps: map[string]any{
				"modes": map[string]any{
					"search":    []string{"q"},
					"tv-search": []string{"q", "season", "ep"},
				},
			},
			Settings: []map[string]any{},
		},
		{
			ID:          "eztv",
			Name:        "EZTV",
			Description: "EZTV is a Public torrent site for TV shows",
			Language:    "en-US",
			Type:        "public",
			Encoding:    "UTF-8",
			Links:       []string{"https://eztvx.to/"},
			Caps: map[string]any{
				"modes": map[string]any{
					"search":    []string{"q"},
					"tv-search": []string{"q", "season", "ep", "imdbid"},
				},
			},
			Settings: []map[string]any{},
		},
		{
			ID:          "torrentleech",
			Name:        "TorrentLeech",
			Description: "TorrentLeech (TL) is a Private Torrent Tracker for 0DAY / GENERAL",
			Language:    "en-US",
			Type:        "private",
			Encoding:    "UTF-8",
			Links:       []string{"https://www.torrentleech.org/"},
			Caps: map[string]any{
				"modes": map[string]any{
					"search":       []string{"q"},
					"tv-search":    []string{"q", "season", "ep"},
					"movie-search": []string{"q", "imdbid"},
				},
			},
			Settings: []map[string]any{
				{"name": "username", "type": "text", "label": "Username"},
				{"name": "password", "type": "password", "label": "Password"},
				{"name": "freeleech", "type": "checkbox", "label": "Search freeleech only", "default": false},
			},
		},
	}
}
