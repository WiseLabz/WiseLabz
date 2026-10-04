package pbs

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const (
	testTokenID     = "root@pam!monitoring"
	testTokenSecret = "secret-value"
)

// testPBS serves handler and fails the test on any non-GET request or a wrong
// Authorization header. It returns a connector pointed at the server.
func testPBS(t *testing.T, handler http.HandlerFunc) *Connector {
	t.Helper()
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method != http.MethodGet {
			t.Errorf("received %s %s, want GET only", r.Method, r.URL.Path)
			http.Error(w, "read-only", http.StatusMethodNotAllowed)
			return
		}
		if got, want := r.Header.Get("Authorization"), "PBSAPIToken="+testTokenID+":"+testTokenSecret; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	connector.AllowLoopbackForTest(t)
	created, err := connector.Get(typeName, map[string]any{"url": server.URL, "token_id": testTokenID, "token_secret": testTokenSecret})
	if err != nil {
		t.Fatalf("connector.Get: %v", err)
	}
	return created.(*Connector)
}

func TestListDatastoresDecodesLooseForms(t *testing.T) {
	c := testPBS(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"data":[{"store":"main","comment":"Local","backend-type":"s3"},{"store":"bare"}]}`)
	})
	stores, err := c.listDatastores(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(stores) != 2 || stores[0] != (datastore{Store: "main", Comment: "Local", BackendType: "s3"}) || stores[1] != (datastore{Store: "bare"}) {
		t.Fatalf("stores = %+v", stores)
	}
}

func TestListNamespacesAlwaysIncludesRootSortedAndDeduplicated(t *testing.T) {
	c := testPBS(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/admin/datastore/store1/namespace" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, `{"data":[{"ns":"team/b"},{"ns":"team/a"},{"ns":"team/b"},{"ns":""}]}`)
	})
	got, err := c.listNamespaces(context.Background(), "store1")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"", "team/a", "team/b"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("namespaces = %q, want %q", got, want)
	}
}

func TestListNamespacesNullDataStillHasRoot(t *testing.T) {
	c := testPBS(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, `{"data":null}`) })
	got, err := c.listNamespaces(context.Background(), "s")
	if err != nil || len(got) != 1 || got[0] != "" {
		t.Fatalf("namespaces = %q, err = %v, want [\"\"]", got, err)
	}
}

func TestListGroupsDecodesLooseTypesAndSetsOrigin(t *testing.T) {
	var query string
	c := testPBS(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = fmt.Fprint(w, `{"data":[
			{"backup-type":"vm","backup-id":"100","backup-count":"5","last-backup":"1767225600"},
			{"backup-type":"ct","backup-id":200,"backup-count":2,"last-backup":1767225601.0},
			{"backup-type":"host","backup-id":"nas"}]}`)
	})
	groups, err := c.listGroups(context.Background(), "my/store", "team/a")
	if err != nil {
		t.Fatal(err)
	}
	if query != "ns=team%2Fa" {
		t.Errorf("query = %q, want ns=team%%2Fa", query)
	}
	want := []backupGroup{
		{Store: "my/store", Namespace: "team/a", Type: "vm", ID: "100", BackupCount: 5, LastBackup: 1767225600},
		{Store: "my/store", Namespace: "team/a", Type: "ct", ID: "200", BackupCount: 2, LastBackup: 1767225601},
		{Store: "my/store", Namespace: "team/a", Type: "host", ID: "nas"},
	}
	if len(groups) != len(want) {
		t.Fatalf("groups = %+v", groups)
	}
	for i := range want {
		if groups[i] != want[i] {
			t.Errorf("group %d = %+v, want %+v", i, groups[i], want[i])
		}
	}
}

func TestListGroupsEscapesStoreInPathAndOmitsRootNamespace(t *testing.T) {
	var path, query string
	c := testPBS(t, func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.EscapedPath(), r.URL.RawQuery
		_, _ = fmt.Fprint(w, `{"data":[]}`)
	})
	if _, err := c.listGroups(context.Background(), "my/store", ""); err != nil {
		t.Fatal(err)
	}
	if path != "/api2/json/admin/datastore/my%2Fstore/groups" || query != "" {
		t.Errorf("path = %q query = %q", path, query)
	}
}

func TestListJobsDecodeLooseTypes(t *testing.T) {
	c := testPBS(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/config/verify":
			_, _ = fmt.Fprint(w, `{"data":[{"id":"v1","store":"main","ignore-verified":"1","outdated-after":"30","max-depth":0},{"id":"v2","store":"main","ignore-verified":false}]}`)
		case "/api2/json/config/prune":
			_, _ = fmt.Fprint(w, `{"data":[{"id":"p1","store":"main","disable":1,"keep-last":"3","keep-daily":7,"max-depth":"2"},{"id":"p2","store":"main","disable":"false"}]}`)
		}
	})
	verify, err := c.listVerifyJobs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !bool(verify[0].IgnoreVerified) || verify[0].OutdatedAfter == nil || *verify[0].OutdatedAfter != 30 ||
		verify[0].MaxDepth == nil || *verify[0].MaxDepth != 0 || bool(verify[1].IgnoreVerified) || verify[1].OutdatedAfter != nil || verify[1].MaxDepth != nil {
		t.Errorf("verify jobs = %+v", verify)
	}
	prune, err := c.listPruneJobs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !bool(prune[0].Disable) || prune[0].KeepLast != 3 || prune[0].KeepDaily != 7 || prune[0].MaxDepth == nil || *prune[0].MaxDepth != 2 ||
		bool(prune[1].Disable) || prune[1].KeepLast != 0 || prune[1].MaxDepth != nil {
		t.Errorf("prune jobs = %+v", prune)
	}
}

func TestNewestVerifyState(t *testing.T) {
	tests := []struct {
		name, body, want string
	}{
		{"newest wins regardless of order", `{"data":[{"backup-time":"100","verification":{"state":"ok"}},{"backup-time":"200","verification":{"state":"failed"}},{"backup-time":150,"verification":{"state":"ok"}}]}`, "failed"},
		{"newest has no verification", `{"data":[{"backup-time":100,"verification":{"state":"ok"}},{"backup-time":200}]}`, "none"},
		{"empty list", `{"data":[]}`, "none"},
		{"unknown state", `{"data":[{"backup-time":1,"verification":{"state":"weird"}}]}`, "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var query string
			c := testPBS(t, func(w http.ResponseWriter, r *http.Request) {
				query = r.URL.Query().Encode()
				_, _ = fmt.Fprint(w, tt.body)
			})
			got, err := c.newestVerifyState(context.Background(), backupGroup{Store: "s", Namespace: "team/a", Type: "vm", ID: "100"})
			if err != nil || got != tt.want {
				t.Fatalf("state = %q, err = %v, want %q", got, err, tt.want)
			}
			if query != "backup-id=100&backup-type=vm&ns=team%2Fa" {
				t.Errorf("query = %q", query)
			}
		})
	}
}

func TestStatusMapping(t *testing.T) {
	for status, check := range map[int]func(error) bool{
		http.StatusUnauthorized:       func(err error) bool { var e *connector.AuthError; return errors.As(err, &e) },
		http.StatusForbidden:          func(err error) bool { var e *connector.AuthError; return errors.As(err, &e) },
		http.StatusBadGateway:         func(err error) bool { var e *connector.ServiceUnavailableError; return errors.As(err, &e) },
		http.StatusServiceUnavailable: func(err error) bool { var e *connector.ServiceUnavailableError; return errors.As(err, &e) },
		http.StatusGatewayTimeout:     func(err error) bool { var e *connector.ServiceUnavailableError; return errors.As(err, &e) },
	} {
		c := testPBS(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) })
		if _, err := c.listDatastores(context.Background()); !check(err) {
			t.Errorf("status %d mapped to %T: %v", status, err, err)
		}
	}
}

func TestErrorsNeverEchoResponseBodyOrSecret(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusUnauthorized, http.StatusServiceUnavailable} {
		c := testPBS(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = fmt.Fprint(w, "body-leak-marker "+testTokenSecret)
		})
		_, err := c.listDatastores(context.Background())
		if err == nil {
			t.Fatalf("status %d: want error", status)
		}
		if strings.Contains(err.Error(), "body-leak-marker") || strings.Contains(err.Error(), testTokenSecret) {
			t.Errorf("status %d error leaks upstream body or secret: %v", status, err)
		}
	}
}

func TestMalformedResponses(t *testing.T) {
	for name, body := range map[string]string{
		"missing data key": `{}`,
		"invalid json":     `{invalid`,
		"wrong data shape": `{"data":{"id":"x"}}`,
		"bad boolean":      `{"data":[{"id":"v","ignore-verified":"maybe"}]}`,
	} {
		c := testPBS(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, body) })
		_, err := c.listVerifyJobs(context.Background())
		var malformed *connector.MalformedResponseError
		if !errors.As(err, &malformed) {
			t.Errorf("%s: error %T %v, want MalformedResponseError", name, err, err)
		} else if strings.Contains(err.Error(), "maybe") {
			t.Errorf("%s: error echoes payload: %v", name, err)
		}
	}
}

func TestTransportErrorKeepsCauseAndRedactsSecret(t *testing.T) {
	c := testPBS(t, func(http.ResponseWriter, *http.Request) {})
	c.url = "http://127.0.0.1:1" // nothing listens here
	_, err := c.listDatastores(context.Background())
	if err == nil || !strings.Contains(err.Error(), "request failed") || !strings.Contains(err.Error(), "connect") {
		t.Fatalf("error = %v, want a wrapped connection failure", err)
	}
	if strings.Contains(err.Error(), testTokenSecret) {
		t.Errorf("error leaks secret: %v", err)
	}
}

func TestURLNormalizationReachesSamePath(t *testing.T) {
	for _, suffix := range []string{"", "/", "/api2/json", "/api2/json/"} {
		var path string
		c := testPBS(t, func(w http.ResponseWriter, r *http.Request) {
			path = r.URL.Path
			_, _ = fmt.Fprint(w, `{"data":[]}`)
		})
		c.url = normalizeURL(c.url + suffix)
		if _, err := c.listDatastores(context.Background()); err != nil || path != "/api2/json/admin/datastore" {
			t.Errorf("suffix %q: path = %q, err = %v", suffix, path, err)
		}
	}
}
