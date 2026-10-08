package npm

import (
	"github.com/WiseLabz/wiselabz/internal/connector"
)

// discovery lets a network scan find Nginx Proxy Manager: its unauthenticated
// API root reports status "OK" and a version object with numeric major, minor
// and revision. The boolean setup flag is not required, because it only exists
// from 2.13.0 on. Traefik's version answer has no status and is not matched.
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
	if status, _ := obj["status"].(string); status != "OK" {
		return false
	}
	version, _ := obj["version"].(map[string]any)
	for _, k := range []string{"major", "minor", "revision"} {
		if _, ok := version[k].(float64); !ok {
			return false
		}
	}
	return true
}
