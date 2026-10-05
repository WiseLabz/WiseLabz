package caddy

import (
	"crypto/sha256"
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
		{Name: "hosts", Type: "string_array", Description: "Configured host matchers, sorted and deduplicated"},
		{Name: "paths", Type: "string_array", Description: "Path matchers configured on this route"},
		{Name: "upstream", Type: "string", Description: "Configured reverse proxy upstream dial targets"},
	},
}

const maxRouteDepth = 32

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
		serverRoutes := make([]routeRecord, 0, len(server.Routes))
		for routeIndex, route := range server.Routes {
			hosts := routeHosts(route.Match)
			paths := routePaths(route.Match, route.Handle, 0)
			upstreams := routeUpstreams(route.Handle, 0)
			name, aliases := hostIdentity(hosts)
			if name == "" {
				name = fmt.Sprintf("%s route %d", serverName, routeIndex+1)
			}
			canonicalHosts := []string(nil)
			if len(hosts) > 0 {
				canonicalHosts = append([]string{name}, aliases...)
			}
			row := routeRecord{server: serverName, name: name, hosts: canonicalHosts, paths: paths, upstreams: upstreams, index: routeIndex}
			for _, dial := range upstreams {
				host, port, dependency, ip := analyzeDial(dial)
				if port != "" {
					row.ports = append(row.ports, port)
				}
				if dependency != "" {
					out.deps = append(out.deps, connector.ServiceDependency{Kind: "upstream_service", Name: dependency})
				}
				if rowIP := net.ParseIP(host); rowIP != nil && ip && !rowIP.IsLoopback() && !rowIP.IsUnspecified() {
					// The first linkable literal IP is the route's representative target.
					if row.entityIP == "" {
						row.entityIP = rowIP.String()
					}
				}
			}
			row.ports = sortedUnique(row.ports)
			serverRoutes = append(serverRoutes, row)
		}
		assignRouteIDs(serverRoutes)
		sort.SliceStable(serverRoutes, func(i, j int) bool {
			if serverRoutes[i].name != serverRoutes[j].name {
				return serverRoutes[i].name < serverRoutes[j].name
			}
			return serverRoutes[i].externalID < serverRoutes[j].externalID
		})
		for _, route := range serverRoutes {
			out.routes = append(out.routes, route)
			entity := connector.SnapshotEntity{Kind: "http_route", Name: route.name, Hostname: route.name, Aliases: aliasesFor(route.hosts), ExternalID: route.externalID, Attributes: map[string]any{}}
			if len(route.hosts) == 0 {
				entity.Hostname = ""
			}
			snapshotutil.PutStrings(entity.Attributes, "hosts", sortedUnique(route.hosts))
			snapshotutil.PutStrings(entity.Attributes, "paths", route.paths)
			snapshotutil.PutString(entity.Attributes, "upstream", strings.Join(route.upstreams, ", "))
			entity.IP = route.entityIP
			out.entities = append(out.entities, entity)
		}
	}
	for _, policy := range cfg.Apps.TLS.Automation.Policies {
		out.subjects = append(out.subjects, policy.Subjects...)
	}
	out.subjects = sortedUnique(out.subjects)
	sort.SliceStable(out.routes, func(i, j int) bool {
		if out.routes[i].server != out.routes[j].server {
			return out.routes[i].server < out.routes[j].server
		}
		if out.routes[i].name != out.routes[j].name {
			return out.routes[i].name < out.routes[j].name
		}
		return out.routes[i].externalID < out.routes[j].externalID
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

func routePaths(matches []routeMatch, handlers []handler, depth int) []string {
	var paths []string
	for _, matcher := range matches {
		paths = append(paths, matcher.Path...)
	}
	if depth <= maxRouteDepth {
		for _, h := range handlers {
			for _, nested := range h.Routes {
				paths = append(paths, routePaths(nested.Match, nested.Handle, depth+1)...)
			}
		}
	}
	return sortedUnique(paths)
}

func routeUpstreams(handlers []handler, depth int) []string {
	if depth > maxRouteDepth {
		return nil
	}
	var dials []string
	for _, h := range handlers {
		if h.Handler == "reverse_proxy" {
			for _, target := range h.Upstreams {
				if target.Dial != "" {
					dials = append(dials, target.Dial)
				}
			}
		}
		for _, route := range h.Routes {
			dials = append(dials, routeUpstreams(route.Handle, depth+1)...)
		}
	}
	return uniquePreserve(dials)
}

func analyzeDial(dial string) (host, port, dependency string, isIP bool) {
	target := strings.TrimSpace(dial)
	if target == "" || strings.HasPrefix(target, "unix/") || strings.HasPrefix(target, "unixgram/") {
		return "", "", "", false
	}
	if scheme := strings.Index(target, "://"); scheme >= 0 {
		target = target[scheme+3:]
	}
	if slash := strings.IndexByte(target, '/'); slash > 0 {
		target = target[slash+1:]
	}
	if strings.ContainsAny(target, "{}") || strings.Contains(target, "/") {
		return "", "", "", false
	}
	if parsed := net.ParseIP(strings.Trim(target, "[]")); parsed != nil {
		return parsed.String(), "", "", true
	}
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		host = strings.Trim(target, "[]")
		port = ""
	}
	if host == "" || strings.ContainsAny(host, "{}") || strings.Contains(host, "/") {
		return "", port, "", false
	}
	if parsed := net.ParseIP(host); parsed != nil {
		return parsed.String(), port, "", true
	}
	return host, port, host, false
}

func assignRouteIDs(routes []routeRecord) {
	groups := make(map[string][]int)
	for i := range routes {
		if len(routes[i].hosts) == 0 {
			routes[i].externalID = fmt.Sprintf("%s:%d", routes[i].server, routes[i].index)
			continue
		}
		identity, _ := json.Marshal(struct {
			Server string   `json:"server"`
			Hosts  []string `json:"hosts"`
			Paths  []string `json:"paths"`
		}{routes[i].server, sortedUnique(routes[i].hosts), routes[i].paths})
		baseHash := sha256.Sum256(identity)
		base := fmt.Sprintf("route:%x", baseHash)
		routes[i].externalID = base
		groups[base] = append(groups[base], i)
	}
	for base, indexes := range groups {
		if len(indexes) < 2 {
			continue
		}
		sort.Slice(indexes, func(i, j int) bool {
			return strings.Join(sortedUnique(routes[indexes[i]].upstreams), "\x00") < strings.Join(sortedUnique(routes[indexes[j]].upstreams), "\x00")
		})
		for position, index := range indexes {
			routes[index].externalID = fmt.Sprintf("%s:%d", base, position+1)
		}
	}
	for i := range routes {
		routes[i].index = 0
	}
}

func aliasesFor(hosts []string) []string {
	if len(hosts) < 2 {
		return nil
	}
	return sortedUnique(hosts[1:])
}

func uniquePreserve(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func tables(parsed *parsedConfig) (string, string, string) {
	serverRows := make([][]string, 0, len(parsed.servers))
	for _, server := range parsed.servers {
		serverRows = append(serverRows, []string{server.name, strings.Join(server.listen, ", "), fmt.Sprint(lenRoutes(parsed.routes, server.name))})
	}
	routeRows := make([][]string, 0, len(parsed.routes))
	for _, route := range parsed.routes {
		routeRows = append(routeRows, []string{route.externalID, route.server, route.name, strings.Join(route.hosts, ", "), strings.Join(route.paths, ", "), strings.Join(route.upstreams, ", "), strings.Join(route.ports, ", ")})
	}
	tlsRows := make([][]string, 0, len(parsed.subjects))
	for _, subject := range parsed.subjects {
		tlsRows = append(tlsRows, []string{subject})
	}
	return resourceTable("HTTP servers", []string{"Server", "Listen", "Routes"}, serverRows),
		resourceTable("HTTP routes", []string{"Route ID", "Server", "Hostname", "Hosts", "Paths", "Upstream", "Ports"}, routeRows),
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
	primary := hosts[0]
	var remaining []string
	for _, host := range hosts[1:] {
		if host != primary {
			remaining = append(remaining, host)
		}
	}
	aliases := sortedUnique(remaining)
	return primary, aliases
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
