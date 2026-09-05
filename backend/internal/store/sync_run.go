package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SyncRunStatus is the outcome of one sync attempt.
type SyncRunStatus string

// Sync run lifecycle states.
const (
	SyncRunStatusSuccess SyncRunStatus = "success"
	SyncRunStatusError   SyncRunStatus = "error"
	SyncRunStatusSkipped SyncRunStatus = "skipped"
)

// SyncRunRecord represents a row in the sync_runs table: one historical sync
// attempt (manual or scheduled) for a connector.
type SyncRunRecord struct {
	ID          string        `json:"id"`
	ConnectorID string        `json:"connectorId"`
	StartedAt   string        `json:"startedAt"`
	FinishedAt  string        `json:"finishedAt,omitempty"`
	DurationMs  *int          `json:"durationMs,omitempty"`
	Status      SyncRunStatus `json:"status"`
	Error       string        `json:"error,omitempty"`
	// Attempt is the retry attempt number for this run; 1 for the first try.
	Attempt      int `json:"attempt"`
	ChangesCount int `json:"changesCount"`
	AlertsCount  int `json:"alertsCount"`
}

// CreateSyncRun inserts a new sync run row.
func (s *Store) CreateSyncRun(ctx context.Context, r *SyncRunRecord) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	if r.StartedAt == "" {
		r.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if r.Attempt == 0 {
		r.Attempt = 1
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sync_runs (id, connector_id, started_at, finished_at, duration_ms, status, error, attempt, changes_count, alerts_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, r.ID, r.ConnectorID, r.StartedAt, nilToStr(r.FinishedAt), r.DurationMs, r.Status, r.Error, r.Attempt, r.ChangesCount, r.AlertsCount)
	if err != nil {
		return fmt.Errorf("create sync run: %w", err)
	}
	return nil
}

// ListSyncRunsByConnector returns sync runs for a connector, newest first.
// Never returns a nil slice.
func (s *Store) ListSyncRunsByConnector(ctx context.Context, connectorID string, limit int) ([]SyncRunRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, connector_id, started_at, finished_at, duration_ms, status, error, attempt, changes_count, alerts_count
		FROM sync_runs WHERE connector_id = ?
		ORDER BY started_at DESC LIMIT ?
	`, connectorID, limit)
	if err != nil {
		return nil, fmt.Errorf("list sync runs: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	runs := []SyncRunRecord{}
	for rows.Next() {
		r, err := scanSyncRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		runs = append(runs, r)
	}
	return runs, nil
}

func scanSyncRun(row rowScanner) (SyncRunRecord, error) {
	var r SyncRunRecord
	var finishedAt sql.NullString
	var durationMs sql.NullInt64
	err := row.Scan(&r.ID, &r.ConnectorID, &r.StartedAt, &finishedAt, &durationMs, &r.Status, &r.Error, &r.Attempt, &r.ChangesCount, &r.AlertsCount)
	if errors.Is(err, sql.ErrNoRows) {
		return SyncRunRecord{}, ErrNotFound
	}
	if err != nil {
		return SyncRunRecord{}, err
	}
	r.FinishedAt = nullStrToStr(finishedAt)
	r.DurationMs = nullInt64ToIntPtr(durationMs)
	return r, nil
}
