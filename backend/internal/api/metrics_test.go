package api_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/config"
)

func TestMetricsDisabledByDefault(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	rec := app.req(t, http.MethodGet, "/metrics", nil, "")
	if rec.Code == http.StatusOK {
		t.Fatalf("GET /metrics = 200 with metrics disabled; body = %s", rec.Body)
	}
}

func TestMetricsRequiresToken(t *testing.T) {
	t.Parallel()
	app := newTestAppWithOptions(t, t.TempDir(), func(c *config.Config) {
		c.Metrics = config.MetricsSettings{Enabled: true, Token: "scrape-token"}
	})
	for _, token := range []string{"", "wrong"} {
		if rec := app.req(t, http.MethodGet, "/metrics", nil, token); rec.Code != http.StatusUnauthorized {
			t.Errorf("token %q: GET /metrics = %d, want 401", token, rec.Code)
		}
	}
}

func TestMetricsExposesState(t *testing.T) {
	t.Parallel()
	app := newTestAppWithOptions(t, t.TempDir(), func(c *config.Config) {
		c.Metrics = config.MetricsSettings{Enabled: true, Token: "scrape-token"}
	})
	rec := app.req(t, http.MethodGet, "/metrics", nil, "scrape-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /metrics = %d; body = %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`wiselabz_attention_items{kind="changes_new"} 0`,
		`wiselabz_attention_items{kind="alerts_pending"} 0`,
		`wiselabz_attention_items{kind="findings_open"} 0`,
		`wiselabz_connectors{status="online"} 0`,
		"# TYPE wiselabz_job_up gauge",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics output missing %q:\n%s", want, body)
		}
	}
}
