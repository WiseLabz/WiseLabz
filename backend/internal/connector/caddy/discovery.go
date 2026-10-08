package caddy

import (
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// discovery lets a network scan find a Caddy admin API: GET /config/ answers
// with an Etag of the form "/config/ <hash>" (the request path and a hash),
// which a generic web server does not produce.
var discovery = &connector.DiscoveryHint{
	Probes: []connector.DiscoveryProbe{
		{Port: 2019, Scheme: "http", Path: "/config/", Match: matchDiscovery},
	},
	URLTemplate: "{scheme}://{host}:{port}",
}

func matchDiscovery(r connector.DiscoveryResponse) bool {
	return r.Status == 200 && strings.HasPrefix(r.Header.Get("Etag"), `"/config/ `)
}
