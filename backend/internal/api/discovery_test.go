package api_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api"
	discoveryhandler "github.com/WiseLabz/wiselabz/internal/api/discovery"
	"github.com/WiseLabz/wiselabz/internal/auth"
	disc "github.com/WiseLabz/wiselabz/internal/discovery"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// scriptedScanner stands in for the network scanner: it reports its
// candidates, then holds the scan open until release is closed or the scan is
// cancelled, so tests can observe a running scan. Nothing touches the network.
type scriptedScanner struct {
	calls      atomic.Int32
	candidates []disc.Candidate
	hold       bool
	release    chan struct{}
	once       sync.Once
}

func newScriptedScanner(hold bool, cands ...disc.Candidate) *scriptedScanner {
	return &scriptedScanner{candidates: cands, hold: hold, release: make(chan struct{})}
}

func (s *scriptedScanner) Run(ctx context.Context, r disc.Range, cb disc.Callbacks) disc.Progress {
	s.calls.Add(1)
	total := len(r.Hosts())
	for _, c := range s.candidates {
		cb.OnCandidate(c)
	}
	if s.hold {
		cb.OnProgress(disc.Progress{Done: 5, Total: total, Answered: len(s.candidates)})
		select {
		case <-s.release:
		case <-ctx.Done():
		}
		return disc.Progress{Done: 5, Total: total, Answered: len(s.candidates)}
	}
	cb.OnProgress(disc.Progress{Done: total, Total: total, Answered: len(s.candidates)})
	return disc.Progress{Done: total, Total: total, Answered: len(s.candidates)}
}

func (s *scriptedScanner) finish() { s.once.Do(func() { close(s.release) }) }

var (
	pveCandidate = disc.Candidate{Type: "proxmox", Name: "Proxmox VE", Address: "10.0.0.5", Port: 8006, URL: "https://10.0.0.5:8006/api2/json", URLField: "url"}
	haCandidate  = disc.Candidate{Type: "home_assistant", Name: "Home Assistant", Address: "10.0.0.7", Port: 8123, URL: "http://10.0.0.7:8123", URLField: "url"}
)

func newDiscoveryApp(t *testing.T, runner disc.Runner, addrs ...net.Addr) *testApp {
	t.Helper()
	return newTestAppWithOptions(t, t.TempDir(), nil, func(c *api.Config) {
		c.Discovery = discoveryhandler.Options{
			Runner:         runner,
			InterfaceAddrs: func() ([]net.Addr, error) { return addrs, nil },
		}
	})
}

// stepUp turns the instance's step-up for destructive actions on or off.
func stepUp(app *testApp, on bool) {
	app.JWT.SetSettingsSource(func() (auth.RuntimeSettings, bool) {
		return auth.RuntimeSettings{StepUpForDestructive: on}, true
	})
}

type scanBody struct {
	Scan disc.Scan `json:"scan"`
}

func decodeScan(t *testing.T, rec *httptest.ResponseRecorder) disc.Scan {
	t.Helper()
	var b scanBody
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("decode scan: %v; body = %s", err, rec.Body)
	}
	return b.Scan
}

type errBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details json.RawMessage
}

func decodeErr(t *testing.T, rec *httptest.ResponseRecorder) errBody {
	t.Helper()
	var e errBody
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("decode error: %v; body = %s", err, rec.Body)
	}
	return e
}

func startScan(t *testing.T, app *testApp, token, cidr string) *httptest.ResponseRecorder {
	t.Helper()
	return app.req(t, http.MethodPost, "/api/discovery/scan", map[string]string{"cidr": cidr}, token)
}

