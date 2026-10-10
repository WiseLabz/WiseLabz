package store

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
	if total != 9 {
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
	mustCreateUser(t, s, "reader")
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

func TestTimelineAuditAllConnectorGrantsAndPaging(t *testing.T) {
	s := newDocTestStore(t)
	ctx := auth.ContextWithUser(context.Background(), "reader", false)
	mustCreateUser(t, s, "reader")
	a := createTestConnector(ctx, t, s)
	b := createTestConnector(ctx, t, s)
	if _, err := s.UpsertConnectorGrant(ctx, "reader", a, "viewer"); err != nil {
		t.Fatal(err)
	}
	// Duplicate grant sources must not duplicate an event or alter its total.
	if _, err := upsertConnectorGrant(ctx, s.db, "reader", a, "operator", "oidc"); err != nil {
		t.Fatal(err)
	}
	ts := "2026-10-09T12:00:00Z"
	for _, record := range []AuditRecord{
		{ID: "a1", Action: "connector.sync", TargetType: "connector", TargetID: a, CreatedAt: ts},
		{ID: "a2", Action: "change.ack", TargetType: "change", TargetID: "deleted-change", ConnectorIDs: []string{a}, CreatedAt: ts},
		{ID: "b", Action: "connector.sync", TargetType: "connector", TargetID: b, CreatedAt: ts},
		{ID: "multi", Action: "runbook.update", TargetType: "runbook", TargetID: "deleted-runbook", ConnectorIDs: []string{a, b, a, ""}, CreatedAt: ts},
		{ID: "unscoped", Action: "backup.import", CreatedAt: ts},
		{ID: "security", Action: "auth.elevate", ConnectorIDs: []string{a}, CreatedAt: ts},
		{ID: "future-security", Action: "connector.future_security", TargetType: "connector", TargetID: a, CreatedAt: ts},
	} {
		if err := s.CreateAuditRecord(ctx, &record); err != nil {
			t.Fatal(err)
		}
	}
	f := TimelineFilter{UserID: "reader", Kinds: []string{"audit"}}
	assertPage := func(ctx context.Context, f TimelineFilter, want []string) {
		t.Helper()
		var got []string
		for {
			rows, total, more, err := s.ListTimeline(ctx, f, 1)
			if err != nil || total != len(want) {
				t.Fatalf("page total = %d, want %d: %v", total, len(want), err)
			}
			for _, row := range rows {
				got = append(got, row.ID)
				if row.Body != "" {
					t.Fatal("audit detail exposed")
				}
			}
			if !more {
				break
			}
			last := rows[len(rows)-1]
			f.Cursor = TimelineCursor{last.Timestamp, last.Kind, last.ID}
		}
		if !slices.Equal(got, want) {
			t.Fatalf("paged ids = %v, want %v", got, want)
		}
	}
	assertPage(ctx, f, []string{"a2", "a1"})
	// A caller-supplied admin flag cannot grant admin visibility.
	f.Admin = true
	assertPage(ctx, f, []string{"a2", "a1"})
	f.Admin = false
	if _, err := s.UpsertConnectorGrant(ctx, "reader", b, "viewer"); err != nil {
		t.Fatal(err)
	}
	assertPage(ctx, f, []string{"multi", "b", "a2", "a1"})
	rows, _, _, err := s.ListTimeline(ctx, f, 100)
	if err != nil || rows[0].ConnectorID != "" {
		t.Fatalf("multi scope must have no arbitrary connector: %+v, %v", rows, err)
	}
	f.ConnectorID = a
	assertPage(ctx, f, []string{"multi", "a2", "a1"})
	rows, _, _, err = s.ListTimeline(ctx, f, 100)
	if err != nil || rows[0].ConnectorID != a {
		t.Fatalf("filtered multi scope connector: %+v, %v", rows, err)
	}
	f.ConnectorID = ""
	restricted := auth.ContextWithAPIKeyRestriction(ctx, auth.APIKeyRestriction{ConnectorIDs: []string{a}})
	assertPage(restricted, f, []string{"a2", "a1"})
	f.ConnectorID = b
	assertPage(restricted, f, nil)
}

func TestTimelineAuditSnapshotSurvivesDeletedTargets(t *testing.T) {
	s := newDocTestStore(t)
	ctx := auth.ContextWithUser(context.Background(), "reader", false)
	mustCreateUser(t, s, "reader")
	cid := createTestConnector(ctx, t, s)
	if _, err := s.UpsertConnectorGrant(ctx, "reader", cid, "viewer"); err != nil {
		t.Fatal(err)
	}
	c := ChangeRecord{ServiceID: cid, ChangeType: "updated", Severity: "info", Status: "acknowledged", DetectedAt: "2000-01-01T00:00:00Z"}
	if err := s.CreateChange(ctx, &c); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordAuditScopedFromContext(ctx, "change.ack", "change", c.ID, map[string]string{"secret": "detail"}, []string{cid}); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"connector.sync_all", "backup.import", "runbook.delete"} {
		if err := s.RecordAuditFromContext(ctx, action, "runbook", "gone", nil); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := s.DeleteOldChanges(ctx, "2020-01-01T00:00:00Z"); err != nil || n != 1 {
		t.Fatalf("prune changes = %d, %v", n, err)
	}
	f := TimelineFilter{UserID: "reader", Kinds: []string{"audit"}}
	rows, total, _, err := s.ListTimeline(ctx, f, 100)
	if err != nil || total != 1 || rows[0].ConnectorID != cid || rows[0].CreatedBy != "reader" || rows[0].Body != "" {
		t.Fatalf("snapshot after target prune: %+v, %d, %v", rows, total, err)
	}
	// Enable cascades on SQLite so deleting the connector also removes its grants.
	if s.driver == "sqlite" {
		if _, err := s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM connectors WHERE id = ?", cid); err != nil {
		t.Fatal(err)
	}
	rows, total, _, err = s.ListTimeline(ctx, f, 100)
	if err != nil || total != 0 {
		t.Fatalf("deleted connector member rows: %+v, %d, %v", rows, total, err)
	}
	f.Admin = true
	rows, total, _, err = s.ListTimeline(auth.ContextWithUser(ctx, "reader", true), f, 100)
	if err != nil || total != 4 {
		t.Fatalf("deleted/unscoped admin rows: %+v, %d, %v", rows, total, err)
	}
	// A restricted key stays inside its connectors even for an admin owner.
	restricted := auth.ContextWithAPIKeyRestriction(auth.ContextWithUser(ctx, "reader", true), auth.APIKeyRestriction{ConnectorIDs: []string{"other"}})
	if rows, total, _, err = s.ListTimeline(restricted, f, 100); err != nil || total != 0 {
		t.Fatalf("restricted admin rows: %+v, %d, %v", rows, total, err)
	}
}

func TestAuditScopeDefaultsFailureAndRetention(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	cid := createTestConnector(ctx, t, s)
	records := []AuditRecord{
		{ID: "default", Action: "connector.sync", TargetType: "connector", TargetID: cid},
		{ID: "empty", Action: "connector.sync", TargetType: "connector", TargetID: cid, ConnectorIDs: []string{}},
		{ID: "dedup", Action: "runbook.update", ConnectorIDs: []string{cid, cid, "", "gone"}},
	}
	if err := s.CreateAuditRecords(ctx, records); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]int{"default": 1, "empty": 0, "dedup": 2} {
		var count int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_log_connectors WHERE audit_id = ?", id).Scan(&count); err != nil || count != want {
			t.Fatalf("scope %s = %d, want %d: %v", id, count, want, err)
		}
	}
	if s.driver == "sqlite" {
		// Force a failure after the first tuple would have been inserted.
		if _, err := s.db.ExecContext(ctx, `CREATE TRIGGER fail_scope BEFORE INSERT ON audit_log_connectors
 WHEN NEW.connector_id = 'fail' BEGIN SELECT RAISE(ABORT, 'scope failed'); END`); err != nil {
			t.Fatal(err)
		}
		record := AuditRecord{ID: "failed", Action: "runbook.update", ConnectorIDs: []string{cid, "fail"}}
		if err := s.CreateAuditRecord(ctx, &record); err == nil {
			t.Fatal("expected scope write failure")
		}
		var count int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_log WHERE id = 'failed'").Scan(&count); err != nil || count != 1 {
			t.Fatalf("audit row lost on scope failure: %d, %v", count, err)
		}
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_log_connectors WHERE audit_id = 'failed'").Scan(&count); err != nil || count != 0 {
			t.Fatalf("partial scope leaked: %d, %v", count, err)
		}
	}
	if _, err := s.DeleteOldAuditRecords(ctx, "2100-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_log_connectors").Scan(&count); err != nil || count != 0 {
		t.Fatalf("orphan scopes retained: %d, %v", count, err)
	}
}

