package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/ws"
)

type fakeRunner struct {
	run func(ctx context.Context, r Range, cb Callbacks) Progress
}

func (f fakeRunner) Run(ctx context.Context, r Range, cb Callbacks) Progress {
	return f.run(ctx, r, cb)
}

type sentEvent struct {
	user    string
	typ     string
	payload any
}

type fakeHub struct {
	mu     sync.Mutex
	events []sentEvent
}

func (h *fakeHub) BroadcastToUser(userID, eventType string, payload any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, sentEvent{userID, eventType, payload})
}

func (h *fakeHub) all() []sentEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]sentEvent(nil), h.events...)
}

type auditEntry struct {
	actor, action, targetType, targetID string
	admin                               bool
	detail                              map[string]any
}

type fakeAudit struct {
	mu      sync.Mutex
	entries []auditEntry
}

func (a *fakeAudit) RecordAuditAs(_ context.Context, actor string, admin bool, action, targetType, targetID string, detail any) error {
	raw, _ := json.Marshal(detail)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, auditEntry{actor, action, targetType, targetID, admin, m})
	return nil
}

func (a *fakeAudit) all() []auditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]auditEntry(nil), a.entries...)
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newClock() *fakeClock { return &fakeClock{t: time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)} }

var admin = Actor{UserID: "user-1", InstanceAdmin: true}

