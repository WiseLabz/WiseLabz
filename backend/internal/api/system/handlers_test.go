package system

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	s := apitest.NewStore(t)
	return NewHandler(s.DB(), &config.Config{}, s, nil, t.TempDir(), nil)
}

func TestHealth(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	h.Health(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestLivenessAlwaysOK(t *testing.T) {
	h := newTestHandler(t)
	if err := h.DB.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	rr := httptest.NewRecorder()
	h.Liveness(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestReadiness(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		h := newTestHandler(t)
		rr := httptest.NewRecorder()
		h.Readiness(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
	})

	t.Run("migrations pending", func(t *testing.T) {
		h := newTestHandler(t)
		db, ok := h.DB.(*sql.DB)
		if !ok {
			t.Fatalf("DB = %T, want *sql.DB", h.DB)
		}
		if err := store.RunMigrationsDown(db, "sqlite", slog.Default()); err != nil {
			t.Fatalf("run migrations down: %v", err)
		}

		rr := httptest.NewRecorder()
		h.Readiness(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusServiceUnavailable, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), `"pending"`) {
			t.Errorf("body = %s, want pending migration status", rr.Body.String())
		}
	})

	t.Run("database unavailable", func(t *testing.T) {
		h := newTestHandler(t)
		if err := h.DB.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}

		rr := httptest.NewRecorder()
		h.Readiness(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusServiceUnavailable, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), `"database","status":"down"`) {
			t.Errorf("body = %s, want database/down", rr.Body.String())
		}
	})
}

