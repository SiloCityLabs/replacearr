package deluge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/replacearr/replacearr/internal/domain"
)

func TestJSONEndpoint(t *testing.T) {
	cases := map[string]string{
		"http://deluge:8112":          "http://deluge:8112/json",
		"http://deluge:8112/":         "http://deluge:8112/json",
		"http://host:8112/json":       "http://host:8112/json",
		"http://host:8112/deluge":     "http://host:8112/deluge/json",
	}
	for in, want := range cases {
		if got := jsonEndpoint(in); got != want {
			t.Errorf("jsonEndpoint(%q)=%q want %q", in, got, want)
		}
	}
}

func TestAddListRemove(t *testing.T) {
	logged := false
	torrents := map[string]map[string]any{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json" {
			http.NotFound(w, r)
			return
		}
		var req rpcEnvelope
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		write := func(result any, errMsg string) {
			var errObj any
			if errMsg != "" {
				errObj = map[string]any{"message": errMsg, "code": 1}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": req.ID, "result": result, "error": errObj})
		}
		switch req.Method {
		case "auth.login":
			logged = true
			write(true, "")
		case "web.connected":
			write(true, "")
		case "core.add_torrent_magnet":
			if !logged {
				write(nil, "Not authenticated")
				return
			}
			magnet, _ := req.Params[0].(string)
			hash := strings.ToLower(extractHashFromMagnet(magnet))
			torrents[hash] = map[string]any{
				"name": "Show", "state": "Downloading", "progress": 10.0,
				"total_size": 100, "total_done": 10, "total_remaining": 90,
				"save_path": "/downloads", "hash": hash,
			}
			write(hash, "")
		case "core.add_torrent_file":
			if !logged {
				write(nil, "Not authenticated")
				return
			}
			if len(req.Params) < 2 {
				write(nil, "missing params")
				return
			}
			hash := "cccccccccccccccccccccccccccccccccccccccc"
			torrents[hash] = map[string]any{
				"name": "File", "state": "Downloading", "progress": 0.0,
				"total_size": 50, "total_done": 0, "total_remaining": 50,
				"save_path": "/downloads", "hash": hash,
			}
			write(hash, "")
		case "core.get_torrents_status":
			write(torrents, "")
		case "core.remove_torrent":
			hash, _ := req.Params[0].(string)
			delete(torrents, strings.ToLower(hash))
			write(true, "")
		case "label.set_torrent":
			write(nil, "")
		default:
			write(nil, "unknown method "+req.Method)
		}
	}))
	defer ts.Close()

	cli := NewFromDownloadClient(domain.DownloadClient{
		BaseURL: ts.URL, Password: "deluge", Category: "tv",
	})
	cli.http = ts.Client()

	id, err := cli.Add(context.Background(), "Show", "magnet:?xt=urn:btih:ABCDEF0123456789ABCDEF0123456789ABCDEF01&dn=x", "torrent")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if id != "abcdef0123456789abcdef0123456789abcdef01" {
		t.Fatalf("hash=%q", id)
	}
	items, err := cli.List(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("List: %v %#v", err, items)
	}
	if IsCompleted("Downloading", 0.1) || !IsCompleted("Seeding", 1) {
		t.Fatal("IsCompleted logic")
	}
	if IsSeedingComplete("Seeding", 1) || !IsSeedingComplete("Paused", 1) {
		t.Fatal("IsSeedingComplete logic")
	}
	if err := cli.Remove(context.Background(), id, false); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	fileID, err := cli.AddFile(context.Background(), "File", "file.torrent", []byte("d8:announce"))
	if err != nil {
		t.Fatalf("AddFile: %v", err)
	}
	if fileID != "cccccccccccccccccccccccccccccccccccccccc" {
		t.Fatalf("AddFile hash=%q", fileID)
	}
}
