package runbooks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/runbookrun"
	"github.com/WiseLabz/wiselabz/internal/store"
)

type approvalSpawner struct{ calls int }

func (s *approvalSpawner) TryGo(func(context.Context)) bool { s.calls++; return true }

func approvalFixture(t *testing.T) (*Handler, string, string, string, *approvalSpawner) {
	t.Helper()
	h, id, initiator, a, b := runFixture(t)
	if _, err := h.Store.UpdateRunbook(context.Background(), id, map[string]any{"requires_approval": true}); err != nil {
		t.Fatal(err)
	}
	approver := operatorOn(t, h, a)
	apitest.GrantConnectorRole(t, h.Store, approver, b, "operator")
	spawner := &approvalSpawner{}
	h.Executor = runbookrun.New(runbookrun.Deps{Store: h.Store, Spawner: spawner})
	return h, id, initiator, approver, spawner
}

func requestApproval(t *testing.T, h *Handler, id, user string) RunResponse {
	t.Helper()
	r := runRequest(user, id, "", "")
	elevateRun(t, h, r, id)
	rr := httptest.NewRecorder()
	h.StartRun(rr, r)
	assertRunStatus(t, rr, 202)
	var response RunResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func elevateApproval(t *testing.T, h *Handler, r *http.Request, actor, action, target string) {
	t.Helper()
	token, err := h.ConnH.JWT.IssueElevationBound(actor, action, auth.ElevationBinding{Target: target})
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("X-Elevation-Token", token.Token)
}

func TestApprovalRequestFreezesStepsWithoutExecuting(t *testing.T) {
	h, id, initiator, approver, spawner := approvalFixture(t)
	run := requestApproval(t, h, id, initiator)
	if run.State != "awaiting_approval" || !run.RequiresApproval || run.StartedBy != initiator || spawner.calls != 0 {
		t.Fatalf("run=%+v spawned=%d", run, spawner.calls)
	}
	for _, step := range run.Steps {
		if step.State != "pending" {
			t.Fatalf("step=%+v", step)
		}
	}
	if run.CanApprove == nil || *run.CanApprove || run.ApprovalExpiresAt == "" {
		t.Fatalf("initiator view=%+v", run)
	}
	started, _ := time.Parse(time.RFC3339Nano, run.UpdatedAt)
	expiry, err := time.Parse(time.RFC3339Nano, run.ApprovalExpiresAt)
	if err != nil || expiry.Sub(started) != 24*time.Hour {
		t.Fatalf("expiry=%s err=%v", run.ApprovalExpiresAt, err)
	}
	assertRunAudit(t, h, "runbook.run.approval_requested", run.ID, id, initiator)
	assertNoRunAudit(t, h, "runbook.run.start")
	if _, err := h.Store.ReplaceRunbookSteps(context.Background(), id, []*store.RunbookStepRecord{{Kind: "manual", Title: "Edited after request"}}); err != nil {
		t.Fatal(err)
	}
	r := runRequest(approver, id, run.ID, "")
	rr := httptest.NewRecorder()
	h.GetRun(rr, r)
	assertRunStatus(t, rr, 200)
	var view RunResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.CanApprove == nil || !*view.CanApprove || len(view.Steps) != 3 || view.Steps[0].Title != "Verify backup" {
		t.Fatalf("frozen view=%+v", view)
	}
}

func TestApprovalNoEligibleOperatorDoesNotSpendElevation(t *testing.T) {
	h, id, user, _, b := runFixture(t)
	if _, err := h.Store.UpdateRunbook(context.Background(), id, map[string]any{"requires_approval": true}); err != nil {
		t.Fatal(err)
	}
	// A different operator with only one of the two required grants is insufficient.
	operatorOn(t, h, b)
	r := runRequest(user, id, "", "")
	elevateRun(t, h, r, id)
	rr := httptest.NewRecorder()
	h.StartRun(rr, r)
	assertRunStatus(t, rr, 409)
	if !strings.Contains(rr.Body.String(), "no_eligible_approver") {
		t.Fatal(rr.Body.String())
	}
	if _, err := h.ConnH.JWT.ConsumeElevation(r.Header.Get("X-Elevation-Token"), "runbook.run", user, auth.ElevationBinding{Target: id}); err != nil {
		t.Fatalf("token spent: %v", err)
	}
	_, total, err := h.Store.ListRunbookRuns(r.Context(), id, 20, 0)
	if err != nil || total != 0 {
		t.Fatalf("total=%d err=%v", total, err)
	}
	preview := runRequest(user, id, "", "")
	preview.URL.RawQuery = "dryRun=true"
	previewRR := httptest.NewRecorder()
	h.StartRun(previewRR, preview)
	assertRunStatus(t, previewRR, 200)
	var p runPreviewResponse
	if err := json.Unmarshal(previewRR.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if !p.RequiresApproval || p.ApproverAvailable {
		t.Fatalf("preview=%+v", p)
	}
}

func TestApprovalAuthorizationMatrix(t *testing.T) {
	for _, stepUp := range []struct {
		name    string
		enabled bool
	}{{"step-up-on", true}, {"step-up-off", false}} {
		for _, action := range []string{"approve", "reject"} {
			for _, mode := range []string{"initiator", "partial", "disabled", "restricted", "read-only"} {
				t.Run(stepUp.name+"/"+action+"/"+mode, func(t *testing.T) {
					h, id, initiator, approver, spawner := approvalFixture(t)
					h.ConnH.JWT.SetSettingsSource(func() (auth.RuntimeSettings, bool) {
						return auth.RuntimeSettings{StepUpForDestructive: stepUp.enabled}, true
					})
					run := requestApproval(t, h, id, initiator)
					_, steps, err := h.Store.GetRunbookRun(context.Background(), run.ID)
					if err != nil {
						t.Fatal(err)
					}
					actor := approver
					if mode == "initiator" {
						actor = initiator
					}
					if mode == "partial" {
						if _, err := h.Store.UpsertConnectorGrant(context.Background(), approver, steps[2].ConnectorID, "viewer"); err != nil {
							t.Fatal(err)
						}
					}
					if mode == "disabled" {
						if _, err := h.Store.DB().ExecContext(context.Background(), "UPDATE users SET disabled = 1 WHERE id = ?", approver); err != nil {
							t.Fatal(err)
						}
					}
					r := runRequest(actor, id, run.ID, "")
					if mode == "restricted" {
						r = r.WithContext(auth.ContextWithAPIKeyRestriction(r.Context(), auth.APIKeyRestriction{ConnectorIDs: []string{steps[1].ConnectorID}}))
					}
					if mode == "read-only" {
						r = r.WithContext(auth.ContextWithAPIKeyRestriction(r.Context(), auth.APIKeyRestriction{ReadOnly: true}))
					}
					elevateApproval(t, h, r, actor, "runbook.approve", run.ID)
					rr := httptest.NewRecorder()
					if action == "approve" {
						h.ApproveRun(rr, r)
					} else {
						h.RejectRun(rr, r)
					}
					assertRunStatus(t, rr, 403)
					current, _, err := h.Store.GetRunbookRun(context.Background(), run.ID)
					if err != nil || current.State != "awaiting_approval" || spawner.calls != 0 {
						t.Fatalf("run=%+v err=%v spawned=%d", current, err, spawner.calls)
					}
				})
			}
		}

	}
}

func TestApproveRunElevationAndConditionalTransition(t *testing.T) {
	for _, tc := range []struct {
		name, action, target, actor string
		want                        int
	}{
		{name: "missing", want: 400},
		{name: "wrong action", action: "runbook.run", want: 401},
		{name: "wrong target", action: "runbook.approve", target: "other-run", want: 401},
		{name: "another user", action: "runbook.approve", actor: "initiator", want: 401},
		{name: "approve", action: "runbook.approve", want: 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, id, initiator, approver, spawner := approvalFixture(t)
			run := requestApproval(t, h, id, initiator)
			r := runRequest(approver, id, run.ID, "")
			if tc.action != "" {
				target := run.ID
				if tc.target != "" {
					target = tc.target
				}
				actor := approver
				if tc.actor != "" {
					actor = initiator
				}
				elevateApproval(t, h, r, actor, tc.action, target)
			}
			rr := httptest.NewRecorder()
			h.ApproveRun(rr, r)
			assertRunStatus(t, rr, tc.want)
			current, steps, err := h.Store.GetRunbookRun(r.Context(), run.ID)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want != 204 {
				if current.State != "awaiting_approval" || spawner.calls != 0 {
					t.Fatalf("run=%+v spawned=%d", current, spawner.calls)
				}
				assertNoRunAudit(t, h, "runbook.run.approved")
				return
			}
			if current.State != "running" || current.ApprovedBy == nil || *current.ApprovedBy != approver || current.ApprovedAt == "" || current.StartedBy != initiator || current.ResumedBy != nil || spawner.calls != 1 {
				t.Fatalf("run=%+v spawned=%d", current, spawner.calls)
			}
			for _, step := range steps {
				if step.State != "pending" {
					t.Fatalf("step=%+v", step)
				}
			}
			assertRunAudit(t, h, "runbook.run.approved", run.ID, id, approver)
			again := runRequest(approver, id, run.ID, "")
			elevateApproval(t, h, again, approver, "runbook.approve", run.ID)
			againRR := httptest.NewRecorder()
			h.ApproveRun(againRR, again)
			assertRunStatus(t, againRR, 409)
			rejectRR := httptest.NewRecorder()
			h.RejectRun(rejectRR, runRequest(approver, id, run.ID, ""))
			assertRunStatus(t, rejectRR, 409)
			if spawner.calls != 1 {
				t.Fatal("execution spawned twice")
			}
			assertRunAudit(t, h, "runbook.run.approved", run.ID, id, approver)
			assertNoRunAudit(t, h, "runbook.run.rejected")
		})
	}
}

func TestRejectApprovalWithoutElevation(t *testing.T) {
	h, id, initiator, approver, spawner := approvalFixture(t)
	run := requestApproval(t, h, id, initiator)
	rr := httptest.NewRecorder()
	h.RejectRun(rr, runRequest(approver, id, run.ID, ""))
	assertRunStatus(t, rr, 204)
	current, steps, err := h.Store.GetRunbookRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.State != "rejected" || current.RejectedBy == nil || *current.RejectedBy != approver || current.FinishedAt == "" || spawner.calls != 0 {
		t.Fatalf("run=%+v spawned=%d", current, spawner.calls)
	}
	for _, step := range steps {
		if step.State != "skipped" {
			t.Fatalf("step=%+v", step)
		}
	}
	assertRunAudit(t, h, "runbook.run.rejected", run.ID, id, approver)
	again := httptest.NewRecorder()
	h.RejectRun(again, runRequest(approver, id, run.ID, ""))
	assertRunStatus(t, again, 409)
}

func TestAwaitingApprovalExecutionBoundariesAndWithdrawal(t *testing.T) {
	h, id, initiator, _, spawner := approvalFixture(t)
	run := requestApproval(t, h, id, initiator)
	for _, action := range []string{"resume", "confirm", "single-step"} {
		r := runRequest(initiator, id, run.ID, run.Steps[0].ID)
		elevateRun(t, h, r, id)
		rr := httptest.NewRecorder()
		switch action {
		case "resume":
			h.ResumeRun(rr, r)
		case "confirm":
			h.ConfirmRunStep(rr, r)
		case "single-step":
			h.ExecuteStep(rr, r)
		}
		assertRunStatus(t, rr, 409)
		if action == "single-step" && !strings.Contains(rr.Body.String(), "approval_required") {
			t.Fatal(rr.Body.String())
		}
	}
	if spawner.calls != 0 {
		t.Fatal("awaiting run executed")
	}
	rr := httptest.NewRecorder()
	h.CancelRun(rr, runRequest(initiator, id, run.ID, ""))
	assertRunStatus(t, rr, 204)
	current, steps, err := h.Store.GetRunbookRun(context.Background(), run.ID)
	if err != nil || current.State != "cancelled" {
		t.Fatalf("run=%+v err=%v", current, err)
	}
	for _, step := range steps {
		if step.State != "skipped" {
			t.Fatalf("step=%+v", step)
		}
	}
	again := httptest.NewRecorder()
	h.CancelRun(again, runRequest(initiator, id, run.ID, ""))
	assertRunStatus(t, again, 409)
	requestApproval(t, h, id, initiator)
}

func TestApprovalTokenCannotStartOrResume(t *testing.T) {
	h, id, user, _, _ := runFixture(t)
	r := runRequest(user, id, "", "")
	elevateApproval(t, h, r, user, "runbook.approve", id)
	rr := httptest.NewRecorder()
	h.StartRun(rr, r)
	assertRunStatus(t, rr, 401)
	run, _ := seededRun(t, h, id, user, false)
	resume := runRequest(user, id, run.ID, "")
	elevateApproval(t, h, resume, user, "runbook.approve", id)
	resumed := httptest.NewRecorder()
	h.ResumeRun(resumed, resume)
	assertRunStatus(t, resumed, 401)
}

func TestOrdinaryRunStartResponseKeepsShape(t *testing.T) {
	h, id, user, _, _ := runFixture(t)
	r := runRequest(user, id, "", "")
	elevateRun(t, h, r, id)
	rr := httptest.NewRecorder()
	h.StartRun(rr, r)
	assertRunStatus(t, rr, 202)
	var response map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"requiresApproval", "canApprove", "approvalExpiresAt", "approvedBy", "approvedAt", "rejectedBy"} {
		if _, exists := response[key]; exists {
			t.Fatalf("ordinary start gained %s: %s", key, rr.Body.String())
		}
	}
}

