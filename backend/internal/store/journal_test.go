package store

import (
	"context"
	"errors"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/auth"
)

func TestJournalCRUDAndLinkSurvival(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	if s.driver == "sqlite" {
		if _, err := s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
			t.Fatal(err)
		}
	}
	cid := createTestConnector(ctx, t, s)
	d := DocRecord{Title: "Linked doc", Kind: "service", ServiceID: cid, Origin: DocOriginHuman}
	if err := s.CreateDoc(ctx, &d); err != nil {
		t.Fatal(err)
	}
	e := JournalEntry{Body: "Maintenance", CreatedBy: "deleted-user", ConnectorID: cid, DocID: d.ID, EntityKind: "vm", EntityName: "router", EntityRef: "vm/100"}
	if err := s.CreateJournalEntry(ctx, &e); err != nil {
		t.Fatal(err)
	}
	if e.OccurredAt == "" || e.CreatedAt == "" {
		t.Fatal("missing default times")
	}
	e.Body = "Updated"
	if err := s.UpdateJournalEntry(ctx, &e); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM docs WHERE id = ?", d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM connectors WHERE id = ?", cid); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetJournalEntry(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != "Updated" || got.CreatedBy != "deleted-user" || got.ConnectorID != "" || got.DocID != "" || got.EntityRef != "vm/100" {
		t.Fatalf("note did not survive: %+v", got)
	}
	if err := s.DeleteJournalEntry(ctx, e.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetJournalEntry(ctx, e.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted: %v", err)
	}
}

func TestTimelineMergePagingAndPermissions(t *testing.T) {
	s := newDocTestStore(t)
	ctx := auth.ContextWithUser(context.Background(), "reader", true)
	if err := s.CreateUser(ctx, &User{ID: "reader", Username: "reader"}); err != nil {
		t.Fatal(err)
	}
	cid := createTestConnector(ctx, t, s)
	other := createTestConnector(ctx, t, s)
	if _, err := s.UpsertConnectorGrant(ctx, "reader", cid, "viewer"); err != nil {
		t.Fatal(err)
	}
	if _, err := upsertConnectorGrant(ctx, s.db, "reader", cid, "operator", "oidc"); err != nil {
		t.Fatal(err)
	}
	ts := "2026-10-03T12:00:00Z"
	c := ChangeRecord{ID: "c", ServiceID: cid, ChangeType: "updated", Severity: "info", Summary: "change", DetectedAt: ts}
	if err := s.CreateChange(ctx, &c); err != nil {
		t.Fatal(err)
	}
	for _, r := range []SyncRunRecord{
		{ID: "r1", ConnectorID: cid, StartedAt: "2026-10-03T12:00:00.1Z", Status: SyncRunStatusError},
		{ID: "r2", ConnectorID: cid, StartedAt: "2026-10-03T12:00:00.000000001Z", Status: SyncRunStatusSuccess, ChangesCount: 1},
		{ID: "quiet", ConnectorID: cid, StartedAt: ts, Status: SyncRunStatusSuccess},
		{ID: "hidden-sync", ConnectorID: other, StartedAt: ts, Status: SyncRunStatusError},
	} {
		if err := s.CreateSyncRun(ctx, &r); err != nil {
			t.Fatal(err)
		}
	}
	a := AlertRecord{ID: "a", ServiceID: cid, Title: "alert", Severity: "info", CreatedAt: ts}
	if err := s.CreateAlert(ctx, &a); err != nil {
		t.Fatal(err)
	}
	d := DocRecord{ID: "d", Title: "Doc", Kind: "service", ServiceID: cid, Origin: DocOriginHuman}
	if err := s.CreateDoc(ctx, &d); err != nil {
		t.Fatal(err)
	}
	v := DocVersionRecord{ID: "v", DocID: d.ID, Rev: 1, Trigger: "manual", CreatedAt: ts}
	if err := s.CreateDocVersion(ctx, &v); err != nil {
		t.Fatal(err)
	}
	for _, e := range []JournalEntry{
		{ID: "j", Body: "backdated", OccurredAt: ts, ConnectorID: cid},
		{ID: "lab", Body: "lab note", OccurredAt: ts},
		{ID: "hidden", Body: "secret", OccurredAt: ts, ConnectorID: other},
	} {
		if err := s.CreateJournalEntry(ctx, &e); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range []AuditRecord{
		{ID: "audit", Action: "connector.sync", TargetType: "connector", TargetID: cid, CreatedAt: ts},
		{ID: "security", Action: "auth.elevate", CreatedAt: ts},
		{ID: "future-security", Action: "connector.security_changed", TargetType: "connector", TargetID: cid, CreatedAt: ts},
	} {
		if err := s.CreateAuditRecord(ctx, &a); err != nil {
			t.Fatal(err)
		}
	}
	hiddenChange := ChangeRecord{ID: "hidden-change", ServiceID: other, ChangeType: "config", Severity: "info", Summary: "hidden", DetectedAt: ts}
	if err := s.CreateChange(ctx, &hiddenChange); err != nil {
		t.Fatal(err)
	}
	hiddenAlert := AlertRecord{ID: "hidden-alert", ServiceID: other, Title: "hidden", Severity: "info", CreatedAt: ts}
	if err := s.CreateAlert(ctx, &hiddenAlert); err != nil {
		t.Fatal(err)
	}
	hiddenDoc := DocRecord{ID: "hidden-doc", Title: "hidden", Kind: "service", ServiceID: other, Origin: DocOriginHuman}
	if err := s.CreateDoc(ctx, &hiddenDoc); err != nil {
		t.Fatal(err)
	}
	hiddenVersion := DocVersionRecord{ID: "hidden-version", DocID: hiddenDoc.ID, Rev: 1, Trigger: "manual", CreatedAt: ts}
	if err := s.CreateDocVersion(ctx, &hiddenVersion); err != nil {
		t.Fatal(err)
	}
	hiddenAudit := AuditRecord{ID: "hidden-audit", Action: "connector.sync", TargetType: "connector", TargetID: other, CreatedAt: ts}
	if err := s.CreateAuditRecord(ctx, &hiddenAudit); err != nil {
		t.Fatal(err)
	}
	f := TimelineFilter{UserID: "reader", Admin: true}
	want := []string{"r1", "r2", "lab", "j", "v", "c", "audit", "a"}
	var ids []string
	for {
		rows, total, more, err := s.ListTimeline(ctx, f, 2)
		if err != nil {
			t.Fatal(err)
		}
		if total != len(want) {
			t.Fatalf("total %d, want %d", total, len(want))
		}
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
		if !more {
			break
		}
		last := rows[len(rows)-1]
		f.Cursor = TimelineCursor{last.Timestamp, last.Kind, last.ID}
	}
	if len(ids) != len(want) {
		t.Fatalf("ids %v", ids)
	}
	for i, id := range want {
		if ids[i] != id {
			t.Fatalf("order %v, want %v", ids, want)
		}
	}
	f.Cursor = TimelineCursor{}
	f.Admin = false
	f.AllSyncRuns = true
	rows, total, _, err := s.ListTimeline(ctx, f, 100)
	if err != nil {
		t.Fatal(err)
	}
	if total != 8 {
		t.Fatalf("non-admin all sync total %d: %+v", total, rows)
	}
	restricted := auth.ContextWithAPIKeyRestriction(ctx, auth.APIKeyRestriction{ConnectorIDs: []string{other}})
	rows, total, _, err = s.ListTimeline(restricted, f, 100)
	if err != nil || total != 0 {
		t.Fatalf("key leak %v %d: %v", rows, total, err)
	}
	f.Kinds = []string{"journal"}
	f.ConnectorID = cid
	f.After = "2026-10-03T12:00:00.000000000Z"
	f.Before = f.After
	rows, total, _, err = s.ListTimeline(ctx, f, 100)
	if err != nil || total != 1 || rows[0].ID != "j" {
		t.Fatalf("filters %+v %d: %v", rows, total, err)
	}
	if _, err := s.db.ExecContext(ctx, "UPDATE docs SET deleted_at = ? WHERE id = ?", ts, d.ID); err != nil {
		t.Fatal(err)
	}
	f = TimelineFilter{UserID: "reader", Kinds: []string{"doc"}}
	rows, total, _, err = s.ListTimeline(ctx, f, 100)
	if err != nil || total != 0 {
		t.Fatalf("deleted doc %+v %d: %v", rows, total, err)
	}
}

func TestJournalSurvivesOperationalRetention(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	e := JournalEntry{Body: "Irreplaceable old context", OccurredAt: "2000-01-01T00:00:00Z", CreatedAt: "2000-01-01T00:00:00Z"}
	if err := s.CreateJournalEntry(ctx, &e); err != nil {
		t.Fatal(err)
	}
	for _, prune := range []func(context.Context, string) (int64, error){s.DeleteOldChanges, s.DeleteOldSyncRuns, s.DeleteOldAlerts, s.DeleteOldAuditRecords, s.DeleteOldDocVersions} {
		if _, err := prune(ctx, "2030-01-01T00:00:00Z"); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.GetJournalEntry(ctx, e.ID)
	if err != nil || got != e {
		t.Fatalf("retained %+v: %v", got, err)
	}
}

func TestRecipeActionJournalAuditBoundary(t *testing.T) {
	s := newDocTestStore(t)
	ctx := auth.ContextWithUser(context.Background(), "reader", true)
	cid := createTestConnector(ctx, t, s)
	if _, err := s.UpsertConnectorGrant(ctx, "reader", cid, "viewer"); err != nil {
		t.Fatal(err)
	}
	actions := []string{"connector.action", "backup.import", "runbook.run.step_resent", "runbook.run.step_marked_done", "connector.recipe_actions_changed"}
	for _, action := range actions {
		record := AuditRecord{Action: action, TargetType: "connector", TargetID: cid}
		if err := s.CreateAuditRecord(ctx, &record); err != nil {
			t.Fatal(err)
		}
	}
	rows, total, _, err := s.ListTimeline(ctx, TimelineFilter{UserID: "reader", Admin: true, Kinds: []string{"audit"}}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 {
		t.Fatalf("journal lab actions = %d, want 4", total)
	}
	for _, row := range rows {
		if row.Title == "connector.recipe_actions_changed" {
			t.Fatal("security boundary leaked into Journal")
		}
	}
}
