package opnsense

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	ruleB = "rule-b"

	callApplyR2  = "POST apply/" + rev2
	callCancelR2 = "POST cancelRollback/" + rev2
	callRevertR2 = "POST revert/" + rev2
)

// fakeClock lets rollback-window tests advance time without waiting.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers chan *fakeTimer // every timer a push starts waiting on
}

type fakeTimer struct {
	d time.Duration
	c chan time.Time
}

// installFakeClock must run before the first push to this firewall.
func installFakeClock(f *fakeFilter) *fakeClock {
	clk := &fakeClock{now: time.Now(), timers: make(chan *fakeTimer, 8)}
	st := f.filterState()
	st.now = clk.Now
	st.newTimer = clk.newTimer
	return clk
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

func (c *fakeClock) newTimer(d time.Duration) (<-chan time.Time, func()) {
	ft := &fakeTimer{d: d, c: make(chan time.Time, 1)}
	c.timers <- ft
	return ft.c, func() {}
}

func (c *fakeClock) awaitTimer(t *testing.T) *fakeTimer {
	t.Helper()
	select {
	case ft := <-c.timers:
		return ft
	case <-time.After(30 * time.Second):
		t.Fatal("the push never started waiting for the rollback window")
		return nil
	}
}

func (c *fakeClock) expectNoTimer(t *testing.T) {
	t.Helper()
	select {
	case ft := <-c.timers:
		t.Errorf("a push waited %v, want no wait", ft.d)
	default:
	}
}

type waitingPush struct {
	clk   *fakeClock
	timer *fakeTimer
	done  chan error
}

// startPush returns when ConfigPush reaches the rollback wait.
func startPush(ctx context.Context, t *testing.T, clk *fakeClock, c *Connector, ref string, value bool) *waitingPush {
	t.Helper()
	p := &waitingPush{clk: clk, done: make(chan error, 1)}
	go func() { p.done <- c.ConfigPush(ctx, nil, ref, "enabled", value) }()
	p.timer = clk.awaitTimer(t)
	return p
}

func (p *waitingPush) result(t *testing.T) error {
	t.Helper()
	select {
	case err := <-p.done:
		return err
	case <-time.After(30 * time.Second):
		t.Fatal("the push did not return")
		return nil
	}
}

// release advances past the window and fires the timer.
func (p *waitingPush) release(t *testing.T, until time.Time) error {
	t.Helper()
	p.clk.set(until.Add(time.Second))
	p.timer.c <- p.clk.Now()
	return p.result(t)
}

// retryAfterWait verifies a push sends no requests until the rollback window ends.
func retryAfterWait(t *testing.T, f *fakeFilter, clk *fakeClock, c *Connector, ref string, value bool, duringWait func()) error {
	t.Helper()
	until := f.filterState().waitUntil()
	if !until.After(clk.Now()) {
		t.Fatal("no rollback window is open, so the scenario does not test a wait")
	}
	mark := f.callCount()
	p := startPush(context.Background(), t, clk, c, ref, value)
	if want := until.Sub(clk.Now()); p.timer.d != want {
		t.Errorf("the push waited for %v, want %v", p.timer.d, want)
	}
	if got := f.callCount(); got != mark {
		t.Errorf("%d request(s) reached the firewall during the wait", got-mark)
	}
	if duringWait != nil {
		duringWait()
	}
	return p.release(t, until)
}

// byCall replaces the answer of the numbered calls only.
func byCall(faults map[int]fault) func(int) *fault {
	return func(n int) *fault {
		if f, ok := faults[n]; ok {
			return &f
		}
		return nil
	}
}

func pushTo(c *Connector, ref string, value bool) error {
	return c.ConfigPush(context.Background(), nil, ref, "enabled", value)
}

func (f *fakeFilter) addRule(uuid, value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved[uuid], f.live[uuid] = value, value
}

func (f *fakeFilter) ruleState(uuid string) (saved, live string, ok bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	saved, ok = f.saved[uuid]
	return saved, f.live[uuid], ok
}

func (f *fakeFilter) deleteRule(uuid string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.saved, uuid)
	delete(f.live, uuid)
}

// callCount is the number of requests the fake has received so far.
func (f *fakeFilter) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeFilter) filterState() *filterState { return filterStateFor(f.srv.URL) }

