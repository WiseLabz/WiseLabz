package mcp

import (
	"context"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestProposeDocEditSnapshotsConnectorScope(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()
	user := createUser(t, h.Store)
	cid := createConnector(t, h.Store, "Lab", "virtualization")
	if _, err := h.Store.UpsertConnectorGrant(ctx, user, cid, "operator"); err != nil {
		t.Fatal(err)
	}
	doc := store.DocRecord{Title: "Doc", Kind: "service", ServiceID: cid, Origin: store.DocOriginHuman, Content: "original", CurrentVersion: 1}
	if err := h.Store.CreateDoc(ctx, &doc); err != nil {
		t.Fatal(err)
	}
	h.callTool(userCtx(user), t, "propose_doc_edit", map[string]any{"docId": doc.ID, "content": "proposed"}, nil)
	records, total, err := h.Store.ListAuditRecords(ctx, "doc.edit_proposed", "doc", "", "", 0, 100)
	if err != nil || total != 1 || records[0].TargetID != doc.ID {
		t.Fatalf("proposal audit = %+v, %d, %v", records, total, err)
	}
	var scope string
	if err := h.Store.DB().QueryRowContext(ctx, "SELECT connector_id FROM audit_log_connectors WHERE audit_id = ?", records[0].ID).Scan(&scope); err != nil || scope != cid {
		t.Fatalf("proposal scope = %q, want %q: %v", scope, cid, err)
	}
}
