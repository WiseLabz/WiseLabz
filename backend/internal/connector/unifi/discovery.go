package unifi

import (
	"github.com/WiseLabz/wiselabz/internal/connector"
)

// discovery lets a network scan find a UniFi Network controller: its
// unauthenticated status endpoint answers with a meta object carrying the
// server version and controller uuid. A login-required answer has rc "error"
// and is not matched.
var discovery = &connector.DiscoveryHint{
	Probes: []connector.DiscoveryProbe{
		{Port: 8443, Scheme: "https", Path: "/status", Match: matchDiscovery},
	},
	URLTemplate: "{scheme}://{host}:{port}",
}

func matchDiscovery(r connector.DiscoveryResponse) bool {
	meta, _ := r.JSONObject()["meta"].(map[string]any)
	rc, _ := meta["rc"].(string)
	serverVersion, _ := meta["server_version"].(string)
	uuid, _ := meta["uuid"].(string)
	return r.Status == 200 && rc == "ok" && serverVersion != "" && uuid != ""
}