// waitUntil returns the time before which a rollback timer may still fire.
func (s *filterState) waitUntil() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.notBefore
}

func (s *filterState) waiting() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.now().Before(s.notBefore)
}

func (s *filterState) markCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.uncertain)
}

func expectRead(t *testing.T, c *Connector, ref string, want any) {
	t.Helper()
	got, err := c.ConfigRead(context.Background(), nil, ref, "enabled")
	if err != nil {
		t.Fatalf("ConfigRead(%s) error = %v", ref, err)
	}
	if got != want {
		t.Errorf("ConfigRead(%s) = %#v, want %#v", ref, got, want)
	}
}

func expectPush(t *testing.T, c *Connector, ref string, value bool) {
	t.Helper()
	if err := pushTo(c, ref, value); err != nil {
		t.Fatalf("ConfigPush(%s, %v) error = %v", ref, value, err)
	}
}

func concat(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// failedSavepointPush runs a first push whose apply reached the firewall
// (timer pending, rule live) but answered an error, and whose revert failed.
func failedSavepointPush(t *testing.T, f *fakeFilter, target bool) *Connector {
	t.Helper()
	f.faults["apply"] = onCall(1, fault{code: 500, effect: true})
	f.faults["revert"] = onCall(1, fault{code: 500})
	c := f.conn()
	err := pushTo(c, testRule, target)
	expectErrContains(t, err, "API returned 500", "revert failed", "the next push to this firewall waits for that")
	if got := f.pendingTimers(); got != 1 {
		t.Fatalf("pending timers = %d, want 1", got)
	}
	return c
}

var firstFailedSeq = []string{callGetRule, callSavepoint, callSetRule, callApplyR1, callRevertR1}

func TestConfigPush_DoubleFaultWithTimerWaitsForIt(t *testing.T) {
	t.Run("same value", func(t *testing.T) {
		f := newFakeFilter(t, true)
		clk := installFakeClock(f)
		c := failedSavepointPush(t, f, true)
		f.expectSeq(t, firstFailedSeq...)
		f.expectState(t, "1", "1")
		expectRead(t, c, testRule, nil)

		// The old timer fires inside the window, while the push still waits.
		if err := retryAfterWait(t, f, clk, c, testRule, true, f.fireRollbackTimers); err != nil {
			t.Fatalf("ConfigPush() error = %v", err)
		}
		f.expectSeq(t, concat(firstFailedSeq, []string{callSetRule, callGetRule, callSavepoint, callSetRule, callApplyR2, callCancelR2})...)
		f.expectState(t, "1", "1")
		f.fireRollbackTimers()
		f.expectState(t, "1", "1")
		expectRead(t, c, testRule, true)
	})

	t.Run("other value inside the window", func(t *testing.T) {
		f := newFakeFilter(t, true)
		clk := installFakeClock(f)
		f.addRule(testRule, "1")
		c := failedSavepointPush(t, f, false)
		f.expectState(t, "0", "0")

		if err := retryAfterWait(t, f, clk, c, testRule, true, f.fireRollbackTimers); err != nil {
			t.Fatalf("ConfigPush() error = %v", err)
		}
		f.expectState(t, "1", "1")
		f.fireRollbackTimers()
		f.expectState(t, "1", "1")
		expectRead(t, c, testRule, true)
	})
}

func TestConfigPush_ApplyNeverReachedAndRevertFails(t *testing.T) {
	f := newFakeFilter(t, true)
	clk := installFakeClock(f)
	f.faults["apply"] = onCall(1, fault{code: 500})
	f.faults["revert"] = onCall(1, fault{code: 500})
	c := f.conn()

	err := pushTo(c, testRule, true)
	expectErrContains(t, err, "revert failed")
	f.expectSeq(t, firstFailedSeq...)
	// Saved but not live, and no timer to roll it back.
	f.expectState(t, "1", "0")
	expectRead(t, c, testRule, nil)

	if err := retryAfterWait(t, f, clk, c, testRule, true, nil); err != nil {
		t.Fatalf("ConfigPush() error = %v", err)
	}
	f.expectSeq(t, concat(firstFailedSeq, []string{callSetRule, callGetRule, callSavepoint, callSetRule, callApplyR2, callCancelR2})...)
	if want := []string{"1", "0", "1"}; strings.Join(f.setRules, ",") != strings.Join(want, ",") {
		t.Errorf("setRule values = %v, want %v", f.setRules, want)
	}
	f.expectState(t, "1", "1")
	expectRead(t, c, testRule, true)
}

func TestConfigPush_RevertWorksButTimerNotCancelled(t *testing.T) {
	tests := []struct {
		name  string
		fault fault
	}{
		{"HTTP 500", fault{code: 500}},
		{"empty status", fault{body: `{"status":""}`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeFilter(t, true)
			clk := installFakeClock(f)
			f.faults["apply"] = onCall(1, fault{code: 500, effect: true})
			f.faults["cancelRollback"] = onCall(1, tt.fault)
			c := f.conn()

			err := pushTo(c, testRule, true)
			expectErrContains(t, err, "change rolled back", "could not be cancelled and may still fire within about a minute")
			f.expectState(t, "0", "0")
			if got := f.pendingTimers(); got != 1 {
				t.Fatalf("pending timers = %d, want 1", got)
			}
			// The saved value is the previous one, so no mark is needed.
			expectRead(t, c, testRule, false)

			if err := retryAfterWait(t, f, clk, c, testRule, true, f.fireRollbackTimers); err != nil {
				t.Fatalf("ConfigPush() error = %v", err)
			}
			f.fireRollbackTimers()
			f.expectState(t, "1", "1")
		})
	}
}

func TestConfigPush_AppliedButTimerNotFound(t *testing.T) {
	tests := []struct {
		name     string
		cancel   func(int) *fault
		wantErr  []string
		wantSeq  []string
		wantHold bool
	}{
		{
			name:     "no timer on both attempts",
			cancel:   always(fault{body: `{"status":""}`}),
			wantErr:  []string{"rollback timer was not found", "change rolled back", "cancelRollback: timer not found"},
			wantSeq:  []string{callGetRule, callSavepoint, callSetRule, callApplyR1, callCancelR1, callRevertR1, callCancelR1},
			wantHold: true,
		},
		{
			name:    "no timer on the first attempt only",
			cancel:  onCall(1, fault{body: `{"status":""}`}),
			wantErr: []string{"rollback timer was not found", "change rolled back"},
			wantSeq: []string{callGetRule, callSavepoint, callSetRule, callApplyR1, callCancelR1, callRevertR1, callCancelR1},
		},
		{
			// Older Backend.php returns null instead of "" for a failed script.
			name:    "null status counts as no timer",
			cancel:  onCall(1, fault{body: `{"status":null}`}),
			wantErr: []string{"rollback timer was not found", "change rolled back"},
			wantSeq: []string{callGetRule, callSavepoint, callSetRule, callApplyR1, callCancelR1, callRevertR1, callCancelR1},
		},
		{
			name:    "unexpected status",
			cancel:  onCall(1, fault{body: `{"status":"error"}`}),
			wantErr: []string{`unexpected status "error"`, "change rolled back"},
			wantSeq: []string{callGetRule, callSavepoint, callSetRule, callApplyR1, callCancelR1, callRevertR1, callCancelR1},
		},
		{
			name:    "undecodable body",
			cancel:  onCall(1, fault{body: `not json`}),
			wantErr: []string{"rollback could not be cancelled", "change rolled back"},
			wantSeq: []string{callGetRule, callSavepoint, callSetRule, callApplyR1, callCancelR1, callRevertR1, callCancelR1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeFilter(t, true)
			f.faults["cancelRollback"] = tt.cancel
			err := pushTo(f.conn(), testRule, true)
			expectErrContains(t, err, tt.wantErr...)
			f.expectSeq(t, tt.wantSeq...)
			f.expectState(t, "0", "0")
			if got := f.filterState().waiting(); got != tt.wantHold {
				t.Errorf("waiting for a rollback timer = %v, want %v", got, tt.wantHold)
			}
		})
	}
}

func TestConfigPush_StateLostAfterRestart(t *testing.T) {
	f := newFakeFilter(t, true)
	c := failedSavepointPush(t, f, true)

	// As after a backend restart: the wait and the marks are gone, the first
	// timer is still pending, so the second apply starts no timer of its own.
	filterStates.Delete(filterKey(f.srv.URL))
	err := pushTo(c, testRule, true)
	expectErrContains(t, err, "rollback timer was not found", "may still fire")
	f.expectSeq(t, concat(firstFailedSeq, []string{callGetRule, callSavepoint, callSetRule, callApplyR2, callCancelR2, callRevertR2, callCancelR2})...)
}

func TestConfigPush_RollbackWindowAndContext(t *testing.T) {
	t.Run("a deadline before the end of the window returns at once", func(t *testing.T) {
		f := newFakeFilter(t, true)
		clk := installFakeClock(f)
		f.filterState().holdRollback()

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		err := f.conn().ConfigPush(ctx, nil, testRule, "enabled", true)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("ConfigPush() error = %v, want context.DeadlineExceeded", err)
		}
		expectErrContains(t, err, "rollback timer from an earlier failed push", "70 seconds remain")
		clk.expectNoTimer(t)
		f.expectSeq(t)
	})

	t.Run("a context cancelled during the wait", func(t *testing.T) {
		f := newFakeFilter(t, true)
		clk := installFakeClock(f)
		f.filterState().holdRollback()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		p := startPush(ctx, t, clk, f.conn(), testRule, true)
		cancel()
		err := p.result(t)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ConfigPush() error = %v, want context.Canceled", err)
		}
		expectErrContains(t, err, "rollback timer from an earlier failed push", "70 seconds remain")
		f.expectSeq(t)
	})

	t.Run("other pushes queue behind the lock", func(t *testing.T) {
		f := newFakeFilter(t, true)
		clk := installFakeClock(f)
		f.filterState().holdRollback()
		until := f.filterState().waitUntil()

		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i := range errs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = pushTo(f.conn(), testRule, true)
			}()
		}
		// One push holds the lock and waits; the other waits for the lock.
		timer := clk.awaitTimer(t)
		if got := f.callCount(); got != 0 {
			t.Errorf("%d request(s) reached the firewall during the wait", got)
		}
		clk.set(until.Add(time.Second))
		timer.c <- clk.Now()
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				t.Errorf("ConfigPush() error = %v", err)
			}
		}
		clk.expectNoTimer(t)
		// One push finishes before the other starts.
		f.expectSeq(t,
			callGetRule, callSavepoint, callSetRule, callApplyR1, callCancelR1,
			callGetRule, callSavepoint, callSetRule, callApplyR2, callCancelR2)
		f.mu.Lock()
		overlaps := f.overlaps
		f.mu.Unlock()
		if overlaps != 0 {
			t.Errorf("%d push(es) started while another was in flight", overlaps)
		}
		f.expectState(t, "1", "1")
	})
}

