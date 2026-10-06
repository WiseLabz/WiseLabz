package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var entityIdentityReconcileMu sync.Mutex

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
	id, kind, name, firstSeen, lastSeen string
	goneAt, mergedInto, mergedAt        sql.NullString
}

type storedIdentityMember struct {
	entityID, connectorID, kind, ref, name string
	goneAt                                 sql.NullString
}

// ReconcileEntityIdentities serializes and applies a precomputed identity graph.
// Snapshot-backed callers should use ReconcileEntityIdentitiesWith so snapshot
// reads happen after the reconcile lock is held.
func (s *Store) ReconcileEntityIdentities(ctx context.Context, clusters [][]EntityMemberRecord) error {
	return s.ReconcileEntityIdentitiesWith(ctx, func(context.Context, *Store) ([][]EntityMemberRecord, []string, error) {
		return clusters, nil, nil
	})
}

// ReconcileEntityIdentitiesWith reads and writes identities inside one
// serialized transaction. preserveConnectors are connectors whose latest
// snapshots could not be read; their last known memberships remain active.
func (s *Store) ReconcileEntityIdentitiesWith(ctx context.Context, build func(context.Context, *Store) ([][]EntityMemberRecord, []string, error)) error {
	entityIdentityReconcileMu.Lock()
	defer entityIdentityReconcileMu.Unlock()
	return s.WithinTransaction(ctx, func(tx *Store) error {
		if s.driver == "postgres" {
			if _, err := tx.db.ExecContext(ctx, `SELECT pg_advisory_xact_lock(731058502)`); err != nil {
				return fmt.Errorf("lock entity identity reconciliation: %w", err)
			}
		}
		clusters, preserve, err := build(ctx, tx)
		if err != nil {
			return err
		}
		return tx.reconcileEntityIdentities(ctx, clusters, preserve)
	})
}

func (s *Store) reconcileEntityIdentities(ctx context.Context, clusters [][]EntityMemberRecord, preserveConnectors []string) error {
	entities, oldMembers, err := loadIdentityState(ctx, s)
	if err != nil {
		return err
	}
	preserve := make(map[string]bool, len(preserveConnectors))
	for _, id := range preserveConnectors {
		preserve[id] = true
	}
	sortIdentityClusters(clusters)
	chosen, mergedInto := chooseIdentityIDs(clusters, oldMembers, entities)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	updates, active, desired, gone := buildIdentityDiff(clusters, chosen, oldMembers, entities, preserve, now)
	flattenIdentityRedirects(entities, mergedInto, updates, now)
	markUnobservedIdentitiesGone(entities, active, mergedInto, updates, now)

	if err := upsertIdentityRows(ctx, s, updates); err != nil {
		return err
	}
	if err := markIdentityMembersGone(ctx, s, gone, now); err != nil {
		return err
	}
	if err := upsertIdentityMembers(ctx, s, oldMembers, desired); err != nil {
		return err
	}
	return moveMergedIdentityMembers(ctx, s, mergedInto)
}

func loadIdentityState(ctx context.Context, s *Store) (map[string]entityIdentity, map[string]storedIdentityMember, error) {
	entities, err := loadEntityRows(ctx, s)
	if err != nil {
		return nil, nil, err
	}
	members, err := loadIdentityMemberRows(ctx, s)
	if err != nil {
		return nil, nil, err
	}
	return entities, members, nil
}

func loadEntityRows(ctx context.Context, s *Store) (map[string]entityIdentity, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, kind, display_name, first_seen_at, last_seen_at, gone_at, merged_into, merged_at FROM entities`)
	if err != nil {
		return nil, fmt.Errorf("load existing entity identities: %w", err)
	}
	defer func() { _ = rows.Close() }()
	entities := make(map[string]entityIdentity)
	for rows.Next() {
		var e entityIdentity
		if err := rows.Scan(&e.id, &e.kind, &e.name, &e.firstSeen, &e.lastSeen, &e.goneAt, &e.mergedInto, &e.mergedAt); err != nil {
			return nil, fmt.Errorf("scan existing entity identity: %w", err)
		}
		entities[e.id] = e
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate existing entity identities: %w", err)
	}
	return entities, nil
}

func loadIdentityMemberRows(ctx context.Context, s *Store) (map[string]storedIdentityMember, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT entity_id, connector_id, kind, ref, name, gone_at FROM entity_members`)
	if err != nil {
		return nil, fmt.Errorf("load existing entity members: %w", err)
	}
	defer func() { _ = rows.Close() }()
	members := make(map[string]storedIdentityMember)
	for rows.Next() {
		var m storedIdentityMember
		if err := rows.Scan(&m.entityID, &m.connectorID, &m.kind, &m.ref, &m.name, &m.goneAt); err != nil {
			return nil, fmt.Errorf("scan existing entity member: %w", err)
		}
		members[identityMemberKey(m.connectorID, m.kind, m.ref)] = m
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate existing entity members: %w", err)
	}
	return members, nil
}

