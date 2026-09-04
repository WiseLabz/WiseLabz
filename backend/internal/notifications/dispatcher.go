// Package notifications provides notification dispatching (in-app, SMTP stub, webhook stub).
package notifications

import (
	"context"
	"log/slog"

	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

// Dispatcher routes alert events to notification channels based on config.
type Dispatcher struct {
	store *store.Store
	hub   *ws.Hub
}

// NewDispatcher creates a new notification dispatcher.
func NewDispatcher(s *store.Store, hub *ws.Hub) *Dispatcher {
	return &Dispatcher{store: s, hub: hub}
}

// maxNotifyUsers bounds the single-page user fetch in NotifyAlertCreated.
// ponytail: fine for a self-hosted ops tool's user count; paginate if that changes.
const maxNotifyUsers = 1000

// NotifyAlertCreated dispatches a newly created alert to every active user.
func (d *Dispatcher) NotifyAlertCreated(ctx context.Context, alertID, title, message string) {
	users, _, err := d.store.ListUsers(ctx, 0, maxNotifyUsers)
	if err != nil {
		slog.Error("failed to list users for alert notification", "error", err)
		return
	}
	for _, u := range users {
		if u.Disabled {
			continue
		}
		d.NotifyAlert(alertID, u.ID, "alert.created", title, message)
	}
}

// NotifyAlert dispatches an alert to all configured channels.
func (d *Dispatcher) NotifyAlert(alertID, userID, eventType, title, message string) {
	// Always create in-app notification
	if err := d.createInApp(userID, alertID, eventType, title, message); err != nil {
		slog.Error("failed to create in-app notification", "error", err)
	}

	// Push via WebSocket
	if d.hub != nil {
		d.hub.BroadcastToUser(userID, eventType, map[string]any{
			"alertId": alertID,
			"title":   title,
			"message": message,
		})
	}

	// SMTP stub — logs, returns success
	slog.Info("SMTP notification (stub)", "userID", userID, "title", title)

	// Webhook stub — logs, returns success
	slog.Info("Webhook notification (stub)", "userID", userID, "title", title)
}

func (d *Dispatcher) createInApp(userID, alertID, eventType, title, message string) error {
	return d.store.CreateNotification(context.TODO(), &store.NotificationRecord{
		UserID:    userID,
		AlertID:   alertID,
		EventType: eventType,
		Title:     title,
		Message:   message,
	})
}
