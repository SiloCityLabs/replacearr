package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSPAServesIndexForAppRoutes(t *testing.T) {
	h := NewHandler()

	for _, path := range []string{"/", "/series", "/movies"} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status %d", path, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "root") {
			t.Fatalf("%s body missing app root: %s", path, rr.Body.String())
		}
		if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
			t.Fatalf("%s content-type %s", path, ct)
		}
	}
}
