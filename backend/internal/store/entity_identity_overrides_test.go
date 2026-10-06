package store

import (
	"context"
	"errors"
	"sort"
	"testing"
)

type overrideFixture struct {
	t       *testing.T
	s       *Store
	members map[string]EntityMemberRecord
	conns   []string
}

// newOverrideFixture creates n connectors and reconciles one hostname-less vm
// member per connector (ref "m<i>") so overrides can reference them.
func newOverrideFixture(t *testing.T, n int, hostname string) *overrideFixture {
	t.Helper()
	ctx := context.Background()
	f := &overrideFixture{t: t, s: newDocTestStore(t), members: map[string]EntityMemberRecord{}}
	for i := 0; i < n; i++ {
		c := ConnectorRecord{Name: "conn-" + string(rune('a'+i)), Type: "test", Category: "virtualization", URL: "https://c.test"}
		if err := f.s.CreateConnector(ctx, &c); err != nil {
			t.Fatal(err)
		}
		f.conns = append(f.conns, c.ID)
		ref := "m" + string(rune('0'+i))
		f.members[ref] = EntityMemberRecord{ConnectorID: c.ID, Kind: "vm", Ref: ref, Name: ref, Hostname: hostname}
	}
	f.reconcile()
	return f
}

// overrideClusters is the precedence the doc package applies, over a simple
// hostname match, so the store's lifecycle can be tested without the engine.
func overrideClusters(members []EntityMemberRecord, overrides []EntityIdentityOverride) [][]EntityMemberRecord {
	parent := make([]int, len(members))
	index := map[string]int{}
	detached := map[string]bool{}
	for i, m := range members {
		parent[i] = i
		index[identityMemberKey(m.ConnectorID, m.Kind, m.Ref)] = i
	}
	for _, o := range overrides {
		if o.Action == EntityOverrideDetach {
			detached[identityMemberKey(o.ConnectorID, o.Kind, o.Ref)] = true
		}
	}
	var root func(int) int
	root = func(i int) int {
		if parent[i] != i {
			parent[i] = root(parent[i])
		}
		return parent[i]
	}
	join := func(a, b int) { parent[root(b)] = root(a) }
	byHost := map[string]int{}
	for i, m := range members {
		if m.Hostname == "" || detached[identityMemberKey(m.ConnectorID, m.Kind, m.Ref)] {
			continue
		}
		if j, ok := byHost[m.Hostname]; ok {
			join(j, i)
		} else {
			byHost[m.Hostname] = i
		}
	}
	for _, o := range overrides {
		a, okA := index[identityMemberKey(o.ConnectorID, o.Kind, o.Ref)]
		b, okB := index[identityMemberKey(o.OtherConnectorID, o.OtherKind, o.OtherRef)]
		if o.Action == EntityOverrideMerge && okA && okB {
			join(a, b)
		}
	}
	groups := map[int][]EntityMemberRecord{}
	for i, m := range members {
		groups[root(i)] = append(groups[root(i)], m)
	}
	var clusters [][]EntityMemberRecord
	for _, g := range groups {
		clusters = append(clusters, g)
	}
	return clusters
}

