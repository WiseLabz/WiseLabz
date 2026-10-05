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

// QualityFindingRecord represents an actionable documentation quality gap.
type QualityFindingRecord struct {
	ID              string `json:"id"`
	ConnectorID     string `json:"connectorId"`
	DocID           string `json:"docId,omitempty"`
	EntityKind      string `json:"entityKind,omitempty"`
	EntityRef       string `json:"entityRef,omitempty"`
	RuleID          string `json:"ruleId,omitempty"`
	CheckType       string `json:"checkType"`
	Severity        string `json:"severity"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	RemediationLink string `json:"remediationLink"`
	Status          string `json:"status"`
	DetectedCount   int    `json:"detectedCount"`
	FirstDetectedAt string `json:"firstDetectedAt"`
	LastSeenAt      string `json:"lastSeenAt"`
	ResolvedAt      string `json:"resolvedAt,omitempty"`
	// NotifiedSeverity is the severity ("info"/"warning"/"critical") this
	// finding was last notified at, or "" if it has never been notified (or
	// was reset by a resolve). The notification hook compares this against
	// the current Severity to decide whether an escalation deserves a new
	// notification, then calls SetQualityFindingNotifiedSeverity.
	NotifiedSeverity string `json:"-"`
}

const qualityFindingColumns = `id, connector_id, doc_id, rule_id, check_type, severity, title, description,
	remediation_link, status, detected_count, first_detected_at, last_seen_at, resolved_at, notified_severity, entity_kind, entity_ref`

// UpsertQualityFinding atomically inserts a new open finding or records another
// detection of the existing open finding for the same connector and check.
func (s *Store) UpsertQualityFinding(ctx context.Context, f *QualityFindingRecord) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if f.FirstDetectedAt == "" {
		f.FirstDetectedAt = now
	}
	if f.LastSeenAt == "" {
		f.LastSeenAt = now
	}

	var notifiedSeverity sql.NullString
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO quality_findings (id, connector_id, doc_id, rule_id, check_type, severity, title, description,
			remediation_link, status, detected_count, first_detected_at, last_seen_at, resolved_at, notified_severity, entity_kind, entity_ref)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'open', 1, ?, ?, NULL, NULL, ?, ?)
		ON CONFLICT(connector_id, check_type, COALESCE(rule_id, ''), COALESCE(entity_kind, ''), COALESCE(entity_ref, '')) WHERE status = 'open'
		DO UPDATE SET last_seen_at = excluded.last_seen_at,
			detected_count = quality_findings.detected_count + 1,
			doc_id = excluded.doc_id,
			severity = excluded.severity, title = excluded.title,
			description = excluded.description, remediation_link = excluded.remediation_link
		RETURNING id, notified_severity
	`, f.ID, f.ConnectorID, nilToStr(f.DocID), nilToStr(f.RuleID), f.CheckType, f.Severity, f.Title,
		f.Description, f.RemediationLink, f.FirstDetectedAt, f.LastSeenAt, nilToStr(f.EntityKind), nilToStr(f.EntityRef)).Scan(&f.ID, &notifiedSeverity)
	if err != nil {
		return fmt.Errorf("upsert quality finding: %w", err)
	}
	f.NotifiedSeverity = notifiedSeverity.String
	return nil
}

