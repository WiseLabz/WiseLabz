// Package retention runs the background job that bounds the growth of
// historical WiseLabz data (service snapshots, doc revisions, alerts, and
// sync run history) by deleting rows past a configurable per-category age.
package retention

import (
	"context"
	"log/slog"
	"time"

	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/store"
)

const defaultInterval = 24 * time.Hour

// RunScheduler periodically purges historical data past each category's
// retention window, modeled on sync.RunScheduler.
func RunScheduler(ctx context.Context, s *store.Store, cfg config.RetentionSettings, logger *slog.Logger) {
	logger.Info("Retention scheduler started")
	interval := defaultInterval
	if cfg.IntervalHours > 0 {
		interval = time.Duration(cfg.IntervalHours) * time.Hour
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			runCleanup(ctx, s, cfg, logger)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

// runCleanup performs one cleanup pass: for every category whose *Days
// config value is > 0, deletes rows older than the cutoff. A category with
// Days <= 0 is skipped (retention disabled). Errors in one category are
// logged and do not stop the others from running.
func runCleanup(ctx context.Context, s *store.Store, cfg config.RetentionSettings, logger *slog.Logger) {
	cutoff := func(days int) string {
		return time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
	}

	if cfg.SnapshotDays > 0 {
		n, err := s.DeleteOldSnapshots(ctx, cutoff(cfg.SnapshotDays))
		if err != nil {
			logger.Error("delete old snapshots", "error", err)
		} else if n > 0 {
			logger.Info("Purged old snapshots", "count", n)
		}
	}

	if cfg.DocVersionDays > 0 {
		n, err := s.DeleteOldDocVersions(ctx, cutoff(cfg.DocVersionDays))
		if err != nil {
			logger.Error("delete old doc versions", "error", err)
		} else if n > 0 {
			logger.Info("Purged old doc versions", "count", n)
		}
	}

	if cfg.AlertDays > 0 {
		n, err := s.DeleteOldAlerts(ctx, cutoff(cfg.AlertDays))
		if err != nil {
			logger.Error("delete old alerts", "error", err)
		} else if n > 0 {
			logger.Info("Purged old alerts", "count", n)
		}
	}

	if cfg.SyncRunDays > 0 {
		n, err := s.DeleteOldSyncRuns(ctx, cutoff(cfg.SyncRunDays))
		if err != nil {
			logger.Error("delete old sync runs", "error", err)
		} else if n > 0 {
			logger.Info("Purged old sync runs", "count", n)
		}
	}
}
