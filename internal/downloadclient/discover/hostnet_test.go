package discover

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseLinuxProcNetRoute(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "route")
	// Synthetic host route table: default via docker gw + LAN 10.5.0.0/21 + docker 172.19.0.0/16
	content := `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT                                                       
eth0	00000000	010013AC	0003	0	0	0	00000000	0	0	0                                                                               
eth0	0000050A	00000000	0001	0	0	0	00F8FFFF	0	0	0                                                                               
eth0	000013AC	00000000	0001	0	0	0	0000FFFF	0	0	0                                                                               
`
	// 0000050A little-endian = 10.5.0.0, mask 00F8FFFF = 255.255.248.0 (/21)
	// 000013AC = 172.19.0.0, mask 0000FFFF = 255.255.0.0
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	nets := parseLinuxProcNetRoute(path)
	if len(nets) != 1 {
		t.Fatalf("expected only LAN net, got %#v", nets)
	}
	if nets[0].CIDR != "10.5.0.0/21" {
		t.Fatalf("got %q", nets[0].CIDR)
	}
}

func TestParseProcNetHexIP(t *testing.T) {
	ip := parseProcNetHexIP("0000050A")
	if ip.String() != "10.5.0.0" {
		t.Fatalf("got %v", ip)
	}
}
