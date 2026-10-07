package runbooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/runbookrun"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Hold background work at the executor seam: HTTP tests assert durable command
// results without racing the next automated step (covered by executor tests).
type runSpawner struct{ accept bool }

func (s runSpawner) TryGo(func(context.Context)) bool { return s.accept }

func runFixture(t *testing.T) (*Handler, string, string, string, string) {
	t.Helper()
	h := newTestHandler(t)
	if _, err := h.Store.DB().ExecContext(context.Background(), "PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}
	a := seedProxmoxConnector(t, h)
	if err := h.Store.CreateSnapshot(context.Background(), &store.SnapshotRecord{ConnectorID: a, Data: `{"serviceName":"Proxmox","entities":[{"kind":"vm","name":"Guest","externalId":"vm:100"}],"dependencies":[]}`, FetchedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	bRec := &store.ConnectorRecord{Name: "Hidden storage", Type: "proxmox", Category: "virtualization", URL: "https://hidden.test"}
	if err := h.Store.CreateConnector(context.Background(), bRec); err != nil {
		t.Fatal(err)
	}
	user := operatorOn(t, h, a)
	apitest.GrantConnectorRole(t, h.Store, user, bRec.ID, "operator")
	rb, _, err := h.Store.CreateRunbookWithSteps(context.Background(), &store.RunbookRecord{Title: "Restore", TargetType: "change_type", TargetValue: "test.run"}, []*store.RunbookStepRecord{
		{Kind: "manual", Title: "Verify backup"},
		{Kind: "lifecycle", Title: "Restart visible", ConnectorID: a, Verb: "restart", EntityRef: "vm:100"},
		{Kind: "sync_and_wait", Title: "Secret restore", ConnectorID: bRec.ID, TimeoutSeconds: 300},
	})
	if err != nil {
		t.Fatal(err)
	}
	h.Executor = runbookrun.New(runbookrun.Deps{Store: h.Store, Spawner: runSpawner{true}})
	return h, rb.ID, user, a, bRec.ID
}

func runRequest(user, id, runID, stepID string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.SetPathValue("id", id)
	r.SetPathValue("runId", runID)
	r.SetPathValue("stepId", stepID)
	return r.WithContext(auth.ContextWithUser(r.Context(), user, false))
}

func elevateRun(t *testing.T, h *Handler, r *http.Request, target string) {
	t.Helper()
	tok, err := h.ConnH.JWT.IssueElevationBound(auth.UserIDFromContext(r.Context()), "runbook.run", auth.ElevationBinding{Target: target})
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("X-Elevation-Token", tok.Token)
}

func assertRunStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rr.Code != want {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, want, rr.Body.String())
	}
}

func TestStartRunAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode string
		want int
	}{
		{"missing elevation", "", 400}, {"wrong runbook token", "wrong", 401}, {"other action token", "action", 401},
		{"missing grant precedes elevation", "grant", 403}, {"restricted key", "restricted", 403},
		{"read-only key", "read", 403}, {"valid elevation", "valid", 202},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, id, user, a, b := runFixture(t)
			r := runRequest(user, id, "", "")
			switch tc.mode {
			case "wrong":
				elevateRun(t, h, r, "another-runbook")
			case "action":
				tok, err := h.ConnH.JWT.IssueElevationBound(user, "connector.restart", auth.ElevationBinding{Target: id})
				if err != nil {
					t.Fatal(err)
				}
				r.Header.Set("X-Elevation-Token", tok.Token)
			case "grant":
				if _, err := h.Store.UpsertConnectorGrant(r.Context(), user, b, "viewer"); err != nil {
					t.Fatal(err)
				}
			case "restricted":
				r = r.WithContext(auth.ContextWithAPIKeyRestriction(r.Context(), auth.APIKeyRestriction{ConnectorIDs: []string{a}}))
			case "read":
				r = r.WithContext(auth.ContextWithAPIKeyRestriction(r.Context(), auth.APIKeyRestriction{ReadOnly: true}))
			case "valid":
				elevateRun(t, h, r, id)
			}
			rr := httptest.NewRecorder()
			h.StartRun(rr, r)
			assertRunStatus(t, rr, tc.want)
			runs, total, err := h.Store.ListRunbookRuns(r.Context(), id, 20, 0)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want != 202 {
				if total != 0 {
					t.Fatal("rejected request created run")
				}
				assertNoRunAudit(t, h, "runbook.run.start")
				return
			}
			if total != 1 || runs[0].StartedBy != user {
				t.Fatalf("runs=%+v", runs)
			}
			assertRunAudit(t, h, "runbook.run.start", runs[0].ID, id, user)
			second := runRequest(user, id, "", "")
			elevateRun(t, h, second, id)
			secondRR := httptest.NewRecorder()
			h.StartRun(secondRR, second)
			assertRunStatus(t, secondRR, 409)
			if !strings.Contains(secondRR.Body.String(), runs[0].ID) {
				t.Fatal("conflict did not identify active run")
			}
			// A successful start spends the elevation exactly once.
			if _, err := h.ConnH.JWT.ConsumeElevation(r.Header.Get("X-Elevation-Token"), "runbook.run", user, auth.ElevationBinding{Target: id}); err == nil {
				t.Fatal("start did not consume elevation")
			}
		})
	}
}

