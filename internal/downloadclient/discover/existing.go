package discover

import (
	"net"
	"strconv"
	"strings"

	"github.com/replacearr/replacearr/internal/domain"
)

// ExistingClient is a configured download client used to filter suggestions.
type ExistingClient struct {
	Implementation string
	BaseURL        string
}

type existingIndex struct {
	urls      map[string]struct{} // normalized lowercase base URL
	endpoints map[string]struct{} // resolved ip:port
	implPort  map[string]struct{} // lowercase implementation|port
}

func buildExistingIndex(existing []ExistingClient) existingIndex {
	idx := existingIndex{
		urls:      map[string]struct{}{},
		endpoints: map[string]struct{}{},
		implPort:  map[string]struct{}{},
	}
	for _, c := range existing {
		norm, err := domain.NormalizeDownloadClientBaseURL(c.BaseURL)
		if err != nil {
			continue
		}
		idx.urls[strings.ToLower(norm)] = struct{}{}
		idx.endpoints[endpointKey(norm)] = struct{}{}
		port := resultPort(norm)
		impl := strings.ToLower(strings.TrimSpace(c.Implementation))
		if impl != "" && port > 0 {
			idx.implPort[impl+"|"+strconv.Itoa(port)] = struct{}{}
		}
	}
	return idx
}

func endpointKey(baseURL string) string {
	host, port, _ := domain.SplitDownloadClientBaseURL(baseURL)
	if ip := net.ParseIP(host); ip != nil {
		return strings.ToLower(ip.String()) + ":" + strconv.Itoa(port)
	}
	ips, err := net.LookupHost(host)
	if err == nil && len(ips) > 0 {
		return strings.ToLower(ips[0]) + ":" + strconv.Itoa(port)
	}
	return strings.ToLower(host) + ":" + strconv.Itoa(port)
}

func (idx existingIndex) blocks(res Result) bool {
	urlKey := strings.ToLower(res.BaseURL)
	if _, ok := idx.urls[urlKey]; ok {
		return true
	}
	if _, ok := idx.endpoints[endpointKey(res.BaseURL)]; ok {
		return true
	}
	impl := strings.ToLower(strings.TrimSpace(res.Implementation))
	port := resultPort(res.BaseURL)
	if impl != "" && port > 0 {
		if _, ok := idx.implPort[impl+"|"+strconv.Itoa(port)]; ok {
			return true
		}
	}
	return false
}

// filterExisting removes suggestions that match an already-configured client by
// base URL, resolved IP:port, or implementation+port (so transmission:9091 hides
// both the compose URL and any gateway/IP equivalent).
func filterExisting(results []Result, existing []ExistingClient) []Result {
	if len(existing) == 0 {
		return results
	}
	idx := buildExistingIndex(existing)
	out := make([]Result, 0, len(results))
	for _, res := range results {
		if idx.blocks(res) {
			continue
		}
		out = append(out, res)
	}
	return out
}

// dropGatewayWhenComposeExists removes raw-IP / gateway suggestions when the
// same implementation was already found via a compose DNS name (e.g. sabnzbd:8080
// vs 172.19.0.1:8085 — host port publish of the same service).
func dropGatewayWhenComposeExists(results []Result) []Result {
	composeImpls := map[string]struct{}{}
	for _, res := range results {
		if isComposeHost(resultHost(res.BaseURL)) {
			composeImpls[strings.ToLower(res.Implementation)] = struct{}{}
		}
	}
	if len(composeImpls) == 0 {
		return results
	}
	out := make([]Result, 0, len(results))
	for _, res := range results {
		host := resultHost(res.BaseURL)
		if isIPHost(host) {
			if _, ok := composeImpls[strings.ToLower(res.Implementation)]; ok {
				continue
			}
		}
		out = append(out, res)
	}
	return out
}
