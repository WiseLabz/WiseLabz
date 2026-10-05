package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestEntityDetailAuthorizationRedactionRedirectsAndGone(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	userID, token := app.user(t, "viewer")
	keyOwnerID, _ := app.user(t, "viewer")
	_, noAccessToken := app.user(t, "viewer")
	c1, c2 := entityTestConnector(t, app, "visible connector"), entityTestConnector(t, app, "hidden connector")
	app.connectorGrant(t, userID, c1.ID, "viewer")
	app.connectorGrant(t, keyOwnerID, c1.ID, "viewer")
	app.connectorGrant(t, keyOwnerID, c2.ID, "viewer")

	seedEntity(t, app, "entity-visible", "stored hidden name", "vm", c1.ID, "visible-ref", "Visible name", "")
	seedEntityMember(t, app, "entity-visible", c2.ID, "vm", "hidden-ref", "Secret name", "")
	seedEntity(t, app, "gone-entity", "Gone name", "vm", c1.ID, "gone-ref", "Gone name", "2026-09-01T00:00:00Z")
	seedEntity(t, app, "hidden-target", "Secret target", "vm", c2.ID, "secret-target-ref", "Secret target", "")
	seedIdentity(t, app, "redirect-visible", "vm", "entity-visible")
	seedIdentity(t, app, "redirect-hidden", "vm", "hidden-target")

	// A one-connector viewer sees the identity through c1, with no evidence
	// that its second member or stored display name came from c2.
	rec := app.req(t, http.MethodGet, "/api/entities/entity-visible", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("visible member: status %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	members := body["members"].([]any)
	if len(members) != 1 || body["name"] != "Visible name" || strings.Contains(rec.Body.String(), "Secret name") || strings.Contains(rec.Body.String(), c2.ID) {
		t.Fatalf("hidden member leaked or visible identity derived incorrectly: %s", rec.Body.String())
	}

	if got := app.req(t, http.MethodGet, "/api/entities/entity-visible", nil, noAccessToken).Code; got != http.StatusNotFound {
		t.Fatalf("no-grant status = %d, want 404", got)
	}
	if got := app.req(t, http.MethodGet, "/api/entities/redirect-visible", nil, token).Code; got != http.StatusOK {
		t.Fatalf("redirect to visible target status = %d, want 200", got)
	}
	if got := app.req(t, http.MethodGet, "/api/entities/redirect-hidden", nil, token).Code; got != http.StatusNotFound {
		t.Fatalf("redirect to hidden target status = %d, want 404", got)
	}

	key := mintAPIKey(t, app, keyOwnerID, "full", []string{c1.ID})
	keyRec := app.req(t, http.MethodGet, "/api/entities/entity-visible", nil, key)
	if keyRec.Code != http.StatusOK || strings.Contains(keyRec.Body.String(), c2.ID) || strings.Contains(keyRec.Body.String(), "Secret name") {
		t.Fatalf("connector-restricted key response: status=%d body=%s", keyRec.Code, keyRec.Body.String())
	}

	gone := app.req(t, http.MethodGet, "/api/entities/gone-entity", nil, token)
	if gone.Code != http.StatusOK || !strings.Contains(gone.Body.String(), `"gone":true`) {
		t.Fatalf("gone entity banner state: status=%d body=%s", gone.Code, gone.Body.String())
	}
}

func entityTestConnector(t *testing.T, app *testApp, name string) *store.ConnectorRecord {
	t.Helper()
	c := &store.ConnectorRecord{Name: name, Type: "test", Category: "networking", URL: "https://example.com"}
	if err := app.Store.CreateConnector(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	return c
}

func seedIdentity(t *testing.T, app *testApp, id, kind, mergedInto string) {
	t.Helper()
	var target any
	if mergedInto != "" {
		target = mergedInto
	}
	_, err := app.Store.DB().ExecContext(context.Background(), `INSERT INTO entities (id, kind, display_name, first_seen_at, last_seen_at, merged_into) VALUES (?, ?, ?, ?, ?, ?)`, id, kind, "internal name", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", target)
	if err != nil {
		t.Fatal(err)
	}
}

func seedEntity(t *testing.T, app *testApp, id, displayName, kind, connectorID, ref, name, goneAt string) {
	t.Helper()
	var gone any
	if goneAt != "" {
		gone = goneAt
	}
	_, err := app.Store.DB().ExecContext(context.Background(), `INSERT INTO entities (id, kind, display_name, first_seen_at, last_seen_at, gone_at) VALUES (?, ?, ?, ?, ?, ?)`, id, kind, displayName, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", gone)
	if err != nil {
		t.Fatal(err)
	}
	seedEntityMember(t, app, id, connectorID, kind, ref, name, goneAt)
}

func seedEntityMember(t *testing.T, app *testApp, id, connectorID, kind, ref, name, goneAt string) {
	t.Helper()
	var gone any
	if goneAt != "" {
		gone = goneAt
	}
	_, err := app.Store.DB().ExecContext(context.Background(), `INSERT INTO entity_members (entity_id, connector_id, kind, ref, name, gone_at) VALUES (?, ?, ?, ?, ?, ?)`, id, connectorID, kind, ref, name, gone)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEntityDetailHistoryUsesSnapshotEntityChanges(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	userID, token := app.user(t, "viewer")
	c := entityTestConnector(t, app, "history connector")
	app.connectorGrant(t, userID, c.ID, "viewer")
	seedEntity(t, app, "history-entity", "VM", "vm", c.ID, "42", "VM", "")
	for _, tc := range []struct {
		at      string
		enabled bool
	}{{"2026-09-01T00:00:00Z", false}, {"2026-09-02T00:00:00Z", true}} {
		fetchedAt, err := time.Parse(time.RFC3339, tc.at)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := json.Marshal(connector.ServiceSnapshot{ServiceName: c.Name, Type: c.Type, FetchedAt: fetchedAt, Entities: []connector.SnapshotEntity{{Kind: "vm", Name: "VM", ExternalID: "42", Attributes: map[string]any{"enabled": tc.enabled}}}})
		if err != nil {
			t.Fatal(err)
		}
		if err := app.Store.CreateSnapshot(context.Background(), &store.SnapshotRecord{ConnectorID: c.ID, Data: string(snapshot), FetchedAt: tc.at}); err != nil {
			t.Fatal(err)
		}
	}
	rec := app.req(t, http.MethodGet, "/api/entities/history-entity", nil, token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "attributes.enabled") {
		t.Fatalf("history response: status=%d body=%s", rec.Code, rec.Body.String())
	}
}
