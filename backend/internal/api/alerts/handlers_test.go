package alerts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestListEmpty(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestGetNotFound(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/api/alerts/missing", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestResolveNotFound(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s)

	req := httptest.NewRequest(http.MethodPost, "/api/alerts/missing/resolve", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()
	h.Resolve(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestDismissNotFound(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s)

	req := httptest.NewRequest(http.MethodPost, "/api/alerts/missing/dismiss", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()
	h.Dismiss(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestSnooze(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s)

	t.Run("missing until", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/alerts/x/snooze", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "x")
		rr := httptest.NewRecorder()
		h.Snooze(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid timestamp", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/alerts/x/snooze", strings.NewReader(`{"until":"not-a-date"}`))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "x")
		rr := httptest.NewRecorder()
		h.Snooze(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/alerts/x/snooze", strings.NewReader(`{`))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "x")
		rr := httptest.NewRecorder()
		h.Snooze(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("valid until but alert not found", func(t *testing.T) {
		until := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
		req := httptest.NewRequest(http.MethodPost, "/api/alerts/missing/snooze", strings.NewReader(`{"until":"`+until+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "missing")
		rr := httptest.NewRecorder()
		h.Snooze(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})
}

func TestBulkSnooze(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s)

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/alerts/bulk-snooze", strings.NewReader(`{`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.BulkSnooze(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing until", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/alerts/bulk-snooze", strings.NewReader(`{"ids":["a"]}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.BulkSnooze(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid until", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/alerts/bulk-snooze", strings.NewReader(`{"ids":["a"],"until":"nope"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.BulkSnooze(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	until := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)

	t.Run("empty ids", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/alerts/bulk-snooze", strings.NewReader(`{"ids":[],"until":"`+until+`"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.BulkSnooze(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("too many ids", func(t *testing.T) {
		ids := make([]string, 501)
		for i := range ids {
			ids[i] = `"id` + strconv.Itoa(i) + `"`
		}
		body := `{"ids":[` + strings.Join(ids, ",") + `],"until":"` + until + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/alerts/bulk-snooze", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.BulkSnooze(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("unknown ids reported per-item, not fatal", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/alerts/bulk-snooze", strings.NewReader(`{"ids":["missing-1","missing-2"],"until":"`+until+`"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.BulkSnooze(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), `"not_found"`) {
			t.Errorf("expected per-item not_found reasons, got: %s", rr.Body.String())
		}
	})
}

func TestListServiceNameAndServerSeverity(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s)
	ctx := context.Background()
	c := store.ConnectorRecord{Name: "Router cluster", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
	if err := s.CreateConnector(ctx, &c); err != nil {
		t.Fatal(err)
	}
	user := apitest.NewUser(t, s, "viewer")
	apitest.GrantConnectorRole(t, s, user, c.ID, "viewer")
	for _, severity := range []string{"info", "warning", "critical"} {
		alert := store.AlertRecord{ServiceID: c.ID, Severity: severity, Title: severity}
		if err := s.CreateAlert(ctx, &alert); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/alerts?severity=critical&status=pending&pageSize=1", nil)
	req = req.WithContext(auth.ContextWithUser(ctx, user, false))
	rec := httptest.NewRecorder()
	h.List(rec, req)
	var page struct {
		Items []struct {
			ServiceName string `json:"serviceName"`
			Severity    string `json:"severity"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ServiceName != c.Name || page.Items[0].Severity != "critical" {
		t.Fatalf("list %s", rec.Body.String())
	}
}
