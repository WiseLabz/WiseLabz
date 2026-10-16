package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/netip"
	"reflect"
	"strings"
	"testing"
)

func TestManagerV2CompleteAuditIncludesAdmissionAndResultCounts(t *testing.T) {
	ranges, err := ParseRanges([]string{"192.168.0.0/22"})
	if err != nil {
		t.Fatal(err)
	}
	passiveFinished := make(chan struct{})
	passive := passiveFunc(func(_ context.Context, cb func(netip.Addr)) {
		cb(netip.MustParseAddr("192.168.0.5")) // already included by the typed range
		cb(netip.MustParseAddr("10.60.0.1"))   // the budget is full; this is skipped
		cb(netip.MustParseAddr("10.60.0.1"))   // duplicate answers count once
		close(passiveFinished)
	})
	audit := &fakeAudit{}
	runner := fakeRunner{run: func(_ context.Context, targets Targets, cb Callbacks) Progress {
		<-passiveFinished
		for range targets.Late {
		}
		cb.OnCandidate(Candidate{Type: "proxmox", Address: "10.50.0.1", Port: 8006})
		cb.OnTLSTarget(TLSTarget{Address: "192.168.0.5", Port: 443})
		cb.OnTLSTarget(TLSTarget{Address: "192.168.0.5", Port: 443}) // duplicate target
		cb.OnTLSTarget(TLSTarget{Address: "10.50.0.1", Port: 443})
		cb.OnProgress(Progress{Done: 1001, Total: MaxAddresses, Answered: 4})
		return Progress{}
	}}
	m := NewManager(Config{Runner: runner, Passive: passive, Audit: audit, Now: newClock().now})
	started, err := m.Start(StartRequest{
		Ranges:           ranges,
		IncludeInventory: true,
		AskNetwork:       true,
		Inventory: []netip.Addr{
			netip.MustParseAddr("10.50.0.3"),
			netip.MustParseAddr("10.50.0.1"),
			netip.MustParseAddr("10.50.0.2"),
		},
	}, Actor{UserID: "starter-complete", InstanceAdmin: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if started.Total != MaxAddresses || started.Sources != (SourceCounts{Range: 1022, Inventory: 2}) || started.Skipped.Inventory != 1 {
		t.Fatalf("initial admission = %+v", started)
	}
	ended := waitEnded(t, m)
	if ended.State != StateCompleted || ended.Done != 1001 || ended.Answered != 4 || ended.Partial {
		t.Fatalf("completed scan = %+v", ended)
	}
	entries := audit.all()
	if len(entries) != 1 {
		t.Fatalf("audit entries = %+v, want one completion entry", entries)
	}
	entry := entries[0]
	if entry.actor != "starter-complete" || !entry.admin || entry.action != AuditComplete || entry.targetType != AuditTargetType || entry.targetID != started.ID {
		t.Errorf("audit metadata = %+v", entry)
	}
	assertAuditV2Detail(t, entry.detail, map[string]any{
		"ranges":          []any{"192.168.0.0/22"},
		"state":           string(StateCompleted),
		"probed":          float64(1001),
		"answered":        float64(4),
		"candidates":      map[string]any{"proxmox": float64(1)},
		"durationMs":      float64(0),
		"partial":         false,
		"sources":         map[string]any{"range": float64(1022), "inventory": float64(2), "passive": float64(0)},
		"skipped":         map[string]any{"inventory": float64(1), "passive": float64(1)},
		"passiveAnswered": float64(2),
		"tlsTargets":      float64(2),
	})
}

func TestManagerV2CancelAuditRetainsStarterAndCounts(t *testing.T) {
	ranges, err := ParseRanges([]string{"192.168.0.0/22"})
	if err != nil {
		t.Fatal(err)
	}
	passiveFinished := make(chan struct{})
	passive := passiveFunc(func(_ context.Context, cb func(netip.Addr)) {
		cb(netip.MustParseAddr("192.168.0.5"))
		cb(netip.MustParseAddr("10.60.0.1"))
		cb(netip.MustParseAddr("10.60.0.1"))
		close(passiveFinished)
	})
	runnerReady := make(chan struct{})
	audit := &fakeAudit{}
	runner := fakeRunner{run: func(ctx context.Context, targets Targets, cb Callbacks) Progress {
		<-passiveFinished
		for range targets.Late {
		}
		cb.OnCandidate(Candidate{Type: "proxmox", Address: "10.50.0.1", Port: 8006})
		cb.OnTLSTarget(TLSTarget{Address: "192.168.0.5", Port: 443})
		cb.OnTLSTarget(TLSTarget{Address: "192.168.0.5", Port: 443})
		cb.OnTLSTarget(TLSTarget{Address: "10.50.0.1", Port: 443})
		cb.OnProgress(Progress{Done: 333, Total: MaxAddresses, Answered: 6})
		close(runnerReady)
		<-ctx.Done()
		return Progress{}
	}}
	m := NewManager(Config{Runner: runner, Passive: passive, Audit: audit, Now: newClock().now})
	started, err := m.Start(StartRequest{
		Ranges:           ranges,
		IncludeInventory: true,
		AskNetwork:       true,
		Inventory: []netip.Addr{
			netip.MustParseAddr("10.50.0.3"),
			netip.MustParseAddr("10.50.0.1"),
			netip.MustParseAddr("10.50.0.2"),
		},
	}, Actor{UserID: "starter-cancel", InstanceAdmin: false}, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-runnerReady
	cancelled, ok := m.Cancel(Actor{UserID: "canceller", InstanceAdmin: true})
	if !ok || cancelled.State != StateCancelled || cancelled.ID != started.ID {
		t.Fatalf("Cancel() = %+v, %t", cancelled, ok)
	}
	ended := waitEnded(t, m)
	if ended.State != StateCancelled || ended.Done != 333 || ended.Answered != 6 || ended.Partial {
		t.Fatalf("cancelled scan = %+v", ended)
	}
	entries := audit.all()
	if len(entries) != 1 {
		t.Fatalf("audit entries = %+v, want one cancellation entry", entries)
	}
	entry := entries[0]
	if entry.actor != "starter-cancel" || entry.admin || entry.action != AuditCancel || entry.targetType != AuditTargetType || entry.targetID != started.ID {
		t.Errorf("cancel audit metadata = %+v", entry)
	}
	assertAuditV2Detail(t, entry.detail, map[string]any{
		"ranges":          []any{"192.168.0.0/22"},
		"state":           string(StateCancelled),
		"probed":          float64(333),
		"answered":        float64(6),
		"candidates":      map[string]any{"proxmox": float64(1)},
		"durationMs":      float64(0),
		"partial":         false,
		"sources":         map[string]any{"range": float64(1022), "inventory": float64(2), "passive": float64(0)},
		"skipped":         map[string]any{"inventory": float64(1), "passive": float64(1)},
		"passiveAnswered": float64(2),
		"tlsTargets":      float64(2),
		"cancelledBy":     "canceller",
	})
}

func TestManagerV2AuditContainsNoHostAddresses(t *testing.T) {
	ranges, err := ParseRanges([]string{"192.168.77.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	passive := passiveFunc(func(_ context.Context, cb func(netip.Addr)) {
		cb(netip.MustParseAddr("192.168.77.12"))
		cb(netip.MustParseAddr("10.90.0.10"))
		cb(netip.MustParseAddr("10.90.0.11"))
	})
	runner := fakeRunner{run: func(_ context.Context, targets Targets, cb Callbacks) Progress {
		for range targets.Late {
		}
		cb.OnCandidate(Candidate{Type: "proxmox", Address: "192.168.77.10", Port: 8006})
		cb.OnCandidate(Candidate{Type: "home_assistant", Address: "10.80.0.10", Port: 8123})
		cb.OnCandidate(Candidate{Type: "fake", Address: "10.90.0.10", Port: 443})
		cb.OnTLSTarget(TLSTarget{Address: "192.168.77.11", Port: 443})
		cb.OnTLSTarget(TLSTarget{Address: "10.80.0.11", Port: 443})
		cb.OnTLSTarget(TLSTarget{Address: "10.90.0.11", Port: 443})
		cb.OnProgress(Progress{Done: 258, Total: 258, Answered: 5})
		return Progress{}
	}}
	audit := &fakeAudit{}
	m := NewManager(Config{Runner: runner, Passive: passive, Audit: audit, Now: newClock().now})
	_, err = m.Start(StartRequest{
		Ranges:           ranges,
		IncludeInventory: true,
		AskNetwork:       true,
		Inventory: []netip.Addr{
			netip.MustParseAddr("10.80.0.11"),
			netip.MustParseAddr("192.168.77.13"), // ignored because the range already includes it
			netip.MustParseAddr("10.80.0.10"),
		},
	}, Actor{UserID: "privacy-test", InstanceAdmin: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	waitEnded(t, m)
	entries := audit.all()
	if len(entries) != 1 {
		t.Fatalf("audit entries = %+v, want one completion entry", entries)
	}
	raw, err := json.Marshal(entries[0].detail)
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(raw)
	if !strings.Contains(serialized, "192.168.77.0/24") {
		t.Errorf("audit detail omitted the typed range: %s", serialized)
	}
	for _, host := range []string{
		"192.168.77.10", "192.168.77.11", "192.168.77.12", "192.168.77.13",
		"10.80.0.10", "10.80.0.11", "10.90.0.10", "10.90.0.11",
	} {
		if strings.Contains(serialized, host) {
			t.Errorf("audit detail leaked host %s: %s", host, serialized)
		}
	}
}

func TestManagerV2SanitizesAuditorAndEndpointErrors(t *testing.T) {
	secretErr := errors.New("10.99.0.1\nsecret-key")
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	runner := fakeRunner{run: func(_ context.Context, _ Targets, cb Callbacks) Progress {
		cb.OnCandidate(Candidate{Type: "fake", Address: "10.99.0.1", Port: 443})
		cb.OnProgress(Progress{Done: 1, Total: 1})
		return Progress{}
	}}
	audit := failingAuditor{err: secretErr}
	m := NewManager(Config{
		Runner:    runner,
		Audit:     audit,
		Endpoints: func(context.Context) ([]Endpoint, error) { return nil, secretErr },
	})
	if _, err := m.Start(mustRange(t, "10.0.0.0/24"), admin, nil); err != nil {
		t.Fatal(err)
	}
	waitEnded(t, m)
	m.Shutdown(context.Background())
	if got := logs.String(); strings.Contains(got, "10.99.0.1") || strings.Contains(got, "secret-key") {
		t.Errorf("error logs leaked details or included injected lines: %q", got)
	}
}

type failingAuditor struct{ err error }

func (a failingAuditor) RecordAuditAs(context.Context, string, bool, string, string, string, any) error {
	return a.err
}

func assertAuditV2Detail(t *testing.T, got, want map[string]any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("audit detail = %#v, want %#v", got, want)
	}
}
