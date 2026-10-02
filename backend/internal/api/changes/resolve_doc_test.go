package changes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/doc"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// seedDocConflict stores a doc whose "snap.status" block was edited by a
// human and an open doc_conflict Change proposing "degraded" for it.
func seedDocConflict(t *testing.T, h *Handler) (*store.ChangeRecord, *store.DocRecord) {
	t.Helper()
	ctx := context.Background()
	conn := &store.ConnectorRecord{Name: "Node", Category: "virtualization", Type: "proxmox", URL: "https://example.test"}
	if err := h.Store.CreateConnector(ctx, conn); err != nil {
		t.Fatalf("create connector: %v", err)
	}
	edited := doc.NewBlock("snap.status", "healthy")
	edited.Body = "healthy (human note)"
	d := &store.DocRecord{
		Title: "Node", Kind: "service", ServiceID: conn.ID,
		Content: doc.RenderSegments([]doc.Segment{{Text: "intro\n\n"}, {Block: &edited}, {Text: "\n"}}),
	}
	if err := h.Store.CreateDoc(ctx, d); err != nil {
		t.Fatalf("create doc: %v", err)
	}
	gen := doc.NewBlock("snap.status", "degraded")
	diff, _ := json.Marshal(doc.ChangeDiff{Format: "doc", DocID: d.ID, Key: "snap.status",
		Human: edited.Body, Generated: gen.Body, GenHash: gen.Hash})
	c := &store.ChangeRecord{
		ServiceID: conn.ID, ChangeType: doc.ChangeTypeDocConflict, Severity: "info", Summary: "conflict",
		Diff: string(diff), AffectedDocIDs: `["` + d.ID + `"]`, PatternID: "doc_conflict:" + d.ID + ":snap.status",
	}
	if err := h.Store.CreateChange(ctx, c); err != nil {
		t.Fatalf("create change: %v", err)
	}
	return c, d
}

func resolveDoc(t *testing.T, h *Handler, changeID, body, role, connectorID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/changes/"+changeID+"/resolve-doc", strings.NewReader(body))
	req.SetPathValue("id", changeID)
	userID := apitest.NewUser(t, h.Store, "viewer")
	if role != "" {
		apitest.GrantConnectorRole(t, h.Store, userID, connectorID, role)
	}
	req = req.WithContext(auth.ContextWithUser(req.Context(), userID, false))
	rr := httptest.NewRecorder()
	h.ResolveDoc(rr, req)
	return rr
}

func TestResolveDocAcceptAndKeep(t *testing.T) {
	for _, tt := range []struct {
		action, wantIn, wantOut string
		stillBlock              bool
	}{
		{"accept", "degraded", "human note", true},
		{"keep", "healthy (human note)", "degraded", false},
	} {
		t.Run(tt.action, func(t *testing.T) {
			h := newTestHandler(t)
			c, d := seedDocConflict(t, h)
			rr := resolveDoc(t, h, c.ID, `{"action":"`+tt.action+`"}`, "operator", c.ServiceID)
			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d; body=%s", rr.Code, rr.Body.String())
			}
			var detail map[string]any
			_ = json.Unmarshal(rr.Body.Bytes(), &detail)
			if detail["status"] != "acknowledged" {
				t.Fatalf("change status = %v, want acknowledged", detail["status"])
			}
			got, _ := h.Store.GetDoc(context.Background(), d.ID)
			if !strings.Contains(got.Content, tt.wantIn) || strings.Contains(got.Content, tt.wantOut) || !strings.HasPrefix(got.Content, "intro\n\n") {
				t.Fatalf("doc after %s:\n%s", tt.action, got.Content)
			}
			if hasBlock := strings.Contains(got.Content, `key="snap.status"`); hasBlock != tt.stillBlock {
				t.Fatalf("block present = %v, want %v", hasBlock, tt.stillBlock)
			}
			versions, _ := h.Store.GetDocVersions(context.Background(), d.ID)
			if len(versions) == 0 || versions[0].Trigger != "sync-resolve" || versions[0].Author == "" {
				t.Fatalf("latest version = %+v, want an authored sync-resolve version", versions)
			}
		})
	}
}

func TestResolveDocErrors(t *testing.T) {
	h := newTestHandler(t)
	c, d := seedDocConflict(t, h)

	if rr := resolveDoc(t, h, "missing", `{"action":"accept"}`, "", ""); rr.Code != http.StatusNotFound {
		t.Fatalf("missing change: status = %d, want 404", rr.Code)
	}
	if rr := resolveDoc(t, h, c.ID, `{"action":"accept"}`, "", ""); rr.Code != http.StatusNotFound {
		t.Fatalf("no grant: status = %d, want 404", rr.Code)
	}
	if rr := resolveDoc(t, h, c.ID, `{"action":"accept"}`, "viewer", c.ServiceID); rr.Code != http.StatusForbidden {
		t.Fatalf("viewer: status = %d, want 403", rr.Code)
	}
	if rr := resolveDoc(t, h, c.ID, `{"action":"merge"}`, "operator", c.ServiceID); rr.Code != http.StatusBadRequest {
		t.Fatalf("bad action: status = %d, want 400", rr.Code)
	}

	// Block removed by hand since the conflict was raised.
	v := 1
	if _, err := h.Store.UpdateDocWithVersion(context.Background(), d.ID, "rewritten\n", &v, "u", "manual"); err != nil {
		t.Fatalf("update doc: %v", err)
	}
	if rr := resolveDoc(t, h, c.ID, `{"action":"accept"}`, "operator", c.ServiceID); rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "block_gone") {
		t.Fatalf("block gone: status = %d body=%s, want 409 block_gone", rr.Code, rr.Body.String())
	}

	// Not a doc change at all.
	infra := &store.ChangeRecord{ServiceID: c.ServiceID, ChangeType: "config", Severity: "info", Summary: "x", Diff: "[]"}
	if err := h.Store.CreateChange(context.Background(), infra); err != nil {
		t.Fatalf("create change: %v", err)
	}
	if rr := resolveDoc(t, h, infra.ID, `{"action":"accept"}`, "operator", c.ServiceID); rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "not_resolvable") {
		t.Fatalf("infra change: status = %d body=%s, want 409 not_resolvable", rr.Code, rr.Body.String())
	}
}

func TestChangeDetailRendersDocDiff(t *testing.T) {
	h := newTestHandler(t)
	c, _ := seedDocConflict(t, h)
	detail, err := h.changeDetail(context.Background(), c.ID)
	if err != nil {
		t.Fatalf("changeDetail() error: %v", err)
	}
	diff := detail["diff"].(map[string]any)
	if diff["format"] != "doc" || diff["baseText"] != "healthy (human note)" || diff["headText"] != "degraded" {
		t.Fatalf("diff = %+v", diff)
	}
}
