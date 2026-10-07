package connector

import (
	"fmt"
	"regexp"
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

// Reserved snapshot metadata keys by which a connector reports a status that
// only its fetch can see (Validate runs without stored snapshots). The sync
// engine honours MetadataHealthStatus when a sync succeeds.
const (
	MetadataHealthStatus  = "health_status"
	MetadataHealthMessage = "health_message"
)

// ReportOffline marks a successful fetch's metadata as an offline connector:
// the sync still succeeds and stores its entities, but the connector's status
// becomes offline with the message AllTargetsUnreachable(n).
func ReportOffline(metadata map[string]string, targets int) {
	metadata[MetadataHealthStatus] = "offline"
	metadata[MetadataHealthMessage] = AllTargetsUnreachable(targets)
}

// SnapshotOfflineMessage returns the offline message a snapshot reports via
// ReportOffline, if it does.
func SnapshotOfflineMessage(sn *ServiceSnapshot) (string, bool) {
	if sn == nil || sn.Metadata[MetadataHealthStatus] != "offline" {
		return "", false
	}
	if msg := sn.Metadata[MetadataHealthMessage]; msg != "" {
		return msg, true
	}
	return "Offline", true
}

// AllTargetsUnreachable is the status message of a connector none of whose
// targets could be reached.
func AllTargetsUnreachable(n int) string {
	if n == 1 {
		return "All 1 target unreachable"
	}
	return fmt.Sprintf("All %d targets unreachable", n)
}

var allTargetsUnreachablePattern = regexp.MustCompile(`^All \d+ targets? unreachable$`)

// IsAllTargetsUnreachable reports whether a stored status message is the one
// AllTargetsUnreachable wrote. A health check, which cannot see fetch results,
// leaves such an offline status for the next sync to clear.
func IsAllTargetsUnreachable(message string) bool {
	return allTargetsUnreachablePattern.MatchString(message)
}