func waitScanEnded(t *testing.T, app *testApp, token string) disc.Scan {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		rec := app.req(t, http.MethodGet, "/api/discovery/scan", nil, token)
		if rec.Code == http.StatusOK {
			if s := decodeScan(t, rec); s.State != disc.StateRunning {
				// The end-of-scan audit entry is written just after the state flips.
				time.Sleep(30 * time.Millisecond)
				return s
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("scan did not end")
	return disc.Scan{}
}

func auditEntries(t *testing.T, app *testApp, action string) []store.AuditRecord {
	t.Helper()
	recs, _, err := app.Store.ListAuditRecords(context.Background(), action, "", "", "", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

func TestDiscoveryNonAdminIsForbidden(t *testing.T) {
	t.Parallel()
	scanner := newScriptedScanner(false)
	app := newDiscoveryApp(t, scanner)
	stepUp(app, false)
	_, viewer := app.user(t, "viewer")

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/discovery/suggestions"},
		{http.MethodPost, "/api/discovery/scan"},
		{http.MethodGet, "/api/discovery/scan"},
		{http.MethodDelete, "/api/discovery/scan"},
	} {
		var body any
		if tc.method == http.MethodPost {
			body = map[string]string{"cidr": "192.168.1.0/24"}
		}
		if rec := app.req(t, tc.method, tc.path, body, viewer); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s = %d, want 403; body = %s", tc.method, tc.path, rec.Code, rec.Body)
		}
	}
	if rec := app.req(t, http.MethodGet, "/api/discovery/suggestions", nil, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated = %d, want 401", rec.Code)
	}
	if scanner.calls.Load() != 0 {
		t.Errorf("scanner ran %d times for refused requests", scanner.calls.Load())
	}
}

func TestDiscoveryStartRequiresElevationWhenStepUpIsOn(t *testing.T) {
	t.Parallel()
	scanner := newScriptedScanner(false)
	app := newDiscoveryApp(t, scanner)
	stepUp(app, true)
	_, token := app.user(t, "operator")

	rec := startScan(t, app, token, "192.168.1.0/24")
	if rec.Code != http.StatusBadRequest || decodeErr(t, rec).Code != "elevation_required" {
		t.Fatalf("status = %d body = %s, want 400 elevation_required", rec.Code, rec.Body)
	}
	if scanner.calls.Load() != 0 {
		t.Errorf("scanner ran without elevation")
	}

	rec = app.reqElevated(t, http.MethodPost, "/api/discovery/scan", map[string]string{"cidr": "192.168.1.0/24"}, token, "discovery.scan")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("elevated start = %d, want 202; body = %s", rec.Code, rec.Body)
	}
	waitScanEnded(t, app, token)

	// A token for another action does not open the scan.
	rec = app.reqElevated(t, http.MethodPost, "/api/discovery/scan", map[string]string{"cidr": "192.168.1.0/24"}, token, "connector.delete")
	if rec.Code == http.StatusAccepted {
		t.Errorf("a token for another action started a scan")
	}

	// Reading and cancelling never ask for elevation.
	if rec := app.req(t, http.MethodGet, "/api/discovery/scan", nil, token); rec.Code != http.StatusOK {
		t.Errorf("read = %d, want 200", rec.Code)
	}
	if rec := app.req(t, http.MethodDelete, "/api/discovery/scan", nil, token); rec.Code != http.StatusNotFound {
		t.Errorf("cancel with nothing running = %d, want 404 not an elevation error", rec.Code)
	}
}

func TestDiscoveryStartWithoutTokenWhenStepUpIsOff(t *testing.T) {
	t.Parallel()
	scanner := newScriptedScanner(false, pveCandidate)
	app := newDiscoveryApp(t, scanner)
	stepUp(app, false)
	_, token := app.user(t, "operator")

	rec := startScan(t, app, token, "192.168.1.0/24")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body = %s", rec.Code, rec.Body)
	}
	started := decodeScan(t, rec)
	if started.ID == "" || started.CIDR != "192.168.1.0/24" || started.Total != 254 || started.State != disc.StateRunning {
		t.Errorf("started = %+v", started)
	}
	done := waitScanEnded(t, app, token)
	if done.ID != started.ID || done.State != disc.StateCompleted || done.Done != 254 || len(done.Candidates) != 1 {
		t.Errorf("finished = %+v", done)
	}
}

