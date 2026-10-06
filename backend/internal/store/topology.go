package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Topology edge kinds.
const (
	// TopologyEdgeDependency links a connector's service node to something
	// it depends on (host, network, storage, upstream service).
	TopologyEdgeDependency = "dependency"
	// TopologyEdgeSameAs links entities of two connectors that were matched
	// as the same object (external ID, IP address or hostname).
	TopologyEdgeSameAs = "same_as"
	// TopologyEdgeContains links a connector's service node to one of its
	// own entities that participates in a match or dependency.
	TopologyEdgeContains = "contains"
	// TopologyEdgeResolvesTo is directed from a DNS record to the entity that
	// owns the IP or hostname it resolves to. Its owner is the DNS entity's
	// connector.
	TopologyEdgeResolvesTo = "resolves_to"
	// TopologyEdgeProxiesTo is directed from a proxy host, route, router or
	// stream to its upstream; Detail holds the upstream port when known.
	TopologyEdgeProxiesTo = "proxies_to"
	// TopologyEdgeRunsOn is directed from a container or VM to its host,
	// node or environment.
	TopologyEdgeRunsOn = "runs_on"
)

// TopologyEdge is one persisted edge of the entity-level topology graph.
// Both endpoints are identified by their owning connector plus kind and
// name (Ref is the connector-local external ID when known). The graph is
// treated as undirected by path queries unless the caller asks for directed
// traversal; Source records how the edge was derived so heuristic matches
// stay explainable.
type TopologyEdge struct {
	ID             string `json:"id"`
	ConnectorID    string `json:"connectorId"` // connector whose sync produced the edge
	SrcConnectorID string `json:"srcConnectorId"`
	SrcKind        string `json:"srcKind"`
	SrcName        string `json:"srcName"`
	SrcRef         string `json:"srcRef"`
	DstConnectorID string `json:"dstConnectorId"`
	DstKind        string `json:"dstKind"`
	DstName        string `json:"dstName"`
	DstRef         string `json:"dstRef"`
	Kind           string `json:"kind"`
	Source         string `json:"source"`
	Detail         string `json:"detail,omitempty"`
	CreatedAt      string `json:"createdAt"`
}

const topologyEdgeColumns = `id, connector_id, src_connector_id, src_kind, src_name, src_ref,
	dst_connector_id, dst_kind, dst_name, dst_ref, kind, source, detail, created_at`

// topologyRebuildMu and the advisory lock below serialize edge replacement so
// two connectors' rebuilds cannot both delete and re-insert the same shared
// logical rows and leave duplicates (key distinct from the identity lock).
var topologyRebuildMu sync.Mutex

const topologyRebuildAdvisoryLock = 731058503

// topologyEdgesAffectedBy selects the edges a rebuild of one connector
// replaces (argument: the connector ID, four times): everything it owns, any
// same_as edge touching it (re-derived symmetrically from its latest
// snapshot), and any resolves_to edge pointing at one of its entities, which
// belongs to the DNS connector but is re-derived by this rebuild so it cannot
// outlive the entity it points at.
const topologyEdgesAffectedBy = `connector_id = ?
	OR (kind = '` + TopologyEdgeSameAs + `' AND (src_connector_id = ? OR dst_connector_id = ?))
	OR (kind = '` + TopologyEdgeResolvesTo + `' AND dst_connector_id = ?`

// topologyInsertBatch is the rows per INSERT: 14 columns each keeps a
// statement far below both databases' bind-parameter limits.
const topologyInsertBatch = 200

// topologyLogicalEdgeColumns identify an edge without its storage owner, row
// ID, or creation time. same_as ownership is canonicalized from its endpoint
// order below, while other shared edges retain the owner that re-derived them.
const topologyLogicalEdgeColumns = `src_connector_id, src_kind, COALESCE(NULLIF(src_ref, ''), src_name),
	dst_connector_id, dst_kind, COALESCE(NULLIF(dst_ref, ''), dst_name), kind, source, detail`

// ReplaceTopologyEdgesForConnector atomically rebuilds the edges affected by
// connectorID's rebuild (see topologyEdgesAffectedBy), so entities that
// disappeared from either side don't leave stale edges behind. An empty
// ConnectorID defaults to connectorID; resolves_to keeps its DNS connector
// owner, while same_as uses the connector at its canonical source endpoint.
func (s *Store) ReplaceTopologyEdgesForConnector(ctx context.Context, connectorID string, edges []TopologyEdge) error {
	return s.ReplaceTopologyEdgesPreserving(ctx, connectorID, edges, nil)
}

