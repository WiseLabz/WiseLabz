package runbooks

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/runbookrun"
)

func TestConnectorActionPreviewShowsExactRequestAndDowntime(t *testing.T) {
	h := newTestHandler(t)
	server, hits := countingServer(t)
	recipe := strings.Replace(serviceRescanRecipe, "downtime_seconds: 0", "downtime_seconds: 37", 1)
	conn := seedActionConnector(t, h, server.URL, recipe)
	seedActionSnapshot(t, h, conn)
	user := operatorOn(t, h, conn)
	id := decodeRunbook(t, createWithSteps(t, h, user, "exact-preview", connectorActionStep(conn, "rescan", "", ""))).ID
	preview := previewStart(t, h, user, id).Steps[0].Preview
	if preview == nil || preview.Request == nil {
		t.Fatal("preview omitted request")
	}
	request := preview.Request
	if request.Method != "POST" || request.URL != server.URL+"/rescan?source=library" || request.Headers["X-Mode"] != "safe" ||
		!reflect.DeepEqual(request.Body, map[string]any{"scope": "all"}) || preview.EstimatedDowntimeSeconds != 37 {
		t.Fatalf("preview does not match declared request: %+v, %+v", preview, request)
	}
	if hits.Load() != 0 {
		t.Fatalf("preview sent %d requests", hits.Load())
	}
}

func TestUnknownActionResumeWithoutElevationKeepsStateAndSendsNothing(t *testing.T) {
	f := newUnknownActionFixture(t)
	f.h.Executor = runbookrun.New(runbookrun.Deps{Store: f.h.Store, Actions: f.h.ConnH,
		Grants: runbookrun.StoreGrants{Store: f.h.Store}, Spawner: goSpawner{}})
	run, frozen := seedUnknownActionRun(t, f.h, f.runbookID, f.user, f.fingerprint)
	request := runRequest(f.user, f.runbookID, run.ID, "")
	// No decision can authorize state mutation without fresh elevation.
	request.Body = io.NopCloser(strings.NewReader(`{"decision":"mark_done"}`))
	rr := httptest.NewRecorder()
	f.h.ResumeRun(rr, request)
	assertRunStatus(t, rr, http.StatusBadRequest)
	if !strings.Contains(rr.Body.String(), "elevation_required") {
		t.Fatalf("resume rejection was not elevation: %s", rr.Body.String())
	}
	got, after, err := f.h.Store.GetRunbookRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "failed" || got.UpdatedAt != run.UpdatedAt || after[0].ID != frozen[0].ID || after[0].State != "unknown" || f.hits.Load() != 0 {
		t.Fatalf("unelevated resume mutated or sent: run=%+v step=%+v sends=%d", got, after[0], f.hits.Load())
	}
	assertNoRunAudit(t, f.h, "runbook.run.step_marked_done")
	assertNoRunAudit(t, f.h, "runbook.run.resume")
	// The same authorized request succeeds with the correct fresh token.
	rr = resumeWithBody(t, f.h, f.user, f.runbookID, run.ID, `{"decision":"mark_done"}`)
	assertRunStatus(t, rr, http.StatusAccepted)
	waitRunState(t, f.h, run.ID, "waiting_manual")
	if f.hits.Load() != 0 {
		t.Fatalf("mark_done positive control sent %d requests", f.hits.Load())
	}
}

func TestUnknownActionResumeShutdownAuditsAppliedDecision(t *testing.T) {
	for _, decision := range []string{"resend", "mark_done"} {
		t.Run(decision, func(t *testing.T) {
			f := newUnknownActionFixture(t)
			f.h.Executor = runbookrun.New(runbookrun.Deps{Store: f.h.Store, Actions: f.h.ConnH,
				Grants: runbookrun.StoreGrants{Store: f.h.Store}, Spawner: runSpawner{false}})
			run, frozen := seedUnknownActionRun(t, f.h, f.runbookID, f.user, f.fingerprint)
			connID := frozen[0].ConnectorID
			resumer := operatorOn(t, f.h, connID)
			rr := resumeWithBody(t, f.h, resumer, f.runbookID, run.ID, `{"decision":"`+decision+`"}`)
			assertRunStatus(t, rr, http.StatusServiceUnavailable)
			got, after, err := f.h.Store.GetRunbookRun(context.Background(), run.ID)
			if err != nil {
				t.Fatal(err)
			}
			wantState := "unknown"
			action := "runbook.run.step_resent"
			if decision == "mark_done" {
				wantState = "succeeded"
				action = "runbook.run.step_marked_done"
			}
			if got.State != "failed" || after[0].State != wantState || got.ResumedBy == nil || *got.ResumedBy != resumer || f.hits.Load() != 0 {
				t.Fatalf("shutdown resume state = %+v, %+v, sends=%d", got, after[0], f.hits.Load())
			}
			assertStepDecisionAudit(t, f.h, action, run.ID, frozen[0].ID, decision)
			assertRunAudit(t, f.h, "runbook.run.resume", run.ID, f.runbookID, resumer)
			rows, _, err := f.h.Store.ListAuditRecords(context.Background(), action, "runbook_run", "", "", 0, 10)
			if err != nil || len(rows) != 1 || rows[0].ActorUserID != resumer {
				t.Fatalf("decision audit actor = %+v, %v; want resumer %s", rows, err, resumer)
			}
		})
	}
}

type actionReviewSpawner struct{ sync.WaitGroup }

func (s *actionReviewSpawner) TryGo(work func(context.Context)) bool {
	s.Add(1)
	go func() {
		defer s.Done()
		work(context.Background())
	}()
	return true
}

func TestCancelConnectorActionDuringHTTPNeverResends(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	received := make(chan struct{})
	release := make(chan struct{})
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			close(received)
		}
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	defer close(release)
	h := newTestHandler(t)
	conn := seedActionConnector(t, h, server.URL, serviceRescanRecipe)
	user := operatorOn(t, h, conn)
	id := decodeRunbook(t, createWithSteps(t, h, user, "cancel-http", connectorActionStep(conn, "rescan", "", "")+`,{"kind":"manual","title":"Verify"}`)).ID
	spawner := &actionReviewSpawner{}
	t.Cleanup(spawner.Wait)
	h.Executor = runbookrun.New(runbookrun.Deps{Store: h.Store, Actions: h.ConnH,
		Grants: runbookrun.StoreGrants{Store: h.Store}, Spawner: spawner})
	authored, err := h.Store.ListRunbookStepsFor(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	run, _, err := h.Executor.Start(context.Background(), id, user, authored)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("action request was not received")
	}
	rr := httptest.NewRecorder()
	h.CancelRun(rr, runRequest(user, id, run.ID, ""))
	assertRunStatus(t, rr, http.StatusNoContent)
	got, steps, err := h.Store.GetRunbookRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "cancelled" || steps[0].State != "unknown" || steps[1].State != "skipped" {
		t.Fatalf("cancel during HTTP = %+v, %+v", got, steps)
	}
	if count, err := runbookrun.Recover(context.Background(), h.Store, nil, nil); err != nil || count != 0 || hits.Load() != 1 {
		t.Fatalf("recovery of cancelled run = %d, %v, sends=%d", count, err, hits.Load())
	}
}
