package pbs_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	_ "github.com/WiseLabz/wiselabz/internal/connector/pbs"
	syncdiff "github.com/WiseLabz/wiselabz/internal/sync"
)

const stabilityTokenSecret = "pbs-stability-secret-value"

// pbsFixture is one rendering of the same upstream data. Variant 1 reorders
// every array and writes numbers and booleans in their string/0-1 forms, which
// Proxmox-family APIs are known to emit.
func pbsFixture(variant int, recent, older int64) map[string]string {
	if variant == 0 {
		return map[string]string{
			"/api2/json/admin/datastore":                   `{"data":[{"store":"main","comment":"primary","backend-type":"filesystem"},{"store":"archive"}]}`,
			"/api2/json/admin/datastore/main/namespace":    `{"data":[{"ns":"team/a"},{"ns":"team"}]}`,
			"/api2/json/admin/datastore/archive/namespace": `{"data":[]}`,
			"main/":                    fmt.Sprintf(`{"data":[{"backup-type":"vm","backup-id":"100","backup-count":2,"last-backup":%d},{"backup-type":"ct","backup-id":"200","backup-count":1,"last-backup":%d},{"backup-type":"host","backup-id":"nas","backup-count":4,"last-backup":%d}]}`, older, recent, recent),
			"main/team/a":              fmt.Sprintf(`{"data":[{"backup-type":"vm","backup-id":"100","backup-count":3,"last-backup":%d}]}`, recent),
			"main/team":                `{"data":[]}`,
			"archive/":                 fmt.Sprintf(`{"data":[{"backup-type":"vm","backup-id":"300","backup-count":1,"last-backup":%d}]}`, older),
			"/api2/json/config/verify": `{"data":[{"id":"v1","store":"main","schedule":"daily","ignore-verified":true,"outdated-after":30},{"id":"v2","store":"archive","ns":"x","max-depth":0}]}`,
			"/api2/json/config/prune":  `{"data":[{"id":"p1","store":"main","schedule":"daily","keep-last":3,"keep-daily":7},{"id":"p2","store":"archive","disable":true,"keep-weekly":4,"max-depth":1}]}`,
			"snap:main:team/a:vm:100":  fmt.Sprintf(`{"data":[{"backup-type":"vm","backup-id":"100","backup-time":%d,"verification":{"state":"ok"}},{"backup-type":"vm","backup-id":"100","backup-time":%d,"verification":{"state":"failed"}}]}`, recent, older),
		}
	}
	return map[string]string{
		"/api2/json/admin/datastore":                   `{"data":[{"store":"archive"},{"store":"main","comment":"primary","backend-type":"filesystem"}]}`,
		"/api2/json/admin/datastore/main/namespace":    `{"data":[{"ns":"team"},{"ns":"team/a"}]}`,
		"/api2/json/admin/datastore/archive/namespace": `{"data":null}`,
		"main/":                    fmt.Sprintf(`{"data":[{"backup-type":"host","backup-id":"nas","backup-count":"4","last-backup":"%d"},{"backup-type":"ct","backup-id":200,"backup-count":"1","last-backup":"%d"},{"backup-type":"vm","backup-id":"100","backup-count":"2","last-backup":"%d"}]}`, recent, recent, older),
		"main/team/a":              fmt.Sprintf(`{"data":[{"backup-type":"vm","backup-id":"100","backup-count":"3","last-backup":"%d"}]}`, recent),
		"main/team":                `{"data":[]}`,
		"archive/":                 fmt.Sprintf(`{"data":[{"backup-type":"vm","backup-id":"300","backup-count":"1","last-backup":"%d"}]}`, older),
		"/api2/json/config/verify": `{"data":[{"id":"v2","store":"archive","ns":"x","max-depth":"0"},{"id":"v1","store":"main","schedule":"daily","ignore-verified":1,"outdated-after":"30"}]}`,
		"/api2/json/config/prune":  `{"data":[{"id":"p2","store":"archive","disable":1,"keep-weekly":"4","max-depth":"1"},{"id":"p1","store":"main","schedule":"daily","keep-daily":"7","keep-last":"3"}]}`,
		"snap:main:team/a:vm:100":  fmt.Sprintf(`{"data":[{"backup-type":"vm","backup-id":"100","backup-time":"%d","verification":{"state":"failed"}},{"backup-type":"vm","backup-id":"100","backup-time":"%d","verification":{"state":"ok"}}]}`, older, recent),
	}
}

