package caddy

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/snapshotutil"
)

var attributeCatalog = map[string][]connector.AttributeSpec{
	"http_server": {
		{Name: "listen", Type: "string_array", Description: "Addresses on which Caddy listens"},
		{Name: "route_count", Type: "number", Description: "Number of routes configured on this server"},
	},
	"http_route": {
		{Name: "hosts", Type: "string_array", Description: "Host matchers in configured order"},
		{Name: "upstream", Type: "string", Description: "Configured reverse proxy upstream dial target"},
	},
}

func buildTables(cfg caddyConfig) *parsedConfig {
	out := &parsedConfig{}
	serverNames := make([]string, 0, len(cfg.Apps.HTTP.Servers))
	for name := range cfg.Apps.HTTP.Servers {
		serverNames = append(serverNames, name)
	}
	sort.Strings(serverNames)
	for _, serverName := range serverNames {
		server := cfg.Apps.HTTP.Servers[serverName]
		listen := sortedUnique(server.Listen)
		out.servers = append(out.servers, serverRecord{name: serverName, listen: listen})
		out.entities = append(out.entities, connector.SnapshotEntity{
			Kind: "http_server", Name: serverName, ExternalID: serverName,
			Attributes: map[string]any{"listen": listen, "route_count": len(server.Routes)},
		})
		for routeIndex, route := range server.Routes {
			hosts := routeHosts(route.Match)
			upstreamTarget := routeUpstream(route.Handle)
			name, aliases := hostIdentity(hosts)
			if name == "" {
				name = fmt.Sprintf("%s route %d", serverName, routeIndex+1)
			}
			row := routeRecord{server: serverName, name: name, hosts: hosts, upstream: upstreamTarget}
			if host, port, err := net.SplitHostPort(upstreamTarget); err == nil {
				row.upstream, row.port = host, port
			} else {
				row.upstream = upstreamTarget
			}
			out.routes = append(out.routes, row)
			entity := connector.SnapshotEntity{Kind: "http_route", Name: name, Hostname: name, Aliases: aliases, ExternalID: fmt.Sprintf("%s:%d", serverName, routeIndex), Attributes: map[string]any{}}
			if len(hosts) == 0 {
				entity.Hostname = ""
			}
			snapshotutil.PutStrings(entity.Attributes, "hosts", hosts)
			snapshotutil.PutString(entity.Attributes, "upstream", upstreamTarget)
			if ip := net.ParseIP(strings.Trim(row.upstream, "[]")); ip != nil {
				if !ip.IsLoopback() && !ip.IsUnspecified() {
					entity.IP = ip.String()
				}
			} else if row.upstream != "" {
				out.deps = append(out.deps, connector.ServiceDependency{Kind: "upstream_service", Name: row.upstream})
			}
			out.entities = append(out.entities, entity)
		}
	}
	for _, policy := range cfg.Apps.TLS.Automation.Policies {
		out.subjects = append(out.subjects, policy.Subjects...)
	}
	out.subjects = sortedUnique(out.subjects)
	sort.Slice(out.routes, func(i, j int) bool {
		if out.routes[i].server != out.routes[j].server {
			return out.routes[i].server < out.routes[j].server
		}
		return out.routes[i].name < out.routes[j].name
	})
	out.deps = uniqueDependencies(out.deps)
	return out
}

func routeHosts(matches []routeMatch) []string {
	for _, matcher := range matches {
		if len(matcher.Host) > 0 {
			return matcher.Host
		}
	}
	return nil
}

func routeUpstream(handlers []handler) string {
	for _, h := range handlers {
		if h.Handler == "reverse_proxy" && len(h.Upstreams) > 0 {
			return h.Upstreams[0].Dial
		}
	}
	return ""
}

func tables(parsed *parsedConfig) (string, string, string) {
	serverRows := make([][]string, 0, len(parsed.servers))
	for _, server := range parsed.servers {
		serverRows = append(serverRows, []string{server.name, strings.Join(server.listen, ", "), fmt.Sprint(lenRoutes(parsed.routes, server.name))})
	}
	routeRows := make([][]string, 0, len(parsed.routes))
	for _, route := range parsed.routes {
		routeRows = append(routeRows, []string{route.server, route.name, strings.Join(route.hosts, ", "), route.upstream, route.port})
	}
	tlsRows := make([][]string, 0, len(parsed.subjects))
	for _, subject := range parsed.subjects {
		tlsRows = append(tlsRows, []string{subject})
	}
	return resourceTable("HTTP servers", []string{"Server", "Listen", "Routes"}, serverRows),
		resourceTable("HTTP routes", []string{"Server", "Hostname", "Hosts", "Upstream", "Port"}, routeRows),
		resourceTable("TLS automation subjects", []string{"Subject"}, tlsRows)
}

func lenRoutes(routes []routeRecord, server string) int {
	n := 0
	for _, route := range routes {
		if route.server == server {
			n++
		}
	}
	return n
}

func resourceTable(noun string, headers []string, rows [][]string) string {
	if len(rows) == 0 {
		return snapshotutil.Empty(noun)
	}
	var b strings.Builder
	b.WriteString("| " + strings.Join(headers, " | ") + " |\n")
	separators := make([]string, len(headers))
	for i := range separators {
		separators[i] = strings.Repeat("-", len(headers[i]))
	}
	b.WriteString("| " + strings.Join(separators, " | ") + " |\n")
	for _, row := range rows {
		for i := range row {
			row[i] = snapshotutil.MDCell(row[i])
		}
		b.WriteString("| " + strings.Join(row, " | ") + " |\n")
	}
	return b.String()
}

func hostIdentity(hosts []string) (string, []string) {
	if len(hosts) == 0 {
		return "", nil
	}
	aliases := sortedUnique(hosts[1:])
	return hosts[0], aliases
}

func sortedUnique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func uniqueDependencies(deps []connector.ServiceDependency) []connector.ServiceDependency {
	seen := make(map[string]struct{}, len(deps))
	out := make([]connector.ServiceDependency, 0, len(deps))
	for _, dep := range deps {
		key, _ := json.Marshal(dep)
		if _, ok := seen[string(key)]; ok {
			continue
		}
		seen[string(key)] = struct{}{}
		out = append(out, dep)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
