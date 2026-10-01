package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestAlertsDraftRunbook(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	ctx := context.Background()
	opUserID, opToken := app.user(t, "operator")
	_, viewerToken := app.user(t, "viewer")

	change := &store.ChangeRecord{ServiceID: "svc-1", ChangeType: "config_change", Severity: "warning", Summary: "Port 8080 opened", Diff: `{"added":"8080"}`}
	if err := app.Store.CreateChange(ctx, change); err != nil {
		t.Fatalf("create change: %v", err)
	}
	a := &store.AlertRecord{ServiceID: "svc-1", ChangeID: change.ID, Severity: "warning", Title: "Firewall drift", Description: "Rule set changed", Status: "pending"}
	if err := app.Store.CreateAlert(ctx, a); err != nil {
		t.Fatalf("create alert: %v", err)
	}
	app.connectorGrant(t, opUserID, "svc-1", "operator")

	t.Run("operator gets a draft and nothing is persisted", func(t *testing.T) {
		// AI is not configured in the test app: the draft must not need it.
		rec := app.req(t, http.MethodPost, "/api/alerts/"+a.ID+"/draft-runbook", nil, opToken)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
		}
		var d struct {
			Title             string `json:"title"`
			Body              string `json:"body"`
			TargetType        string `json:"targetType"`
			TargetValue       string `json:"targetValue"`
			ExistingRunbookID string `json:"existingRunbookId"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !strings.Contains(d.Body, "Firewall drift") || !strings.Contains(d.Body, "Port 8080 opened") || !strings.Contains(d.Body, "8080") {
			t.Errorf("body missing alert/change details: %s", d.Body)
		}
		if d.TargetType != "change_type" || d.TargetValue != "config_change" || d.ExistingRunbookID != "" {
			t.Errorf("draft = %+v", d)
		}
		list, err := app.Store.ListRunbooks(ctx)
		if err != nil || len(list) != 0 {
			t.Fatalf("runbooks = %+v, %v; want none persisted", list, err)
		}
	})

	t.Run("references an existing runbook for the target", func(t *testing.T) {
		rb, err := app.Store.CreateRunbook(ctx, &store.RunbookRecord{Title: "Drift playbook", Body: "x", TargetType: "change_type", TargetValue: "config_change"})
		if err != nil {
			t.Fatal(err)
		}
		rec := app.req(t, http.MethodPost, "/api/alerts/"+a.ID+"/draft-runbook", nil, opToken)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d; body = %s", rec.Code, rec.Body)
		}
		var d struct {
			Body              string `json:"body"`
			ExistingRunbookID string `json:"existingRunbookId"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &d)
		if d.ExistingRunbookID != rb.ID || !strings.Contains(d.Body, "Drift playbook") {
			t.Errorf("draft = %+v", d)
		}
	})

	t.Run("unknown alert is 404", func(t *testing.T) {
		rec := app.req(t, http.MethodPost, "/api/alerts/does-not-exist/draft-runbook", nil, opToken)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body)
		}
	})

	t.Run("viewer is forbidden", func(t *testing.T) {
		rec := app.req(t, http.MethodPost, "/api/alerts/"+a.ID+"/draft-runbook", nil, viewerToken)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body)
		}
	})
}
