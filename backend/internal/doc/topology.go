package doc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/logsafe"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// topologyServiceKind is the node kind used for a connector's own service
// node (the anchor its dependencies and entities hang off).
const topologyServiceKind = "service"

// entityRef is the connector-local identity of an entity: its external ID
// when the connector provides one, else its name.
func entityRef(e connector.SnapshotEntity) string {
	if e.ExternalID != "" {
		return e.ExternalID
	}
	return e.Name
}

// RebuildTopologyForConnector recomputes and persists the topology edges for
// connectorID from its latest snapshot: declared ServiceDependencies, plus
// cross-connector entity matches found by matchEntities (external ID, IP,
// hostname). Edges are replaced atomically, so a re-sync never leaves stale
// edges behind. A connector with no snapshot simply loses its edges; any other failure
// loading the snapshot returns the error and leaves existing edges untouched.
//
// Matches are heuristic (they can link unrelated objects sharing an IP), so
// every edge records how it was derived in Source.
//
// After the edges are stored, an existing generated Lab Topology doc is
// refreshed when its recorded edge fingerprint no longer matches the stored
// edges. A failure there is logged, not returned: the edges are already
// committed and the next rebuild retries because the fingerprint still differs.
func (e *Engine) RebuildTopologyForConnector(ctx context.Context, connectorID string) error {
	conn, err := e.store.GetConnector(ctx, connectorID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("get connector: %w", err)
	}

	snap, err := e.snapshots.latest(ctx, connectorID)
	if errors.Is(err, store.ErrNotFound) {
		// No snapshot at all: drop whatever edges exist.
		if err := e.store.ReplaceTopologyEdgesForConnector(ctx, connectorID, nil); err != nil {
			return err
		}
		e.refreshTopologyDoc(ctx)
		return nil
	}
	if err != nil {
		// A transient DB error or an unparseable snapshot says nothing about
		// the real topology; keep the last good edges rather than wiping them.
		return fmt.Errorf("load latest snapshot: %w", err)
	}

	b := &edgeBuilder{seen: map[string]bool{}}
	self := store.TopologyEdge{SrcConnectorID: connectorID, SrcKind: topologyServiceKind, SrcName: conn.Name, SrcRef: connectorID}

	links, err := matchEntities(ctx, e.store, e.snapshots, connectorID, snap.Entities)
	if err != nil {
		return err
	}
	for _, l := range links {
		b.add(self, connectorID, l.Local.Kind, l.Local.Name, entityRef(l.Local), store.TopologyEdgeContains, "")
		b.add(store.TopologyEdge{
			SrcConnectorID: connectorID, SrcKind: l.Local.Kind, SrcName: l.Local.Name, SrcRef: entityRef(l.Local),
		}, l.ConnectorID, l.Entity.Kind, l.Entity.Name, entityRef(l.Entity), store.TopologyEdgeSameAs, l.Reason)
		other := store.TopologyEdge{SrcConnectorID: l.ConnectorID, SrcKind: topologyServiceKind, SrcName: l.ConnectorName, SrcRef: l.ConnectorID}
		b.add(other, l.ConnectorID, l.Entity.Kind, l.Entity.Name, entityRef(l.Entity), store.TopologyEdgeContains, "")
	}

	// Other connectors' snapshots are loaded once and reused by every
	// cross-connector lookup below, in connector ID order so the first match
	// is deterministic.
	all, err := e.store.ListAllConnectors(ctx)
	if err != nil {
		return fmt.Errorf("list connectors: %w", err)
	}
	others, unreadable := e.loadOtherSnapshots(ctx, connectorID, all)

	addResolvesToEdges(b, connectorID, snap, others)
	for _, dep := range snap.Dependencies {
		if strings.TrimSpace(dep.Name) == "" {
			continue
		}
		addDependencyEdge(b, self, connectorID, snap, all, others, dep)
	}
	addRuntimeEdges(b, connectorID, snap, others)

	if err := e.store.ReplaceTopologyEdgesPreserving(ctx, connectorID, b.edges, unreadable); err != nil {
		return err
	}
	e.refreshTopologyDoc(ctx)
	return nil
}

