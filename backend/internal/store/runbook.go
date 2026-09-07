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

// RunbookRecord represents a row in the runbooks table: a read-only binding
// from a change type or alert severity to operator guidance, plus optional
// pointers to a known-good snapshot and a doc. It grants no mutation
// permission over anything — SnapshotID/DocID are inert references only.
type RunbookRecord struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Body        string  `json:"body"`
	TargetType  string  `json:"targetType"`
	TargetValue string  `json:"targetValue"`
	SnapshotID  *string `json:"snapshotId"`
	DocID       *string `json:"docId"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

const runbookColumns = `id, title, body, target_type, target_value, snapshot_id, doc_id, created_at, updated_at`

// CreateRunbook inserts a new runbook.
func (s *Store) CreateRunbook(ctx context.Context, r *RunbookRecord) (*RunbookRecord, error) {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if r.CreatedAt == "" {
		r.CreatedAt = now
	}
	if r.UpdatedAt == "" {
		r.UpdatedAt = now
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO runbooks (id, title, body, target_type, target_value, snapshot_id, doc_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, r.ID, r.Title, r.Body, r.TargetType, r.TargetValue, r.SnapshotID, r.DocID, r.CreatedAt, r.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("create runbook: %w", err)
	}
	return r, nil
}

// GetRunbook retrieves a runbook by ID.
func (s *Store) GetRunbook(ctx context.Context, id string) (*RunbookRecord, error) {
	r, err := scanRunbook(s.db.QueryRowContext(ctx, `SELECT `+runbookColumns+` FROM runbooks WHERE id = ?`, id))
	if err != nil {
		return nil, fmt.Errorf("get runbook: %w", err)
	}
	return r, nil
}

// GetRunbookByTarget retrieves the runbook bound to a change type or alert
// severity, using the unique index on (target_type, target_value).
func (s *Store) GetRunbookByTarget(ctx context.Context, targetType, targetValue string) (*RunbookRecord, error) {
	r, err := scanRunbook(s.db.QueryRowContext(ctx,
		`SELECT `+runbookColumns+` FROM runbooks WHERE target_type = ? AND target_value = ?`,
		targetType, targetValue))
	if err != nil {
		return nil, fmt.Errorf("get runbook by target: %w", err)
	}
	return r, nil
}

// ListRunbooks returns all runbooks, newest first.
func (s *Store) ListRunbooks(ctx context.Context) ([]*RunbookRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+runbookColumns+` FROM runbooks ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list runbooks: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	runbooks := []*RunbookRecord{}
	for rows.Next() {
		r, err := scanRunbook(rows)
		if err != nil {
			return nil, fmt.Errorf("scan runbook: %w", err)
		}
		runbooks = append(runbooks, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runbooks: %w", err)
	}
	return runbooks, nil
}

// UpdateRunbook updates fields on an existing runbook from a partial map of
// column values, then returns the refreshed record.
func (s *Store) UpdateRunbook(ctx context.Context, id string, updates map[string]any) (*RunbookRecord, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	args := []any{now}
	var parts []string

	for k, v := range updates {
		switch k {
		case "title":
			parts = append(parts, "title = ?")
			args = append(args, v)
		case "body":
			parts = append(parts, "body = ?")
			args = append(args, v)
		case "target_type":
			parts = append(parts, "target_type = ?")
			args = append(args, v)
		case "target_value":
			parts = append(parts, "target_value = ?")
			args = append(args, v)
		case "snapshot_id":
			parts = append(parts, "snapshot_id = ?")
			args = append(args, v)
		case "doc_id":
			parts = append(parts, "doc_id = ?")
			args = append(args, v)
		}
	}

	query := "UPDATE runbooks SET updated_at = ?"
	if len(parts) > 0 {
		query += ", " + strings.Join(parts, ", ")
	}
	query += " WHERE id = ?"
	args = append(args, id)

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("update runbook: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return nil, ErrNotFound
	}
	return s.GetRunbook(ctx, id)
}

// DeleteRunbook deletes a runbook by ID.
func (s *Store) DeleteRunbook(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM runbooks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete runbook: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func scanRunbook(row rowScanner) (*RunbookRecord, error) {
	var r RunbookRecord
	var snapshotID, docID sql.NullString
	err := row.Scan(&r.ID, &r.Title, &r.Body, &r.TargetType, &r.TargetValue,
		&snapshotID, &docID, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if snapshotID.Valid {
		r.SnapshotID = &snapshotID.String
	}
	if docID.Valid {
		r.DocID = &docID.String
	}
	return &r, nil
}