func mustRange(t *testing.T, cidr string) Range {
	t.Helper()
	r, err := ParseRange(cidr)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func waitEnded(t *testing.T, m *Manager) Scan {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s, ok := m.Current(context.Background()); ok && s.State != StateRunning {
			// The end-of-scan audit entry is written just after the state flips.
			time.Sleep(20 * time.Millisecond)
			return s
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("scan did not end")
	return Scan{}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

var proxmoxAt5 = Candidate{Type: "proxmox", Name: "Proxmox VE", Address: "10.0.0.5", Port: 8006, URL: "https://10.0.0.5:8006/api2/json", URLField: "url"}
var haAt7 = Candidate{Type: "home_assistant", Name: "Home Assistant", Address: "10.0.0.7", Port: 8123, URL: "http://10.0.0.7:8123", URLField: "url"}

func quickRunner(cands ...Candidate) fakeRunner {
	return fakeRunner{run: func(_ context.Context, r Range, cb Callbacks) Progress {
		total := len(r.Hosts())
		for _, c := range cands {
			cb.OnCandidate(c)
		}
		cb.OnProgress(Progress{Done: total, Total: total, Answered: 3})
		return Progress{Done: total, Total: total, Answered: 3}
	}}
}

func TestManagerStartAndComplete(t *testing.T) {
	hub, aud, clock := &fakeHub{}, &fakeAudit{}, newClock()
	m := NewManager(Config{Runner: quickRunner(haAt7, proxmoxAt5), Events: hub, Audit: aud, Now: clock.now})

	started, err := m.Start(mustRange(t, "10.0.0.57/24"), admin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if started.ID == "" || started.State != StateRunning || started.CIDR != "10.0.0.0/24" || started.Total != 254 {
		t.Errorf("start snapshot = %+v", started)
	}

	got := waitEnded(t, m)
	if got.ID != started.ID || got.State != StateCompleted || got.Partial {
		t.Errorf("scan = %+v", got)
	}
	if got.Done != 254 || got.Total != 254 || got.Answered != 3 || got.EndedAt == nil {
		t.Errorf("counts = %+v", got)
	}
	if len(got.Candidates) != 2 || got.Candidates[0].Address != "10.0.0.5" || got.Candidates[1].Address != "10.0.0.7" {
		t.Errorf("candidates = %+v, want ordered by address", got.Candidates)
	}

	// Events go to the starter only, ending with complete.
	events := hub.all()
	var types []string
	for _, e := range events {
		if e.user != "user-1" {
			t.Errorf("event %s sent to %q, want the starter only", e.typ, e.user)
		}
		types = append(types, e.typ)
	}
	want := []string{ws.EventDiscoveryCandidate, ws.EventDiscoveryCandidate, ws.EventDiscoveryProgress, ws.EventDiscoveryComplete}
	if strings.Join(types, ",") != strings.Join(want, ",") {
		t.Errorf("event types = %v, want %v", types, want)
	}
	last := events[len(events)-1].payload.(map[string]any)
	if last["scanId"] != started.ID || last["state"] != StateCompleted || last["partial"] != false {
		t.Errorf("complete payload = %v", last)
	}
	progress := events[2].payload.(map[string]any)
	if progress["done"] != 254 || progress["total"] != 254 || progress["answered"] != 3 || progress["scanId"] != started.ID {
		t.Errorf("progress payload = %v", progress)
	}
	cand := events[0].payload.(map[string]any)
	if cand["scanId"] != started.ID || cand["candidate"] != haAt7 {
		t.Errorf("candidate payload = %v", cand)
	}

	// The audit entry is attributed to the starter and carries counts, no hosts.
	entries := aud.all()
	if len(entries) != 1 {
		t.Fatalf("audit entries = %+v", entries)
	}
	e := entries[0]
	if e.action != AuditComplete || e.actor != "user-1" || !e.admin || e.targetType != AuditTargetType || e.targetID != started.ID {
		t.Errorf("audit entry = %+v", e)
	}
	if e.detail["range"] != "10.0.0.0/24" || e.detail["probed"] != float64(254) || e.detail["answered"] != float64(3) || e.detail["partial"] != false {
		t.Errorf("audit detail = %v", e.detail)
	}
	perType := e.detail["candidates"].(map[string]any)
	if perType["proxmox"] != float64(1) || perType["home_assistant"] != float64(1) {
		t.Errorf("per-type counts = %v", perType)
	}
	raw, _ := json.Marshal(e.detail)
	if strings.Contains(string(raw), "10.0.0.5") || strings.Contains(string(raw), "10.0.0.7") {
		t.Errorf("audit detail contains a host address: %s", raw)
	}
}

func TestManagerReadWhileRunning(t *testing.T) {
	release := make(chan struct{})
	m := NewManager(Config{Runner: fakeRunner{run: func(_ context.Context, _ Range, cb Callbacks) Progress {
		cb.OnCandidate(proxmoxAt5)
		cb.OnProgress(Progress{Done: 10, Total: 254, Answered: 1})
		<-release
		return Progress{Done: 254, Total: 254, Answered: 1}
	}}})
	if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "candidate", func() bool { s, _ := m.Current(context.Background()); return len(s.Candidates) == 1 })
	s, ok := m.Current(context.Background())
	if !ok || s.State != StateRunning || s.EndedAt != nil || s.Done != 10 || s.Candidates[0] != proxmoxAt5 {
		t.Errorf("running scan = %+v ok=%v", s, ok)
	}
	close(release)
	waitEnded(t, m)
}

func TestManagerNothingFound(t *testing.T) {
	m := NewManager(Config{Runner: quickRunner()})
	if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); err != nil {
		t.Fatal(err)
	}
	s := waitEnded(t, m)
	if s.State != StateCompleted || len(s.Candidates) != 0 || s.Candidates == nil || s.Answered != 3 {
		t.Errorf("scan = %+v, want completed with an empty (non-nil) candidate list and the answered count", s)
	}
}

func TestManagerNoScanBeforeAnyStart(t *testing.T) {
	m := NewManager(Config{Runner: quickRunner()})
	if _, ok := m.Current(context.Background()); ok {
		t.Error("Current reported a scan before any start")
	}
	if _, ok := m.Cancel(admin); ok {
		t.Error("Cancel reported a running scan before any start")
	}
}

func TestManagerRefusesSecondScanWhileRunning(t *testing.T) {
	release := make(chan struct{})
	m := NewManager(Config{Runner: fakeRunner{run: func(_ context.Context, _ Range, _ Callbacks) Progress {
		<-release
		return Progress{}
	}}})
	first, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Start(mustRange(t, "10.0.1.0/24"), Actor{UserID: "user-2", InstanceAdmin: true}, nil)
	var running *ScanRunningError
	if !errors.As(err, &running) || !errors.Is(err, ErrScanRunning) {
		t.Fatalf("second start error = %v, want ScanRunningError", err)
	}
	if running.Scan.ID != first.ID || running.Scan.CIDR != "10.0.0.0/24" {
		t.Errorf("conflict names %+v, want the running scan", running.Scan)
	}
	if s, _ := m.Current(context.Background()); s.ID != first.ID || s.State != StateRunning {
		t.Errorf("running scan affected: %+v", s)
	}
	close(release)
	waitEnded(t, m)
}

func TestManagerHourlyLimit(t *testing.T) {
	clock := newClock()
	var runs atomic.Int32
	m := NewManager(Config{Runner: fakeRunner{run: func(_ context.Context, _ Range, _ Callbacks) Progress {
		runs.Add(1)
		return Progress{}
	}}, Now: clock.now})
	start := func() (Scan, error) { return m.Start(mustRange(t, "10.0.0.0/24"), admin, nil) }

	// Five invalid ranges never reach the manager; and conflicts do not count:
	// provoke some, then confirm six real starts are still possible.
	for i := 0; i < 6; i++ {
		if _, err := start(); err != nil {
			t.Fatalf("start %d: %v", i+1, err)
		}
		waitEnded(t, m)
		clock.advance(5 * time.Minute)
	}
	// now = t0+30m; the first start was at t0.
	_, err := start()
	var limit *RateLimitError
	if !errors.As(err, &limit) {
		t.Fatalf("seventh start error = %v, want RateLimitError", err)
	}
	if limit.RetryAfter != 30*time.Minute {
		t.Errorf("RetryAfter = %v, want 30m (first start leaves the window at t0+1h)", limit.RetryAfter)
	}
	if runs.Load() != 6 {
		t.Errorf("runs = %d, want 6", runs.Load())
	}

	clock.advance(30 * time.Minute)
	if _, err := start(); err != nil {
		t.Fatalf("start after the window moved: %v", err)
	}
	waitEnded(t, m)
}

func TestManagerRefusedStartsDoNotCount(t *testing.T) {
	clock := newClock()
	release := make(chan struct{})
	m := NewManager(Config{Runner: fakeRunner{run: func(ctx context.Context, _ Range, _ Callbacks) Progress {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return Progress{}
	}}, Now: clock.now})
	if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); !errors.Is(err, ErrScanRunning) {
			t.Fatalf("conflict %d: %v", i, err)
		}
	}
	close(release)
	waitEnded(t, m)
	// One real start so far; five more must fit in the window, the seventh must not.
	for i := 0; i < 5; i++ {
		if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); err != nil {
			t.Fatalf("start %d after conflicts: %v", i+2, err)
		}
		waitEnded(t, m)
	}
	var limit *RateLimitError
	if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); !errors.As(err, &limit) {
		t.Fatalf("seventh start error = %v, want RateLimitError", err)
	}
}

