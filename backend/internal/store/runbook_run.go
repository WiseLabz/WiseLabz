package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

const runbookRunTimestampLayout = "2006-01-02T15:04:05.000000000Z"

// RunbookRunRecord is an immutable snapshot header and its current state.
// RunbookID becomes nil if the authored runbook is deleted; the frozen title
// and step rows remain available as history.
type RunbookRunRecord struct {
	ID           string  `json:"id"`
	RunbookID    *string `json:"runbookId,omitempty"`
	RunbookTitle string  `json:"runbookTitle"`
	State        string  `json:"state"`
	Reason       string  `json:"reason,omitempty"`
	StartedBy    string  `json:"startedBy"`
	ResumedBy    *string `json:"resumedBy,omitempty"`
	CancelledBy  *string `json:"cancelledBy,omitempty"`
	StartedAt    string  `json:"startedAt"`
	UpdatedAt    string  `json:"updatedAt"`
	FinishedAt   string  `json:"finishedAt,omitempty"`
}

// RunbookRunStepRecord is one frozen authored step and its execution state.
// ConnectorID and Verb are empty for manual steps and are stored as SQL NULL.
type RunbookRunStepRecord struct {
	ID             string `json:"id"`
	RunID          string `json:"runId"`
	Position       int    `json:"position"`
	Kind           string `json:"kind"`
	Title          string `json:"title"`
	ConnectorID    string `json:"connectorId,omitempty"`
	Verb           string `json:"verb,omitempty"`
	EntityRef      string `json:"entityRef,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	State          string `json:"state"`
	StartedAt      string `json:"startedAt,omitempty"`
	FinishedAt     string `json:"finishedAt,omitempty"`
	Error          string `json:"error,omitempty"`
	ConfirmedBy    string `json:"confirmedBy,omitempty"`
}

// RunbookRunConflictError identifies the active run that prevented a second
// run for the same runbook. It remains compatible with errors.Is(err,
// ErrConflict).
type RunbookRunConflictError struct {
	RunID string
}

func (e *RunbookRunConflictError) Error() string {
	return fmt.Sprintf("active run %s already exists: %v", e.RunID, ErrConflict)
}

func (e *RunbookRunConflictError) Unwrap() error { return ErrConflict }

// ErrRunbookRunStepCount reports a run created without steps, with a nil step
// or with more steps than a runbook may hold.
var ErrRunbookRunStepCount = errors.New("a runbook run needs between 1 and 20 steps")

// runbookRunActiveIndex is the partial unique index allowing one active run
// per runbook (migration 000062).
const runbookRunActiveIndex = "idx_runbook_runs_one_active"

// isActiveRunbookRunViolation reports whether err is a violation of
// runbookRunActiveIndex specifically. PostgreSQL names the index; SQLite names
// the indexed column, which no other unique constraint covers.
func isActiveRunbookRunViolation(err error) bool {
	if err == nil {
		return false
	}
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code == "23505" && pgErr.ConstraintName == runbookRunActiveIndex
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed: runbook_runs.runbook_id") ||
		strings.Contains(msg, "UNIQUE constraint failed: index '"+runbookRunActiveIndex+"'")
}

const runbookRunColumns = `id, runbook_id, runbook_title, state, reason, started_by, resumed_by, cancelled_by, started_at, updated_at, finished_at`

const runbookRunStepColumns = `id, run_id, position, kind, title, connector_id, verb, entity_ref, timeout_seconds, state, started_at, finished_at, error, confirmed_by`

// CreateRunbookRun atomically stores a new active run and its frozen steps.
// The runbook title is read inside the transaction, and the supplied steps
// are copied in order without retaining a reference to authored step rows.
// A step count outside 1 to 20 is ErrRunbookRunStepCount; a second active run
// for the runbook is a *RunbookRunConflictError naming the existing run.
func (s *Store) CreateRunbookRun(ctx context.Context, runbookID, startedBy string, steps []*RunbookRunStepRecord) (*RunbookRunRecord, []*RunbookRunStepRecord, error) {
	if len(steps) == 0 || len(steps) > 20 {
		return nil, nil, fmt.Errorf("create runbook run: %w", ErrRunbookRunStepCount)
	}

	run := &RunbookRunRecord{
		ID:        uuid.New().String(),
		State:     "running",
		StartedBy: startedBy,
	}
	savedSteps := make([]*RunbookRunStepRecord, 0, len(steps))
	for position, step := range steps {
		if step == nil {
			return nil, nil, fmt.Errorf("create runbook run: frozen step %d is nil: %w", position, ErrRunbookRunStepCount)
		}
		savedSteps = append(savedSteps, &RunbookRunStepRecord{
			ID:             uuid.New().String(),
			RunID:          run.ID,
			Position:       position,
			Kind:           step.Kind,
			Title:          step.Title,
			ConnectorID:    step.ConnectorID,
			Verb:           step.Verb,
			EntityRef:      step.EntityRef,
			TimeoutSeconds: step.TimeoutSeconds,
			State:          "pending",
		})
	}

	now := runbookRunTimestamp(time.Now())
	run.StartedAt = now
	run.UpdatedAt = now
	err := s.WithinTransaction(ctx, func(tx *Store) error {
		if err := tx.db.QueryRowContext(ctx, `SELECT title FROM runbooks WHERE id = ?`, runbookID).Scan(&run.RunbookTitle); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("read runbook title for run: %w", err)
		}
		run.RunbookID = &runbookID
		if _, err := tx.db.ExecContext(ctx, `
			INSERT INTO runbook_runs (id, runbook_id, runbook_title, state, reason, started_by, resumed_by, cancelled_by, started_at, updated_at, finished_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, run.ID, runbookID, run.RunbookTitle, run.State, run.Reason, run.StartedBy, nilToStrPtr(run.ResumedBy), nilToStrPtr(run.CancelledBy), run.StartedAt, run.UpdatedAt, nilToStr(run.FinishedAt)); err != nil {
			return fmt.Errorf("insert runbook run: %w", err)
		}
		for _, step := range savedSteps {
			if _, err := tx.db.ExecContext(ctx, `
				INSERT INTO runbook_run_steps (id, run_id, position, kind, title, connector_id, verb, entity_ref, timeout_seconds, state, started_at, finished_at, error, confirmed_by)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, step.ID, step.RunID, step.Position, step.Kind, step.Title, nilToStr(step.ConnectorID), nilToStr(step.Verb), step.EntityRef, step.TimeoutSeconds, step.State, nilToStr(step.StartedAt), nilToStr(step.FinishedAt), step.Error, nilToStr(step.ConfirmedBy)); err != nil {
				return fmt.Errorf("insert frozen runbook step: %w", err)
			}
		}
		return nil
	})
	if err == nil {
		return run, savedSteps, nil
	}

	if isActiveRunbookRunViolation(err) {
		// The winner can finish before this lookup; the conflict stays typed
		// and RunID is then empty.
		existingID, lookupErr := s.activeRunbookRunID(ctx, runbookID)
		if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
			return nil, nil, fmt.Errorf("create runbook run: find active run: %w", lookupErr)
		}
		return nil, nil, &RunbookRunConflictError{RunID: existingID}
	}
	return nil, nil, fmt.Errorf("create runbook run: %w", err)
}

// GetRunbookRun retrieves a run and its frozen steps ordered by position.
func (s *Store) GetRunbookRun(ctx context.Context, id string) (*RunbookRunRecord, []*RunbookRunStepRecord, error) {
	run, err := scanRunbookRun(s.db.QueryRowContext(ctx, `SELECT `+runbookRunColumns+` FROM runbook_runs WHERE id = ?`, id))
	if err != nil {
		return nil, nil, fmt.Errorf("get runbook run: %w", err)
	}
	steps, err := listRunbookRunSteps(ctx, s.db, id)
	if err != nil {
		return nil, nil, err
	}
	return run, steps, nil
}

// ListRunbookRuns returns one runbook's history newest first and its total
// count. A limit of zero or less uses the default page size of 20; a larger
// limit is clamped to 100.
func (s *Store) ListRunbookRuns(ctx context.Context, runbookID string, limit, offset int) ([]*RunbookRunRecord, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return paginatedQuery(ctx, s.db, "runbook_runs", runbookRunColumns, "WHERE runbook_id = ?", []any{runbookID}, "started_at DESC, id DESC", limit, offset, scanRunbookRun)
}

// UpdateRunbookRun applies an allowlisted state or actor update only if the
// run remains in expectedState. Every successful update refreshes activity.
// A key outside the allowlist (state, reason, resumed_by, cancelled_by,
// finished_at) is an error rather than being ignored. Moving a run to
// cancelled or expired is rejected: CancelRunbookRun and ExpireOpenRunbookRuns
// own those transitions because they also skip unfinished steps. A succeeded
// update with an absent or empty finished_at defaults it to now.
func (s *Store) UpdateRunbookRun(ctx context.Context, id, expectedState string, updates map[string]any) (*RunbookRunRecord, error) {
	if len(updates) == 0 {
		return nil, fmt.Errorf("update runbook run: no fields to update")
	}
	switch updates["state"] {
	case "cancelled":
		return nil, fmt.Errorf("update runbook run: state cancelled must be set with CancelRunbookRun")
	case "expired":
		return nil, fmt.Errorf("update runbook run: state expired must be set with ExpireOpenRunbookRuns")
	}
	var updated *RunbookRunRecord
	err := s.WithinTransaction(ctx, func(tx *Store) error {
		current, err := lockRunbookRun(ctx, tx.db, s.driver == "postgres", id)
		if err != nil {
			return err
		}
		if current.State != expectedState {
			return ErrConflict
		}
		if !isOpenRunbookRunState(current.State) {
			return ErrConflict
		}
		now := nextRunbookRunTimestamp(current.UpdatedAt)
		updates, err = normalizedRunbookRunUpdates(updates, now)
		if err != nil {
			return err
		}
		if err := updateRunbookRun(ctx, tx.db, id, expectedState, updates, now); err != nil {
			return err
		}
		updated, err = scanRunbookRun(tx.db.QueryRowContext(ctx, `SELECT `+runbookRunColumns+` FROM runbook_runs WHERE id = ?`, id))
		if err != nil {
			return fmt.Errorf("read updated runbook run: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update runbook run: %w", err)
	}
	return updated, nil
}

// UpdateRunbookRunStep transitions one frozen step only when it is still in
// expectedState and its parent run remains active. The parent row is locked
// and touched in the same transaction, so cancellation and expiry serialize
// with step progress. A key outside the allowlist (state, started_at,
// finished_at, error, confirmed_by) is an error rather than being ignored.
func (s *Store) UpdateRunbookRunStep(ctx context.Context, runID, stepID, expectedState string, updates map[string]any) (*RunbookRunStepRecord, error) {
	if len(updates) == 0 {
		return nil, fmt.Errorf("update runbook run step: no fields to update")
	}
	var updated *RunbookRunStepRecord
	err := s.WithinTransaction(ctx, func(tx *Store) error {
		parent, err := lockRunbookRun(ctx, tx.db, s.driver == "postgres", runID)
		if err != nil {
			return err
		}
		if parent.State != "running" {
			return ErrConflict
		}
		now := nextRunbookRunTimestamp(parent.UpdatedAt)
		result, err := tx.db.ExecContext(ctx, `UPDATE runbook_runs SET updated_at = ? WHERE id = ? AND state IN ('running','waiting_manual','failed')`, now, runID)
		if err != nil {
			return fmt.Errorf("touch parent runbook run: %w", err)
		}
		if n, err := result.RowsAffected(); err != nil {
			return fmt.Errorf("count touched runbook runs: %w", err)
		} else if n != 1 {
			return ErrConflict
		}
		updates, err = normalizedRunbookRunStepUpdates(updates, expectedState, now)
		if err != nil {
			return err
		}
		if err := updateRunbookRunStep(ctx, tx.db, runID, stepID, expectedState, updates); err != nil {
			return err
		}
		updated, err = scanRunbookRunStep(tx.db.QueryRowContext(ctx, `SELECT `+runbookRunStepColumns+` FROM runbook_run_steps WHERE run_id = ? AND id = ?`, runID, stepID))
		if err != nil {
			return fmt.Errorf("read updated runbook run step: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update runbook run step: %w", err)
	}
	return updated, nil
}

// ConfirmRunbookRunStep atomically records the confirming actor, completes a
// waiting manual step and moves its parent run back to running.
func (s *Store) ConfirmRunbookRunStep(ctx context.Context, runID, stepID, confirmedBy string) error {
	return s.WithinTransaction(ctx, func(tx *Store) error {
		parent, err := lockRunbookRun(ctx, tx.db, s.driver == "postgres", runID)
		if err != nil {
			return err
		}
		if parent.State != "waiting_manual" {
			return ErrConflict
		}
		now := nextRunbookRunTimestamp(parent.UpdatedAt)
		result, err := tx.db.ExecContext(ctx, `UPDATE runbook_run_steps SET state = 'succeeded', confirmed_by = ?, finished_at = ? WHERE run_id = ? AND id = ? AND state = 'waiting' AND kind = 'manual'`, confirmedBy, now, runID, stepID)
		if err != nil {
			return fmt.Errorf("confirm manual runbook step: %w", err)
		}
		if n, err := result.RowsAffected(); err != nil {
			return fmt.Errorf("count confirmed runbook steps: %w", err)
		} else if n != 1 {
			return ErrConflict
		}
		result, err = tx.db.ExecContext(ctx, `UPDATE runbook_runs SET state = 'running', reason = '', updated_at = ?, finished_at = NULL WHERE id = ? AND state = 'waiting_manual'`, now, runID)
		if err != nil {
			return fmt.Errorf("resume runbook after manual confirmation: %w", err)
		}
		if n, err := result.RowsAffected(); err != nil {
			return fmt.Errorf("count resumed runbook runs: %w", err)
		} else if n != 1 {
			return ErrConflict
		}
		return nil
	})
}

// PauseRunbookRunOnManualStep atomically moves a pending manual step to
// waiting and its running parent to waiting_manual, so a crash cannot leave a
// waiting step inside a running run.
func (s *Store) PauseRunbookRunOnManualStep(ctx context.Context, runID, stepID string) (*RunbookRunRecord, *RunbookRunStepRecord, error) {
	return s.transitionRunbookRunWithStep(ctx, "pause runbook run on manual step", runID, stepID, "pending",
		map[string]any{"state": "waiting_manual"}, map[string]any{"state": "waiting"})
}

// FailRunbookRunStep atomically ends a running step and fails its running
// parent with reason. stepState is failed when the step is known to have
// failed, or unknown when its outcome could not be established.
func (s *Store) FailRunbookRunStep(ctx context.Context, runID, stepID, stepState, stepError, reason string) (*RunbookRunRecord, *RunbookRunStepRecord, error) {
	if stepState != "failed" && stepState != "unknown" {
		return nil, nil, fmt.Errorf("fail runbook run step: unsupported step state %q", stepState)
	}
	return s.transitionRunbookRunWithStep(ctx, "fail runbook run step", runID, stepID, "running",
		map[string]any{"state": "failed", "reason": reason}, map[string]any{"state": stepState, "error": stepError})
}

// FinishRunbookRun atomically completes the last running step and its running
// parent. It is ErrConflict while any other step has not succeeded.
func (s *Store) FinishRunbookRun(ctx context.Context, runID, stepID string) (*RunbookRunRecord, *RunbookRunStepRecord, error) {
	return s.transitionRunbookRunWithStep(ctx, "finish runbook run", runID, stepID, "running",
		map[string]any{"state": "succeeded"}, map[string]any{"state": "succeeded"})
}

// transitionRunbookRunWithStep applies one step update and one run update in a
// single transaction. The run must be running and the step in
// expectedStepState, otherwise it is ErrConflict and nothing changes. A run
// may only succeed once every step has.
func (s *Store) transitionRunbookRunWithStep(
	ctx context.Context,
	op, runID, stepID, expectedStepState string,
	runUpdates, stepUpdates map[string]any,
) (*RunbookRunRecord, *RunbookRunStepRecord, error) {
	var run *RunbookRunRecord
	var step *RunbookRunStepRecord
	err := s.WithinTransaction(ctx, func(tx *Store) error {
		parent, err := lockRunbookRun(ctx, tx.db, s.driver == "postgres", runID)
		if err != nil {
			return err
		}
		if parent.State != "running" {
			return ErrConflict
		}
		now := nextRunbookRunTimestamp(parent.UpdatedAt)
		stepSet, err := normalizedRunbookRunStepUpdates(stepUpdates, expectedStepState, now)
		if err != nil {
			return err
		}
		if err := updateRunbookRunStep(ctx, tx.db, runID, stepID, expectedStepState, stepSet); err != nil {
			return err
		}
		if runUpdates["state"] == "succeeded" {
			var unfinished int
			if err := tx.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM runbook_run_steps WHERE run_id = ? AND state <> 'succeeded'`, runID).Scan(&unfinished); err != nil {
				return fmt.Errorf("count unfinished runbook steps: %w", err)
			}
			if unfinished != 0 {
				return ErrConflict
			}
		}
		runSet, err := normalizedRunbookRunUpdates(runUpdates, now)
		if err != nil {
			return err
		}
		if err := updateRunbookRun(ctx, tx.db, runID, "running", runSet, now); err != nil {
			return err
		}
		run, err = scanRunbookRun(tx.db.QueryRowContext(ctx, `SELECT `+runbookRunColumns+` FROM runbook_runs WHERE id = ?`, runID))
		if err != nil {
			return fmt.Errorf("read updated runbook run: %w", err)
		}
		step, err = scanRunbookRunStep(tx.db.QueryRowContext(ctx, `SELECT `+runbookRunStepColumns+` FROM runbook_run_steps WHERE run_id = ? AND id = ?`, runID, stepID))
		if err != nil {
			return fmt.Errorf("read updated runbook run step: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", op, err)
	}
	return run, step, nil
}

