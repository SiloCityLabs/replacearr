package discover

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const igdHTTPTimeout = 3 * time.Second

var (
	controlURLRe = regexp.MustCompile(`(?is)<serviceType>\s*urn:schemas-upnp-org:service:LANHostConfigManagement:\d+\s*</serviceType>.*?<controlURL>\s*([^<]+)\s*</controlURL>`)
	anyControlRe = regexp.MustCompile(`(?is)<controlURL>\s*([^<]+)\s*</controlURL>`)
	ipRe         = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
)

// lanNetwork is a router-learned private network to probe.
type lanNetwork struct {
	CIDR     string // e.g. 10.5.0.0/21
	MinHost  net.IP // optional DHCP range start
	MaxHost  net.IP // optional DHCP range end
	Via      string
}

// discoverRouterLANNetworks finds LAN CIDRs / DHCP ranges via UPnP InternetGatewayDevice.
func discoverRouterLANNetworks(ctx context.Context, locationURLs []string) []lanNetwork {
	client := &http.Client{Timeout: igdHTTPTimeout}
	seen := map[string]struct{}{}
	var out []lanNetwork

	add := func(n lanNetwork) {
		key := n.CIDR + "|" + n.MinHost.String() + "|" + n.MaxHost.String()
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, n)
	}

	for _, loc := range locationURLs {
		select {
		case <-ctx.Done():
			return out
		default:
		}
		for _, n := range probeIGDLocation(ctx, client, loc) {
			add(n)
		}
	}
	return out
}

func probeIGDLocation(ctx context.Context, client *http.Client, location string) []lanNetwork {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || len(body) == 0 {
		return nil
	}

	base, err := url.Parse(location)
	if err != nil {
		return nil
	}

	var nets []lanNetwork
	// Seed from any private IPs mentioned in the device description / presentation URL.
	for _, ipStr := range ipRe.FindAllString(string(body), -1) {
		ip := net.ParseIP(ipStr)
		if ip == nil || ip.To4() == nil || !ip.IsPrivate() || ip.IsLoopback() {
			continue
		}
		// Tentative /24 until SOAP mask/range refines it — still useful as a seed host network.
		n := &net.IPNet{IP: ip.Mask(net.CIDRMask(24, 32)), Mask: net.CIDRMask(24, 32)}
		nets = append(nets, lanNetwork{CIDR: n.String(), Via: "igd"})
	}

	controlPath := findLANHostConfigControlURL(body)
	if controlPath == "" {
		return uniqueLANNetworks(nets)
	}
	controlURL := resolveURL(base, controlPath)

	mask := soapGetNewValue(ctx, client, controlURL, "GetSubnetMask", "NewSubnetMask")
	routerIP := firstIP(soapGetNewValue(ctx, client, controlURL, "GetIPRoutersList", "NewIPRouters"))
	rangeXML := soapActionRaw(ctx, client, controlURL, "GetAddressRange")
	minAddr := firstIP(extractXMLTag(rangeXML, "NewMinAddress"))
	maxAddr := firstIP(extractXMLTag(rangeXML, "NewMaxAddress"))

	if routerIP == nil {
		if h := base.Hostname(); h != "" {
			routerIP = net.ParseIP(h)
		}
	}

	if routerIP != nil && routerIP.To4() != nil && routerIP.IsPrivate() {
		ones := 24
		if maskIP := net.ParseIP(strings.TrimSpace(mask)); maskIP != nil {
			if m := net.IPMask(maskIP.To4()); m != nil {
				if o, bits := m.Size(); bits == 32 && o > 0 {
					ones = o
				}
			}
		}
		ip4 := routerIP.To4()
		network := &net.IPNet{IP: ip4.Mask(net.CIDRMask(ones, 32)), Mask: net.CIDRMask(ones, 32)}
		ln := lanNetwork{CIDR: network.String(), Via: "igd"}
		if minAddr != nil && maxAddr != nil {
			ln.MinHost = minAddr.To4()
			ln.MaxHost = maxAddr.To4()
		}
		nets = append([]lanNetwork{ln}, nets...)
	} else if minAddr != nil && maxAddr != nil {
		nets = append(nets, lanNetwork{
			MinHost: minAddr.To4(),
			MaxHost: maxAddr.To4(),
			Via:     "igd",
		})
	}

	return uniqueLANNetworks(nets)
}

func findLANHostConfigControlURL(desc []byte) string {
	if m := controlURLRe.FindSubmatch(desc); len(m) > 1 {
		return strings.TrimSpace(string(m[1]))
	}
	// Fallback: first controlURL in description (less precise).
	_ = anyControlRe
	return ""
}

func resolveURL(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	return base.ResolveReference(u).String()
}

func soapGetNewValue(ctx context.Context, client *http.Client, controlURL, action, valueTag string) string {
	return extractXMLTag(soapActionRaw(ctx, client, controlURL, action), valueTag)
}

func soapActionRaw(ctx context.Context, client *http.Client, controlURL, action string) []byte {
	if controlURL == "" {
		return nil
	}
	const service = "urn:schemas-upnp-org:service:LANHostConfigManagement:1"
	body := fmt.Sprintf(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body><u:%s xmlns:u="%s"></u:%s></s:Body>
</s:Envelope>`, action, service, action)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, controlURL, bytes.NewReader([]byte(body)))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", `"`+service+`#`+action+`"`)

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil
	}
	return raw
}

func extractXMLTag(raw []byte, tag string) string {
	// Lightweight tag extract — router XML is usually simple.
	re := regexp.MustCompile(`(?is)<` + regexp.QuoteMeta(tag) + `[^>]*>\s*([^<]+)\s*</` + regexp.QuoteMeta(tag) + `>`)
	if m := re.FindSubmatch(raw); len(m) > 1 {
		return strings.TrimSpace(string(m[1]))
	}
	// Decode as generic XML tokens if regex misses namespaces.
	dec := xml.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		name := se.Name.Local
		if !strings.EqualFold(name, tag) {
			continue
		}
		var val string
		if err := dec.DecodeElement(&val, &se); err == nil {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

func firstIP(s string) net.IP {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// GetIPRoutersList can be comma/space separated.
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t'
	}) {
		if ip := net.ParseIP(part); ip != nil {
			return ip
		}
	}
	if m := ipRe.FindString(s); m != "" {
		return net.ParseIP(m)
	}
	return nil
}

func uniqueLANNetworks(in []lanNetwork) []lanNetwork {
	seen := map[string]struct{}{}
	var out []lanNetwork
	for _, n := range in {
		key := n.CIDR + "|" + n.MinHost.String() + "|" + n.MaxHost.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, n)
	}
	return out
}
