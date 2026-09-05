package sync

import (
	"testing"
	"time"
)

func TestComputeNextRun_ManualOnlyNeverSchedules(t *testing.T) {
	now := time.Now()
	next, retryCount := computeNextRun(nil, 3, false, now)
	if next != nil {
		t.Fatalf("nextRunAt = %v, want nil (manual-only connector)", next)
	}
	if retryCount != 4 {
		t.Fatalf("retryCount = %d, want 4 (still tracked for display)", retryCount)
	}

	next, retryCount = computeNextRun(nil, 3, true, now)
	if next != nil {
		t.Fatalf("nextRunAt = %v, want nil (manual-only connector)", next)
	}
	if retryCount != 0 {
		t.Fatalf("retryCount = %d, want 0 after success", retryCount)
	}
}

func TestComputeNextRun_SuccessSchedulesAtCadenceAndResetsRetries(t *testing.T) {
	schedule := 1800 // 30 minutes
	now := time.Now()

	next, retryCount := computeNextRun(&schedule, 4, true, now)
	if retryCount != 0 {
		t.Fatalf("retryCount = %d, want 0 after success", retryCount)
	}
	if next == nil {
		t.Fatal("nextRunAt = nil, want scheduled time")
	}
	want := now.Add(30 * time.Minute)
	if diff := next.Sub(want); diff < -time.Second || diff > time.Second {
		t.Fatalf("nextRunAt = %v, want ~%v", *next, want)
	}
}

func TestComputeNextRun_FailureUsesBackoffSchedule(t *testing.T) {
	schedule := 7200 // 2h cadence, larger than every backoff step
	now := time.Now()

	cases := []struct {
		priorRetryCount int
		wantDelay       time.Duration
	}{
		{0, time.Minute},     // 1st failure -> retrySchedule[0]
		{1, 5 * time.Minute}, // 2nd failure -> retrySchedule[1]
		{4, time.Hour},       // 5th failure -> retrySchedule[4] (last step)
		{10, time.Hour},      // way past exhaustion -> still last step, never silent
	}
	for _, tc := range cases {
		next, retryCount := computeNextRun(&schedule, tc.priorRetryCount, false, now)
		if retryCount != tc.priorRetryCount+1 {
			t.Fatalf("priorRetryCount=%d: retryCount = %d, want %d", tc.priorRetryCount, retryCount, tc.priorRetryCount+1)
		}
		if next == nil {
			t.Fatalf("priorRetryCount=%d: nextRunAt = nil, want scheduled time", tc.priorRetryCount)
		}
		want := now.Add(tc.wantDelay)
		if diff := next.Sub(want); diff < -time.Second || diff > time.Second {
			t.Fatalf("priorRetryCount=%d: nextRunAt = %v, want ~%v (delay %v)", tc.priorRetryCount, *next, want, tc.wantDelay)
		}
	}
}

func TestComputeNextRun_BackoffNeverExceedsScheduleCadence(t *testing.T) {
	schedule := 120 // 2 minutes, shorter than the 1h max backoff step
	now := time.Now()

	// Even at the longest backoff step, a short-cadence connector must not
	// wait longer than its own schedule to retry.
	next, retryCount := computeNextRun(&schedule, 10, false, now)
	if retryCount != 11 {
		t.Fatalf("retryCount = %d, want 11", retryCount)
	}
	want := now.Add(2 * time.Minute)
	if diff := next.Sub(want); diff < -time.Second || diff > time.Second {
		t.Fatalf("nextRunAt = %v, want ~%v (capped at schedule cadence)", *next, want)
	}
}
