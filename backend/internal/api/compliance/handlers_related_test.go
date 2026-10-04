package compliance_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/api/compliance"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// relatedRuleBody is a docker-container rule whose related clause points at
// proxmox vms, using attributes that exist in the real catalog.
func relatedRuleBody(mode string, clauseConditions []map[string]any) map[string]any {
	clause := map[string]any{
		"mode": mode, "connectorType": "proxmox", "entityKind": "vm",
		"join": map[string]any{"sourceField": "name", "relatedField": "name"},
	}
	if clauseConditions != nil {
		clause["conditions"] = clauseConditions
	}
	return map[string]any{
		"name": "Related rule", "connectorType": "docker", "entityKind": "container",
		"conditions": []map[string]any{{"attribute": "image", "op": "contains", "value": "nginx"}},
		"related":    []map[string]any{clause},
		"severity":   "info", "title": "Related", "remediationLink": "", "enabled": false,
	}
}

func serve(t *testing.T, handler http.HandlerFunc, method, path, id string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, path, strings.NewReader(string(raw)))
	r.Header.Set("Content-Type", "application/json")
	if id != "" {
		r.SetPathValue("id", id)
	}
	rr := httptest.NewRecorder()
	handler(rr, r)
	return rr
}

func decodeMap(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %q: %v", rr.Body.String(), err)
	}
	return out
}

func TestCreateRuleStoresRelatedClausesAndGetReturnsThem(t *testing.T) {
	h := compliance.NewHandler(apitest.NewStore(t), nil)
	body := relatedRuleBody("requires", []map[string]any{{"attribute": "status", "op": "eq", "value": "running"}})

	created := serve(t, h.Create, http.MethodPost, "/api/compliance/rules", "", body)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", created.Code, created.Body.String())
	}
	id := decodeMap(t, created)["id"].(string)

	got := serve(t, h.Get, http.MethodGet, "/api/compliance/rules/"+id, id, nil)
	if got.Code != http.StatusOK {
		t.Fatalf("get status %d: %s", got.Code, got.Body.String())
	}
	// Round-trip through JSON so the comparison sees what the client sees.
	want := map[string]any{}
	raw, _ := json.Marshal(body["related"])
	var wantRelated []any
	if err := json.Unmarshal(raw, &wantRelated); err != nil {
		t.Fatal(err)
	}
	want["related"] = wantRelated
	if !reflect.DeepEqual(decodeMap(t, got)["related"], want["related"]) {
		t.Fatalf("related = %v, want %v", decodeMap(t, got)["related"], want["related"])
	}
}

func TestRuleWithoutRelatedClausesReturnsEmptyArray(t *testing.T) {
	h := compliance.NewHandler(apitest.NewStore(t), nil)
	body := relatedRuleBody("requires", nil)
	delete(body, "related")

	created := serve(t, h.Create, http.MethodPost, "/api/compliance/rules", "", body)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", created.Code, created.Body.String())
	}
	related, ok := decodeMap(t, created)["related"].([]any)
	if !ok || len(related) != 0 {
		t.Fatalf("related = %#v, want an empty array (never null)", decodeMap(t, created)["related"])
	}
}

func TestCreateRuleRejectsClauseConditionOnSourceOnlyAttribute(t *testing.T) {
	h := compliance.NewHandler(apitest.NewStore(t), nil)
	// "image" exists on docker containers (the source) but not on proxmox vms.
	body := relatedRuleBody("requires", []map[string]any{{"attribute": "image", "op": "eq", "value": "x"}})

	rr := serve(t, h.Create, http.MethodPost, "/api/compliance/rules", "", body)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "related[0].conditions[0].attribute") {
		t.Fatalf("status %d body %s, want 400 naming related[0].conditions[0].attribute", rr.Code, rr.Body.String())
	}
}

func TestCreateRuleRejectsMoreThanFiveClauses(t *testing.T) {
	h := compliance.NewHandler(apitest.NewStore(t), nil)
	body := relatedRuleBody("requires", nil)
	clause := body["related"].([]map[string]any)[0]
	body["related"] = []map[string]any{clause, clause, clause, clause, clause, clause}

	rr := serve(t, h.Create, http.MethodPost, "/api/compliance/rules", "", body)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), `"related"`) {
		t.Fatalf("status %d body %s, want 400 on field related", rr.Code, rr.Body.String())
	}
}

func seedConnector(t *testing.T, s *store.Store, name, category, typ, snapshot string) {
	t.Helper()
	ctx := context.Background()
	conn := &store.ConnectorRecord{Name: name, Category: category, Type: typ, URL: "https://example.test", Owner: "owner"}
	if err := s.CreateConnector(ctx, conn); err != nil {
		t.Fatalf("CreateConnector(%s): %v", name, err)
	}
	if snapshot == "" {
		return
	}
	if err := s.CreateSnapshot(ctx, &store.SnapshotRecord{ConnectorID: conn.ID, Data: snapshot, FetchedAt: "2020-01-01T00:00:00Z"}); err != nil {
		t.Fatalf("CreateSnapshot(%s): %v", name, err)
	}
}

func testItems(t *testing.T, rr *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp.Items
}

func TestTestEndpointEvaluatesRelatedClauses(t *testing.T) {
	s := apitest.NewStore(t)
	h := compliance.NewHandler(s, nil)
	seedConnector(t, s, "docker-1", "containers_paas", "docker",
		`{"entities":[`+
			`{"kind":"container","name":"web","attributes":{"image":"nginx"}},`+
			`{"kind":"container","name":"db","attributes":{"image":"nginx-db"}}]}`)

	requires := relatedRuleBody("requires", nil)

	// No proxmox connector at all: the rule is skipped, not reported as violated.
	if items := testItems(t, serve(t, h.Test, http.MethodPost, "/api/compliance/rules/test", "", requires)); len(items) != 0 {
		t.Fatalf("items without any related connector = %v, want none (skipped)", items)
	}

	// A proxmox snapshot that has only "web": "db" lacks its related vm.
	seedConnector(t, s, "pve-1", "virtualization", "proxmox",
		`{"entities":[{"kind":"vm","name":"web","attributes":{"status":"running","secret":"pve-private-vm"}}]}`)
	items := testItems(t, serve(t, h.Test, http.MethodPost, "/api/compliance/rules/test", "", requires))
	if len(items) != 1 || items[0]["connectorName"] != "docker-1" {
		t.Fatalf("items = %v, want one on docker-1", items)
	}
	entities := items[0]["entities"].([]any)
	if len(entities) != 1 || entities[0].(map[string]any)["name"] != "db" {
		t.Fatalf("entities = %v, want only db", entities)
	}

	// The report names source entities only: nothing of the related connector.
	raw, _ := json.Marshal(items)
	if strings.Contains(string(raw), "pve-1") || strings.Contains(string(raw), "pve-private-vm") {
		t.Fatalf("test result leaked related connector data: %s", raw)
	}

	// Flipped to forbids, the vm that does exist is the violation.
	forbids := relatedRuleBody("forbids", nil)
	items = testItems(t, serve(t, h.Test, http.MethodPost, "/api/compliance/rules/test", "", forbids))
	entities = items[0]["entities"].([]any)
	if len(items) != 1 || len(entities) != 1 || entities[0].(map[string]any)["name"] != "web" {
		t.Fatalf("forbids items = %v, want only web", items)
	}
}