// SetQualityFindingNotifiedSeverity records the severity a finding was just
// notified at, so a later re-detection at the same severity is deduplicated
// while an escalation (or a resolve-then-reopen, which resets this to "")
// still notifies.
func (s *Store) SetQualityFindingNotifiedSeverity(ctx context.Context, id, severity string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE quality_findings SET notified_severity = ? WHERE id = ?`, severity, id)
	if err != nil {
		return fmt.Errorf("set quality finding notified severity: %w", err)
	}
	return nil
}

// OpenNotifiedSeveritiesForRule returns the distinct notified_severity values of
// the open findings of one rule on one connector. A rule notifies once per
// connector, so this is the level already announced to the user.
func (s *Store) OpenNotifiedSeveritiesForRule(ctx context.Context, connectorID, ruleID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT notified_severity FROM quality_findings
		WHERE connector_id = ? AND rule_id = ? AND status = 'open' AND notified_severity IS NOT NULL`, connectorID, ruleID)
	if err != nil {
		return nil, fmt.Errorf("list notified severities for rule: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var sev string
		if err := rows.Scan(&sev); err != nil {
			return nil, fmt.Errorf("scan notified severity: %w", err)
		}
		out = append(out, sev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notified severities: %w", err)
	}
	return out, nil
}

// ResolveQualityFinding resolves the currently open finding, if any, and
// clears notified_severity so a later re-open is treated as a new finding
// for notification purposes.
func (s *Store) ResolveQualityFinding(ctx context.Context, connectorID, checkType string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
		UPDATE quality_findings SET status = 'resolved', resolved_at = ?, notified_severity = NULL
		WHERE connector_id = ? AND check_type = ? AND status = 'open'
	`, now, connectorID, checkType)
	if err != nil {
		return fmt.Errorf("resolve quality finding: %w", err)
	}
	return nil
}

// EntityIdentityRef identifies the member key a finding describes.
type EntityIdentityRef struct{ Kind, Ref string }

// ResolveQualityFindingsForRuleExceptEntities resolves entity-specific findings
// for matches that disappeared from the latest evaluation.
func (s *Store) ResolveQualityFindingsForRuleExceptEntities(ctx context.Context, connectorID, ruleID string, keep []EntityIdentityRef) error {
	keepSet := make(map[string]bool, len(keep))
	for _, ref := range keep {
		keepSet[ref.Kind+"\x00"+ref.Ref] = true
	}
	stale, err := staleQualityFindingIDs(ctx, s, connectorID, ruleID, keepSet)
	if err != nil {
		return err
	}
	if len(stale) == 0 {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	const batch = 500
	for start := 0; start < len(stale); start += batch {
		end := start + batch
		if end > len(stale) {
			end = len(stale)
		}
		placeholders := make([]string, end-start)
		args := []any{now}
		for i, id := range stale[start:end] {
			placeholders[i] = "?"
			args = append(args, id)
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE quality_findings SET status='resolved', resolved_at=?, notified_severity=NULL WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...); err != nil {
			return fmt.Errorf("resolve stale entity findings for rule: %w", err)
		}
	}
	return nil
}

func staleQualityFindingIDs(ctx context.Context, s *Store, connectorID, ruleID string, keep map[string]bool) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, entity_kind, entity_ref FROM quality_findings WHERE connector_id = ? AND rule_id = ? AND status = 'open'`, connectorID, ruleID)
	if err != nil {
		return nil, fmt.Errorf("list entity findings for resolution: %w", err)
	}
	defer func() { _ = rows.Close() }()
	stale := make([]string, 0)
	for rows.Next() {
		var id string
		var kind, ref sql.NullString
		if err := rows.Scan(&id, &kind, &ref); err != nil {
			return nil, fmt.Errorf("scan entity finding for resolution: %w", err)
		}
		if !kind.Valid || !ref.Valid || !keep[kind.String+"\x00"+ref.String] {
			stale = append(stale, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entity findings for resolution: %w", err)
	}
	return stale, nil
}

// ResolveQualityFindingForRule resolves an open compliance finding for one
// connector and rule. Unlike ResolveQualityFinding it cannot affect a
// non-compliance finding with the same check type.
func (s *Store) ResolveQualityFindingForRule(ctx context.Context, connectorID, ruleID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
		UPDATE quality_findings SET status = 'resolved', resolved_at = ?, notified_severity = NULL
		WHERE connector_id = ? AND rule_id = ? AND status = 'open'
	`, now, connectorID, ruleID)
	if err != nil {
		return fmt.Errorf("resolve quality finding for rule: %w", err)
	}
	return nil
}

// ResolveQualityFindingsForRule resolves every open finding created by a
// rule. Call this before disabling or deleting a rule.
func (s *Store) ResolveQualityFindingsForRule(ctx context.Context, ruleID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
		UPDATE quality_findings SET status = 'resolved', resolved_at = ?, notified_severity = NULL
		WHERE rule_id = ? AND status = 'open'
	`, now, ruleID)
	if err != nil {
		return fmt.Errorf("resolve quality findings for rule: %w", err)
	}
	return nil
}

// GetQualityFinding retrieves a quality finding by ID.
func (s *Store) GetQualityFinding(ctx context.Context, id string) (*QualityFindingRecord, error) {
	f, err := scanQualityFinding(s.db.QueryRowContext(ctx,
		`SELECT `+qualityFindingColumns+` FROM quality_findings WHERE id = ?`, id))
	if err != nil {
		return nil, fmt.Errorf("get quality finding: %w", err)
	}
	return &f, nil
}

// ListQualityFindings returns findings newest-seen first with optional filters.
// since, when non-empty, is an RFC3339 cutoff applied to last_seen_at.
func (s *Store) ListQualityFindings(ctx context.Context, connectorID, checkType, status, since string, offset, limit int) ([]QualityFindingRecord, int, error) {
	where := "WHERE 1=1"
	var args []any
	for _, filter := range []struct {
		column string
		value  string
	}{
		{"connector_id", connectorID},
		{"check_type", checkType},
		{"status", status},
	} {
		if filter.value != "" {
			where += " AND " + filter.column + " = ?"
			args = append(args, filter.value)
		}
	}
	if since != "" {
		where += " AND last_seen_at >= ?"
		args = append(args, since)
	}

	return paginatedQuery(ctx, s.db, "quality_findings", qualityFindingColumns, where, args, "last_seen_at DESC", limit, offset, scanQualityFinding)
}

// UpdateQualityFindingStatus changes a finding's status and resolution time.
func (s *Store) UpdateQualityFindingStatus(ctx context.Context, id, status string) error {
	resolvedAt := any(nil)
	if status == "resolved" {
		resolvedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	result, err := s.db.ExecContext(ctx,
		`UPDATE quality_findings SET status = ?, resolved_at = ? WHERE id = ?`, status, resolvedAt, id)
	if err != nil {
		return fmt.Errorf("update quality finding status: %w", err)
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

// CountQualityFindingsOpen returns the number of unresolved findings.
func (s *Store) CountQualityFindingsOpen(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM quality_findings WHERE status = 'open'`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count open quality findings: %w", err)
	}
	return count, nil
}

func scanQualityFinding(row rowScanner) (QualityFindingRecord, error) {
	var f QualityFindingRecord
	var docID, ruleID, resolvedAt, notifiedSeverity, entityKind, entityRef sql.NullString
	err := row.Scan(&f.ID, &f.ConnectorID, &docID, &ruleID, &f.CheckType, &f.Severity, &f.Title,
		&f.Description, &f.RemediationLink, &f.Status, &f.DetectedCount,
		&f.FirstDetectedAt, &f.LastSeenAt, &resolvedAt, &notifiedSeverity, &entityKind, &entityRef)
	if errors.Is(err, sql.ErrNoRows) {
		return QualityFindingRecord{}, ErrNotFound
	}
	if err != nil {
		return QualityFindingRecord{}, err
	}
	f.DocID = docID.String
	f.RuleID = ruleID.String
	f.ResolvedAt = resolvedAt.String
	f.NotifiedSeverity = notifiedSeverity.String
	f.EntityKind = entityKind.String
	f.EntityRef = entityRef.String
	return f, nil
}
