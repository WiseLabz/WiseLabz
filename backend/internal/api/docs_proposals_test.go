package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestDocEditProposalsREST(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	ctx := context.Background()

	if err := app.Store.CreateConnector(ctx, &store.ConnectorRecord{
		ID: "svc-1", Name: "svc-1", Category: "networking", Type: "test", Enabled: true,
	}); err != nil {
		t.Fatalf("create connector: %v", err)
	}
	if err := app.Store.CreateDoc(ctx, &store.DocRecord{ID: "doc-1", Title: "Doc", Kind: "service", ServiceID: "svc-1", Content: "v1"}); err != nil {
		t.Fatalf("create doc: %v", err)
	}
	if err := app.Store.CreateDoc(ctx, &store.DocRecord{ID: "lab-1", Title: "Lab", Kind: "lab", Content: "lab v1"}); err != nil {
		t.Fatalf("create lab doc: %v", err)
	}

	authorID, _ := app.user(t, "viewer")
	opID, opToken := app.user(t, "operator") // instance admin
	app.connectorGrant(t, opID, "svc-1", "operator")
	viewerID, viewerToken := app.user(t, "viewer")
	app.connectorGrant(t, viewerID, "svc-1", "viewer")

	otherAuthorID, _ := app.user(t, "viewer")
	proposeAs := func(author, docID string, base int, content string) *store.DocEditProposal {
		t.Helper()
		p := &store.DocEditProposal{DocID: docID, BaseVersion: base, Content: content, AuthorID: author}
		if err := app.Store.CreateDocEditProposal(ctx, p); err != nil {
			t.Fatalf("create proposal: %v", err)
		}
		return p
	}
	propose := func(docID string, base int, content string) *store.DocEditProposal {
		t.Helper()
		return proposeAs(authorID, docID, base, content)
	}
	approve := func(token, id string) int {
		return app.req(t, http.MethodPost, "/api/docs/edit-proposals/"+id+"/approve", nil, token).Code
	}

	t.Run("list is limited to proposals the caller can review", func(t *testing.T) {
		propose("doc-1", 1, "listed")
		rec := app.req(t, http.MethodGet, "/api/docs/edit-proposals", nil, opToken)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d; body = %s", rec.Code, rec.Body)
		}
		var page struct {
			Items []store.DocEditProposal `json:"items"`
			Total int                     `json:"total"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if page.Total != 1 || page.Items[0].DocTitle != "Doc" {
			t.Fatalf("operator page = %+v", page)
		}

		rec = app.req(t, http.MethodGet, "/api/docs/edit-proposals", nil, viewerToken)
		_ = json.Unmarshal(rec.Body.Bytes(), &page)
		if rec.Code != http.StatusOK || page.Total != 0 {
			t.Fatalf("viewer page = %d %+v, want empty", rec.Code, page)
		}

		if rec := app.req(t, http.MethodGet, "/api/docs/edit-proposals?status=bogus", nil, opToken); rec.Code != http.StatusBadRequest {
			t.Fatalf("bad status = %d, want 400", rec.Code)
		}
	})

	t.Run("list omits content; get returns it to reviewers only", func(t *testing.T) {
		p := propose("doc-1", 1, "full body text")
		rec := app.req(t, http.MethodGet, "/api/docs/edit-proposals", nil, opToken)
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "full body text") {
			t.Fatalf("list leaked content or failed: %d %s", rec.Code, rec.Body)
		}
		rec = app.req(t, http.MethodGet, "/api/docs/edit-proposals/"+p.ID, nil, opToken)
		var got store.DocEditProposal
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got.Content != "full body text" {
			t.Fatalf("get = %d %s", rec.Code, rec.Body)
		}
		if rec := app.req(t, http.MethodGet, "/api/docs/edit-proposals/"+p.ID, nil, viewerToken); rec.Code != http.StatusNotFound {
			t.Fatalf("viewer get = %d, want 404", rec.Code)
		}
		if rec := app.req(t, http.MethodGet, "/api/docs/edit-proposals/missing", nil, opToken); rec.Code != http.StatusNotFound {
			t.Fatalf("missing get = %d, want 404", rec.Code)
		}
	})

	t.Run("pagination happens in SQL", func(t *testing.T) {
		for _, id := range []string{"pg-1", "pg-2", "pg-3"} {
			if err := app.Store.CreateDoc(ctx, &store.DocRecord{ID: id, Title: id, Kind: "service", ServiceID: "svc-1", Content: "v1"}); err != nil {
				t.Fatal(err)
			}
			propose(id, 1, "x")
		}
		var page struct {
			Items []store.DocEditProposal `json:"items"`
			Total int                     `json:"total"`
		}
		rec := app.req(t, http.MethodGet, "/api/docs/edit-proposals?page=2&pageSize=2", nil, opToken)
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &page) != nil {
			t.Fatalf("page 2 = %d %s", rec.Code, rec.Body)
		}
		if page.Total != 4 || len(page.Items) != 2 {
			t.Fatalf("page 2 = %d items of %d", len(page.Items), page.Total)
		}
	})

	t.Run("viewer cannot approve or reject", func(t *testing.T) {
		p := propose("doc-1", 1, "nope")
		if code := approve(viewerToken, p.ID); code != http.StatusNotFound {
			t.Fatalf("viewer approve = %d, want 404", code)
		}
		if rec := app.req(t, http.MethodPost, "/api/docs/edit-proposals/"+p.ID+"/reject", nil, viewerToken); rec.Code != http.StatusNotFound {
			t.Fatalf("viewer reject = %d, want 404", rec.Code)
		}
	})

	t.Run("approve applies, writes audit, then conflicts when stale", func(t *testing.T) {
		p := propose("doc-1", 1, "approved body")
		stale := proposeAs(otherAuthorID, "doc-1", 1, "stale body")
		if code := approve(opToken, p.ID); code != http.StatusOK {
			t.Fatalf("approve = %d, want 200", code)
		}
		d, _ := app.Store.GetDoc(ctx, "doc-1")
		if d.Content != "approved body" || d.CurrentVersion != 2 {
			t.Fatalf("doc = %+v", d)
		}
		audits, _, _ := app.Store.ListAuditRecords(ctx, "doc.edit_approved", "", "", "", 0, 10)
		if len(audits) != 1 {
			t.Fatalf("audits = %+v", audits)
		}
		if code := approve(opToken, p.ID); code != http.StatusConflict {
			t.Fatalf("re-approve = %d, want 409", code)
		}

		rec := app.req(t, http.MethodPost, "/api/docs/edit-proposals/"+stale.ID+"/approve", nil, opToken)
		if rec.Code != http.StatusConflict {
			t.Fatalf("stale approve = %d, want 409", rec.Code)
		}
		var current store.DocRecord
		if err := json.Unmarshal(rec.Body.Bytes(), &current); err != nil || current.CurrentVersion != 2 {
			t.Fatalf("conflict body = %s", rec.Body)
		}
		got, _ := app.Store.GetDocEditProposal(ctx, stale.ID)
		if got.Status != store.ProposalPending {
			t.Fatalf("stale proposal status = %s, want pending", got.Status)
		}
	})

	t.Run("reject leaves the doc unchanged", func(t *testing.T) {
		before, _ := app.Store.GetDoc(ctx, "doc-1")
		p := propose("doc-1", before.CurrentVersion, "rejected body")
		rec := app.req(t, http.MethodPost, "/api/docs/edit-proposals/"+p.ID+"/reject", nil, opToken)
		if rec.Code != http.StatusOK {
			t.Fatalf("reject = %d; body = %s", rec.Code, rec.Body)
		}
		after, _ := app.Store.GetDoc(ctx, "doc-1")
		if after.Content != before.Content || after.CurrentVersion != before.CurrentVersion {
			t.Fatalf("doc changed: %+v", after)
		}
	})

	t.Run("lab-wide docs need instance admin", func(t *testing.T) {
		p := propose("lab-1", 1, "lab edit")
		// A connector operator who is not an instance admin.
		nonAdminID, nonAdminToken := app.user(t, "viewer")
		app.connectorGrant(t, nonAdminID, "svc-1", "operator")
		if code := approve(nonAdminToken, p.ID); code != http.StatusNotFound {
			t.Fatalf("non-admin approve lab doc = %d, want 404", code)
		}
		if code := approve(opToken, p.ID); code != http.StatusOK {
			t.Fatalf("admin approve lab doc = %d, want 200", code)
		}
	})
}
