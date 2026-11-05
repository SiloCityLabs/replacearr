package discover

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestPortsForHostIncludesDefaultsAndSSDP(t *testing.T) {
	ports := portsForHost(HostHint{Host: "10.0.0.5", Ports: []int{30042}}, false)
	want := map[int]bool{8080: true, 9091: true, 30042: true, 8081: true}
	for p := range want {
		found := false
		for _, got := range ports {
			if got == p {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing port %d in %#v", p, ports)
		}
	}
	for _, p := range ports {
		if p >= trueNASFrom && p <= trueNASTo && p != 30042 {
			t.Fatalf("unexpected TrueNAS port %d when allowTrueNAS=false", p)
		}
	}
}

func TestPortsForHostTrueNASRangeRequiresFullPortScan(t *testing.T) {
	ports := portsForHost(HostHint{Host: "h", FullPortScan: true}, true)
	if len(ports) < (trueNASTo-trueNASFrom+1)+len(defaultPorts) {
		t.Fatalf("expected TrueNAS range included, got %d ports", len(ports))
	}
	ports = portsForHost(HostHint{Host: "h", FullPortScan: false}, true)
	for _, p := range ports {
		if p >= trueNASFrom && p <= trueNASTo {
			t.Fatalf("subnet peers should not scan TrueNAS range, got %d", p)
		}
	}
}

func TestScanOpenPortsFindsListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	open := scanOpenPorts(ctx, []HostHint{{
		Host:  "127.0.0.1",
		Ports: []int{port},
		Via:   "local",
	}}, false)

	found := false
	for _, op := range open {
		if op.Host == "127.0.0.1" && op.Port == port {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected open port %d, got %#v", port, open)
	}
}

func TestMergeHostHintsUnionsPorts(t *testing.T) {
	merged := mergeHostHints(
		[]HostHint{{Host: "a", Via: "local", Ports: []int{8080}}},
		[]HostHint{{Host: "a", Via: "ssdp", Ports: []int{30001}}, {Host: "b", Via: "ssdp"}},
	)
	if len(merged) != 2 {
		t.Fatalf("got %d hosts", len(merged))
	}
	var a HostHint
	for _, h := range merged {
		if h.Host == "a" {
			a = h
		}
	}
	if len(a.Ports) != 2 {
		t.Fatalf("expected unioned ports, got %#v", a.Ports)
	}
}