func TestDiscoveryRangeFieldErrors(t *testing.T) {
	t.Parallel()
	scanner := newScriptedScanner(false)
	app := newDiscoveryApp(t, scanner)
	stepUp(app, false)
	_, token := app.user(t, "operator")

	tests := []struct{ cidr, wantMsg string }{
		{"10.0.0.0/23", "at most a /24"},
		{"8.8.8.0/24", "only private ranges"},
		{"127.0.0.0/24", "only private ranges"},
		{"169.254.169.0/24", "only private ranges"},
		{"fd00::/120", "IPv4"},
		{"not a range", "CIDR"},
		{"", "CIDR"},
	}
	for _, tc := range tests {
		rec := startScan(t, app, token, tc.cidr)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q: status = %d, want 400; body = %s", tc.cidr, rec.Code, rec.Body)
			continue
		}
		var e struct {
			Code    string `json:"code"`
			Details []struct{ Field, Msg string }
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &e)
		if e.Code != "invalid_request" || len(e.Details) != 1 || e.Details[0].Field != "cidr" || !strings.Contains(e.Details[0].Msg, tc.wantMsg) {
			t.Errorf("%q: error = %s, want a cidr field error containing %q", tc.cidr, rec.Body, tc.wantMsg)
		}
	}
	if rec := app.req(t, http.MethodPost, "/api/discovery/scan", "not an object", token); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed body = %d, want 400", rec.Code)
	}
	if scanner.calls.Load() != 0 {
		t.Errorf("scanner ran for %d rejected requests", scanner.calls.Load())
	}
	if rec := app.req(t, http.MethodGet, "/api/discovery/scan", nil, token); rec.Code != http.StatusNotFound {
		t.Errorf("read after only rejected starts = %d, want 404", rec.Code)
	}

	// Host bits are masked and the masked range is reported.
	rec := startScan(t, app, token, "192.168.1.57/24")
	if rec.Code != http.StatusAccepted || decodeScan(t, rec).CIDR != "192.168.1.0/24" {
		t.Errorf("host bits set: status = %d body = %s", rec.Code, rec.Body)
	}
	waitScanEnded(t, app, token)
}

func TestDiscoveryConflictWhileRunning(t *testing.T) {
	t.Parallel()
	scanner := newScriptedScanner(true, pveCandidate)
	t.Cleanup(scanner.finish)
	app := newDiscoveryApp(t, scanner)
	stepUp(app, false)
	_, token := app.user(t, "operator")

	first := decodeScan(t, startScan(t, app, token, "192.168.1.0/24"))
	rec := startScan(t, app, token, "192.168.2.0/24")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body)
	}
	var e struct {
		Code    string `json:"code"`
		Details struct{ Scan disc.Scan }
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	if e.Code != "scan_in_progress" || e.Details.Scan.ID != first.ID || e.Details.Scan.CIDR != "192.168.1.0/24" {
		t.Errorf("conflict body = %s, want scan_in_progress naming the running scan", rec.Body)
	}
	if got := decodeScan(t, app.req(t, http.MethodGet, "/api/discovery/scan", nil, token)); got.ID != first.ID || got.State != disc.StateRunning {
		t.Errorf("running scan affected: %+v", got)
	}
	if scanner.calls.Load() != 1 {
		t.Errorf("scanner calls = %d, want 1", scanner.calls.Load())
	}
}

func TestDiscoveryHourlyLimit(t *testing.T) {
	t.Parallel()
	scanner := newScriptedScanner(false)
	app := newDiscoveryApp(t, scanner)
	stepUp(app, false)
	_, token := app.user(t, "operator")

	// Rejected requests do not count.
	for i := 0; i < 5; i++ {
		startScan(t, app, token, "8.8.8.0/24")
	}
	for i := 0; i < 6; i++ {
		if rec := startScan(t, app, token, "192.168.1.0/24"); rec.Code != http.StatusAccepted {
			t.Fatalf("start %d = %d, want 202; body = %s", i+1, rec.Code, rec.Body)
		}
		waitScanEnded(t, app, token)
	}
	rec := startScan(t, app, token, "192.168.1.0/24")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("seventh start = %d, want 429; body = %s", rec.Code, rec.Body)
	}
	secs, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || secs <= 0 || secs > 3600 {
		t.Errorf("Retry-After = %q, want seconds within the hour", rec.Header().Get("Retry-After"))
	}
	if e := decodeErr(t, rec); e.Code != "rate_limited" {
		t.Errorf("code = %q", e.Code)
	}
	if scanner.calls.Load() != 6 {
		t.Errorf("scanner calls = %d, want 6", scanner.calls.Load())
	}
}