func TestManagerCancelKeepsCandidatesAndAddsNoMore(t *testing.T) {
	hub, aud := &fakeHub{}, &fakeAudit{}
	late := make(chan struct{})
	m := NewManager(Config{Runner: fakeRunner{run: func(ctx context.Context, _ Range, cb Callbacks) Progress {
		cb.OnCandidate(proxmoxAt5)
		cb.OnCandidate(haAt7)
		<-ctx.Done()
		// Probes already in flight may still report after the cancel.
		cb.OnCandidate(Candidate{Type: "portainer", Address: "10.0.0.9", Port: 9443})
		close(late)
		return Progress{Done: 20, Total: 254, Answered: 2}
	}}, Events: hub, Audit: aud})
	if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "two candidates", func() bool { s, _ := m.Current(context.Background()); return len(s.Candidates) == 2 })

	snap, ok := m.Cancel(Actor{UserID: "user-2", InstanceAdmin: true})
	if !ok || snap.State != StateCancelled || snap.EndedAt == nil || len(snap.Candidates) != 2 {
		t.Fatalf("Cancel = %+v, %v, want the scan returned as cancelled", snap, ok)
	}
	<-late
	s := waitEnded(t, m)
	if s.State != StateCancelled || s.Partial || len(s.Candidates) != 2 || s.EndedAt == nil {
		t.Errorf("scan = %+v, want cancelled with exactly the two earlier candidates", s)
	}
	if _, ok := m.Cancel(admin); ok {
		t.Error("Cancel on an ended scan reported a running scan")
	}
	entries := aud.all()
	if len(entries) != 1 || entries[0].action != AuditCancel || entries[0].detail["state"] != "cancelled" {
		t.Fatalf("audit = %+v, want one cancel entry", entries)
	}
	if entries[0].actor != "user-1" || entries[0].detail["cancelledBy"] != "user-2" || entries[0].detail["cancelReason"] != nil {
		t.Errorf("cancel entry = %+v, want the starter as actor and the canceller in detail", entries[0])
	}
	if per := entries[0].detail["candidates"].(map[string]any); per["proxmox"] != float64(1) || per["home_assistant"] != float64(1) || len(per) != 2 {
		t.Errorf("cancel counts = %v", per)
	}
	events := hub.all()
	if last := events[len(events)-1]; last.typ != ws.EventDiscoveryComplete || last.payload.(map[string]any)["state"] != StateCancelled {
		t.Errorf("last event = %+v", last)
	}
}

