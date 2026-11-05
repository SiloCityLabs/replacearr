package discover

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	defaultPorts = []int{8080, 8085, 9091, 8112, 6789}
	altPorts     = []int{8081, 8082, 8181, 9092}
	trueNASFrom  = 30000
	trueNASTo    = 32000 // inclusive
)

const (
	dialTimeout = 180 * time.Millisecond
	dialWorkers = 128
	composeVia  = "compose"
	localVia    = "local"
	gatewayVia  = "gateway"
	subnetVia   = "subnet"
)

var composeHosts = []string{
	"transmission",
	"qbittorrent",
	"deluge",
	"sabnzbd",
	"nzbget",
}

// localHostHints returns always-probed hosts (loopback, compose DNS, gateway, docker host).
func localHostHints() []HostHint {
	seen := map[string]struct{}{}
	var out []HostHint
	add := func(host, via string) {
		host = normalizeHost(host)
		if host == "" {
			return
		}
		if _, ok := seen[host]; ok {
			return
		}
		seen[host] = struct{}{}
		out = append(out, HostHint{Host: host, Via: via, FullPortScan: true})
	}

	add("127.0.0.1", localVia)
	add("localhost", localVia)
	for _, h := range composeHosts {
		add(h, composeVia)
	}
	if h := resolveHostDockerInternal(); h != "" {
		add(h, gatewayVia)
	}
	if gw := defaultGatewayIP(); gw != "" {
		add(gw, gatewayVia)
	}
	return out
}

const (
	maxLocalSubnetHosts = 256  // docker/bridge interface peers (defaults only)
	maxLANSubnetHosts   = 1024 // router-learned / env LAN (may include TrueNAS ports)
)

// subnetHostHints enumerates peers on local interfaces, router/host-learned LAN, and optional env CIDRs.
func subnetHostHints(lanNets []lanNetwork, extraCIDRs []string) []HostHint {
	seen := map[string]struct{}{}
	var out []HostHint
	addIP := func(ip net.IP, via string, full bool) {
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() {
			return
		}
		v4 := ip.To4()
		if v4 == nil || !v4.IsPrivate() {
			return
		}
		key := v4.String()
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, HostHint{Host: key, Via: via, FullPortScan: full})
	}

	// Local interface peers (usually the compose/docker bridge) — defaults only.
	for _, cidr := range localSubnetCIDRs() {
		for _, peer := range hostsInCIDR(cidr, maxLocalSubnetHosts) {
			addIP(peer, subnetVia, false)
		}
	}

	// Router IGD + host /proc/net/route LAN — real mask/range, include TrueNAS ports.
	for _, ln := range lanNets {
		via := ln.Via
		if via == "" {
			via = "igd"
		}
		for _, peer := range hostsFromLANNetwork(ln, maxLANSubnetHosts) {
			addIP(peer, via, true)
		}
	}

	// Optional manual override (REPLACEARR_DISCOVER_SUBNETS) for unusual setups.
	for _, cidr := range extraCIDRs {
		for _, peer := range hostsInCIDR(cidr, maxLANSubnetHosts) {
			addIP(peer, subnetVia, true)
		}
	}
	return out
}

func hostsFromLANNetwork(ln lanNetwork, limit int) []net.IP {
	if ln.MinHost != nil && ln.MaxHost != nil {
		return enumerateIPv4Range(ln.MinHost, ln.MaxHost, limit)
	}
	if ln.CIDR != "" {
		return hostsInCIDR(ln.CIDR, limit)
	}
	return nil
}

func hostsInCIDR(cidr string, limit int) []net.IP {
	_, ipNet, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil || ipNet == nil {
		return nil
	}
	ones, bits := ipNet.Mask.Size()
	if bits != 32 || ones <= 0 || ones > 30 {
		return nil
	}
	return enumerateIPv4Net(ipNet, limit)
}

func localSubnetCIDRs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []string
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
			ones, bits := ipNet.Mask.Size()
			if bits != 32 {
				continue
			}
			if ones < 24 {
				base := make(net.IP, 4)
				copy(base, ip)
				base[3] = 0
				out = append(out, base.String()+"/24")
				continue
			}
			out = append(out, ipNet.String())
		}
	}
	return out
}

func enumerateIPv4Net(ipNet *net.IPNet, limit int) []net.IP {
	ip := ipNet.IP.To4()
	if ip == nil {
		return nil
	}
	start := ipToUint32(ip.Mask(ipNet.Mask))
	ones, _ := ipNet.Mask.Size()
	hostBits := 32 - ones
	if hostBits <= 0 || hostBits > 16 {
		// Refuse enormous sweeps (/15 and wider); caller should use DHCP range from IGD.
		return nil
	}
	count := uint32(1) << uint(hostBits)
	if count < 2 {
		return nil
	}
	// Skip network and broadcast addresses.
	var out []net.IP
	for i := uint32(1); i < count-1 && len(out) < limit; i++ {
		out = append(out, uint32ToIP(start+i))
	}
	return out
}