// failedFallbackPush leaves rule uuid saved with the new value and not live:
// the apply fails and the undo setRule fails.
func failedFallbackPush(t *testing.T, f *fakeFilter, uuid string) *Connector {
	t.Helper()
	f.faults["apply"] = onCall(1, fault{body: `{"status":"Error (1)\n\n"}`})
	f.faults["setRule"] = onCall(2, fault{code: 500})
	c := f.conn()
	err := pushTo(c, uuid, true)
	expectErrContains(t, err, "Error (1)", "undo failed", "the next push writes and applies again")
	return c
}

func TestConfigPush_FallbackUndoSetRuleFails(t *testing.T) {
	f := newFakeFilter(t, false)
	c := failedFallbackPush(t, f, testRule)
	f.expectSeq(t, callGetRule, callSavepoint, callSetRule, callApply, callSetRule)
	f.expectState(t, "1", "0")
	expectRead(t, c, testRule, nil)

	expectPush(t, c, testRule, true)
	f.expectSeq(t, callGetRule, callSavepoint, callSetRule, callApply, callSetRule,
		callSetRule, callGetRule, callSavepoint, callSetRule, callApply)
	f.expectState(t, "1", "1")
	expectRead(t, c, testRule, true)
}

func TestConfigPush_FallbackUndoApplyFails(t *testing.T) {
	f := newFakeFilter(t, false)
	f.faults["apply"] = byCall(map[int]fault{
		1: {body: `{"status":"Error (1)\n\n"}`},
		2: {code: 500},
	})
	c := f.conn()
	err := pushTo(c, testRule, true)
	expectErrContains(t, err, "Error (1)", "saved again but could not be applied", "the next push applies again")
	if strings.Contains(err.Error(), "saved with the new value") {
		t.Errorf("error %q claims the new value is saved", err)
	}
	f.expectSeq(t, callGetRule, callSavepoint, callSetRule, callApply, callSetRule, callApply)
	f.expectState(t, "0", "0")
	expectRead(t, c, testRule, nil)

	expectPush(t, c, testRule, true)
	f.expectState(t, "1", "1")
	expectRead(t, c, testRule, true)
}

