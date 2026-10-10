package docs

import (
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/doclink"
	"github.com/WiseLabz/wiselabz/internal/httputil"
)

func (h *Handler) resolveLinks(r *http.Request, content string) (string, []string, error) {
	return doclink.Resolve(content, func(target string) ([]doclink.Target, error) {
		return h.Store.LookupDocLink(r.Context(), auth.UserIDFromContext(r.Context()), target)
	})
}

// Backlinks lists visible source docs after checking the target's viewer gate.
func (h *Handler) Backlinks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.loadDocForViewer(w, r, id) {
		return
	}
	links, err := h.Store.ListDocBacklinks(r.Context(), auth.UserIDFromContext(r.Context()), "doc", id)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, links)
}
