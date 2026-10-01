package mcp

import (
	"context"
	"testing"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestRunbookTools(t *testing.T) {
	ctx := context.Background()
	h := newTestHarness(t)
	visible := createConnector(t, h.Store, "pve", "virtualization")
	hidden := createConnector(t, h.Store, "nas", "networking")
	userID := createUser(t, h.Store)
	for _, cid := range []string{visible, hidden} {
		if _, err := h.Store.UpsertConnectorGrant(ctx, userID, cid, "operator"); err != nil {
			t.Fatal(err)
		}
	}

	rb, _, err := h.Store.CreateRunbookWithSteps(ctx,
		&store.RunbookRecord{Title: "Recover storage", Body: "Restart in order", TargetType: "alert_severity", TargetValue: "critical"},
		[]*store.RunbookStepRecord{
			{Title: "Restart VM", ConnectorID: visible, Verb: "restart", EntityRef: "vm:100"},
			{Title: "Restart NAS", ConnectorID: hidden, Verb: "restart", EntityRef: "svc:smb"},
		})
	if err != nil {
		t.Fatal(err)
	}

	var list struct {
		Runbooks []runbookSummary `json:"runbooks"`
		Total    int              `json:"total"`
	}
	h.callTool(userCtx(userID), t, "list_runbooks", nil, &list)
	if list.Total != 1 || len(list.Runbooks) != 1 || list.Runbooks[0].StepCount != 2 {
		t.Fatalf("list = %+v", list)
	}

	type got struct {
		Title string            `json:"title"`
		Steps []runbookStepView `json:"steps"`
	}

	t.Run("all steps visible", func(t *testing.T) {
		var out got
		h.callTool(userCtx(userID), t, "get_runbook", map[string]any{"id": rb.ID}, &out)
		if len(out.Steps) != 2 || out.Steps[0].Redacted || out.Steps[1].Redacted || out.Steps[1].ConnectorName != "nas" {
			t.Fatalf("steps = %+v", out.Steps)
		}
	})

	t.Run("connector-limited key redacts other steps", func(t *testing.T) {
		var out got
		h.callTool(restrictedCtx(userID, []string{visible}), t, "get_runbook", map[string]any{"id": rb.ID}, &out)
		if len(out.Steps) != 2 || out.Steps[0].Redacted || !out.Steps[1].Redacted {
			t.Fatalf("steps = %+v", out.Steps)
		}
		r := out.Steps[1]
		if r.ConnectorID != "" || r.ConnectorName != "" || r.Verb != "" || r.EntityRef != "" || r.Title != "Restricted step" {
			t.Fatalf("redacted step leaks details: %+v", r)
		}
	})

	t.Run("missing runbook is a tool error", func(t *testing.T) {
		res, err := h.client.CallTool(userCtx(userID), mcpsdk.CallToolRequest{Params: mcpsdk.CallToolParams{Name: "get_runbook", Arguments: map[string]any{"id": "nope"}}})
		if err != nil || !res.IsError {
			t.Fatalf("res = %+v, err = %v, want tool error", res, err)
		}
	})
}

// get_runbook drops the linked doc id unless the caller may view the doc; a
// lab-wide doc is only viewable by an unrestricted instance admin.
func TestGetRunbookDocLinkVisibility(t *testing.T) {
	ctx := context.Background()
	h := newTestHarness(t)
	userID := createUser(t, h.Store)
	connID := createConnector(t, h.Store, "pve", "virtualization")
	if _, err := h.Store.UpsertConnectorGrant(ctx, userID, connID, "viewer"); err != nil {
		t.Fatal(err)
	}
	if err := h.Store.CreateDoc(ctx, &store.DocRecord{ID: "lab", Title: "Lab", Kind: "lab", Content: "x"}); err != nil {
		t.Fatal(err)
	}
	docID := "lab"
	rb, err := h.Store.CreateRunbook(ctx, &store.RunbookRecord{Title: "RB", Body: "b", TargetType: "alert_severity", TargetValue: "critical", DocID: &docID})
	if err != nil {
		t.Fatal(err)
	}
	docOf := func(c context.Context) string {
		var out struct {
			DocID string `json:"docId"`
		}
		h.callTool(c, t, "get_runbook", map[string]any{"id": rb.ID}, &out)
		return out.DocID
	}
	if got := docOf(adminCtx(userID)); got != "lab" {
		t.Errorf("admin docId = %q, want lab", got)
	}
	if got := docOf(userCtx(userID)); got != "" {
		t.Errorf("non-admin docId = %q, want hidden", got)
	}
	if got := docOf(restrictedAdminCtx(userID, []string{connID})); got != "" {
		t.Errorf("restricted key docId = %q, want hidden", got)
	}
}
