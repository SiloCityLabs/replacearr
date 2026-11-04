package changelog

import (
	"embed"
	"regexp"
	"strings"
)

//go:embed CHANGELOG.md
var raw embed.FS

type Section struct {
	Version string   `json:"version"`
	Date    string   `json:"date,omitempty"`
	Notes   []string `json:"notes"`
}

var headerRe = regexp.MustCompile(`(?m)^## \[([^\]]+)\](?: - (.+))?$`)

func Sections() ([]Section, error) {
	b, err := raw.ReadFile("CHANGELOG.md")
	if err != nil {
		return nil, err
	}
	text := string(b)
	matches := headerRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return []Section{}, nil
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
	return out, nil
}
