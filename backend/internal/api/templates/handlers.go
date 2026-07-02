// Package templates provides API handlers for template CRUD and preview.
package templates

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/WiseLabz/wiselabz/internal/doc"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Handler holds dependencies for template endpoints.
type Handler struct {
	Store     *store.Store
	DocEngine *doc.Engine
}

// NewHandler creates a new template handler.
func NewHandler(s *store.Store, eng *doc.Engine) *Handler {
	return &Handler{Store: s, DocEngine: eng}
}

// List handles GET /api/templates.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	_, pageSize, offset := httputil.Paginate(r)
	templates, _, err := h.Store.ListTemplates(r.Context(), offset, pageSize)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if templates == nil {
		templates = []store.TemplateRecord{}
	}

	// Spec: GET /templates returns a bare Template[] (see openapi.yaml).
	httputil.JSON(w, http.StatusOK, templates)
}

// templateResponse builds the flat Template JSON shape the spec expects,
// decoding the opaque appliesTo text column into an object when present.
func templateResponse(t *store.TemplateRecord, sections []store.TemplateSectionRecord) map[string]any {
	var appliesTo any
	if t.AppliesTo != "" {
		var m map[string]any
		if err := json.Unmarshal([]byte(t.AppliesTo), &m); err == nil {
			appliesTo = m
		}
	}
	return map[string]any{
		"id":          t.ID,
		"name":        t.Name,
		"description": t.Description,
		"appliesTo":   appliesTo,
		"sections":    sections,
	}
}

// Get handles GET /api/templates/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := h.Store.GetTemplate(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Template not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	sections, _ := h.Store.GetTemplateSections(r.Context(), id)

	httputil.JSON(w, http.StatusOK, templateResponse(t, sections))
}

// Create handles POST /api/templates.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		AppliesTo   json.RawMessage `json:"appliesTo"`
		Sections    []struct {
			Title string `json:"title"`
			Order int    `json:"order"`
			Body  string `json:"body"`
		} `json:"sections"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if req.Name == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}

	t := &store.TemplateRecord{
		Name:        req.Name,
		Description: req.Description,
		AppliesTo:   string(req.AppliesTo),
	}
	if err := h.Store.CreateTemplate(r.Context(), t); err != nil {
		httputil.Errorf(w, err)
		return
	}

	sections := make([]store.TemplateSectionRecord, 0, len(req.Sections))
	for _, sec := range req.Sections {
		rec := store.TemplateSectionRecord{
			TemplateID: t.ID,
			Title:      sec.Title,
			Ord:        sec.Order,
			Body:       sec.Body,
		}
		_ = h.Store.CreateTemplateSection(r.Context(), &rec)
		sections = append(sections, rec)
	}

	httputil.JSON(w, http.StatusCreated, templateResponse(t, sections))
}

// Update handles PUT /api/templates/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name        *string          `json:"name"`
		Description *string          `json:"description"`
		AppliesTo   *json.RawMessage `json:"appliesTo"`
		Sections    *[]struct {
			ID    *string `json:"id"`
			Title string  `json:"title"`
			Order int     `json:"order"`
			Body  string  `json:"body"`
		} `json:"sections"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	updates := make(map[string]any)
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.AppliesTo != nil {
		updates["applies_to"] = string(*req.AppliesTo)
	}
	if len(updates) > 0 {
		if err := h.Store.UpdateTemplate(r.Context(), id, updates); err != nil {
			httputil.Errorf(w, err)
			return
		}
	}

	if req.Sections != nil {
		_ = h.Store.DeleteTemplateSections(r.Context(), id)
		for _, sec := range *req.Sections {
			_ = h.Store.CreateTemplateSection(r.Context(), &store.TemplateSectionRecord{
				TemplateID: id,
				Title:      sec.Title,
				Ord:        sec.Order,
				Body:       sec.Body,
			})
		}
	}

	t, err := h.Store.GetTemplate(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Template not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	sections, _ := h.Store.GetTemplateSections(r.Context(), id)
	httputil.JSON(w, http.StatusOK, templateResponse(t, sections))
}

// Delete handles DELETE /api/templates/{id}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Store.DeleteTemplate(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Template not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}
	httputil.NoContent(w)
}

// Preview handles POST /api/templates/{id}/preview.
// Renders a template against a snapshot without saving.
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		ConnectorID string `json:"connectorId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	result, err := h.DocEngine.GenerateFromTemplate(r.Context(), id, req.ConnectorID)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{
		"rev":       0,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
		"author":    nil,
		"trigger":   "template",
		"content":   result.Content,
	})
}
