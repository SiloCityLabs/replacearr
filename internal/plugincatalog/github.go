package plugincatalog

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	ownerRepoRe = regexp.MustCompile(`(?i)^([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+?)(?:\.git)?$`)
	tagPathRe   = regexp.MustCompile(`(?i)/releases/tag/(v?[0-9][^/]*)`)
	tagDlRe     = regexp.MustCompile(`(?i)/releases/download/(v?[0-9][^/]*)/`)
)

// ParseGitHubPluginURL accepts owner/repo, github.com URLs, and optional release tags.
func ParseGitHubPluginURL(raw string) (owner, repo, tag string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", "", fmt.Errorf("empty github url")
	}
	if m := ownerRepoRe.FindStringSubmatch(raw); m != nil && !strings.Contains(raw, "://") && !strings.Contains(raw, "github.com") {
		return m[1], strings.TrimSuffix(m[2], ".git"), "", nil
	}

	if !strings.Contains(raw, "://") {
		raw = "https://" + strings.TrimPrefix(raw, "//")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid github url: %w", err)
	}
	host := strings.ToLower(u.Host)
	if host != "github.com" && host != "www.github.com" {
		return "", "", "", fmt.Errorf("only github.com plugin urls are supported (got %q)", u.Host)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("expected github.com/owner/repo in %q", raw)
	}
	owner = parts[0]
	repo = strings.TrimSuffix(parts[1], ".git")

	if q := strings.TrimSpace(u.Query().Get("tag")); q != "" {
		tag = normalizeTag(q)
	}
	if tag == "" {
		if m := tagPathRe.FindStringSubmatch(u.Path); m != nil {
			tag = normalizeTag(m[1])
		}
	}
	if tag == "" {
		if m := tagDlRe.FindStringSubmatch(u.Path); m != nil {
			tag = normalizeTag(m[1])
		}
	}
	return owner, repo, tag, nil
}

func normalizeTag(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ""
	}
	if !strings.HasPrefix(tag, "v") && regexp.MustCompile(`^[0-9]`).MatchString(tag) {
		return "v" + tag
	}
	return tag
}
