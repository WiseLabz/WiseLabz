package runbooks

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/api/connectors"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
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
}

// RunResponse is the history projection shared by HTTP and read-only MCP tools.
type RunResponse struct {
	store.RunbookRunRecord
	Steps []RunStepResponse `json:"steps"`
}

type runPreviewResponse struct {
	ID       string            `json:"id"`
	CanStart bool              `json:"canStart"`
	Steps    []RunStepResponse `json:"steps"`
}

// runAuthorized checks all distinct frozen connectors before inspecting state
// or elevation. Store role checks apply API-key connector restrictions too.
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
	// A manual-only run must not let a read-only key mutate history.
	if auth.APIKeyRestrictionFromContext(r.Context()).ReadOnly {
		httputil.Error(w, http.StatusForbidden, "forbidden", "insufficient permissions")
		return false
	}
	return true
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
	case errors.Is(err, store.ErrRunbookRunStepCount):
		httputil.Error(w, http.StatusBadRequest, "invalid_steps", "A runbook run needs between 1 and 20 steps")
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
	return RunResponse{RunbookRunRecord: *run, Steps: views}, nil
}

func (h *Handler) runStepViews(ctx context.Context, steps []*store.RunbookRunStepRecord) ([]RunStepResponse, error) {
	views := make([]RunStepResponse, 0, len(steps))
	roles := map[string]string{}
	names := map[string]string{}
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
		view.TimeoutSeconds = reportedTimeout(view.Kind, step.TimeoutSeconds)
		view.Error = step.Error
		view.CanExecute = role == "operator" && !auth.APIKeyRestrictionFromContext(ctx).ReadOnly
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
	if _, err := h.Store.GetRunbook(r.Context(), id); err != nil {
		writeRunError(w, err)
		return
	}
	if len(steps) == 0 {
		writeRunError(w, store.ErrRunbookRunStepCount)
		return
	}
	if err := auth.ValidateElevationHeaderFor(h.ConnH.JWT, h.Store, "runbook.run", id, r); err != nil {
		auth.WriteElevationError(w, err)
		return
	}
	run, saved, err := h.Executor.Start(r.Context(), id, auth.UserIDFromContext(r.Context()), steps)
	if err != nil {
		h.auditShutdownTransition(r, "runbook.run.start", err, "")
		writeRunError(w, err)
		return
	}
	h.auditRun(r, "runbook.run.start", run, "")
	resp, err := h.RunView(r.Context(), run, saved)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusAccepted, resp)
}

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
	canStart := len(views) > 0
	for i := range views {
		view := &views[i]
		if view.Kind == kindLifecycle && !view.Redacted {
			preview, err := h.ConnH.PreviewLifecycleOp(r.Context(), view.ConnectorID, view.Verb, view.EntityRef)
			if err != nil {
				view.CanExecute = false
				if view.ExecuteBlockedReason == "" {
					view.ExecuteBlockedReason = err.Error()
				}
			} else {
				view.Preview = preview
			}
		}
		if !view.CanExecute {
			canStart = false
		}
	}
	httputil.JSON(w, http.StatusOK, runPreviewResponse{ID: id, CanStart: canStart, Steps: views})
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

func (h *Handler) authorizedRun(w http.ResponseWriter, r *http.Request) (*store.RunbookRunRecord, bool) {
	run, steps, err := h.Store.GetRunbookRun(r.Context(), r.PathValue("runId"))
	if err != nil {
		writeRunError(w, err)
		return nil, false
	}
	return run, h.runAuthorized(w, r, steps)
}

// ConfirmRunStep confirms a waiting manual step without elevation.
func (h *Handler) ConfirmRunStep(w http.ResponseWriter, r *http.Request) {
	run, ok := h.authorizedRun(w, r)
	if !ok {
		return
	}
	stepID := r.PathValue("stepId")
	if err := h.Executor.Confirm(r.Context(), run.ID, stepID, auth.UserIDFromContext(r.Context())); err != nil {
		h.auditShutdownTransition(r, "runbook.run.confirm", err, stepID)
		writeRunError(w, err)
		return
	}
	h.auditRun(r, "runbook.run.confirm", run, stepID)
	httputil.NoContent(w)
}

// ResumeRun delegates a failed run continuation after fresh targeted elevation.
func (h *Handler) ResumeRun(w http.ResponseWriter, r *http.Request) {
	run, ok := h.authorizedRun(w, r)
	if !ok {
		return
	}
	if run.State != runbookrun.RunFailed {
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
	resumed, err := h.Executor.Resume(r.Context(), run.ID, auth.UserIDFromContext(r.Context()))
	if err != nil {
		h.auditShutdownTransition(r, "runbook.run.resume", err, "")
		writeRunError(w, err)
		return
	}
	h.auditRun(r, "runbook.run.resume", resumed, "")
	httputil.JSON(w, http.StatusAccepted, resumed)
}

// CancelRun cancels an open run after checking every frozen connector grant.
func (h *Handler) CancelRun(w http.ResponseWriter, r *http.Request) {
	run, ok := h.authorizedRun(w, r)
	if !ok {
		return
	}
	if err := h.Executor.Cancel(r.Context(), run.ID, auth.UserIDFromContext(r.Context())); err != nil {
		writeRunError(w, err)
		return
	}
	h.auditRun(r, "runbook.run.cancel", run, "")
	httputil.NoContent(w)
}

func (h *Handler) auditRun(r *http.Request, action string, run *store.RunbookRunRecord, stepID string) {
	detail := map[string]any{"runId": run.ID, "runbookId": run.RunbookID}
	if stepID != "" {
		detail["stepId"] = stepID
	}
	if err := h.Store.RecordAuditFromContext(r.Context(), action, "runbook_run", run.ID, detail); err != nil {
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
	run, _, loadErr := h.Store.GetRunbookRun(r.Context(), shutdown.RunID)
	if loadErr != nil {
		slog.Error("failed to load shutdown run for audit", "error", loadErr)
		return
	}
	h.auditRun(r, action, run, stepID)
}
