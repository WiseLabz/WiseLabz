package system

import (
	"fmt"
	"net/http"
	"time"

	"github.com/WiseLabz/wiselabz/internal/diagnostics"
	"github.com/WiseLabz/wiselabz/internal/httputil"
)

// Diagnostics handles GET /api/system/diagnostics. Operator-only. Returns a
// sanitized support bundle (health, versions, secret-free config, recent
// failures) as a downloadable JSON attachment — see docs/DIAGNOSTICS.md.
func (h *Handler) Diagnostics(w http.ResponseWriter, r *http.Request) {
	b, err := diagnostics.Collect(r.Context(), h.Store, h.Config)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	filename := fmt.Sprintf("wiselabz-diagnostics-%s.json", time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	httputil.JSON(w, http.StatusOK, b)
}
