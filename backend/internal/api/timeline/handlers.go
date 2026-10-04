// Package timeline exposes lab history and durable manual notes.
package timeline

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/go-chi/chi/v5"
)

// Handler serves merged history and journal mutations.
type Handler struct{ Store *store.Store }

const timestampLayout = "2006-01-02T15:04:05.000000000Z"

var sourceKinds = []string{"change", "sync", "alert", "doc", "journal", "audit"}

func normalizedTime(raw string) (string, bool) {
	if raw == "" {
		return "", true
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return "", false
	}
	return t.UTC().Format(timestampLayout), true
}

// List handles GET /api/timeline with SQL-filtered keyset pagination.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	_, size, _ := httputil.Paginate(r)
	q := r.URL.Query()
	f := store.TimelineFilter{UserID: auth.UserIDFromContext(r.Context()), Admin: auth.InstanceAdminFromContext(r.Context()),
		ConnectorID: q.Get("connectorId"), AllSyncRuns: q.Get("allSyncRuns") == "true"}
	var ok bool
	f.After, ok = normalizedTime(q.Get("after"))
	if !ok {
		httputil.Error(w, 400, "invalid_request", "Invalid after timestamp")
		return
	}
	f.Before, ok = normalizedTime(q.Get("before"))
	if !ok || (f.After != "" && f.Before != "" && f.After > f.Before) {
		httputil.Error(w, 400, "invalid_request", "Invalid date range")
		return
	}
	if raw := q.Get("kinds"); raw != "" {
		f.Kinds = strings.Split(raw, ",")
		for _, kind := range f.Kinds {
			if !slices.Contains(sourceKinds, kind) {
				httputil.Error(w, 400, "invalid_request", "Invalid source kind")
				return
			}
		}
	}
	if raw := q.Get("cursor"); raw != "" {
		ts, key, valid := httputil.DecodeCursor(raw)
		kind, id, found := strings.Cut(key, ":")
		normalized, validTime := normalizedTime(ts)
		if !valid || !validTime || normalized != ts || !found || id == "" || !slices.Contains(sourceKinds, kind) {
			httputil.Error(w, 400, "invalid_request", "Invalid cursor")
			return
		}
		f.Cursor = store.TimelineCursor{Timestamp: ts, Kind: kind, ID: id}
	}
	items, total, more, err := h.Store.ListTimeline(r.Context(), f, size)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	next := ""
	if more {
		last := items[len(items)-1]
		next = httputil.EncodeCursor(last.Timestamp, last.Kind+":"+last.ID)
	}
	httputil.WritePaginatedCursor(w, items, 1, size, total, next)
}

type entryInput struct {
	Body        string `json:"body"`
	OccurredAt  string `json:"occurredAt"`
	ConnectorID string `json:"connectorId"`
	DocID       string `json:"docId"`
	EntityKind  string `json:"entityKind"`
	EntityName  string `json:"entityName"`
	EntityRef   string `json:"entityRef"`
}

func readInput(w http.ResponseWriter, r *http.Request) (entryInput, bool) {
	var in entryInput
	r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		httputil.Error(w, 400, "invalid_request", "Invalid entry")
		return in, false
	}
	if strings.TrimSpace(in.Body) == "" || utf8.RuneCountInString(in.Body) > 50000 ||
		len(in.ConnectorID) > 128 || len(in.DocID) > 128 || utf8.RuneCountInString(in.EntityKind) > 100 ||
		utf8.RuneCountInString(in.EntityName) > 500 || utf8.RuneCountInString(in.EntityRef) > 2048 {
		httputil.Error(w, 400, "invalid_request", "Invalid entry fields")
		return in, false
	}
	if in.OccurredAt == "" {
		in.OccurredAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	ts, ok := normalizedTime(in.OccurredAt)
	if !ok {
		httputil.Error(w, 400, "invalid_request", "Invalid occurrence time")
		return in, false
	}
	in.OccurredAt = ts
	return in, true
}

func (h *Handler) canRead(w http.ResponseWriter, r *http.Request, cid string) bool {
	if cid == "" && len(auth.APIKeyRestrictionFromContext(r.Context()).ConnectorIDs) == 0 {
		return true
	}
	ok, err := h.Store.UserHasConnectorRole(r.Context(), auth.UserIDFromContext(r.Context()), cid, "viewer")
	if err != nil {
		httputil.Errorf(w, err)
		return false
	}
	if !ok {
		httputil.Error(w, 404, "not_found", "Entry not found")
		return false
	}
	return true
}

