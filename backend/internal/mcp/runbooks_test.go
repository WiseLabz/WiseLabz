package mcp

import (
	"context"
	"strings"
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

// get_runbook reports each visible step's kind and wait timeout, redacts a
// connector step the caller cannot view down to nothing, and never redacts a
// manual step (it has no connector).
func TestGetRunbookStepKinds(t *testing.T) {
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
		&store.RunbookRecord{Title: "Failover", Body: "b", TargetType: "alert_severity", TargetValue: "critical"},
		[]*store.RunbookStepRecord{
			{Title: "Restart VM", ConnectorID: visible, Verb: "restart", EntityRef: "vm:100"},
			{Title: "Wait for NAS", Kind: "wait_until_healthy", TimeoutSeconds: 600, ConnectorID: hidden},
			{Title: "Check the console", Kind: "manual"},
		})
	if err != nil {
		t.Fatal(err)
	}

	type got struct {
		Steps []runbookStepView `json:"steps"`
	}

	t.Run("restricted key redacts the wait step only", func(t *testing.T) {
		var out got
		h.callTool(restrictedCtx(userID, []string{visible}), t, "get_runbook", map[string]any{"id": rb.ID}, &out)
		if len(out.Steps) != 3 {
			t.Fatalf("steps = %+v", out.Steps)
		}
		if out.Steps[0].Redacted || out.Steps[0].Kind != "lifecycle" || out.Steps[0].TimeoutSeconds != 0 {
			t.Errorf("lifecycle step = %+v, want visible lifecycle with no timeout", out.Steps[0])
		}
		wait := out.Steps[1]
		if !wait.Redacted || wait.Title != "Restricted step" || wait.Kind != "" || wait.TimeoutSeconds != 0 || wait.ConnectorID != "" {
			t.Errorf("wait step = %+v, want redacted with no kind or timeout", wait)
		}
		manual := out.Steps[2]
		if manual.Redacted || manual.Title != "Check the console" || manual.Kind != "manual" {
			t.Errorf("manual step = %+v, want visible title and kind manual", manual)
		}
	})

	t.Run("full access reports kind and timeout", func(t *testing.T) {
		var out got
		h.callTool(userCtx(userID), t, "get_runbook", map[string]any{"id": rb.ID}, &out)
		if len(out.Steps) != 3 {
			t.Fatalf("steps = %+v", out.Steps)
		}
		wait := out.Steps[1]
		if wait.Redacted || wait.Kind != "wait_until_healthy" || wait.TimeoutSeconds != 600 || wait.ConnectorName != "nas" {
			t.Errorf("wait step = %+v, want wait_until_healthy/600 on nas", wait)
		}
		if out.Steps[2].Redacted || out.Steps[2].Kind != "manual" {
			t.Errorf("manual step = %+v", out.Steps[2])
		}
	})
}

func TestRunHistoryToolsVisibility(t *testing.T) {
	ctx := context.Background()
	h := newTestHarness(t)
	if _, err := h.Store.DB().ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}
	visible := createConnector(t, h.Store, "visible", "virtualization")
	hidden := createConnector(t, h.Store, "hidden", "networking")
	user := createUser(t, h.Store)
	mixed := createUser(t, h.Store)
	for _, id := range []string{visible, hidden} {
		if _, err := h.Store.UpsertConnectorGrant(ctx, user, id, "viewer"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.Store.UpsertConnectorGrant(ctx, mixed, visible, "viewer"); err != nil {
		t.Fatal(err)
	}
	rb, err := h.Store.CreateRunbook(ctx, &store.RunbookRecord{Title: "Run history", TargetType: "change_type", TargetValue: "run.history"})
	if err != nil {
		t.Fatal(err)
	}
	run, _, err := h.Store.CreateRunbookRun(ctx, rb.ID, user, []*store.RunbookRunStepRecord{
		{Kind: "manual", Title: "Confirm"},
		{Kind: "lifecycle", Title: "Visible", ConnectorID: visible, Verb: "restart"},
		{Kind: "wait_until_healthy", Title: "Secret title", ConnectorID: hidden, EntityRef: "secret-entity", TimeoutSeconds: 300},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Store.DB().ExecContext(ctx, "UPDATE runbook_run_steps SET error = ? WHERE run_id = ? AND position = 2", "secret failure", run.ID); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"mixed", "restricted", "viewer"} {
		for _, tool := range []string{"get_runbook_run", "list_runbook_runs"} {
			t.Run(mode+"/"+tool, func(t *testing.T) {
				caller := userCtx(mixed)
				if mode == "restricted" {
					caller = restrictedCtx(user, []string{visible})
				}
				if mode == "viewer" {
					caller = userCtx(user)
				}
				args := map[string]any{"id": run.ID}
				if tool == "list_runbook_runs" {
					args = map[string]any{"runbookId": rb.ID}
				}
				var out map[string]any
				h.callTool(caller, t, tool, args, &out)
				if tool == "list_runbook_runs" {
					if out["total"] != float64(1) {
						t.Fatalf("list=%+v", out)
					}
					out = out["runs"].([]any)[0].(map[string]any)
				}
				steps := out["steps"].([]any)
				secret := steps[2].(map[string]any)
				if mode == "viewer" {
					if secret["title"] != "Secret title" || secret["error"] != "secret failure" {
						t.Fatalf("viewer=%+v", secret)
					}
					return
				}
				if secret["redacted"] != true {
					t.Fatalf("hidden=%+v", secret)
				}
				for _, key := range []string{"kind", "title", "connectorId", "connectorName", "entityRef", "verb", "timeoutSeconds", "error", "preview"} {
					if _, ok := secret[key]; ok {
						t.Fatalf("hidden field %s: %+v", key, secret)
					}
				}
				if steps[0].(map[string]any)["kind"] != "manual" || steps[1].(map[string]any)["connectorId"] != visible {
					t.Fatalf("visible steps=%+v", steps)
				}
			})
		}
	}
	tools, err := h.client.ListTools(userCtx(user), mcpsdk.ListToolsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range tools.Tools {
		names[tool.Name] = true
		if strings.Contains(tool.Name, "run") && (strings.Contains(tool.Name, "start") || strings.Contains(tool.Name, "resume") || strings.Contains(tool.Name, "confirm") || strings.Contains(tool.Name, "cancel") || strings.Contains(tool.Name, "execute")) {
			t.Fatalf("mutating run tool registered: %s", tool.Name)
		}
	}
	if !names["list_runbook_runs"] || !names["get_runbook_run"] {
		t.Fatalf("tools=%+v", names)
	}
	if err := h.Store.DeleteRunbook(ctx, rb.ID); err != nil {
		t.Fatal(err)
	}
	var deleted map[string]any
	h.callTool(userCtx(user), t, "get_runbook_run", map[string]any{"id": run.ID}, &deleted)
	if deleted["runbookTitle"] != "Run history" || len(deleted["steps"].([]any)) != 3 {
		t.Fatalf("deleted run=%+v", deleted)
	}
	for _, tool := range []string{"get_runbook_run", "list_runbook_runs"} {
		args := map[string]any{"id": "missing"}
		if tool == "list_runbook_runs" {
			args = map[string]any{"runbookId": "missing"}
		}
		res, err := h.client.CallTool(userCtx(user), mcpsdk.CallToolRequest{Params: mcpsdk.CallToolParams{Name: tool, Arguments: args}})
		if err != nil || !res.IsError {
			t.Fatalf("missing %s res=%+v err=%v", tool, res, err)
		}
	}
}
