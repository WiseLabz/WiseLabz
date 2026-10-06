package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/api/entities"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/doc"
	"github.com/WiseLabz/wiselabz/internal/store"
)

type overrideFixture struct {
	app          *testApp
	adminID      string
	admin        string
	c1, c2       *store.ConnectorRecord
	solo         *store.ConnectorRecord
	hostA, hostB string // refs of the two members sharing a hostname
}

// newOverrideFixture seeds two connectors whose vms share a hostname (so they
// form one identity), a third connector with an unrelated vm, and an instance
// admin who can view all of them.
func newOverrideFixture(t *testing.T) *overrideFixture {
	t.Helper()
	app := newTestApp(t)
	adminID, admin := app.user(t, "operator")
	f := &overrideFixture{app: app, adminID: adminID, admin: admin, hostA: "a1", hostB: "b1"}
	f.c1, f.c2, f.solo = entityTestConnector(t, app, "conn one"), entityTestConnector(t, app, "conn two"), entityTestConnector(t, app, "conn solo")
	at := "2026-09-01T00:00:00Z"
	seedSnapshots(t, app, f.c1, []snap{{at: at, entities: []connector.SnapshotEntity{{Kind: "vm", Name: "A", ExternalID: "a1", Hostname: "shared-host"}}}})
	seedSnapshots(t, app, f.c2, []snap{{at: at, entities: []connector.SnapshotEntity{{Kind: "vm", Name: "B", ExternalID: "b1", Hostname: "shared-host"}}}})
	seedSnapshots(t, app, f.solo, []snap{{at: at, entities: []connector.SnapshotEntity{
		{Kind: "vm", Name: "S", ExternalID: "s1", Hostname: "lonely"},
		{Kind: "service", Name: "Svc", ExternalID: "svc1"},
	}}})
	if _, err := doc.NewEngine(app.Store).BackfillEntityIdentities(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, c := range []*store.ConnectorRecord{f.c1, f.c2, f.solo} {
		app.connectorGrant(t, adminID, c.ID, "viewer")
	}
	return f
}

type overrideBody struct {
	ID      string `json:"id"`
	Action  string `json:"action"`
	State   string `json:"state"`
	Members []struct {
		ConnectorID string `json:"connectorId"`
		Ref         string `json:"ref"`
		EntityID    string `json:"entityId"`
	} `json:"members"`
}

func decodeOverride(t *testing.T, rec *httptest.ResponseRecorder, want int) overrideBody {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status %d, want %d: %s", rec.Code, want, rec.Body.String())
	}
	var out overrideBody
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// call serves an admin request and asserts the response against docs/openapi.yaml.
func (f *overrideFixture) call(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	req := f.app.newRequest(t, method, path, body, f.admin)
	rec := f.app.serve(req)
	apitest.AssertMatchesSpec(t, req, rec.Result())
	return rec
}

func (f *overrideFixture) detach(t *testing.T, c *store.ConnectorRecord, ref string) *httptest.ResponseRecorder {
	t.Helper()
	return f.call(t, http.MethodPost, "/api/entity-overrides", map[string]any{"action": "detach", "connectorId": c.ID, "kind": "vm", "ref": ref, "note": "split"})
}

func (f *overrideFixture) merge(t *testing.T, c1 *store.ConnectorRecord, ref1 string, c2 *store.ConnectorRecord, ref2 string) *httptest.ResponseRecorder {
	t.Helper()
	return f.call(t, http.MethodPost, "/api/entity-overrides", map[string]any{"action": "merge", "connectorId": c1.ID, "kind": "vm", "ref": ref1, "otherConnectorId": c2.ID, "otherKind": "vm", "otherRef": ref2})
}

func memberIdentity(t *testing.T, o overrideBody, connectorID, ref string) string {
	t.Helper()
	for _, m := range o.Members {
		if m.ConnectorID == connectorID && m.Ref == ref {
			return m.EntityID
		}
	}
	t.Fatalf("member %s/%s not in response %+v", connectorID, ref, o)
	return ""
}

func TestEntityOverridesRequireInstanceAdmin(t *testing.T) {
	t.Parallel()
	f := newOverrideFixture(t)
	_, viewer := f.app.user(t, "viewer")
	body := map[string]any{"action": "detach", "connectorId": f.c1.ID, "kind": "vm", "ref": f.hostA}
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/entity-overrides"},
		{http.MethodPost, "/api/entity-overrides"},
		{http.MethodDelete, "/api/entity-overrides/" + newID()},
	} {
		if rec := f.app.req(t, tc.method, tc.path, body, viewer); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s as non-admin = %d, want 403: %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
	list, err := f.app.Store.LoadEntityIdentityOverrides(context.Background())
	if err != nil || len(list) != 0 {
		t.Fatalf("a rejected caller must not create anything: %v %v", list, err)
	}
}

func TestEntityOverrideCreateDetachReturnsNewIdentity(t *testing.T) {
	t.Parallel()
	f := newOverrideFixture(t)
	created := decodeOverride(t, f.detach(t, f.c2, f.hostB), http.StatusCreated)
	if created.Action != "detach" || created.State != "active" || len(created.Members) != 1 {
		t.Fatalf("unexpected create response: %+v", created)
	}
	id := created.Members[0].EntityID
	if id == "" {
		t.Fatal("create response has no identity ID for the member")
	}
	got := decodeEntity(t, getEntity(t, f.app, id, f.admin))
	members := got["members"].([]any)
	if len(members) != 1 || members[0].(map[string]any)["ref"] != f.hostB {
		t.Fatalf("GET /api/entities/%s = %v, want only the detached member", id, got)
	}
	// The other member kept a different identity.
	other := decodeEntity(t, getEntity(t, f.app, memberIdentityFromList(t, f, f.c1.ID, f.hostA), f.admin))
	if other["id"] == id {
		t.Fatalf("detached member still shares identity %s", id)
	}
}

func memberIdentityFromList(t *testing.T, f *overrideFixture, connectorID, ref string) string {
	t.Helper()
	var list []overrideBody
	rec := f.call(t, http.MethodGet, "/api/entity-overrides", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	for _, o := range list {
		for _, m := range o.Members {
			if m.ConnectorID == connectorID && m.Ref == ref {
				return m.EntityID
			}
		}
	}
	// Not referenced by an override: create a throwaway merge-free lookup via the store.
	members, err := f.app.Store.EntityOverrideMembers(context.Background(), store.EntityIdentityOverride{Action: "detach", ConnectorID: connectorID, Kind: "vm", Ref: ref})
	if err != nil || len(members) != 1 {
		t.Fatalf("lookup member identity: %v %v", members, err)
	}
	return members[0].EntityID
}

func TestEntityOverrideCreateRejections(t *testing.T) {
	t.Parallel()
	f := newOverrideFixture(t)
	post := func(body any) *httptest.ResponseRecorder {
		return f.call(t, http.MethodPost, "/api/entity-overrides", body)
	}
	member := map[string]any{"connectorId": f.c1.ID, "kind": "vm", "ref": f.hostA}
	with := func(extra map[string]any) map[string]any {
		out := map[string]any{}
		for k, v := range member {
			out[k] = v
		}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}
	cases := map[string]any{
		"unknown action":     with(map[string]any{"action": "explode"}),
		"missing action":     member,
		"missing ref":        map[string]any{"action": "detach", "connectorId": f.c1.ID, "kind": "vm"},
		"unknown member":     with(map[string]any{"action": "detach", "ref": "never-seen"}),
		"unknown connector":  with(map[string]any{"action": "detach", "connectorId": newID()}),
		"detach with other":  with(map[string]any{"action": "detach", "otherConnectorId": f.c2.ID, "otherKind": "vm", "otherRef": f.hostB}),
		"merge without pair": with(map[string]any{"action": "merge"}),
		"merge same member":  with(map[string]any{"action": "merge", "otherConnectorId": f.c1.ID, "otherKind": "vm", "otherRef": f.hostA}),
		"merge across kinds": with(map[string]any{"action": "merge", "otherConnectorId": f.solo.ID, "otherKind": "service", "otherRef": "svc1"}),
		"merge unknown pair": with(map[string]any{"action": "merge", "otherConnectorId": f.c2.ID, "otherKind": "vm", "otherRef": "never-seen"}),
		"note too long":      with(map[string]any{"action": "detach", "note": strings.Repeat("x", 1001)}),
		"NUL in note":        with(map[string]any{"action": "detach", "note": "a\u0000b"}),
		"NUL in ref":         with(map[string]any{"action": "detach", "ref": "a1\u0000"}),
		"connector not uuid": with(map[string]any{"action": "detach", "connectorId": "not-a-uuid"}),
	}
	for name, body := range cases {
		if rec := post(body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400: %s", name, rec.Code, rec.Body.String())
		}
	}
	if rec := f.call(t, http.MethodPost, "/api/entity-overrides", "{not json"); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed body: status %d, want 400", rec.Code)
	}
	list, err := f.app.Store.LoadEntityIdentityOverrides(context.Background())
	if err != nil || len(list) != 0 {
		t.Fatalf("rejected creations must store nothing: %v %v", list, err)
	}
}

func TestEntityOverrideDuplicateIs409(t *testing.T) {
	t.Parallel()
	f := newOverrideFixture(t)
	decodeOverride(t, f.merge(t, f.c1, f.hostA, f.solo, "s1"), http.StatusCreated)
	if rec := f.merge(t, f.solo, "s1", f.c1, f.hostA); rec.Code != http.StatusConflict {
		t.Fatalf("reversed duplicate merge: status %d, want 409: %s", rec.Code, rec.Body.String())
	}
	decodeOverride(t, f.detach(t, f.c1, f.hostA), http.StatusCreated)
	if rec := f.detach(t, f.c1, f.hostA); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate detach: status %d, want 409: %s", rec.Code, rec.Body.String())
	}
}

