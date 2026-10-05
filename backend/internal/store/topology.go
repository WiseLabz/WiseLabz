package store

import (
	"context"
	"fmt"
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
	TopologyEdgeContains   = "contains"
	TopologyEdgeResolvesTo = "resolves_to"
	TopologyEdgeProxiesTo  = "proxies_to"
	TopologyEdgeRunsOn     = "runs_on"
)

// TopologyEdge is one persisted edge of the entity-level topology graph.
// Both endpoints are identified by their owning connector plus kind and
// name (Ref is the connector-local external ID when known). The graph is
// treated as undirected by path queries; Source records how the edge was
// derived so heuristic matches stay explainable.
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

// ReplaceTopologyEdgesForConnector atomically rebuilds the edges attributable
// to connectorID: everything it produced earlier plus any same_as edge that
// touches it (those are re-derived symmetrically from its latest snapshot, so
// entities that disappeared from either side don't leave stale edges behind).
func (s *Store) ReplaceTopologyEdgesForConnector(ctx context.Context, connectorID string, edges []TopologyEdge) error {
	_, err := s.ReplaceTopologyEdgesForConnectorChanged(ctx, connectorID, edges)
	return err
}

// ReplaceTopologyEdgesForConnectorChanged atomically replaces edges and reports
// whether their observable fields changed.
func (s *Store) ReplaceTopologyEdgesForConnectorChanged(ctx context.Context, connectorID string, edges []TopologyEdge) (bool, error) {
	changed := false
	err := s.WithinTransaction(ctx, func(tx *Store) error {
		old, err := tx.listTopologyEdgesAffectedByConnector(ctx, connectorID)
		if err != nil {
			return err
		}
		changed = !sameTopologyEdges(old, edges)
		if _, err := tx.db.ExecContext(ctx, `
			DELETE FROM topology_edges
			WHERE connector_id = ?
			   OR (kind = ? AND (src_connector_id = ? OR dst_connector_id = ?))
		`, connectorID, TopologyEdgeSameAs, connectorID, connectorID); err != nil {
			return fmt.Errorf("delete topology edges: %w", err)
		}
		now := time.Now().UTC().Format(time.RFC3339)
		for i := range edges {
			e := edges[i]
			if e.ID == "" {
				e.ID = uuid.New().String()
			}
			if e.CreatedAt == "" {
				e.CreatedAt = now
			}
			e.ConnectorID = connectorID
			if _, err := tx.db.ExecContext(ctx, `
				INSERT INTO topology_edges (`+topologyEdgeColumns+`)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, e.ID, e.ConnectorID, e.SrcConnectorID, e.SrcKind, e.SrcName, e.SrcRef,
				e.DstConnectorID, e.DstKind, e.DstName, e.DstRef, e.Kind, e.Source, e.Detail, e.CreatedAt); err != nil {
				return fmt.Errorf("insert topology edge: %w", err)
			}
		}
		return nil
	})
	return changed, err
}

func sameTopologyEdges(old, next []TopologyEdge) bool {
	key := func(e TopologyEdge) string {
		return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s",
			e.SrcConnectorID, e.SrcKind, e.SrcName, e.SrcRef, e.DstConnectorID,
			e.DstKind, e.DstName, e.DstRef, e.Kind, e.Source+"\x00"+e.Detail)
	}
	if len(old) != len(next) {
		return false
	}
	counts := make(map[string]int, len(old))
	for _, e := range old {
		counts[key(e)]++
	}
	for _, e := range next {
		k := key(e)
		if counts[k] == 0 {
			return false
		}
		counts[k]--
	}
	return true
}

func (s *Store) listTopologyEdgesAffectedByConnector(ctx context.Context, connectorID string) ([]TopologyEdge, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+topologyEdgeColumns+` FROM topology_edges WHERE connector_id = ? OR (kind = ? AND (src_connector_id = ? OR dst_connector_id = ?)) ORDER BY id`, connectorID, TopologyEdgeSameAs, connectorID, connectorID)
	if err != nil {
		return nil, fmt.Errorf("list connector topology edges: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	var edges []TopologyEdge
	for rows.Next() {
		var e TopologyEdge
		if err := rows.Scan(&e.ID, &e.ConnectorID, &e.SrcConnectorID, &e.SrcKind, &e.SrcName, &e.SrcRef,
			&e.DstConnectorID, &e.DstKind, &e.DstName, &e.DstRef, &e.Kind, &e.Source, &e.Detail, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan connector topology edge: %w", err)
		}
		edges = append(edges, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate connector topology edges: %w", err)
	}
	return edges, nil
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
