package search

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestSearch(t *testing.T) {
	s := apitest.NewStore(t)
	h := Handler{Store: s}
	ctx := context.Background()
	user := apitest.NewUser(t, s, "viewer")
	a := store.ConnectorRecord{Name: "Router", Category: "networking", Type: "unifi", URL: "https://a.test"}
	b := store.ConnectorRecord{Name: "Hidden", Category: "networking", Type: "unifi", URL: "https://b.test"}
	for _, c := range []*store.ConnectorRecord{&a, &b} {
		if err := s.CreateConnector(ctx, c); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateDoc(ctx, &store.DocRecord{ID: "doc-" + c.ID, Title: "Router", Kind: "service", ServiceID: c.ID, Content: "router gateway"}); err != nil {
			t.Fatal(err)
		}
		if err := s.WithinTransaction(ctx, func(tx *store.Store) error {
			return tx.ReplaceEntityIndexForConnector(ctx, c.ID, []connector.SnapshotEntity{{Kind: "device", Name: "router", IP: "10.0.0.1"}})
		}); err != nil {
			t.Fatal(err)
		}
	}
	apitest.GrantConnectorRole(t, s, user, a.ID, "viewer")
	// Global runbooks return their body snippet unchanged; connector-bound steps
	// are not part of search results (matching the MCP contract).
	if _, err := s.CreateRunbook(ctx, &store.RunbookRecord{ID: "rb", Title: "Router recovery", Body: "router restart instructions", TargetType: "alert_severity", TargetValue: "critical"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReplaceRunbookSteps(ctx, "rb", []*store.RunbookStepRecord{{Position: 0, Title: "secret step", ConnectorID: b.ID, Verb: "restart", EntityRef: "secret"}}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		query                            string
		docs, runbooks, entities, status int
	}{
		{"q=router", 1, 1, 1, 200},
		{"q=router&type=doc", 1, 0, 0, 200},
		{"q=router&type=runbook", 0, 1, 0, 200},
		{"q=router&type=entity", 0, 0, 1, 200},
		{"q=router&connector=" + a.ID, 1, 0, 1, 200},
		{"q=router&connector=" + b.ID, 0, 0, 0, 200},
		{"q=router&kind=device", 0, 0, 1, 200},
		{"q=router&kind=vm", 0, 0, 0, 200},
		{"q=" + url.QueryEscape("  "), 0, 0, 0, 200},
		{"q=router&limit=1", 1, 1, 1, 200},
		{"q=router&type=bogus", 0, 0, 0, 400},
		{"q=router&limit=0", 0, 0, 0, 400},
		{"q=router&limit=101", 0, 0, 0, 400},
		{"q=router&limit=no", 0, 0, 0, 400},
	} {
		t.Run(tc.query, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/search?"+tc.query, nil)
			r = r.WithContext(auth.ContextWithUser(r.Context(), user, true))
			w := httptest.NewRecorder()
			h.List(w, r)
			if w.Code != tc.status {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if tc.status != 200 {
				return
			}
			var result store.SearchResults
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Docs == nil || result.Runbooks == nil || result.Entities == nil {
				t.Fatalf("null groups: %s", w.Body.String())
			}
			if len(result.Docs) != tc.docs || len(result.Runbooks) != tc.runbooks || len(result.Entities) != tc.entities {
				t.Fatalf("results %+v", result)
			}
			if len(result.Runbooks) > 0 && result.Runbooks[0].Snippet != "router restart instructions" {
				t.Fatalf("runbook snippet: %+v", result.Runbooks)
			}
		})
	}
}
