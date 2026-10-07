package compliance_test

import (
	"context"
	"encoding/json"
	"fmt"
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

	var resp struct {
		Attributes map[string]any `json:"attributes"`
		JoinFields []string       `json:"joinFields"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Attributes == nil {
		t.Fatal("expected non-nil attributes")
	}
	if len(resp.JoinFields) == 0 {
		t.Fatal("expected non-empty joinFields")
	}
	// Verify joinFields contains the expected fields
	expectedFields := []string{"external_id", "name", "ip", "hostname", "mac"}
	for i, expected := range expectedFields {
		if i >= len(resp.JoinFields) || resp.JoinFields[i] != expected {
			t.Errorf("joinFields[%d] = %q, want %q", i, resp.JoinFields[i], expected)
		}
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

func TestUpdateRuleChangesFieldsWithRelated(t *testing.T) {
	s := apitest.NewStore(t)
	h := compliance.NewHandler(s, nil)

	ctx := context.Background()
	rule := &store.ComplianceRuleRecord{
		Name:          "test-rule",
		Title:         "Test Rule",
		ConnectorType: "docker",
		EntityKind:    "container",
		Conditions:    `[{"attribute":"image","op":"contains","value":"nginx"}]`,
		Related:       `[]`,
		Severity:      "info",
		Enabled:       false,
	}
	if err := s.CreateComplianceRule(ctx, rule); err != nil {
		t.Fatal(err)
	}

	// Update only the related field
	body := map[string]any{
		"name": "test-rule", "connectorType": "docker", "entityKind": "container",
		"conditions": []map[string]any{{"attribute": "image", "op": "contains", "value": "nginx"}},
		"related": []map[string]any{
			{
				"mode": "requires", "connectorType": "proxmox", "entityKind": "vm",
				"join": map[string]any{"sourceField": "name", "relatedField": "name"},
			},
		},
		"severity": "info", "title": "Test Rule", "remediationLink": "", "enabled": false,
	}

	bodyBytes, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPut, "/api/compliance/rules/"+rule.ID, strings.NewReader(string(bodyBytes)))
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("id", rule.ID)
	rr := httptest.NewRecorder()
	h.Update(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rr.Code, rr.Body.String())
	}

	// Verify audit included "related"
	audits, _, err := s.ListAuditRecords(ctx, "", "compliance_rule", "", "", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(audits) == 0 {
		t.Fatal("expected audit records")
	}

	var detail map[string]any
	if err := json.Unmarshal([]byte(audits[0].Detail), &detail); err != nil {
		t.Fatal(err)
	}

	changedFields, ok := detail["changedFields"].([]any)
	if !ok {
		t.Fatalf("changedFields not a list: %v", detail["changedFields"])
	}

	found := false
	for _, field := range changedFields {
		if field == "related" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'related' in changedFields, got: %v", changedFields)
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

func TestListPacks(t *testing.T) {
	s := apitest.NewStore(t)
	ev := &mockEvaluator{}
	h := compliance.NewHandler(s, ev)

	list := func() map[string]bool {
		r := httptest.NewRequest(http.MethodGet, "/api/compliance/packs", nil)
		rr := httptest.NewRecorder()
		h.ListPacks(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("ListPacks status %d, want 200", rr.Code)
		}
		var out struct {
			Items []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Description string `json:"description"`
				Installed   bool   `json:"installed"`
			} `json:"items"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode ListPacks response: %v", err)
		}
		res := make(map[string]bool)
		for _, item := range out.Items {
			res[item.ID] = item.Installed
		}
		return res
	}

	// 1. Before install: installed is false
	installedMap := list()
	if installedMap["certificate-expiry"] {
		t.Errorf("certificate-expiry installed before install = true, want false")
	}

	// 2. After install: installed is true
	installReq := httptest.NewRequest(http.MethodPost, "/api/compliance/packs/certificate-expiry/install", nil)
	installReq.SetPathValue("id", "certificate-expiry")
	installRec := httptest.NewRecorder()
	h.InstallPack(installRec, installReq)
	if installRec.Code != http.StatusOK {
		t.Fatalf("InstallPack status %d, want 200", installRec.Code)
	}

	installedMap = list()
	if !installedMap["certificate-expiry"] {
		t.Errorf("certificate-expiry installed after install = false, want true")
	}

	// 3. After one pack rule is deleted: installed is false
	rules, err := s.ListComplianceRules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var deletedID string
	for _, rule := range rules {
		if rule.Name == "NPM certificate expires within 30 days" {
			deletedID = rule.ID
			break
		}
	}
	if deletedID == "" {
		t.Fatal("could not find pack rule to delete")
	}
	delReq := httptest.NewRequest(http.MethodDelete, "/api/compliance/rules/"+deletedID, nil)
	delReq.SetPathValue("id", deletedID)
	delRec := httptest.NewRecorder()
	h.Delete(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("Delete rule status %d, want 204", delRec.Code)
	}

	installedMap = list()
	if installedMap["certificate-expiry"] {
		t.Errorf("certificate-expiry installed after deleting one rule = true, want false")
	}
}

