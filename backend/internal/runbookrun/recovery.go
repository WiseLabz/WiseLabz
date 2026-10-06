package runbookrun

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/WiseLabz/wiselabz/internal/logsafe"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// RecoveryStore is the persistence startup recovery needs. *store.Store
// satisfies it.
type RecoveryStore interface {
	InterruptRunningRunbookRuns(ctx context.Context) ([]string, error)
	GetRunbookRun(ctx context.Context, id string) (*store.RunbookRunRecord, []*store.RunbookRunStepRecord, error)
}

// Recover marks every run left running by a previous process as failed with
// reason interrupted and its in-flight step as unknown, then announces each
// one. It never continues a run: the outcome of the interrupted step is not
// known and no user is present, so a user must resume. Runs waiting on a
// manual step are untouched. It must run before the server accepts requests,
// while no executor goroutine exists. events and notifier may be nil.
func Recover(ctx context.Context, s RecoveryStore, events Publisher, notifier Notifier) (int, error) {
	ids, err := s.InterruptRunningRunbookRuns(ctx)
	if err != nil {
		return 0, fmt.Errorf("recover runbook runs: %w", err)
	}
	for _, id := range ids {
		run, steps, err := s.GetRunbookRun(ctx, id)
		if err != nil {
			// The run is already marked; only its announcement is lost.
			slog.Error("runbook run: read interrupted run failed", "run", logsafe.Sanitize(id), "error", err)
			continue
		}
		interrupted := interruptedStep(run, steps)
		if interrupted != nil {
			publishStep(events, run, interrupted)
		}
		publishRun(events, run)
		notifyFailed(ctx, notifier, run, interrupted)
	}
	return len(ids), nil
}

// interruptedStep returns the step that was in flight when the run was
// interrupted, or nil when the run stopped between steps. The store stamps
// the run and that step with the same time.
func interruptedStep(run *store.RunbookRunRecord, steps []*store.RunbookRunStepRecord) *store.RunbookRunStepRecord {
	for _, step := range steps {
		if step.State == StepUnknown && step.FinishedAt == run.UpdatedAt {
			return step
		}
	}
	return nil
}