func TestTimelineAuditQueryPlan(t *testing.T) {
	s := newDocTestStore(t)
	ctx := auth.ContextWithUser(context.Background(), "reader", false)
	mustCreateUser(t, s, "reader")
	a := createTestConnector(ctx, t, s)
	b := createTestConnector(ctx, t, s)
	if _, err := s.UpsertConnectorGrant(ctx, "reader", a, "viewer"); err != nil {
		t.Fatal(err)
	}
	records := make([]AuditRecord, 3000)
	for i := range records {
		ids := [][]string{{a}, {b}, {a, b}}[i%3]
		records[i] = AuditRecord{ID: fmt.Sprintf("audit-%04d", i), Action: "runbook.update", TargetType: "runbook", TargetID: "deleted-runbook",
			ConnectorIDs: ids, CreatedAt: "2026-10-09T12:00:00Z"}
	}
	if err := s.CreateAuditRecords(ctx, records); err != nil {
		t.Fatal(err)
	}
	f := TimelineFilter{UserID: "reader", Kinds: []string{"audit"}}
	items, total, more, err := s.ListTimeline(ctx, f, 2)
	if err != nil || total != 1000 || len(items) != 2 || !more {
		t.Fatalf("seeded audit page: %d rows, total %d, more %v: %v", len(items), total, more, err)
	}
	union, args := s.timelineUnion(ctx, f)
	query := "SELECT * FROM (" + union + ") timeline WHERE kind = ? ORDER BY timestamp DESC, kind DESC, id DESC LIMIT ?"
	args = append(args, "audit", 3)
	explain := "EXPLAIN QUERY PLAN "
	if s.driver == "postgres" {
		explain = "EXPLAIN (ANALYZE, BUFFERS) "
	}
	rows, err := s.db.QueryContext(ctx, explain+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck
	t.Logf("%s timeline plan: 3000 audit rows, 4000 scope rows, 1000 visible", s.driver)
	for rows.Next() {
		var detail string
		if s.driver == "postgres" {
			err = rows.Scan(&detail)
		} else {
			var id, parent, unused int
			err = rows.Scan(&id, &parent, &unused, &detail)
		}
		if err != nil {
			t.Fatal(err)
		}
		t.Log(detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
