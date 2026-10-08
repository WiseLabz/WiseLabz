package docker

import (
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// discovery lets a network scan find a Docker Engine API on its TCP port. The
// connector endpoint lives in the host field, so the prefilled URL goes there.
// The engine names itself in the Server header; without it, the version body
// must list an "Engine" component, which Podman's compat API does not.
var discovery = &connector.DiscoveryHint{
	Probes: []connector.DiscoveryProbe{
		{Port: 2375, Scheme: "http", Path: "/version", Match: matchDiscovery},
	},
	URLTemplate: "tcp://{host}:{port}",
	URLField:    "host",
}

func matchDiscovery(r connector.DiscoveryResponse) bool {
	if r.Status != 200 {
		return false
	}
	if strings.HasPrefix(r.Header.Get("Server"), "Docker/") {
		return true
	}
	obj := r.JSONObject()
	apiVersion, _ := obj["ApiVersion"].(string)
	components, _ := obj["Components"].([]any)
	if apiVersion == "" {
		return false
	}
	for _, c := range components {
		comp, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if name, _ := comp["Name"].(string); name == "Engine" {
			return true
		}
	}
	return false
}
