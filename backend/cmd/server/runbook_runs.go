package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/WiseLabz/wiselabz/internal/notifications"
	"github.com/WiseLabz/wiselabz/internal/runbookrun"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

// mustRecoverRunbookRuns runs recoverRunbookRuns and exits when it fails: a
// run still marked running with nothing executing it would block its runbook
// and mislead whoever reads it.
func mustRecoverRunbookRuns(ctx context.Context, s *store.Store, hub *ws.Hub, d *notifications.Dispatcher, logger *slog.Logger) {
	if err := recoverRunbookRuns(ctx, s, hub, d, logger); err != nil {
		logger.Error("Failed to recover runbook runs", "error", err)
		os.Exit(1)
	}
}

// recoverRunbookRuns marks the runbook runs a previous process left running
// as interrupted: their in-flight step has an unknown outcome. main calls it
// before the lifecycle manager starts the HTTP server, so no request can
// start, resume or confirm a run while a stale one still looks active. It
// never continues a run.
func recoverRunbookRuns(ctx context.Context, s *store.Store, hub *ws.Hub, d *notifications.Dispatcher, logger *slog.Logger) error {
	n, err := runbookrun.Recover(ctx, s, hub, d)
	if err != nil {
		return err
	}
	if n > 0 {
		logger.Warn("Runbook runs interrupted by the previous shutdown; resume them to continue", "runs", n)
	}
	return nil
}
