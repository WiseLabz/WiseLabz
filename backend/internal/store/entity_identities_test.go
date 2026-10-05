package store

import (
	"context"
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
}
