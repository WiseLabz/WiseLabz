package traefik

import (
	"github.com/WiseLabz/wiselabz/internal/connector"
)

// discovery lets a network scan find Traefik: its version endpoint returns a
// Version, a Codename and a startDate. Portainer and Nginx Proxy Manager
// version answers lack the Codename and startDate and are not matched.
var discovery = &connector.DiscoveryHint{
	Probes: []connector.DiscoveryProbe{
		{Port: 8080, Scheme: "http", Path: "/api/version", Match: matchDiscovery},
	},
	URLTemplate: "{scheme}://{host}:{port}",
}

func matchDiscovery(r connector.DiscoveryResponse) bool {
	obj := r.JSONObject()
	version, _ := obj["Version"].(string)
	codename, _ := obj["Codename"].(string)
	_, hasStartDate := obj["startDate"].(string)
	return r.Status == 200 && version != "" && codename != "" && hasStartDate
}
