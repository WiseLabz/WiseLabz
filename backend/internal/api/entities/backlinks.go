package entities

import (
	"errors"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Backlinks uses the same non-disclosing target checks as the detail endpoint.
func (h *Handler) Backlinks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !validID(r.PathValue("id")) {
		notFound(w)
		return
	}
	entity, err := h.Store.ResolveEntityIdentity(ctx, r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		httpError(w, err)
		return
	}
	connectors, err := h.Store.ListEntityMemberConnectorIDs(ctx, entity.ID)
	if err != nil {
		httpError(w, err)
		return
	}
	allowed, err := h.Store.FilterConnectorIDsByGrant(ctx, auth.UserIDFromContext(ctx), connectors, "viewer")
	if err != nil {
		httpError(w, err)
		return
	}
	if len(allowed) == 0 {
		notFound(w)
		return
	}
	links, err := h.Store.ListDocBacklinks(ctx, auth.UserIDFromContext(ctx), "entity", entity.ID)
	if err != nil {
		httpError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, links)
}
