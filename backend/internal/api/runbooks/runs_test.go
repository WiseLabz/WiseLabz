package runbooks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
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
		{"missing elevation", "", 400}, {"wrong runbook token", "wrong", 401},
		{"missing grant precedes elevation", "grant", 403}, {"restricted key", "restricted", 403},
		{"read-only key", "read", 403}, {"valid elevation", "valid", 202},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, id, user, a, b := runFixture(t)
			r := runRequest(user, id, "", "")
			switch tc.mode {
			case "wrong":
				elevateRun(t, h, r, "another-runbook")
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
				if mode == "valid" || mode == "shutdown" {
					assertRunAudit(t, h, "runbook.run."+action, run.ID, id, user)
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
