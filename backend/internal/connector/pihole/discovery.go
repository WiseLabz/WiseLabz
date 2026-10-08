package pihole

import (
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// discovery lets a network scan find Pi-hole. Pi-hole v5 sets an X-Pi-hole
// header on every /admin response, whatever the status. The v6 admin page
// is titled "Pi-hole <hostname>" and is matched when served directly.
// Pi-hole v6 behind a login redirect with no title is not recognised.
var discovery = &connector.DiscoveryHint{
	Probes: []connector.DiscoveryProbe{
		{Port: 443, Scheme: "https", Path: "/admin/", Match: matchDiscovery},
		{Port: 80, Scheme: "http", Path: "/admin/", Match: matchDiscovery},
	},
	URLTemplate: "{scheme}://{host}:{port}",
}

func matchDiscovery(r connector.DiscoveryResponse) bool {
	if r.Header.Get("X-Pi-hole") != "" {
		return true
	}
	return r.Status == 200 && strings.HasPrefix(r.Title(), "Pi-hole")
}
