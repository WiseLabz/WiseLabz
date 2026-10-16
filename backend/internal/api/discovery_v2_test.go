package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api"
	discoveryhandler "github.com/WiseLabz/wiselabz/internal/api/discovery"
	"github.com/WiseLabz/wiselabz/internal/connector"
	disc "github.com/WiseLabz/wiselabz/internal/discovery"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func newDiscoveryV2App(t *testing.T, runner disc.Runner, opts discoveryhandler.Options) *testApp {
	t.Helper()
	opts.Runner = runner
	return newTestAppWithOptions(t, t.TempDir(), nil, func(c *api.Config) {
		c.Discovery = opts
	})
}

func postDiscovery(t *testing.T, app *testApp, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return app.req(t, http.MethodPost, "/api/discovery/scan", body, token)
}

func fieldError(t *testing.T, rec *httptest.ResponseRecorder) (string, string) {
	t.Helper()
	var body struct {
		Details []struct {
			Field string `json:"field"`
			Msg   string `json:"msg"`
		} `json:"details"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode field error: %v; body = %s", err, rec.Body)
	}
	if len(body.Details) != 1 {
		t.Fatalf("field errors = %+v; body = %s", body.Details, rec.Body)
	}
	return body.Details[0].Field, body.Details[0].Msg
}

func TestDiscoveryAcceptedRangeScenarios(t *testing.T) {
	cases := []struct {
		name  string
		body  any
		cidr  string
		total int
	}{
		{"private /24", map[string]any{"ranges": []string{"192.168.1.0/24"}}, "192.168.1.0/24", 254},
		{"several ranges", map[string]any{"ranges": []string{"192.168.1.0/24", "192.168.20.0/24"}}, "192.168.1.0/24", 508},
		{"overlapping ranges deduplicate hosts", map[string]any{"ranges": []string{"192.168.1.0/24", "192.168.1.128/25"}}, "192.168.1.0/24", 254},
		{"mesh range", map[string]any{"ranges": []string{"100.100.4.0/24"}}, "100.100.4.0/24", 254},
		{"host bits set", map[string]any{"ranges": []string{"192.168.1.57/24"}}, "192.168.1.0/24", 254},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scanner := newScriptedScanner(false)
			app := newDiscoveryV2App(t, scanner, discoveryhandler.Options{})
			stepUp(app, false)
			_, token := app.user(t, "operator")
			rec := postDiscovery(t, app, token, tc.body)
			if rec.Code != http.StatusAccepted {
				t.Fatalf("start = %d, want 202; body = %s", rec.Code, rec.Body)
			}
			started := decodeScan(t, rec)
			if started.CIDR != tc.cidr || started.Total != tc.total || len(started.Ranges) == 0 || started.Sources.Range != tc.total {
				t.Errorf("started scan = %+v", started)
			}
			if ended := waitScanEnded(t, app, token); ended.State != disc.StateCompleted || ended.Done != tc.total {
				t.Errorf("ended scan = %+v", ended)
			}
		})
	}
}

func TestDiscoveryV2InvalidRangesNameIndexedFieldAndDoNotStart(t *testing.T) {
	cases := []struct {
		name, cidr, wantMessage string
	}{
		{"too wide", "10.0.0.0/21", "at most a /22"},
		{"public", "8.8.8.0/24", "only private ranges"},
		{"loopback", "127.0.0.0/24", "only private ranges"},
		{"link local", "169.254.1.0/24", "only private ranges"},
		{"IPv6", "fd00::/120", "IPv4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scanner := newScriptedScanner(false)
			app := newDiscoveryV2App(t, scanner, discoveryhandler.Options{})
			stepUp(app, false)
			_, token := app.user(t, "operator")
			rec := postDiscovery(t, app, token, map[string]any{"ranges": []string{tc.cidr}})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("start = %d, want 400; body = %s", rec.Code, rec.Body)
			}
			field, msg := fieldError(t, rec)
			if field != "ranges[0]" || !strings.Contains(msg, tc.wantMessage) {
				t.Errorf("field error = (%q, %q), want ranges[0] with %q", field, msg, tc.wantMessage)
			}
			if scanner.calls.Load() != 0 {
				t.Errorf("scanner ran %d times", scanner.calls.Load())
			}
		})
	}
}

func TestDiscoveryV1AndRangesPresencePrecedence(t *testing.T) {
	var inventoryCalls atomic.Int32
	opts := discoveryhandler.Options{
		InventoryQuery: func(context.Context) ([]store.IndexedAddress, error) {
			inventoryCalls.Add(1)
			return nil, errors.New("the v1 request must not read inventory")
		},
	}
	scanner := newScriptedScanner(false)
	app := newDiscoveryV2App(t, scanner, opts)
	stepUp(app, false)
	_, token := app.user(t, "operator")

	legacy := postDiscovery(t, app, token, map[string]any{
		"cidr": "192.168.1.0/24", "includeInventory": true, "askNetwork": true,
	})
	if legacy.Code != http.StatusAccepted {
		t.Fatalf("legacy start = %d; body = %s", legacy.Code, legacy.Body)
	}
	started := decodeScan(t, legacy)
	if started.IncludeInventory || started.AskNetwork || started.Total != 254 {
		t.Errorf("earlier request form scan = %+v", started)
	}
	waitScanEnded(t, app, token)
	if got := inventoryCalls.Load(); got != 0 {
		t.Errorf("legacy request queried inventory %d times", got)
	}

	// A present ranges array wins even when the deprecated field is also sent.
	current := postDiscovery(t, app, token, map[string]any{
		"ranges": []string{"100.100.4.0/24"}, "cidr": "8.8.8.0/24",
	})
	if current.Code != http.StatusAccepted {
		t.Fatalf("ranges + cidr start = %d; body = %s", current.Code, current.Body)
	}
	if got := decodeScan(t, current); got.CIDR != "100.100.4.0/24" || got.Total != 254 {
		t.Errorf("ranges field did not win: %+v", got)
	}
	waitScanEnded(t, app, token)
}

func TestDiscoveryRejectsNullAndEmptyRangesWithoutFallingBackToCIDR(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"null ranges", `{"ranges":null,"cidr":"192.168.1.0/24"}`},
		{"empty ranges", `{"ranges":[],"cidr":"192.168.1.0/24"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scanner := newScriptedScanner(false)
			app := newDiscoveryV2App(t, scanner, discoveryhandler.Options{})
			stepUp(app, false)
			_, token := app.user(t, "operator")
			rec := app.req(t, http.MethodPost, "/api/discovery/scan", json.RawMessage(tc.body), token)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("start = %d, want 400; body = %s", rec.Code, rec.Body)
			}
			if tc.name == "empty ranges" {
				field, msg := fieldError(t, rec)
				if field != "ranges" || !strings.Contains(msg, "include known hosts") {
					t.Errorf("field error = (%q, %q)", field, msg)
				}
			}
			if scanner.calls.Load() != 0 {
				t.Errorf("scanner ran %d times after rejected request", scanner.calls.Load())
			}
		})
	}
}

