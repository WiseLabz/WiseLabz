package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	if err := s.insertAuditRecords(ctx, []AuditRecord{*a}); err != nil {
		return err
	}
	return nil
}

// CreateAuditRecords inserts audit_log rows in one statement.
func (s *Store) CreateAuditRecords(ctx context.Context, records []AuditRecord) error {
	if len(records) == 0 {
		return nil
	}

	for i := range records {
		a := &records[i]
		if a.ID == "" {
			a.ID = uuid.New().String()
		}
		if a.CreatedAt == "" {
			a.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		}
		if a.Detail == "" {
			a.Detail = "{}"
		}
	}
	return s.insertAuditRecords(ctx, records)
}

func (s *Store) insertAuditRecords(ctx context.Context, records []AuditRecord) error {
	args := make([]any, 0, len(records)*8)
	values := make([]string, 0, len(records))
	for _, a := range records {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args, a.ID, a.ActorUserID, a.ActorRole, a.Action, a.TargetType, a.TargetID, a.Detail, a.CreatedAt)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_log (id, actor_user_id, actor_role, action, target_type, target_id, detail, created_at)
		VALUES `+strings.Join(values, ", "), args...)
	if err != nil {
		return fmt.Errorf("create audit records: %w", err)
	}
	return nil
}

// RecordAuditBatchFromContext records one audit entry for each target using the
// authenticated actor in ctx.
func (s *Store) RecordAuditBatchFromContext(ctx context.Context, action, targetType string, records []AuditRecord) error {
	for i := range records {
		actorUserID := auth.UserIDFromContext(ctx)
		records[i].ActorUserID = actorUserID
		records[i].ActorRole = actorRoleLabel(actorUserID, auth.InstanceAdminFromContext(ctx))
		records[i].Action = action
		records[i].TargetType = targetType
	}
	return s.CreateAuditRecords(ctx, records)
}

// RecordAuditFromContext is the one-line call handlers make after an audited
// action succeeds: it marshals detail to JSON (nil -> "{}") and pulls the
// actor from the request context (set by auth.AuthMiddleware).
//
// Called by handlers for audited actions; most call sites record only after
// success, with security-relevant elevation denials as the documented
// exception. Call sites treat a returned error as non-fatal (slog.Error and
// continue); see docs/AUDIT.md.
func (s *Store) RecordAuditFromContext(ctx context.Context, action, targetType, targetID string, detail any) error {
	return s.RecordAuditAs(
		ctx,
		auth.UserIDFromContext(ctx),
		auth.InstanceAdminFromContext(ctx),
		action,
		targetType,
		targetID,
		detail,
	)
}

// RecordAuditAs records one audit entry using the explicitly resolved actor.
// Actor attribution and role do not depend on auth values or restrictions in
// ctx, allowing callers that resolve an actor before entering a core operation
// to preserve that identity in the audit trail.
func (s *Store) RecordAuditAs(
	ctx context.Context,
	actorUserID string,
	instanceAdmin bool,
	action string,
	targetType string,
	targetID string,
	detail any,
) error {
	detailJSON := ""
	if detail != nil {
		data, err := json.Marshal(detail)
		if err != nil {
			return fmt.Errorf("marshal audit detail: %w", err)
		}
		detailJSON = string(data)
	}

	return s.CreateAuditRecord(ctx, &AuditRecord{
		ActorUserID: actorUserID,
		ActorRole:   actorRoleLabel(actorUserID, instanceAdmin),
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		Detail:      detailJSON,
	})
}

// actorRoleLabel records the actor's flat instance-admin role for the audit
// trail ("admin"/"user"). Per-connector grant changes are self-describing
// via the audit action/target/detail, so this stays the coarse instance-wide
// label rather than trying to cram a connector-scoped role into one column.
// Empty when there's no actor (mirrors ActorUserID), even if instanceAdmin is true.
func actorRoleLabel(userID string, instanceAdmin bool) string {
	if userID == "" {
		return ""
	}
	if instanceAdmin {
		return "admin"
	}
	return "user"
}

// ListAuditRecords returns a paginated list of audit records, newest first,
// optionally filtered by action, target type, and/or a created_at range
// (createdAfter/createdBefore are RFC3339 timestamps; empty = unbounded).
func (s *Store) ListAuditRecords(ctx context.Context, action, targetType, createdAfter, createdBefore string, offset, limit int) ([]AuditRecord, int, error) {
	where, args := auditFilterClause(action, targetType, createdAfter, createdBefore)

	return paginatedQuery(ctx, s.db, "audit_log", auditColumns, where, args, "created_at DESC", limit, offset, scanAuditRecord)
}

// ListAuditRecordsKeyset returns one keyset (cursor) page of audit records,
// newest first, using the same filters as ListAuditRecords. Rows strictly
// before cur in (created_at, id) order are returned; a zero cur starts at the
// newest record. total counts every record matching the filters, ignoring cur.
func (s *Store) ListAuditRecordsKeyset(ctx context.Context, action, targetType, createdAfter, createdBefore string, cur Keyset, limit int) ([]AuditRecord, int, error) {
	where, args := auditFilterClause(action, targetType, createdAfter, createdBefore)

	return keysetQuery(ctx, s.db, "audit_log", auditColumns, where, args, "created_at", cur, limit, scanAuditRecord)
}

// auditExportPageSize is the keyset page size EachAuditRecord reads per query.
const auditExportPageSize = 1000

// EachAuditRecord calls fn for every audit record matching the given filters,
// newest first, until fn returns an error. Used by the export endpoint, which
// needs the full matching set: rows are read one keyset page at a time so
// memory stays bounded and no query holds the (single, on SQLite) connection
// while fn writes to a slow client.
func (s *Store) EachAuditRecord(ctx context.Context, action, targetType, createdAfter, createdBefore string, fn func(AuditRecord) error) error {
	where, args := auditFilterClause(action, targetType, createdAfter, createdBefore)

	var cur Keyset
	for {
		pageWhere, pageArgs := where, args
		if !cur.Empty() {
			pageWhere += " AND (created_at, id) < (?, ?)"
			pageArgs = append(append([]any{}, args...), cur.Sort, cur.ID)
		}
		query := `SELECT ` + auditColumns + ` FROM audit_log ` + pageWhere +
			` ORDER BY created_at DESC, id DESC LIMIT ?`
		page, err := scanAll(ctx, s.db, "audit_log", query, append(pageArgs, auditExportPageSize), scanAuditRecord)
		if err != nil {
			return fmt.Errorf("export audit records: %w", err)
		}
		for _, a := range page {
			if err := fn(a); err != nil {
				return err
			}
		}
		if len(page) < auditExportPageSize {
			return nil
		}
		last := page[len(page)-1]
		cur = Keyset{Sort: last.CreatedAt, ID: last.ID}
	}
}

// auditFilterClause builds the shared WHERE clause + args for
// ListAuditRecords and EachAuditRecord.
func auditFilterClause(action, targetType, createdAfter, createdBefore string) (string, []any) {
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
	if createdAfter != "" {
		where += " AND created_at >= ?"
		args = append(args, createdAfter)
	}
	if createdBefore != "" {
		where += " AND created_at <= ?"
		args = append(args, createdBefore)
	}
	return where, args
}

// auditColumns is the shared column list for every audit_log SELECT.
const auditColumns = `id, actor_user_id, actor_role, action, target_type, target_id, detail, created_at`

func scanAuditRecord(row rowScanner) (AuditRecord, error) {
	var a AuditRecord
	err := row.Scan(&a.ID, &a.ActorUserID, &a.ActorRole, &a.Action, &a.TargetType, &a.TargetID, &a.Detail, &a.CreatedAt)
	return a, err
}
