package opnsense

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSearchRulesPaginatesAndBuildsAllEntities(t *testing.T) {
	for _, count := range []int{0, 50, 51, 1001} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != filterAPI+"searchRule" {
					t.Fatalf("request = %s %s, want POST searchRule", r.Method, r.URL.Path)
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
				}
				user, password, ok := r.BasicAuth()
				if !ok || user != "key" || password != "secret" {
					t.Errorf("missing authenticated request")
				}
				var request struct {
					Current  int `json:"current"`
					RowCount int `json:"rowCount"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatal(err)
				}
				requests++
				if request.Current != requests || request.RowCount != rulePageSize {
					t.Errorf("request body = %+v on request %d", request, requests)
				}
				start := (request.Current - 1) * request.RowCount
				end := start + request.RowCount
				if end > count {
					end = count
				}
				rows := []string{}
				for i := start; i < end; i++ {
					rows = append(rows, fmt.Sprintf(`{"uuid":"rule-%d","description":"rule %d","enabled":"1"}`, i, i))
				}
				_, _ = fmt.Fprintf(w, `{"total":%d,"rows":[%s]}`, count, strings.Join(rows, ","))
			}))
			defer server.Close()

			c := &Connector{url: server.URL, apiKey: "key", apiSecret: "secret", client: testClient(server)}
			raw, err := c.searchRules(context.Background())
			if err != nil {
				t.Fatalf("searchRules() error = %v", err)
			}
			_, entities := buildRuleTable(raw)
			if len(entities) != count {
				t.Fatalf("entities = %d, want %d", len(entities), count)
			}
			wantRequests := (count + rulePageSize - 1) / rulePageSize
			if count == 0 {
				wantRequests = 1
			}
			if requests != wantRequests {
				t.Errorf("requests = %d, want %d", requests, wantRequests)
			}
			if count > 50 {
				content, allEntities := buildRuleTable(raw)
				if len(allEntities) != count || !strings.Contains(content, fmt.Sprintf("...and %d more rules", count-50)) {
					t.Errorf("display/content did not retain overflow and all entities (entities=%d)", len(allEntities))
				}
			}
		})
	}
}

func TestSearchRulesRejectsBadPagination(t *testing.T) {
	firstPage := make([]string, rulePageSize)
	for i := range firstPage {
		firstPage[i] = fmt.Sprintf(`{"uuid":"rule-%d"}`, i)
	}
	firstRows := strings.Join(firstPage, ",")
	tests := []struct {
		name string
		page func(current int) string
		want string
	}{
		{name: "missing total", page: func(int) string { return `{"rows":[]}` }, want: "total"},
		{name: "invalid total", page: func(int) string { return `{"total":"2","rows":[]}` }, want: "total"},
		{name: "null total", page: func(int) string { return `{"total":null,"rows":[]}` }, want: "total"},
		{name: "missing rows", page: func(int) string { return `{"total":0}` }, want: "rows"},
		{name: "malformed row", page: func(int) string { return `{"total":1,"rows":[{"description":{}}]}` }, want: "row"},
		{name: "inconsistent total", page: func(current int) string {
			if current == 1 {
				return fmt.Sprintf(`{"total":%d,"rows":[%s]}`, rulePageSize+1, firstRows)
			}
			return `{"total":1002,"rows":[{"uuid":"last"}]}`
		}, want: "total changed"},
		{name: "premature exhaustion", page: func(current int) string {
			if current == 1 {
				return fmt.Sprintf(`{"total":%d,"rows":[%s]}`, rulePageSize+1, firstRows)
			}
			return fmt.Sprintf(`{"total":%d,"rows":[]}`, rulePageSize+1)
		}, want: "ended"},
		{name: "repeated page", page: func(int) string {
			duplicateRows := strings.ReplaceAll(firstRows, `"uuid":"rule-`, `"description":"fallback-`)
			return fmt.Sprintf(`{"total":%d,"rows":[%s]}`, 2*rulePageSize, duplicateRows)
		}, want: "repeated page"},
		{name: "inconsistent echoed page", page: func(int) string {
			return `{"total":0,"current":2,"rowCount":500,"rows":[]}`
		}, want: "current"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Current int `json:"current"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatal(err)
				}
				_, _ = w.Write([]byte(tt.page(request.Current)))
			}))
			defer server.Close()
			c := &Connector{url: server.URL, client: testClient(server)}
			_, err := c.searchRules(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("searchRules() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestFetchKeepsOtherSectionsWhenRuleSearchIsMalformed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/core/firmware/status":
			_, _ = w.Write([]byte(`{"product_name":"OPNsense","product_version":"26.7"}`))
		case "/api/diagnostics/interface/getInterfaces":
			_, _ = w.Write([]byte(`{"rows":[{"device":"igb0"}]}`))
		case filterAPI + "searchRule":
			_, _ = w.Write([]byte(`{"rows":[{"uuid":"partial"}]}`))
		case "/api/routes/gateway/status":
			_, _ = w.Write([]byte(`{"items":[]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	c := &Connector{url: server.URL, client: testClient(server)}
	snapshot, err := c.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(snapshot.Entities) != 1 || snapshot.Entities[0].Kind != "interface" {
		t.Fatalf("entities = %+v, want only interface entity", snapshot.Entities)
	}
	foundRuleError := false
	for _, section := range snapshot.Sections {
		if section.Title == "Firewall Rules" && strings.Contains(section.Content, "malformed response") {
			foundRuleError = true
		}
	}
	if !foundRuleError {
		t.Fatalf("sections = %+v, want Firewall Rules error", snapshot.Sections)
	}
}

func TestFetchDiscardsRulesWhenSecondPageFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/core/firmware/status":
			_, _ = w.Write([]byte(`{"product_name":"OPNsense","product_version":"26.7"}`))
		case "/api/diagnostics/interface/getInterfaces":
			_, _ = w.Write([]byte(`{"rows":[{"device":"igb0"}]}`))
		case filterAPI + "searchRule":
			var request struct {
				Current int `json:"current"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request.Current == 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			rows := make([]string, rulePageSize)
			for i := range rows {
				rows[i] = fmt.Sprintf(`{"uuid":"rule-%d"}`, i)
			}
			_, _ = fmt.Fprintf(w, `{"total":%d,"rows":[%s]}`, rulePageSize+1, strings.Join(rows, ","))
		case "/api/routes/gateway/status":
			_, _ = w.Write([]byte(`{"items":[]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	c := &Connector{url: server.URL, client: testClient(server)}
	snapshot, err := c.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(snapshot.Entities) != 1 || snapshot.Entities[0].Kind != "interface" {
		t.Fatalf("entities = %+v, want interface only and no partial rules", snapshot.Entities)
	}
	for _, section := range snapshot.Sections {
		if section.Title == "Firewall Rules" {
			if !strings.Contains(section.Content, "API returned 500") {
				t.Errorf("Firewall Rules section = %q, want page-two error", section.Content)
			}
			return
		}
	}
	t.Fatal("snapshot has no Firewall Rules section")
}

func TestSearchRulesPreservesCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = w.Write([]byte(`{"total":0,"rows":[]}`))
	}))
	defer server.Close()
	c := &Connector{url: server.URL, client: testClient(server)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := c.searchRules(ctx)
		done <- err
	}()
	<-started
	cancel()
	close(release)
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "canceled") {
			t.Fatalf("searchRules() error = %v, want cancellation", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("searchRules() did not return after cancellation")
	}
}

func TestSearchRulesPreservesUUIDLessDuplicateFallbackRows(t *testing.T) {
	row := `{"description":"identical fallback rule","action":"pass"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Current int `json:"current"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		count := rulePageSize
		if request.Current == 2 {
			count = 1
		}
		rows := make([]string, count)
		for i := range rows {
			rows[i] = row
		}
		_, _ = fmt.Fprintf(w, `{"total":%d,"rows":[%s]}`, rulePageSize+1, strings.Join(rows, ","))
	}))
	defer server.Close()
	c := &Connector{url: server.URL, client: testClient(server)}
	raw, err := c.searchRules(context.Background())
	if err != nil {
		t.Fatalf("searchRules() error = %v", err)
	}
	_, entities := buildRuleTable(raw)
	if len(entities) != rulePageSize+1 {
		t.Fatalf("entities = %d, want %d", len(entities), rulePageSize+1)
	}
	if entities[0].ExternalID == entities[1].ExternalID {
		t.Fatal("duplicate UUID-less rules must keep occurrence-distinct fallback identities")
	}
}
