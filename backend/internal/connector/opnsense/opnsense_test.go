package opnsense

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestValidateUsesBasicAuthAndSurfacesStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "key" || password != "secret" || r.URL.Path != "/api/core/firmware/status" {
			t.Fatalf("request auth/path invalid")
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("denied"))
	}))
	defer server.Close()
	c := &Connector{url: server.URL, apiKey: "key", apiSecret: "secret", client: server.Client()}
	err := c.Validate(context.Background(), nil)
	var authErr *connector.AuthError
	if !errors.As(err, &authErr) || !strings.Contains(err.Error(), "API returned 401: denied") {
		t.Fatalf("Validate() error = %v, want *connector.AuthError", err)
	}
}

func TestFetchSurfacesMalformedSystemResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/core/firmware/status":
			_, _ = w.Write([]byte(`not json`))
		case "/api/diagnostics/interface/getInterfaces", "/api/firewall/filter/searchRule", "/api/routes/gateway/status":
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	c := &Connector{url: server.URL, apiKey: "key", apiSecret: "secret", client: server.Client()}
	snap, err := c.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if !strings.Contains(snap.Sections[0].Content, "malformed response") {
		t.Fatalf("System section = %q, want malformed response placeholder", snap.Sections[0].Content)
	}
}

func TestFetchSurfacesWANAndUpstreamDependencies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/core/firmware/status":
			_, _ = w.Write([]byte(`{"product_name":"OPNsense","product_version":"24.1"}`))
		case "/api/diagnostics/interface/getInterfaces":
			_, _ = w.Write([]byte(`{"rows":[
				{"identifier":"wan","device":"igb0","ipaddr":"203.0.113.5","status":"up","media":"1000baseT"},
				{"identifier":"lan","device":"igb1","ipaddr":"10.0.0.1","status":"up","media":"1000baseT"}
			]}`))
		case "/api/firewall/filter/searchRule":
			_, _ = w.Write([]byte(`{"rows":[]}`))
		case "/api/routes/gateway/status":
			_, _ = w.Write([]byte(`{"items":[{"name":"WAN_GW","address":"203.0.113.1","status":"online","rtt":"5ms","loss":"0%"}]}`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	c := &Connector{url: server.URL, apiKey: "key", apiSecret: "secret", client: server.Client()}
	snap, err := c.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if len(snap.Sections) != 4 {
		t.Fatalf("Sections = %d, want 4", len(snap.Sections))
	}

	wantDeps := []connector.ServiceDependency{
		{Kind: "network", Name: "igb0"},
		{Kind: "upstream_service", Name: "WAN_GW"},
	}
	for _, want := range wantDeps {
		found := false
		for _, got := range snap.Dependencies {
			if got.Kind == want.Kind && got.Name == want.Name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Dependencies missing %+v, got %+v", want, snap.Dependencies)
		}
	}
}

func TestFetchScopesInterfaceAndFallbackRuleIDsBySource(t *testing.T) {
	newServer := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/core/firmware/status":
				_, _ = w.Write([]byte(`{"product_name":"OPNsense","product_version":"24.1"}`))
			case "/api/diagnostics/interface/getInterfaces":
				_, _ = w.Write([]byte(`{"rows":[{"device":"igb0","ipaddr":"203.0.113.5","status":"up"}]}`))
			case "/api/firewall/filter/searchRule":
				_, _ = w.Write([]byte(`{"rows":[
					{"uuid":"f4cba8a1-0c93-4cb2-9c5c-821331233db9","description":"UUID rule","action":"pass","protocol":"tcp","source_net":"any","destination_net":"any","destination_port":"22"},
					{"description":"Fallback rule","action":"block","protocol":"udp","ipprotocol":"inet","source_net":"10.0.0.0/8","source_port":"53","destination_net":"any","interface":"lan","direction":"in"}
				]}`))
			case "/api/routes/gateway/status":
				_, _ = w.Write([]byte(`{"items":[]}`))
			default:
				t.Fatalf("unexpected request path: %s", r.URL.Path)
			}
		}))
	}

	firstServer := newServer()
	defer firstServer.Close()
	secondServer := newServer()
	defer secondServer.Close()

	newConnector := func(server *httptest.Server) *Connector {
		return &Connector{url: server.URL, apiKey: "key", apiSecret: "secret", client: server.Client()}
	}
	first, err := newConnector(firstServer).Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("first Fetch() error = %v", err)
	}
	repeated, err := newConnector(firstServer).Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("repeated Fetch() error = %v", err)
	}
	otherSource, err := newConnector(secondServer).Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("other-source Fetch() error = %v", err)
	}

	findEntity := func(snapshot *connector.ServiceSnapshot, kind, name string) connector.SnapshotEntity {
		t.Helper()
		for _, entity := range snapshot.Entities {
			if entity.Kind == kind && entity.Name == name {
				return entity
			}
		}
		t.Fatalf("snapshot has no %s entity named %q: %+v", kind, name, snapshot.Entities)
		return connector.SnapshotEntity{}
	}
	firstDevice := findEntity(first, "interface", "igb0")
	repeatedDevice := findEntity(repeated, "interface", "igb0")
	otherDevice := findEntity(otherSource, "interface", "igb0")
	if firstDevice.ExternalID == "" || firstDevice.ExternalID != repeatedDevice.ExternalID || firstDevice.ExternalID == otherDevice.ExternalID {
		t.Errorf("scoped interface IDs: first %q, repeated %q, other source %q", firstDevice.ExternalID, repeatedDevice.ExternalID, otherDevice.ExternalID)
	}
	firstFallback := findEntity(first, "rule", "Fallback rule")
	repeatedFallback := findEntity(repeated, "rule", "Fallback rule")
	otherFallback := findEntity(otherSource, "rule", "Fallback rule")
	if firstFallback.ExternalID == "" || firstFallback.ExternalID != repeatedFallback.ExternalID || firstFallback.ExternalID == otherFallback.ExternalID {
		t.Errorf("scoped fallback-rule IDs: first %q, repeated %q, other source %q", firstFallback.ExternalID, repeatedFallback.ExternalID, otherFallback.ExternalID)
	}
	for _, snapshot := range []*connector.ServiceSnapshot{first, repeated, otherSource} {
		if got := findEntity(snapshot, "rule", "UUID rule").ExternalID; got != "f4cba8a1-0c93-4cb2-9c5c-821331233db9" {
			t.Errorf("upstream UUID ExternalID = %q, want unchanged UUID", got)
		}
	}
}

