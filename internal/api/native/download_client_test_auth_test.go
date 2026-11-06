package native

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDownloadClientAuthTestRejectsBadCredentials(t *testing.T) {
	_, _, mux := newTestStack(t)
	token := login(t, mux)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Unauthorized"))
			return
		}
		http.NotFound(w, r)
	}))
	defer upstream.Close()

	body, _ := json.Marshal(map[string]any{
		"implementation": "qBittorrent",
		"baseUrl":        upstream.URL,
		"username":       "admin",
		"password":       "wrong",
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, authed(http.MethodPost, "/api/v1/downloadclients/test", token, body))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		OK      bool   `json:"ok"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected auth failure, got ok: %#v", resp)
	}
	if !strings.Contains(strings.ToLower(resp.Message), "login failed") {
		t.Fatalf("message=%q", resp.Message)
	}
}

func TestDownloadClientAuthTestSucceeds(t *testing.T) {
	_, _, mux := newTestStack(t)
	token := login(t, mux)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			_ = r.ParseForm()
			if r.Form.Get("username") != "admin" || r.Form.Get("password") != "secret" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("Unauthorized"))
				return
			}
			_, _ = w.Write([]byte("Ok."))
			return
		}
		http.NotFound(w, r)
	}))
	defer upstream.Close()

	body, _ := json.Marshal(map[string]any{
		"implementation": "qBittorrent",
		"baseUrl":        upstream.URL,
		"username":       "admin",
		"password":       "secret",
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, authed(http.MethodPost, "/api/v1/downloadclients/test", token, body))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		OK      bool   `json:"ok"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok, got %#v", resp)
	}
	if !strings.Contains(resp.Message, "authenticated with") {
		t.Fatalf("message=%q", resp.Message)
	}
}
