// Package docs provides API handlers for documentation CRUD and AI suggestions.
package docs

import (
	"errors"
	"net/http"
	"sync"

	"github.com/WiseLabz/wiselabz/internal/ai"
	"github.com/WiseLabz/wiselabz/internal/api/settings"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/doc"
	"github.com/WiseLabz/wiselabz/internal/docimport/pull"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

// Handler holds dependencies for doc endpoints.
type Handler struct {
	Store     *store.Store
	DocEngine *doc.Engine
	Settings  *settings.Handler
	AI        *ai.Registry
	Embed     *ai.EmbedRegistry
	WSHub     *ws.Hub
	Pull      *pull.Manager
	pullOnce  sync.Once
}

// NewHandler creates a new doc handler.
func NewHandler(s *store.Store, eng *doc.Engine, settingsH *settings.Handler, aiRegistry *ai.Registry, embedRegistry *ai.EmbedRegistry, hub *ws.Hub) *Handler {
	return &Handler{Store: s, DocEngine: eng, Settings: settingsH, AI: aiRegistry, Embed: embedRegistry, WSHub: hub}
}

// Tree handles GET /api/docs/tree.
// Returns docs grouped by service (lab root + per-connector children).
func (h *Handler) Tree(w http.ResponseWriter, r *http.Request) {
	connectors, err := h.Store.ListConnectorNames(r.Context())
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	connectorIDs := make([]string, len(connectors))
	for i, c := range connectors {
		connectorIDs[i] = c.ID
	}
	allowedIDs, err := h.Store.FilterConnectorIDsByGrant(r.Context(), auth.UserIDFromContext(r.Context()), connectorIDs, "viewer")
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	isAllowed := make(map[string]bool, len(allowedIDs))
	for _, id := range allowedIDs {
		isAllowed[id] = true
	}
	docsByService, err := h.Store.ListDocsGroupedByService(r.Context())
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	root := DocTreeNode{ID: "root", Title: "Lab Documentation", Kind: "lab", Branch: true}
	labDocs := []store.DocRecord{}
	for _, d := range docsByService[""] {
		if d.Origin == store.DocOriginHuman || auth.InstanceAdminFromContext(r.Context()) {
			labDocs = append(labDocs, d)
		}
	}
	root.Children = append(root.Children, DocTreeNode{ID: "lab", Title: "Lab", Kind: "lab", Branch: true, Children: nestedDocNodes(labDocs, "")})
	for _, c := range connectors {
		if !isAllowed[c.ID] {
			continue
		}
		root.Children = append(root.Children, DocTreeNode{ID: c.ID, Title: c.Name, Kind: "service", ServiceID: c.ID, Branch: true, Children: nestedDocNodes(docsByService[c.ID], "")})
	}

	httputil.JSON(w, http.StatusOK, root)
}

// List handles GET /api/docs.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := httputil.Paginate(r)
	search := r.URL.Query().Get("search")

	docs, total, err := h.Store.ListViewableDocs(r.Context(), auth.UserIDFromContext(r.Context()), search, offset, pageSize)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.WritePaginated(w, docs, page, pageSize, total)
}

// Get handles GET /api/docs/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// The tree returns docId "root" for the lab overview. There is no real doc
	// with that ID — return a synthetic placeholder.
	if id == "root" || id == "lab" {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"docId":          "root",
			"title":          "Lab Documentation",
			"content":        "_Welcome to your lab documentation. Sync a service to populate this page._",
			"currentVersion": 1,
			"kind":           "lab",
			"serviceId":      "",
		})
		return
	}

	d, err := h.Store.GetDoc(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		if _, deletedErr := h.Store.GetDeletedDoc(r.Context(), id); deletedErr == nil {
			httputil.Error(w, http.StatusNotFound, "not_found", "Doc not found")
			return
		} else if !errors.Is(deletedErr, store.ErrNotFound) {
			httputil.Errorf(w, deletedErr)
			return
		}
		// Not a doc ID — maybe it's a connector ID. Fall back to service lookup.
		if !h.requireDocViewer(w, r, id) {
			return
		}
		docs, svcErr := h.Store.ListDocsByService(r.Context(), id)
		if svcErr == nil && len(docs) > 0 {
			h.writeDoc(w, r, &docs[0])
			return
		}
		if svcErr == nil {
			httputil.JSON(w, http.StatusOK, map[string]any{
				"docId":          "",
				"title":          "No documentation yet",
				"content":        "Run a sync to generate documentation for this service.",
				"currentVersion": 0,
				"kind":           "service",
				"serviceId":      id,
			})
			return
		}
		httputil.Error(w, http.StatusNotFound, "not_found", "Doc not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if !h.requireDocViewer(w, r, d.ServiceID, d.Origin) {
		return
	}
	h.writeDoc(w, r, d)
}

// ByService handles GET /api/docs/service/{connectorId}.
func (h *Handler) ByService(w http.ResponseWriter, r *http.Request) {
	connectorID := r.PathValue("id")
	if !h.requireDocViewer(w, r, connectorID) {
		return
	}
	docs, err := h.Store.ListDocsByService(r.Context(), connectorID)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if len(docs) == 0 {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"docId":          "",
			"title":          "No documentation yet",
			"content":        "Run a sync to generate documentation for this service.",
			"currentVersion": 0,
			"kind":           "service",
			"serviceId":      connectorID,
		})
		return
	}
	h.writeDoc(w, r, &docs[0])
}

// Save handles PUT /api/docs/{id}.
func (h *Handler) Save(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	req, ok := httputil.DecodeJSON[struct {
		Content     string `json:"content"`
		BaseVersion *int   `json:"baseVersion"`
		Trigger     string `json:"trigger"`
	}](w, r)
	if !ok {
		return
	}

	existing, err := h.Store.GetDoc(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Doc not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if !h.requireDocOperator(w, r, existing.ServiceID) {
		return
	}

	trigger := req.Trigger
	if trigger == "" {
		trigger = "manual"
	}
	if _, err := h.Store.UpdateDocWithVersion(r.Context(), id, req.Content, req.BaseVersion, auth.UserIDFromContext(r.Context()), trigger); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Doc not found")
			return
		}
		if errors.Is(err, store.ErrVersionConflict) {
			current, getErr := h.Store.GetDoc(r.Context(), id)
			if getErr != nil {
				httputil.Errorf(w, getErr)
				return
			}
			httputil.JSON(w, http.StatusConflict, current)
			return
		}
		httputil.Errorf(w, err)
		return
	}

	d, _ := h.Store.GetDoc(r.Context(), id)
	if d != nil {
		h.SyncEmbeddings(r.Context(), d.ID, d.Content)
	}
	h.writeDoc(w, r, d)
}