func TestDoRequestErrorCases(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		checkAuthError bool
		checkUnavail   bool
	}{
		{
			name:           "401 Unauthorized returns AuthError",
			statusCode:     http.StatusUnauthorized,
			checkAuthError: true,
		},
		{
			name:           "403 Forbidden returns AuthError",
			statusCode:     http.StatusForbidden,
			checkAuthError: true,
		},
		{
			name:         "502 BadGateway returns ServiceUnavailableError",
			statusCode:   http.StatusBadGateway,
			checkUnavail: true,
		},
		{
			name:         "503 ServiceUnavailable returns ServiceUnavailableError",
			statusCode:   http.StatusServiceUnavailable,
			checkUnavail: true,
		},
		{
			name:         "504 GatewayTimeout returns ServiceUnavailableError",
			statusCode:   http.StatusGatewayTimeout,
			checkUnavail: true,
		},
		{
			name:       "500 InternalServerError returns generic error",
			statusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte("error response"))
			}))
			defer server.Close()

			c := &Connector{url: server.URL, apiKey: "key", apiSecret: "secret", client: server.Client()}
			_, err := c.doRequest(context.Background(), "GET", "/api/test")

			if err == nil {
				t.Errorf("doRequest() error = nil, want error")
				return
			}

			if tt.checkAuthError {
				var authErr *connector.AuthError
				if !errors.As(err, &authErr) {
					t.Errorf("doRequest() error = %T, want *connector.AuthError", err)
				}
			}
			if tt.checkUnavail {
				var unavailErr *connector.ServiceUnavailableError
				if !errors.As(err, &unavailErr) {
					t.Errorf("doRequest() error = %T, want *connector.ServiceUnavailableError", err)
				}
			}
		})
	}
}

func TestDoRequestContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// Hang until the client gives up, so the test waits for the
		// client timeout only, not a fixed server-side sleep.
		<-r.Context().Done()
	}))
	defer server.Close()

	c := &Connector{url: server.URL, apiKey: "key", apiSecret: "secret", client: server.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := c.doRequest(ctx, "GET", "/api/test")
	var timeoutErr *connector.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Errorf("doRequest() error = %v, want *connector.TimeoutError", err)
	}
}

