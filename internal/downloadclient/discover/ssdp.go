package discover

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ssdpMulticast   = "239.255.255.250:1900"
	ssdpMaxHosts    = 64
	ssdpListenBuf   = 8192
	defaultSSDPWait = 3 * time.Second
	ssdpVia         = "ssdp"
)

// HostHint is a discovered host plus optional ports advertised via SSDP LOCATION.
type HostHint struct {
	Host         string
	Ports        []int
	Via          string // "ssdp", "local", "compose", "gateway", "subnet", "igd"
	FullPortScan bool   // include TrueNAS-style 3xxxx range
}

// ssdpDiscovery is the raw SSDP pass result (hosts + device description URLs).
type ssdpDiscovery struct {
	Hosts        []HostHint
	LocationURLs []string
}

var ssdpSearches = []string{
	"M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: ssdp:all\r\n\r\n",
	"M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: upnp:rootdevice\r\n\r\n",
	"M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: urn:schemas-upnp-org:device:InternetGatewayDevice:1\r\n\r\n",
	"M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: urn:schemas-upnp-org:device:InternetGatewayDevice:2\r\n\r\n",
}

func discoverSSDP(ctx context.Context, wait time.Duration) ssdpDiscovery {
	if wait <= 0 {
		wait = defaultSSDPWait
	}
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < wait {
			wait = remaining
		}
	}
	if wait <= 0 {
		return ssdpDiscovery{}
	}

	dst, err := net.ResolveUDPAddr("udp4", ssdpMulticast)
	if err != nil {
		return ssdpDiscovery{}
	}

	var mu sync.Mutex
	byHost := map[string]*HostHint{}
	locations := map[string]struct{}{}
	end := time.Now().Add(wait)

	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return ssdpDiscovery{}
	}
	defer conn.Close()
	_ = conn.SetDeadline(end)

	for _, msg := range ssdpSearches {
		_, _ = conn.WriteTo([]byte(msg), dst)
	}

	var wg sync.WaitGroup
	for _, local := range localIPv4Addrs() {
		laddr := &net.UDPAddr{IP: local, Port: 0}
		c, err := net.ListenUDP("udp4", laddr)
		if err != nil {
			continue
		}
		_ = c.SetDeadline(end)
		for _, msg := range ssdpSearches {
			_, _ = c.WriteToUDP([]byte(msg), dst)
		}
		wg.Add(1)
		go func(c *net.UDPConn) {
			defer wg.Done()
			defer c.Close()
			drainSSDP(ctx, c, &mu, byHost, locations, end)
		}(c)
	}

	buf := make([]byte, ssdpListenBuf)
	for time.Now().Before(end) {
		select {
		case <-ctx.Done():
			wg.Wait()
			return finishSSDP(byHost, locations)
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			break
		}
		recordSSDPReply(&mu, byHost, locations, string(buf[:n]))
	}
	wg.Wait()
	return finishSSDP(byHost, locations)
}

func finishSSDP(byHost map[string]*HostHint, locations map[string]struct{}) ssdpDiscovery {
	out := ssdpDiscovery{Hosts: hostHintsFromMap(byHost)}
	for loc := range locations {
		out.LocationURLs = append(out.LocationURLs, loc)
	}
	return out
}

func drainSSDP(ctx context.Context, c *net.UDPConn, mu *sync.Mutex, byHost map[string]*HostHint, locations map[string]struct{}, end time.Time) {
	buf := make([]byte, ssdpListenBuf)
	for time.Now().Before(end) {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_ = c.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, _, err := c.ReadFrom(buf)
		if err != nil {
			continue
		}
		recordSSDPReply(mu, byHost, locations, string(buf[:n]))
	}
}

func recordSSDPReply(mu *sync.Mutex, byHost map[string]*HostHint, locations map[string]struct{}, raw string) {
	host, port, loc := parseSSDPLocation(raw)
	mu.Lock()
	defer mu.Unlock()
	if loc != "" {
		locations[loc] = struct{}{}
	}
	if host == "" {
		return
	}
	h, ok := byHost[host]
	if !ok {
		if len(byHost) >= ssdpMaxHosts {
			return
		}
		h = &HostHint{Host: host, Via: ssdpVia, FullPortScan: true}
		byHost[host] = h
	}
	if port > 0 {
		h.Ports = appendUniquePort(h.Ports, port)
	}
}

func localIPv4Addrs() []net.IP {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []net.IP
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil || !ip.IsPrivate() {
				continue
			}
			out = append(out, append(net.IP(nil), ip...))
		}
	}
	return out
}

func hostHintsFromMap(byHost map[string]*HostHint) []HostHint {
	out := make([]HostHint, 0, len(byHost))
	for _, h := range byHost {
		out = append(out, *h)
	}
	return out
}

func parseSSDPLocation(raw string) (host string, port int, location string) {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(line), "location:") {
			continue
		}
		loc := strings.TrimSpace(line[len("location:"):])
		u, err := url.Parse(loc)
		if err != nil || u.Hostname() == "" {
			continue
		}
		host = u.Hostname()
		location = loc
		if p := u.Port(); p != "" {
			port, _ = strconv.Atoi(p)
		}
		return host, port, location
	}
	return "", 0, ""
}

func appendUniquePort(ports []int, port int) []int {
	if port <= 0 || port > 65535 {
		return ports
	}
	for _, p := range ports {
		if p == port {
			return ports
		}
	}
	return append(ports, port)
}
