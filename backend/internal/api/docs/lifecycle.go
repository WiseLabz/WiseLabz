package docs

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// DocTreeNode represents either a virtual scope branch or a nested doc.
type DocTreeNode struct {
	ID        string        `json:"docId"`
	Title     string        `json:"title"`
	Kind      string        `json:"kind"`
	ServiceID string        `json:"serviceId"`
	ParentID  string        `json:"parentId"`
	Origin    string        `json:"origin,omitempty"`
	Branch    bool          `json:"branch"`
	Children  []DocTreeNode `json:"children,omitempty"`
}

// nestedDocNodes nests docs under their parents, keeping input order among
// siblings. Docs whose parent is not in the list (hidden or missing) become
// roots. The parent index is built once, so nesting is O(n).
func nestedDocNodes(docs []store.DocRecord, parent string) []DocTreeNode {
	visible := make(map[string]bool, len(docs))
	for _, d := range docs {
		visible[d.ID] = true
	}
	children := make(map[string][]*store.DocRecord, len(docs))
	for i := range docs {
		effectiveParent := docs[i].ParentID
		if !visible[effectiveParent] {
			effectiveParent = ""
		}
		children[effectiveParent] = append(children[effectiveParent], &docs[i])
	}
	var build func(parent string) []DocTreeNode
	build = func(parent string) []DocTreeNode {
		nodes := []DocTreeNode{}
		for _, d := range children[parent] {
			nodes = append(nodes, DocTreeNode{
				ID: d.ID, Title: d.Title, Kind: d.Kind,
				ServiceID: d.ServiceID, ParentID: d.ParentID, Origin: d.Origin,
				Children: build(d.ID),
			})
		}
		return nodes
	}
	return build(parent)
}

func (h *Handler) lifecycleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		httputil.Error(w, http.StatusNotFound, "not_found", "Doc not found")
	case errors.Is(err, store.ErrDocHierarchy):
		httputil.Error(w, http.StatusBadRequest, "invalid_parent", err.Error())
	case errors.Is(err, store.ErrGeneratedDocExists):
		httputil.Error(w, http.StatusConflict, "generated_doc_exists", err.Error())
	default:
		httputil.Errorf(w, err)
	}
}

// Create handles POST /api/docs and writes the first human revision.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	req, ok := httputil.DecodeJSON[struct {
		Title     string `json:"title"`
		ServiceID string `json:"serviceId"`
		ParentID  string `json:"parentId"`
		Content   string `json:"content"`
	}](w, r)
	if !ok {
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || utf8.RuneCountInString(req.Title) > 500 {
		httputil.Error(w, http.StatusBadRequest, "invalid_title", "Title must contain 1 to 500 characters")
		return
	}
	if !h.requireDocOperator(w, r, req.ServiceID) {
		return
	}
	if req.ServiceID != "" {
		if _, err := h.Store.GetConnector(r.Context(), req.ServiceID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				httputil.Error(w, http.StatusNotFound, "not_found", "Service not found")
			} else {
				httputil.Errorf(w, err)
			}
			return
		}
	}
	d := &store.DocRecord{
		Title: req.Title, ServiceID: req.ServiceID, ParentID: req.ParentID,
		Content: req.Content, CreatedBy: auth.UserIDFromContext(r.Context()),
		Origin: store.DocOriginHuman,
	}
	if err := h.Store.CreateHumanDoc(r.Context(), d); err != nil {
		h.lifecycleError(w, err)
		return
	}
	h.SyncEmbeddings(r.Context(), d.ID, d.Content)
	httputil.JSON(w, http.StatusCreated, d)
}

// Patch handles metadata changes on an active doc.
func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	req, ok := httputil.DecodeJSON[struct {
		Title    *string `json:"title"`
		ParentID *string `json:"parentId"`
	}](w, r)
	if !ok {
		return
	}
	if req.Title != nil {
		trimmed := strings.TrimSpace(*req.Title)
		req.Title = &trimmed
		if trimmed == "" || utf8.RuneCountInString(trimmed) > 500 {
			httputil.Error(w, http.StatusBadRequest, "invalid_title", "Title must contain 1 to 500 characters")
			return
		}
	}
	id := r.PathValue("id")
	d, err := h.Store.GetDoc(r.Context(), id)
	if err != nil {
		h.lifecycleError(w, err)
		return
	}
	if !h.requireDocEditor(w, r, d) {
		return
	}
	if err := h.Store.UpdateDocMetadata(r.Context(), id, req.Title, req.ParentID); err != nil {
		h.lifecycleError(w, err)
		return
	}
	d, err = h.Store.GetDoc(r.Context(), id)
	if err != nil {
		h.lifecycleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, d)
}

// Delete moves the active subtree into trash.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, err := h.Store.GetDoc(r.Context(), id)
	if err != nil {
		h.lifecycleError(w, err)
		return
	}
	if !h.requireDocEditor(w, r, d) {
		return
	}
	if err := h.Store.SoftDeleteDoc(r.Context(), id); err != nil {
		h.lifecycleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Trash lists deleted docs for instance administrators.
func (h *Handler) Trash(w http.ResponseWriter, r *http.Request) {
	if !h.requireDocOperator(w, r, "") {
		return
	}
	docs, err := h.Store.ListDeletedDocs(r.Context())
	if err != nil {
		h.lifecycleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, docs)
}

// RestoreDeleted restores the selected subtree deletion batch.
func (h *Handler) RestoreDeleted(w http.ResponseWriter, r *http.Request) {
	if !h.requireDocOperator(w, r, "") {
		return
	}
	id := r.PathValue("id")
	if err := h.Store.RestoreDeletedDoc(r.Context(), id); err != nil {
		h.lifecycleError(w, err)
		return
	}
	d, err := h.Store.GetDoc(r.Context(), id)
	if err != nil {
		h.lifecycleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, d)
}