// otherSnapshot is another connector's latest parsed snapshot.
type otherSnapshot struct {
	ID   string
	Name string
	Snap *connector.ServiceSnapshot
}

// loadOtherSnapshots returns the latest snapshot of every connector except
// connectorID, ordered by connector ID. Connectors without any snapshot are
// skipped; connectors whose snapshot exists but could not be read are
// returned in unreadable, so the caller keeps the edges derived from them.
func (e *Engine) loadOtherSnapshots(ctx context.Context, connectorID string, all []store.ConnectorRecord) (out []otherSnapshot, unreadable []string) {
	out = make([]otherSnapshot, 0, len(all))
	for _, c := range all {
		if c.ID == connectorID {
			continue
		}
		snap, err := e.snapshots.latest(ctx, c.ID)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				unreadable = append(unreadable, c.ID)
			}
			continue
		}
		out = append(out, otherSnapshot{ID: c.ID, Name: c.Name, Snap: snap})
	}
	slices.SortFunc(out, func(a, b otherSnapshot) int { return strings.Compare(a.ID, b.ID) })
	return out, unreadable
}

func isDNSEntity(kind string) bool { return kind == "dns_record" || kind == "dns_rewrite" }

// topologyDocMarker prefixes the edge fingerprint embedded in the generated
// Lab Topology doc, so a later rebuild can tell whether the doc reflects the
// stored edges.
const topologyDocMarker = "<!-- wl:topology-edges:"

// refreshTopologyDoc regenerates the generated Lab Topology doc when it
// exists and its recorded edge fingerprint differs from the stored edges. It
// never fails the caller: errors are logged, and the next rebuild retries
// because the doc still carries the old fingerprint.
func (e *Engine) refreshTopologyDoc(ctx context.Context) {
	if err := e.regenerateTopologyDocIfStale(ctx); err != nil {
		slog.Error("topology doc regeneration failed", "error", logsafe.Sanitize(err.Error()))
	}
}

func (e *Engine) regenerateTopologyDocIfStale(ctx context.Context) error {
	docs, _, err := e.store.ListAllDocs(ctx, labTopologyTitle, 0, 50)
	if err != nil {
		return fmt.Errorf("list topology docs: %w", err)
	}
	for _, d := range docs {
		if d.Kind != "lab" || d.Title != labTopologyTitle || d.Origin == store.DocOriginHuman {
			continue
		}
		fingerprint, err := e.store.TopologyEdgesFingerprint(ctx)
		if err != nil {
			return err
		}
		// The listing omits content; read the doc itself.
		full, err := e.store.GetDoc(ctx, d.ID)
		if err != nil {
			return fmt.Errorf("get topology doc: %w", err)
		}
		if strings.Contains(full.Content, topologyDocMarker+fingerprint+" -->") {
			return nil
		}
		if _, err := e.generateLabTopology(ctx, true); err != nil {
			return fmt.Errorf("regenerate topology doc: %w", err)
		}
		return nil
	}
	return nil
}

// addResolvesToEdges derives resolves_to edges (DNS record to the entity that
// owns its IP or hostname) between connectorID and every other connector.
// Each edge has a single owner: the connector of the DNS entity. Edges from
// connectorID's own DNS entities are owned by connectorID; edges from other
// connectors' DNS entities that point at connectorID's entities are emitted
// here too, owned by the DNS connector, because this rebuild deletes them (see
// Store.ReplaceTopologyEdgesForConnector) so they cannot go stale when an
// entity of connectorID disappears or changes IP.
func addResolvesToEdges(b *edgeBuilder, connectorID string, snap *connector.ServiceSnapshot, others []otherSnapshot) {
	link := func(owner string, dns connector.SnapshotEntity, dstConnectorID string, target connector.SnapshotEntity) {
		reason := matchReason(dns, target)
		if reason == "" {
			return
		}
		src := store.TopologyEdge{SrcConnectorID: owner, SrcKind: dns.Kind, SrcName: dns.Name, SrcRef: entityRef(dns)}
		b.addOwned(owner, src, dstConnectorID, target.Kind, target.Name, entityRef(target), store.TopologyEdgeResolvesTo, reason, "")
	}
	for _, dns := range snap.Entities {
		if !isDNSEntity(dns.Kind) {
			continue
		}
		for _, o := range others {
			for _, target := range o.Snap.Entities {
				if !isDNSEntity(target.Kind) {
					link(connectorID, dns, o.ID, target)
				}
			}
		}
	}
	for _, o := range others {
		for _, dns := range o.Snap.Entities {
			if !isDNSEntity(dns.Kind) {
				continue
			}
			for _, target := range snap.Entities {
				if !isDNSEntity(target.Kind) {
					link(o.ID, dns, connectorID, target)
				}
			}
		}
	}
}

