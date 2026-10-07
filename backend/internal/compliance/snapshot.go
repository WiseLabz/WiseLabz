package compliance

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// SnapshotFromConnector converts a stored connector snapshot into the engine's
// view, carrying the typed fields a related clause can join on.
func SnapshotFromConnector(sn connector.ServiceSnapshot) Snapshot {
	entities := make([]Entity, len(sn.Entities))
	for i, e := range sn.Entities {
		entities[i] = Entity{
			Kind: e.Kind, Name: e.Name, ExternalID: e.ExternalID, IP: e.IP,
			Hostname: e.Hostname, MAC: e.MAC, Attributes: e.Attributes,
		}
	}
	return Snapshot{Entities: entities}
}

// SnapshotSource is the store read LoadLatestSnapshot needs. *store.Store
// satisfies it.
type SnapshotSource interface {
	GetLatestSnapshot(ctx context.Context, connectorID string) (*store.SnapshotRecord, error)
}

// LoadLatestSnapshot loads connectorID's latest stored snapshot in the shape
// the engine reads. It returns nil, nil when the connector has no snapshot or
// its stored snapshot is malformed, so callers treat both as "nothing to
// evaluate" rather than as a failure.
func LoadLatestSnapshot(ctx context.Context, source SnapshotSource, connectorID string) (*Snapshot, error) {
	record, err := source.GetLatestSnapshot(ctx, connectorID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var snapshot connector.ServiceSnapshot
	if err := json.Unmarshal([]byte(record.Data), &snapshot); err != nil {
		slog.Warn("skipping malformed snapshot for compliance rule", "error", err)
		return nil, nil
	}
	result := SnapshotFromConnector(snapshot)
	return &result, nil
}
