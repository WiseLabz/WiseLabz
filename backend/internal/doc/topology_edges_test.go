package doc

import (
	"context"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func edgesOfKind(t *testing.T, s *store.Store, kind string, ids ...string) []store.TopologyEdge {
	t.Helper()
	all, err := s.ListTopologyEdges(context.Background(), ids)
	if err != nil {
		t.Fatal(err)
	}
	var out []store.TopologyEdge
	for _, e := range all {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

func rebuild(t *testing.T, e *Engine, ids ...string) {
	t.Helper()
	for _, id := range ids {
		if err := e.RebuildTopologyForConnector(context.Background(), id); err != nil {
			t.Fatalf("rebuild %s: %v", id, err)
		}
	}
}

// proxyFixture seeds one connector whose snapshot is snap and rebuilds it.
func proxyEdges(t *testing.T, snap connector.ServiceSnapshot) []store.TopologyEdge {
	t.Helper()
	s := newEngineTestStore(t)
	e := NewEngine(s)
	id := seedEngineConnectorWithEntities(t, s, "Proxy", "networking", "proxy", nil)
	addTopologySnapshot(t, s, id, snap)
	rebuild(t, e, id)
	return edgesOfKind(t, s, store.TopologyEdgeProxiesTo, id)
}

func TestProxiesToTraefikRoutersTargetServices(t *testing.T) {
	edges := proxyEdges(t, connector.ServiceSnapshot{
		ServiceName: "Traefik",
		Entities: []connector.SnapshotEntity{
			// A router named like its service must still resolve to the service.
			{Kind: "router", Name: "a@docker", ExternalID: "a@docker", Attributes: map[string]any{"service": "a@docker"}},
			{Kind: "router", Name: "no-service", ExternalID: "no-service"},
			{Kind: "service", Name: "a@docker", ExternalID: "a@docker"},
			{Kind: "service", Name: "b@docker", ExternalID: "b@docker"},
		},
		Dependencies: []connector.ServiceDependency{
			{Kind: "upstream_service", Name: "a@docker"}, {Kind: "upstream_service", Name: "b@docker"},
		},
	})
	if len(edges) != 1 {
		t.Fatalf("proxies_to edges = %+v, want exactly router->service a", edges)
	}
	e := edges[0]
	if e.SrcKind != "router" || e.DstKind != "service" || e.DstName != "a@docker" {
		t.Fatalf("edge = %+v, want router a@docker -> service a@docker", e)
	}
	if e.SrcKind == e.DstKind && e.SrcRef == e.DstRef {
		t.Fatalf("self-loop: %+v", e)
	}
}

func TestProxiesToNeverCrossProductsOrLoops(t *testing.T) {
	// Services without any upstream attribute are not sources, and entities
	// with no declared target must not fan out to every dependency.
	edges := proxyEdges(t, connector.ServiceSnapshot{
		ServiceName: "Traefik",
		Entities: []connector.SnapshotEntity{
			{Kind: "service", Name: "a@docker", ExternalID: "a@docker"},
			{Kind: "service", Name: "b@docker", ExternalID: "b@docker"},
			{Kind: "router", Name: "r", ExternalID: "r"},
			{Kind: "proxy_host", Name: "p", ExternalID: "1", Attributes: map[string]any{"forward_host": ""}},
		},
		Dependencies: []connector.ServiceDependency{
			{Kind: "upstream_service", Name: "a@docker"}, {Kind: "upstream_service", Name: "b@docker"},
		},
	})
	if len(edges) != 0 {
		t.Fatalf("proxies_to edges = %+v, want none", edges)
	}
}

func TestProxiesToNginxProxyManager(t *testing.T) {
	s := newEngineTestStore(t)
	e := NewEngine(s)
	pve := seedEngineConnectorWithEntities(t, s, "Proxmox", "virtualization", "proxmox", []connector.SnapshotEntity{
		{Kind: "vm", Name: "web-01", ExternalID: "100"},
		{Kind: "vm", Name: "web", ExternalID: "101"},
	})
	npm := seedEngineConnectorWithEntities(t, s, "NPM", "networking", "npm", nil)
	addTopologySnapshot(t, s, npm, connector.ServiceSnapshot{
		ServiceName: "NPM",
		Entities: []connector.SnapshotEntity{
			{Kind: "proxy_host", Name: "app.lan", ExternalID: "1", Attributes: map[string]any{"forward_host": "Web-01.", "forward_port": 8443}},
			{Kind: "stream", Name: "tcp-in", ExternalID: "2", Attributes: map[string]any{"forwarding_host": "web-01", "forwarding_port": 22}},
			// "web" is a prefix of "web-01": no match without an exact host.
			{Kind: "proxy_host", Name: "short.lan", ExternalID: "3", Attributes: map[string]any{"forward_host": "web"}},
			{Kind: "redirection_host", Name: "redir", ExternalID: "4", Attributes: map[string]any{"forward_domain_name": "web-01"}},
		},
		Dependencies: []connector.ServiceDependency{
			{Kind: "upstream_service", Name: "Web-01."}, {Kind: "upstream_service", Name: "web-01"}, {Kind: "upstream_service", Name: "web"},
		},
	})
	rebuild(t, e, npm)

	got := map[string]string{}
	for _, ed := range edgesOfKind(t, s, store.TopologyEdgeProxiesTo, pve, npm) {
		if ed.DstConnectorID != pve || ed.DstKind != "vm" {
			t.Errorf("edge %+v does not resolve to the proxmox vm", ed)
		}
		got[ed.SrcName+">"+ed.DstName] = ed.Detail
	}
	want := map[string]string{"app.lan>web-01": "8443", "tcp-in>web-01": "22", "short.lan>web": ""}
	if len(got) != len(want) {
		t.Fatalf("proxies_to = %v, want %v", got, want)
	}
	for k, v := range want {
		if d, ok := got[k]; !ok || d != v {
			t.Fatalf("proxies_to = %v, want %v", got, want)
		}
	}
}

func TestProxiesToCaddyRouteUpstreams(t *testing.T) {
	s := newEngineTestStore(t)
	e := NewEngine(s)
	pve := seedEngineConnectorWithEntities(t, s, "Proxmox", "virtualization", "proxmox", []connector.SnapshotEntity{
		{Kind: "vm", Name: "app", Hostname: "app.lan", ExternalID: "100"},
	})
	caddy := seedEngineConnectorWithEntities(t, s, "Caddy", "networking", "caddy", nil)
	addTopologySnapshot(t, s, caddy, connector.ServiceSnapshot{
		ServiceName: "Caddy",
		Entities: []connector.SnapshotEntity{
			// The route's own hostname equals its upstream host: it must not
			// resolve to itself, only to the VM.
			{Kind: "http_route", Name: "app.lan", Hostname: "app.lan", ExternalID: "route:1", Attributes: map[string]any{"upstream": "app.lan:8080, other:9000"}},
			{Kind: "http_route", Name: "bare", ExternalID: "route:2", Attributes: map[string]any{"upstream": "app:80"}},
			{Kind: "http_server", Name: "srv0", ExternalID: "srv0"},
		},
		Dependencies: []connector.ServiceDependency{
			{Kind: "upstream_service", Name: "app.lan"}, {Kind: "upstream_service", Name: "other"},
		},
	})
	rebuild(t, e, caddy)

	type res struct{ dstConn, dstKind, dstName, detail string }
	got := map[string]res{}
	for _, ed := range edgesOfKind(t, s, store.TopologyEdgeProxiesTo, pve, caddy) {
		if ed.SrcConnectorID == ed.DstConnectorID && ed.SrcKind == ed.DstKind && ed.SrcRef == ed.DstRef {
			t.Fatalf("self-loop %+v", ed)
		}
		got[ed.SrcRef+">"+ed.DstName] = res{ed.DstConnectorID, ed.DstKind, ed.DstName, ed.Detail}
	}
	if len(got) != 2 {
		t.Fatalf("proxies_to = %+v, want 2 edges", got)
	}
	if r := got["route:1>app"]; r != (res{pve, "vm", "app", "8080"}) {
		t.Fatalf("route:1 -> app = %+v in %+v", r, got)
	}
	if r := got["route:1>other"]; r != (res{caddy, "upstream_service", "other", "9000"}) {
		t.Fatalf("route:1 -> other = %+v in %+v", r, got)
	}
	// "app" is not an exact match for dependency "app.lan": no edge for route:2.
	for k := range got {
		if strings.HasPrefix(k, "route:2") {
			t.Fatalf("route:2 matched loosely: %+v", got)
		}
	}
}

func dnsFixture(t *testing.T) (s *store.Store, e *Engine, dns, vm string) {
	t.Helper()
	s = newEngineTestStore(t)
	e = NewEngine(s)
	dns = seedEngineConnectorWithEntities(t, s, "Pihole", "networking", "pihole", []connector.SnapshotEntity{
		{Kind: "dns_record", Name: "app.lan", IP: "10.0.0.5"},
	})
	vm = seedEngineConnectorWithEntities(t, s, "Proxmox", "virtualization", "proxmox", []connector.SnapshotEntity{
		{Kind: "vm", Name: "web-01", ExternalID: "100", IP: "10.0.0.5"},
	})
	return s, e, dns, vm
}

func TestResolvesToHasSingleOwnerAcrossRebuilds(t *testing.T) {
	s, e, dns, vm := dnsFixture(t)
	rebuild(t, e, dns, vm, dns, vm)

	edges := edgesOfKind(t, s, store.TopologyEdgeResolvesTo, dns, vm)
	if len(edges) != 1 {
		t.Fatalf("resolves_to rows = %+v, want exactly one", edges)
	}
	ed := edges[0]
	if ed.ConnectorID != dns || ed.SrcConnectorID != dns || ed.SrcName != "app.lan" || ed.DstConnectorID != vm || ed.DstName != "web-01" {
		t.Fatalf("edge = %+v, want dns-owned app.lan -> web-01", ed)
	}

	// Rebuilding only the target connector keeps the same single edge.
	rebuild(t, e, vm)
	if edges := edgesOfKind(t, s, store.TopologyEdgeResolvesTo, dns, vm); len(edges) != 1 || edges[0].ConnectorID != dns {
		t.Fatalf("after target-only rebuild: %+v", edges)
	}
}

func TestResolvesToRemovedWhenDNSRecordDisappears(t *testing.T) {
	s, e, dns, vm := dnsFixture(t)
	rebuild(t, e, dns, vm)
	addTopologySnapshot(t, s, dns, connector.ServiceSnapshot{ServiceName: "Pihole"})
	rebuild(t, e, dns)
	if edges := edgesOfKind(t, s, store.TopologyEdgeResolvesTo, dns, vm); len(edges) != 0 {
		t.Fatalf("stale resolves_to after record removal: %+v", edges)
	}
	// The target connector rebuilding afterwards must not resurrect it.
	rebuild(t, e, vm)
	if edges := edgesOfKind(t, s, store.TopologyEdgeResolvesTo, dns, vm); len(edges) != 0 {
		t.Fatalf("target rebuild resurrected resolves_to: %+v", edges)
	}
}

func TestResolvesToFollowsTargetChangesWithoutDNSSync(t *testing.T) {
	s, e, dns, vm := dnsFixture(t)
	rebuild(t, e, dns, vm)

	// The VM changes IP: only the target connector is rebuilt.
	addTopologySnapshot(t, s, vm, connector.ServiceSnapshot{ServiceName: "Proxmox", Entities: []connector.SnapshotEntity{
		{Kind: "vm", Name: "web-01", ExternalID: "100", IP: "10.0.0.99"},
	}})
	rebuild(t, e, vm)
	if edges := edgesOfKind(t, s, store.TopologyEdgeResolvesTo, dns, vm); len(edges) != 0 {
		t.Fatalf("resolves_to survived the target changing IP: %+v", edges)
	}

	// It comes back (new IP matches again) and is then removed with the entity.
	addTopologySnapshot(t, s, vm, connector.ServiceSnapshot{ServiceName: "Proxmox", Entities: []connector.SnapshotEntity{
		{Kind: "vm", Name: "web-01", ExternalID: "100", IP: "10.0.0.5"},
	}})
	rebuild(t, e, vm)
	if edges := edgesOfKind(t, s, store.TopologyEdgeResolvesTo, dns, vm); len(edges) != 1 || edges[0].ConnectorID != dns {
		t.Fatalf("resolves_to after target matched again: %+v", edges)
	}
	addTopologySnapshot(t, s, vm, connector.ServiceSnapshot{ServiceName: "Proxmox"})
	rebuild(t, e, vm)
	if edges := edgesOfKind(t, s, store.TopologyEdgeResolvesTo, dns, vm); len(edges) != 0 {
		t.Fatalf("resolves_to survived the target entity disappearing: %+v", edges)
	}
}

func TestResolvesToSkipsDNSToDNS(t *testing.T) {
	s := newEngineTestStore(t)
	e := NewEngine(s)
	a := seedEngineConnectorWithEntities(t, s, "Pihole", "networking", "pihole", []connector.SnapshotEntity{{Kind: "dns_record", Name: "a.lan", IP: "10.0.0.5"}})
	b := seedEngineConnectorWithEntities(t, s, "Adguard", "networking", "adguardhome", []connector.SnapshotEntity{{Kind: "dns_rewrite", Name: "b.lan", IP: "10.0.0.5"}})
	rebuild(t, e, a, b)
	if edges := edgesOfKind(t, s, store.TopologyEdgeResolvesTo, a, b); len(edges) != 0 {
		t.Fatalf("dns->dns resolves_to = %+v, want none", edges)
	}
}

func TestRebuildIsIdempotent(t *testing.T) {
	s, e, dns, vm := dnsFixture(t)
	rebuild(t, e, dns, vm)
	first, err := s.TopologyEdgesFingerprint(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	rebuild(t, e, dns, vm, dns, vm)
	second, err := s.TopologyEdgesFingerprint(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("edge set changed across identical rebuilds")
	}
}

func TestRuntimeTargetIsDeterministicAcrossConnectors(t *testing.T) {
	s := newEngineTestStore(t)
	e := NewEngine(s)
	// Both other connectors have an entity named "local"; the lower connector
	// ID must win regardless of creation order.
	x := seedEngineConnectorWithEntities(t, s, "X", "virtualization", "proxmox", []connector.SnapshotEntity{{Kind: "node", Name: "local", ExternalID: "x"}})
	y := seedEngineConnectorWithEntities(t, s, "Y", "virtualization", "proxmox", []connector.SnapshotEntity{{Kind: "node", Name: "local", ExternalID: "y"}})
	port := seedEngineConnectorWithEntities(t, s, "Portainer", "containers_paas", "portainer", []connector.SnapshotEntity{
		{Kind: "container", Name: "c1", ExternalID: "c1", Attributes: map[string]any{"environment": "local"}},
	})
	want := x
	if y < x {
		want = y
	}
	for range 3 {
		rebuild(t, e, port)
		edges := edgesOfKind(t, s, store.TopologyEdgeRunsOn, x, y, port)
		if len(edges) != 1 || edges[0].DstConnectorID != want {
			t.Fatalf("runs_on = %+v, want destination connector %s", edges, want)
		}
	}
}

func labDocVersions(t *testing.T, s *store.Store) (versions int, found bool) {
	t.Helper()
	docs, _, err := s.ListAllDocs(context.Background(), labTopologyTitle, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		if d.Kind == "lab" && d.Title == labTopologyTitle {
			vs, err := s.GetDocVersions(context.Background(), d.ID)
			if err != nil {
				t.Fatal(err)
			}
			return len(vs), true
		}
	}
	return 0, false
}

func TestTopologyDocNotCreatedWhenAbsent(t *testing.T) {
	s, e, dns, vm := dnsFixture(t)
	rebuild(t, e, dns, vm)
	if _, found := labDocVersions(t, s); found {
		t.Fatal("rebuild created a Lab Topology doc that did not exist")
	}
}

func TestTopologyDocRegeneratedOnlyWhenEdgesChange(t *testing.T) {
	ctx := context.Background()
	s, e, dns, vm := dnsFixture(t)
	rebuild(t, e, dns, vm)
	if _, err := e.GenerateLabTopology(ctx); err != nil {
		t.Fatal(err)
	}
	base, _ := labDocVersions(t, s)

	// Identical rebuilds: no new doc version.
	rebuild(t, e, dns, vm, dns, vm)
	if n, _ := labDocVersions(t, s); n != base {
		t.Fatalf("doc versions = %d, want %d after unchanged rebuilds", n, base)
	}

	// The edge set changes: the doc is regenerated.
	addTopologySnapshot(t, s, dns, connector.ServiceSnapshot{ServiceName: "Pihole"})
	rebuild(t, e, dns)
	changed, _ := labDocVersions(t, s)
	if changed != base+1 {
		t.Fatalf("doc versions = %d, want %d after the edge set changed", changed, base+1)
	}
	// Once the other side has settled too, further rebuilds add nothing.
	rebuild(t, e, vm)
	settled, _ := labDocVersions(t, s)
	rebuild(t, e, dns, vm, dns, vm)
	if n, _ := labDocVersions(t, s); n != settled {
		t.Fatalf("doc versions = %d, want %d after unchanged rebuilds", n, settled)
	}
}

func TestTopologyDocLeftAloneWhenHumanOwned(t *testing.T) {
	ctx := context.Background()
	s, e, dns, vm := dnsFixture(t)
	note := &store.DocRecord{Title: labTopologyTitle, Kind: "lab", Content: "my notes", Origin: store.DocOriginHuman}
	if err := s.CreateDoc(ctx, note); err != nil {
		t.Fatal(err)
	}
	rebuild(t, e, dns, vm)
	got, err := s.GetDoc(ctx, note.ID)
	if err != nil || got.Content != "my notes" {
		t.Fatalf("human doc = %+v, %v; want untouched", got, err)
	}
}

func TestTopologyDocRegenerationFailureDoesNotFailRebuildAndRetries(t *testing.T) {
	ctx := context.Background()
	s, e, dns, vm := dnsFixture(t)
	rebuild(t, e, dns, vm)
	if _, err := e.GenerateLabTopology(ctx); err != nil {
		t.Fatal(err)
	}
	base, _ := labDocVersions(t, s)

	// Make every doc update fail, then change the edge set.
	if _, err := s.DB().ExecContext(ctx, `CREATE TRIGGER fail_doc_update BEFORE UPDATE ON docs BEGIN SELECT RAISE(ABORT, 'boom'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.GenerateLabTopology(ctx); err == nil {
		t.Fatal("expected GenerateLabTopology to fail with the trigger installed")
	}
	addTopologySnapshot(t, s, dns, connector.ServiceSnapshot{ServiceName: "Pihole"})
	if err := e.RebuildTopologyForConnector(ctx, dns); err != nil {
		t.Fatalf("rebuild failed because doc regeneration failed: %v", err)
	}
	if edges := edgesOfKind(t, s, store.TopologyEdgeResolvesTo, dns, vm); len(edges) != 0 {
		t.Fatalf("edges not committed: %+v", edges)
	}
	if n, _ := labDocVersions(t, s); n != base {
		t.Fatalf("doc versions = %d, want %d while regeneration fails", n, base)
	}

	// A later rebuild with unchanged edges retries and heals the doc.
	if _, err := s.DB().ExecContext(ctx, `DROP TRIGGER fail_doc_update`); err != nil {
		t.Fatal(err)
	}
	rebuild(t, e, dns)
	if n, _ := labDocVersions(t, s); n != base+1 {
		t.Fatalf("doc versions = %d, want %d after the retry", n, base+1)
	}
}

func TestTopologyDocWithoutMarkerGetsNoVersionWhenContentUnchanged(t *testing.T) {
	ctx := context.Background()
	s, e, dns, vm := dnsFixture(t)
	rebuild(t, e, dns, vm)
	res, err := e.GenerateLabTopology(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// A doc generated before fingerprints existed: same body, no marker line.
	if err := s.UpdateDoc(ctx, res.DocID, StripMarkers(res.Content), nil); err != nil {
		t.Fatal(err)
	}
	base, _ := labDocVersions(t, s)
	rebuild(t, e, dns, vm, dns, vm)
	if n, _ := labDocVersions(t, s); n != base {
		t.Fatalf("doc versions = %d, want %d: unchanged legacy doc got a new version", n, base)
	}
	// A real edge change still regenerates it.
	addTopologySnapshot(t, s, dns, connector.ServiceSnapshot{ServiceName: "Pihole"})
	rebuild(t, e, dns)
	if n, _ := labDocVersions(t, s); n != base+1 {
		t.Fatalf("doc versions = %d, want %d after an edge change", n, base+1)
	}
}

func TestResolvesToKeptWhenOtherConnectorSnapshotUnreadable(t *testing.T) {
	ctx := context.Background()
	s, e, dns, vm := dnsFixture(t)
	rebuild(t, e, dns, vm)
	if edges := edgesOfKind(t, s, store.TopologyEdgeResolvesTo, dns, vm); len(edges) != 1 {
		t.Fatalf("setup: resolves_to = %+v", edges)
	}
	// The DNS connector's latest snapshot cannot be read during the target's rebuild.
	if err := s.CreateSnapshot(ctx, &store.SnapshotRecord{ConnectorID: dns, Data: "{not json"}); err != nil {
		t.Fatal(err)
	}
	rebuild(t, e, vm)
	edges := edgesOfKind(t, s, store.TopologyEdgeResolvesTo, dns, vm)
	if len(edges) != 1 || edges[0].ConnectorID != dns {
		t.Fatalf("resolves_to after unreadable DNS snapshot = %+v, want the existing edge kept", edges)
	}
}
