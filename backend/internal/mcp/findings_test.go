package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func seedFinding(t *testing.T, s *store.Store, connectorID string) *store.QualityFindingRecord {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	f := &store.QualityFindingRecord{
		ConnectorID: connectorID, CheckType: "ownership_incomplete", Severity: "warning",
		Title: "Connector has no owner", Description: "Assign an owner",
		FirstDetectedAt: now, LastSeenAt: now,
	}
	if err := s.UpsertQualityFinding(context.Background(), f); err != nil {
		t.Fatalf("upsert finding: %v", err)
	}
	return f
}

func TestListFindings(t *testing.T) {
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
	f1 := seedFinding(t, h.Store, c1)
	f2 := seedFinding(t, h.Store, c2)

	t.Run("unrestricted read sees findings on every granted connector", func(t *testing.T) {
		var out struct {
			Findings []findingSummary `json:"findings"`
		}
		h.callTool(userCtx(userID), t, "list_findings", nil, &out)
		if len(out.Findings) != 2 {
			t.Fatalf("got %d findings, want 2", len(out.Findings))
		}
	})

	t.Run("connector-restricted key filters out the disallowed connector's finding", func(t *testing.T) {
		var out struct {
			Findings []findingSummary `json:"findings"`
			Total    int              `json:"total"`
		}
		h.callTool(restrictedCtx(userID, []string{c1}), t, "list_findings", nil, &out)
		if len(out.Findings) != 1 || out.Findings[0].ID != f1.ID {
			t.Fatalf("got %+v, want only finding %s", out.Findings, f1.ID)
		}
		// Total reflects the unfiltered store page (matches GET /api/findings'
		// pagination semantics); only the returned items are RBAC-filtered.
		if out.Total != 2 {
			t.Fatalf("total = %d, want 2", out.Total)
		}
	})

	t.Run("pagination via offset", func(t *testing.T) {
		var page1, page2 struct {
			Findings []findingSummary `json:"findings"`
		}
		h.callTool(userCtx(userID), t, "list_findings", map[string]any{"offset": float64(0)}, &page1)
		h.callTool(userCtx(userID), t, "list_findings", map[string]any{"offset": float64(1)}, &page2)
		if len(page1.Findings) == 0 || len(page2.Findings) == 0 {
			t.Fatalf("expected both pages non-empty")
		}
		if page1.Findings[0].ID == page2.Findings[0].ID {
			t.Fatalf("expected offset to advance the page")
		}
	})

	_ = f2
}