func seededRun(t *testing.T, h *Handler, id, user string, waiting bool) (*store.RunbookRunRecord, []*store.RunbookRunStepRecord) {
	t.Helper()
	authored, err := h.Store.ListRunbookStepsFor(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	run, steps, err := h.Store.CreateRunbookRun(context.Background(), id, user, runbookrun.FreezeSteps(authored))
	if err != nil {
		t.Fatal(err)
	}
	if waiting {

		if _, _, err := h.Store.PauseRunbookRunOnManualStep(context.Background(), run.ID, steps[0].ID); err != nil {
			t.Fatal(err)
		}
	} else {
		if _, err := h.Store.UpdateRunbookRun(context.Background(), run.ID, "running", map[string]any{"state": "failed", "reason": "step_failed"}); err != nil {
			t.Fatal(err)
		}
	}
	return run, steps
}

func assertNoRunAudit(t *testing.T, h *Handler, action string) {
	t.Helper()
	records, total, err := h.Store.ListAuditRecords(context.Background(), action, "runbook_run", "", "", 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("rejected request wrote audit: %+v", records)
	}
}

func createManualRunbook(t *testing.T, h *Handler, value string) string {
	t.Helper()
	rb, _, err := h.Store.CreateRunbookWithSteps(context.Background(), &store.RunbookRecord{Title: "Manual " + value, TargetType: "change_type", TargetValue: value}, []*store.RunbookStepRecord{{Kind: "manual", Title: "Check " + value}})
	if err != nil {
		t.Fatal(err)
	}
	return rb.ID
}

// manualRun seeds a manual-only runbook with one run paused on its first step.
func manualRun(t *testing.T, h *Handler, user, value string) (string, *store.RunbookRunRecord, []*store.RunbookRunStepRecord) {
	t.Helper()
	runbookID := createManualRunbook(t, h, value)
	run, steps := seededRun(t, h, runbookID, user, true)
	return runbookID, run, steps
}

func connectorlessActor(t *testing.T, h *Handler, mode string) (string, bool, *auth.APIKeyRestriction) {
	t.Helper()
	role := "viewer"
	if mode == "admin" {
		role = "operator"
	}
	user := apitest.NewUser(t, h.Store, role)
	admin := mode == "admin"
	switch mode {
	case "viewer", "operator":
		connectorID := seedProxmoxConnector(t, h)
		grantRole := mode
		apitest.GrantConnectorRole(t, h.Store, user, connectorID, grantRole)
	case "restricted_operator":
		grantedConnector := seedProxmoxConnector(t, h)
		apitest.GrantConnectorRole(t, h.Store, user, grantedConnector, "operator")
		restrictedTo := seedProxmoxConnector(t, h)
		return user, admin, &auth.APIKeyRestriction{ConnectorIDs: []string{restrictedTo}}
	}
	return user, admin, nil
}

func withConnectorlessActor(r *http.Request, user string, admin bool, restriction *auth.APIKeyRestriction) *http.Request {
	r = r.WithContext(auth.ContextWithUser(r.Context(), user, admin))
	if restriction != nil {
		r = r.WithContext(auth.ContextWithAPIKeyRestriction(r.Context(), *restriction))
	}
	return r
}

func assertRunAudit(t *testing.T, h *Handler, action, runID, rbID, user string) {
	t.Helper()
	records, total, err := h.Store.ListAuditRecords(context.Background(), action, "runbook_run", "", "", 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || records[0].ActorUserID != user || records[0].TargetID != runID {
		t.Fatalf("audit=%+v", records)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(records[0].Detail), &detail); err != nil {
		t.Fatal(err)
	}
	if detail["runId"] != runID || detail["runbookId"] != rbID {
		t.Fatalf("detail=%+v", detail)
	}
}

func TestRunActionsAuthorizationAndAudit(t *testing.T) {
	for _, action := range []string{"confirm", "resume", "cancel"} {
		for _, mode := range []string{"valid", "missing_grant", "restricted", "wrong_state", "unknown", "shutdown"} {
			t.Run(action+"/"+mode, func(t *testing.T) {
				h, id, user, a, b := runFixture(t)
				run, steps := seededRun(t, h, id, user, action == "confirm")
				r := runRequest(user, id, run.ID, steps[0].ID)
				want := 204
				if action == "resume" {
					want = 202
					elevateRun(t, h, r, id)
				}
				switch mode {
				case "missing_grant":
					if _, err := h.Store.UpsertConnectorGrant(r.Context(), user, b, "viewer"); err != nil {
						t.Fatal(err)
					}
					// State/step checks must not win over the frozen connector grant check.
					r.SetPathValue("stepId", "missing")
					if _, err := h.Store.UpdateRunbookRun(r.Context(), run.ID, map[bool]string{true: "waiting_manual", false: "failed"}[action == "confirm"], map[string]any{"state": "succeeded"}); err != nil {
						t.Fatal(err)
					}
					want = 403
				case "restricted":
					r = r.WithContext(auth.ContextWithAPIKeyRestriction(r.Context(), auth.APIKeyRestriction{ConnectorIDs: []string{a}}))
					want = 403
				case "wrong_state":
					state := "failed"
					if action == "confirm" {
						state = "waiting_manual"
					}
					if _, err := h.Store.UpdateRunbookRun(r.Context(), run.ID, state, map[string]any{"state": "succeeded"}); err != nil {
						t.Fatal(err)
					}
					want = 409
				case "unknown":
					r.SetPathValue("runId", "missing")
					want = 404
				case "shutdown":
					h.Executor = runbookrun.New(runbookrun.Deps{Store: h.Store, Spawner: runSpawner{false}})
					if action != "cancel" {
						want = 503
					}
				}
				rr := httptest.NewRecorder()
				switch action {
				case "confirm":
					h.ConfirmRunStep(rr, r)
				case "resume":
					h.ResumeRun(rr, r)
				case "cancel":
					h.CancelRun(rr, r)
				}
				assertRunStatus(t, rr, want)
				switch mode {
				case "valid", "shutdown":
					assertRunAudit(t, h, "runbook.run."+action, run.ID, id, user)
				case "missing_grant", "restricted", "wrong_state", "unknown":
					assertNoRunAudit(t, h, "runbook.run."+action)
				}
				if mode == "missing_grant" {
					for _, leaked := range []string{b, "Secret restore", "sync_and_wait"} {
						if strings.Contains(rr.Body.String(), leaked) {
							t.Fatalf("403 body leaked %q: %s", leaked, rr.Body.String())
						}
					}
				}
				if action == "resume" && mode == "valid" {
					var resumed store.RunbookRunRecord
					if err := json.Unmarshal(rr.Body.Bytes(), &resumed); err != nil {
						t.Fatal(err)
					}
					if resumed.State != "running" || resumed.ResumedBy == nil || *resumed.ResumedBy != user {
						t.Fatalf("resumed=%+v", resumed)
					}
				}
			})
		}
	}
	t.Run("resume missing elevation", func(t *testing.T) {
		h, id, user, _, _ := runFixture(t)
		run, _ := seededRun(t, h, id, user, false)
		rr := httptest.NewRecorder()
		h.ResumeRun(rr, runRequest(user, id, run.ID, ""))
		assertRunStatus(t, rr, 400)
	})
}

func TestStartRunShutdownAndEmpty(t *testing.T) {
	h, id, user, _, _ := runFixture(t)
	h.Executor = runbookrun.New(runbookrun.Deps{Store: h.Store, Spawner: runSpawner{false}})
	r := runRequest(user, id, "", "")
	elevateRun(t, h, r, id)
	rr := httptest.NewRecorder()
	h.StartRun(rr, r)
	assertRunStatus(t, rr, 503)
	runs, _, err := h.Store.ListRunbookRuns(r.Context(), id, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	assertRunAudit(t, h, "runbook.run.start", runs[0].ID, id, user)
	if len(runs) != 1 || runs[0].Reason != "interrupted" {
		t.Fatalf("runs=%+v", runs)
	}
	empty, err := h.Store.CreateRunbook(r.Context(), &store.RunbookRecord{Title: "Empty", TargetType: "change_type", TargetValue: "empty"})
	if err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	h.StartRun(rr, runRequest(user, empty.ID, "", ""))
	assertRunStatus(t, rr, 400)
}

func TestRunVisibilityAndPreview(t *testing.T) {
	h, id, user, a, b := runFixture(t)
	run, steps := seededRun(t, h, id, user, false)
	if _, err := h.Store.DB().ExecContext(context.Background(), "UPDATE runbook_run_steps SET error = ? WHERE id = ?", "secret target failed", steps[2].ID); err != nil {
		t.Fatal(err)
	}
	stranger := apitest.NewUser(t, h.Store, "viewer")
	apitest.GrantConnectorRole(t, h.Store, stranger, a, "viewer")
	for _, mode := range []string{"mixed_grants", "restricted_key"} {
		for _, endpoint := range []string{"get", "list", "preview"} {
			t.Run(mode+"/"+endpoint, func(t *testing.T) {
				r := runRequest(stranger, id, run.ID, "")
				if mode == "restricted_key" {
					r = runRequest(user, id, run.ID, "")
					r = r.WithContext(auth.ContextWithAPIKeyRestriction(r.Context(), auth.APIKeyRestriction{ConnectorIDs: []string{a}}))
				}
				rr := httptest.NewRecorder()
				switch endpoint {
				case "get":
					h.GetRun(rr, r)
				case "list":
					h.ListRuns(rr, r)
				case "preview":
					r.URL.RawQuery = "dryRun=true"
					h.StartRun(rr, r)
				}
				assertRunStatus(t, rr, 200)
				var root map[string]any
				if err := json.Unmarshal(rr.Body.Bytes(), &root); err != nil {
					t.Fatal(err)
				}
				if endpoint == "list" {
					root = root["items"].([]any)[0].(map[string]any)
				}
				hidden := root["steps"].([]any)[2].(map[string]any)
				if hidden["redacted"] != true {
					t.Fatalf("hidden=%+v", hidden)
				}
				for _, key := range []string{"kind", "title", "connectorId", "connectorName", "verb", "entityRef", "error", "preview", "timeoutSeconds"} {
					if _, ok := hidden[key]; ok {
						t.Fatalf("hidden field %s: %+v", key, hidden)
					}
				}
				if strings.Contains(rr.Body.String(), b) || strings.Contains(rr.Body.String(), "secret target") {
					t.Fatal("response leaked hidden connector")
				}
				manual := root["steps"].([]any)[0].(map[string]any)
				if manual["kind"] != "manual" || manual["redacted"] != false {
					t.Fatalf("manual=%+v", manual)
				}
				if endpoint == "preview" && root["canStart"] != false {
					t.Fatal("blocked preview can start")
				}
				if endpoint == "preview" && mode == "mixed_grants" {
					lifecycle := root["steps"].([]any)[1].(map[string]any)
					if lifecycle["redacted"] != false || lifecycle["canExecute"] != false || lifecycle["executeBlockedReason"] != "no_operator_grant" {
						t.Fatalf("viewer lifecycle step=%+v", lifecycle)
					}
					if _, ok := lifecycle["preview"]; ok {
						t.Fatalf("viewer got lifecycle preview: %+v", lifecycle)
					}
				}
			})
		}
	}
	t.Run("all grants lifecycle preview no elevation or mutation", func(t *testing.T) {
		r := runRequest(user, id, "", "")
		r.URL.RawQuery = "dryRun=true"
		rr := httptest.NewRecorder()
		h.StartRun(rr, r)
		assertRunStatus(t, rr, 200)
		var preview runPreviewResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		direct, err := h.ConnH.PreviewLifecycleOp(r.Context(), a, "restart", "vm:100")
		if err != nil {
			t.Fatal(err)
		}
		want, _ := json.Marshal(direct)
		got, _ := json.Marshal(preview.Steps[1].Preview)
		if !preview.CanStart || string(want) != string(got) {
			t.Fatalf("preview=%+v got=%s want=%s", preview, got, want)
		}
		_, total, err := h.Store.ListRunbookRuns(r.Context(), id, 20, 0)
		if err != nil || total != 1 {
			t.Fatalf("preview created run: total=%d err=%v", total, err)
		}
	})
}

func TestDeletedRunbookRunActions(t *testing.T) {
	for _, action := range []string{"get", "resume", "confirm", "cancel"} {
		t.Run(action, func(t *testing.T) {
			h, id, user, _, _ := runFixture(t)
			run, steps := seededRun(t, h, id, user, action == "confirm")
			if err := h.Store.DeleteRunbook(context.Background(), id); err != nil {
				t.Fatal(err)
			}
			r := runRequest(user, id, run.ID, steps[0].ID)
			rr := httptest.NewRecorder()
			want := 204
			switch action {
			case "get":
				h.GetRun(rr, r)
				want = 200
			case "resume":
				h.ResumeRun(rr, r)
				want = 409
			case "confirm":
				h.ConfirmRunStep(rr, r)
			case "cancel":
				h.CancelRun(rr, r)
			}
			assertRunStatus(t, rr, want)
			if action == "resume" {
				var body struct {
					Code string `json:"code"`
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body.Code != "runbook_deleted" {
					t.Fatalf("resume body=%s err=%v", rr.Body.String(), err)
				}
			}
		})
	}
}

func TestConfirmOtherOperatorAndPendingStep(t *testing.T) {
	h, id, starter, a, b := runFixture(t)
	run, steps := seededRun(t, h, id, starter, true)
	other := operatorOn(t, h, a)
	apitest.GrantConnectorRole(t, h.Store, other, b, "operator")
	rr := httptest.NewRecorder()
	h.ConfirmRunStep(rr, runRequest(other, id, run.ID, steps[1].ID))
	assertRunStatus(t, rr, 409)
	rr = httptest.NewRecorder()
	h.ConfirmRunStep(rr, runRequest(other, id, run.ID, steps[0].ID))
	assertRunStatus(t, rr, 204)
	_, saved, err := h.Store.GetRunbookRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved[0].ConfirmedBy != other || saved[0].State != "succeeded" {
		t.Fatalf("confirmed=%+v", saved[0])
	}
	assertRunAudit(t, h, "runbook.run.confirm", run.ID, id, other)
}

func TestRunHistoryPaginationAndResumeTarget(t *testing.T) {
	h, id, user, _, _ := runFixture(t)
	older, _ := seededRun(t, h, id, user, false)
	if err := h.Executor.Cancel(context.Background(), older.ID, user); err != nil {
		t.Fatal(err)
	}
	newer, _ := seededRun(t, h, id, user, false)
	r := runRequest(user, id, newer.ID, "")
	r.URL.RawQuery = "page=2&pageSize=1"
	rr := httptest.NewRecorder()
	h.ListRuns(rr, r)
	assertRunStatus(t, rr, 200)
	var page struct {
		Items []RunResponse `json:"items"`
		Total int           `json:"total"`
		Page  int           `json:"page"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || page.Page != 2 || len(page.Items) != 1 || page.Items[0].ID != older.ID {
		t.Fatalf("page=%+v", page)
	}
	r = runRequest(user, id, newer.ID, "")
	elevateRun(t, h, r, "another-runbook")
	rr = httptest.NewRecorder()
	h.ResumeRun(rr, r)
	assertRunStatus(t, rr, 401)
	current, _, err := h.Store.GetRunbookRun(context.Background(), newer.ID)
	if err != nil || current.State != "failed" {
		t.Fatalf("run=%+v err=%v", current, err)
	}
}

func TestRunPreviewUnavailableHidesCause(t *testing.T) {
	h, id, user, a, _ := runFixture(t)
	if _, err := h.Store.DB().ExecContext(context.Background(), "DELETE FROM service_snapshots WHERE connector_id = ?", a); err != nil {
		t.Fatal(err)
	}
	r := runRequest(user, id, "", "")
	r.URL.RawQuery = "dryRun=true"
	rr := httptest.NewRecorder()
	h.StartRun(rr, r)
	assertRunStatus(t, rr, 200)
	var preview runPreviewResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	step := preview.Steps[1]
	if preview.CanStart || step.CanExecute || step.ExecuteBlockedReason != "preview_unavailable" || step.Preview != nil {
		t.Fatalf("preview=%+v", preview)
	}
	if strings.Contains(rr.Body.String(), "snapshot") {
		t.Fatalf("preview leaked internal error: %s", rr.Body.String())
	}
}

func TestReadOnlyKeyCannotMutateManualRun(t *testing.T) {
	for _, action := range []string{"confirm", "cancel"} {
		t.Run(action, func(t *testing.T) {
			h, _, user, _, _ := runFixture(t)
			rbID, run, steps := manualRun(t, h, user, "manual.readonly")
			r := runRequest(user, rbID, run.ID, steps[0].ID)
			r = r.WithContext(auth.ContextWithAPIKeyRestriction(r.Context(), auth.APIKeyRestriction{ReadOnly: true}))
			rr := httptest.NewRecorder()
			if action == "confirm" {
				h.ConfirmRunStep(rr, r)
			} else {
				h.CancelRun(rr, r)
			}
			assertRunStatus(t, rr, 403)
			current, _, err := h.Store.GetRunbookRun(context.Background(), run.ID)
			if err != nil || current.State != "waiting_manual" {
				t.Fatalf("run=%+v err=%v", current, err)
			}
			assertNoRunAudit(t, h, "runbook.run."+action)
		})
	}
}

func TestConfirmRejectsStepOfAnotherRun(t *testing.T) {
	h, _, user, _, _ := runFixture(t)
	idA, runA, _ := manualRun(t, h, user, "manual.idor.a")
	_, runB, stepsB := manualRun(t, h, user, "manual.idor.b")
	rr := httptest.NewRecorder()
	h.ConfirmRunStep(rr, runRequest(user, idA, runA.ID, stepsB[0].ID))
	assertRunStatus(t, rr, 409)
	_, saved, err := h.Store.GetRunbookRun(context.Background(), runB.ID)
	if err != nil || saved[0].State != "waiting" {
		t.Fatalf("other run step=%+v err=%v", saved[0], err)
	}
	assertNoRunAudit(t, h, "runbook.run.confirm")
}

func deleteRunConnectors(t *testing.T, h *Handler, ids ...string) {
	t.Helper()
	for _, id := range ids {
		if err := h.Store.DeleteConnector(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
}

func assertRunState(t *testing.T, h *Handler, runID, want string) {
	t.Helper()
	run, _, err := h.Store.GetRunbookRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	if run.State != want {
		t.Fatalf("state=%s want=%s", run.State, want)
	}
}

// assertDeletedStepRedacted checks history still hides a step whose connector is gone.
func assertDeletedStepRedacted(t *testing.T, h *Handler, id, user, runID string) {
	t.Helper()
	rr := httptest.NewRecorder()
	h.GetRun(rr, runRequest(user, id, runID, ""))
	assertRunStatus(t, rr, 200)
	var view struct {
		Steps []struct {
			Redacted bool `json:"redacted"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Steps) != 3 || !view.Steps[2].Redacted {
		t.Fatalf("deleted connector step not redacted: %s", rr.Body.String())
	}
}

func TestCancelRunAfterConnectorDeleted(t *testing.T) {
	t.Run("surviving grant cancels", func(t *testing.T) {
		h, id, user, _, b := runFixture(t)
		run, _ := seededRun(t, h, id, user, false)
		deleteRunConnectors(t, h, b)
		rr := httptest.NewRecorder()
		h.CancelRun(rr, runRequest(user, id, run.ID, ""))
		assertRunStatus(t, rr, 204)
		assertRunState(t, h, run.ID, "cancelled")
		assertRunAudit(t, h, "runbook.run.cancel", run.ID, id, user)
		assertDeletedStepRedacted(t, h, id, user, run.ID)
	})
	t.Run("missing grant on surviving connector", func(t *testing.T) {
		h, id, user, a, b := runFixture(t)
		run, _ := seededRun(t, h, id, user, false)
		deleteRunConnectors(t, h, b)
		if _, err := h.Store.UpsertConnectorGrant(context.Background(), user, a, "viewer"); err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		h.CancelRun(rr, runRequest(user, id, run.ID, ""))
		assertRunStatus(t, rr, 403)
		assertRunState(t, h, run.ID, "failed")
		assertNoRunAudit(t, h, "runbook.run.cancel")
	})
	t.Run("viewer on surviving connector with operator elsewhere", func(t *testing.T) {
		h, id, user, a, b := runFixture(t)
		run, _ := seededRun(t, h, id, user, false)
		deleteRunConnectors(t, h, b)
		if _, err := h.Store.UpsertConnectorGrant(context.Background(), user, a, "viewer"); err != nil {
			t.Fatal(err)
		}
		apitest.GrantConnectorRole(t, h.Store, user, seedProxmoxConnector(t, h), "operator")
		rr := httptest.NewRecorder()
		h.CancelRun(rr, runRequest(user, id, run.ID, ""))
		assertRunStatus(t, rr, 403)
		assertRunState(t, h, run.ID, "failed")
		assertNoRunAudit(t, h, "runbook.run.cancel")
	})
	for _, mode := range []string{"none", "viewer", "operator", "admin"} {
		t.Run("all connectors deleted/"+mode, func(t *testing.T) {
			h, id, starter, a, b := runFixture(t)
			run, _ := seededRun(t, h, id, starter, false)
			deleteRunConnectors(t, h, a, b)
			user, admin, restriction := connectorlessActor(t, h, mode)
			r := withConnectorlessActor(runRequest(user, id, run.ID, ""), user, admin, restriction)
			rr := httptest.NewRecorder()
			h.CancelRun(rr, r)
			allowed := mode == "operator" || mode == "admin"
			want := http.StatusForbidden
			if allowed {
				want = http.StatusNoContent
			}
			assertRunStatus(t, rr, want)
			if !allowed {
				assertRunState(t, h, run.ID, "failed")
				assertNoRunAudit(t, h, "runbook.run.cancel")
				return
			}
			assertRunState(t, h, run.ID, "cancelled")
			assertRunAudit(t, h, "runbook.run.cancel", run.ID, id, user)
		})
	}
}

func TestConfirmResumeStillRequireDeletedConnectorGrant(t *testing.T) {
	t.Run("confirm", func(t *testing.T) {
		h, id, user, _, b := runFixture(t)
		run, steps := seededRun(t, h, id, user, true)
		deleteRunConnectors(t, h, b)
		rr := httptest.NewRecorder()
		h.ConfirmRunStep(rr, runRequest(user, id, run.ID, steps[0].ID))
		assertRunStatus(t, rr, 403)
		assertRunState(t, h, run.ID, "waiting_manual")
		assertNoRunAudit(t, h, "runbook.run.confirm")
	})
	t.Run("resume", func(t *testing.T) {
		h, id, user, _, b := runFixture(t)
		run, _ := seededRun(t, h, id, user, false)
		deleteRunConnectors(t, h, b)
		r := runRequest(user, id, run.ID, "")
		elevateRun(t, h, r, id)
		rr := httptest.NewRecorder()
		h.ResumeRun(rr, r)
		assertRunStatus(t, rr, 403)
		assertRunState(t, h, run.ID, "failed")
		assertNoRunAudit(t, h, "runbook.run.resume")
	})
}

func TestConnectorlessRunActionsAuthorization(t *testing.T) {
	for _, action := range []string{"start", "confirm", "resume", "cancel"} {
		for _, mode := range []string{"none", "viewer", "operator", "admin", "restricted_operator"} {
			t.Run(action+"/"+mode, func(t *testing.T) {
				h, _, starter, _, _ := runFixture(t)
				runbookID := createManualRunbook(t, h, action+"."+mode)
				var run *store.RunbookRunRecord
				var steps []*store.RunbookRunStepRecord
				if action != "start" {
					run, steps = seededRun(t, h, runbookID, starter, action != "resume")
				}
				user, admin, restriction := connectorlessActor(t, h, mode)
				r := runRequest(user, runbookID, "", "")
				if run != nil {
					r.SetPathValue("runId", run.ID)
					r.SetPathValue("stepId", steps[0].ID)
				}
				r = withConnectorlessActor(r, user, admin, restriction)
				allowed := mode == "operator" || mode == "admin"
				if allowed && (action == "start" || action == "resume") {
					elevateRun(t, h, r, runbookID)
				}
				rr := httptest.NewRecorder()
				switch action {
				case "start":
					h.StartRun(rr, r)
				case "confirm":
					h.ConfirmRunStep(rr, r)
				case "resume":
					h.ResumeRun(rr, r)
				case "cancel":
					h.CancelRun(rr, r)
				}
				want := http.StatusForbidden
				if allowed {
					want = http.StatusNoContent
					if action == "start" || action == "resume" {
						want = http.StatusAccepted
					}
				}
				assertRunStatus(t, rr, want)
				if !allowed {
					assertNoRunAudit(t, h, "runbook.run."+action)
					return
				}
				runID := ""
				if run != nil {
					runID = run.ID
				} else {
					runs, total, err := h.Store.ListRunbookRuns(context.Background(), runbookID, 20, 0)
					if err != nil || total != 1 {
						t.Fatalf("runs=%+v total=%d err=%v", runs, total, err)
					}
					runID = runs[0].ID
				}
				assertRunAudit(t, h, "runbook.run."+action, runID, runbookID, user)
			})
		}
	}
}

func TestConnectorlessRunViewActionPermission(t *testing.T) {
	for _, mode := range []string{"none", "viewer", "operator", "admin", "restricted_operator"} {
		t.Run(mode, func(t *testing.T) {
			h, runbookID, starter, _, _ := runFixture(t)
			_, run, _ := manualRun(t, h, starter, "view."+mode)
			user, admin, restriction := connectorlessActor(t, h, mode)
			r := withConnectorlessActor(runRequest(user, runbookID, run.ID, ""), user, admin, restriction)
			rr := httptest.NewRecorder()
			h.GetRun(rr, r)
			assertRunStatus(t, rr, http.StatusOK)
			var view RunResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &view); err != nil {
				t.Fatal(err)
			}
			want := mode == "operator" || mode == "admin"
			if got := view.Steps[0].CanExecute; got != want {
				t.Fatalf("CanExecute=%v want=%v for %s", got, want, mode)
			}
		})
	}
}

type fakePusherReader struct {
	fakePusher
	readVal    any
	readErr    error
	blockOnCtx bool
	readDelay  time.Duration

	mu          sync.Mutex
	readCalls   int
	pushCalls   int
	inFlight    int
	maxInFlight int
}

func (f *fakePusherReader) ConfigPush(_ context.Context, _ map[string]any, _, _ string, _ any) error {
	f.mu.Lock()
	f.pushCalls++
	f.mu.Unlock()
	return nil
}

func (f *fakePusherReader) ConfigRead(ctx context.Context, _ map[string]any, _, _ string) (any, error) {
	f.mu.Lock()
	f.readCalls++
	f.inFlight++
	if f.inFlight > f.maxInFlight {
		f.maxInFlight = f.inFlight
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.inFlight--
		f.mu.Unlock()
	}()

	if f.blockOnCtx {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if f.readDelay > 0 {
		select {
		case <-time.After(f.readDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return f.readVal, f.readErr
}

func seedFakePusherReader(t *testing.T, h *Handler, fields []connector.ConfigField, readVal any, readErr error) (string, *fakePusherReader) {
	t.Helper()
	typ := fmt.Sprintf("fake_pusher_reader_%d", time.Now().UnixNano())
	fake := &fakePusherReader{
		fakePusher: fakePusher{typ: typ, fields: fields},
		readVal:    readVal,
		readErr:    readErr,
	}
	connector.Register(connector.TypeSchema{Type: typ, Name: "Fake Pusher Reader", Category: "virtualization"},
		func(map[string]any) (connector.Connector, error) {
			return fake, nil
		})
	c := &store.ConnectorRecord{Name: "Fake Reader Connector", Category: "virtualization", Type: typ, URL: "https://example.com"}
	if err := h.Store.CreateConnector(context.Background(), c); err != nil {
		t.Fatalf("CreateConnector error: %v", err)
	}
	return c.ID, fake
}

func TestRunPreviewScenarios(t *testing.T) {
	ctx := context.Background()
	h := newTestHandler(t)
	user := apitest.NewUser(t, h.Store, "operator")

	t.Run("known value shows current and target without writing", func(t *testing.T) {
		fields := []connector.ConfigField{{Key: "memory", Type: "number", EntityScope: true}}
		connID, fake := seedFakePusherReader(t, h, fields, float64(4096), nil)
		apitest.GrantConnectorRole(t, h.Store, user, connID, "operator")

		rb, _, err := h.Store.CreateRunbookWithSteps(ctx,
			&store.RunbookRecord{Title: "Preview Known", TargetType: "change_type", TargetValue: "preview.known"},
			[]*store.RunbookStepRecord{
				{Kind: "config_push", Title: "Push Mem", ConnectorID: connID, EntityRef: "vm:100", FieldKey: "memory", TargetValue: "4096"},
			})
		if err != nil {
			t.Fatal(err)
		}

		r := runRequest(user, rb.ID, "", "")
		r.URL.RawQuery = "dryRun=true"
		rr := httptest.NewRecorder()
		h.StartRun(rr, r)
		assertRunStatus(t, rr, http.StatusOK)

		var preview runPreviewResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		if !preview.CanStart {
			t.Fatalf("preview cannot start: %+v", preview)
		}
		if len(preview.Steps) != 1 {
			t.Fatalf("steps count = %d, want 1", len(preview.Steps))
		}
		st := preview.Steps[0]
		if !st.CanExecute || st.ExecuteBlockedReason != "" {
			t.Fatalf("step canExecute=%v reason=%s", st.CanExecute, st.ExecuteBlockedReason)
		}
		if st.FieldKey != "memory" || st.TargetValue != "4096" {
			t.Fatalf("step fieldKey=%q targetValue=%q", st.FieldKey, st.TargetValue)
		}
		if st.CurrentValueKnown == nil || !*st.CurrentValueKnown {
			t.Fatalf("CurrentValueKnown = %v, want true", st.CurrentValueKnown)
		}
		if st.CurrentValue != float64(4096) {
			t.Fatalf("CurrentValue = %v, want 4096", st.CurrentValue)
		}
		if fake.readCalls != 1 || fake.pushCalls != 0 {
			t.Fatalf("readCalls=%d pushCalls=%d, want readCalls=1 pushCalls=0", fake.readCalls, fake.pushCalls)
		}
	})

	t.Run("unknown value because connector lacks reader is still startable", func(t *testing.T) {
		fields := []connector.ConfigField{{Key: "memory", Type: "number", EntityScope: true}}
		connID := seedCustomPusherConnector(t, h, fields)
		apitest.GrantConnectorRole(t, h.Store, user, connID, "operator")

		rb, _, err := h.Store.CreateRunbookWithSteps(ctx,
			&store.RunbookRecord{Title: "Preview No Reader", TargetType: "change_type", TargetValue: "preview.no_reader"},
			[]*store.RunbookStepRecord{
				{Kind: "config_push", Title: "Push Mem", ConnectorID: connID, EntityRef: "vm:100", FieldKey: "memory", TargetValue: "4096"},
			})
		if err != nil {
			t.Fatal(err)
		}

		r := runRequest(user, rb.ID, "", "")
		r.URL.RawQuery = "dryRun=true"
		rr := httptest.NewRecorder()
		h.StartRun(rr, r)
		assertRunStatus(t, rr, http.StatusOK)

		var preview runPreviewResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		if !preview.CanStart {
			t.Fatalf("preview cannot start: %+v", preview)
		}
		st := preview.Steps[0]
		if !st.CanExecute {
			t.Fatalf("step canExecute=false, want true")
		}
		if st.CurrentValueKnown == nil || *st.CurrentValueKnown {
			t.Fatalf("CurrentValueKnown = %v, want false", st.CurrentValueKnown)
		}
		if st.CurrentValue != nil {
			t.Fatalf("CurrentValue = %v, want nil", st.CurrentValue)
		}
		if st.TargetValue != "4096" {
			t.Fatalf("TargetValue = %q, want 4096", st.TargetValue)
		}
	})

	t.Run("reader error fails soft and run is still startable", func(t *testing.T) {
		fields := []connector.ConfigField{{Key: "memory", Type: "number", EntityScope: true}}
		connID, _ := seedFakePusherReader(t, h, fields, nil, errors.New("temporary upstream timeout"))
		apitest.GrantConnectorRole(t, h.Store, user, connID, "operator")

		rb, _, err := h.Store.CreateRunbookWithSteps(ctx,
			&store.RunbookRecord{Title: "Preview Reader Err", TargetType: "change_type", TargetValue: "preview.err"},
			[]*store.RunbookStepRecord{
				{Kind: "config_push", Title: "Push Mem", ConnectorID: connID, EntityRef: "vm:100", FieldKey: "memory", TargetValue: "4096"},
			})
		if err != nil {
			t.Fatal(err)
		}

		r := runRequest(user, rb.ID, "", "")
		r.URL.RawQuery = "dryRun=true"
		rr := httptest.NewRecorder()
		h.StartRun(rr, r)
		assertRunStatus(t, rr, http.StatusOK)

		var preview runPreviewResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		if !preview.CanStart {
			t.Fatalf("preview cannot start: %+v", preview)
		}
		st := preview.Steps[0]
		if !st.CanExecute {
			t.Fatalf("step canExecute=false, want true")
		}
		if st.CurrentValueKnown == nil || *st.CurrentValueKnown {
			t.Fatalf("CurrentValueKnown = %v, want false", st.CurrentValueKnown)
		}
		if st.CurrentValue != nil {
			t.Fatalf("CurrentValue = %v, want nil", st.CurrentValue)
		}
	})

	t.Run("field withdrawn marks step not executable and blocks run start", func(t *testing.T) {
		fields := []connector.ConfigField{{Key: "cores", Type: "number", EntityScope: true}}
		connID, _ := seedFakePusherReader(t, h, fields, float64(4), nil)
		apitest.GrantConnectorRole(t, h.Store, user, connID, "operator")

		rb, _, err := h.Store.CreateRunbookWithSteps(ctx,
			&store.RunbookRecord{Title: "Preview Withdrawn", TargetType: "change_type", TargetValue: "preview.withdrawn"},
			[]*store.RunbookStepRecord{
				{Kind: "config_push", Title: "Push Memory", ConnectorID: connID, EntityRef: "vm:100", FieldKey: "memory", TargetValue: "4096"},
			})
		if err != nil {
			t.Fatal(err)
		}

		r := runRequest(user, rb.ID, "", "")
		r.URL.RawQuery = "dryRun=true"
		rr := httptest.NewRecorder()
		h.StartRun(rr, r)
		assertRunStatus(t, rr, http.StatusOK)

		var preview runPreviewResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		if preview.CanStart {
			t.Fatal("preview canStart = true, want false")
		}
		st := preview.Steps[0]
		if st.CanExecute {
			t.Fatal("step canExecute = true, want false")
		}
		if st.ExecuteBlockedReason != "unsupported_field" {
			t.Fatalf("executeBlockedReason = %q, want unsupported_field", st.ExecuteBlockedReason)
		}
	})

	t.Run("wait condition is returned in preview", func(t *testing.T) {
		connID := seedProxmoxConnector(t, h)
		apitest.GrantConnectorRole(t, h.Store, user, connID, "operator")

		rb, _, err := h.Store.CreateRunbookWithSteps(ctx,
			&store.RunbookRecord{Title: "Preview Wait", TargetType: "change_type", TargetValue: "preview.wait"},
			[]*store.RunbookStepRecord{
				{
					Kind:           "wait_for_entity",
					Title:          "Wait for VM running",
					ConnectorID:    connID,
					EntityRef:      "vm:100",
					Attribute:      "status",
					Operator:       "eq",
					ExpectedValue:  `"running"`,
					TimeoutSeconds: 600,
				},
			})
		if err != nil {
			t.Fatal(err)
		}

		r := runRequest(user, rb.ID, "", "")
		r.URL.RawQuery = "dryRun=true"
		rr := httptest.NewRecorder()
		h.StartRun(rr, r)
		assertRunStatus(t, rr, http.StatusOK)

		var preview runPreviewResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		if !preview.CanStart {
			t.Fatalf("preview cannot start: %+v", preview)
		}
		st := preview.Steps[0]
		if !st.CanExecute {
			t.Fatalf("step canExecute=false, want true")
		}
		if st.Kind != "wait_for_entity" || st.EntityRef != "vm:100" || st.Attribute != "status" || st.Operator != "eq" || st.ExpectedValue != `"running"` || st.TimeoutSeconds != 600 {
			t.Fatalf("wait step condition mismatch: %+v", st)
		}
	})

	t.Run("hidden connector redaction hides all metadata and current value in preview", func(t *testing.T) {
		fields := []connector.ConfigField{{Key: "memory", Type: "number", EntityScope: true}}
		visibleID, _ := seedFakePusherReader(t, h, fields, float64(2048), nil)
		hiddenID, hiddenFake := seedFakePusherReader(t, h, fields, float64(4096), nil)

		apitest.GrantConnectorRole(t, h.Store, user, visibleID, "operator")
		// hiddenID has no grant for user

		rb, _, err := h.Store.CreateRunbookWithSteps(ctx,
			&store.RunbookRecord{Title: "Mixed Preview", TargetType: "change_type", TargetValue: "preview.mixed"},
			[]*store.RunbookStepRecord{
				{Kind: "config_push", Title: "Visible Push", ConnectorID: visibleID, EntityRef: "vm:10", FieldKey: "memory", TargetValue: "2048"},
				{Kind: "config_push", Title: "Secret Push", ConnectorID: hiddenID, EntityRef: "vm:20", FieldKey: "memory", TargetValue: "4096"},
				{Kind: "wait_for_entity", Title: "Secret Wait", ConnectorID: hiddenID, EntityRef: "vm:20", Attribute: "status", Operator: "eq", ExpectedValue: `"running"`, TimeoutSeconds: 300},
			})
		if err != nil {
			t.Fatal(err)
		}

		r := runRequest(user, rb.ID, "", "")
		r.URL.RawQuery = "dryRun=true"
		rr := httptest.NewRecorder()
		h.StartRun(rr, r)
		assertRunStatus(t, rr, http.StatusOK)

		var raw map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
			t.Fatal(err)
		}
		if raw["canStart"] != false {
			t.Fatal("canStart = true, want false when hidden steps cannot be executed")
		}

		steps := raw["steps"].([]any)
		if len(steps) != 3 {
			t.Fatalf("steps count = %d, want 3", len(steps))
		}

		// Visible step 0
		vis := steps[0].(map[string]any)
		if vis["redacted"] != false || vis["canExecute"] != true || vis["fieldKey"] != "memory" || vis["targetValue"] != "2048" || vis["currentValueKnown"] != true || vis["currentValue"] != float64(2048) {
			t.Fatalf("visible step = %+v", vis)
		}

		// Hidden push step 1
		hidPush := steps[1].(map[string]any)
		if hidPush["redacted"] != true || hidPush["executeBlockedReason"] != "no_viewer_grant" {
			t.Fatalf("hidden push step = %+v", hidPush)
		}
		for _, forbidden := range []string{"fieldKey", "targetValue", "currentValue", "currentValueKnown", "connectorId", "connectorName", "entityRef", "title", "kind"} {
			if _, exists := hidPush[forbidden]; exists {
				t.Fatalf("hidden push step leaked %s: %+v", forbidden, hidPush)
			}
		}

		// Hidden wait step 2
		hidWait := steps[2].(map[string]any)
		if hidWait["redacted"] != true || hidWait["executeBlockedReason"] != "no_viewer_grant" {
			t.Fatalf("hidden wait step = %+v", hidWait)
		}
		for _, forbidden := range []string{"attribute", "operator", "expectedValue", "timeoutSeconds", "connectorId", "connectorName", "entityRef", "title", "kind"} {
			if _, exists := hidWait[forbidden]; exists {
				t.Fatalf("hidden wait step leaked %s: %+v", forbidden, hidWait)
			}
		}
		if hiddenFake.readCalls != 0 {
			t.Fatalf("hidden connector readCalls = %d, want 0", hiddenFake.readCalls)
		}
	})

	t.Run("secret-typed field is never read in preview", func(t *testing.T) {
		fields := []connector.ConfigField{{Key: "api_token", Type: "password", EntityScope: true}}
		connID, fake := seedFakePusherReader(t, h, fields, "super-secret", nil)
		apitest.GrantConnectorRole(t, h.Store, user, connID, "operator")

		rb, _, err := h.Store.CreateRunbookWithSteps(ctx,
			&store.RunbookRecord{Title: "Preview Secret", TargetType: "change_type", TargetValue: "preview.secret"},
			[]*store.RunbookStepRecord{
				{Kind: "config_push", Title: "Push Token", ConnectorID: connID, EntityRef: "vm:100", FieldKey: "api_token", TargetValue: "new-token"},
			})
		if err != nil {
			t.Fatal(err)
		}

		r := runRequest(user, rb.ID, "", "")
		r.URL.RawQuery = "dryRun=true"
		rr := httptest.NewRecorder()
		h.StartRun(rr, r)
		assertRunStatus(t, rr, http.StatusOK)

		var preview runPreviewResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		if !preview.CanStart {
			t.Fatalf("preview cannot start: %+v", preview)
		}
		st := preview.Steps[0]
		if !st.CanExecute {
			t.Fatal("step canExecute=false, want true")
		}
		if st.CurrentValueKnown == nil || *st.CurrentValueKnown {
			t.Fatalf("CurrentValueKnown = %v, want false", st.CurrentValueKnown)
		}
		if st.CurrentValue != nil {
			t.Fatalf("CurrentValue = %v, want nil", st.CurrentValue)
		}
		if strings.Contains(rr.Body.String(), "super-secret") {
			t.Fatal("preview response leaked the secret value")
		}
		if fake.readCalls != 0 {
			t.Fatalf("readCalls = %d, want 0", fake.readCalls)
		}
	})

	t.Run("hung reader fails soft within the step timeout", func(t *testing.T) {
		fields := []connector.ConfigField{{Key: "memory", Type: "number", EntityScope: true}}
		connID, fake := seedFakePusherReader(t, h, fields, float64(4096), nil)
		fake.blockOnCtx = true
		apitest.GrantConnectorRole(t, h.Store, user, connID, "operator")

		rb, _, err := h.Store.CreateRunbookWithSteps(ctx,
			&store.RunbookRecord{Title: "Preview Hung", TargetType: "change_type", TargetValue: "preview.hung"},
			[]*store.RunbookStepRecord{
				{Kind: "config_push", Title: "Push Mem", ConnectorID: connID, EntityRef: "vm:100", FieldKey: "memory", TargetValue: "4096"},
			})
		if err != nil {
			t.Fatal(err)
		}

		r := runRequest(user, rb.ID, "", "")
		r.URL.RawQuery = "dryRun=true"
		rr := httptest.NewRecorder()
		start := time.Now()
		h.StartRun(rr, r)
		elapsed := time.Since(start)
		assertRunStatus(t, rr, http.StatusOK)
		if elapsed >= 4*time.Second {
			t.Fatalf("preview took %s, want < 4s", elapsed)
		}

		var preview runPreviewResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		if !preview.CanStart {
			t.Fatalf("preview cannot start: %+v", preview)
		}
		st := preview.Steps[0]
		if !st.CanExecute {
			t.Fatal("step canExecute=false, want true")
		}
		if st.CurrentValueKnown == nil || *st.CurrentValueKnown {
			t.Fatalf("CurrentValueKnown = %v, want false", st.CurrentValueKnown)
		}
		if st.CurrentValue != nil {
			t.Fatalf("CurrentValue = %v, want nil", st.CurrentValue)
		}
	})

	t.Run("config reads run with bounded concurrency", func(t *testing.T) {
		fields := []connector.ConfigField{{Key: "memory", Type: "number", EntityScope: true}}
		connID, fake := seedFakePusherReader(t, h, fields, float64(4096), nil)
		fake.readDelay = 50 * time.Millisecond
		apitest.GrantConnectorRole(t, h.Store, user, connID, "operator")

		const stepCount = 12
		steps := make([]*store.RunbookStepRecord, 0, stepCount)
		for i := 0; i < stepCount; i++ {
			steps = append(steps, &store.RunbookStepRecord{
				Kind: "config_push", Title: fmt.Sprintf("Push Mem %d", i), ConnectorID: connID,
				EntityRef: fmt.Sprintf("vm:%d", 100+i), FieldKey: "memory", TargetValue: "4096",
			})
		}
		rb, _, err := h.Store.CreateRunbookWithSteps(ctx,
			&store.RunbookRecord{Title: "Preview Concurrency", TargetType: "change_type", TargetValue: "preview.concurrency"},
			steps)
		if err != nil {
			t.Fatal(err)
		}

		r := runRequest(user, rb.ID, "", "")
		r.URL.RawQuery = "dryRun=true"
		rr := httptest.NewRecorder()
		h.StartRun(rr, r)
		assertRunStatus(t, rr, http.StatusOK)

		var preview runPreviewResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		if len(preview.Steps) != stepCount {
			t.Fatalf("steps count = %d, want %d", len(preview.Steps), stepCount)
		}
		for i, st := range preview.Steps {
			if st.CurrentValueKnown == nil || !*st.CurrentValueKnown {
				t.Fatalf("step %d CurrentValueKnown = %v, want true", i, st.CurrentValueKnown)
			}
		}
		fake.mu.Lock()
		maxInFlight, readCalls := fake.maxInFlight, fake.readCalls
		fake.mu.Unlock()
		if readCalls != stepCount {
			t.Fatalf("readCalls = %d, want %d", readCalls, stepCount)
		}
		if maxInFlight > previewMaxConcurrency || maxInFlight <= 1 {
			t.Fatalf("max in-flight reads = %d, want between 2 and %d", maxInFlight, previewMaxConcurrency)
		}
	})
}

func TestRunHistoryDetailAndListMixedGrants(t *testing.T) {
	ctx := context.Background()
	h := newTestHandler(t)
	if _, err := h.Store.DB().ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}

	visibleID := seedProxmoxConnector(t, h)
	hiddenID := seedProxmoxConnector(t, h)
	creator := apitest.NewUser(t, h.Store, "operator")
	viewer := apitest.NewUser(t, h.Store, "viewer")

	apitest.GrantConnectorRole(t, h.Store, creator, visibleID, "operator")
	apitest.GrantConnectorRole(t, h.Store, creator, hiddenID, "operator")
	apitest.GrantConnectorRole(t, h.Store, viewer, visibleID, "viewer")
	// viewer has no grant on hiddenID

	rb, err := h.Store.CreateRunbook(ctx, &store.RunbookRecord{
		Title: "History RB", TargetType: "change_type", TargetValue: "hist.mixed",
	})
	if err != nil {
		t.Fatal(err)
	}

	run, _, err := h.Store.CreateRunbookRun(ctx, rb.ID, creator, []*store.RunbookRunStepRecord{
		{Kind: "config_push", Title: "Push Visible", ConnectorID: visibleID, EntityRef: "vm:100", FieldKey: "memory", TargetValue: "4096"},
		{Kind: "wait_for_entity", Title: "Wait Visible", ConnectorID: visibleID, EntityRef: "vm:100", Attribute: "status", Operator: "eq", ExpectedValue: `"running"`, TimeoutSeconds: 300},
		{Kind: "config_push", Title: "Push Secret", ConnectorID: hiddenID, EntityRef: "vm:200", FieldKey: "cores", TargetValue: "8"},
		{Kind: "wait_for_entity", Title: "Wait Secret", ConnectorID: hiddenID, EntityRef: "vm:200", Attribute: "health", Operator: "eq", ExpectedValue: `"ok"`, TimeoutSeconds: 180},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, endpoint := range []string{"get", "list"} {
		t.Run(endpoint, func(t *testing.T) {
			r := runRequest(viewer, rb.ID, run.ID, "")
			rr := httptest.NewRecorder()
			if endpoint == "get" {
				h.GetRun(rr, r)
			} else {
				h.ListRuns(rr, r)
			}
			assertRunStatus(t, rr, http.StatusOK)

			var root map[string]any
			if err := json.Unmarshal(rr.Body.Bytes(), &root); err != nil {
				t.Fatal(err)
			}
			if endpoint == "list" {
				items := root["items"].([]any)
				if len(items) != 1 {
					t.Fatalf("items len = %d, want 1", len(items))
				}
				root = items[0].(map[string]any)
			}

			steps := root["steps"].([]any)
			if len(steps) != 4 {
				t.Fatalf("steps len = %d, want 4", len(steps))
			}

			// Step 0: visible config_push
			s0 := steps[0].(map[string]any)
			if s0["redacted"] != false || s0["fieldKey"] != "memory" || s0["targetValue"] != "4096" || s0["connectorId"] != visibleID {
				t.Fatalf("step 0 = %+v", s0)
			}

			// Step 1: visible wait_for_entity
			s1 := steps[1].(map[string]any)
			if s1["redacted"] != false || s1["entityRef"] != "vm:100" || s1["attribute"] != "status" || s1["operator"] != "eq" || s1["expectedValue"] != `"running"` || s1["timeoutSeconds"] != float64(300) {
				t.Fatalf("step 1 = %+v", s1)
			}

			// Step 2: hidden config_push
			s2 := steps[2].(map[string]any)
			if s2["redacted"] != true || s2["executeBlockedReason"] != "no_viewer_grant" {
				t.Fatalf("step 2 = %+v", s2)
			}
			for _, k := range []string{"fieldKey", "targetValue", "currentValue", "currentValueKnown", "connectorId", "connectorName", "entityRef", "title", "kind"} {
				if _, ok := s2[k]; ok {
					t.Fatalf("hidden step 2 leaked %s: %+v", k, s2)
				}
			}

			// Step 3: hidden wait_for_entity
			s3 := steps[3].(map[string]any)
			if s3["redacted"] != true || s3["executeBlockedReason"] != "no_viewer_grant" {
				t.Fatalf("step 3 = %+v", s3)
			}
			for _, k := range []string{"attribute", "operator", "expectedValue", "timeoutSeconds", "connectorId", "connectorName", "entityRef", "title", "kind"} {
				if _, ok := s3[k]; ok {
					t.Fatalf("hidden step 3 leaked %s: %+v", k, s3)
				}
			}
		})
	}
}