// proxySourceKinds are the entity kinds that forward traffic to an upstream.
// A Traefik "service" is deliberately absent: it is a target of routers, not
// a source.
var proxySourceKinds = map[string]bool{"proxy_host": true, "stream": true, "router": true, "http_route": true}

// proxyTarget is one upstream a proxy entity declares. kind, when set,
// restricts which entity kind may be resolved as the destination.
type proxyTarget struct{ host, port, kind string }

// proxyTargets reads the upstreams a proxy entity declares in its attributes:
// NPM forward_host/forwarding_host (+ port), a Traefik router's service, and
// the comma-separated dial targets of a Caddy route's upstream.
func proxyTargets(ent connector.SnapshotEntity) []proxyTarget {
	var out []proxyTarget
	for _, hostKey := range []string{"forward_host", "forwarding_host"} {
		raw, ok := ent.Attributes[hostKey].(string)
		if !ok {
			continue
		}
		host, port := runtimeTargetParts(raw)
		for _, portKey := range []string{"forward_port", "forwarding_port"} {
			if v, ok := ent.Attributes[portKey]; ok && fmt.Sprint(v) != "0" {
				port = fmt.Sprint(v)
				break
			}
		}
		out = append(out, proxyTarget{host: host, port: port})
	}
	if raw, ok := ent.Attributes["service"].(string); ok {
		host, port := runtimeTargetParts(raw)
		out = append(out, proxyTarget{host: host, port: port, kind: "service"})
	}
	if raw, ok := ent.Attributes["upstream"].(string); ok {
		for _, dial := range strings.Split(raw, ",") {
			host, port := runtimeTargetParts(dial)
			out = append(out, proxyTarget{host: host, port: port})
		}
	}
	return out
}

// normalizeHost lower-cases a host and drops a trailing root dot.
func normalizeHost(h string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
}

func addRuntimeEdges(b *edgeBuilder, connectorID string, snap *connector.ServiceSnapshot, others []otherSnapshot) {
	for _, ent := range snap.Entities {
		if ent.Kind != "container" {
			continue
		}
		if raw, ok := ent.Attributes["environment"].(string); ok && raw != "" {
			dstConnector, dstKind, dstName, dstRef := connectorID, "environment", raw, raw
			if target, cid, ok := runtimeTarget(connectorID, snap, others, raw, "", ent); ok {
				dstConnector, dstKind, dstName, dstRef = cid, target.Kind, target.Name, entityRef(target)
			}
			b.add(entitySource(connectorID, ent), dstConnector, dstKind, dstName, dstRef, store.TopologyEdgeRunsOn, "Portainer environment")
		}
	}
	for _, dep := range snap.Dependencies {
		switch dep.Kind {
		case "upstream_service":
			addProxyEdges(b, connectorID, snap, others, dep)
		case "host":
			for _, ent := range snap.Entities {
				if ent.Kind != "container" && ent.Kind != "vm" {
					continue
				}
				if _, hasEnvironment := ent.Attributes["environment"]; hasEnvironment {
					continue
				}
				if host, ok := ent.Attributes["node"].(string); ok && host != dep.Name {
					continue
				}
				dstConnector, dstKind, dstName, dstRef := connectorID, "host", dep.Name, ""
				if target, cid, ok := runtimeTarget(connectorID, snap, others, dep.Name, "", ent); ok {
					dstConnector, dstKind, dstName, dstRef = cid, target.Kind, target.Name, entityRef(target)
				}
				b.add(entitySource(connectorID, ent), dstConnector, dstKind, dstName, dstRef, store.TopologyEdgeRunsOn, "connector host")
			}
		}
	}
}

