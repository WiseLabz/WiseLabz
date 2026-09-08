package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/WiseLabz/wiselabz/internal/backup"
	"github.com/WiseLabz/wiselabz/internal/httputil"
)

// MaxImportBytes bounds a backup upload before decoding to prevent a single
// request from consuming unbounded server memory.
const MaxImportBytes = 10 << 20

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
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxImportBytes))
	if err := decoder.Decode(&b); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			httputil.Error(w, http.StatusRequestEntityTooLarge, "request_too_large", "Backup upload exceeds the 10 MiB limit")
			return
		}
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
