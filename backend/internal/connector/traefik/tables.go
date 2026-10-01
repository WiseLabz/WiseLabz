package traefik

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/snapshotutil"
)

// meta keys a middleware object carries besides its actual configuration
// block; everything else is a middleware kind (basicAuth, headers, ...).
var middlewareMetaKeys = map[string]struct{}{
	"status": {}, "usedBy": {}, "name": {}, "provider": {}, "type": {}, "error": {},
}

// buildOverview renders /api/overview and returns count metadata alongside.
func buildOverview(raw []byte) (string, map[string]string) {
	type counts struct {
		Total    int `json:"total"`
		Warnings int `json:"warnings"`
		Errors   int `json:"errors"`
	}
	type section struct {
		Routers     counts `json:"routers"`
		Services    counts `json:"services"`
		Middlewares counts `json:"middlewares"`
	}
	var overview struct {
		HTTP      section  `json:"http"`
		TCP       section  `json:"tcp"`
		UDP       section  `json:"udp"`
		Providers []string `json:"providers"`
		Features  struct {
			Tracing   string `json:"tracing"`
			Metrics   string `json:"metrics"`
			AccessLog bool   `json:"accessLog"`
		} `json:"features"`
	}
	if err := json.Unmarshal(raw, &overview); err != nil {
		return snapshotutil.MalformedSection("Overview", err), nil
	}

	var b strings.Builder
	b.WriteString("| Protocol | Routers | Services | Middlewares | Errors |\n")
	b.WriteString("|----------|---------|----------|-------------|--------|\n")
	rows := []struct {
		name string
		s    section
	}{{"HTTP", overview.HTTP}, {"TCP", overview.TCP}, {"UDP", overview.UDP}}
	for _, r := range rows {
		errs := r.s.Routers.Errors + r.s.Services.Errors + r.s.Middlewares.Errors
		_, _ = fmt.Fprintf(&b, "| %s | %d | %d | %d | %d |\n",
			r.name, r.s.Routers.Total, r.s.Services.Total, r.s.Middlewares.Total, errs)
	}
	if len(overview.Providers) > 0 {
		_, _ = fmt.Fprintf(&b, "\nProviders: %s\n", snapshotutil.MDCell(strings.Join(overview.Providers, ", ")))
	}
	_, _ = fmt.Fprintf(&b, "\nAccess log: %t", overview.Features.AccessLog)
	if overview.Features.Metrics != "" {
		_, _ = fmt.Fprintf(&b, " · Metrics: %s", snapshotutil.MDCell(overview.Features.Metrics))
	}
	if overview.Features.Tracing != "" {
		_, _ = fmt.Fprintf(&b, " · Tracing: %s", snapshotutil.MDCell(overview.Features.Tracing))
	}
	b.WriteString("\n")

	metadata := map[string]string{
		"http_routers_total":     fmt.Sprintf("%d", overview.HTTP.Routers.Total),
		"http_services_total":    fmt.Sprintf("%d", overview.HTTP.Services.Total),
		"http_middlewares_total": fmt.Sprintf("%d", overview.HTTP.Middlewares.Total),
		"tcp_routers_total":      fmt.Sprintf("%d", overview.TCP.Routers.Total),
		"udp_routers_total":      fmt.Sprintf("%d", overview.UDP.Routers.Total),
	}
	return b.String(), metadata
}

