package compliance_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/api/compliance"
	_ "github.com/WiseLabz/wiselabz/internal/connector/all"
	"github.com/WiseLabz/wiselabz/internal/store"
)

type mockEvaluator struct {
	evaluateCalls int
	resolveCalls  int
}

func (m *mockEvaluator) EvaluateRule(_ context.Context, _ string) error {
	m.evaluateCalls++
	return nil
}

func (m *mockEvaluator) ResolveRule(_ context.Context, _ string) error {
	m.resolveCalls++
	return nil
}

func TestSchemaReturnsCatalog(t *testing.T) {
	s := apitest.NewStore(t)
	h := compliance.NewHandler(s, nil)

	r := httptest.NewRequest(http.MethodGet, "/api/compliance/schema", nil)
	rr := httptest.NewRecorder()
	h.Schema(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rr.Code, rr.Body.String())
	}

	var schema map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &schema); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if schema == nil {
		t.Fatal("expected non-nil catalog")
	}
}

func TestListRules(t *testing.T) {
	s := apitest.NewStore(t)
	h := compliance.NewHandler(s, nil)

	r := httptest.NewRequest(http.MethodGet, "/api/compliance/rules", nil)
	rr := httptest.NewRecorder()
	h.List(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("expected items list in response, got %v", resp["items"])
	}
	if len(items) <= 0 {
		t.Fatal("expected at least seed compliance rules")
	}
}

func TestCreateRuleValidation(t *testing.T) {
	s := apitest.NewStore(t)
	h := compliance.NewHandler(s, nil)

	for _, tc := range []struct {
		name   string
		body   string
		status int
	}{
		{"empty name", `{"name":"","title":"Rule","connectorType":"pfsense","entityKind":"rule","conditions":[],"severity":"info"}`, 400},
		{"empty title", `{"name":"rule1","title":"","connectorType":"pfsense","entityKind":"rule","conditions":[],"severity":"info"}`, 400},
		{"invalid severity", `{"name":"rule1","title":"Rule","connectorType":"pfsense","entityKind":"rule","conditions":[],"severity":"invalid"}`, 400},
		{"bad json", `{invalid json}`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/compliance/rules", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.Create(rr, r)

			if rr.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", rr.Code, tc.status, rr.Body.String())
			}
		})
	}
}

func TestCreateRuleCallsEvaluatorWhenEnabled(t *testing.T) {
	s := apitest.NewStore(t)
	m := &mockEvaluator{}
	h := compliance.NewHandler(s, m)

	ctx := context.Background()
	rule := &store.ComplianceRuleRecord{
		Name:          "test-rule",
		Title:         "Test Rule",
		ConnectorType: "pfsense",
		EntityKind:    "rule",
		Conditions:    `[{"attribute":"enabled","operator":"equals","value":"true"}]`,
		Severity:      "info",
		Enabled:       true,
	}
	if err := s.CreateComplianceRule(ctx, rule); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/compliance/rules/"+rule.ID, nil)
	r.SetPathValue("id", rule.ID)
	rr := httptest.NewRecorder()
	h.Get(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["name"] != "test-rule" {
		t.Fatalf("name %v, want test-rule", resp["name"])
	}
}

func TestCreateAndGetRuleAudit(t *testing.T) {
	s := apitest.NewStore(t)
	m := &mockEvaluator{}
	h := compliance.NewHandler(s, m)

	ctx := context.Background()
	rule := &store.ComplianceRuleRecord{
		Name:          "test-rule",
		Title:         "Test Rule",
		ConnectorType: "pfsense",
		EntityKind:    "rule",
		Conditions:    `[{"attribute":"enabled","operator":"equals","value":"true"}]`,
		Severity:      "info",
		Enabled:       false,
	}
	if err := s.CreateComplianceRule(ctx, rule); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/compliance/rules", nil)
	rr := httptest.NewRecorder()
	h.List(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("expected items list, got %v", resp)
	}
	found := false
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if ok && itemMap["name"] == "test-rule" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("test-rule not found in list")
	}
}

func TestGetRuleNotFound(t *testing.T) {
	s := apitest.NewStore(t)
	h := compliance.NewHandler(s, nil)

	r := httptest.NewRequest(http.MethodGet, "/api/compliance/rules/unknown", nil)
	r.SetPathValue("id", "unknown")
	rr := httptest.NewRecorder()
	h.Get(rr, r)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404: %s", rr.Code, rr.Body.String())
	}
}

