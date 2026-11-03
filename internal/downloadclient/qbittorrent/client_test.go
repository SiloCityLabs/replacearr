package qbittorrent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/replacearr/replacearr/internal/domain"
)

func TestAddListRemove(t *testing.T) {
	var torrents []map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/auth/login":
			_, _ = w.Write([]byte("Ok."))
		case r.URL.Path == "/api/v2/torrents/add":
			_ = r.ParseForm()
			hash := "abcdef0123456789abcdef0123456789abcdef01"
			if h := extractHashFromMagnet(r.Form.Get("urls")); h != "" {
				hash = strings.ToLower(h)
			}
			torrents = append(torrents, map[string]any{
				"hash": hash, "name": r.Form.Get("rename"), "state": "downloading",
				"progress": 0.1, "size": 100, "amount_left": 90,
				"save_path": "/downloads", "content_path": "/downloads/Show",
			})
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Ok."))
		case r.URL.Path == "/api/v2/torrents/info":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(torrents)
		case r.URL.Path == "/api/v2/torrents/delete":
			_ = r.ParseForm()
			want := strings.ToLower(r.Form.Get("hashes"))
			filtered := torrents[:0]
			for _, tor := range torrents {
				if strings.ToLower(tor["hash"].(string)) != want {
					filtered = append(filtered, tor)
				}
			}
			torrents = filtered
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	cli := NewFromDownloadClient(domain.DownloadClient{
		BaseURL: ts.URL, Username: "admin", Password: "admin", Category: "tv",
	})
	cli.http = ts.Client()
	cli.baseURL = ts.URL

	id, err := cli.Add(context.Background(), "Severance", "magnet:?xt=urn:btih:ABCDEF0123456789ABCDEF0123456789ABCDEF01&dn=test", "torrent")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if id != "abcdef0123456789abcdef0123456789abcdef01" {
		t.Fatalf("hash = %q", id)
	}
	items, err := cli.List(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("List: %v %#v", err, items)
	}
	if !IsCompleted("uploading", 1) || IsCompleted("downloading", 0.5) {
		t.Fatalf("IsCompleted logic")
	}
	if err := cli.Remove(context.Background(), id, false); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}
