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

func TestAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/auth/login" {
			http.NotFound(w, r)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("username") != "admin" || r.Form.Get("password") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Unauthorized"))
			return
		}
		_, _ = w.Write([]byte("Ok."))
	}))
	defer ts.Close()

	ok := NewFromDownloadClient(domain.DownloadClient{
		BaseURL: ts.URL, Username: "admin", Password: "secret",
	})
	ok.http = ts.Client()
	ok.baseURL = ts.URL
	if err := ok.Test(context.Background()); err != nil {
		t.Fatalf("Test: %v", err)
	}

	bad := NewFromDownloadClient(domain.DownloadClient{
		BaseURL: ts.URL, Username: "admin", Password: "wrong",
	})
	bad.http = ts.Client()
	bad.baseURL = ts.URL
	if err := bad.Test(context.Background()); err == nil {
		t.Fatal("expected auth failure")
	}
}

func TestAuthAccepts204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/auth/login" {
			http.NotFound(w, r)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "SID", Value: "abc123", Path: "/"})
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	cli := NewFromDownloadClient(domain.DownloadClient{
		BaseURL: ts.URL, Username: "admin", Password: "secret",
	})
	cli.http = ts.Client()
	cli.baseURL = ts.URL
	if err := cli.Test(context.Background()); err != nil {
		t.Fatalf("Test: %v", err)
	}
}

func TestLoginSucceeded(t *testing.T) {
	okBody := httptest.NewRecorder()
	okBody.WriteHeader(http.StatusOK)
	if !loginSucceeded(okBody.Result(), []byte("Ok.")) {
		t.Fatal("expected Ok. body to succeed")
	}
	noContent := httptest.NewRecorder()
	noContent.WriteHeader(http.StatusNoContent)
	if !loginSucceeded(noContent.Result(), nil) {
		t.Fatal("expected 204 to succeed")
	}
	unauthorized := httptest.NewRecorder()
	unauthorized.WriteHeader(http.StatusUnauthorized)
	if loginSucceeded(unauthorized.Result(), []byte("Unauthorized")) {
		t.Fatal("expected 401 to fail")
	}
}

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
				"progress": 0.1, "size": 100, "amount_left": 90, "eta": 250,
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
	if items[0].EtaSeconds != 250 {
		t.Fatalf("EtaSeconds = %d", items[0].EtaSeconds)
	}
	if !IsCompleted("uploading", 1) || IsCompleted("downloading", 0.5) {
		t.Fatalf("IsCompleted logic")
	}
	if IsSeedingComplete("uploading", 1) || !IsSeedingComplete("stoppedUP", 1) {
		t.Fatalf("IsSeedingComplete logic")
	}
	if err := cli.Remove(context.Background(), id, false); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}

func TestAddFile(t *testing.T) {
	var gotFile, gotCategory, gotRename string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/auth/login":
			_, _ = w.Write([]byte("Ok."))
		case r.URL.Path == "/api/v2/torrents/add":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("ParseMultipartForm: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			gotCategory = r.FormValue("category")
			gotRename = r.FormValue("rename")
			f, hdr, err := r.FormFile("torrents")
			if err != nil {
				t.Errorf("FormFile: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer f.Close()
			gotFile = hdr.Filename
			buf := make([]byte, 64)
			n, _ := f.Read(buf)
			if string(buf[:n]) != "d8:announce" {
				t.Errorf("file body = %q", buf[:n])
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Ok."))
		case r.URL.Path == "/api/v2/torrents/info":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"hash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "name": "Manual",
				"state": "downloading", "progress": 0, "size": 1, "amount_left": 1, "eta": 1,
			}})
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

	id, err := cli.AddFile(context.Background(), "Manual", "show", []byte("d8:announce"))
	if err != nil {
		t.Fatalf("AddFile: %v", err)
	}
	if id != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("hash = %q", id)
	}
	if gotFile != "show.torrent" || gotCategory != "tv" || gotRename != "Manual" {
		t.Fatalf("got file=%q category=%q rename=%q", gotFile, gotCategory, gotRename)
	}
}
