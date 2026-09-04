package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func seedChange(t *testing.T, app *testApp) *store.ChangeRecord {
	t.Helper()
	c := &store.ChangeRecord{
		ServiceID: "svc-1", ChangeType: "config", Severity: "info",
		Summary: "something changed", Diff: "[]", Status: "new", AffectedDocIDs: "[]",
	}
	if err := app.Store.CreateChange(context.Background(), c); err != nil {
		t.Fatalf("seed change: %v", err)
	}
	return c
}

func TestChangesListSuccess(t *testing.T) {
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")
	seedChange(t, app)

	rec := app.req(t, http.MethodGet, "/api/changes", nil, viewerToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}
}

func TestChangesAcknowledgeRoleBoundary(t *testing.T) {
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")
	c := seedChange(t, app)

	rec := app.req(t, http.MethodPost, "/api/changes/"+c.ID+"/ack", nil, viewerToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body)
	}
}

func TestChangesAcknowledgeSuccess(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")
	c := seedChange(t, app)

	rec := app.req(t, http.MethodPost, "/api/changes/"+c.ID+"/ack", nil, opToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}

	got, err := app.Store.GetChange(context.Background(), c.ID)
	if err != nil {
		t.Fatalf("GetChange: %v", err)
	}
	if got.Status != "acknowledged" {
		t.Errorf("Status = %q, want acknowledged", got.Status)
	}
}

func TestChangesDismissNotFound(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")

	rec := app.req(t, http.MethodPost, "/api/changes/does-not-exist/dismiss", nil, opToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body)
	}
}