func TestDiscoveryReadWhileRunningAndCancel(t *testing.T) {
	t.Parallel()
	scanner := newScriptedScanner(true, pveCandidate, haCandidate)
	t.Cleanup(scanner.finish)
	app := newDiscoveryApp(t, scanner)
	stepUp(app, false)
	_, token := app.user(t, "operator")

	if rec := app.req(t, http.MethodGet, "/api/discovery/scan", nil, token); rec.Code != http.StatusNotFound {
		t.Fatalf("read with no scan = %d, want 404", rec.Code)
	}
	if rec := app.req(t, http.MethodDelete, "/api/discovery/scan", nil, token); rec.Code != http.StatusNotFound {
		t.Fatalf("cancel with no scan = %d, want 404", rec.Code)
	}

	started := decodeScan(t, startScan(t, app, token, "10.0.0.0/24"))
	var running disc.Scan
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		running = decodeScan(t, app.req(t, http.MethodGet, "/api/discovery/scan", nil, token))
		if len(running.Candidates) == 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if running.ID != started.ID || running.State != disc.StateRunning || len(running.Candidates) != 2 || running.Done != 5 {
		t.Fatalf("running scan = %+v, want running with the candidates so far", running)
	}

	// Another instance admin may read and cancel it.
	_, other := app.user(t, "operator")
	rec := app.req(t, http.MethodDelete, "/api/discovery/scan", nil, other)
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel = %d, want 200; body = %s", rec.Code, rec.Body)
	}
	cancelled := decodeScan(t, rec)
	if cancelled.State != disc.StateCancelled || len(cancelled.Candidates) != 2 || cancelled.EndedAt == nil {
		t.Errorf("cancelled = %+v, want cancelled with both candidates kept", cancelled)
	}
	if rec := app.req(t, http.MethodDelete, "/api/discovery/scan", nil, token); rec.Code != http.StatusNotFound {
		t.Errorf("second cancel = %d, want 404", rec.Code)
	}
	if got := decodeScan(t, app.req(t, http.MethodGet, "/api/discovery/scan", nil, token)); got.State != disc.StateCancelled {
		t.Errorf("read after cancel = %+v", got)
	}
}

func TestDiscoveryMarksAlreadyConnectedCandidates(t *testing.T) {
	t.Parallel()
	docker := disc.Candidate{Type: "docker", Name: "Docker", Address: "10.0.0.3", Port: 2375, URL: "tcp://10.0.0.3:2375", URLField: "host"}
	scanner := newScriptedScanner(false, pveCandidate, haCandidate, docker)
	app := newDiscoveryApp(t, scanner)
	stepUp(app, false)
	_, token := app.user(t, "operator")

	pve := &store.ConnectorRecord{Name: "pve", Type: "proxmox", Category: "virtualization", URL: "https://10.0.0.5:8006/api2/json", ConfigData: "{}"}
	dock := &store.ConnectorRecord{Name: "dock", Type: "docker", Category: "containers_paas", ConfigData: `{"host":"tcp://10.0.0.3:2375"}`}
	// Same address as the Home Assistant candidate but another type and port.
	other := &store.ConnectorRecord{Name: "pbs", Type: "pbs", Category: "virtualization", URL: "https://10.0.0.7:8007", ConfigData: "{}"}
	for _, c := range []*store.ConnectorRecord{pve, dock, other} {
		if err := app.Store.CreateConnector(context.Background(), c); err != nil {
			t.Fatal(err)
		}
	}

	startScan(t, app, token, "10.0.0.0/24")
	scan := waitScanEnded(t, app, token)
	byType := map[string]string{}
	for _, c := range scan.Candidates {
		byType[c.Type] = c.ConnectorID
	}
	if byType["proxmox"] != pve.ID || byType["docker"] != dock.ID || byType["home_assistant"] != "" {
		t.Errorf("connector ids by type = %v", byType)
	}
}

