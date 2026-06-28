package docs

import (
	"encoding/json"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/doc"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Handler holds dependencies for doc endpoints.
type Handler struct {
	Store     *store.Store
	DocEngine *doc.Engine
}

// NewHandler creates a new doc handler.
func NewHandler(s *store.Store, eng *doc.Engine) *Handler {
	return &Handler{Store: s, DocEngine: eng}
}

// Tree handles GET /api/docs/tree.
// Returns docs grouped by service (lab root + per-connector children).
func (h *Handler) Tree(w http.ResponseWriter, r *http.Request) {
	connectors, err := h.Store.ListAllConnectors(r.Context())
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	type TreeNode struct {
		ID       string     `json:"id"`
		Title    string     `json:"title"`
		Kind     string     `json:"kind"`
		Children []TreeNode `json:"children,omitempty"`
	}

	root := TreeNode{
		ID:    "root",
		Title: "Lab Documentation",
		Kind:  "lab",
	}

	for _, c := range connectors {
		docs, _ := h.Store.ListDocsByService(r.Context(), c.ID)
		connNode := TreeNode{
			ID:    c.ID,
			Title: c.Name,
			Kind:  "service",
		}
		for _, d := range docs {
			connNode.Children = append(connNode.Children, TreeNode{
				ID:    d.ID,
				Title: d.Title,
				Kind:  d.Kind,
			})
		}
		root.Children = append(root.Children, connNode)
	}

	if root.Children == nil {
		root.Children = []TreeNode{}
	}

	httputil.JSON(w, http.StatusOK, root)
}

// List handles GET /api/docs.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := httputil.Paginate(r)
	docs, total, err := h.Store.ListDocs(r.Context(), offset, pageSize)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.WritePaginated(w, docs, page, pageSize, total)
}

// Get handles GET /api/docs/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, err := h.Store.GetDoc(r.Context(), id)
	if err == store.ErrNotFound {
		httputil.Error(w, http.StatusNotFound, "not_found", "Doc not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, d)
}

// Save handles PUT /api/docs/{id}.
func (h *Handler) Save(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if err := h.Store.UpdateDoc(r.Context(), id, req.Content); err != nil {
		if err == store.ErrNotFound {
			httputil.Error(w, http.StatusNotFound, "not_found", "Doc not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	// Create version
	d, _ := h.Store.GetDoc(r.Context(), id)
	userID := auth.UserIDFromContext(r.Context())
	if d != nil {
		h.Store.CreateDocVersion(r.Context(), &store.DocVersionRecord{
			DocID:   id,
			Rev:     d.CurrentVersion,
			Content: req.Content,
			Author:  userID,
			Trigger: "manual",
		})
	}

	httputil.JSON(w, http.StatusOK, d)
}

// Versions handles GET /api/docs/{id}/versions.
func (h *Handler) Versions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	versions, err := h.Store.GetDocVersions(r.Context(), id)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"versions": versions})
}

// Restore handles POST /api/docs/{id}/restore.
// Restores a doc to a previous version.
func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	docID := r.PathValue("id")
	var req struct {
		Rev int `json:"rev"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	versions, err := h.Store.GetDocVersions(r.Context(), docID)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	var target *store.DocVersionRecord
	for _, v := range versions {
		if v.Rev == req.Rev {
			target = &v
			break
		}
	}
	if target == nil {
		httputil.Error(w, http.StatusNotFound, "not_found", "Version not found")
		return
	}

	if err := h.Store.UpdateDoc(r.Context(), docID, target.Content); err != nil {
		httputil.Errorf(w, err)
		return
	}

	d, _ := h.Store.GetDoc(r.Context(), docID)
	httputil.JSON(w, http.StatusOK, d)
}

// Generate handles POST /api/docs/generate.
// Generates a doc from a template and connector snapshot.
func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TemplateID  string `json:"templateId"`
		ConnectorID string `json:"connectorId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if req.TemplateID == "" || req.ConnectorID == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "templateId and connectorId are required")
		return
	}

	result, err := h.DocEngine.GenerateFromTemplate(r.Context(), req.TemplateID, req.ConnectorID)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, result)
}

// AISuggest handles POST /api/docs/{id}/ai-suggest.
// Stub — full implementation in Phase 9.
func (h *Handler) AISuggest(w http.ResponseWriter, r *http.Request) {
	httputil.JSON(w, http.StatusAccepted, map[string]any{
		"message":   "AI suggestion not yet available",
		"requestId": "ai-placeholder",
	})
}
