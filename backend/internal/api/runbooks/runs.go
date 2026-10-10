package runbooks

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/connectors"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/logsafe"
	"github.com/WiseLabz/wiselabz/internal/runbookrun"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// RunStepResponse carries frozen metadata only when the caller can view its connector.
type RunStepResponse struct {
	ID                   string                       `json:"id"`
	Position             int                          `json:"position"`
	Kind                 string                       `json:"kind,omitempty"`
	Title                string                       `json:"title,omitempty"`
	ConnectorID          string                       `json:"connectorId,omitempty"`
	ConnectorName        string                       `json:"connectorName,omitempty"`
	Verb                 string                       `json:"verb,omitempty"`
	EntityRef            string                       `json:"entityRef,omitempty"`
	Action               string                       `json:"action,omitempty"`
	FieldKey             string                       `json:"fieldKey,omitempty"`
	TargetValue          string                       `json:"targetValue,omitempty"`
	Attribute            string                       `json:"attribute,omitempty"`
	Operator             string                       `json:"operator,omitempty"`
	ExpectedValue        string                       `json:"expectedValue,omitempty"`
	TimeoutSeconds       int                          `json:"timeoutSeconds,omitempty"`
	State                string                       `json:"state,omitempty"`
	StartedAt            string                       `json:"startedAt,omitempty"`
	FinishedAt           string                       `json:"finishedAt,omitempty"`
	Error                string                       `json:"error,omitempty"`
	ConfirmedBy          string                       `json:"confirmedBy,omitempty"`
	Redacted             bool                         `json:"redacted"`
	CanExecute           bool                         `json:"canExecute"`
	ExecuteBlockedReason string                       `json:"executeBlockedReason,omitempty"`
	Preview              *connectors.LifecyclePreview `json:"preview,omitempty"`
	CurrentValue         any                          `json:"currentValue,omitempty"`
	CurrentValueKnown    *bool                        `json:"currentValueKnown,omitempty"`
}

// RunResponse is the history projection shared by HTTP and read-only MCP tools.
type RunResponse struct {
	store.RunbookRunRecord
	Steps             []RunStepResponse `json:"steps"`
	CanApprove        *bool             `json:"canApprove,omitempty"`
	ApprovalExpiresAt string            `json:"approvalExpiresAt,omitempty"`
}

type runPreviewResponse struct {
	ID                string            `json:"id"`
	CanStart          bool              `json:"canStart"`
	Steps             []RunStepResponse `json:"steps"`
	RequiresApproval  bool              `json:"requiresApproval"`
	ApproverAvailable bool              `json:"approverAvailable"`
}

// runAuthorized checks all distinct frozen connectors before inspecting state
// or elevation. When none remain to check, an instance admin or an operator on
// any connector may act. Store role checks apply API-key connector restrictions
// too. CancelRun alone passes only the steps whose connector still exists.
func (h *Handler) runAuthorized(w http.ResponseWriter, r *http.Request, steps []*store.RunbookRunStepRecord) bool {
	checked := map[string]bool{}
	for _, step := range steps {
		if step.ConnectorID == "" || checked[step.ConnectorID] {
			continue
		}
		checked[step.ConnectorID] = true
		ok, err := h.Store.UserHasConnectorRole(r.Context(), auth.UserIDFromContext(r.Context()), step.ConnectorID, "operator")
		if err != nil {
			httputil.Errorf(w, err)
			return false
		}
		if !ok {
			httputil.Error(w, http.StatusForbidden, "forbidden", "insufficient permissions")
			return false
		}
	}
	if len(checked) == 0 {
		ok, err := h.connectorlessRunAuthorized(r.Context(), auth.UserIDFromContext(r.Context()))
		if err != nil {
			httputil.Errorf(w, err)
			return false
		}
		if !ok {
			httputil.Error(w, http.StatusForbidden, "forbidden", "insufficient permissions")
			return false
		}
	}
	// A read-only key must not mutate run history, including on manual-only runs.
	if auth.APIKeyRestrictionFromContext(r.Context()).ReadOnly {
		httputil.Error(w, http.StatusForbidden, "forbidden", "insufficient permissions")
		return false
	}
	return true
}

