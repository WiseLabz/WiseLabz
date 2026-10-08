package proxmox

import (
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// discovery lets a network scan find Proxmox VE: its API daemon names itself
// in the Server header of every response, and its login page title ends in
// "Proxmox Virtual Environment". Both are served without credentials.
var discovery = &connector.DiscoveryHint{
	Probes: []connector.DiscoveryProbe{
		{Port: 8006, Scheme: "https", Path: "/", Match: matchDiscovery},
	},
	URLTemplate: "{scheme}://{host}:{port}/api2/json",
}

func matchDiscovery(r connector.DiscoveryResponse) bool {
	if strings.HasPrefix(r.Header.Get("Server"), "pve-api-daemon") {
		return true
	}
	return r.Status == 200 && strings.HasSuffix(r.Title(), " - Proxmox Virtual Environment")
}
