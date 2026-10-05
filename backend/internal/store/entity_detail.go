package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// EntityIdentity is the resolved head of an entity's merged_into chain.
type EntityIdentity struct {
	ID   string
	Kind string
	Name string
}

// EntityMemberKey identifies one connector-local observation of an entity.
type EntityMemberKey struct {
	ConnectorID string
	Kind        string
	Ref         string
}

// EntityMemberDetail is a member row joined with its connector's name.
type EntityMemberDetail struct {
	EntityMemberKey
	ConnectorName string
	// DocID is the connector's generated service doc, or "" when none exists.
	DocID  string
	Name   string
	GoneAt string
}

// EntityEdgeEndpoint is one side of a topology edge. EntityID is the active
// entity the endpoint belongs to, or "" when it has no active membership.
type EntityEdgeEndpoint struct {
	ConnectorID string
	Kind        string
	Name        string
	Ref         string
	EntityID    string
}

// EntityEdge is a topology edge touching an entity's members.
type EntityEdge struct {
	Src    EntityEdgeEndpoint
	Dst    EntityEdgeEndpoint
	Kind   string
	Source string
	Detail string
}

// EntityRunbookStep is a runbook step that targets an entity member.
type EntityRunbookStep struct {
	RunbookID    string
	RunbookTitle string
	RunbookBody  string
	StepID       string
	StepTitle    string
	StepVerb     string
}

// EntityLabel is a member's claim to name its entity: the fields that decide
// which member supplies an identity's name and kind.
type EntityLabel struct {
	ConnectorID, Kind, Ref, Name string
}

// Text is the member's display name, or its ref when it has no name.
func (l EntityLabel) Text() string {
	if l.Name != "" {
		return l.Name
	}
	return l.Ref
}

// IsServicePlaceholder reports whether the member is a connector's own service
// placeholder (kind service, ref = connector ID), which never names an entity
// that has a real member.
func (l EntityLabel) IsServicePlaceholder() bool {
	return l.Kind == "service" && l.Ref == l.ConnectorID
}

// Better reports whether l should name the entity instead of o. Priority:
// a member with a non-empty name, then the lowest display text, connector ID,
// kind and ref. Callers pass only active, visible members, so the choice never
// depends on row order or on a member the caller may not see.
func (l EntityLabel) Better(o EntityLabel) bool {
	if (l.Name == "") != (o.Name == "") {
		return l.Name != ""
	}
	if a, b := l.Text(), o.Text(); a != b {
		return a < b
	}
	if l.ConnectorID != o.ConnectorID {
		return l.ConnectorID < o.ConnectorID
	}
	if l.Kind != o.Kind {
		return l.Kind < o.Kind
	}
	return l.Ref < o.Ref
}

// ResolveEntityIdentity follows the merged_into chain from id to the surviving
// identity. A missing id, a dangling redirect and a redirect loop all return
// ErrNotFound.
func (s *Store) ResolveEntityIdentity(ctx context.Context, id string) (EntityIdentity, error) {
	seen := map[string]bool{}
	for id != "" && !seen[id] {
		seen[id] = true
		var e EntityIdentity
		var mergedInto sql.NullString
		err := s.db.QueryRowContext(ctx, `SELECT id, kind, display_name, merged_into FROM entities WHERE id = ?`, id).
			Scan(&e.ID, &e.Kind, &e.Name, &mergedInto)
		if errors.Is(err, sql.ErrNoRows) {
			return EntityIdentity{}, ErrNotFound
		}
		if err != nil {
			return EntityIdentity{}, fmt.Errorf("get entity identity: %w", err)
		}
		if !mergedInto.Valid {
			return e, nil
		}
		id = mergedInto.String
	}
	return EntityIdentity{}, ErrNotFound
}

