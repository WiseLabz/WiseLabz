package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestConnectorsListSuccess(t *testing.T) {
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")

	rec := app.req(t, http.MethodGet, "/api/connectors", nil, viewerToken)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}
	var got []store.ConnectorRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}

func TestConnectorsCreateRoleBoundary(t *testing.T) {
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")

	rec := app.req(t, http.MethodPost, "/api/connectors", map[string]any{
		"name": "svc", "category": "virtualization", "type": "proxmox", "url": "https://example.com",
	}, viewerToken)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body)
	}
}

func TestConnectorsCreateValidation(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")

	tests := []struct {
		name string
		body any
	}{
		{"malformed json", "not-json"},
		{"missing required fields", map[string]any{"name": "svc"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := app.req(t, http.MethodPost, "/api/connectors", tt.body, opToken)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestConnectorsCreateSuccess(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")

	rec := app.req(t, http.MethodPost, "/api/connectors", map[string]any{
		"name": "svc", "category": "virtualization", "type": "proxmox", "url": "https://example.com",
	}, opToken)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body)
	}
	var created store.ConnectorRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created connector to have an ID")
	}

	got, err := app.Store.GetConnector(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetConnector: %v", err)
	}
	if got.Name != "svc" {
		t.Errorf("Name = %q, want svc", got.Name)
	}
}

func TestConnectorsUpdateRoleBoundary(t *testing.T) {
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")

	rec := app.req(t, http.MethodPatch, "/api/connectors/some-id", map[string]any{"name": "x"}, viewerToken)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body)
	}
}

func TestConnectorsDeleteElevationBoundary(t *testing.T) {
	app := newTestApp(t)
	opID, opToken := app.user(t, "operator")
	_, viewerToken := app.user(t, "viewer")

	conn := &store.ConnectorRecord{Name: "svc", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
	if err := app.Store.CreateConnector(context.Background(), conn); err != nil {
		t.Fatalf("seed connector: %v", err)
	}

	t.Run("viewer forbidden", func(t *testing.T) {
		rec := app.req(t, http.MethodDelete, "/api/connectors/"+conn.ID, nil, viewerToken)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body)
		}
	})

	t.Run("operator missing elevation token", func(t *testing.T) {
		rec := app.req(t, http.MethodDelete, "/api/connectors/"+conn.ID, nil, opToken)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
		}
	})

	t.Run("operator invalid elevation token", func(t *testing.T) {
		req := app.newRequest(t, http.MethodDelete, "/api/connectors/"+conn.ID, nil, opToken)
		req.Header.Set("X-Elevation-Token", "garbage")
		rec := app.serve(req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401; body = %s", rec.Code, rec.Body)
		}
	})

	t.Run("operator elevation token scoped to wrong action", func(t *testing.T) {
		tok := app.elevationToken(t, opID, "user.delete")
		req := app.newRequest(t, http.MethodDelete, "/api/connectors/"+conn.ID, nil, opToken)
		req.Header.Set("X-Elevation-Token", tok)
		rec := app.serve(req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401; body = %s", rec.Code, rec.Body)
		}
	})

	t.Run("operator valid elevation token", func(t *testing.T) {
		tok := app.elevationToken(t, opID, "connector.delete")
		req := app.newRequest(t, http.MethodDelete, "/api/connectors/"+conn.ID, nil, opToken)
		req.Header.Set("X-Elevation-Token", tok)
		rec := app.serve(req)
		if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 200 or 204; body = %s", rec.Code, rec.Body)
		}

		if _, err := app.Store.GetConnector(context.Background(), conn.ID); err == nil {
			t.Fatal("expected connector to be deleted")
		}
	})
}
