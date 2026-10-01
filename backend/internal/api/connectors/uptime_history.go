package connectors

import (
	"errors"
	"net/http"
	"time"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// historyWindows maps the supported ?window= values to their lookback and
// bucket count (bucket width: 24h/96 = 15m, 7d/168 = 1h, 30d/120 = 6h).
var historyWindows = map[string]struct {
	d       time.Duration
	buckets int
}{
	"24h": {24 * time.Hour, 96},
	"7d":  {7 * 24 * time.Hour, 168},
	"30d": {30 * 24 * time.Hour, 120},
}

// UptimeHistory handles GET /api/connectors/{id}/uptime/history?window=24h|7d|30d.
// Returns downsampled health-check buckets (worst status, mean latency) for
// status/latency sparklines. window defaults to 24h.
func (h *Handler) UptimeHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	label := r.URL.Query().Get("window")
	if label == "" {
		label = "24h"
	}
	win, ok := historyWindows[label]
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_window", "window must be one of 24h, 7d, 30d")
		return
	}

	if _, err := h.Store.GetConnector(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	now := time.Now().UTC()
	buckets, err := h.Store.GetHealthHistory(r.Context(), id, now.Add(-win.d), now, win.buckets)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{
		"connectorId": id,
		"window":      label,
		"buckets":     buckets,
	})
}

// FleetUptime handles GET /api/uptime?window=24h|7d|30d.
// Returns per-connector availability for the connectors the caller can view
// (viewer grant, which also applies API-key connector restrictions). window
// defaults to 7d.
func (h *Handler) FleetUptime(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	label := r.URL.Query().Get("window")
	if label == "" {
		label = "7d"
	}
	win, ok := historyWindows[label]
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_window", "window must be one of 24h, 7d, 30d")
		return
	}

	names, err := h.Store.ListConnectorNames(ctx)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	allIDs := make([]string, len(names))
	nameByID := make(map[string]string, len(names))
	for i, n := range names {
		allIDs[i] = n.ID
		nameByID[n.ID] = n.Name
	}
	ids, err := h.Store.FilterConnectorIDsByGrant(ctx, auth.UserIDFromContext(ctx), allIDs, "viewer")
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	now := time.Now().UTC()
	stats, err := h.Store.GetFleetUptime(ctx, ids, now.Add(-win.d), now)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	out := make([]map[string]any, 0, len(stats))
	for _, st := range stats {
		out = append(out, map[string]any{
			"connectorId":     st.ConnectorID,
			"name":            nameByID[st.ConnectorID],
			"checkCount":      st.CheckCount,
			"availabilityPct": st.AvailabilityPct,
			"mttrSeconds":     st.MTTRSeconds,
			"outageCount":     st.OutageCount,
		})
	}
	httputil.JSON(w, http.StatusOK, map[string]any{
		"window":     label,
		"connectors": out,
	})
}
