package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ConnectorRecord represents a row in the connectors table.
type ConnectorRecord struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Category      string `json:"category"`
	Type          string `json:"type"`
	URL           string `json:"url"`
	VerifyTLS     bool   `json:"verifyTls"`
	ConfigData    string `json:"configData"`
	Enabled       bool   `json:"enabled"`
	Status        string `json:"status"`
	StatusMessage string `json:"statusMessage"`
	LastSyncAt    string `json:"lastSyncAt"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

// SnapshotRecord represents a row in the service_snapshots table.
type SnapshotRecord struct {
	ID          string `json:"id"`
	ConnectorID string `json:"connectorId"`
	Data        string `json:"data"`
	FetchedAt   string `json:"fetchedAt"`
}

// --- Connector CRUD ---

// CreateConnector inserts a new connector.
func (s *Store) CreateConnector(ctx context.Context, c *ConnectorRecord) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if c.CreatedAt == "" {
		c.CreatedAt = now
	}
	if c.UpdatedAt == "" {
		c.UpdatedAt = now
	}
	if c.Status == "" {
		c.Status = "unknown"
	}
	if c.ConfigData == "" {
		c.ConfigData = "{}"
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO connectors (id, name, category, type, url, verify_tls, config_data, enabled, status, status_message, last_sync_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, c.ID, c.Name, c.Category, c.Type, c.URL, boolToInt(c.VerifyTLS), c.ConfigData,
		boolToInt(c.Enabled), c.Status, c.StatusMessage, nilToStr(c.LastSyncAt),
		c.CreatedAt, c.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("create connector: %w", err)
	}
	return nil
}

// GetConnector retrieves a connector by ID.
func (s *Store) GetConnector(ctx context.Context, id string) (*ConnectorRecord, error) {
	c := &ConnectorRecord{}
	var verifyTLS, enabled int
	var lastSyncAt sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, category, type, url, verify_tls, config_data, enabled, status, status_message, last_sync_at, created_at, updated_at
		FROM connectors WHERE id = ?
	`, id).Scan(&c.ID, &c.Name, &c.Category, &c.Type, &c.URL, &verifyTLS, &c.ConfigData,
		&enabled, &c.Status, &c.StatusMessage, &lastSyncAt, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get connector: %w", err)
	}
	c.VerifyTLS = verifyTLS != 0
	c.Enabled = enabled != 0
	c.LastSyncAt = nullStrToStr(lastSyncAt)
	return c, nil
}

