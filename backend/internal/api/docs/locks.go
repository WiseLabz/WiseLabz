package docs

import (
	"context"
	"errors"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

// GetLock handles GET /api/docs/{id}/lock. Returns the current lock, or an
// empty object if the doc is unlocked/expired. Requires viewer access on the doc's connector.
func (h *Handler) GetLock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.loadDocForViewer(w, r, id) {
		return
	}
	lock, err := h.Store.GetDocLock(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.JSON(w, http.StatusOK, map[string]any{})
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	response, err := h.lockWithName(r.Context(), lock)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

// AcquireLock handles POST /api/docs/{id}/lock. Acquires or renews the
// advisory editing lock; 409 with the current holder if held by someone
// else. Operator-only, same gate as Save.
func (h *Handler) AcquireLock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userID := auth.UserIDFromContext(r.Context())

	existing, err := h.Store.GetDoc(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Doc not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if !h.requireDocOperator(w, r, existing.ServiceID) {
		return
	}

	lock, err := h.Store.AcquireDocLock(r.Context(), id, userID)
	conflict := errors.Is(err, store.ErrLockHeldByOther)
	if err != nil && !conflict {
		httputil.Errorf(w, err)
		return
	}
	response, err := h.lockWithName(r.Context(), lock)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if conflict {
		httputil.JSON(w, http.StatusConflict, response)
		return
	}
	h.broadcastDocEvent(existing.ServiceID, ws.EventDocLockAcquired, response)
	httputil.JSON(w, http.StatusOK, response)
}

// ReleaseLock handles POST /api/docs/{id}/lock/release. Operator-only.
func (h *Handler) ReleaseLock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userID := auth.UserIDFromContext(r.Context())

	existing, err := h.Store.GetDoc(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Doc not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if !h.requireDocOperator(w, r, existing.ServiceID) {
		return
	}

	if err := h.Store.ReleaseDocLock(r.Context(), id, userID); err != nil {
		httputil.Errorf(w, err)
		return
	}
	h.broadcastDocEvent(existing.ServiceID, ws.EventDocLockReleased, map[string]any{"docId": id, "userId": userID})
	httputil.JSON(w, http.StatusOK, map[string]any{})
}

// broadcastDocEvent emits a doc-lock event scoped to the doc's connector; a
// lab-wide doc (no connector) is global.
func (h *Handler) broadcastDocEvent(connectorID, eventType string, payload any) {
	if h.WSHub == nil {
		return
	}
	if connectorID == "" {
		h.WSHub.Broadcast(eventType, payload)
		return
	}
	h.WSHub.BroadcastConnector(connectorID, eventType, payload)
}

// Include only the holder's public name, without exposing the user directory.
type docLockResponse struct {
	*store.DocLockRecord
	UserName string `json:"userName,omitempty"`
}

func (h *Handler) lockWithName(ctx context.Context, lock *store.DocLockRecord) (*docLockResponse, error) {
	user, err := h.Store.GetUserByID(ctx, lock.UserID)
	if errors.Is(err, store.ErrNotFound) {
		return &docLockResponse{DocLockRecord: lock}, nil
	}
	if err != nil {
		return nil, err
	}
	name := user.DisplayName
	if name == "" {
		name = user.Username
	}
	return &docLockResponse{DocLockRecord: lock, UserName: name}, nil
}
