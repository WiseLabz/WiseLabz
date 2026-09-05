package sync

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

const schedulerTick = 30 * time.Second

// dueConnectorsLimit bounds how many due connectors are synced per tick.
// ponytail: fine for a self-hosted ops tool's connector count; paginate if that changes.
const dueConnectorsLimit = 50

// RunScheduler polls for connectors whose next_run_at is due and syncs them,
// modeled on notifications.RunDeliveryRetries.
func RunScheduler(ctx context.Context, e *Engine, logger *slog.Logger) {
	logger.Info("Connector sync scheduler started")
	for {
		select {
		case <-ctx.Done():
			return
		default:
			e.runDueSyncs(ctx, logger)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(schedulerTick):
		}
	}
}

// runDueSyncs syncs every connector whose next_run_at has passed.
func (e *Engine) runDueSyncs(ctx context.Context, logger *slog.Logger) {
	now := time.Now().UTC().Format(time.RFC3339)
	due, err := e.store.ListDueConnectors(ctx, now, dueConnectorsLimit)
	if err != nil {
		logger.Error("list due connectors", "error", err)
		return
	}
	for _, c := range due {
		if _, err := e.RunSync(ctx, c.ID, uuid.New().String()); err != nil {
			logger.Error("scheduled sync failed", "connector", c.ID, "error", err)
		}
	}
}
