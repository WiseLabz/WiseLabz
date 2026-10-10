package docs

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
)

func TestSaveResolvesVisibleLinksAndLeavesHiddenOrAmbiguousLinks(t *testing.T) {
	h := newTestHandler(t)
	ctx := context.Background()
	conn := &store.ConnectorRecord{Name: "writer service", Category: "virtualization", Type: "test", URL: "https://writer.test"}
	hiddenConn := &store.ConnectorRecord{Name: "hidden service", Category: "virtualization", Type: "test", URL: "https://hidden.test"}
	for _, c := range []*store.ConnectorRecord{conn, hiddenConn} {
		if err := h.Store.CreateConnector(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []*store.DocRecord{
		{ID: "11111111-1111-4111-8111-111111111111", Title: "Visible Target", Content: "target", Origin: store.DocOriginHuman},
		{ID: "22222222-2222-4222-8222-222222222222", Title: "Repeated", Content: "first", Origin: store.DocOriginHuman},
		{ID: "33333333-3333-4333-8333-333333333333", Title: "Repeated", Content: "second", Origin: store.DocOriginHuman},
		{ID: "44444444-4444-4444-8444-444444444444", Title: "Secret Target", ServiceID: hiddenConn.ID, Content: "secret"},
		{ID: "55555555-5555-4555-8555-555555555555", Title: "Writer", ServiceID: conn.ID, Content: "old"},
	} {
		if err := h.Store.CreateDoc(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	userID := apitest.NewUser(t, h.Store, "viewer")
	apitest.GrantConnectorRole(t, h.Store, userID, conn.ID, "operator")
	requestBody := `{"content":"[[Visible Target|See it]] [[Secret Target]] [[Repeated]] [[Missing]]"}`
	req := httptest.NewRequest(http.MethodPut, "/api/docs/55555555-5555-4555-8555-555555555555", strings.NewReader(requestBody))
	req = req.WithContext(auth.ContextWithUser(req.Context(), userID, false))
	req.SetPathValue("id", "55555555-5555-4555-8555-555555555555")
	rr := httptest.NewRecorder()
	h.Save(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Save status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var response struct {
		Content      string   `json:"content"`
		LinkWarnings []string `json:"linkWarnings"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	wantContent := "[See it](/docs/11111111-1111-4111-8111-111111111111) [[Secret Target]] [[Repeated]] [[Missing]]"
	if response.Content != wantContent {
		t.Fatalf("saved content = %q, want %q", response.Content, wantContent)
	}
	if len(response.LinkWarnings) != 3 || !strings.Contains(strings.Join(response.LinkWarnings, " "), "ambiguous") || !strings.Contains(strings.Join(response.LinkWarnings, " "), "unresolved") {
		t.Fatalf("linkWarnings = %v, want ambiguity plus two unresolved links", response.LinkWarnings)
	}
	if strings.Contains(rr.Body.String(), "44444444-4444-4444-8444-444444444444") {
		t.Fatalf("response disclosed hidden target id: %s", rr.Body.String())
	}
}

func TestCreateResolvesContentAndReturnsLinkWarnings(t *testing.T) {
	h := newTestHandler(t)
	target := &store.DocRecord{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Title: "Setup", Content: "guide", Origin: store.DocOriginHuman}
	if err := h.Store.CreateDoc(context.Background(), target); err != nil {
		t.Fatal(err)
	}
	userID := apitest.NewUser(t, h.Store, "operator")
	body := `{"title":"New note","content":"See [[Setup]] and [[Unknown]]."}`
	req := httptest.NewRequest(http.MethodPost, "/api/docs", strings.NewReader(body))
	req = req.WithContext(auth.ContextWithUser(req.Context(), userID, true))
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Create status = %d, want 201: %s", rr.Code, rr.Body.String())
	}
	var response struct {
		Content      string   `json:"content"`
		LinkWarnings []string `json:"linkWarnings"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if want := "See [Setup](/docs/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa) and [[Unknown]]."; response.Content != want {
		t.Fatalf("created content = %q, want %q", response.Content, want)
	}
	if len(response.LinkWarnings) != 1 {
		t.Fatalf("linkWarnings = %v, want one unresolved warning", response.LinkWarnings)
	}
}

func TestDocBacklinksFiltersHiddenAndSoftDeletedSourcesAndHidesUnknownTarget(t *testing.T) {
	h := newTestHandler(t)
	ctx := context.Background()
	visible := &store.ConnectorRecord{Name: "visible", Category: "virtualization", Type: "test", URL: "https://visible.test"}
	hidden := &store.ConnectorRecord{Name: "hidden", Category: "virtualization", Type: "test", URL: "https://hidden.test"}
	for _, c := range []*store.ConnectorRecord{visible, hidden} {
		if err := h.Store.CreateConnector(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	targetID := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	if err := h.Store.CreateDoc(ctx, &store.DocRecord{ID: targetID, Title: "Target", Content: "target", Origin: store.DocOriginHuman}); err != nil {
		t.Fatal(err)
	}
	for _, d := range []*store.DocRecord{
		{ID: "visible", Title: "Visible source", ServiceID: visible.ID, Content: "[target](/docs/" + targetID + ")"},
		{ID: "hidden", Title: "Hidden source", ServiceID: hidden.ID, Content: "[target](/docs/" + targetID + ")"},
		{ID: "deleted", Title: "Deleted source", ServiceID: visible.ID, Content: "[target](/docs/" + targetID + ")"},
	} {
		if err := h.Store.CreateDoc(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.Store.SoftDeleteDoc(ctx, "deleted"); err != nil {
		t.Fatal(err)
	}
	userID := apitest.NewUser(t, h.Store, "viewer")
	apitest.GrantConnectorRole(t, h.Store, userID, visible.ID, "viewer")
	call := func(id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/docs/"+id+"/backlinks", nil)
		req = req.WithContext(auth.ContextWithUser(req.Context(), userID, false))
		req.SetPathValue("id", id)
		rr := httptest.NewRecorder()
		h.Backlinks(rr, req)
		return rr
	}
	rr := call(targetID)
	if rr.Code != http.StatusOK {
		t.Fatalf("Backlinks status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var got []store.DocBacklink
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != (store.DocBacklink{ID: "visible", Title: "Visible source"}) {
		t.Fatalf("backlinks = %+v, want only visible source", got)
	}
	if rr := call("cccccccc-cccc-4ccc-8ccc-cccccccccccc"); rr.Code != http.StatusNotFound {
		t.Fatalf("unknown target status = %d, want 404: %s", rr.Code, rr.Body.String())
	}
}

func TestRestoreVersionRebuildsDocLinkIndex(t *testing.T) {
	h := newTestHandler(t)
	ctx := context.Background()
	d := &store.DocRecord{ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", Title: "Restore target", Content: "[current](/docs/eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee)", Origin: store.DocOriginHuman}
	if err := h.Store.CreateDoc(ctx, d); err != nil {
		t.Fatal(err)
	}
	old := "[restored](/entities/ffffffff-ffff-4fff-8fff-ffffffffffff)"
	if err := h.Store.CreateDocVersion(ctx, &store.DocVersionRecord{DocID: d.ID, Rev: 1, Content: old, Trigger: "manual"}); err != nil {
		t.Fatal(err)
	}
	userID := apitest.NewUser(t, h.Store, "operator")
	req := httptest.NewRequest(http.MethodPost, "/api/docs/"+d.ID+"/versions/1/restore", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), userID, true))
	req.SetPathValue("id", d.ID)
	req.SetPathValue("rev", "1")
	rr := httptest.NewRecorder()
	h.Restore(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Restore status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	rows, err := h.Store.DB().QueryContext(ctx, `SELECT target_type, target_id FROM doc_links WHERE source_doc_id = ?`, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck
	var typ, id string
	if !rows.Next() {
		t.Fatal("restored document has no link index row")
	}
	if err := rows.Scan(&typ, &id); err != nil {
		t.Fatal(err)
	}
	if typ != "entity" || id != "ffffffff-ffff-4fff-8fff-ffffffffffff" || rows.Next() {
		t.Fatalf("restored index starts with %s/%s or has extra rows", typ, id)
	}
}
