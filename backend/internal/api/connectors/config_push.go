package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/logsafe"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/sync"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

// configPushRequest is the POST /api/connectors/{id}/config-push body.
type configPushRequest struct {
	EntityRef string `json:"entityRef"`
	FieldKey  string `json:"fieldKey"`
	Value     any    `json:"value"`
	// PreviousValue is the field's currently-displayed value, as the
	// frontend read it from GET /{id}/data before the user edited it.
	// Used for revert only when the connector has no ConfigReader.
	PreviousValue any `json:"previousValue"`
}

// ConfigPush handles POST /api/connectors/{id}/config-push. Per ADR 0003:
// field-level partial update against a per-connector whitelist, snapshot
// before write, verify-diff after, auto-revert-then-alert on mismatch.
func (h *Handler) ConfigPush(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	req, ok := httputil.DecodeJSON[configPushRequest](w, r)
	if !ok {
		return
	}
	if !validateConfigPushRequest(w, &req) {
		return
	}

	prepared, err := h.prepareConfigPush(r.Context(), id, req.FieldKey)
	if err != nil {
		writeConfigPushError(w, err)
		return
	}
	if err := auth.ValidateElevationHeader(h.JWT, h.Store, "connector.configPush", r); err != nil {
		auth.WriteElevationError(w, err)
		return
	}
	actor := LifecycleActor{
		UserID:        auth.UserIDFromContext(r.Context()),
		InstanceAdmin: auth.InstanceAdminFromContext(r.Context()),
	}
	post, err := h.mutateConfigPush(r.Context(), prepared, req, actor, nil)
	if err != nil {
		writeConfigPushError(w, err)
		return
	}
	if post == nil {
		post, err = prepared.conn.Fetch(r.Context(), prepared.config)
		if err != nil {
			httputil.Errorf(w, err)
			return
		}
	}
	httputil.JSON(w, http.StatusOK, post)
}

// ConfigPushMismatchError indicates the write could not be verified.
// RevertAttempted reports whether a known previous value was written back.
type ConfigPushMismatchError struct {
	RevertAttempted bool
	RevertErr       error
}

func (e *ConfigPushMismatchError) Error() string {
	if e.RevertAttempted {
		return "pushed value did not verify; auto-revert attempted, see the new alert"
	}
	return "could not verify pushed value; previous value unknown, see the new alert"
}

func (e *ConfigPushMismatchError) Unwrap() error { return e.RevertErr }

type preparedConfigPush struct {
	conn   connector.Connector
	pusher connector.ConfigPusher
	config map[string]any
	record *store.ConnectorRecord
}

func writeConfigPushError(w http.ResponseWriter, err error) {
	var mismatch *ConfigPushMismatchError
	if errors.As(err, &mismatch) {
		httputil.Error(w, http.StatusConflict, "config_push_mismatch", mismatch.Error())
		return
	}
	writeLifecycleError(w, err)
}

// MutateRunbookConfigPush performs an already-authorized config-push step.
// Callers must check operator grants and elevation before invoking the core.
func (h *Handler) MutateRunbookConfigPush(
	ctx context.Context,
	connectorID, entityRef, fieldKey string,
	value any,
	actor LifecycleActor,
	extraAudit map[string]any,
) error {
	if err := connector.ValidateCompositeRef(entityRef); err != nil {
		return &lifecycleError{status: http.StatusBadRequest, code: "invalid_request", message: "invalid entityRef", cause: err}
	}
	prepared, err := h.prepareConfigPush(ctx, connectorID, fieldKey)
	if err != nil {
		return err
	}
	if prepared.record.ManagedBy != store.ManagedByUI && prepared.record.ManagedBy != store.ManagedByConfig {
		return managedConflictError(prepared.record.ManagedBy)
	}
	_, err = h.mutateConfigPush(ctx, prepared, configPushRequest{
		EntityRef: entityRef, FieldKey: fieldKey, Value: value,
	}, actor, extraAudit)
	return err
}

