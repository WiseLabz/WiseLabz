// Package findings provides API handlers for documentation quality findings.
package findings

import (
	"context"
	"errors"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Handler holds dependencies for quality finding endpoints.
type Handler struct {
	Store *store.Store
}

// NewHandler creates a quality finding handler.
func NewHandler(s *store.Store) *Handler {
	return &Handler{Store: s}
}

func (h *Handler) response(ctx context.Context, finding store.QualityFindingRecord) (map[string]any, error) {
	connector, err := h.Store.GetConnector(ctx, finding.ConnectorID)
	if err != nil {
		return nil, err
	}

	var docID, resolvedAt any
	if finding.DocID != "" {
		docID = finding.DocID
	}
	if finding.ResolvedAt != "" {
		resolvedAt = finding.ResolvedAt
	}

	return map[string]any{
		"id":              finding.ID,
		"connectorId":     finding.ConnectorID,
		"connectorName":   connector.Name,
		"docId":           docID,
		"checkType":       finding.CheckType,
		"severity":        finding.Severity,
		"title":           finding.Title,
		"description":     finding.Description,
		"remediationLink": finding.RemediationLink,
		"status":          finding.Status,
		"detectedCount":   finding.DetectedCount,
		"firstDetectedAt": finding.FirstDetectedAt,
		"lastSeenAt":      finding.LastSeenAt,
		"resolvedAt":      resolvedAt,
	}, nil
}

// List handles GET /api/findings.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := httputil.Paginate(r)
	findings, total, err := h.Store.ListQualityFindings(
		r.Context(),
		r.URL.Query().Get("connectorId"),
		r.URL.Query().Get("checkType"),
		r.URL.Query().Get("status"),
		offset,
		pageSize,
	)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	items := make([]map[string]any, 0, len(findings))
	for _, finding := range findings {
		item, err := h.response(r.Context(), finding)
		if err != nil {
			httputil.Errorf(w, err)
			return
		}
		items = append(items, item)
	}
	// The API contract calls this field items; WritePaginated uses data.
	httputil.JSON(w, http.StatusOK, map[string]any{
		"items": items, "total": total, "page": page, "pageSize": pageSize,
	})
}

// Get handles GET /api/findings/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	finding, err := h.Store.GetQualityFinding(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Quality finding not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	response, err := h.response(r.Context(), *finding)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

// Resolve handles POST /api/findings/{id}/resolve.
func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Store.UpdateQualityFindingStatus(r.Context(), id, "resolved"); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Quality finding not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}
	finding, err := h.Store.GetQualityFinding(r.Context(), id)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	response, err := h.response(r.Context(), *finding)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}
