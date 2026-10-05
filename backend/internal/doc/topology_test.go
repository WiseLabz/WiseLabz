package doc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func addTopologySnapshot(t *testing.T, s *store.Store, connectorID string, snap connector.ServiceSnapshot) {
	t.Helper()
	snap.FetchedAt = time.Now().UTC()
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := s.CreateSnapshot(context.Background(), &store.SnapshotRecord{ConnectorID: connectorID, Data: string(data)}); err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
}

func TestRebuildTopologyForConnector(t *testing.T) {
	ctx := context.Background()
	s := newEngineTestStore(t)
	e := NewEngine(s)

	pve := seedEngineConnectorWithEntities(t, s, "Proxmox", "virtualization", "proxmox", []connector.SnapshotEntity{
		{Kind: "vm", Name: "web-01", ExternalID: "100", IP: "10.0.0.5"},
		{Kind: "node", Name: "pve1"},
	})
	dock := seedEngineConnectorWithEntities(t, s, "Docker", "containers_paas", "docker", nil)
	addTopologySnapshot(t, s, dock, connector.ServiceSnapshot{
		ServiceName: "Docker",
		Entities: []connector.SnapshotEntity{
			{Kind: "container", Name: "nginx", IP: "10.0.0.5", Attributes: map[string]any{"node": "pve1"}},
			{Kind: "proxy_host", Name: "web-proxy", Attributes: map[string]any{"forward_host": "web-01", "forward_port": 8443}},
		},
		Dependencies: []connector.ServiceDependency{{Kind: "host", Name: "pve1"}, {Kind: "network", Name: "lan"}, {Kind: "upstream_service", Name: "web-01"}},
	})

	if err := e.RebuildTopologyForConnector(ctx, dock); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	edges, err := s.ListTopologyEdges(ctx, []string{pve, dock})
	if err != nil {
		t.Fatal(err)
	}
	has := func(kind, srcName, dstName string) bool {
		for _, ed := range edges {
			if ed.Kind == kind && ed.SrcName == srcName && ed.DstName == dstName {
				return true
			}
		}
		return false
	}
	if !has(store.TopologyEdgeSameAs, "nginx", "web-01") {
		t.Errorf("missing same_as nginx->web-01 in %+v", edges)
	}
	if !has(store.TopologyEdgeDependency, "Docker", "pve1") {
		t.Errorf("missing dependency Docker->pve1 (resolved to proxmox entity) in %+v", edges)
	}
	if !has(store.TopologyEdgeDependency, "Docker", "lan") {
		t.Errorf("missing placeholder dependency Docker->lan in %+v", edges)
	}
	if !has(store.TopologyEdgeRunsOn, "nginx", "pve1") {
		t.Errorf("missing directed runs_on edge nginx->pve1 in %+v", edges)
	}
	proxyFound := false
	for _, ed := range edges {
		if ed.Kind == store.TopologyEdgeProxiesTo && ed.SrcName == "web-proxy" && ed.DstName == "web-01" && ed.Detail == "8443" {
			proxyFound = true
		}
	}
	if !proxyFound {
		t.Errorf("missing directed proxy edge with upstream port in %+v", edges)
	}

	// Re-sync with the container gone and no dependencies: edges are replaced.
	addTopologySnapshot(t, s, dock, connector.ServiceSnapshot{ServiceName: "Docker"})
	if err := e.RebuildTopologyForConnector(ctx, dock); err != nil {
		t.Fatalf("rebuild after change: %v", err)
	}
	edges, err = s.ListTopologyEdges(ctx, []string{pve, dock})
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Fatalf("stale edges after re-sync: %+v", edges)
	}
}

// Only a missing snapshot (store.ErrNotFound) may clear a connector's edges;
// an unparseable one must leave the last good edges in place.
func TestRebuildTopologyKeepsEdgesOnSnapshotError(t *testing.T) {
	ctx := context.Background()
	s := newEngineTestStore(t)
	e := NewEngine(s)

	dock := seedEngineConnectorWithEntities(t, s, "Docker", "containers_paas", "docker", nil)
	addTopologySnapshot(t, s, dock, connector.ServiceSnapshot{
		ServiceName:  "Docker",
		Dependencies: []connector.ServiceDependency{{Kind: "network", Name: "lan"}},
	})
	if err := e.RebuildTopologyForConnector(ctx, dock); err != nil {
		t.Fatal(err)
	}
	before, _ := s.ListTopologyEdges(ctx, []string{dock})
	if len(before) == 0 {
		t.Fatal("expected edges before corrupting the snapshot")
	}

	if err := s.CreateSnapshot(ctx, &store.SnapshotRecord{ConnectorID: dock, Data: "{not json"}); err != nil {
		t.Fatal(err)
	}
	if err := e.RebuildTopologyForConnector(ctx, dock); err == nil {
		t.Fatal("expected an error for an unparseable snapshot")
	}
	after, _ := s.ListTopologyEdges(ctx, []string{dock})
	if len(after) != len(before) {
		t.Fatalf("edges changed on snapshot error: before %d, after %d", len(before), len(after))
	}

	// A connector with no snapshot at all does lose its edges.
	bare := seedEngineConnectorWithEntities(t, s, "Bare", "containers_paas", "docker", nil)
	if err := s.ReplaceTopologyEdgesForConnector(ctx, bare, []store.TopologyEdge{{
		SrcConnectorID: bare, SrcKind: "service", SrcName: "Bare", DstConnectorID: bare, DstKind: "network", DstName: "lan", Kind: store.TopologyEdgeDependency,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := e.RebuildTopologyForConnector(ctx, bare); err != nil {
		t.Fatal(err)
	}
	if edges, _ := s.ListTopologyEdges(ctx, []string{bare}); len(edges) != 0 {
		t.Fatalf("snapshot-less connector kept edges: %+v", edges)
	}
}

func TestBackfillTopology(t *testing.T) {
	ctx := context.Background()
	s := newEngineTestStore(t)
	e := NewEngine(s)

	withSnap := seedEngineConnectorWithEntities(t, s, "Docker", "containers_paas", "docker", nil)
	addTopologySnapshot(t, s, withSnap, connector.ServiceSnapshot{
		ServiceName:  "Docker",
		Dependencies: []connector.ServiceDependency{{Kind: "network", Name: "lan"}},
	})
	// Has a snapshot but it yields no edges, so it is "missing" on every run.
	noEdges := seedEngineConnectorWithEntities(t, s, "Empty", "containers_paas", "docker", nil)

	n, err := e.BackfillTopology(ctx)
	if err != nil || n != 2 {
		t.Fatalf("backfill = %d, %v; want 2 connectors rebuilt", n, err)
	}
	if edges, _ := s.ListTopologyEdges(ctx, []string{withSnap}); len(edges) == 0 {
		t.Fatal("snapshot connector has no edges after backfill")
	}
	if edges, _ := s.ListTopologyEdges(ctx, []string{noEdges}); len(edges) != 0 {
		t.Fatalf("edge-less connector got edges: %+v", edges)
	}
	// Idempotent: connectors that already have edges are skipped; the
	// edge-less one is just rebuilt to nothing again.
	if n, err := e.BackfillTopology(ctx); err != nil || n != 1 {
		t.Fatalf("second backfill = %d, %v; want 1", n, err)
	}
	if edges, _ := s.ListTopologyEdges(ctx, []string{withSnap}); len(edges) == 0 {
		t.Fatal("existing edges were lost")
	}
}
