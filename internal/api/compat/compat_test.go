package compat_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/replacearr/replacearr/internal/api/compat/prowlarr"
	"github.com/replacearr/replacearr/internal/api/compat/radarr"
	"github.com/replacearr/replacearr/internal/api/compat/sonarr"
	"github.com/replacearr/replacearr/internal/config"
	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/domain"
)

func setupTestEnv(t *testing.T) (*db.Store, *config.Config, sonarr.SearchFunc) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	cfg := &config.Config{
		Host:         "127.0.0.1",
		Port:         8989,
		ConfigDir:    tmpDir,
		DatabasePath: dbPath,
		Environment:  "test",
	}

	searchFn := func(query string, mediaType string, tvdbID int, tmdbID int) ([]domain.ReleaseResult, error) {
		q := query
		if q == "" {
			q = "Replacearr"
		}
		return []domain.ReleaseResult{
			{
				Guid:        "guid-1-" + strings.ToLower(strings.ReplaceAll(q, " ", "-")),
				Title:       q + ".S01E01.2160p.WEB-DL.DDP5.1.Atmos.DV.H.265-GRP",
				IndexerID:   1,
				Indexer:     "TestIndexer",
				Protocol:    "torrent",
				Size:        4294967296,
				Age:         1,
				PublishDate: time.Now().UTC(),
				DownloadURL: "http://localhost:8989/prowlarr/1/download?guid=1",
				InfoURL:     "http://localhost:8989/prowlarr/1/info?guid=1",
				Seeders:     150,
				Leechers:    10,
				Peers:       160,
				Categories:  []int{5000, 5045},
				Quality:     "WEBDL-2160p",
				TvdbID:      tvdbID,
				TmdbID:      tmdbID,
			},
		}, nil
	}

	return store, cfg, searchFn
}

