package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/custom"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/logsafe"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

// RestartPreview handles POST /api/connectors/{id}/restart. dryRun=true
// previews the restart from the latest stored snapshot without touching the
// connector; dryRun absent/false performs the real, elevation-gated restart.
func (h *Handler) RestartPreview(w http.ResponseWriter, r *http.Request) {
	h.serveLifecycleFromRequest(w, r, "restart")
}

// StartPreview handles POST /api/connectors/{id}/start. Same dry-run/mutate
// split as RestartPreview.
func (h *Handler) StartPreview(w http.ResponseWriter, r *http.Request) {
	h.serveLifecycleFromRequest(w, r, "start")
}

// StopPreview handles POST /api/connectors/{id}/stop. Same dry-run/mutate
// split as RestartPreview. estimatedDowntimeSeconds is 0 in the preview
// since a stop's downtime is indefinite until an explicit start, not a
// bounded window.
func (h *Handler) StopPreview(w http.ResponseWriter, r *http.Request) {
	h.serveLifecycleFromRequest(w, r, "stop")
}

// serveLifecycleFromRequest resolves connectorID from the path and
// entityRef from the JSON body — read for both the dry-run and real-mutate
// cases, so a dry-run can preview against a specific entity too — and
// delegates to ServeLifecycleOp with no extra audit detail. This is the
// path used by the connector-scoped restart/start/stop endpoints; the
// runbooks handler calls ServeLifecycleOp directly with a stored
// connector/verb/entityRef and its own extraAudit instead.
func (h *Handler) serveLifecycleFromRequest(w http.ResponseWriter, r *http.Request, verb string) {
	id := r.PathValue("id")
	var body struct {
		EntityRef string `json:"entityRef"`
	}
	if r.Body != nil {
		// An absent or malformed body means entityRef == "" (existing behaviour),
		// but a body cut off by the size limit must not fall through to a
		// whole-service operation.
		err := json.NewDecoder(http.MaxBytesReader(w, r.Body, httputil.MaxJSONBodyBytes)).Decode(&body)
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httputil.Error(w, http.StatusRequestEntityTooLarge, "request_too_large", "Request body too large")
			return
		}
	}
	h.ServeLifecycleOp(w, r, id, verb, body.EntityRef, nil)
}

// ServeLifecycleOp implements the shared dry-run/mutate split for the
// lifecycle verbs (restart/start/stop), driven by an already-resolved
// connectorID/verb/entityRef rather than reading them itself — so both the
// connector-scoped handlers above (id from the path, entityRef from the
// body) and the runbooks ExecuteStep handler (id/verb/entityRef from a
// stored RunbookStepRecord, body ignored) can share it.
//
// dryRun=true previews from the latest stored snapshot without touching the
// connector, targeting entityRef's matching entity when one is given.
// dryRun absent/false performs the real, elevation-gated mutation per ADR
// 0001/0002: config parse, verb-support check ("unsupported_operation"),
// X-Elevation-Token validation against "connector.<verb>", entityRef
// validation, a critical AlertRecord + ws broadcast on failure (no
// rollback), and an audit row on success whose detail is {entityRef} merged
// with extraAudit (e.g. {runbookId, stepId} from the runbooks handler).
func (h *Handler) ServeLifecycleOp(w http.ResponseWriter, r *http.Request, connectorID, verb, entityRef string, extraAudit map[string]any) {
	dryRun := len(r.URL.Query()["dryRun"]) == 1 && r.URL.Query()["dryRun"][0] == "true"
	if dryRun {
		h.lifecycleOpPreview(w, r, connectorID, verb, entityRef)
		return
	}
	h.lifecycleOpMutate(w, r, connectorID, verb, entityRef, extraAudit)
}

// LifecyclePreview describes the blast radius from the latest stored snapshot.
type LifecyclePreview struct {
	TargetService            string                        `json:"targetService"`
	EstimatedDowntimeSeconds int                           `json:"estimatedDowntimeSeconds"`
	DependentServices        []connector.ServiceDependency `json:"dependentServices"`
	AffectedEntities         []string                      `json:"affectedEntities"`
	UserDefined              bool                          `json:"userDefined"`
	Label                    string                        `json:"label,omitempty"`
	Description              string                        `json:"description,omitempty"`
	Request                  *connector.ActionRequest      `json:"request,omitempty"`
}

