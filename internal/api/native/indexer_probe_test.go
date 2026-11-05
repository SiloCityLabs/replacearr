package native

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildIndexerCapsURL(t *testing.T) {
	got, err := buildIndexerCapsURL("https://indexer.example/api", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "t=caps") || !strings.Contains(got, "apikey=secret") {
		t.Fatalf("unexpected caps url %q", got)
	}

	got, err = buildIndexerCapsURL("https://indexer.example/api?t=caps&apikey=keep", "other")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "apikey=other") {
		t.Fatalf("should preserve existing apikey: %q", got)
	}
}

func TestHandleTestIndexerTorznabCaps(t *testing.T) {
	_, _, mux := newTestStack(t)
	token := login(t, mux)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("t") != "caps" {
			http.Error(w, "missing t=caps", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><caps><server title="demo"/></caps>`))
	}))
	t.Cleanup(upstream.Close)

	body := []byte(`{"name":"Demo","implementation":"Torznab","baseUrl":"` + upstream.URL + `/api","apiKey":"k"}`)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, authed(http.MethodPost, "/api/v1/indexers/test", token, body))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var res indexerTestResult
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("expected ok: %+v", res)
	}
	if res.LatencyMs < 0 {
		t.Fatalf("latency %d", res.LatencyMs)
	}
}

func TestHandleTestIndexerMissingImplementation(t *testing.T) {
	_, _, mux := newTestStack(t)
	token := login(t, mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, authed(http.MethodPost, "/api/v1/indexers/test", token, []byte(`{"name":"x"}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}
