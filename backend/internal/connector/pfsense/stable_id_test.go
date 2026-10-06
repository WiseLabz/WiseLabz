package pfsense

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestBuildInterfaceFallbackIdentityUsesIdentifierAndMAC(t *testing.T) {
	first := []byte(`{"data":[
		{"id":"wan","if":"","mac":"00:11:22:33:44:55","ipaddr":"203.0.113.5"},
		{"id":"lan","if":"","mac":"00:11:22:33:44:66","ipaddr":"10.0.0.1"}
	]}`)
	second := []byte(`{"data":[
		{"id":"lan","if":"","mac":"00:11:22:33:44:66","ipaddr":"10.0.0.1"},
		{"id":"wan","if":"","mac":"00:11:22:33:44:55","ipaddr":"203.0.113.5"}
	]}`)
	_, before := buildInterfaceTable(first)
	_, after := buildInterfaceTable(second)
	if len(before) != 2 || len(after) != 2 {
		t.Fatalf("interface entity counts = %d and %d, want 2 each", len(before), len(after))
	}
	if before[0].ExternalID == "" || before[0].ExternalID == before[1].ExternalID {
		t.Fatalf("fallback interface IDs = %q, %q; want non-empty distinct IDs", before[0].ExternalID, before[1].ExternalID)
	}
	if before[0].ExternalID != after[1].ExternalID || before[1].ExternalID != after[0].ExternalID {
		t.Errorf("interface IDs changed after reorder: before=%q,%q after=%q,%q",
			before[0].ExternalID, before[1].ExternalID, after[0].ExternalID, after[1].ExternalID)
	}
}

func TestBuildInterfaceAnonymousFallbackGetsOccurrenceIDs(t *testing.T) {
	_, entities := buildInterfaceTable([]byte(`{"data":[{},{}]}`))
	if len(entities) != 2 || entities[0].ExternalID == "" || entities[1].ExternalID == "" {
		t.Fatalf("anonymous interface entities = %+v; want two non-empty IDs", entities)
	}
	if entities[0].ExternalID == entities[1].ExternalID ||
		!strings.HasSuffix(entities[0].ExternalID, ":1") || !strings.HasSuffix(entities[1].ExternalID, ":2") {
		t.Errorf("anonymous interface IDs = %q, %q; want distinct occurrence suffixes", entities[0].ExternalID, entities[1].ExternalID)
	}
}

func TestBuildRuleTableTrackerIdentitySurvivesMutableChangesAndReordering(t *testing.T) {
	first := []byte(`{"data":[
		{"id":0,"tracker":18446744073709551615,"descr":"Duplicate","disabled":false,"log":true,"interface":["wan","lan"]},
		{"id":1,"tracker":"rule-b","descr":"Duplicate","disabled":false,"log":false}
	]}`)
	second := []byte(`{"data":[
		{"id":3,"tracker":"rule-b","descr":"Renamed B","disabled":true,"log":true},
		{"id":2,"tracker":"18446744073709551615","descr":"Renamed A","disabled":true,"log":false,"interface":["wan","lan"]}
	]}`)

	_, before := buildRuleTable(first)
	_, after := buildRuleTable(second)
	if len(before) != 2 || len(after) != 2 {
		t.Fatalf("rule entity counts = %d and %d, want 2 each", len(before), len(after))
	}
	if before[0].ExternalID != "tracker:18446744073709551615" {
		t.Errorf("numeric tracker ExternalID = %q, want exact integer tracker", before[0].ExternalID)
	}
	if before[1].ExternalID != "tracker:rule-b" {
		t.Errorf("string tracker ExternalID = %q, want tracker:rule-b", before[1].ExternalID)
	}
	if before[0].ExternalID != after[1].ExternalID || before[1].ExternalID != after[0].ExternalID {
		t.Errorf("tracker ExternalIDs changed after reorder/state/description edits: before=%q,%q after=%q,%q",
			before[0].ExternalID, before[1].ExternalID, after[0].ExternalID, after[1].ExternalID)
	}
	if before[0].Name != "Duplicate" || after[1].Name != "Renamed A" {
		t.Errorf("displayed rule names changed unexpectedly: before=%q after=%q", before[0].Name, after[1].Name)
	}
	if before[0].Attributes["interface"] != "wan,lan" {
		t.Errorf("interface array attribute = %#v, want string \"wan,lan\"", before[0].Attributes["interface"])
	}
}