func (h *Handler) mutateConfigPush(
	ctx context.Context,
	prepared *preparedConfigPush,
	req configPushRequest,
	actor LifecycleActor,
	extraAudit map[string]any,
) (*connector.ServiceSnapshot, error) {
	previous, known := req.PreviousValue, req.PreviousValue != nil
	if reader, ok := prepared.conn.(connector.ConfigReader); ok {
		current, err := reader.ConfigRead(ctx, prepared.config, req.EntityRef, req.FieldKey)
		if err != nil {
			return nil, fmt.Errorf("pre-push config read: %w", err)
		}
		// The reader's answer, including "unknown" (nil), replaces the browser's
		// previousValue so a client cannot choose the revert value.
		previous, known = current, current != nil
		if known && configValuesEqual(current, req.Value) {
			return nil, nil
		}
	}
	pre, err := prepared.conn.Fetch(ctx, prepared.config)
	if err != nil {
		return nil, fmt.Errorf("pre-push fetch: %w", err)
	}
	if err := prepared.pusher.ConfigPush(ctx, prepared.config, req.EntityRef, req.FieldKey, req.Value); err != nil {
		slog.Error("connector config-push failed", "connector", logsafe.Sanitize(prepared.record.ID),
			"field", logsafe.Sanitize(req.FieldKey), "error", logsafe.Err(err))
		return nil, &lifecycleError{status: http.StatusBadGateway, code: "config_push_failed", message: err.Error(), cause: err}
	}
	post, err := prepared.conn.Fetch(ctx, prepared.config)
	if err != nil {
		return nil, fmt.Errorf("post-push fetch: %w", err)
	}
	if !configPushLanded(pre, post) && !configPushConfirmed(ctx, prepared, req) {
		return nil, h.revertConfigPush(ctx, prepared, req.EntityRef, req.FieldKey, previous, known)
	}

	detail := make(map[string]any, len(extraAudit)+2)
	for k, v := range extraAudit {
		detail[k] = v
	}
	detail["fieldKey"] = req.FieldKey
	detail["entityRef"] = req.EntityRef
	if err := h.recordLifecycleAudit(context.WithoutCancel(ctx), actor, "connector.configPush", prepared.record.ID, detail); err != nil {
		slog.Error("failed to record audit", "action", "connector.configPush", "error", err)
	}
	return post, nil
}

// JSON represents driver numbers and browser numbers identically (int vs float64).
func configValuesEqual(current, target any) bool {
	a, err := json.Marshal(current)
	if err != nil {
		return false
	}
	b, err := json.Marshal(target)
	return err == nil && bytes.Equal(a, b)
}

// validateConfigPushRequest checks the body's addressing fields. It writes
// the error response and reports false when the request is malformed.
func validateConfigPushRequest(w http.ResponseWriter, req *configPushRequest) bool {
	if err := connector.ValidateCompositeRef(req.EntityRef); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "invalid entityRef")
		return false
	}
	if req.FieldKey == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "fieldKey is required")
		return false
	}
	return true
}

// prepareConfigPush resolves the live connector and its writable-field whitelist.
func (h *Handler) prepareConfigPush(ctx context.Context, id, fieldKey string) (*preparedConfigPush, error) {
	rec, err := h.Store.GetConnector(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, &lifecycleError{status: http.StatusNotFound, code: "not_found", message: "Connector not found", cause: err}
	}
	if err != nil {
		return nil, err
	}
	cfg, err := store.ParseConnectorConfig(rec.Type, rec.ConfigData, h.Config.Encryption.Key)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	connector.ApplyRecordConfig(cfg, rec.URL, rec.VerifyTLS)
	conn, err := connector.Get(rec.Type, cfg)
	if err != nil {
		return nil, err
	}
	pusher, ok := conn.(connector.ConfigPusher)
	if !ok {
		return nil, &lifecycleError{status: http.StatusBadRequest, code: "unsupported_operation", message: "connector does not support config-push"}
	}
	if !isWritableField(pusher, fieldKey) {
		return nil, &lifecycleError{
			status: http.StatusBadRequest, code: "unsupported_field",
			message: fmt.Sprintf("field %q is not writable for this connector", fieldKey),
		}
	}
	return &preparedConfigPush{conn: conn, pusher: pusher, config: cfg, record: rec}, nil
}

