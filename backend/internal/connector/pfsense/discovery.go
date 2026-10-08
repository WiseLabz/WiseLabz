package pfsense

import (
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// discovery lets a network scan find pfSense: its webConfigurator login page
// has a body id of "login", links /css/login.css and names its username field
// "usernamefld". The page is served without credentials on both ports.
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
	return strings.Contains(body, `<body id="login"`) &&
		strings.Contains(body, "/css/login.css") &&
		strings.Contains(body, `name="usernamefld"`)
}