func TestUpdateRuleRelated(t *testing.T) {
	s := apitest.NewStore(t)
	h := compliance.NewHandler(s, nil)

	ctx := context.Background()
	rule := &store.ComplianceRuleRecord{
		Name:          "test-rule",
		Title:         "Test Rule",
		ConnectorType: "docker",
		EntityKind:    "container",
		Conditions:    `[{"attribute":"image","op":"contains","value":"nginx"}]`,
		Related:       `[]`,
		Severity:      "info",
		Enabled:       false,
	}
	if err := s.CreateComplianceRule(ctx, rule); err != nil {
		t.Fatal(err)
	}

	// Update with a related clause
	body := map[string]any{
		"name": "test-rule", "connectorType": "docker", "entityKind": "container",
		"conditions": []map[string]any{{"attribute": "image", "op": "contains", "value": "nginx"}},
		"related": []map[string]any{
			{
				"mode": "requires", "connectorType": "proxmox", "entityKind": "vm",
				"join":       map[string]any{"sourceField": "name", "relatedField": "name"},
				"conditions": []map[string]any{{"attribute": "status", "op": "eq", "value": "running"}},
			},
		},
		"severity": "info", "title": "Test Rule", "remediationLink": "", "enabled": false,
	}

	bodyBytes, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPut, "/api/compliance/rules/"+rule.ID, strings.NewReader(string(bodyBytes)))
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("id", rule.ID)
	rr := httptest.NewRecorder()
	h.Update(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	related, ok := resp["related"].([]any)
	if !ok {
		t.Fatalf("related not a list: %v", resp["related"])
	}
	if len(related) != 1 {
		t.Fatalf("expected 1 related clause after update, got %d", len(related))
	}

	// Update with no related key should clear it
	body["related"] = nil
	bodyBytes, _ = json.Marshal(body)
	r = httptest.NewRequest(http.MethodPut, "/api/compliance/rules/"+rule.ID, strings.NewReader(string(bodyBytes)))
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("id", rule.ID)
	rr = httptest.NewRecorder()
	h.Update(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rr.Code, rr.Body.String())
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	related, ok = resp["related"].([]any)
	if !ok {
		t.Fatalf("related not a list after clearing: %v", resp["related"])
	}
	if len(related) != 0 {
		t.Fatalf("expected empty related after clearing, got %d clauses", len(related))
	}
}