func TestDiscoveryNothingToScanAndRangesOverBudget(t *testing.T) {
	t.Run("nothing to scan", func(t *testing.T) {
		scanner := newScriptedScanner(false)
		app := newDiscoveryV2App(t, scanner, discoveryhandler.Options{})
		stepUp(app, false)
		_, token := app.user(t, "operator")
		rec := postDiscovery(t, app, token, map[string]any{"ranges": []string{}, "askNetwork": true})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("start = %d, want 400; body = %s", rec.Code, rec.Body)
		}
		field, msg := fieldError(t, rec)
		if field != "ranges" || !strings.Contains(msg, "include known hosts") {
			t.Errorf("field error = (%q, %q)", field, msg)
		}
		if scanner.calls.Load() != 0 {
			t.Errorf("scanner ran %d times", scanner.calls.Load())
		}
	})

	t.Run("ranges over budget", func(t *testing.T) {
		scanner := newScriptedScanner(false)
		app := newDiscoveryV2App(t, scanner, discoveryhandler.Options{})
		stepUp(app, false)
		_, token := app.user(t, "operator")
		cidrs := []string{"192.168.1.0/24", "192.168.2.0/24", "192.168.3.0/24", "192.168.4.0/24", "192.168.5.0/24"}
		rec := postDiscovery(t, app, token, map[string]any{"ranges": cidrs})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("start = %d, want 400; body = %s", rec.Code, rec.Body)
		}
		field, msg := fieldError(t, rec)
		if field != "ranges" || !strings.Contains(msg, "at most 1024") {
			t.Errorf("field error = (%q, %q)", field, msg)
		}
		if scanner.calls.Load() != 0 {
			t.Errorf("scanner ran %d times", scanner.calls.Load())
		}
		rejects := auditEntries(t, app, disc.AuditReject)
		if len(rejects) != 1 {
			t.Fatalf("reject audit entries = %+v", rejects)
		}
		detail := detailOf(t, rejects[0])
		if detail["reason"] != disc.ReasonTooManyAddresses || len(detail["ranges"].([]any)) != 5 {
			t.Errorf("reject detail = %v", detail)
		}
	})
}