// ReplaceTopologyEdgesPreserving is ReplaceTopologyEdgesForConnector for a
// rebuild that could not read the snapshots of preserveOwners. Their
// resolves_to edges pointing at connectorID would have been re-derived from
// those snapshots, so the existing ones are kept rather than deleted.
func (s *Store) ReplaceTopologyEdgesPreserving(ctx context.Context, connectorID string, edges []TopologyEdge, preserveOwners []string) error {
	// Normalize and collapse this rebuild's input before taking the global lock.
	// The lock still protects replacement against another rebuild inserting the
	// same shared edge between the logical-row cleanup and its insert.
	prepared := make([]TopologyEdge, 0, len(edges))
	seen := make(map[string]struct{}, len(edges))
	for _, edge := range edges {
		edge = canonicalTopologyEdge(edge, connectorID)
		key := topologyLogicalEdgeKey(edge)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		prepared = append(prepared, edge)
	}

	topologyRebuildMu.Lock()
	defer topologyRebuildMu.Unlock()
	return s.WithinTransaction(ctx, func(tx *Store) error {
		if s.driver == "postgres" {
			if _, err := tx.db.ExecContext(ctx, `SELECT pg_advisory_xact_lock(`+fmt.Sprint(topologyRebuildAdvisoryLock)+`)`); err != nil {
				return fmt.Errorf("lock topology rebuild: %w", err)
			}
		}
		where := topologyEdgesAffectedBy
		args := []any{connectorID, connectorID, connectorID, connectorID}
		if len(preserveOwners) > 0 {
			where += ` AND connector_id NOT IN (` + placeholders(len(preserveOwners)) + `)`
			for _, id := range preserveOwners {
				args = append(args, id)
			}
		}
		where += `)`
		if _, err := tx.db.ExecContext(ctx, `DELETE FROM topology_edges WHERE `+where, args...); err != nil {
			return fmt.Errorf("delete topology edges: %w", err)
		}
		// Rebuilds can derive a shared edge already stored by another connector,
		// or under several legacy owners. Replace every row with the same
		// endpoint identity before inserting one row for this rebuild. The
		// global in-process and PostgreSQL locks make this delete/insert safe
		// without a schema migration.
		for start := 0; start < len(prepared); start += topologyInsertBatch {
			end := min(start+topologyInsertBatch, len(prepared))
			if err := deleteTopologyLogicalEdges(ctx, tx, prepared[start:end]); err != nil {
				return err
			}
		}
		now := time.Now().UTC().Format(time.RFC3339)
		// Multi-row inserts: a shared IP can mean thousands of rows per rebuild.
		for start := 0; start < len(prepared); start += topologyInsertBatch {
			end := min(start+topologyInsertBatch, len(prepared))
			rows := make([]string, 0, end-start)
			args := make([]any, 0, (end-start)*14)
			for i := start; i < end; i++ {
				e := prepared[i]
				if e.ID == "" {
					e.ID = uuid.New().String()
				}
				if e.CreatedAt == "" {
					e.CreatedAt = now
				}
				if e.ConnectorID == "" {
					e.ConnectorID = connectorID
				}
				rows = append(rows, "("+placeholders(14)+")")
				args = append(args, e.ID, e.ConnectorID, e.SrcConnectorID, e.SrcKind, e.SrcName, e.SrcRef,
					e.DstConnectorID, e.DstKind, e.DstName, e.DstRef, e.Kind, e.Source, e.Detail, e.CreatedAt)
			}
			if _, err := tx.db.ExecContext(ctx, `INSERT INTO topology_edges (`+topologyEdgeColumns+`) VALUES `+strings.Join(rows, ", "), args...); err != nil {
				return fmt.Errorf("insert topology edges: %w", err)
			}
		}
		return nil
	})
}

func canonicalTopologyEdge(edge TopologyEdge, defaultOwner string) TopologyEdge {
	srcKey := topologyEndpointKey(edge.SrcConnectorID, edge.SrcKind, edge.SrcRef)
	dstKey := topologyEndpointKey(edge.DstConnectorID, edge.DstKind, edge.DstRef)
	if edge.Kind == TopologyEdgeSameAs && dstKey < srcKey {
		edge.SrcConnectorID, edge.DstConnectorID = edge.DstConnectorID, edge.SrcConnectorID
		edge.SrcKind, edge.DstKind = edge.DstKind, edge.SrcKind
		edge.SrcName, edge.DstName = edge.DstName, edge.SrcName
		edge.SrcRef, edge.DstRef = edge.DstRef, edge.SrcRef
	}
	if edge.ConnectorID == "" {
		edge.ConnectorID = defaultOwner
	}
	if edge.Kind == TopologyEdgeSameAs {
		edge.ConnectorID = edge.SrcConnectorID
	}
	return edge
}

func topologyEndpointKey(connectorID, kind, ref string) string {
	return strings.Join([]string{connectorID, kind, ref}, "\x00")
}

func topologyLogicalEdgeKey(edge TopologyEdge) string {
	srcRef := edge.SrcRef
	if srcRef == "" {
		srcRef = edge.SrcName
	}
	dstRef := edge.DstRef
	if dstRef == "" {
		dstRef = edge.DstName
	}
	return strings.Join([]string{
		edge.SrcConnectorID, edge.SrcKind, srcRef,
		edge.DstConnectorID, edge.DstKind, dstRef,
		edge.Kind, edge.Source, edge.Detail,
	}, "\x00")
}

