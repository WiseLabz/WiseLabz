package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestJournalBackdatedEntryAmongRealEvents(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	cid := store.ConnectorRecord{Name: "Journal E2E", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
	if err := app.Store.CreateConnector(ctx, &cid); err != nil {
		t.Fatal(err)
	}
	user, token := app.user(t, "viewer")
	app.connectorGrant(t, user, cid.ID, "operator")
	c := store.ChangeRecord{ID: "recent-change", ServiceID: cid.ID, ChangeType: "config", Severity: "info", Summary: "Changed", DetectedAt: "2020-01-01T12:00:00.2Z"}
	if err := app.Store.CreateChange(ctx, &c); err != nil {
		t.Fatal(err)
	}
	run := store.SyncRunRecord{ID: "older-sync", ConnectorID: cid.ID, StartedAt: "2020-01-01T12:00:00Z", Status: store.SyncRunStatusError}
	if err := app.Store.CreateSyncRun(ctx, &run); err != nil {
		t.Fatal(err)
	}
	entry := map[string]any{"body": "Backdated router maintenance", "occurredAt": "2020-01-01T12:00:00.1Z", "connectorId": cid.ID, "entityKind": "vm", "entityName": "router", "entityRef": "vm/100"}
	rec := app.req(t, http.MethodPost, "/api/journal", entry, token)
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var e store.JournalEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	next := ""
	want := []string{c.ID, e.ID, run.ID}
	for i, id := range want {
		rec = app.req(t, http.MethodGet, "/api/v1/timeline?connectorId="+cid.ID+"&pageSize=1&cursor="+next, nil, token)
		var page struct {
			Items []store.TimelineItem `json:"items"`
			Total int                  `json:"total"`
			Next  string               `json:"nextCursor"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if rec.Code != 200 || page.Total != 3 || len(page.Items) != 1 || page.Items[0].ID != id {
			t.Fatalf("page %d: %d %s", i, rec.Code, rec.Body.String())
		}
		if i == 1 && (page.Items[0].EntityRef != "vm/100" || page.Items[0].CreatedBy != user) {
			t.Fatalf("context %+v", page.Items[0])
		}
		next = page.Next
		if (i < 2) != (next != "") {
			t.Fatalf("page %d next %q", i, next)
		}
	}
	entry["body"] = "Updated note"
	rec = app.req(t, http.MethodPut, "/api/journal/"+e.ID, entry, token)
	if rec.Code != 200 {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body.String())
	}
	rec = app.req(t, http.MethodDelete, "/api/journal/"+e.ID, nil, token)
	if rec.Code != 204 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
}
