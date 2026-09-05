package system

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/WiseLabz/wiselabz/internal/backup"
	"github.com/WiseLabz/wiselabz/internal/httputil"
)

// ExportBackup handles GET /api/system/backup/export. Operator-only.
func (h *Handler) ExportBackup(w http.ResponseWriter, r *http.Request) {
	b, err := backup.Export(r.Context(), h.Store)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	filename := fmt.Sprintf("wiselabz-backup-%s.json", time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	httputil.JSON(w, http.StatusOK, b)
}

// ImportBackup handles POST /api/system/backup/import. Operator-only.
func (h *Handler) ImportBackup(w http.ResponseWriter, r *http.Request) {
	var b backup.Bundle
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	result, err := backup.Import(r.Context(), h.Store, &b)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_backup", err.Error())
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}
