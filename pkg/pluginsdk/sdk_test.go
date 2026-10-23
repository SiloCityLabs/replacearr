package pluginsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type stubPlugin struct {
	configured map[string]any
	notified   []string
}

func (s *stubPlugin) Manifest() Manifest {
	return Manifest{
		ID:          "stub-plugin",
		Name:        "Stub Plugin",
		Version:     "1.0.0",
		Author:      "Test",
		Description: "Stub plugin for SDK unit tests",
		Types: []PluginType{
			PluginTypeIndexer,
			PluginTypeDownloadClient,
			PluginTypeMetadata,
			PluginTypeNotification,
		},
		ConfigSchema: []ConfigFieldSchema{
			{Name: "api_key", Label: "API Key", Type: "password", Required: true},
		},
	}
}

func (s *stubPlugin) Configure(settings map[string]any) error {
	s.configured = settings
	return nil
}

func (s *stubPlugin) Search(_ context.Context, req SearchRequest) ([]Release, error) {
	return []Release{
		{
			Guid:        "guid-1",
			Title:       req.Query + ".1080p.WEB-DL",
			Protocol:    "torrent",
			Size:        1024 * 1024 * 1500,
			Seeders:     42,
			Peers:       50,
			PublishDate: "2026-10-23T12:00:00Z",
			DownloadURL: "magnet:?xt=urn:btih:12345",
			InfoURL:     "https://example.local/details/1",
			Categories:  []int{2000, 5000},
			Quality:     "WEBDL-1080p",
		},
	}, nil
}

func (s *stubPlugin) EnqueueDownload(_ context.Context, title, _, _ string) (string, error) {
	return "dl-" + title, nil
}

func (s *stubPlugin) LookupMedia(_ context.Context, mediaType, query string) ([]map[string]any, error) {
	return []map[string]any{
		{"mediaType": mediaType, "title": query, "year": 2026},
	}, nil
}

func (s *stubPlugin) Notify(_ context.Context, eventType string, _ map[string]any) error {
	s.notified = append(s.notified, eventType)
	return nil
}

func TestServeIOAllMethods(t *testing.T) {
	p := &stubPlugin{}

	requests := []string{
		`{"id":1,"method":"manifest"}`,
		`{"id":2,"method":"configure","params":{"api_key":"secret123"}}`,
		`{"id":3,"method":"search","params":{"query":"Severance","type":"tvsearch","season":2,"episode":1}}`,
		`{"id":4,"method":"enqueue_download","params":{"title":"Severance.S02E01","downloadUrl":"magnet:?xt=1","protocol":"torrent"}}`,
		`{"id":5,"method":"lookup_media","params":{"mediaType":"tv","query":"Severance"}}`,
		`{"id":6,"method":"notify","params":{"eventType":"OnGrab","payload":{"title":"Severance"}}}`,
		`{"id":7,"method":"health"}`,
	}

	in := strings.NewReader(strings.Join(requests, "\n") + "\n")
	var out bytes.Buffer

	if err := ServeIO(context.Background(), p, in, &out); err != nil {
		t.Fatalf("ServeIO failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != len(requests) {
		t.Fatalf("expected %d responses, got %d: %s", len(requests), len(lines), out.String())
	}

	for i, line := range lines {
		var resp RPCResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("unmarshal response %d: %v", i+1, err)
		}
		if resp.Error != "" {
			t.Fatalf("unexpected error on request %d: %s", i+1, resp.Error)
		}
		if resp.ID != int64(i+1) {
			t.Fatalf("expected ID %d, got %d", i+1, resp.ID)
		}
	}

	if p.configured["api_key"] != "secret123" {
		t.Fatalf("expected configured api_key='secret123', got %v", p.configured)
	}
	if len(p.notified) != 1 || p.notified[0] != "OnGrab" {
		t.Fatalf("expected OnGrab notification, got %v", p.notified)
	}
}
