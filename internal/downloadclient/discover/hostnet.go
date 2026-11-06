package discover

import (
	"bufio"
	"net"
	"os"
	"strconv"
	"strings"
)

// hostLANNetworksFromProcRoute reads the host routing table and returns private
// LAN networks that are not Docker/bridge ranges. dirs are candidate paths for
// the Docker host's /proc/net (bind-mounted); empty entries are skipped.
func hostLANNetworksFromProcRoute(dirs ...string) []lanNetwork {
	var out []lanNetwork
	seen := map[string]struct{}{}
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		path := strings.TrimRight(dir, "/") + "/route"
		nets := parseLinuxProcNetRoute(path)
		for _, n := range nets {
			if _, ok := seen[n.CIDR]; ok {
				continue
			}
			seen[n.CIDR] = struct{}{}
			out = append(out, n)
		}
	}
	return out
}

func parseLinuxProcNetRoute(path string) []lanNetwork {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var out []lanNetwork
	sc := bufio.NewScanner(f)
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			first = false
			continue // header
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		// destination, gateway, flags, ..., mask (hex little-endian)
		dest := parseProcNetHexIP(fields[1])
		mask := parseProcNetHexIP(fields[7])
		if dest == nil || mask == nil {
			continue
		}
		m := net.IPMask(mask.To4())
		if m == nil {
			continue
		}
		ones, bits := m.Size()
		if bits != 32 || ones == 0 || ones > 30 || ones < 8 {
			continue
		}
		// Skip default route (0.0.0.0/0) and host routes that are too specific for scanning.
		if ones == 0 || dest.Equal(net.IPv4zero) {
			continue
		}
		if !dest.IsPrivate() {
			continue
		}
		// Skip Docker / typical container bridge ranges.
		if isDockerOrLinkLocalNet(dest) {
			continue
		}
		network := &net.IPNet{IP: dest.Mask(m), Mask: m}
		out = append(out, lanNetwork{CIDR: network.String(), Via: "host-route"})
	}
	return out
}

func parseProcNetHexIP(hexStr string) net.IP {
	// /proc/net/route uses little-endian hex, e.g. 0105A8C0 = 192.168.5.1
	if len(hexStr) != 8 {
		return nil
	}
	v, err := strconv.ParseUint(hexStr, 16, 32)
	if err != nil {
		return nil
	}
	return net.IPv4(byte(v), byte(v>>8), byte(v>>16), byte(v>>24)).To4()
}

func isDockerOrLinkLocalNet(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return true
	}
	// 172.16.0.0/12 — Docker bridge / compose defaults
	if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
		return true
	}
	// 169.254.0.0/16 link-local
	if ip4[0] == 169 && ip4[1] == 254 {
		return true
	}
	return false
}