func TestManagerDeadlineEndsScanAsPartial(t *testing.T) {
	aud := &fakeAudit{}
	m := NewManager(Config{Timeout: 40 * time.Millisecond, Audit: aud, Runner: fakeRunner{run: func(ctx context.Context, _ Range, cb Callbacks) Progress {
		cb.OnCandidate(proxmoxAt5)
		<-ctx.Done()
		return Progress{Done: 100, Total: 254, Answered: 1}
	}}})
	if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); err != nil {
		t.Fatal(err)
	}
	s := waitEnded(t, m)
	if s.State != StateCompleted || !s.Partial || len(s.Candidates) != 1 {
		t.Errorf("scan = %+v, want completed, partial, with the candidate found so far", s)
	}
	if e := aud.all(); len(e) != 1 || e[0].action != AuditComplete || e[0].detail["partial"] != true {
		t.Errorf("audit = %+v", e)
	}
}

func TestManagerFinishedScanAtDeadlineIsNotPartial(t *testing.T) {
	m := NewManager(Config{Timeout: 40 * time.Millisecond, Runner: fakeRunner{run: func(ctx context.Context, r Range, cb Callbacks) Progress {
		total := len(r.Hosts())
		cb.OnProgress(Progress{Done: total, Total: total})
		<-ctx.Done()
		return Progress{Done: total, Total: total}
	}}})
	if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); err != nil {
		t.Fatal(err)
	}
	if s := waitEnded(t, m); s.State != StateCompleted || s.Partial {
		t.Errorf("scan = %+v, want completed and not partial", s)
	}
}

func TestManagerLateCandidateSkipsLookup(t *testing.T) {
	var lookups atomic.Int32
	m := NewManager(Config{
		Endpoints: func(context.Context) ([]Endpoint, error) { lookups.Add(1); return nil, nil },
		Runner: fakeRunner{run: func(ctx context.Context, _ Range, cb Callbacks) Progress {
			<-ctx.Done()
			cb.OnCandidate(proxmoxAt5)
			return Progress{}
		}},
	})
	if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); err != nil {
		t.Fatal(err)
	}
	m.Cancel(admin)
	if s := waitEnded(t, m); len(s.Candidates) != 0 || lookups.Load() != 0 {
		t.Errorf("candidates = %+v, lookups = %d, want a late candidate dropped without a lookup", s.Candidates, lookups.Load())
	}
}

