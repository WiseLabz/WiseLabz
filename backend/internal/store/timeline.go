package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/auth"
)

// TimelineItem carries source links and manual entry context in one row shape.
type TimelineItem struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Timestamp   string `json:"timestamp"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	ConnectorID string `json:"connectorId"`
	DocID       string `json:"docId"`
	CreatedBy   string `json:"createdBy"`
	EntityKind  string `json:"entityKind"`
	EntityName  string `json:"entityName"`
	EntityRef   string `json:"entityRef"`
	Status      string `json:"status"`
}

// TimelineCursor identifies the last returned event in descending order.
type TimelineCursor struct{ Timestamp, Kind, ID string }

// TimelineFilter applies visibility and user-selected filters before paging.
type TimelineFilter struct {
	UserID      string
	Admin       bool
	ConnectorID string
	After       string
	Before      string
	Kinds       []string
	AllSyncRuns bool
	Cursor      TimelineCursor
}

// Explicit actions keep future security-related audit additions out of Journal.
var timelineLabActions = []string{
	"connector.create", "connector.update", "connector.delete", "connector.toggle_enabled",
	"connector.sync", "connector.sync_all", "connector.restart", "connector.start", "connector.stop",
	"connector.configPush", "connector.action", "backup.import",
	"runbook.run.step_resent", "runbook.run.step_marked_done",
	"connector.maintenanceWindow.open", "connector.maintenanceWindow.close",
	"connector.bulk_sync", "connector.bulk_reauth", "connector.bulk_restart",
	"change.ack", "change.dismiss", "change.bulk_ack", "change.bulk_dismiss",
	"alert.resolve", "alert.dismiss", "alert.snooze",
	"doc.restore", "doc.edit_proposed", "doc.edit_approved", "doc.edit_rejected",
	"runbook.create", "runbook.update", "runbook.delete",
}

// timelineTimestamp pads UTC timestamps to nanoseconds without rounding away
// within-second order. Store writers use UTC RFC3339/RFC3339Nano throughout.
func timelineTimestamp(column string) string {
	return `substr(` + column + `, 1, 19) || '.' || substr((CASE WHEN substr(` + column + `, 20, 1) = '.'
 THEN substr(` + column + `, 21, length(` + column + `) - 21) ELSE '' END) || '000000000', 1, 9) || 'Z'`
}

func timelineScope(ctx context.Context, column, userID string, labWide bool) (string, []any) {
	where := `EXISTS (SELECT 1 FROM user_connector_roles g WHERE g.connector_id = ` + column +
		` AND g.user_id = ? AND g.role IN ('viewer', 'operator'))`
	if labWide {
		where = `(` + column + ` IS NULL OR ` + column + ` = '' OR ` + where + `)`
	}
	keyFilter, keyArgs := apiKeyConnectorFilter(ctx, column)
	// Restricted keys cannot read unscoped data via a broader owner grant.
	return " AND " + where + keyFilter, append([]any{userID}, keyArgs...)
}

func (s *Store) timelineUnion(ctx context.Context, f TimelineFilter) (string, []any) {
	var branches []string
	var args []any
	add := func(query, column string, labWide bool) {
		scope, scopeArgs := timelineScope(ctx, column, f.UserID, labWide)
		branches = append(branches, query+scope)
		args = append(args, scopeArgs...)
	}
	add(`SELECT c.id, 'change' AS kind, `+timelineTimestamp("c.detected_at")+` AS timestamp,
 c.summary AS title, '' AS body, c.service_id AS connector_id, '' AS doc_id,
 '' AS created_by, '' AS entity_kind, '' AS entity_name, '' AS entity_ref, c.status
 FROM changes c WHERE 1=1`, "c.service_id", false)
	syncWhere := ""
	if !f.AllSyncRuns {
		syncWhere = " AND (r.status = 'error' OR r.changes_count > 0 OR r.alerts_count > 0)"
	}
	add(`SELECT r.id, 'sync', `+timelineTimestamp("r.started_at")+`, r.status, r.error,
 r.connector_id, '', '', '', '', '', r.status FROM sync_runs r WHERE 1=1`+syncWhere, "r.connector_id", false)
	add(`SELECT a.id, 'alert', `+timelineTimestamp("a.created_at")+`, a.title, a.description,
 a.service_id, '', '', '', '', '', a.status FROM alerts a WHERE 1=1`, "a.service_id", false)
	docWhere := ""
	if !f.Admin {
		docWhere = " AND (d.service_id IS NOT NULL OR d.origin = 'human')"
	}
	add(`SELECT v.id, 'doc', `+timelineTimestamp("v.created_at")+`, d.title, '',
 COALESCE(d.service_id, ''), d.id, COALESCE(v.author, ''), '', '', '', v.trigger
 FROM doc_versions v JOIN docs d ON d.id = v.doc_id WHERE d.deleted_at IS NULL`+docWhere,
		"d.service_id", true)
	add(`SELECT j.id, 'journal', `+timelineTimestamp("j.occurred_at")+`, '', j.body,
 COALESCE(j.connector_id, ''), COALESCE(j.doc_id, ''), j.created_by,
 j.entity_kind, j.entity_name, j.entity_ref, '' FROM journal_entries j WHERE 1=1`, "j.connector_id", true)
	actions := make([]any, len(timelineLabActions))
	for i, action := range timelineLabActions {
		actions[i] = action
	}
	connector := `(SELECT CASE WHEN COUNT(*) = 1 THEN MIN(sc.connector_id) ELSE '' END
 FROM audit_log_connectors sc WHERE sc.audit_id = a.id)`
	if f.ConnectorID != "" {
		connector = `COALESCE((SELECT sc.connector_id FROM audit_log_connectors sc
 WHERE sc.audit_id = a.id AND sc.connector_id = ?), '')`
		args = append(args, f.ConnectorID)
	}
	args = append(args, actions...)
	admin := f.Admin && auth.InstanceAdminFromContext(ctx)
	docID := `CASE WHEN a.target_type = 'doc' THEN a.target_id ELSE '' END`
	if !admin {
		// Members get the doc link only while the doc is live and still in the row's scope.
		docID = `CASE WHEN a.target_type = 'doc' AND EXISTS (SELECT 1 FROM docs d
 JOIN audit_log_connectors dsc ON dsc.audit_id = a.id AND dsc.connector_id = d.service_id
 WHERE d.id = a.target_id AND d.deleted_at IS NULL) THEN a.target_id ELSE '' END`
	}
	scope, scopeArgs := timelineAuditScope(ctx, f)
	branches = append(branches, `SELECT a.id, 'audit', `+timelineTimestamp("a.created_at")+`, a.action, '',
 `+connector+`, `+docID+`,
 a.actor_user_id, '', '', '', a.target_type FROM audit_log a
 WHERE a.action IN (`+placeholders(len(actions))+`)`+scope)
	args = append(args, scopeArgs...)

	return strings.Join(branches, " UNION ALL "), args
}

// timelineAuditScope requires grants on every snapshotted connector. Only
// instance admins may read empty scopes and ignore connectors that were deleted.
func timelineAuditScope(ctx context.Context, f TimelineFilter) (string, []any) {
	admin := f.Admin && auth.InstanceAdminFromContext(ctx)
	where := ""
	if !admin {
		where = ` AND EXISTS (SELECT 1 FROM audit_log_connectors sc WHERE sc.audit_id = a.id)`
	}
	denied := `NOT EXISTS (SELECT 1 FROM user_connector_roles g
 WHERE g.connector_id = sc.connector_id AND g.user_id = ? AND g.role IN ('viewer', 'operator'))`
	args := []any{f.UserID}
	ids := auth.APIKeyRestrictionFromContext(ctx).ConnectorIDs
	if len(ids) > 0 {
		// A restricted key cannot read an empty scope, even for an admin owner.
		if admin {
			where += ` AND EXISTS (SELECT 1 FROM audit_log_connectors sc WHERE sc.audit_id = a.id)`
		}
		denied += ` OR sc.connector_id NOT IN (` + placeholders(len(ids)) + `)`
		for _, id := range ids {
			args = append(args, id)
		}
	}
	live := ""
	if admin && len(ids) == 0 {
		live = ` AND EXISTS (SELECT 1 FROM connectors c WHERE c.id = sc.connector_id)`
	}
	return where + ` AND NOT EXISTS (SELECT 1 FROM audit_log_connectors sc
 WHERE sc.audit_id = a.id` + live + ` AND (` + denied + `))`, args
}

// ListTimeline filters every source before counting and keyset pagination.
func (s *Store) ListTimeline(ctx context.Context, f TimelineFilter, limit int) ([]TimelineItem, int, bool, error) {
	union, args := s.timelineUnion(ctx, f)
	where := " WHERE 1=1"
	if f.ConnectorID != "" {
		where += " AND connector_id = ?"
		args = append(args, f.ConnectorID)
	}
	if f.After != "" {
		where += " AND timestamp >= ?"
		args = append(args, f.After)
	}
	if f.Before != "" {
		where += " AND timestamp <= ?"
		args = append(args, f.Before)
	}
	if len(f.Kinds) > 0 {
		where += " AND kind IN (" + placeholders(len(f.Kinds)) + ")"
		for _, kind := range f.Kinds {
			args = append(args, kind)
		}
	}
	query := ` FROM (` + union + `) timeline` + where
	var total int
	if err := s.reader().QueryRowContext(ctx, "SELECT COUNT(*)"+query, args...).Scan(&total); err != nil {
		return nil, 0, false, fmt.Errorf("count timeline: %w", err)
	}
	if f.Cursor.ID != "" {
		query += " AND (timestamp, kind, id) < (?, ?, ?)"
		args = append(args, f.Cursor.Timestamp, f.Cursor.Kind, f.Cursor.ID)
	}
	query = "SELECT *" + query + " ORDER BY timestamp DESC, kind DESC, id DESC LIMIT ?"
	args = append(args, limit+1)
	items, err := scanAll(ctx, s.reader(), "timeline", query, args, func(row rowScanner) (TimelineItem, error) {
		var it TimelineItem
		err := row.Scan(&it.ID, &it.Kind, &it.Timestamp, &it.Title, &it.Body, &it.ConnectorID, &it.DocID,
			&it.CreatedBy, &it.EntityKind, &it.EntityName, &it.EntityRef, &it.Status)
		return it, err
	})
	if err != nil {
		return nil, 0, false, err
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	return items, total, more, nil
}
