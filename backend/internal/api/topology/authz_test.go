package topology

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// fixture: viewer sees connectors "pve" and "net"; "secret" is hidden.
//
//	vm-a (pve) --runs_on--> node-1 (net)      visible
//	vm-a (pve) --same_as--> vm-hidden (secret) hidden
//	vm-hidden (secret) --same_as--> node-9 (net) hidden hop between visible ends
//	node-1 (net) --dependency--> uplink (net)
type fixture struct {
	s                 *store.Store
	h                 *Handler
	viewer, adminless string
	pve, net, secret  string
}

type edgeSpec struct{ src, dst, srcKind, srcName, dstKind, dstName, kind string }

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	s := apitest.NewStore(t)
	f := &fixture{s: s, h: &Handler{Store: s}}
	f.viewer = apitest.NewUser(t, s, "viewer")
	f.adminless = apitest.NewUser(t, s, "operator")
	mk := func(name string) string {
		c := &store.ConnectorRecord{Name: name, Category: "networking", Type: "test", URL: "https://" + name + ".test"}
		if err := s.CreateConnector(ctx, c); err != nil {
			t.Fatal(err)
		}
		return c.ID
	}
	f.pve, f.net, f.secret = mk("pve"), mk("net"), mk("secret")
	for _, id := range []string{f.pve, f.net} {
		if _, err := s.UpsertConnectorGrant(ctx, f.viewer, id, "viewer"); err != nil {
			t.Fatal(err)
		}
	}
	f.replace(t, f.pve, []edgeSpec{
		{f.pve, f.net, "vm", "vm-a", "node", "node-1", store.TopologyEdgeRunsOn},
		{f.pve, f.secret, "vm", "vm-a", "vm", "vm-hidden", store.TopologyEdgeSameAs},
	})
	f.replace(t, f.net, []edgeSpec{
		{f.net, f.net, "node", "node-1", "network", "uplink", store.TopologyEdgeDependency},
	})
	f.replace(t, f.secret, []edgeSpec{
		{f.secret, f.net, "vm", "vm-hidden", "node", "node-9", store.TopologyEdgeSameAs},
	})
	return f
}

func (f *fixture) replace(t *testing.T, owner string, specs []edgeSpec) {
	t.Helper()
	edges := make([]store.TopologyEdge, len(specs))
	for i, e := range specs {
		edges[i] = store.TopologyEdge{
			SrcConnectorID: e.src, SrcKind: e.srcKind, SrcName: e.srcName, SrcRef: e.srcName,
			DstConnectorID: e.dst, DstKind: e.dstKind, DstName: e.dstName, DstRef: e.dstName, Kind: e.kind, Source: "test",
		}
	}
	if err := f.s.ReplaceTopologyEdgesForConnector(context.Background(), owner, edges); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) identity(t *testing.T, id, display string, merged string, members ...[5]string) {
	t.Helper()
	ctx := context.Background()
	var mergedInto any
	if merged != "" {
		mergedInto = merged
	}
	if _, err := f.s.DB().ExecContext(ctx, `INSERT INTO entities(id,kind,display_name,first_seen_at,last_seen_at,merged_into) VALUES(?,?,?,?,?,?)`, id, "vm", display, "2026-01-01", "2026-01-01", mergedInto); err != nil {
		t.Fatal(err)
	}
	for _, m := range members { // connector, kind, ref, name, gone_at ("" = active)
		var gone any
		if m[4] != "" {
			gone = m[4]
		}
		if _, err := f.s.DB().ExecContext(ctx, `INSERT INTO entity_members(entity_id,connector_id,kind,ref,name,gone_at) VALUES(?,?,?,?,?,?)`, id, m[0], m[1], m[2], m[3], gone); err != nil {
			t.Fatal(err)
		}
	}
}

type call struct {
	userID string
	admin  bool
	keyIDs []string
}

func (c call) ctx(r *http.Request) *http.Request {
	ctx := auth.ContextWithUser(r.Context(), c.userID, c.admin)
	if c.keyIDs != nil {
		ctx = auth.ContextWithAPIKeyRestriction(ctx, auth.APIKeyRestriction{ConnectorIDs: c.keyIDs})
	}
	return r.WithContext(ctx)
}

