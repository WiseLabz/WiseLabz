// Package notifications provides API handlers for the in-app notification center.
package notifications

import (
	"errors"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Handler holds dependencies for notification endpoints.
type Handler struct {
	Store *store.Store
}

// NewHandler creates a new notification handler.
func NewHandler(s *store.Store) *Handler {
	return &Handler{Store: s}
}

// List handles GET /api/notifications. Scoped to the calling user.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	page, pageSize, offset := httputil.Paginate(r)
	unreadOnly := r.URL.Query().Get("unread") == "true"

	notifications, total, err := h.Store.ListNotifications(r.Context(), userID, unreadOnly, offset, pageSize)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.WritePaginated(w, notifications, page, pageSize, total)
}

// MarkRead handles POST /api/notifications/{id}/read.
func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	id := r.PathValue("id")

	n, err := h.Store.MarkNotificationRead(r.Context(), userID, id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Notification not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, n)
}

// ReadAll handles POST /api/notifications/read-all.
func (h *Handler) ReadAll(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if err := h.Store.MarkAllNotificationsRead(r.Context(), userID); err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.NoContent(w)
}
