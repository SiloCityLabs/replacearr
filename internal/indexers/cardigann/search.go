package cardigann

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cdefinition "github.com/Kcchouette/cardigann-go/definition"
	cengine "github.com/Kcchouette/cardigann-go/engine"
	chttp "github.com/Kcchouette/cardigann-go/httpclient"
	ctemplate "github.com/Kcchouette/cardigann-go/template"

	"github.com/replacearr/replacearr/internal/domain"
)

// SearchRequest is a Cardigann keyword search against one enabled indexer.
type SearchRequest struct {
	DefinitionsDir string
	DefinitionFile string
	BaseURL        string
	IndexerID      int64
	IndexerName    string
	Fields         []domain.ConfigField
	Query          string
	Season         int
	Episode        int
	SearchMode     string // search|tvsearch|movie|music|book (Cardigann Query.Type)
	Categories     []int
	HTTPClient     *http.Client // optional; when nil a default client is used
	ProxyURL       string
}

// ResolveDefinitionPath finds definitionFile.yml/.yaml under dir.
func ResolveDefinitionPath(dir, definitionFile string) (string, error) {
	id := strings.TrimSpace(definitionFile)
	if id == "" {
		return "", fmt.Errorf("empty definition file")
	}
	id = strings.TrimSuffix(strings.TrimSuffix(id, ".yml"), ".yaml")
	candidates := []string{
		filepath.Join(dir, id+".yml"),
		filepath.Join(dir, id+".yaml"),
		filepath.Join(dir, definitionFile),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("definition %q not found in %s", definitionFile, dir)
}

// Search executes a Cardigann definition search and maps results to domain.ReleaseResult.
func Search(ctx context.Context, req SearchRequest) ([]domain.ReleaseResult, error) {
	path, err := ResolveDefinitionPath(req.DefinitionsDir, req.DefinitionFile)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	def, err := cdefinition.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse definition %s: %w", path, err)
	}

	baseURL := strings.TrimSpace(req.BaseURL)
	if baseURL == "" && len(def.Links) > 0 {
		baseURL = def.Links[0]
	}
	if baseURL == "" {
		return nil, fmt.Errorf("no base URL for indexer %s", req.IndexerName)
	}
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	cfg := configFromDefinition(def, req.Fields)
	// Prefer unsorted keyword paths for sites that break sort under load (1337x).
	if _, ok := cfg["disablesort"]; !ok {
		cfg["disablesort"] = "true"
	}

	delay := time.Duration(0)
	if def.RequestDelay > 0 {
		delay = time.Duration(def.RequestDelay * float64(time.Second))
	}
	proxyForClient := req.ProxyURL
	if req.HTTPClient != nil && req.HTTPClient.Transport != nil {
		// Caller already built a (possibly proxied) transport; do not also
		// wrap SOCKS inside IndexerClient or requests double-proxy.
		proxyForClient = ""
	}
	client, err := chttp.New(chttp.Options{
		IndexerID:    def.ID,
		ProxyURL:     proxyForClient,
		RequestDelay: delay,
		Timeout:      45 * time.Second,
		MaxRetries:   2,
	})
	if err != nil {
		return nil, err
	}
	if req.HTTPClient != nil && req.HTTPClient.Transport != nil {
		client.HTTPClient().Transport = req.HTTPClient.Transport
		if req.HTTPClient.Timeout > 0 {
			client.HTTPClient().Timeout = req.HTTPClient.Timeout
		}
	}

	mode := strings.ToLower(strings.TrimSpace(req.SearchMode))
	switch mode {
	case "tv", "tvsearch":
		mode = "tvsearch"
	case "movie", "movies":
		mode = "movie"
	case "music":
		mode = "music"
	case "book", "ebook", "audiobook":
		mode = "book"
	case "":
		mode = "search"
	}
	q := cengine.Query{
		Keywords:   strings.TrimSpace(req.Query),
		Type:       mode,
		Categories: req.Categories,
	}
	if req.Season > 0 {
		q.Season = strconv.Itoa(req.Season)
	}
	if req.Episode > 0 {
		q.Ep = strconv.Itoa(req.Episode)
	}

	// cardigann-go does not render path methods and mis-parses Prowlarr regex
	// escapes in Go templates; normalize before the engine runs.
	if err := applyDefinitionCompat(&def, ctemplate.Context{
		Config:   cfg,
		Keywords: q.Keywords,
		Query:    map[string]string{"Type": q.Type, "Season": q.Season, "Ep": q.Ep},
	}); err != nil {
		return nil, err
	}

	provider := cengine.NewAuthenticatedProvider(def, baseURL, cfg, client)
	releases, err := provider.Search(ctx, q)
	if err != nil {
		return nil, err
	}

	out := make([]domain.ReleaseResult, 0, len(releases))
	for _, rel := range releases {
		pub := rel.PublishDate
		if pub.IsZero() {
			pub = time.Now().UTC()
		}
		ageDays := int(time.Since(pub).Hours() / 24)
		if ageDays < 0 {
			ageDays = 0
		}
		// promoteLeechersToPeers maps YAML `leechers` onto cardigann-go's
		// `peers` field (the engine drops `leechers`). Treat that value as
		// leechers-only and build Torznab peers = seeders + leechers.
		seeders := rel.Seeders
		leechers := rel.Peers
		if seeders < 0 || seeders >= 5_000_000 {
			seeders = 0
		}
		if leechers < 0 || leechers >= 5_000_000 {
			leechers = 0
		}
		peers := seeders + leechers
		protocol := string(rel.Protocol)
		if protocol == "" {
			protocol = "torrent"
		}
		cats := rel.Categories
		if cats == nil {
			cats = []int{}
		}
		downloadURL := resolveDownloadURL(rel.DownloadLink, rel.InfoHash, rel.Title, baseURL)
		infoURL := rel.Details
		if infoURL != "" && strings.HasPrefix(infoURL, "/") {
			infoURL = strings.TrimRight(baseURL, "/") + infoURL
		}
		guid := downloadURL
		if guid == "" {
			guid = infoURL
		}
		if guid == "" {
			guid = fmt.Sprintf("%s-%s", def.ID, rel.Title)
		}
		quality := domain.InferReleaseQuality(rel.Title, "")
		out = append(out, domain.ReleaseResult{
			Guid:        guid,
			Title:       rel.Title,
			IndexerID:   req.IndexerID,
			Indexer:     req.IndexerName,
			Protocol:    protocol,
			Size:        rel.Size,
			Age:         ageDays,
			PublishDate: pub.UTC(),
			DownloadURL: downloadURL,
			InfoURL:     infoURL,
			Seeders:     seeders,
			Leechers:    leechers,
			Peers:       peers,
			Categories:  cats,
			Quality:     quality,
		})
	}
	return out, nil
}