func TestRunbookApprovalOptInAuthoring(t *testing.T) {
	h := newTestHandler(t)
	create := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"title":"Review","targetType":"change_type","targetValue":"approval.authoring","requiresApproval":true,"steps":[{"kind":"manual","title":"Review"}]}`))
	rr := httptest.NewRecorder()
	h.Create(rr, create)
	assertRunStatus(t, rr, 201)
	var runbook runbookResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &runbook); err != nil {
		t.Fatal(err)
	}
	if !runbook.RequiresApproval {
		t.Fatal("create lost opt-in")
	}
	for _, tc := range []struct {
		body     string
		want     int
		required bool
	}{
		{`{"title":"Renamed"}`, 200, true}, {`{"requiresApproval":null}`, 400, true},
		{`{"requiresApproval":"yes"}`, 400, true}, {`{"requiresApproval":false}`, 200, false},
	} {
		update := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(tc.body))
		update.SetPathValue("id", runbook.ID)
		updated := httptest.NewRecorder()
		h.Update(updated, update)
		assertRunStatus(t, updated, tc.want)
		stored, err := h.Store.GetRunbook(context.Background(), runbook.ID)
		if err != nil || stored.RequiresApproval != tc.required {
			t.Fatalf("runbook=%+v err=%v", stored, err)
		}
	}
}

func TestApprovalWithInstanceStepUpDisabled(t *testing.T) {
	h, id, initiator, approver, spawner := approvalFixture(t)
	h.ConnH.JWT.SetSettingsSource(func() (auth.RuntimeSettings, bool) {
		return auth.RuntimeSettings{StepUpForDestructive: false}, true
	})
	request := runRequest(initiator, id, "", "")
	rr := httptest.NewRecorder()
	h.StartRun(rr, request)
	assertRunStatus(t, rr, http.StatusAccepted)
	var run RunResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &run); err != nil {
		t.Fatal(err)
	}
	if run.State != "awaiting_approval" || spawner.calls != 0 {
		t.Fatalf("run=%+v spawned=%d", run, spawner.calls)
	}
	approval := runRequest(approver, id, run.ID, "")
	approved := httptest.NewRecorder()
	h.ApproveRun(approved, approval)
	assertRunStatus(t, approved, http.StatusBadRequest)

	elevateApproval(t, h, approval, approver, "runbook.approve", run.ID)
	approvedWithToken := httptest.NewRecorder()
	h.ApproveRun(approvedWithToken, approval)
	assertRunStatus(t, approvedWithToken, http.StatusNoContent)

	current, _, err := h.Store.GetRunbookRun(context.Background(), run.ID)
	if err != nil || current.ApprovedBy == nil || *current.ApprovedBy != approver || current.StartedBy != initiator || spawner.calls != 1 {
		t.Fatalf("run=%+v err=%v spawned=%d", current, err, spawner.calls)
	}
	again := runRequest(approver, id, run.ID, "")
	elevateApproval(t, h, again, approver, "runbook.approve", run.ID)
	againRR := httptest.NewRecorder()
	h.ApproveRun(againRR, again)
	assertRunStatus(t, againRR, http.StatusConflict)
	if spawner.calls != 1 {
		t.Fatal("step-up disabled duplicated execution")
	}
}

func TestApproveAfterRunbookDeleted(t *testing.T) {
	h, id, initiator, approver, spawner := approvalFixture(t)
	run := requestApproval(t, h, id, initiator)
	if err := h.Store.DeleteRunbook(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	r := runRequest(approver, id, run.ID, "")
	elevateApproval(t, h, r, approver, "runbook.approve", run.ID)
	rr := httptest.NewRecorder()
	h.ApproveRun(rr, r)
	assertRunStatus(t, rr, http.StatusNoContent)
	current, _, err := h.Store.GetRunbookRun(context.Background(), run.ID)
	if err != nil || current.State != "running" || current.ApprovedBy == nil || *current.ApprovedBy != approver || spawner.calls != 1 {
		t.Fatalf("run=%+v err=%v spawned=%d", current, err, spawner.calls)
	}
	records, total, err := h.Store.ListAuditRecords(context.Background(), "runbook.run.approved", "runbook_run", "", "", 0, 20)
	if err != nil || total != 1 || records[0].ActorUserID != approver || records[0].TargetID != run.ID {
		t.Fatalf("audit=%+v err=%v", records, err)
	}
	assertRunAuditScope(t, h, records[0].ID, run.ID)
}

func TestApproveDuringShutdownFailsRunAndKeepsApproval(t *testing.T) {
	h, id, initiator, approver, _ := approvalFixture(t)
	run := requestApproval(t, h, id, initiator)
	h.Executor = runbookrun.New(runbookrun.Deps{Store: h.Store, Spawner: runSpawner{false}})
	r := runRequest(approver, id, run.ID, "")
	elevateApproval(t, h, r, approver, "runbook.approve", run.ID)
	rr := httptest.NewRecorder()
	h.ApproveRun(rr, r)
	assertRunStatus(t, rr, http.StatusServiceUnavailable)
	if !strings.Contains(rr.Body.String(), "shutting_down") {
		t.Fatal(rr.Body.String())
	}
	current, _, err := h.Store.GetRunbookRun(context.Background(), run.ID)
	if err != nil || current.State != "failed" || current.Reason != "interrupted" || current.ApprovedBy == nil || *current.ApprovedBy != approver || current.StartedBy != initiator {
		t.Fatalf("run=%+v err=%v", current, err)
	}
	assertRunAudit(t, h, "runbook.run.approved", run.ID, id, approver)
}

func TestConnectorlessApprovalAuthorization(t *testing.T) {
	h := newTestHandler(t)
	initiator, _, _ := connectorlessActor(t, h, "operator")
	approver, _, _ := connectorlessActor(t, h, "operator")
	id := createManualRunbook(t, h, "approval.connectorless")
	if _, err := h.Store.UpdateRunbook(context.Background(), id, map[string]any{"requires_approval": true}); err != nil {
		t.Fatal(err)
	}
	spawner := &approvalSpawner{}
	h.Executor = runbookrun.New(runbookrun.Deps{Store: h.Store, Spawner: spawner})
	run := requestApproval(t, h, id, initiator)
	stranger, _, _ := connectorlessActor(t, h, "none")
	for _, tc := range []struct {
		name, user, action string
		want               int
	}{
		{"initiator approve", initiator, "approve", 403},
		{"initiator reject", initiator, "reject", 403},
		{"no grant approve", stranger, "approve", 403},
		{"no grant reject", stranger, "reject", 403},
		{"other operator reject", approver, "reject", 204},
	} {
		rr := httptest.NewRecorder()
		if tc.action == "approve" {
			h.ApproveRun(rr, runRequest(tc.user, id, run.ID, ""))
		} else {
			h.RejectRun(rr, runRequest(tc.user, id, run.ID, ""))
		}
		if rr.Code != tc.want {
			t.Fatalf("%s: status=%d want=%d body=%s", tc.name, rr.Code, tc.want, rr.Body.String())
		}
	}
	assertRunState(t, h, run.ID, "rejected")
	if spawner.calls != 0 {
		t.Fatal("connectorless rejection executed")
	}
}

func TestCancelAwaitingApprovalByAnotherOperator(t *testing.T) {
	h, id, initiator, approver, spawner := approvalFixture(t)
	run := requestApproval(t, h, id, initiator)
	rr := httptest.NewRecorder()
	h.CancelRun(rr, runRequest(approver, id, run.ID, ""))
	assertRunStatus(t, rr, http.StatusNoContent)
	assertRunState(t, h, run.ID, "cancelled")
	if spawner.calls != 0 {
		t.Fatal("withdrawn request executed")
	}
}

// auditScopeIDs returns the connector scope persisted for the single audit row
// recorded for action.
func auditScopeIDs(t *testing.T, h *Handler, action string) []string {
	t.Helper()
	records, total, err := h.Store.ListAuditRecords(context.Background(), action, "runbook_run", "", "", 0, 20)
	if err != nil || total != 1 {
		t.Fatalf("%s audit total=%d err=%v", action, total, err)
	}
	rows, err := h.Store.DB().QueryContext(context.Background(), `SELECT connector_id FROM audit_log_connectors WHERE audit_id = ? ORDER BY connector_id`, records[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return ids
}

func TestApprovalAuditRowsCarryFrozenConnectorScope(t *testing.T) {
	h, id, initiator, approver, _ := approvalFixture(t)
	steps, err := h.Store.ListRunbookStepsFor(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{}
	for _, step := range steps {
		if step.ConnectorID != "" {
			want = append(want, step.ConnectorID)
		}
	}
	sort.Strings(want)
	if len(want) != 2 {
		t.Fatalf("fixture connectors = %v, want two", want)
	}

	run := requestApproval(t, h, id, initiator)
	if got := auditScopeIDs(t, h, "runbook.run.approval_requested"); !slices.Equal(got, want) {
		t.Fatalf("approval_requested scope = %v, want %v", got, want)
	}

	r := runRequest(approver, id, run.ID, "")
	elevateApproval(t, h, r, approver, "runbook.approve", run.ID)
	rr := httptest.NewRecorder()
	h.ApproveRun(rr, r)
	assertRunStatus(t, rr, http.StatusNoContent)
	if got := auditScopeIDs(t, h, "runbook.run.approved"); !slices.Equal(got, want) {
		t.Fatalf("approved scope = %v, want %v", got, want)
	}

	// The approved run is still active; withdraw it so the runbook can take a second request.
	cr := httptest.NewRecorder()
	h.CancelRun(cr, runRequest(initiator, id, run.ID, ""))
	assertRunStatus(t, cr, http.StatusNoContent)
	second := requestApproval(t, h, id, initiator)
	rej := httptest.NewRecorder()
	h.RejectRun(rej, runRequest(approver, id, second.ID, ""))
	assertRunStatus(t, rej, http.StatusNoContent)
	if got := auditScopeIDs(t, h, "runbook.run.rejected"); !slices.Equal(got, want) {
		t.Fatalf("rejected scope = %v, want %v", got, want)
	}
}

func TestManualOnlyApprovalAuditRowsWriteNoScope(t *testing.T) {
	h := newTestHandler(t)
	initiator, _, _ := connectorlessActor(t, h, "operator")
	approver, _, _ := connectorlessActor(t, h, "operator")
	id := createManualRunbook(t, h, "approval.noscope")
	if _, err := h.Store.UpdateRunbook(context.Background(), id, map[string]any{"requires_approval": true}); err != nil {
		t.Fatal(err)
	}
	h.Executor = runbookrun.New(runbookrun.Deps{Store: h.Store, Spawner: &approvalSpawner{}})
	run := requestApproval(t, h, id, initiator)
	r := runRequest(approver, id, run.ID, "")
	elevateApproval(t, h, r, approver, "runbook.approve", run.ID)
	rr := httptest.NewRecorder()
	h.ApproveRun(rr, r)
	assertRunStatus(t, rr, http.StatusNoContent)
	for _, action := range []string{"runbook.run.approval_requested", "runbook.run.approved"} {
		if got := auditScopeIDs(t, h, action); len(got) != 0 {
			t.Fatalf("%s scope = %v, want none", action, got)
		}
	}
}
