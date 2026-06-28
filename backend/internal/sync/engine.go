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
)

// Engine runs sync jobs against connectors.
type Engine struct {
	store *store.Store
}

// NewEngine creates a new sync engine.
func NewEngine(s *store.Store) *Engine {
	return &Engine{store: s}
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
func (e *Engine) RunSync(ctx context.Context, connectorID string) (*RunResult, error) {
	start := time.Now()
	result := &RunResult{ConnectorID: connectorID}

	// Get connector record
	rec, err := e.store.GetConnector(ctx, connectorID)
	if err != nil {
		return nil, fmt.Errorf("get connector: %w", err)
	}

	if !rec.Enabled {
		result.Status = "skipped"
		result.Duration = time.Since(start).String()
		return result, nil
	}

	// Parse config
	cfg, err := store.ParseConnectorConfig(rec.ConfigData)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Get connector implementation
	conn, err := connector.Get(rec.Type, cfg)
	if err != nil {
		return markError(result, start, fmt.Errorf("get connector impl: %w", err))
	}

	// Update status to fetching
	e.store.UpdateConnector(ctx, connectorID, map[string]any{
		"status":         "online",
		"status_message": "Syncing...",
	})

	// Fetch data
	sn, err := conn.Fetch(ctx, cfg)
	if err != nil {
		e.store.UpdateConnector(ctx, connectorID, map[string]any{
			"status":         "degraded",
			"status_message": fmt.Sprintf("Fetch failed: %v", err),
		})
		return markError(result, start, fmt.Errorf("fetch: %w", err))
	}

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
		return markError(result, start, fmt.Errorf("save snapshot: %w", err))
	}
	result.SnapshotID = snRec.ID

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
				}
			}
		}
	}

	// Update connector status
	now := time.Now().UTC().Format(time.RFC3339)
	e.store.UpdateConnector(ctx, connectorID, map[string]any{
		"status":         "online",
		"status_message": "Sync successful",
		"last_sync_at":   now,
	})

	result.Status = "success"
	result.Duration = time.Since(start).String()
	return result, nil
}

// RunSyncAll runs sync for all enabled connectors.
func (e *Engine) RunSyncAll(ctx context.Context) ([]RunResult, error) {
	connectors, err := e.store.ListAllConnectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("list connectors: %w", err)
	}

	var results []RunResult
	for _, c := range connectors {
		if !c.Enabled {
			continue
		}
		result, err := e.RunSync(ctx, c.ID)
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
