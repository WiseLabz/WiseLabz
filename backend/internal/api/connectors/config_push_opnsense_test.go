package connectors

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// opnsenseFilterFake is a stateful fake of the OPNsense 26.7+ filter API (no
// savepoint endpoint). saved is what the rule table shows, live is what the
// firewall runs after an apply.
type opnsenseFilterFake struct {
	mu            sync.Mutex
	saved, live   string
	failApply     bool
	failUndoWrite bool
	setRules      []string // enabled values received by setRule, in order
	searches      int
	applies       int
}

func (f *opnsenseFilterFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	const uuid = "11111111-2222-3333-4444-555555555555"
	switch {
	case r.URL.Path == "/api/core/firmware/status":
		_, _ = w.Write([]byte(`{"product_name":"OPNsense","product_version":"26.7"}`))
	case r.URL.Path == "/api/diagnostics/interface/getInterfaces":
		_, _ = w.Write([]byte(`{"rows":[]}`))
	case r.URL.Path == "/api/routes/gateway/status":
		_, _ = w.Write([]byte(`{"items":[]}`))
	case r.URL.Path == "/api/firewall/filter/savepoint":
		w.WriteHeader(http.StatusNotFound)
	case r.URL.Path == "/api/firewall/filter/searchRule" && r.Method == http.MethodPost:
		f.searches++
		var request struct {
			Current  int `json:"current"`
			RowCount int `json:"rowCount"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.RowCount != 500 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var rows strings.Builder
		switch request.Current {
		case 1:
			for i := range 500 {
				if i > 0 {
					rows.WriteByte(',')
				}
				rows.WriteString(`{"uuid":"rule-` + strconv.Itoa(i) + `","description":"rule"}`)
			}
		case 2:
			rows.WriteString(`{"uuid":"` + uuid + `","description":"allow ssh","action":"pass","protocol":"TCP","enabled":"` + f.saved + `"}`)
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"total":501,"current":` + strconv.Itoa(request.Current) + `,"rowCount":500,"rows":[` + rows.String() + `]}`))
	case r.URL.Path == "/api/firewall/filter/getRule/"+uuid:
		_, _ = w.Write([]byte(`{"rule":{"enabled":"` + f.saved + `"}}`))
	case r.URL.Path == "/api/firewall/filter/setRule/"+uuid && r.Method == http.MethodPost:
		var body struct {
			Rule struct {
				Enabled string `json:"enabled"`
			} `json:"rule"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		f.setRules = append(f.setRules, body.Rule.Enabled)
		// The push's own write is the first one; the failing undo is the second.
		if f.failUndoWrite && len(f.setRules) == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		f.saved = body.Rule.Enabled
		_, _ = w.Write([]byte(`{"result":"saved"}`))
	case r.URL.Path == "/api/firewall/filter/apply" && r.Method == http.MethodPost:
		f.applies++
		if f.failApply {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		f.live = f.saved
		_, _ = w.Write([]byte(`{"status":"OK\n\n"}`))
	default:
		w.WriteHeader(http.StatusNotFound) // firmware, interfaces and gateways are not needed
	}
}

func (f *opnsenseFilterFake) counts() (setRules, applies int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.setRules), f.applies
}

func TestConfigPushOPNsenseRetryAfterFailedUndo(t *testing.T) {
	t.Parallel()
	fake := &opnsenseFilterFake{saved: "0", live: "0", failApply: true, failUndoWrite: true}
	server := httptest.NewServer(fake)
	defer server.Close()

	h := newTestHandler(t)
	body := `{"name":"fw","category":"networking","type":"opnsense","url":"` + server.URL + `","config":{"api_key":"k","api_secret":"s"}}`
	rr := httptest.NewRecorder()
	h.Create(rr, httptest.NewRequest(http.MethodPost, "/api/connectors", strings.NewReader(body)))
	var created map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("create: %v body=%s", err, rr.Body.String())
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("create status = %d, body = %s", rr.Code, rr.Body.String())
	}

	push := func(wantStatus int) *httptest.ResponseRecorder {
		t.Helper()
		token, err := h.JWT.IssueElevation("", "connector.configPush")
		if err != nil {
			t.Fatalf("IssueElevation() error = %v", err)
		}
		r := actionRequest(id, `{"entityRef":"11111111-2222-3333-4444-555555555555","fieldKey":"enabled","value":true}`)
		r.Header.Set("X-Elevation-Token", token.Token)
		return actionResponse(t, h.ConfigPush, r, wantStatus)
	}

	// The apply fails and so does the undo: saved is "1", live is still "0".
	first := push(http.StatusBadGateway)
	if !strings.Contains(first.Body.String(), "config_push_failed") {
		t.Fatalf("first push body = %s, want config_push_failed", first.Body.String())
	}
	fake.mu.Lock()
	saved, live := fake.saved, fake.live
	fake.failApply, fake.failUndoWrite = false, false
	fake.mu.Unlock()
	if saved != "1" || live != "0" {
		t.Fatalf("after first push saved=%q live=%q, want 1 and 0", saved, live)
	}

	// The rule table already shows the target, so only the read-back can tell
	// that this push landed. It must write and apply again, not skip.
	writesBefore, appliesBefore := fake.counts()
	push(http.StatusOK)
	writes, applies := fake.counts()
	if writes <= writesBefore || applies <= appliesBefore {
		t.Fatalf("second push wrote %d times and applied %d times, want at least one of each", writes-writesBefore, applies-appliesBefore)
	}
	fake.mu.Lock()
	lastWrite, live := fake.setRules[len(fake.setRules)-1], fake.live
	fake.mu.Unlock()
	if lastWrite != "1" || live != "1" {
		t.Fatalf("after second push last setRule=%q live=%q, want 1 and 1", lastWrite, live)
	}
	alerts, _, err := h.Store.ListAlerts(context.Background(), id, "", "", "", 0, 10)
	if err != nil || len(alerts) != 0 {
		t.Fatalf("alerts=%+v err=%v, want none", alerts, err)
	}
	records, _, err := h.Store.ListAuditRecords(context.Background(), "connector.configPush", "connector", "", "", 0, 10)
	if err != nil || len(records) != 1 {
		t.Fatalf("audit records=%+v err=%v, want one", records, err)
	}
	fake.mu.Lock()
	searches := fake.searches
	fake.mu.Unlock()
	if searches != 6 {
		t.Errorf("config push made %d searchRule page requests, want three 501-rule snapshot fetches (two pages each) with direct getRule reads", searches)
	}

	// Now at target: the next push is skipped without any write.
	writesBefore, appliesBefore = fake.counts()
	push(http.StatusOK)
	writes, applies = fake.counts()
	if writes != writesBefore || applies != appliesBefore {
		t.Fatalf("third push wrote %d times and applied %d times, want none", writes-writesBefore, applies-appliesBefore)
	}
}
