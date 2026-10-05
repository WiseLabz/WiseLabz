package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

const hiddenMarker = "HIDDEN-MARKER-7f3a"

func getEntity(t *testing.T, app *testApp, id, token string) *httptest.ResponseRecorder {
	t.Helper()
	return app.req(t, http.MethodGet, "/api/entities/"+id, nil, token)
}

func decodeEntity(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func newID() string { return uuid.NewString() }

func TestEntityDetailAuthorizationRedactionRedirectsAndGone(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	userID, token := app.user(t, "viewer")
	keyOwnerID, _ := app.user(t, "viewer")
	_, noAccessToken := app.user(t, "viewer")
	c1, c2 := entityTestConnector(t, app, "visible connector"), entityTestConnector(t, app, "hidden connector "+hiddenMarker)
	app.connectorGrant(t, userID, c1.ID, "viewer")
	app.connectorGrant(t, keyOwnerID, c1.ID, "viewer")
	app.connectorGrant(t, keyOwnerID, c2.ID, "viewer")

	visible, goneID, hiddenTarget, redirectVisible, redirectHidden := newID(), newID(), newID(), newID(), newID()
	seedEntity(t, app, visible, "stored hidden name", "vm", c1.ID, "visible-ref", "Visible name", "")
	seedEntityMember(t, app, visible, c2.ID, "vm", "hidden-ref", "Secret name", "")
	seedEntity(t, app, goneID, "Gone name", "vm", c1.ID, "gone-ref", "Gone name", "2026-09-01T00:00:00Z")
	seedEntity(t, app, hiddenTarget, "Secret target", "vm", c2.ID, "secret-target-ref", "Secret target", "")
	seedIdentity(t, app, redirectVisible, "vm", visible)
	seedIdentity(t, app, redirectHidden, "vm", hiddenTarget)

	// A one-connector viewer sees the identity through c1, with no evidence
	// that its second member or stored display name came from c2.
	rec := getEntity(t, app, visible, token)
	body := decodeEntity(t, rec)
	members := body["members"].([]any)
	if len(members) != 1 || body["name"] != "Visible name" || strings.Contains(rec.Body.String(), "Secret name") || strings.Contains(rec.Body.String(), c2.ID) || strings.Contains(rec.Body.String(), hiddenMarker) {
		t.Fatalf("hidden member leaked or visible identity derived incorrectly: %s", rec.Body.String())
	}
	doc := &store.DocRecord{Title: "visible doc", Kind: "service", ServiceID: c1.ID, Content: "x"}
	if err := app.Store.CreateDoc(context.Background(), doc); err != nil {
		t.Fatal(err)
	}
	if got := decodeEntity(t, getEntity(t, app, visible, token))["members"].([]any)[0].(map[string]any)["docId"]; got != doc.ID {
		t.Fatalf("member docId = %v, want the visible connector's doc %s", got, doc.ID)
	}
	if members[0].(map[string]any)["connectorName"] != "visible connector" {
		t.Fatalf("visible member connectorName = %v", members[0])
	}

	if got := getEntity(t, app, redirectVisible, token); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"id":"`+visible+`"`) {
		t.Fatalf("redirect to visible target: status=%d body=%s", got.Code, got.Body.String())
	}

	key := mintAPIKey(t, app, keyOwnerID, "full", []string{c1.ID})
	keyRec := getEntity(t, app, visible, key)
	if keyRec.Code != http.StatusOK || strings.Contains(keyRec.Body.String(), c2.ID) || strings.Contains(keyRec.Body.String(), "Secret name") {
		t.Fatalf("connector-restricted key response: status=%d body=%s", keyRec.Code, keyRec.Body.String())
	}
	readKey := mintAPIKey(t, app, keyOwnerID, "read", nil)
	if got := getEntity(t, app, visible, readKey); got.Code != http.StatusOK {
		t.Fatalf("read-only key status = %d, want 200: %s", got.Code, got.Body.String())
	}

	gone := getEntity(t, app, goneID, token)
	if gone.Code != http.StatusOK || !strings.Contains(gone.Body.String(), `"gone":true`) || !strings.Contains(gone.Body.String(), `"goneAt":"2026-09-01T00:00:00Z"`) {
		t.Fatalf("gone entity banner state: status=%d body=%s", gone.Code, gone.Body.String())
	}

	// Every "you may not see this" outcome is byte-identical to "no such
	// entity": same status, headers and body.
	loopA, loopB := newID(), newID()
	seedIdentity(t, app, loopA, "vm", loopB)
	seedIdentity(t, app, loopB, "vm", loopA)
	want := getEntity(t, app, newID(), token)
	if want.Code != http.StatusNotFound {
		t.Fatalf("unknown id status = %d, want 404", want.Code)
	}
	for name, got := range map[string]*httptest.ResponseRecorder{
		"no grant":             getEntity(t, app, visible, noAccessToken),
		"hidden merge target":  getEntity(t, app, redirectHidden, token),
		"hidden target direct": getEntity(t, app, hiddenTarget, token),
		"merge loop":           getEntity(t, app, loopA, token),
		"malformed id":         getEntity(t, app, "not-a-uuid", token),
		"restricted key":       getEntity(t, app, hiddenTarget, key),
	} {
		assertSameResponse(t, name, want, got)
	}
}

func assertSameResponse(t *testing.T, name string, want, got *httptest.ResponseRecorder) {
	t.Helper()
	if got.Code != want.Code || got.Body.String() != want.Body.String() {
		t.Fatalf("%s: status/body differ from unknown id: %d %q vs %d %q", name, got.Code, got.Body.String(), want.Code, want.Body.String())
	}
	wantHeaders, gotHeaders := want.Header().Clone(), got.Header().Clone()
	for _, h := range []string{"X-Request-Id", "Date"} {
		wantHeaders.Del(h)
		gotHeaders.Del(h)
	}
	if fmt.Sprint(wantHeaders) != fmt.Sprint(gotHeaders) {
		t.Fatalf("%s: headers differ from unknown id: %v vs %v", name, gotHeaders, wantHeaders)
	}
}

func TestEntityDetailInstanceAdminWithoutGrantGetsNotFound(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	_, adminToken := app.user(t, "operator")
	c := entityTestConnector(t, app, "ungranted")
	id := newID()
	seedEntity(t, app, id, "x", "vm", c.ID, "ref", "X", "")
	if got := getEntity(t, app, id, adminToken); got.Code != http.StatusNotFound {
		t.Fatalf("instance admin without grant: status %d, want 404 (no implicit access)", got.Code)
	}
}

// TestEntityDetailHiddenConnectorDataAbsent seeds an edge, finding, runbook
// step and snapshot history on a hidden connector and checks none of it, nor
// its marker, reaches a caller who only sees the other connector.
func TestEntityDetailHiddenConnectorDataAbsent(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	userID, token := app.user(t, "viewer")
	c1, c2 := entityTestConnector(t, app, "visible"), entityTestConnector(t, app, "hidden")
	app.connectorGrant(t, userID, c1.ID, "viewer")
	ctx := context.Background()

	id, hiddenOnly := newID(), newID()
	seedEntity(t, app, id, "e", "vm", c1.ID, "vis", "Visible", "")
	seedEntityMember(t, app, id, c2.ID, "vm", "hid-"+hiddenMarker, "Name "+hiddenMarker, "")
	seedEntity(t, app, hiddenOnly, "h", "vm", c2.ID, "other-"+hiddenMarker, "Other "+hiddenMarker, "")

	if err := app.Store.ReplaceTopologyEdgesForConnector(ctx, c2.ID, []store.TopologyEdge{
		// hidden member -> hidden entity; visible member -> hidden entity
		{SrcConnectorID: c2.ID, SrcKind: "vm", SrcName: "Name " + hiddenMarker, SrcRef: "hid-" + hiddenMarker, DstConnectorID: c2.ID, DstKind: "vm", DstName: "Other " + hiddenMarker, DstRef: "other-" + hiddenMarker, Kind: "dependency", Source: "hidden-edge-" + hiddenMarker},
		{SrcConnectorID: c1.ID, SrcKind: "vm", SrcName: "Visible", SrcRef: "vis", DstConnectorID: c2.ID, DstKind: "vm", DstName: "Other " + hiddenMarker, DstRef: "other-" + hiddenMarker, Kind: "dependency", Source: "mixed-edge-" + hiddenMarker},
	}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []store.QualityFindingRecord{
		{ConnectorID: c2.ID, CheckType: "compliance", RuleID: "r", Severity: "warning", Title: "finding " + hiddenMarker, EntityKind: "vm", EntityRef: "hid-" + hiddenMarker},
		{ConnectorID: c2.ID, CheckType: "stale", Severity: "info", Title: "connector finding " + hiddenMarker},
	} {
		f := f
		if err := app.Store.UpsertQualityFinding(ctx, &f); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := app.Store.CreateRunbookWithSteps(ctx, &store.RunbookRecord{Title: "runbook " + hiddenMarker, Body: "body", TargetType: "change_type", TargetValue: "x"}, []*store.RunbookStepRecord{
		{Title: "step " + hiddenMarker, ConnectorID: c2.ID, Verb: "restart", EntityRef: "hid-" + hiddenMarker},
	}); err != nil {
		t.Fatal(err)
	}
	seedSnapshots(t, app, c2, []snap{
		{"2026-09-01T00:00:00Z", []connector.SnapshotEntity{{Kind: "vm", Name: "Name " + hiddenMarker, ExternalID: "hid-" + hiddenMarker, Attributes: map[string]any{"v": "1"}}}},
		{"2026-09-02T00:00:00Z", []connector.SnapshotEntity{{Kind: "vm", Name: "Name " + hiddenMarker, ExternalID: "hid-" + hiddenMarker, Attributes: map[string]any{"v": "2"}}}},
	})

	rec := getEntity(t, app, id, token)
	body := decodeEntity(t, rec)
	if strings.Contains(rec.Body.String(), hiddenMarker) || strings.Contains(rec.Body.String(), c2.ID) {
		t.Fatalf("hidden connector data leaked: %s", rec.Body.String())
	}
	for _, key := range []string{"neighbors", "relatedByIp", "history", "findings", "onReportingConnectors", "runbooks"} {
		if got := body[key].([]any); len(got) != 0 {
			t.Fatalf("%s = %v, want empty", key, got)
		}
	}
}

func TestEntityDetailActiveMembersDriveIdentityAndRelations(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	userID, token := app.user(t, "viewer")
	c1, c2 := entityTestConnector(t, app, "alpha"), entityTestConnector(t, app, "beta")
	app.connectorGrant(t, userID, c1.ID, "viewer")
	app.connectorGrant(t, userID, c2.ID, "viewer")
	ctx := context.Background()

	id, other := newID(), newID()
	// The gone member sorts first by name but must not name the entity.
	seedEntity(t, app, id, "stored", "vm", c1.ID, "gone-ref", "Aardvark old", "2026-09-01T00:00:00Z")
	seedEntityMember(t, app, id, c2.ID, "vm", "live-ref", "Zebra live", "")
	seedEntity(t, app, other, "o", "service", c2.ID, "other-ref", "Other", "")

	edge := func(kind, source string, srcConn *store.ConnectorRecord, srcRef, dstRef string) store.TopologyEdge {
		return store.TopologyEdge{SrcConnectorID: srcConn.ID, SrcKind: "vm", SrcName: srcRef, SrcRef: srcRef, DstConnectorID: c2.ID, DstKind: "service", DstName: "Other", DstRef: dstRef, Kind: kind, Source: source}
	}
	if err := app.Store.ReplaceTopologyEdgesForConnector(ctx, c2.ID, []store.TopologyEdge{
		edge("dependency", "x", c2, "live-ref", "other-ref"),
		edge("same_as", "IP address", c2, "live-ref", "other-ref"),
		edge("dependency", "gone-edge", c1, "gone-ref", "other-ref"),
	}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []store.QualityFindingRecord{
		{ConnectorID: c2.ID, CheckType: "compliance", RuleID: "live", Severity: "critical", Title: "live finding", EntityKind: "vm", EntityRef: "live-ref"},
		{ConnectorID: c2.ID, CheckType: "compliance", RuleID: "resolved", Severity: "critical", Title: "resolved finding", EntityKind: "vm", EntityRef: "live-ref"},
		{ConnectorID: c1.ID, CheckType: "compliance", RuleID: "gone", Severity: "critical", Title: "gone member finding", EntityKind: "vm", EntityRef: "gone-ref"},
		{ConnectorID: c2.ID, CheckType: "stale", Severity: "info", Title: "beta connector finding"},
		{ConnectorID: c1.ID, CheckType: "stale", Severity: "info", Title: "alpha connector finding (gone member only)"},
	} {
		f := f
		if err := app.Store.UpsertQualityFinding(ctx, &f); err != nil {
			t.Fatal(err)
		}
		if f.RuleID == "resolved" {
			if _, err := app.Store.DB().ExecContext(ctx, `UPDATE quality_findings SET status = 'resolved', resolved_at = '2026-09-01T00:00:00Z' WHERE id = ?`, f.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i, step := range []store.RunbookStepRecord{
		{Title: "live step", ConnectorID: c2.ID, Verb: "restart", EntityRef: "live-ref"},
		{Title: "gone step", ConnectorID: c1.ID, Verb: "restart", EntityRef: "gone-ref"},
	} {
		step := step
		if _, _, err := app.Store.CreateRunbookWithSteps(ctx, &store.RunbookRecord{Title: step.Title, Body: "# body", TargetType: "change_type", TargetValue: fmt.Sprint("t", i)}, []*store.RunbookStepRecord{&step}); err != nil {
			t.Fatal(err)
		}
	}

	rec := getEntity(t, app, id, token)
	body := decodeEntity(t, rec)
	if body["name"] != "Zebra live" || body["kind"] != "vm" || body["gone"] != false {
		t.Fatalf("identity not taken from the active member: %s", rec.Body.String())
	}
	members := body["members"].([]any)
	if len(members) != 2 || members[0].(map[string]any)["ref"] != "live-ref" || members[1].(map[string]any)["goneAt"] != "2026-09-01T00:00:00Z" {
		t.Fatalf("members should list active first and keep goneAt: %v", members)
	}
	neighbors, links := body["neighbors"].([]any), body["relatedByIp"].([]any)
	if len(neighbors) != 1 || len(links) != 1 {
		t.Fatalf("neighbors=%v links=%v; gone member's edge must not appear", neighbors, links)
	}
	to := neighbors[0].(map[string]any)["to"].(map[string]any)
	if to["entityId"] != other {
		t.Fatalf("neighbour should link to the other visible entity %s: %v", other, to)
	}
	if from := neighbors[0].(map[string]any)["from"].(map[string]any); from["entityId"] != nil && from["entityId"] != "" && from["entityId"] != id {
		t.Fatalf("self endpoint carries a foreign entity id: %v", from)
	}
	findings, level := body["findings"].([]any), body["onReportingConnectors"].([]any)
	if len(findings) != 1 || findings[0].(map[string]any)["title"] != "live finding" || findings[0].(map[string]any)["status"] != "open" {
		t.Fatalf("findings must be open and from active members only: %v", findings)
	}
	if len(level) != 1 || level[0].(map[string]any)["title"] != "beta connector finding" {
		t.Fatalf("connector-level findings must come from active member connectors only: %v", level)
	}
	runbooks := body["runbooks"].([]any)
	if len(runbooks) != 1 || runbooks[0].(map[string]any)["title"] != "live step" {
		t.Fatalf("runbooks must target active members only: %v", runbooks)
	}
}

type snap struct {
	at       string
	entities []connector.SnapshotEntity
}

func seedSnapshots(t *testing.T, app *testApp, c *store.ConnectorRecord, snaps []snap) {
	t.Helper()
	for _, s := range snaps {
		fetchedAt, err := time.Parse(time.RFC3339, s.at)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(connector.ServiceSnapshot{ServiceName: c.Name, Type: c.Type, FetchedAt: fetchedAt, Entities: s.entities})
		if err != nil {
			t.Fatal(err)
		}
		if err := app.Store.CreateSnapshot(context.Background(), &store.SnapshotRecord{ConnectorID: c.ID, Data: string(data), FetchedAt: s.at}); err != nil {
			t.Fatal(err)
		}
	}
}

// atIs compares a response timestamp to want as instants; the store
// normalizes snapshot times to nanosecond precision.
func atIs(got any, want string) bool {
	g, ok := got.(string)
	if !ok {
		return false
	}
	gt, err1 := time.Parse(time.RFC3339Nano, g)
	wt, err2 := time.Parse(time.RFC3339Nano, want)
	return err1 == nil && err2 == nil && gt.Equal(wt)
}

func vm(enabled bool) []connector.SnapshotEntity {
	return []connector.SnapshotEntity{{Kind: "vm", Name: "VM", ExternalID: "42", Attributes: map[string]any{"enabled": enabled}}}
}

func TestEntityDetailHistoryDirectionTimestampAndAddedRemoved(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	userID, token := app.user(t, "viewer")
	c := entityTestConnector(t, app, "history connector")
	app.connectorGrant(t, userID, c.ID, "viewer")
	id := newID()
	seedEntity(t, app, id, "VM", "vm", c.ID, "42", "VM", "")
	seedSnapshots(t, app, c, []snap{
		{"2026-09-01T00:00:00Z", nil},
		{"2026-09-02T00:00:00Z", vm(false)},
		{"2026-09-03T00:00:00Z", vm(true)},
		{"2026-09-04T00:00:00Z", nil},
	})
	body := decodeEntity(t, getEntity(t, app, id, token))
	history := body["history"].([]any)
	if len(history) != 3 {
		t.Fatalf("history = %v, want removed, modified, added", history)
	}
	removed, modified, added := history[0].(map[string]any), history[1].(map[string]any), history[2].(map[string]any)
	if !atIs(removed["at"], "2026-09-04T00:00:00Z") || removed["change"] != "removed" || removed["field"] != "entity" || removed["old"] == nil || removed["new"] != nil {
		t.Fatalf("removed change wrong (newest first): %v", removed)
	}
	if !atIs(modified["at"], "2026-09-03T00:00:00Z") || modified["field"] != "attributes.enabled" || modified["old"] != false || modified["new"] != true || modified["connectorId"] != c.ID {
		t.Fatalf("modified change must be old=false new=true at the newer snapshot: %v", modified)
	}
	if !atIs(added["at"], "2026-09-02T00:00:00Z") || added["change"] != "added" || added["field"] != "entity" || added["new"] == nil || added["old"] != nil {
		t.Fatalf("added change wrong: %v", added)
	}
}

func TestEntityDetailHistoryIsBounded(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	userID, token := app.user(t, "viewer")
	c := entityTestConnector(t, app, "bounded")
	app.connectorGrant(t, userID, c.ID, "viewer")
	id := newID()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	const members = 4
	for m := 0; m < members; m++ {
		ref := fmt.Sprint("m", m)
		if m == 0 {
			seedEntity(t, app, id, "e", "vm", c.ID, ref, ref, "")
		} else {
			seedEntityMember(t, app, id, c.ID, "vm", ref, ref, "")
		}
	}
	var snaps []snap
	for i := 0; i < 40; i++ {
		var entities []connector.SnapshotEntity
		for m := 0; m < members; m++ {
			entities = append(entities, connector.SnapshotEntity{Kind: "vm", Name: fmt.Sprint("m", m), ExternalID: fmt.Sprint("m", m), Attributes: map[string]any{"n": float64(i)}})
		}
		snaps = append(snaps, snap{base.Add(time.Duration(i) * time.Hour).Format(time.RFC3339), entities})
	}
	seedSnapshots(t, app, c, snaps)
	history := decodeEntity(t, getEntity(t, app, id, token))["history"].([]any)
	// 30 newest snapshots give 29 diffs of 4 members = 116 rows, capped at 100.
	if len(history) != 100 {
		t.Fatalf("history rows = %d, want the 100-row cap", len(history))
	}
	for i := 1; i < len(history); i++ {
		if history[i-1].(map[string]any)["at"].(string) < history[i].(map[string]any)["at"].(string) {
			t.Fatalf("history not newest first at %d", i)
		}
	}
	newest := base.Add(39 * time.Hour).Format(time.RFC3339)
	if !atIs(history[0].(map[string]any)["at"], newest) {
		t.Fatalf("newest history row at %v, want %s", history[0].(map[string]any)["at"], newest)
	}
	// Changes older than the 30 most recent snapshots are never read.
	cutoff := base.Add(10 * time.Hour).Format(time.RFC3339)
	for _, h := range history {
		if h.(map[string]any)["at"].(string) <= cutoff {
			t.Fatalf("history row at %v predates the 30-snapshot window", h)
		}
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

const farMarker = "FAR-MARKER-91c2"

// seedFarEdgeFixture builds identity `near` (member on `member`) with a
// proxies_to neighbour and an IP relation to identity `far`, whose only member
// lives on connector `other`, which is not a member connector of `near`.
func seedFarEdgeFixture(t *testing.T, app *testApp) (near string, member, other *store.ConnectorRecord) {
	t.Helper()
	member, other = entityTestConnector(t, app, "member"), entityTestConnector(t, app, "other "+farMarker)
	near, far := newID(), newID()
	seedEntity(t, app, near, "n", "vm", member.ID, "near-ref", "Near", "")
	seedEntity(t, app, far, "f", "service", other.ID, "far-ref", "Far "+farMarker, "")
	if err := app.Store.ReplaceTopologyEdgesForConnector(context.Background(), other.ID, []store.TopologyEdge{
		{SrcConnectorID: other.ID, SrcKind: "service", SrcName: "Far " + farMarker, SrcRef: "far-ref", DstConnectorID: member.ID, DstKind: "vm", DstName: "Near", DstRef: "near-ref", Kind: "proxies_to", Source: "far-proxy", Detail: "upstream-" + farMarker},
		{SrcConnectorID: member.ID, SrcKind: "vm", SrcName: "Near", SrcRef: "near-ref", DstConnectorID: other.ID, DstKind: "service", DstName: "Far " + farMarker, DstRef: "far-ref", Kind: "same_as", Source: "IP address"},
	}); err != nil {
		t.Fatal(err)
	}
	return near, member, other
}

func TestEntityDetailEdgesToNonMemberConnectorVisibleWithGrant(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	adminID, adminToken := app.user(t, "operator")
	near, member, other := seedFarEdgeFixture(t, app)
	app.connectorGrant(t, adminID, member.ID, "viewer")
	app.connectorGrant(t, adminID, other.ID, "viewer")

	body := decodeEntity(t, getEntity(t, app, near, adminToken))
	neighbors, links := body["neighbors"].([]any), body["relatedByIp"].([]any)
	if len(neighbors) != 1 || neighbors[0].(map[string]any)["kind"] != "proxies_to" {
		t.Fatalf("proxies_to neighbour on a viewable non-member connector missing: %v", neighbors)
	}
	if len(links) != 1 {
		t.Fatalf("IP relation to a viewable non-member connector missing: %v", links)
	}
}

func TestEntityDetailEdgesToUnviewableConnectorLeaveNoTrace(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	userID, token := app.user(t, "viewer")
	near, member, other := seedFarEdgeFixture(t, app)
	app.connectorGrant(t, userID, member.ID, "viewer")
	keyOwner, _ := app.user(t, "viewer")
	app.connectorGrant(t, keyOwner, member.ID, "viewer")
	app.connectorGrant(t, keyOwner, other.ID, "viewer")
	key := mintAPIKey(t, app, keyOwner, "full", []string{member.ID})

	for name, tok := range map[string]string{"viewer on member only": token, "key restricted to member": key} {
		rec := getEntity(t, app, near, tok)
		body := decodeEntity(t, rec)
		if s := rec.Body.String(); strings.Contains(s, farMarker) || strings.Contains(s, other.ID) || strings.Contains(s, "far-ref") {
			t.Fatalf("%s: far endpoint leaked: %s", name, s)
		}
		if len(body["neighbors"].([]any)) != 0 || len(body["relatedByIp"].([]any)) != 0 {
			t.Fatalf("%s: edges to an unviewable connector listed: %s", name, rec.Body.String())
		}
	}
}

func TestEntityDetailDeduplicatesEdgesAndNamesFromVisibleMembers(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	userID, token := app.user(t, "viewer")
	c1, c2 := entityTestConnector(t, app, "alpha"), entityTestConnector(t, app, "beta")
	app.connectorGrant(t, userID, c1.ID, "viewer")
	app.connectorGrant(t, userID, c2.ID, "viewer")
	id, other := newID(), newID()
	// A nameless dns_record sorts first by name but must not blank the title.
	seedEntity(t, app, id, "stored", "dns_record", c1.ID, "web01.lab.test", "", "")
	seedEntityMember(t, app, id, c2.ID, "vm", "vm-ref", "web01", "")
	seedEntity(t, app, other, "o", "service", c2.ID, "other-ref", "Other", "")
	edge := func(kind, source string) store.TopologyEdge {
		return store.TopologyEdge{SrcConnectorID: c2.ID, SrcKind: "vm", SrcName: "web01", SrcRef: "vm-ref", DstConnectorID: c2.ID, DstKind: "service", DstName: "Other", DstRef: "other-ref", Kind: kind, Source: source, Detail: "d"}
	}
	for _, owner := range []string{c1.ID, c2.ID} {
		if err := app.Store.ReplaceTopologyEdgesForConnector(context.Background(), owner, []store.TopologyEdge{
			edge("dependency", "x"), edge("same_as", "IP address"),
			// both ends are members of this identity: not a relation to another entity
			{SrcConnectorID: c1.ID, SrcKind: "dns_record", SrcName: "", SrcRef: "web01.lab.test", DstConnectorID: c2.ID, DstKind: "vm", DstName: "web01", DstRef: "vm-ref", Kind: "same_as", Source: "IP address"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	rec := getEntity(t, app, id, token)
	body := decodeEntity(t, rec)
	if body["name"] != "web01" || body["kind"] != "vm" {
		t.Fatalf("identity should be named from the non-empty member: %s", rec.Body.String())
	}
	if n := body["neighbors"].([]any); len(n) != 1 {
		t.Fatalf("duplicate neighbours not collapsed: %v", n)
	}
	if l := body["relatedByIp"].([]any); len(l) != 1 {
		t.Fatalf("want one IP relation (deduped, no self relation): %v", l)
	}
}

func TestEntityDetailAllNamesEmptyFallsBackToRef(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	userID, token := app.user(t, "viewer")
	c := entityTestConnector(t, app, "alpha")
	app.connectorGrant(t, userID, c.ID, "viewer")
	id := newID()
	seedEntity(t, app, id, "stored", "dns_record", c.ID, "b.lab.test", "", "")
	seedEntityMember(t, app, id, c.ID, "dns_record", "a.lab.test", "", "")
	body := decodeEntity(t, getEntity(t, app, id, token))
	if body["name"] != "a.lab.test" || body["kind"] != "dns_record" {
		t.Fatalf("name = %v kind = %v, want ref fallback a.lab.test", body["name"], body["kind"])
	}
}
