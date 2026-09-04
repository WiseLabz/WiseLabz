package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func seedDoc(t *testing.T, app *testApp) *store.DocRecord {
	t.Helper()
	d := &store.DocRecord{Title: "Doc", Kind: "service", ServiceID: "svc-1", Content: "hello"}
	if err := app.Store.CreateDoc(context.Background(), d); err != nil {
		t.Fatalf("seed doc: %v", err)
	}
	return d
}

func TestDocsListAndGetSuccess(t *testing.T) {
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")
	d := seedDoc(t, app)

	rec := app.req(t, http.MethodGet, "/api/docs", nil, viewerToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body = %s", rec.Code, rec.Body)
	}

	rec = app.req(t, http.MethodGet, "/api/docs/"+d.ID, nil, viewerToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200; body = %s", rec.Code, rec.Body)
	}
}

func TestDocsSaveRoleBoundary(t *testing.T) {
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")
	d := seedDoc(t, app)

	rec := app.req(t, http.MethodPut, "/api/docs/"+d.ID, map[string]any{"content": "x"}, viewerToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body)
	}
}

func TestDocsSaveValidation(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")
	d := seedDoc(t, app)

	rec := app.req(t, http.MethodPut, "/api/docs/"+d.ID, "not-json", opToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
	}
}

func TestDocsSaveSuccess(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")
	d := seedDoc(t, app)

	rec := app.req(t, http.MethodPut, "/api/docs/"+d.ID, map[string]any{"content": "updated content"}, opToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}

	got, err := app.Store.GetDoc(context.Background(), d.ID)
	if err != nil {
		t.Fatalf("GetDoc: %v", err)
	}
	if got.Content != "updated content" {
		t.Errorf("Content = %q, want %q", got.Content, "updated content")
	}
}