func (f *fixture) get(t *testing.T, c call, h http.HandlerFunc, path string, params url.Values) (int, string) {
	t.Helper()
	target := path
	if len(params) > 0 {
		target += "?" + params.Encode()
	}
	rr := httptest.NewRecorder()
	h(rr, c.ctx(httptest.NewRequest(http.MethodGet, target, nil)))
	return rr.Code, strings.TrimSpace(rr.Body.String())
}

func (f *fixture) path(t *testing.T, c call, params url.Values) (int, string) {
	return f.get(t, c, f.h.Path, "/api/topology/path", params)
}

func (f *fixture) graph(t *testing.T, c call, params url.Values) (int, string) {
	return f.get(t, c, f.h.Graph, "/api/topology/graph", params)
}

type pathBody struct {
	Found     bool `json:"found"`
	Hops      int  `json:"hops"`
	Truncated bool `json:"truncated"`
	Path      []struct {
		ConnectorID   string `json:"connectorId"`
		ConnectorName string `json:"connectorName"`
		Name          string `json:"name"`
		EdgeKind      string `json:"edgeKind"`
	} `json:"path"`
}

func decodePath(t *testing.T, body string) pathBody {
	t.Helper()
	var out pathBody
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	return out
}

type graphBody struct {
	Nodes []struct {
		ID, Type, Name, Kind string
		ConnectorID          string `json:"connectorId"`
	} `json:"nodes"`
	Edges []struct {
		ID, Source, Target, Kind, Detail string
	} `json:"edges"`
	Truncated bool `json:"truncated"`
}

func decodeGraph(t *testing.T, body string) graphBody {
	t.Helper()
	var out graphBody
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	return out
}

func q(kv ...string) url.Values {
	v := url.Values{}
	for i := 0; i < len(kv); i += 2 {
		v.Set(kv[i], kv[i+1])
	}
	return v
}

func TestPathValidation(t *testing.T) {
	f := newFixture(t)
	viewer := call{userID: f.viewer}
	for name, params := range map[string]url.Values{
		"missing from": q("to", "node-1"),
		"blank from":   q("from", "   "),
		"long from":    q("from", strings.Repeat("a", 257)),
		"long to":      q("from", "vm-a", "to", strings.Repeat("a", 257)),
	} {
		if code, body := f.path(t, viewer, params); code != 400 {
			t.Errorf("%s: status %d body %s, want 400", name, code, body)
		}
	}
	if code, body := f.path(t, viewer, q("from", strings.Repeat("a", 256))); code != 200 {
		t.Errorf("256-char from: status %d body %s, want 200", code, body)
	}
}

func TestPathVisibleRoute(t *testing.T) {
	f := newFixture(t)
	code, body := f.path(t, call{userID: f.viewer}, q("from", "VM-A", "to", "uplink"))
	if code != 200 {
		t.Fatalf("status %d body %s", code, body)
	}
	p := decodePath(t, body)
	if !p.Found || p.Hops != 2 || len(p.Path) != 3 || p.Path[0].Name != "vm-a" || p.Path[1].Name != "node-1" || p.Path[2].Name != "uplink" {
		t.Fatalf("path = %s", body)
	}
	if p.Path[0].ConnectorName != "pve" || p.Path[1].ConnectorName != "net" || p.Path[1].EdgeKind != store.TopologyEdgeRunsOn {
		t.Fatalf("path details = %s", body)
	}
}

func TestPathDirectedWalkWithoutTo(t *testing.T) {
	f := newFixture(t)
	_, body := f.path(t, call{userID: f.viewer}, q("from", "vm-a"))
	p := decodePath(t, body)
	var got []string
	for _, s := range p.Path {
		got = append(got, s.Name)
	}
	if !p.Found || strings.Join(got, ",") != "vm-a,node-1,uplink" || p.Truncated {
		t.Fatalf("walk = %s", body)
	}
	// Edges are only followed forward.
	_, body = f.path(t, call{userID: f.viewer}, q("from", "uplink"))
	if p := decodePath(t, body); len(p.Path) != 1 {
		t.Fatalf("reverse walk = %s", body)
	}
}

