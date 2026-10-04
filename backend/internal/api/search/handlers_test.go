package search

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
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
			return tx.ReplaceEntityIndexForConnector(ctx, c.ID, []connector.SnapshotEntity{{Kind: "device", Name: "router", IP: "10.0.0.1"}, {Kind: "device", Name: "router-extra", IP: "10.0.0.2"}})
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
		restricted                       bool
	}{
		{"q=router", 1, 1, 2, 200, false},
		{"q=router&type=doc", 1, 0, 0, 200, false},
		{"q=router&type=runbook", 0, 1, 0, 200, false},
		{"q=router&type=entity", 0, 0, 2, 200, false},
		{"q=router&connector=" + a.ID, 1, 0, 2, 200, false},
		{"q=router&connector=" + b.ID, 0, 0, 0, 200, false},
		{"q=router&kind=device", 0, 0, 2, 200, false},
		{"q=router&kind=vm", 0, 0, 0, 200, false},
		{"q=" + url.QueryEscape("  "), 0, 0, 0, 200, false},
		{"q=router&limit=1", 1, 1, 1, 200, false},
		{"q=router&type=bogus", 0, 0, 0, 400, false},
		{"q=router&limit=0", 0, 0, 0, 400, false},
		{"q=router&limit=101", 0, 0, 0, 400, false},
		{"q=router&limit=no", 0, 0, 0, 400, false},
		{"q=" + strings.Repeat("a", 501), 0, 0, 0, 400, false},
		{"q=router&connector=" + strings.Repeat("a", 129), 0, 0, 0, 400, false},
		{"q=router&kind=" + strings.Repeat("a", 101), 0, 0, 0, 400, false},
		{"q=router", 0, 1, 0, 200, true},
		{"q=router&connector=" + a.ID, 0, 0, 0, 200, true},
		{"q=router&kind=DEVICE", 0, 0, 2, 200, false},
	} {
		t.Run(tc.query, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/search?"+tc.query, nil)
			r = r.WithContext(auth.ContextWithUser(r.Context(), user, true))
			if tc.restricted {
				r = r.WithContext(auth.ContextWithAPIKeyRestriction(r.Context(), auth.APIKeyRestriction{ConnectorIDs: []string{b.ID}}))
			}
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