// LifecycleActor identifies the acting user explicitly, including the audit role.
type LifecycleActor struct {
	UserID        string
	InstanceAdmin bool
}

type lifecycleError struct {
	status  int
	code    string
	message string
	cause   error
}

func (e *lifecycleError) Error() string { return e.message }
func (e *lifecycleError) Unwrap() error { return e.cause }

type preparedLifecycleOp struct {
	record *store.ConnectorRecord
	config map[string]any
	conn   connector.Connector
	apply  func(context.Context, map[string]any, string) error
	action *connector.ResolvedAction
}

func writeLifecycleError(w http.ResponseWriter, err error) {
	var lifecycleErr *lifecycleError
	if errors.As(err, &lifecycleErr) {
		httputil.Error(w, lifecycleErr.status, lifecycleErr.code, lifecycleErr.message)
		return
	}
	httputil.Errorf(w, err)
}

func (h *Handler) lifecycleOpPreview(w http.ResponseWriter, r *http.Request, connectorID, verb, entityRef string) {
	preview, err := h.PreviewLifecycleOp(r.Context(), connectorID, verb, entityRef)
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, preview)
}

// PreviewLifecycleOp previews a connector or entity without mutating it.
// Callers must verify the connector operator grant first. HTTP preview routes
// remain elevation-free.
func (h *Handler) PreviewLifecycleOp(
	ctx context.Context,
	connectorID, verb, entityRef string,
) (*LifecyclePreview, error) {
	prepared, err := h.prepareLifecycleOp(ctx, connectorID, verb)
	if err != nil {
		return nil, err
	}
	snap, err := h.latestLifecycleSnapshot(ctx, connectorID)
	if err != nil {
		return nil, err
	}
	preview := lifecyclePreviewFromSnapshot(snap, entityRef, verb)
	if executor, ok := prepared.conn.(connector.ActionExecutor); ok {
		if err := connector.ValidateCompositeRef(entityRef); err != nil {
			return nil, invalidEntityRef(err)
		}
		resolved, err := executor.ResolveAction(prepared.config, verb, entityRef, &snap)
		if err != nil {
			return nil, resolveLifecycleActionError(prepared.conn, verb, entityRef, err)
		}
		request := custom.RedactedActionRequest(prepared.config, resolved)
		preview.UserDefined = true
		preview.Label = resolved.Descriptor.Label
		preview.Description = resolved.Descriptor.Description
		preview.EstimatedDowntimeSeconds = resolved.Descriptor.DowntimeSeconds
		preview.Request = &request
	}
	return preview, nil
}

func lifecyclePreviewFromSnapshot(snap connector.ServiceSnapshot, entityRef, verb string) *LifecyclePreview {
	targetService := snap.ServiceName
	affected := []string{}
	if entityRef != "" {
		for _, entity := range snap.Entities {
			if entity.ExternalID == entityRef {
				targetService = entity.Name
				affected = connectedDevices(entity)
				break
			}
		}
	}
	dependencies := snap.Dependencies
	if dependencies == nil {
		dependencies = []connector.ServiceDependency{}
	}
	downtime := restartPreviewDowntimeSeconds
	if verb == "stop" {
		// A stop's downtime is indefinite (no scheduled restart), not a
		// bounded estimate — 0 rather than inventing a new field.
		downtime = 0
	}
	return &LifecyclePreview{
		TargetService:            targetService,
		EstimatedDowntimeSeconds: downtime,
		DependentServices:        dependencies,
		AffectedEntities:         affected,
	}
}

