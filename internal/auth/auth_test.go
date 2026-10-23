package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/replacearr/replacearr/internal/config"
	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/domain"
)

func setupTestService(t *testing.T) (*Service, *db.Store, string) {
	t.Helper()
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	cfg := &config.Config{
		ConfigDir: tmpDir,
		JWTSecret: "test-jwt-secret-key-for-unit-tests-12345",
		AdminUser: "admin",
		AdminPass: "supersecret",
	}

	svc, err := NewService(store, cfg)
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	return svc, store, tmpDir
}

func TestBootstrapAdminAndMasterAPIKey(t *testing.T) {
	svc, store, tmpDir := setupTestService(t)

	users, err := store.ListUsers()
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 bootstrap user, got %d", len(users))
	}
	if users[0].Username != "admin" || users[0].Email != "admin@replacearr.local" || users[0].RoleName != "admin" {
		t.Fatalf("unexpected bootstrap admin user: %+v", users[0])
	}

	masterKey := svc.GetMasterAPIKey()
	if len(masterKey) != 32 {
		t.Fatalf("expected 32-char hex master API key, got %q (len=%d)", masterKey, len(masterKey))
	}

	keyBytes, err := os.ReadFile(filepath.Join(tmpDir, "api_key.txt"))
	if err != nil {
		t.Fatalf("read api_key.txt: %v", err)
	}
	if string(keyBytes) != masterKey {
		t.Fatalf("expected api_key.txt %q to match masterKey %q", string(keyBytes), masterKey)
	}

	apiKey, user, err := svc.ValidateAPIKey(masterKey, "sonarr")
	if err != nil {
		t.Fatalf("validate master key for sonarr scope: %v", err)
	}
	if apiKey.Scope != "all" || user.Username != "admin" {
		t.Fatalf("unexpected validated master key/user: %+v / %+v", apiKey, user)
	}
}