func TestDiscoveryAuditEntries(t *testing.T) {
	t.Parallel()
	scanner := newScriptedScanner(false, pveCandidate, haCandidate)
	app := newDiscoveryApp(t, scanner)
	stepUp(app, false)
	userID, token := app.user(t, "operator")

	startScan(t, app, token, "8.8.8.0/24")
	started := decodeScan(t, startScan(t, app, token, "192.168.1.0/24"))
	waitScanEnded(t, app, token)

	starts := auditEntries(t, app, disc.AuditStart)
	if len(starts) != 1 {
		t.Fatalf("start entries = %+v", starts)
	}
	if s := starts[0]; s.ActorUserID != userID || s.ActorRole != "admin" || s.TargetType != disc.AuditTargetType || s.TargetID != started.ID {
		t.Errorf("start entry = %+v", s)
	}
	if got := detailOf(t, starts[0]); got["range"] != "192.168.1.0/24" {
		t.Errorf("start detail = %v", got)
	}

	rejects := auditEntries(t, app, disc.AuditReject)
	if len(rejects) != 1 {
		t.Fatalf("reject entries = %+v", rejects)
	}
	if got := detailOf(t, rejects[0]); got["range"] != "8.8.8.0/24" || got["reason"] != "not_private" {
		t.Errorf("reject detail = %v", got)
	}

	completes := auditEntries(t, app, disc.AuditComplete)
	if len(completes) != 1 {
		t.Fatalf("complete entries = %+v", completes)
	}
	if c := completes[0]; c.ActorUserID != userID || c.TargetID != started.ID {
		t.Errorf("complete entry = %+v", c)
	}
	d := detailOf(t, completes[0])
	counts, _ := d["candidates"].(map[string]any)
	if d["probed"] != float64(254) || d["answered"] != float64(2) || d["partial"] != false ||
		counts["proxmox"] != float64(1) || counts["home_assistant"] != float64(1) {
		t.Errorf("complete detail = %v", d)
	}
	if _, ok := d["durationMs"]; !ok {
		t.Errorf("complete detail lacks the duration: %v", d)
	}

	// No entry of any discovery action holds a scanned or found host address.
	for _, action := range []string{disc.AuditStart, disc.AuditComplete, disc.AuditCancel, disc.AuditReject} {
		for _, e := range auditEntries(t, app, action) {
			for _, addr := range []string{"10.0.0.5", "10.0.0.7", "192.168.1.1", "192.168.1.254"} {
				if strings.Contains(e.Detail, addr) {
					t.Errorf("%s entry detail contains host address %s: %s", action, addr, e.Detail)
				}
			}
		}
	}
}

func TestDiscoveryAuditRecordsRejectedConflictAndLimitAndCancel(t *testing.T) {
	t.Parallel()
	scanner := newScriptedScanner(true, pveCandidate)
	t.Cleanup(scanner.finish)
	app := newDiscoveryApp(t, scanner)
	stepUp(app, false)
	_, token := app.user(t, "operator")

	startScan(t, app, token, "192.168.1.0/24")
	startScan(t, app, token, "192.168.2.0/24")
	if rec := app.req(t, http.MethodDelete, "/api/discovery/scan", nil, token); rec.Code != http.StatusOK {
		t.Fatalf("cancel = %d", rec.Code)
	}
	time.Sleep(30 * time.Millisecond)

	rejects := auditEntries(t, app, disc.AuditReject)
	if len(rejects) != 1 || detailOf(t, rejects[0])["reason"] != "scan_in_progress" || detailOf(t, rejects[0])["range"] != "192.168.2.0/24" {
		t.Errorf("reject entries = %+v", rejects)
	}
	cancels := auditEntries(t, app, disc.AuditCancel)
	if len(cancels) != 1 {
		t.Fatalf("cancel entries = %+v", cancels)
	}
	d := detailOf(t, cancels[0])
	if d["state"] != "cancelled" || d["answered"] != float64(1) || d["range"] != "192.168.1.0/24" {
		t.Errorf("cancel detail = %v", d)
	}
	if len(auditEntries(t, app, disc.AuditComplete)) != 0 {
		t.Errorf("a cancelled scan also wrote a complete entry")
	}

	// A long junk range is cut before it reaches the audit log.
	startScan(t, app, token, strings.Repeat("x", 500))
	var long int
	for _, e := range auditEntries(t, app, disc.AuditReject) {
		if r, _ := detailOf(t, e)["range"].(string); len(r) > long {
			long = len(r)
		}
	}
	if long > 64 {
		t.Errorf("audited range is %d bytes, want at most 64", long)
	}
}

