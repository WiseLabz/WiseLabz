package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DocAttachment records the owning doc, immutable blob and original upload metadata.
type DocAttachment struct {
	ID          string `json:"id"`
	DocID       string `json:"docId"`
	SHA256      string `json:"sha256"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	CreatedBy   string `json:"createdBy"`
	CreatedAt   string `json:"createdAt"`
	URL         string `json:"url,omitempty"`
}

const attachmentColumns = `id, doc_id, sha256, filename, content_type, size, created_by, created_at`

func scanAttachment(row rowScanner) (DocAttachment, error) {
	var a DocAttachment
	err := row.Scan(&a.ID, &a.DocID, &a.SHA256, &a.Filename, &a.ContentType, &a.Size, &a.CreatedBy, &a.CreatedAt)
	return a, err
}

// CreateDocAttachment inserts metadata with a generated ID and creation time.
func (s *Store) CreateDocAttachment(ctx context.Context, a *DocAttachment) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	if a.CreatedAt == "" {
		a.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO doc_attachments (`+attachmentColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.DocID, a.SHA256, a.Filename, a.ContentType, a.Size, a.CreatedBy, a.CreatedAt)
	if err != nil {
		return fmt.Errorf("create doc attachment: %w", err)
	}
	return nil
}

// ListDocAttachments includes trash when docID is empty (backup and garbage collection).
func (s *Store) ListDocAttachments(ctx context.Context, docID string) ([]DocAttachment, error) {
	query := `SELECT ` + attachmentColumns + ` FROM doc_attachments`
	args := []any{}
	if docID != "" {
		query += ` WHERE doc_id = ?`
		args = append(args, docID)
	}
	rows, err := s.db.QueryContext(ctx, query+` ORDER BY created_at, id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list doc attachments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	attachments := []DocAttachment{}
	for rows.Next() {
		a, err := scanAttachment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		attachments = append(attachments, a)
	}
	return attachments, rows.Err()
}

// GetDocAttachment returns metadata by attachment ID, including trash references.
func (s *Store) GetDocAttachment(ctx context.Context, id string) (*DocAttachment, error) {
	a, err := scanAttachment(s.db.QueryRowContext(ctx, `SELECT `+attachmentColumns+` FROM doc_attachments WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get attachment: %w", err)
	}
	return &a, nil
}

// DeleteDocAttachment deletes metadata only when the doc and attachment IDs match.
func (s *Store) DeleteDocAttachment(ctx context.Context, docID, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM doc_attachments WHERE doc_id = ? AND id = ?`, docID, id)
	if err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// BlobReferenced checks references from every live or deleted doc.
func (s *Store) BlobReferenced(ctx context.Context, hash string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM doc_attachments WHERE sha256 = ?`, hash).Scan(&n)
	return n > 0, err
}
