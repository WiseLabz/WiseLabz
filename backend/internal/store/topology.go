package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
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

// topologyEdgesAffectedBy selects the edges a rebuild of one connector
// replaces (argument: the connector ID, four times): everything it owns, any
// same_as edge touching it (re-derived symmetrically from its latest
// snapshot), and any resolves_to edge pointing at one of its entities, which
// belongs to the DNS connector but is re-derived by this rebuild so it cannot
// outlive the entity it points at.
const topologyEdgesAffectedBy = `connector_id = ?
	OR (kind = '` + TopologyEdgeSameAs + `' AND (src_connector_id = ? OR dst_connector_id = ?))
	OR (kind = '` + TopologyEdgeResolvesTo + `' AND dst_connector_id = ?)`

// ReplaceTopologyEdgesForConnector atomically rebuilds the edges affected by
// connectorID's rebuild (see topologyEdgesAffectedBy), so entities that
// disappeared from either side don't leave stale edges behind. An edge with
// an empty ConnectorID is owned by connectorID; a non-empty one keeps its
// owner (a resolves_to edge owned by the DNS connector).
func (s *Store) ReplaceTopologyEdgesForConnector(ctx context.Context, connectorID string, edges []TopologyEdge) error {
	return s.WithinTransaction(ctx, func(tx *Store) error {
		if _, err := tx.db.ExecContext(ctx, `DELETE FROM topology_edges WHERE `+topologyEdgesAffectedBy,
			connectorID, connectorID, connectorID, connectorID); err != nil {
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
			if e.ConnectorID == "" {
				e.ConnectorID = connectorID
			}
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
// return the same value exactly when the edge set is unchanged.
func (s *Store) TopologyEdgesFingerprint(ctx context.Context) (string, error) {
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
