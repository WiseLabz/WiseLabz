package api_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

func TestComplianceSchemaRequiresInstanceAdmin(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")

	rec := app.req(t, http.MethodGet, "/api/compliance/schema", nil, viewerToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body)
	}
}

func TestComplianceSchemaReturnsConnectorAttributeCatalog(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	_, adminToken := app.user(t, "operator")

	rec := app.req(t, http.MethodGet, "/api/compliance/schema", nil, adminToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}

	var resp struct {
		Attributes map[string]map[string][]struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"attributes"`
		JoinFields []string `json:"joinFields"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if want := []string{"external_id", "name", "ip", "hostname", "mac"}; !reflect.DeepEqual(resp.JoinFields, want) {
		t.Fatalf("joinFields = %v, want %v", resp.JoinFields, want)
	}

	schema := resp.Attributes
	pfsense, ok := schema["pfsense"]
	if !ok {
		t.Fatalf("schema missing pfsense connector type: %+v", schema)
	}
	ruleAttrs, ok := pfsense["rule"]
	if !ok || len(ruleAttrs) == 0 {
		t.Fatalf("schema missing pfsense rule attributes: %+v", pfsense)
	}
	found := false
	for _, a := range ruleAttrs {
		if a.Name == "enabled" && a.Type == "boolean" {
			found = true
		}
	}
	if !found {
		t.Errorf("pfsense rule attributes missing boolean \"enabled\": %+v", ruleAttrs)
	}
}
