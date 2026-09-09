package connector

import (
	"fmt"
	"time"
)

// DegradedLatencyThreshold is the Validate() latency above which a
// successful health check is reported as "degraded" rather than "online".
// A var (not const) so tests can override it instead of sleeping for real.
// ponytail: fixed threshold, make per-connector configurable if a real
// deployment needs different SLAs.
var DegradedLatencyThreshold = 2 * time.Second

// ClassifyHealth turns the outcome of a cheap connectivity check (Validate)
// into one of the ServiceStatus values used for a connector's persisted
// status: "online", "degraded", or "offline". It never returns "unknown" —
// that value is reserved for connectors that have never been checked.
//
// threshold optionally overrides DegradedLatencyThreshold (e.g. with a
// connector type's own TypeSchema.DegradedLatencyThreshold()); pass none to
// use the package default.
func ClassifyHealth(err error, latency time.Duration, threshold ...time.Duration) (status, message string) {
	limit := DegradedLatencyThreshold
	if len(threshold) > 0 && threshold[0] > 0 {
		limit = threshold[0]
	}
	if err != nil {
		return "offline", err.Error()
	}
	if latency > limit {
		return "degraded", fmt.Sprintf("Slow response (%s)", latency.Round(time.Millisecond))
	}
	return "online", "Healthy"
}
