package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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

const runbookRunColumns = `id, runbook_id, runbook_title, state, reason, started_by, resumed_by, cancelled_by, started_at, updated_at, finished_at`

const runbookRunStepColumns = `id, run_id, position, kind, title, connector_id, verb, entity_ref, timeout_seconds, state, started_at, finished_at, error, confirmed_by`

// CreateRunbookRun atomically stores a new active run and its frozen steps.
// The runbook title is read inside the transaction, and the supplied steps
// are copied in order without retaining a reference to authored step rows.
func (s *Store) CreateRunbookRun(ctx context.Context, runbookID, startedBy string, steps []*RunbookRunStepRecord) (*RunbookRunRecord, []*RunbookRunStepRecord, error) {
	if len(steps) == 0 || len(steps) > 20 {
		return nil, nil, fmt.Errorf("create runbook run: expected 1 to 20 frozen steps")
	}

	run := &RunbookRunRecord{
		ID:        uuid.New().String(),
		State:     "running",
		StartedBy: startedBy,
	}
	savedSteps := make([]*RunbookRunStepRecord, 0, len(steps))
	for position, step := range steps {
		if step == nil {
			return nil, nil, fmt.Errorf("create runbook run: frozen step %d is nil", position)
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

	if isUniqueViolation(err) {
		if existingID, lookupErr := s.activeRunbookRunID(ctx, runbookID); lookupErr == nil {
			return nil, nil, &RunbookRunConflictError{RunID: existingID}
		}
		return nil, nil, ErrConflict
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
// count. A limit outside the supported range uses the default page size.
func (s *Store) ListRunbookRuns(ctx context.Context, runbookID string, limit, offset int) ([]*RunbookRunRecord, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return paginatedQuery(ctx, s.db, "runbook_runs", runbookRunColumns, "WHERE runbook_id = ?", []any{runbookID}, "started_at DESC, id DESC", limit, offset, scanRunbookRun)
}

// UpdateRunbookRun applies an allowlisted state or actor update only if the
// run remains in expectedState. Every successful update refreshes activity.
func (s *Store) UpdateRunbookRun(ctx context.Context, id, expectedState string, updates map[string]any) (*RunbookRunRecord, error) {
	if len(updates) == 0 {
		return nil, fmt.Errorf("update runbook run: no fields to update")
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
// with step progress.
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

// CancelRunbookRun cancels an open run and atomically skips unfinished work.
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
		if _, err := tx.db.ExecContext(ctx, `UPDATE runbook_run_steps SET state = 'skipped', finished_at = ? WHERE run_id = ? AND state IN ('pending','running','waiting','unknown')`, now, id); err != nil {
			return fmt.Errorf("skip unfinished runbook steps: %w", err)
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
// continuing them and marks their in-flight steps unknown. Manual waits are
// intentionally unchanged.
func (s *Store) InterruptRunningRunbookRuns(ctx context.Context) (int64, error) {
	var affected int64
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
			result, err := tx.db.ExecContext(ctx, `UPDATE runbook_runs SET state = 'failed', reason = 'interrupted', updated_at = ? WHERE id = ? AND state = 'running'`, now, id)
			if err != nil {
				return fmt.Errorf("mark runbook run interrupted: %w", err)
			}
			n, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("count interrupted runbook runs: %w", err)
			}
			affected += n
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("interrupt running runbook runs: %w", err)
	}
	return affected, nil
}

// ExpireOpenRunbookRuns expires stale manual waits and failed runs, skipping
// unfinished steps. An empty cutoff is a no-op.
func (s *Store) ExpireOpenRunbookRuns(ctx context.Context, before string) (int64, error) {
	if before == "" {
		return 0, nil
	}
	cutoff, err := normalizeRunbookRunCutoff(before)
	if err != nil {
		return 0, fmt.Errorf("expire open runbook runs: %w", err)
	}
	var affected int64
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
			if _, err := tx.db.ExecContext(ctx, `UPDATE runbook_run_steps SET state = 'skipped', finished_at = ? WHERE run_id = ? AND state IN ('pending','running','waiting','unknown')`, now, id); err != nil {
				return fmt.Errorf("skip expired runbook steps: %w", err)
			}
			result, err := tx.db.ExecContext(ctx, `UPDATE runbook_runs SET state = 'expired', updated_at = ?, finished_at = ? WHERE id = ? AND state IN ('waiting_manual','failed') AND updated_at < ?`, now, now, id, cutoff)
			if err != nil {
				return fmt.Errorf("expire runbook run: %w", err)
			}
			n, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("count expired runbook runs: %w", err)
			}
			affected += n
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("expire open runbook runs: %w", err)
	}
	return affected, nil
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
	sets := make([]string, 0, len(updates)+1)
	args := make([]any, 0, len(updates)+3)
	for _, key := range []string{"state", "reason", "resumed_by", "cancelled_by", "finished_at"} {
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
	sets := make([]string, 0, len(updates))
	args := make([]any, 0, len(updates)+3)
	for _, key := range []string{"state", "started_at", "finished_at", "error", "confirmed_by"} {
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
		case "succeeded", "cancelled", "expired":
			if _, ok := normalized["finished_at"]; !ok {
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