func TestEntityOverrideMergeAndDeleteReturnIdentities(t *testing.T) {
	t.Parallel()
	f := newOverrideFixture(t)
	created := decodeOverride(t, f.merge(t, f.c1, f.hostA, f.solo, "s1"), http.StatusCreated)
	a, s := memberIdentity(t, created, f.c1.ID, f.hostA), memberIdentity(t, created, f.solo.ID, "s1")
	if a == "" || a != s {
		t.Fatalf("merge response identities %q and %q, want one shared ID", a, s)
	}
	// The merged identity is readable and holds the solo member.
	body := decodeEntity(t, getEntity(t, f.app, a, f.admin))
	if len(body["members"].([]any)) != 3 {
		t.Fatalf("merged identity members = %v, want 3", body["members"])
	}

	deleted := decodeOverride(t, f.call(t, http.MethodDelete, "/api/entity-overrides/"+created.ID, nil), http.StatusOK)
	a2, s2 := memberIdentity(t, deleted, f.c1.ID, f.hostA), memberIdentity(t, deleted, f.solo.ID, "s1")
	if a2 == "" || s2 == "" || a2 == s2 {
		t.Fatalf("delete response identities %q and %q, want two different IDs", a2, s2)
	}
	if rec := f.call(t, http.MethodDelete, "/api/entity-overrides/"+created.ID, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("second delete: status %d, want 404", rec.Code)
	}
	if rec := getEntity(t, f.app, s2, f.admin); rec.Code != http.StatusOK {
		t.Fatalf("split-off identity %s: status %d", s2, rec.Code)
	}
}