// connectorlessRunAuthorized prevents manual-only runs (and cancellations
// after every connector was deleted) from becoming writable by any user.
func (h *Handler) connectorlessRunAuthorized(ctx context.Context, userID string) (bool, error) {
	if auth.InstanceAdminFromContext(ctx) {
		return true, nil
	}
	return h.Store.UserHasAnyConnectorRole(ctx, userID, "operator")
}

func writeRunError(w http.ResponseWriter, err error) {
	var active *store.RunbookRunConflictError
	switch {
	case errors.As(err, &active):
		httputil.JSON(w, http.StatusConflict, map[string]string{"code": "conflict", "message": "An active run already exists", "runId": active.RunID})
	case errors.Is(err, store.ErrConflict):
		httputil.Error(w, http.StatusConflict, "conflict", "Run or step is not in the required state")
	case errors.Is(err, store.ErrNotFound):
		httputil.Error(w, http.StatusNotFound, "not_found", "Run or runbook not found")
	case errors.Is(err, runbookrun.ErrShuttingDown):
		httputil.Error(w, http.StatusServiceUnavailable, "shutting_down", "The server is shutting down")
	case errors.Is(err, runbookrun.ErrNoEligibleApprover):
		httputil.Error(w, http.StatusConflict, "no_eligible_approver", "No other eligible operator is available to approve this run")
	case errors.Is(err, store.ErrRunbookRunStepCount):
		httputil.Error(w, http.StatusBadRequest, "invalid_steps", "A runbook run needs between 1 and 20 steps")
	case errors.Is(err, runbookrun.ErrActionUnavailable):
		httputil.Error(w, http.StatusConflict, "action_unavailable", "A connector action of this run cannot be prepared; check that its recipe still declares it")
	case errors.Is(err, runbookrun.ErrDecisionRequired):
		httputil.Error(w, http.StatusConflict, "unknown_step_decision_required", "The unknown connector action step needs a decision to resume: resend or mark_done")
	case errors.Is(err, runbookrun.ErrInvalidDecision):
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "decision must be resend or mark_done", []httputil.FieldError{{Field: "decision", Msg: "must be resend or mark_done"}})
	default:
		httputil.Errorf(w, err)
	}
}

// RunView applies the same grant and API-key visibility rules to HTTP and MCP.
// Hidden steps contain no authored metadata, errors or lifecycle previews.
func (h *Handler) RunView(ctx context.Context, run *store.RunbookRunRecord, steps []*store.RunbookRunStepRecord) (RunResponse, error) {
	views, err := h.runStepViews(ctx, steps)
	if err != nil {
		return RunResponse{}, err
	}
	response := RunResponse{RunbookRunRecord: *run, Steps: views}
	if run.RequiresApproval {
		canApprove := false
		if run.State == runbookrun.RunAwaitingApproval {
			canApprove, err = h.canApproveRun(ctx, run, steps)
			if err != nil {
				return RunResponse{}, err
			}
			settings, err := h.Store.GetRetentionSettings(ctx)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return RunResponse{}, err
			}
			hours := store.DefaultRunbookApprovalHours
			if err == nil && settings.RunbookApprovalHours > 0 {
				hours = settings.RunbookApprovalHours
			}
			updatedAt, err := time.Parse(time.RFC3339Nano, run.UpdatedAt)
			if err != nil {
				return RunResponse{}, err
			}
			response.ApprovalExpiresAt = updatedAt.Add(time.Duration(hours) * time.Hour).UTC().Format(time.RFC3339Nano)
		}
		response.CanApprove = &canApprove
	}
	return response, nil
}