func TestFetchStableAcrossReorderingAndLooseEncodings(t *testing.T) {
	// Mid-day offsets keep the whole-day ages away from a day boundary.
	now := time.Now()
	recent := now.Add(-84 * time.Hour).Unix()
	older := now.Add(-300 * time.Hour).Unix()
	fixtures := [2]map[string]string{pbsFixture(0, recent, older), pbsFixture(1, recent, older)}
	variant := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("PBS received %s %s, want GET only", r.Method, r.URL.Path)
			http.Error(w, "read-only", http.StatusMethodNotAllowed)
			return
		}
		if want := "PBSAPIToken=root@pbs!sync:" + stabilityTokenSecret; r.Header.Get("Authorization") != want {
			http.Error(w, "bad token", http.StatusUnauthorized)
			return
		}
		key := r.URL.Path
		if strings.HasSuffix(r.URL.Path, "/groups") {
			store := strings.Split(strings.TrimPrefix(r.URL.Path, "/api2/json/admin/datastore/"), "/")[0]
			key = store + "/" + r.URL.Query().Get("ns")
		}
		if strings.HasSuffix(r.URL.Path, "/snapshots") {
			store := strings.Split(strings.TrimPrefix(r.URL.Path, "/api2/json/admin/datastore/"), "/")[0]
			q := r.URL.Query()
			key = "snap:" + store + ":" + q.Get("ns") + ":" + q.Get("backup-type") + ":" + q.Get("backup-id")
		}
		body, ok := fixtures[variant][key]
		if !ok {
			if strings.HasSuffix(r.URL.Path, "/snapshots") {
				body = `{"data":[]}`
			} else {
				http.NotFound(w, r)
				return
			}
		}
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	connector.AllowLoopbackForTest(t)
	created, err := connector.Get("pbs", map[string]any{"url": server.URL, "token_id": "root@pbs!sync", "token_secret": stabilityTokenSecret, "verify_tls": true})
	if err != nil {
		t.Fatalf("connector.Get: %v", err)
	}
	fetch := func(v int) *connector.ServiceSnapshot {
		variant = v
		sn, err := created.Fetch(context.Background(), nil)
		if err != nil {
			t.Fatalf("Fetch variant %d: %v", v, err)
		}
		for _, section := range sn.Sections {
			if section.Error != "" {
				t.Fatalf("variant %d section %q failed: %s", v, section.Title, section.Error)
			}
		}
		return sn
	}
	first, repeated, reordered := fetch(0), fetch(0), fetch(1)

	kinds := map[string]int{}
	for _, e := range first.Entities {
		kinds[e.Kind]++
		if e.IP != "" || e.MAC != "" {
			t.Errorf("entity %s/%s has IP %q MAC %q, want none", e.Kind, e.Name, e.IP, e.MAC)
		}
	}
	want := map[string]int{"datastore": 2, "verify_job": 2, "prune_job": 2, "vm": 2, "container": 1, "host": 1}
	for kind, n := range want {
		if kinds[kind] != n {
			t.Errorf("entities of kind %q = %d, want %d (all kinds: %v)", kind, kinds[kind], n, kinds)
		}
	}
	for _, e := range first.Entities {
		if e.Kind == "vm" && e.ExternalID == "100" {
			if e.Attributes["last_backup_age_days"] != float64(3) || e.Attributes["backup_count"] != float64(5) ||
				e.Attributes["namespace"] != "team/a" || e.Attributes["verify_state"] != "ok" {
				t.Errorf("vm 100 attributes = %v, want newest group team/a, 5 backups, 3 days, verify ok", e.Attributes)
			}
		}
	}

	assertNoChanges(t, "identical", first, repeated)
	assertNoChanges(t, "reordered and re-encoded", first, reordered)

	var persisted [3]connector.ServiceSnapshot
	for i, sn := range []*connector.ServiceSnapshot{first, repeated, reordered} {
		raw, err := json.Marshal(sn)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if strings.Contains(string(raw), stabilityTokenSecret) {
			t.Fatal("snapshot JSON contains the token secret")
		}
		if err := json.Unmarshal(raw, &persisted[i]); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
	}
	assertNoChanges(t, "JSON round-trip identical", &persisted[0], &persisted[1])
	assertNoChanges(t, "JSON round-trip reordered", &persisted[0], &persisted[2])
}

