package pbs

import (
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// discovery lets a network scan find Proxmox Backup Server: its web UI index
// page title ends in "Proxmox Backup Server". The page is served without
// credentials.
var discovery = &connector.DiscoveryHint{
	Probes: []connector.DiscoveryProbe{
		{Port: 8007, Scheme: "https", Path: "/", Match: matchDiscovery},
	},
	URLTemplate: "{scheme}://{host}:{port}",
}

func matchDiscovery(r connector.DiscoveryResponse) bool {
	return r.Status == 200 && strings.HasSuffix(r.Title(), " - Proxmox Backup Server")
}
