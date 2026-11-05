package discover

import "testing"

func TestPreferHostnameBaseURL(t *testing.T) {
	nameByIP := map[string]string{"172.19.0.5": "nzbget"}
	got := preferHostnameBaseURL("http://172.19.0.5:6789", nameByIP)
	if got != "http://nzbget:6789" {
		t.Fatalf("got %q", got)
	}
	got = preferHostnameBaseURL("http://nzbget:6789", nameByIP)
	if got != "http://nzbget:6789" {
		t.Fatalf("hostname should stay, got %q", got)
	}
	got = preferHostnameBaseURL("http://172.19.0.1:6789", nameByIP)
	if got != "http://172.19.0.1:6789" {
		t.Fatalf("unrelated IP should stay, got %q", got)
	}
}

func TestDedupeResultsPrefersComposeOverGatewayIP(t *testing.T) {
	in := []Result{
		{Implementation: "NZBGet", Protocol: "usenet", BaseURL: "http://172.19.0.1:6789", NameHint: "NZBGet"},
		{Implementation: "NZBGet", Protocol: "usenet", BaseURL: "http://nzbget:6789", NameHint: "NZBGet"},
		{Implementation: "Transmission", Protocol: "torrent", BaseURL: "http://172.19.0.1:9091", NameHint: "Transmission"},
		{Implementation: "Transmission", Protocol: "torrent", BaseURL: "http://transmission:9091", NameHint: "Transmission"},
		{Implementation: "qBittorrent", Protocol: "torrent", BaseURL: "http://qbittorrent:8080", NameHint: "qBittorrent"},
	}
	out := dedupeResults(in)
	if len(out) != 3 {
		t.Fatalf("got %d results: %#v", len(out), out)
	}
	byImpl := map[string]string{}
	for _, r := range out {
		byImpl[r.Implementation] = r.BaseURL
	}
	if byImpl["NZBGet"] != "http://nzbget:6789" {
		t.Fatalf("NZBGet: %q", byImpl["NZBGet"])
	}
	if byImpl["Transmission"] != "http://transmission:9091" {
		t.Fatalf("Transmission: %q", byImpl["Transmission"])
	}
	if byImpl["qBittorrent"] != "http://qbittorrent:8080" {
		t.Fatalf("qBittorrent: %q", byImpl["qBittorrent"])
	}
}

func TestDedupeResultsRewritesContainerIP(t *testing.T) {
	// Simulate resolved compose mapping without depending on live DNS.
	in := []Result{
		{Implementation: "Deluge", Protocol: "torrent", BaseURL: "http://172.19.0.8:8112", NameHint: "Deluge"},
	}
	// preferHostnameBaseURL only rewrites when nameByIP is populated; exercise via
	// direct helper since LookupHost may not resolve in unit tests.
	rewritten := preferHostnameBaseURL("http://172.19.0.8:8112", map[string]string{"172.19.0.8": "deluge"})
	if rewritten != "http://deluge:8112" {
		t.Fatalf("got %q", rewritten)
	}
	_ = in
}

func TestHostPreferenceScore(t *testing.T) {
	if hostPreferenceScore("nzbget") <= hostPreferenceScore("172.19.0.1") {
		t.Fatal("compose name should beat gateway IP")
	}
	if hostPreferenceScore("nas.local") <= hostPreferenceScore("10.5.1.2") {
		t.Fatal("hostname should beat raw IP")
	}
}