// CancelRunbookRun cancels an open run and atomically closes unfinished work
// without rewriting what happened: pending and waiting steps become skipped, a
// running step becomes unknown because its real outcome is not known, and an
// unknown step stays unknown.
func (s *Store) CancelRunbookRun(ctx context.Context, id, cancelledBy string) error {
	return s.WithinTransaction(ctx, func(tx *Store) error {
		run, err := lockRunbookRun(ctx, tx.db, s.driver == "postgres", id)
		if err != nil {
			return err
		}
		if !isOpenRunbookRunState(run.State) {
			return ErrConflict
		}
		now := nextRunbookRunTimestamp(run.UpdatedAt)
		if err := closeUnfinishedRunbookRunSteps(ctx, tx.db, id, now); err != nil {
			return err
		}
		result, err := tx.db.ExecContext(ctx, `UPDATE runbook_runs SET state = 'cancelled', cancelled_by = ?, updated_at = ?, finished_at = ? WHERE id = ? AND state IN ('running','waiting_manual','failed')`, cancelledBy, now, now, id)
		if err != nil {
			return fmt.Errorf("cancel runbook run: %w", err)
		}
		if n, err := result.RowsAffected(); err != nil {
			return fmt.Errorf("count cancelled runbook runs: %w", err)
		} else if n != 1 {
			return ErrConflict
		}
		return nil
	})
}