func TestPathHiddenConnectorLooksLikeNoPath(t *testing.T) {
	f := newFixture(t)
	// node-9 is only reachable through the hidden connector's edges, so it is
	// unreachable for the viewer; the answer must equal that for a name that
	// exists nowhere.
	viewer := call{userID: f.viewer}
	codeHidden, hidden := f.path(t, viewer, q("from", "vm-a", "to", "node-9"))
	codeNone, none := f.path(t, viewer, q("from", "vm-a", "to", "no-such-thing"))
	if codeHidden != 200 || codeNone != 200 || hidden != none {
		t.Fatalf("hidden route = %d %s, nonexistent = %d %s; must be identical", codeHidden, hidden, codeNone, none)
	}
	if p := decodePath(t, hidden); p.Found || len(p.Path) != 0 {
		t.Fatalf("hidden route found: %s", hidden)
	}
	// Starting at a hidden entity finds nothing either, and names never leak.
	_, body := f.path(t, viewer, q("from", "vm-hidden"))
	if strings.Contains(body, "vm-hidden") && decodePath(t, body).Found {
		t.Fatalf("hidden start found: %s", body)
	}
	for _, name := range []string{"vm-hidden", "secret"} {
		_, body := f.path(t, viewer, q("from", "vm-a"))
		if strings.Contains(body, name) {
			t.Fatalf("walk leaked %q: %s", name, body)
		}
	}
}

func TestPathAPIKeyConnectorRestriction(t *testing.T) {
	f := newFixture(t)
	params := q("from", "vm-a", "to", "uplink")
	if _, body := f.path(t, call{userID: f.viewer, keyIDs: []string{f.pve}}, params); decodePath(t, body).Found {
		t.Fatalf("key limited to pve found a path through net: %s", body)
	}
	if _, body := f.path(t, call{userID: f.viewer, keyIDs: []string{f.pve, f.net}}, params); !decodePath(t, body).Found {
		t.Fatalf("key covering both connectors lost the path: %s", body)
	}
	// A key naming a connector the user cannot view does not widen access.
	if _, body := f.path(t, call{userID: f.viewer, keyIDs: []string{f.secret}}, q("from", "vm-a")); decodePath(t, body).Found {
		t.Fatalf("key for a hidden connector widened access: %s", body)
	}
}

func TestPathInstanceAdminWithoutGrantsSeesNothing(t *testing.T) {
	f := newFixture(t)
	code, body := f.path(t, call{userID: f.adminless, admin: true}, q("from", "vm-a", "to", "uplink"))
	if code != 200 || decodePath(t, body).Found {
		t.Fatalf("admin without grants: %d %s, want no path (access is per connector grant)", code, body)
	}
}

func TestPathWalkIsCapped(t *testing.T) {
	f := newFixture(t)
	specs := make([]edgeSpec, 0, maxPathSteps+10)
	for i := 0; i < maxPathSteps+10; i++ {
		specs = append(specs, edgeSpec{f.net, f.net, "x", fmt.Sprintf("n%05d", i), "x", fmt.Sprintf("n%05d", i+1), store.TopologyEdgeRunsOn})
	}
	f.replace(t, f.net, specs)
	_, body := f.path(t, call{userID: f.viewer}, q("from", "n00000"))
	p := decodePath(t, body)
	if len(p.Path) != maxPathSteps || !p.Truncated {
		t.Fatalf("walk returned %d steps truncated=%v, want %d truncated", len(p.Path), p.Truncated, maxPathSteps)
	}
}

func TestGraphValidationAndFilters(t *testing.T) {
	f := newFixture(t)
	viewer := call{userID: f.viewer}
	if code, body := f.graph(t, viewer, q("includeUnlinked", "notabool")); code != 400 {
		t.Fatalf("includeUnlinked=notabool: %d %s, want 400", code, body)
	}

	// A hidden connector in the connector filter equals a nonexistent one.
	codeH, hidden := f.graph(t, viewer, q("connector", "secret"))
	codeHID, hiddenID := f.graph(t, viewer, q("connector", f.secret))
	codeN, none := f.graph(t, viewer, q("connector", "no-such-connector"))
	if codeH != 200 || codeHID != 200 || codeN != 200 || hidden != none || hiddenID != none {
		t.Fatalf("hidden=%d %s / %d %s, nonexistent=%d %s; must be identical", codeH, hidden, codeHID, hiddenID, codeN, none)
	}
	if g := decodeGraph(t, none); len(g.Nodes) != 0 || len(g.Edges) != 0 || g.Truncated {
		t.Fatalf("empty graph = %s", none)
	}

	// A visible connector filter works by name and by ID and narrows the edges.
	for _, key := range []string{"pve", f.pve} {
		_, body := f.graph(t, viewer, q("connector", key))
		g := decodeGraph(t, body)
		if len(g.Edges) != 0 {
			// pve's only visible edge goes to net, which the filter excludes.
			t.Fatalf("connector=%s edges = %s, want none (other endpoint filtered out)", key, body)
		}
	}
	_, body := f.graph(t, viewer, q("connector", "net"))
	if g := decodeGraph(t, body); len(g.Edges) != 1 || g.Edges[0].Kind != store.TopologyEdgeDependency {
		t.Fatalf("connector=net graph = %s", body)
	}

	// kind= restricts to matching edges and never widens to hidden ones.
	_, body = f.graph(t, viewer, q("kind", "vm"))
	g := decodeGraph(t, body)
	if len(g.Edges) != 1 || g.Edges[0].Kind != store.TopologyEdgeRunsOn {
		t.Fatalf("kind=vm graph = %s, want only the visible vm edge", body)
	}
	for _, leaked := range []string{"vm-hidden", "node-9", "secret", f.secret} {
		if strings.Contains(body, leaked) {
			t.Fatalf("kind=vm graph leaked %q: %s", leaked, body)
		}
	}
}

