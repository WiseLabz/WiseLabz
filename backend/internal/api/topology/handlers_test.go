package topology

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestGraphScopesEdgesAndUnlinkedMembers(t *testing.T) {
	ctx := context.Background()
	s := apitest.NewStore(t)
	userID := apitest.NewUser(t, s, "viewer")
	visible := &store.ConnectorRecord{Name: "visible", Category: "networking", Type: "test", URL: "https://visible.test"}
	hidden := &store.ConnectorRecord{Name: "hidden-secret", Category: "networking", Type: "test", URL: "https://hidden.test"}
	for _, c := range []*store.ConnectorRecord{visible, hidden} {
		if err := s.CreateConnector(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.UpsertConnectorGrant(ctx, userID, visible.ID, "viewer"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceTopologyEdgesForConnector(ctx, visible.ID, []store.TopologyEdge{{SrcConnectorID: visible.ID, SrcKind: "service", SrcName: "visible", SrcRef: visible.ID, DstConnectorID: hidden.ID, DstKind: "vm", DstName: "hidden-secret-vm", DstRef: "42", Kind: store.TopologyEdgeRunsOn}}); err != nil {
		t.Fatal(err)
	}
	for _, row := range [][5]string{{"identity-visible", visible.ID, "vm", "v1", "visible-vm"}, {"identity-hidden", hidden.ID, "vm", "h1", "hidden-secret-vm"}} {
		if _, err := s.DB().ExecContext(ctx, `INSERT INTO entities(id,kind,display_name,first_seen_at,last_seen_at) VALUES(?,?,?,?,?)`, row[0], row[2], row[4], "2026-01-01", "2026-01-01"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.DB().ExecContext(ctx, `INSERT INTO entity_members(entity_id,connector_id,kind,ref,name,gone_at) VALUES(?,?,?,?,?,NULL)`, row[0], row[1], row[2], row[3], row[4]); err != nil {
			t.Fatal(err)
		}
	}
	h := &Handler{Store: s}
	request := func(query string) string {
		req := httptest.NewRequest("GET", "/api/topology/graph"+query, nil)
		req = req.WithContext(auth.ContextWithUser(req.Context(), userID, false))
		rr := httptest.NewRecorder()
		h.Graph(rr, req)
		if rr.Code != 200 {
			t.Fatalf("Graph(%s) status=%d body=%s", query, rr.Code, rr.Body.String())
		}
		return rr.Body.String()
	}
	defaultBody := request("")
	if strings.Contains(defaultBody, "identity-visible") || strings.Contains(defaultBody, "hidden-secret") {
		t.Fatalf("default graph=%s; expected linked visible graph only", defaultBody)
	}
	allBody := request("?includeUnlinked=true")
	if !strings.Contains(allBody, "identity-visible") || strings.Contains(allBody, "identity-hidden") || strings.Contains(allBody, "hidden-secret") {
		t.Fatalf("unlinked graph leaked or omitted members: %s", allBody)
	}
}
