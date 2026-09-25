package mcp

import (
	"context"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func seedAlert(t *testing.T, s *store.Store, serviceID string) *store.AlertRecord {
	t.Helper()
	a := &store.AlertRecord{ServiceID: serviceID, Severity: "critical", Title: "alert", Description: "desc", Status: "pending"}
	if err := s.CreateAlert(context.Background(), a); err != nil {
		t.Fatalf("seed alert: %v", err)
	}
	return a
}

func TestListAttentionItems(t *testing.T) {
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
	a1 := seedAlert(t, h.Store, c1)
	seedAlert(t, h.Store, c2)

	t.Run("unrestricted read sees pending alerts on every granted connector", func(t *testing.T) {
		var out struct {
			Items []store.AttentionItem `json:"items"`
		}
		h.callTool(userCtx(userID), t, "list_attention_items", nil, &out)
		if len(out.Items) != 2 {
			t.Fatalf("got %d items, want 2", len(out.Items))
		}
	})

	t.Run("connector-restricted key only sees its allow-listed connector's alert", func(t *testing.T) {
		// MergedAttentionItems applies the API-key ConnectorIDs restriction
		// internally (apiKeyConnectorFilter), so unlike list_findings/
		// list_changes there is no separate handler-side filter pass here.
		var out struct {
			Items []store.AttentionItem `json:"items"`
			Total int                   `json:"total"`
		}
		h.callTool(restrictedCtx(userID, []string{c1}), t, "list_attention_items", nil, &out)
		if out.Total != 1 || len(out.Items) != 1 || out.Items[0].ID != a1.ID {
			t.Fatalf("got %+v (total=%d), want only alert %s", out.Items, out.Total, a1.ID)
		}
	})

	t.Run("pagination via offset", func(t *testing.T) {
		var page1, page2 struct {
			Items []store.AttentionItem `json:"items"`
		}
		h.callTool(userCtx(userID), t, "list_attention_items", map[string]any{"offset": float64(0)}, &page1)
		h.callTool(userCtx(userID), t, "list_attention_items", map[string]any{"offset": float64(1)}, &page2)
		if len(page1.Items) == 0 || len(page2.Items) == 0 {
			t.Fatalf("expected both pages non-empty")
		}
		if page1.Items[0].ID == page2.Items[0].ID {
			t.Fatalf("expected offset to advance the page")
		}
	})
}
