package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/custom"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/logsafe"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

type actionRequestBody struct {
	EntityRef string `json:"entityRef"`
}

type preparedAction struct {
	record   *store.ConnectorRecord
	config   map[string]any
	executor connector.ActionExecutor
	resolved *connector.ResolvedAction
}

// Action handles POST /api/connectors/{id}/actions/{name}. A dry run resolves
// the fixed recipe request without contacting the service; a mutation sends
// that same resolved request after the connector.action elevation check.
func (h *Handler) Action(w http.ResponseWriter, r *http.Request) {
	var body actionRequestBody
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			httputil.Error(w, http.StatusBadRequest, "invalid_request", "invalid request body")
			return
		}
	}
	h.ServeActionOp(w, r, r.PathValue("id"), r.PathValue("name"), body.EntityRef)
}

// ServeActionOp separates the endpoint parser from its behavior so an action
// can be invoked by other in-process executors without reinterpreting an HTTP
// body. Callers exposing this over HTTP must enforce the connector operator
// grant before calling it.
func (h *Handler) ServeActionOp(w http.ResponseWriter, r *http.Request, connectorID, name, entityRef string) {
	dryRun := len(r.URL.Query()["dryRun"]) == 1 && r.URL.Query()["dryRun"][0] == "true"
	if dryRun {
		h.actionPreview(w, r, connectorID, name, entityRef)
		return
	}
	h.actionMutate(w, r, connectorID, name, entityRef)
}

func (h *Handler) actionPreview(w http.ResponseWriter, r *http.Request, connectorID, name, entityRef string) {
	preview, err := h.PreviewNamedAction(r.Context(), connectorID, name, entityRef)
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, preview)
}

// PreviewNamedAction resolves one named action's request for a dry run. It
// contacts the service only to read the snapshot an entity-scoped action needs,
// and sends nothing.
func (h *Handler) PreviewNamedAction(ctx context.Context, connectorID, name, entityRef string) (*LifecyclePreview, error) {
	prepared, snapshot, err := h.prepareNamedAction(ctx, connectorID, name, entityRef, true)
	if err != nil {
		return nil, err
	}
	preview := lifecyclePreviewFromSnapshot(snapshot, entityRef, name)
	preview.UserDefined = true
	preview.Label = prepared.resolved.Descriptor.Label
	preview.Description = prepared.resolved.Descriptor.Description
	preview.EstimatedDowntimeSeconds = prepared.resolved.Descriptor.DowntimeSeconds
	request := custom.RedactedActionRequest(prepared.config, prepared.resolved)
	preview.Request = &request
	return preview, nil
}

// ActionFingerprint returns the fingerprint of the named action's definition as
// the connector's recipe declares it now. It resolves the request without
// sending anything.
func (h *Handler) ActionFingerprint(ctx context.Context, connectorID, name, entityRef string) (string, error) {
	prepared, _, err := h.prepareNamedAction(ctx, connectorID, name, entityRef, false)
	if err != nil {
		return "", err
	}
	return prepared.resolved.Fingerprint, nil
}

