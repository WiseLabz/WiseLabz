package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/backup"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestBackupExportRoleBoundary(t *testing.T) {
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")

	rec := app.req(t, http.MethodGet, "/api/system/backup/export", nil, viewerToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body)
	}
}

func TestBackupExportRedactsSecrets(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")

	cfg, err := store.MarshalConnectorConfig(map[string]any{
		"url": "https://pve.example.com", "token_id": "root@pam!monitoring", "token_secret": "super-secret-token",
	})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := app.Store.CreateConnector(context.Background(), &store.ConnectorRecord{
		Name: "pve1", Category: "virtualization", Type: "proxmox", URL: "https://pve.example.com", ConfigData: cfg,
	}); err != nil {
		t.Fatalf("create connector: %v", err)
	}

	rec := app.req(t, http.MethodGet, "/api/system/backup/export", nil, opToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if strings.Contains(body, "token_secret") || strings.Contains(body, "super-secret-token") {
		t.Errorf("export body contains connector secret: %s", body)
	}
	if strings.Contains(body, "api_secret") {
		t.Errorf("export body contains opnsense secret field name: %s", body)
	}
}

func TestBackupImportBadVersion(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")

	rec := app.req(t, http.MethodPost, "/api/system/backup/import", map[string]any{"version": 9999}, opToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
	}
}

func TestBackupExportImportRoundTrip(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")

	if err := app.Store.CreateConnector(context.Background(), &store.ConnectorRecord{
		Name: "pve1", Category: "virtualization", Type: "proxmox", URL: "https://pve.example.com",
	}); err != nil {
		t.Fatalf("create connector: %v", err)
	}
	doc := &store.DocRecord{Title: "Doc 1", Kind: "lab", Content: "hello"}
	if err := app.Store.CreateDoc(context.Background(), doc); err != nil {
		t.Fatalf("create doc: %v", err)
	}
	if err := app.Store.CreateDocVersion(context.Background(), &store.DocVersionRecord{
		DocID: doc.ID, Rev: 1, Content: "hello", Trigger: "manual",
	}); err != nil {
		t.Fatalf("create doc version: %v", err)
	}
	tmpl := &store.TemplateRecord{Name: "Template 1"}
	if err := app.Store.CreateTemplate(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	if err := app.Store.CreateTemplateSection(context.Background(), &store.TemplateSectionRecord{
		TemplateID: tmpl.ID, Title: "Section 1", Ord: 0, Body: "body",
	}); err != nil {
		t.Fatalf("create template section: %v", err)
	}

	exportRec := app.req(t, http.MethodGet, "/api/system/backup/export", nil, opToken)
	if exportRec.Code != http.StatusOK {
		t.Fatalf("export status = %d, want 200; body = %s", exportRec.Code, exportRec.Body)
	}
	var bundle backup.Bundle
	if err := json.Unmarshal(exportRec.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("decode export body: %v", err)
	}

	// Import into a fresh second app/store so nothing already exists.
	app2 := newTestApp(t)
	_, opToken2 := app2.user(t, "operator")

	importRec := app2.req(t, http.MethodPost, "/api/system/backup/import", bundle, opToken2)
	if importRec.Code != http.StatusOK {
		t.Fatalf("import status = %d, want 200; body = %s", importRec.Code, importRec.Body)
	}
	var result backup.Result
	if err := json.Unmarshal(importRec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode import body: %v", err)
	}
	if result.Connectors.Imported != 1 {
		t.Errorf("Connectors.Imported = %d, want 1", result.Connectors.Imported)
	}
	if result.Docs.Imported != 1 {
		t.Errorf("Docs.Imported = %d, want 1", result.Docs.Imported)
	}
	if result.DocVersions.Imported != 1 {
		t.Errorf("DocVersions.Imported = %d, want 1", result.DocVersions.Imported)
	}
	if result.Templates.Imported != 1 {
		t.Errorf("Templates.Imported = %d, want 1", result.Templates.Imported)
	}
	if result.TemplateSections.Imported != 1 {
		t.Errorf("TemplateSections.Imported = %d, want 1", result.TemplateSections.Imported)
	}
}