func (h *Handler) runStepViews(ctx context.Context, steps []*store.RunbookRunStepRecord) ([]RunStepResponse, error) {
	views := make([]RunStepResponse, 0, len(steps))
	roles := map[string]string{}
	names := map[string]string{}
	hasConnector := false
	for _, step := range steps {
		if step.ConnectorID != "" {
			hasConnector = true
			break
		}
	}
	connectorlessCanExecute := false
	if len(steps) > 0 && !hasConnector {
		var err error
		connectorlessCanExecute, err = h.connectorlessRunAuthorized(ctx, auth.UserIDFromContext(ctx))
		if err != nil {
			return nil, err
		}
	}
	for _, step := range steps {
		view := RunStepResponse{ID: step.ID, Position: step.Position, State: step.State, StartedAt: step.StartedAt, FinishedAt: step.FinishedAt, ConfirmedBy: step.ConfirmedBy}
		role := "operator"
		if step.ConnectorID != "" {
			var err error
			role, err = h.connectorRole(ctx, auth.UserIDFromContext(ctx), step.ConnectorID, roles)
			if err != nil {
				return nil, err
			}
		}
		if role == "" {
			view.Redacted = true
			view.ExecuteBlockedReason = "no_viewer_grant"
			views = append(views, view)
			continue
		}
		view.Kind = effectiveKind(step.Kind)
		view.Title = step.Title
		view.ConnectorID = step.ConnectorID
		view.Verb = step.Verb
		view.EntityRef = step.EntityRef
		view.Action = step.Action
		view.FieldKey = step.FieldKey
		view.TargetValue = step.TargetValue
		view.Attribute = step.Attribute
		view.Operator = step.Operator
		view.ExpectedValue = step.ExpectedValue
		view.TimeoutSeconds = reportedTimeout(view.Kind, step.TimeoutSeconds)
		view.Error = step.Error
		view.CanExecute = role == "operator" && !auth.APIKeyRestrictionFromContext(ctx).ReadOnly
		if step.ConnectorID == "" && !hasConnector {
			view.CanExecute = connectorlessCanExecute && !auth.APIKeyRestrictionFromContext(ctx).ReadOnly
		}
		if !view.CanExecute {
			view.ExecuteBlockedReason = "no_operator_grant"
		}
		if step.ConnectorID != "" {
			name, found := names[step.ConnectorID]
			if !found {
				conn, err := h.Store.GetConnector(ctx, step.ConnectorID)
				if err != nil && !errors.Is(err, store.ErrNotFound) {
					return nil, err
				}
				if conn != nil {
					name = conn.Name
				}
				names[step.ConnectorID] = name
			}
			view.ConnectorName = name
		}
		views = append(views, view)
	}
	return views, nil
}

// StartRun previews with dryRun=true or starts one asynchronously after one elevation.
func (h *Handler) StartRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	steps, err := h.Store.ListRunbookStepsFor(r.Context(), id)
	if err != nil {
		writeRunError(w, err)
		return
	}
	frozen := runbookrun.FreezeSteps(steps)
	if r.URL.Query().Get("dryRun") == "true" {
		if _, err := h.Store.GetRunbook(r.Context(), id); err != nil {
			writeRunError(w, err)
			return
		}
		h.previewRun(w, r, id, steps, frozen)
		return
	}
	if !h.runAuthorized(w, r, frozen) {
		return
	}
	runbook, err := h.Store.GetRunbook(r.Context(), id)
	if err != nil {
		writeRunError(w, err)
		return
	}
	if len(steps) == 0 {
		writeRunError(w, store.ErrRunbookRunStepCount)
		return
	}
	if runbook.RequiresApproval {
		approvers, err := h.Store.EligibleRunbookApprovers(r.Context(), runConnectorIDs(frozen), auth.UserIDFromContext(r.Context()))
		if err != nil {
			httputil.Errorf(w, err)
			return
		}
		if len(approvers) == 0 {
			httputil.Error(w, http.StatusConflict, "no_eligible_approver", "No other eligible operator is available to approve this run")
			return
		}
	}
	if err := auth.ValidateElevationHeaderFor(h.ConnH.JWT, h.Store, "runbook.run", id, r); err != nil {
		auth.WriteElevationError(w, err)
		return
	}
	start := h.Executor.Start
	action := "runbook.run.start"
	if runbook.RequiresApproval {
		start = h.Executor.Request
		action = "runbook.run.approval_requested"
	}
	run, saved, err := start(r.Context(), id, auth.UserIDFromContext(r.Context()), steps)
	if err != nil {
		h.auditShutdownTransition(r, action, err, "")
		writeRunError(w, err)
		return
	}
	h.auditRun(r, action, run, "", runConnectorIDs(saved))
	resp, err := h.RunView(r.Context(), run, saved)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusAccepted, resp)
}

