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
		ServiceName:  "Docker",
		Entities:     []connector.SnapshotEntity{{Kind: "container", Name: "nginx", IP: "10.0.0.5"}},
		Dependencies: []connector.ServiceDependency{{Kind: "host", Name: "pve1"}, {Kind: "network", Name: "lan"}},
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
