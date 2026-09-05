package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/WiseLabz/wiselabz/internal/auth"
)

// AuditRecord represents a row in the audit_log table: who did what to which
// target, and when. See docs/AUDIT.md for exactly which actions are covered.
type AuditRecord struct {
	ID          string `json:"id"`
	ActorUserID string `json:"actorUserId"`
	ActorRole   string `json:"actorRole"`
	Action      string `json:"action"`
	TargetType  string `json:"targetType"`
	TargetID    string `json:"targetId"`
	Detail      string `json:"detail"`
	CreatedAt   string `json:"createdAt"`
}

// CreateAuditRecord inserts an audit_log row, filling ID/CreatedAt/Detail
// defaults when left zero-valued (same convention as CreateChange/CreateAlert).
func (s *Store) CreateAuditRecord(ctx context.Context, a *AuditRecord) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.CreatedAt == "" {
		a.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if a.Detail == "" {
		a.Detail = "{}"
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_log (id, actor_user_id, actor_role, action, target_type, target_id, detail, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, a.ID, a.ActorUserID, a.ActorRole, a.Action, a.TargetType, a.TargetID, a.Detail, a.CreatedAt)
	if err != nil {
		return fmt.Errorf("create audit record: %w", err)
	}
	return nil
}

// RecordAuditFromContext is the one-line call handlers make after an audited
// action succeeds: it marshals detail to JSON (nil -> "{}") and pulls the
// actor from the request context (set by auth.AuthMiddleware).
//
// Called only on success — a failed attempt isn't accountability-worthy the
// way a completed one is. Call sites treat a returned error as non-fatal
// (slog.Error and continue) since the audited action has already gone
// through; see docs/AUDIT.md.
func (s *Store) RecordAuditFromContext(ctx context.Context, action, targetType, targetID string, detail any) error {
	detailJSON := ""
	if detail != nil {
		data, err := json.Marshal(detail)
		if err != nil {
			return fmt.Errorf("marshal audit detail: %w", err)
		}
		detailJSON = string(data)
	}

	return s.CreateAuditRecord(ctx, &AuditRecord{
		ActorUserID: auth.UserIDFromContext(ctx),
		ActorRole:   auth.RoleFromContext(ctx),
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		Detail:      detailJSON,
	})
}

// ListAuditRecords returns a paginated list of audit records, newest first,
// optionally filtered by action and/or target type.
func (s *Store) ListAuditRecords(ctx context.Context, action, targetType string, offset, limit int) ([]AuditRecord, int, error) {
	where := "WHERE 1=1"
	var args []any
	if action != "" {
		where += " AND action = ?"
		args = append(args, action)
	}
	if targetType != "" {
		where += " AND target_type = ?"
		args = append(args, targetType)
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM audit_log " + where
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit records: %w", err)
	}

	query := `SELECT id, actor_user_id, actor_role, action, target_type, target_id, detail, created_at
		FROM audit_log ` + where + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit records: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var records []AuditRecord
	for rows.Next() {
		var a AuditRecord
		if err := rows.Scan(&a.ID, &a.ActorUserID, &a.ActorRole, &a.Action, &a.TargetType, &a.TargetID, &a.Detail, &a.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}
		records = append(records, a)
	}
	if records == nil {
		records = []AuditRecord{}
	}
	return records, total, nil
}