func sortIdentityClusters(clusters [][]EntityMemberRecord) {
	for i := range clusters {
		sort.Slice(clusters[i], func(a, b int) bool {
			return identityMemberKey(clusters[i][a].ConnectorID, clusters[i][a].Kind, clusters[i][a].Ref) < identityMemberKey(clusters[i][b].ConnectorID, clusters[i][b].Kind, clusters[i][b].Ref)
		})
	}
	sort.Slice(clusters, func(i, j int) bool { return clusterKey(clusters[i]) < clusterKey(clusters[j]) })
}

func resolveExistingIdentity(id string, entities map[string]entityIdentity) string {
	seen := map[string]bool{}
	for id != "" && entities[id].mergedInto.Valid && !seen[id] {
		seen[id] = true
		id = entities[id].mergedInto.String
	}
	return id
}

func chooseIdentityIDs(clusters [][]EntityMemberRecord, members map[string]storedIdentityMember, entities map[string]entityIdentity) (map[string]string, map[string]string) {
	chosen, mergedInto, used := make(map[string]string, len(clusters)), map[string]string{}, map[string]bool{}
	// Clusters holding currently observed members claim existing IDs before
	// clusters made only of returning (gone) members; within each group the
	// cluster holding the oldest candidate identity goes first, so the outcome
	// never depends on connector UUID or map order. clusterKey only breaks ties.
	type claimOrder struct {
		index  int
		active bool
		oldest string // "" when the cluster has no existing identity
		spread int    // distinct existing identities the cluster would merge
	}
	older := func(a, b string) bool {
		fa, fb := entities[a].firstSeen, entities[b].firstSeen
		if fa != fb {
			return fa < fb
		}
		return a < b
	}
	claims := make([]claimOrder, len(clusters))
	for i, cluster := range clusters {
		claims[i] = claimOrder{index: i, active: clusterHasActiveMember(cluster, members)}
		seen := map[string]bool{}
		for _, member := range cluster {
			if old, ok := members[identityMemberKey(member.ConnectorID, member.Kind, member.Ref)]; ok {
				id := resolveExistingIdentity(old.entityID, entities)
				if id == "" {
					continue
				}
				seen[id] = true
				if claims[i].oldest == "" || older(id, claims[i].oldest) {
					claims[i].oldest = id
				}
			}
		}
		claims[i].spread = len(seen)
	}
	sort.SliceStable(claims, func(i, j int) bool {
		x, y := claims[i], claims[j]
		switch {
		case x.active != y.active:
			return x.active
		case x.oldest != y.oldest:
			if x.oldest == "" || y.oldest == "" {
				return y.oldest == ""
			}
			return older(x.oldest, y.oldest)
		case x.spread != y.spread:
			return x.spread > y.spread
		}
		return clusterKey(clusters[x.index]) < clusterKey(clusters[y.index])
	})
	order := make([]int, len(claims))
	for i, c := range claims {
		order[i] = c.index
	}
	for _, ci := range order {
		cluster := clusters[ci]
		candidates := map[string]bool{}
		for _, member := range cluster {
			if old, ok := members[identityMemberKey(member.ConnectorID, member.Kind, member.Ref)]; ok {
				candidates[resolveExistingIdentity(old.entityID, entities)] = true
			}
		}
		ids := make([]string, 0, len(candidates))
		for id := range candidates {
			if id != "" && !used[id] && mergedInto[id] == "" {
				ids = append(ids, id)
			}
		}
		sort.Slice(ids, func(i, j int) bool {
			a, b := entities[ids[i]].firstSeen, entities[ids[j]].firstSeen
			if a == b {
				return ids[i] < ids[j]
			}
			return a < b
		})
		id := uuid.NewString()
		if len(ids) > 0 {
			id, used[ids[0]] = ids[0], true
			for _, loser := range ids[1:] {
				mergedInto[loser] = id
			}
		}
		chosen[clusterKey(cluster)] = id
	}
	return chosen, mergedInto
}