func detailOf(t *testing.T, e store.AuditRecord) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(e.Detail), &m); err != nil {
		t.Fatalf("audit detail %q: %v", e.Detail, err)
	}
	return m
}

func ipnet(t *testing.T, cidr string) net.Addr {
	t.Helper()
	ip, n, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatal(err)
	}
	n.IP = ip
	return n
}

func getSuggestions(t *testing.T, app *testApp, token, remote string) []map[string]string {
	t.Helper()
	r := app.newRequest(t, http.MethodGet, "/api/discovery/suggestions", nil, token)
	r.RemoteAddr = remote
	rec := app.serve(r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", rec.Code, rec.Body)
	}
	var body struct {
		Suggestions []map[string]string `json:"suggestions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Suggestions == nil {
		t.Fatalf("suggestions must be an array, got null; body = %s", rec.Body)
	}
	return body.Suggestions
}

func TestDiscoverySuggestionsClientThenServer(t *testing.T) {
	t.Parallel()
	app := newDiscoveryApp(t, newScriptedScanner(false),
		ipnet(t, "172.18.0.3/16"),
		ipnet(t, "127.0.0.1/8"),    // loopback is never suggested
		ipnet(t, "8.8.8.8/32"),     // nor a public address
		ipnet(t, "169.254.1.1/16"), // nor link-local
		ipnet(t, "fd00::5/64"),     // nor IPv6
	)
	_, token := app.user(t, "operator")

	got := getSuggestions(t, app, token, "192.168.1.50:5555")
	want := []map[string]string{
		{"cidr": "192.168.1.0/24", "source": "client"},
		{"cidr": "172.18.0.0/24", "source": "server"},
	}
	if len(got) != len(want) || got[0]["cidr"] != want[0]["cidr"] || got[0]["source"] != want[0]["source"] ||
		got[1]["cidr"] != want[1]["cidr"] || got[1]["source"] != want[1]["source"] {
		t.Errorf("suggestions = %v, want %v", got, want)
	}
}

func TestDiscoverySuggestionsDeduplicateSharedRange(t *testing.T) {
	t.Parallel()
	app := newDiscoveryApp(t, newScriptedScanner(false), ipnet(t, "192.168.1.10/24"), ipnet(t, "10.5.5.9/8"))
	_, token := app.user(t, "operator")

	got := getSuggestions(t, app, token, "192.168.1.50:5555")
	if len(got) != 2 || got[0]["cidr"] != "192.168.1.0/24" || got[0]["source"] != "client" ||
		got[1]["cidr"] != "10.5.5.0/24" || got[1]["source"] != "server" {
		t.Errorf("suggestions = %v, want the shared /24 once, as the client's", got)
	}
}

func TestDiscoverySuggestionsEmptyForPublicClientAndNoPrivateInterface(t *testing.T) {
	t.Parallel()
	app := newDiscoveryApp(t, newScriptedScanner(false), ipnet(t, "203.0.113.4/24"))
	_, token := app.user(t, "operator")
	if got := getSuggestions(t, app, token, "198.51.100.7:4444"); len(got) != 0 {
		t.Errorf("suggestions = %v, want none", got)
	}
}

func TestDiscoverySuggestionsIgnoreForgedForwardingHeaders(t *testing.T) {
	t.Parallel()
	app := newDiscoveryApp(t, newScriptedScanner(false))
	_, token := app.user(t, "operator")

	r := app.newRequest(t, http.MethodGet, "/api/discovery/suggestions", nil, token)
	r.RemoteAddr = "198.51.100.7:4444"
	r.Header.Set("X-Forwarded-For", "10.9.9.9")
	r.Header.Set("X-Real-IP", "10.9.9.9")
	rec := app.serve(r)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "10.9.9") {
		t.Errorf("an untrusted peer's forwarding headers were honoured: %s", rec.Body)
	}
}
