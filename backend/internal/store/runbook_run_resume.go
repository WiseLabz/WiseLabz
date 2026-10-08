package store

import (
	"context"
	"fmt"
)

// ResumeRunbookRunMarkingStepDone resumes a failed run as userID and marks one
// of its steps succeeded, in one transaction, so a crash cannot leave the step
// done while the run stays failed or the run resumes on the step. The run must
// be failed. The step must be unknown and must be the first step of the run
// that has not succeeded; otherwise the result is ErrConflict and nothing
// changes. The run's fields change as in a plain resume, and the step's error
// is cleared and its finish time set. The caller decides what the step's kind
// allows.
func (s *Store) ResumeRunbookRunMarkingStepDone(ctx context.Context, runID, stepID, userID string) (*RunbookRunRecord, error) {
	var run *RunbookRunRecord
	err := s.WithinTransaction(ctx, func(tx *Store) error {
		parent, err := lockRunbookRun(ctx, tx.db, s.driver == "postgres", runID)
		if err != nil {
			return err
		}
		if parent.State != "failed" {
			return ErrConflict
		}
		steps, err := listRunbookRunSteps(ctx, tx.db, runID)
		if err != nil {
			return err
		}
		first := firstUnfinishedRunbookRunStep(steps)
		if first == nil || first.ID != stepID || first.State != "unknown" {
			return ErrConflict
		}
		now := nextRunbookRunTimestamp(parent.UpdatedAt)
		stepSet, err := normalizedRunbookRunStepUpdates(map[string]any{"state": "succeeded", "error": ""}, "unknown", now)
		if err != nil {
			return err
		}
		if err := updateRunbookRunStep(ctx, tx.db, runID, stepID, "unknown", stepSet); err != nil {
			return err
		}
		runSet, err := normalizedRunbookRunUpdates(map[string]any{"state": "running", "resumed_by": userID}, now)
		if err != nil {
			return err
		}
		if err := updateRunbookRun(ctx, tx.db, runID, "failed", runSet, now); err != nil {
			return err
		}
		run, err = scanRunbookRun(tx.db.QueryRowContext(ctx, `SELECT `+runbookRunColumns+` FROM runbook_runs WHERE id = ?`, runID))
		if err != nil {
			return fmt.Errorf("read resumed runbook run: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("resume runbook run marking step done: %w", err)
	}
	return run, nil
}

// firstUnfinishedRunbookRunStep returns the first step, in position order, that
// has not succeeded, or nil when every step has.
func firstUnfinishedRunbookRunStep(steps []*RunbookRunStepRecord) *RunbookRunStepRecord {
	for _, step := range steps {
		if step.State != "succeeded" {
			return step
		}
	}
	return nil
}
