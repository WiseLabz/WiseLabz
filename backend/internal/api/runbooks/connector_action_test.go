package runbooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/runbookrun"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// actionRecipeEntities declares two entity-scoped actions on kind item. It
// declares no service action, so it has no service lifecycle verb either.
const actionRecipeEntities = `version: 1
category: other
auth: {mode: query, name: api_token}
endpoints:
  - name: items
    path: /items
    method: GET
    items: '@this'
    entity:
      kind: item
      name: title
      external_id: id
      attributes:
        node: {path: node}
      actions:
        restart:
          method: PATCH
          path: /items/{external_id}/restart
          label: Restart item
          description: Restart this item
          downtime_seconds: 7
        drain:
          method: POST
          path: /items/{external_id}/drain
`

// serviceRescanRecipe adds a service-scoped action rescan to the entity recipe.
const serviceRescanRecipe = actionRecipeEntities + `actions:
  rescan:
    method: POST
    path: /rescan
    query: {source: library}
    headers: {X-Mode: safe}
    body: {scope: all}
    label: Rescan library
    description: Refresh the library index
    downtime_seconds: 0
`

// serviceRestartRecipe declares a service restart, which makes the lifecycle
// restart verb supported on the connector.
const serviceRestartRecipe = actionRecipeEntities + `actions:
  restart:
    method: POST
    path: /service-restart
`

// goSpawner runs executor work on its own goroutine, so a resume test can
// wait for the run to reach its next durable state.
type goSpawner struct{}

func (goSpawner) TryGo(work func(context.Context)) bool {
	go work(context.Background())
	return true
}

// countingServer answers every request with 204 and counts them, so a test can
// assert how many requests reached the service.
func countingServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	// The connectors' guarded dialer refuses loopback; a test server lives there.
	connector.AllowLoopbackForTest(t)
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	return server, &hits
}

