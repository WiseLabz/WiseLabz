package timeline

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/go-chi/chi/v5"
)

func TestJournalAuthorizationAndBackdatedEntry(t *testing.T) {
	s := apitest.NewStore(t)
	h := &Handler{Store: s}
	cid := store.ConnectorRecord{Name: "Lab", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
	if err := s.CreateConnector(context.Background(), &cid); err != nil {
		t.Fatal(err)
	}
	author := apitest.NewUser(t, s, "viewer")
	viewer := apitest.NewUser(t, s, "viewer")
	admin := apitest.NewUser(t, s, "operator")
	outsider := apitest.NewUser(t, s, "viewer")
	apitest.GrantConnectorRole(t, s, author, cid.ID, "operator")
	apitest.GrantConnectorRole(t, s, viewer, cid.ID, "viewer")
	apitest.GrantConnectorRole(t, s, admin, cid.ID, "viewer")
	mux := chi.NewRouter()
	mux.Post("/journal", h.Create)
	mux.Put("/journal/{id}", h.Update)
	mux.Delete("/journal/{id}", h.Delete)
	mux.Get("/timeline", h.List)
	call := func(method, path, user string, isAdmin bool, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req = req.WithContext(auth.ContextWithUser(req.Context(), user, isAdmin))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	body := `{"body":"Replaced router","occurredAt":"2020-01-01T00:00:00-03:00","connectorId":"` + cid.ID + `","entityKind":"vm","entityName":"router","entityRef":"vm/100"}`
	for _, tc := range []struct {
		name, user string
		admin      bool
		body       string
		want       int
	}{
		{"operator creates", author, false, body, 201},
		{"viewer denied", viewer, false, body, 403},
		{"admin viewer cannot create scoped", admin, true, body, 403},
		{"no grant denied", outsider, false, body, 403},
		{"lab admin", admin, true, `{"body":"Lab note"}`, 201},
		{"lab user denied", author, false, `{"body":"Lab note"}`, 403},
		{"invalid time", author, false, `{"body":"Bad","occurredAt":"yesterday"}`, 400},
		{"empty body", admin, true, `{"body":" "}`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := call("POST", "/journal", tc.user, tc.admin, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status %d want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
	rec := call("POST", "/journal", author, false, body)
	var e store.JournalEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e.OccurredAt != "2020-01-01T03:00:00.000000000Z" || e.EntityRef != "vm/100" || e.CreatedBy != author {
		t.Fatalf("entry %+v", e)
	}
	// Authors retain edit/delete rights after an operator grant is downgraded.
	apitest.GrantConnectorRole(t, s, author, cid.ID, "viewer")
	for _, tc := range []struct {
		name, user   string
		admin        bool
		method, body string
		want         int
	}{
		{"non-author viewer", viewer, false, "PUT", body, 403},
		{"outsider hidden", outsider, false, "PUT", body, 404},
		{"author viewer edits", author, false, "PUT", body, 200},
		{"author cannot move lab", author, false, "PUT", `{"body":"Moved"}`, 403},
		{"admin edits", admin, true, "PUT", body, 200},
		{"non-author delete denied", viewer, false, "DELETE", "", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := call(tc.method, "/journal/"+e.ID, tc.user, tc.admin, tc.body)
			if r.Code != tc.want {
				t.Fatalf("status %d want %d: %s", r.Code, tc.want, r.Body.String())
			}
		})
	}
	rec = call("GET", "/timeline?kinds=journal", viewer, false, "")
	var page struct {
		Items []store.TimelineItem `json:"items"`
		Total int                  `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || page.Total != 3 || page.Items[len(page.Items)-1].Timestamp != e.OccurredAt {
		t.Fatalf("backdated timeline: %s", rec.Body.String())
	}
	rec = call("DELETE", "/journal/"+e.ID, author, false, "")
	if rec.Code != 204 {
		t.Fatalf("delete: %s", rec.Body.String())
	}
	audit, total, err := s.ListAuditRecords(context.Background(), "journal.delete", "", "", "", 0, 100)
	if err != nil || total != 1 || audit[0].ActorUserID != author {
		t.Fatalf("audit %+v %d: %v", audit, total, err)
	}
}

func TestJournalDocScopeAndRestrictedKey(t *testing.T) {
	s := apitest.NewStore(t)
	h := &Handler{Store: s}
	ctx := context.Background()
	user := apitest.NewUser(t, s, "operator")
	c := store.ConnectorRecord{Name: "c", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
	if err := s.CreateConnector(ctx, &c); err != nil {
		t.Fatal(err)
	}
	apitest.GrantConnectorRole(t, s, user, c.ID, "operator")
	d := store.DocRecord{Title: "Private", Kind: "service", ServiceID: c.ID, Origin: store.DocOriginHuman}
	if err := s.CreateDoc(ctx, &d); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		body        string
		restriction auth.APIKeyRestriction
		want        int
	}{
		{`{"body":"cross scope","docId":"` + d.ID + `"}`, auth.APIKeyRestriction{}, 400},
		{`{"body":"lab"}`, auth.APIKeyRestriction{ConnectorIDs: []string{c.ID}}, 403},
		{`{"body":"scoped","connectorId":"` + c.ID + `"}`, auth.APIKeyRestriction{ReadOnly: true}, 403},
	} {
		req := httptest.NewRequest("POST", "/journal", strings.NewReader(tc.body))
		req = req.WithContext(auth.ContextWithAPIKeyRestriction(auth.ContextWithUser(ctx, user, true), tc.restriction))
		rec := httptest.NewRecorder()
		h.Create(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("got %d want %d: %s", rec.Code, tc.want, rec.Body.String())
		}
	}
	for _, q := range []string{"after=no", "after=2026-01-01T00:00:00Z&before=2025-01-01T00:00:00Z", "kinds=security", "cursor=bad"} {
		req := httptest.NewRequest(http.MethodGet, "/timeline?"+q, nil)
		rec := httptest.NewRecorder()
		h.List(rec, req)
		if rec.Code != 400 {
			t.Fatalf("%s: %d", q, rec.Code)
		}
	}
}