const (
	previewOverallTimeout = 5 * time.Second
	previewStepTimeout    = 2 * time.Second
	previewMaxConcurrency = 4
)

func (h *Handler) previewRun(w http.ResponseWriter, r *http.Request, id string, authored []*store.RunbookStepRecord, frozen []*store.RunbookRunStepRecord) {
	// FreezeSteps intentionally copies execution fields only, so preview IDs and
	// positions come from the authored rows rather than newly created run rows.
	for i, step := range frozen {
		step.ID = authored[i].ID
		step.Position = authored[i].Position
	}
	views, err := h.runStepViews(r.Context(), frozen)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	previewCtx, cancel := context.WithTimeout(r.Context(), previewOverallTimeout)
	defer cancel()

	sem := make(chan struct{}, previewMaxConcurrency)
	var wg sync.WaitGroup

	for i := range views {
		view := &views[i]
		if view.Redacted {
			continue
		}
		if view.Kind == kindLifecycle && view.CanExecute {
			wg.Add(1)
			go func(v *RunStepResponse) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				stepCtx, stepCancel := context.WithTimeout(previewCtx, previewStepTimeout)
				defer stepCancel()

				preview, err := h.ConnH.PreviewLifecycleOp(stepCtx, v.ConnectorID, v.Verb, v.EntityRef)
				if err != nil {
					slog.Warn("runbook run preview failed", "error", logsafe.Err(err))
					v.CanExecute = false
					v.ExecuteBlockedReason = "preview_unavailable"
				} else {
					v.Preview = preview
				}
			}(view)
		} else if view.Kind == kindConnectorAction && view.CanExecute {
			wg.Add(1)
			go func(v *RunStepResponse) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				stepCtx, stepCancel := context.WithTimeout(previewCtx, previewStepTimeout)
				defer stepCancel()

				// The preview resolves the request the step would send without sending it.
				preview, err := h.ConnH.PreviewNamedAction(stepCtx, v.ConnectorID, v.Action, v.EntityRef)
				if err != nil {
					slog.Warn("runbook run preview failed", "error", logsafe.Err(err))
					v.CanExecute = false
					v.ExecuteBlockedReason = "preview_unavailable"
				} else {
					v.Preview = preview
				}
			}(view)
		} else if view.Kind == kindConfigPush {
			wg.Add(1)
			go func(v *RunStepResponse) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				stepCtx, stepCancel := context.WithTimeout(previewCtx, previewStepTimeout)
				defer stepCancel()

				h.previewConfigPushStep(stepCtx, v)
			}(view)
		}
	}
	wg.Wait()

	canStart := len(views) > 0
	for i := range views {
		if !views[i].CanExecute {
			canStart = false
			break
		}
	}
	runbook, err := h.Store.GetRunbook(r.Context(), id)
	if err != nil {
		writeRunError(w, err)
		return
	}
	approverAvailable := true
	if runbook.RequiresApproval {
		approvers, err := h.Store.EligibleRunbookApprovers(r.Context(), runConnectorIDs(frozen), auth.UserIDFromContext(r.Context()))
		if err != nil {
			httputil.Errorf(w, err)
			return
		}
		approverAvailable = len(approvers) > 0
	}
	httputil.JSON(w, http.StatusOK, runPreviewResponse{
		ID: id, CanStart: canStart, Steps: views,
		RequiresApproval: runbook.RequiresApproval, ApproverAvailable: approverAvailable,
	})
}