// ListEntityMemberConnectorIDs returns every connector with a member of the entity.
func (s *Store) ListEntityMemberConnectorIDs(ctx context.Context, entityID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT connector_id FROM entity_members WHERE entity_id = ? ORDER BY connector_id`, entityID)
	if err != nil {
		return nil, fmt.Errorf("list entity connectors: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan entity connector: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListEntityMembers returns the entity's members on connectorIDs, active
// members first, each with its connector's name.
func (s *Store) ListEntityMembers(ctx context.Context, entityID string, connectorIDs []string) ([]EntityMemberDetail, error) {
	if len(connectorIDs) == 0 {
		return []EntityMemberDetail{}, nil
	}
	query := `SELECT m.connector_id, c.name,
			COALESCE((SELECT d.id FROM docs d WHERE d.service_id = m.connector_id AND d.deleted_at IS NULL AND d.kind = 'service' ORDER BY d.id LIMIT 1), ''),
			m.kind, m.ref, m.name, m.gone_at
		FROM entity_members m JOIN connectors c ON c.id = m.connector_id
		WHERE m.entity_id = ? AND m.connector_id IN (` + inPlaceholders(len(connectorIDs)) + `)
		ORDER BY CASE WHEN m.gone_at IS NULL THEN 0 ELSE 1 END, m.name, m.connector_id, m.kind, m.ref`
	args := append([]any{entityID}, stringArgs(connectorIDs)...)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list entity members: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	members := []EntityMemberDetail{}
	for rows.Next() {
		var m EntityMemberDetail
		var gone sql.NullString
		if err := rows.Scan(&m.ConnectorID, &m.ConnectorName, &m.DocID, &m.Kind, &m.Ref, &m.Name, &gone); err != nil {
			return nil, fmt.Errorf("scan entity member: %w", err)
		}
		m.GoneAt = gone.String
		members = append(members, m)
	}
	return members, rows.Err()
}

// ListEntityEdges returns up to limit topology edges that touch one of members
// and whose both endpoints lie on viewable connectors. members are the near
// ends (active, on connectors the caller may view); the far end may be on any
// viewable connector, not only a member connector of the entity.
func (s *Store) ListEntityEdges(ctx context.Context, members []EntityMemberKey, viewable []string, limit int) ([]EntityEdge, error) {
	allowed := viewable
	if len(members) == 0 || len(allowed) == 0 {
		return []EntityEdge{}, nil
	}
	srcPred, srcArgs := memberPredicate("e.src_connector_id", "e.src_kind", "e.src_ref", members)
	dstPred, dstArgs := memberPredicate("e.dst_connector_id", "e.dst_kind", "e.dst_ref", members)
	query := `SELECT e.src_connector_id, e.src_kind, e.src_name, e.src_ref, COALESCE(sm.entity_id, ''),
			e.dst_connector_id, e.dst_kind, e.dst_name, e.dst_ref, COALESCE(dm.entity_id, ''), e.kind, e.source, e.detail
		FROM topology_edges e
		LEFT JOIN entity_members sm ON sm.connector_id = e.src_connector_id AND sm.kind = e.src_kind AND sm.ref = e.src_ref AND sm.gone_at IS NULL
			AND sm.entity_id IN (SELECT id FROM entities WHERE merged_into IS NULL)
		LEFT JOIN entity_members dm ON dm.connector_id = e.dst_connector_id AND dm.kind = e.dst_kind AND dm.ref = e.dst_ref AND dm.gone_at IS NULL
			AND dm.entity_id IN (SELECT id FROM entities WHERE merged_into IS NULL)
		WHERE e.src_connector_id IN (` + inPlaceholders(len(allowed)) + `) AND e.dst_connector_id IN (` + inPlaceholders(len(allowed)) + `)
		AND (` + srcPred + ` OR ` + dstPred + `)
		ORDER BY e.kind, e.src_name, e.dst_name, e.src_connector_id, e.src_kind, e.src_ref, e.dst_connector_id, e.dst_kind, e.dst_ref, e.source, e.detail, e.id LIMIT ?`
	args := append(stringArgs(allowed), stringArgs(allowed)...)
	args = append(args, srcArgs...)
	args = append(args, dstArgs...)
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list entity topology edges: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	edges := []EntityEdge{}
	for rows.Next() {
		var e EntityEdge
		if err := rows.Scan(&e.Src.ConnectorID, &e.Src.Kind, &e.Src.Name, &e.Src.Ref, &e.Src.EntityID,
			&e.Dst.ConnectorID, &e.Dst.Kind, &e.Dst.Name, &e.Dst.Ref, &e.Dst.EntityID, &e.Kind, &e.Source, &e.Detail); err != nil {
			return nil, fmt.Errorf("scan entity topology edge: %w", err)
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// ListOpenEntityFindings returns up to limit open findings recorded against
// exactly one of members, newest first.
func (s *Store) ListOpenEntityFindings(ctx context.Context, members []EntityMemberKey, limit int) ([]QualityFindingRecord, error) {
	if len(members) == 0 {
		return []QualityFindingRecord{}, nil
	}
	pred, args := memberPredicate("connector_id", "entity_kind", "entity_ref", members)
	return s.queryFindings(ctx, `status = 'open' AND (`+pred+`)`, args, limit)
}

// ListOpenConnectorLevelFindings returns up to limit open findings of
// connectorIDs that are not tied to a specific entity, newest first.
func (s *Store) ListOpenConnectorLevelFindings(ctx context.Context, connectorIDs []string, limit int) ([]QualityFindingRecord, error) {
	if len(connectorIDs) == 0 {
		return []QualityFindingRecord{}, nil
	}
	return s.queryFindings(ctx, `status = 'open' AND connector_id IN (`+inPlaceholders(len(connectorIDs))+`)
		AND COALESCE(entity_kind, '') = '' AND COALESCE(entity_ref, '') = ''`, stringArgs(connectorIDs), limit)
}

func (s *Store) queryFindings(ctx context.Context, where string, args []any, limit int) ([]QualityFindingRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+qualityFindingColumns+` FROM quality_findings WHERE `+where+` ORDER BY last_seen_at DESC, id LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, fmt.Errorf("list entity findings: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	findings := []QualityFindingRecord{}
	for rows.Next() {
		f, err := scanQualityFinding(rows)
		if err != nil {
			return nil, fmt.Errorf("scan entity finding: %w", err)
		}
		findings = append(findings, f)
	}
	return findings, rows.Err()
}

// ListEntityRunbookSteps returns up to limit runbook steps that target one of
// members (connector and entity ref), ordered by runbook title then position.
func (s *Store) ListEntityRunbookSteps(ctx context.Context, members []EntityMemberKey, limit int) ([]EntityRunbookStep, error) {
	if len(members) == 0 {
		return []EntityRunbookStep{}, nil
	}
	var preds []string
	var args []any
	for _, m := range members {
		preds = append(preds, `(s.connector_id = ? AND s.entity_ref = ?)`)
		args = append(args, m.ConnectorID, m.Ref)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT r.id, r.title, r.body, s.id, s.title, s.verb
		FROM runbooks r JOIN runbook_steps s ON s.runbook_id = r.id
		WHERE `+strings.Join(preds, ` OR `)+`
		ORDER BY r.title, r.id, s.position, s.id LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, fmt.Errorf("list entity runbook steps: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	steps := []EntityRunbookStep{}
	for rows.Next() {
		var st EntityRunbookStep
		if err := rows.Scan(&st.RunbookID, &st.RunbookTitle, &st.RunbookBody, &st.StepID, &st.StepTitle, &st.StepVerb); err != nil {
			return nil, fmt.Errorf("scan entity runbook step: %w", err)
		}
		steps = append(steps, st)
	}
	return steps, rows.Err()
}

// memberPredicate builds `(c = ? AND k = ? AND r = ?) OR ...` for members.
func memberPredicate(connectorCol, kindCol, refCol string, members []EntityMemberKey) (string, []any) {
	parts := make([]string, 0, len(members))
	args := make([]any, 0, 3*len(members))
	for _, m := range members {
		parts = append(parts, `(`+connectorCol+` = ? AND `+kindCol+` = ? AND `+refCol+` = ?)`)
		args = append(args, m.ConnectorID, m.Kind, m.Ref)
	}
	return strings.Join(parts, ` OR `), args
}

func inPlaceholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func stringArgs(ids []string) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}