func TestConfigPush_RetryThatFailsAgainNeverMakesUnappliedValueLive(t *testing.T) {
	t.Run("savepoint generation", func(t *testing.T) {
		f := newFakeFilter(t, true)
		clk := installFakeClock(f)
		f.faults["apply"] = byCall(map[int]fault{1: {code: 500}, 2: {code: 500, effect: true}})
		f.faults["revert"] = onCall(1, fault{code: 500})
		c := f.conn()

		if err := pushTo(c, testRule, true); err == nil {
			t.Fatal("first ConfigPush() error = nil, want error")
		}
		f.expectState(t, "1", "0")

		err := retryAfterWait(t, f, clk, c, testRule, true, nil)
		expectErrContains(t, err, "change rolled back")
		f.expectSeq(t, concat(firstFailedSeq, []string{callSetRule, callGetRule, callSavepoint, callSetRule, callApplyR2, callRevertR2, callCancelR2})...)
		f.expectState(t, "0", "0")
	})

	t.Run("fallback generation", func(t *testing.T) {
		f := newFakeFilter(t, false)
		c := failedFallbackPush(t, f, testRule)
		f.expectState(t, "1", "0")

		f.faults["apply"] = onCall(2, fault{code: 500})
		err := pushTo(c, testRule, true)
		expectErrContains(t, err, "API returned 500", "previous value restored")
		f.expectState(t, "0", "0")
		if got := f.filterState().markCount(); got != 0 {
			t.Errorf("marks = %d, want 0 after the undo worked", got)
		}
		expectRead(t, c, testRule, false)
	})
}

