// Package sync provides the sync engine for fetching connector data,
// diffing snapshots, and creating changes/alerts.
package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

// retrySchedule is the backoff delay before each retry attempt (index 0 =
// delay before the 2nd consecutive attempt, etc), mirroring
// notifications.retrySchedule.
// ponytail: fixed schedule, not exponential-from-config; add jitter/config if a real deployment needs it.
var retrySchedule = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, 30 * time.Minute, time.Hour}

// computeNextRun decides a connector's next scheduled run time and updated
// retry count after one sync attempt. Unlike notification delivery retries
// (which give up after a fixed number of attempts), a scheduled connector
// must keep trying forever: once retryCount exceeds the backoff schedule, it
// keeps retrying at the schedule's last (longest) step. The retry delay is
// also never allowed to exceed the connector's own scheduleSeconds cadence,
// so a short-cadence connector doesn't end up waiting an hour to recover.
//
// scheduleSeconds nil means manual-only: nextRunAt is always nil, but
// retryCount is still tracked for display purposes. retryCount is the
// connector's consecutive-failure count *before* this attempt.
func computeNextRun(scheduleSeconds *int, retryCount int, success bool, now time.Time) (nextRunAt *time.Time, newRetryCount int) {
	if success {
		newRetryCount = 0
	} else {
		newRetryCount = retryCount + 1
	}

	if scheduleSeconds == nil {
		return nil, newRetryCount
	}
	cadence := time.Duration(*scheduleSeconds) * time.Second

	delay := cadence
	if !success {
		idx := newRetryCount - 1
		if idx >= len(retrySchedule) {
			idx = len(retrySchedule) - 1
		}
		delay = retrySchedule[idx]
		if cadence < delay {
			delay = cadence
		}
	}

	next := now.Add(delay)
	return &next, newRetryCount
}

// AlertNotifier dispatches notifications for a newly created alert.
type AlertNotifier interface {
	NotifyAlertCreated(ctx context.Context, alertID, title, message string)
}

// Engine runs sync jobs against connectors.
type Engine struct {
	store    *store.Store
	hub      *ws.Hub
	notifier AlertNotifier
}

// NewEngine creates a new sync engine.
func NewEngine(s *store.Store, h *ws.Hub, notifier AlertNotifier) *Engine {
	return &Engine{store: s, hub: h, notifier: notifier}
}

// RunResult holds the outcome of a sync run.
type RunResult struct {
	ConnectorID  string `json:"connectorId"`
	SnapshotID   string `json:"snapshotId"`
	ChangesCount int    `json:"changesCount"`
	AlertsCount  int    `json:"alertsCount"`
	Status       string `json:"status"` // "success", "error"
	Error        string `json:"error,omitempty"`
	Duration     string `json:"duration"`
}

