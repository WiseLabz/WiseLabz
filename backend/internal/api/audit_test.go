package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestAuditListRoleBoundary(t *testing.T) {
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")

	rec := app.req(t, http.MethodGet, "/api/system/audit", nil, viewerToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body)
	}
}

// TestConnectorCreateProducesAuditRecord exercises one instrumented action
// end-to-end: creating a connector must leave a matching row retrievable
// through the audit list endpoint, attributed to the acting operator.
func TestConnectorCreateProducesAuditRecord(t *testing.T) {
	app := newTestApp(t)
	opUserID, opToken := app.user(t, "operator")

	createRec := app.req(t, http.MethodPost, "/api/connectors", map[string]any{
		"name": "svc", "category": "virtualization", "type": "proxmox", "url": "https://example.com",
	}, opToken)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201; body = %s", createRec.Code, createRec.Body)
	}
	var created store.ConnectorRecord
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create body: %v", err)
	}

	listRec := app.req(t, http.MethodGet, "/api/system/audit", nil, opToken)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body = %s", listRec.Code, listRec.Body)
	}

	var page struct {
		Items []store.AuditRecord `json:"items"`
		Total int                 `json:"total"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode list body: %v", err)
	}

	var found *store.AuditRecord
	for i := range page.Items {
		if page.Items[i].Action == "connector.create" && page.Items[i].TargetID == created.ID {
			found = &page.Items[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no audit record for connector.create/%s in %+v", created.ID, page.Items)
	}
	if found.ActorUserID != opUserID {
		t.Errorf("ActorUserID = %q, want %q", found.ActorUserID, opUserID)
	}
	if found.ActorRole != "operator" {
		t.Errorf("ActorRole = %q, want operator", found.ActorRole)
	}
	if found.TargetType != "connector" {
		t.Errorf("TargetType = %q, want connector", found.TargetType)
	}
}

func TestAuditListFiltersByAction(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")

	for _, name := range []string{"svc-a", "svc-b"} {
		rec := app.req(t, http.MethodPost, "/api/connectors", map[string]any{
			"name": name, "category": "virtualization", "type": "proxmox", "url": "https://example.com",
		}, opToken)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %s status = %d, want 201; body = %s", name, rec.Code, rec.Body)
		}
	}

	rec := app.req(t, http.MethodGet, "/api/system/audit?action=connector.create", nil, opToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}
	var page struct {
		Items []store.AuditRecord `json:"items"`
		Total int                 `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if page.Total != 2 {
		t.Fatalf("Total = %d, want 2", page.Total)
	}
	for _, r := range page.Items {
		if r.Action != "connector.create" {
			t.Errorf("item action = %q, want connector.create", r.Action)
		}
	}
}

