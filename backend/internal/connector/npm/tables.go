package npm

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/snapshotutil"
)

var attributeCatalog = map[string][]connector.AttributeSpec{
	"proxy_host": {
		{Name: "domain_names", Type: "string_array", Description: "Configured hostnames, sorted for stable snapshots"},
		{Name: "forward_host", Type: "string", Description: "Upstream host configured for the proxy"},
		{Name: "forward_port", Type: "number", Description: "Upstream port configured for the proxy"},
		{Name: "forward_scheme", Type: "string", Description: "Upstream scheme configured for the proxy"},
		{Name: "access_list_id", Type: "number", Description: "Numeric access list ID attached to the proxy"},
		{Name: "certificate_id", Type: "number", Description: "Numeric certificate ID attached to the proxy"},
		{Name: "ssl_forced", Type: "boolean", Description: "Whether HTTPS is forced for the proxy"},
		{Name: "enabled", Type: "boolean", Description: "Whether the proxy host is enabled"},
	},
	"redirection_host": {
		{Name: "domain_names", Type: "string_array", Description: "Configured hostnames, sorted for stable snapshots"},
		{Name: "forward_http_code", Type: "number", Description: "HTTP status code used for the redirect"},
		{Name: "forward_scheme", Type: "string", Description: "Scheme used for the redirect target"},
		{Name: "forward_domain_name", Type: "string", Description: "Domain used as the redirect target"},
		{Name: "preserve_path", Type: "boolean", Description: "Whether the request path is preserved"},
		{Name: "certificate_id", Type: "number", Description: "Numeric certificate ID attached to the redirect"},
		{Name: "ssl_forced", Type: "boolean", Description: "Whether HTTPS is forced for the redirect"},
		{Name: "enabled", Type: "boolean", Description: "Whether the redirect host is enabled"},
	},
	"stream": {
		{Name: "incoming_port", Type: "number", Description: "Port exposed by the stream"},
		{Name: "forwarding_host", Type: "string", Description: "Configured upstream host for the stream"},
		{Name: "forwarding_port", Type: "number", Description: "Configured upstream port for the stream"},
		{Name: "protocol", Type: "string", Description: "Forwarding protocol enabled for the stream"},
		{Name: "tcp_forwarding", Type: "boolean", Description: "Whether TCP forwarding is enabled"},
		{Name: "udp_forwarding", Type: "boolean", Description: "Whether UDP forwarding is enabled"},
		{Name: "certificate_id", Type: "number", Description: "Numeric certificate ID attached to the stream"},
		{Name: "enabled", Type: "boolean", Description: "Whether the stream is enabled"},
	},
	"dead_host": {
		{Name: "domain_names", Type: "string_array", Description: "Configured hostnames, sorted for stable snapshots"},
		{Name: "certificate_id", Type: "number", Description: "Numeric certificate ID attached to the 404 host"},
		{Name: "ssl_forced", Type: "boolean", Description: "Whether HTTPS is forced for the 404 host"},
		{Name: "enabled", Type: "boolean", Description: "Whether the 404 host is enabled"},
	},
	"certificate": {
		{Name: "provider", Type: "string", Description: "Certificate provider reported by NPM"},
		{Name: "domain_names", Type: "string_array", Description: "Certificate domains, sorted for stable snapshots"},
		{Name: "expires_on", Type: "string", Description: "Certificate expiration timestamp as reported by NPM"},
		{Name: "not_after", Type: "string", Description: "Certificate expiration timestamp normalized to UTC"},
	},
	"access_list": {
		{Name: "satisfy_any", Type: "boolean", Description: "Whether any access list rule may match"},
		{Name: "pass_auth", Type: "boolean", Description: "Whether authentication headers pass to the upstream"},
		{Name: "proxy_host_count", Type: "number", Description: "Number of proxy hosts using this access list"},
	},
}

func decodeResources(raw []byte, dst any) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return connector.NewMalformedResponseError(errors.New("expected a JSON array"))
	}
	if err := json.Unmarshal(trimmed, dst); err != nil {
		return connector.NewMalformedResponseError(errors.New("invalid JSON resource array"))
	}
	return nil
}