func enumerateIPv4Range(minIP, maxIP net.IP, limit int) []net.IP {
	min4 := minIP.To4()
	max4 := maxIP.To4()
	if min4 == nil || max4 == nil {
		return nil
	}
	lo := ipToUint32(min4)
	hi := ipToUint32(max4)
	if hi < lo {
		lo, hi = hi, lo
	}
	var out []net.IP
	for cur := lo; cur <= hi && len(out) < limit; cur++ {
		out = append(out, uint32ToIP(cur))
	}
	return out
}

func ipToUint32(ip net.IP) uint32 {
	v4 := ip.To4()
	if v4 == nil {
		return 0
	}
	return uint32(v4[0])<<24 | uint32(v4[1])<<16 | uint32(v4[2])<<8 | uint32(v4[3])
}

func uint32ToIP(n uint32) net.IP {
	return net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n)).To4()
}

// ExtraSubnetsFromEnv reads REPLACEARR_DISCOVER_SUBNETS (comma-separated CIDRs).
func ExtraSubnetsFromEnv() []string {
	raw := strings.TrimSpace(os.Getenv("REPLACEARR_DISCOVER_SUBNETS"))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return strings.ToLower(host)
}

func resolveHostDockerInternal() string {
	ips, err := net.LookupHost("host.docker.internal")
	if err != nil || len(ips) == 0 {
		return ""
	}
	return ips[0]
}

func defaultGatewayIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
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
			if ip == nil || ip.IsLoopback() || !ip.IsPrivate() {
				continue
			}
			gw := make(net.IP, len(ip))
			copy(gw, ip)
			gw[3] = 1
			if gw.Equal(ip) {
				continue
			}
			return gw.String()
		}
	}
	return ""
}

// mergeHostHints dedupes by host, unioning SSDP ports onto existing entries.
func mergeHostHints(groups ...[]HostHint) []HostHint {
	byHost := map[string]*HostHint{}
	order := make([]string, 0)
	add := func(h HostHint) {
		key := h.Host
		if existing, ok := byHost[key]; ok {
			for _, p := range h.Ports {
				existing.Ports = appendUniquePort(existing.Ports, p)
			}
			if h.FullPortScan {
				existing.FullPortScan = true
			}
			if existing.Via == subnetVia && h.Via != "" && h.Via != subnetVia {
				existing.Via = h.Via
			} else if existing.Via == "" {
				existing.Via = h.Via
			}
			return
		}
		cp := h
		byHost[key] = &cp
		order = append(order, key)
	}
	for _, group := range groups {
		for _, h := range group {
			add(h)
		}
	}
	out := make([]HostHint, 0, len(order))
	for _, key := range order {
		out = append(out, *byHost[key])
	}
	return out
}

// portsForHost builds the port set for a single host.
func portsForHost(h HostHint, allowTrueNAS bool) []int {
	seen := map[int]struct{}{}
	var out []int
	add := func(p int) {
		if p <= 0 || p > 65535 {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	for _, p := range defaultPorts {
		add(p)
	}
	for _, p := range altPorts {
		add(p)
	}
	for _, p := range h.Ports {
		add(p)
	}
	if allowTrueNAS && h.FullPortScan {
		for p := trueNASFrom; p <= trueNASTo; p++ {
			add(p)
		}
	}
	return out
}

// openPort is a host:port that accepted a TCP connection.
type openPort struct {
	Host string
	Port int
	Via  string
}

// scanOpenPorts concurrently dials host:port pairs and returns those that connect.
func scanOpenPorts(ctx context.Context, hosts []HostHint, allowTrueNAS bool) []openPort {
	type job struct {
		host string
		port int
		via  string
	}
	jobs := make(chan job, 512)
	results := make(chan openPort, 512)

	var wg sync.WaitGroup
	workers := dialWorkers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				addr := net.JoinHostPort(j.host, strconv.Itoa(j.port))
				d := net.Dialer{Timeout: dialTimeout}
				conn, err := d.DialContext(ctx, "tcp", addr)
				if err != nil {
					continue
				}
				_ = conn.Close()
				select {
				case results <- openPort{Host: j.host, Port: j.port, Via: j.via}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, h := range hosts {
			ports := portsForHost(h, allowTrueNAS)
			for _, p := range ports {
				select {
				case <-ctx.Done():
					return
				case jobs <- job{host: h.Host, port: p, via: h.Via}:
				}
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var open []openPort
	for r := range results {
		open = append(open, r)
	}
	return open
}
