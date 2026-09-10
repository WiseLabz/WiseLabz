package store

import (
	"strconv"
	"time"
)

// SinceFromDays turns a "?days=" query value into an RFC3339 cutoff
// timestamp, defaultDays days before now when v is empty or invalid.
// defaultDays <= 0 means "no cutoff", returning "".
func SinceFromDays(v string, defaultDays int) string {
	days := defaultDays
	if v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = n
		}
	}
	if days <= 0 {
		return ""
	}
	return time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
}
