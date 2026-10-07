package tlsprobe

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestHostsFromRule(t *testing.T) {
	tests := []struct {
		name string
		rule string
		want []string
	}{
		{"backtick", "Host(`a.lab`)", []string{"a.lab"}},
		{"double quotes", `Host("a.lab")`, []string{"a.lab"}},
		{"multiple arguments", "Host(`a.lab`, `b.lab`)", []string{"a.lab", "b.lab"}},
		{"or", "Host(`b.lab`) || Host(`c.lab`)", []string{"b.lab", "c.lab"}},
		{"and", "Host(`a.lab`) && PathPrefix(`/api`)", []string{"a.lab"}},
		{"and with or", "(Host(`a.lab`) || Host(`b.lab`)) && PathPrefix(`/x`)", []string{"a.lab", "b.lab"}},
		{"upper case", "Host(`App.Lab`)", []string{"app.lab"}},
		{"negated", "!Host(`a.lab`)", nil},
		{"negated with space", "! Host(`a.lab`)", nil},
		{"negated beside positive", "Host(`a.lab`) && !Host(`b.lab`)", []string{"a.lab"}},
		{"negated group", "!(Host(`a.lab`) || Host(`b.lab`))", nil},
		{"negated group beside positive", "Host(`c.lab`) && !(Host(`a.lab`) || Host(`b.lab`))", []string{"c.lab"}},
		{"negated group with spaces", "! ( Host(`a.lab`) )", nil},
		{"group then negated other matcher", "(Host(`a.lab`) || Host(`b.lab`)) && !PathPrefix(`/x`)", []string{"a.lab", "b.lab"}},
		{"regexp with group then host", "HostRegexp(`^(a|b)\\.lab$`) || Host(`c.lab`)", []string{"c.lab"}},
		{"negated group of other matcher", "!(PathPrefix(`/x`)) && Host(`a.lab`)", []string{"a.lab"}},
		{"lower case matcher", "host(`a.lab`)", []string{"a.lab"}},
		{"upper case matcher", "HOST(`a.lab`)", []string{"a.lab"}},
		{"lower case regexp", "hostregexp(`^a$`)", nil},
		{"upper case hostsni", "HOSTSNI(`a.lab`)", nil},
		{"lower case longer name", "clienthost(`a.lab`)", nil},
		{"negated lower case", "!host(`a.lab`)", nil},
		{"regexp", "HostRegexp(`^.+\\.lab$`)", nil},
		{"regexp v2 braces", "Host(`{sub:[a-z]+}.lab`)", nil},
		{"hostsni", "HostSNI(`a.lab`)", nil},
		{"hostsni regexp", "HostSNIRegexp(`a.lab`)", nil},
		{"wildcard", "Host(`*.lab`)", nil},
		{"longer name", "ClientHost(`a.lab`)", nil},
		{"other matchers only", "PathPrefix(`/`)", nil},
		{"empty literal", "Host(``)", nil},
		{"empty rule", "", nil},
		{"mixed with regexp", "HostRegexp(`^x$`) || Host(`a.lab`)", []string{"a.lab"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hostsFromRule(tt.rule); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("hostsFromRule(%q) = %v, want %v", tt.rule, got, tt.want)
			}
		})
	}
}

func router(name, rule string, tls bool) connector.SnapshotEntity {
	return connector.SnapshotEntity{Kind: "router", Name: name, ExternalID: name, Attributes: map[string]any{"rule": rule, "tls": tls}}
}

func traefikSnapshot(routers ...connector.SnapshotEntity) *connector.ServiceSnapshot {
	return &connector.ServiceSnapshot{Type: "traefik", Entities: routers}
}

func importConfig(listed []string, snap *connector.ServiceSnapshot) map[string]any {
	config := targetsConfig(listed...)
	config["import_connector_id"] = "traefik-1"
	if snap != nil {
		config["_related_snapshots"] = map[string]*connector.ServiceSnapshot{"traefik-1": snap}
	}
	return config
}

func ids(entities []connector.SnapshotEntity) []string {
	out := make([]string, 0, len(entities))
	for _, e := range entities {
		out = append(out, e.ExternalID)
	}
	return out
}

func TestImportedHostsFromTLSRoutersOnly(t *testing.T) {
	snap := traefikSnapshot(
		router("a", "Host(`a.lab`)", true),
		router("bc", "Host(`b.lab`) || Host(`c.lab`)", true),
		router("plain", "Host(`plain.lab`)", false),
		router("pattern", "HostRegexp(`^.+\\.lab$`)", true),
		router("dup", "Host(`A.lab`)", true),
	)
	got := importedHosts(importConfig(nil, snap))
	if want := []string{"a.lab", "b.lab", "c.lab"}; !reflect.DeepEqual(got, want) {
		t.Errorf("hosts = %v, want %v", got, want)
	}
}

func TestFetchProbesImportedHosts(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	srv := selfSignedServer(t)
	port := srv.port()
	// "localhost" stands in for a routed name; it resolves to the test server.
	config := importConfig(nil, traefikSnapshot(router("r", "Host(`localhost`)", true)))
	config["import_port"] = float64(port)
	snap := fetch(t, config)
	if len(snap.Entities) != 1 {
		t.Fatalf("entities = %v", ids(snap.Entities))
	}
	e := snap.Entities[0]
	if e.ExternalID != fmt.Sprintf("localhost:%d", port) || e.Attributes["source"] != "imported" || e.Attributes["reachable"] != true {
		t.Errorf("entity %+v", e)
	}
	if name := <-srv.hello; name != "localhost" {
		t.Errorf("SNI = %q", name)
	}
}

