// Package entities serves connector-scoped entity identity details.
package entities

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/sync"
)

// Handler serves entity identity details.
type Handler struct{ Store *store.Store }

// NewHandler creates an entity detail handler.
func NewHandler(s *store.Store) *Handler { return &Handler{Store: s} }

type member struct {
	ConnectorID string `json:"connectorId"`
	Kind        string `json:"kind"`
	Ref         string `json:"ref"`
	Name        string `json:"name"`
	GoneAt      string `json:"goneAt,omitempty"`
}

type endpoint struct {
	ConnectorID string `json:"connectorId"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Ref         string `json:"ref"`
}

type link struct {
	From   endpoint `json:"from"`
	To     endpoint `json:"to"`
	Reason string   `json:"reason"`
}

type neighbor struct {
	Kind string   `json:"kind"`
	From endpoint `json:"from"`
	To   endpoint `json:"to"`
}

type change struct {
	ConnectorID string `json:"connectorId"`
	At          string `json:"at"`
	sync.EntityChange
}

type runbook struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
	Step  struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Verb  string `json:"verb"`
	} `json:"step"`
}

type detail struct {
	ID                string                       `json:"id"`
	Kind              string                       `json:"kind"`
	Name              string                       `json:"name"`
	Gone              bool                         `json:"gone"`
	Members           []member                     `json:"members"`
	RelatedByIP       []link                       `json:"relatedByIp"`
	Neighbors         []neighbor                   `json:"neighbors"`
	History           []change                     `json:"history"`
	Findings          []store.QualityFindingRecord `json:"findings"`
	ConnectorFindings []store.QualityFindingRecord `json:"onReportingConnectors"`
	Runbooks          []runbook                    `json:"runbooks"`
}

type identityRow struct {
	id, kind, name string
	mergedInto     sql.NullString
}

// Get handles GET /api/entities/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	entity, err := h.identity(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		httpError(w, err)
		return
	}
	connectors, err := h.memberConnectorIDs(ctx, entity.id)
	if err != nil {
		httpError(w, err)
		return
	}
	allowed, err := h.Store.FilterConnectorIDsByGrant(ctx, auth.UserIDFromContext(ctx), connectors, "viewer")
	if err != nil {
		httpError(w, err)
		return
	}
	if len(allowed) == 0 {
		notFound(w)
		return
	}
	out, err := h.detail(ctx, entity.id, allowed)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			notFound(w)
			return
		}
		httpError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, out)
}

func (h *Handler) identity(ctx context.Context, id string) (identityRow, error) {
	seen := map[string]bool{}
	for id != "" && !seen[id] {
		seen[id] = true
		var e identityRow
		err := h.Store.DB().QueryRowContext(ctx, `SELECT id, kind, display_name, merged_into FROM entities WHERE id = ?`, id).
			Scan(&e.id, &e.kind, &e.name, &e.mergedInto)
		if errors.Is(err, sql.ErrNoRows) {
			return identityRow{}, store.ErrNotFound
		}
		if err != nil {
			return identityRow{}, fmt.Errorf("get entity identity: %w", err)
		}
		if !e.mergedInto.Valid {
			return e, nil
		}
		id = e.mergedInto.String
	}
	return identityRow{}, store.ErrNotFound
}

func (h *Handler) memberConnectorIDs(ctx context.Context, id string) ([]string, error) {
	rows, err := h.Store.DB().QueryContext(ctx, `SELECT DISTINCT connector_id FROM entity_members WHERE entity_id = ? ORDER BY connector_id`, id)
	if err != nil {
		return nil, fmt.Errorf("list entity connectors: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (h *Handler) detail(ctx context.Context, id string, allowed []string) (*detail, error) {
	allowedSet := make(map[string]bool, len(allowed))
	for _, connectorID := range allowed {
		allowedSet[connectorID] = true
	}
	query, args := `SELECT connector_id, kind, ref, name, gone_at FROM entity_members WHERE entity_id = ? AND connector_id IN (`, []any{id}
	query += placeholders(len(allowed)) + `) ORDER BY name, connector_id, kind, ref`
	for _, cid := range allowed {
		args = append(args, cid)
	}
	rows, err := h.Store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list visible entity members: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	out := &detail{ID: id, Members: []member{}, RelatedByIP: []link{}, Neighbors: []neighbor{}, History: []change{}, Findings: []store.QualityFindingRecord{}, ConnectorFindings: []store.QualityFindingRecord{}, Runbooks: []runbook{}}
	members := []member{}
	for rows.Next() {
		var m member
		var gone sql.NullString
		if err := rows.Scan(&m.ConnectorID, &m.Kind, &m.Ref, &m.Name, &gone); err != nil {
			return nil, err
		}
		m.GoneAt = gone.String
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, store.ErrNotFound
	}
	out.Members = members
	out.Kind, out.Name = members[0].Kind, members[0].Name
	out.Gone = true
	for _, m := range members {
		if m.GoneAt == "" {
			out.Gone = false
		}
	}
	if err := h.edges(ctx, members, allowed, allowedSet, out); err != nil {
		return nil, err
	}
	if err := h.history(ctx, members, out); err != nil {
		return nil, err
	}
	if err := h.findings(ctx, members, allowed, out); err != nil {
		return nil, err
	}
	if err := h.runbooks(ctx, members, allowed, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (h *Handler) edges(ctx context.Context, members []member, allowed []string, allowedSet map[string]bool, out *detail) error {
	query, args := `SELECT src_connector_id, src_kind, src_name, src_ref, dst_connector_id, dst_kind, dst_name, dst_ref, kind, source FROM topology_edges WHERE src_connector_id IN (`, []any{}
	query += placeholders(len(allowed)) + `) AND dst_connector_id IN (` + placeholders(len(allowed)) + `)`
	for _, cid := range allowed {
		args = append(args, cid)
	}
	for _, cid := range allowed {
		args = append(args, cid)
	}
	rows, err := h.Store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("list entity topology edges: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	memberKeys := map[string]bool{}
	for _, m := range members {
		memberKeys[endpointKey(m.ConnectorID, m.Kind, m.Ref)] = true
	}
	for rows.Next() {
		var src, dst endpoint
		var kind, source string
		if err := rows.Scan(&src.ConnectorID, &src.Kind, &src.Name, &src.Ref, &dst.ConnectorID, &dst.Kind, &dst.Name, &dst.Ref, &kind, &source); err != nil {
			return err
		}
		if !allowedSet[src.ConnectorID] || !allowedSet[dst.ConnectorID] {
			continue
		}
		fromMember, toMember := memberKeys[endpointKey(src.ConnectorID, src.Kind, src.Ref)], memberKeys[endpointKey(dst.ConnectorID, dst.Kind, dst.Ref)]
		if !fromMember && !toMember {
			continue
		}
		if kind == store.TopologyEdgeSameAs && source == "IP address" {
			out.RelatedByIP = append(out.RelatedByIP, link{From: src, To: dst, Reason: source})
		} else {
			out.Neighbors = append(out.Neighbors, neighbor{Kind: kind, From: src, To: dst})
		}
	}
	return rows.Err()
}

func (h *Handler) history(ctx context.Context, members []member, out *detail) error {
	byConnector := map[string][]member{}
	for _, m := range members {
		byConnector[m.ConnectorID] = append(byConnector[m.ConnectorID], m)
	}
	for cid, connectorMembers := range byConnector {
		snapshots, err := h.snapshotHistory(ctx, cid)
		if err != nil {
			return fmt.Errorf("get entity snapshots: %w", err)
		}
		for i := len(snapshots) - 1; i > 0; i-- {
			var previous, current connector.ServiceSnapshot
			if json.Unmarshal([]byte(snapshots[i].Data), &previous) != nil || json.Unmarshal([]byte(snapshots[i-1].Data), &current) != nil {
				continue
			}
			diff := sync.BuildSnapshotDiff(&previous, &current)
			for _, m := range connectorMembers {
				key := memberSnapshotKey(m, previous.Entities, current.Entities)
				for _, item := range diff.Entities {
					if item.Kind == m.Kind && item.Key == key {
						out.History = append(out.History, change{ConnectorID: cid, At: snapshots[i-1].FetchedAt, EntityChange: item})
					}
				}
			}
		}
	}
	sort.Slice(out.History, func(i, j int) bool { return out.History[i].At > out.History[j].At })
	return nil
}

func (h *Handler) snapshotHistory(ctx context.Context, connectorID string) ([]store.SnapshotRecord, error) {
	rows, err := h.Store.DB().QueryContext(ctx, `SELECT id, connector_id, data, fetched_at FROM service_snapshots WHERE connector_id = ? ORDER BY fetched_at, id`, connectorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	snapshots := []store.SnapshotRecord{}
	for rows.Next() {
		var snapshot store.SnapshotRecord
		if err := rows.Scan(&snapshot.ID, &snapshot.ConnectorID, &snapshot.Data, &snapshot.FetchedAt); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, rows.Err()
}

func memberSnapshotKey(m member, snapshots ...[]connector.SnapshotEntity) string {
	for _, entities := range snapshots {
		for _, e := range entities {
			if e.Kind == m.Kind && entityRef(e) == m.Ref {
				return snapshotEntityKey(e)
			}
		}
	}
	return "name:" + m.Ref
}

func (h *Handler) findings(ctx context.Context, members []member, allowed []string, out *detail) error {
	memberKeys := map[string]bool{}
	for _, m := range members {
		memberKeys[m.ConnectorID+"\x00"+m.Kind+"\x00"+m.Ref] = true
	}
	query, args := `SELECT `+findingColumns+` FROM quality_findings WHERE connector_id IN (`, []any{}
	query += placeholders(len(allowed)) + `) ORDER BY last_seen_at DESC`
	for _, cid := range allowed {
		args = append(args, cid)
	}
	rows, err := h.Store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("list entity findings: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	for rows.Next() {
		f, err := scanFinding(rows)
		if err != nil {
			return err
		}
		if f.EntityKind != "" && memberKeys[f.ConnectorID+"\x00"+f.EntityKind+"\x00"+f.EntityRef] {
			out.Findings = append(out.Findings, f)
		}
		if f.EntityKind == "" && f.EntityRef == "" {
			out.ConnectorFindings = append(out.ConnectorFindings, f)
		}
	}
	return rows.Err()
}

func (h *Handler) runbooks(ctx context.Context, members []member, allowed []string, out *detail) error {
	query, args := `SELECT DISTINCT r.id, r.title, r.body, s.id, s.title, s.verb FROM runbooks r JOIN runbook_steps s ON s.runbook_id = r.id WHERE s.connector_id IN (`, []any{}
	query += placeholders(len(allowed)) + `) AND (`
	for i, m := range members {
		if i > 0 {
			query += ` OR `
		}
		query += `(s.connector_id = ? AND s.entity_ref = ?)`
		args = append(args, m.ConnectorID, m.Ref)
	}
	query += `) ORDER BY r.title, s.position`
	args = append(allowedArgs(allowed), args...)
	rows, err := h.Store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("list entity runbooks: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	for rows.Next() {
		var r runbook
		if err := rows.Scan(&r.ID, &r.Title, &r.Body, &r.Step.ID, &r.Step.Title, &r.Step.Verb); err != nil {
			return err
		}
		out.Runbooks = append(out.Runbooks, r)
	}
	return rows.Err()
}

const findingColumns = `id, connector_id, COALESCE(doc_id,''), COALESCE(rule_id,''), check_type, severity, title, description, remediation_link, status, detected_count, first_detected_at, last_seen_at, COALESCE(resolved_at,''), COALESCE(notified_severity,''), COALESCE(entity_kind,''), COALESCE(entity_ref,'')`

func scanFinding(row interface{ Scan(...any) error }) (store.QualityFindingRecord, error) {
	var f store.QualityFindingRecord
	err := row.Scan(&f.ID, &f.ConnectorID, &f.DocID, &f.RuleID, &f.CheckType, &f.Severity, &f.Title, &f.Description, &f.RemediationLink, &f.Status, &f.DetectedCount, &f.FirstDetectedAt, &f.LastSeenAt, &f.ResolvedAt, &f.NotifiedSeverity, &f.EntityKind, &f.EntityRef)
	return f, err
}

func endpointKey(connectorID, kind, ref string) string {
	return connectorID + "\x00" + kind + "\x00" + ref
}
func entityRef(e connector.SnapshotEntity) string {
	if e.ExternalID != "" {
		return e.ExternalID
	}
	return e.Name
}
func snapshotEntityKey(e connector.SnapshotEntity) string {
	if e.ExternalID != "" {
		return "externalId:" + e.ExternalID
	}
	return "name:" + e.Name
}
func placeholders(n int) string {
	out := "?"
	for i := 1; i < n; i++ {
		out += ",?"
	}
	return out
}
func allowedArgs(ids []string) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}
func notFound(w http.ResponseWriter) {
	httputil.Error(w, http.StatusNotFound, "not_found", "Entity not found")
}
func httpError(w http.ResponseWriter, err error) { httputil.Errorf(w, err) }
