// Package attention provides the attention queue API handler (aggregated alerts + findings).
package attention

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Handler holds dependencies for attention queue endpoints.
type Handler struct {
	Store *store.Store
}

// NewHandler creates a new attention handler.
func NewHandler(s *store.Store) *Handler {
	return &Handler{Store: s}
}

// Item represents a merged alert or finding for the attention queue.
type Item struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"` // "alert" or "finding"
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	ConnectorID string `json:"connectorId"`
	DetectedAt  string `json:"detectedAt"`
	ChangeID    string `json:"changeId,omitempty"`  // alerts only
	FindingType string `json:"checkType,omitempty"` // findings only
	RunbookID   string `json:"runbookId,omitempty"`
}

// severityRank returns a numeric rank for sorting: critical=0, warning=1, info=2.
func severityRank(severity string) int {
	switch severity {
	case "critical":
		return 0
	case "warning":
		return 1
	case "info":
		return 2
	default:
		return 3
	}
}

// List handles GET /api/attention.
// Returns merged, sorted list of open alerts and findings.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := httputil.Paginate(r)
	ctx := r.Context()

	// days is optional; unset keeps today's back-compat behavior of showing everything.
	since := ""
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			since = time.Now().UTC().AddDate(0, 0, -n).Format(time.RFC3339)
		}
	}

	// Fetch pending alerts (unresolved and not dismissed)
	alerts, _, err := h.Store.ListAlerts(ctx, "", "", "pending", "", 0, 1000) // ponytail: unbounded fetch for merge
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	// Fetch open findings
	findings, _, err := h.Store.ListQualityFindings(ctx, "", "", "open", 0, 1000)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	// Merge into Items
	items := make([]Item, 0, len(alerts)+len(findings))

	// Add alerts
	for _, a := range alerts {
		items = append(items, Item{
			ID:          a.ID,
			Kind:        "alert",
			Severity:    a.Severity,
			Title:       a.Title,
			ConnectorID: a.ServiceID,
			DetectedAt:  a.CreatedAt,
			ChangeID:    a.ChangeID,
			RunbookID:   "", // will populate if runbook binding exists
		})
	}

	// Add findings
	for _, f := range findings {
		items = append(items, Item{
			ID:          f.ID,
			Kind:        "finding",
			Severity:    f.Severity,
			Title:       f.Title,
			ConnectorID: f.ConnectorID,
			DetectedAt:  f.LastSeenAt,
			FindingType: f.CheckType,
			RunbookID:   "", // will populate if runbook binding exists
		})
	}

	// Filter merged items by the optional days window.
	if since != "" {
		filtered := items[:0]
		for _, item := range items {
			if item.DetectedAt >= since {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	// Sort by severity (critical first) then by detectedAt (newest first)
	sort.Slice(items, func(i, j int) bool {
		if severityRank(items[i].Severity) != severityRank(items[j].Severity) {
			return severityRank(items[i].Severity) < severityRank(items[j].Severity)
		}
		return items[i].DetectedAt > items[j].DetectedAt
	})

	// Populate runbook IDs by matching severity/checkType
	for i, item := range items {
		var rb *store.RunbookRecord
		switch item.Kind {
		case "alert":
			rb, _ = h.Store.GetRunbookByTarget(ctx, "alert_severity", item.Severity)
		case "finding":
			rb, _ = h.Store.GetRunbookByTarget(ctx, "finding_check_type", item.FindingType)
		}
		if rb != nil {
			items[i].RunbookID = rb.ID
		}
	}

	// Paginate in memory
	total := len(items)
	start := offset
	end := offset + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	paginatedItems := items[start:end]
	httputil.JSON(w, http.StatusOK, map[string]any{
		"data":     paginatedItems,
		"page":     page,
		"pageSize": pageSize,
		"total":    total,
	})
}
