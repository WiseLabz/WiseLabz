package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/crypto"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/logsafe"
	"github.com/WiseLabz/wiselabz/internal/notifications"
)

// notificationConfigDoc mirrors the NotificationConfig OpenAPI schema.
type notificationConfigDoc struct {
	Channels []map[string]any `json:"channels"`
	Routing  []map[string]any `json:"routing"`
}

func (h *Handler) loadNotificationConfig(ctx context.Context) notificationConfigDoc {
	cfg := notificationConfigDoc{Channels: []map[string]any{}, Routing: []map[string]any{}}
	var raw string
	if err := h.Store.DB().QueryRowContext(ctx, `SELECT config_json FROM notification_config WHERE id = 1`).Scan(&raw); err == nil && raw != "" {
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			slog.Warn("settings: invalid stored notification config, using defaults", "error", err)
		}
		if cfg.Channels == nil {
			cfg.Channels = []map[string]any{}
		}
		if cfg.Routing == nil {
			cfg.Routing = []map[string]any{}
		}
	}
	return cfg
}

// GetNotificationsConfig handles GET /api/notifications/config. Signing secrets are write-only:
// the encrypted value is replaced by a secretSet flag.
func (h *Handler) GetNotificationsConfig(w http.ResponseWriter, r *http.Request) {
	cfg := h.loadNotificationConfig(r.Context())
	for _, ch := range cfg.Channels {
		conf, _ := ch["config"].(map[string]any)
		if conf == nil {
			continue
		}
		if enc, _ := conf["secretEncrypted"].(string); enc != "" {
			conf["secretSet"] = true
		}
		delete(conf, "secretEncrypted")
	}
	httputil.JSON(w, http.StatusOK, cfg)
}

// applyChannelSecrets turns each channel's write-only config.secret into config.secretEncrypted.
// An absent secret keeps the previously stored one for that channel type; an empty string clears it.
func (h *Handler) applyChannelSecrets(ctx context.Context, cfg *notificationConfigDoc) error {
	prev := map[string]string{}
	for _, ch := range h.loadNotificationConfig(ctx).Channels {
		typ, _ := ch["type"].(string)
		if conf, _ := ch["config"].(map[string]any); conf != nil {
			prev[typ], _ = conf["secretEncrypted"].(string)
		}
	}
	var key []byte
	for _, ch := range cfg.Channels {
		conf, _ := ch["config"].(map[string]any)
		if conf == nil {
			continue
		}
		typ, _ := ch["type"].(string)
		delete(conf, "secretSet")
		delete(conf, "secretEncrypted")
		secret, provided := conf["secret"].(string)
		delete(conf, "secret")
		switch {
		case !provided:
			if prev[typ] != "" {
				conf["secretEncrypted"] = prev[typ]
			}
		case secret != "":
			if key == nil {
				var err error
				if key, err = crypto.DecodeKey(h.Config.Encryption.Key); err != nil {
					return fmt.Errorf("load encryption key: %w", err)
				}
			}
			enc, err := crypto.Encrypt(secret, key)
			if err != nil {
				return fmt.Errorf("encrypt signing secret: %w", err)
			}
			conf["secretEncrypted"] = enc
		}
	}
	return nil
}

// UpdateNotificationsConfig handles PUT /api/notifications/config.
func (h *Handler) UpdateNotificationsConfig(w http.ResponseWriter, r *http.Request) {
	cfg, ok := httputil.DecodeJSON[notificationConfigDoc](w, r)
	if !ok {
		return
	}
	if cfg.Channels == nil {
		cfg.Channels = []map[string]any{}
	}
	if cfg.Routing == nil {
		cfg.Routing = []map[string]any{}
	}

	if err := h.applyChannelSecrets(r.Context(), &cfg); err != nil {
		httputil.Errorf(w, err)
		return
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if _, err := h.Store.DB().ExecContext(r.Context(), `UPDATE notification_config SET config_json = ? WHERE id = 1`, string(data)); err != nil {
		httputil.Errorf(w, err)
		return
	}

	h.GetNotificationsConfig(w, r)
}

// TestNotificationsConfig handles POST /api/notifications/config/test. It sends a real test
// message through the saved config of the requested channel and reports the sender's actual error
// (senders never embed URLs or secrets) on failure. The attempt is audit-logged.
func (h *Handler) TestNotificationsConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Channel string `json:"channel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Channel == "" {
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "channel is required", []httputil.FieldError{{Field: "channel", Msg: "is required"}})
		return
	}

	var conf map[string]any
	if req.Channel != "in_app" {
		found := false
		for _, ch := range h.loadNotificationConfig(r.Context()).Channels {
			if typ, _ := ch["type"].(string); typ == req.Channel {
				conf, _ = ch["config"].(map[string]any)
				found = true
				break
			}
		}
		if !found {
			httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "channel is not configured; save it first", []httputil.FieldError{{Field: "channel", Msg: "is not configured"}})
			return
		}
	}

	var secret string
	if enc, _ := conf["secretEncrypted"].(string); enc != "" {
		key, err := crypto.DecodeKey(h.Config.Encryption.Key)
		if err == nil {
			secret, err = crypto.Decrypt(enc, key)
		}
		if err != nil {
			slog.Error("settings: decrypt channel secret for test", "channel", logsafe.Sanitize(req.Channel), "error", logsafe.Err(err))
			secret = ""
		}
	}

	sendErr := notifications.SendTest(r.Context(), req.Channel, conf, secret)
	detail := map[string]any{"channel": req.Channel, "ok": sendErr == nil}
	if sendErr != nil {
		detail["error"] = sendErr.Error()
	}
	if err := h.Store.RecordAuditFromContext(r.Context(), "notifications.test", "notification_channel", req.Channel, detail); err != nil {
		slog.Error("failed to record audit", "action", "notifications.test", "error", err)
	}
	if sendErr != nil {
		slog.Warn("test notification failed", "channel", logsafe.Sanitize(req.Channel), "error", logsafe.Err(sendErr))
		httputil.JSON(w, http.StatusOK, map[string]any{"ok": false, "message": "Test notification failed: " + sendErr.Error()})
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Test notification sent"})
}
