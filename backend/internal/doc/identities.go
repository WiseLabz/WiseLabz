package doc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/logsafe"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// RebuildEntityIdentitiesForConnector refreshes persisted identities from all
// latest connector snapshots after one connector has synced.
func (e *Engine) RebuildEntityIdentitiesForConnector(ctx context.Context, _ string) error {
	_, err := e.BackfillEntityIdentities(ctx)
	return err
}

// BackfillEntityIdentities initializes or reconciles identities across every
// connector with a readable latest snapshot.
func (e *Engine) BackfillEntityIdentities(ctx context.Context) (int, error) {
	count := 0
	err := e.store.ReconcileEntityIdentitiesWith(ctx, func(ctx context.Context, tx *store.Store) ([][]store.EntityMemberRecord, []string, error) {
		connectors, err := tx.ListAllConnectors(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("list connectors for identity backfill: %w", err)
		}
		members := make([]store.EntityMemberRecord, 0)
		preserve := make([]string, 0)
		for _, c := range connectors {
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
			record, err := tx.GetLatestSnapshot(ctx, c.ID)
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			if err != nil {
				slog.Warn("identity reconciliation retained unreadable connector snapshot", "connector", logsafe.Sanitize(c.ID), "error", logsafe.Sanitize(err.Error()))
				preserve = append(preserve, c.ID)
				continue
			}
			var snap connector.ServiceSnapshot
			if err := json.Unmarshal([]byte(record.Data), &snap); err != nil {
				slog.Warn("identity reconciliation retained malformed connector snapshot", "connector", logsafe.Sanitize(c.ID), "error", logsafe.Sanitize(err.Error()))
				preserve = append(preserve, c.ID)
				continue
			}
			count++
			seen := map[string]bool{}
			for _, entity := range snap.Entities {
				ref := entityRef(entity)
				key := identityKey(c.ID, entity.Kind, ref)
				if seen[key] {
					continue
				}
				seen[key] = true
				members = append(members, store.EntityMemberRecord{ConnectorID: c.ID, Kind: entity.Kind, Ref: ref, Name: entity.Name, ObservedAt: record.FetchedAt, ExternalID: entity.ExternalID, Hostname: entity.Hostname, Aliases: entity.Aliases})
			}
		}
		return identityClusters(members), preserve, nil
	})
	if err != nil {
		return count, fmt.Errorf("reconcile entity identities: %w", err)
	}
	return count, nil
}

func identityKey(connectorID, kind, ref string) string {
	return connectorID + "\x00" + kind + "\x00" + ref
}

func identityClusters(members []store.EntityMemberRecord) [][]store.EntityMemberRecord {
	parent := make([]int, len(members))
	for i := range parent {
		parent[i] = i
	}
	var root func(int) int
	root = func(i int) int {
		if parent[i] != i {
			parent[i] = root(parent[i])
		}
		return parent[i]
	}
	join := func(a, b int) {
		ra, rb := root(a), root(b)
		if ra != rb {
			parent[rb] = ra
		}
	}
	features := make(map[string][]int)
	for i, m := range members {
		for _, key := range strongIdentityFeatures(connector.SnapshotEntity{Kind: m.Kind, ExternalID: m.ExternalID, Hostname: m.Hostname, Aliases: m.Aliases}) {
			features[key] = append(features[key], i)
		}
	}
	for _, indices := range features {
		byConnector := make(map[string][]int)
		connectorOrder := make([]string, 0)
		for _, index := range indices {
			id := members[index].ConnectorID
			if _, ok := byConnector[id]; !ok {
				connectorOrder = append(connectorOrder, id)
			}
			byConnector[id] = append(byConnector[id], index)
		}
		if len(connectorOrder) < 2 {
			continue
		}
		primary := byConnector[connectorOrder[0]]
		other := byConnector[connectorOrder[1]][0]
		for _, index := range primary {
			join(index, other)
		}
		for _, id := range connectorOrder[1:] {
			for _, index := range byConnector[id] {
				join(index, primary[0])
			}
		}
	}
	groups := map[int][]store.EntityMemberRecord{}
	for i, m := range members {
		groups[root(i)] = append(groups[root(i)], m)
	}
	clusters := make([][]store.EntityMemberRecord, 0, len(groups))
	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool {
			return identityKey(group[i].ConnectorID, group[i].Kind, group[i].Ref) < identityKey(group[j].ConnectorID, group[j].Kind, group[j].Ref)
		})
		clusters = append(clusters, group)
	}
	sort.Slice(clusters, func(i, j int) bool {
		return identityKey(clusters[i][0].ConnectorID, clusters[i][0].Kind, clusters[i][0].Ref) < identityKey(clusters[j][0].ConnectorID, clusters[j][0].Kind, clusters[j][0].Ref)
	})
	return clusters
}

// BackfillTopologyAndIdentities initializes both persisted graph layers for
// existing snapshots when this instance acquires leadership.
func (e *Engine) BackfillTopologyAndIdentities(ctx context.Context) (int, error) {
	identities, identityErr := e.BackfillEntityIdentities(ctx)
	topology, topologyErr := e.BackfillTopology(ctx)
	return identities + topology, errors.Join(identityErr, topologyErr)
}
