// Package entities serves connector-scoped entity identity details.
package entities

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/google/uuid"

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

// Bounds on what one detail request reads and returns.
const (
	maxEdges            = 200
	maxFindings         = 50
	maxRunbookSteps     = 100
	historySnapshots    = 30
	maxHistoryRows      = 100
	canonicalUUIDLength = 36
)

type member struct {
	ConnectorID   string `json:"connectorId"`
	ConnectorName string `json:"connectorName"`
	DocID         string `json:"docId,omitempty"`
	Kind          string `json:"kind"`
	Ref           string `json:"ref"`
	Name          string `json:"name"`
	GoneAt        string `json:"goneAt,omitempty"`
}

type endpoint struct {
	ConnectorID string `json:"connectorId"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Ref         string `json:"ref"`
	// EntityID is set when the endpoint belongs to another entity the caller
	// can open (it has an active member on a connector the caller may view).
	EntityID string `json:"entityId,omitempty"`
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
	// Detail is the edge's context (e.g. the proxy upstream), omitted when empty.
	Detail string `json:"detail,omitempty"`
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

// Get handles GET /api/entities/{id}. Every reason the caller may not see the
// entity (malformed, unknown, redirect loop, no grant) returns the same 404.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !validID(r.PathValue("id")) {
		notFound(w)
		return
	}
	entity, err := h.Store.ResolveEntityIdentity(ctx, r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		httpError(w, err)
		return
	}
	connectors, err := h.Store.ListEntityMemberConnectorIDs(ctx, entity.ID)
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
	out, err := h.detail(ctx, entity.ID, allowed)
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

func validID(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil && len(id) == canonicalUUIDLength
}

func (h *Handler) detail(ctx context.Context, id string, allowed []string) (*detail, error) {
	visible, err := h.Store.ListEntityMembers(ctx, id, allowed)
	if err != nil {
		return nil, err
	}
	if len(visible) == 0 {
		return nil, store.ErrNotFound
	}
	out := &detail{ID: id, Members: []member{}, RelatedByIP: []link{}, Neighbors: []neighbor{}, History: []change{}, Findings: []store.QualityFindingRecord{}, ConnectorFindings: []store.QualityFindingRecord{}, Runbooks: []runbook{}}
	// Members arrive active first, so the first row names the entity unless
	// every visible member is gone.
	out.Kind, out.Name, out.Gone = visible[0].Kind, visible[0].Name, visible[0].GoneAt != ""
	var active []store.EntityMemberKey
	activeConnectors := map[string]bool{}
	for _, m := range visible {
		out.Members = append(out.Members, member{ConnectorID: m.ConnectorID, ConnectorName: m.ConnectorName, DocID: m.DocID, Kind: m.Kind, Ref: m.Ref, Name: m.Name, GoneAt: m.GoneAt})
		if m.GoneAt == "" {
			active = append(active, m.EntityMemberKey)
			activeConnectors[m.ConnectorID] = true
		}
	}
	if err := h.edges(ctx, id, active, allowed, out); err != nil {
		return nil, err
	}
	if err := h.history(ctx, visible, out); err != nil {
		return nil, err
	}
	if err := h.findings(ctx, active, activeConnectors, out); err != nil {
		return nil, err
	}
	return out, h.runbooks(ctx, active, out)
}

func (h *Handler) edges(ctx context.Context, id string, active []store.EntityMemberKey, allowed []string, out *detail) error {
	edges, err := h.Store.ListEntityEdges(ctx, active, allowed, maxEdges)
	if err != nil {
		return err
	}
	for _, e := range edges {
		src, dst := toEndpoint(e.Src, id), toEndpoint(e.Dst, id)
		if e.Kind == store.TopologyEdgeSameAs && e.Source == "IP address" {
			out.RelatedByIP = append(out.RelatedByIP, link{From: src, To: dst, Reason: e.Source})
		} else {
			out.Neighbors = append(out.Neighbors, neighbor{Kind: e.Kind, From: src, To: dst, Detail: e.Detail})
		}
	}
	return nil
}

func toEndpoint(e store.EntityEdgeEndpoint, self string) endpoint {
	out := endpoint{ConnectorID: e.ConnectorID, Kind: e.Kind, Name: e.Name, Ref: e.Ref}
	if e.EntityID != self {
		out.EntityID = e.EntityID
	}
	return out
}

// history derives attribute changes from the most recent snapshots of each
// member connector, decoding each snapshot once and diffing adjacent pairs.
func (h *Handler) history(ctx context.Context, members []store.EntityMemberDetail, out *detail) error {
	byConnector := map[string][]store.EntityMemberDetail{}
	var order []string
	for _, m := range members {
		if _, ok := byConnector[m.ConnectorID]; !ok {
			order = append(order, m.ConnectorID)
		}
		byConnector[m.ConnectorID] = append(byConnector[m.ConnectorID], m)
	}
	for _, cid := range order {
		newestFirst, err := h.Store.GetSnapshotsByConnector(ctx, cid, historySnapshots)
		if err != nil {
			return fmt.Errorf("get entity snapshots: %w", err)
		}
		decoded := make([]*connector.ServiceSnapshot, len(newestFirst))
		for i := range newestFirst {
			var snap connector.ServiceSnapshot
			if json.Unmarshal([]byte(newestFirst[len(newestFirst)-1-i].Data), &snap) == nil {
				decoded[i] = &snap
			}
		}
		for i := 1; i < len(decoded); i++ {
			previous, current := decoded[i-1], decoded[i]
			if previous == nil || current == nil {
				continue
			}
			at := newestFirst[len(newestFirst)-1-i].FetchedAt
			changes := sync.CompareEntities(previous.Entities, current.Entities)
			for _, m := range byConnector[cid] {
				key := memberSnapshotKey(m, previous.Entities, current.Entities)
				for _, item := range changes {
					if item.Kind == m.Kind && item.Key == key {
						out.History = append(out.History, change{ConnectorID: cid, At: at, EntityChange: item})
					}
				}
			}
		}
	}
	sort.SliceStable(out.History, func(i, j int) bool { return out.History[i].At > out.History[j].At })
	if len(out.History) > maxHistoryRows {
		out.History = out.History[:maxHistoryRows]
	}
	return nil
}

func memberSnapshotKey(m store.EntityMemberDetail, snapshots ...[]connector.SnapshotEntity) string {
	for _, entities := range snapshots {
		for _, e := range entities {
			if e.Kind == m.Kind && entityRef(e) == m.Ref {
				return snapshotEntityKey(e)
			}
		}
	}
	return "name:" + m.Ref
}

func (h *Handler) findings(ctx context.Context, active []store.EntityMemberKey, activeConnectors map[string]bool, out *detail) error {
	findings, err := h.Store.ListOpenEntityFindings(ctx, active, maxFindings)
	if err != nil {
		return err
	}
	out.Findings = findings
	connectors := make([]string, 0, len(activeConnectors))
	for cid := range activeConnectors {
		connectors = append(connectors, cid)
	}
	sort.Strings(connectors)
	out.ConnectorFindings, err = h.Store.ListOpenConnectorLevelFindings(ctx, connectors, maxFindings)
	return err
}

func (h *Handler) runbooks(ctx context.Context, active []store.EntityMemberKey, out *detail) error {
	steps, err := h.Store.ListEntityRunbookSteps(ctx, active, maxRunbookSteps)
	if err != nil {
		return err
	}
	for _, st := range steps {
		r := runbook{ID: st.RunbookID, Title: st.RunbookTitle, Body: st.RunbookBody}
		r.Step.ID, r.Step.Title, r.Step.Verb = st.StepID, st.StepTitle, st.StepVerb
		out.Runbooks = append(out.Runbooks, r)
	}
	return nil
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

func notFound(w http.ResponseWriter) {
	httputil.Error(w, http.StatusNotFound, "not_found", "Entity not found")
}

func httpError(w http.ResponseWriter, err error) { httputil.Errorf(w, err) }