func deleteTopologyLogicalEdges(ctx context.Context, tx *Store, edges []TopologyEdge) error {
	if len(edges) == 0 {
		return nil
	}
	columns := "(" + topologyLogicalEdgeColumns + ")"
	rows := make([]string, 0, len(edges))
	args := make([]any, 0, len(edges)*9)
	for _, edge := range edges {
		rows = append(rows, "("+placeholders(9)+")")
		srcRef := edge.SrcRef
		if srcRef == "" {
			srcRef = edge.SrcName
		}
		dstRef := edge.DstRef
		if dstRef == "" {
			dstRef = edge.DstName
		}
		args = append(args, edge.SrcConnectorID, edge.SrcKind, srcRef,
			edge.DstConnectorID, edge.DstKind, dstRef,
			edge.Kind, edge.Source, edge.Detail)
	}
	query := `DELETE FROM topology_edges WHERE ` + columns + ` IN (VALUES ` + strings.Join(rows, ", ") + `)`
	if _, err := tx.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("deduplicate topology edges: %w", err)
	}
	return nil
}

// fingerprintKey identifies an edge by what it says about the lab, ignoring
// row ID, timestamp and owner. A same_as edge is symmetric, so its endpoints
// are put in a fixed order: rebuilding either side re-derives it in its own
// direction, and that must not look like a change.
func fingerprintKey(e TopologyEdge) string {
	src := strings.Join([]string{e.SrcConnectorID, e.SrcKind, e.SrcName, e.SrcRef}, "\x00")
	dst := strings.Join([]string{e.DstConnectorID, e.DstKind, e.DstName, e.DstRef}, "\x00")
	if e.Kind == TopologyEdgeSameAs && dst < src {
		src, dst = dst, src
	}
	return strings.Join([]string{src, dst, e.Kind, e.Source, e.Detail}, "\x01")
}

// TopologyEdgesFingerprint hashes the set of stored edges, independent of row
// IDs, timestamps and which connector's rebuild produced a row. Two calls
// return the same value exactly when the edge set is unchanged. With kinds,
// only edges of those kinds are hashed, so a fingerprint can cover exactly
// what a consumer renders.
func (s *Store) TopologyEdgesFingerprint(ctx context.Context, kinds ...string) (string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+topologyEdgeColumns+` FROM topology_edges`)
	if err != nil {
		return "", fmt.Errorf("list topology edges: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	seen := map[string]struct{}{}
	for rows.Next() {
		var e TopologyEdge
		if err := rows.Scan(&e.ID, &e.ConnectorID, &e.SrcConnectorID, &e.SrcKind, &e.SrcName, &e.SrcRef,
			&e.DstConnectorID, &e.DstKind, &e.DstName, &e.DstRef, &e.Kind, &e.Source, &e.Detail, &e.CreatedAt); err != nil {
			return "", fmt.Errorf("scan topology edge: %w", err)
		}
		if len(kinds) > 0 && !slices.Contains(kinds, e.Kind) {
			continue
		}
		seen[fingerprintKey(e)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate topology edges: %w", err)
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	sum := sha256.Sum256([]byte(strings.Join(keys, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

// ListTopologyEdges returns every edge whose two endpoints both belong to
// one of connectorIDs. Edges touching any other connector are never loaded,
// so callers can pass the caller's allowed set and traverse safely.
func (s *Store) ListTopologyEdges(ctx context.Context, connectorIDs []string) ([]TopologyEdge, error) {
	if len(connectorIDs) == 0 {
		return []TopologyEdge{}, nil
	}
	in := placeholders(len(connectorIDs))
	args := make([]any, 0, 2*len(connectorIDs))
	for range 2 {
		for _, id := range connectorIDs {
			args = append(args, id)
		}
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+topologyEdgeColumns+` FROM topology_edges
		WHERE src_connector_id IN (`+in+`) AND dst_connector_id IN (`+in+`)
		ORDER BY kind, src_name, dst_name, id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list topology edges: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	edges := []TopologyEdge{}
	for rows.Next() {
		var e TopologyEdge
		if err := rows.Scan(&e.ID, &e.ConnectorID, &e.SrcConnectorID, &e.SrcKind, &e.SrcName, &e.SrcRef,
			&e.DstConnectorID, &e.DstKind, &e.DstName, &e.DstRef, &e.Kind, &e.Source, &e.Detail, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan topology edge: %w", err)
		}
		edges = append(edges, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate topology edges: %w", err)
	}
	return edges, nil
}

// ListConnectorIDsMissingTopology returns connectors that have at least one
// snapshot but no topology edges: the ones that synced before edge building
// existed (or whose rebuild failed) and so need a backfill.
func (s *Store) ListConnectorIDsMissingTopology(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id FROM connectors c
		WHERE EXISTS (SELECT 1 FROM service_snapshots sn WHERE sn.connector_id = c.id)
		  AND NOT EXISTS (SELECT 1 FROM topology_edges t WHERE t.connector_id = c.id)
		ORDER BY c.id
	`)
	if err != nil {
		return nil, fmt.Errorf("list connectors missing topology: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan connector id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate connectors missing topology: %w", err)
	}
	return ids, nil
}