// seedActionConnector creates a custom connector whose config carries recipe.
func seedActionConnector(t *testing.T, h *Handler, targetURL, recipe string) string {
	t.Helper()
	configData, err := store.MarshalConnectorConfig("custom", map[string]any{
		"recipe":     recipe,
		"auth_token": "query-secret",
	}, h.ConnH.Config.Encryption.Key)
	if err != nil {
		t.Fatal(err)
	}
	record := &store.ConnectorRecord{Name: "Action test", Type: "custom", Category: "other", URL: targetURL, ConfigData: configData}
	if err := h.Store.CreateConnector(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	return record.ID
}

// seedActionSnapshot stores a snapshot with one item entity, db|1 on node-1.
func seedActionSnapshot(t *testing.T, h *Handler, connectorID string) {
	t.Helper()
	data := `{"serviceName":"library","entities":[{"kind":"item","name":"Database item","externalId":"db|1","attributes":{"node":"node-1"}}],"dependencies":[]}`
	if err := h.Store.CreateSnapshot(context.Background(), &store.SnapshotRecord{ConnectorID: connectorID, Data: data, FetchedAt: "2026-10-08T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
}

// connectorActionStep is one connector_action step as authoring JSON. extra is
// appended as more members of the object, for example a verb or a timeout.
func connectorActionStep(connectorID, action, entityRef, extra string) string {
	return `{"kind":"connector_action","title":"Run ` + action + `","connectorId":"` + connectorID + `","action":"` + action + `","entityRef":"` + entityRef + `"` + extra + `}`
}

func TestConnectorActionAuthoring(t *testing.T) {
	h := newTestHandler(t)
	custom := seedActionConnector(t, h, "https://example.com", serviceRescanRecipe)
	restartable := seedActionConnector(t, h, "https://example.com", serviceRestartRecipe)
	docker := seedDockerConnector(t, h)
	user := operatorOn(t, h, custom)

	cases := []struct {
		name   string
		steps  string
		status int
		fields []string
	}{
		{"declared service action", connectorActionStep(custom, "rescan", "", ""), http.StatusCreated, nil},
		{"declared entity action", connectorActionStep(custom, "drain", "db|1", ""), http.StatusCreated, nil},
		{"undeclared action", connectorActionStep(custom, "reboot", "", ""), http.StatusBadRequest, []string{"steps[0].action"}},
		{"service action given an entity", connectorActionStep(custom, "rescan", "db|1", ""), http.StatusBadRequest, []string{"steps[0].action"}},
		{"entity action without an entity", connectorActionStep(custom, "drain", "", ""), http.StatusBadRequest, []string{"steps[0].action"}},
		{"lifecycle verb as action", connectorActionStep(custom, "restart", "", ""), http.StatusBadRequest, []string{"steps[0].action"}},
		{"missing action", connectorActionStep(custom, "", "", ""), http.StatusBadRequest, []string{"steps[0].action"}},
		{"non custom connector", connectorActionStep(docker, "rescan", "", ""), http.StatusBadRequest, []string{"steps[0].action"}},
		{"no connector", connectorActionStep("", "rescan", "", ""), http.StatusBadRequest, []string{"steps[0].connectorId"}},
		{"verb on action step", connectorActionStep(custom, "rescan", "", `,"verb":"restart"`), http.StatusBadRequest, []string{"steps[0].verb"}},
		{"timeout on action step", connectorActionStep(custom, "rescan", "", `,"timeoutSeconds":300`), http.StatusBadRequest, []string{"steps[0].timeoutSeconds"}},
		{"action on lifecycle step", `{"kind":"lifecycle","title":"Restart","connectorId":"` + restartable + `","verb":"restart","action":"rescan"}`, http.StatusBadRequest, []string{"steps[0].action"}},
		{"lifecycle verb declared only on entities, no entity", `{"kind":"lifecycle","title":"Restart","connectorId":"` + custom + `","verb":"restart"}`, http.StatusBadRequest, []string{"steps[0].verb"}},
		{"lifecycle verb declared on an entity, with an entity", `{"kind":"lifecycle","title":"Restart","connectorId":"` + custom + `","verb":"restart","entityRef":"db|1"}`, http.StatusCreated, nil},
		{"lifecycle verb not declared at all", `{"kind":"lifecycle","title":"Stop","connectorId":"` + custom + `","verb":"stop","entityRef":"db|1"}`, http.StatusBadRequest, []string{"steps[0].verb"}},
		{"lifecycle verb the recipe declares", `{"kind":"lifecycle","title":"Restart","connectorId":"` + restartable + `","verb":"restart"}`, http.StatusCreated, nil},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := createWithSteps(t, h, user, fmt.Sprintf("authoring-%d", i), tc.steps)
			assertRunStatus(t, rr, tc.status)
			if tc.status == http.StatusBadRequest {
				if got := fieldErrorFields(t, rr); !reflect.DeepEqual(got, tc.fields) {
					t.Fatalf("field errors = %v, want %v; body=%s", got, tc.fields, rr.Body.String())
				}
			}
		})
	}
}

func TestConnectorActionStepStoredAndReturned(t *testing.T) {
	h := newTestHandler(t)
	conn := seedActionConnector(t, h, "https://example.com", serviceRescanRecipe)
	user := operatorOn(t, h, conn)

	rr := createWithSteps(t, h, user, "stored", connectorActionStep(conn, "drain", "db|1", ""))
	assertRunStatus(t, rr, http.StatusCreated)
	created := decodeRunbook(t, rr)
	got := getRunbook(t, h, user, created.ID).Steps[0]
	for _, step := range []stepResponse{created.Steps[0], got} {
		if step.Kind != kindConnectorAction || step.ConnectorID != conn || step.Action != "drain" || step.EntityRef != "db|1" || step.TimeoutSeconds != 0 || step.Verb != "" {
			t.Fatalf("step = %+v", step)
		}
	}
}

func TestLegacyStepWithoutKindReportsLifecycle(t *testing.T) {
	h := newTestHandler(t)
	conn := seedDockerConnector(t, h)
	user := operatorOn(t, h, conn)
	rb, _, err := h.Store.CreateRunbookWithSteps(context.Background(), &store.RunbookRecord{Title: "Legacy", TargetType: "change_type", TargetValue: "legacy"}, []*store.RunbookStepRecord{
		{Title: "Restart legacy", ConnectorID: conn, Verb: "restart"},
	})
	if err != nil {
		t.Fatal(err)
	}
	step := getRunbook(t, h, user, rb.ID).Steps[0]
	if step.Kind != kindLifecycle || step.Verb != "restart" || step.ConnectorID != conn || step.Action != "" {
		t.Fatalf("legacy step = %+v", step)
	}
}

func TestConnectorActionStepRedactedWithoutViewerGrant(t *testing.T) {
	h := newTestHandler(t)
	conn := seedActionConnector(t, h, "https://example.com", serviceRescanRecipe)
	owner := operatorOn(t, h, conn)
	rr := createWithSteps(t, h, owner, "redacted", connectorActionStep(conn, "rescan", "", ""))
	assertRunStatus(t, rr, http.StatusCreated)
	id := decodeRunbook(t, rr).ID

	stranger := apitest.NewUser(t, h.Store, "viewer")
	step := getRunbook(t, h, stranger, id).Steps[0]
	if step.Title != "Restricted step" || step.Action != "" || step.Kind != "" || step.ConnectorID != "" || step.ExecuteBlockedReason != "no_viewer_grant" {
		t.Fatalf("redacted step leaked: %+v", step)
	}
}

func TestExecuteStepRejectsConnectorAction(t *testing.T) {
	h := newTestHandler(t)
	server, hits := countingServer(t)
	conn := seedActionConnector(t, h, server.URL, serviceRescanRecipe)
	user := operatorOn(t, h, conn)
	created := decodeRunbook(t, createWithSteps(t, h, user, "single", connectorActionStep(conn, "rescan", "", "")))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.SetPathValue("id", created.ID)
	req.SetPathValue("stepId", created.Steps[0].ID)
	req = req.WithContext(auth.ContextWithUser(req.Context(), user, false))
	rr := httptest.NewRecorder()
	h.ExecuteStep(rr, req)
	assertRunStatus(t, rr, http.StatusBadRequest)
	if !strings.Contains(rr.Body.String(), "unsupported_step_kind") {
		t.Fatalf("body = %s", rr.Body.String())
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("single-step execute sent %d requests", n)
	}
}

// previewStart runs StartRun with dryRun=true for userID.
func previewStart(t *testing.T, h *Handler, userID, runbookID string) runPreviewResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/?dryRun=true", nil)
	req.SetPathValue("id", runbookID)
	req = req.WithContext(auth.ContextWithUser(req.Context(), userID, false))
	rr := httptest.NewRecorder()
	h.StartRun(rr, req)
	assertRunStatus(t, rr, http.StatusOK)
	var resp runPreviewResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestConnectorActionRunPreview(t *testing.T) {
	h := newTestHandler(t)
	server, hits := countingServer(t)
	conn := seedActionConnector(t, h, server.URL, serviceRescanRecipe)
	seedActionSnapshot(t, h, conn)
	user := operatorOn(t, h, conn)
	id := decodeRunbook(t, createWithSteps(t, h, user, "preview", connectorActionStep(conn, "rescan", "", "")+","+connectorActionStep(conn, "drain", "db|1", ""))).ID

	resp := previewStart(t, h, user, id)
	if !resp.CanStart || len(resp.Steps) != 2 {
		t.Fatalf("preview = %+v", resp)
	}
	service, entity := resp.Steps[0], resp.Steps[1]
	if service.Action != "rescan" || !service.CanExecute || service.Preview == nil || service.Preview.Request == nil {
		t.Fatalf("service step = %+v", service)
	}
	if service.Preview.Request.Method != "POST" || service.Preview.Label != "Rescan library" || service.Preview.Description != "Refresh the library index" || !service.Preview.UserDefined {
		t.Fatalf("service preview = %+v", service.Preview)
	}
	if entity.Action != "drain" || entity.Preview == nil || entity.Preview.Request == nil || entity.Preview.Request.Method != "POST" || !strings.HasSuffix(entity.Preview.Request.URL, "/drain") {
		t.Fatalf("entity step = %+v", entity)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("preview sent %d requests", n)
	}

	// An entity that is not in the snapshot cannot be previewed, so the run
	// cannot start from the preview.
	missing := decodeRunbook(t, createWithSteps(t, h, user, "preview-missing", connectorActionStep(conn, "drain", "missing|1", "")))
	resp = previewStart(t, h, user, missing.ID)
	if resp.CanStart || resp.Steps[0].CanExecute || resp.Steps[0].Preview != nil || resp.Steps[0].ExecuteBlockedReason != "preview_unavailable" {
		t.Fatalf("unavailable preview = %+v", resp.Steps[0])
	}

	// Redaction: a viewer sees the action name but no request, and a caller
	// with no grant sees nothing of the step.
	viewer := apitest.NewUser(t, h.Store, "viewer")
	apitest.GrantConnectorRole(t, h.Store, viewer, conn, "viewer")
	viewerStep := previewStart(t, h, viewer, id).Steps[0]
	if viewerStep.Action != "rescan" || viewerStep.CanExecute || viewerStep.Preview != nil || viewerStep.ExecuteBlockedReason != "no_operator_grant" {
		t.Fatalf("viewer step = %+v", viewerStep)
	}
	stranger := apitest.NewUser(t, h.Store, "viewer")
	strangerStep := previewStart(t, h, stranger, id).Steps[0]
	if !strangerStep.Redacted || strangerStep.Action != "" || strangerStep.Preview != nil || strangerStep.ExecuteBlockedReason != "no_viewer_grant" {
		t.Fatalf("redacted step = %+v", strangerStep)
	}
}

func TestStartConnectorActionUnavailable(t *testing.T) {
	h := newTestHandler(t)
	server, _ := countingServer(t)
	conn := seedActionConnector(t, h, server.URL, serviceRescanRecipe)
	user := operatorOn(t, h, conn)
	id := decodeRunbook(t, createWithSteps(t, h, user, "start-unavailable", connectorActionStep(conn, "rescan", "", ""))).ID
	// Without an action service the executor cannot freeze the action's
	// fingerprint, so the run must not start.
	h.Executor = runbookrun.New(runbookrun.Deps{Store: h.Store, Spawner: runSpawner{true}})

	req := httptest.NewRequest(http.MethodPost, "/api/runbooks/"+id+"/run", nil)
	req.SetPathValue("id", id)
	req = req.WithContext(auth.ContextWithUser(req.Context(), user, false))
	elevateRun(t, h, req, id)
	rr := httptest.NewRecorder()
	h.StartRun(rr, req)
	assertRunStatus(t, rr, http.StatusConflict)
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "action_unavailable" {
		t.Fatalf("code = %q; body=%s", body.Code, rr.Body.String())
	}
	if _, total, err := h.Store.ListRunbookRuns(context.Background(), id, 10, 0); err != nil || total != 0 {
		t.Fatalf("runs = %d, err = %v; a refused start stored a run", total, err)
	}
}

// seedUnknownActionRun starts a failed run whose first step is a connector
// action left unknown, with the fingerprint frozen as fingerprint.
func seedUnknownActionRun(t *testing.T, h *Handler, runbookID, user, fingerprint string) (*store.RunbookRunRecord, []*store.RunbookRunStepRecord) {
	t.Helper()
	ctx := context.Background()
	authored, err := h.Store.ListRunbookStepsFor(ctx, runbookID)
	if err != nil {
		t.Fatal(err)
	}
	frozen := runbookrun.FreezeSteps(authored)
	frozen[0].ActionFingerprint = fingerprint
	run, steps, err := h.Store.CreateRunbookRun(ctx, runbookID, user, frozen)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Store.UpdateRunbookRunStep(ctx, run.ID, steps[0].ID, "pending", map[string]any{"state": "running"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.Store.FailRunbookRunStep(ctx, run.ID, steps[0].ID, "unknown", "request written, no status received", "step_unknown"); err != nil {
		t.Fatal(err)
	}
	run, steps, err = h.Store.GetRunbookRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	return run, steps
}

// resumeWithBody calls ResumeRun with an optional JSON body and the elevation
// token that the resume requires.
func resumeWithBody(t *testing.T, h *Handler, userID, runbookID, runID, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.SetPathValue("id", runbookID)
	r.SetPathValue("runId", runID)
	r = r.WithContext(auth.ContextWithUser(r.Context(), userID, false))
	elevateRun(t, h, r, runbookID)
	rr := httptest.NewRecorder()
	h.ResumeRun(rr, r)
	return rr
}

// decisionBody is the resume body an operator sends after loading run: the
// decision with the step it is for and the run's updatedAt as received.
func decisionBody(decision, stepID string, run *store.RunbookRunRecord) string {
	return fmt.Sprintf(`{"decision":%q,"stepId":%q,"updatedAt":%q}`, decision, stepID, run.UpdatedAt)
}

func waitRunState(t *testing.T, h *Handler, runID, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		run, steps, err := h.Store.GetRunbookRun(context.Background(), runID)
		if err != nil {
			t.Fatal(err)
		}
		if run.State == want {
			return
		}
		if time.Now().After(deadline) || run.State == "failed" {
			for _, st := range steps {
				t.Logf("step %d state=%s error=%q", st.Position, st.State, st.Error)
			}
			t.Fatalf("run state = %s (reason %q), want %s", run.State, run.Reason, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// assertStepDecisionAudit checks the one audit row of action for runID, with
// the step and decision in its detail.
func assertStepDecisionAudit(t *testing.T, h *Handler, action, runID, stepID, decision string) {
	t.Helper()
	records, total, err := h.Store.ListAuditRecords(context.Background(), action, "runbook_run", "", "", 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || records[0].TargetID != runID {
		t.Fatalf("%s audit = %+v", action, records)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(records[0].Detail), &detail); err != nil {
		t.Fatal(err)
	}
	if detail["stepId"] != stepID || detail["decision"] != decision || detail["runId"] != runID {
		t.Fatalf("%s detail = %+v", action, detail)
	}
}

// unknownActionFixture is a runbook whose first step is a rescan connector
// action and whose second step is a manual check, on a connector that counts
// the requests it receives.
type unknownActionFixture struct {
	h           *Handler
	runbookID   string
	user        string
	fingerprint string
	hits        *atomic.Int32
}

func newUnknownActionFixture(t *testing.T) unknownActionFixture {
	t.Helper()
	server, hits := countingServer(t)
	return newUnknownActionFixtureOn(t, server, hits)
}

func newUnknownActionFixtureOn(t *testing.T, server *httptest.Server, hits *atomic.Int32) unknownActionFixture {
	t.Helper()
	h := newTestHandler(t)
	conn := seedActionConnector(t, h, server.URL, serviceRescanRecipe)
	user := operatorOn(t, h, conn)
	steps := connectorActionStep(conn, "rescan", "", "") + `,{"kind":"manual","title":"Verify"}`
	id := decodeRunbook(t, createWithSteps(t, h, user, "unknown", steps)).ID
	fingerprint, err := h.ConnH.ActionFingerprint(context.Background(), conn, "rescan", "")
	if err != nil {
		t.Fatal(err)
	}
	return unknownActionFixture{h: h, runbookID: id, user: user, fingerprint: fingerprint, hits: hits}
}

func TestResumeUnknownActionWithoutDecisionIsRejected(t *testing.T) {
	f := newUnknownActionFixture(t)
	f.h.Executor = runbookrun.New(runbookrun.Deps{Store: f.h.Store, Actions: f.h.ConnH, Grants: runbookrun.StoreGrants{Store: f.h.Store}, Spawner: goSpawner{}})
	run, _ := seedUnknownActionRun(t, f.h, f.runbookID, f.user, f.fingerprint)

	rr := resumeWithBody(t, f.h, f.user, f.runbookID, run.ID, "")
	assertRunStatus(t, rr, http.StatusConflict)
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "unknown_step_decision_required" || !strings.Contains(body.Message, "resend or mark_done") {
		t.Fatalf("body = %+v", body)
	}
	assertRunState(t, f.h, run.ID, "failed")
	if n := f.hits.Load(); n != 0 {
		t.Fatalf("rejected resume sent %d requests", n)
	}
	assertNoRunAudit(t, f.h, "runbook.run.step_resent")
	assertNoRunAudit(t, f.h, "runbook.run.step_marked_done")
	assertNoRunAudit(t, f.h, "runbook.run.resume")
}

func TestResumeUnknownActionRejectsUnknownDecision(t *testing.T) {
	f := newUnknownActionFixture(t)
	f.h.Executor = runbookrun.New(runbookrun.Deps{Store: f.h.Store, Actions: f.h.ConnH, Grants: runbookrun.StoreGrants{Store: f.h.Store}, Spawner: goSpawner{}})
	run, _ := seedUnknownActionRun(t, f.h, f.runbookID, f.user, f.fingerprint)

	rr := resumeWithBody(t, f.h, f.user, f.runbookID, run.ID, `{"decision":"later"}`)
	assertRunStatus(t, rr, http.StatusBadRequest)
	if got := fieldErrorFields(t, rr); !reflect.DeepEqual(got, []string{"decision"}) {
		t.Fatalf("field errors = %v; body=%s", got, rr.Body.String())
	}
	assertRunState(t, f.h, run.ID, "failed")
	if n := f.hits.Load(); n != 0 {
		t.Fatalf("rejected resume sent %d requests", n)
	}
}

func TestResumeUnknownActionSendAgain(t *testing.T) {
	f := newUnknownActionFixture(t)
	f.h.Executor = runbookrun.New(runbookrun.Deps{Store: f.h.Store, Actions: f.h.ConnH, Grants: runbookrun.StoreGrants{Store: f.h.Store}, Spawner: goSpawner{}})
	run, steps := seedUnknownActionRun(t, f.h, f.runbookID, f.user, f.fingerprint)

	rr := resumeWithBody(t, f.h, f.user, f.runbookID, run.ID, decisionBody("resend", steps[0].ID, run))
	assertRunStatus(t, rr, http.StatusAccepted)
	// The run sends the action, records success and then pauses on the manual step.
	waitRunState(t, f.h, run.ID, "waiting_manual")
	if n := f.hits.Load(); n != 1 {
		t.Fatalf("send again sent %d requests, want 1", n)
	}
	assertStepDecisionAudit(t, f.h, "runbook.run.step_resent", run.ID, steps[0].ID, "resend")
	assertRunAudit(t, f.h, "runbook.run.resume", run.ID, f.runbookID, f.user)
	assertNoRunAudit(t, f.h, "runbook.run.step_marked_done")
}

func TestResumeUnknownActionMarkDone(t *testing.T) {
	f := newUnknownActionFixture(t)
	f.h.Executor = runbookrun.New(runbookrun.Deps{Store: f.h.Store, Actions: f.h.ConnH, Grants: runbookrun.StoreGrants{Store: f.h.Store}, Spawner: goSpawner{}})
	run, steps := seedUnknownActionRun(t, f.h, f.runbookID, f.user, f.fingerprint)

	rr := resumeWithBody(t, f.h, f.user, f.runbookID, run.ID, decisionBody("mark_done", steps[0].ID, run))
	assertRunStatus(t, rr, http.StatusAccepted)
	// The step is marked succeeded without a request, and the manual step after it runs.
	waitRunState(t, f.h, run.ID, "waiting_manual")
	if n := f.hits.Load(); n != 0 {
		t.Fatalf("mark done sent %d requests", n)
	}
	_, got, err := f.h.Store.GetRunbookRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].State != "succeeded" || got[1].State != "waiting" {
		t.Fatalf("steps after mark done = %s, %s", got[0].State, got[1].State)
	}
	assertStepDecisionAudit(t, f.h, "runbook.run.step_marked_done", run.ID, steps[0].ID, "mark_done")
	assertRunAudit(t, f.h, "runbook.run.resume", run.ID, f.runbookID, f.user)
	assertNoRunAudit(t, f.h, "runbook.run.step_resent")
}

func TestResumeUnknownLifecycleStepNeedsNoDecision(t *testing.T) {
	h := newTestHandler(t)
	conn := seedProxmoxConnector(t, h)
	user := operatorOn(t, h, conn)
	rb, _, err := h.Store.CreateRunbookWithSteps(context.Background(), &store.RunbookRecord{Title: "Lifecycle", TargetType: "change_type", TargetValue: "lifecycle-unknown"}, []*store.RunbookStepRecord{
		{Kind: kindLifecycle, Title: "Restart", ConnectorID: conn, Verb: "restart"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Accept the resumed run without executing it, so the test stays off the network.
	h.Executor = runbookrun.New(runbookrun.Deps{Store: h.Store, Spawner: runSpawner{true}})
	run, _ := seedUnknownActionRun(t, h, rb.ID, user, "")

	rr := resumeWithBody(t, h, user, rb.ID, run.ID, "")
	assertRunStatus(t, rr, http.StatusAccepted)
	var resumed store.RunbookRunRecord
	if err := json.Unmarshal(rr.Body.Bytes(), &resumed); err != nil {
		t.Fatal(err)
	}
	if resumed.State != "running" {
		t.Fatalf("resumed = %+v", resumed)
	}
	assertNoRunAudit(t, h, "runbook.run.step_resent")
	assertNoRunAudit(t, h, "runbook.run.step_marked_done")
}

func TestResumeUnknownActionDecisionNeedsStepAndRevision(t *testing.T) {
	f := newUnknownActionFixture(t)
	f.h.Executor = runbookrun.New(runbookrun.Deps{Store: f.h.Store, Actions: f.h.ConnH, Grants: runbookrun.StoreGrants{Store: f.h.Store}, Spawner: goSpawner{}})
	run, steps := seedUnknownActionRun(t, f.h, f.runbookID, f.user, f.fingerprint)

	for _, tt := range []struct {
		name, body string
		want       []string
	}{
		{"neither", `{"decision":"mark_done"}`, []string{"stepId", "updatedAt"}},
		{"no step", fmt.Sprintf(`{"decision":"resend","updatedAt":%q}`, run.UpdatedAt), []string{"stepId"}},
		{"no revision", fmt.Sprintf(`{"decision":"resend","stepId":%q}`, steps[0].ID), []string{"updatedAt"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rr := resumeWithBody(t, f.h, f.user, f.runbookID, run.ID, tt.body)
			assertRunStatus(t, rr, http.StatusBadRequest)
			if got := fieldErrorFields(t, rr); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("field errors = %v, want %v; body=%s", got, tt.want, rr.Body.String())
			}
		})
	}
	assertRunState(t, f.h, run.ID, "failed")
	if n := f.hits.Load(); n != 0 {
		t.Fatalf("rejected resumes sent %d requests", n)
	}
	assertNoRunAudit(t, f.h, "runbook.run.step_resent")
	assertNoRunAudit(t, f.h, "runbook.run.step_marked_done")
	assertNoRunAudit(t, f.h, "runbook.run.resume")
}

func TestResumeUnknownActionRejectsDecisionMadeOnAnEarlierAttempt(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// The first request loses its response; any later one succeeds.
		if hits.Add(1) == 1 {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	f := newUnknownActionFixtureOn(t, server, &hits)
	f.h.Executor = runbookrun.New(runbookrun.Deps{Store: f.h.Store, Actions: f.h.ConnH, Grants: runbookrun.StoreGrants{Store: f.h.Store}, Spawner: goSpawner{}})
	// A loads the run and sees the step unknown.
	loaded, steps := seedUnknownActionRun(t, f.h, f.runbookID, f.user, f.fingerprint)
	stepID := steps[0].ID
	other := operatorOn(t, f.h, steps[0].ConnectorID)

	// B resends with the same, still fresh, values; the new attempt is unknown again.
	rr := resumeWithBody(t, f.h, other, f.runbookID, loaded.ID, decisionBody("resend", stepID, loaded))
	assertRunStatus(t, rr, http.StatusAccepted)
	waitRunState(t, f.h, loaded.ID, "failed")
	current, after, err := f.h.Store.GetRunbookRun(context.Background(), loaded.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after[0].ID != stepID || after[0].State != "unknown" || current.UpdatedAt == loaded.UpdatedAt || hits.Load() != 1 {
		t.Fatalf("after B: step=%+v run=%+v sends=%d; want the same step unknown again on a newer revision", after[0], current, hits.Load())
	}

	// A decides with what A saw: refused, nothing changes.
	rr = resumeWithBody(t, f.h, f.user, f.runbookID, loaded.ID, decisionBody("mark_done", stepID, loaded))
	assertRunStatus(t, rr, http.StatusConflict)
	var changed struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		State     string `json:"state"`
		UpdatedAt string `json:"updatedAt"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &changed); err != nil {
		t.Fatal(err)
	}
	if changed.Code != "run_changed" || changed.Message == "" || changed.State != "failed" || changed.UpdatedAt != current.UpdatedAt {
		t.Fatalf("body = %+v, want run_changed with state failed and updatedAt %s", changed, current.UpdatedAt)
	}
	again, after, err := f.h.Store.GetRunbookRun(context.Background(), loaded.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.State != "failed" || again.UpdatedAt != current.UpdatedAt || after[0].State != "unknown" || hits.Load() != 1 {
		t.Fatalf("stale decision changed the run: %+v, %+v, sends=%d", again, after[0], hits.Load())
	}
	assertNoRunAudit(t, f.h, "runbook.run.step_marked_done")

	// A retries with the current values and succeeds, with exactly one audit row.
	rr = resumeWithBody(t, f.h, f.user, f.runbookID, loaded.ID, decisionBody("mark_done", stepID, current))
	assertRunStatus(t, rr, http.StatusAccepted)
	waitRunState(t, f.h, loaded.ID, "waiting_manual")
	if hits.Load() != 1 {
		t.Fatalf("mark_done sent %d requests in total, want 1", hits.Load())
	}
	assertStepDecisionAudit(t, f.h, "runbook.run.step_marked_done", loaded.ID, stepID, "mark_done")
	assertStepDecisionAudit(t, f.h, "runbook.run.step_resent", loaded.ID, stepID, "resend")
}
