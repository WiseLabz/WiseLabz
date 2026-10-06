package main

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/notifications"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

// TestRecoverRunbookRunsMarksRunningRunsInterrupted asserts the startup step
// fails a run the previous process left running, marks its in-flight step
// unknown, notifies, and leaves a run waiting on a manual step alone.
func TestRecoverRunbookRunsMarksRunningRunsInterrupted(t *testing.T) {
	ctx := context.Background()
	s := apitest.NewStore(t)
	userID := apitest.NewUser(t, s, "viewer")
	apitest.GrantConnectorRole(t, s, userID, "svc-1", "operator")
	dispatcher := notifications.NewDispatcher(s, nil)

	newRun := func(step *store.RunbookRunStepRecord) (*store.RunbookRunRecord, *store.RunbookRunStepRecord) {
		t.Helper()
		suffix := uuid.NewString()
		book, err := s.CreateRunbook(ctx, &store.RunbookRecord{Title: "Runbook " + suffix, TargetType: "change_type", TargetValue: suffix})
		if err != nil {
			t.Fatalf("create runbook: %v", err)
		}
		run, steps, err := s.CreateRunbookRun(ctx, book.ID, userID, []*store.RunbookRunStepRecord{step})
		if err != nil {
			t.Fatalf("create run: %v", err)
		}
		return run, steps[0]
	}
	running, runningStep := newRun(&store.RunbookRunStepRecord{Kind: "lifecycle", Title: "Restart", ConnectorID: "svc-1", Verb: "restart"})
	if _, err := s.UpdateRunbookRunStep(ctx, running.ID, runningStep.ID, "pending", map[string]any{"state": "running"}); err != nil {
		t.Fatalf("start step: %v", err)
	}
	waiting, waitingStep := newRun(&store.RunbookRunStepRecord{Kind: "manual", Title: "Check"})
	if _, _, err := s.PauseRunbookRunOnManualStep(ctx, waiting.ID, waitingStep.ID); err != nil {
		t.Fatalf("pause run: %v", err)
	}

	// The hub is not running yet, exactly as in main before lifecycle.Start.
	if err := recoverRunbookRuns(ctx, s, ws.NewHub(), dispatcher, testLogger()); err != nil {
		t.Fatalf("recoverRunbookRuns: %v", err)
	}
	dispatcher.Wait()

	got, steps, err := s.GetRunbookRun(ctx, running.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got.State != "failed" || got.Reason != "interrupted" || steps[0].State != "unknown" {
		t.Fatalf("running run after recovery = %+v, step %+v; want failed/interrupted with the step unknown", got, steps[0])
	}
	got, steps, err = s.GetRunbookRun(ctx, waiting.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got.State != "waiting_manual" || steps[0].State != "waiting" {
		t.Fatalf("waiting run after recovery = %+v, step %+v; want it untouched", got, steps[0])
	}

	notifs, _, err := s.ListNotifications(ctx, userID, false, 0, 10)
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	if len(notifs) != 1 || notifs[0].EventType != notifications.EventRunbookRunFailed {
		t.Fatalf("notifications = %+v, want one %s", notifs, notifications.EventRunbookRunFailed)
	}
}