// InterruptRunningRunbookRuns fails in-flight runs at startup without
// continuing them and marks their in-flight steps unknown. A waiting step left
// inside a running run (a pause that did not complete) goes back to pending so
// resuming pauses on it again. Runs in waiting_manual are intentionally
// unchanged. It returns the IDs of the runs it transitioned, in
// processing order, so callers can publish an update per run.
func (s *Store) InterruptRunningRunbookRuns(ctx context.Context) ([]string, error) {
	interrupted := make([]string, 0)
	err := s.WithinTransaction(ctx, func(tx *Store) error {
		ids, err := runbookRunIDs(ctx, tx.db, s.driver == "postgres", `state = 'running'`)
		if err != nil {
			return err
		}
		for _, id := range ids {
			run, err := lockRunbookRun(ctx, tx.db, s.driver == "postgres", id)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					continue
				}
				return err
			}
			if run.State != "running" {
				continue
			}
			now := nextRunbookRunTimestamp(run.UpdatedAt)
			if _, err := tx.db.ExecContext(ctx, `UPDATE runbook_run_steps SET state = 'unknown', finished_at = ? WHERE run_id = ? AND state = 'running'`, now, id); err != nil {
				return fmt.Errorf("mark interrupted runbook steps unknown: %w", err)
			}
			if _, err := tx.db.ExecContext(ctx, `UPDATE runbook_run_steps SET state = 'pending', started_at = NULL WHERE run_id = ? AND state = 'waiting'`, id); err != nil {
				return fmt.Errorf("reset orphaned waiting runbook steps: %w", err)
			}
			result, err := tx.db.ExecContext(ctx, `UPDATE runbook_runs SET state = 'failed', reason = 'interrupted', updated_at = ? WHERE id = ? AND state = 'running'`, now, id)
			if err != nil {
				return fmt.Errorf("mark runbook run interrupted: %w", err)
			}
			n, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("count interrupted runbook runs: %w", err)
			}
			if n == 1 {
				interrupted = append(interrupted, id)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("interrupt running runbook runs: %w", err)
	}
	return interrupted, nil
}

