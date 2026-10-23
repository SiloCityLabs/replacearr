package native

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/replacearr/replacearr/internal/auth"
	"github.com/replacearr/replacearr/internal/config"
	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/pluginhost"
)

func newTestStack(t *testing.T) (*Server, *auth.Service, *http.ServeMux) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("REPLACEARR_CONFIG_DIR", dir)
	t.Setenv("REPLACEARR_DB_PATH", filepath.Join(dir, "replacearr.db"))
	t.Setenv("REPLACEARR_PLUGIN_DIR", filepath.Join(dir, "plugins"))
	t.Setenv("REPLACEARR_DEFINITIONS_DIR", filepath.Join(dir, "definitions"))
	t.Setenv("REPLACEARR_ADMIN_USER", "admin")
	t.Setenv("REPLACEARR_ADMIN_PASS", "replacearr")
	t.Setenv("REPLACEARR_JWT_SECRET", "test-jwt-secret-at-least-32-bytes-long")
	t.Setenv("REPLACEARR_FLARESOLVERR_URL", "http://127.0.0.1:1/")

	cfg := config.Load()
	store, err := db.Open(cfg.DatabasePath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	authSvc, err := auth.NewService(store, cfg)
	if err != nil {
		t.Fatalf("auth: %v", err)
	}
	mgr := pluginhost.NewManager(store, cfg.PluginDir, cfg.DefinitionsDir)
	srv := NewServer(store, cfg, authSvc, mgr, "test")

	mux := http.NewServeMux()
	authSvc.RegisterRoutes(mux)
	srv.RegisterRoutes(mux)
	return srv, authSvc, mux
}

func login(t *testing.T, mux *http.ServeMux) string {
	t.Helper()
	body := bytes.NewBufferString(`{"username":"admin","password":"replacearr"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil || resp.Token == "" {
		t.Fatalf("login token: %v body=%s", err, rr.Body.String())
	}
	return resp.Token
}

func authed(method, path, token string, body []byte) *http.Request {
	var r *http.Request
	if body == nil {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	r.Header.Set("Authorization", "Bearer "+token)
	return r
}

func TestHealthIsPublicAndOverviewRequiresAuth(t *testing.T) {
	_, _, mux := newTestStack(t)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/system/health", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("health %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("overview without auth %d", rr.Code)
	}
}

func TestSeriesAndIndexerProxyFlow(t *testing.T) {
	_, _, mux := newTestStack(t)
	token := login(t, mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, authed(http.MethodGet, "/api/v1/overview", token, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("overview %d %s", rr.Code, rr.Body.String())
	}
	var overview struct {
		Counts map[string]int `json:"counts"`
		System struct {
			FlareSolverrURL string `json:"flareSolverrUrl"`
		} `json:"system"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &overview); err != nil {
		t.Fatal(err)
	}
	if overview.Counts["indexerProxies"] < 1 || overview.System.FlareSolverrURL == "" {
		t.Fatalf("overview missing flaresolverr seed: %+v", overview)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, authed(http.MethodPost, "/api/v1/series", token, []byte(`{"title":"Severance","year":2022,"tvdbId":371980,"monitored":true}`)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create series %d %s", rr.Code, rr.Body.String())
	}
	var created struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	if created.ID == 0 {
		t.Fatal("missing series id")
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, authed(http.MethodGet, "/api/v1/series", token, nil))
	if rr.Code != http.StatusOK || !bytes.Contains(rr.Body.Bytes(), []byte("Severance")) {
		t.Fatalf("list series %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, authed(http.MethodDelete, "/api/v1/series/1", token, nil))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete series %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, authed(http.MethodPost, "/api/v1/indexerproxies/test", token, []byte(`{"host":"http://127.0.0.1:1"}`)))
	if rr.Code != http.StatusOK || !bytes.Contains(rr.Body.Bytes(), []byte(`"healthy":false`)) {
		t.Fatalf("proxy test %d %s", rr.Code, rr.Body.String())
	}
}