func (h *Handler) previewConfigPushStep(ctx context.Context, view *RunStepResponse) {
	conn, rec, cfg, err := h.resolveConnector(ctx, view.ConnectorID)
	if err != nil || rec == nil {
		if view.ExecuteBlockedReason == "" {
			view.ExecuteBlockedReason = "preview_unavailable"
		}
		view.CanExecute = false
		known := false
		view.CurrentValueKnown = &known
		return
	}
	pusher, ok := conn.(connector.ConfigPusher)
	if !ok {
		if view.ExecuteBlockedReason == "" {
			view.ExecuteBlockedReason = "unsupported_field"
		}
		view.CanExecute = false
		known := false
		view.CurrentValueKnown = &known
		return
	}
	var targetField *connector.ConfigField
	for _, wf := range pusher.WritableFields() {
		if wf.Key == view.FieldKey {
			targetField = &wf
			break
		}
	}
	if targetField == nil {
		if view.ExecuteBlockedReason == "" {
			view.ExecuteBlockedReason = "unsupported_field"
		}
		view.CanExecute = false
		known := false
		view.CurrentValueKnown = &known
		return
	}
	if store.IsSecretFieldType(targetField.Type) {
		known := false
		view.CurrentValueKnown = &known
		return
	}
	reader, ok := conn.(connector.ConfigReader)
	if !ok {
		known := false
		view.CurrentValueKnown = &known
		return
	}
	val, err := reader.ConfigRead(ctx, cfg, view.EntityRef, view.FieldKey)
	if err != nil || val == nil {
		if err != nil {
			slog.Warn("runbook run preview config read failed", "connector", logsafe.Sanitize(view.ConnectorID), "field", logsafe.Sanitize(view.FieldKey), "error", logsafe.Err(err))
		}
		known := false
		view.CurrentValueKnown = &known
		return
	}
	known := true
	view.CurrentValueKnown = &known
	view.CurrentValue = val
}

func (h *Handler) resolveConnector(ctx context.Context, connectorID string) (connector.Connector, *store.ConnectorRecord, map[string]any, error) {
	rec, err := h.Store.GetConnector(ctx, connectorID)
	if err != nil {
		return nil, nil, nil, err
	}
	var encKey string
	if h.ConnH != nil && h.ConnH.Config != nil {
		encKey = h.ConnH.Config.Encryption.Key
	}
	cfg, err := store.ParseConnectorConfig(rec.Type, rec.ConfigData, encKey)
	if err != nil {
		return nil, rec, nil, err
	}
	connector.ApplyRecordConfig(cfg, rec.URL, rec.VerifyTLS)
	conn, err := connector.Get(rec.Type, cfg)
	if err != nil {
		return nil, rec, cfg, err
	}
	return conn, rec, cfg, nil
}

// ListRuns returns paginated history newest first, with per-caller step redaction.
func (h *Handler) ListRuns(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := h.Store.GetRunbook(r.Context(), id); err != nil {
		writeRunError(w, err)
		return
	}
	page, size, offset := httputil.Paginate(r)
	runs, total, err := h.Store.ListRunbookRuns(r.Context(), id, size, offset)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	views := make([]RunResponse, 0, len(runs))
	for _, run := range runs {
		_, steps, err := h.Store.GetRunbookRun(r.Context(), run.ID)
		if err != nil {
			writeRunError(w, err)
			return
		}
		view, err := h.RunView(r.Context(), run, steps)
		if err != nil {
			httputil.Errorf(w, err)
			return
		}
		views = append(views, view)
	}
	httputil.WritePaginated(w, views, page, size, total)
}

// GetRun reads frozen run history independently of its authored runbook.
func (h *Handler) GetRun(w http.ResponseWriter, r *http.Request) {
	run, steps, err := h.Store.GetRunbookRun(r.Context(), r.PathValue("runId"))
	if err != nil {
		writeRunError(w, err)
		return
	}
	view, err := h.RunView(r.Context(), run, steps)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, view)
}

func (h *Handler) authorizedRun(w http.ResponseWriter, r *http.Request) (*store.RunbookRunRecord, []*store.RunbookRunStepRecord, bool) {
	run, steps, err := h.Store.GetRunbookRun(r.Context(), r.PathValue("runId"))
	if err != nil {
		writeRunError(w, err)
		return nil, nil, false
	}
	return run, steps, h.runAuthorized(w, r, steps)
}

