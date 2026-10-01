package doc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// snapshotCache keeps each connector's latest parsed snapshot in memory so
// rendering and cross-connector linking don't reload and re-parse every
// snapshot blob. An entry is reused only while the connector's latest
// snapshot ID is unchanged (checked with a cheap ID-only query), so a new
// sync invalidates it. Cached snapshots are shared and must not be mutated.
type snapshotCache struct {
	store   *store.Store
	mu      sync.Mutex
	entries map[string]cachedSnapshot
}

type cachedSnapshot struct {
	snapshotID string
	snap       *connector.ServiceSnapshot
}

func newSnapshotCache(s *store.Store) *snapshotCache {
	return &snapshotCache{store: s, entries: map[string]cachedSnapshot{}}
}

// latest returns the parsed latest snapshot of a connector.
func (c *snapshotCache) latest(ctx context.Context, connectorID string) (*connector.ServiceSnapshot, error) {
	id, err := c.store.GetLatestSnapshotID(ctx, connectorID)
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}
	c.mu.Lock()
	entry, ok := c.entries[connectorID]
	c.mu.Unlock()
	if ok && entry.snapshotID == id {
		return entry.snap, nil
	}

	sn, err := c.store.GetLatestSnapshot(ctx, connectorID)
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}
	var snap connector.ServiceSnapshot
	if err := json.Unmarshal([]byte(sn.Data), &snap); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}
	c.mu.Lock()
	c.entries[connectorID] = cachedSnapshot{snapshotID: sn.ID, snap: &snap}
	c.mu.Unlock()
	return &snap, nil
}
