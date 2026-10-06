package system

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/robfig/cron/v3"

	"github.com/WiseLabz/wiselabz/internal/blobstore"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/retention"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// GetRetentionSettings handles GET /api/system/settings/retention. Operator-only.
// Returns the current data-retention cleanup configuration.
func (h *Handler) GetRetentionSettings(w http.ResponseWriter, r *http.Request) {
	rs, err := h.Store.GetRetentionSettings(r.Context())
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			httputil.Errorf(w, err)
			return
		}
		// If settings don't exist (shouldn't happen after init), return a sensible default.
		// keep in sync with config.go retention defaults
		slog.Warn("retention settings not found, returning default", "error", err)
		rs = store.RetentionSettings{
			SnapshotDays:        90,
			DocVersionDays:      365,
			AlertDays:           180,
			SyncRunDays:         90,
			AuditDays:           180,
			HealthCheckDays:     90,
			ReportDays:          90,
			DeletedDocsDays:     30,
			CronExpr:            "0 0 * * *",
			RunbookOpenRunHours: store.DefaultRunbookOpenRunHours,
			RunbookRunDays:      store.DefaultRunbookRunDays,
		}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"snapshotDays":        rs.SnapshotDays,
		"docVersionDays":      rs.DocVersionDays,
		"alertDays":           rs.AlertDays,
		"syncRunDays":         rs.SyncRunDays,
		"auditDays":           rs.AuditDays,
		"healthCheckDays":     rs.HealthCheckDays,
		"reportDays":          rs.ReportDays,
		"deletedDocsDays":     rs.DeletedDocsDays,
		"runbookOpenRunHours": rs.RunbookOpenRunHours,
		"runbookRunDays":      rs.RunbookRunDays,
		"cronExpr":            rs.CronExpr,
		"updatedAt":           rs.UpdatedAt,
	})
}

// RetentionSettingsRequest is the request body for PUT /api/system/settings/retention.
type RetentionSettingsRequest struct {
	SnapshotDays        int    `json:"snapshotDays"`
	DocVersionDays      int    `json:"docVersionDays"`
	AlertDays           int    `json:"alertDays"`
	SyncRunDays         int    `json:"syncRunDays"`
	AuditDays           int    `json:"auditDays"`
	HealthCheckDays     int    `json:"healthCheckDays"`
	DeletedDocsDays     *int   `json:"deletedDocsDays"`
	RunbookOpenRunHours *int   `json:"runbookOpenRunHours"`
	RunbookRunDays      *int   `json:"runbookRunDays"`
	ReportDays          int    `json:"reportDays"`
	CronExpr            string `json:"cronExpr"`
}

// UpdateRetentionSettings handles PUT /api/system/settings/retention. Operator-only.
// Updates the retention settings and re-registers the cron job.
func (h *Handler) UpdateRetentionSettings(w http.ResponseWriter, r *http.Request) {
	req, ok := httputil.DecodeJSON[RetentionSettingsRequest](w, r)
	if !ok {
		return
	}

	if req.CronExpr == "" {
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_cron", "cronExpr must not be empty", []httputil.FieldError{{Field: "cronExpr", Msg: "must not be empty"}})
		return
	}
	parser5Field := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	parser6Field := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err5 := parser5Field.Parse(req.CronExpr)
	_, err6 := parser6Field.Parse(req.CronExpr)
	if err5 != nil && err6 != nil {
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_cron", "Invalid cron expression: must be valid 5-field or 6-field cron format", []httputil.FieldError{{Field: "cronExpr", Msg: "must be a valid 5-field or 6-field cron expression"}})
		return
	}

	// Omitting deletedDocsDays or the runbook run settings keeps the stored
	// values so older clients don't reset them.
	deletedDays := 30
	runbookOpenRunHours := store.DefaultRunbookOpenRunHours
	runbookRunDays := store.DefaultRunbookRunDays
	current, currentErr := h.Store.GetRetentionSettings(r.Context())
	if currentErr == nil {
		runbookOpenRunHours = current.RunbookOpenRunHours
		runbookRunDays = current.RunbookRunDays
	}
	if req.DeletedDocsDays != nil {
		deletedDays = *req.DeletedDocsDays
	} else if currentErr == nil {
		deletedDays = current.DeletedDocsDays
	}
	if req.RunbookOpenRunHours != nil {
		runbookOpenRunHours = *req.RunbookOpenRunHours
	}
	if req.RunbookRunDays != nil {
		runbookRunDays = *req.RunbookRunDays
	}

	if runbookOpenRunHours < 1 || runbookOpenRunHours > 8760 {
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_hours", "runbookOpenRunHours must be between 1 and 8760", []httputil.FieldError{{Field: "runbookOpenRunHours", Msg: "must be between 1 and 8760"}})
		return
	}

	var dayErrs []httputil.FieldError
	for _, f := range []struct {
		name string
		days int
	}{
		{"snapshotDays", req.SnapshotDays},
		{"docVersionDays", req.DocVersionDays},
		{"alertDays", req.AlertDays},
		{"syncRunDays", req.SyncRunDays},
		{"auditDays", req.AuditDays},
		{"healthCheckDays", req.HealthCheckDays},
		{"reportDays", req.ReportDays},
		{"deletedDocsDays", deletedDays},
	} {
		if f.days < 0 {
			dayErrs = append(dayErrs, httputil.FieldError{Field: f.name, Msg: "must be >= 0 (0 disables cleanup)"})
		}
	}
	if runbookRunDays < 0 || runbookRunDays > 3650 {
		dayErrs = append(dayErrs, httputil.FieldError{Field: "runbookRunDays", Msg: "must be between 0 and 3650 (0 disables cleanup)"})
	}
	if len(dayErrs) > 0 {
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_days", "retention day values must be >= 0 (0 disables cleanup)", dayErrs)
		return
	}

	newSettings := store.RetentionSettings{
		SnapshotDays:        req.SnapshotDays,
		DocVersionDays:      req.DocVersionDays,
		AlertDays:           req.AlertDays,
		SyncRunDays:         req.SyncRunDays,
		AuditDays:           req.AuditDays,
		HealthCheckDays:     req.HealthCheckDays,
		ReportDays:          req.ReportDays,
		DeletedDocsDays:     deletedDays,
		CronExpr:            req.CronExpr,
		RunbookOpenRunHours: runbookOpenRunHours,
		RunbookRunDays:      runbookRunDays,
	}
	if err := h.Store.UpsertRetentionSettings(r.Context(), newSettings); err != nil {
		httputil.Errorf(w, err)
		return
	}

	if err := h.Store.RecordAuditFromContext(r.Context(), "retention.settings.update", "retention_settings", "default", req); err != nil {
		slog.Error("audit retention settings update", "error", err)
	}

	h.reregisterRetentionJob(newSettings)

	httputil.JSON(w, http.StatusOK, map[string]any{
		"snapshotDays":        newSettings.SnapshotDays,
		"docVersionDays":      newSettings.DocVersionDays,
		"alertDays":           newSettings.AlertDays,
		"syncRunDays":         newSettings.SyncRunDays,
		"auditDays":           newSettings.AuditDays,
		"healthCheckDays":     newSettings.HealthCheckDays,
		"reportDays":          newSettings.ReportDays,
		"deletedDocsDays":     newSettings.DeletedDocsDays,
		"runbookOpenRunHours": newSettings.RunbookOpenRunHours,
		"runbookRunDays":      newSettings.RunbookRunDays,
		"cronExpr":            newSettings.CronExpr,
		"updatedAt":           newSettings.UpdatedAt,
	})
}

