package npm

import (
	"github.com/WiseLabz/wiselabz/internal/connector"
)

// discovery lets a network scan find Nginx Proxy Manager: its unauthenticated
// API root reports status "OK", a boolean setup flag and a version object with
// a numeric major. Traefik's version answer has no status and is not matched.
var discovery = &connector.DiscoveryHint{
	Probes: []connector.DiscoveryProbe{
		{Port: 81, Scheme: "http", Path: "/api/", Match: matchDiscovery},
	},
	URLTemplate: "{scheme}://{host}:{port}",
}

func matchDiscovery(r connector.DiscoveryResponse) bool {
	if r.Status != 200 {
		return false
	}
	obj := r.JSONObject()
	status, _ := obj["status"].(string)
	_, hasSetup := obj["setup"].(bool)
	version, _ := obj["version"].(map[string]any)
	_, hasMajor := version["major"].(float64)
	return status == "OK" && hasSetup && hasMajor
}