// ExpireOpenRunbookRuns expires stale manual waits and failed runs, closing
// unfinished steps the way CancelRunbookRun does. It returns the IDs of the runs it transitioned, in
// processing order, so callers can publish an update per run. An empty cutoff
// is a no-op that returns an empty slice.
func (s *Store) ExpireOpenRunbookRuns(ctx context.Context, before string) ([]string, error) {
	expired := make([]string, 0)
	if before == "" {
		return expired, nil
	}
	cutoff, err := normalizeRunbookRunCutoff(before)
	if err != nil {
		return nil, fmt.Errorf("expire open runbook runs: %w", err)
	}
	err = s.WithinTransaction(ctx, func(tx *Store) error {
		ids, err := runbookRunIDs(ctx, tx.db, s.driver == "postgres", `state IN ('waiting_manual','failed') AND updated_at < ?`, cutoff)
		if err != nil {
			return err
		}
		for _, id := range ids {
			run, err := lockRunbookRun(ctx, tx.db, s.driver == "postgres", id)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					continue
				}
				return err
			}
			if (run.State != "waiting_manual" && run.State != "failed") || run.UpdatedAt >= cutoff {
				continue
			}
			now := nextRunbookRunTimestamp(run.UpdatedAt)
			if err := closeUnfinishedRunbookRunSteps(ctx, tx.db, id, now); err != nil {
				return err
			}
			result, err := tx.db.ExecContext(ctx, `UPDATE runbook_runs SET state = 'expired', updated_at = ?, finished_at = ? WHERE id = ? AND state IN ('waiting_manual','failed') AND updated_at < ?`, now, now, id, cutoff)
			if err != nil {
				return fmt.Errorf("expire runbook run: %w", err)
			}
			n, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("count expired runbook runs: %w", err)
			}
			if n == 1 {
				expired = append(expired, id)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("expire open runbook runs: %w", err)
	}
	return expired, nil
}

