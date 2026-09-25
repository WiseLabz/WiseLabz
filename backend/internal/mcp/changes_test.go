package mcp

import (
	"context"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func seedChange(t *testing.T, s *store.Store, serviceID string) *store.ChangeRecord {
	t.Helper()
	c := &store.ChangeRecord{
		ServiceID: serviceID, ChangeType: "config", Severity: "info",
		Summary: "something changed", Diff: "[]", Status: "new", AffectedDocIDs: "[]",
	}
	if err := s.CreateChange(context.Background(), c); err != nil {
		t.Fatalf("seed change: %v", err)
	}
	return c
}

func TestListChanges(t *testing.T) {
	h := newTestHarness(t)
	userID := createUser(t, h.Store)
	c1 := createConnector(t, h.Store, "conn-1", "containers_paas")
	c2 := createConnector(t, h.Store, "conn-2", "virtualization")
	if _, err := h.Store.UpsertConnectorGrant(context.Background(), userID, c1, "viewer"); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if _, err := h.Store.UpsertConnectorGrant(context.Background(), userID, c2, "viewer"); err != nil {
		t.Fatalf("grant: %v", err)
	}
	ch1 := seedChange(t, h.Store, c1)
	seedChange(t, h.Store, c2)

	t.Run("unrestricted read sees changes on every granted connector", func(t *testing.T) {
		var out struct {
			Changes []changeSummary `json:"changes"`
		}
		h.callTool(userCtx(userID), t, "list_changes", nil, &out)
		if len(out.Changes) != 2 {
			t.Fatalf("got %d changes, want 2", len(out.Changes))
		}
	})

	t.Run("connector-restricted key filters out the disallowed connector's change", func(t *testing.T) {
		var out struct {
			Changes []changeSummary `json:"changes"`
		}
		h.callTool(restrictedCtx(userID, []string{c1}), t, "list_changes", nil, &out)
		if len(out.Changes) != 1 || out.Changes[0].ID != ch1.ID {
			t.Fatalf("got %+v, want only change %s", out.Changes, ch1.ID)
		}
	})

	t.Run("pagination via offset", func(t *testing.T) {
		var page1, page2 struct {
			Changes []changeSummary `json:"changes"`
		}
		h.callTool(userCtx(userID), t, "list_changes", map[string]any{"offset": float64(0)}, &page1)
		h.callTool(userCtx(userID), t, "list_changes", map[string]any{"offset": float64(1)}, &page2)
		if len(page1.Changes) == 0 || len(page2.Changes) == 0 {
			t.Fatalf("expected both pages non-empty")
		}
		if page1.Changes[0].ID == page2.Changes[0].ID {
			t.Fatalf("expected offset to advance the page")
		}
	})
}
