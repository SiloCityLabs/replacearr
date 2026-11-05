package discover

import "testing"

func TestFilterExistingByImplPort(t *testing.T) {
	existing := []ExistingClient{
		{Implementation: "Transmission", BaseURL: "http://transmission:9091"},
	}
	in := []Result{
		{Implementation: "Transmission", BaseURL: "http://transmission:9091", NameHint: "Transmission"},
		{Implementation: "Transmission", BaseURL: "http://172.19.0.1:9091", NameHint: "Transmission"},
		{Implementation: "NZBGet", BaseURL: "http://nzbget:6789", NameHint: "NZBGet"},
	}
	out := filterExisting(in, existing)
	if len(out) != 1 || out[0].Implementation != "NZBGet" {
		t.Fatalf("got %#v", out)
	}
}

func TestDropGatewayWhenComposeExists(t *testing.T) {
	in := []Result{
		{Implementation: "SABnzbd", BaseURL: "http://sabnzbd:8080"},
		{Implementation: "SABnzbd", BaseURL: "http://172.19.0.1:8085"},
		{Implementation: "qBittorrent", BaseURL: "http://10.5.1.2:30042"},
	}
	out := dropGatewayWhenComposeExists(in)
	if len(out) != 2 {
		t.Fatalf("got %#v", out)
	}
	for _, r := range out {
		if r.BaseURL == "http://172.19.0.1:8085" {
			t.Fatal("gateway SABnzbd should be dropped")
		}
	}
}