func TestImportedDuplicateOfListedTargetProbedOnce(t *testing.T) {
	config := importConfig([]string{"a.lab:443"}, traefikSnapshot(router("a", "Host(`a.lab`)", true), router("b", "Host(`b.lab`)", true)))
	all, leftOut := withImported(mustListed(t, config), importedHosts(config), 443)
	if leftOut != 0 || len(all) != 2 {
		t.Fatalf("targets = %v leftOut = %d", all, leftOut)
	}
	if all[0].id() != "a.lab:443" || all[0].imported || all[1].id() != "b.lab:443" || !all[1].imported {
		t.Errorf("targets = %+v", all)
	}
	// The same host on another port is a different target.
	all, _ = withImported(mustListed(t, config), []string{"a.lab"}, 8443)
	if len(all) != 2 {
		t.Errorf("targets = %+v", all)
	}
}

func mustListed(t *testing.T, config map[string]any) []target {
	t.Helper()
	listed, errs := listedTargets(config)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	return listed
}

func TestImportLimitKeepsListedAndTruncatesAlphabetically(t *testing.T) {
	listed := make([]string, 90)
	for i := range listed {
		listed[i] = fmt.Sprintf("listed%02d.lab:443", i)
	}
	routers := make([]connector.SnapshotEntity, 0, 30)
	for i := 29; i >= 0; i-- { // out of order on purpose
		routers = append(routers, router(fmt.Sprintf("r%d", i), fmt.Sprintf("Host(`imp%02d.lab`)", i), true))
	}
	config := importConfig(listed, traefikSnapshot(routers...))
	all, leftOut := withImported(mustListed(t, config), importedHosts(config), 443)
	if len(all) != 100 || leftOut != 20 {
		t.Fatalf("targets = %d leftOut = %d", len(all), leftOut)
	}
	var imported []string
	for _, tg := range all {
		if tg.imported {
			imported = append(imported, tg.host)
		}
	}
	if len(imported) != 10 || imported[0] != "imp00.lab" || imported[9] != "imp09.lab" {
		t.Errorf("imported = %v", imported)
	}
}

func TestFetchReportsLeftOutImports(t *testing.T) {
	listed := make([]string, 99)
	for i := range listed {
		listed[i] = fmt.Sprintf("127.0.1.%d:443", i+1) // loopback: blocked at once, no network needed
	}
	snap := traefikSnapshot(router("a", "Host(`127.0.2.1`) || Host(`127.0.2.2`) || Host(`127.0.2.3`)", true))
	got := fetch(t, importConfig(listed, snap))
	if len(got.Entities) != 100 {
		t.Fatalf("entities = %d", len(got.Entities))
	}
	if got.Metadata["import_left_out"] != "2" {
		t.Errorf("metadata = %v", got.Metadata)
	}
	var summary string
	for _, s := range got.Sections {
		if s.Title == "Import" {
			summary = s.Content
		}
	}
	if !strings.Contains(summary, "Left out because the connector is limited to 100 targets: 2.") {
		t.Errorf("import summary = %q", summary)
	}
}

func TestRemovedRouterDropsEntity(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	config := importConfig(nil, traefikSnapshot(router("a", "Host(`127.0.2.1`)", true)))
	first := fetch(t, config)
	if len(first.Entities) != 1 {
		t.Fatalf("entities = %v", ids(first.Entities))
	}
	config["_previous_snapshot"] = roundTrip(t, first)
	config["_related_snapshots"] = map[string]*connector.ServiceSnapshot{"traefik-1": traefikSnapshot()}
	if second := fetch(t, config); len(second.Entities) != 0 {
		t.Errorf("entities after router removal = %v", ids(second.Entities))
	}
	// Still listed by hand: the entity stays.
	config = importConfig([]string{"127.0.2.1:443"}, traefikSnapshot())
	if got := fetch(t, config); len(got.Entities) != 1 || got.Entities[0].Attributes["source"] != "manual" {
		t.Errorf("hand-listed entity = %+v", got.Entities)
	}
}

func TestNeverSyncedOrMissingTraefikConnector(t *testing.T) {
	// No related snapshot supplied: the connector was never synced or was deleted.
	got := fetch(t, importConfig([]string{"127.0.2.1:443"}, nil))
	if len(got.Entities) != 1 {
		t.Errorf("entities = %v", ids(got.Entities))
	}
	if _, offline := connector.SnapshotOfflineMessage(fetch(t, importConfig(nil, nil))); offline {
		t.Error("an import with nothing to import reported offline")
	}
	// A snapshot of another connector type is ignored.
	other := &connector.ServiceSnapshot{Type: "npm", Entities: []connector.SnapshotEntity{router("a", "Host(`a.lab`)", true)}}
	if hosts := importedHosts(importConfig(nil, other)); len(hosts) != 0 {
		t.Errorf("hosts from a non-Traefik snapshot = %v", hosts)
	}
}