// PruneRunbookRuns removes terminal history older than before. An empty
// cutoff is a no-op, which lets a zero-day retention setting mean keep all.
func (s *Store) PruneRunbookRuns(ctx context.Context, before string) (int64, error) {
	if before == "" {
		return 0, nil
	}
	cutoff, err := normalizeRunbookRunCutoff(before)
	if err != nil {
		return 0, fmt.Errorf("prune finished runbook runs: %w", err)
	}
	var affected int64
	err = s.WithinTransaction(ctx, func(tx *Store) error {
		ids, err := runbookRunIDs(ctx, tx.db, s.driver == "postgres", `state IN ('succeeded','cancelled','expired') AND finished_at < ?`, cutoff)
		if err != nil {
			return err
		}
		for _, id := range ids {
			run, err := lockRunbookRun(ctx, tx.db, s.driver == "postgres", id)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					continue
				}
				return err
			}
			if !isTerminalRunbookRunState(run.State) || run.FinishedAt == "" || run.FinishedAt >= cutoff {
				continue
			}
			if _, err := tx.db.ExecContext(ctx, `DELETE FROM runbook_run_steps WHERE run_id = ?`, id); err != nil {
				return fmt.Errorf("delete pruned runbook steps: %w", err)
			}
			result, err := tx.db.ExecContext(ctx, `DELETE FROM runbook_runs WHERE id = ? AND state IN ('succeeded','cancelled','expired') AND finished_at < ?`, id, cutoff)
			if err != nil {
				return fmt.Errorf("delete pruned runbook run: %w", err)
			}
			n, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("count pruned runbook runs: %w", err)
			}
			affected += n
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("prune finished runbook runs: %w", err)
	}
	return affected, nil
}