// buildRouterTable renders /api/http/routers. It returns the section
// content, one "router" entity per router, and the service names the
// routers forward to (for snapshot dependencies).
func buildRouterTable(raw []byte) (string, []connector.SnapshotEntity, []string) {
	var routers []struct {
		Name        string   `json:"name"`
		Rule        string   `json:"rule"`
		Service     string   `json:"service"`
		Status      string   `json:"status"`
		Provider    string   `json:"provider"`
		EntryPoints []string `json:"entryPoints"`
		Middlewares []string `json:"middlewares"`
		TLS         *struct {
			CertResolver string `json:"certResolver"`
		} `json:"tls"`
	}
	if err := json.Unmarshal(raw, &routers); err != nil {
		return snapshotutil.MalformedSection("HTTP Routers", err), nil, nil
	}
	if len(routers) == 0 {
		return "_No routers returned_", nil, nil
	}

	var b strings.Builder
	b.WriteString("| Router | Rule | Service | Entry Points | Middlewares | TLS | Status |\n")
	b.WriteString("|--------|------|---------|--------------|-------------|-----|--------|\n")
	entities := make([]connector.SnapshotEntity, 0, len(routers))
	services := make([]string, 0, len(routers))
	for _, r := range routers {
		tlsDesc := "no"
		if r.TLS != nil {
			tlsDesc = "yes"
			if r.TLS.CertResolver != "" {
				tlsDesc = "yes (" + r.TLS.CertResolver + ")"
			}
		}
		_, _ = fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s |\n",
			snapshotutil.MDCell(r.Name), snapshotutil.MDCell(r.Rule), snapshotutil.MDCell(r.Service),
			snapshotutil.MDCell(strings.Join(r.EntryPoints, ", ")), snapshotutil.MDCell(strings.Join(r.Middlewares, ", ")),
			tlsDesc, snapshotutil.MDCell(r.Status))

		attrs := map[string]any{"tls": r.TLS != nil}
		snapshotutil.PutString(attrs, "status", r.Status)
		snapshotutil.PutString(attrs, "rule", r.Rule)
		snapshotutil.PutString(attrs, "service", r.Service)
		snapshotutil.PutString(attrs, "provider", r.Provider)
		snapshotutil.PutStrings(attrs, "entryPoints", r.EntryPoints)
		snapshotutil.PutStrings(attrs, "middlewares", r.Middlewares)
		if r.TLS != nil {
			snapshotutil.PutString(attrs, "certResolver", r.TLS.CertResolver)
		}
		entities = append(entities, connector.SnapshotEntity{
			Kind: "router", Name: r.Name, ExternalID: r.Name, Hostname: hostFromRule(r.Rule), Attributes: attrs,
		})
		services = append(services, r.Service)
	}
	return b.String(), entities, services
}

// hostFromRule extracts the first literal hostname out of a Traefik rule so
// routers can be cross-linked with DNS records and containers. Only the
// simple Host(`x`) form is matched; regexp and composite rules yield "".
func hostFromRule(rule string) string {
	const marker = "Host(`"
	i := strings.Index(rule, marker)
	if i < 0 {
		return ""
	}
	rest := rule[i+len(marker):]
	j := strings.IndexByte(rest, '`')
	if j <= 0 {
		return ""
	}
	return rest[:j]
}

// buildServiceTable renders /api/http/services.
func buildServiceTable(raw []byte) (string, []connector.SnapshotEntity) {
	var services []struct {
		Name         string            `json:"name"`
		Status       string            `json:"status"`
		Provider     string            `json:"provider"`
		ServerStatus map[string]string `json:"serverStatus"`
		LoadBalancer *struct {
			PassHostHeader *bool `json:"passHostHeader"`
			Servers        []struct {
				URL     string `json:"url"`
				Address string `json:"address"`
			} `json:"servers"`
		} `json:"loadBalancer"`
	}
	if err := json.Unmarshal(raw, &services); err != nil {
		return snapshotutil.MalformedSection("HTTP Services", err), nil
	}
	if len(services) == 0 {
		return "_No services returned_", nil
	}

	var b strings.Builder
	b.WriteString("| Service | Provider | Servers | Health | Status |\n")
	b.WriteString("|---------|----------|---------|--------|--------|\n")
	entities := make([]connector.SnapshotEntity, 0, len(services))
	for _, s := range services {
		var servers []string
		passHost := false
		if s.LoadBalancer != nil {
			for _, srv := range s.LoadBalancer.Servers {
				if srv.URL != "" {
					servers = append(servers, srv.URL)
				} else if srv.Address != "" {
					servers = append(servers, srv.Address)
				}
			}
			passHost = s.LoadBalancer.PassHostHeader == nil || *s.LoadBalancer.PassHostHeader
		}
		_, _ = fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
			snapshotutil.MDCell(s.Name), snapshotutil.MDCell(s.Provider), snapshotutil.MDCell(strings.Join(servers, "<br>")),
			snapshotutil.MDCell(healthSummary(s.ServerStatus)), snapshotutil.MDCell(s.Status))

		attrs := map[string]any{"serverCount": len(servers), "passHostHeader": passHost}
		snapshotutil.PutString(attrs, "status", s.Status)
		snapshotutil.PutString(attrs, "provider", s.Provider)
		snapshotutil.PutStrings(attrs, "servers", servers)
		entities = append(entities, connector.SnapshotEntity{
			Kind: "service", Name: s.Name, ExternalID: s.Name, Attributes: attrs,
		})
	}
	return b.String(), entities
}

