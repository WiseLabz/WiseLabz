package all

import (
	"reflect"
	"sort"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/connectortest"
)

// discoverable is the "Discoverable products" requirement of the
// network-discovery spec: connector type -> ports it is looked for on.
var discoverable = map[string][]int{
	"proxmox":        {8006},
	"pbs":            {8007},
	"home_assistant": {8123},
	"portainer":      {9443},
	"unifi":          {8443},
	"adguardhome":    {3000},
	"traefik":        {8080},
	"docker":         {2375},
	"caddy":          {2019},
	"npm":            {81},
	"pfsense":        {80, 443},
	"opnsense":       {80, 443},
	"truenas":        {80, 443},
	"pihole":         {80, 443},
}

func TestDiscoverableTypesAndPorts(t *testing.T) {
	got := map[string][]int{}
	for _, h := range connector.DiscoveryHints() {
		var ports []int
		for _, p := range h.Probes {
			ports = append(ports, p.Port)
		}
		sort.Ints(ports)
		got[h.Type] = ports
	}
	if !reflect.DeepEqual(got, discoverable) {
		t.Errorf("discoverable types and ports = %v, want %v", got, discoverable)
	}

	wantPorts := []int{80, 81, 443, 2019, 2375, 3000, 8006, 8007, 8080, 8123, 8443, 9443}
	if ports := connector.DiscoveryPorts(); !reflect.DeepEqual(ports, wantPorts) {
		t.Errorf("DiscoveryPorts() = %v, want %v", ports, wantPorts)
	}
}

func TestUndiscoverableTypesHaveNoHint(t *testing.T) {
	// Hosted services, custom REST, DNS resolver and TLS probe are never reported.
	for _, typ := range []string{"cloudflare", "netbird", "tailscale", "custom", "dnsresolver", "tlsprobe"} {
		schema, err := connector.GetTypeSchema(typ)
		if err != nil {
			t.Fatalf("GetTypeSchema(%s): %v", typ, err)
		}
		if schema.Discovery != nil {
			t.Errorf("%s declares a discovery hint", typ)
		}
	}
}

func TestDiscoveryHintsAreComplete(t *testing.T) {
	for _, h := range connector.DiscoveryHints() {
		schema, err := connector.GetTypeSchema(h.Type)
		if err != nil {
			t.Fatal(err)
		}
		if h.URLField != "" {
			found := false
			for _, f := range schema.Fields {
				found = found || f.Key == h.URLField
			}
			if !found {
				t.Errorf("%s: URLField %q is not a field of the type", h.Type, h.URLField)
			}
		}
	}
}

// TestDiscoveryMatchersDoNotCrossMatch checks that no product's identifying
// response is mistaken for another product's on a shared port, and that every
// discoverable type has fixtures.
func TestDiscoveryMatchersDoNotCrossMatch(t *testing.T) {
	covered := connectortest.RunDiscoveryCross(t)
	var want []string
	for typ := range discoverable {
		want = append(want, typ)
	}
	sort.Strings(want)
	if !reflect.DeepEqual(covered, want) {
		t.Errorf("fixtures cover %v, want %v", covered, want)
	}
}