func TestGraphNeverReturnsHiddenConnectorData(t *testing.T) {
	f := newFixture(t)
	f.identity(t, "id-hidden", "vm-hidden", "", [5]string{f.secret, "vm", "vm-hidden", "vm-hidden", ""})
	for _, params := range []url.Values{nil, q("includeUnlinked", "true"), q("kind", "node")} {
		_, body := f.graph(t, call{userID: f.viewer}, params)
		for _, leaked := range []string{"vm-hidden", "node-9", "id-hidden", f.secret} {
			if strings.Contains(body, leaked) {
				t.Fatalf("graph(%v) leaked %q: %s", params, leaked, body)
			}
		}
	}
}

func TestGraphMixedIdentityExposesOnlyVisibleMembers(t *testing.T) {
	f := newFixture(t)
	// One identity seen by the visible pve connector and the hidden one; the
	// hidden member's name sorts first and must not become the display name.
	f.identity(t, "id-mixed", "ignored", "",
		[5]string{f.pve, "vm", "vm-a", "vm-a", ""},
		[5]string{f.secret, "vm", "vm-hidden", "aaa-hidden-name", ""})
	_, body := f.graph(t, call{userID: f.viewer}, nil)
	for _, leaked := range []string{"aaa-hidden-name", "vm-hidden", f.secret} {
		if strings.Contains(body, leaked) {
			t.Fatalf("mixed identity leaked %q: %s", leaked, body)
		}
	}
	g := decodeGraph(t, body)
	found := false
	for _, n := range g.Nodes {
		if n.ID == "id-mixed" {
			found = true
			if n.Type != "identity" || n.Name != "vm-a" {
				t.Fatalf("mixed identity node = %+v, want identity named after the visible member", n)
			}
		}
	}
	if !found {
		t.Fatalf("visible member did not resolve to its identity: %s", body)
	}
}

func TestGraphMergedAndGoneIdentities(t *testing.T) {
	f := newFixture(t)
	f.identity(t, "id-survivor", "survivor", "")
	// Merged away: its member rows are no longer authoritative.
	f.identity(t, "id-merged", "merged", "id-survivor", [5]string{f.pve, "vm", "vm-a", "vm-a", ""})
	// Gone member of a live identity.
	f.identity(t, "id-gone", "gone", "", [5]string{f.net, "node", "node-1", "node-1", "2026-02-01"})
	for _, params := range []url.Values{nil, q("includeUnlinked", "true")} {
		_, body := f.graph(t, call{userID: f.viewer}, params)
		for _, id := range []string{"id-merged", "id-gone", "id-survivor"} {
			if strings.Contains(body, id) {
				t.Fatalf("graph(%v) exposed %s: %s", params, id, body)
			}
		}
		g := decodeGraph(t, body)
		if len(g.Edges) == 0 {
			t.Fatalf("edges whose members are merged or gone must stay as plain nodes: %s", body)
		}
		for _, n := range g.Nodes {
			if n.Type == "identity" {
				t.Fatalf("unexpected identity node %+v in %s", n, body)
			}
		}
	}
}

