package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ChangeRecord represents a row in the changes table.
type ChangeRecord struct {
	ID             string `json:"id"`
	ServiceID      string `json:"serviceId"`
	ChangeType     string `json:"changeType"`
	Severity       string `json:"severity"`
	Summary        string `json:"summary"`
	Diff           string `json:"diff"`
	Status         string `json:"status"`
	DetectedAt     string `json:"detectedAt"`
	AffectedDocIDs string `json:"affectedDocIds"`
}

// AlertRecord represents a row in the alerts table.
type AlertRecord struct {
	ID           string `json:"id"`
	ChangeID     string `json:"changeId"`
	ServiceID    string `json:"serviceId"`
	Severity     string `json:"severity"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	SnoozedUntil string `json:"snoozedUntil"`
	CreatedAt    string `json:"createdAt"`
}

// --- Change CRUD ---

func (s *Store) CreateChange(ctx context.Context, c *ChangeRecord) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	if c.DetectedAt == "" {
		c.DetectedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if c.Status == "" {
		c.Status = "new"
	}
	if c.Diff == "" {
		c.Diff = "{}"
	}
	if c.AffectedDocIDs == "" {
		c.AffectedDocIDs = "[]"
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO changes (id, service_id, change_type, severity, summary, diff, status, detected_at, affected_doc_ids)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, c.ID, c.ServiceID, c.ChangeType, c.Severity, c.Summary, c.Diff, c.Status, c.DetectedAt, c.AffectedDocIDs)
	if err != nil {
		return fmt.Errorf("create change: %w", err)
	}
	return nil
}

func (s *Store) GetChange(ctx context.Context, id string) (*ChangeRecord, error) {
	c := &ChangeRecord{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, service_id, change_type, severity, summary, diff, status, detected_at, affected_doc_ids
		FROM changes WHERE id = ?
	`, id).Scan(&c.ID, &c.ServiceID, &c.ChangeType, &c.Severity, &c.Summary,
		&c.Diff, &c.Status, &c.DetectedAt, &c.AffectedDocIDs)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get change: %w", err)
	}
	return c, nil
}

func (s *Store) UpdateChangeStatus(ctx context.Context, id, status string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE changes SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("update change status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListChanges(ctx context.Context, serviceID, severity string, offset, limit int) ([]ChangeRecord, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	if serviceID != "" {
		where += " AND service_id = ?"
		args = append(args, serviceID)
	}
	if severity != "" {
		where += " AND severity = ?"
		args = append(args, severity)
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM changes " + where
	s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)

	query := `SELECT id, service_id, change_type, severity, summary, diff, status, detected_at, affected_doc_ids
		FROM changes ` + where + ` ORDER BY detected_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list changes: %w", err)
	}
	defer rows.Close()

	var changes []ChangeRecord
	for rows.Next() {
		var c ChangeRecord
		rows.Scan(&c.ID, &c.ServiceID, &c.ChangeType, &c.Severity, &c.Summary, &c.Diff, &c.Status, &c.DetectedAt, &c.AffectedDocIDs)
		changes = append(changes, c)
	}
	if changes == nil {
		changes = []ChangeRecord{}
	}
	return changes, total, nil
}

func (s *Store) CountChanges(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM changes`).Scan(&count)
	return count, err
}

// CountChangesNew returns count of unacknowledged changes.
func (s *Store) CountChangesNew(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM changes WHERE status = 'new'`).Scan(&count)
	return count, err
}

// --- Alert CRUD ---

func (s *Store) CreateAlert(ctx context.Context, a *AlertRecord) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.CreatedAt == "" {
		a.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if a.Status == "" {
		a.Status = "pending"
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO alerts (id, change_id, service_id, severity, title, description, status, snoozed_until, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, a.ID, nilToStr(a.ChangeID), a.ServiceID, a.Severity, a.Title, a.Description,
		a.Status, nilToStr(a.SnoozedUntil), a.CreatedAt)
	if err != nil {
		return fmt.Errorf("create alert: %w", err)
	}
	return nil
}

func (s *Store) GetAlert(ctx context.Context, id string) (*AlertRecord, error) {
	a := &AlertRecord{}
	var changeID, snoozedUntil sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, change_id, service_id, severity, title, description, status, snoozed_until, created_at
		FROM alerts WHERE id = ?
	`, id).Scan(&a.ID, &changeID, &a.ServiceID, &a.Severity, &a.Title,
		&a.Description, &a.Status, &snoozedUntil, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get alert: %w", err)
	}
	a.ChangeID = changeID.String
	a.SnoozedUntil = snoozedUntil.String
	return a, nil
}