func TestConfigPush_MarkedRuleIsRestoredBeforeAPushToAnotherRule(t *testing.T) {
	f := newFakeFilter(t, false)
	f.addRule(ruleB, "0")
	c := failedFallbackPush(t, f, testRule)
	if saved, live, _ := f.ruleState(testRule); saved != "1" || live != "0" {
		t.Fatalf("rule A saved=%q live=%q, want saved=1 live=0", saved, live)
	}

	expectPush(t, c, ruleB, true)
	for uuid, want := range map[string]string{testRule: "0", ruleB: "1"} {
		if saved, live, _ := f.ruleState(uuid); saved != want || live != want {
			t.Errorf("rule %s saved=%q live=%q, want both %q", uuid, saved, live, want)
		}
	}
	expectRead(t, c, testRule, false)
	expectRead(t, c, ruleB, true)
}

func TestConfigPush_MarkedRuleDeletedUpstreamIsDropped(t *testing.T) {
	f := newFakeFilter(t, false)
	f.addRule(ruleB, "0")
	c := failedFallbackPush(t, f, testRule)
	f.deleteRule(testRule)

	expectPush(t, c, ruleB, true)
	if got := f.filterState().markCount(); got != 0 {
		t.Errorf("marks = %d, want 0", got)
	}
	if saved, live, _ := f.ruleState(ruleB); saved != "1" || live != "1" {
		t.Errorf("rule B saved=%q live=%q, want both 1", saved, live)
	}
}

func TestConfigPush_MarkedRuleDeletedUpstreamIsDroppedWhenWriteBackErrors(t *testing.T) {
	for _, code := range []int{404, 500} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			f := newFakeFilter(t, false)
			f.addRule(ruleB, "0")
			c := failedFallbackPush(t, f, testRule)
			f.deleteRule(testRule)
			// The write-back of the deleted rule is the next setRule call.
			f.faults["setRule"] = onCall(3, fault{code: code})

			expectPush(t, c, ruleB, true)
			if got := f.filterState().markCount(); got != 0 {
				t.Errorf("marks = %d, want 0", got)
			}
			if saved, live, _ := f.ruleState(ruleB); saved != "1" || live != "1" {
				t.Errorf("rule B saved=%q live=%q, want both 1", saved, live)
			}
		})
	}
}