func TestBuildRuleTableFallbackIdentityIgnoresDescriptionAndState(t *testing.T) {
	first := []byte(`{"data":[
		{"id":0,"descr":"Duplicate","type":"pass","protocol":"tcp","source":"any","destination":"10.0.0.1","disabled":false,"log":true},
		{"id":1,"descr":"Duplicate","type":"block","protocol":"udp","source":"192.0.2.0/24","destination":"any","disabled":true,"log":false},
		{"id":2,"descr":"","type":"pass","protocol":"tcp","source":"198.51.100.4","destination":"any","disabled":false,"log":false}
	]}`)
	second := []byte(`{"data":[
		{"id":8,"descr":"Named later","type":"pass","protocol":"tcp","source":"198.51.100.4","destination":"any","disabled":true,"log":true},
		{"id":9,"descr":"Renamed duplicate","type":"pass","protocol":"tcp","source":"any","destination":"10.0.0.1","disabled":true,"log":false},
		{"id":10,"descr":"Renamed duplicate","type":"block","protocol":"udp","source":"192.0.2.0/24","destination":"any","disabled":false,"log":true}
	]}`)

	_, before := buildRuleTable(first)
	_, after := buildRuleTable(second)
	if len(before) != 3 || len(after) != 3 {
		t.Fatalf("rule entity counts = %d and %d, want 3 each", len(before), len(after))
	}
	beforeBySource := make(map[string]string, len(before))
	for _, entity := range before {
		beforeBySource[entity.Attributes["source"].(string)] = entity.ExternalID
	}
	for _, entity := range after {
		if got := beforeBySource[entity.Attributes["source"].(string)]; entity.ExternalID != got {
			t.Errorf("fallback ID for source %q = %q, want %q after reorder/edits", entity.Attributes["source"], entity.ExternalID, got)
		}
	}
	if before[0].ExternalID == before[1].ExternalID || before[2].ExternalID == "" {
		t.Errorf("fallback IDs do not distinguish duplicate and nameless rules: %+v", before)
	}
}

func TestBuildRuleTableFallbackDuplicatesGetOccurrenceIDs(t *testing.T) {
	raw := []byte(`{"data":[
		{"id":0,"descr":"one","type":"pass","protocol":"tcp","source":"any","destination":"any","disabled":false,"log":false},
		{"id":1,"descr":"two","type":"pass","protocol":"tcp","source":"any","destination":"any","disabled":true,"log":true}
	]}`)
	_, entities := buildRuleTable(raw)
	if len(entities) != 2 || entities[0].ExternalID == entities[1].ExternalID {
		t.Fatalf("fallback IDs = %+v; want unique occurrence IDs", entities)
	}
	if !strings.HasSuffix(entities[0].ExternalID, ":1") || !strings.HasSuffix(entities[1].ExternalID, ":2") {
		t.Errorf("fallback IDs = %q, %q; want colon occurrence suffixes", entities[0].ExternalID, entities[1].ExternalID)
	}
}

func TestConfigPushScopedTrackerResolvesCurrentPositionalID(t *testing.T) {
	var gotBody string
	ruleListRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			switch r.URL.Path {
			case "/api/v2/system/version":
				_, _ = w.Write([]byte(`{"data":{"platform":"pfSense","config_version":"2.7.2"}}`))
			case "/api/v2/interfaces":
				_, _ = w.Write([]byte(`{"data":[{"id":"wan","if":"em0","ipaddr":"203.0.113.5","status":"up","enable":true}]}`))
			case "/api/v2/firewall/rules":
				ruleListRequests++
				if ruleListRequests == 1 {
					_, _ = w.Write([]byte(`{"data":[
						{"id":0,"tracker":1714079391,"descr":"Original SSH"},
						{"id":1,"tracker":200,"descr":"Other"}
					]}`))
					return
				}
				_, _ = w.Write([]byte(`{"data":[
					{"id":0,"tracker":200,"descr":"Other"},
					{"id":1,"tracker":1714079391,"descr":"Renamed SSH","disabled":false}
				]}`))
			case "/api/v2/routing/gateways":
				_, _ = w.Write([]byte(`{"data":[]}`))
			default:
				t.Fatalf("unexpected GET path %s", r.URL.Path)
			}
			return
		}
		if r.Method != http.MethodPatch || r.URL.Path != "/api/v2/firewall/rule" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c := &Connector{url: server.URL, apiKey: "key", client: server.Client()}
	snapshot, err := c.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	var ref string
	for _, entity := range snapshot.Entities {
		if entity.Kind == "rule" && entity.Name == "Original SSH" {
			ref = entity.ExternalID
			break
		}
	}
	if ref == "" {
		t.Fatalf("Fetch() did not emit an external ID for Original SSH: %+v", snapshot.Entities)
	}
	if err := c.ConfigPush(context.Background(), nil, ref, "enabled", false); err != nil {
		t.Fatalf("ConfigPush() error = %v", err)
	}
	var got struct {
		ID       int  `json:"id"`
		Disabled bool `json:"disabled"`
	}
	if err := json.Unmarshal([]byte(gotBody), &got); err != nil {
		t.Fatalf("decode PATCH body: %v", err)
	}
	if got.ID != 1 || !got.Disabled {
		t.Errorf("PATCH body = %s; want reordered current id 1 and disabled=true", gotBody)
	}
}

