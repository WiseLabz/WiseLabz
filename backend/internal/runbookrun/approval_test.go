package runbookrun

import (
	"context"
	"slices"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/notifications"
	"github.com/WiseLabz/wiselabz/internal/store"
)

type approvalNoteRecorder struct {
	noteRecorder
	users [][]string
}

func (r *approvalNoteRecorder) NotifyUsers(_ context.Context, userIDs []string, eventType, severity, title, message string) {
	r.users = append(r.users, append([]string(nil), userIDs...))
	r.NotifyRunbookRun(context.Background(), eventType, severity, "", "", title, message)
}

func TestRequestWaitsForApprovalAndTargetsEligibleApprovers(t *testing.T) {
	e := newEnv(t)
	connectorID := e.connector()
	approver := e.operator(connectorID)
	book, authored := e.runbook(&store.RunbookStepRecord{
		Kind: KindLifecycle, Title: "Restart", ConnectorID: connectorID, Verb: "restart", EntityRef: "100",
	})
	notifier := &approvalNoteRecorder{}
	e.exec.notifier = notifier

	run, frozen, err := e.exec.Request(context.Background(), book.ID, e.starter, authored)
	if err != nil {
		t.Fatalf("Request() error: %v", err)
	}
	if run.State != RunAwaitingApproval || !run.RequiresApproval || run.StartedBy != e.starter {
		t.Fatalf("requested run = %+v, want awaiting approval by starter", run)
	}
	if len(frozen) != 1 || frozen[0].State != StepPending || len(e.lifecycle.snapshot()) != 0 {
		t.Fatalf("request steps = %+v, lifecycle calls = %d; want one pending step and no execution", frozen, len(e.lifecycle.snapshot()))
	}
	if len(notifier.users) != 1 || !slices.Equal(notifier.users[0], []string{approver}) {
		t.Fatalf("approval notification recipients = %v, want only %q", notifier.users, approver)
	}
	if notes := notifier.snapshot(); len(notes) != 1 || notes[0].EventType != notifications.EventRunbookRunApprovalRequested {
		t.Fatalf("approval notifications = %+v, want one approval-request event", notes)
	}
	if starts := e.spawner.starts.Load(); starts != 0 {
		t.Fatalf("background executions started = %d, want 0", starts)
	}
}

func TestApproveExecutesAsInitiatorAndKeepsApproverSeparate(t *testing.T) {
	e := newEnv(t)
	connectorID := e.connector()
	approver := e.operator(connectorID)
	book, authored := e.runbook(&store.RunbookStepRecord{
		Kind: KindLifecycle, Title: "Restart", ConnectorID: connectorID, Verb: "restart", EntityRef: "100",
	})
	run, _, err := e.exec.Request(context.Background(), book.ID, e.starter, authored)
	if err != nil {
		t.Fatalf("Request() error: %v", err)
	}
	if _, _, err := e.exec.Approve(context.Background(), run.ID, approver); err != nil {
		t.Fatalf("Approve() error: %v", err)
	}
	e.settle()

	got, _ := e.get(run.ID)
	if got.State != RunSucceeded || got.ApprovedBy == nil || *got.ApprovedBy != approver || got.ResumedBy != nil {
		t.Fatalf("approved run = %+v, want successful run with approver stored separately", got)
	}
	calls := e.lifecycle.snapshot()
	if len(calls) != 1 || calls[0].Actor.UserID != e.starter {
		t.Fatalf("lifecycle actors = %+v, want original initiator %q", calls, e.starter)
	}
}

func TestApproveRechecksInitiatorGrantBeforeExecution(t *testing.T) {
	e := newEnv(t)
	connectorID := e.connector()
	approver := e.operator(connectorID)
	book, authored := e.runbook(&store.RunbookStepRecord{
		Kind: KindLifecycle, Title: "Restart", ConnectorID: connectorID, Verb: "restart", EntityRef: "100",
	})
	run, _, err := e.exec.Request(context.Background(), book.ID, e.starter, authored)
	if err != nil {
		t.Fatalf("Request() error: %v", err)
	}
	if err := e.s.DeleteConnectorGrant(context.Background(), e.starter, connectorID); err != nil {
		t.Fatalf("revoke initiator grant: %v", err)
	}
	if _, _, err := e.exec.Approve(context.Background(), run.ID, approver); err != nil {
		t.Fatalf("Approve() error: %v", err)
	}
	e.settle()

	got, steps := e.get(run.ID)
	if got.State != RunFailed || got.Reason != ReasonPermissionDenied || stepStates(steps)[0] != StepFailed {
		t.Fatalf("run = %+v, steps = %v; want permission-denied failure", got, stepStates(steps))
	}
	if calls := e.lifecycle.snapshot(); len(calls) != 0 {
		t.Fatalf("lifecycle calls = %+v, want no operation after initiator grant revocation", calls)
	}
}

func TestRequestFreezesConnectorActionFingerprint(t *testing.T) {
	fa := newFakeActions("approval-fingerprint-1")
	e := newActionEnv(t, fa)
	connectorID := e.connector()
	approver := e.operator(connectorID)
	book, authored := e.runbook(actionStep(connectorID, "rescan", "100"))
	run, frozen, err := e.exec.Request(context.Background(), book.ID, e.starter, authored)
	if err != nil {
		t.Fatalf("Request() error: %v", err)
	}
	if len(frozen) != 1 || frozen[0].ActionFingerprint != "approval-fingerprint-1" {
		t.Fatalf("requested step = %+v, want request-time action fingerprint", frozen)
	}
	fa.setFingerprint("approval-fingerprint-2")
	if _, _, err := e.exec.Approve(context.Background(), run.ID, approver); err != nil {
		t.Fatalf("Approve() error: %v", err)
	}
	e.settle()

	got, steps := e.get(run.ID)
	if got.State != RunFailed || len(steps) != 1 || len(fa.snapshot()) != 0 {
		t.Fatalf("run = %+v, steps = %+v, action calls = %d; want changed action rejected before send", got, steps, len(fa.snapshot()))
	}
}

func TestRejectSkipsStepsWithoutSpawning(t *testing.T) {
	e := newEnv(t)
	connectorID := e.connector()
	approver := e.operator(connectorID)
	book, authored := e.runbook(&store.RunbookStepRecord{
		Kind: KindLifecycle, Title: "Restart", ConnectorID: connectorID, Verb: "restart", EntityRef: "100",
	})
	run, _, err := e.exec.Request(context.Background(), book.ID, e.starter, authored)
	if err != nil {
		t.Fatalf("Request() error: %v", err)
	}
	rejected, steps, err := e.exec.Reject(context.Background(), run.ID, approver)
	if err != nil {
		t.Fatalf("Reject() error: %v", err)
	}
	if rejected.State != RunRejected || rejected.RejectedBy == nil || *rejected.RejectedBy != approver || len(steps) != 1 || steps[0].State != StepSkipped {
		t.Fatalf("rejected run = %+v, steps = %+v; want rejected with skipped step", rejected, steps)
	}
	if calls := e.lifecycle.snapshot(); len(calls) != 0 {
		t.Fatalf("lifecycle calls = %+v, want none", calls)
	}
}