func (h *Handler) latestLifecycleSnapshot(ctx context.Context, connectorID string) (connector.ServiceSnapshot, error) {
	record, err := h.Store.GetLatestSnapshot(ctx, connectorID)
	if errors.Is(err, store.ErrNotFound) {
		return connector.ServiceSnapshot{}, &lifecycleError{
			status:  http.StatusNotFound,
			code:    "not_found",
			message: "No snapshot available for connector",
			cause:   err,
		}
	}
	if err != nil {
		return connector.ServiceSnapshot{}, err
	}
	// Numbers decode as json.Number so an integer attribute keeps its exact
	// digits in an action placeholder (float64 would round 2^53+1). Unmarshal
	// into a RawMessage first so malformed input and trailing data fail with
	// the same errors as a plain Unmarshal.
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(record.Data), &raw); err != nil {
		return connector.ServiceSnapshot{}, fmt.Errorf("decode service snapshot: %w", err)
	}
	var snapshot connector.ServiceSnapshot
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&snapshot); err != nil {
		return connector.ServiceSnapshot{}, fmt.Errorf("decode service snapshot: %w", err)
	}
	return snapshot, nil
}

func invalidEntityRef(err error) error {
	return &lifecycleError{status: http.StatusBadRequest, code: "invalid_request", message: "invalid entityRef", cause: err}
}

func resolveLifecycleActionError(conn connector.Connector, verb, entityRef string, cause error) error {
	code := "invalid_request"
	message := cause.Error()
	if strings.Contains(message, "not declared for entity kind") {
		code = "unsupported_operation"
	}
	if entityRef == "" {
		for _, descriptor := range declaredActions(conn) {
			if descriptor.Name == verb && descriptor.EntityScope {
				message = "action requires an entityRef"
				break
			}
		}
	}
	return &lifecycleError{status: http.StatusBadRequest, code: code, message: message, cause: cause}
}

func declaredActions(conn connector.Connector) []connector.ActionDescriptor {
	if capabilities, ok := conn.(connector.InstanceCapabilities); ok {
		return capabilities.DeclaredActions()
	}
	return []connector.ActionDescriptor{}
}

// connectedDevices returns the names an entity powers or carries (its
// "connectedDevices" attribute, e.g. what hangs off a UniFi PoE port): the
// blast radius a preview must show alongside the target. Empty when the
// entity declares none.
func connectedDevices(e connector.SnapshotEntity) []string {
	out := []string{}
	switch v := e.Attributes["connectedDevices"].(type) {
	case []string:
		out = append(out, v...)
	case []any: // snapshot decoded from stored JSON
		for _, item := range v {
			if name, ok := item.(string); ok {
				out = append(out, name)
			}
		}
	}
	return out
}

// lifecycleOpMutate preserves the direct operation's validation and elevation order.
func (h *Handler) lifecycleOpMutate(w http.ResponseWriter, r *http.Request, connectorID, verb, entityRef string, extraAudit map[string]any) {
	prepared, err := h.prepareLifecycleOp(r.Context(), connectorID, verb)
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	if err := auth.ValidateElevationHeader(h.JWT, h.Store, "connector."+verb, r); err != nil {
		auth.WriteElevationError(w, err)
		return
	}
	if err := connector.ValidateCompositeRef(entityRef); err != nil {
		writeLifecycleError(w, invalidEntityRef(err))
		return
	}
	if executor, ok := prepared.conn.(connector.ActionExecutor); ok {
		resolved, err := resolvePreparedAction(r.Context(), h, prepared, executor, verb, entityRef, false)
		if err != nil {
			writeLifecycleError(w, err)
			return
		}
		prepared.action = resolved
	}
	actor := LifecycleActor{
		UserID:        auth.UserIDFromContext(r.Context()),
		InstanceAdmin: auth.InstanceAdminFromContext(r.Context()),
	}
	result, err := h.mutateLifecycleOp(r.Context(), prepared, verb, entityRef, actor, extraAudit, false)
	if err != nil {
		writeLifecycleActionError(w, err, result)
		return
	}
	response := map[string]any{"status": verb + "ed"}
	if prepared.action != nil {
		response["statusCode"] = result.Status
		if result.Excerpt != "" {
			response["excerpt"] = result.Excerpt
		}
	}
	httputil.JSON(w, http.StatusOK, response)
}