func TestGraphAPIKeyConnectorRestriction(t *testing.T) {
	f := newFixture(t)
	f.identity(t, "id-net-only", "net-vm", "", [5]string{f.net, "vm", "net-vm", "net-vm", ""})
	// Key limited to net: the pve<->net edge disappears, the net-internal one stays.
	_, body := f.graph(t, call{userID: f.viewer, keyIDs: []string{f.net}}, q("includeUnlinked", "true"))
	g := decodeGraph(t, body)
	if len(g.Edges) != 1 || g.Edges[0].Kind != store.TopologyEdgeDependency {
		t.Fatalf("restricted graph = %s", body)
	}
	if strings.Contains(body, "vm-a") {
		t.Fatalf("key limited to net saw pve data: %s", body)
	}
	// A connector filter naming a connector outside the key is empty.
	_, body = f.graph(t, call{userID: f.viewer, keyIDs: []string{f.net}}, q("connector", "pve"))
	if g := decodeGraph(t, body); len(g.Nodes) != 0 || len(g.Edges) != 0 {
		t.Fatalf("connector filter widened the key: %s", body)
	}
}

func TestGraphInstanceAdminWithoutGrantsSeesNothing(t *testing.T) {
	f := newFixture(t)
	code, body := f.graph(t, call{userID: f.adminless, admin: true}, q("includeUnlinked", "true"))
	if g := decodeGraph(t, body); code != 200 || len(g.Nodes) != 0 || len(g.Edges) != 0 {
		t.Fatalf("admin without grants: %d %s, want empty", code, body)
	}
}

func TestGraphCollapsesDuplicateEdges(t *testing.T) {
	f := newFixture(t)
	// The same logical contains edge written by two different owners.
	contains := edgeSpec{f.pve, f.pve, "service", "pve", "vm", "vm-a", store.TopologyEdgeContains}
	f.replace(t, f.pve, []edgeSpec{contains})
	f.replace(t, f.net, []edgeSpec{contains})
	_, body := f.graph(t, call{userID: f.viewer}, nil)
	n := 0
	for _, e := range decodeGraph(t, body).Edges {
		if e.Kind == store.TopologyEdgeContains {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("contains edges = %d, want 1 in %s", n, body)
	}
}

func TestGraphIsCapped(t *testing.T) {
	f := newFixture(t)

	// More nodes than the cap: a chain.
	chain := make([]edgeSpec, 0, maxGraphNodes+100)
	for i := 0; i < maxGraphNodes+100; i++ {
		chain = append(chain, edgeSpec{f.net, f.net, "x", fmt.Sprintf("n%05d", i), "x", fmt.Sprintf("n%05d", i+1), store.TopologyEdgeRunsOn})
	}
	f.replace(t, f.net, chain)
	_, body := f.graph(t, call{userID: f.viewer}, nil)
	g := decodeGraph(t, body)
	if len(g.Nodes) != maxGraphNodes || !g.Truncated {
		t.Fatalf("nodes = %d truncated=%v, want %d truncated", len(g.Nodes), g.Truncated, maxGraphNodes)
	}
	ids := map[string]bool{}
	for _, n := range g.Nodes {
		ids[n.ID] = true
	}
	for _, e := range g.Edges {
		if !ids[e.Source] || !ids[e.Target] {
			t.Fatalf("edge %+v references a node that was cut", e)
		}
	}

	// More edges than the cap between two nodes (distinct details).
	many := make([]store.TopologyEdge, 0, maxGraphEdges+50)
	for i := 0; i < maxGraphEdges+50; i++ {
		many = append(many, store.TopologyEdge{
			SrcConnectorID: f.net, SrcKind: "x", SrcName: "a", SrcRef: "a", DstConnectorID: f.net, DstKind: "x", DstName: "b", DstRef: "b",
			Kind: store.TopologyEdgeProxiesTo, Detail: fmt.Sprint(i),
		})
	}
	if err := f.s.ReplaceTopologyEdgesForConnector(context.Background(), f.net, many); err != nil {
		t.Fatal(err)
	}
	_, body = f.graph(t, call{userID: f.viewer}, nil)
	g = decodeGraph(t, body)
	if len(g.Edges) != maxGraphEdges || !g.Truncated {
		t.Fatalf("edges = %d truncated=%v, want %d truncated", len(g.Edges), g.Truncated, maxGraphEdges)
	}

	// A small graph is not marked truncated.
	f.replace(t, f.net, nil)
	_, body = f.graph(t, call{userID: f.viewer}, nil)
	if g := decodeGraph(t, body); g.Truncated {
		t.Fatalf("small graph marked truncated: %s", body)
	}
}
