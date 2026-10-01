package doc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
		return e.store.ReplaceTopologyEdgesForConnector(ctx, connectorID, nil)
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

	if len(snap.Dependencies) > 0 {
		others, err := e.store.ListAllConnectors(ctx)
		if err != nil {
			return fmt.Errorf("list connectors: %w", err)
		}
		for _, dep := range snap.Dependencies {
			if strings.TrimSpace(dep.Name) == "" {
				continue
			}
			e.addDependencyEdge(ctx, b, self, connectorID, snap, others, dep)
		}
	}

	return e.store.ReplaceTopologyEdgesForConnector(ctx, connectorID, b.edges)
}

// addDependencyEdge resolves a declared dependency to the best known node:
// an explicit connector Ref, an entity of the same connector with the same
// name, an entity of another connector with the same name or hostname, or
// finally a placeholder node (kind + name) owned by this connector.
func (e *Engine) addDependencyEdge(ctx context.Context, b *edgeBuilder, self store.TopologyEdge, connectorID string, snap *connector.ServiceSnapshot, all []store.ConnectorRecord, dep connector.ServiceDependency) {
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
	for _, c := range all {
		if c.ID == connectorID {
			continue
		}
		other, err := e.snapshots.latest(ctx, c.ID)
		if err != nil {
			continue
		}
		for _, ent := range other.Entities {
			if strings.EqualFold(ent.Name, dep.Name) || (ent.Hostname != "" && strings.EqualFold(ent.Hostname, dep.Name)) {
				b.add(self, c.ID, ent.Kind, ent.Name, entityRef(ent), store.TopologyEdgeDependency, dep.Kind)
				b.add(store.TopologyEdge{SrcConnectorID: c.ID, SrcKind: topologyServiceKind, SrcName: c.Name, SrcRef: c.ID},
					c.ID, ent.Kind, ent.Name, entityRef(ent), store.TopologyEdgeContains, "")
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

// add appends an edge from src's endpoint fields to the given destination.
func (b *edgeBuilder) add(src store.TopologyEdge, dstConnectorID, dstKind, dstName, dstRef, kind, source string) {
	e := src
	e.DstConnectorID, e.DstKind, e.DstName, e.DstRef = dstConnectorID, dstKind, dstName, dstRef
	e.Kind, e.Source = kind, source
	key := strings.Join([]string{
		e.SrcConnectorID, e.SrcKind, e.SrcName, e.SrcRef,
		e.DstConnectorID, e.DstKind, e.DstName, e.DstRef, e.Kind, e.Source,
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