func clusterHasActiveMember(cluster []EntityMemberRecord, members map[string]storedIdentityMember) bool {
	for _, m := range cluster {
		if old, ok := members[identityMemberKey(m.ConnectorID, m.Kind, m.Ref)]; ok && !old.goneAt.Valid {
			return true
		}
	}
	return false
}

func buildIdentityDiff(clusters [][]EntityMemberRecord, chosen map[string]string, oldMembers map[string]storedIdentityMember, entities map[string]entityIdentity, preserve map[string]bool, now string) (map[string]entityIdentity, map[string]bool, map[string]storedIdentityMember, []storedIdentityMember) {
	updates, active, desired := map[string]entityIdentity{}, map[string]bool{}, map[string]storedIdentityMember{}
	for _, old := range oldMembers {
		if preserve[old.connectorID] && !old.goneAt.Valid {
			active[resolveExistingIdentity(old.entityID, entities)] = true
		}
	}
	for _, cluster := range clusters {
		id, first, last := chosen[clusterKey(cluster)], now, ""
		for _, m := range cluster {
			if m.ObservedAt != "" && m.ObservedAt < first {
				first = m.ObservedAt
			}
			if m.ObservedAt > last {
				last = m.ObservedAt
			}
			key := identityMemberKey(m.ConnectorID, m.Kind, m.Ref)
			desired[key] = storedIdentityMember{entityID: id, connectorID: m.ConnectorID, kind: m.Kind, ref: m.Ref, name: m.Name}
			active[id] = true
		}
		if last == "" {
			last = now
		}
		member, old := cluster[0], entities[id]
		_, exists := entities[id]
		if exists && old.firstSeen < first {
			first = old.firstSeen
		}
		updated := entityIdentity{id: id, kind: member.Kind, name: member.Name, firstSeen: first, lastSeen: last}
		if !exists || old.kind != updated.kind || old.name != updated.name || old.firstSeen != updated.firstSeen || old.lastSeen != updated.lastSeen || old.goneAt.Valid || old.mergedInto.Valid {
			updates[id] = updated
		}
	}
	gone := make([]storedIdentityMember, 0)
	for key, old := range oldMembers {
		if preserve[old.connectorID] {
			continue
		}
		if _, present := desired[key]; !present && !old.goneAt.Valid {
			gone = append(gone, old)
		}
	}
	return updates, active, desired, gone
}

func resolveFinalIdentity(id string, entities map[string]entityIdentity, mergedInto map[string]string) string {
	seen := map[string]bool{}
	for id != "" && !seen[id] {
		seen[id] = true
		if target := mergedInto[id]; target != "" {
			id = target
			continue
		}
		if entities[id].mergedInto.Valid {
			id = entities[id].mergedInto.String
			continue
		}
		break
	}
	return id
}

// flattenIdentityRedirects records new merges (stamping merged_at) and re-points
// older redirects at their final winner, keeping their original merged_at.
func flattenIdentityRedirects(entities map[string]entityIdentity, mergedInto map[string]string, updates map[string]entityIdentity, now string) {
	for id, target := range mergedInto {
		old := entities[id]
		old.id, old.mergedInto, old.goneAt = id, sql.NullString{String: target, Valid: true}, sql.NullString{}
		old.mergedAt = sql.NullString{String: now, Valid: true}
		updates[id] = old
	}
	for id, old := range entities {
		if old.mergedInto.Valid && resolveFinalIdentity(old.mergedInto.String, entities, mergedInto) != old.mergedInto.String {
			old.id = id
			old.mergedInto = sql.NullString{String: resolveFinalIdentity(old.mergedInto.String, entities, mergedInto), Valid: true}
			updates[id] = old
		}
	}
}

func markUnobservedIdentitiesGone(entities map[string]entityIdentity, active map[string]bool, mergedInto map[string]string, updates map[string]entityIdentity, now string) {
	for id, old := range entities {
		if old.mergedInto.Valid || active[id] || mergedInto[id] != "" || old.goneAt.Valid {
			continue
		}
		old.id, old.goneAt = id, sql.NullString{String: now, Valid: true}
		updates[id] = old
	}
}

