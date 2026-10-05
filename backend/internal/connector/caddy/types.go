package caddy

import "github.com/WiseLabz/wiselabz/internal/connector"

type caddyConfig struct {
	Apps struct {
		HTTP struct {
			Servers map[string]httpServer `json:"servers"`
		} `json:"http"`
		TLS struct {
			Automation struct {
				Policies []struct {
					Subjects []string `json:"subjects"`
				} `json:"policies"`
			} `json:"automation"`
		} `json:"tls"`
	} `json:"apps"`
}

type httpServer struct {
	Listen []string    `json:"listen"`
	Routes []httpRoute `json:"routes"`
}

type httpRoute struct {
	Match    []routeMatch `json:"match"`
	Handle   []handler    `json:"handle"`
	Terminal bool         `json:"terminal"`
}

type routeMatch struct {
	Host []string `json:"host"`
	Path []string `json:"path"`
}

type handler struct {
	Handler   string      `json:"handler"`
	Upstreams []upstream  `json:"upstreams"`
	Routes    []httpRoute `json:"routes"`
}

type upstream struct {
	Dial string `json:"dial"`
}

type parsedConfig struct {
	servers  []serverRecord
	routes   []routeRecord
	subjects []string
	entities []connector.SnapshotEntity
	deps     []connector.ServiceDependency
}

type serverRecord struct {
	name   string
	listen []string
}

type routeRecord struct {
	server     string
	name       string
	hosts      []string
	paths      []string
	upstreams  []string
	ports      []string
	entityIP   string
	externalID string
	index      int
}
