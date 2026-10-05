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

func TestTopologyEdgesResolvesToOwnershipAndDetail(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	var ids []string
	for _, n := range []string{"dns", "vm", "other"} {
		c := ConnectorRecord{Name: n, Category: "virtualization", Type: "proxmox", URL: "https://" + n + ".test"}
		if err := s.CreateConnector(ctx, &c); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, c.ID)
	}
	dns, vm, other := ids[0], ids[1], ids[2]
	resolves := func(owner, dst string) TopologyEdge {
		return TopologyEdge{ConnectorID: owner, SrcConnectorID: dns, SrcKind: "dns_record", SrcName: "app.lan", DstConnectorID: dst, DstKind: "vm", DstName: "web", Kind: TopologyEdgeResolvesTo, Detail: "443"}
	}
	// The vm connector's rebuild writes a dns-owned edge; the owner is kept.
	if err := s.ReplaceTopologyEdgesForConnector(ctx, vm, []TopologyEdge{resolves(dns, vm)}); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListTopologyEdges(ctx, ids)
	if err != nil || len(got) != 1 || got[0].ConnectorID != dns || got[0].Detail != "443" {
		t.Fatalf("edges = %+v, %v; want one dns-owned edge with its detail", got, err)
	}
	// A resolves_to edge to an unrelated connector survives the vm rebuild...
	if err := s.ReplaceTopologyEdgesForConnector(ctx, other, []TopologyEdge{resolves(dns, other)}); err != nil {
		t.Fatal(err)
	}
	// ...and the vm rebuild drops only the edge pointing at vm.
	if err := s.ReplaceTopologyEdgesForConnector(ctx, vm, nil); err != nil {
		t.Fatal(err)
	}
	got, _ = s.ListTopologyEdges(ctx, ids)
	if len(got) != 1 || got[0].DstConnectorID != other {
		t.Fatalf("after vm rebuild = %+v, want only the edge to the other connector", got)
	}
	// The dns connector's own rebuild removes whatever it owns.
	if err := s.ReplaceTopologyEdgesForConnector(ctx, dns, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.ListTopologyEdges(ctx, ids); len(got) != 0 {
		t.Fatalf("after dns rebuild = %+v, want none", got)
	}
}

func TestTopologyEdgesFingerprint(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	var ids []string
	for _, n := range []string{"a", "b"} {
		c := ConnectorRecord{Name: n, Category: "virtualization", Type: "proxmox", URL: "https://" + n + ".test"}
		if err := s.CreateConnector(ctx, &c); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, c.ID)
	}
	a, b := ids[0], ids[1]
	same := func(owner, src, dst string) TopologyEdge {
		return TopologyEdge{ConnectorID: owner, SrcConnectorID: src, SrcKind: "vm", SrcName: "x", DstConnectorID: dst, DstKind: "vm", DstName: "y", Kind: TopologyEdgeSameAs, Source: "IP address"}
	}
	fp := func() string {
		t.Helper()
		v, err := s.TopologyEdgesFingerprint(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	empty := fp()
	if err := s.ReplaceTopologyEdgesForConnector(ctx, a, []TopologyEdge{same(a, a, b)}); err != nil {
		t.Fatal(err)
	}
	first := fp()
	if first == empty {
		t.Fatal("fingerprint ignored a new edge")
	}
	// Re-deriving the same logical edge: fresh row ID and timestamp.
	if err := s.ReplaceTopologyEdgesForConnector(ctx, a, []TopologyEdge{same(a, a, b)}); err != nil {
		t.Fatal(err)
	}
	if fp() != first {
		t.Fatal("fingerprint changed although the edge set did not")
	}
	// The other side re-derives the symmetric same_as in the opposite direction and as its owner.
	if err := s.ReplaceTopologyEdgesForConnector(ctx, b, []TopologyEdge{{
		SrcConnectorID: a, SrcKind: "vm", SrcName: "x", DstConnectorID: b, DstKind: "vm", DstName: "y", Kind: TopologyEdgeSameAs, Source: "IP address",
	}}); err != nil {
		t.Fatal(err)
	}
	flipped := TopologyEdge{ConnectorID: b, SrcConnectorID: b, SrcKind: "vm", SrcName: "y", DstConnectorID: a, DstKind: "vm", DstName: "x", Kind: TopologyEdgeSameAs, Source: "IP address"}
	if err := s.ReplaceTopologyEdgesForConnector(ctx, b, []TopologyEdge{flipped}); err != nil {
		t.Fatal(err)
	}
	if fp() != first {
		t.Fatal("fingerprint changed when a symmetric same_as flipped direction and owner")
	}
	// A different detail is a different edge.
	d := flipped
	d.Kind, d.Detail = TopologyEdgeProxiesTo, "8443"
	if err := s.ReplaceTopologyEdgesForConnector(ctx, b, []TopologyEdge{d}); err != nil {
		t.Fatal(err)
	}
	withDetail := fp()
	d.Detail = "9000"
	if err := s.ReplaceTopologyEdgesForConnector(ctx, b, []TopologyEdge{d}); err != nil {
		t.Fatal(err)
	}
	if fp() == withDetail {
		t.Fatal("fingerprint ignored a changed detail")
	}
}
