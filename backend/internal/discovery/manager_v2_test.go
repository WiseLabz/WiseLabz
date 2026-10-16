package discovery

import (
	"context"
	"errors"
	"net/netip"
	"reflect"
	"testing"
	"time"
)

type passiveFunc func(context.Context, func(netip.Addr))

func (f passiveFunc) Run(ctx context.Context, callback func(netip.Addr)) { f(ctx, callback) }

func addressesFrom(first string, count int) []netip.Addr {
	out := make([]netip.Addr, 0, count)
	for addr := netip.MustParseAddr(first); len(out) < count; addr = addr.Next() {
		out = append(out, addr)
	}
	return out
}

func TestManagerV2DeadlineScaling(t *testing.T) {
	m := NewManager(Config{})
	for _, tc := range []struct {
		static         int
		plain, passive time.Duration
	}{
		{static: 0, plain: time.Minute, passive: 2 * time.Minute},
		{static: 256, plain: time.Minute, passive: 2 * time.Minute},
		{static: 257, plain: 2 * time.Minute, passive: 3 * time.Minute},
		{static: 600, plain: 3 * time.Minute, passive: 4 * time.Minute},
		{static: 1024, plain: 4 * time.Minute, passive: 4 * time.Minute},
	} {
		if got := m.timeout(tc.static, false); got != tc.plain {
			t.Errorf("timeout(%d, false) = %v, want %v", tc.static, got, tc.plain)
		}
		if got := m.timeout(tc.static, true); got != tc.passive {
			t.Errorf("timeout(%d, true) = %v, want %v", tc.static, got, tc.passive)
		}
	}
}

