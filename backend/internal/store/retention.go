package store

import (
	"context"
	"fmt"
)

// DeleteOldSnapshots removes service_snapshots rows fetched before cutoff,
// never deleting a connector's most recent snapshot (GetLatestSnapshot's
// "current data" for that service).
func (s *Store) DeleteOldSnapshots(ctx context.Context, cutoff string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM service_snapshots
		WHERE fetched_at < ?
		AND (connector_id, fetched_at) NOT IN (
			SELECT connector_id, MAX(fetched_at) FROM service_snapshots GROUP BY connector_id
		)
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old snapshots: %w", err)
	}
	return rowsAffected(res), nil
}

// DeleteOldDocVersions removes doc_versions rows created before cutoff,
// never deleting the revision matching the doc's current_version.
func (s *Store) DeleteOldDocVersions(ctx context.Context, cutoff string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM doc_versions
		WHERE created_at < ?
		AND rev != (SELECT current_version FROM docs WHERE docs.id = doc_versions.doc_id)
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old doc versions: %w", err)
	}
	return rowsAffected(res), nil
}

// DeleteOldAlerts removes resolved/dismissed alerts created before cutoff.
// Pending and snoozed alerts are active workflow state and are never purged
// regardless of age.
func (s *Store) DeleteOldAlerts(ctx context.Context, cutoff string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM alerts
		WHERE created_at < ?
		AND status IN ('resolved', 'dismissed')
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old alerts: %w", err)
	}
	return rowsAffected(res), nil
}

// DeleteOldSyncRuns removes sync_runs rows started before cutoff. A
// connector's last-sync status lives on the connectors row itself, so no
// "keep latest" guard is needed here.
func (s *Store) DeleteOldSyncRuns(ctx context.Context, cutoff string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sync_runs WHERE started_at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old sync runs: %w", err)
	}
	return rowsAffected(res), nil
}

// rowsAffected returns res.RowsAffected(), or 0 if the driver doesn't
// support it — retention counts are informational, not worth failing over.
func rowsAffected(res interface{ RowsAffected() (int64, error) }) int64 {
	n, err := res.RowsAffected()
	if err != nil {
		return 0
	}
	return n
}
