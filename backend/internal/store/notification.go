package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// NotificationRecord represents a row in the in_app_notifications table.
type NotificationRecord struct {
	ID        string `json:"id"`
	UserID    string `json:"-"`
	AlertID   string `json:"alertId,omitempty"` // empty = NULL, no deep-link target
	EventType string `json:"eventType"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"createdAt"`
}

// CreateNotification inserts a new in-app notification.
func (s *Store) CreateNotification(ctx context.Context, n *NotificationRecord) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	if n.CreatedAt == "" {
		n.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO in_app_notifications (id, user_id, alert_id, event_type, title, message, read, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, n.ID, n.UserID, nilToStr(n.AlertID), n.EventType, n.Title, n.Message, n.Read, n.CreatedAt)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

// ListNotifications returns a paginated list of a user's notifications, most recent first.
func (s *Store) ListNotifications(ctx context.Context, userID string, unreadOnly bool, offset, limit int) ([]NotificationRecord, int, error) {
	where := "WHERE user_id = ?"
	args := []any{userID}
	if unreadOnly {
		where += " AND read = 0"
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM in_app_notifications " + where
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}

	query := `SELECT id, user_id, alert_id, event_type, title, message, read, created_at
		FROM in_app_notifications ` + where + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var notifications []NotificationRecord
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}
		notifications = append(notifications, n)
	}
	if notifications == nil {
		notifications = []NotificationRecord{}
	}
	return notifications, total, nil
}

// MarkNotificationRead marks one of a user's notifications as read and returns
// the updated record. Scoping the UPDATE by userID means a wrong owner and a
// nonexistent id are indistinguishable (both ErrNotFound) — existence isn't
// leaked across users.
func (s *Store) MarkNotificationRead(ctx context.Context, userID, id string) (*NotificationRecord, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE in_app_notifications SET read = 1 WHERE id = ? AND user_id = ?
	`, id, userID)
	if err != nil {
		return nil, fmt.Errorf("mark notification read: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return nil, ErrNotFound
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, alert_id, event_type, title, message, read, created_at
		FROM in_app_notifications WHERE id = ?
	`, id)
	n, err := scanNotification(row)
	if err != nil {
		return nil, fmt.Errorf("get notification: %w", err)
	}
	return &n, nil
}

// MarkAllNotificationsRead marks all of a user's unread notifications as read.
// Zero rows affected (nothing unread) is a valid outcome, not an error.
func (s *Store) MarkAllNotificationsRead(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE in_app_notifications SET read = 1 WHERE user_id = ? AND read = 0
	`, userID)
	if err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}

// rowScanner abstracts *sql.Row and *sql.Rows so scanNotification serves both
// the single-row (mark-read) and multi-row (list) query paths.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanNotification(row rowScanner) (NotificationRecord, error) {
	var n NotificationRecord
	var alertID sql.NullString
	err := row.Scan(&n.ID, &n.UserID, &alertID, &n.EventType, &n.Title, &n.Message, &n.Read, &n.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return NotificationRecord{}, ErrNotFound
	}
	if err != nil {
		return NotificationRecord{}, err
	}
	n.AlertID = alertID.String
	return n, nil
}
