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
