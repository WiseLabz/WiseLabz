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