// closeUnfinishedRunbookRunSteps ends the steps of a run that is being
// cancelled or expired. A running step becomes unknown rather than skipped:
// its operation may have reached the connector. Unknown steps are left alone.
func closeUnfinishedRunbookRunSteps(ctx context.Context, db DBTX, runID, now string) error {
	if _, err := db.ExecContext(ctx, `UPDATE runbook_run_steps SET state = 'unknown', finished_at = ? WHERE run_id = ? AND state = 'running'`, now, runID); err != nil {
		return fmt.Errorf("mark in-flight runbook steps unknown: %w", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE runbook_run_steps SET state = 'skipped', finished_at = ? WHERE run_id = ? AND state IN ('pending','waiting')`, now, runID); err != nil {
		return fmt.Errorf("skip unfinished runbook steps: %w", err)
	}
	return nil
}

func activeRunbookRunID(ctx context.Context, db DBTX, runbookID string) (string, error) {
	var id string
	err := db.QueryRowContext(ctx, `SELECT id FROM runbook_runs WHERE runbook_id = ? AND state IN ('running','waiting_manual','failed') ORDER BY started_at DESC, id DESC LIMIT 1`, runbookID).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) activeRunbookRunID(ctx context.Context, runbookID string) (string, error) {
	return activeRunbookRunID(ctx, s.db, runbookID)
}

func scanRunbookRun(row rowScanner) (*RunbookRunRecord, error) {
	var run RunbookRunRecord
	var runbookID, resumedBy, cancelledBy, finishedAt sql.NullString
	err := row.Scan(&run.ID, &runbookID, &run.RunbookTitle, &run.State, &run.Reason, &run.StartedBy, &resumedBy, &cancelledBy, &run.StartedAt, &run.UpdatedAt, &finishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if runbookID.Valid {
		run.RunbookID = &runbookID.String
	}
	if resumedBy.Valid {
		run.ResumedBy = &resumedBy.String
	}
	if cancelledBy.Valid {
		run.CancelledBy = &cancelledBy.String
	}
	if finishedAt.Valid {
		run.FinishedAt = finishedAt.String
	}
	return &run, nil
}

func listRunbookRunSteps(ctx context.Context, db DBTX, runID string) ([]*RunbookRunStepRecord, error) {
	rows, err := db.QueryContext(ctx, `SELECT `+runbookRunStepColumns+` FROM runbook_run_steps WHERE run_id = ? ORDER BY position, id`, runID)
	if err != nil {
		return nil, fmt.Errorf("list frozen runbook steps: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	steps := make([]*RunbookRunStepRecord, 0)
	for rows.Next() {
		step, err := scanRunbookRunStep(rows)
		if err != nil {
			return nil, fmt.Errorf("scan frozen runbook step: %w", err)
		}
		steps = append(steps, step)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate frozen runbook steps: %w", err)
	}
	return steps, nil
}

func scanRunbookRunStep(row rowScanner) (*RunbookRunStepRecord, error) {
	var step RunbookRunStepRecord
	var connectorID, verb, startedAt, finishedAt, confirmedBy sql.NullString
	err := row.Scan(&step.ID, &step.RunID, &step.Position, &step.Kind, &step.Title, &connectorID, &verb, &step.EntityRef, &step.TimeoutSeconds, &step.State, &startedAt, &finishedAt, &step.Error, &confirmedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	step.ConnectorID = connectorID.String
	step.Verb = verb.String
	step.StartedAt = startedAt.String
	step.FinishedAt = finishedAt.String
	step.ConfirmedBy = confirmedBy.String
	return &step, nil
}

func updateRunbookRun(ctx context.Context, db DBTX, id, expectedState string, updates map[string]any, now string) error {
	allowed := []string{"state", "reason", "resumed_by", "cancelled_by", "finished_at"}
	if key, ok := unsupportedRunbookRunField(updates, allowed); ok {
		return fmt.Errorf("unsupported runbook run field %q", key)
	}
	sets := make([]string, 0, len(updates)+1)
	args := make([]any, 0, len(updates)+3)
	for _, key := range allowed {
		value, ok := updates[key]
		if !ok {
			continue
		}
		sets = append(sets, key+" = ?")
		if key == "resumed_by" || key == "cancelled_by" || key == "finished_at" {
			if str, ok := value.(string); ok {
				value = nilToStr(str)
			}
		}
		args = append(args, value)
	}
	if len(sets) == 0 {
		return fmt.Errorf("update runbook run: no supported fields to update")
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, now, id, expectedState)
	result, err := db.ExecContext(ctx, `UPDATE runbook_runs SET `+strings.Join(sets, ", ")+` WHERE id = ? AND state = ?`, args...)
	if err != nil {
		return fmt.Errorf("write runbook run: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated runbook runs: %w", err)
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}

func updateRunbookRunStep(ctx context.Context, db DBTX, runID, stepID, expectedState string, updates map[string]any) error {
	allowed := []string{"state", "started_at", "finished_at", "error", "confirmed_by"}
	if key, ok := unsupportedRunbookRunField(updates, allowed); ok {
		return fmt.Errorf("unsupported runbook run step field %q", key)
	}
	sets := make([]string, 0, len(updates))
	args := make([]any, 0, len(updates)+3)
	for _, key := range allowed {
		value, ok := updates[key]
		if !ok {
			continue
		}
		sets = append(sets, key+" = ?")
		if key == "started_at" || key == "finished_at" || key == "confirmed_by" {
			if str, ok := value.(string); ok {
				value = nilToStr(str)
			}
		}
		args = append(args, value)
	}
	if len(sets) == 0 {
		return fmt.Errorf("update runbook run step: no supported fields to update")
	}
	args = append(args, runID, stepID, expectedState)
	result, err := db.ExecContext(ctx, `UPDATE runbook_run_steps SET `+strings.Join(sets, ", ")+` WHERE run_id = ? AND id = ? AND state = ?`, args...)
	if err != nil {
		return fmt.Errorf("write runbook run step: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated runbook run steps: %w", err)
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}

// unsupportedRunbookRunField returns a key of updates that is not in allowed.
func unsupportedRunbookRunField(updates map[string]any, allowed []string) (string, bool) {
	for key := range updates {
		if !slices.Contains(allowed, key) {
			return key, true
		}
	}
	return "", false
}

func lockRunbookRun(ctx context.Context, db DBTX, postgres bool, id string) (*RunbookRunRecord, error) {
	query := `SELECT ` + runbookRunColumns + ` FROM runbook_runs WHERE id = ?`
	if postgres {
		query += ` FOR UPDATE`
	}
	run, err := scanRunbookRun(db.QueryRowContext(ctx, query, id))
	if err != nil {
		return nil, fmt.Errorf("lock runbook run: %w", err)
	}
	return run, nil
}

func runbookRunIDs(ctx context.Context, db DBTX, postgres bool, predicate string, args ...any) ([]string, error) {
	query := `SELECT id FROM runbook_runs WHERE ` + predicate + ` ORDER BY id`
	if postgres {
		query += ` FOR UPDATE`
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select runbook runs: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan runbook run ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runbook runs: %w", err)
	}
	return ids, nil
}

func isOpenRunbookRunState(state string) bool {
	return state == "running" || state == "waiting_manual" || state == "failed"
}

func isTerminalRunbookRunState(state string) bool {
	return state == "succeeded" || state == "cancelled" || state == "expired"
}

func runbookRunTimestamp(t time.Time) string {
	return t.UTC().Format(runbookRunTimestampLayout)
}

func nilToStrPtr(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nextRunbookRunTimestamp(previous string) string {
	now := time.Now().UTC()
	if previous != "" {
		last, err := time.Parse(time.RFC3339Nano, previous)
		if err == nil && !now.After(last) {
			now = last.Add(time.Nanosecond)
		}
	}
	return runbookRunTimestamp(now)
}

func normalizeRunbookRunCutoff(cutoff string) (string, error) {
	t, err := time.Parse(time.RFC3339Nano, cutoff)
	if err != nil {
		return "", fmt.Errorf("parse runbook retention cutoff: %w", err)
	}
	return runbookRunTimestamp(t), nil
}

func normalizedRunbookRunUpdates(updates map[string]any, now string) (map[string]any, error) {
	normalized := make(map[string]any, len(updates)+2)
	for key, value := range updates {
		normalized[key] = value
	}
	if state, ok := updates["state"].(string); ok {
		switch state {
		case "running":
			if _, ok := normalized["reason"]; !ok {
				normalized["reason"] = ""
			}
			if _, ok := normalized["finished_at"]; !ok {
				normalized["finished_at"] = ""
			}
		case "succeeded":
			// An empty finished_at would store NULL and make the terminal run unprunable.
			if value, ok := normalized["finished_at"]; !ok || value == nil || value == "" {
				normalized["finished_at"] = now
			}
		}
	}
	if value, ok := normalized["finished_at"]; ok {
		canonical, err := canonicalRunbookTimeValue(value)
		if err != nil {
			return nil, fmt.Errorf("normalize runbook run finished_at: %w", err)
		}
		normalized["finished_at"] = canonical
	}
	return normalized, nil
}

func normalizedRunbookRunStepUpdates(updates map[string]any, expectedState, now string) (map[string]any, error) {
	normalized := make(map[string]any, len(updates)+3)
	for key, value := range updates {
		normalized[key] = value
	}
	if state, ok := updates["state"].(string); ok {
		switch state {
		case "running":
			if _, ok := normalized["started_at"]; !ok && (expectedState == "pending" || expectedState == "failed" || expectedState == "unknown") {
				normalized["started_at"] = now
			}
			if _, ok := normalized["finished_at"]; !ok {
				normalized["finished_at"] = ""
			}
			if _, ok := normalized["error"]; !ok {
				normalized["error"] = ""
			}
		case "waiting", "succeeded", "failed", "skipped", "unknown":
			if _, ok := normalized["started_at"]; !ok && state == "waiting" && expectedState == "pending" {
				normalized["started_at"] = now
			}
			if _, ok := normalized["finished_at"]; !ok && state != "waiting" {
				normalized["finished_at"] = now
			}
		}
	}
	for _, key := range []string{"started_at", "finished_at"} {
		if value, ok := normalized[key]; ok {
			canonical, err := canonicalRunbookTimeValue(value)
			if err != nil {
				return nil, fmt.Errorf("normalize runbook step %s: %w", key, err)
			}
			normalized[key] = canonical
		}
	}
	return normalized, nil
}

func canonicalRunbookTimeValue(value any) (any, error) {
	str, ok := value.(string)
	if !ok {
		return value, nil
	}
	if str == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, str)
	if err != nil {
		return nil, err
	}
	return runbookRunTimestamp(t), nil
}
