package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func seedRunbook(t *testing.T, app *testApp, targetType, targetValue string) *store.RunbookRecord {
	t.Helper()
	rb, err := app.Store.CreateRunbook(context.Background(), &store.RunbookRecord{
		Title:       "Runbook for " + targetValue,
		Body:        "Do the thing.",
		TargetType:  targetType,
		TargetValue: targetValue,
	})
	if err != nil {
		t.Fatalf("seed runbook: %v", err)
	}
	return rb
}

func TestRunbooksListSuccess(t *testing.T) {
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")
	seedRunbook(t, app, "change_type", "vm.created")

	rec := app.req(t, http.MethodGet, "/api/runbooks", nil, viewerToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}
}

func TestRunbooksListFilterByChangeType(t *testing.T) {
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")
	seedRunbook(t, app, "change_type", "vm.created")
	seedRunbook(t, app, "alert_severity", "critical")

	rec := app.req(t, http.MethodGet, "/api/runbooks?changeType=vm.created", nil, viewerToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}

	var page struct {
		Items []store.RunbookRecord `json:"items"`
		Total int                   `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("total = %d, len(items) = %d, want 1, 1", page.Total, len(page.Items))
	}
	if page.Items[0].TargetValue != "vm.created" {
		t.Errorf("TargetValue = %q, want vm.created", page.Items[0].TargetValue)
	}
}

func TestRunbooksListRejectsBothFilters(t *testing.T) {
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")

	rec := app.req(t, http.MethodGet, "/api/runbooks?changeType=vm.created&alertSeverity=critical", nil, viewerToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
	}
}

func TestRunbooksCreateRoleBoundary(t *testing.T) {
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")

	body := map[string]any{"title": "t", "targetType": "change_type", "targetValue": "vm.created"}
	rec := app.req(t, http.MethodPost, "/api/runbooks", body, viewerToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body)
	}
}

func TestRunbooksCreateSuccess(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")

	body := map[string]any{"title": "t", "targetType": "change_type", "targetValue": "vm.created"}
	rec := app.req(t, http.MethodPost, "/api/runbooks", body, opToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body)
	}
}

func TestRunbooksGetNotFound(t *testing.T) {
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")

	rec := app.req(t, http.MethodGet, "/api/runbooks/unknown-id", nil, viewerToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body)
	}
}

func TestRunbooksUpdateNotFound(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")

	rec := app.req(t, http.MethodPut, "/api/runbooks/unknown-id", map[string]any{"title": "new"}, opToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body)
	}
}

func TestRunbooksDeleteNotFound(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")

	rec := app.req(t, http.MethodDelete, "/api/runbooks/unknown-id", nil, opToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body)
	}
}