func (h *Handler) canApproveRun(ctx context.Context, run *store.RunbookRunRecord, steps []*store.RunbookRunStepRecord) (bool, error) {
	userID := auth.UserIDFromContext(ctx)
	if userID == "" || userID == run.StartedBy || auth.APIKeyRestrictionFromContext(ctx).ReadOnly {
		return false, nil
	}
	user, err := h.Store.GetUserByIDFromWriter(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if user.Disabled {
		return false, nil
	}
	checked := map[string]bool{}
	for _, step := range steps {
		if step.ConnectorID == "" || checked[step.ConnectorID] {
			continue
		}
		checked[step.ConnectorID] = true
		ok, err := h.Store.UserHasConnectorRole(ctx, userID, step.ConnectorID, "operator")
		if err != nil || !ok {
			return false, err
		}
	}
	if len(checked) == 0 {
		return h.connectorlessRunAuthorized(ctx, userID)
	}
	return true, nil
}

func (h *Handler) approvalRun(w http.ResponseWriter, r *http.Request) (*store.RunbookRunRecord, bool) {
	run, steps, err := h.Store.GetRunbookRun(r.Context(), r.PathValue("runId"))
	if err != nil {
		writeRunError(w, err)
		return nil, false
	}
	if !h.runAuthorized(w, r, steps) {
		return nil, false
	}
	eligible, err := h.canApproveRun(r.Context(), run, steps)
	if err != nil {
		httputil.Errorf(w, err)
		return nil, false
	}
	if !eligible {
		httputil.Error(w, http.StatusForbidden, "forbidden", "insufficient permissions")
		return nil, false
	}
	if run.State != runbookrun.RunAwaitingApproval {
		writeRunError(w, store.ErrConflict)
		return nil, false
	}
	return run, true
}

// ApproveRun requires a different operator's own elevation on the frozen run.
func (h *Handler) ApproveRun(w http.ResponseWriter, r *http.Request) {
	run, ok := h.approvalRun(w, r)
	if !ok {
		return
	}
	if err := auth.ValidateElevationHeaderFor(h.ConnH.JWT, h.Store, "runbook.approve", run.ID, r); err != nil {
		auth.WriteElevationError(w, err)
		return
	}
	approved, steps, err := h.Executor.Approve(r.Context(), run.ID, auth.UserIDFromContext(r.Context()))
	if err != nil {
		h.auditShutdownTransition(r, "runbook.run.approved", err, "")
		writeRunError(w, err)
		return
	}
	h.auditRun(r, "runbook.run.approved", approved, "", runConnectorIDs(steps))
	httputil.NoContent(w)
}

// RejectRun does not require elevation and never starts execution.
func (h *Handler) RejectRun(w http.ResponseWriter, r *http.Request) {
	run, ok := h.approvalRun(w, r)
	if !ok {
		return
	}
	rejected, steps, err := h.Executor.Reject(r.Context(), run.ID, auth.UserIDFromContext(r.Context()))
	if err != nil {
		writeRunError(w, err)
		return
	}
	h.auditRun(r, "runbook.run.rejected", rejected, "", runConnectorIDs(steps))
	httputil.NoContent(w)
}

// ConfirmRunStep confirms a waiting manual step without elevation.
func (h *Handler) ConfirmRunStep(w http.ResponseWriter, r *http.Request) {
	run, steps, ok := h.authorizedRun(w, r)
	if !ok {
		return
	}
	stepID := r.PathValue("stepId")
	if err := h.Executor.Confirm(r.Context(), run.ID, stepID, auth.UserIDFromContext(r.Context())); err != nil {
		h.auditShutdownTransition(r, "runbook.run.confirm", err, stepID)
		writeRunError(w, err)
		return
	}
	h.auditRun(r, "runbook.run.confirm", run, stepID, runConnectorIDs(steps))
	httputil.NoContent(w)
}

// resumeRequest is the optional body of ResumeRun. Decision answers the
// unknown connector_action or config_push step a resume would start with:
// "resend" retries it, "mark_done" marks it succeeded without sending. Empty
// means no decision. StepID is the step the decision is for and UpdatedAt the
// run's updatedAt exactly as the client received it; both are required with a
// decision on such a step and checked whenever present.
type resumeRequest struct {
	Decision  string `json:"decision"`
	StepID    string `json:"stepId"`
	UpdatedAt string `json:"updatedAt"`
}

// decodeResumeBody reads the optional resume body. An empty body is the zero
// request.
func decodeResumeBody(w http.ResponseWriter, r *http.Request) (resumeRequest, bool) {
	var body resumeRequest
	if r.Body != nil {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, httputil.MaxJSONBodyBytes)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
			return body, false
		}
	}
	return body, true
}