func TestGetRuleSuccess(t *testing.T) {
	s := apitest.NewStore(t)
	h := compliance.NewHandler(s, nil)

	ctx := context.Background()
	rule := &store.ComplianceRuleRecord{
		Name:            "test-rule",
		Title:           "Test Rule",
		ConnectorType:   "pfsense",
		EntityKind:      "rule",
		Conditions:      `[{"attribute":"enabled","operator":"equals","value":"true"}]`,
		Severity:        "info",
		RemediationLink: "https://example.com",
		Enabled:         true,
	}
	if err := s.CreateComplianceRule(ctx, rule); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/compliance/rules/"+rule.ID, nil)
	r.SetPathValue("id", rule.ID)
	rr := httptest.NewRecorder()
	h.Get(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["name"] != "test-rule" {
		t.Fatalf("name %v, want test-rule", resp["name"])
	}
}

func TestUpdateRuleNotFound(t *testing.T) {
	s := apitest.NewStore(t)
	h := compliance.NewHandler(s, nil)

	body := `{"name":"rule","title":"Rule","connectorType":"pfsense","entityKind":"rule","conditions":[],"severity":"info"}`
	r := httptest.NewRequest(http.MethodPut, "/api/compliance/rules/unknown", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("id", "unknown")
	rr := httptest.NewRecorder()
	h.Update(rr, r)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404: %s", rr.Code, rr.Body.String())
	}
}

func TestUpdateRuleChangesFields(t *testing.T) {
	s := apitest.NewStore(t)
	h := compliance.NewHandler(s, nil)

	ctx := context.Background()
	rule := &store.ComplianceRuleRecord{
		Name:          "test-rule",
		Title:         "Test Rule",
		ConnectorType: "pfsense",
		EntityKind:    "rule",
		Conditions:    `[{"attribute":"enabled","operator":"equals","value":"true"}]`,
		Severity:      "info",
		Enabled:       false,
	}
	if err := s.CreateComplianceRule(ctx, rule); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/compliance/rules/"+rule.ID, nil)
	r.SetPathValue("id", rule.ID)
	rr := httptest.NewRecorder()
	h.Get(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["severity"] != "info" {
		t.Fatalf("severity %v, want 'info'", resp["severity"])
	}
}

func TestCreateAndDeleteRuleAudit(t *testing.T) {
	s := apitest.NewStore(t)
	m := &mockEvaluator{}
	h := compliance.NewHandler(s, m)

	ctx := context.Background()
	rule := &store.ComplianceRuleRecord{
		Name:          "test-rule",
		Title:         "Test Rule",
		ConnectorType: "docker",
		EntityKind:    "container",
		Conditions:    `[{"attribute":"enabled","operator":"equals","value":"true"}]`,
		Severity:      "info",
		Enabled:       true,
	}
	if err := s.CreateComplianceRule(ctx, rule); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodDelete, "/api/compliance/rules/"+rule.ID, nil)
	r.SetPathValue("id", rule.ID)
	rr := httptest.NewRecorder()
	h.Delete(rr, r)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status %d, want 204: %s", rr.Code, rr.Body.String())
	}
	if m.resolveCalls != 1 {
		t.Fatalf("expected 1 resolve call when deleting enabled rule, got %d", m.resolveCalls)
	}
}

func TestDeleteRuleNotFound(t *testing.T) {
	s := apitest.NewStore(t)
	h := compliance.NewHandler(s, nil)

	r := httptest.NewRequest(http.MethodDelete, "/api/compliance/rules/unknown", nil)
	r.SetPathValue("id", "unknown")
	rr := httptest.NewRecorder()
	h.Delete(rr, r)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404: %s", rr.Code, rr.Body.String())
	}
}

func TestDeleteRuleSuccess(t *testing.T) {
	s := apitest.NewStore(t)
	m := &mockEvaluator{}
	h := compliance.NewHandler(s, m)

	ctx := context.Background()
	rule := &store.ComplianceRuleRecord{
		Name:          "test-rule",
		Title:         "Test Rule",
		ConnectorType: "pfsense",
		EntityKind:    "rule",
		Conditions:    `[{"attribute":"enabled","operator":"equals","value":"true"}]`,
		Severity:      "info",
		Enabled:       true,
	}
	if err := s.CreateComplianceRule(ctx, rule); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodDelete, "/api/compliance/rules/"+rule.ID, nil)
	r.SetPathValue("id", rule.ID)
	rr := httptest.NewRecorder()
	h.Delete(rr, r)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status %d, want 204: %s", rr.Code, rr.Body.String())
	}
	if m.resolveCalls != 1 {
		t.Fatalf("expected 1 resolve call on delete, got %d", m.resolveCalls)
	}
}

func TestTestRuleValidation(t *testing.T) {
	s := apitest.NewStore(t)
	h := compliance.NewHandler(s, nil)

	body := `{"name":"","title":"Rule","connectorType":"pfsense","entityKind":"rule","conditions":[],"severity":"info"}`
	r := httptest.NewRequest(http.MethodPost, "/api/compliance/rules/test", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Test(rr, r)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400: %s", rr.Code, rr.Body.String())
	}
}

func TestInstallPack(t *testing.T) {
	s := apitest.NewStore(t)
	ev := &mockEvaluator{}
	h := compliance.NewHandler(s, ev)

	install := func(id string) (*httptest.ResponseRecorder, map[string]int) {
		r := httptest.NewRequest(http.MethodPost, "/api/compliance/packs/"+id+"/install", nil)
		r.SetPathValue("id", id)
		rr := httptest.NewRecorder()
		h.InstallPack(rr, r)
		var out map[string]int
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return rr, out
	}

	rr, out := install("recommended")
	if rr.Code != http.StatusOK || out["installed"] == 0 {
		t.Fatalf("first install: status %d body %s", rr.Code, rr.Body.String())
	}
	if ev.evaluateCalls != out["installed"] {
		t.Fatalf("evaluateCalls = %d, want %d", ev.evaluateCalls, out["installed"])
	}
	total := out["installed"] + out["skipped"]

	rr, out = install("recommended")
	if rr.Code != http.StatusOK || out["installed"] != 0 || out["skipped"] != total {
		t.Fatalf("second install should be a no-op: %s", rr.Body.String())
	}

	if rr, _ := install("nope"); rr.Code != http.StatusNotFound {
		t.Fatalf("unknown pack status %d, want 404", rr.Code)
	}
}
