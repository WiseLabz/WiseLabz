package store

import (
	"context"
	"testing"
)

func TestRunbookRunRetentionSettings(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	if _, err := s.DB().ExecContext(ctx, `
		INSERT INTO retention_settings (id, snapshot_days, doc_version_days, alert_days, sync_run_days,
			audit_days, cron_expr, updated_at)
		VALUES ('default', 90, 30, 90, 30, 180, '0 3 * * *', '2026-01-01T00:00:00Z')
	`); err != nil {
		t.Fatal(err)
	}
	rs, err := s.GetRetentionSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rs.RunbookOpenRunHours != 24 || rs.RunbookRunDays != 90 || rs.RunbookApprovalHours != DefaultRunbookApprovalHours {
		t.Fatalf("runbook retention defaults = %d open hours, %d days, %d approval hours; want 24, 90, %d", rs.RunbookOpenRunHours, rs.RunbookRunDays, rs.RunbookApprovalHours, DefaultRunbookApprovalHours)
	}
	originalSnapshotDays := rs.SnapshotDays
	for _, days := range []int{180, 0} {
		rs.RunbookOpenRunHours = 48
		rs.RunbookRunDays = days
		rs.RunbookApprovalHours = 12
		rs.UpdatedAt = ""
		if err := s.UpsertRetentionSettings(ctx, rs); err != nil {
			t.Fatal(err)
		}
		got, err := s.GetRetentionSettings(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got.RunbookOpenRunHours != 48 || got.RunbookRunDays != days || got.RunbookApprovalHours != 12 || got.UpdatedAt == "" {
			t.Fatalf("stored runbook retention = %+v; want 48 open hours, %d days, 12 approval hours and timestamp", got, days)
		}
		if got.SnapshotDays != originalSnapshotDays {
			t.Fatal("unrelated retention value changed")
		}
	}
	if _, err := s.DB().ExecContext(ctx, `DELETE FROM retention_settings WHERE id = 'default'`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertRetentionSettings(ctx, rs); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetRetentionSettings(ctx)
	if err != nil || got.RunbookRunDays != 0 || got.RunbookOpenRunHours != 48 || got.RunbookApprovalHours != 12 {
		t.Fatalf("insert missing singleton = %+v, %v", got, err)
	}
}
