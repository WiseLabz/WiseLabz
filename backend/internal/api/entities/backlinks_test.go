package entities

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestEntityBacklinksIncludesMergedTargetsAndFiltersSources(t *testing.T) {
	s := apitest.NewStore(t)
	ctx := context.Background()
	visible := &store.ConnectorRecord{Name: "visible", Category: "virtualization", Type: "test", URL: "https://visible.test"}
	hidden := &store.ConnectorRecord{Name: "hidden", Category: "virtualization", Type: "test", URL: "https://hidden.test"}
	for _, c := range []*store.ConnectorRecord{visible, hidden} {
		if err := s.CreateConnector(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	headID := "11111111-1111-4111-8111-111111111111"
	mergedID := "22222222-2222-4222-8222-222222222222"
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO entities (id, kind, display_name, first_seen_at, last_seen_at, merged_into) VALUES (?, 'vm', 'Canonical', '2026-01-01', '2026-01-01', NULL)`, headID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO entities (id, kind, display_name, first_seen_at, last_seen_at, merged_into) VALUES (?, 'vm', 'Old name', '2026-01-01', '2026-01-01', ?)`, mergedID, headID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO entity_members (entity_id, connector_id, kind, ref, name, gone_at) VALUES (?, ?, 'vm', 'head-ref', 'Canonical', NULL)`, headID, visible.ID); err != nil {
		t.Fatal(err)
	}
	for _, d := range []*store.DocRecord{
		{ID: "visible-source", Title: "Visible source", ServiceID: visible.ID, Content: "[old name](/entities/" + mergedID + ")"},
		{ID: "hidden-source", Title: "Hidden source", ServiceID: hidden.ID, Content: "[old name](/entities/" + mergedID + ")"},
		{ID: "deleted-source", Title: "Deleted source", ServiceID: visible.ID, Content: "[old name](/entities/" + mergedID + ")"},
	} {
		if err := s.CreateDoc(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SoftDeleteDoc(ctx, "deleted-source"); err != nil {
		t.Fatal(err)
	}
	userID := apitest.NewUser(t, s, "viewer")
	apitest.GrantConnectorRole(t, s, userID, visible.ID, "viewer")
	h := NewHandler(s, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/entities/"+headID+"/backlinks", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), userID, false))
	req.SetPathValue("id", headID)
	rr := httptest.NewRecorder()
	h.Backlinks(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Backlinks status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var got []store.DocBacklink
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != (store.DocBacklink{ID: "visible-source", Title: "Visible source"}) {
		t.Fatalf("backlinks = %+v, want only the visible source linked to merged target", got)
	}

	// Requesting the old identity redirects to the canonical entity and returns
	// the same backlinks while the target itself remains visible.
	req = httptest.NewRequest(http.MethodGet, "/api/entities/"+mergedID+"/backlinks", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), userID, false))
	req.SetPathValue("id", mergedID)
	rr = httptest.NewRecorder()
	h.Backlinks(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("merged Backlinks status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
}

func TestEntityBacklinksReturnsNotFoundForUnknownOrHiddenTarget(t *testing.T) {
	s := apitest.NewStore(t)
	ctx := context.Background()
	conn := &store.ConnectorRecord{Name: "private", Category: "virtualization", Type: "test", URL: "https://private.test"}
	if err := s.CreateConnector(ctx, conn); err != nil {
		t.Fatal(err)
	}
	entityID := "33333333-3333-4333-8333-333333333333"
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO entities (id, kind, display_name, first_seen_at, last_seen_at) VALUES (?, 'vm', 'Private', '2026-01-01', '2026-01-01')`, entityID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO entity_members (entity_id, connector_id, kind, ref, name) VALUES (?, ?, 'vm', 'private-ref', 'Private')`, entityID, conn.ID); err != nil {
		t.Fatal(err)
	}
	userID := apitest.NewUser(t, s, "viewer")
	h := NewHandler(s, nil)
	for _, id := range []string{entityID, "44444444-4444-4444-8444-444444444444"} {
		req := httptest.NewRequest(http.MethodGet, "/api/entities/"+id+"/backlinks", nil)
		req = req.WithContext(auth.ContextWithUser(req.Context(), userID, false))
		req.SetPathValue("id", id)
		rr := httptest.NewRecorder()
		h.Backlinks(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("Backlinks(%s) status = %d, want 404: %s", id, rr.Code, rr.Body.String())
		}
	}
}
