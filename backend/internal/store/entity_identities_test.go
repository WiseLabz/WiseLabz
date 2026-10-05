package store

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestReconcileEntityIdentitiesMergeSplitAndGone(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	c1 := ConnectorRecord{Name: "one", Type: "test", Category: "virtualization", URL: "https://one.test"}
	c2 := ConnectorRecord{Name: "two", Type: "test", Category: "virtualization", URL: "https://two.test"}
	for _, c := range []*ConnectorRecord{&c1, &c2} {
		if err := s.CreateConnector(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	a := EntityMemberRecord{ConnectorID: c1.ID, Kind: "vm", Ref: "1", Name: "one"}
	b := EntityMemberRecord{ConnectorID: c2.ID, Kind: "vm", Ref: "2", Name: "two"}
	if err := s.ReconcileEntityIdentities(ctx, [][]EntityMemberRecord{{a}, {b}}); err != nil {
		t.Fatal(err)
	}
	idFor := func(connectorID string) string {
		t.Helper()
		var id string
		if err := s.db.QueryRowContext(ctx, `SELECT entity_id FROM entity_members WHERE connector_id = ?`, connectorID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	first, second := idFor(c1.ID), idFor(c2.ID)
	var oldest string
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM entities WHERE id IN (?, ?) ORDER BY first_seen_at, id LIMIT 1`, first, second).Scan(&oldest); err != nil {
		t.Fatal(err)
	}
	if err := s.ReconcileEntityIdentities(ctx, [][]EntityMemberRecord{{a, b}}); err != nil {
		t.Fatal(err)
	}
	if idFor(c1.ID) != oldest || idFor(c2.ID) != oldest {
		t.Fatalf("merge did not preserve oldest identity %q", oldest)
	}
	var loserTarget string
	loser := first
	if loser == oldest {
		loser = second
	}
	if err := s.db.QueryRowContext(ctx, `SELECT merged_into FROM entities WHERE id = ?`, loser).Scan(&loserTarget); err != nil {
		t.Fatal(err)
	}
	if loserTarget != oldest {
		t.Fatalf("merged_into = %q, want %q", loserTarget, oldest)
	}
	if err := s.ReconcileEntityIdentities(ctx, [][]EntityMemberRecord{{a}, {b}}); err != nil {
		t.Fatal(err)
	}
	id1, id2 := idFor(c1.ID), idFor(c2.ID)
	if (id1 == oldest) == (id2 == oldest) {
		t.Fatalf("split must retain the old ID for one member only: %q, %q", id1, id2)
	}
	if err := s.ReconcileEntityIdentities(ctx, nil); err != nil {
		t.Fatal(err)
	}
	var gone int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM entities WHERE gone_at IS NOT NULL AND merged_into IS NULL`).Scan(&gone); err != nil {
		t.Fatal(err)
	}
	if gone != 2 {
		t.Fatalf("gone identities = %d, want 2 active identities marked gone", gone)
	}
	if err := s.ReconcileEntityIdentities(ctx, [][]EntityMemberRecord{{a}}); err != nil {
		t.Fatal(err)
	}
	if got := idFor(c1.ID); got != id1 {
		t.Fatalf("returning member identity = %q, want original %q", got, id1)
	}
	var memberGone, entityGone bool
	if err := s.db.QueryRowContext(ctx, `SELECT m.gone_at IS NOT NULL, e.gone_at IS NOT NULL FROM entity_members m JOIN entities e ON e.id=m.entity_id WHERE m.connector_id=?`, c1.ID).Scan(&memberGone, &entityGone); err != nil {
		t.Fatal(err)
	}
	if memberGone || entityGone {
		t.Fatalf("return did not clear gone flags: member=%v entity=%v", memberGone, entityGone)
	}
}

func TestReconcileEntityIdentitiesIsIdempotent(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	c := ConnectorRecord{Name: "stable", Type: "test", Category: "virtualization", URL: "https://stable.test"}
	if err := s.CreateConnector(ctx, &c); err != nil {
		t.Fatal(err)
	}
	m := EntityMemberRecord{ConnectorID: c.ID, Kind: "vm", Ref: "stable", Name: "stable", ObservedAt: "2026-01-01T00:00:00Z"}
	clusters := [][]EntityMemberRecord{{m}}
	if err := s.ReconcileEntityIdentities(ctx, clusters); err != nil {
		t.Fatal(err)
	}
	var id, firstSeen, lastSeen string
	if err := s.db.QueryRowContext(ctx, `SELECT m.entity_id, e.first_seen_at, e.last_seen_at FROM entity_members m JOIN entities e ON e.id=m.entity_id WHERE m.connector_id=?`, c.ID).Scan(&id, &firstSeen, &lastSeen); err != nil {
		t.Fatal(err)
	}
	if firstSeen != m.ObservedAt {
		t.Fatalf("first_seen_at=%q, want observation %q", firstSeen, m.ObservedAt)
	}
	if err := s.ReconcileEntityIdentities(ctx, clusters); err != nil {
		t.Fatal(err)
	}
	var gotID, gotFirstSeen, gotLastSeen string
	var memberCount, entityCount int
	if err := s.db.QueryRowContext(ctx, `SELECT m.entity_id, e.first_seen_at, e.last_seen_at FROM entity_members m JOIN entities e ON e.id=m.entity_id WHERE m.connector_id=?`, c.ID).Scan(&gotID, &gotFirstSeen, &gotLastSeen); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM entity_members WHERE connector_id=? AND gone_at IS NULL`, c.ID).Scan(&memberCount); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM entities WHERE gone_at IS NULL AND merged_into IS NULL`).Scan(&entityCount); err != nil {
		t.Fatal(err)
	}
	if gotID != id || gotFirstSeen != firstSeen || gotLastSeen != lastSeen || memberCount != 1 || entityCount != 1 {
		t.Fatalf("second reconcile changed state: id=%q firstSeen=%q lastSeen=%q members=%d entities=%d", gotID, gotFirstSeen, gotLastSeen, memberCount, entityCount)
	}
}

func TestReconcileEntityIdentitiesFlattensMergeRedirects(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	connectors := make([]ConnectorRecord, 3)
	members := make([]EntityMemberRecord, 3)
	for i := range connectors {
		connectors[i] = ConnectorRecord{Name: fmt.Sprintf("redirect-%d", i), Type: "test", Category: "virtualization", URL: fmt.Sprintf("https://redirect-%d.test", i)}
		if err := s.CreateConnector(ctx, &connectors[i]); err != nil {
			t.Fatal(err)
		}
		members[i] = EntityMemberRecord{ConnectorID: connectors[i].ID, Kind: "vm", Ref: fmt.Sprintf("vm-%d", i), Name: fmt.Sprintf("vm-%d", i), ObservedAt: fmt.Sprintf("2026-01-0%dT00:00:00Z", i+1)}
	}
	if err := s.ReconcileEntityIdentities(ctx, [][]EntityMemberRecord{{members[0]}, {members[1]}, {members[2]}}); err != nil {
		t.Fatal(err)
	}
	idFor := func(index int) string {
		t.Helper()
		var id string
		if err := s.db.QueryRowContext(ctx, `SELECT entity_id FROM entity_members WHERE connector_id=?`, connectors[index].ID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	winner := idFor(0)
	firstRedirect := idFor(2)
	if err := s.ReconcileEntityIdentities(ctx, [][]EntityMemberRecord{{members[1], members[2]}, {members[0]}}); err != nil {
		t.Fatal(err)
	}
	if firstRedirect == winner {
		t.Fatal("expected the older connector 0 identity to remain separate")
	}
	if err := s.ReconcileEntityIdentities(ctx, [][]EntityMemberRecord{{members[0], members[1], members[2]}}); err != nil {
		t.Fatal(err)
	}
	var target string
	if err := s.db.QueryRowContext(ctx, `SELECT merged_into FROM entities WHERE id=?`, firstRedirect).Scan(&target); err != nil {
		t.Fatal(err)
	}
	if target != winner {
		t.Fatalf("earlier merge redirect points to %q, want final winner %q", target, winner)
	}
	if idFor(1) != winner || idFor(2) != winner {
		t.Fatal("all merged member rows should point to final winner")
	}
}

func TestReconcileEntityIdentitiesConcurrent(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	connectors := make([]ConnectorRecord, 4)
	cluster := make([]EntityMemberRecord, 4)
	for i := range connectors {
		connectors[i] = ConnectorRecord{Name: fmt.Sprintf("concurrent-%d", i), Type: "test", Category: "virtualization", URL: fmt.Sprintf("https://%d.test", i)}
		if err := s.CreateConnector(ctx, &connectors[i]); err != nil {
			t.Fatal(err)
		}
		cluster[i] = EntityMemberRecord{ConnectorID: connectors[i].ID, Kind: "vm", Ref: fmt.Sprintf("vm-%d", i), Name: "same", ObservedAt: "2026-01-01T00:00:00Z"}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < cap(errs); i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- s.ReconcileEntityIdentities(ctx, [][]EntityMemberRecord{cluster}) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent reconcile: %v", err)
		}
	}
	var count, distinct int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(DISTINCT entity_id) FROM entity_members WHERE gone_at IS NULL`).Scan(&count, &distinct); err != nil {
		t.Fatal(err)
	}
	if count != len(cluster) || distinct != 1 {
		t.Fatalf("active members=%d identities=%d, want %d and 1", count, distinct, len(cluster))
	}
}

func TestReconcileEntityIdentitiesSplitMemberDoesNotJoinMergeLoser(t *testing.T) {
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		s := newDocTestStore(t)
		var ids []string
		for _, n := range []string{"c1", "c2", "c3"} {
			c := ConnectorRecord{Name: n, Type: "test", Category: "virtualization", URL: "https://" + n + ".test"}
			if err := s.CreateConnector(ctx, &c); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, c.ID)
		}
		mk := func(c, ref string) EntityMemberRecord {
			return EntityMemberRecord{ConnectorID: c, Kind: "vm", Ref: ref, Name: ref}
		}
		a, m1, m2 := mk(ids[0], "a"), mk(ids[1], "m1"), mk(ids[2], "m2")
		if err := s.ReconcileEntityIdentities(ctx, [][]EntityMemberRecord{{a}, {m1, m2}}); err != nil {
			t.Fatal(err)
		}
		if err := s.ReconcileEntityIdentities(ctx, [][]EntityMemberRecord{{a, m1}, {m2}}); err != nil {
			t.Fatal(err)
		}
		idOf := func(ref string) string {
			var id string
			if err := s.db.QueryRowContext(ctx, `SELECT entity_id FROM entity_members WHERE ref = ? AND gone_at IS NULL`, ref).Scan(&id); err != nil {
				t.Fatal(err)
			}
			return id
		}
		if idOf("a") != idOf("m1") || idOf("m2") == idOf("a") {
			t.Fatalf("run %d: split member landed in the wrong identity: a=%s m1=%s m2=%s", i, idOf("a"), idOf("m1"), idOf("m2"))
		}
	}
}