func moveMergedIdentityMembers(ctx context.Context, s *Store, mergedInto map[string]string) error {
	for loser, winner := range mergedInto {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM entity_members WHERE entity_id = ? AND EXISTS (
			SELECT 1 FROM entity_members winner_member WHERE winner_member.entity_id = ?
			AND winner_member.connector_id = entity_members.connector_id AND winner_member.kind = entity_members.kind AND winner_member.ref = entity_members.ref)`, loser, winner); err != nil {
			return fmt.Errorf("deduplicate merged entity members: %w", err)
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE entity_members SET entity_id = ? WHERE entity_id = ?`, winner, loser); err != nil {
			return fmt.Errorf("move merged entity members: %w", err)
		}
	}
	return nil
}

func upsertIdentityRows(ctx context.Context, s *Store, rows map[string]entityIdentity) error {
	ids := make([]string, 0, len(rows))
	for id := range rows {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	const batch = 100
	for start := 0; start < len(ids); start += batch {
		end := start + batch
		if end > len(ids) {
			end = len(ids)
		}
		values, args := make([]string, 0, end-start), make([]any, 0, (end-start)*8)
		for _, id := range ids[start:end] {
			e := rows[id]
			gone, merged, mergedAt := any(nil), any(nil), any(nil)
			if e.goneAt.Valid {
				gone = e.goneAt.String
			}
			if e.mergedInto.Valid {
				merged = e.mergedInto.String
			}
			if e.mergedAt.Valid {
				mergedAt = e.mergedAt.String
			}
			values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?)")
			args = append(args, e.id, e.kind, e.name, e.firstSeen, e.lastSeen, gone, merged, mergedAt)
		}
		query := `INSERT INTO entities (id, kind, display_name, first_seen_at, last_seen_at, gone_at, merged_into, merged_at) VALUES ` + strings.Join(values, ",") +
			` ON CONFLICT(id) DO UPDATE SET kind=excluded.kind, display_name=excluded.display_name, first_seen_at=excluded.first_seen_at, last_seen_at=excluded.last_seen_at, gone_at=excluded.gone_at, merged_into=excluded.merged_into, merged_at=excluded.merged_at`
		if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
			return fmt.Errorf("upsert entity identities: %w", err)
		}
	}
	return nil
}

func markIdentityMembersGone(ctx context.Context, s *Store, members []storedIdentityMember, goneAt string) error {
	const batch = 200
	for start := 0; start < len(members); start += batch {
		end := start + batch
		if end > len(members) {
			end = len(members)
		}
		values, args := make([]string, 0, end-start), []any{goneAt}
		for _, m := range members[start:end] {
			values = append(values, "(?, ?, ?)")
			args = append(args, m.connectorID, m.kind, m.ref)
		}
		query := `UPDATE entity_members SET gone_at = ? WHERE (connector_id, kind, ref) IN (` + strings.Join(values, ",") + `) AND gone_at IS NULL`
		if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
			return fmt.Errorf("mark missing entity members gone: %w", err)
		}
	}
	return nil
}