// UpdateConnector updates fields on an existing connector.
func (s *Store) UpdateConnector(ctx context.Context, id string, updates map[string]any) error {
	now := time.Now().UTC().Format(time.RFC3339)
	args := []any{now}
	var parts []string

	for k, v := range updates {
		switch k {
		case "name":
			parts = append(parts, "name = ?")
			args = append(args, v)
		case "url":
			parts = append(parts, "url = ?")
			args = append(args, v)
		case "verify_tls":
			parts = append(parts, "verify_tls = ?")
			args = append(args, boolToInt(v.(bool)))
		case "config_data":
			parts = append(parts, "config_data = ?")
			args = append(args, v)
		case "enabled":
			parts = append(parts, "enabled = ?")
			args = append(args, boolToInt(v.(bool)))
		case "status":
			parts = append(parts, "status = ?")
			args = append(args, v)
		case "status_message":
			parts = append(parts, "status_message = ?")
			args = append(args, v)
		case "last_sync_at":
			parts = append(parts, "last_sync_at = ?")
			args = append(args, v)
		case "category":
			parts = append(parts, "category = ?")
			args = append(args, v)
		case "type":
			parts = append(parts, "type = ?")
			args = append(args, v)
		}
	}

	query := "UPDATE connectors SET updated_at = ?"
	if len(parts) > 0 {
		query += ", " + strings.Join(parts, ", ")
	}
	query += " WHERE id = ?"
	args = append(args, id)

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update connector: %w", err)
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

// DeleteConnector deletes a connector by ID.
func (s *Store) DeleteConnector(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM connectors WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete connector: %w", err)
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

// ListConnectors returns a paginated list of connectors.
func (s *Store) ListConnectors(ctx context.Context, offset, limit int) ([]ConnectorRecord, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM connectors`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count connectors: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, category, type, url, verify_tls, config_data, enabled, status, status_message, last_sync_at, created_at, updated_at
		FROM connectors ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list connectors: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	return scanConnectors(rows)
}

// ListConnectorsByCategory returns connectors filtered by category.
func (s *Store) ListConnectorsByCategory(ctx context.Context, category string, offset, limit int) ([]ConnectorRecord, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM connectors WHERE category = ?`, category).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count connectors by category: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, category, type, url, verify_tls, config_data, enabled, status, status_message, last_sync_at, created_at, updated_at
		FROM connectors WHERE category = ? ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, category, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list connectors by category: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	return scanConnectors(rows)
}

// ListAllConnectors returns all connectors (no pagination).
func (s *Store) ListAllConnectors(ctx context.Context) ([]ConnectorRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, category, type, url, verify_tls, config_data, enabled, status, status_message, last_sync_at, created_at, updated_at
		FROM connectors ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list all connectors: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	connectors, _, err := scanConnectors(rows)
	return connectors, err
}

// --- Snapshot operations ---

// CreateSnapshot inserts a new service snapshot.
func (s *Store) CreateSnapshot(ctx context.Context, sn *SnapshotRecord) error {
	if sn.ID == "" {
		sn.ID = uuid.New().String()
	}
	if sn.FetchedAt == "" {
		sn.FetchedAt = time.Now().UTC().Format(time.RFC3339)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO service_snapshots (id, connector_id, data, fetched_at)
		VALUES (?, ?, ?, ?)
	`, sn.ID, sn.ConnectorID, sn.Data, sn.FetchedAt)
	if err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}
	return nil
}

// GetLatestSnapshot returns the most recent snapshot for a connector.
func (s *Store) GetLatestSnapshot(ctx context.Context, connectorID string) (*SnapshotRecord, error) {
	sn := &SnapshotRecord{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, connector_id, data, fetched_at
		FROM service_snapshots WHERE connector_id = ?
		ORDER BY fetched_at DESC LIMIT 1
	`, connectorID).Scan(&sn.ID, &sn.ConnectorID, &sn.Data, &sn.FetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get latest snapshot: %w", err)
	}
	return sn, nil
}

// GetSnapshotsByConnector returns snapshots for a connector (newest first).
func (s *Store) GetSnapshotsByConnector(ctx context.Context, connectorID string, limit int) ([]SnapshotRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, connector_id, data, fetched_at
		FROM service_snapshots WHERE connector_id = ?
		ORDER BY fetched_at DESC LIMIT ?
	`, connectorID, limit)
	if err != nil {
		return nil, fmt.Errorf("get snapshots: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var snapshots []SnapshotRecord
	for rows.Next() {
		var sn SnapshotRecord
		if err := rows.Scan(&sn.ID, &sn.ConnectorID, &sn.Data, &sn.FetchedAt); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		snapshots = append(snapshots, sn)
	}
	if snapshots == nil {
		snapshots = []SnapshotRecord{}
	}
	return snapshots, nil
}

// CountSnapshotsByConnector returns the number of snapshots for a connector.
func (s *Store) CountSnapshotsByConnector(ctx context.Context, connectorID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM service_snapshots WHERE connector_id = ?`, connectorID).Scan(&count)
	return count, err
}

// CountDocsByConnector returns the number of docs linked to a connector.
func (s *Store) CountDocsByConnector(ctx context.Context, connectorID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM docs WHERE service_id = ?`, connectorID).Scan(&count)
	return count, err
}

// CountChangesByConnector returns the number of changes for a connector.
func (s *Store) CountChangesByConnector(ctx context.Context, connectorID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM changes WHERE service_id = ?`, connectorID).Scan(&count)
	return count, err
}

// CountAlertsByConnector returns the number of alerts for a connector.
func (s *Store) CountAlertsByConnector(ctx context.Context, connectorID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM alerts WHERE service_id = ?`, connectorID).Scan(&count)
	return count, err
}

// --- helpers ---

func scanConnectors(rows *sql.Rows) ([]ConnectorRecord, int, error) {
	var connectors []ConnectorRecord
	for rows.Next() {
		var c ConnectorRecord
		var verifyTLS, enabled int
		var lastSyncAt sql.NullString
		if err := rows.Scan(&c.ID, &c.Name, &c.Category, &c.Type, &c.URL, &verifyTLS, &c.ConfigData,
			&enabled, &c.Status, &c.StatusMessage, &lastSyncAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan connector: %w", err)
		}
		c.VerifyTLS = verifyTLS != 0
		c.Enabled = enabled != 0
		c.LastSyncAt = nullStrToStr(lastSyncAt)
		connectors = append(connectors, c)
	}
	if connectors == nil {
		connectors = []ConnectorRecord{}
	}
	return connectors, len(connectors), nil
}

func nullStrToStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func nilToStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ParseConnectorConfig parses the config_data JSON string into a map.
func ParseConnectorConfig(data string) (map[string]any, error) {
	var cfg map[string]any
	if err := json.Unmarshal([]byte(data), &cfg); err != nil {
		return nil, fmt.Errorf("parse connector config: %w", err)
	}
	return cfg, nil
}

// MarshalConnectorConfig marshals a config map to a JSON string.
func MarshalConnectorConfig(cfg map[string]any) (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal connector config: %w", err)
	}
	return string(b), nil
}