func forwardIP(host string) net.IP { return net.ParseIP(strings.Trim(strings.TrimSpace(host), "[]")) }

func resourceTable(noun string, headers []string, rows [][]string) string {
	if len(rows) == 0 {
		return snapshotutil.Empty(noun)
	}
	var b strings.Builder
	b.WriteString("| " + strings.Join(headers, " | ") + " |\n")
	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = strings.Repeat("-", len(headers[i]))
	}
	b.WriteString("| " + strings.Join(seps, " | ") + " |\n")
	for _, row := range rows {
		for i := range row {
			row[i] = snapshotutil.MDCell(row[i])
		}
		b.WriteString("| " + strings.Join(row, " | ") + " |\n")
	}
	return b.String()
}

func sortedUnique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; !ok {
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func hostIdentity(domains []string) (string, []string) {
	if len(domains) == 0 {
		return "", nil
	}
	primary := domains[0]
	rest := make([]string, 0, len(domains)-1)
	for _, domain := range domains[1:] {
		if domain != primary {
			rest = append(rest, domain)
		}
	}
	if len(rest) == 0 {
		return primary, nil
	}
	return primary, sortedUnique(rest)
}

func numericID(id int) string { return strconv.Itoa(id) }

func normalizeExpiry(expiresOn string) (string, bool) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02"} {
		expiresAt, err := time.Parse(layout, expiresOn)
		if err == nil {
			return expiresAt.UTC().Truncate(time.Second).Format(time.RFC3339), true
		}
	}
	return "", false
}