// resumeDecision validates the decision of body: a value that is not
// resend or mark_done is a 400 with a field error on decision.
func resumeDecision(w http.ResponseWriter, body resumeRequest) (runbookrun.ResumeDecision, bool) {
	decision := runbookrun.ResumeDecision(body.Decision)
	switch decision {
	case runbookrun.ResumeNone, runbookrun.ResumeResend, runbookrun.ResumeMarkDone:
		return decision, true
	default:
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "decision must be resend or mark_done", []httputil.FieldError{{Field: "decision", Msg: "must be resend or mark_done"}})
		return "", false
	}
}

// writeRunChanged answers 409 run_changed with the run's current state and
// updatedAt so the client can show them and decide again.
func (h *Handler) writeRunChanged(w http.ResponseWriter, r *http.Request, run *store.RunbookRunRecord) {
	if current, _, err := h.Store.GetRunbookRun(r.Context(), run.ID); err == nil {
		run = current
	}
	httputil.JSON(w, http.StatusConflict, map[string]string{
		"code":      "run_changed",
		"message":   "The run changed since the decision was made; review its current state and decide again",
		"state":     run.State,
		"updatedAt": run.UpdatedAt,
	})
}

// resumeAudit builds the audit row of a resume decision, written in the resume
// transaction. It is nil without a decision. The executor writes it only when
// the decision is applied to an unknown connector_action or config_push step.
func resumeAudit(r *http.Request, run *store.RunbookRunRecord, body resumeRequest, decision runbookrun.ResumeDecision, connectorIDs []string) (*store.AuditRecord, error) {
	var action string
	switch decision {
	case runbookrun.ResumeResend:
		action = "runbook.run.step_resent"
	case runbookrun.ResumeMarkDone:
		action = "runbook.run.step_marked_done"
	default:
		return nil, nil
	}
	detail := map[string]any{"runId": run.ID, "runbookId": run.RunbookID, "stepId": body.StepID, "decision": string(decision)}
	record, err := store.NewAuditRecord(auth.UserIDFromContext(r.Context()), auth.InstanceAdminFromContext(r.Context()), action, "runbook_run", run.ID, detail)
	if err != nil {
		return nil, err
	}
	record.ConnectorIDs = connectorIDs
	return record, nil
}

// ResumeRun delegates a failed run continuation after fresh targeted elevation.
// The optional body carries the decision for an unknown connector_action step
// and the step and run revision the operator saw it on.
func (h *Handler) ResumeRun(w http.ResponseWriter, r *http.Request) {
	run, steps, ok := h.authorizedRun(w, r)
	if !ok {
		return
	}
	body, ok := decodeResumeBody(w, r)
	if !ok {
		return
	}
	if run.State != runbookrun.RunFailed {
		if body.UpdatedAt != "" && body.UpdatedAt != run.UpdatedAt {
			h.writeRunChanged(w, r, run)
			return
		}
		writeRunError(w, store.ErrConflict)
		return
	}
	if run.RunbookID == nil {
		httputil.Error(w, http.StatusConflict, "runbook_deleted", "The runbook no longer exists; this run cannot be resumed")
		return
	}
	if err := auth.ValidateElevationHeaderFor(h.ConnH.JWT, h.Store, "runbook.run", *run.RunbookID, r); err != nil {
		auth.WriteElevationError(w, err)
		return
	}
	decision, ok := resumeDecision(w, body)
	if !ok {
		return
	}
	audit, err := resumeAudit(r, run, body, decision, runConnectorIDs(steps))
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	expect := runbookrun.ResumeExpectation{StepID: body.StepID, UpdatedAt: body.UpdatedAt, Audit: audit}
	resumed, _, err := h.Executor.ResumeExpecting(r.Context(), run.ID, auth.UserIDFromContext(r.Context()), decision, expect)
	if err != nil {
		h.auditShutdownTransition(r, "runbook.run.resume", err, "")
		switch {
		case errors.Is(err, runbookrun.ErrRunChanged):
			h.writeRunChanged(w, r, run)
		case errors.Is(err, runbookrun.ErrDecisionFieldsRequired):
			var fields []httputil.FieldError
			if body.StepID == "" {
				fields = append(fields, httputil.FieldError{Field: "stepId", Msg: "is required with a decision"})
			}
			if body.UpdatedAt == "" {
				fields = append(fields, httputil.FieldError{Field: "updatedAt", Msg: "is required with a decision"})
			}
			httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "stepId and updatedAt are required with a decision on an unknown connector action or config push step", fields)
		default:
			writeRunError(w, err)
		}
		return
	}
	h.auditRun(r, "runbook.run.resume", resumed, "", runConnectorIDs(steps))
	httputil.JSON(w, http.StatusAccepted, resumed)
}

