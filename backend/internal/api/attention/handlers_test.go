package attention

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

func TestList(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/api/attention", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestListWithDaysFilter(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/api/attention?days=7&page=1&pageSize=10", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

// TestListCacheSeparatesRestrictedKeys: a connector-restricted API key must
// not be served its owner's cached unrestricted page (#527).
func TestListCacheSeparatesRestrictedKeys(t *testing.T) {
	ctx := context.Background()
	s := apitest.NewStore(t)
	h := NewHandler(s)
	userID := apitest.NewUser(t, s, "viewer")
	var connIDs []string
	for _, name := range []string{"a", "b"} {
		c := &store.ConnectorRecord{Name: name, Category: "networking", Type: "generic", Enabled: true}
		if err := s.CreateConnector(ctx, c); err != nil {
			t.Fatal(err)
		}
		apitest.GrantConnectorRole(t, s, userID, c.ID, "viewer")
		if err := s.CreateAlert(ctx, &store.AlertRecord{ChangeID: "c-" + name, ServiceID: c.ID, Severity: "critical", Title: name, Status: "pending"}); err != nil {
			t.Fatal(err)
		}
		connIDs = append(connIDs, c.ID)
	}

	total := func(ctx context.Context) int {
		req := httptest.NewRequest(http.MethodGet, "/api/attention", nil).WithContext(auth.ContextWithUser(ctx, userID, false))
		rr := httptest.NewRecorder()
		h.List(rr, req)
		var resp struct {
			Total int `json:"total"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v; body=%s", err, rr.Body.String())
		}
		return resp.Total
	}
	if got := total(ctx); got != 2 {
		t.Fatalf("session total = %d, want 2", got)
	}
	keyCtx := auth.ContextWithAPIKeyRestriction(ctx, auth.APIKeyRestriction{ConnectorIDs: connIDs[:1]})
	if got := total(keyCtx); got != 1 {
		t.Fatalf("restricted key total = %d, want 1 (cache leaked the owner's page)", got)
	}
}
