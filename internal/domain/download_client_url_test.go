package domain

import "testing"

func TestNormalizeDownloadClientBaseURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "http://localhost:8080"},
		{"http://localhost:8080", "http://localhost:8080"},
		{"http://localhost:8080/", "http://localhost:8080"},
		{"https://qbittorrent.example/qbittorrent/", "https://qbittorrent.example/qbittorrent"},
		{"localhost:9091", "http://localhost:9091"},
	}
	for _, tc := range cases {
		got, err := NormalizeDownloadClientBaseURL(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeDownloadClientBaseURLRejectsBadScheme(t *testing.T) {
	if _, err := NormalizeDownloadClientBaseURL("ftp://localhost:21"); err == nil {
		t.Fatal("expected error")
	}
}

func TestProtocolForDownloadClientImplementation(t *testing.T) {
	cases := []struct {
		impl, preferred, want string
	}{
		{"Transmission", "usenet", "torrent"},
		{"qBittorrent", "", "torrent"},
		{"SABnzbd", "torrent", "usenet"},
		{"NZBGet", "", "usenet"},
		{"Plugin", "usenet", "usenet"},
		{"Plugin", "torrent", "torrent"},
	}
	for _, tc := range cases {
		if got := ProtocolForDownloadClientImplementation(tc.impl, tc.preferred); got != tc.want {
			t.Fatalf("%s/%s: got %q want %q", tc.impl, tc.preferred, got, tc.want)
		}
	}
}
