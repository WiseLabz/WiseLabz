package portainer

import (
	"github.com/WiseLabz/wiselabz/internal/connector"
)

// discovery lets a network scan find Portainer: its unauthenticated system
// status endpoint returns a Version and an InstanceID.
var discovery = &connector.DiscoveryHint{
	Probes: []connector.DiscoveryProbe{
		{Port: 9443, Scheme: "https", Path: "/api/system/status", Match: matchDiscovery},
	},
	URLTemplate: "{scheme}://{host}:{port}",
}

func matchDiscovery(r connector.DiscoveryResponse) bool {
	obj := r.JSONObject()
	version, _ := obj["Version"].(string)
	instanceID, _ := obj["InstanceID"].(string)
	return r.Status == 200 && version != "" && instanceID != ""
}
