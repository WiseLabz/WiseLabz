package runbookrun

import (
	"context"
	"fmt"

	"github.com/WiseLabz/wiselabz/internal/notifications"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

// Event is the payload of a runbook.run.updated WebSocket event. It carries
// identifiers and states only; clients read the run through the API, which
// applies step redaction.
type Event struct {
	RunID     string `json:"runId"`
	RunbookID string `json:"runbookId,omitempty"`
	// State is the run's state after the change.
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
	// Step is set when the event concerns one step.
	Step *StepEvent `json:"step,omitempty"`
}

// StepEvent identifies a step and its new state.
type StepEvent struct {
	ID       string `json:"id"`
	Position int    `json:"position"`
	State    string `json:"state"`
}

func runEvent(run *store.RunbookRunRecord) Event {
	event := Event{RunID: run.ID, State: run.State, Reason: run.Reason}
	if run.RunbookID != nil {
		event.RunbookID = *run.RunbookID
	}
	return event
}

// publishRun announces a run-level change to every client. The event names no
// step, so it reveals nothing about a connector.
func (e *Executor) publishRun(run *store.RunbookRunRecord) {
	publishRun(e.events, run)
}

// publishStep announces a step change to the clients allowed to read the
// step's connector. A manual step has no connector and is not redacted for
// anyone, so its event is global.
func (e *Executor) publishStep(run *store.RunbookRunRecord, step *store.RunbookRunStepRecord) {
	publishStep(e.events, run, step)
}

func publishRun(events Publisher, run *store.RunbookRunRecord) {
	if events == nil {
		return
	}
	events.Broadcast(ws.EventRunbookRunUpdated, runEvent(run))
}

func publishStep(events Publisher, run *store.RunbookRunRecord, step *store.RunbookRunStepRecord) {
	if events == nil {
		return
	}
	event := runEvent(run)
	event.Step = &StepEvent{ID: step.ID, Position: step.Position, State: step.State}
	if step.ConnectorID == "" {
		events.Broadcast(ws.EventRunbookRunUpdated, event)
		return
	}
	events.BroadcastConnector(step.ConnectorID, ws.EventRunbookRunUpdated, event)
}

func (e *Executor) notifyFailed(ctx context.Context, run *store.RunbookRunRecord, step *store.RunbookRunStepRecord) {
	notifyFailed(ctx, e.notifier, run, step)
}

func (e *Executor) notifyWaiting(ctx context.Context, run *store.RunbookRunRecord, step *store.RunbookRunStepRecord) {
	if e.notifier == nil {
		return
	}
	e.notifier.NotifyRunbookRun(ctx, notifications.EventRunbookRunWaiting, "info", step.ConnectorID, "",
		"Runbook run waiting for confirmation: "+run.RunbookTitle,
		fmt.Sprintf("Step %d %q is waiting for a manual confirmation.", step.Position+1, step.Title))
}

// notifyFailed dispatches runbook.run_failed naming the runbook, the step the
// run stopped on (nil when it stopped between steps) and the reason. The
// notification is scoped to that step's connector. When the run failed for
// lack of a grant, the acting user is notified as well: they may no longer
// hold a grant on the connector and would otherwise never hear of it.
func notifyFailed(ctx context.Context, notifier Notifier, run *store.RunbookRunRecord, step *store.RunbookRunStepRecord) {
	if notifier == nil {
		return
	}
	connectorID := ""
	message := failureSummary(run.Reason)
	if step != nil {
		connectorID = step.ConnectorID
		detail := step.Error
		if detail == "" {
			detail = failureSummary(run.Reason)
		}
		message = fmt.Sprintf("Step %d %q: %s", step.Position+1, step.Title, detail)
		if step.State == StepUnknown {
			message += " Its outcome is unknown."
		}
	}
	actorID := ""
	if run.Reason == ReasonPermissionDenied {
		actorID = ActingUser(run)
	}
	notifier.NotifyRunbookRun(ctx, notifications.EventRunbookRunFailed, "warning", connectorID, actorID,
		"Runbook run failed: "+run.RunbookTitle, message)
}

// failureSummary is the sentence used for a run reason when no step error
// describes the failure.
func failureSummary(reason string) string {
	switch reason {
	case ReasonInterrupted:
		return "The run was interrupted by a backend restart."
	case ReasonStepTimeout:
		return "A step timed out."
	case ReasonPermissionDenied:
		return "The acting user lacks an operator grant."
	case ReasonInternalError:
		return "The run stopped on an internal error."
	default:
		return "A step failed."
	}
}
