package discover

import (
	"net"
	"strconv"
	"strings"

	"github.com/replacearr/replacearr/internal/domain"
)

// composeNameByIP maps resolved container IPs to preferred Docker DNS names.
func composeNameByIP() map[string]string {
	out := map[string]string{}
	for _, name := range composeHosts {
		ips, err := net.LookupHost(name)
		if err != nil {
			continue
		}
		for _, ip := range ips {
			ip = strings.ToLower(strings.TrimSpace(ip))
			if ip == "" {
				continue
			}
			// First hostname wins if two names somehow share an IP.
			if _, ok := out[ip]; !ok {
				out[ip] = name
			}
		}
	}
	return out
}

// preferHostnameBaseURL rewrites http://172.x.x.x:port to http://nzbget:port when
// the IP resolves from a known compose service name.
func preferHostnameBaseURL(baseURL string, nameByIP map[string]string) string {
	host, port, useSSL := domain.SplitDownloadClientBaseURL(baseURL)
	ip := net.ParseIP(host)
	if ip == nil {
		return baseURL
	}
	name, ok := nameByIP[strings.ToLower(ip.String())]
	if !ok || name == "" {
		return baseURL
	}
	scheme := "http"
	if useSSL {
		scheme = "https"
	}
	return scheme + "://" + name + ":" + strconv.Itoa(port)
}

func resultPort(baseURL string) int {
	_, port, _ := domain.SplitDownloadClientBaseURL(baseURL)
	return port
}

func resultHost(baseURL string) string {
	host, _, _ := domain.SplitDownloadClientBaseURL(baseURL)
	return host
}

func isIPHost(host string) bool {
	return net.ParseIP(host) != nil
}

func isComposeHost(host string) bool {
	host = strings.ToLower(host)
	for _, name := range composeHosts {
		if host == name {
			return true
		}
	}
	return false
}

// preferResult reports whether a is a better suggestion than b for the same endpoint.
// Prefer compose DNS names, then any hostname, then raw IPs.
func preferResult(a, b Result) bool {
	aHost := resultHost(a.BaseURL)
	bHost := resultHost(b.BaseURL)
	aScore := hostPreferenceScore(aHost)
	bScore := hostPreferenceScore(bHost)
	if aScore != bScore {
		return aScore > bScore
	}
	return false
}

func hostPreferenceScore(host string) int {
	switch {
	case isComposeHost(host):
		return 3
	case !isIPHost(host) && host != "" && !strings.EqualFold(host, "localhost"):
		return 2
	case strings.EqualFold(host, "localhost") || host == "127.0.0.1":
		return 1
	default:
		return 0 // raw LAN/gateway IP
	}
}

// dedupeResults collapses hits that are the same service reached via different
// addresses (compose DNS vs container IP vs gateway published port).
//
// Primary key: implementation + port (Docker publish maps gateway:port to the
// same service as composeName:port even when IPs differ).
// Within a key, prefer compose hostnames over IPs; rewrite container IPs to
// compose names when LookupHost matches.
func dedupeResults(in []Result) []Result {
	nameByIP := composeNameByIP()

	type key struct {
		impl string
		port int
	}
	best := map[key]Result{}
	var order []key

	for _, res := range in {
		res.BaseURL = preferHostnameBaseURL(res.BaseURL, nameByIP)
		k := key{
			impl: strings.ToLower(res.Implementation),
			port: resultPort(res.BaseURL),
		}
		if k.port <= 0 {
			continue
		}
		prev, ok := best[k]
		if !ok {
			best[k] = res
			order = append(order, k)
			continue
		}
		if preferResult(res, prev) {
			best[k] = res
		}
	}

	out := make([]Result, 0, len(order))
	for _, k := range order {
		out = append(out, best[k])
	}
	return out
}