func TestManagerStartCallbackRunsBeforeTheScan(t *testing.T) {
	var order []string
	var mu sync.Mutex
	note := func(what string) { mu.Lock(); order = append(order, what); mu.Unlock() }
	m := NewManager(Config{Runner: fakeRunner{run: func(context.Context, Range, Callbacks) Progress {
		note("run")
		return Progress{}
	}}})
	var got Scan
	started, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, func(s Scan) { got = s; note("started") })
	if err != nil {
		t.Fatal(err)
	}
	waitEnded(t, m)
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(order, ",") != "started,run" || got.ID != started.ID || got.State != StateRunning {
		t.Errorf("order = %v, callback scan = %+v, want the callback first with the new scan", order, got)
	}
}

func TestManagerStartCallbackPanicStillRunsTheScan(t *testing.T) {
	m := NewManager(Config{Runner: quickRunner()})
	func() {
		defer func() { _ = recover() }()
		_, _ = m.Start(mustRange(t, "10.0.0.0/24"), admin, func(Scan) { panic("audit write failed") })
	}()
	if s := waitEnded(t, m); s.State != StateCompleted {
		t.Fatalf("scan = %+v, want it to run and complete", s)
	}
	if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); err != nil {
		t.Errorf("second Start after a panicking callback = %v, want accepted", err)
	}
}

func TestManagerConcurrentStartsAdmitOne(t *testing.T) {
	release := make(chan struct{})
	m := NewManager(Config{Runner: fakeRunner{run: func(context.Context, Range, Callbacks) Progress {
		<-release
		return Progress{}
	}}})
	const n = 16
	var ok, running atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil)
			var re *ScanRunningError
			switch {
			case err == nil:
				ok.Add(1)
			case errors.As(err, &re):
				running.Add(1)
			default:
				t.Errorf("Start error = %v", err)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 1 || running.Load() != n-1 {
		t.Errorf("%d started, %d refused as running, want 1 and %d", ok.Load(), running.Load(), n-1)
	}
	m.mu.Lock()
	slots := len(m.starts)
	m.mu.Unlock()
	if slots != 1 {
		t.Errorf("start slots used = %d, want 1", slots)
	}
	close(release)
	waitEnded(t, m)
}

func TestManagerShutdownCancelsRunningScan(t *testing.T) {
	aud := &fakeAudit{}
	running := make(chan struct{})
	m := NewManager(Config{Audit: aud, Runner: fakeRunner{run: func(ctx context.Context, _ Range, cb Callbacks) Progress {
		cb.OnCandidate(proxmoxAt5)
		close(running)
		<-ctx.Done()
		return Progress{Done: 3, Total: 254}
	}}})
	if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); err != nil {
		t.Fatal(err)
	}
	<-running

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m.Shutdown(ctx)

	// Shutdown returns only once the scan ended and was audited.
	entries := aud.all()
	if len(entries) != 1 || entries[0].action != AuditCancel || entries[0].actor != "user-1" {
		t.Fatalf("audit = %+v, want one cancel entry for the starter", entries)
	}
	if d := entries[0].detail; d["cancelReason"] != "shutdown" || d["cancelledBy"] != nil || d["state"] != "cancelled" {
		t.Errorf("cancel detail = %v", d)
	}
	if s, _ := m.Current(context.Background()); s.State != StateCancelled || s.EndedAt == nil {
		t.Errorf("scan = %+v, want cancelled", s)
	}
	m.Shutdown(ctx) // again: nothing to do
}