// existingConnectorSteps drops steps whose connector was deleted, since the
// grants cascaded away with it and nobody could ever pass the check again.
func (h *Handler) existingConnectorSteps(ctx context.Context, steps []*store.RunbookRunStepRecord) ([]*store.RunbookRunStepRecord, error) {
	exists := map[string]bool{}
	kept := make([]*store.RunbookRunStepRecord, 0, len(steps))
	for _, step := range steps {
		id := step.ConnectorID
		if id != "" {
			if _, seen := exists[id]; !seen {
				_, err := h.Store.GetConnector(ctx, id)
				if err != nil && !errors.Is(err, store.ErrNotFound) {
					return nil, err
				}
				exists[id] = err == nil
			}
			if !exists[id] {
				continue
			}
		}
		kept = append(kept, step)
	}
	return kept, nil
}

// CancelRun cancels an open run after checking the grant on every frozen
// connector that still exists; deleted connectors do not block cancellation.
func (h *Handler) CancelRun(w http.ResponseWriter, r *http.Request) {
	run, steps, err := h.Store.GetRunbookRun(r.Context(), r.PathValue("runId"))
	if err != nil {
		writeRunError(w, err)
		return
	}
	connectorIDs := runConnectorIDs(steps)
	steps, err = h.existingConnectorSteps(r.Context(), steps)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if !h.runAuthorized(w, r, steps) {
		return
	}
	if err := h.Executor.Cancel(r.Context(), run.ID, auth.UserIDFromContext(r.Context())); err != nil {
		writeRunError(w, err)
		return
	}
	h.auditRun(r, "runbook.run.cancel", run, "", connectorIDs)
	httputil.NoContent(w)
}

func runConnectorIDs(steps []*store.RunbookRunStepRecord) []string {
	ids := make([]string, 0, len(steps))
	for _, step := range steps {
		ids = append(ids, step.ConnectorID)
	}
	return ids
}

// auditRun snapshots connectorIDs from every frozen step, including deleted
// connectors; pass the whole run scope rather than just the acted-on step.
func (h *Handler) auditRun(r *http.Request, action string, run *store.RunbookRunRecord, stepID string, connectorIDs []string) {
	detail := map[string]any{"runId": run.ID, "runbookId": run.RunbookID}
	if stepID != "" {
		detail["stepId"] = stepID
	}
	if err := h.Store.RecordAuditScopedFromContext(r.Context(), action, "runbook_run", run.ID, detail, connectorIDs); err != nil {
		slog.Error("failed to record audit", "action", action, "error", err)
	}
}

// The executor records start/resume/confirmation before discovering shutdown.
// Audit those durable actions even though their continuation was refused.
func (h *Handler) auditShutdownTransition(r *http.Request, action string, err error, stepID string) {
	var shutdown *runbookrun.ShutdownError
	if !errors.As(err, &shutdown) {
		return
	}
	run, steps, loadErr := h.Store.GetRunbookRun(r.Context(), shutdown.RunID)
	if loadErr != nil {
		slog.Error("failed to load shutdown run for audit", "error", loadErr)
		return
	}
	h.auditRun(r, action, run, stepID, runConnectorIDs(steps))
}