func TestDiscoveryInventoryReadFailureRejectsWithoutStarting(t *testing.T) {
	scanner := newScriptedScanner(false)
	opts := discoveryhandler.Options{
		InventoryQuery: func(context.Context) ([]store.IndexedAddress, error) {
			return nil, errors.New("database failure mentioning 10.12.13.14")
		},
	}
	app := newDiscoveryV2App(t, scanner, opts)
	stepUp(app, false)
	_, token := app.user(t, "operator")
	rec := postDiscovery(t, app, token, map[string]any{
		"ranges": []string{"192.168.1.0/24"}, "includeInventory": true,
	})
	if rec.Code != http.StatusInternalServerError || decodeErr(t, rec).Code != "internal_error" {
		t.Fatalf("start = %d %s, want safe 500; body = %s", rec.Code, decodeErr(t, rec).Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "10.12.13.14") || scanner.calls.Load() != 0 {
		t.Errorf("failure leaked an address or started scanner: body=%s calls=%d", rec.Body, scanner.calls.Load())
	}
	if starts := auditEntries(t, app, disc.AuditStart); len(starts) != 0 {
		t.Errorf("start audit entries after failed read = %+v", starts)
	}
	rejects := auditEntries(t, app, disc.AuditReject)
	if len(rejects) != 1 {
		t.Fatalf("reject audit entries = %+v", rejects)
	}
	detail := detailOf(t, rejects[0])
	if detail["reason"] != "internal" || fmt.Sprint(detail["ranges"]) != "[192.168.1.0/24]" {
		t.Errorf("reject detail = %v", detail)
	}
}

func TestDiscoveryUnscopedInventoryIsGuardedByInstanceAdmin(t *testing.T) {
	var inventoryCalls, interfaceCalls atomic.Int32
	scanner := newScriptedScanner(false)
	opts := discoveryhandler.Options{
		InventoryQuery: func(context.Context) ([]store.IndexedAddress, error) {
			inventoryCalls.Add(1)
			return nil, nil
		},
		InterfaceAddrs: func() ([]net.Addr, error) {
			interfaceCalls.Add(1)
			return nil, nil
		},
	}
	app := newDiscoveryV2App(t, scanner, opts)
	stepUp(app, false)
	_, viewer := app.user(t, "viewer")
	for _, request := range []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/discovery/suggestions", nil},
		{http.MethodPost, "/api/discovery/scan", map[string]any{"ranges": []string{}, "includeInventory": true}},
	} {
		if rec := app.req(t, request.method, request.path, request.body, viewer); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s = %d, want 403; body = %s", request.method, request.path, rec.Code, rec.Body)
		}
	}
	if inventoryCalls.Load() != 0 || interfaceCalls.Load() != 0 || scanner.calls.Load() != 0 {
		t.Errorf("forbidden requests reached dependencies: inventory=%d interfaces=%d scanner=%d",
			inventoryCalls.Load(), interfaceCalls.Load(), scanner.calls.Load())
	}
}