func writeLifecycleActionError(w http.ResponseWriter, err error, result connector.ActionResult) {
	var lifecycleErr *lifecycleError
	if !errors.As(err, &lifecycleErr) {
		httputil.Errorf(w, err)
		return
	}
	if result.Status == 0 && result.Excerpt == "" {
		writeLifecycleError(w, err)
		return
	}
	response := map[string]any{
		"code":    lifecycleErr.code,
		"message": lifecycleErr.message,
	}
	if result.Status != 0 {
		response["statusCode"] = result.Status
	}
	if result.Excerpt != "" {
		response["excerpt"] = result.Excerpt
	}
	httputil.JSON(w, lifecycleErr.status, response)
}

// MutateLifecycleOp performs an already-authorized lifecycle operation.
// Callers must verify the connector operator grant first; HTTP mutation
// handlers must also validate elevation. A failed operation creates a
// critical alert even if the caller's context was cancelled; success is
// audited. The core does not inspect elevation tokens or request actors.
func (h *Handler) MutateLifecycleOp(
	ctx context.Context,
	connectorID, verb, entityRef string,
	actor LifecycleActor,
	extraAudit map[string]any,
) error {
	_, err := h.mutateLifecycleOpCore(ctx, connectorID, verb, entityRef, actor, extraAudit, false)
	return err
}

// MutateRunbookLifecycleOp performs a lifecycle step for the run executor.
// Only this path suppresses a failure alert when the step ended because its
// run was cancelled or the server shut down.
func (h *Handler) MutateRunbookLifecycleOp(
	ctx context.Context,
	connectorID, verb, entityRef string,
	actor LifecycleActor,
	extraAudit map[string]any,
) error {
	_, err := h.mutateLifecycleOpCore(ctx, connectorID, verb, entityRef, actor, extraAudit, true)
	return err
}

func (h *Handler) mutateLifecycleOpCore(
	ctx context.Context,
	connectorID, verb, entityRef string,
	actor LifecycleActor,
	extraAudit map[string]any,
	suppressAbandonedAlert bool,
) (connector.ActionResult, error) {
	prepared, err := h.prepareLifecycleOp(ctx, connectorID, verb)
	if err != nil {
		return connector.ActionResult{}, err
	}
	if err := connector.ValidateCompositeRef(entityRef); err != nil {
		return connector.ActionResult{}, invalidEntityRef(err)
	}
	if executor, ok := prepared.conn.(connector.ActionExecutor); ok {
		resolved, err := resolvePreparedAction(ctx, h, prepared, executor, verb, entityRef, false)
		if err != nil {
			return connector.ActionResult{}, err
		}
		prepared.action = resolved
	}
	return h.mutateLifecycleOp(ctx, prepared, verb, entityRef, actor, extraAudit, suppressAbandonedAlert)
}

