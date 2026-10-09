package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

// TestConnectorActionStepMCP covers the connector_action step in MCP: the
// action name is reported on the authored step and on the frozen run step for
// a caller who can view the connector, the step is redacted otherwise, and the
// frozen fingerprint never leaves the server.
func TestConnectorActionStepMCP(t *testing.T) {
	ctx := context.Background()
	h := newTestHarness(t)
	visible := createConnector(t, h.Store, "library", "virtualization")
	hidden := createConnector(t, h.Store, "vault", "networking")
	userID := createUser(t, h.Store)
	for _, cid := range []string{visible, hidden} {
		if _, err := h.Store.UpsertConnectorGrant(ctx, userID, cid, "operator"); err != nil {
			t.Fatal(err)
		}
	}

	rb, _, err := h.Store.CreateRunbookWithSteps(ctx,
		&store.RunbookRecord{Title: "Rescan", Body: "b", TargetType: "alert_severity", TargetValue: "critical"},
		[]*store.RunbookStepRecord{
			{Kind: "connector_action", Title: "Rescan library", ConnectorID: visible, Action: "rescan"},
			{Kind: "connector_action", Title: "Drain vault", ConnectorID: hidden, Action: "drain", EntityRef: "db|1"},
		})
	if err != nil {
		t.Fatal(err)
	}

	type got struct {
		Steps []runbookStepView `json:"steps"`
	}

	t.Run("restricted key sees the action only on the visible connector", func(t *testing.T) {
		var out got
		h.callTool(restrictedCtx(userID, []string{visible}), t, "get_runbook", map[string]any{"id": rb.ID}, &out)
		if len(out.Steps) != 2 {
			t.Fatalf("steps = %+v", out.Steps)
		}
		if first := out.Steps[0]; first.Redacted || first.Kind != "connector_action" || first.Action != "rescan" {
			t.Errorf("visible step = %+v, want connector_action rescan", first)
		}
		if hiddenStep := out.Steps[1]; !hiddenStep.Redacted || hiddenStep.Action != "" || hiddenStep.Kind != "" || hiddenStep.EntityRef != "" {
			t.Errorf("hidden step = %+v, want redacted with no action", hiddenStep)
		}
	})

	t.Run("full access reports the action on both steps", func(t *testing.T) {
		var out got
		h.callTool(userCtx(userID), t, "get_runbook", map[string]any{"id": rb.ID}, &out)
		if len(out.Steps) != 2 || out.Steps[1].Action != "drain" || out.Steps[1].ConnectorName != "vault" {
			t.Errorf("steps = %+v, want drain on vault", out.Steps)
		}
	})

	t.Run("run history carries the action and never the fingerprint", func(t *testing.T) {
		run, _, err := h.Store.CreateRunbookRun(ctx, rb.ID, userID, []*store.RunbookRunStepRecord{
			{Kind: "connector_action", Title: "Rescan library", ConnectorID: visible, Action: "rescan", ActionFingerprint: "fp-secret-7"},
			{Kind: "connector_action", Title: "Drain vault", ConnectorID: hidden, Action: "drain", EntityRef: "db|1", ActionFingerprint: "fp-secret-8"},
		})
		if err != nil {
			t.Fatal(err)
		}

		var raw map[string]any
		h.callTool(restrictedCtx(userID, []string{visible}), t, "get_runbook_run", map[string]any{"id": run.ID}, &raw)
		restricted, err := json.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		text := string(restricted)
		if !strings.Contains(text, `"action":"rescan"`) {
			t.Errorf("run history lacks the visible action: %s", text)
		}
		for _, leaked := range []string{"fp-secret", "actionFingerprint", "drain", "Drain vault", "vault", "db|1"} {
			if strings.Contains(text, leaked) {
				t.Errorf("restricted run history leaks %q: %s", leaked, text)
			}
		}

		var full map[string]any
		h.callTool(userCtx(userID), t, "get_runbook_run", map[string]any{"id": run.ID}, &full)
		fullText, err := json.Marshal(full)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(fullText), `"action":"drain"`) || strings.Contains(string(fullText), "fp-secret") || strings.Contains(string(fullText), "actionFingerprint") {
			t.Errorf("full run history = %s; want drain action and no fingerprint", fullText)
		}
	})
}
