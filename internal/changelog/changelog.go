package changelog

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DefaultURL is the canonical CHANGELOG.md on GitHub (main branch).
const DefaultURL = "https://raw.githubusercontent.com/replacearr/replacearr/main/CHANGELOG.md"

const cacheTTL = time.Hour

type Section struct {
	Version string   `json:"version"`
	Date    string   `json:"date,omitempty"`
	Notes   []string `json:"notes"`
}

var (
	headerRe = regexp.MustCompile(`(?m)^## \[([^\]]+)\](?: - (.+))?$`)

	httpClient = &http.Client{Timeout: 15 * time.Second}
	sourceURL  = DefaultURL

	cacheMu  sync.Mutex
	cached   []Section
	cachedAt time.Time
)

// SetSourceURL overrides the GitHub raw URL (tests).
func SetSourceURL(u string) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	sourceURL = u
	cached = nil
	cachedAt = time.Time{}
}

// Sections fetches and parses CHANGELOG.md from GitHub, with a short in-memory cache.
func Sections(ctx context.Context) ([]Section, error) {
	cacheMu.Lock()
	if len(cached) > 0 && time.Since(cachedAt) < cacheTTL {
		out := append([]Section(nil), cached...)
		cacheMu.Unlock()
		return out, nil
	}
	url := sourceURL
	cacheMu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Replacearr (+https://github.com/replacearr/replacearr)")
	req.Header.Set("Accept", "text/plain")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("changelog fetch %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	sections := parse(string(b))

	cacheMu.Lock()
	cached = sections
	cachedAt = time.Now()
	cacheMu.Unlock()
	return append([]Section(nil), sections...), nil
}

func parse(text string) []Section {
	matches := headerRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return []Section{}
	}
	out := make([]Section, 0, len(matches))
	for i, m := range matches {
		version := text[m[2]:m[3]]
		date := ""
		if m[4] >= 0 {
			date = strings.TrimSpace(text[m[4]:m[5]])
		}
		start := m[1]
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		body := text[start:end]
		notes := []string{}
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "- ") {
				notes = append(notes, strings.TrimSpace(strings.TrimPrefix(line, "- ")))
			}
		}
		out = append(out, Section{Version: version, Date: date, Notes: notes})
	}
	return out
}