// healthSummary condenses the per-backend serverStatus map into "N/M UP".
func healthSummary(status map[string]string) string {
	if len(status) == 0 {
		return ""
	}
	up := 0
	for _, v := range status {
		if strings.EqualFold(v, "UP") {
			up++
		}
	}
	return fmt.Sprintf("%d/%d UP", up, len(status))
}

// buildMiddlewareTable renders /api/http/middlewares. A middleware object
// carries its kind as an extra key (v2) or a "type" field (v3), so it is
// decoded generically.
func buildMiddlewareTable(raw []byte) (string, []connector.SnapshotEntity) {
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return snapshotutil.MalformedSection("HTTP Middlewares", err), nil
	}
	if len(items) == 0 {
		return "_No middlewares returned_", nil
	}

	var b strings.Builder
	b.WriteString("| Middleware | Type | Provider | Used By | Status |\n")
	b.WriteString("|------------|------|----------|---------|--------|\n")
	entities := make([]connector.SnapshotEntity, 0, len(items))
	for _, item := range items {
		name := rawString(item, "name")
		status := rawString(item, "status")
		provider := rawString(item, "provider")
		types := middlewareTypes(item)
		var usedBy []string
		if v, ok := item["usedBy"]; ok {
			_ = json.Unmarshal(v, &usedBy)
		}

		_, _ = fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
			snapshotutil.MDCell(name), snapshotutil.MDCell(strings.Join(types, ", ")), snapshotutil.MDCell(provider),
			snapshotutil.MDCell(strings.Join(usedBy, ", ")), snapshotutil.MDCell(status))

		attrs := map[string]any{}
		snapshotutil.PutString(attrs, "status", status)
		snapshotutil.PutString(attrs, "provider", provider)
		snapshotutil.PutStrings(attrs, "types", types)
		entities = append(entities, connector.SnapshotEntity{
			Kind: "middleware", Name: name, ExternalID: name, Attributes: attrs,
		})
	}
	return b.String(), entities
}

// middlewareTypes reports the configured middleware kinds, preferring the
// explicit "type" field (Traefik v3) and falling back to the non-meta keys
// of the object (Traefik v2).
func middlewareTypes(item map[string]json.RawMessage) []string {
	if t := rawString(item, "type"); t != "" {
		return []string{t}
	}
	var types []string
	for k := range item {
		if _, meta := middlewareMetaKeys[k]; meta {
			continue
		}
		types = append(types, k)
	}
	sort.Strings(types)
	return types
}

// buildEntryPointTable renders /api/entrypoints.
func buildEntryPointTable(raw []byte) (string, []connector.SnapshotEntity) {
	var eps []struct {
		Name      string `json:"name"`
		Address   string `json:"address"`
		AsDefault bool   `json:"asDefault"`
		HTTP      struct {
			TLS *struct {
				CertResolver string `json:"certResolver"`
			} `json:"tls"`
		} `json:"http"`
	}
	if err := json.Unmarshal(raw, &eps); err != nil {
		return snapshotutil.MalformedSection("Entry Points", err), nil
	}
	if len(eps) == 0 {
		return "_No entry points returned_", nil
	}

	var b strings.Builder
	b.WriteString("| Entry Point | Address | Default | TLS |\n")
	b.WriteString("|-------------|---------|---------|-----|\n")
	entities := make([]connector.SnapshotEntity, 0, len(eps))
	for _, e := range eps {
		tlsDesc := "no"
		if e.HTTP.TLS != nil {
			tlsDesc = "yes"
			if e.HTTP.TLS.CertResolver != "" {
				tlsDesc = "yes (" + e.HTTP.TLS.CertResolver + ")"
			}
		}
		_, _ = fmt.Fprintf(&b, "| %s | %s | %t | %s |\n", snapshotutil.MDCell(e.Name), snapshotutil.MDCell(e.Address), e.AsDefault, tlsDesc)

		attrs := map[string]any{"asDefault": e.AsDefault, "tls": e.HTTP.TLS != nil}
		snapshotutil.PutString(attrs, "address", e.Address)
		entities = append(entities, connector.SnapshotEntity{
			Kind: "entrypoint", Name: e.Name, ExternalID: e.Name, Attributes: attrs,
		})
	}
	return b.String(), entities
}

func rawString(item map[string]json.RawMessage, key string) string {
	v, ok := item[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return ""
	}
	return s
}