func TestSonarrV3Compat(t *testing.T) {
	store, cfg, searchFn := setupTestEnv(t)
	ts := httptest.NewServer(sonarr.NewHandler(store, cfg, searchFn))
	defer ts.Close()

	// 1. GET /api/v3/system/status (and /sonarr/api/v3/system/status)
	for _, path := range []string{"/api/v3/system/status", "/sonarr/api/v3/system/status", "/api"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", path, resp.StatusCode)
		}
		var status map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			t.Fatalf("decode status: %v", err)
		}
		_ = resp.Body.Close()
		if status["appName"] != "Sonarr" {
			t.Errorf("appName = %v, want Sonarr", status["appName"])
		}
		if status["urlBase"] != "/sonarr" {
			t.Errorf("urlBase = %v, want /sonarr", status["urlBase"])
		}
	}

	// 2. GET /api/v3/series/lookup?term=Severance
	resp, err := http.Get(ts.URL + "/api/v3/series/lookup?term=Severance")
	if err != nil {
		t.Fatalf("lookup series: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("lookup status = %d", resp.StatusCode)
	}
	var lookupResults []domain.Series
	if err := json.NewDecoder(resp.Body).Decode(&lookupResults); err != nil {
		t.Fatalf("decode lookup: %v", err)
	}
	_ = resp.Body.Close()
	if len(lookupResults) == 0 || lookupResults[0].Title != "Severance" {
		t.Fatalf("expected Severance first in lookup results, got %+v", lookupResults)
	}

	// 3. POST /api/v3/series & GET /api/v3/series
	createBody, _ := json.Marshal(map[string]any{
		"title":            "Severance",
		"tvdbId":           371980,
		"year":             2022,
		"qualityProfileId": 1,
		"rootFolderPath":   "/data/tv",
		"monitored":        true,
	})
	resp, err = http.Post(ts.URL+"/api/v3/series", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("POST /series: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /series status = %d, want 201", resp.StatusCode)
	}
	var createdSeries domain.Series
	_ = json.NewDecoder(resp.Body).Decode(&createdSeries)
	_ = resp.Body.Close()
	if createdSeries.ID == 0 || createdSeries.Title != "Severance" {
		t.Fatalf("unexpected created series: %+v", createdSeries)
	}

	resp, err = http.Get(ts.URL + "/api/v3/series")
	if err != nil {
		t.Fatalf("GET /series: %v", err)
	}
	var seriesList []domain.Series
	_ = json.NewDecoder(resp.Body).Decode(&seriesList)
	_ = resp.Body.Close()
	if len(seriesList) != 1 {
		t.Fatalf("len(seriesList) = %d, want 1", len(seriesList))
	}

	// 4. POST /api/v3/command
	cmdBody, _ := json.Marshal(map[string]any{
		"name":     "SeriesSearch",
		"seriesId": createdSeries.ID,
	})
	resp, err = http.Post(ts.URL+"/api/v3/command", "application/json", bytes.NewReader(cmdBody))
	if err != nil {
		t.Fatalf("POST /command: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /command status = %d, want 201", resp.StatusCode)
	}
	var cmdRec domain.CommandRecord
	_ = json.NewDecoder(resp.Body).Decode(&cmdRec)
	_ = resp.Body.Close()
	if cmdRec.Status != "completed" || cmdRec.Name != "SeriesSearch" {
		t.Fatalf("unexpected command record: %+v", cmdRec)
	}

	// 5. POST /api/v3/release & GET /api/v3/queue
	grabBody, _ := json.Marshal(map[string]any{
		"guid":     "guid-1-severance",
		"title":    "Severance.S01E01.2160p.WEB-DL",
		"seriesId": createdSeries.ID,
		"size":     4294967296,
		"protocol": "torrent",
	})
	resp, err = http.Post(ts.URL+"/api/v3/release", "application/json", bytes.NewReader(grabBody))
	if err != nil {
		t.Fatalf("POST /release: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /release status = %d, want 201", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v3/queue")
	if err != nil {
		t.Fatalf("GET /queue: %v", err)
	}
	var queueResp struct {
		Page         int                `json:"page"`
		TotalRecords int                `json:"totalRecords"`
		Records      []domain.QueueItem `json:"records"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&queueResp); err != nil {
		t.Fatalf("decode queue: %v", err)
	}
	_ = resp.Body.Close()
	if queueResp.TotalRecords != 1 || len(queueResp.Records) != 1 {
		t.Fatalf("expected 1 TV queue item, got %+v", queueResp)
	}
}

func TestRadarrV3Compat(t *testing.T) {
	store, cfg, searchFn := setupTestEnv(t)
	ts := httptest.NewServer(radarr.NewHandler(store, cfg, searchFn))
	defer ts.Close()

	// 1. GET /api/v3/system/status
	resp, err := http.Get(ts.URL + "/api/v3/system/status")
	if err != nil {
		t.Fatalf("GET /system/status: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var status map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&status)
	_ = resp.Body.Close()
	if status["appName"] != "Radarr" || status["urlBase"] != "/radarr" {
		t.Fatalf("unexpected radarr status: %+v", status)
	}

	// 2. GET /api/v3/movie/lookup/tmdb?tmdbId=693134
	resp, err = http.Get(ts.URL + "/api/v3/movie/lookup/tmdb?tmdbId=693134")
	if err != nil {
		t.Fatalf("lookup tmdb: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("lookup tmdb status = %d", resp.StatusCode)
	}
	var tmdbMovie domain.Movie
	if err := json.NewDecoder(resp.Body).Decode(&tmdbMovie); err != nil {
		t.Fatalf("decode tmdb lookup: %v", err)
	}
	_ = resp.Body.Close()
	if tmdbMovie.Title != "Dune: Part Two" || tmdbMovie.TmdbID != 693134 {
		t.Fatalf("unexpected tmdb lookup result: %+v", tmdbMovie)
	}

	// 3. POST /api/v3/movie & GET /api/v3/movie?tmdbId=693134
	createBody, _ := json.Marshal(map[string]any{
		"title":            "Dune: Part Two",
		"tmdbId":           693134,
		"year":             2024,
		"qualityProfileId": 1,
		"rootFolderPath":   "/data/movies",
		"monitored":        true,
	})
	resp, err = http.Post(ts.URL+"/api/v3/movie", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("POST /movie: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /movie status = %d, want 201", resp.StatusCode)
	}
	var createdMovie domain.Movie
	_ = json.NewDecoder(resp.Body).Decode(&createdMovie)
	_ = resp.Body.Close()

	resp, err = http.Get(ts.URL + "/api/v3/movie?tmdbId=693134")
	if err != nil {
		t.Fatalf("GET /movie?tmdbId=693134: %v", err)
	}
	var movies []domain.Movie
	_ = json.NewDecoder(resp.Body).Decode(&movies)
	_ = resp.Body.Close()
	if len(movies) != 1 || movies[0].TmdbID != 693134 {
		t.Fatalf("expected 1 movie matching tmdbId=693134, got %+v", movies)
	}

	// 4. POST /api/v3/command
	cmdBody, _ := json.Marshal(map[string]any{
		"name":     "MoviesSearch",
		"movieIds": []int64{createdMovie.ID},
	})
	resp, err = http.Post(ts.URL+"/api/v3/command", "application/json", bytes.NewReader(cmdBody))
	if err != nil {
		t.Fatalf("POST /command: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /command status = %d, want 201", resp.StatusCode)
	}
	var cmdRec domain.CommandRecord
	_ = json.NewDecoder(resp.Body).Decode(&cmdRec)
	_ = resp.Body.Close()
	if cmdRec.Status != "completed" || cmdRec.Name != "MoviesSearch" {
		t.Fatalf("unexpected command record: %+v", cmdRec)
	}
}

func TestProwlarrV1AndTorznabCompat(t *testing.T) {
	store, cfg, searchFn := setupTestEnv(t)
	ts := httptest.NewServer(prowlarr.NewHandler(store, cfg, searchFn))
	defer ts.Close()

	// 1. GET /api/v1/system/status
	resp, err := http.Get(ts.URL + "/api/v1/system/status")
	if err != nil {
		t.Fatalf("GET /system/status: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var status map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&status)
	_ = resp.Body.Close()
	if status["appName"] != "Prowlarr" || status["urlBase"] != "/prowlarr" {
		t.Fatalf("unexpected prowlarr status: %+v", status)
	}

	// 2. POST /api/v1/indexer & GET /api/v1/indexer
	idxBody, _ := json.Marshal(map[string]any{
		"name":           "1337x",
		"implementation": "Cardigann",
		"protocol":       "torrent",
		"enable":         true,
		"priority":       15,
	})
	resp, err = http.Post(ts.URL+"/api/v1/indexer", "application/json", bytes.NewReader(idxBody))
	if err != nil {
		t.Fatalf("POST /indexer: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /indexer status = %d, want 201", resp.StatusCode)
	}
	var createdIdx domain.Indexer
	_ = json.NewDecoder(resp.Body).Decode(&createdIdx)
	_ = resp.Body.Close()
	if createdIdx.ID == 0 || createdIdx.Name != "1337x" {
		t.Fatalf("unexpected indexer: %+v", createdIdx)
	}

	resp, err = http.Get(ts.URL + "/api/v1/indexer")
	if err != nil {
		t.Fatalf("GET /indexer: %v", err)
	}
	var idxList []domain.Indexer
	_ = json.NewDecoder(resp.Body).Decode(&idxList)
	_ = resp.Body.Close()
	if len(idxList) != 1 {
		t.Fatalf("len(idxList) = %d, want 1", len(idxList))
	}

	// 3. GET /api/v1/search?query=Severance&type=tvsearch
	resp, err = http.Get(ts.URL + "/api/v1/search?query=Severance&type=tvsearch")
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /search status = %d", resp.StatusCode)
	}
	var searchResults []domain.ReleaseResult
	_ = json.NewDecoder(resp.Body).Decode(&searchResults)
	_ = resp.Body.Close()
	if len(searchResults) == 0 || !strings.Contains(searchResults[0].Title, "Severance") {
		t.Fatalf("expected Severance release results, got %+v", searchResults)
	}

	// 4. Torznab XML /{id}/api?t=caps
	resp, err = http.Get(ts.URL + "/1/api?t=caps")
	if err != nil {
		t.Fatalf("GET /1/api?t=caps: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("caps status = %d", resp.StatusCode)
	}
	capsBytes, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	capsStr := string(capsBytes)
	if !strings.Contains(capsStr, "<caps>") || !strings.Contains(capsStr, "<tv-search") {
		t.Fatalf("unexpected caps XML: %s", capsStr)
	}

	// 5. Torznab XML /{id}/api?t=tvsearch&q=Severance
	resp, err = http.Get(ts.URL + "/1/api?t=tvsearch&q=Severance")
	if err != nil {
		t.Fatalf("GET /1/api?t=tvsearch&q=Severance: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tvsearch status = %d", resp.StatusCode)
	}
	feedBytes, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	feedStr := string(feedBytes)
	if !strings.Contains(feedStr, `<rss version="2.0" xmlns:torznab="http://torznab.com/schemas/2015/feed">`) ||
		!strings.Contains(feedStr, "Severance.S01E01") ||
		!strings.Contains(feedStr, "<torznab:attr") {
		t.Fatalf("unexpected torznab feed XML: %s", feedStr)
	}

	// 6. Prowlarr v1 IndexerProxy (FlareSolverr) endpoints
	resp, err = http.Get(ts.URL + "/api/v1/indexerproxy/schema")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/indexerproxy/schema failed: status=%v err=%v", resp, err)
	}
	_ = resp.Body.Close()

	proxyBody, _ := json.Marshal(map[string]any{
		"name":               "FlareSolverr",
		"implementation":     "FlareSolverr",
		"implementationName": "FlareSolverr",
		"configContract":     "FlareSolverrSettings",
		"fields": []map[string]any{
			{"name": "host", "value": "http://flaresolverr:8191/"},
			{"name": "requestTimeout", "value": 90},
		},
	})
	resp, err = http.Post(ts.URL+"/api/v1/indexerproxy", "application/json", bytes.NewReader(proxyBody))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/v1/indexerproxy failed: status=%v err=%v", resp, err)
	}
	var createdProxy domain.IndexerProxy
	_ = json.NewDecoder(resp.Body).Decode(&createdProxy)
	_ = resp.Body.Close()
	if createdProxy.Host != "http://flaresolverr:8191/" || createdProxy.RequestTimeout != 90 {
		t.Fatalf("unexpected created FlareSolverr proxy: %+v", createdProxy)
	}
}
