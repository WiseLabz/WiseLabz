// Package search exposes grouped lab-wide content and entity search.
package search

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Handler serves grouped content and entity search.
type Handler struct{ Store *store.Store }

// List handles GET /api/search. Limits apply independently to each group.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := strings.TrimSpace(q.Get("q"))
	filters := store.SearchFilter{Type: q.Get("type"), ConnectorID: q.Get("connector"), Kind: q.Get("kind")}
	switch filters.Type {
	case "", "doc", "runbook", "entity":
	default:
		httputil.Error(w, 400, "invalid_request", "Invalid search type")
		return
	}
	if len(query) > 500 || len(filters.ConnectorID) > 128 || len(filters.Kind) > 100 {
		httputil.Error(w, 400, "invalid_request", "Search filters are too long")
		return
	}
	limit := 20
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			httputil.Error(w, 400, "invalid_request", "Search limit must be between 1 and 100")
			return
		}
		limit = n
	}
	result, err := h.Store.SearchLab(r.Context(), auth.UserIDFromContext(r.Context()), query, filters, limit)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, result)
}
