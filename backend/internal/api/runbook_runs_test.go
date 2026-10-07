package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestRunRoutesExecuteAndAuthorize(t *testing.T) {
	for _, prefix := range []string{"/api", "/api/v1"} {
		t.Run(prefix, func(t *testing.T) {
			app := newTestApp(t)
			user, token := app.user(t, "viewer")
			connectorID := seedPreviewConnector(t, app, "Run operator", "virtualization", "custom", false)
			app.connectorGrant(t, user, connectorID, "operator")
			rb, _, err := app.Store.CreateRunbookWithSteps(context.Background(), &store.RunbookRecord{Title: "Manual recovery", TargetType: "change_type", TargetValue: "manual.route"}, []*store.RunbookStepRecord{{Kind: "manual", Title: "Confirm one"}, {Kind: "manual", Title: "Confirm two"}})
			if err != nil {
				t.Fatal(err)
			}
			path := prefix + "/runbooks/" + rb.ID
			rr := app.req(t, http.MethodPost, path+"/run?dryRun=true", nil, token)
			apitest.AssertMatchesSpec(t, app.newRequest(t, http.MethodPost, path+"/run?dryRun=true", nil, token), rr.Result())
			if rr.Code != 200 {
				t.Fatalf("preview=%d %s", rr.Code, rr.Body.String())
			}
			rr = app.req(t, http.MethodPost, path+"/run", nil, token)
			apitest.AssertMatchesSpec(t, app.newRequest(t, http.MethodPost, path+"/run", nil, token), rr.Result())
			if rr.Code != 400 {
				t.Fatalf("missing elevation=%d %s", rr.Code, rr.Body.String())
			}
			rr = app.reqElevated(t, http.MethodPost, path+"/run", nil, token, "runbook.run", rb.ID)
			apitest.AssertMatchesSpec(t, app.newRequest(t, http.MethodPost, path+"/run", nil, token), rr.Result())
			if rr.Code != 202 {
				t.Fatalf("start=%d %s", rr.Code, rr.Body.String())
			}
			var started store.RunbookRunRecord
			if err := json.Unmarshal(rr.Body.Bytes(), &started); err != nil {
				t.Fatal(err)
			}
			wait := func(state string) []*store.RunbookRunStepRecord {
				t.Helper()
				deadline := time.Now().Add(3 * time.Second)
				for time.Now().Before(deadline) {
					run, steps, err := app.Store.GetRunbookRun(context.Background(), started.ID)
					if err != nil {
						t.Fatal(err)
					}
					if run.State == state {
						return steps
					}
					time.Sleep(5 * time.Millisecond)
				}
				t.Fatalf("run never reached %s", state)
				return nil
			}
			steps := wait("waiting_manual")
			rr = app.reqElevated(t, http.MethodPost, path+"/run", nil, token, "runbook.run", rb.ID)
			apitest.AssertMatchesSpec(t, app.newRequest(t, http.MethodPost, path+"/run", nil, token), rr.Result())
			if rr.Code != 409 {
				t.Fatalf("second start=%d %s", rr.Code, rr.Body.String())
			}
			runPath := prefix + "/runbook-runs/" + started.ID
			rr = app.req(t, http.MethodGet, path+"/runs", nil, token)
			apitest.AssertMatchesSpec(t, app.newRequest(t, http.MethodGet, path+"/runs", nil, token), rr.Result())
			if rr.Code != 200 {
				t.Fatalf("list=%d %s", rr.Code, rr.Body.String())
			}
			rr = app.req(t, http.MethodGet, runPath, nil, token)
			apitest.AssertMatchesSpec(t, app.newRequest(t, http.MethodGet, runPath, nil, token), rr.Result())
			if rr.Code != 200 {
				t.Fatalf("get=%d %s", rr.Code, rr.Body.String())
			}
			rr = app.req(t, http.MethodPost, runPath+"/resume", nil, token)
			apitest.AssertMatchesSpec(t, app.newRequest(t, http.MethodPost, runPath+"/resume", nil, token), rr.Result())
			if rr.Code != 409 {
				t.Fatalf("resume waiting=%d %s", rr.Code, rr.Body.String())
			}
			rr = app.req(t, http.MethodPost, runPath+"/steps/"+steps[0].ID+"/confirm", nil, token)
			apitest.AssertMatchesSpec(t, app.newRequest(t, http.MethodPost, runPath+"/steps/"+steps[0].ID+"/confirm", nil, token), rr.Result())
			if rr.Code != 204 {
				t.Fatalf("confirm=%d %s", rr.Code, rr.Body.String())
			}
			wait("waiting_manual")
			rr = app.req(t, http.MethodPost, runPath+"/cancel", nil, token)
			apitest.AssertMatchesSpec(t, app.newRequest(t, http.MethodPost, runPath+"/cancel", nil, token), rr.Result())
			if rr.Code != 204 {
				t.Fatalf("cancel=%d %s", rr.Code, rr.Body.String())
			}
			wait("cancelled")
			records, total, err := app.Store.ListAuditRecords(context.Background(), "runbook.run.start", "runbook_run", "", "", 0, 10)
			if err != nil || total != 1 || records[0].ActorUserID != user {
				t.Fatalf("audit=%+v total=%d err=%v", records, total, err)
			}
		})
	}
}

func TestRunStartWithRealElevation(t *testing.T) {
	app := newTestApp(t)
	user, token := app.user(t, "viewer")
	connectorID := seedPreviewConnector(t, app, "Run operator", "virtualization", "custom", false)
	app.connectorGrant(t, user, connectorID, "operator")
	ctx := context.Background()
	rb, _, err := app.Store.CreateRunbookWithSteps(ctx, &store.RunbookRecord{Title: "Manual recovery", TargetType: "change_type", TargetValue: "manual.elevate"}, []*store.RunbookStepRecord{{Kind: "manual", Title: "Confirm one"}})
	if err != nil {
		t.Fatal(err)
	}
	elevate := func(target string) string {
		t.Helper()
		rr := app.req(t, http.MethodPost, "/api/auth/elevate", map[string]any{"password": "password123", "action": "runbook.run", "target": target}, token)
		if rr.Code != http.StatusOK {
			t.Fatalf("elevate %q=%d %s", target, rr.Code, rr.Body.String())
		}
		var resp struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil || resp.Token == "" {
			t.Fatalf("elevate body=%s err=%v", rr.Body.String(), err)
		}
		return resp.Token
	}
	start := func(elevation string) *httptest.ResponseRecorder {
		r := app.newRequest(t, http.MethodPost, "/api/runbooks/"+rb.ID+"/run", nil, token)
		r.Header.Set("X-Elevation-Token", elevation)
		return app.serve(r)
	}

	if rr := start(elevate("another-runbook")); rr.Code != http.StatusUnauthorized {
		t.Fatalf("other target=%d %s", rr.Code, rr.Body.String())
	}
	if runs, _, err := app.Store.ListRunbookRuns(ctx, rb.ID, 10, 0); err != nil || len(runs) != 0 {
		t.Fatalf("runs after rejected start=%v err=%v", runs, err)
	}
	if rr := start(elevate(rb.ID)); rr.Code != http.StatusAccepted {
		t.Fatalf("start=%d %s", rr.Code, rr.Body.String())
	}
	records, total, err := app.Store.ListAuditRecords(ctx, "runbook.run.start", "runbook_run", "", "", 0, 10)
	if err != nil || total != 1 || records[0].ActorUserID != user {
		t.Fatalf("audit=%+v total=%d err=%v", records, total, err)
	}
}
