package homeassistant

import (
	"github.com/WiseLabz/wiselabz/internal/connector"
)

// discovery lets a network scan find Home Assistant: its web app manifest
// names the instance "Home Assistant". The manifest is served without
// credentials.
var discovery = &connector.DiscoveryHint{
	Probes: []connector.DiscoveryProbe{
		{Port: 8123, Scheme: "http", Path: "/manifest.json", Match: matchDiscovery},
	},
	URLTemplate: "{scheme}://{host}:{port}",
}

func matchDiscovery(r connector.DiscoveryResponse) bool {
	name, _ := r.JSONObject()["name"].(string)
	return r.Status == 200 && name == "Home Assistant"
}
