package discovery

import (
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
)

func TestNewTargetSetContainsOnlyUsableRangeHosts(t *testing.T) {
	ranges, err := ParseRanges([]string{"192.168.1.0/30"})
	if err != nil {
		t.Fatal(err)
	}
	targets := NewTargetSet(ranges)
	if got := targets.Len(); got != 2 {
		t.Fatalf("Len() = %d, want 2", got)
	}
	for _, address := range []string{"192.168.1.1", "192.168.1.2"} {
		addr := netip.MustParseAddr(address)
		if !targets.Contains(addr) || targets.Source(addr) != SourceRange {
			t.Errorf("range host %s missing or has source %q", addr, targets.Source(addr))
		}
	}
	for _, address := range []string{"192.168.1.0", "192.168.1.3"} {
		if targets.Contains(netip.MustParseAddr(address)) {
			t.Errorf("network/broadcast address %s was admitted", address)
		}
	}
}

func TestTargetSetAdmitChecksClassBudgetAndFirstSource(t *testing.T) {
	targets := NewTargetSet(nil)
	addr := netip.MustParseAddr("::ffff:100.64.0.1")
	if !targets.Admit(addr, SourceInventory) {
		t.Fatal("Admit(mapped CGNAT address) = false, want true")
	}
	if !targets.Contains(netip.MustParseAddr("100.64.0.1")) {
		t.Error("mapped IPv4 admission was not normalized")
	}
	if targets.Admit(netip.MustParseAddr("100.64.0.1"), SourcePassive) {
		t.Error("duplicate Admit = true, want false")
	}
	if got := targets.Source(netip.MustParseAddr("100.64.0.1")); got != SourceInventory {
		t.Errorf("Source(duplicate address) = %q, want %q", got, SourceInventory)
	}

	for _, address := range []string{"8.8.8.8", "127.0.0.1", "169.254.1.1", "224.0.0.1", "2001:4860::1"} {
		if targets.Admit(netip.MustParseAddr(address), SourcePassive) {
			t.Errorf("Admit(%s) = true, want disallowed", address)
		}
	}
	if targets.Admit(netip.MustParseAddr("10.0.0.1"), Source("unknown")) {
		t.Error("Admit with an unknown source = true, want false")
	}
	if targets.Len() != 1 {
		t.Errorf("Len() after rejected admissions = %d, want 1", targets.Len())
	}
	if got := targets.Source(netip.MustParseAddr("10.0.0.1")); got != "" {
		t.Errorf("Source(unadmitted address) = %q, want empty", got)
	}
}

func TestTargetSetBudgetIsAtomic(t *testing.T) {
	rangeValue, err := ParseRange("10.0.0.0/22")
	if err != nil {
		t.Fatal(err)
	}
	targets := NewTargetSet(Ranges{rangeValue})
	if !targets.Admit(netip.MustParseAddr("10.0.4.1"), SourceInventory) {
		t.Fatal("failed to seed 1023rd target")
	}

	var admitted atomic.Int32
	var wait sync.WaitGroup
	for i := 1; i <= 100; i++ {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			addr := netip.MustParseAddr(fmt.Sprintf("100.64.0.%d", i))
			source := SourcePassive
			if i%2 == 0 {
				source = SourceInventory
			}
			if targets.Admit(addr, source) {
				admitted.Add(1)
			}
		}(i)
	}
	wait.Wait()
	if got := admitted.Load(); got != 1 {
		t.Fatalf("concurrent admissions = %d, want exactly one remaining slot", got)
	}
	if got := targets.Len(); got != MaxAddresses {
		t.Fatalf("Len() = %d, want %d", got, MaxAddresses)
	}
}

func TestNilTargetSetIsEmpty(t *testing.T) {
	var targets *TargetSet
	if targets.Contains(netip.MustParseAddr("10.0.0.1")) || targets.Admit(netip.MustParseAddr("10.0.0.1"), SourceRange) {
		t.Fatal("nil TargetSet contains/admitted an address")
	}
	if targets.Len() != 0 || targets.Source(netip.MustParseAddr("10.0.0.1")) != "" {
		t.Fatal("nil TargetSet reported contents")
	}
}
