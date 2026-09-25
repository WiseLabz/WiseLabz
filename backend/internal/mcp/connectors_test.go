package mcp

import (
	"context"
	"testing"
)

func TestListConnectors(t *testing.T) {
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

	t.Run("unrestricted read sees every granted connector", func(t *testing.T) {
		var out struct {
			Connectors []connectorSummary `json:"connectors"`
			Total      int                `json:"total"`
		}
		h.callTool(userCtx(userID), t, "list_connectors", nil, &out)
		if out.Total != 2 || len(out.Connectors) != 2 {
			t.Fatalf("got total=%d len=%d, want 2/2", out.Total, len(out.Connectors))
		}
	})

	t.Run("connector-restricted key only sees its allow-listed connector", func(t *testing.T) {
		var out struct {
			Connectors []connectorSummary `json:"connectors"`
			Total      int                `json:"total"`
		}
		h.callTool(restrictedCtx(userID, []string{c1}), t, "list_connectors", nil, &out)
		if len(out.Connectors) != 1 || out.Connectors[0].ID != c1 {
			t.Fatalf("got %+v, want only %s", out.Connectors, c1)
		}
	})

	t.Run("pagination via offset", func(t *testing.T) {
		var page1, page2 struct {
			Connectors []connectorSummary `json:"connectors"`
			Total      int                `json:"total"`
			Offset     int                `json:"offset"`
		}
		h.callTool(userCtx(userID), t, "list_connectors", map[string]any{"offset": float64(0)}, &page1)
		h.callTool(userCtx(userID), t, "list_connectors", map[string]any{"offset": float64(1)}, &page2)
		if len(page1.Connectors) == 0 || len(page2.Connectors) == 0 {
			t.Fatalf("expected both pages non-empty: page1=%+v page2=%+v", page1, page2)
		}
		if page1.Connectors[0].ID == page2.Connectors[0].ID {
			t.Fatalf("expected offset to advance the page, got same connector %s twice", page1.Connectors[0].ID)
		}
	})
}