func assertNoChanges(t *testing.T, label string, prev, curr *connector.ServiceSnapshot) {
	t.Helper()
	if changes := syncdiff.Compare(prev, curr); len(changes) != 0 {
		t.Errorf("%s section changes = %+v, want none", label, changes)
	}
	if changes := syncdiff.CompareEntities(prev.Entities, curr.Entities); len(changes) != 0 {
		t.Errorf("%s entity changes = %+v, want none", label, changes)
	}
	if changes := syncdiff.CompareDependencies(prev.Dependencies, curr.Dependencies); len(changes) != 0 {
		t.Errorf("%s dependency changes = %+v, want none", label, changes)
	}
}

// TestBackupDoesNotChangeSections proves a new backup (higher count, newer
// time, different verify result) moves entity attributes only.
func TestBackupDoesNotChangeSections(t *testing.T) {
	now := time.Now()
	older := now.Add(-300 * time.Hour).Unix()
	state := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api2/json/admin/datastore":
			_, _ = w.Write([]byte(`{"data":[{"store":"main"}]}`))
		case strings.HasSuffix(r.URL.Path, "/namespace"):
			_, _ = w.Write([]byte(`{"data":[]}`))
		case strings.HasSuffix(r.URL.Path, "/groups"):
			count, last := 1, older
			if state == 1 {
				count, last = 2, now.Add(-time.Hour).Unix()
			}
			_, _ = fmt.Fprintf(w, `{"data":[{"backup-type":"vm","backup-id":"100","backup-count":%d,"last-backup":%d}]}`, count, last)
		case strings.HasSuffix(r.URL.Path, "/snapshots"):
			verdict := "failed"
			if state == 1 {
				verdict = "ok"
			}
			_, _ = fmt.Fprintf(w, `{"data":[{"backup-time":1,"verification":{"state":%q}}]}`, verdict)
		default:
			_, _ = w.Write([]byte(`{"data":[]}`))
		}
	}))
	defer server.Close()
	connector.AllowLoopbackForTest(t)
	created, err := connector.Get("pbs", map[string]any{"url": server.URL, "token_id": "root@pbs!sync", "token_secret": "s"})
	if err != nil {
		t.Fatal(err)
	}
	before, err := created.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	state = 1
	after, err := created.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if changes := syncdiff.Compare(before, after); len(changes) != 0 {
		t.Errorf("a new backup changed section content: %+v", changes)
	}
	if len(before.Entities) != 2 || len(after.Entities) != 2 {
		t.Fatalf("entities = %d / %d, want datastore + vm each", len(before.Entities), len(after.Entities))
	}
	var vmBefore, vmAfter map[string]any
	for _, e := range before.Entities {
		if e.Kind == "vm" {
			vmBefore = e.Attributes
		}
	}
	for _, e := range after.Entities {
		if e.Kind == "vm" {
			vmAfter = e.Attributes
		}
	}
	if vmBefore["last_backup_age_days"] == vmAfter["last_backup_age_days"] || vmBefore["verify_state"] == vmAfter["verify_state"] {
		t.Errorf("attributes did not reflect the new backup: before %v after %v", vmBefore, vmAfter)
	}
}
