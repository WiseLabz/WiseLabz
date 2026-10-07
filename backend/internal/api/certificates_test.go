package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/connector"
)

type listedCertificate struct {
	Name, ConnectorID, ConnectorName, ExternalID, EntityID, NotAfter string
	DaysLeft                                                         int
	Unreachable                                                      bool
}

func certificate(t *testing.T, name string, expiry time.Time, unreachable bool) connector.SnapshotEntity {
	t.Helper()
	return connector.SnapshotEntity{Kind: "certificate", Name: name, ExternalID: name,
		Attributes: map[string]any{"not_after": expiry.Format(time.RFC3339), "reachable": !unreachable}}
}

func listCertificates(t *testing.T, app *testApp, token, query string) []listedCertificate {
	t.Helper()
	path := "/api/certificates" + query
	rec := app.req(t, http.MethodGet, path, nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	apitest.AssertMatchesSpec(t, app.newRequest(t, http.MethodGet, path, nil, token), rec.Result())
	var items []listedCertificate
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if items == nil {
		t.Fatal("listing must serialize an empty array, not null")
	}
	return items
}

func TestCertificatesOrderingVisibilityAndNoRules(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	user, token := app.user(t, "viewer")
	npm := seedHealthTestConnector(t, app, "npm")
	probe := seedHealthTestConnector(t, app, "tlsprobe")
	hidden := seedHealthTestConnector(t, app, "tlsprobe")
	other := seedHealthTestConnector(t, app, "traefik")
	for _, c := range []string{npm.ID, probe.ID, other.ID} {
		app.connectorGrant(t, user, c, "viewer")
	}
	now := time.Now().UTC()
	seedSnapshots(t, app, npm, []snap{{at: now.Add(-time.Hour).Format(time.RFC3339), entities: []connector.SnapshotEntity{
		certificate(t, "old snapshot", now.Add(-100*24*time.Hour), false),
	}}, {at: now.Format(time.RFC3339), entities: []connector.SnapshotEntity{
		certificate(t, "later", now.Add(40*24*time.Hour+12*time.Hour), false),
		{Kind: "certificate", Name: "bad expiry", Attributes: map[string]any{"not_after": "bad"}},
		{Kind: "certificate", Name: "no expiry"},
		{Kind: "router", Name: "wrong kind", Attributes: map[string]any{"not_after": now.Format(time.RFC3339)}},
	}}})
	seedSnapshots(t, app, probe, []snap{{at: now.Format(time.RFC3339), entities: []connector.SnapshotEntity{
		certificate(t, "soon", now.Add(7*24*time.Hour+12*time.Hour), true),
		certificate(t, "expired", now.Add(-12*time.Hour), false),
	}}})
	seedSnapshots(t, app, hidden, []snap{{at: now.Format(time.RFC3339), entities: []connector.SnapshotEntity{
		certificate(t, "hidden", now.Add(-30*24*time.Hour), false),
	}}})
	seedSnapshots(t, app, other, []snap{{at: now.Format(time.RFC3339), entities: []connector.SnapshotEntity{
		certificate(t, "undeclared kind", now.Add(-40*24*time.Hour), false),
	}}})
	// The fresh database has no compliance rules or installed packs.
	entityID := newID()
	seedEntity(t, app, entityID, "expired", "certificate", probe.ID, "expired", "expired", "")
	items := listCertificates(t, app, token, "")
	if len(items) != 3 || items[0].Name != "expired" || items[1].Name != "soon" || items[2].Name != "later" {
		t.Fatalf("ordered visible certificates = %+v", items)
	}
	if items[0].DaysLeft != -1 || items[1].DaysLeft != 7 || !items[1].Unreachable || items[2].DaysLeft != 40 {
		t.Fatalf("days left and reachability = %+v", items)
	}
	if items[0].ConnectorID != probe.ID || items[0].ConnectorName != probe.Name || items[0].ExternalID != "expired" {
		t.Fatalf("certificate source = %+v", items[0])
	}
	if items[0].EntityID != entityID {
		t.Fatalf("certificate entity link = %q, want %q", items[0].EntityID, entityID)
	}
	if got := listCertificates(t, app, token, "?limit=1"); len(got) != 1 || got[0].Name != "expired" {
		t.Fatalf("truncated listing = %+v", got)
	}
	_, noAccessToken := app.user(t, "viewer")
	if got := listCertificates(t, app, noAccessToken, ""); len(got) != 0 {
		t.Fatalf("ungranted listing = %+v", got)
	}
}

func TestCertificatesRestrictedAPIKey(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	user, token := app.user(t, "operator")
	a := seedHealthTestConnector(t, app, "npm")
	b := seedHealthTestConnector(t, app, "tlsprobe")
	now := time.Now().UTC()
	for _, c := range []string{a.ID, b.ID} {
		app.connectorGrant(t, user, c, "operator")
	}
	seedSnapshots(t, app, a, []snap{{at: now.Format(time.RFC3339), entities: []connector.SnapshotEntity{certificate(t, "a", now.Add(12*time.Hour), false)}}})
	seedSnapshots(t, app, b, []snap{{at: now.Format(time.RFC3339), entities: []connector.SnapshotEntity{certificate(t, "b", now.Add(-12*time.Hour), false)}}})
	raw := createKey(t, app, token, map[string]any{"name": "one", "connectorIds": []string{a.ID}})["token"].(string)
	if got := listCertificates(t, app, raw, ""); len(got) != 1 || got[0].ConnectorID != a.ID {
		t.Fatalf("restricted key listing = %+v", got)
	}
}

func TestCertificatesLimitBounds(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	user, token := app.user(t, "viewer")
	c := seedHealthTestConnector(t, app, "tlsprobe")
	app.connectorGrant(t, user, c.ID, "viewer")
	now := time.Now().UTC()
	entities := []connector.SnapshotEntity{}
	for i := range 105 {
		entities = append(entities, certificate(t, fmt.Sprintf("cert-%03d", i), now.Add(time.Duration(i)*24*time.Hour), false))
	}
	seedSnapshots(t, app, c, []snap{{at: now.Format(time.RFC3339), entities: entities}})
	for query, want := range map[string]int{"": 10, "?limit=1": 1, "?limit=100": 100, "?limit=101": 100} {
		if got := listCertificates(t, app, token, query); len(got) != want {
			t.Errorf("%s returned %d certificates, want %d", query, len(got), want)
		}
	}
	for _, limit := range []string{"0", "-1", "abc", "1.5"} {
		rec := app.req(t, http.MethodGet, "/api/certificates?limit="+limit, nil, token)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("limit %s status = %d: %s", limit, rec.Code, rec.Body)
		}
	}
}

func TestCertificatesEntityLinkFallsBackToName(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	user, token := app.user(t, "viewer")
	probe := seedHealthTestConnector(t, app, "tlsprobe")
	app.connectorGrant(t, user, probe.ID, "viewer")
	now := time.Now().UTC()
	seedSnapshots(t, app, probe, []snap{{at: now.Format(time.RFC3339), entities: []connector.SnapshotEntity{
		{Kind: "certificate", Name: "unnamed-id", Attributes: map[string]any{"not_after": now.Add(24 * time.Hour).Format(time.RFC3339)}},
	}}})
	entityID := newID()
	seedEntity(t, app, entityID, "unnamed-id", "certificate", probe.ID, "unnamed-id", "unnamed-id", "")
	items := listCertificates(t, app, token, "")
	if len(items) != 1 || items[0].ExternalID != "" || items[0].EntityID != entityID {
		t.Fatalf("certificate without external id = %+v, want entityId %q", items, entityID)
	}
}
