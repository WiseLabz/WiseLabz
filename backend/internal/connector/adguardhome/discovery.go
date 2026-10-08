package adguardhome

import (
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// discovery lets a network scan find AdGuard Home: its JSON API answers with
// "Server: AdGuardHome/<version>". With web login enabled, the unauthenticated
// status request gets a bare 401 without that header, so such an instance is
// not recognised.
var discovery = &connector.DiscoveryHint{
	Probes: []connector.DiscoveryProbe{
		{Port: 3000, Scheme: "http", Path: "/control/status", Match: matchDiscovery},
	},
	URLTemplate: "{scheme}://{host}:{port}",
}

// matchDiscovery accepts any status that carries the AdGuardHome Server header.
// It deliberately ignores the body.
func matchDiscovery(r connector.DiscoveryResponse) bool {
	return strings.HasPrefix(r.Header.Get("Server"), "AdGuardHome/")
}
