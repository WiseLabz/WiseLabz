package store

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestNotificationFileLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newDocTestStore(t)
	if os.Getenv("WISELABZ_TEST_POSTGRES_DSN") == "" {
		if _, err := s.DB().ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
			t.Fatalf("enable SQLite foreign keys: %v", err)
		}
	}
	mustCreateUser(t, s, "file-user")
	notification := &NotificationRecord{UserID: "file-user", EventType: "report.generated", Title: "Report"}
	if err := s.CreateNotification(ctx, notification); err != nil {
		t.Fatal(err)
	}
	file := &NotificationFile{NotificationID: notification.ID, Filename: "lab-book.html", ContentType: "text/html", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 42}
	if err := s.PutNotificationFile(ctx, file); err != nil {
		t.Fatalf("PutNotificationFile(): %v", err)
	}
	// Repeated failures for other channels keep one retained file per notification.
	if err := s.PutNotificationFile(ctx, file); err != nil {
		t.Fatalf("PutNotificationFile() idempotent insert: %v", err)
	}
	got, err := s.GetNotificationFile(ctx, notification.ID)
	if err != nil || got.SHA256 != file.SHA256 || got.Size != 42 {
		t.Fatalf("GetNotificationFile() = %+v, %v", got, err)
	}
	referenced, err := s.BlobReferenced(ctx, file.SHA256)
	if err != nil || !referenced {
		t.Fatalf("BlobReferenced() = %v, %v, want true", referenced, err)
	}
	if err := s.DeleteUser(ctx, "file-user"); err != nil {
		t.Fatalf("DeleteUser(): %v", err)
	}
	if _, err := s.GetNotificationFile(ctx, notification.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetNotificationFile() after notification cascade = %v, want ErrNotFound", err)
	}
	referenced, err = s.BlobReferenced(ctx, file.SHA256)
	if err != nil || referenced {
		t.Fatalf("BlobReferenced() after cascade = %v, %v, want false", referenced, err)
	}
}

func TestNotificationFileDeleteAndPendingRetry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newDocTestStore(t)
	mustCreateUser(t, s, "file-user")
	notification := &NotificationRecord{UserID: "file-user", EventType: "report.generated", Title: "Report"}
	if err := s.CreateNotification(ctx, notification); err != nil {
		t.Fatal(err)
	}
	pending, err := s.HasPendingDeliveryRetry(ctx, notification.ID)
	if err != nil || pending {
		t.Fatalf("HasPendingDeliveryRetry() empty = %v, %v, want false", pending, err)
	}
	if err := s.CreateDelivery(ctx, &DeliveryRecord{NotificationID: notification.ID, Channel: "webhook", Status: DeliveryStatusFailed, NextAttemptAt: "2030-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	pending, err = s.HasPendingDeliveryRetry(ctx, notification.ID)
	if err != nil || !pending {
		t.Fatalf("HasPendingDeliveryRetry() failed delivery = %v, %v, want true", pending, err)
	}
	file := &NotificationFile{NotificationID: notification.ID, Filename: "lab-book.html", ContentType: "text/html", SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Size: 42}
	if err := s.PutNotificationFile(ctx, file); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteNotificationFile(ctx, notification.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetNotificationFile(ctx, notification.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetNotificationFile() after delete = %v, want ErrNotFound", err)
	}
}
