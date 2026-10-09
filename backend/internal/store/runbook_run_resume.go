package store

import (
	"context"
	"fmt"
)

// ResumeRunbookRun resumes a failed run as userID. It is ErrConflict, and
// nothing changes, unless the run is failed and its updated_at equals
// expectedUpdatedAt and, when expectedStepID is not empty, that step is the
// first step of the run that has not succeeded. The comparison happens inside
// the transaction that changes the run, so it protects exactly the revision the
// value came from: pass the revision the operator saw to tie a decision to it,
// or the one the server just read to cover only that read-to-transaction
// window. A non-nil audit is inserted in the same transaction.
func (s *Store) ResumeRunbookRun(ctx context.Context, runID, expectedUpdatedAt, expectedStepID, userID string, audit *AuditRecord) (*RunbookRunRecord, error) {
	return s.resumeRunbookRun(ctx, runID, expectedUpdatedAt, expectedStepID, false, userID, audit)
}

// ResumeRunbookRunMarkingStepDone resumes a failed run as userID and marks one
// of its steps succeeded, in one transaction, so a crash cannot leave the step
// done while the run stays failed or the run resumes on the step. The run must
// be failed and its updated_at must equal expectedUpdatedAt. The step must be
// unknown and must be the first step of the run that has not succeeded;
// otherwise the result is ErrConflict and nothing changes. The run's fields
// change as in a plain resume, and the step's error is cleared and its finish
// time set. A non-nil audit is inserted in the same transaction. The caller
// decides what the step's kind allows.
func (s *Store) ResumeRunbookRunMarkingStepDone(ctx context.Context, runID, expectedUpdatedAt, stepID, userID string, audit *AuditRecord) (*RunbookRunRecord, error) {
	if stepID == "" {
		return nil, ErrConflict
	}
	return s.resumeRunbookRun(ctx, runID, expectedUpdatedAt, stepID, true, userID, audit)
}

func (s *Store) resumeRunbookRun(ctx context.Context, runID, expectedUpdatedAt, expectedStepID string, markDone bool, userID string, audit *AuditRecord) (*RunbookRunRecord, error) {
	var run *RunbookRunRecord
	err := s.WithinTransaction(ctx, func(tx *Store) error {
		parent, err := lockRunbookRun(ctx, tx.db, s.driver == "postgres", runID)
		if err != nil {
			return err
		}
		if parent.State != "failed" || parent.UpdatedAt != expectedUpdatedAt {
			return ErrConflict
		}
		now := nextRunbookRunTimestamp(parent.UpdatedAt)
		if expectedStepID != "" {
			steps, err := listRunbookRunSteps(ctx, tx.db, runID)
			if err != nil {
				return err
			}
			first := firstUnfinishedRunbookRunStep(steps)
			if first == nil || first.ID != expectedStepID {
				return ErrConflict
			}
			if markDone {
				if first.State != "unknown" {
					return ErrConflict
				}
				stepSet, err := normalizedRunbookRunStepUpdates(map[string]any{"state": "succeeded", "error": ""}, "unknown", now)
				if err != nil {
					return err
				}
				if err := updateRunbookRunStep(ctx, tx.db, runID, expectedStepID, "unknown", stepSet); err != nil {
					return err
				}
			}
		}
		runSet, err := normalizedRunbookRunUpdates(map[string]any{"state": "running", "resumed_by": userID}, now)
		if err != nil {
			return err
		}
		if err := updateRunbookRun(ctx, tx.db, runID, "failed", runSet, now); err != nil {
			return err
		}
		if audit != nil {
			if err := tx.CreateAuditRecord(ctx, audit); err != nil {
				return err
			}
		}
		run, err = scanRunbookRun(tx.db.QueryRowContext(ctx, `SELECT `+runbookRunColumns+` FROM runbook_runs WHERE id = ?`, runID))
		if err != nil {
			return fmt.Errorf("read resumed runbook run: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("resume runbook run: %w", err)
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
