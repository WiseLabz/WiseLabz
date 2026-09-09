package connector

import (
	"errors"
	"testing"
	"time"
)

func TestClassifyHealth(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		latency    time.Duration
		wantStatus string
	}{
		{"fast success is online", nil, 10 * time.Millisecond, "online"},
		{"slow success is degraded", nil, 3 * time.Second, "degraded"},
		{"error is offline", errors.New("connection refused"), 5 * time.Millisecond, "offline"},
		{"slow error is still offline", errors.New("timeout"), 3 * time.Second, "offline"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, message := ClassifyHealth(tt.err, tt.latency)
			if status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status, tt.wantStatus)
			}
			if message == "" {
				t.Error("message = \"\", want non-empty")
			}
		})
	}
}

func TestClassifyHealthPerTypeThreshold(t *testing.T) {
	// A latency that's fine under a looser per-type threshold but would be
	// "degraded" under the package default.
	latency := 3 * time.Second
	if status, _ := ClassifyHealth(nil, latency); status != "degraded" {
		t.Fatalf("status with default threshold = %q, want degraded", status)
	}
	if status, _ := ClassifyHealth(nil, latency, 5*time.Second); status != "online" {
		t.Errorf("status with 5s override = %q, want online", status)
	}
	// A zero override falls back to the package default rather than always-online.
	if status, _ := ClassifyHealth(nil, latency, 0); status != "degraded" {
		t.Errorf("status with zero override = %q, want degraded (falls back to default)", status)
	}
}