func (h *Handler) actionMutate(w http.ResponseWriter, r *http.Request, connectorID, name, entityRef string) {
	prepared, _, err := h.prepareNamedAction(r.Context(), connectorID, name, entityRef, false)
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	target := connectorID + ":" + name
	if err := auth.ValidateElevationHeaderFor(h.JWT, h.Store, "connector.action", target, r); err != nil {
		auth.WriteElevationError(w, err)
		return
	}
	actor := LifecycleActor{
		UserID:        auth.UserIDFromContext(r.Context()),
		InstanceAdmin: auth.InstanceAdminFromContext(r.Context()),
	}
	result, err := h.mutateAction(r.Context(), prepared, name, entityRef, actor, nil, false)
	if err != nil {
		writeLifecycleActionError(w, err, result)
		return
	}
	response := map[string]any{"status": result.Status}
	if result.Excerpt != "" {
		response["excerpt"] = result.Excerpt
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *Handler) prepareNamedAction(
	ctx context.Context,
	connectorID, name, entityRef string,
	includeServiceSnapshot bool,
) (*preparedAction, connector.ServiceSnapshot, error) {
	record, err := h.Store.GetConnector(ctx, connectorID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, connector.ServiceSnapshot{}, &lifecycleError{
			status:  http.StatusNotFound,
			code:    "not_found",
			message: "Connector not found",
			cause:   err,
		}
	}
	if err != nil {
		return nil, connector.ServiceSnapshot{}, err
	}
	config, err := store.ParseConnectorConfig(record.Type, record.ConfigData, h.Config.Encryption.Key)
	if err != nil {
		return nil, connector.ServiceSnapshot{}, fmt.Errorf("parse config: %w", err)
	}
	connector.ApplyRecordConfig(config, record.URL, record.VerifyTLS)
	conn, err := connector.Get(record.Type, config)
	if err != nil {
		return nil, connector.ServiceSnapshot{}, err
	}
	executor, ok := conn.(connector.ActionExecutor)
	if !ok {
		return nil, connector.ServiceSnapshot{}, unsupportedNamedAction(name)
	}
	if isLifecycleVerb(name) {
		return nil, connector.ServiceSnapshot{}, unsupportedNamedAction(name)
	}
	if err := validateNamedActionScope(conn, name, entityRef); err != nil {
		return nil, connector.ServiceSnapshot{}, err
	}

	var snapshot connector.ServiceSnapshot
	var snapshotPtr *connector.ServiceSnapshot
	if includeServiceSnapshot || entityRef != "" {
		snapshot, err = h.latestLifecycleSnapshot(ctx, connectorID)
		if err != nil {
			return nil, connector.ServiceSnapshot{}, err
		}
		snapshotPtr = &snapshot
		if entityRef != "" {
			if err := validateNamedActionTarget(conn, name, entityRef, snapshot); err != nil {
				return nil, connector.ServiceSnapshot{}, err
			}
		}
	}
	resolved, err := executor.ResolveAction(config, name, entityRef, snapshotPtr)
	if err != nil {
		return nil, connector.ServiceSnapshot{}, &lifecycleError{
			status:  http.StatusBadRequest,
			code:    "invalid_request",
			message: err.Error(),
			cause:   err,
		}
	}
	return &preparedAction{record: record, config: config, executor: executor, resolved: resolved}, snapshot, nil
}

func validateNamedActionTarget(
	conn connector.Connector,
	name, entityRef string,
	snapshot connector.ServiceSnapshot,
) error {
	entityFound := false
	entityKind := ""
	for _, entity := range snapshot.Entities {
		if entity.ExternalID != entityRef {
			continue
		}
		entityFound = true
		if entityKind == "" {
			entityKind = entity.Kind
		}
		for _, action := range declaredActions(conn) {
			if action.Name == name && action.EntityScope && action.EntityKind == entity.Kind {
				return nil
			}
		}
	}
	if !entityFound {
		return &lifecycleError{
			status:  http.StatusBadRequest,
			code:    "invalid_request",
			message: fmt.Sprintf("entity reference %q was not found in the latest snapshot", entityRef),
		}
	}
	return &lifecycleError{
		status:  http.StatusBadRequest,
		code:    "unsupported_operation",
		message: fmt.Sprintf("action %q is not declared for entity kind %q", name, entityKind),
	}
}

func validateNamedActionScope(conn connector.Connector, name, entityRef string) error {
	declared := false
	serviceScope := false
	entityScope := false
	for _, action := range declaredActions(conn) {
		if action.Name != name {
			continue
		}
		declared = true
		if action.EntityScope {
			entityScope = true
		} else {
			serviceScope = true
		}
	}
	if !declared {
		return unsupportedNamedAction(name)
	}
	if entityRef == "" && !serviceScope {
		return &lifecycleError{
			status:  http.StatusBadRequest,
			code:    "invalid_request",
			message: "action requires an entityRef",
		}
	}
	if entityRef != "" && !entityScope {
		return &lifecycleError{
			status:  http.StatusBadRequest,
			code:    "invalid_request",
			message: "action is not declared for entities",
		}
	}
	return nil
}

func unsupportedNamedAction(name string) error {
	return &lifecycleError{
		status:  http.StatusBadRequest,
		code:    "unsupported_operation",
		message: "connector does not support action " + name,
	}
}

func isLifecycleVerb(name string) bool {
	for _, verb := range connector.LifecycleVerbs {
		if name == verb {
			return true
		}
	}
	return false
}

// ErrActionChanged reports that a runbook step's named action no longer
// resolves to the fingerprint frozen when its run started. Nothing was sent.
var ErrActionChanged = errors.New("the action changed since the run started")

// MutateRunbookAction executes one recipe-declared named action for an
// already-authorized runbook step. When expectedFingerprint is not empty and
// the action's current fingerprint differs, it returns an error wrapping
// ErrActionChanged and sends nothing, with no alert and no audit entry. The
// returned result carries the upstream status, excerpt, and internal Written
// marker; callers must not persist or log the excerpt.
func (h *Handler) MutateRunbookAction(
	ctx context.Context,
	connectorID, name, entityRef, expectedFingerprint string,
	actor LifecycleActor,
	extraAudit map[string]any,
) (connector.ActionResult, error) {
	prepared, _, err := h.prepareNamedAction(ctx, connectorID, name, entityRef, false)
	if err != nil {
		return connector.ActionResult{}, err
	}
	if expectedFingerprint != "" && prepared.resolved.Fingerprint != expectedFingerprint {
		return connector.ActionResult{}, fmt.Errorf("action %q: %w", name, ErrActionChanged)
	}
	return h.mutateAction(ctx, prepared, name, entityRef, actor, extraAudit, true)
}

func (h *Handler) mutateAction(
	ctx context.Context,
	prepared *preparedAction,
	name, entityRef string,
	actor LifecycleActor,
	extraAudit map[string]any,
	suppressAbandonedAlert bool,
) (connector.ActionResult, error) {
	result, err := prepared.executor.SendAction(ctx, prepared.config, prepared.resolved)
	if err != nil {
		failure := &lifecycleError{
			status:  http.StatusBadGateway,
			code:    "action_failed",
			message: err.Error(),
			cause:   err,
		}
		if suppressAbandonedAlert && abandonedByCaller(ctx, err) {
			slog.Info("connector action abandoned by caller", "connector", logsafe.Sanitize(prepared.record.ID), "action", logsafe.Sanitize(name), "error", logsafe.Err(err))
			return result, failure
		}
		h.recordActionFailure(ctx, prepared.record, name, err)
		return result, failure
	}
	detail := make(map[string]any, len(extraAudit)+5)
	for key, value := range extraAudit {
		detail[key] = value
	}
	detail["action"] = name
	if entityRef != "" {
		detail["entityRef"] = entityRef
	}
	detail["method"] = prepared.resolved.Request.Method
	detail["url"] = connector.RedactURL(prepared.resolved.Request.URL)
	detail["status"] = result.Status
	if err := h.Store.RecordAuditAs(context.WithoutCancel(ctx), actor.UserID, actor.InstanceAdmin, "connector.action", "connector", prepared.record.ID, detail); err != nil {
		slog.Error("failed to record audit", "action", "connector.action", "error", err)
	}
	return result, nil
}

func (h *Handler) recordActionFailure(ctx context.Context, record *store.ConnectorRecord, name string, err error) {
	slog.Error("connector action failed", "connector", logsafe.Sanitize(record.ID), "action", logsafe.Sanitize(name), "error", logsafe.Err(err))
	alert := &store.AlertRecord{
		ServiceID:   record.ID,
		Severity:    "critical",
		Title:       fmt.Sprintf("Action %s failed for %s", name, record.Name),
		Description: err.Error(),
	}
	if createErr := h.Store.CreateAlert(context.WithoutCancel(ctx), alert); createErr != nil {
		slog.Error("failed to create action failure alert", "error", createErr)
		return
	}
	if h.WSHub != nil {
		h.WSHub.BroadcastConnector(record.ID, ws.EventAlertCreated, map[string]any{
			"alertId":   alert.ID,
			"serviceId": record.ID,
			"severity":  alert.Severity,
			"title":     alert.Title,
		})
	}
}