// isWritableField reports whether key is on the connector's config-push
// whitelist.
func isWritableField(pusher connector.ConfigPusher, key string) bool {
	for _, f := range pusher.WritableFields() {
		if f.Key == key {
			return true
		}
	}
	return false
}

// configPushLanded reports whether pre→post shows any change in the
// documented content. sync.Compare's diff results, non-empty, are treated as
// "the write landed as some change" — the field-scoped check ADR 0003 calls
// for; a per-field-key section filter would need a fixed field→section
// mapping per connector, which the curated whitelist doesn't carry today.
// An empty diff is not proof of a failed write; configPushConfirmed settles it.
// ponytail: any detected change counts as "landed" rather than verifying
// the new value matches exactly — tighten if a connector's diff proves too
// coarse to catch a landed-but-wrong-value push.
func configPushLanded(pre, post *connector.ServiceSnapshot) bool {
	return len(sync.Compare(pre, post)) > 0
}

// configPushConfirmed reads the field back after a write that changed nothing
// in the documented content. Some writes land without showing there, for
// example a memory change on a running guest that only appears after a restart.
// It is true only when the connector reports exactly the target value; a nil
// answer, another value or a read error all count as not confirmed.
func configPushConfirmed(ctx context.Context, prepared *preparedConfigPush, req configPushRequest) bool {
	reader, ok := prepared.conn.(connector.ConfigReader)
	if !ok {
		return false
	}
	got, err := reader.ConfigRead(ctx, prepared.config, req.EntityRef, req.FieldKey)
	if err != nil {
		slog.Warn("config-push read-back failed", "connector", logsafe.Sanitize(prepared.record.ID),
			"field", logsafe.Sanitize(req.FieldKey), "error", logsafe.Err(err))
		return false
	}
	return got != nil && configValuesEqual(got, req.Value)
}

// revertConfigPush restores a known previous value and raises the mismatch alert.
func (h *Handler) revertConfigPush(
	ctx context.Context,
	prepared *preparedConfigPush,
	entityRef, fieldKey string,
	previousValue any,
	known bool,
) error {
	id := prepared.record.ID
	mismatch := &ConfigPushMismatchError{RevertAttempted: known}
	desc := fmt.Sprintf("Pushed field %q could not verify after write; previous value unknown, no auto-revert attempted.", fieldKey)
	if known {
		mismatch.RevertErr = prepared.pusher.ConfigPush(ctx, prepared.config, entityRef, fieldKey, previousValue)
		desc = fmt.Sprintf("Pushed field %q did not verify after write; auto-revert to the pre-push value was attempted.", fieldKey)
		if mismatch.RevertErr != nil {
			desc = fmt.Sprintf("Pushed field %q did not verify after write; auto-revert FAILED (%v) — manual intervention required.", fieldKey, mismatch.RevertErr)
			slog.Error("config-push auto-revert failed", "connector", logsafe.Sanitize(id), "field", logsafe.Sanitize(fieldKey), "error", logsafe.Err(mismatch.RevertErr))
		} else {
			slog.Warn("config-push mismatch, auto-reverted", "connector", logsafe.Sanitize(id), "field", logsafe.Sanitize(fieldKey))
		}
	}
	alert := &store.AlertRecord{
		ServiceID: id, Severity: "critical",
		Title: fmt.Sprintf("Config push mismatch for %s", prepared.record.Name), Description: desc,
	}
	if createErr := h.Store.CreateAlert(context.WithoutCancel(ctx), alert); createErr != nil {
		slog.Error("failed to create config-push mismatch alert", "error", createErr)
	} else if h.WSHub != nil {
		h.WSHub.BroadcastConnector(id, ws.EventAlertCreated, map[string]any{
			"alertId": alert.ID, "serviceId": id, "severity": alert.Severity, "title": alert.Title,
		})
	}
	return mismatch
}