func TestFetchScopesInterfaceAndRuleIDsBySource(t *testing.T) {
	newServer := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v2/system/version":
				_, _ = w.Write([]byte(`{"data":{"platform":"pfSense","config_version":"2.7.2"}}`))
			case "/api/v2/interfaces":
				_, _ = w.Write([]byte(`{"data":[{"id":"wan","if":"em0","ipaddr":"203.0.113.5","status":"up","enable":true}]}`))
			case "/api/v2/firewall/rules":
				_, _ = w.Write([]byte(`{"data":[{"id":0,"tracker":1714079391,"descr":"Allow SSH"}]}`))
			case "/api/v2/routing/gateways":
				_, _ = w.Write([]byte(`{"data":[]}`))
			default:
				t.Fatalf("unexpected request path %s", r.URL.Path)
			}
		}))
	}

	firstServer := newServer()
	defer firstServer.Close()
	secondServer := newServer()
	defer secondServer.Close()
	first := &Connector{url: firstServer.URL, client: firstServer.Client()}
	second := &Connector{url: secondServer.URL, client: secondServer.Client()}

	fetch := func(c *Connector) *connector.ServiceSnapshot {
		t.Helper()
		snapshot, err := c.Fetch(context.Background(), nil)
		if err != nil {
			t.Fatalf("Fetch() error = %v", err)
		}
		return snapshot
	}
	firstSnapshot := fetch(first)
	firstAgain := fetch(first)
	secondSnapshot := fetch(second)

	idsByKind := func(snapshot *connector.ServiceSnapshot) map[string]string {
		ids := make(map[string]string, 2)
		for _, entity := range snapshot.Entities {
			if entity.Kind == "interface" || entity.Kind == "rule" {
				ids[entity.Kind] = entity.ExternalID
			}
		}
		return ids
	}
	firstIDs := idsByKind(firstSnapshot)
	firstAgainIDs := idsByKind(firstAgain)
	secondIDs := idsByKind(secondSnapshot)
	for _, kind := range []string{"interface", "rule"} {
		if firstIDs[kind] == "" || firstIDs[kind] != firstAgainIDs[kind] {
			t.Errorf("%s ID is empty or unstable for same source: %q then %q", kind, firstIDs[kind], firstAgainIDs[kind])
		}
		if firstIDs[kind] == secondIDs[kind] {
			t.Errorf("%s IDs from separate sources collided: %q", kind, firstIDs[kind])
		}
	}
}

func TestConfigPushDoesNotPatchMissingOrAmbiguousRules(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		current string
	}{
		{
			name:    "missing tracker",
			ref:     "tracker:1714079391",
			current: `{"data":[{"id":0,"tracker":22,"descr":"Other"}]}`,
		},
		{
			name: "ambiguous fallback even when state differs",
			ref:  "",
			current: `{"data":[
				{"id":0,"descr":"first","type":"pass","protocol":"tcp","source":"any","destination":"any","disabled":false,"log":false},
				{"id":1,"descr":"second","type":"pass","protocol":"tcp","source":"any","destination":"any","disabled":true,"log":true}
			]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := tt.ref
			if ref == "" {
				_, entities := buildRuleTable([]byte(tt.current))
				ref = entities[0].ExternalID
			}
			patches := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPatch {
					patches++
				}
				_, _ = w.Write([]byte(tt.current))
			}))
			defer server.Close()

			c := &Connector{url: server.URL, apiKey: "key", client: server.Client()}
			ref = connector.ScopedExternalID(typeName, server.URL, ref)
			if err := c.ConfigPush(context.Background(), nil, ref, "enabled", true); err == nil {
				t.Fatal("ConfigPush() error = nil, want unresolved or ambiguous rule error")
			}
			if patches != 0 {
				t.Errorf("PATCH count = %d, want no write for unresolved or ambiguous rule", patches)
			}
		})
	}
}

func TestConfigPushRejectsForeignSourceAndPositionalRefs(t *testing.T) {
	tests := []struct {
		name string
		ref  func(serverURL string) string
	}{
		{
			name: "foreign source",
			ref: func(_ string) string {
				return connector.ScopedExternalID(typeName, "https://different-firewall.example", "tracker:1714079391")
			},
		},
		{name: "bare positional id", ref: func(string) string { return "1" }},
		{name: "unscoped tracker", ref: func(string) string { return "tracker:1714079391" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests++
				_, _ = w.Write([]byte(`{"data":[{"id":1,"tracker":1714079391}]}`))
			}))
			defer server.Close()

			c := &Connector{url: server.URL, apiKey: "key", client: server.Client()}
			if err := c.ConfigPush(context.Background(), nil, tt.ref(server.URL), "enabled", true); err == nil {
				t.Fatal("ConfigPush() error = nil, want rejected reference")
			}
			if requests != 0 {
				t.Errorf("API request count = %d, want 0", requests)
			}
		})
	}
}