func entitySource(connectorID string, ent connector.SnapshotEntity) store.TopologyEdge {
	return store.TopologyEdge{SrcConnectorID: connectorID, SrcKind: ent.Kind, SrcName: ent.Name, SrcRef: entityRef(ent)}
}

// addProxyEdges emits proxies_to edges from each proxy entity whose declared
// upstream positively matches dep (exact, after host normalisation). Entities
// declaring no upstream never match, and an edge never points back at its own
// source.
func addProxyEdges(b *edgeBuilder, connectorID string, snap *connector.ServiceSnapshot, others []otherSnapshot, dep connector.ServiceDependency) {
	depHost, depPort := runtimeTargetParts(dep.Name)
	depHost = normalizeHost(depHost)
	if depHost == "" {
		return
	}
	for _, ent := range snap.Entities {
		if !proxySourceKinds[ent.Kind] {
			continue
		}
		for _, t := range proxyTargets(ent) {
			if normalizeHost(t.host) != depHost {
				continue
			}
			port := t.port
			if port == "" {
				port = depPort
			}
			dstConnector, dstKind, dstName, dstRef := connectorID, "upstream_service", dep.Name, ""
			if target, cid, ok := runtimeTarget(connectorID, snap, others, dep.Name, t.kind, ent); ok {
				dstConnector, dstKind, dstName, dstRef = cid, target.Kind, target.Name, entityRef(target)
			}
			b.addOwned("", entitySource(connectorID, ent), dstConnector, dstKind, dstName, dstRef, store.TopologyEdgeProxiesTo, "configured upstream", port)
			break
		}
	}
}

// runtimeTarget resolves name to an entity: first in the connector's own
// snapshot, then in the other connectors' (ordered by ID). kind, when set,
// restricts the match to that entity kind, and exclude (the edge's own
// source) is never returned, so an edge cannot loop back on itself.
func runtimeTarget(connectorID string, snap *connector.ServiceSnapshot, others []otherSnapshot, name, kind string, exclude connector.SnapshotEntity) (connector.SnapshotEntity, string, bool) {
	host, _ := runtimeTargetParts(name)
	target := normalizeHost(host)
	if target == "" {
		return connector.SnapshotEntity{}, "", false
	}
	find := func(entities []connector.SnapshotEntity, sameConnector bool) (connector.SnapshotEntity, bool) {
		for _, ent := range entities {
			if (kind != "" && ent.Kind != kind) || (sameConnector && ent.Kind == exclude.Kind && entityRef(ent) == entityRef(exclude)) {
				continue
			}
			if normalizeHost(ent.Name) == target || normalizeHost(ent.Hostname) == target || normalizeHost(ent.IP) == target {
				return ent, true
			}
			for _, alias := range ent.Aliases {
				if normalizeHost(alias) == target {
					return ent, true
				}
			}
		}
		return connector.SnapshotEntity{}, false
	}
	if ent, ok := find(snap.Entities, true); ok {
		return ent, connectorID, true
	}
	for _, o := range others {
		if ent, ok := find(o.Snap.Entities, false); ok {
			return ent, o.ID, true
		}
	}
	return connector.SnapshotEntity{}, "", false
}

func runtimeTargetParts(name string) (host, port string) {
	raw := strings.TrimSpace(name)
	parsed, err := url.Parse(raw)
	if err == nil && parsed.Hostname() != "" {
		return parsed.Hostname(), parsed.Port()
	}
	if !strings.Contains(raw, "@") {
		parsed, err = url.Parse("//" + raw)
		if err == nil && parsed.Hostname() != "" {
			return parsed.Hostname(), parsed.Port()
		}
	}
	return raw, ""
}