// reconcile runs identity reconciliation over the fixture's current members,
// loading overrides inside the reconcile transaction like the engine does.
func (f *overrideFixture) reconcile() {
	f.t.Helper()
	ctx := context.Background()
	refs := make([]string, 0, len(f.members))
	for ref := range f.members {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	err := f.s.ReconcileEntityIdentitiesWith(ctx, func(ctx context.Context, tx *Store) ([][]EntityMemberRecord, []string, error) {
		overrides, err := tx.LoadEntityIdentityOverrides(ctx)
		if err != nil {
			return nil, nil, err
		}
		members := make([]EntityMemberRecord, 0, len(refs))
		for _, ref := range refs {
			members = append(members, f.members[ref])
		}
		return overrideClusters(members, overrides), nil, nil
	})
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *overrideFixture) ref(ref string) EntityIdentityOverride {
	m := f.members[ref]
	return EntityIdentityOverride{ConnectorID: m.ConnectorID, Kind: m.Kind, Ref: m.Ref}
}

func (f *overrideFixture) detach(ref string) *EntityIdentityOverride {
	f.t.Helper()
	o := f.ref(ref)
	o.Action, o.CreatedBy = EntityOverrideDetach, "admin"
	if err := f.s.CreateEntityIdentityOverride(context.Background(), &o); err != nil {
		f.t.Fatal(err)
	}
	return &o
}

func (f *overrideFixture) merge(a, b string) (*EntityIdentityOverride, error) {
	o, other := f.ref(a), f.ref(b)
	o.Action, o.CreatedBy = EntityOverrideMerge, "admin"
	o.OtherConnectorID, o.OtherKind, o.OtherRef = other.ConnectorID, other.Kind, other.Ref
	return &o, f.s.CreateEntityIdentityOverride(context.Background(), &o)
}

func (f *overrideFixture) identity(ref string) string {
	f.t.Helper()
	m := f.members[ref]
	var id string
	err := f.s.db.QueryRowContext(context.Background(), `SELECT entity_id FROM entity_members WHERE connector_id = ? AND kind = ? AND ref = ?
		ORDER BY CASE WHEN gone_at IS NULL THEN 0 ELSE 1 END LIMIT 1`, m.ConnectorID, m.Kind, m.Ref).Scan(&id)
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}

func TestEntityIdentityOverrideValidation(t *testing.T) {
	ctx := context.Background()
	f := newOverrideFixture(t, 2, "")
	other := EntityMemberRecord{ConnectorID: f.conns[0], Kind: "network", Ref: "net", Name: "net"}
	if err := f.s.ReconcileEntityIdentities(ctx, [][]EntityMemberRecord{{f.members["m0"]}, {f.members["m1"]}, {other}}); err != nil {
		t.Fatal(err)
	}

	unknown := EntityIdentityOverride{Action: EntityOverrideDetach, ConnectorID: f.conns[0], Kind: "vm", Ref: "nope", CreatedBy: "admin"}
	if err := f.s.CreateEntityIdentityOverride(ctx, &unknown); !errors.Is(err, ErrInvalidEntityOverride) {
		t.Fatalf("unknown member error = %v, want ErrInvalidEntityOverride", err)
	}
	if _, err := f.merge("m0", "m0"); !errors.Is(err, ErrInvalidEntityOverride) {
		t.Fatalf("same member twice error = %v, want ErrInvalidEntityOverride", err)
	}
	cross := f.ref("m0")
	cross.Action, cross.CreatedBy = EntityOverrideMerge, "admin"
	cross.OtherConnectorID, cross.OtherKind, cross.OtherRef = other.ConnectorID, other.Kind, other.Ref
	if err := f.s.CreateEntityIdentityOverride(ctx, &cross); !errors.Is(err, ErrInvalidEntityOverride) {
		t.Fatalf("cross-kind merge error = %v, want ErrInvalidEntityOverride", err)
	}
	missingOther := f.ref("m0")
	missingOther.Action, missingOther.CreatedBy = EntityOverrideMerge, "admin"
	if err := f.s.CreateEntityIdentityOverride(ctx, &missingOther); !errors.Is(err, ErrInvalidEntityOverride) {
		t.Fatalf("merge without second member error = %v, want ErrInvalidEntityOverride", err)
	}
	bad := f.ref("m0")
	bad.Action, bad.CreatedBy = "split", "admin"
	if err := f.s.CreateEntityIdentityOverride(ctx, &bad); !errors.Is(err, ErrInvalidEntityOverride) {
		t.Fatalf("unknown action error = %v, want ErrInvalidEntityOverride", err)
	}
	var stored int
	if err := f.s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM entity_identity_overrides`).Scan(&stored); err != nil || stored != 0 {
		t.Fatalf("rejected overrides stored %d rows (%v), want 0", stored, err)
	}

	if _, err := f.merge("m0", "m1"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.merge("m1", "m0"); !errors.Is(err, ErrConflict) {
		t.Fatalf("reversed duplicate merge error = %v, want ErrConflict", err)
	}
	f.detach("m0")
	again := f.ref("m0")
	again.Action, again.CreatedBy = EntityOverrideDetach, "admin"
	if err := f.s.CreateEntityIdentityOverride(ctx, &again); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate detach error = %v, want ErrConflict", err)
	}
}

func TestEntityIdentityOverrideMergeStoredInSortedOrder(t *testing.T) {
	ctx := context.Background()
	f := newOverrideFixture(t, 2, "")
	forward, err := f.merge("m0", "m1")
	if err != nil {
		t.Fatal(err)
	}
	reverse, err := f.merge("m1", "m0")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("reverse merge error = %v, want ErrConflict", err)
	}
	if identityMemberKey(forward.ConnectorID, forward.Kind, forward.Ref) > identityMemberKey(forward.OtherConnectorID, forward.OtherKind, forward.OtherRef) ||
		identityMemberKey(reverse.ConnectorID, reverse.Kind, reverse.Ref) != identityMemberKey(forward.ConnectorID, forward.Kind, forward.Ref) {
		t.Fatalf("merge pair not stored in sorted member-key order: %+v / %+v", forward, reverse)
	}
	if got, err := f.s.GetEntityIdentityOverride(ctx, forward.ID); err != nil || got.Action != EntityOverrideMerge || len(got.Members) != 2 {
		t.Fatalf("GetEntityIdentityOverride() = %+v, %v", got, err)
	}
}

func TestEntityIdentityOverrideListDeleteAndConnectorCascade(t *testing.T) {
	ctx := context.Background()
	f := newOverrideFixture(t, 3, "")
	merge, err := f.merge("m0", "m1")
	if err != nil {
		t.Fatal(err)
	}
	f.detach("m2")
	views, err := f.s.ListEntityIdentityOverrides(ctx)
	if err != nil || len(views) != 2 {
		t.Fatalf("ListEntityIdentityOverrides() = %d views, %v; want 2", len(views), err)
	}
	for _, v := range views {
		if v.State != EntityOverrideActive {
			t.Fatalf("override %s state = %q, want active", v.ID, v.State)
		}
		for _, m := range v.Members {
			if m.ConnectorName == "" || m.Name == "" || m.EntityID != f.identity(m.Ref) {
				t.Fatalf("member not fully resolved: %+v", m)
			}
		}
	}

	if err := f.s.DeleteEntityIdentityOverride(ctx, merge.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.s.DeleteEntityIdentityOverride(ctx, merge.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete error = %v, want ErrNotFound", err)
	}

	// Deleting a connector deletes every override that references one of its members.
	if _, err := f.merge("m0", "m1"); err != nil {
		t.Fatal(err)
	}
	if f.s.driver != "postgres" {
		if _, err := f.s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.s.DeleteConnector(ctx, f.conns[1]); err != nil {
		t.Fatal(err)
	}
	left, err := f.s.LoadEntityIdentityOverrides(ctx)
	if err != nil || len(left) != 1 || left[0].Action != EntityOverrideDetach {
		t.Fatalf("overrides after connector delete = %+v, %v; want only the detach on another connector", left, err)
	}
}

func TestEntityOverrideManualMergeLeavesRedirect(t *testing.T) {
	ctx := context.Background()
	f := newOverrideFixture(t, 2, "")
	first, second := f.identity("m0"), f.identity("m1")
	if first == second {
		t.Fatal("unmatched members share an identity")
	}
	if _, err := f.merge("m0", "m1"); err != nil {
		t.Fatal(err)
	}
	f.reconcile()
	winner, loser := first, second
	var firstSeen, otherSeen string
	_ = f.s.db.QueryRowContext(ctx, `SELECT first_seen_at FROM entities WHERE id = ?`, first).Scan(&firstSeen)
	_ = f.s.db.QueryRowContext(ctx, `SELECT first_seen_at FROM entities WHERE id = ?`, second).Scan(&otherSeen)
	if otherSeen < firstSeen || (otherSeen == firstSeen && second < first) {
		winner, loser = second, first
	}
	if f.identity("m0") != winner || f.identity("m1") != winner {
		t.Fatalf("merged members = %q, %q; want oldest identity %q", f.identity("m0"), f.identity("m1"), winner)
	}
	var target, mergedAt string
	if err := f.s.db.QueryRowContext(ctx, `SELECT merged_into, merged_at FROM entities WHERE id = ?`, loser).Scan(&target, &mergedAt); err != nil || target != winner || mergedAt == "" {
		t.Fatalf("loser redirect = %q merged_at %q (%v), want redirect to %q with merged_at", target, mergedAt, err, winner)
	}
}

func TestEntityOverrideDetachAndRemoval(t *testing.T) {
	ctx := context.Background()
	f := newOverrideFixture(t, 3, "shared.example")
	shared := f.identity("m0")
	if f.identity("m1") != shared || f.identity("m2") != shared {
		t.Fatal("hostname members do not share an identity")
	}
	override := f.detach("m1")
	f.reconcile()
	// The remaining members stay together; exactly one side keeps the old ID
	// (which one is decided by the unchanged identity-selection tie-break).
	rest := f.identity("m0")
	if f.identity("m2") != rest || f.identity("m1") == rest {
		t.Fatalf("detach split wrong: m0=%q m1=%q m2=%q", rest, f.identity("m1"), f.identity("m2"))
	}
	if (rest == shared) == (f.identity("m1") == shared) {
		t.Fatalf("old identity %q must stay on exactly one side: rest=%q detached=%q", shared, rest, f.identity("m1"))
	}

	if err := f.s.DeleteEntityIdentityOverride(ctx, override.ID); err != nil {
		t.Fatal(err)
	}
	f.reconcile()
	if f.identity("m1") != shared || f.identity("m0") != shared || f.identity("m2") != shared {
		t.Fatalf("removing the detach did not restore the cluster: %q %q %q", f.identity("m0"), f.identity("m1"), f.identity("m2"))
	}
}

func TestEntityOverrideDormantAppliesWhenMemberReturns(t *testing.T) {
	ctx := context.Background()
	f := newOverrideFixture(t, 3, "shared.example")
	override := f.detach("m1")
	f.reconcile()
	detachedID, restID := f.identity("m1"), f.identity("m0")
	if detachedID == restID {
		t.Fatal("detach did not separate the member")
	}

	returning := f.members["m1"]
	delete(f.members, "m1")
	f.reconcile()
	view, err := f.s.GetEntityIdentityOverride(ctx, override.ID)
	if err != nil || view.State != EntityOverrideDormant {
		t.Fatalf("state with missing member = %v, %v; want dormant", view, err)
	}

	f.members["m1"] = returning
	f.reconcile()
	if view, err = f.s.GetEntityIdentityOverride(ctx, override.ID); err != nil || view.State != EntityOverrideActive {
		t.Fatalf("state after member returned = %v, %v; want active", view, err)
	}
	if got := f.identity("m1"); got != detachedID || got == f.identity("m0") {
		t.Fatalf("returning member identity = %q, want its detached identity %q apart from %q", got, detachedID, f.identity("m0"))
	}
}

func TestDeleteExpiredEntityIdentitiesPurgesOrphanedOverrides(t *testing.T) {
	ctx := context.Background()
	f := newOverrideFixture(t, 3, "")
	orphan := f.detach("m0")
	if _, err := f.merge("m1", "m2"); err != nil {
		t.Fatal(err)
	}
	f.reconcile()

	// Members still have history: nothing is purged.
	if _, err := f.s.DeleteExpiredEntityIdentities(ctx, "2000-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if left, _ := f.s.LoadEntityIdentityOverrides(ctx); len(left) != 2 {
		t.Fatalf("overrides before expiry = %d, want 2", len(left))
	}

	delete(f.members, "m0")
	f.reconcile()
	if left, _ := f.s.LoadEntityIdentityOverrides(ctx); len(left) != 2 {
		t.Fatalf("a gone member's override must be kept until history expires, got %d", len(left))
	}
	removed, err := f.s.DeleteExpiredEntityIdentities(ctx, "2999-01-01T00:00:00Z")
	if err != nil || removed == 0 {
		t.Fatalf("DeleteExpiredEntityIdentities() = %d, %v; want member history removed", removed, err)
	}
	left, err := f.s.LoadEntityIdentityOverrides(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range left {
		if o.ID == orphan.ID {
			t.Fatal("override of an expired member survived the retention run")
		}
	}
	if len(left) != 1 || left[0].Action != EntityOverrideMerge {
		t.Fatalf("overrides after retention = %+v, want only the merge of members still present", left)
	}
}