func (s *Store) UpdateAlertStatus(ctx context.Context, id, status, snoozedUntil string) error {
	query := `UPDATE alerts SET status = ?`
	args := []any{status}
	if snoozedUntil != "" {
		query += ", snoozed_until = ?"
		args = append(args, snoozedUntil)
	}
	query += " WHERE id = ?"
	args = append(args, id)

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update alert: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListAlerts(ctx context.Context, serviceID, severity, status string, offset, limit int) ([]AlertRecord, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	if serviceID != "" {
		where += " AND service_id = ?"
		args = append(args, serviceID)
	}
	if severity != "" {
		where += " AND severity = ?"
		args = append(args, severity)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM alerts " + where
	s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)

	query := `SELECT id, change_id, service_id, severity, title, description, status, snoozed_until, created_at
		FROM alerts ` + where + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()

	var alerts []AlertRecord
	for rows.Next() {
		var a AlertRecord
		var changeID, snoozedUntil sql.NullString
		rows.Scan(&a.ID, &changeID, &a.ServiceID, &a.Severity, &a.Title, &a.Description,
			&a.Status, &snoozedUntil, &a.CreatedAt)
		a.ChangeID = changeID.String
		a.SnoozedUntil = snoozedUntil.String
		alerts = append(alerts, a)
	}
	if alerts == nil {
		alerts = []AlertRecord{}
	}
	return alerts, total, nil
}

// GetExpiredSnoozedAlerts returns alerts where snoozed_until has passed.
func (s *Store) GetExpiredSnoozedAlerts(ctx context.Context) ([]AlertRecord, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, change_id, service_id, severity, title, description, status, snoozed_until, created_at
		FROM alerts WHERE status = 'snoozed' AND snoozed_until IS NOT NULL AND snoozed_until <= ?
	`, now)
	if err != nil {
		return nil, fmt.Errorf("get expired snoozed: %w", err)
	}
	defer rows.Close()

	var alerts []AlertRecord
	for rows.Next() {
		var a AlertRecord
		var changeID, snoozedUntil sql.NullString
		rows.Scan(&a.ID, &changeID, &a.ServiceID, &a.Severity, &a.Title, &a.Description,
			&a.Status, &snoozedUntil, &a.CreatedAt)
		a.ChangeID = changeID.String
		a.SnoozedUntil = snoozedUntil.String
		alerts = append(alerts, a)
	}
	if alerts == nil {
		alerts = []AlertRecord{}
	}
	return alerts, nil
}

func (s *Store) CountAlerts(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM alerts`).Scan(&count)
	return count, err
}

// CountAlertsPending returns count of pending alerts.
func (s *Store) CountAlertsPending(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM alerts WHERE status = 'pending'`).Scan(&count)
	return count, err
}

// GetLatestChanges returns the most recent N changes.
func (s *Store) GetLatestChanges(ctx context.Context, n int) ([]ChangeRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, service_id, change_type, severity, summary, diff, status, detected_at, affected_doc_ids
		FROM changes ORDER BY detected_at DESC LIMIT ?
	`, n)
	if err != nil {
		return nil, fmt.Errorf("get latest changes: %w", err)
	}
	defer rows.Close()

	var changes []ChangeRecord
	for rows.Next() {
		var c ChangeRecord
		rows.Scan(&c.ID, &c.ServiceID, &c.ChangeType, &c.Severity, &c.Summary, &c.Diff, &c.Status, &c.DetectedAt, &c.AffectedDocIDs)
		changes = append(changes, c)
	}
	if changes == nil {
		changes = []ChangeRecord{}
	}
	return changes, nil
}

// GetLastSyncTimestamp returns the most recent sync timestamp across all connectors.
func (s *Store) GetLastSyncTimestamp(ctx context.Context) (string, error) {
	var ts sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT MAX(last_sync_at) FROM connectors`).Scan(&ts)
	if err != nil {
		return "", err
	}
	return ts.String, nil
}

// CountConnectorsByStatus counts connectors grouped by status.
func (s *Store) CountConnectorsByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM connectors GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{"online": 0, "degraded": 0, "offline": 0, "unknown": 0}
	for rows.Next() {
		var status string
		var count int
		rows.Scan(&status, &count)
		counts[status] = count
	}
	return counts, nil
}