func (h *Handler) canWrite(w http.ResponseWriter, r *http.Request, cid string) bool {
	if auth.APIKeyRestrictionFromContext(r.Context()).ReadOnly {
		httputil.Error(w, 403, "forbidden", "Insufficient permissions")
		return false
	}
	if cid == "" {
		if auth.InstanceAdminFromContext(r.Context()) {
			return true
		}
		httputil.Error(w, 403, "forbidden", "Insufficient permissions")
		return false
	}
	ok, err := h.Store.UserHasConnectorRole(r.Context(), auth.UserIDFromContext(r.Context()), cid, "operator")
	if err != nil {
		httputil.Errorf(w, err)
		return false
	}
	if !ok {
		httputil.Error(w, 403, "forbidden", "Insufficient permissions")
		return false
	}
	return true
}

func (h *Handler) validateLinks(w http.ResponseWriter, r *http.Request, in entryInput) bool {
	if in.ConnectorID != "" {
		_, err := h.Store.GetConnector(r.Context(), in.ConnectorID)
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, 404, "not_found", "Connector not found")
			return false
		}
		if err != nil {
			httputil.Errorf(w, err)
			return false
		}
	}
	if in.DocID == "" {
		return true
	}
	d, err := h.Store.GetDoc(r.Context(), in.DocID)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, 404, "not_found", "Doc not found")
		return false
	}
	if err != nil {
		httputil.Errorf(w, err)
		return false
	}
	if d.ServiceID != in.ConnectorID || (d.ServiceID == "" && d.Origin != store.DocOriginHuman && !auth.InstanceAdminFromContext(r.Context())) {
		httputil.Error(w, 400, "invalid_request", "Doc must be visible and share entry scope")
		return false
	}
	return h.canRead(w, r, d.ServiceID)
}

func (h *Handler) audit(r *http.Request, action, id string) {
	if err := h.Store.RecordAuditFromContext(r.Context(), action, "journal", id, nil); err != nil {
		slog.Error("journal audit failed", "error", err)
	}
}

// Create handles POST /api/journal for connector operators or lab admins.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	in, ok := readInput(w, r)
	if !ok || !h.canWrite(w, r, in.ConnectorID) || !h.validateLinks(w, r, in) {
		return
	}
	e := store.JournalEntry{Body: in.Body, OccurredAt: in.OccurredAt, CreatedBy: auth.UserIDFromContext(r.Context()),
		ConnectorID: in.ConnectorID, DocID: in.DocID, EntityKind: in.EntityKind, EntityName: in.EntityName, EntityRef: in.EntityRef}
	if err := h.Store.CreateJournalEntry(r.Context(), &e); err != nil {
		httputil.Errorf(w, err)
		return
	}
	h.audit(r, "journal.create", e.ID)
	httputil.JSON(w, http.StatusCreated, e)
}

func (h *Handler) loadForEdit(w http.ResponseWriter, r *http.Request) (store.JournalEntry, bool) {
	e, err := h.Store.GetJournalEntry(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, 404, "not_found", "Entry not found")
		return e, false
	}
	if err != nil {
		httputil.Errorf(w, err)
		return e, false
	}
	if !h.canRead(w, r, e.ConnectorID) {
		return e, false
	}
	if auth.APIKeyRestrictionFromContext(r.Context()).ReadOnly ||
		(e.CreatedBy != auth.UserIDFromContext(r.Context()) && !auth.InstanceAdminFromContext(r.Context())) {
		httputil.Error(w, 403, "forbidden", "Insufficient permissions")
		return e, false
	}
	return e, true
}

// Update handles PUT /api/journal/{id} for authors and admins.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	e, ok := h.loadForEdit(w, r)
	if !ok {
		return
	}
	in, ok := readInput(w, r)
	if !ok {
		return
	}
	if in.ConnectorID != e.ConnectorID && !h.canWrite(w, r, in.ConnectorID) {
		return
	}
	if !h.validateLinks(w, r, in) {
		return
	}
	e.Body = in.Body
	e.OccurredAt = in.OccurredAt
	e.ConnectorID = in.ConnectorID
	e.DocID = in.DocID
	e.EntityKind = in.EntityKind
	e.EntityName = in.EntityName
	e.EntityRef = in.EntityRef
	if err := h.Store.UpdateJournalEntry(r.Context(), &e); err != nil {
		httputil.Errorf(w, err)
		return
	}
	h.audit(r, "journal.update", e.ID)
	httputil.JSON(w, 200, e)
}

// Delete handles DELETE /api/journal/{id} for authors and admins.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	e, ok := h.loadForEdit(w, r)
	if !ok {
		return
	}
	if err := h.Store.DeleteJournalEntry(r.Context(), e.ID); err != nil {
		httputil.Errorf(w, err)
		return
	}
	h.audit(r, "journal.delete", e.ID)
	w.WriteHeader(http.StatusNoContent)
}
