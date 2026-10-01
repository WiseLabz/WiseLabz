package notifications

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestExternalChannelsOncePerEvent(t *testing.T) {
	for _, event := range []string{"alert", "finding", "system", "report", "digest"} {
		t.Run(event, func(t *testing.T) {
			s := newTestStore(t)
			ctx := context.Background()
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); w.WriteHeader(200) }))
			defer srv.Close()
			setWebhookConfig(t, s, srv.URL)
			now := time.Now().UTC()
			now = time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, time.UTC)
			var users []store.User
			for i := range 4 {
				u := store.User{Username: fmt.Sprintf("user-%d", i), PasswordHash: "hash", DigestCadence: "off", DigestTimezone: "UTC"}
				if event == "digest" || i == 0 {
					u.DigestCadence = "daily"
				}
				if err := s.CreateUser(ctx, &u); err != nil {
					t.Fatal(err)
				}
				users = append(users, u)
				if event == "digest" {
					n := &store.NotificationRecord{UserID: u.ID, EventType: "alert.created", Title: "Alert", Message: "Message", CreatedAt: now.Add(-time.Hour).Format(time.RFC3339)}
					if err := s.CreateNotification(ctx, n); err != nil {
						t.Fatal(err)
					}
				}
			}
			d := NewDispatcher(s, nil)
			switch event {
			case "alert":
				d.NotifyAlertsCreated(ctx, []store.AlertRecord{{ID: "alert", Title: "Title", Description: "Message"}})
			case "finding":
				d.NotifyFindingCreated(ctx, "", "Title", "Message")
			case "system":
				d.NotifySystemEvent(ctx, EventSystemJobFailed, "warning", "Title", "Message")
			case "report":
				d.NotifyReport(ctx, "Title", "Message", []string{"webhook"})
			case "digest":
				if err := d.RunDigestSweep(ctx, now, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
					t.Fatal(err)
				}
			}
			d.Wait()
			deliveries, _, err := s.ListDeliveries(ctx, "", 0, 100)
			if err != nil {
				t.Fatal(err)
			}
			external := 0
			for _, delivery := range deliveries {
				if delivery.Channel == "webhook" {
					external++
				}
			}
			if external != 1 {
				t.Errorf("external delivery rows = %d, want 1", external)
			}
			if calls.Load() != 1 {
				t.Errorf("external sends = %d, want 1", calls.Load())
			}
			for _, u := range users {
				ns, _, err := s.ListNotifications(ctx, u.ID, false, 0, 10)
				if err != nil {
					t.Fatal(err)
				}
				want := 1
				if event == "digest" {
					want = 2
				}
				if len(ns) != want {
					t.Errorf("user %s: %d notifications, want %d", u.ID, len(ns), want)
				}
			}
		})
	}
}

func TestRetryPoisonRowsLeaveDueQueue(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	n := &store.NotificationRecord{UserID: "user", Title: "Title", Message: "Message"}
	if err := s.CreateNotification(ctx, n); err != nil {
		t.Fatal(err)
	}
	for i := range dueDeliveriesLimit {
		id, channel := n.ID, "in_app"
		if i%2 == 0 {
			id, channel = "missing", "webhook"
		}
		del := &store.DeliveryRecord{NotificationID: id, Channel: channel, Status: store.DeliveryStatusFailed, NextAttemptAt: "1970-01-01T00:00:00Z"}
		if err := s.CreateDelivery(ctx, del); err != nil {
			t.Fatal(err)
		}
	}
	var sends atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { sends.Add(1); w.WriteHeader(200) }))
	defer srv.Close()
	setWebhookConfig(t, s, srv.URL)
	healthy := &store.DeliveryRecord{NotificationID: n.ID, Channel: "webhook", Status: store.DeliveryStatusFailed, NextAttemptAt: "1971-01-01T00:00:00Z"}
	if err := s.CreateDelivery(ctx, healthy); err != nil {
		t.Fatal(err)
	}
	d := NewDispatcher(s, nil)
	d.retryDueDeliveries(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)))
	due, err := s.ListDueDeliveries(ctx, time.Now().UTC().Format(time.RFC3339), dueDeliveriesLimit)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0].ID != healthy.ID {
		t.Fatalf("due rows = %+v, want only healthy delivery", due)
	}
	d.retryDueDeliveries(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if sends.Load() != 1 {
		t.Fatalf("healthy retries = %d, want 1", sends.Load())
	}
	due, err = s.ListDueDeliveries(ctx, time.Now().UTC().Format(time.RFC3339), dueDeliveriesLimit)
	if err != nil || len(due) != 0 {
		t.Fatalf("due after retry = %+v, %v", due, err)
	}
	rows, _, err := s.ListDeliveries(ctx, "failed", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.LastError == "" {
			t.Errorf("terminal failure %s has no reason", row.ID)
		}
	}
}
