package store

import (
	"context"
	"testing"
)

func TestTopologyEdgesReplaceAndFilter(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	var ids []string
	for _, n := range []string{"a", "b", "c"} {
		c := ConnectorRecord{Name: n, Category: "virtualization", Type: "proxmox", URL: "https://" + n + ".test"}
		if err := s.CreateConnector(ctx, &c); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, c.ID)
	}
	a, b, c := ids[0], ids[1], ids[2]

	edge := func(src, dst, kind string) TopologyEdge {
		return TopologyEdge{SrcConnectorID: src, SrcKind: "vm", SrcName: "s", DstConnectorID: dst, DstKind: "vm", DstName: "d", Kind: kind}
	}
	if err := s.ReplaceTopologyEdgesForConnector(ctx, a, []TopologyEdge{edge(a, b, TopologyEdgeSameAs), edge(a, c, TopologyEdgeDependency)}); err != nil {
		t.Fatal(err)
	}

	// Only edges with both endpoints in the allowed set come back.
	got, err := s.ListTopologyEdges(ctx, []string{a, b})
	if err != nil || len(got) != 1 || got[0].DstConnectorID != b || got[0].ConnectorID != a {
		t.Fatalf("filtered edges = %+v, %v", got, err)
	}
	if got, _ = s.ListTopologyEdges(ctx, nil); len(got) != 0 {
		t.Fatalf("empty allowed set returned %+v", got)
	}

	// Rebuilding b drops the same_as edge that touches it (owned by a) but not a's dependency edge.
	if err := s.ReplaceTopologyEdgesForConnector(ctx, b, nil); err != nil {
		t.Fatal(err)
	}
	got, _ = s.ListTopologyEdges(ctx, ids)
	if len(got) != 1 || got[0].Kind != TopologyEdgeDependency {
		t.Fatalf("after rebuilding b = %+v", got)
	}

}
