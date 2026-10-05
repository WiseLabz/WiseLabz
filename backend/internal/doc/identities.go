package doc

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

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
	connectors, err := e.store.ListAllConnectors(ctx)
	if err != nil {
		return 0, fmt.Errorf("list connectors for identity backfill: %w", err)
	}
	members := make([]store.EntityMemberRecord, 0)
	count := 0
	for _, c := range connectors {
		if err := ctx.Err(); err != nil {
			return count, err
		}
		snap, err := e.snapshots.latest(ctx, c.ID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			return count, fmt.Errorf("load identity snapshot for %s: %w", c.ID, err)
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
			members = append(members, store.EntityMemberRecord{ConnectorID: c.ID, Kind: entity.Kind, Ref: ref, Name: entity.Name, ObservedAt: snap.FetchedAt.UTC().Format(time.RFC3339Nano), ExternalID: entity.ExternalID, Hostname: entity.Hostname, Aliases: entity.Aliases})
		}
	}
	clusters := identityClusters(members)
	if err := e.store.ReconcileEntityIdentities(ctx, clusters); err != nil {
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
		if m.ExternalID != "" {
			key := "id\x00" + m.Kind + "\x00" + m.ExternalID
			features[key] = append(features[key], i)
		}
		for _, hostname := range append([]string{m.Hostname}, m.Aliases...) {
			if hostname != "" {
				key := "host\x00" + strings.ToLower(hostname)
				features[key] = append(features[key], i)
			}
		}
	}
	for _, indices := range features {
		for a := 0; a < len(indices); a++ {
			for b := a + 1; b < len(indices); b++ {
				if members[indices[a]].ConnectorID != members[indices[b]].ConnectorID {
					join(indices[a], indices[b])
				}
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
