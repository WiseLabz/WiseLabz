package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// EntityMemberRecord identifies one connector-local observation of an entity.
type EntityMemberRecord struct {
	ConnectorID string
	Kind        string
	Ref         string
	Name        string
	ExternalID  string
	Hostname    string
	Aliases     []string
	ObservedAt  string
}

type entityIdentity struct {
	id, firstSeen string
}

// ReconcileEntityIdentities replaces the current member graph with clusters
// derived from the latest snapshots. Existing IDs are retained once per
// cluster; unmatched clusters receive new IDs and losing merged identities
// remain as redirects.
func (s *Store) ReconcileEntityIdentities(ctx context.Context, clusters [][]EntityMemberRecord) error {
	return s.WithinTransaction(ctx, func(tx *Store) error {
		rows, err := tx.db.QueryContext(ctx, `SELECT e.id, e.first_seen_at, m.connector_id, m.kind, m.ref
			FROM entities e LEFT JOIN entity_members m ON m.entity_id = e.id
			WHERE e.merged_into IS NULL`)
		if err != nil {
			return fmt.Errorf("load existing entity memberships: %w", err)
		}
		existing := map[string]entityIdentity{}
		memberIDs := map[string][]string{}
		for rows.Next() {
			var id, first string
			var connectorID, kind, ref sql.NullString
			if err := rows.Scan(&id, &first, &connectorID, &kind, &ref); err != nil {
				rows.Close() //nolint:errcheck
				return fmt.Errorf("scan existing entity membership: %w", err)
			}
			existing[id] = entityIdentity{id: id, firstSeen: first}
			if connectorID.Valid {
				key := identityMemberKey(connectorID.String, kind.String, ref.String)
				memberIDs[key] = append(memberIDs[key], id)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close() //nolint:errcheck
			return fmt.Errorf("iterate existing entity memberships: %w", err)
		}
		rows.Close() //nolint:errcheck

		sort.Slice(clusters, func(i, j int) bool { return clusterKey(clusters[i]) < clusterKey(clusters[j]) })
		now := time.Now().UTC().Format(time.RFC3339Nano)
		chosen := map[string]string{}
		used := map[string]bool{}
		mergedInto := map[string]string{}
		for i := range clusters {
			sort.Slice(clusters[i], func(a, b int) bool {
				return identityMemberKey(clusters[i][a].ConnectorID, clusters[i][a].Kind, clusters[i][a].Ref) <
					identityMemberKey(clusters[i][b].ConnectorID, clusters[i][b].Kind, clusters[i][b].Ref)
			})
			candidates := map[string]bool{}
			for _, member := range clusters[i] {
				for _, id := range memberIDs[identityMemberKey(member.ConnectorID, member.Kind, member.Ref)] {
					candidates[id] = true
				}
			}
			ids := make([]string, 0, len(candidates))
			for id := range candidates {
				if !used[id] {
					ids = append(ids, id)
				}
			}
			sort.Slice(ids, func(a, b int) bool {
				if existing[ids[a]].firstSeen == existing[ids[b]].firstSeen {
					return ids[a] < ids[b]
				}
				return existing[ids[a]].firstSeen < existing[ids[b]].firstSeen
			})
			id := uuid.NewString()
			if len(ids) > 0 {
				id = ids[0]
				used[id] = true
				for _, loser := range ids[1:] {
					mergedInto[loser] = id
				}
			}
			chosen[clusterKey(clusters[i])] = id
			firstSeen, lastSeen := now, ""
			if old, ok := existing[id]; ok {
				firstSeen = old.firstSeen
			}
			for _, m := range clusters[i] {
				if m.ObservedAt != "" && (firstSeen == now || m.ObservedAt < firstSeen) {
					firstSeen = m.ObservedAt
				}
				if m.ObservedAt > lastSeen {
					lastSeen = m.ObservedAt
				}
			}
			if lastSeen == "" {
				lastSeen = now
			}
			member := clusters[i][0]
			if _, err := tx.db.ExecContext(ctx, `INSERT INTO entities (id, kind, display_name, first_seen_at, last_seen_at, gone_at, merged_into)
				VALUES (?, ?, ?, ?, ?, NULL, NULL)
				ON CONFLICT(id) DO UPDATE SET kind = excluded.kind, display_name = excluded.display_name,
				last_seen_at = excluded.last_seen_at, gone_at = NULL, merged_into = NULL`,
				id, member.Kind, member.Name, firstSeen, lastSeen); err != nil {
				return fmt.Errorf("upsert entity identity: %w", err)
			}
		}
		if _, err := tx.db.ExecContext(ctx, `DELETE FROM entity_members`); err != nil {
			return fmt.Errorf("clear entity members: %w", err)
		}
		// Reinsert membership rows after clearing them to avoid cross-cluster
		// uniqueness conflicts while a split is being reconciled.
		for i := range clusters {
			id := chosen[clusterKey(clusters[i])]
			for _, m := range clusters[i] {
				if _, err := tx.db.ExecContext(ctx, `INSERT INTO entity_members (entity_id, connector_id, kind, ref, name) VALUES (?, ?, ?, ?, ?)`,
					id, m.ConnectorID, m.Kind, m.Ref, m.Name); err != nil {
					return fmt.Errorf("restore entity member: %w", err)
				}
			}
		}
		for id, old := range existing {
			if used[id] {
				continue
			}
			if target := mergedInto[id]; target != "" {
				if _, err := tx.db.ExecContext(ctx, `UPDATE entities SET merged_into = ?, gone_at = NULL WHERE id = ?`, target, id); err != nil {
					return fmt.Errorf("mark merged entity: %w", err)
				}
			} else if _, err := tx.db.ExecContext(ctx, `UPDATE entities SET gone_at = COALESCE(gone_at, ?), merged_into = NULL WHERE id = ?`, now, old.id); err != nil {
				return fmt.Errorf("mark gone entity: %w", err)
			}
		}
		return nil
	})
}

func identityMemberKey(connectorID, kind, ref string) string {
	return connectorID + "\x00" + kind + "\x00" + ref
}

func clusterKey(cluster []EntityMemberRecord) string {
	if len(cluster) == 0 {
		return ""
	}
	return identityMemberKey(cluster[0].ConnectorID, cluster[0].Kind, cluster[0].Ref)
}

// DeleteExpiredEntityIdentities removes gone and merged rows older than cutoff.
func (s *Store) DeleteExpiredEntityIdentities(ctx context.Context, cutoff string) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM entities WHERE
		(gone_at IS NOT NULL AND gone_at < ?) OR
		(merged_into IS NOT NULL AND last_seen_at < ?)`, cutoff, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete expired entity identities: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count expired entity identities: %w", err)
	}
	return n, nil
}
