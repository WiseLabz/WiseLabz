package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// JournalEntry is durable human context; author IDs deliberately have no FK.
type JournalEntry struct {
	ID          string `json:"id"`
	Body        string `json:"body"`
	OccurredAt  string `json:"occurredAt"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	CreatedBy   string `json:"createdBy"`
	ConnectorID string `json:"connectorId"`
	DocID       string `json:"docId"`
	EntityKind  string `json:"entityKind"`
	EntityName  string `json:"entityName"`
	EntityRef   string `json:"entityRef"`
}

const journalColumns = `id, body, occurred_at, created_at, updated_at, created_by,
 COALESCE(connector_id, ''), COALESCE(doc_id, ''), entity_kind, entity_name, entity_ref`

// CreateJournalEntry inserts a note, defaulting its ID and timestamps.
func (s *Store) CreateJournalEntry(ctx context.Context, e *JournalEntry) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if e.OccurredAt == "" {
		e.OccurredAt = now
	}
	if e.CreatedAt == "" {
		e.CreatedAt = now
	}
	if e.UpdatedAt == "" {
		e.UpdatedAt = e.CreatedAt
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO journal_entries
 (id, body, occurred_at, created_at, updated_at, created_by, connector_id, doc_id, entity_kind, entity_name, entity_ref)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Body, e.OccurredAt, e.CreatedAt, e.UpdatedAt, e.CreatedBy,
		nilToStr(e.ConnectorID), nilToStr(e.DocID), e.EntityKind, e.EntityName, e.EntityRef)
	if err != nil {
		return fmt.Errorf("create journal entry: %w", err)
	}
	return nil
}

func scanJournalEntry(row rowScanner) (JournalEntry, error) {
	var e JournalEntry
	err := row.Scan(&e.ID, &e.Body, &e.OccurredAt, &e.CreatedAt, &e.UpdatedAt, &e.CreatedBy,
		&e.ConnectorID, &e.DocID, &e.EntityKind, &e.EntityName, &e.EntityRef)
	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrNotFound
	}
	return e, err
}

// GetJournalEntry loads a note by ID.
func (s *Store) GetJournalEntry(ctx context.Context, id string) (JournalEntry, error) {
	e, err := scanJournalEntry(s.reader().QueryRowContext(ctx,
		`SELECT `+journalColumns+` FROM journal_entries WHERE id = ?`, id))
	if err != nil {
		return e, fmt.Errorf("get journal entry: %w", err)
	}
	return e, nil
}

// ListJournalEntries is for complete, transaction-consistent backup export.
func (s *Store) ListJournalEntries(ctx context.Context) ([]JournalEntry, error) {
	return scanAll(ctx, s.reader(), "journal entries", `SELECT `+journalColumns+
		` FROM journal_entries ORDER BY occurred_at DESC, id DESC`, nil, scanJournalEntry)
}

// UpdateJournalEntry replaces editable fields without changing attribution.
func (s *Store) UpdateJournalEntry(ctx context.Context, e *JournalEntry) error {
	e.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx, `UPDATE journal_entries SET body = ?, occurred_at = ?, updated_at = ?,
 connector_id = ?, doc_id = ?, entity_kind = ?, entity_name = ?, entity_ref = ? WHERE id = ?`,
		e.Body, e.OccurredAt, e.UpdatedAt, nilToStr(e.ConnectorID), nilToStr(e.DocID),
		e.EntityKind, e.EntityName, e.EntityRef, e.ID)
	if err != nil {
		return fmt.Errorf("update journal entry: %w", err)
	}
	if rowsAffected(res) == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteJournalEntry permanently removes a manual note.
func (s *Store) DeleteJournalEntry(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM journal_entries WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete journal entry: %w", err)
	}
	if rowsAffected(res) == 0 {
		return ErrNotFound
	}
	return nil
}