func TestDiscoverySuggestionsRankInventoryAndIncludeCGNAT(t *testing.T) {
	rows := append([]store.IndexedAddress{}, rowsInPrefix("10.0.20.0/24", 12)...)
	rows = append(rows, rowsInPrefix("10.0.30.0/24", 3)...)
	rows = append(rows, rowsInPrefix("10.0.40.0/24", 3)...)
	rows = append(rows, rowsInPrefix("10.0.50.0/24", 3)...)
	rows = append(rows, rowsInPrefix("10.0.60.0/24", 2)...)
	rows = append(rows,
		store.IndexedAddress{ConnectorType: "proxmox", Kind: "vm", IP: "100.64.0.20"},
		store.IndexedAddress{ConnectorType: "proxmox", Kind: "vm", IP: "fd00::25"},
		store.IndexedAddress{ConnectorType: "proxmox", Kind: "container", IP: "10.0.70.1"},
		store.IndexedAddress{ConnectorType: "docker", Kind: "container", IP: "10.0.80.1"},
		store.IndexedAddress{ConnectorType: "portainer", Kind: "container", IP: "10.0.90.1"},
		store.IndexedAddress{ConnectorType: "proxmox", Kind: "vm", IP: "8.8.8.8"},
		rows[0], // repeated index address is counted once
	)
	opts := discoveryhandler.Options{
		InterfaceAddrs: func() ([]net.Addr, error) {
			return []net.Addr{ipnet(t, "10.0.20.9/24"), ipnet(t, "100.64.0.8/24")}, nil
		},
		InventoryQuery: func(context.Context) ([]store.IndexedAddress, error) { return rows, nil },
	}
	app := newDiscoveryV2App(t, newScriptedScanner(false), opts)
	_, token := app.user(t, "operator")
	r := app.newRequest(t, http.MethodGet, "/api/discovery/suggestions", nil, token)
	r.RemoteAddr = "100.64.0.5:5555"
	rec := app.serve(r)
	if rec.Code != http.StatusOK {
		t.Fatalf("suggestions = %d; body = %s", rec.Code, rec.Body)
	}
	var body struct {
		Suggestions []struct {
			CIDR   string `json:"cidr"`
			Source string `json:"source"`
		} `json:"suggestions"`
		KnownHosts int `json:"knownHosts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	want := []struct{ cidr, source string }{
		{"100.64.0.0/24", "client"},
		{"10.0.20.0/24", "server"},
		{"10.0.30.0/24", "inventory"},
		{"10.0.40.0/24", "inventory"},
		{"10.0.50.0/24", "inventory"},
	}
	if len(body.Suggestions) != len(want) {
		t.Fatalf("suggestions = %+v; want %d entries", body.Suggestions, len(want))
	}
	for i, expected := range want {
		got := body.Suggestions[i]
		if got.CIDR != expected.cidr || got.Source != expected.source {
			t.Errorf("suggestion[%d] = %+v, want %+v", i, got, expected)
		}
	}
	if body.KnownHosts != 26 {
		t.Errorf("knownHosts = %d, want 26 including ULA and non-container Proxmox entities", body.KnownHosts)
	}
}

func rowsInPrefix(cidr string, count int) []store.IndexedAddress {
	prefix := netip.MustParsePrefix(cidr)
	rows := make([]store.IndexedAddress, 0, count)
	addr := prefix.Addr()
	for i := 0; i < count; i++ {
		addr = addr.Next()
		rows = append(rows, store.IndexedAddress{ConnectorType: "proxmox", Kind: "vm", IP: addr.String()})
	}
	return rows
}

func TestDiscoveryStartAuditPlansOnlyAdmittedInventory(t *testing.T) {
	rows := budgetInventoryRows(900)
	opts := discoveryhandler.Options{
		InventoryQuery: func(context.Context) ([]store.IndexedAddress, error) { return rows, nil },
	}
	app := newDiscoveryV2App(t, newScriptedScanner(false), opts)
	stepUp(app, false)
	_, token := app.user(t, "operator")
	rec := postDiscovery(t, app, token, map[string]any{
		"ranges": []string{"192.168.1.0/24", "192.168.2.0/24"}, "includeInventory": true,
	})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start = %d; body = %s", rec.Code, rec.Body)
	}
	started := decodeScan(t, rec)
	if started.Sources.Range != 508 || started.Sources.Inventory != 516 || started.Skipped.Inventory != 384 || started.Total != 1024 {
		t.Errorf("start counts = %+v", started)
	}
	entries := auditEntries(t, app, disc.AuditStart)
	if len(entries) != 1 {
		t.Fatalf("start audit = %+v", entries)
	}
	detail := detailOf(t, entries[0])
	planned, _ := detail["planned"].(map[string]any)
	skipped, _ := detail["skipped"].(map[string]any)
	if planned["range"] != float64(508) || planned["inventory"] != float64(516) || skipped["inventory"] != float64(384) {
		t.Errorf("start audit counts = %v", detail)
	}
	if strings.Contains(entries[0].Detail, "10.200.0.1") {
		t.Errorf("start audit contains a known host address: %s", entries[0].Detail)
	}
	waitScanEnded(t, app, token)
}

func budgetInventoryRows(count int) []store.IndexedAddress {
	rows := make([]store.IndexedAddress, 0, count)
	for i := 0; i < count; i++ {
		rows = append(rows, store.IndexedAddress{
			ConnectorType: "proxmox", Kind: "vm",
			IP: fmt.Sprintf("10.200.%d.%d", i/254, i%254+1),
		})
	}
	return rows
}

type passiveSourceFunc func(context.Context, func(netip.Addr))

func (f passiveSourceFunc) Run(ctx context.Context, report func(netip.Addr)) { f(ctx, report) }

func TestDiscoveryStartPassesInventoryAndPassiveOptions(t *testing.T) {
	rows := []store.IndexedAddress{
		{ConnectorType: "proxmox", Kind: "vm", IP: "10.0.0.1"},
		{ConnectorType: "proxmox", Kind: "vm", IP: "10.0.0.1"},
		{ConnectorType: "proxmox", Kind: "vm", IP: "100.64.0.2"},
		{ConnectorType: "proxmox", Kind: "vm", IP: "fd00::1"},
		{ConnectorType: "docker", Kind: "container", IP: "10.0.0.3"},
	}
	passiveCalls := atomic.Int32{}
	opts := discoveryhandler.Options{
		InventoryQuery: func(context.Context) ([]store.IndexedAddress, error) { return rows, nil },
		Passive: passiveSourceFunc(func(_ context.Context, report func(netip.Addr)) {
			passiveCalls.Add(1)
			report(netip.MustParseAddr("100.64.0.9"))
		}),
	}
	app := newDiscoveryV2App(t, newScriptedScanner(false), opts)
	stepUp(app, false)
	_, token := app.user(t, "operator")
	rec := postDiscovery(t, app, token, map[string]any{
		"ranges": []string{"192.168.1.0/24"}, "includeInventory": true, "askNetwork": true,
	})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start = %d; body = %s", rec.Code, rec.Body)
	}
	started := decodeScan(t, rec)
	if !started.IncludeInventory || !started.AskNetwork || started.Sources.Inventory != 3 {
		t.Errorf("started scan = %+v", started)
	}
	ended := waitScanEnded(t, app, token)
	if passiveCalls.Load() != 1 || ended.PassiveAnswered != 1 || ended.Sources.Passive != 1 || ended.Sources.Inventory != 3 {
		t.Errorf("ended scan = %+v; passive calls = %d", ended, passiveCalls.Load())
	}
}

func TestDiscoveryStartsWithInventoryOnly(t *testing.T) {
	rows := []store.IndexedAddress{
		{ConnectorType: "proxmox", Kind: "vm", IP: "10.0.0.1"},
		{ConnectorType: "proxmox", Kind: "vm", IP: "fd00::1"},
	}
	app := newDiscoveryV2App(t, newScriptedScanner(false), discoveryhandler.Options{
		InventoryQuery: func(context.Context) ([]store.IndexedAddress, error) { return rows, nil },
	})
	stepUp(app, false)
	_, token := app.user(t, "operator")
	rec := postDiscovery(t, app, token, map[string]any{"ranges": []string{}, "includeInventory": true})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("inventory-only start = %d; body = %s", rec.Code, rec.Body)
	}
	started := decodeScan(t, rec)
	if started.CIDR != "" || len(started.Ranges) != 0 || started.Total != 2 || started.Sources.Inventory != 2 || !started.IncludeInventory {
		t.Errorf("inventory-only scan = %+v", started)
	}
	if ended := waitScanEnded(t, app, token); ended.Sources.Inventory != 2 {
		t.Errorf("inventory-only completion = %+v", ended)
	}
}

type tlsTargetScanner struct {
	candidates []disc.Candidate
	targets    []disc.TLSTarget
}

func (s tlsTargetScanner) Run(_ context.Context, targets disc.Targets, cb disc.Callbacks) disc.Progress {
	for _, candidate := range s.candidates {
		cb.OnCandidate(candidate)
	}
	for _, target := range s.targets {
		cb.OnTLSTarget(target)
	}
	progress := disc.Progress{Done: len(targets.Static), Total: len(targets.Static)}
	cb.OnProgress(progress)
	return progress
}

func TestDiscoveryAPIRefreshesImportedAndManualTLSMarkers(t *testing.T) {
	ctx := context.Background()
	app := newDiscoveryV2App(t, tlsTargetScanner{
		candidates: []disc.Candidate{{
			Type: "opnsense", Name: "OPNsense", Address: "10.0.0.5", Port: 443,
			URL: "https://10.0.0.5:443", URLField: "url", Source: disc.SourceRange,
			AddOns: []disc.AddOn{{Type: "dnsresolver", URL: "https://10.0.0.5:443"}},
		}},
		targets: []disc.TLSTarget{
			{Address: "10.0.0.5", Port: 443},
			{Address: "10.0.0.5", Port: 8443},
		},
	}, discoveryhandler.Options{})
	stepUp(app, false)
	_, token := app.user(t, "operator")

	traefik := &store.ConnectorRecord{ID: "traefik-marker", Name: "Traefik", Category: "networking", Type: "traefik", URL: "http://10.0.0.2:8080", ConfigData: "{}"}
	probe := &store.ConnectorRecord{ID: "probe-marker", Name: "TLS Probe", Category: "monitoring", Type: "tlsprobe", ConfigData: `{"targets":"10.0.0.5:443","import_connector_id":"traefik-marker","import_port":8443}`}
	dns := &store.ConnectorRecord{ID: "dns-marker", Name: "DNS Resolver", Category: "dns", Type: "dnsresolver", URL: "https://10.0.0.5:443", ConfigData: "{}"}
	for _, record := range []*store.ConnectorRecord{traefik, probe, dns} {
		if err := app.Store.CreateConnector(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	createTraefikSnapshot(t, app.Store, traefik.ID, "2026-10-15T00:00:00Z", "10.0.0.5")

	rec := postDiscovery(t, app, token, map[string]any{"ranges": []string{"10.0.0.0/24"}})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start = %d; body = %s", rec.Code, rec.Body)
	}
	ended := waitScanEnded(t, app, token)
	if len(ended.Candidates) != 1 || len(ended.Candidates[0].AddOns) != 1 || ended.Candidates[0].AddOns[0].ConnectorID != dns.ID {
		t.Errorf("candidate add-on marker = %+v", ended.Candidates)
	}
	if len(ended.TLSTargets) != 2 || ended.TLSTargets[0].MonitoredBy != probe.ID || ended.TLSTargets[1].MonitoredBy != probe.ID {
		t.Errorf("TLS markers = %+v; want manual and imported targets marked by %q", ended.TLSTargets, probe.ID)
	}

	createTraefikSnapshot(t, app.Store, traefik.ID, "2026-10-16T00:00:00Z", "10.0.0.6")
	refreshed := decodeScan(t, app.req(t, http.MethodGet, "/api/discovery/scan", nil, token))
	if refreshed.TLSTargets[0].MonitoredBy != probe.ID || refreshed.TLSTargets[1].MonitoredBy != "" {
		t.Errorf("snapshot update did not refresh imported marker: %+v", refreshed.TLSTargets)
	}

	if err := app.Store.UpdateConnector(ctx, probe.ID, map[string]any{"config_data": `{"targets":"manual.example.test:443"}`}); err != nil {
		t.Fatal(err)
	}
	refreshed = decodeScan(t, app.req(t, http.MethodGet, "/api/discovery/scan", nil, token))
	for _, target := range refreshed.TLSTargets {
		if target.MonitoredBy != "" {
			t.Errorf("stale configured marker survived config update: %+v", target)
		}
	}
}

func createTraefikSnapshot(t *testing.T, s *store.Store, connectorID, fetchedAt, host string) {
	t.Helper()
	snapshot := connector.ServiceSnapshot{
		Type: "traefik",
		Entities: []connector.SnapshotEntity{{
			Kind: "router",
			Attributes: map[string]any{
				"tls":  true,
				"rule": "Host(`" + host + "`)",
			},
		}},
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSnapshot(context.Background(), &store.SnapshotRecord{
		ConnectorID: connectorID, Data: string(data), FetchedAt: fetchedAt,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverySuggestionsIncludeCGNATClientAndServer(t *testing.T) {
	app := newDiscoveryV2App(t, newScriptedScanner(false), discoveryhandler.Options{
		InterfaceAddrs: func() ([]net.Addr, error) { return []net.Addr{ipnet(t, "100.64.1.8/24")}, nil },
	})
	_, token := app.user(t, "operator")
	got := getSuggestions(t, app, token, "100.64.0.5:5555")
	if len(got) != 2 || got[0]["cidr"] != "100.64.0.0/24" || got[0]["source"] != "client" ||
		got[1]["cidr"] != "100.64.1.0/24" || got[1]["source"] != "server" {
		t.Errorf("client/server CGNAT suggestions = %v", got)
	}
}

func TestDiscoveryInventoryQueryOnlyRunsWhenNeeded(t *testing.T) {
	var calls atomic.Int32
	app := newDiscoveryV2App(t, newScriptedScanner(false), discoveryhandler.Options{
		InventoryQuery: func(context.Context) ([]store.IndexedAddress, error) {
			calls.Add(1)
			return nil, nil
		},
	})
	stepUp(app, false)
	_, token := app.user(t, "operator")
	rec := postDiscovery(t, app, token, map[string]any{"ranges": []string{"192.168.1.0/24"}})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("range-only start = %d; body = %s", rec.Code, rec.Body)
	}
	waitScanEnded(t, app, token)
	if calls.Load() != 0 {
		t.Errorf("range-only start queried inventory %d times", calls.Load())
	}
}

func TestDiscoveryInventorySourceIncludesULAInKnownHostCount(t *testing.T) {
	rows := []store.IndexedAddress{{ConnectorType: "proxmox", Kind: "vm", IP: "fd00::42"}}
	app := newDiscoveryV2App(t, newScriptedScanner(false), discoveryhandler.Options{
		InterfaceAddrs: func() ([]net.Addr, error) { return nil, nil },
		InventoryQuery: func(context.Context) ([]store.IndexedAddress, error) { return rows, nil },
	})
	_, token := app.user(t, "operator")
	r := app.newRequest(t, http.MethodGet, "/api/discovery/suggestions", nil, token)
	r.RemoteAddr = "198.51.100.10:4444"
	rec := app.serve(r)
	var body struct {
		Suggestions []any `json:"suggestions"`
		KnownHosts  int   `json:"knownHosts"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil {
		t.Fatalf("suggestions = %d; body = %s", rec.Code, rec.Body)
	}
	if body.KnownHosts != 1 || len(body.Suggestions) != 0 {
		t.Errorf("ULA inventory response = %+v", body)
	}
}

func TestDiscoveryReadInventoryFailureDoesNotLeakRawError(t *testing.T) {
	app := newDiscoveryV2App(t, newScriptedScanner(false), discoveryhandler.Options{
		InventoryQuery: func(context.Context) ([]store.IndexedAddress, error) {
			return nil, errors.New("private host 10.1.2.3")
		},
	})
	stepUp(app, false)
	_, token := app.user(t, "operator")
	rec := postDiscovery(t, app, token, map[string]any{"ranges": []string{}, "includeInventory": true})
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "10.1.2.3") {
		t.Errorf("inventory-only failure response = %d %s", rec.Code, rec.Body)
	}
}

var _ disc.PassiveRunner = passiveSourceFunc(nil)