func TestManagerShutdownGivesUpWhenContextEnds(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	m := NewManager(Config{Runner: fakeRunner{run: func(context.Context, Range, Callbacks) Progress {
		<-release // ignores cancellation
		return Progress{}
	}}})
	if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	m.Shutdown(ctx)
	if time.Since(start) > 2*time.Second {
		t.Errorf("Shutdown took %v with a 30ms context", time.Since(start))
	}
}

func TestManagerShutdownWithoutScan(t *testing.T) {
	m := NewManager(Config{Runner: quickRunner()})
	done := make(chan struct{})
	go func() { m.Shutdown(context.Background()); m.Shutdown(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Shutdown with no scan did not return at once")
	}
}

func TestManagerResultExpires(t *testing.T) {
	clock := newClock()
	m := NewManager(Config{Runner: quickRunner(proxmoxAt5), Now: clock.now})
	first, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil)
	if err != nil {
		t.Fatal(err)
	}
	waitEnded(t, m)

	clock.advance(14*time.Minute + 59*time.Second)
	if s, ok := m.Current(context.Background()); !ok || s.ID != first.ID {
		t.Fatalf("result 14m59s after the end: %+v ok=%v", s, ok)
	}
	clock.advance(time.Second)
	if _, ok := m.Current(context.Background()); ok {
		t.Error("result still readable 15 minutes after the end")
	}
}

func TestManagerNewScanReplacesResult(t *testing.T) {
	m := NewManager(Config{Runner: quickRunner(proxmoxAt5)})
	first, _ := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil)
	waitEnded(t, m)
	second, err := m.Start(mustRange(t, "10.0.1.0/24"), admin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == first.ID {
		t.Fatal("scan id reused")
	}
	s, ok := m.Current(context.Background())
	if !ok || s.ID != second.ID || s.CIDR != "10.0.1.0/24" {
		t.Errorf("current = %+v, want only the new scan", s)
	}
	waitEnded(t, m)
}

func TestManagerRecoversFromRunnerPanic(t *testing.T) {
	aud := &fakeAudit{}
	m := NewManager(Config{Audit: aud, Runner: fakeRunner{run: func(context.Context, Range, Callbacks) Progress { panic("boom") }}})
	if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); err != nil {
		t.Fatal(err)
	}
	s := waitEnded(t, m)
	if s.State != StateFailed || s.EndedAt == nil {
		t.Errorf("scan = %+v, want failed", s)
	}
	if e := aud.all(); len(e) != 1 || e[0].action != AuditComplete || e[0].detail["state"] != "failed" {
		t.Errorf("audit = %+v", e)
	}
	// A failed scan is over: the next one may start.
	if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); err != nil {
		t.Errorf("start after a failed scan: %v", err)
	}
}

func TestManagerThrottlesProgressEventsAndDedupesCandidates(t *testing.T) {
	hub := &fakeHub{}
	m := NewManager(Config{Events: hub, ProgressEvery: time.Hour, Runner: fakeRunner{run: func(_ context.Context, _ Range, cb Callbacks) Progress {
		cb.OnCandidate(proxmoxAt5)
		cb.OnCandidate(proxmoxAt5)
		for i := 1; i <= 254; i++ {
			cb.OnProgress(Progress{Done: i, Total: 254, Answered: 1})
		}
		return Progress{Done: 254, Total: 254, Answered: 1}
	}}})
	if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); err != nil {
		t.Fatal(err)
	}
	s := waitEnded(t, m)
	var progress, candidates int
	for _, e := range hub.all() {
		switch e.typ {
		case ws.EventDiscoveryProgress:
			progress++
		case ws.EventDiscoveryCandidate:
			candidates++
		}
	}
	if progress != 2 {
		t.Errorf("progress events = %d, want the first and the final one", progress)
	}
	if candidates != 1 || len(s.Candidates) != 1 {
		t.Errorf("candidate events = %d, candidates = %d, want the duplicate dropped", candidates, len(s.Candidates))
	}
}

