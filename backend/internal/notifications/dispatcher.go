// Package notifications provides notification dispatching (in-app, SMTP stub, webhook) with
// per-channel delivery tracking and bounded retry for failed deliveries.
package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

// retrySchedule is the backoff delay before each retry attempt (index 0 = delay before the
// 2nd attempt, etc).
// ponytail: fixed schedule, not exponential-from-config; add jitter/config if a real deployment needs it.
var retrySchedule = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, 30 * time.Minute, time.Hour}

// maxDeliveryAttempts caps retries; once exhausted a failed delivery stays failed with no next_attempt_at.
const maxDeliveryAttempts = 5 // len(retrySchedule)

var webhookClient = &http.Client{Timeout: 10 * time.Second}

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

// channelCfg is one entry of the notification_config "channels" array.
type channelCfg struct {
	Type    string         `json:"type"`
	Enabled bool           `json:"enabled"`
	Config  map[string]any `json:"config"`
}

// loadChannels reads the configured notification channels. Returns nil (no channels) on any
// read/parse error so callers just skip optional channels — in-app delivery never depends on this.
func (d *Dispatcher) loadChannels(ctx context.Context) []channelCfg {
	var raw string
	if err := d.store.DB().QueryRowContext(ctx, `SELECT config_json FROM notification_config WHERE id = 1`).Scan(&raw); err != nil || raw == "" {
		return nil
	}
	var doc struct {
		Channels []channelCfg `json:"channels"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil
	}
	return doc.Channels
}

// channel returns the config for the given channel type and whether it's enabled.
func (d *Dispatcher) channel(ctx context.Context, typ string) (channelCfg, bool) {
	for _, c := range d.loadChannels(ctx) {
		if c.Type == typ {
			return c, c.Enabled
		}
	}
	return channelCfg{}, false
}

// NotifyAlert dispatches an alert to all configured channels, recording a delivery row per
// channel attempted (see store.DeliveryRecord).
func (d *Dispatcher) NotifyAlert(alertID, userID, eventType, title, message string) {
	ctx := context.Background()

	notifID, err := d.createInApp(ctx, userID, alertID, eventType, title, message)
	if err != nil {
		slog.Error("failed to create in-app notification", "error", err)
		return
	}
	d.recordDelivery(ctx, notifID, "in_app", store.DeliveryStatusSent, "")

	if d.hub != nil {
		d.hub.BroadcastToUser(userID, eventType, map[string]any{
			"alertId": alertID,
			"title":   title,
			"message": message,
		})
	}

	if _, enabled := d.channel(ctx, "smtp"); enabled {
		// ponytail: SMTP delivery is a stub — always "succeeds" and only logs. Real sending
		// (user email lookup, SMTP auth/TLS, credential decryption) is out of scope for issue #18;
		// still recorded as a real delivery row so retry/observability plumbing already covers it
		// once real sending lands.
		slog.Info("SMTP notification (stub)", "userID", userID, "title", title)
		d.recordDelivery(ctx, notifID, "smtp", store.DeliveryStatusSent, "")
	}

	if cfg, enabled := d.channel(ctx, "webhook"); enabled {
		d.attemptWebhook(ctx, notifID, cfg, title, message)
	}
}

// createInApp creates the in-app notification row and returns its ID.
func (d *Dispatcher) createInApp(ctx context.Context, userID, alertID, eventType, title, message string) (string, error) {
	n := &store.NotificationRecord{
		UserID:    userID,
		AlertID:   alertID,
		EventType: eventType,
		Title:     title,
		Message:   message,
	}
	if err := d.store.CreateNotification(ctx, n); err != nil {
		return "", err
	}
	return n.ID, nil
}

// recordDelivery persists the outcome of a first delivery attempt for one channel.
func (d *Dispatcher) recordDelivery(ctx context.Context, notificationID, channel string, status store.DeliveryStatus, lastError string) {
	rec := &store.DeliveryRecord{
		NotificationID: notificationID,
		Channel:        channel,
		Status:         status,
		Attempts:       1,
		LastError:      lastError,
	}
	if status == store.DeliveryStatusFailed {
		rec.NextAttemptAt = time.Now().UTC().Add(retrySchedule[0]).Format(time.RFC3339)
	}
	if err := d.store.CreateDelivery(ctx, rec); err != nil {
		slog.Error("record delivery", "error", err, "channel", channel)
	}
}

// attemptWebhook sends the webhook and records the resulting delivery status.
func (d *Dispatcher) attemptWebhook(ctx context.Context, notificationID string, cfg channelCfg, title, message string) {
	url, _ := cfg.Config["url"].(string)
	if url == "" {
		d.recordDelivery(ctx, notificationID, "webhook", store.DeliveryStatusFailed, "webhook url not configured")
		return
	}
	if err := sendWebhook(ctx, url, title, message); err != nil {
		d.recordDelivery(ctx, notificationID, "webhook", store.DeliveryStatusFailed, err.Error())
		return
	}
	d.recordDelivery(ctx, notificationID, "webhook", store.DeliveryStatusSent, "")
}

// sendWebhook POSTs a JSON payload to url and treats any transport error or non-2xx response as failure.
func sendWebhook(ctx context.Context, url, title, message string) error {
	body, err := json.Marshal(map[string]string{"title": title, "message": message})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := webhookClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
