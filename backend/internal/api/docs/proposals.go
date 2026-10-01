package docs

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// canReviewProposal reports whether the caller may approve/reject a proposal
// on a doc scoped to connectorID: operator on that connector, or instance
// admin for lab-wide docs - the same rule as saving the doc directly.
func (h *Handler) canReviewProposal(r *http.Request, connectorID string) (bool, error) {
	if connectorID == "" {
		return auth.InstanceAdminFromContext(r.Context()), nil
	}
	return h.Store.UserHasConnectorRole(r.Context(), auth.UserIDFromContext(r.Context()), connectorID, "operator")
}

// reviewableScope is the SQL-side equivalent of canReviewProposal: the set of
// connectors the caller holds operator on (API-key restrictions applied),
// plus lab-wide docs for instance admins.
func (h *Handler) reviewableScope(r *http.Request) (store.ProposalScope, error) {
	all, err := h.Store.ListConnectorIDs(r.Context())
	if err != nil {
		return store.ProposalScope{}, err
	}
	ids, err := h.Store.FilterConnectorIDsByGrant(r.Context(), auth.UserIDFromContext(r.Context()), all, "operator")
	if err != nil {
		return store.ProposalScope{}, err
	}
	return store.ProposalScope{LabWide: auth.InstanceAdminFromContext(r.Context()), ConnectorIDs: ids}, nil
}

// ListProposals handles GET /api/docs/edit-proposals: proposals (default
// status pending) on docs the caller may review. The list omits proposal
// content; fetch it with GetProposal.
func (h *Handler) ListProposals(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = store.ProposalPending
	}
	if status != "all" && status != store.ProposalPending && status != store.ProposalApproved && status != store.ProposalRejected {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "status must be pending, approved, rejected or all")
		return
	}
	if status == "all" {
		status = ""
	}
	page, pageSize, offset := httputil.Paginate(r)

	scope, err := h.reviewableScope(r)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	items, total, err := h.Store.ListDocEditProposals(r.Context(), status, scope, pageSize, offset)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.WritePaginated(w, items, page, pageSize, total)
}

// loadReviewableProposal loads the proposal and enforces review access,
// writing the error response and returning nil on failure. Callers without
// access get a 404 so proposals on hidden docs aren't confirmed to exist.
func (h *Handler) loadReviewableProposal(w http.ResponseWriter, r *http.Request) *store.DocEditProposal {
	p, err := h.Store.GetDocEditProposal(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Proposal not found")
		return nil
	}
	if err != nil {
		httputil.Errorf(w, err)
		return nil
	}
	ok, err := h.canReviewProposal(r, p.ServiceID)
	if err != nil {
		httputil.Errorf(w, err)
		return nil
	}
	if !ok {
		httputil.Error(w, http.StatusNotFound, "not_found", "Proposal not found")
		return nil
	}
	return p
}

// GetProposal handles GET /api/docs/edit-proposals/{id}: one proposal
// including its proposed content (the list endpoint omits it).
func (h *Handler) GetProposal(w http.ResponseWriter, r *http.Request) {
	p := h.loadReviewableProposal(w, r)
	if p == nil {
		return
	}
	httputil.JSON(w, http.StatusOK, p)
}

// ApproveProposal handles POST /api/docs/edit-proposals/{id}/approve: applies
// the proposal as a new doc version. 409 with the current doc when the doc
// changed since the proposal's base version (the proposal stays pending), or
// a plain 409 when it was already reviewed.
func (h *Handler) ApproveProposal(w http.ResponseWriter, r *http.Request) {
	p := h.loadReviewableProposal(w, r)
	if p == nil {
		return
	}
	reviewer := auth.UserIDFromContext(r.Context())
	if _, err := h.Store.ApproveDocEditProposal(r.Context(), p.ID, reviewer); err != nil {
		switch {
		case errors.Is(err, store.ErrVersionConflict):
			current, getErr := h.Store.GetDoc(r.Context(), p.DocID)
			if getErr != nil {
				httputil.Errorf(w, getErr)
				return
			}
			httputil.JSON(w, http.StatusConflict, current)
		case errors.Is(err, store.ErrConflict):
			httputil.Error(w, http.StatusConflict, "already_reviewed", "Proposal has already been reviewed")
		case errors.Is(err, store.ErrNotFound):
			httputil.Error(w, http.StatusNotFound, "not_found", "Proposal not found")
		default:
			httputil.Errorf(w, err)
		}
		return
	}

	if err := h.Store.RecordAuditFromContext(r.Context(), "doc.edit_approved", "doc", p.DocID, map[string]any{"proposalId": p.ID, "baseVersion": p.BaseVersion}); err != nil {
		slog.Error("failed to record audit", "action", "doc.edit_approved", "error", err)
	}
	h.syncDocEmbeddings(r.Context(), p.DocID, p.Content)

	updated, err := h.Store.GetDocEditProposal(r.Context(), p.ID)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, updated)
}

// RejectProposal handles POST /api/docs/edit-proposals/{id}/reject.
func (h *Handler) RejectProposal(w http.ResponseWriter, r *http.Request) {
	p := h.loadReviewableProposal(w, r)
	if p == nil {
		return
	}
	if err := h.Store.RejectDocEditProposal(r.Context(), p.ID, auth.UserIDFromContext(r.Context())); err != nil {
		switch {
		case errors.Is(err, store.ErrConflict):
			httputil.Error(w, http.StatusConflict, "already_reviewed", "Proposal has already been reviewed")
		case errors.Is(err, store.ErrNotFound):
			httputil.Error(w, http.StatusNotFound, "not_found", "Proposal not found")
		default:
			httputil.Errorf(w, err)
		}
		return
	}
	if err := h.Store.RecordAuditFromContext(r.Context(), "doc.edit_rejected", "doc", p.DocID, map[string]any{"proposalId": p.ID}); err != nil {
		slog.Error("failed to record audit", "action", "doc.edit_rejected", "error", err)
	}
	updated, err := h.Store.GetDocEditProposal(r.Context(), p.ID)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, updated)
}
