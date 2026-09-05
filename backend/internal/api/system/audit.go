package system

import (
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/httputil"
)

// ListAudit handles GET /api/system/audit. Operator-only.
// Returns the audit trail (docs/AUDIT.md), newest first, optionally
// filtered by action and/or targetType.
func (h *Handler) ListAudit(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := httputil.Paginate(r)
	action := r.URL.Query().Get("action")
	targetType := r.URL.Query().Get("targetType")

	records, total, err := h.Store.ListAuditRecords(r.Context(), action, targetType, offset, pageSize)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.WritePaginated(w, records, page, pageSize, total)
}