func TestInfo(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/system/info", nil)
	rr := httptest.NewRecorder()
	h.Info(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestDiagnostics(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/system/diagnostics", nil)
	rr := httptest.NewRecorder()
	h.Diagnostics(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestListAudit(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/system/audit", nil)
	rr := httptest.NewRecorder()
	h.ListAudit(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestExportAudit(t *testing.T) {
	h := newTestHandler(t)

	t.Run("invalid format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/system/audit/export?format=xml", nil)
		rr := httptest.NewRecorder()
		h.ExportAudit(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("csv default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/system/audit/export?format=csv", nil)
		rr := httptest.NewRecorder()
		h.ExportAudit(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if !strings.Contains(rr.Body.String(), "id,actorUserId") {
			t.Errorf("expected CSV header, got: %s", rr.Body.String())
		}
	})
}

func TestGetRetentionSettingsDefault(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/system/settings/retention", nil)
	rr := httptest.NewRecorder()
	h.GetRetentionSettings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["runbookOpenRunHours"] != float64(store.DefaultRunbookOpenRunHours) {
		t.Errorf("runbookOpenRunHours = %v, want %d", resp["runbookOpenRunHours"], store.DefaultRunbookOpenRunHours)
	}
	if resp["runbookRunDays"] != float64(store.DefaultRunbookRunDays) {
		t.Errorf("runbookRunDays = %v, want %d", resp["runbookRunDays"], store.DefaultRunbookRunDays)
	}
	if resp["runbookApprovalHours"] != float64(store.DefaultRunbookApprovalHours) {
		t.Errorf("runbookApprovalHours = %v, want %d", resp["runbookApprovalHours"], store.DefaultRunbookApprovalHours)
	}
}

// assertFieldError fails unless body is an error response with the given code
// whose details name field.
func assertFieldError(t *testing.T, body []byte, code, field string) {
	t.Helper()
	var resp struct {
		Code    string `json:"code"`
		Details []struct {
			Field string `json:"field"`
			Msg   string `json:"msg"`
		} `json:"details"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal error body: %v; body=%s", err, body)
	}
	if resp.Code != code {
		t.Errorf("code = %q, want %q; body=%s", resp.Code, code, body)
	}
	for _, d := range resp.Details {
		if d.Field == field {
			return
		}
	}
	t.Errorf("details %+v do not name field %q; body=%s", resp.Details, field, body)
}

func TestUpdateRetentionSettings(t *testing.T) {
	h := newTestHandler(t)

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/system/settings/retention", strings.NewReader(`{`))
		rr := httptest.NewRecorder()
		h.UpdateRetentionSettings(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("empty cron", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/system/settings/retention", strings.NewReader(`{"cronExpr":""}`))
		rr := httptest.NewRecorder()
		h.UpdateRetentionSettings(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid cron", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/system/settings/retention", strings.NewReader(`{"cronExpr":"not a cron"}`))
		rr := httptest.NewRecorder()
		h.UpdateRetentionSettings(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("negative retention days", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/system/settings/retention", strings.NewReader(`{"cronExpr":"0 0 * * *","snapshotDays":-1}`))
		rr := httptest.NewRecorder()
		h.UpdateRetentionSettings(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("runbook open run hours bounds", func(t *testing.T) {
		for _, hours := range []int{0, -5, 8761} {
			body := fmt.Sprintf(`{"cronExpr":"0 0 * * *","runbookOpenRunHours":%d}`, hours)
			req := httptest.NewRequest(http.MethodPut, "/api/system/settings/retention", strings.NewReader(body))
			rr := httptest.NewRecorder()
			h.UpdateRetentionSettings(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("hours=%d: status = %d, want 400; body=%s", hours, rr.Code, rr.Body.String())
			}
			assertFieldError(t, rr.Body.Bytes(), "invalid_hours", "runbookOpenRunHours")
		}
	})

	t.Run("runbook approval hours bounds", func(t *testing.T) {
		for _, hours := range []int{0, -5, 8761} {
			body := fmt.Sprintf(`{"cronExpr":"0 0 * * *","runbookApprovalHours":%d}`, hours)
			req := httptest.NewRequest(http.MethodPut, "/api/system/settings/retention", strings.NewReader(body))
			rr := httptest.NewRecorder()
			h.UpdateRetentionSettings(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("hours=%d: status = %d, want 400; body=%s", hours, rr.Code, rr.Body.String())
			}
			assertFieldError(t, rr.Body.Bytes(), "invalid_hours", "runbookApprovalHours")
		}
	})

	t.Run("runbook run days bounds", func(t *testing.T) {
		for _, days := range []int{-1, 3651} {
			body := fmt.Sprintf(`{"cronExpr":"0 0 * * *","runbookRunDays":%d}`, days)
			req := httptest.NewRequest(http.MethodPut, "/api/system/settings/retention", strings.NewReader(body))
			rr := httptest.NewRecorder()
			h.UpdateRetentionSettings(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("days=%d: status = %d, want 400; body=%s", days, rr.Code, rr.Body.String())
			}
			assertFieldError(t, rr.Body.Bytes(), "invalid_days", "runbookRunDays")
		}
	})

	t.Run("runbook run days zero keeps forever", func(t *testing.T) {
		body := `{"cronExpr":"0 0 * * *","runbookRunDays":0,"runbookOpenRunHours":12,"runbookApprovalHours":6}`
		req := httptest.NewRequest(http.MethodPut, "/api/system/settings/retention", strings.NewReader(body))
		rr := httptest.NewRecorder()
		h.UpdateRetentionSettings(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp["runbookRunDays"] != float64(0) {
			t.Errorf("runbookRunDays = %v, want 0", resp["runbookRunDays"])
		}
		if resp["runbookOpenRunHours"] != float64(12) {
			t.Errorf("runbookOpenRunHours = %v, want 12", resp["runbookOpenRunHours"])
		}
		if resp["runbookApprovalHours"] != float64(6) {
			t.Errorf("runbookApprovalHours = %v, want 6", resp["runbookApprovalHours"])
		}
	})

	t.Run("happy path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/system/settings/retention", strings.NewReader(`{"cronExpr":"0 0 * * *","snapshotDays":30,"docVersionDays":60,"alertDays":90,"syncRunDays":30,"auditDays":90,"runbookOpenRunHours":48,"runbookRunDays":180}`))
		rr := httptest.NewRecorder()
		h.UpdateRetentionSettings(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp["runbookOpenRunHours"] != float64(48) {
			t.Errorf("runbookOpenRunHours = %v, want 48", resp["runbookOpenRunHours"])
		}
		if resp["runbookRunDays"] != float64(180) {
			t.Errorf("runbookRunDays = %v, want 180", resp["runbookRunDays"])
		}
	})

	t.Run("omitted deletedDocsDays keeps stored value", func(t *testing.T) {
		for _, body := range []string{
			`{"cronExpr":"0 0 * * *","deletedDocsDays":7}`,
			`{"cronExpr":"0 0 * * *","snapshotDays":30}`,
		} {
			rr := httptest.NewRecorder()
			h.UpdateRetentionSettings(rr, httptest.NewRequest(http.MethodPut, "/api/system/settings/retention", strings.NewReader(body)))
			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
			}
		}
		rs, err := h.Store.GetRetentionSettings(t.Context())
		if err != nil {
			t.Fatalf("GetRetentionSettings() error: %v", err)
		}
		if rs.DeletedDocsDays != 7 {
			t.Fatalf("DeletedDocsDays = %d, want 7", rs.DeletedDocsDays)
		}
	})

	t.Run("omitted runbook run settings keep stored values", func(t *testing.T) {
		put := func(body string) {
			t.Helper()
			rr := httptest.NewRecorder()
			h.UpdateRetentionSettings(rr, httptest.NewRequest(http.MethodPut, "/api/system/settings/retention", strings.NewReader(body)))
			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
			}
		}
		put(`{"cronExpr":"0 0 * * *","runbookOpenRunHours":48,"runbookRunDays":180,"runbookApprovalHours":6}`)
		put(`{"cronExpr":"0 0 * * *","snapshotDays":45}`)
		rs, err := h.Store.GetRetentionSettings(t.Context())
		if err != nil {
			t.Fatalf("GetRetentionSettings() error: %v", err)
		}
		if rs.SnapshotDays != 45 {
			t.Fatalf("SnapshotDays = %d, want 45", rs.SnapshotDays)
		}
		if rs.RunbookOpenRunHours != 48 || rs.RunbookRunDays != 180 || rs.RunbookApprovalHours != 6 {
			t.Fatalf("runbook settings = %d open hours / %d days / %d approval hours, want 48 / 180 / 6", rs.RunbookOpenRunHours, rs.RunbookRunDays, rs.RunbookApprovalHours)
		}
	})
}

func TestUpdateRetentionSettingsKeepsRunbookRunSettings(t *testing.T) {
	h := newTestHandler(t)
	ctx := t.Context()
	if err := h.Store.UpsertRetentionSettings(ctx, store.RetentionSettings{
		SnapshotDays: 90, DocVersionDays: 365, AlertDays: 180, SyncRunDays: 90, AuditDays: 180,
		HealthCheckDays: 90, ReportDays: 90, DeletedDocsDays: 30, CronExpr: "0 0 * * *",
		RunbookOpenRunHours: 48, RunbookRunDays: 30, RunbookApprovalHours: 36,
	}); err != nil {
		t.Fatalf("UpsertRetentionSettings() error: %v", err)
	}

	body := `{"cronExpr":"0 1 * * *","snapshotDays":30,"docVersionDays":60,"alertDays":90,"syncRunDays":30,"auditDays":90,"healthCheckDays":30,"reportDays":30,"deletedDocsDays":7}`
	rr := httptest.NewRecorder()
	h.UpdateRetentionSettings(rr, httptest.NewRequest(http.MethodPut, "/api/system/settings/retention", strings.NewReader(body)))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	rs, err := h.Store.GetRetentionSettings(ctx)
	if err != nil {
		t.Fatalf("GetRetentionSettings() error: %v", err)
	}
	if rs.SnapshotDays != 30 || rs.CronExpr != "0 1 * * *" {
		t.Fatalf("settings not updated: %+v", rs)
	}
	if rs.RunbookOpenRunHours != 48 || rs.RunbookRunDays != 30 || rs.RunbookApprovalHours != 36 {
		t.Fatalf("runbook settings = %d open hours / %d days / %d approval hours, want 48 / 30 / 36", rs.RunbookOpenRunHours, rs.RunbookRunDays, rs.RunbookApprovalHours)
	}
}

func TestGetBackupScheduleDefault(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/system/backup/schedule", nil)
	rr := httptest.NewRecorder()
	h.GetBackupSchedule(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestUpdateBackupSchedule(t *testing.T) {
	h := newTestHandler(t)

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/system/backup/schedule", strings.NewReader(`{`))
		rr := httptest.NewRecorder()
		h.UpdateBackupSchedule(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("empty cron", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/system/backup/schedule", strings.NewReader(`{"cronExpr":""}`))
		rr := httptest.NewRecorder()
		h.UpdateBackupSchedule(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid cron", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/system/backup/schedule", strings.NewReader(`{"cronExpr":"garbage"}`))
		rr := httptest.NewRecorder()
		h.UpdateBackupSchedule(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("happy path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/system/backup/schedule", strings.NewReader(`{"cronExpr":"0 3 * * *","maxBackups":7,"maxAgeHours":168,"enabled":true}`))
		rr := httptest.NewRecorder()
		h.UpdateBackupSchedule(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
	})
}

func TestExportImportBackupRoundTrip(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/system/backup/export", nil)
	rr := httptest.NewRecorder()
	h.ExportBackup(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("ExportBackup() status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	t.Run("import invalid json", func(t *testing.T) {
		importReq := httptest.NewRequest(http.MethodPost, "/api/system/backup/import", strings.NewReader(`{`))
		importRR := httptest.NewRecorder()
		h.ImportBackup(importRR, importReq)
		if importRR.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", importRR.Code, http.StatusBadRequest)
		}
	})

	t.Run("import the export back in", func(t *testing.T) {
		importReq := httptest.NewRequest(http.MethodPost, "/api/system/backup/import", strings.NewReader(rr.Body.String()))
		importRR := httptest.NewRecorder()
		h.ImportBackup(importRR, importReq)
		if importRR.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", importRR.Code, http.StatusOK, importRR.Body.String())
		}
	})
}

func TestListBackupRuns(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/system/backup/runs", nil)
	rr := httptest.NewRecorder()
	h.ListBackupRuns(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}
