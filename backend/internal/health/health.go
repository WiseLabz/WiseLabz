// Package health runs connector health checks, shared by the manual
// POST /connectors/{id}/health endpoint and the scheduled health job so both
// classify, persist status and record history identically.
package health

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Result is the outcome of one health check.
type Result struct {
	Status    string
	Message   string
	LatencyMs int64
}

// RunHealthCheck runs a cheap connectivity check (Validate only — no Fetch,
// no snapshot, no docs) for rec, persists the resulting status on the
// connector row, and records a health_checks history row. A failure to
// record the history row is logged, not returned: it is additive to the
// status update. An error is returned only when config parsing or the status
// update fails.
func RunHealthCheck(ctx context.Context, s *store.Store, rec *store.ConnectorRecord, encKey string) (Result, error) {
	cfg, err := store.ParseConnectorConfig(rec.Type, rec.ConfigData, encKey)
	if err != nil {
		return Result{}, err
	}
	cfg["url"] = rec.URL
	cfg["verify_tls"] = rec.VerifyTLS

	start := time.Now()
	c, connErr := connector.Get(rec.Type, cfg)
	validateErr := connErr
	if connErr == nil {
		validateErr = c.Validate(ctx, cfg)
	}
	latency := time.Since(start)

	threshold := connector.DegradedLatencyThreshold
	if schema, err := connector.GetTypeSchema(rec.Type); err == nil {
		threshold = schema.DegradedLatencyThreshold()
	}
	status, message := connector.ClassifyHealth(validateErr, latency, threshold)

	// Persist even if the check itself timed out or the caller went away, so a
	// hung connector is recorded as offline rather than silently skipped.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := s.UpdateConnector(ctx, rec.ID, map[string]any{
		"status":         status,
		"status_message": message,
	}); err != nil {
		return Result{}, err
	}

	latencyMs := latency.Milliseconds()
	if err := s.RecordHealthCheck(ctx, &store.HealthCheckRecord{
		ConnectorID: rec.ID,
		Status:      status,
		Message:     message,
		LatencyMs:   &latencyMs,
	}); err != nil {
		slog.Error("record health check", "connectorId", rec.ID, "error", err)
	}
	return Result{Status: status, Message: message, LatencyMs: latencyMs}, nil
}

// Runner runs scheduled health checks over all enabled, non-maintenance
// connectors with bounded concurrency and a per-check timeout.
type Runner struct {
	Store          *store.Store
	EncKey         string
	MaxConcurrency int
	CheckTimeout   time.Duration
}

// DefaultCheckTimeout bounds a single scheduled check.
const DefaultCheckTimeout = 30 * time.Second

// RunDueChecks health-checks every enabled connector not in an active
// maintenance window. Only the sweep-level error (listing connectors) is
// returned for job health purposes; a single connector's failure is logged.
func (r *Runner) RunDueChecks(ctx context.Context, logger *slog.Logger) error {
	conns, err := r.Store.ListHealthCheckConnectors(ctx, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("list connectors for health check: %w", err)
	}
	maxConc := r.MaxConcurrency
	if maxConc <= 0 {
		maxConc = 8
	}
	timeout := r.CheckTimeout
	if timeout <= 0 {
		timeout = DefaultCheckTimeout
	}
	sem := make(chan struct{}, maxConc)
	var wg sync.WaitGroup
loop:
	for i := range conns {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break loop
		}
		wg.Add(1)
		go func(rec *store.ConnectorRecord) {
			defer wg.Done()
			defer func() { <-sem }()
			checkCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			if _, err := RunHealthCheck(checkCtx, r.Store, rec, r.EncKey); err != nil {
				logger.Error("scheduled health check failed", "connector", rec.ID, "error", err)
			}
		}(&conns[i])
	}
	wg.Wait()
	return nil
}
