package store

import (
	"context"
	"testing"
	"time"
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
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("filtered edges = %+v, want one same_as edge", got)
	}
	edgeAB := got[0]
	matchesEndpoints := edgeAB.SrcConnectorID == a && edgeAB.DstConnectorID == b ||
		edgeAB.SrcConnectorID == b && edgeAB.DstConnectorID == a
	if edgeAB.Kind != TopologyEdgeSameAs || edgeAB.ConnectorID != edgeAB.SrcConnectorID || !matchesEndpoints {
		t.Fatalf("filtered edges = %+v, %v", got, err)
	}
	if got, _ = s.ListTopologyEdges(ctx, nil); len(got) != 0 {
		t.Fatalf("empty allowed set returned %+v", got)
	}

	// Rebuilding b drops the symmetric edge that touches it, but not a's dependency edge.
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

func TestTopologyReplacementKeepsOneLogicalEdgeAcrossOwners(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	ids := createTopologyTestConnectors(t, s, "a", "b")
	a, b := ids[0], ids[1]

	contains := func(owner, name string) TopologyEdge {
		return TopologyEdge{
			ConnectorID:    owner,
			SrcConnectorID: b,
			SrcKind:        "service",
			SrcName:        "B",
			SrcRef:         b,
			DstConnectorID: b,
			DstKind:        "vm",
			DstName:        name,
			DstRef:         "stable-vm-id",
			Kind:           TopologyEdgeContains,
		}
	}
	if err := s.ReplaceTopologyEdgesForConnector(ctx, a, []TopologyEdge{contains(a, "old-name")}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceTopologyEdgesForConnector(ctx, b, []TopologyEdge{contains(b, "new-name")}); err != nil {
		t.Fatal(err)
	}
	got := topologyEdgesByKind(t, s, []string{a, b}, TopologyEdgeContains)
	if len(got) != 1 || got[0].ConnectorID != b || got[0].DstName != "new-name" {
		t.Fatalf("contains after rebuilds = %+v, want one refreshed logical edge", got)
	}
	if err := s.ReplaceTopologyEdgesForConnector(ctx, a, nil); err != nil {
		t.Fatal(err)
	}
	got = topologyEdgesByKind(t, s, []string{a, b}, TopologyEdgeContains)
	if len(got) != 1 || got[0].ConnectorID != b {
		t.Fatalf("foreign rebuild removed the surviving contains edge: %+v", got)
	}

	// Simulate old rows written by both connector rebuilds. Rebuilding either
	// side cleans up the logical edge across owners before inserting its row.
	insertLegacyTopologyTestEdge(t, s, contains(a, "legacy-name"), "legacy-owner-a")
	insertLegacyTopologyTestEdge(t, s, contains(b, "legacy-name"), "legacy-owner-b")
	if err := s.ReplaceTopologyEdgesForConnector(ctx, a, []TopologyEdge{contains(a, "new-name")}); err != nil {
		t.Fatal(err)
	}
	got = topologyEdgesByKind(t, s, []string{a, b}, TopologyEdgeContains)
	if len(got) != 1 || got[0].ConnectorID != a || got[0].DstName != "new-name" {
		t.Fatalf("contains after legacy dedupe = %+v, want one refreshed row owned by the rebuild", got)
	}
}

func TestTopologyPreservedOwnersDoNotDuplicateRefreshedSharedEdges(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	ids := createTopologyTestConnectors(t, s, "dns", "target", "matcher")
	dns, target, matcher := ids[0], ids[1], ids[2]

	sharedContains := TopologyEdge{
		ConnectorID:    matcher,
		SrcConnectorID: target,
		SrcKind:        "service",
		SrcName:        "Target",
		SrcRef:         target,
		DstConnectorID: target,
		DstKind:        "vm",
		DstName:        "web",
		DstRef:         "web-id",
		Kind:           TopologyEdgeContains,
	}
	insertLegacyTopologyTestEdge(t, s, sharedContains, "preserved-owner-contains")
	resolves := TopologyEdge{
		ConnectorID:    dns,
		SrcConnectorID: dns,
		SrcKind:        "dns_record",
		SrcName:        "app.lan",
		SrcRef:         "app.lan",
		DstConnectorID: target,
		DstKind:        "vm",
		DstName:        "web",
		DstRef:         "web-id",
		Kind:           TopologyEdgeResolvesTo,
	}
	insertLegacyTopologyTestEdge(t, s, resolves, "preserved-owner-resolves")

	sharedContains.ConnectorID = target
	if err := s.ReplaceTopologyEdgesPreserving(ctx, target, []TopologyEdge{sharedContains}, []string{matcher, dns}); err != nil {
		t.Fatal(err)
	}
	got := topologyEdgesByKind(t, s, []string{dns, target, matcher}, TopologyEdgeContains)
	if len(got) != 1 || got[0].ConnectorID != target {
		t.Fatalf("refreshed contains rows = %+v, want one row for fresh evidence", got)
	}
	got = topologyEdgesByKind(t, s, []string{dns, target, matcher}, TopologyEdgeResolvesTo)
	if len(got) != 1 || got[0].ConnectorID != dns {
		t.Fatalf("preserved resolves_to rows = %+v, want the unreadable owner's prior edge", got)
	}
}

func TestTopologySameAsHasCanonicalDirectionAndOwner(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	ids := createTopologyTestConnectors(t, s, "a", "b")
	a, b := ids[0], ids[1]

	legacy := TopologyEdge{
		ConnectorID:    b,
		SrcConnectorID: b,
		SrcKind:        "vm",
		SrcName:        "new-name",
		SrcRef:         "stable-vm-id",
		DstConnectorID: a,
		DstKind:        "container",
		DstName:        "app",
		DstRef:         "app-id",
		Kind:           TopologyEdgeSameAs,
		Source:         "hostname",
	}
	insertLegacyTopologyTestEdge(t, s, legacy, "legacy-reversed")

	// Rebuild in the reverse direction with a renamed endpoint. Ref identifies
	// the same entity, so the legacy row is replaced instead of left stale.
	updated := legacy
	updated.ConnectorID = a
	updated.SrcConnectorID, updated.DstConnectorID = a, b
	updated.SrcKind, updated.DstKind = "container", "vm"
	updated.SrcName, updated.DstName = "app", "renamed-vm"
	updated.SrcRef, updated.DstRef = "app-id", "stable-vm-id"
	if err := s.ReplaceTopologyEdgesForConnector(ctx, a, []TopologyEdge{updated}); err != nil {
		t.Fatal(err)
	}
	got := topologyEdgesByKind(t, s, []string{a, b}, TopologyEdgeSameAs)
	if len(got) != 1 {
		t.Fatalf("same_as rows = %+v, want one row after reversed legacy cleanup", got)
	}
	e := got[0]
	wantSrc, wantDst := a, b
	if b+"\x00vm\x00stable-vm-id" < a+"\x00container\x00app-id" {
		wantSrc, wantDst = b, a
	}
	if e.ConnectorID != wantSrc || e.SrcConnectorID != wantSrc || e.DstConnectorID != wantDst {
		t.Fatalf("same_as = %+v, want canonical direction and owner %s", e, wantSrc)
	}
	if e.SrcName != "renamed-vm" && e.DstName != "renamed-vm" {
		t.Fatalf("same_as retained the stale endpoint name: %+v", e)
	}
}

func TestTopologyReplacementConcurrentRebuildsDoNotDuplicateSharedEdges(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s := newDocTestStore(t)
	ids := createTopologyTestConnectors(t, s, "a", "b")
	a, b := ids[0], ids[1]
	sharedContains := TopologyEdge{
		SrcConnectorID: b, SrcKind: "service", SrcName: "B", SrcRef: b,
		DstConnectorID: b, DstKind: "vm", DstName: "web", DstRef: "web-id",
		Kind: TopologyEdgeContains,
	}
	sharedSameAs := TopologyEdge{
		SrcConnectorID: a, SrcKind: "container", SrcName: "web", SrcRef: "web-id",
		DstConnectorID: b, DstKind: "vm", DstName: "web", DstRef: "web-id",
		Kind: TopologyEdgeSameAs, Source: "IP address",
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	joined := 0
	defer func() {
		cancel()
		for joined < 2 {
			select {
			case <-results:
				joined++
			case <-time.After(time.Second):
				t.Error("concurrent topology rebuild goroutine did not stop")
				return
			}
		}
	}()
	for _, connectorID := range []string{a, b} {
		connectorID := connectorID
		go func() {
			<-start
			results <- s.ReplaceTopologyEdgesForConnector(ctx, connectorID, []TopologyEdge{sharedContains, sharedSameAs})
		}()
	}
	close(start)
	for range 2 {
		select {
		case err := <-results:
			joined++
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal("concurrent topology rebuilds did not finish")
		}
	}

	for _, kind := range []string{TopologyEdgeContains, TopologyEdgeSameAs} {
		if got := topologyEdgesByKind(t, s, []string{a, b}, kind); len(got) != 1 {
			t.Fatalf("%s rows after concurrent rebuilds = %+v, want one logical edge", kind, got)
		}
	}
}

func TestTopologyReplacementWaitsForPostgresAdvisoryLock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s := newDocTestStore(t)
	if s.driver != "postgres" {
		t.Skip("requires WISELABZ_TEST_POSTGRES_DSN")
	}
	ids := createTopologyTestConnectors(t, s, "a", "b")
	a, b := ids[0], ids[1]
	conn, err := s.rawDB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close postgres test connection: %v", err)
		}
	}()
	lockTx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lockTx.Rollback() //nolint:errcheck
	if _, err := lockTx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(731058503)`); err != nil {
		t.Fatal(err)
	}

	edge := TopologyEdge{
		SrcConnectorID: a, SrcKind: "container", SrcName: "web", SrcRef: "web-id",
		DstConnectorID: b, DstKind: "vm", DstName: "web", DstRef: "web-id",
		Kind: TopologyEdgeSameAs,
	}
	result := make(chan error, 1)
	go func() { result <- s.ReplaceTopologyEdgesForConnector(ctx, a, []TopologyEdge{edge}) }()
	joined := false
	defer func() {
		cancel()
		if joined {
			return
		}
		select {
		case <-result:
		case <-time.After(time.Second):
			t.Error("topology replacement goroutine did not stop")
		}
	}()

	waitCtx, cancelWait := context.WithTimeout(ctx, 5*time.Second)
	defer cancelWait()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		query := `
			SELECT EXISTS (
				SELECT 1 FROM pg_locks
				WHERE locktype = 'advisory' AND classid = 0 AND objid = 731058503
				  AND objsubid = 1 AND NOT granted
			)`
		err := s.rawDB.QueryRowContext(waitCtx, query).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case <-waitCtx.Done():
			t.Fatal("replacement never waited for the held PostgreSQL advisory lock")
		case <-ticker.C:
		}
	}
	select {
	case err := <-result:
		t.Fatalf("replacement completed while an independent session held the advisory lock: %v", err)
	default:
	}
	if err := lockTx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		joined = true
		if err != nil {
			t.Fatal(err)
		}
	case <-waitCtx.Done():
		t.Fatal("replacement did not continue after the PostgreSQL advisory lock was released")
	}
}

func createTopologyTestConnectors(t *testing.T, s *Store, names ...string) []string {
	t.Helper()
	ids := make([]string, 0, len(names))
	for _, name := range names {
		c := ConnectorRecord{Name: name, Category: "virtualization", Type: "proxmox", URL: "https://" + name + ".test"}
		if err := s.CreateConnector(context.Background(), &c); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, c.ID)
	}
	return ids
}

func insertLegacyTopologyTestEdge(t *testing.T, s *Store, edge TopologyEdge, id string) {
	t.Helper()
	if edge.ConnectorID == "" {
		edge.ConnectorID = edge.SrcConnectorID
	}
	_, err := s.db.ExecContext(context.Background(), `INSERT INTO topology_edges (`+topologyEdgeColumns+`) VALUES (`+placeholders(14)+`)`,
		id, edge.ConnectorID, edge.SrcConnectorID, edge.SrcKind, edge.SrcName, edge.SrcRef,
		edge.DstConnectorID, edge.DstKind, edge.DstName, edge.DstRef, edge.Kind, edge.Source, edge.Detail, "2026-10-05T00:00:00Z")
	if err != nil {
		t.Fatalf("insert legacy topology edge: %v", err)
	}
}

func topologyEdgesByKind(t *testing.T, s *Store, connectorIDs []string, kind string) []TopologyEdge {
	t.Helper()
	edges, err := s.ListTopologyEdges(context.Background(), connectorIDs)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]TopologyEdge, 0, len(edges))
	for _, edge := range edges {
		if edge.Kind == kind {
			out = append(out, edge)
		}
	}
	return out
}
