package truenas

import (
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// discovery lets a network scan find TrueNAS: its web UI is an Angular shell
// whose title element has the id "main-page-title" and whose root element is
// an <ix-root> (or <app-root> in older builds). The shell is served without
// credentials on both ports.
var discovery = &connector.DiscoveryHint{
	Probes: []connector.DiscoveryProbe{
		{Port: 443, Scheme: "https", Path: "/", Match: matchDiscovery},
		{Port: 80, Scheme: "http", Path: "/", Match: matchDiscovery},
	},
	URLTemplate: "{scheme}://{host}:{port}",
}

func matchDiscovery(r connector.DiscoveryResponse) bool {
	if r.Status != 200 {
		return false
	}
	body := string(r.Body)
	return strings.Contains(body, `id="main-page-title"`) &&
		(strings.Contains(body, "<ix-root") || strings.Contains(body, "<app-root"))
}