// addDependencyEdge resolves a declared dependency to the best known node:
// an explicit connector Ref, an entity of the same connector with the same
// name, an entity of another connector with the same name or hostname, or
// finally a placeholder node (kind + name) owned by this connector.
func addDependencyEdge(b *edgeBuilder, self store.TopologyEdge, connectorID string, snap *connector.ServiceSnapshot, all []store.ConnectorRecord, others []otherSnapshot, dep connector.ServiceDependency) {
	if dep.Ref != "" {
		for _, c := range all {
			if c.ID == dep.Ref && c.ID != connectorID {
				b.add(self, c.ID, topologyServiceKind, c.Name, c.ID, store.TopologyEdgeDependency, dep.Kind)
				return
			}
		}
	}
	for _, ent := range snap.Entities {
		if strings.EqualFold(ent.Name, dep.Name) {
			b.add(self, connectorID, ent.Kind, ent.Name, entityRef(ent), store.TopologyEdgeDependency, dep.Kind)
			b.add(self, connectorID, ent.Kind, ent.Name, entityRef(ent), store.TopologyEdgeContains, "")
			return
		}
	}
	for _, o := range others {
		for _, ent := range o.Snap.Entities {
			if strings.EqualFold(ent.Name, dep.Name) || (ent.Hostname != "" && strings.EqualFold(ent.Hostname, dep.Name)) {
				b.add(self, o.ID, ent.Kind, ent.Name, entityRef(ent), store.TopologyEdgeDependency, dep.Kind)
				b.add(store.TopologyEdge{SrcConnectorID: o.ID, SrcKind: topologyServiceKind, SrcName: o.Name, SrcRef: o.ID},
					o.ID, ent.Kind, ent.Name, entityRef(ent), store.TopologyEdgeContains, "")
				return
			}
		}
	}
	b.add(self, connectorID, dep.Kind, dep.Name, "", store.TopologyEdgeDependency, dep.Kind)
}

// edgeBuilder accumulates de-duplicated edges.
type edgeBuilder struct {
	edges []store.TopologyEdge
	seen  map[string]bool
}

// add appends an edge owned by the connector being rebuilt.
func (b *edgeBuilder) add(src store.TopologyEdge, dstConnectorID, dstKind, dstName, dstRef, kind, source string) {
	b.addOwned("", src, dstConnectorID, dstKind, dstName, dstRef, kind, source, "")
}

// addOwned appends an edge from src's endpoint fields to the given
// destination. A non-empty owner makes that connector the edge's owner
// instead of the connector being rebuilt.
func (b *edgeBuilder) addOwned(owner string, src store.TopologyEdge, dstConnectorID, dstKind, dstName, dstRef, kind, source, detail string) {
	e := src
	e.ConnectorID = owner
	e.DstConnectorID, e.DstKind, e.DstName, e.DstRef = dstConnectorID, dstKind, dstName, dstRef
	e.Kind, e.Source = kind, source
	e.Detail = detail
	key := strings.Join([]string{
		e.ConnectorID, e.SrcConnectorID, e.SrcKind, e.SrcName, e.SrcRef,
		e.DstConnectorID, e.DstKind, e.DstName, e.DstRef, e.Kind, e.Source, e.Detail,
	}, "\x00")
	if b.seen[key] {
		return
	}
	b.seen[key] = true
	b.edges = append(b.edges, e)
}

// BackfillTopology rebuilds edges for connectors that have a snapshot but no
// edges yet, so topology_path works right after upgrade instead of waiting for
// each connector's next sync. It is idempotent and cheap (a connector whose
// snapshot genuinely yields no edges is just rebuilt to nothing again), and a
// per-connector failure is logged and skipped. It returns how many connectors
// were rebuilt.
func (e *Engine) BackfillTopology(ctx context.Context) (int, error) {
	ids, err := e.store.ListConnectorIDsMissingTopology(ctx)
	if err != nil {
		return 0, err
	}
	rebuilt := 0
	for _, id := range ids {
		if ctx.Err() != nil {
			return rebuilt, ctx.Err()
		}
		if err := e.RebuildTopologyForConnector(ctx, id); err != nil {
			slog.Error("topology backfill failed", "connector", logsafe.Sanitize(id), "error", logsafe.Sanitize(err.Error()))
			continue
		}
		rebuilt++
	}
	return rebuilt, nil
}
