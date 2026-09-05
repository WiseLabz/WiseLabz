package notifications

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
	_ "modernc.org/sqlite"
)

// newTestStore mirrors internal/sync/engine_test.go's helper: a real sqlite-backed store so
// dispatcher logic exercises actual SQL instead of a mock.
func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	dsn := "file:" + dir + "/test.db?cache=shared"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() }) //nolint:errcheck

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	if err := store.RunMigrations(db, "sqlite", logger); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return store.New(db, "sqlite")
}

// setWebhookConfig writes a notification_config row enabling only the webhook channel.
func setWebhookConfig(t *testing.T, s *store.Store, url string) {
	t.Helper()
	cfgJSON := `{"channels":[{"type":"webhook","enabled":true,"config":{"url":"` + url + `"}}]}`
	// newTestStore doesn't call Store.Init, so the id=1 singleton row may not exist yet — upsert.
	if _, err := s.DB().ExecContext(context.Background(),
		`INSERT INTO notification_config (id, config_json) VALUES (1, ?) ON CONFLICT(id) DO UPDATE SET config_json = excluded.config_json`, cfgJSON); err != nil {
		t.Fatalf("set webhook config: %v", err)
	}
}

func deliveriesFor(t *testing.T, s *store.Store, notificationID string) []store.DeliveryRecord {
	t.Helper()
	// ponytail: ListDeliveries only filters by status, not notification_id; query directly
	// instead of adding a store method the dispatcher package doesn't otherwise need.
	rows, err := s.DB().QueryContext(context.Background(),
		`SELECT id, notification_id, channel, status, attempts, last_error, IFNULL(next_attempt_at, '') FROM notification_deliveries WHERE notification_id = ?`, notificationID)
	if err != nil {
		t.Fatalf("query deliveries: %v", err)
	}
	defer rows.Close() //nolint:errcheck
	var out []store.DeliveryRecord
	for rows.Next() {
		var d store.DeliveryRecord
		if err := rows.Scan(&d.ID, &d.NotificationID, &d.Channel, &d.Status, &d.Attempts, &d.LastError, &d.NextAttemptAt); err != nil {
			t.Fatalf("scan delivery: %v", err)
		}
		out = append(out, d)
	}
	return out
}

func findDelivery(deliveries []store.DeliveryRecord, channel string) (store.DeliveryRecord, bool) {
	for _, d := range deliveries {
		if d.Channel == channel {
			return d, true
		}
	}
	return store.DeliveryRecord{}, false
}

func TestNotifyAlert_NoChannelsConfigured(t *testing.T) {
	s := newTestStore(t)
	d := NewDispatcher(s, nil)

	d.NotifyAlert("alert-1", "user-1", "alert.created", "Title", "Message")

	notifs, _, err := s.ListNotifications(context.Background(), "user-1", false, 0, 10)
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifs))
	}

	deliveries := deliveriesFor(t, s, notifs[0].ID)
	inApp, ok := findDelivery(deliveries, "in_app")
	if !ok {
		t.Fatalf("expected in_app delivery row, got %+v", deliveries)
	}
	if inApp.Status != store.DeliveryStatusSent {
		t.Errorf("expected in_app status sent, got %s", inApp.Status)
	}
	if _, ok := findDelivery(deliveries, "smtp"); ok {
		t.Errorf("expected no smtp delivery row when smtp not configured")
	}
	if _, ok := findDelivery(deliveries, "webhook"); ok {
		t.Errorf("expected no webhook delivery row when webhook not configured")
	}
}

func TestNotifyAlert_WebhookSuccess(t *testing.T) {
	s := newTestStore(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	setWebhookConfig(t, s, srv.URL)

	d := NewDispatcher(s, nil)
	d.NotifyAlert("alert-1", "user-1", "alert.created", "Title", "Message")

	notifs, _, err := s.ListNotifications(context.Background(), "user-1", false, 0, 10)
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	deliveries := deliveriesFor(t, s, notifs[0].ID)
	webhook, ok := findDelivery(deliveries, "webhook")
	if !ok {
		t.Fatalf("expected webhook delivery row, got %+v", deliveries)
	}
	if webhook.Status != store.DeliveryStatusSent {
		t.Errorf("expected webhook status sent, got %s (err=%s)", webhook.Status, webhook.LastError)
	}
}

func TestNotifyAlert_WebhookFailure(t *testing.T) {
	s := newTestStore(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	setWebhookConfig(t, s, srv.URL)

	d := NewDispatcher(s, nil)
	d.NotifyAlert("alert-1", "user-1", "alert.created", "Title", "Message")

	notifs, _, err := s.ListNotifications(context.Background(), "user-1", false, 0, 10)
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	deliveries := deliveriesFor(t, s, notifs[0].ID)
	webhook, ok := findDelivery(deliveries, "webhook")
	if !ok {
		t.Fatalf("expected webhook delivery row, got %+v", deliveries)
	}
	if webhook.Status != store.DeliveryStatusFailed {
		t.Errorf("expected webhook status failed, got %s", webhook.Status)
	}
	if webhook.NextAttemptAt == "" {
		t.Errorf("expected non-empty NextAttemptAt for failed delivery")
	}
	if webhook.LastError == "" {
		t.Errorf("expected non-empty LastError for failed delivery")
	}
}

// TestRetryDueDeliveries_RecoversAfterFailure drives one retry loop iteration directly: a
// webhook delivery that failed against a dead endpoint is retried once the endpoint (the same
// URL, backed by a mutable handler) starts returning 200, ending up sent with attempts incremented.
func TestRetryDueDeliveries_RecoversAfterFailure(t *testing.T) {
	s := newTestStore(t)

	failing := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if failing {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	setWebhookConfig(t, s, srv.URL)

	d := NewDispatcher(s, nil)
	d.NotifyAlert("alert-1", "user-1", "alert.created", "Title", "Message")

	notifs, _, err := s.ListNotifications(context.Background(), "user-1", false, 0, 10)
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	before, ok := findDelivery(deliveriesFor(t, s, notifs[0].ID), "webhook")
	if !ok || before.Status != store.DeliveryStatusFailed {
		t.Fatalf("expected initial webhook delivery to be failed, got %+v (ok=%v)", before, ok)
	}

	// Force the delivery due now (its natural next_attempt_at is minutes in the future).
	if _, err := s.DB().ExecContext(context.Background(),
		`UPDATE notification_deliveries SET next_attempt_at = ? WHERE id = ?`, "1970-01-01T00:00:00Z", before.ID); err != nil {
		t.Fatalf("force due: %v", err)
	}

	failing = false
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	d.retryDueDeliveries(context.Background(), logger)

	after, ok := findDelivery(deliveriesFor(t, s, notifs[0].ID), "webhook")
	if !ok {
		t.Fatalf("expected webhook delivery row to still exist")
	}
	if after.Status != store.DeliveryStatusSent {
		t.Errorf("expected webhook status sent after retry, got %s", after.Status)
	}
	if after.Attempts != before.Attempts+1 {
		t.Errorf("expected attempts %d, got %d", before.Attempts+1, after.Attempts)
	}
}
