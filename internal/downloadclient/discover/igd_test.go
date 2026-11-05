package discover

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeIGDLocationSubnetAndRange(t *testing.T) {
	desc := `<?xml version="1.0"?>
<root>
  <device>
    <deviceType>urn:schemas-upnp-org:device:InternetGatewayDevice:1</deviceType>
    <presentationURL>http://10.5.0.1/</presentationURL>
    <serviceList>
      <service>
        <serviceType>urn:schemas-upnp-org:service:LANHostConfigManagement:1</serviceType>
        <controlURL>/upnp/control/LANHostConfigManagement</controlURL>
      </service>
    </serviceList>
  </device>
</root>`

	mux := http.NewServeMux()
	mux.HandleFunc("/rootDesc.xml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(desc))
	})
	mux.HandleFunc("/upnp/control/LANHostConfigManagement", func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 2048)
		n, _ := r.Body.Read(buf)
		body := string(buf[:n])
		w.Header().Set("Content-Type", "text/xml")
		switch {
		case strings.Contains(body, "GetSubnetMask"):
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope><s:Body><u:GetSubnetMaskResponse>
				<NewSubnetMask>255.255.248.0</NewSubnetMask>
			</u:GetSubnetMaskResponse></s:Body></s:Envelope>`))
		case strings.Contains(body, "GetIPRoutersList"):
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope><s:Body><u:GetIPRoutersListResponse>
				<NewIPRouters>10.5.0.1</NewIPRouters>
			</u:GetIPRoutersListResponse></s:Body></s:Envelope>`))
		case strings.Contains(body, "GetAddressRange"):
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope><s:Body><u:GetAddressRangeResponse>
				<NewMinAddress>10.5.0.10</NewMinAddress>
				<NewMaxAddress>10.5.7.250</NewMaxAddress>
			</u:GetAddressRangeResponse></s:Body></s:Envelope>`))
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	nets := probeIGDLocation(context.Background(), srv.Client(), srv.URL+"/rootDesc.xml")
	if len(nets) == 0 {
		t.Fatal("expected LAN networks")
	}
	foundWide := false
	foundRange := false
	for _, n := range nets {
		if n.CIDR == "10.5.0.0/21" {
			foundWide = true
		}
		if n.MinHost != nil && n.MaxHost != nil &&
			n.MinHost.Equal(net.ParseIP("10.5.0.10")) &&
			n.MaxHost.Equal(net.ParseIP("10.5.7.250")) {
			foundRange = true
		}
	}
	if !foundWide {
		t.Fatalf("expected 10.5.0.0/21 from mask, got %#v", nets)
	}
	if !foundRange {
		t.Fatalf("expected DHCP range, got %#v", nets)
	}
}

func TestEnumerateIPv4NetWideSubnet(t *testing.T) {
	_, ipNet, err := net.ParseCIDR("10.5.0.0/21")
	if err != nil {
		t.Fatal(err)
	}
	hosts := enumerateIPv4Net(ipNet, 100)
	if len(hosts) != 100 {
		t.Fatalf("got %d hosts", len(hosts))
	}
	if !hosts[0].Equal(net.ParseIP("10.5.0.1")) {
		t.Fatalf("first host %v", hosts[0])
	}
}

func TestHostsFromLANNetworkPrefersDHCPRange(t *testing.T) {
	hosts := hostsFromLANNetwork(lanNetwork{
		CIDR:    "10.5.0.0/21",
		MinHost: net.ParseIP("10.5.1.2").To4(),
		MaxHost: net.ParseIP("10.5.1.5").To4(),
	}, 100)
	if len(hosts) != 4 {
		t.Fatalf("got %d", len(hosts))
	}
}