func (h *Handler) prepareLifecycleOp(ctx context.Context, connectorID, verb string) (*preparedLifecycleOp, error) {
	rec, err := h.Store.GetConnector(ctx, connectorID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, &lifecycleError{
			status:  http.StatusNotFound,
			code:    "not_found",
			message: "Connector not found",
			cause:   err,
		}
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

	fn, ok := connector.LifecycleOp(conn, verb)
	if !ok {
		return nil, &lifecycleError{
			status:  http.StatusBadRequest,
			code:    "unsupported_operation",
			message: "connector does not support " + verb,
		}
	}

	return &preparedLifecycleOp{record: rec, config: cfg, conn: conn, apply: fn}, nil
}

func (h *Handler) mutateLifecycleOp(
	ctx context.Context,
	prepared *preparedLifecycleOp,
	verb, entityRef string,
	actor LifecycleActor,
	extraAudit map[string]any,
	suppressAbandonedAlert bool,
) (connector.ActionResult, error) {
	rec := prepared.record
	connectorID := rec.ID
	if err := connector.ValidateCompositeRef(entityRef); err != nil {
		return connector.ActionResult{}, invalidEntityRef(err)
	}

	result, err := h.performLifecycleOp(ctx, prepared, verb, entityRef)
	if err != nil {
		failure := &lifecycleError{
			status:  http.StatusBadGateway,
			code:    verb + "_failed",
			message: err.Error(),
			cause:   err,
		}
		if suppressAbandonedAlert && abandonedByCaller(ctx, err) {
			slog.Info("connector "+logsafe.Sanitize(verb)+" abandoned by caller", "connector", logsafe.Sanitize(connectorID), "error", logsafe.Err(err))
			return result, failure
		}
		slog.Error("connector "+logsafe.Sanitize(verb)+" failed", "connector", logsafe.Sanitize(connectorID), "error", logsafe.Err(err))
		alert := &store.AlertRecord{
			ServiceID:   connectorID,
			Severity:    "critical",
			Title:       fmt.Sprintf("%s failed for %s", capitalize(verb), rec.Name),
			Description: err.Error(),
		}
		if createErr := h.Store.CreateAlert(context.WithoutCancel(ctx), alert); createErr != nil {
			slog.Error("failed to create "+verb+" failure alert", "error", createErr)
		} else if h.WSHub != nil {
			h.WSHub.BroadcastConnector(connectorID, ws.EventAlertCreated, map[string]any{
				"alertId":   alert.ID,
				"serviceId": connectorID,
				"severity":  alert.Severity,
				"title":     alert.Title,
			})
		}
		return result, failure
	}

	detail := make(map[string]any, len(extraAudit)+1)
	for k, v := range extraAudit {
		detail[k] = v
	}
	detail["entityRef"] = entityRef
	if prepared.action != nil {
		detail["method"] = prepared.action.Request.Method
		detail["url"] = connector.RedactURL(prepared.action.Request.URL)
		detail["status"] = result.Status
	}
	auditAction := "connector." + verb
	if err := h.recordLifecycleAudit(context.WithoutCancel(ctx), actor, auditAction, connectorID, detail); err != nil {
		slog.Error("failed to record audit", "action", auditAction, "error", err)
	}

	return result, nil
}

func resolvePreparedAction(
	ctx context.Context,
	h *Handler,
	prepared *preparedLifecycleOp,
	executor connector.ActionExecutor,
	name, entityRef string,
	loadServiceSnapshot bool,
) (*connector.ResolvedAction, error) {
	var snapshot *connector.ServiceSnapshot
	if entityRef != "" || loadServiceSnapshot {
		loaded, err := h.latestLifecycleSnapshot(ctx, prepared.record.ID)
		if err != nil {
			return nil, err
		}
		snapshot = &loaded
	}
	resolved, err := executor.ResolveAction(prepared.config, name, entityRef, snapshot)
	if err != nil {
		return nil, resolveLifecycleActionError(prepared.conn, name, entityRef, err)
	}
	return resolved, nil
}

func (h *Handler) performLifecycleOp(
	ctx context.Context,
	prepared *preparedLifecycleOp,
	verb, entityRef string,
) (connector.ActionResult, error) {
	if executor, ok := prepared.conn.(connector.ActionExecutor); ok {
		resolved := prepared.action
		if resolved == nil {
			var err error
			resolved, err = resolvePreparedAction(ctx, h, prepared, executor, verb, entityRef, false)
			if err != nil {
				return connector.ActionResult{}, err
			}
			prepared.action = resolved
		}
		return executor.SendAction(ctx, prepared.config, resolved)
	}
	if err := prepared.apply(ctx, prepared.config, entityRef); err != nil {
		return connector.ActionResult{}, err
	}
	return connector.ActionResult{}, nil
}

// abandonedByCaller reports whether err is the caller's own context error. The
// run executor suppresses alerts for this case because cancellation or shutdown
// says nothing about the connector; HTTP lifecycle calls still alert. A
// different error is a real failure even when the context is also done.
func abandonedByCaller(ctx context.Context, err error) bool {
	ctxErr := ctx.Err()
	return ctxErr != nil && errors.Is(err, ctxErr)
}

func (h *Handler) recordLifecycleAudit(
	ctx context.Context,
	actor LifecycleActor,
	action, connectorID string,
	detail map[string]any,
) error {
	return h.Store.RecordAuditAs(ctx, actor.UserID, actor.InstanceAdmin, action, "connector", connectorID, detail)
}

// capitalize upper-cases a word's first byte (ASCII verbs only: "restart",
// "start", "stop") for a display-friendly alert title.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// BulkRestart handles POST /api/connectors/bulk-restart. The router gates
// this route with one elevation check for the whole batch (action
// "connector.bulkRestart" — a distinct action from "connector.restart"
// since RequireElevation validates one token against one exact action
// string, not per item). Per ID, reuses the same restart logic as the
// single-connector Restart handler.
func (h *Handler) BulkRestart(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBulkRequest(w, r)
	if !ok {
		return
	}

	allowedIDs, found, results, err := h.splitByConnectorGrant(r.Context(), req.IDs)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	auditRecords := make([]store.AuditRecord, 0, len(allowedIDs))
	for _, id := range allowedIDs {
		rec := found[id]
		detail, err := h.restartConnector(r.Context(), &rec)
		if err != nil {
			results = append(results, bulkItemResult{ID: id, Status: "error", Reason: err.Error()})
			continue
		}
		auditRecord := store.AuditRecord{TargetID: id}
		if len(detail) != 0 {
			data, marshalErr := json.Marshal(detail)
			if marshalErr != nil {
				slog.Error("failed to marshal bulk restart audit detail", "connector", logsafe.Sanitize(id), "error", logsafe.Err(marshalErr))
			} else {
				auditRecord.Detail = string(data)
			}
		}
		auditRecords = append(auditRecords, auditRecord)
		results = append(results, bulkItemResult{ID: id, Status: "success"})
	}

	if err := h.Store.RecordAuditBatchFromContext(r.Context(), "connector.bulk_restart", "connector", auditRecords); err != nil {
		slog.Error("failed to record audit", "action", "connector.bulk_restart", "error", err)
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"results": results})
}

