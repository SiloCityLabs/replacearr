package nzbget

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/replacearr/replacearr/internal/domain"
)

func TestRPCEndpoint(t *testing.T) {
	cases := map[string]string{
		"http://nzbget:6789":            "http://nzbget:6789/jsonrpc",
		"http://nzbget:6789/":           "http://nzbget:6789/jsonrpc",
		"http://host:6789/jsonrpc":      "http://host:6789/jsonrpc",
		"http://host:6789/nzbget":       "http://host:6789/nzbget/jsonrpc",
	}
	for in, want := range cases {
		if got := rpcEndpoint(in); got != want {
			t.Errorf("rpcEndpoint(%q)=%q want %q", in, got, want)
		}
	}
}

func TestAddListRemove(t *testing.T) {
	nzbSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?><nzb></nzb>`))
	}))
	defer nzbSrv.Close()

	groups := []map[string]any{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jsonrpc" {
			http.NotFound(w, r)
			return
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		write := func(result any, errMsg string) {
			var errObj any
			if errMsg != "" {
				errObj = map[string]any{"code": 1, "message": errMsg}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result, "error": errObj})
		}
		switch req.Method {
		case "append":
			if len(req.Params) < 2 {
				write(nil, "bad params")
				return
			}
			content, _ := req.Params[1].(string)
			raw, err := base64.StdEncoding.DecodeString(content)
			if err != nil || !strings.Contains(string(raw), "<nzb") {
				write(nil, "bad nzb content")
				return
			}
			groups = append(groups, map[string]any{
				"NZBID": 42, "NZBName": "Show", "Status": "DOWNLOADING",
				"FileSizeLo": 100, "FileSizeHi": 0, "RemainingSizeLo": 50, "RemainingSizeHi": 0,
				"DestDir": "/downloads",
			})
			write(42, "")
		case "listgroups":
			write(groups, "")
		case "history":
			write([]any{}, "")
		case "editqueue":
			groups = nil
			write(true, "")
		default:
			write(nil, "unknown "+req.Method)
		}
	}))
	defer ts.Close()

	cli := NewFromDownloadClient(domain.DownloadClient{
		BaseURL: ts.URL, Username: "admin", Password: "secret", Category: "tv",
	})
	cli.http = ts.Client()

	id, err := cli.Add(context.Background(), "Show.S01E01", nzbSrv.URL+"/a.nzb", "usenet")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if id != "42" {
		t.Fatalf("id=%q", id)
	}
	items, err := cli.List(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("List: %v %#v", err, items)
	}
	if IsCompleted("DOWNLOADING", 0.2) || !IsCompleted("SUCCESS", 1) {
		t.Fatal("IsCompleted")
	}
	if err := cli.Remove(context.Background(), id, false); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}
