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
	_, err := d.store.DB().ExecContext(context.TODO(), `
		INSERT INTO in_app_notifications (id, user_id, alert_id, event_type, title, message, read, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, datetime('now'))
	`, generateID(), userID, alertID, eventType, title, message)
	return err
}

func generateID() string {
	// Simple ID generation using time
	return "notif_" + store.HashToken("inapp")
}
