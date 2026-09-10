package store

import (
	"testing"
	"time"
)

func TestSinceFromDays(t *testing.T) {
	tests := []struct {
		name        string
		v           string
		defaultDays int
		wantDays    int // 0 means want ""
	}{
		{"empty uses default", "", 7, 7},
		{"empty with zero default means no cutoff", "", 0, 0},
		{"valid overrides default", "3", 7, 3},
		{"zero is invalid, falls back to default", "0", 7, 7},
		{"negative is invalid, falls back to default", "-1", 7, 7},
		{"malformed is invalid, falls back to default", "abc", 7, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SinceFromDays(tt.v, tt.defaultDays)
			if tt.wantDays == 0 {
				if got != "" {
					t.Fatalf("SinceFromDays(%q, %d) = %q, want \"\"", tt.v, tt.defaultDays, got)
				}
				return
			}
			want := time.Now().UTC().AddDate(0, 0, -tt.wantDays)
			gotTime, err := time.Parse(time.RFC3339, got)
			if err != nil {
				t.Fatalf("SinceFromDays(%q, %d) = %q, not RFC3339: %v", tt.v, tt.defaultDays, got, err)
			}
			if diff := want.Sub(gotTime); diff < -time.Minute || diff > time.Minute {
				t.Fatalf("SinceFromDays(%q, %d) = %v, want ~%v", tt.v, tt.defaultDays, gotTime, want)
			}
		})
	}
}
