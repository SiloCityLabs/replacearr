package flaresolverr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCloudflareDetectionAndSolveFlow(t *testing.T) {
	// 1. Verify challenge detection matching Prowlarr & FlareSolverr rules
	hdr := http.Header{}
	hdr.Set("Server", "cloudflare")

	if !IsCloudflareChallenge(http.StatusServiceUnavailable, hdr, "<html><head><title>Just a moment...</title></head><body><div id=\"cf-challenge-running\"></div></body></html>") {
		t.Fatalf("expected 503 Just a moment... to be detected as Cloudflare challenge")
	}
	if !IsCloudflareChallenge(http.StatusForbidden, hdr, "<html><head><title>Attention Required! | Cloudflare</title></head></html>") {
		t.Fatalf("expected 403 Attention Required to be detected as Cloudflare challenge")
	}
	if IsCloudflareChallenge(http.StatusOK, hdr, "<html><head><title>Normal Tracker</title></head></html>") {
		t.Fatalf("did not expect 200 OK page to be flagged as challenge")
	}

	// 2. Mock a FlareSolverr server implementing GET /, POST /v1 (request.get, sessions.create, sessions.list, sessions.destroy)
	activeSessions := []string{"default-session-1"}
	mockFS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && r.URL.Path == "/" {
			_ = json.NewEncoder(w).Encode(IndexResponse{
				Msg:       "FlareSolverr is ready!",
				Version:   "3.3.21",
				UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/124.0.0.0 Safari/537.36",
			})
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/v1" {
			var req V1Request
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			switch req.Cmd {
			case CmdSessionsList:
				_ = json.NewEncoder(w).Encode(V1Response{
					Status:   StatusOK,
					Version:  "3.3.21",
					Sessions: activeSessions,
				})
			case CmdSessionsCreate:
				sid := req.Session
				if sid == "" {
					sid = "new-session-uuid"
				}
				activeSessions = append(activeSessions, sid)
				_ = json.NewEncoder(w).Encode(V1Response{
					Status:  StatusOK,
					Version: "3.3.21",
					Session: sid,
				})
			case CmdSessionsDestroy:
				_ = json.NewEncoder(w).Encode(V1Response{
					Status:  StatusOK,
					Version: "3.3.21",
				})
			case CmdRequestGet, CmdRequestPost:
				_ = json.NewEncoder(w).Encode(V1Response{
					Status:         StatusOK,
					Message:        "Challenge solved!",
					Version:        "3.3.21",
					StartTimestamp: 1700000000000,
					EndTimestamp:   1700000001500,
					Solution: &Solution{
						URL:       req.URL,
						Status:    200,
						Response:  "<html><body>Solved tracker content</body></html>",
						UserAgent: "Mozilla/5.0 (X11; Linux x86_64) Chrome/124.0.0.0",
						Cookies: []Cookie{
							{
								Name:     "cf_clearance",
								Value:    "solved_clearance_token_abc123",
								Domain:   ".1337x.to",
								Path:     "/",
								HTTPOnly: true,
								Secure:   true,
							},
						},
					},
				})
			default:
				_ = json.NewEncoder(w).Encode(V1Response{
					Status:  StatusError,
					Message: "unknown cmd: " + req.Cmd,
				})
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer mockFS.Close()

	client := NewClient()
	ctx := context.Background()

	// 3. Test connection health check
	health := client.TestConnection(ctx, mockFS.URL)
	if !health.Healthy || health.Version != "3.3.21" {
		t.Fatalf("expected healthy FlareSolverr v3.3.21, got %+v", health)
	}
	if len(health.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %v", health.Sessions)
	}

	// 4. Test SolveURL & automatic clearance caching
	solResp, err := client.SolveURL(ctx, mockFS.URL, "https://1337x.to/search/Severance/1/", http.MethodGet, "", 60, 15, true)
	if err != nil {
		t.Fatalf("SolveURL error: %v", err)
	}
	if solResp.Solution == nil || len(solResp.Solution.Cookies) == 0 {
		t.Fatalf("expected cookies in solution, got %+v", solResp)
	}

	// 5. Verify clearance was cached for 1337x.to and InjectClearance applies UA + cf_clearance cookie
	cl, ok := client.GetClearance("1337x.to")
	if !ok || cl.UserAgent == "" {
		t.Fatalf("expected cached clearance for 1337x.to, got %+v", cl)
	}

	outReq, _ := http.NewRequest(http.MethodGet, "https://1337x.to/torrent/12345", nil)
	if !client.InjectClearance(outReq) {
		t.Fatalf("expected InjectClearance to return true")
	}
	if outReq.Header.Get("User-Agent") != "Mozilla/5.0 (X11; Linux x86_64) Chrome/124.0.0.0" {
		t.Fatalf("unexpected User-Agent: %s", outReq.Header.Get("User-Agent"))
	}
	ck, err := outReq.Cookie("cf_clearance")
	if err != nil || ck.Value != "solved_clearance_token_abc123" {
		t.Fatalf("expected cf_clearance cookie on request, got %v (err: %v)", ck, err)
	}

	// 6. Test session management (create, list, destroy)
	sid, err := client.CreateSession(ctx, mockFS.URL, "replacearr-session", nil)
	if err != nil || sid != "replacearr-session" {
		t.Fatalf("CreateSession failed: sid=%s err=%v", sid, err)
	}
	if err := client.DestroySession(ctx, mockFS.URL, sid); err != nil {
		t.Fatalf("DestroySession failed: %v", err)
	}
}
