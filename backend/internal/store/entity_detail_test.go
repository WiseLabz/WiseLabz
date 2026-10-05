package store

import (
	"context"
	"errors"
	"testing"
)

type entityDetailFixture struct {
	s      *Store
	c1, c2 *ConnectorRecord
}

func newEntityDetailFixture(t *testing.T) entityDetailFixture {
	t.Helper()
	s := newDocTestStore(t)
	f := entityDetailFixture{s: s, c1: &ConnectorRecord{Name: "alpha", Type: "test", Category: "virtualization", URL: "https://a.test"}, c2: &ConnectorRecord{Name: "beta", Type: "test", Category: "virtualization", URL: "https://b.test"}}
	for _, c := range []*ConnectorRecord{f.c1, f.c2} {
		if err := s.CreateConnector(context.Background(), c); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f entityDetailFixture) entity(t *testing.T, id, mergedInto string) {
	t.Helper()
	var target any
	if mergedInto != "" {
		target = mergedInto
	}
	if _, err := f.s.db.ExecContext(context.Background(), `INSERT INTO entities (id, kind, display_name, first_seen_at, last_seen_at, merged_into) VALUES (?, 'vm', ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', ?)`, id, "name-"+id, target); err != nil {
		t.Fatal(err)
	}
}

func (f entityDetailFixture) member(t *testing.T, entityID, connectorID, ref, name, goneAt string) {
	t.Helper()
	var gone any
	if goneAt != "" {
		gone = goneAt
	}
	if _, err := f.s.db.ExecContext(context.Background(), `INSERT INTO entity_members (entity_id, connector_id, kind, ref, name, gone_at) VALUES (?, ?, 'vm', ?, ?, ?)`, entityID, connectorID, ref, name, gone); err != nil {
		t.Fatal(err)
	}
}

func TestEntityDetailResolveIdentity(t *testing.T) {
	ctx := context.Background()
	f := newEntityDetailFixture(t)
	f.entity(t, "head", "")
	f.entity(t, "mid", "head")
	f.entity(t, "tail", "mid")
	got, err := f.s.ResolveEntityIdentity(ctx, "tail")
	if err != nil || got.ID != "head" || got.Name != "name-head" {
		t.Fatalf("ResolveEntityIdentity(tail) = %+v, %v; want head", got, err)
	}
	if _, err := f.s.ResolveEntityIdentity(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing id error = %v, want ErrNotFound", err)
	}
	// A redirect loop must terminate with ErrNotFound. Break the chain's end
	// to point back at its start.
	if _, err := f.s.db.ExecContext(ctx, `UPDATE entities SET merged_into = 'tail' WHERE id = 'head'`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.ResolveEntityIdentity(ctx, "tail"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("redirect loop error = %v, want ErrNotFound", err)
	}
}

func TestEntityDetailMembersActiveFirstWithConnectorName(t *testing.T) {
	ctx := context.Background()
	f := newEntityDetailFixture(t)
	f.entity(t, "e1", "")
	f.member(t, "e1", f.c1.ID, "gone-ref", "Aardvark", "2026-09-01T00:00:00Z")
	f.member(t, "e1", f.c2.ID, "live-ref", "Zebra", "")
	ids, err := f.s.ListEntityMemberConnectorIDs(ctx, "e1")
	if err != nil || len(ids) != 2 {
		t.Fatalf("connector ids = %v, %v", ids, err)
	}
	members, err := f.s.ListEntityMembers(ctx, "e1", []string{f.c1.ID, f.c2.ID})
	if err != nil || len(members) != 2 {
		t.Fatalf("members = %+v, %v", members, err)
	}
	if members[0].Ref != "live-ref" || members[0].ConnectorName != "beta" || members[1].GoneAt == "" || members[1].ConnectorName != "alpha" {
		t.Fatalf("members not active-first with connector names: %+v", members)
	}
	doc := &DocRecord{Title: "alpha doc", Kind: "service", ServiceID: f.c1.ID, Content: "x"}
	if err := f.s.CreateDoc(ctx, doc); err != nil {
		t.Fatal(err)
	}
	withDoc, err := f.s.ListEntityMembers(ctx, "e1", []string{f.c1.ID, f.c2.ID})
	if err != nil || withDoc[0].DocID != "" || withDoc[1].DocID != doc.ID {
		t.Fatalf("connector doc ids = %+v, %v; want alpha's doc only", withDoc, err)
	}
	only, err := f.s.ListEntityMembers(ctx, "e1", []string{f.c1.ID})
	if err != nil || len(only) != 1 || only[0].Ref != "gone-ref" {
		t.Fatalf("grant-filtered members = %+v, %v", only, err)
	}
	if empty, err := f.s.ListEntityMembers(ctx, "e1", nil); err != nil || len(empty) != 0 {
		t.Fatalf("empty connector list = %+v, %v", empty, err)
	}
}

func TestEntityDetailEdgesPredicateAndEndpointEntities(t *testing.T) {
	ctx := context.Background()
	f := newEntityDetailFixture(t)
	f.entity(t, "e1", "")
	f.entity(t, "e2", "")
	f.member(t, "e1", f.c1.ID, "a", "A", "")
	f.member(t, "e2", f.c2.ID, "b", "B", "")
	edges := []TopologyEdge{
		{SrcConnectorID: f.c1.ID, SrcKind: "vm", SrcName: "A", SrcRef: "a", DstConnectorID: f.c2.ID, DstKind: "vm", DstName: "B", DstRef: "b", Kind: "dependency", Source: "x"},
		{SrcConnectorID: f.c1.ID, SrcKind: "vm", SrcName: "Other", SrcRef: "other", DstConnectorID: f.c2.ID, DstKind: "vm", DstName: "B", DstRef: "b", Kind: "dependency", Source: "unrelated"},
		{SrcConnectorID: f.c2.ID, SrcKind: "vm", SrcName: "B", SrcRef: "b", DstConnectorID: f.c1.ID, DstKind: "vm", DstName: "A", DstRef: "a", Kind: TopologyEdgeSameAs, Source: "IP address"},
		{SrcConnectorID: f.c1.ID, SrcKind: "vm", SrcName: "A", SrcRef: "a", DstConnectorID: f.c2.ID, DstKind: "vm", DstName: "B", DstRef: "b", Kind: "proxies_to", Source: "caddy", Detail: "/api -> :8080"},
	}
	if err := f.s.ReplaceTopologyEdgesForConnector(ctx, f.c1.ID, edges); err != nil {
		t.Fatal(err)
	}
	member := []EntityMemberKey{{ConnectorID: f.c1.ID, Kind: "vm", Ref: "a"}}
	got, err := f.s.ListEntityEdges(ctx, member, []string{f.c1.ID, f.c2.ID}, 10)
	if err != nil || len(got) != 3 {
		t.Fatalf("edges = %+v, %v; want the three edges touching a", got, err)
	}
	var sawDetail bool
	for _, e := range got {
		if e.Kind == "proxies_to" {
			sawDetail = e.Detail == "/api -> :8080"
		}
		if e.Source == "unrelated" {
			t.Fatalf("edge not touching the member returned: %+v", e)
		}
		if (e.Src.Ref == "b" && e.Src.EntityID != "e2") || (e.Dst.Ref == "b" && e.Dst.EntityID != "e2") || (e.Src.Ref == "a" && e.Src.EntityID != "e1") {
			t.Fatalf("endpoint entity ids wrong: %+v", e)
		}
	}
	if !sawDetail {
		t.Fatalf("proxies_to edge detail not returned: %+v", got)
	}
	// A hidden connector on either end drops the edge.
	if hidden, err := f.s.ListEntityEdges(ctx, member, []string{f.c1.ID}, 10); err != nil || len(hidden) != 0 {
		t.Fatalf("edges with hidden endpoint = %+v, %v", hidden, err)
	}
	if limited, err := f.s.ListEntityEdges(ctx, member, []string{f.c1.ID, f.c2.ID}, 1); err != nil || len(limited) != 1 {
		t.Fatalf("limited edges = %+v, %v", limited, err)
	}
	if none, err := f.s.ListEntityEdges(ctx, nil, []string{f.c1.ID}, 10); err != nil || len(none) != 0 {
		t.Fatalf("no members = %+v, %v", none, err)
	}
}

func TestEntityDetailFindingsOpenOnlyAndConnectorLevel(t *testing.T) {
	ctx := context.Background()
	f := newEntityDetailFixture(t)
	for _, fi := range []QualityFindingRecord{
		{ConnectorID: f.c1.ID, CheckType: "compliance", Severity: "warning", Title: "open", EntityKind: "vm", EntityRef: "a"},
		{ConnectorID: f.c1.ID, CheckType: "config_drift", Severity: "warning", Title: "resolved", EntityKind: "vm", EntityRef: "a"},
		{ConnectorID: f.c1.ID, CheckType: "compliance", Severity: "warning", Title: "other entity", EntityKind: "vm", EntityRef: "z"},
		{ConnectorID: f.c1.ID, CheckType: "stale", Severity: "info", Title: "connector level"},
		{ConnectorID: f.c2.ID, CheckType: "stale", Severity: "info", Title: "other connector level"},
	} {
		fi := fi
		if err := f.s.UpsertQualityFinding(ctx, &fi); err != nil {
			t.Fatal(err)
		}
		if fi.CheckType == "config_drift" {
			if _, err := f.s.db.ExecContext(ctx, `UPDATE quality_findings SET status = 'resolved', resolved_at = '2026-09-01T00:00:00Z' WHERE id = ?`, fi.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	got, err := f.s.ListOpenEntityFindings(ctx, []EntityMemberKey{{ConnectorID: f.c1.ID, Kind: "vm", Ref: "a"}}, 10)
	if err != nil || len(got) != 1 || got[0].Title != "open" || got[0].Status != "open" {
		t.Fatalf("entity findings = %+v, %v; want only the open exact match", got, err)
	}
	level, err := f.s.ListOpenConnectorLevelFindings(ctx, []string{f.c1.ID}, 10)
	if err != nil || len(level) != 1 || level[0].Title != "connector level" {
		t.Fatalf("connector-level findings = %+v, %v", level, err)
	}
	if none, err := f.s.ListOpenConnectorLevelFindings(ctx, nil, 10); err != nil || len(none) != 0 {
		t.Fatalf("empty connectors = %+v, %v", none, err)
	}
}

func TestEntityDetailRunbookStepsOrderedWithoutDistinct(t *testing.T) {
	ctx := context.Background()
	f := newEntityDetailFixture(t)
	mk := func(title, value string, steps ...*RunbookStepRecord) {
		t.Helper()
		if _, _, err := f.s.CreateRunbookWithSteps(ctx, &RunbookRecord{Title: title, Body: "body " + title, TargetType: "change_type", TargetValue: value}, steps); err != nil {
			t.Fatal(err)
		}
	}
	mk("B runbook", "v1", &RunbookStepRecord{Title: "restart a", ConnectorID: f.c1.ID, Verb: "restart", EntityRef: "a"})
	mk("A runbook", "v2",
		&RunbookStepRecord{Title: "second", ConnectorID: f.c1.ID, Verb: "stop", EntityRef: "a"},
		&RunbookStepRecord{Title: "first", ConnectorID: f.c1.ID, Verb: "start", EntityRef: "a"},
		&RunbookStepRecord{Title: "elsewhere", ConnectorID: f.c2.ID, Verb: "start", EntityRef: "a"})
	got, err := f.s.ListEntityRunbookSteps(ctx, []EntityMemberKey{{ConnectorID: f.c1.ID, Kind: "vm", Ref: "a"}}, 10)
	if err != nil || len(got) != 3 {
		t.Fatalf("steps = %+v, %v; want 3", got, err)
	}
	if got[0].RunbookTitle != "A runbook" || got[0].StepTitle != "second" || got[1].StepTitle != "first" || got[2].RunbookTitle != "B runbook" {
		t.Fatalf("steps not ordered by runbook title then position: %+v", got)
	}
	if limited, err := f.s.ListEntityRunbookSteps(ctx, []EntityMemberKey{{ConnectorID: f.c1.ID, Kind: "vm", Ref: "a"}}, 2); err != nil || len(limited) != 2 {
		t.Fatalf("limited steps = %+v, %v", limited, err)
	}
}
