package discover

import (
	"strings"
	"testing"
)

func TestParseSSDPLocation(t *testing.T) {
	raw := "HTTP/1.1 200 OK\r\nCACHE-CONTROL: max-age=120\r\nLOCATION: http://192.168.1.50:1400/xml/device_description.xml\r\nST: upnp:rootdevice\r\n\r\n"
	host, port, loc := parseSSDPLocation(raw)
	if host != "192.168.1.50" || port != 1400 {
		t.Fatalf("got %s:%d", host, port)
	}
	if !strings.Contains(loc, "device_description.xml") {
		t.Fatalf("location %q", loc)
	}
}

func TestParseSSDPLocationCaseInsensitive(t *testing.T) {
	raw := "HTTP/1.1 200 OK\r\nLocation: http://10.0.0.2:8080/rootDesc.xml\r\n\r\n"
	host, port, _ := parseSSDPLocation(raw)
	if host != "10.0.0.2" || port != 8080 {
		t.Fatalf("got %s:%d", host, port)
	}
}

func TestParseSSDPLocationMissing(t *testing.T) {
	host, port, loc := parseSSDPLocation("HTTP/1.1 200 OK\r\nST: ssdp:all\r\n\r\n")
	if host != "" || port != 0 || loc != "" {
		t.Fatalf("expected empty, got %s:%d %q", host, port, loc)
	}
}

func TestAppendUniquePort(t *testing.T) {
	ports := appendUniquePort(nil, 8080)
	ports = appendUniquePort(ports, 8080)
	ports = appendUniquePort(ports, 9091)
	if len(ports) != 2 || ports[0] != 8080 || ports[1] != 9091 {
		t.Fatalf("got %#v", ports)
	}
}
