package transmission

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/replacearr/replacearr/internal/domain"
)

func TestRPCEndpoint(t *testing.T) {
	cases := map[string]string{
		"http://transmission:9091":                   "http://transmission:9091/transmission/rpc",
		"http://transmission:9091/":                  "http://transmission:9091/transmission/rpc",
		"http://host:9091/transmission":              "http://host:9091/transmission/rpc",
		"http://host:9091/transmission/rpc":          "http://host:9091/transmission/rpc",
		"https://host/custom/rpc":                    "https://host/custom/rpc",
	}
	for in, want := range cases {
		if got := rpcEndpoint(in); got != want {
			t.Errorf("rpcEndpoint(%q)=%q want %q", in, got, want)
		}
	}
}

func TestAddListRemove(t *testing.T) {
	session := ""
	var torrents []map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transmission/rpc" {
			http.NotFound(w, r)
			return
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("X-Transmission-Session-Id") != session {
			session = "sid-test"
			w.Header().Set("X-Transmission-Session-Id", session)
			w.WriteHeader(http.StatusConflict)
			return
		}
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch req.Method {
		case "torrent-add":
			args, _ := req.Arguments.(map[string]any)
			if args == nil {
				_ = json.Unmarshal(mustJSON(req.Arguments), &args)
			}
			filename, _ := args["filename"].(string)
			metainfo, _ := args["metainfo"].(string)
			hash := "abcdef0123456789abcdef0123456789abcdef01"
			if h := extractHashFromMagnet(filename); h != "" {
				hash = strings.ToLower(h)
			}
			if metainfo != "" {
				hash = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			}
			torrents = append(torrents, map[string]any{
				"id": 1, "hashString": hash, "name": "Severance",
				"status": 4, "totalSize": 100, "leftUntilDone": 90, "percentDone": 0.1,
				"eta": 125, "downloadDir": "/downloads",
			})
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": "success",
				"arguments": map[string]any{
					"torrent-added": map[string]any{"id": 1, "hashString": hash, "name": "Severance"},
				},
			})
		case "torrent-get":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result":    "success",
				"arguments": map[string]any{"torrents": torrents},
			})
		case "torrent-remove":
			torrents = nil
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "success", "arguments": map[string]any{}})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	cli := NewFromDownloadClient(domain.DownloadClient{
		BaseURL: ts.URL, Username: "admin", Password: "secret", Category: "tv",
	})
	cli.http = ts.Client()

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
	if items[0].EtaSeconds != 125 {
		t.Fatalf("EtaSeconds = %d", items[0].EtaSeconds)
	}
	if items[0].ContentPath != "/downloads/Severance" {
		t.Fatalf("ContentPath = %q", items[0].ContentPath)
	}
	if IsCompleted(4, 0.1, 90) || !IsCompleted(6, 1, 0) {
		t.Fatalf("IsCompleted logic")
	}
	if IsCompleted(0, 0, 0) || IsCompleted(4, 0, 0) {
		t.Fatalf("metadata-only torrent must not be completed")
	}
	if IsSeedingComplete(6, 1, 0, false) || !IsSeedingComplete(0, 1, 0, true) {
		t.Fatalf("IsSeedingComplete logic")
	}
	if err := cli.Remove(context.Background(), id, false); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	fileID, err := cli.AddFile(context.Background(), "Manual", "manual.torrent", []byte("d8:announce"))
	if err != nil {
		t.Fatalf("AddFile: %v", err)
	}
	if fileID != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("AddFile hash = %q", fileID)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