func TestManagerMarksCandidatesAlreadyConnected(t *testing.T) {
	var mu sync.Mutex
	endpoints := []Endpoint{{ID: "c-pve", Type: "proxmox", Value: "https://10.0.0.5:8006/api2/json"}}
	m := NewManager(Config{
		Runner: quickRunner(proxmoxAt5, haAt7),
		Endpoints: func(context.Context) ([]Endpoint, error) {
			mu.Lock()
			defer mu.Unlock()
			return append([]Endpoint(nil), endpoints...), nil
		},
	})
	if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); err != nil {
		t.Fatal(err)
	}
	s := waitEnded(t, m)
	if s.Candidates[0].ConnectorID != "c-pve" || s.Candidates[1].ConnectorID != "" {
		t.Errorf("candidates = %+v", s.Candidates)
	}

	// A connector created after the scan shows on the next read.
	mu.Lock()
	endpoints = append(endpoints, Endpoint{ID: "c-ha", Type: "home_assistant", Value: "http://10.0.0.7:8123"})
	mu.Unlock()
	s, _ = m.Current(context.Background())
	if s.Candidates[1].ConnectorID != "c-ha" {
		t.Errorf("candidates = %+v, want Home Assistant marked after its connector was created", s.Candidates)
	}
}

func TestMarkConnected(t *testing.T) {
	pve := Candidate{Type: "proxmox", Address: "10.0.0.5", Port: 8006}
	tests := []struct {
		name      string
		cand      Candidate
		endpoints []Endpoint
		want      string
	}{
		{"same type address and port", pve, []Endpoint{{"c1", "proxmox", "https://10.0.0.5:8006/api2/json"}}, "c1"},
		{"different port", pve, []Endpoint{{"c1", "proxmox", "https://10.0.0.5:8007/api2/json"}}, ""},
		{"different address", pve, []Endpoint{{"c1", "proxmox", "https://10.0.0.6:8006/api2/json"}}, ""},
		{"different type on the same address", pve, []Endpoint{{"c1", "pbs", "https://10.0.0.5:8006"}}, ""},
		{"configured by host name", pve, []Endpoint{{"c1", "proxmox", "https://pve.lan:8006/api2/json"}}, ""},
		{"https default port", Candidate{Type: "pfsense", Address: "10.0.0.1", Port: 443}, []Endpoint{{"c2", "pfsense", "https://10.0.0.1"}}, "c2"},
		{"http default port", Candidate{Type: "pihole", Address: "10.0.0.2", Port: 80}, []Endpoint{{"c3", "pihole", "http://10.0.0.2/"}}, "c3"},
		{"default port is not the listed port", Candidate{Type: "pfsense", Address: "10.0.0.1", Port: 80}, []Endpoint{{"c2", "pfsense", "https://10.0.0.1"}}, ""},
		{"docker tcp host", Candidate{Type: "docker", Address: "10.0.0.3", Port: 2375}, []Endpoint{{"c4", "docker", "tcp://10.0.0.3:2375"}}, "c4"},
		{"docker unix socket", Candidate{Type: "docker", Address: "10.0.0.3", Port: 2375}, []Endpoint{{"c4", "docker", "unix:///var/run/docker.sock"}}, ""},
		{"scheme-less value", pve, []Endpoint{{"c1", "proxmox", "10.0.0.5:8006"}}, "c1"},
		{"empty and garbage values", pve, []Endpoint{{"c0", "proxmox", ""}, {"c1", "proxmox", "::::"}, {"c5", "proxmox", "https://[fe80::1]:8006"}}, ""},
		{"stale marker cleared", Candidate{Type: "proxmox", Address: "10.0.0.5", Port: 8006, ConnectorID: "gone"}, nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cands := []Candidate{tc.cand}
			MarkConnected(cands, tc.endpoints)
			if cands[0].ConnectorID != tc.want {
				t.Errorf("ConnectorID = %q, want %q", cands[0].ConnectorID, tc.want)
			}
		})
	}
}