func TestEntityOverrideListShowsStateAndMembers(t *testing.T) {
	t.Parallel()
	f := newOverrideFixture(t)
	decodeOverride(t, f.detach(t, f.c1, f.hostA), http.StatusCreated)
	rec := f.call(t, http.MethodGet, "/api/entity-overrides", nil)
	var list []map[string]any
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list) != 1 {
		t.Fatalf("list: status %d body %s", rec.Code, rec.Body.String())
	}
	o := list[0]
	m := o["members"].([]any)[0].(map[string]any)
	if o["state"] != "active" || o["createdBy"] != f.adminID || o["note"] != "split" || o["createdAt"] == "" || m["connectorName"] != "conn one" || m["name"] != "A" || m["entityId"] == "" {
		t.Fatalf("unexpected list entry: %v", o)
	}
}

func TestEntityOverrideMutationsAreAudited(t *testing.T) {
	t.Parallel()
	f := newOverrideFixture(t)
	created := decodeOverride(t, f.merge(t, f.c1, f.hostA, f.solo, "s1"), http.StatusCreated)
	if rec := f.call(t, http.MethodDelete, "/api/entity-overrides/"+created.ID, nil); rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	for _, action := range []string{"entity.override.create", "entity.override.delete"} {
		rec := f.app.req(t, http.MethodGet, "/api/system/audit?action="+action, nil, f.admin) // audit page schema drift predates this PR
		var page struct {
			Items []store.AuditRecord `json:"items"`
		}
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &page) != nil {
			t.Fatalf("audit list %s: %d %s", action, rec.Code, rec.Body.String())
		}
		if len(page.Items) != 1 {
			t.Fatalf("%s rows = %d, want 1", action, len(page.Items))
		}
		row := page.Items[0]
		if row.TargetID != created.ID || row.ActorUserID != f.adminID {
			t.Errorf("%s row = %+v, want target %s actor %s", action, row, created.ID, f.adminID)
		}
		for _, want := range []string{f.c1.ID, f.hostA, f.solo.ID, "s1"} {
			if !strings.Contains(row.Detail, want) {
				t.Errorf("%s detail %q lacks member %q", action, row.Detail, want)
			}
		}
	}
}

