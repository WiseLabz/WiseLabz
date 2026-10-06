package entities

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Reconciler rebuilds persisted entity identities from the latest snapshots.
// *doc.Engine implements it.
type Reconciler interface {
	BackfillEntityIdentities(ctx context.Context) (int, error)
}

const maxOverrideNoteRunes = 1000

type overrideMember struct {
	ConnectorID   string `json:"connectorId"`
	ConnectorName string `json:"connectorName"`
	Kind          string `json:"kind"`
	Ref           string `json:"ref"`
	Name          string `json:"name"`
	// EntityID is the identity the member belongs to now, empty when the member
	// has no stored membership.
	EntityID string `json:"entityId"`
}

type overrideResponse struct {
	ID        string           `json:"id"`
	Action    string           `json:"action"`
	Note      string           `json:"note"`
	CreatedBy string           `json:"createdBy"`
	CreatedAt string           `json:"createdAt"`
	State     string           `json:"state"`
	Members   []overrideMember `json:"members"`
}

type createOverrideRequest struct {
	Action           string `json:"action"`
	ConnectorID      string `json:"connectorId"`
	Kind             string `json:"kind"`
	Ref              string `json:"ref"`
	OtherConnectorID string `json:"otherConnectorId"`
	OtherKind        string `json:"otherKind"`
	OtherRef         string `json:"otherRef"`
	Note             string `json:"note"`
}

func toOverrideResponse(o store.EntityIdentityOverride, state string, members []store.EntityOverrideMember) overrideResponse {
	out := overrideResponse{ID: o.ID, Action: o.Action, Note: o.Note, CreatedBy: o.CreatedBy, CreatedAt: o.CreatedAt, State: state, Members: make([]overrideMember, 0, len(members))}
	for _, m := range members {
		out.Members = append(out.Members, overrideMember{ConnectorID: m.ConnectorID, ConnectorName: m.ConnectorName, Kind: m.Kind, Ref: m.Ref, Name: m.Name, EntityID: m.EntityID})
	}
	return out
}

func stateOf(members []store.EntityOverrideMember) string {
	for _, m := range members {
		if m.EntityID == "" || m.Gone {
			return store.EntityOverrideDormant
		}
	}
	return store.EntityOverrideActive
}

// ListOverrides handles GET /api/entity-overrides. It returns every override
// unfiltered, so the route must sit behind auth.RequireInstanceAdmin.
func (h *Handler) ListOverrides(w http.ResponseWriter, r *http.Request) {
	views, err := h.Store.ListEntityIdentityOverrides(r.Context())
	if err != nil {
		httpError(w, err)
		return
	}
	out := make([]overrideResponse, 0, len(views))
	for _, v := range views {
		out = append(out, toOverrideResponse(v.EntityIdentityOverride, v.State, v.Members))
	}
	httputil.JSON(w, http.StatusOK, out)
}

// CreateOverride handles POST /api/entity-overrides: it stores the override,
// reconciles identities synchronously and returns the override with each
// member's resulting identity ID. Route behind auth.RequireInstanceAdmin.
func (h *Handler) CreateOverride(w http.ResponseWriter, r *http.Request) {
	req, ok := httputil.DecodeJSON[createOverrideRequest](w, r)
	if !ok {
		return
	}
	if utf8.RuneCountInString(req.Note) > maxOverrideNoteRunes {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Note is too long")
		return
	}
	o := store.EntityIdentityOverride{
		Action: req.Action, ConnectorID: req.ConnectorID, Kind: req.Kind, Ref: req.Ref,
		OtherConnectorID: req.OtherConnectorID, OtherKind: req.OtherKind, OtherRef: req.OtherRef,
		Note: req.Note, CreatedBy: auth.UserIDFromContext(r.Context()),
	}
	if err := h.Store.CreateEntityIdentityOverride(r.Context(), &o); err != nil {
		if errors.Is(err, store.ErrInvalidEntityOverride) {
			httputil.Error(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		httputil.HandleStoreError(w, err)
		return
	}
	h.audit(r, "entity.override.create", o)
	members, err := h.reconcileMembers(r.Context(), o)
	if err != nil {
		httpError(w, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, toOverrideResponse(o, stateOf(members), members))
}

// DeleteOverride handles DELETE /api/entity-overrides/{id}: it removes the
// override, reconciles identities synchronously and returns the removed
// override with each member's resulting identity ID. Route behind
// auth.RequireInstanceAdmin.
func (h *Handler) DeleteOverride(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	view, err := h.Store.GetEntityIdentityOverride(r.Context(), id)
	if err != nil {
		httputil.HandleStoreError(w, err)
		return
	}
	if err := h.Store.DeleteEntityIdentityOverride(r.Context(), id); err != nil {
		httputil.HandleStoreError(w, err)
		return
	}
	h.audit(r, "entity.override.delete", view.EntityIdentityOverride)
	members, err := h.reconcileMembers(r.Context(), view.EntityIdentityOverride)
	if err != nil {
		httpError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, toOverrideResponse(view.EntityIdentityOverride, stateOf(members), members))
}

// reconcileMembers rebuilds identities, then reads where each of the
// override's members lives now. A detach gives the old ID to whichever cluster
// wins the tie-break, so the IDs are always read back, never assumed.
func (h *Handler) reconcileMembers(ctx context.Context, o store.EntityIdentityOverride) ([]store.EntityOverrideMember, error) {
	if _, err := h.Reconciler.BackfillEntityIdentities(ctx); err != nil {
		return nil, err
	}
	return h.Store.EntityOverrideMembers(ctx, o)
}

func (h *Handler) audit(r *http.Request, action string, o store.EntityIdentityOverride) {
	members := []map[string]string{{"connectorId": o.ConnectorID, "kind": o.Kind, "ref": o.Ref}}
	if o.Action == store.EntityOverrideMerge {
		members = append(members, map[string]string{"connectorId": o.OtherConnectorID, "kind": o.OtherKind, "ref": o.OtherRef})
	}
	if err := h.Store.RecordAuditFromContext(r.Context(), action, "entity_override", o.ID, map[string]any{
		"action": o.Action, "note": o.Note, "members": members,
	}); err != nil {
		slog.Error("failed to record audit", "action", action, "error", err)
	}
}
