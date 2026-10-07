package connectors

import (
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/compliance"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/httputil"
)

type certificateExpiry struct {
	Name          string    `json:"name"`
	ConnectorID   string    `json:"connectorId"`
	ConnectorName string    `json:"connectorName"`
	ExternalID    string    `json:"externalId"`
	EntityID      string    `json:"entityId,omitempty"`
	NotAfter      time.Time `json:"notAfter"`
	DaysLeft      int       `json:"daysLeft"`
	Unreachable   bool      `json:"unreachable"`
}

// Certificates lists dated certificates independently of compliance rules.
func (h *Handler) Certificates(w http.ResponseWriter, r *http.Request) {
	limit := 10
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "Invalid limit",
				[]httputil.FieldError{{Field: "limit", Msg: "must be a positive integer"}})
			return
		}
		limit = min(n, 100)
	}
	ctx := r.Context()
	names, err := h.Store.ListConnectorNames(ctx)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	allIDs := make([]string, len(names))
	for i, c := range names {
		allIDs[i] = c.ID
	}
	ids, err := h.Store.FilterConnectorIDsByGrant(ctx, auth.UserIDFromContext(ctx), allIDs, "viewer")
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	connectors, err := h.Store.ListConnectorsByID(ctx, ids)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	catalog := connector.AttributeCatalog()
	now := time.Now().UTC()
	items := []certificateExpiry{}
	for _, id := range ids {
		c := connectors[id]
		if !slices.ContainsFunc(catalog[c.Type]["certificate"], func(a connector.AttributeSpec) bool {
			return a.Name == "not_after"
		}) {
			continue
		}
		snapshot, err := compliance.LoadLatestSnapshot(ctx, h.Store, id)
		if err != nil {
			httputil.Errorf(w, err)
			return
		}
		if snapshot == nil {
			continue
		}
		for _, e := range snapshot.Entities {
			date, ok := e.Attributes["not_after"].(string)
			if e.Kind != "certificate" || !ok {
				continue
			}
			expiry, err := time.Parse(time.RFC3339, date)
			if err != nil {
				continue
			}
			reachable, reported := e.Attributes["reachable"].(bool)
			items = append(items, certificateExpiry{
				Name: e.Name, ConnectorID: c.ID, ConnectorName: c.Name, ExternalID: e.ExternalID,
				NotAfter: expiry.UTC(), DaysLeft: compliance.DaysLeft(expiry, now), Unreachable: reported && !reachable,
			})
		}
	}
	slices.SortStableFunc(items, func(a, b certificateExpiry) int { return a.NotAfter.Compare(b.NotAfter) })
	items = items[:min(limit, len(items))]
	for i := range items {
		id, err := h.Store.CertificateEntityID(ctx, items[i].ConnectorID, items[i].ExternalID)
		if err != nil {
			httputil.Errorf(w, err)
			return
		}
		items[i].EntityID = id
	}
	httputil.JSON(w, http.StatusOK, items)
}
