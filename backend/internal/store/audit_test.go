package store

import (
	"context"
	"testing"
	"time"
)

func TestCreateAuditRecordAndListFiltering(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)

	base := time.Now().UTC()
	if err := s.CreateAuditRecord(ctx, &AuditRecord{
		ActorUserID: "user-1", ActorRole: "operator", Action: "connector.create",
		TargetType: "connector", TargetID: "conn-1", Detail: `{"name":"svc"}`,
		CreatedAt: base.Add(-time.Minute).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("CreateAuditRecord() error: %v", err)
	}
	if err := s.CreateAuditRecord(ctx, &AuditRecord{
		ActorUserID: "user-1", ActorRole: "operator", Action: "doc.restore",
		TargetType: "doc", TargetID: "doc-1", CreatedAt: base.Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("CreateAuditRecord() error: %v", err)
	}

	records, total, err := s.ListAuditRecords(ctx, "", "", "", "", 0, 20)
	if err != nil {
		t.Fatalf("ListAuditRecords() error: %v", err)
	}
	if total != 2 || len(records) != 2 {
		t.Fatalf("ListAuditRecords() = %d/%d records, want 2/2", len(records), total)
	}
	if records[0].Action != "doc.restore" {
		t.Errorf("records[0].Action = %q, want doc.restore (newest first)", records[0].Action)
	}
	if records[0].Detail != "{}" {
		t.Errorf("records[0].Detail = %q, want default {} when unset", records[0].Detail)
	}
	if records[1].Detail != `{"name":"svc"}` {
		t.Errorf("records[1].Detail = %q, want the detail passed in", records[1].Detail)
	}

	filtered, total, err := s.ListAuditRecords(ctx, "connector.create", "", "", "", 0, 20)
	if err != nil {
		t.Fatalf("ListAuditRecords(action filter) error: %v", err)
	}
	if total != 1 || len(filtered) != 1 || filtered[0].Action != "connector.create" {
		t.Fatalf("ListAuditRecords(action filter) = %+v, want only connector.create", filtered)
	}

	filtered, total, err = s.ListAuditRecords(ctx, "", "doc", "", "", 0, 20)
	if err != nil {
		t.Fatalf("ListAuditRecords(targetType filter) error: %v", err)
	}
	if total != 1 || len(filtered) != 1 || filtered[0].TargetType != "doc" {
		t.Fatalf("ListAuditRecords(targetType filter) = %+v, want only doc target", filtered)
	}
}

func TestRecordAuditFromContextMarshalsDetail(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)

	if err := s.RecordAuditFromContext(ctx, "connector.sync", "connector", "conn-1", map[string]any{"jobId": "job-1"}); err != nil {
		t.Fatalf("RecordAuditFromContext() error: %v", err)
	}
	if err := s.RecordAuditFromContext(ctx, "connector.delete", "connector", "conn-2", nil); err != nil {
		t.Fatalf("RecordAuditFromContext(nil detail) error: %v", err)
	}

	records, total, err := s.ListAuditRecords(ctx, "", "", "", "", 0, 20)
	if err != nil {
		t.Fatalf("ListAuditRecords() error: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	// No auth context set on ctx, so actor fields fall back to zero values —
	// verifying the wiring, not auth's own context helpers.
	for _, r := range records {
		if r.ActorUserID != "" || r.ActorRole != "" {
			t.Errorf("record actor = %q/%q, want empty (no auth context in this test)", r.ActorUserID, r.ActorRole)
		}
	}
}

func TestListAuditRecordsDateRange(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)

	base := time.Now().UTC()
	if err := s.CreateAuditRecord(ctx, &AuditRecord{
		ActorUserID: "u1", ActorRole: "operator", Action: "old.action",
		CreatedAt: base.Add(-48 * time.Hour).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("CreateAuditRecord(old) error: %v", err)
	}
	if err := s.CreateAuditRecord(ctx, &AuditRecord{
		ActorUserID: "u1", ActorRole: "operator", Action: "mid.action",
		CreatedAt: base.Add(-24 * time.Hour).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("CreateAuditRecord(mid) error: %v", err)
	}
	if err := s.CreateAuditRecord(ctx, &AuditRecord{
		ActorUserID: "u1", ActorRole: "operator", Action: "new.action",
		CreatedAt: base.Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("CreateAuditRecord(new) error: %v", err)
	}

	after := base.Add(-36 * time.Hour).Format(time.RFC3339)
	_, total, err := s.ListAuditRecords(ctx, "", "", after, "", 0, 20)
	if err != nil {
		t.Fatalf("ListAuditRecords(createdAfter) error: %v", err)
	}
	if total != 2 {
		t.Fatalf("ListAuditRecords(createdAfter) total = %d, want 2 (mid + new)", total)
	}

	before := base.Add(-12 * time.Hour).Format(time.RFC3339)
	_, total, err = s.ListAuditRecords(ctx, "", "", "", before, 0, 20)
	if err != nil {
		t.Fatalf("ListAuditRecords(createdBefore) error: %v", err)
	}
	if total != 2 {
		t.Fatalf("ListAuditRecords(createdBefore) total = %d, want 2 (old + mid)", total)
	}

	records, total, err := s.ListAuditRecords(ctx, "", "", after, before, 0, 20)
	if err != nil {
		t.Fatalf("ListAuditRecords(range) error: %v", err)
	}
	if total != 1 || len(records) != 1 || records[0].Action != "mid.action" {
		t.Fatalf("ListAuditRecords(range) = %+v, want only mid.action", records)
	}
}

func TestListAllAuditRecords(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)

	for i := 0; i < 3; i++ {
		if err := s.CreateAuditRecord(ctx, &AuditRecord{
			ActorUserID: "u1", ActorRole: "operator", Action: "export.action", TargetType: "widget",
		}); err != nil {
			t.Fatalf("CreateAuditRecord() error: %v", err)
		}
	}
	if err := s.CreateAuditRecord(ctx, &AuditRecord{
		ActorUserID: "u1", ActorRole: "operator", Action: "other.action", TargetType: "gadget",
	}); err != nil {
		t.Fatalf("CreateAuditRecord(other) error: %v", err)
	}

	all, err := s.ListAllAuditRecords(ctx, "", "", "", "")
	if err != nil {
		t.Fatalf("ListAllAuditRecords() error: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("ListAllAuditRecords() = %d records, want 4 (no LIMIT)", len(all))
	}

	filtered, err := s.ListAllAuditRecords(ctx, "export.action", "widget", "", "")
	if err != nil {
		t.Fatalf("ListAllAuditRecords(filtered) error: %v", err)
	}
	if len(filtered) != 3 {
		t.Fatalf("ListAllAuditRecords(filtered) = %d records, want 3", len(filtered))
	}
	for _, r := range filtered {
		if r.Action != "export.action" || r.TargetType != "widget" {
			t.Errorf("unexpected record in filtered result: %+v", r)
		}
	}
}
