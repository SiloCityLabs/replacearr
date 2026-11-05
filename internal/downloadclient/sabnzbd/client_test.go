package sabnzbd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replacearr/replacearr/internal/domain"
)

func TestAddListRemove(t *testing.T) {
	queue := []map[string]any{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		if q.Get("apikey") != "secret-key" {
			_ = json.NewEncoder(w).Encode(map[string]any{"status": false, "error": "API Key Incorrect"})
			return
		}
		switch q.Get("mode") {
		case "addurl":
			id := "SABnzbd_nzo_1"
			queue = append(queue, map[string]any{
				"nzo_id": id, "filename": q.Get("nzbname"), "status": "Downloading",
				"percentage": "10.0", "mb": "100.0", "mbleft": "90.0", "timeleft": "0:05:23",
				"storage": "/downloads",
			})
			_ = json.NewEncoder(w).Encode(map[string]any{"status": true, "nzo_ids": []string{id}})
		case "queue":
			if q.Get("name") == "delete" {
				queue = nil
				_ = json.NewEncoder(w).Encode(map[string]any{"status": true})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"queue": map[string]any{"slots": queue}})
		case "history":
			_ = json.NewEncoder(w).Encode(map[string]any{"history": map[string]any{"slots": []any{}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	cli := NewFromDownloadClient(domain.DownloadClient{
		BaseURL: ts.URL, Password: "secret-key", Category: "tv",
	})
	cli.http = ts.Client()

	id, err := cli.Add(context.Background(), "Show.S01E01", "http://example.invalid/a.nzb", "usenet")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if id != "SABnzbd_nzo_1" {
		t.Fatalf("id=%q", id)
	}
	items, err := cli.List(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("List: %v %#v", err, items)
	}
	if items[0].Timeleft != "0:05:23" {
		t.Fatalf("Timeleft = %q", items[0].Timeleft)
	}
	if IsCompleted("Downloading", 0.1) || !IsCompleted("Completed", 1) {
		t.Fatal("IsCompleted")
	}
	if err := cli.Remove(context.Background(), id, false); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}
