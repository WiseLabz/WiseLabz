package compliance_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/compliance"
	"github.com/WiseLabz/wiselabz/internal/connector"
	_ "github.com/WiseLabz/wiselabz/internal/connector/pbs"
	_ "github.com/WiseLabz/wiselabz/internal/connector/proxmox"
)

// pveServer serves a small Proxmox VE: VMs 100 (backed up), 101 (never backed
// up), 102 (old backup), 103 (tagged no-backup), 104 (template) and containers
// 200 (backed up) and 201 (never backed up, but a VM 201 has a backup).
func pveServer(t *testing.T) *httptest.Server {
	t.Helper()
	templates := map[string]bool{"104": true}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("PVE received %s %s, want GET only", r.Method, r.URL.Path)
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		switch {
		case r.URL.Path == "/nodes":
			_, _ = w.Write([]byte(`{"data":[{"node":"pve1","status":"online","maxmem":8589934592}]}`))
		case r.URL.Path == "/nodes/pve1/qemu":
			_, _ = w.Write([]byte(`{"data":[
				{"vmid":100,"name":"backed-up","status":"running","cpus":1,"maxmem":1048576,"tags":"prod"},
				{"vmid":101,"name":"never-backed-up","status":"running","cpus":1,"maxmem":1048576,"tags":"prod;web"},
				{"vmid":102,"name":"old-backup","status":"stopped","cpus":1,"maxmem":1048576},
				{"vmid":103,"name":"opted-out","status":"running","cpus":1,"maxmem":1048576,"tags":"web;no-backup"},
				{"vmid":104,"name":"template","status":"stopped","cpus":1,"maxmem":1048576}
			]}`))
		case r.URL.Path == "/nodes/pve1/lxc":
			_, _ = w.Write([]byte(`{"data":[
				{"vmid":200,"name":"ct-backed-up","status":"running","cpus":1,"maxmem":1048576},
				{"vmid":201,"name":"ct-never-backed-up","status":"running","cpus":1,"maxmem":1048576}
			]}`))
		case r.URL.Path == "/nodes/pve1/storage":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case len(parts) == 5 && parts[4] == "config":
			template := 0
			if templates[parts[3]] {
				template = 1
			}
			_, _ = fmt.Fprintf(w, `{"data":{"template":%d}}`, template)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// pbsServer serves a PBS whose groups are: vm/100 fresh, vm/102 ten days old,
// vm/201 fresh (a VM, not the container 201), ct/200 fresh.
func pbsServer(t *testing.T, now time.Time) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("PBS received %s %s, want GET only", r.Method, r.URL.Path)
		}
		switch r.URL.Path {
		case "/api2/json/admin/datastore":
			_, _ = w.Write([]byte(`{"data":[{"store":"main"}]}`))
		case "/api2/json/admin/datastore/main/namespace":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/api2/json/admin/datastore/main/groups":
			_, _ = fmt.Fprintf(w, `{"data":[
				{"backup-type":"vm","backup-id":"100","backup-count":3,"last-backup":%d},
				{"backup-type":"vm","backup-id":"102","backup-count":"1","last-backup":"%d"},
				{"backup-type":"vm","backup-id":"201","backup-count":2,"last-backup":%d},
				{"backup-type":"ct","backup-id":"200","backup-count":2,"last-backup":%d}
			]}`, now.Add(-time.Hour).Unix(), now.Add(-10*24*time.Hour).Unix(), now.Add(-time.Hour).Unix(), now.Add(-2*time.Hour).Unix())
		case "/api2/json/admin/datastore/main/snapshots":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/api2/json/config/verify", "/api2/json/config/prune":
			_, _ = w.Write([]byte(`{"data":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func fetchSnapshot(t *testing.T, typ string, config map[string]any) compliance.Snapshot {
	t.Helper()
	c, err := connector.Get(typ, config)
	if err != nil {
		t.Fatalf("connector.Get(%s): %v", typ, err)
	}
	sn, err := c.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("%s Fetch: %v", typ, err)
	}
	for _, section := range sn.Sections {
		if section.Error != "" {
			t.Fatalf("%s section %q failed: %s", typ, section.Title, section.Error)
		}
	}
	return compliance.SnapshotFromConnector(*sn)
}

func realCatalog() compliance.Catalog {
	source := connector.AttributeCatalog()
	out := make(compliance.Catalog, len(source))
	for typ, kinds := range source {
		out[typ] = map[string][]compliance.AttributeSpec{}
		for kind, specs := range kinds {
			for _, spec := range specs {
				out[typ][kind] = append(out[typ][kind], compliance.AttributeSpec{Name: spec.Name, Type: spec.Type})
			}
		}
	}
	return out
}

func backupRules(t *testing.T) map[string]compliance.Rule {
	t.Helper()
	pack, ok, err := compliance.FindPack("recommended")
	if err != nil || !ok {
		t.Fatalf("FindPack(recommended) = %v, %v", ok, err)
	}
	rules := map[string]compliance.Rule{}
	for _, rule := range pack.Rules {
		if rule.ConnectorType == "proxmox" && len(rule.Related) > 0 {
			rules[rule.EntityKind] = rule
		}
	}
	if len(rules) != 2 {
		t.Fatalf("recommended pack has %d proxmox rules with related clauses, want 2 (vm, container)", len(rules))
	}
	return rules
}

func names(entities []compliance.Entity) []string {
	out := make([]string, 0, len(entities))
	for _, e := range entities {
		out = append(out, e.Name)
	}
	sort.Strings(out)
	return out
}

func equalNames(got []string, want ...string) bool {
	sort.Strings(want)
	return strings.Join(got, ",") == strings.Join(want, ",")
}

// TestRecommendedBackupRulesAgainstRealConnectors runs the shipped pack rules
// over snapshots produced by the real Proxmox and PBS connectors.
func TestRecommendedBackupRulesAgainstRealConnectors(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	pve := pveServer(t)
	pbsSrv := pbsServer(t, time.Now())

	pveSnapshot := fetchSnapshot(t, "proxmox", map[string]any{"url": pve.URL, "token_id": "root@pam!t", "token_secret": "s"})
	pbsSnapshot := fetchSnapshot(t, "pbs", map[string]any{"url": pbsSrv.URL, "token_id": "root@pam!t", "token_secret": "s"})
	rules := backupRules(t)
	catalog := realCatalog()
	for kind, rule := range rules {
		if err := compliance.Validate(rule, catalog); err != nil {
			t.Fatalf("pack rule for %s fails validation against the real catalogs: %v", kind, err)
		}
	}
	related := compliance.RelatedEntities{"pbs": pbsSnapshot.Entities}

	vmFlagged, skipped := compliance.EvaluateWithRelated(rules["vm"], pveSnapshot, related)
	if skipped {
		t.Fatal("vm rule skipped although a PBS snapshot exists")
	}
	if got := names(vmFlagged); !equalNames(got, "never-backed-up", "old-backup") {
		t.Errorf("vm rule flagged %v, want [never-backed-up old-backup] (not the fresh backup, the no-backup tag or the template)", got)
	}

	ctFlagged, skipped := compliance.EvaluateWithRelated(rules["container"], pveSnapshot, related)
	if skipped {
		t.Fatal("container rule skipped although a PBS snapshot exists")
	}
	if got := names(ctFlagged); !equalNames(got, "ct-never-backed-up") {
		t.Errorf("container rule flagged %v, want [ct-never-backed-up] (a PBS vm/201 must not satisfy container 201)", got)
	}

	for kind, rule := range rules {
		if flagged, skipped := compliance.EvaluateWithRelated(rule, pveSnapshot, compliance.RelatedEntities{}); !skipped || flagged != nil {
			t.Errorf("%s rule without a PBS snapshot: skipped=%v flagged=%v, want skipped and nothing flagged", kind, skipped, names(flagged))
		}
	}
}