func buildProxyHostTable(raw []byte) (string, []connector.SnapshotEntity, []connector.ServiceDependency, error) {
	var items []proxyHost
	if err := decodeResources(raw, &items); err != nil {
		return "", nil, nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	rows := make([][]string, 0, len(items))
	entities := make([]connector.SnapshotEntity, 0, len(items))
	deps := []connector.ServiceDependency{}
	for _, p := range items {
		name, aliases := hostIdentity(p.DomainNames)
		if name == "" {
			name = numericID(p.ID)
		}
		rows = append(rows, []string{displayDomains(name, aliases), p.ForwardHost, strconv.Itoa(p.ForwardPort), snapshotutil.YesNo(bool(p.Enabled))})
		attrs := map[string]any{"forward_port": p.ForwardPort, "access_list_id": p.AccessListID, "certificate_id": p.CertificateID, "ssl_forced": bool(p.SSLForced), "enabled": bool(p.Enabled)}
		snapshotutil.PutString(attrs, "forward_host", p.ForwardHost)
		snapshotutil.PutString(attrs, "forward_scheme", p.ForwardScheme)
		snapshotutil.PutStrings(attrs, "domain_names", sortedUnique(p.DomainNames))
		hostname := name
		if len(p.DomainNames) == 0 {
			hostname = ""
		}
		entity := connector.SnapshotEntity{Kind: "proxy_host", Name: name, Hostname: hostname, Aliases: aliases, ExternalID: numericID(p.ID), Attributes: attrs}
		if ip := forwardIP(p.ForwardHost); ip != nil {
			if !ip.IsLoopback() && !ip.IsUnspecified() {
				entity.IP = ip.String()
			}
		} else if p.ForwardHost != "" {
			deps = append(deps, connector.ServiceDependency{Kind: "upstream_service", Name: p.ForwardHost})
		}
		entities = append(entities, entity)
	}
	return resourceTable("proxy hosts", []string{"Host", "Forward host", "Forward port", "Enabled"}, rows), entities, uniqueDependencies(deps), nil
}

func buildRedirectionHostTable(raw []byte) (string, []connector.SnapshotEntity, []connector.ServiceDependency, error) {
	var items []redirectionHost
	if err := decodeResources(raw, &items); err != nil {
		return "", nil, nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	rows := make([][]string, 0, len(items))
	entities := make([]connector.SnapshotEntity, 0, len(items))
	for _, h := range items {
		name, aliases := hostIdentity(h.DomainNames)
		if name == "" {
			name = numericID(h.ID)
		}
		target := h.ForwardDomainName
		if h.ForwardScheme != "auto" {
			target = h.ForwardScheme + "://" + target
		}
		rows = append(rows, []string{displayDomains(name, aliases), target, strconv.Itoa(h.ForwardHTTPCode), snapshotutil.YesNo(bool(h.Enabled))})
		attrs := map[string]any{"forward_http_code": h.ForwardHTTPCode, "preserve_path": bool(h.PreservePath), "certificate_id": h.CertificateID, "ssl_forced": bool(h.SSLForced), "enabled": bool(h.Enabled)}
		snapshotutil.PutString(attrs, "forward_scheme", h.ForwardScheme)
		snapshotutil.PutString(attrs, "forward_domain_name", h.ForwardDomainName)
		snapshotutil.PutStrings(attrs, "domain_names", sortedUnique(h.DomainNames))
		hostname := name
		if len(h.DomainNames) == 0 {
			hostname = ""
		}
		entities = append(entities, connector.SnapshotEntity{Kind: "redirection_host", Name: name, Hostname: hostname, Aliases: aliases, ExternalID: numericID(h.ID), Attributes: attrs})
	}
	return resourceTable("redirection hosts", []string{"Host", "Target", "HTTP code", "Enabled"}, rows), entities, nil, nil
}

func buildStreamTable(raw []byte) (string, []connector.SnapshotEntity, []connector.ServiceDependency, error) {
	var items []stream
	if err := decodeResources(raw, &items); err != nil {
		return "", nil, nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	rows := make([][]string, 0, len(items))
	entities := make([]connector.SnapshotEntity, 0, len(items))
	deps := []connector.ServiceDependency{}
	for _, s := range items {
		name := "stream :" + strconv.Itoa(s.IncomingPort)
		protocol := streamProtocol(bool(s.TCPForwarding), bool(s.UDPForwarding))
		rows = append(rows, []string{strconv.Itoa(s.IncomingPort), protocol, s.ForwardingHost, strconv.Itoa(s.ForwardingPort), snapshotutil.YesNo(bool(s.Enabled))})
		attrs := map[string]any{"incoming_port": s.IncomingPort, "forwarding_port": s.ForwardingPort, "tcp_forwarding": bool(s.TCPForwarding), "udp_forwarding": bool(s.UDPForwarding), "certificate_id": s.CertificateID, "enabled": bool(s.Enabled), "protocol": protocol}
		snapshotutil.PutString(attrs, "forwarding_host", s.ForwardingHost)
		entity := connector.SnapshotEntity{Kind: "stream", Name: name, ExternalID: numericID(s.ID), Attributes: attrs}
		if ip := forwardIP(s.ForwardingHost); ip != nil {
			if !ip.IsLoopback() && !ip.IsUnspecified() {
				entity.IP = ip.String()
			}
		} else if s.ForwardingHost != "" {
			deps = append(deps, connector.ServiceDependency{Kind: "upstream_service", Name: s.ForwardingHost})
		}
		entities = append(entities, entity)
	}
	return resourceTable("streams", []string{"Incoming port", "Protocol", "Forward host", "Forward port", "Enabled"}, rows), entities, uniqueDependencies(deps), nil
}

func buildDeadHostTable(raw []byte) (string, []connector.SnapshotEntity, []connector.ServiceDependency, error) {
	var items []deadHost
	if err := decodeResources(raw, &items); err != nil {
		return "", nil, nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	rows := make([][]string, 0, len(items))
	entities := make([]connector.SnapshotEntity, 0, len(items))
	for _, h := range items {
		name, aliases := hostIdentity(h.DomainNames)
		if name == "" {
			name = numericID(h.ID)
		}
		rows = append(rows, []string{displayDomains(name, aliases), snapshotutil.YesNo(bool(h.Enabled)), snapshotutil.YesNo(bool(h.SSLForced))})
		attrs := map[string]any{"certificate_id": h.CertificateID, "ssl_forced": bool(h.SSLForced), "enabled": bool(h.Enabled)}
		snapshotutil.PutStrings(attrs, "domain_names", sortedUnique(h.DomainNames))
		hostname := name
		if len(h.DomainNames) == 0 {
			hostname = ""
		}
		entities = append(entities, connector.SnapshotEntity{Kind: "dead_host", Name: name, Hostname: hostname, Aliases: aliases, ExternalID: numericID(h.ID), Attributes: attrs})
	}
	return resourceTable("404 hosts", []string{"Host", "Enabled", "SSL forced"}, rows), entities, nil, nil
}

func buildCertificateTable(raw []byte) (string, []connector.SnapshotEntity, []connector.ServiceDependency, error) {
	var items []certificate
	if err := decodeResources(raw, &items); err != nil {
		return "", nil, nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	rows := make([][]string, 0, len(items))
	entities := make([]connector.SnapshotEntity, 0, len(items))
	for _, cert := range items {
		name := cert.NiceName
		if name == "" {
			name = strings.Join(sortedUnique(cert.DomainNames), ", ")
		}
		if name == "" {
			name = cert.Provider
		}
		if name == "" {
			name = numericID(cert.ID)
		}
		rows = append(rows, []string{name, cert.Provider, strings.Join(sortedUnique(cert.DomainNames), ", "), cert.ExpiresOn})
		attrs := map[string]any{}
		snapshotutil.PutString(attrs, "provider", cert.Provider)
		snapshotutil.PutStrings(attrs, "domain_names", sortedUnique(cert.DomainNames))
		snapshotutil.PutString(attrs, "expires_on", cert.ExpiresOn)
		if notAfter, ok := normalizeExpiry(cert.ExpiresOn); ok {
			attrs["not_after"] = notAfter
		}
		entities = append(entities, connector.SnapshotEntity{Kind: "certificate", Name: name, ExternalID: numericID(cert.ID), Attributes: attrs})
	}
	return resourceTable("certificates", []string{"Certificate", "Provider", "Domains", "Expires on"}, rows), entities, nil, nil
}

func buildAccessListTable(raw []byte) (string, []connector.SnapshotEntity, []connector.ServiceDependency, error) {
	var items []accessList
	if err := decodeResources(raw, &items); err != nil {
		return "", nil, nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	rows := make([][]string, 0, len(items))
	entities := make([]connector.SnapshotEntity, 0, len(items))
	for _, a := range items {
		name := a.Name
		if name == "" {
			name = numericID(a.ID)
		}
		rows = append(rows, []string{name, snapshotutil.YesNo(bool(a.SatisfyAny)), snapshotutil.YesNo(bool(a.PassAuth)), strconv.Itoa(a.ProxyHostCount)})
		attrs := map[string]any{"satisfy_any": bool(a.SatisfyAny), "pass_auth": bool(a.PassAuth), "proxy_host_count": a.ProxyHostCount}
		entities = append(entities, connector.SnapshotEntity{Kind: "access_list", Name: name, ExternalID: numericID(a.ID), Attributes: attrs})
	}
	return resourceTable("access lists", []string{"Access list", "Satisfy any", "Pass auth", "Proxy hosts"}, rows), entities, nil, nil
}

func streamProtocol(tcp, udp bool) string {
	switch {
	case tcp && udp:
		return "tcp+udp"
	case tcp:
		return "tcp"
	case udp:
		return "udp"
	default:
		return ""
	}
}

func uniqueDependencies(deps []connector.ServiceDependency) []connector.ServiceDependency {
	if len(deps) == 0 {
		return nil
	}
	seen := make(map[connector.ServiceDependency]struct{}, len(deps))
	out := make([]connector.ServiceDependency, 0, len(deps))
	for _, dep := range deps {
		if _, ok := seen[dep]; !ok {
			seen[dep] = struct{}{}
			out = append(out, dep)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Ref < out[j].Ref
	})
	return out
}

func displayDomains(primary string, aliases []string) string {
	if len(aliases) == 0 {
		return primary
	}
	return strings.Join(append([]string{primary}, aliases...), ", ")
}
