package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// NotificationFile records a blob retained for a notification's pending deliveries.
type NotificationFile struct {
	NotificationID string
	Filename       string
	ContentType    string
	SHA256         string
	Size           int64
	CreatedAt      string
}

// PutNotificationFile inserts the retained file once per notification.
func (s *Store) PutNotificationFile(ctx context.Context, file *NotificationFile) error {
	if file.CreatedAt == "" {
		file.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO notification_files (notification_id, filename, content_type, sha256, size, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(notification_id) DO NOTHING
	`, file.NotificationID, file.Filename, file.ContentType, file.SHA256, file.Size, file.CreatedAt)
	if err != nil {
		return fmt.Errorf("put notification file: %w", err)
	}
	return nil
}

// GetNotificationFile returns retained file metadata or ErrNotFound.
func (s *Store) GetNotificationFile(ctx context.Context, notificationID string) (*NotificationFile, error) {
	var file NotificationFile
	err := s.db.QueryRowContext(ctx, `
		SELECT notification_id, filename, content_type, sha256, size, created_at
		FROM notification_files WHERE notification_id = ?
	`, notificationID).Scan(&file.NotificationID, &file.Filename, &file.ContentType, &file.SHA256, &file.Size, &file.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get notification file: %w", err)
	}
	return &file, nil
}

// DeleteNotificationFile releases the retained blob reference.
func (s *Store) DeleteNotificationFile(ctx context.Context, notificationID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM notification_files WHERE notification_id = ?`, notificationID)
	if err != nil {
		return fmt.Errorf("delete notification file: %w", err)
	}
	return nil
}

// HasPendingDeliveryRetry reports whether any channel can still retry this notification.
func (s *Store) HasPendingDeliveryRetry(ctx context.Context, notificationID string) (bool, error) {
	var pending bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM notification_deliveries
			WHERE notification_id = ? AND status = 'failed'
			  AND next_attempt_at IS NOT NULL AND next_attempt_at <> ''
		)
	`, notificationID).Scan(&pending)
	if err != nil {
		return false, fmt.Errorf("check pending delivery retry: %w", err)
	}
	return pending, nil
}
