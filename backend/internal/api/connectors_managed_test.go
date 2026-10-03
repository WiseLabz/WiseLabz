package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func seedManagedConnector(t *testing.T, app *testApp, managedBy string) *store.ConnectorRecord {
	t.Helper()
	conn := &store.ConnectorRecord{
		Name: "svc-" + managedBy, Category: "virtualization", Type: "proxmox", URL: "https://example.com",
		Enabled: managedBy != store.ManagedByConfigOrphaned, ManagedBy: managedBy,
	}
	if err := app.Store.CreateConnector(context.Background(), conn); err != nil {
		t.Fatalf("seed connector: %v", err)
	}
	return conn
}

func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var e struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		t.Fatalf("decode error body %s: %v", body, err)
	}
	return e.Code
}

// TestConfigManagedConnectorIsLocked covers #500: a connector declared in
// config.yaml rejects every change to its settings but can still be operated.
func TestConfigManagedConnectorIsLocked(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	opID, opToken := app.user(t, "operator")
	conn := seedManagedConnector(t, app, store.ManagedByConfig)
	app.connectorGrant(t, opID, conn.ID, "operator")
	base := "/api/connectors/" + conn.ID

	rec := app.req(t, http.MethodGet, base, nil, opToken)
	var got store.ConnectorRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.ManagedBy != store.ManagedByConfig {
		t.Fatalf("GET managedBy = %q (%v), want config; body = %s", got.ManagedBy, err, rec.Body)
	}

	locked := []struct {
		name, method, path string
		body               any
	}{
		{"update", http.MethodPut, base, map[string]any{"name": "renamed"}},
		{"toggle", http.MethodPut, base + "/enabled", map[string]any{"enabled": false}},
		{"schedule", http.MethodPut, base, map[string]any{"scheduleSeconds": 60}},
		{"release", http.MethodPost, base + "/release", nil},
	}
	for _, tt := range locked {
		t.Run(tt.name, func(t *testing.T) {
			rec := app.req(t, tt.method, tt.path, tt.body, opToken)
			if rec.Code != http.StatusConflict || errorCode(t, rec.Body.Bytes()) != "connector_managed" {
				t.Fatalf("status = %d, body = %s; want 409 connector_managed", rec.Code, rec.Body)
			}
		})
	}
	t.Run("delete", func(t *testing.T) {
		rec := app.reqElevated(t, http.MethodDelete, base, nil, opToken, "connector.delete")
		if rec.Code != http.StatusConflict || errorCode(t, rec.Body.Bytes()) != "connector_managed" {
			t.Fatalf("status = %d, body = %s; want 409 connector_managed", rec.Code, rec.Body)
		}
	})

	after, err := app.Store.GetConnector(context.Background(), conn.ID)
	if err != nil || after.Name != conn.Name || !after.Enabled || after.ScheduleSeconds != nil || after.ManagedBy != store.ManagedByConfig {
		t.Fatalf("connector changed despite the lock: %+v (%v)", after, err)
	}

	t.Run("operational actions stay open", func(t *testing.T) {
		rec := app.req(t, http.MethodPost, base+"/maintenance-window", map[string]any{"durationMinutes": 30}, opToken)
		if rec.Code != http.StatusCreated {
			t.Fatalf("maintenance window: status = %d, want 201; body = %s", rec.Code, rec.Body)
		}
		rec = app.req(t, http.MethodPost, base+"/sync", nil, opToken)
		if rec.Code == http.StatusConflict {
			t.Fatalf("sync rejected for a config-managed connector: %s", rec.Body)
		}
	})
}

// TestOrphanedConnectorAllowsOnlyReleaseAndDelete covers the state a connector
// is left in once its config.yaml entry is removed.
func TestOrphanedConnectorAllowsOnlyReleaseAndDelete(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	opID, opToken := app.user(t, "operator")
	_, viewerToken := app.user(t, "viewer")

	conn := seedManagedConnector(t, app, store.ManagedByConfigOrphaned)
	app.connectorGrant(t, opID, conn.ID, "operator")
	base := "/api/connectors/" + conn.ID

	for _, tt := range []struct {
		name, method, path string
		body               any
	}{
		{"update", http.MethodPut, base, map[string]any{"name": "renamed"}},
		{"toggle", http.MethodPut, base + "/enabled", map[string]any{"enabled": true}},
		{"sync", http.MethodPost, base + "/sync", nil},
		{"test", http.MethodPost, base + "/test", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := app.req(t, tt.method, tt.path, tt.body, opToken)
			if rec.Code != http.StatusConflict || errorCode(t, rec.Body.Bytes()) != "connector_orphaned" {
				t.Fatalf("status = %d, body = %s; want 409 connector_orphaned", rec.Code, rec.Body)
			}
		})
	}

	t.Run("bulk sync skips it", func(t *testing.T) {
		rec := app.req(t, http.MethodPost, "/api/connectors/bulk-sync", map[string]any{"ids": []string{conn.ID}}, opToken)
		var out struct {
			Results []struct{ ID, Status, Reason string } `json:"results"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Results) != 1 || out.Results[0].Reason != "orphaned" {
			t.Fatalf("bulk sync = %s (%v), want one result with reason orphaned", rec.Body, err)
		}
	})

	t.Run("release needs operator", func(t *testing.T) {
		rec := app.req(t, http.MethodPost, base+"/release", nil, viewerToken)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body)
		}
	})

	t.Run("release returns it to the UI, still disabled", func(t *testing.T) {
		rec := app.req(t, http.MethodPost, base+"/release", nil, opToken)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
		}
		var got store.ConnectorRecord
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.ManagedBy != store.ManagedByUI || got.Enabled {
			t.Fatalf("released connector = %+v (%v), want managedBy ui and disabled", got, err)
		}
		rec = app.req(t, http.MethodPut, base+"/enabled", map[string]any{"enabled": true}, opToken)
		if rec.Code != http.StatusOK {
			t.Fatalf("enable after release: status = %d; body = %s", rec.Code, rec.Body)
		}
		var actions int
		if err := app.Store.DB().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM audit_log WHERE action = 'connector.release' AND target_id = ?`, conn.ID).Scan(&actions); err != nil || actions != 1 {
			t.Fatalf("connector.release audit rows = %d (%v), want 1", actions, err)
		}
	})

	t.Run("delete is allowed while orphaned", func(t *testing.T) {
		other := seedManagedConnector(t, app, store.ManagedByConfigOrphaned)
		app.connectorGrant(t, opID, other.ID, "operator")
		rec := app.reqElevated(t, http.MethodDelete, "/api/connectors/"+other.ID, nil, opToken, "connector.delete")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204; body = %s", rec.Code, rec.Body)
		}
	})
}