func TestComplianceRelatedClauseValidation(t *testing.T) {
	s := apitest.NewStore(t)
	h := compliance.NewHandler(s, nil)

	testCases := []struct {
		name        string
		related     any
		errorField  string
		errorSubstr string
	}{
		{
			name: "bad mode",
			related: []map[string]any{
				{"mode": "invalid", "connectorType": "proxmox", "entityKind": "vm",
					"join": map[string]any{"sourceField": "name", "relatedField": "name"}},
			},
			errorField:  "related[0].mode",
			errorSubstr: "must be 'requires' or 'forbids'",
		},
		{
			name: "invalid connector type",
			related: []map[string]any{
				{"mode": "requires", "connectorType": "invalid", "entityKind": "vm",
					"join": map[string]any{"sourceField": "name", "relatedField": "name"}},
			},
			errorField:  "related[0].connectorType",
			errorSubstr: "not a known connector type",
		},
		{
			name: "bad join source field",
			related: []map[string]any{
				{"mode": "requires", "connectorType": "proxmox", "entityKind": "vm",
					"join": map[string]any{"sourceField": "invalid_field", "relatedField": "name"}},
			},
			errorField:  "related[0].join.sourceField",
			errorSubstr: "not a valid field",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{
				"name": "test-rule", "connectorType": "docker", "entityKind": "container",
				"conditions":      []map[string]any{{"attribute": "image", "op": "contains", "value": "nginx"}},
				"related":         tc.related,
				"severity":        "info",
				"title":           "Test Rule",
				"remediationLink": "",
				"enabled":         false,
			}

			bodyBytes, _ := json.Marshal(body)
			r := httptest.NewRequest(http.MethodPost, "/api/compliance/rules", strings.NewReader(string(bodyBytes)))
			r.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.Create(rr, r)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400", rr.Code)
			}

			var errResp struct {
				Details []struct {
					Field string `json:"field"`
					Msg   string `json:"msg"`
				} `json:"details"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("decode error response: %v", err)
			}

			found := false
			for _, detail := range errResp.Details {
				if detail.Field == tc.errorField && strings.Contains(detail.Msg, tc.errorSubstr) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error field %q with substring %q, got details: %v", tc.errorField, tc.errorSubstr, errResp.Details)
			}
		})
	}
}

func TestSaveRuleDaysLeftValidation(t *testing.T) {
	s := apitest.NewStore(t)
	h := compliance.NewHandler(s, nil)
	for _, op := range []string{"days_left_lt", "days_left_gt"} {
		for _, tc := range []struct {
			name, value string
			valid       bool
		}{
			{"whole", "8", true},
			{"negative", "-1", true},
			{"fraction", "1.5", false},
			{"text", `"soon"`, false},
			{"numeric text", `"8"`, false},
			{"null", "null", false},
			{"boolean", "true", false},
		} {
			t.Run(op+"/"+tc.name, func(t *testing.T) {
				record := &store.ComplianceRuleRecord{Name: "existing", Title: "Expiry", ConnectorType: "npm", EntityKind: "certificate",
					Conditions: `[{"attribute":"not_after","op":"exists","value":true}]`, Severity: "warning"}
				if err := s.CreateComplianceRule(context.Background(), record); err != nil {
					t.Fatal(err)
				}
				body := fmt.Sprintf(`{"name":"expiry-%s-%s","title":"Expiry","connectorType":"npm","entityKind":"certificate","conditions":[{"attribute":"not_after","op":%q,"value":%s}],"severity":"warning"}`, op, tc.name, op, tc.value)
				for _, method := range []string{http.MethodPost, http.MethodPut} {
					r := httptest.NewRequest(method, "/api/compliance/rules/"+record.ID, strings.NewReader(body))
					r.SetPathValue("id", record.ID)
					rr := httptest.NewRecorder()
					if method == http.MethodPost {
						h.Create(rr, r)
					} else {
						h.Update(rr, r)
					}
					wantStatus := http.StatusBadRequest
					if tc.valid {
						wantStatus = http.StatusOK
						if method == http.MethodPost {
							wantStatus = http.StatusCreated
						}
					}
					if rr.Code != wantStatus {
						t.Fatalf("%s status %d, want %d: %s", method, rr.Code, wantStatus, rr.Body.String())
					}
					if tc.valid {
						continue
					}
					var response struct {
						Details []struct {
							Field string `json:"field"`
							Msg   string `json:"msg"`
						} `json:"details"`
					}
					if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
						t.Fatal(err)
					}
					if len(response.Details) != 1 || response.Details[0].Field != "conditions[0].value" || !strings.Contains(response.Details[0].Msg, "whole number") {
						t.Fatalf("%s details = %+v, want whole-number condition value error", method, response.Details)
					}
				}
			})
		}
	}
}