func TestEntityOverrideDeleteMalformedIDIs404(t *testing.T) {
	t.Parallel()
	f := newOverrideFixture(t)
	for _, id := range []string{"not-a-uuid", "a%00b", newID()} {
		if rec := f.call(t, http.MethodDelete, "/api/entity-overrides/"+id, nil); rec.Code != http.StatusNotFound {
			t.Errorf("DELETE %q: status %d, want 404: %s", id, rec.Code, rec.Body.String())
		}
	}
}

// Instance admins get no implicit entity access: the override response names
// the identity, but GET /api/entities/{id} keeps its grant rule.
func TestEntityOverrideAdminWithoutGrantGetsIdentityButEntity404(t *testing.T) {
	t.Parallel()
	f := newOverrideFixture(t)
	_, ungranted := f.app.user(t, "operator")
	rec := f.app.req(t, http.MethodPost, "/api/entity-overrides", map[string]any{"action": "detach", "connectorId": f.c2.ID, "kind": "vm", "ref": f.hostB}, ungranted)
	created := decodeOverride(t, rec, http.StatusCreated)
	id := created.Members[0].EntityID
	if id == "" {
		t.Fatal("create response has no identity ID")
	}
	if rec := getEntity(t, f.app, id, ungranted); rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/entities/%s without a grant = %d, want 404: %s", id, rec.Code, rec.Body.String())
	}
	if rec := getEntity(t, f.app, id, f.admin); rec.Code != http.StatusOK {
		t.Fatalf("GET /api/entities/%s with a grant = %d, want 200", id, rec.Code)
	}
}

func TestEntityOverridesRejectOtherCallers(t *testing.T) {
	t.Parallel()
	f := newOverrideFixture(t)
	body := map[string]any{"action": "detach", "connectorId": f.c1.ID, "kind": "vm", "ref": f.hostA}

	// A per-connector operator on the member's connector is not an instance admin.
	opID, opToken := f.app.user(t, "viewer")
	f.app.connectorGrant(t, opID, f.c1.ID, "operator")
	if rec := f.app.req(t, http.MethodPost, "/api/entity-overrides", body, opToken); rec.Code != http.StatusForbidden {
		t.Errorf("connector operator POST = %d, want 403: %s", rec.Code, rec.Body.String())
	}

	// A connector-restricted key loses its owner's instance-admin rights.
	key := createKey(t, f.app, f.admin, map[string]any{"name": "narrow", "connectorIds": []string{f.c1.ID}})["token"].(string)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		if rec := f.app.req(t, method, "/api/entity-overrides", body, key); rec.Code != http.StatusForbidden {
			t.Errorf("restricted key %s = %d, want 403: %s", method, rec.Code, rec.Body.String())
		}
	}
	if list, err := f.app.Store.LoadEntityIdentityOverrides(context.Background()); err != nil || len(list) != 0 {
		t.Fatalf("rejected callers must not create anything: %v %v", list, err)
	}
}

