package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func seedChecks(t *testing.T, app *testApp, connID string, statuses ...string) {
	t.Helper()
	now := time.Now().UTC()
	for i, st := range statuses {
		lat := int64(100 * (i + 1))
		err := app.Store.RecordHealthCheck(context.Background(), &store.HealthCheckRecord{
			ConnectorID: connID, Status: st, LatencyMs: &lat,
			CheckedAt: now.Add(-time.Duration(len(statuses)-i) * time.Minute).Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("RecordHealthCheck: %v", err)
		}
	}
}

func TestConnectorsUptimeHistory(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	viewerID, viewerToken := app.user(t, "viewer")
	conn := seedHealthTestConnector(t, app, "proxmox")
	hidden := seedHealthTestConnector(t, app, "proxmox")
	app.connectorGrant(t, viewerID, conn.ID, "viewer")
	seedChecks(t, app, conn.ID, "online", "offline")

	rec := app.req(t, http.MethodGet, "/api/connectors/"+conn.ID+"/uptime/history?window=24h", nil, viewerToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", rec.Code, rec.Body)
	}
	var got struct {
		Window  string `json:"window"`
		Buckets []struct {
			Status     string `json:"status"`
			CheckCount int    `json:"checkCount"`
		} `json:"buckets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Window != "24h" || len(got.Buckets) == 0 {
		t.Fatalf("unexpected body: %s", rec.Body)
	}
	total := 0
	worst := ""
	for _, b := range got.Buckets {
		total += b.CheckCount
		if b.Status == "offline" {
			worst = "offline"
		}
	}
	if total != 2 || worst != "offline" {
		t.Errorf("total = %d worst = %q, want 2 / offline; body = %s", total, worst, rec.Body)
	}

	if rec := app.req(t, http.MethodGet, "/api/connectors/"+conn.ID+"/uptime/history?window=1y", nil, viewerToken); rec.Code != http.StatusBadRequest {
		t.Errorf("invalid window status = %d, want 400", rec.Code)
	}
	// Same as the other viewer-gated connector routes (e.g. /syncs): 403.
	if rec := app.req(t, http.MethodGet, "/api/connectors/"+hidden.ID+"/uptime/history", nil, viewerToken); rec.Code != http.StatusForbidden {
		t.Errorf("ungranted connector status = %d, want 403", rec.Code)
	}
}

func TestFleetUptimeGrantFiltering(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	viewerID, viewerToken := app.user(t, "viewer")
	a := seedHealthTestConnector(t, app, "proxmox")
	b := seedHealthTestConnector(t, app, "proxmox")
	c := seedHealthTestConnector(t, app, "proxmox")
	app.connectorGrant(t, viewerID, a.ID, "viewer")
	app.connectorGrant(t, viewerID, b.ID, "viewer")
	seedChecks(t, app, a.ID, "online", "online")
	seedChecks(t, app, c.ID, "offline")

	rec := app.req(t, http.MethodGet, "/api/uptime?window=24h", nil, viewerToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", rec.Code, rec.Body)
	}
	var got struct {
		Window     string `json:"window"`
		Connectors []struct {
			ConnectorID     string  `json:"connectorId"`
			CheckCount      int     `json:"checkCount"`
			AvailabilityPct float64 `json:"availabilityPct"`
		} `json:"connectors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, cn := range got.Connectors {
		seen[cn.ConnectorID] = cn.CheckCount
	}
	if len(seen) != 2 {
		t.Fatalf("connectors = %v, want exactly A and B", seen)
	}
	if _, ok := seen[c.ID]; ok {
		t.Error("hidden connector leaked into fleet uptime")
	}
	if seen[a.ID] != 2 || seen[b.ID] != 0 {
		t.Errorf("check counts = %v", seen)
	}
}

func TestFleetUptimeRestrictedAPIKey(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	userID, token := app.user(t, "operator")
	a := newConnector(t, app, "a")
	b := newConnector(t, app, "b")
	app.connectorGrant(t, userID, a, "operator")
	app.connectorGrant(t, userID, b, "operator")
	seedChecks(t, app, a, "online", "online")
	seedChecks(t, app, b, "online")

	raw := createKey(t, app, token, map[string]any{"name": "one", "connectorIds": []string{a}})["token"].(string)

	fleet := func(tok string) map[string]bool {
		t.Helper()
		rec := app.req(t, http.MethodGet, "/api/uptime?window=24h", nil, tok)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d; body = %s", rec.Code, rec.Body)
		}
		var got struct {
			Connectors []struct {
				ConnectorID string `json:"connectorId"`
			} `json:"connectors"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, c := range got.Connectors {
			seen[c.ConnectorID] = true
		}
		return seen
	}
	if seen := fleet(raw); len(seen) != 1 || !seen[a] {
		t.Errorf("restricted key sees %v, want only %s", seen, a)
	}
	if seen := fleet(token); len(seen) != 2 {
		t.Errorf("unrestricted session sees %v, want both", seen)
	}
}

func TestUptimeNoDataWhenOnlyMaintenanceChecks(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	userID, token := app.user(t, "viewer")
	conn := seedHealthTestConnector(t, app, "proxmox")
	app.connectorGrant(t, userID, conn.ID, "viewer")
	now := time.Now().UTC()
	if err := app.Store.CreateMaintenanceWindow(context.Background(), &store.MaintenanceWindowRecord{
		ConnectorID: conn.ID, StartsAt: now.Add(-time.Hour).Format(time.RFC3339), EndsAt: now.Add(time.Hour).Format(time.RFC3339), CreatedBy: "u",
	}); err != nil {
		t.Fatal(err)
	}
	seedChecks(t, app, conn.ID, "online", "offline")

	rec := app.req(t, http.MethodGet, "/api/connectors/"+conn.ID+"/uptime", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", rec.Code, rec.Body)
	}
	var got struct {
		Windows map[string]struct {
			CheckCount int `json:"checkCount"`
		} `json:"windows"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	for w, st := range got.Windows {
		if st.CheckCount != 0 {
			t.Errorf("window %s checkCount = %d, want 0 (no data)", w, st.CheckCount)
		}
	}
}
