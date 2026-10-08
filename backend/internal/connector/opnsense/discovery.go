package opnsense

import (
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// discovery lets a network scan find OPNsense: its login page title ends in
// "| OPNsense" (the prefix is localised) and the page has a username field
// named "usernamefld". The page is served without credentials on both ports.
var discovery = &connector.DiscoveryHint{
	Probes: []connector.DiscoveryProbe{
		{Port: 443, Scheme: "https", Path: "/", Match: matchDiscovery},
		{Port: 80, Scheme: "http", Path: "/", Match: matchDiscovery},
	},
	URLTemplate: "{scheme}://{host}:{port}",
}

func matchDiscovery(r connector.DiscoveryResponse) bool {
	return r.Status == 200 &&
		strings.HasSuffix(r.Title(), "| OPNsense") &&
		strings.Contains(string(r.Body), `name="usernamefld"`)
}