// restartConnector performs one connector's restart (config load,
// LifecycleOp lookup, call, failure-alert) without the per-call elevation
// check or per-call audit write — those are handled once, batch-wide, by
// BulkRestart. The single-connector RestartPreview/ServeLifecycleOp path
// still does its own per-call elevation + audit, since it isn't part of a
// batch.
func (h *Handler) restartConnector(ctx context.Context, rec *store.ConnectorRecord) (map[string]any, error) {
	cfg, err := store.ParseConnectorConfig(rec.Type, rec.ConfigData, h.Config.Encryption.Key)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	connector.ApplyRecordConfig(cfg, rec.URL, rec.VerifyTLS)

	conn, err := connector.Get(rec.Type, cfg)
	if err != nil {
		return nil, err
	}
	restart, ok := connector.LifecycleOp(conn, "restart")
	if !ok {
		return nil, fmt.Errorf("connector does not support restart")
	}

	detail := map[string]any{}
	var operationErr error
	if executor, ok := conn.(connector.ActionExecutor); ok {
		serviceRestart := false
		for _, action := range declaredActions(conn) {
			if action.Name == "restart" && !action.EntityScope {
				serviceRestart = true
				break
			}
		}
		if !serviceRestart {
			return nil, fmt.Errorf("connector does not support service restart")
		}
		resolved, resolveErr := executor.ResolveAction(cfg, "restart", "", nil)
		if resolveErr != nil {
			operationErr = resolveErr
		} else {
			result, sendErr := executor.SendAction(ctx, cfg, resolved)
			operationErr = sendErr
			if sendErr == nil {
				detail["method"] = resolved.Request.Method
				detail["url"] = connector.RedactURL(resolved.Request.URL)
				detail["status"] = result.Status
			}
		}
	} else {
		operationErr = restart(ctx, cfg, "")
	}
	if operationErr != nil {
		slog.Error("connector restart failed", "connector", logsafe.Sanitize(rec.ID), "error", logsafe.Err(operationErr))
		alert := &store.AlertRecord{
			ServiceID:   rec.ID,
			Severity:    "critical",
			Title:       fmt.Sprintf("Restart failed for %s", rec.Name),
			Description: operationErr.Error(),
		}
		if createErr := h.Store.CreateAlert(ctx, alert); createErr != nil {
			slog.Error("failed to create restart failure alert", "error", createErr)
		} else if h.WSHub != nil {
			h.WSHub.BroadcastConnector(rec.ID, ws.EventAlertCreated, map[string]any{
				"alertId":   alert.ID,
				"serviceId": rec.ID,
				"severity":  alert.Severity,
				"title":     alert.Title,
			})
		}
		return nil, operationErr
	}
	if len(detail) == 0 {
		return nil, nil
	}
	return detail, nil
}