func TestLoginJWTAndHandlersFlow(t *testing.T) {
	svc, _, _ := setupTestService(t)
	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)

	// 1. Invalid password should fail with 401
	badLoginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "wrongpassword",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(badLoginBody))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on bad login, got %d", rec.Code)
	}

	// 2. Valid login sets cookie and returns JWT + masterApiKey
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "supersecret",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d: %s", rec.Code, rec.Body.String())
	}

	var loginResp struct {
		Token        string      `json:"token"`
		User         domain.User `json:"user"`
		MasterAPIKey string      `json:"masterApiKey"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("unmarshal login response: %v", err)
	}
	if loginResp.Token == "" || loginResp.User.Username != "admin" || loginResp.MasterAPIKey != svc.GetMasterAPIKey() {
		t.Fatalf("unexpected login response: %+v", loginResp)
	}

	var authCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "replacearr_token" {
			authCookie = c
			break
		}
	}
	if authCookie == nil || authCookie.Value != loginResp.Token {
		t.Fatalf("expected replacearr_token cookie to be set")
	}

	// 3. Call /api/v1/auth/me using Bearer JWT
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/auth/me, got %d", rec.Code)
	}

	// 4. Call /api/v1/permissions using cookie
	req = httptest.NewRequest(http.MethodGet, "/api/v1/permissions", nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/permissions with cookie, got %d", rec.Code)
	}

	// 5. Exercise roles, apikeys, users update/delete, and logout handlers
	roleBody, _ := json.Marshal(map[string]any{
		"name":        "custom_editor",
		"description": "Custom editor role",
		"permissions": []string{domain.PermTVRead, domain.PermTVWrite},
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/roles", bytes.NewReader(roleBody))
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 on create role, got %d: %s", rec.Code, rec.Body.String())
	}

	keyBody, _ := json.Marshal(map[string]any{
		"name":        "Test Integration Key",
		"scope":       "prowlarr",
		"permissions": []string{domain.PermIndexersRead, domain.PermIndexersSearch},
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/apikeys", bytes.NewReader(keyBody))
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 on create apikey, got %d: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on logout, got %d", rec.Code)
	}
}

func TestAPIKeyScopeEnforcement(t *testing.T) {
	svc, store, _ := setupTestService(t)
	users, _ := store.ListUsers()
	adminID := users[0].ID

	sonarrKey, err := svc.GenerateAPIKey("Sonarr Only Key", adminID, "sonarr", domain.AllPermissions)
	if err != nil {
		t.Fatalf("generate sonarr key: %v", err)
	}
	radarrKey, err := svc.GenerateAPIKey("Radarr Only Key", adminID, "radarr", domain.AllPermissions)
	if err != nil {
		t.Fatalf("generate radarr key: %v", err)
	}
	allKey, err := svc.GenerateAPIKey("All Scope Key", adminID, "all", domain.AllPermissions)
	if err != nil {
		t.Fatalf("generate all key: %v", err)
	}

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		k := APIKeyFromContext(r.Context())
		writeJSON(w, http.StatusOK, map[string]any{
			"user":  u.Username,
			"scope": k.Scope,
		})
	})

	sonarrProtected := svc.Middleware("sonarr", domain.PermTVRead, okHandler)
	radarrProtected := svc.Middleware("radarr", domain.PermMoviesRead, okHandler)

	// Sonarr key -> Sonarr endpoint (200 OK via X-Api-Key)
	req := httptest.NewRequest(http.MethodGet, "/api/v3/series", nil)
	req.Header.Set("X-Api-Key", sonarrKey.RawKey)
	rec := httptest.NewRecorder()
	sonarrProtected.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected sonarr key to access sonarr endpoint, got %d", rec.Code)
	}

	// Sonarr key -> Radarr endpoint (401 Unauthorized due to scope mismatch)
	req = httptest.NewRequest(http.MethodGet, "/api/v3/movie", nil)
	req.Header.Set("X-Api-Key", sonarrKey.RawKey)
	rec = httptest.NewRecorder()
	radarrProtected.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected sonarr key to be rejected on radarr endpoint with 401, got %d", rec.Code)
	}

	// Radarr key -> Radarr endpoint via ?apikey= query param (200 OK)
	req = httptest.NewRequest(http.MethodGet, "/api/v3/movie?apikey="+radarrKey.RawKey, nil)
	rec = httptest.NewRecorder()
	radarrProtected.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected radarr key via ?apikey= to access radarr endpoint, got %d", rec.Code)
	}

	// All scope key -> both endpoints (200 OK, including via Bearer header)
	req = httptest.NewRequest(http.MethodGet, "/api/v3/series", nil)
	req.Header.Set("Authorization", "Bearer "+allKey.RawKey)
	rec = httptest.NewRecorder()
	sonarrProtected.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected all-scope key via Bearer to access sonarr endpoint, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v3/movie?apiKey="+allKey.RawKey, nil)
	rec = httptest.NewRecorder()
	radarrProtected.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected all-scope key via ?apiKey= to access radarr endpoint, got %d", rec.Code)
	}
}

func TestRBACPermissionRejection(t *testing.T) {
	svc, store, _ := setupTestService(t)
	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)

	// Create a viewer user via admin API
	viewerRole, err := store.GetRoleByName("viewer")
	if err != nil {
		t.Fatalf("get viewer role: %v", err)
	}

	createBody, _ := json.Marshal(map[string]any{
		"username": "viewer_user",
		"email":    "viewer@replacearr.local",
		"password": "viewerpass123",
		"roleId":   viewerRole.ID,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(createBody))
	req.Header.Set("X-Api-Key", svc.GetMasterAPIKey())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating viewer user, got %d: %s", rec.Code, rec.Body.String())
	}

	// Authenticate as viewer_user
	viewerUser, viewerToken, err := svc.Authenticate("viewer_user", "viewerpass123")
	if err != nil {
		t.Fatalf("authenticate viewer_user: %v", err)
	}
	if svc.HasPermission(viewerUser, nil, domain.PermUsersManage) {
		t.Fatalf("viewer user should not have users:manage permission")
	}
	if !svc.HasPermission(viewerUser, nil, domain.PermTVRead) {
		t.Fatalf("viewer user should have tv:read permission")
	}

	// Viewer attempting POST /api/v1/users (requires users:manage) -> 403 Forbidden
	attemptBody, _ := json.Marshal(map[string]any{
		"username": "hacker",
		"password": "password123",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(attemptBody))
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when viewer tries users:manage, got %d: %s", rec.Code, rec.Body.String())
	}

	var errResp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &errResp)
	expectedErr := "forbidden: missing permission " + domain.PermUsersManage
	if errResp["error"] != expectedErr {
		t.Fatalf("expected error %q, got %q", expectedErr, errResp["error"])
	}

	// Also verify API key with restricted permissions on an admin user
	restrictedKey, err := svc.GenerateAPIKey("TV Read Only Key", 1, "all", []string{domain.PermTVRead})
	if err != nil {
		t.Fatalf("generate restricted key: %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("X-Api-Key", restrictedKey.RawKey)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when restricted API key lacks users:read, got %d", rec.Code)
	}
}