func TestManagerV2InventoryBudget(t *testing.T) {
	ranges, err := ParseRanges([]string{"192.168.1.0/24", "192.168.2.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	var got Targets
	runner := fakeRunner{run: func(_ context.Context, targets Targets, cb Callbacks) Progress {
		got = targets
		p := Progress{Done: len(targets.Static), Total: targets.Set.Len()}
		cb.OnProgress(p)
		return p
	}}
	m := NewManager(Config{Runner: runner})
	started, err := m.Start(StartRequest{Ranges: ranges, IncludeInventory: true, Inventory: addressesFrom("10.50.0.1", 900)}, admin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if started.Total != 1024 || started.Sources != (SourceCounts{Range: 508, Inventory: 516}) || started.Skipped.Inventory != 384 {
		t.Fatalf("budget = %+v", started)
	}
	ended := waitEnded(t, m)
	if ended.Done != 1024 || got.Set.Len() != 1024 || len(got.Static) != 1024 {
		t.Fatalf("targets = %d, done = %d, set = %d", len(got.Static), ended.Done, got.Set.Len())
	}
	if got.Static[508] != netip.MustParseAddr("10.50.0.1") || got.Set.Contains(netip.MustParseAddr("10.50.3.132")) {
		t.Fatalf("inventory truncation was not deterministic")
	}
}

func TestManagerV2RejectsMissingTargetsAndRangeOverflow(t *testing.T) {
	m := NewManager(Config{})
	_, err := m.Start(StartRequest{}, admin, nil)
	var field *RangeError
	if !errors.As(err, &field) || field.Reason != ReasonNoTargets || field.Field != "ranges" {
		t.Fatalf("empty request = %v", err)
	}
	ranges := Ranges{}
	for _, cidr := range []string{"10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24", "10.0.4.0/24", "10.0.5.0/24"} {
		r, err := ParseRange(cidr)
		if err != nil {
			t.Fatal(err)
		}
		ranges = append(ranges, r)
	}
	_, err = m.Start(StartRequest{Ranges: ranges}, admin, nil)
	if !errors.As(err, &field) || field.Reason != ReasonTooManyAddresses {
		t.Fatalf("overflow = %v", err)
	}
	if len(m.starts) != 0 {
		t.Fatal("refused requests consumed rate limit")
	}
}

func TestManagerV2PassiveGrowsTotalAndKeepsFirstSource(t *testing.T) {
	ranges, err := ParseRanges([]string{"10.0.0.5/32"})
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	admitted := make(chan struct{})
	passive := passiveFunc(func(ctx context.Context, cb func(netip.Addr)) {
		select {
		case <-release:
		case <-ctx.Done():
			return
		}
		for _, raw := range []string{"10.0.0.5", "10.0.0.7", "10.0.0.8", "10.0.0.8", "8.8.8.8", "fe80::1"} {
			cb(netip.MustParseAddr(raw))
		}
		close(admitted)
	})
	runner := fakeRunner{run: func(_ context.Context, targets Targets, cb Callbacks) Progress {
		close(release)
		<-admitted
		for addr := range targets.Late {
			cb.OnCandidate(Candidate{Type: "fake", Address: addr.String(), Port: 443})
			cb.OnTLSTarget(TLSTarget{Address: addr.String(), Port: 443})
			cb.OnTLSTarget(TLSTarget{Address: addr.String(), Port: 443})
		}
		// An older progress callback must not overwrite the admitted total.
		cb.OnProgress(Progress{Done: 1, Total: 1})
		return Progress{}
	}}
	m := NewManager(Config{Runner: runner, Passive: passive})
	started, err := m.Start(StartRequest{Ranges: ranges, IncludeInventory: true, AskNetwork: true, Inventory: []netip.Addr{netip.MustParseAddr("10.0.0.7")}}, admin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if started.Total != 2 {
		t.Fatalf("initial total = %d", started.Total)
	}
	ended := waitEnded(t, m)
	if ended.Total != 3 || ended.PassiveAnswered != 3 || ended.Sources != (SourceCounts{Range: 1, Inventory: 1, Passive: 1}) || ended.Skipped != (SkippedCounts{}) {
		t.Fatalf("passive counts = %+v", ended)
	}
	if len(ended.Candidates) != 1 || ended.Candidates[0].Source != SourcePassive || len(ended.TLSTargets) != 1 || ended.TLSTargets[0].Source != SourcePassive {
		t.Fatalf("passive results = %+v", ended)
	}
}

func TestManagerV2PassiveOverflowCountsDistinctAnswers(t *testing.T) {
	ranges, err := ParseRanges([]string{"192.168.0.0/22"})
	if err != nil {
		t.Fatal(err)
	}
	inventory := []netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.2")}
	passive := passiveFunc(func(_ context.Context, cb func(netip.Addr)) {
		for _, raw := range []string{"10.0.0.1", "10.0.0.3", "10.0.0.4", "10.0.0.3"} {
			cb(netip.MustParseAddr(raw))
		}
	})
	runner := fakeRunner{run: func(_ context.Context, targets Targets, cb Callbacks) Progress {
		for range targets.Late {
			t.Error("full budget admitted a passive target")
		}
		cb.OnProgress(Progress{Done: 1024, Total: 1024})
		return Progress{}
	}}
	m := NewManager(Config{Runner: runner, Passive: passive})
	_, err = m.Start(StartRequest{Ranges: ranges, IncludeInventory: true, AskNetwork: true, Inventory: inventory}, admin, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := waitEnded(t, m)
	if got.Total != 1024 || got.Skipped.Passive != 2 || got.PassiveAnswered != 3 || got.Sources.Passive != 0 {
		t.Fatalf("overflow = %+v", got)
	}
}

func TestManagerV2InventoryOrderDeduplicatesAndIgnoresInvalid(t *testing.T) {
	ranges, err := ParseRanges([]string{"192.168.1.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	want := []netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("fd00::1")}
	m := NewManager(Config{Runner: fakeRunner{run: func(_ context.Context, targets Targets, cb Callbacks) Progress {
		if !reflect.DeepEqual(targets.Static[254:], want) {
			t.Errorf("inventory order = %v", targets.Static[254:])
		}
		cb.OnProgress(Progress{Done: 256, Total: 256})
		return Progress{}
	}}})
	inventory := []netip.Addr{netip.MustParseAddr("fd00::1"), netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("::ffff:10.0.0.1"), netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("192.168.1.0")}
	_, err = m.Start(StartRequest{Ranges: ranges, IncludeInventory: true, Inventory: inventory}, admin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := waitEnded(t, m); got.Sources.Inventory != 2 || got.Skipped.Inventory != 0 {
		t.Fatalf("inventory = %+v", got)
	}
}

func TestMarkMonitoredMatchesOnlyLiteralAddressAndPort(t *testing.T) {
	targets := []TLSTarget{
		{Address: "10.0.0.5", Port: 443},
		{Address: "10.0.0.5", Port: 8443},
		{Address: "fd00::5", Port: 443},
		{Address: "10.0.0.6", Port: 443, MonitoredBy: "stale"},
	}
	MarkMonitored(targets, []Endpoint{
		{ID: "probe", Type: "tlsprobe", Value: "10.0.0.5:443\n[fd00::5]:443\nnas.lab:443"},
		{ID: "other", Type: "proxmox", Value: "https://10.0.0.6"},
	})
	for i, want := range []string{"probe", "", "probe", ""} {
		if targets[i].MonitoredBy != want {
			t.Errorf("target %d marker = %q, want %q", i, targets[i].MonitoredBy, want)
		}
	}
}

func TestManagerV2RefreshesTLSMarkersOnEveryRead(t *testing.T) {
	endpoints := []Endpoint{{ID: "probe", Type: "tlsprobe", Value: "10.0.0.5:443"}}
	m := NewManager(Config{
		Runner: fakeRunner{run: func(_ context.Context, _ Targets, cb Callbacks) Progress {
			cb.OnTLSTarget(TLSTarget{Address: "10.0.0.5", Port: 443})
			return Progress{}
		}},
		Endpoints: func(context.Context) ([]Endpoint, error) { return endpoints, nil },
	})
	if _, err := m.Start(mustRange(t, "10.0.0.5/32"), admin, nil); err != nil {
		t.Fatal(err)
	}
	if got := waitEnded(t, m).TLSTargets; len(got) != 1 || got[0].MonitoredBy != "probe" {
		t.Fatalf("initial markers = %+v", got)
	}
	endpoints = nil
	if got, _ := m.Current(context.Background()); got.TLSTargets[0].MonitoredBy != "" {
		t.Fatalf("stale marker survived: %+v", got.TLSTargets)
	}
}

func TestManagerV2RefreshesAddOnMarkersWithoutMutatingStoredResults(t *testing.T) {
	endpoints := []Endpoint{{ID: "resolver", Type: "dnsresolver", Value: "https://10.0.0.5"}}
	candidate := Candidate{
		Type: "opnsense", Address: "10.0.0.5", Port: 443,
		AddOns: []AddOn{{Type: "dnsresolver", URL: "https://10.0.0.5:443"}},
	}
	m := NewManager(Config{
		Runner:    quickRunner(candidate),
		Endpoints: func(context.Context) ([]Endpoint, error) { return endpoints, nil },
	})
	if _, err := m.Start(mustRange(t, "10.0.0.5/32"), admin, nil); err != nil {
		t.Fatal(err)
	}
	got := waitEnded(t, m)
	if got.Candidates[0].AddOns[0].ConnectorID != "resolver" {
		t.Fatalf("add-on marker = %+v", got.Candidates)
	}
	got.Candidates[0].AddOns[0].URL = "https://10.0.0.99"
	endpoints = nil
	got, _ = m.Current(context.Background())
	if addOn := got.Candidates[0].AddOns[0]; addOn.ConnectorID != "" || addOn.URL != "https://10.0.0.5:443" {
		t.Fatalf("stale or externally modified add-on = %+v", addOn)
	}
	endpoints = []Endpoint{{ID: "wrong-port", Type: "dnsresolver", Value: "https://10.0.0.5:8443"}}
	got, _ = m.Current(context.Background())
	if got.Candidates[0].AddOns[0].ConnectorID != "" {
		t.Fatal("different port marked connected")
	}
}

func TestManagerV2AppliesScaledDeadlineToRunner(t *testing.T) {
	for _, size := range []int{256, 257, 600, 1024} {
		for _, ask := range []bool{false, true} {
			deadline := make(chan time.Time, 1)
			m := NewManager(Config{Runner: fakeRunner{run: func(ctx context.Context, _ Targets, _ Callbacks) Progress {
				end, ok := ctx.Deadline()
				if !ok {
					t.Error("scan context has no deadline")
				}
				deadline <- end
				return Progress{}
			}}})
			before := time.Now()
			if _, err := m.Start(StartRequest{
				IncludeInventory: true, AskNetwork: ask, Inventory: addressesFrom("10.0.0.1", size),
			}, admin, nil); err != nil {
				t.Fatal(err)
			}
			want := time.Duration((size+255)/256) * time.Minute
			if ask && size < 1024 {
				want = min(want+time.Minute, 4*time.Minute)
			}
			if got := (<-deadline).Sub(before); got < want || got > want+time.Second {
				t.Errorf("size %d passive %t: deadline in %v, want %v", size, ask, got, want)
			}
			m.Shutdown(context.Background())
		}
	}
}