func TestConfigPush_MarkedRuleThatCannotBeRestoredBlocksThePush(t *testing.T) {
	f := newFakeFilter(t, false)
	f.addRule(ruleB, "0")
	c := failedFallbackPush(t, f, testRule)
	f.faults["setRule"] = always(fault{code: 500})

	err := pushTo(c, ruleB, true)
	expectErrContains(t, err, testRule, "an earlier failed push could not be cleaned up", "API returned 500")
	if got := f.filterState().markCount(); got != 1 {
		t.Errorf("marks = %d, want 1", got)
	}
}

func TestFilterKey(t *testing.T) {
	same := [][]string{
		{"https://fw", "https://fw:443", "https://FW/", "HTTPS://fw//", "https://fw/", " https://Fw:443/ "},
		{"http://fw", "http://fw:80", "HTTP://FW/"},
		{"https://[::1]", "https://[::1]:443/", "HTTPS://[::1]//"},
		{"https://fw:8443", "https://FW:8443/"},
		{"https://fw/base", "https://FW:443/base/"},
		{"  FW// ", "fw"},
	}
	for _, group := range same {
		want := filterKey(group[0])
		for _, raw := range group[1:] {
			if got := filterKey(raw); got != want {
				t.Errorf("filterKey(%q) = %q, want %q (same as %q)", raw, got, want, group[0])
			}
		}
	}
	distinct := []string{"https://fw", "http://fw", "https://fw:8443", "https://fw/base", "https://other", "https://[::1]", "https://[::2]"}
	seen := map[string]string{}
	for _, raw := range distinct {
		key := filterKey(raw)
		if prev, ok := seen[key]; ok {
			t.Errorf("filterKey(%q) and filterKey(%q) are both %q", raw, prev, key)
		}
		seen[key] = raw
	}
	if got := filterKey("https://[::1]:8443/"); got != "https://[::1]:8443" {
		t.Errorf("filterKey(IPv6 with port) = %q", got)
	}
}

func TestFilterState_SharedBetweenURLSpellings(t *testing.T) {
	f := newFakeFilter(t, false)
	f.addRule(ruleB, "0")
	c1 := f.conn()
	c2 := f.conn()
	c2.url = "HTTP" + strings.TrimPrefix(f.srv.URL, "http")
	if filterStateFor(c1.url) != filterStateFor(c2.url) {
		t.Fatal("equivalent URLs have different filter states")
	}
	if filterStateFor(c1.url) == filterStateFor(c1.url+"/other") {
		t.Fatal("different paths share a filter state")
	}

	failedFallbackPushOn(t, f, c1)
	expectRead(t, c2, testRule, nil)
	expectRead(t, c1, ruleB, false)
}

func failedFallbackPushOn(t *testing.T, f *fakeFilter, c *Connector) {
	t.Helper()
	f.faults["apply"] = onCall(1, fault{body: `{"status":"Error (1)\n\n"}`})
	f.faults["setRule"] = onCall(2, fault{code: 500})
	expectErrContains(t, pushTo(c, testRule, true), "undo failed")
}

func TestConfigPush_SavepointRevisionDecoding(t *testing.T) {
	t.Run("a JSON number is used as written", func(t *testing.T) {
		f := newFakeFilter(t, true)
		f.numRev = true
		expectPush(t, f.conn(), testRule, true)
		f.expectSeq(t, callGetRule, callSavepoint, callSetRule, callApplyR1, callCancelR1)
		f.expectState(t, "1", "1")
	})

	reject := func(body string) pushCase {
		return pushCase{
			name:      "revision " + body,
			faults:    map[string]func(int) *fault{"savepoint": always(fault{body: body})},
			wantSeq:   []string{callGetRule, callSavepoint},
			wantErr:   []string{"opnsense savepoint: missing or malformed revision"},
			wantSaved: "0", wantLive: "0",
		}
	}
	runPushCases(t, true, []pushCase{
		reject(`{"status":"ok","revision":null}`),
		reject(`{"status":"ok"}`),
		reject(`{"status":"ok","revision":-1}`),
		reject(`{"status":"ok","revision":1e9}`),
		reject(`{"status":"ok","revision":true}`),
		reject(`{"status":"ok","revision":{"a":1}}`),
		reject(`{"status":"ok","revision":["1712345678.1"]}`),
		reject(`{"status":"ok","revision":"1712345678.1/x"}`),
	})
}
