package timeline

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/alerts"
	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/api/changes"
	connectorapi "github.com/WiseLabz/wiselabz/internal/api/connectors"
	"github.com/WiseLabz/wiselabz/internal/api/docs"
	"github.com/WiseLabz/wiselabz/internal/api/runbooks"
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

func TestMemberTimelineAuditVisibility(t *testing.T) {
	s := apitest.NewStore(t)
	ctx := context.Background()
	user := apitest.NewUser(t, s, "viewer")
	connectors := make([]store.ConnectorRecord, 2)
	for i := range connectors {
		connectors[i] = store.ConnectorRecord{Name: "Lab", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
		if err := s.CreateConnector(ctx, &connectors[i]); err != nil {
			t.Fatal(err)
		}
		apitest.GrantConnectorRole(t, s, user, connectors[i].ID, "viewer")
	}
	a, b := connectors[0].ID, connectors[1].ID
	for _, record := range []store.AuditRecord{
		{ID: "a", Action: "connector.sync", TargetType: "connector", TargetID: a, ActorUserID: "actor"},
		{ID: "b", Action: "connector.sync", TargetType: "connector", TargetID: b},
		{ID: "multi", Action: "runbook.update", ConnectorIDs: []string{a, b}},
		{ID: "unscoped", Action: "backup.import"},
		{ID: "security", Action: "auth.elevate", ConnectorIDs: []string{a}},
	} {
		if err := s.CreateAuditRecord(ctx, &record); err != nil {
			t.Fatal(err)
		}
	}
	h := &Handler{Store: s}
	for _, tc := range []struct {
		name string
		key  []string
		want int
	}{
		{"member", nil, 3},
		{"restricted key", []string{a}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/timeline?kinds=audit", nil)
			req = req.WithContext(auth.ContextWithAPIKeyRestriction(auth.ContextWithUser(ctx, user, false), auth.APIKeyRestriction{ConnectorIDs: tc.key}))
			rr := httptest.NewRecorder()
			h.List(rr, req)
			var page struct {
				Items []store.TimelineItem `json:"items"`
				Total int                  `json:"total"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &page); err != nil {
				t.Fatal(err)
			}
			if rr.Code != 200 || page.Total != tc.want || len(page.Items) != tc.want {
				t.Fatalf("audit timeline: %d %s", rr.Code, rr.Body.String())
			}
			if tc.key != nil && (page.Items[0].ID != "a" || page.Items[0].CreatedBy != "actor") {
				t.Fatalf("restricted projection: %+v", page.Items)
			}
		})
	}
}

func TestAuditWriterScopeSnapshots(t *testing.T) {
	s := apitest.NewStore(t)
	ctx := context.Background()
	user := apitest.NewUser(t, s, "operator")
	ctx = auth.ContextWithUser(ctx, user, false)
	connectors := make([]store.ConnectorRecord, 2)
	for i := range connectors {
		connectors[i] = store.ConnectorRecord{Name: "Lab", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
		if err := s.CreateConnector(ctx, &connectors[i]); err != nil {
			t.Fatal(err)
		}
		apitest.GrantConnectorRole(t, s, user, connectors[i].ID, "operator")
	}
	a, b := connectors[0].ID, connectors[1].ID
	call := func(handler http.HandlerFunc, id, body string, values map[string]string, want int) {
		t.Helper()
		req := httptest.NewRequest("POST", "/", strings.NewReader(body)).WithContext(ctx)
		req.SetPathValue("id", id)
		for key, value := range values {
			req.SetPathValue(key, value)
		}
		rr := httptest.NewRecorder()
		handler(rr, req)
		if rr.Code != want {
			t.Fatalf("handler status %d, want %d: %s", rr.Code, want, rr.Body.String())
		}
	}
	assertScope := func(action string, want ...string) {
		t.Helper()
		records, total, err := s.ListAuditRecords(ctx, action, "", "", "", 0, 100)
		if err != nil || total == 0 {
			t.Fatalf("audit %s missing: %v", action, err)
		}
		slices.Sort(want)
		for _, record := range records {
			rows, err := s.DB().QueryContext(ctx, "SELECT connector_id FROM audit_log_connectors WHERE audit_id = ? ORDER BY connector_id", record.ID)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for rows.Next() {
				var cid string
				if err := rows.Scan(&cid); err != nil {
					t.Fatal(err)
				}
				got = append(got, cid)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, want) {
				t.Fatalf("%s scope = %v, want %v", action, got, want)
			}
		}
	}
	t.Run("connector default", func(*testing.T) {
		h := &connectorapi.Handler{Store: s}
		call(h.ToggleEnabled, a, `{"enabled":true}`, nil, 200)
		assertScope("connector.toggle_enabled", a)
	})
	t.Run("changes", func(t *testing.T) {
		h := &changes.Handler{Store: s}
		c := store.ChangeRecord{ServiceID: a, ChangeType: "updated", Severity: "info"}
		if err := s.CreateChange(ctx, &c); err != nil {
			t.Fatal(err)
		}
		call(h.Acknowledge, c.ID, "", nil, 200)
		call(h.Dismiss, c.ID, "", nil, 200)
		call(h.BulkResolve, "", `{"ids":["`+c.ID+`"],"status":"acknowledged"}`, nil, 200)
		for _, action := range []string{"change.ack", "change.dismiss", "change.bulk_ack"} {
			assertScope(action, a)
		}
	})
	t.Run("alerts", func(t *testing.T) {
		h := &alerts.Handler{Store: s}
		alert := store.AlertRecord{ServiceID: a, Title: "Alert", Severity: "info"}
		if err := s.CreateAlert(ctx, &alert); err != nil {
			t.Fatal(err)
		}
		call(h.Resolve, alert.ID, "", nil, 200)
		call(h.Dismiss, alert.ID, "", nil, 200)
		call(h.Snooze, alert.ID, `{"until":"2030-01-01T00:00:00Z"}`, nil, 200)
		call(h.BulkSnooze, "", `{"ids":["`+alert.ID+`"],"until":"2030-01-01T00:00:00Z"}`, nil, 200)
		for _, action := range []string{"alert.resolve", "alert.dismiss", "alert.snooze", "alert.bulk_snooze"} {
			assertScope(action, a)
		}
	})
	t.Run("docs and proposals", func(t *testing.T) {
		h := &docs.Handler{Store: s}
		d := store.DocRecord{ServiceID: a, Title: "Doc", Kind: "service", Origin: store.DocOriginHuman, CurrentVersion: 1, Content: "old"}
		if err := s.CreateDoc(ctx, &d); err != nil {
			t.Fatal(err)
		}
		v := store.DocVersionRecord{DocID: d.ID, Rev: 1, Content: "old", Trigger: "manual"}
		if err := s.CreateDocVersion(ctx, &v); err != nil {
			t.Fatal(err)
		}
		call(h.Restore, d.ID, "", map[string]string{"rev": "1"}, 200)
		for _, review := range []http.HandlerFunc{h.ApproveProposal, h.RejectProposal} {
			current, err := s.GetDoc(ctx, d.ID)
			if err != nil {
				t.Fatal(err)
			}
			p := store.DocEditProposal{DocID: d.ID, BaseVersion: current.CurrentVersion, Content: "new", AuthorID: user}
			if err := s.CreateDocEditProposal(ctx, &p); err != nil {
				t.Fatal(err)
			}
			call(review, p.ID, "", nil, 200)
		}
		for _, action := range []string{"doc.restore", "doc.edit_approved", "doc.edit_rejected"} {
			assertScope(action, a)
		}
	})
	t.Run("runbook create update delete", func(t *testing.T) {
		h := runbooks.NewHandler(s, nil)
		body := `{"title":"Two connectors","targetType":"change_type","targetValue":"scope-test","steps":[{"kind":"sync_and_wait","title":"First","connectorId":"` + a + `","timeoutSeconds":300},{"kind":"sync_and_wait","title":"Second","connectorId":"` + b + `","timeoutSeconds":300}]}`
		call(h.Create, "", body, nil, 201)
		records, _, err := s.ListAuditRecords(ctx, "runbook.create", "", "", "", 0, 1)
		if err != nil || len(records) != 1 {
			t.Fatalf("created runbook audit: %v %v", records, err)
		}
		id := records[0].TargetID
		call(h.Update, id, `{"title":"Renamed"}`, nil, 200)
		call(h.Delete, id, "", nil, 204)
		for _, action := range []string{"runbook.create", "runbook.update", "runbook.delete"} {
			assertScope(action, a, b)
		}
	})
}