func upsertIdentityMembers(ctx context.Context, s *Store, old, desired map[string]storedIdentityMember) error {
	keys := make([]string, 0, len(desired))
	newKeys := make([]string, 0, len(desired))
	for key, m := range desired {
		prior, ok := old[key]
		if ok && prior.entityID == m.entityID && prior.name == m.name && !prior.goneAt.Valid {
			continue
		}
		keys = append(keys, key)
		if !ok {
			newKeys = append(newKeys, key)
		}
	}
	sort.Strings(keys)
	sort.Strings(newKeys)
	const batch = 200
	for start := 0; start < len(keys); start += batch {
		end := start + batch
		if end > len(keys) {
			end = len(keys)
		}
		entityCases := make([]string, 0, end-start)
		nameCases := make([]string, 0, end-start)
		keyClauses := make([]string, 0, end-start)
		entityArgs, nameArgs, whereArgs := make([]any, 0, (end-start)*4), make([]any, 0, (end-start)*4), make([]any, 0, (end-start)*3)
		for _, key := range keys[start:end] {
			m := desired[key]
			if _, ok := old[key]; !ok {
				continue
			}
			entityCases = append(entityCases, "WHEN connector_id=? AND kind=? AND ref=? THEN ?")
			entityArgs = append(entityArgs, m.connectorID, m.kind, m.ref, m.entityID)
			nameCases = append(nameCases, "WHEN connector_id=? AND kind=? AND ref=? THEN ?")
			nameArgs = append(nameArgs, m.connectorID, m.kind, m.ref, m.name)
			keyClauses = append(keyClauses, "(?, ?, ?)")
			whereArgs = append(whereArgs, m.connectorID, m.kind, m.ref)
		}
		if len(entityCases) > 0 {
			query := `UPDATE entity_members SET entity_id=CASE ` + strings.Join(entityCases, " ") + ` ELSE entity_id END, name=CASE ` + strings.Join(nameCases, " ") + ` ELSE name END, gone_at=NULL WHERE (connector_id,kind,ref) IN (` + strings.Join(keyClauses, ",") + `)`
			args := append(entityArgs, nameArgs...)
			args = append(args, whereArgs...)
			if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
				return fmt.Errorf("update entity members: %w", err)
			}
		}
	}
	for start := 0; start < len(newKeys); start += batch {
		end := start + batch
		if end > len(newKeys) {
			end = len(newKeys)
		}
		values, args := make([]string, 0, end-start), make([]any, 0, (end-start)*6)
		for _, key := range newKeys[start:end] {
			m := desired[key]
			values = append(values, "(?, ?, ?, ?, ?, NULL)")
			args = append(args, m.entityID, m.connectorID, m.kind, m.ref, m.name)
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO entity_members (entity_id,connector_id,kind,ref,name,gone_at) VALUES `+strings.Join(values, ","), args...); err != nil {
			return fmt.Errorf("insert entity members: %w", err)
		}
	}
	return nil
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

// DeleteExpiredEntityIdentities removes rows gone, or merged, before cutoff
// (merged age is measured from merged_at, not last observation);
// member history is removed by the entity_members foreign-key cascade. In the
// same transaction it deletes manual overrides whose member history is removed by
// this run; overrides of members that were never observed are kept.
func (s *Store) DeleteExpiredEntityIdentities(ctx context.Context, cutoff string) (int64, error) {
	var removed int64
	entityIdentityReconcileMu.Lock()
	defer entityIdentityReconcileMu.Unlock()
	err := s.WithinTransaction(ctx, func(tx *Store) error {
		if s.driver == "postgres" {
			if _, err := tx.db.ExecContext(ctx, `SELECT pg_advisory_xact_lock(731058502)`); err != nil {
				return fmt.Errorf("lock entity identity purge: %w", err)
			}
		}
		const predicate = `(gone_at IS NOT NULL AND gone_at < ?) OR (merged_into IS NOT NULL AND merged_at < ?)`
		// Overrides go first, while the member rows they are judged by still
		// exist: an override is purged only when this run removes the history of
		// one of its members (it has rows and every row is expiring). A member
		// that was never observed, e.g. after a restore, keeps its override.
		const historyExpires = `(EXISTS (SELECT 1 FROM entity_members m WHERE m.connector_id = %[1]s AND m.kind = %[2]s AND m.ref = %[3]s)
			AND NOT EXISTS (SELECT 1 FROM entity_members m WHERE m.connector_id = %[1]s AND m.kind = %[2]s AND m.ref = %[3]s
				AND m.entity_id NOT IN (SELECT id FROM entities WHERE ` + predicate + `)))`
		const o = "entity_identity_overrides."
		overrideQuery := `DELETE FROM entity_identity_overrides WHERE ` +
			fmt.Sprintf(historyExpires, o+"connector_id", o+"kind", o+"ref") +
			` OR (other_connector_id IS NOT NULL AND ` + fmt.Sprintf(historyExpires, o+"other_connector_id", o+"other_kind", o+"other_ref") + `)`
		if _, err := tx.db.ExecContext(ctx, overrideQuery, cutoff, cutoff, cutoff, cutoff); err != nil {
			return fmt.Errorf("delete overrides of expired entity members: %w", err)
		}
		if _, err := tx.db.ExecContext(ctx, `DELETE FROM entity_members WHERE entity_id IN (SELECT id FROM entities WHERE `+predicate+`)`, cutoff, cutoff); err != nil {
			return fmt.Errorf("delete expired entity members: %w", err)
		}
		result, err := tx.db.ExecContext(ctx, `DELETE FROM entities WHERE `+predicate, cutoff, cutoff)
		if err != nil {
			return fmt.Errorf("delete expired entity identities: %w", err)
		}
		removed, err = result.RowsAffected()
		if err != nil {
			return fmt.Errorf("count expired entity identities: %w", err)
		}
		return nil
	})
	return removed, err
}