func TestEntityOverrideDormantStateThroughAPI(t *testing.T) {
	t.Parallel()
	f := newOverrideFixture(t)
	// The solo connector's next snapshot no longer lists the vm: its member goes gone.
	seedSnapshots(t, f.app, f.solo, []snap{{at: "2026-09-02T00:00:00Z", entities: []connector.SnapshotEntity{{Kind: "service", Name: "Svc", ExternalID: "svc1"}}}})
	if _, err := doc.NewEngine(f.app.Store).BackfillEntityIdentities(context.Background()); err != nil {
		t.Fatal(err)
	}
	created := decodeOverride(t, f.detach(t, f.solo, "s1"), http.StatusCreated)
	if created.State != "dormant" {
		t.Fatalf("create on a gone member: state %q, want dormant", created.State)
	}
	var list []overrideBody
	rec := f.call(t, http.MethodGet, "/api/entity-overrides", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 || list[0].State != "dormant" {
		t.Fatalf("list state: %v %s", err, rec.Body.String())
	}
}

func TestEntityOverrideDeleteDetachRestoresHostnameMatch(t *testing.T) {
	t.Parallel()
	f := newOverrideFixture(t)
	created := decodeOverride(t, f.detach(t, f.c2, f.hostB), http.StatusCreated)
	detached := memberIdentity(t, created, f.c2.ID, f.hostB)
	deleted := decodeOverride(t, f.call(t, http.MethodDelete, "/api/entity-overrides/"+created.ID, nil), http.StatusOK)
	rejoined := memberIdentity(t, deleted, f.c2.ID, f.hostB)
	if rejoined == "" {
		t.Fatal("delete response has no identity ID")
	}
	other, err := f.app.Store.EntityOverrideMembers(context.Background(), store.EntityIdentityOverride{Action: "detach", ConnectorID: f.c1.ID, Kind: "vm", Ref: f.hostA})
	if err != nil || len(other) != 1 || other[0].EntityID != rejoined {
		t.Fatalf("after delete the member is in %q (detached was %q), its hostname match is in %v (%v)", rejoined, detached, other, err)
	}
}

type failingReconciler struct{}

func (failingReconciler) BackfillEntityIdentities(context.Context) (int, error) {
	return 0, errors.New("reconcile boom")
}

// A reconcile failure after a committed mutation returns 500 with the change
// (and its audit row) in place; the next sync reconciles it.
func TestEntityOverrideReconcileFailureKeepsMutationAndAudit(t *testing.T) {
	t.Parallel()
	f := newOverrideFixture(t)
	h := entities.NewHandler(f.app.Store, failingReconciler{})
	ctx := auth.ContextWithUser(context.Background(), f.adminID, true)
	audits := func(action string) int {
		_, total, err := f.app.Store.ListAuditRecords(ctx, action, "", "", "", 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		return total
	}

	body, _ := json.Marshal(map[string]any{"action": "detach", "connectorId": f.c1.ID, "kind": "vm", "ref": f.hostA})
	rec := httptest.NewRecorder()
	h.CreateOverride(rec, httptest.NewRequest(http.MethodPost, "/api/entity-overrides", strings.NewReader(string(body))).WithContext(ctx))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("create with failing reconcile = %d, want 500: %s", rec.Code, rec.Body.String())
	}
	stored, err := f.app.Store.LoadEntityIdentityOverrides(ctx)
	if err != nil || len(stored) != 1 || audits("entity.override.create") != 1 {
		t.Fatalf("create must stay committed with one audit row: %v %v", stored, err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/entity-overrides/"+stored[0].ID, nil).WithContext(ctx)
	req.SetPathValue("id", stored[0].ID)
	rec = httptest.NewRecorder()
	h.DeleteOverride(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("delete with failing reconcile = %d, want 500: %s", rec.Code, rec.Body.String())
	}
	left, err := f.app.Store.LoadEntityIdentityOverrides(ctx)
	if err != nil || len(left) != 0 || audits("entity.override.delete") != 1 {
		t.Fatalf("delete must stay committed with one audit row: %v %v", left, err)
	}
}