func TestAlertResolveProducesAuditRecord(t *testing.T) {
	app := newTestApp(t)
	opUserID, opToken := app.user(t, "operator")
	a := seedAlert(t, app)

	rec := app.req(t, http.MethodPost, "/api/alerts/"+a.ID+"/resolve", nil, opToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200; body = %s", rec.Code, rec.Body)
	}

	auditRec := app.req(t, http.MethodGet, "/api/system/audit", nil, opToken)
	if auditRec.Code != http.StatusOK {
		t.Fatalf("audit list status = %d, want 200; body = %s", auditRec.Code, auditRec.Body)
	}

	var page struct {
		Items []store.AuditRecord `json:"items"`
		Total int                 `json:"total"`
	}
	if err := json.Unmarshal(auditRec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode audit body: %v", err)
	}

	var found *store.AuditRecord
	for i := range page.Items {
		if page.Items[i].Action == "alert.resolve" && page.Items[i].TargetID == a.ID {
			found = &page.Items[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no audit record for alert.resolve/%s", a.ID)
	}
	if found.TargetType != "alert" {
		t.Errorf("TargetType = %q, want alert", found.TargetType)
	}
	if found.ActorUserID != opUserID {
		t.Errorf("ActorUserID = %q, want %q", found.ActorUserID, opUserID)
	}
	if found.ActorRole != "operator" {
		t.Errorf("ActorRole = %q, want operator", found.ActorRole)
	}
}

func TestAlertDismissProducesAuditRecord(t *testing.T) {
	app := newTestApp(t)
	opUserID, opToken := app.user(t, "operator")
	a := seedAlert(t, app)

	rec := app.req(t, http.MethodPost, "/api/alerts/"+a.ID+"/dismiss", nil, opToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("dismiss status = %d, want 200; body = %s", rec.Code, rec.Body)
	}

	auditRec := app.req(t, http.MethodGet, "/api/system/audit", nil, opToken)
	if auditRec.Code != http.StatusOK {
		t.Fatalf("audit list status = %d, want 200; body = %s", auditRec.Code, auditRec.Body)
	}

	var page struct {
		Items []store.AuditRecord `json:"items"`
		Total int                 `json:"total"`
	}
	if err := json.Unmarshal(auditRec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode audit body: %v", err)
	}

	var found *store.AuditRecord
	for i := range page.Items {
		if page.Items[i].Action == "alert.dismiss" && page.Items[i].TargetID == a.ID {
			found = &page.Items[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no audit record for alert.dismiss/%s", a.ID)
	}
	if found.TargetType != "alert" {
		t.Errorf("TargetType = %q, want alert", found.TargetType)
	}
	if found.ActorUserID != opUserID {
		t.Errorf("ActorUserID = %q, want %q", found.ActorUserID, opUserID)
	}
	if found.ActorRole != "operator" {
		t.Errorf("ActorRole = %q, want operator", found.ActorRole)
	}
}

func TestAlertSnoozeProducesAuditRecord(t *testing.T) {
	app := newTestApp(t)
	opUserID, opToken := app.user(t, "operator")
	a := seedAlert(t, app)

	rec := app.req(t, http.MethodPost, "/api/alerts/"+a.ID+"/snooze", map[string]any{"until": "2099-01-01T00:00:00Z"}, opToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("snooze status = %d, want 200; body = %s", rec.Code, rec.Body)
	}

	auditRec := app.req(t, http.MethodGet, "/api/system/audit", nil, opToken)
	if auditRec.Code != http.StatusOK {
		t.Fatalf("audit list status = %d, want 200; body = %s", auditRec.Code, auditRec.Body)
	}

	var page struct {
		Items []store.AuditRecord `json:"items"`
		Total int                 `json:"total"`
	}
	if err := json.Unmarshal(auditRec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode audit body: %v", err)
	}

	var found *store.AuditRecord
	for i := range page.Items {
		if page.Items[i].Action == "alert.snooze" && page.Items[i].TargetID == a.ID {
			found = &page.Items[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no audit record for alert.snooze/%s", a.ID)
	}
	if found.TargetType != "alert" {
		t.Errorf("TargetType = %q, want alert", found.TargetType)
	}
	if found.ActorUserID != opUserID {
		t.Errorf("ActorUserID = %q, want %q", found.ActorUserID, opUserID)
	}
	if found.ActorRole != "operator" {
		t.Errorf("ActorRole = %q, want operator", found.ActorRole)
	}
}

func TestFindingResolveProducesAuditRecord(t *testing.T) {
	app := newTestApp(t)
	opUserID, opToken := app.user(t, "operator")
	finding := seedQualityFinding(t, app)

	rec := app.req(t, http.MethodPost, "/api/findings/"+finding.ID+"/resolve", nil, opToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200; body = %s", rec.Code, rec.Body)
	}

	auditRec := app.req(t, http.MethodGet, "/api/system/audit", nil, opToken)
	if auditRec.Code != http.StatusOK {
		t.Fatalf("audit list status = %d, want 200; body = %s", auditRec.Code, auditRec.Body)
	}

	var page struct {
		Items []store.AuditRecord `json:"items"`
		Total int                 `json:"total"`
	}
	if err := json.Unmarshal(auditRec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode audit body: %v", err)
	}

	var found *store.AuditRecord
	for i := range page.Items {
		if page.Items[i].Action == "finding.resolve" && page.Items[i].TargetID == finding.ID {
			found = &page.Items[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no audit record for finding.resolve/%s", finding.ID)
	}
	if found.TargetType != "finding" {
		t.Errorf("TargetType = %q, want finding", found.TargetType)
	}
	if found.ActorUserID != opUserID {
		t.Errorf("ActorUserID = %q, want %q", found.ActorUserID, opUserID)
	}
	if found.ActorRole != "operator" {
		t.Errorf("ActorRole = %q, want operator", found.ActorRole)
	}
}