// InitRetentionJob registers the retention cron job at startup from the
// persisted settings, routing through the same RetentionJobID bookkeeping as
// reregisterRetentionJob uses. This must be the only place that ever calls
// h.Scheduler.AddJob("retention", ...) directly — see InitBackupJob for why.
// Seeds the settings from config defaults if no row exists yet (first boot).
func (h *Handler) InitRetentionJob(ctx context.Context) {
	rs, err := h.Store.GetRetentionSettings(ctx)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("failed to read retention settings", "error", err)
			return
		}
		slog.Info("initializing retention settings with defaults")
		rs = store.RetentionSettings{
			SnapshotDays:        h.Config.Retention.SnapshotDays,
			DocVersionDays:      h.Config.Retention.DocVersionDays,
			AlertDays:           h.Config.Retention.AlertDays,
			SyncRunDays:         h.Config.Retention.SyncRunDays,
			AuditDays:           h.Config.Retention.AuditDays,
			HealthCheckDays:     h.Config.Retention.HealthCheckDays,
			ReportDays:          h.Config.Retention.ReportDays,
			DeletedDocsDays:     h.Config.Retention.DeletedDocsDays,
			CronExpr:            h.Config.Retention.CronExpr,
			RunbookOpenRunHours: store.DefaultRunbookOpenRunHours,
			RunbookRunDays:      store.DefaultRunbookRunDays,
		}
		if err := h.Store.UpsertRetentionSettings(ctx, rs); err != nil {
			slog.Error("failed to initialize retention settings", "error", err)
			return
		}
	}
	h.reregisterRetentionJob(rs)
}

// reregisterRetentionJob removes the old job (if any) and registers a new one with the updated settings.
func (h *Handler) reregisterRetentionJob(rs store.RetentionSettings) {
	if h.Scheduler == nil {
		return
	}

	h.RetentionJobIDMu.Lock()
	defer h.RetentionJobIDMu.Unlock()

	if h.RetentionJobID != 0 {
		h.Scheduler.RemoveJob(h.RetentionJobID)
	}

	id, err := h.Scheduler.AddJob("retention", rs.CronExpr, func(jobCtx context.Context) error {
		blobstore.PublicationMu.Lock()
		defer blobstore.PublicationMu.Unlock()
		purgeErr := retention.RunCleanupOnce(jobCtx, h.Store, rs, slog.Default())
		blobs := blobstore.New(h.Config.Attachments.Dir, h.Config.Attachments.MaxBytes)
		gcErr := blobs.Sweep(func(hash string) (bool, error) { return h.Store.BlobReferenced(jobCtx, hash) })
		return errors.Join(purgeErr, gcErr)
	})
	if err != nil {
		// err wraps rs.CronExpr (user-controlled via PUT /settings/retention);
		// strip line breaks before logging so a crafted cron string can't
		// forge additional log entries (CWE-117).
		slog.Error("register retention job", "error", stripLogControlChars(err.Error()))
		h.RetentionJobID = 0
	} else {
		h.RetentionJobID = id
	}
}