// RunSync runs a sync for a single connector.
// Flow: Fetch -> Save Snapshot -> Diff -> Create Changes -> Create Alerts
func (e *Engine) RunSync(ctx context.Context, connectorID string, jobID string) (*RunResult, error) {
	start := time.Now()
	result := &RunResult{ConnectorID: connectorID}

	broadcast := func(phase string, percent int) {
		if e.hub != nil {
			e.hub.Broadcast(ws.EventSyncProgress, map[string]any{
				"serviceId": connectorID,
				"jobId":     jobID,
				"phase":     phase,
				"percent":   percent,
			})
		}
	}

	broadcast("queued", 0)

	// Get connector record
	rec, err := e.store.GetConnector(ctx, connectorID)
	if err != nil {
		slog.Error("sync get connector failed", "connector", connectorID, "error", err)
		if e.hub != nil {
			e.hub.Broadcast(ws.EventSyncProgress, map[string]any{
				"serviceId": connectorID,
				"jobId":     jobID,
				"phase":     "error",
				"percent":   0,
				"message":   err.Error(),
			})
		}
		return markError(result, start, fmt.Errorf("get connector: %w", err))
	}

	attempt := rec.RetryCount + 1

	// finish records this run's outcome as a sync_runs history row and, for
	// non-skipped runs, persists the resulting retry/backoff + next_run_at
	// schedule state on the connector (see computeNextRun). Called from every
	// exit path below once rec has been loaded.
	finish := func(status string, runErr error) {
		durationMs := int(time.Since(start).Milliseconds())
		errMsg := ""
		if runErr != nil {
			errMsg = runErr.Error()
		}
		runStatus := store.SyncRunStatusSuccess
		switch status {
		case "error":
			runStatus = store.SyncRunStatusError
		case "skipped":
			runStatus = store.SyncRunStatusSkipped
		}
		if err := e.store.CreateSyncRun(ctx, &store.SyncRunRecord{
			ConnectorID:  connectorID,
			StartedAt:    start.UTC().Format(time.RFC3339),
			FinishedAt:   time.Now().UTC().Format(time.RFC3339),
			DurationMs:   &durationMs,
			Status:       runStatus,
			Error:        errMsg,
			Attempt:      attempt,
			ChangesCount: result.ChangesCount,
			AlertsCount:  result.AlertsCount,
		}); err != nil {
			slog.Error("record sync run failed", "connector", connectorID, "error", err)
		}

		// Skipped (disabled connector) runs aren't real attempts: leave the
		// existing retry/schedule state untouched. ListDueConnectors also
		// filters on enabled=1, so a disabled connector's schedule is moot
		// until it's re-enabled anyway.
		if status == "skipped" {
			return
		}

		nextRunAt, newRetryCount := computeNextRun(rec.ScheduleSeconds, rec.RetryCount, status == "success", time.Now())
		updates := map[string]any{
			"last_sync_duration_ms": durationMs,
			"last_sync_error":       errMsg,
			"retry_count":           newRetryCount,
		}
		if nextRunAt != nil {
			updates["next_run_at"] = nextRunAt.UTC().Format(time.RFC3339)
		} else {
			updates["next_run_at"] = nil
		}
		if err := e.store.UpdateConnector(ctx, connectorID, updates); err != nil {
			slog.Error("update connector schedule failed", "connector", connectorID, "error", err)
		}
	}

	if !rec.Enabled {
		result.Status = "skipped"
		result.Duration = time.Since(start).String()
		finish("skipped", nil)
		return result, nil
	}

	// Parse config and inject top-level fields stored in separate columns
	// so connector factories see url + verify_tls alongside their custom fields.
	cfg, err := store.ParseConnectorConfig(rec.ConfigData)
	if err != nil {
		slog.Error("sync parse config failed", "connector", connectorID, "error", err)
		if e.hub != nil {
			e.hub.Broadcast(ws.EventSyncProgress, map[string]any{
				"serviceId": connectorID,
				"jobId":     jobID,
				"phase":     "error",
				"percent":   0,
				"message":   err.Error(),
			})
		}
		finish("error", err)
		return markError(result, start, fmt.Errorf("parse config: %w", err))
	}
	cfg["url"] = rec.URL
	cfg["verify_tls"] = rec.VerifyTLS

	// Get connector implementation
	conn, err := connector.Get(rec.Type, cfg)
	if err != nil {
		slog.Error("sync get connector impl failed", "connector", connectorID, "error", err)
		if e.hub != nil {
			e.hub.Broadcast(ws.EventSyncProgress, map[string]any{
				"serviceId": connectorID,
				"jobId":     jobID,
				"phase":     "error",
				"percent":   0,
				"message":   err.Error(),
			})
			e.hub.Broadcast(ws.EventSyncComplete, map[string]any{
				"serviceId":       connectorID,
				"jobId":           jobID,
				"changesDetected": 0,
				"alertsRaised":    0,
				"durationMs":      time.Since(start).Milliseconds(),
				"error":           err.Error(),
			})
		}
		finish("error", err)
		return markError(result, start, fmt.Errorf("get connector impl: %w", err))
	}

	// Update status to fetching
	_ = e.store.UpdateConnector(ctx, connectorID, map[string]any{
		"status":         "online",
		"status_message": "Syncing...",
	})

	broadcast("fetching", 28)

	// Fetch data
	sn, err := conn.Fetch(ctx, cfg)
	if err != nil {
		_ = e.store.UpdateConnector(ctx, connectorID, map[string]any{
			"status":         "degraded",
			"status_message": fmt.Sprintf("Fetch failed: %v", err),
		})
		slog.Error("sync fetch failed", "connector", connectorID, "error", err)
		if e.hub != nil {
			e.hub.Broadcast(ws.EventSyncProgress, map[string]any{
				"serviceId": connectorID,
				"jobId":     jobID,
				"phase":     "error",
				"percent":   0,
				"message":   err.Error(),
			})
			e.hub.Broadcast(ws.EventSyncComplete, map[string]any{
				"serviceId":       connectorID,
				"jobId":           jobID,
				"changesDetected": 0,
				"alertsRaised":    0,
				"durationMs":      time.Since(start).Milliseconds(),
				"error":           err.Error(),
			})
		}
		finish("error", err)
		return markError(result, start, fmt.Errorf("fetch: %w", err))
	}

	broadcast("diffing", 60)

	// Get previous snapshot for diff
	prevSn, prevErr := e.store.GetLatestSnapshot(ctx, connectorID)

	// Save new snapshot
	snData, _ := json.Marshal(sn)
	snRec := &store.SnapshotRecord{
		ConnectorID: connectorID,
		Data:        string(snData),
		FetchedAt:   sn.FetchedAt.Format(time.RFC3339),
	}
	if err := e.store.CreateSnapshot(ctx, snRec); err != nil {
		slog.Error("sync save snapshot failed", "connector", connectorID, "error", err)
		if e.hub != nil {
			e.hub.Broadcast(ws.EventSyncProgress, map[string]any{
				"serviceId": connectorID,
				"jobId":     jobID,
				"phase":     "error",
				"percent":   0,
				"message":   err.Error(),
			})
			e.hub.Broadcast(ws.EventSyncComplete, map[string]any{
				"serviceId":       connectorID,
				"jobId":           jobID,
				"changesDetected": 0,
				"alertsRaised":    0,
				"durationMs":      time.Since(start).Milliseconds(),
				"error":           err.Error(),
			})
		}
		finish("error", err)
		return markError(result, start, fmt.Errorf("save snapshot: %w", err))
	}
	result.SnapshotID = snRec.ID

	broadcast("generating", 85)

	// Diff against previous snapshot
	if prevErr == nil {
		var prevSnap connector.ServiceSnapshot
		if err := json.Unmarshal([]byte(prevSn.Data), &prevSnap); err == nil {
			diffResults := Compare(&prevSnap, sn)
			for _, d := range diffResults {
				diffJSON, _ := json.Marshal(d.Patches)
				change := &store.ChangeRecord{
					ServiceID:      connectorID,
					ChangeType:     d.Type,
					Severity:       d.Severity,
					Summary:        d.Summary,
					Diff:           string(diffJSON),
					AffectedDocIDs: "[]",
				}
				if err := e.store.CreateChange(ctx, change); err != nil {
					slog.Error("failed to create change", "error", err)
					continue
				}
				result.ChangesCount++

				if e.hub != nil {
					e.hub.Broadcast(ws.EventChangeDetected, map[string]any{
						"changeId":      change.ID,
						"serviceId":     connectorID,
						"changeType":    d.Type,
						"severity":      d.Severity,
						"summary":       d.Summary,
						"willTriggerAi": false,
					})
				}

				// Create alert for non-info changes
				if d.Severity != "info" {
					alert := &store.AlertRecord{
						ChangeID:    change.ID,
						ServiceID:   connectorID,
						Severity:    d.Severity,
						Title:       d.Summary,
						Description: d.Detail,
					}
					if err := e.store.CreateAlert(ctx, alert); err != nil {
						slog.Error("failed to create alert", "error", err)
						continue
					}
					result.AlertsCount++

					if e.notifier != nil {
						e.notifier.NotifyAlertCreated(ctx, alert.ID, alert.Title, alert.Description)
					}

					if e.hub != nil {
						e.hub.Broadcast(ws.EventAlertCreated, map[string]any{
							"alertId":   alert.ID,
							"serviceId": connectorID,
							"severity":  d.Severity,
							"title":     d.Summary,
						})
					}
				}
			}
		}
	}

	// Update connector status
	now := time.Now().UTC().Format(time.RFC3339)
	_ = e.store.UpdateConnector(ctx, connectorID, map[string]any{
		"status":         "online",
		"status_message": "Sync successful",
		"last_sync_at":   now,
	})

	result.Status = "success"
	result.Duration = time.Since(start).String()
	finish("success", nil)

	if e.hub != nil {
		e.hub.Broadcast(ws.EventSyncComplete, map[string]any{
			"serviceId":       connectorID,
			"jobId":           jobID,
			"changesDetected": result.ChangesCount,
			"alertsRaised":    result.AlertsCount,
			"durationMs":      time.Since(start).Milliseconds(),
		})
	}

	return result, nil
}

// RunSyncAll runs sync for all enabled connectors.
func (e *Engine) RunSyncAll(ctx context.Context, jobID string) ([]RunResult, error) {
	connectors, err := e.store.ListAllConnectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("list connectors: %w", err)
	}

	var results []RunResult
	for _, c := range connectors {
		if !c.Enabled {
			continue
		}
		result, err := e.RunSync(ctx, c.ID, jobID)
		if err != nil {
			slog.Error("sync failed", "connector", c.ID, "error", err)
			continue
		}
		results = append(results, *result)
	}
	return results, nil
}

func markError(r *RunResult, start time.Time, err error) (*RunResult, error) {
	r.Status = "error"
	r.Error = err.Error()
	r.Duration = time.Since(start).String()
	return r, err
}
