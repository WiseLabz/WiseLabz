package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func seedAlert(t *testing.T, app *testApp) *store.AlertRecord {
	t.Helper()
	a := &store.AlertRecord{
		ServiceID: "svc-1", Severity: "info", Title: "alert", Description: "desc", Status: "pending",
	}
	if err := app.Store.CreateAlert(context.Background(), a); err != nil {
		t.Fatalf("seed alert: %v", err)
	}
	return a
}

func TestAlertsListSuccess(t *testing.T) {
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")
	seedAlert(t, app)

	rec := app.req(t, http.MethodGet, "/api/alerts", nil, viewerToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}
}

func TestAlertsResolveRoleBoundary(t *testing.T) {
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")
	a := seedAlert(t, app)

	rec := app.req(t, http.MethodPost, "/api/alerts/"+a.ID+"/resolve", nil, viewerToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body)
	}
}

func TestAlertsResolveSuccess(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")
	a := seedAlert(t, app)

	rec := app.req(t, http.MethodPost, "/api/alerts/"+a.ID+"/resolve", nil, opToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}

	got, err := app.Store.GetAlert(context.Background(), a.ID)
	if err != nil {
		t.Fatalf("GetAlert: %v", err)
	}
	if got.Status != "resolved" {
		t.Errorf("Status = %q, want resolved", got.Status)
	}
}

func TestAlertsSnoozeValidation(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")
	a := seedAlert(t, app)

	tests := []struct {
		name string
		body any
	}{
		{"missing until", map[string]any{}},
		{"non-RFC3339 until", map[string]any{"until": "tomorrow"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := app.req(t, http.MethodPost, "/api/alerts/"+a.ID+"/snooze", tt.body, opToken)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestAlertsSnoozeSuccess(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")
	a := seedAlert(t, app)

	rec := app.req(t, http.MethodPost, "/api/alerts/"+a.ID+"/snooze", map[string]any{"until": "2099-01-01T00:00:00Z"}, opToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}
}