// resolveDownloadURL absolutizes a torrent/NZB link, or builds a magnet URI
// from an infohash when the definition only exposes infohash (e.g. Bangumi Moe).
func resolveDownloadURL(downloadLink, infoHash, title, baseURL string) string {
	downloadURL := strings.TrimSpace(downloadLink)
	if downloadURL != "" {
		if strings.HasPrefix(downloadURL, "/") {
			return strings.TrimRight(baseURL, "/") + downloadURL
		}
		return downloadURL
	}
	hash := strings.TrimSpace(infoHash)
	if hash == "" {
		return ""
	}
	// Strip common wrappers some JSON APIs return.
	hash = strings.TrimPrefix(strings.ToLower(hash), "urn:btih:")
	magnet := "magnet:?xt=urn:btih:" + hash
	if title = strings.TrimSpace(title); title != "" {
		magnet += "&dn=" + url.QueryEscape(title)
	}
	return magnet
}

func configFromDefinition(def cdefinition.IndexerDefinition, fields []domain.ConfigField) map[string]string {
	cfg := map[string]string{}
	for _, s := range def.Settings {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			continue
		}
		if s.Default != "" {
			cfg[name] = s.Default
		}
	}
	for _, f := range fields {
		name := strings.TrimSpace(f.Name)
		if name == "" || f.Value == nil {
			continue
		}
		switch v := f.Value.(type) {
		case bool:
			if v {
				cfg[name] = "true"
			} else {
				cfg[name] = "false"
			}
		default:
			cfg[name] = fmt.Sprint(v)
		}
	}
	return cfg
}