func TestBuildRuleTableMalformedCases(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "empty JSON object returns placeholder",
			data: []byte(`{}`),
			want: "_No firewall rules returned_",
		},
		{
			name: "JSON with empty rows returns placeholder",
			data: []byte(`{"rows":[]}`),
			want: "_No firewall rules returned_",
		},
		{
			name: "invalid JSON returns placeholder",
			data: []byte(`not json`),
			want: "_No firewall rules returned_",
		},
		{
			name: "partial JSON returns placeholder",
			data: []byte(`{"rows":[{"description":"test"`),
			want: "_No firewall rules returned_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := buildRuleTable(tt.data)
			if result != tt.want {
				t.Errorf("buildRuleTable() = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestBuildRuleTableValidRules(t *testing.T) {
	data := []byte(`{
		"rows":[
			{"description":"Allow SSH","action":"pass","protocol":"tcp","source_net":"any","destination_net":"any","enabled":"1"},
			{"description":"Block DNS","action":"block","protocol":"udp","source_net":"10.0.0.0/8","destination_net":"any","enabled":""}
		]
	}`)
	result, _ := buildRuleTable(data)
	if !strings.Contains(result, "Allow SSH") || !strings.Contains(result, "Block DNS") {
		t.Errorf("buildRuleTable() missing expected rules in: %q", result)
	}
}

func TestRestart(t *testing.T) {
	tests := []struct {
		name       string
		entityRef  string
		statusCode int
		body       string
		wantErr    bool
	}{
		{name: "success", entityRef: "unbound", statusCode: http.StatusOK, body: `{"status":"ok"}`},
		{name: "empty entityRef errors", entityRef: "", wantErr: true},
		{name: "status not ok errors", entityRef: "unbound", statusCode: http.StatusOK, body: `{"status":"failed"}`, wantErr: true},
		{name: "http error", entityRef: "unbound", statusCode: http.StatusInternalServerError, body: `{}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotMethod string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath, gotMethod = r.URL.Path, r.Method
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			c := &Connector{url: server.URL, apiKey: "key", apiSecret: "secret", client: server.Client()}
			err := c.Restart(context.Background(), nil, tt.entityRef)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Restart() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Restart() error = %v", err)
			}
			if gotMethod != "POST" || gotPath != "/api/core/service/restart/"+tt.entityRef {
				t.Errorf("request = %s %s, want POST /api/core/service/restart/%s", gotMethod, gotPath, tt.entityRef)
			}
		})
	}
}

func TestStartStop(t *testing.T) {
	tests := []struct {
		name       string
		action     string
		entityRef  string
		statusCode int
		body       string
		wantErr    bool
	}{
		{name: "start success", action: "start", entityRef: "unbound", statusCode: http.StatusOK, body: `{"status":"ok"}`},
		{name: "stop success", action: "stop", entityRef: "unbound", statusCode: http.StatusOK, body: `{"status":"ok"}`},
		{name: "start empty entityRef errors", action: "start", entityRef: "", wantErr: true},
		{name: "stop status not ok errors", action: "stop", entityRef: "unbound", statusCode: http.StatusOK, body: `{"status":"failed"}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotMethod string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath, gotMethod = r.URL.Path, r.Method
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			c := &Connector{url: server.URL, apiKey: "key", apiSecret: "secret", client: server.Client()}
			var err error
			if tt.action == "start" {
				err = c.Start(context.Background(), nil, tt.entityRef)
			} else {
				err = c.Stop(context.Background(), nil, tt.entityRef)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("%s() error = nil, want error", tt.action)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s() error = %v", tt.action, err)
			}
			wantPath := "/api/core/service/" + tt.action + "/" + tt.entityRef
			if gotMethod != "POST" || gotPath != wantPath {
				t.Errorf("request = %s %s, want POST %s", gotMethod, gotPath, wantPath)
			}
		})
	}
}

const (
	filterPrefix = "/api/firewall/filter/"
	testRule     = "rule-uuid"
	rev1         = "1712345678.1001"
	rev2         = "1712345678.1002"

	callSavepoint = "POST savepoint"
	callGetRule   = "GET getRule/" + testRule
	callSetRule   = "POST setRule/" + testRule
	callApply     = "POST apply"
	callApplyR1   = "POST apply/" + rev1
	callCancelR1  = "POST cancelRollback/" + rev1
	callRevertR1  = "POST revert/" + rev1
)

// fault replaces the answer of one fake endpoint call. effect says whether the
// call still changes the fake's state (a request that failed after the firewall
// acted on it).
type fault struct {
	code   int
	body   string
	effect bool
}

func always(f fault) func(int) *fault { return func(int) *fault { return &f } }

func onCall(n int, f fault) func(int) *fault {
	return func(i int) *fault {
		if i == n {
			return &f
		}
		return nil
	}
}

// fakeFilter models the filter API of an OPNsense firewall with one rule. With
// legacy set it behaves like 24.1 to 26.1 (savepoints, revert, cancelRollback
// and a rollback timer started by apply/<revision>); otherwise like 26.7 and
// later, where those endpoints answer 404.
type fakeFilter struct {
	t       *testing.T
	srv     *httptest.Server
	legacy  bool
	pushEnd string

	mu         sync.Mutex
	saved      map[string]string
	live       map[string]string
	savepoints map[string]map[string]string
	pending    map[string]bool
	revSeq     int
	calls      []string
	counts     map[string]int
	setRules   []string
	faults     map[string]func(int) *fault
	after      map[string]func(int)
	active     bool
	overlaps   int
}

func newFakeFilter(t *testing.T, legacy bool) *fakeFilter {
	t.Helper()
	f := &fakeFilter{
		t:          t,
		legacy:     legacy,
		pushEnd:    "apply",
		saved:      map[string]string{testRule: "0"},
		live:       map[string]string{testRule: "0"},
		savepoints: map[string]map[string]string{},
		pending:    map[string]bool{},
		counts:     map[string]int{},
		faults:     map[string]func(int) *fault{},
		after:      map[string]func(int){},
	}
	if legacy {
		f.pushEnd = "cancelRollback"
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeFilter) conn() *Connector {
	return &Connector{url: f.srv.URL, apiKey: "key", apiSecret: "secret", client: f.srv.Client()}
}

func (f *fakeFilter) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)

	code, body := http.StatusOK, ""
	if !strings.HasPrefix(r.URL.Path, filterPrefix) {
		if r.Method != http.MethodGet {
			f.t.Errorf("%s %s: want GET", r.Method, r.URL.Path)
		}
		switch r.URL.Path {
		case "/api/core/firmware/status":
			body = `{"product_name":"OPNsense","product_version":"25.1"}`
		case "/api/diagnostics/interface/getInterfaces", "/api/routes/gateway/status":
			body = `{}`
		default:
			code, body = http.StatusNotFound, `{"errorMessage":"Endpoint not found"}`
		}
		w.WriteHeader(code)
		_, _ = io.WriteString(w, body)
		return
	}

	name, arg, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, filterPrefix), "/")
	want := http.MethodPost
	if name == "getRule" || name == "searchRule" {
		want = http.MethodGet
	}
	if r.Method != want {
		f.t.Errorf("%s %s: want %s", r.Method, r.URL.Path, want)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	f.counts[name]++
	n := f.counts[name]
	var ft *fault
	if fn := f.faults[name]; fn != nil {
		ft = fn(n)
	}
	code, body = f.handle(r, name, arg, ft == nil || ft.effect)
	if ft != nil {
		if ft.code != 0 {
			code, body = ft.code, `{"status":"error"}`
		}
		if ft.body != "" {
			body = ft.body
		}
	}
	if name == f.pushEnd {
		f.active = false
	}
	if fn := f.after[name]; fn != nil {
		fn(n)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = io.WriteString(w, body)
}

func (f *fakeFilter) handle(r *http.Request, name, arg string, effect bool) (int, string) {
	notFound := func() (int, string) { return http.StatusNotFound, `{"errorMessage":"Endpoint not found"}` }
	switch name {
	case "savepoint":
		if f.active {
			f.overlaps++
		}
		f.active = true
		if !f.legacy {
			return notFound()
		}
		f.revSeq++
		rev := fmt.Sprintf("1712345678.%d", 1000+f.revSeq)
		if effect {
			f.savepoints[rev] = maps.Clone(f.saved)
		}
		return http.StatusOK, fmt.Sprintf(`{"status":"ok","retention":"10","revision":%q}`, rev)
	case "setRule":
		var req struct {
			Rule struct {
				Enabled string `json:"enabled"`
			} `json:"rule"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.setRules = append(f.setRules, req.Rule.Enabled)
		if _, ok := f.saved[arg]; !ok {
			return http.StatusOK, `{"result":"failed"}`
		}
		if effect {
			f.saved[arg] = req.Rule.Enabled
		}
		return http.StatusOK, `{"result":"saved"}`
	case "apply":
		if f.legacy != (arg != "") {
			return notFound()
		}
		// The rollback timer starts before the reload, and only one can run.
		if f.legacy && len(f.pending) == 0 {
			f.pending[arg] = true
		}
		if effect {
			f.live = maps.Clone(f.saved)
		}
		return http.StatusOK, `{"status":"OK\n\n"}`
	case "revert":
		if !f.legacy {
			return notFound()
		}
		sp, ok := f.savepoints[arg]
		if !ok {
			return http.StatusOK, `{"status":"unknown (or removed) savepoint"}`
		}
		if effect {
			f.saved, f.live = maps.Clone(sp), maps.Clone(sp)
		}
		return http.StatusOK, `{"status":"ok"}`
	case "cancelRollback":
		if !f.legacy {
			return notFound()
		}
		if effect {
			delete(f.pending, arg)
		}
		return http.StatusOK, `{"status":""}`
	case "getRule":
		v, ok := f.saved[arg]
		if !ok {
			return http.StatusOK, `[]`
		}
		return http.StatusOK, fmt.Sprintf(`{"rule":{"enabled":%q,"action":"pass"}}`, v)
	case "searchRule":
		var rows []string
		for uuid, v := range f.saved {
			rows = append(rows, fmt.Sprintf(`{"uuid":%q,"enabled":%q,"action":"pass","protocol":"TCP","source":"any","destination":"any"}`, uuid, v))
		}
		return http.StatusOK, `{"rows":[` + strings.Join(rows, ",") + `]}`
	}
	return notFound()
}

// seq returns the push-related calls ("POST savepoint", "GET getRule/<uuid>",
// ...), leaving out the reader's GETs.
func (f *fakeFilter) seq() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, call := range f.calls {
		path := strings.SplitN(call, " ", 2)[1]
		if !strings.HasPrefix(path, filterPrefix) || call == "GET "+filterPrefix+"searchRule" {
			continue
		}
		out = append(out, strings.SplitN(call, " ", 2)[0]+" "+strings.TrimPrefix(path, filterPrefix))
	}
	return out
}

func (f *fakeFilter) state() (saved, live string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.saved[testRule], f.live[testRule]
}

func (f *fakeFilter) count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[name]
}

func (f *fakeFilter) pendingTimers() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.pending)
}

// fireRollbackTimers simulates the 60 seconds elapsing: every pending timer
// rolls the saved and live state back to its savepoint.
func (f *fakeFilter) fireRollbackTimers() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for rev := range f.pending {
		f.saved, f.live = maps.Clone(f.savepoints[rev]), maps.Clone(f.savepoints[rev])
	}
	f.pending = map[string]bool{}
}

func (f *fakeFilter) expectState(t *testing.T, saved, live string) {
	t.Helper()
	if gotSaved, gotLive := f.state(); gotSaved != saved || gotLive != live {
		t.Errorf("state saved=%q live=%q, want saved=%q live=%q", gotSaved, gotLive, saved, live)
	}
}

func (f *fakeFilter) expectSeq(t *testing.T, want ...string) {
	t.Helper()
	if got := f.seq(); !reflect.DeepEqual(got, want) {
		t.Errorf("call sequence = %v, want %v", got, want)
	}
}

func expectErrContains(t *testing.T, err error, subs ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("ConfigPush() error = nil, want error")
	}
	for _, sub := range subs {
		if !strings.Contains(err.Error(), sub) {
			t.Errorf("error %q does not contain %q", err, sub)
		}
	}
}

func TestConfigPush(t *testing.T) {
	tests := []struct {
		name      string
		entityRef string
		fieldKey  string
		value     any
		wantErr   bool
	}{
		{name: "success", entityRef: testRule, fieldKey: "enabled", value: true},
		{name: "empty entityRef errors", entityRef: "", fieldKey: "enabled", value: true, wantErr: true},
		{name: "fallback entityRef errors", entityRef: "fallback:rule:deadbeef", fieldKey: "enabled", value: true, wantErr: true},
		{name: "scoped fallback entityRef errors", entityRef: "opnsense:deadbeef:fallback:rule:deadbeef", fieldKey: "enabled", value: true, wantErr: true},
		{name: "unsupported field errors", entityRef: testRule, fieldKey: "action", value: "block", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeFilter(t, true)
			err := f.conn().ConfigPush(context.Background(), nil, tt.entityRef, tt.fieldKey, tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ConfigPush() error = nil, want error")
				}
				if got := f.seq(); len(got) != 0 {
					t.Errorf("invalid ConfigPush() called upstream: %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ConfigPush() error = %v", err)
			}
			f.expectState(t, "1", "1")
		})
	}
}

type pushCase struct {
	name      string
	ref       string
	faults    map[string]func(int) *fault
	wantSeq   []string
	wantErr   []string // nil means success
	wantSaved string
	wantLive  string
	wantSets  []string // values sent to setRule, when non-nil
}

func runPushCases(t *testing.T, legacy bool, tests []pushCase) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeFilter(t, legacy)
			f.faults = tt.faults
			ref := tt.ref
			if ref == "" {
				ref = testRule
			}
			err := f.conn().ConfigPush(context.Background(), nil, ref, "enabled", true)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("ConfigPush() error = %v", err)
				}
			} else {
				expectErrContains(t, err, tt.wantErr...)
			}
			f.expectSeq(t, tt.wantSeq...)
			f.expectState(t, tt.wantSaved, tt.wantLive)
			if tt.wantSets != nil && !reflect.DeepEqual(f.setRules, tt.wantSets) {
				t.Errorf("setRule values = %v, want %v", f.setRules, tt.wantSets)
			}
		})
	}
}

func TestConfigPush_SavepointGeneration(t *testing.T) {
	okFlow := []string{callSavepoint, callSetRule, callApplyR1, callCancelR1}
	failedApply := []string{callSavepoint, callSetRule, callApplyR1, callRevertR1, callCancelR1}
	badRevision := func(body string) pushCase {
		return pushCase{
			name:      "savepoint revision " + body,
			faults:    map[string]func(int) *fault{"savepoint": always(fault{body: body})},
			wantSeq:   []string{callSavepoint},
			wantErr:   []string{"opnsense savepoint"},
			wantSaved: "0", wantLive: "0",
		}
	}
	runPushCases(t, true, []pushCase{
		{name: "success", wantSeq: okFlow, wantSaved: "1", wantLive: "1"},
		{
			name:    "lower-case ok status is also success",
			faults:  map[string]func(int) *fault{"apply": always(fault{body: `{"status":"ok"}`, effect: true})},
			wantSeq: okFlow, wantSaved: "1", wantLive: "1",
		},
		{
			name:    "apply HTTP 500 is reverted",
			faults:  map[string]func(int) *fault{"apply": always(fault{code: 500})},
			wantSeq: failedApply, wantErr: []string{"API returned 500", "change rolled back"},
			wantSaved: "0", wantLive: "0",
		},
		{
			name:    "apply status Error (1) is reverted",
			faults:  map[string]func(int) *fault{"apply": always(fault{body: `{"status":"Error (1)\n\n"}`})},
			wantSeq: failedApply, wantErr: []string{"Error (1)", "change rolled back"},
			wantSaved: "0", wantLive: "0",
		},
		{
			name:    "apply empty status (configd timeout) is reverted",
			faults:  map[string]func(int) *fault{"apply": always(fault{body: `{"status":""}`})},
			wantSeq: failedApply, wantErr: []string{`status ""`, "change rolled back"},
			wantSaved: "0", wantLive: "0",
		},
		{
			name:    "apply undecodable body is reverted",
			faults:  map[string]func(int) *fault{"apply": always(fault{body: `not json`})},
			wantSeq: failedApply, wantErr: []string{"change rolled back"},
			wantSaved: "0", wantLive: "0",
		},
		{
			name: "apply and revert fail with HTTP 500",
			faults: map[string]func(int) *fault{
				"apply":  always(fault{code: 500}),
				"revert": always(fault{code: 500}),
			},
			wantSeq: []string{callSavepoint, callSetRule, callApplyR1, callRevertR1},
			wantErr: []string{"API returned 500", "revert failed", "within about 60 seconds"},
			// The apply never reloaded, the rule stays saved but not live.
			wantSaved: "1", wantLive: "0",
		},
		{
			name: "apply fails and revert reports an unknown savepoint",
			faults: map[string]func(int) *fault{
				"apply":  always(fault{code: 500}),
				"revert": always(fault{body: `{"status":"unknown (or removed) savepoint"}`}),
			},
			wantSeq:   []string{callSavepoint, callSetRule, callApplyR1, callRevertR1},
			wantErr:   []string{"API returned 500", "revert failed", "unknown (or removed) savepoint"},
			wantSaved: "1", wantLive: "0",
		},
		{
			name:      "cancelRollback failure after a successful apply is an error and is reverted",
			faults:    map[string]func(int) *fault{"cancelRollback": always(fault{code: 500})},
			wantSeq:   []string{callSavepoint, callSetRule, callApplyR1, callCancelR1, callRevertR1, callCancelR1},
			wantErr:   []string{"rollback could not be cancelled", "change rolled back"},
			wantSaved: "0", wantLive: "0",
		},
		{
			name:    "savepoint HTTP 500",
			faults:  map[string]func(int) *fault{"savepoint": always(fault{code: 500})},
			wantSeq: []string{callSavepoint}, wantErr: []string{"opnsense savepoint: ", "API returned 500"},
			wantSaved: "0", wantLive: "0",
		},
		badRevision(`{"status":"ok","revision":""}`),
		badRevision(`{"status":"ok"}`),
		badRevision(`{"status":"ok","revision":"rev-1"}`),
		badRevision(`{"status":"ok","revision":"../../core/system/reboot"}`),
		badRevision(`{"status":"ok","revision":"1712345678.99/x"}`),
		{
			name:    "setRule result failed is not reverted",
			faults:  map[string]func(int) *fault{"setRule": always(fault{body: `{"result":"failed"}`})},
			wantSeq: []string{callSavepoint, callSetRule}, wantErr: []string{"rule not saved", `"failed"`},
			wantSaved: "0", wantLive: "0",
		},
		{
			name:      "setRule HTTP 500 is reverted",
			faults:    map[string]func(int) *fault{"setRule": always(fault{code: 500, effect: true})},
			wantSeq:   []string{callSavepoint, callSetRule, callRevertR1, callCancelR1},
			wantErr:   []string{"API returned 500", "change rolled back"},
			wantSaved: "0", wantLive: "0",
		},
	})
}

func TestConfigPush_FallbackGeneration(t *testing.T) {
	undone := []string{callSavepoint, callGetRule, callSetRule, callApply, callSetRule, callApply}
	runPushCases(t, false, []pushCase{
		{
			name:    "success",
			wantSeq: []string{callSavepoint, callGetRule, callSetRule, callApply}, wantSaved: "1", wantLive: "1",
		},
		{
			name:    "apply HTTP 500 restores the previous value",
			faults:  map[string]func(int) *fault{"apply": onCall(1, fault{code: 500})},
			wantSeq: undone, wantErr: []string{"API returned 500", "previous value restored"},
			wantSaved: "0", wantLive: "0", wantSets: []string{"1", "0"},
		},
		{
			name:    "apply status Error (1) restores the previous value",
			faults:  map[string]func(int) *fault{"apply": onCall(1, fault{body: `{"status":"Error (1)\n\n"}`})},
			wantSeq: undone, wantErr: []string{"Error (1)", "previous value restored"},
			wantSaved: "0", wantLive: "0", wantSets: []string{"1", "0"},
		},
		{
			name: "apply fails and the undo setRule fails",
			faults: map[string]func(int) *fault{
				"apply":   onCall(1, fault{body: `{"status":"Error (1)\n\n"}`}),
				"setRule": onCall(2, fault{code: 500}),
			},
			wantSeq: []string{callSavepoint, callGetRule, callSetRule, callApply, callSetRule},
			wantErr: []string{"Error (1)", "undo failed", "API returned 500", "saved with the new value"},
			// The new value stays saved but was never applied.
			wantSaved: "1", wantLive: "0",
		},
		{
			name:    "apply fails and the undo apply fails",
			faults:  map[string]func(int) *fault{"apply": always(fault{body: `{"status":"Error (1)\n\n"}`})},
			wantSeq: undone, wantErr: []string{"Error (1)", "undo failed"},
			wantSaved: "0", wantLive: "0",
		},
		{
			name:    "getRule for an unknown rule",
			ref:     "missing-uuid",
			wantSeq: []string{callSavepoint, "GET getRule/missing-uuid"}, wantErr: []string{"not found"},
			wantSaved: "0", wantLive: "0",
		},
		{
			name:    "getRule without a rule object",
			faults:  map[string]func(int) *fault{"getRule": always(fault{body: `{}`})},
			wantSeq: []string{callSavepoint, callGetRule}, wantErr: []string{"not found"},
			wantSaved: "0", wantLive: "0",
		},
		{
			name:    "getRule with an unexpected enabled value",
			faults:  map[string]func(int) *fault{"getRule": always(fault{body: `{"rule":{"enabled":"2"}}`})},
			wantSeq: []string{callSavepoint, callGetRule}, wantErr: []string{"unexpected enabled value"},
			wantSaved: "0", wantLive: "0",
		},
		{
			name:    "setRule result failed is not undone",
			faults:  map[string]func(int) *fault{"setRule": always(fault{body: `{"result":"failed"}`})},
			wantSeq: []string{callSavepoint, callGetRule, callSetRule}, wantErr: []string{"rule not saved"},
			wantSaved: "0", wantLive: "0",
		},
		{
			name:      "setRule HTTP 500 is undone",
			faults:    map[string]func(int) *fault{"setRule": onCall(1, fault{code: 500, effect: true})},
			wantSeq:   []string{callSavepoint, callGetRule, callSetRule, callSetRule, callApply},
			wantErr:   []string{"API returned 500", "previous value restored"},
			wantSaved: "0", wantLive: "0",
		},
	})
}

func TestConfigPush_ContextCancelledMidFlow(t *testing.T) {
	tests := []struct {
		name    string
		legacy  bool
		wantSeq []string
	}{
		{"savepoint generation", true, []string{callSavepoint, callSetRule, callRevertR1, callCancelR1}},
		{"fallback generation", false, []string{callSavepoint, callGetRule, callSetRule, callSetRule, callApply}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeFilter(t, tt.legacy)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f.after["setRule"] = func(n int) {
				if n == 1 {
					cancel()
					time.Sleep(5 * time.Millisecond)
				}
			}
			err := f.conn().ConfigPush(ctx, nil, testRule, "enabled", true)
			if err == nil {
				t.Fatal("ConfigPush() error = nil, want error")
			}
			f.expectSeq(t, tt.wantSeq...)
			f.expectState(t, "0", "0")
		})
	}
}

// TestConfigPush_RetryAfterFailedApply follows the shared core: a failed push,
// then a read that must not report the target, then a push that writes again.
func TestConfigPush_RetryAfterFailedApply(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		t.Run(fmt.Sprintf("legacy=%v", legacy), func(t *testing.T) {
			f := newFakeFilter(t, legacy)
			f.faults["apply"] = onCall(1, fault{code: 500})
			c := f.conn()

			if err := c.ConfigPush(context.Background(), nil, testRule, "enabled", true); err == nil {
				t.Fatal("first ConfigPush() error = nil, want error")
			}
			got, err := c.ConfigRead(context.Background(), nil, testRule, "enabled")
			if err != nil {
				t.Fatalf("ConfigRead() error = %v", err)
			}
			if got != false {
				t.Fatalf("ConfigRead() = %v, want false (the saved value must be back at the previous one)", got)
			}
			if err := c.ConfigPush(context.Background(), nil, testRule, "enabled", true); err != nil {
				t.Fatalf("second ConfigPush() error = %v", err)
			}
			// The fallback generation also undoes the failed push: set and apply
			// run once more for the undo.
			want := 2
			if !legacy {
				want = 3
			}
			if f.count("setRule") != want || f.count("apply") != want {
				t.Errorf("setRule calls = %d, apply calls = %d, want %d each", f.count("setRule"), f.count("apply"), want)
			}
			f.expectState(t, "1", "1")
		})
	}
}

func TestConfigPush_RollbackTimers(t *testing.T) {
	push := func(f *fakeFilter) error {
		return f.conn().ConfigPush(context.Background(), nil, testRule, "enabled", true)
	}

	t.Run("successful push cancels its timer", func(t *testing.T) {
		f := newFakeFilter(t, true)
		if err := push(f); err != nil {
			t.Fatalf("ConfigPush() error = %v", err)
		}
		if got := f.pendingTimers(); got != 0 {
			t.Errorf("pending timers = %d, want 0", got)
		}
		f.fireRollbackTimers()
		f.expectState(t, "1", "1")
	})

	t.Run("a retry after a reverted push is not undone by the stale timer", func(t *testing.T) {
		f := newFakeFilter(t, true)
		f.faults["apply"] = onCall(1, fault{code: 500})
		if err := push(f); err == nil {
			t.Fatal("first ConfigPush() error = nil, want error")
		}
		f.expectSeq(t, callSavepoint, callSetRule, callApplyR1, callRevertR1, callCancelR1)
		if err := push(f); err != nil {
			t.Fatalf("retry ConfigPush() error = %v", err)
		}
		f.fireRollbackTimers()
		f.expectState(t, "1", "1")
	})

	t.Run("failed cancelRollback after a revert is reported", func(t *testing.T) {
		f := newFakeFilter(t, true)
		f.faults["apply"] = always(fault{code: 500})
		f.faults["cancelRollback"] = always(fault{code: 500})
		err := push(f)
		expectErrContains(t, err, "API returned 500", "change rolled back", "rollback timer may still fire within about a minute")
		f.expectSeq(t, callSavepoint, callSetRule, callApplyR1, callRevertR1, callCancelR1)
		f.expectState(t, "0", "0")
	})

	t.Run("failed revert leaves the timer to roll back", func(t *testing.T) {
		f := newFakeFilter(t, true)
		f.faults["apply"] = always(fault{code: 500})
		f.faults["revert"] = always(fault{code: 500})
		if err := push(f); err == nil {
			t.Fatal("ConfigPush() error = nil, want error")
		}
		f.expectSeq(t, callSavepoint, callSetRule, callApplyR1, callRevertR1)
		if got := f.pendingTimers(); got != 1 {
			t.Fatalf("pending timers = %d, want 1", got)
		}
		f.fireRollbackTimers()
		f.expectState(t, "0", "0")
	})
}

func TestConfigPush_LockHonoursContext(t *testing.T) {
	f := newFakeFilter(t, true)
	unlock, err := lockFilter(context.Background(), f.srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = f.conn().ConfigPush(ctx, nil, testRule, "enabled", true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ConfigPush() error = %v, want context.Canceled", err)
	}
	if got := f.seq(); len(got) != 0 {
		t.Errorf("requests sent while waiting for the lock: %v", got)
	}
}

func TestConfigPush_ConcurrentPushesDoNotInterleave(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		t.Run(fmt.Sprintf("legacy=%v", legacy), func(t *testing.T) {
			f := newFakeFilter(t, legacy)
			f.after["setRule"] = func(int) { time.Sleep(5 * time.Millisecond) }

			var wg sync.WaitGroup
			errs := make([]error, 2)
			for i := range errs {
				wg.Add(1)
				go func() {
					defer wg.Done()
					errs[i] = f.conn().ConfigPush(context.Background(), nil, testRule, "enabled", true)
				}()
			}
			wg.Wait()
			for _, err := range errs {
				if err != nil {
					t.Errorf("ConfigPush() error = %v", err)
				}
			}
			f.mu.Lock()
			overlaps := f.overlaps
			f.mu.Unlock()
			if overlaps != 0 {
				t.Errorf("%d push(es) started while another was in flight", overlaps)
			}
			f.expectState(t, "1", "1")
		})
	}
}

func TestOpnsenseWritableFields(t *testing.T) {
	c := &Connector{}
	fields := c.WritableFields()
	if len(fields) != 1 || fields[0].Key != "enabled" {
		t.Errorf("WritableFields() = %+v, want one field \"enabled\"", fields)
	}
}
