package tlsprobe

import (
	"regexp"
	"sort"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const traefikType = "traefik"

var (
	// hostMatcher finds a Host(...) matcher and captures its argument list.
	hostMatcher = regexp.MustCompile(`Host\(([^)]*)\)`)
	// hostLiteral finds the backtick or double quoted literals in an argument list.
	hostLiteral = regexp.MustCompile("`([^`]*)`|\"([^\"]*)\"")
)

// hostsFromRule returns the literal host names of the Host matchers in a
// Traefik rule, lower-cased. It is a pattern match over the rule text, not a
// rule parser: HostRegexp, HostSNI and every other matcher contribute nothing,
// nor does a negated Host matcher or a name that is not a plain host name
// (a wildcard or a {name:regexp} template).
func hostsFromRule(rule string) []string {
	var hosts []string
	for _, m := range hostMatcher.FindAllStringSubmatchIndex(rule, -1) {
		if !standaloneMatcher(rule, m[0]) {
			continue
		}
		for _, lit := range hostLiteral.FindAllStringSubmatch(rule[m[2]:m[3]], -1) {
			host, err := normalizeHost(lit[1] + lit[2])
			if err != nil {
				continue
			}
			hosts = append(hosts, host)
		}
	}
	return hosts
}

// standaloneMatcher reports whether the Host matcher starting at start is a
// whole, non-negated matcher: not the tail of a longer name and not preceded
// by "!".
func standaloneMatcher(rule string, start int) bool {
	if start > 0 {
		prev := rule[start-1]
		if prev == '_' || (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') || (prev >= '0' && prev <= '9') {
			return false
		}
	}
	return !strings.HasSuffix(strings.TrimRight(rule[:start], " \t"), "!")
}

// importedHosts returns the sorted, de-duplicated hosts of the TLS routers in
// the referenced Traefik connector's latest snapshot. A missing connector, a
// snapshot that was never taken or one of another type yields none.
func importedHosts(config map[string]any) []string {
	id := importConnectorID(config)
	if id == "" {
		return nil
	}
	snapshot := connector.RelatedSnapshots(config)[id]
	if snapshot == nil || snapshot.Type != traefikType {
		return nil
	}
	seen := map[string]bool{}
	var hosts []string
	for _, entity := range snapshot.Entities {
		if entity.Kind != "router" {
			continue
		}
		if tls, _ := entity.Attributes["tls"].(bool); !tls {
			continue
		}
		rule, _ := entity.Attributes["rule"].(string)
		for _, host := range hostsFromRule(rule) {
			if !seen[host] {
				seen[host] = true
				hosts = append(hosts, host)
			}
		}
	}
	sort.Strings(hosts)
	return hosts
}

// withImported adds the imported hosts, on the import port, to the listed
// targets. A host already listed on that port is probed once, as listed. When
// the total would pass maxTargets the listed targets are kept and the imported
// hosts are taken alphabetically; leftOut counts those dropped.
func withImported(listed []target, hosts []string, port int) (all []target, leftOut int) {
	all = append([]target(nil), listed...)
	taken := make(map[string]bool, len(listed))
	for _, t := range listed {
		taken[t.id()] = true
	}
	for _, host := range hosts {
		t := target{host: host, port: port, imported: true}
		if taken[t.id()] {
			continue
		}
		if len(all) >= maxTargets {
			leftOut++
			continue
		}
		taken[t.id()] = true
		all = append(all, t)
	}
	return all, leftOut
}
