package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SavedView represents a row in the saved_views table: a user's named,
// reusable filter set for one list surface (services/changes/alerts).
// Filters is an opaque JSON blob — the backend never inspects its shape,
// the frontend owns it.
type SavedView struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	Surface   string `json:"surface"`
	Name      string `json:"name"`
	Filters   string `json:"filters"`
	CreatedAt string `json:"createdAt"`
}

// CreateSavedView inserts a saved_views row, filling ID/CreatedAt/Filters
// defaults when left zero-valued (same convention as CreateAuditRecord).
func (s *Store) CreateSavedView(ctx context.Context, v *SavedView) error {
	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	if v.CreatedAt == "" {
		v.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if v.Filters == "" {
		v.Filters = "{}"
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO saved_views (id, user_id, surface, name, filters, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, v.ID, v.UserID, v.Surface, v.Name, v.Filters, v.CreatedAt)
	if err != nil {
		return fmt.Errorf("create saved view: %w", err)
	}
	return nil
}

// ListSavedViews returns a user's saved views for one surface, newest first.
func (s *Store) ListSavedViews(ctx context.Context, userID, surface string) ([]SavedView, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, surface, name, filters, created_at
		FROM saved_views WHERE user_id = ? AND surface = ?
		ORDER BY created_at DESC
	`, userID, surface)
	if err != nil {
		return nil, fmt.Errorf("list saved views: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var views []SavedView
	for rows.Next() {
		var v SavedView
		if err := rows.Scan(&v.ID, &v.UserID, &v.Surface, &v.Name, &v.Filters, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		views = append(views, v)
	}
	if views == nil {
		views = []SavedView{}
	}
	return views, nil
}

// GetSavedView fetches a single saved view by ID, or ErrNotFound.
func (s *Store) GetSavedView(ctx context.Context, id string) (*SavedView, error) {
	var v SavedView
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, surface, name, filters, created_at
		FROM saved_views WHERE id = ?
	`, id).Scan(&v.ID, &v.UserID, &v.Surface, &v.Name, &v.Filters, &v.CreatedAt)
	if err != nil {
		return nil, ErrNotFound
	}
	return &v, nil
}

// DeleteSavedView deletes a saved view by ID. Ownership is enforced by the
// handler (fetch-then-check, same pattern as DeleteSession) before calling
// this — it deletes unconditionally by ID.
func (s *Store) DeleteSavedView(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM saved_views WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete saved view: %w", err)
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
