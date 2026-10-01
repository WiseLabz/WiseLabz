package system

import (
	"crypto/subtle"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/httputil"
)

// Metrics handles GET /metrics in the Prometheus text exposition format. It is mounted only when
// metrics.enabled is set, and requires the configured token as a bearer credential. A scrape that
// fails to read part of the state returns 500 rather than partial data, so gaps show up as a
// failed scrape instead of silently-wrong gauges.
func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
	want := h.Config.Metrics.Token
	got, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if want == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "metrics token required")
		return
	}

	ctx := r.Context()
	var b strings.Builder
	gauge := func(name, help string) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s gauge\n", name, help, name)
	}

	changesNew, err := h.Store.CountChangesNew(ctx)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	alertsPending, err := h.Store.CountAllAlertsPending(ctx)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	findingsOpen, err := h.Store.CountQualityFindingsOpen(ctx)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	gauge("wiselabz_attention_items", "Open items needing attention, by kind.")
	fmt.Fprintf(&b, "wiselabz_attention_items{kind=\"changes_new\"} %d\n", changesNew)
	fmt.Fprintf(&b, "wiselabz_attention_items{kind=\"alerts_pending\"} %d\n", alertsPending)
	fmt.Fprintf(&b, "wiselabz_attention_items{kind=\"findings_open\"} %d\n", findingsOpen)

	connectors, err := h.Store.CountAllConnectorsByStatus(ctx)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	gauge("wiselabz_connectors", "Connectors by health status.")
	for _, status := range sortedKeys(connectors) {
		fmt.Fprintf(&b, "wiselabz_connectors{status=%q} %d\n", status, connectors[status])
	}

	stats, err := h.Store.SyncRunStats(ctx)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	gauge("wiselabz_sync_runs", "Retained sync runs by status (bounded by sync-run retention).")
	for _, st := range stats {
		fmt.Fprintf(&b, "wiselabz_sync_runs{status=%q} %d\n", st.Status, st.Count)
	}
	gauge("wiselabz_sync_run_duration_seconds", "Summed duration of retained sync runs, by status.")
	for _, st := range stats {
		fmt.Fprintf(&b, "wiselabz_sync_run_duration_seconds{status=%q} %g\n", st.Status, float64(st.DurationMs)/1000)
	}

	jobs, err := h.Store.ListJobHealth(ctx)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	gauge("wiselabz_job_up", "1 if the scheduled job's last run succeeded, 0 if it is failing.")
	for _, j := range jobs {
		up := 0
		if j.LastStatus == "ok" {
			up = 1
		}
		fmt.Fprintf(&b, "wiselabz_job_up{job=%q} %d\n", j.Name, up)
	}
	gauge("wiselabz_job_last_run_timestamp_seconds", "Unix time of the scheduled job's last run.")
	for _, j := range jobs {
		if ts, ok := parseUnix(j.LastRunAt); ok {
			fmt.Fprintf(&b, "wiselabz_job_last_run_timestamp_seconds{job=%q} %d\n", j.Name, ts)
		}
	}
	gauge("wiselabz_job_last_success_timestamp_seconds", "Unix time of the scheduled job's last success.")
	for _, j := range jobs {
		if ts, ok := parseUnix(j.LastSuccessAt); ok {
			fmt.Fprintf(&b, "wiselabz_job_last_success_timestamp_seconds{job=%q} %d\n", j.Name, ts)
		}
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = io.WriteString(w, b.String())
}

func parseUnix(s string) (int64, bool) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return 0, false
	}
	return t.Unix(), true
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
