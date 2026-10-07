package runbooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/api/connectors"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/sync"

	// Register connector implementations (proxmox, etc.), same reasoning as
	// internal/api/connectors' own handlers_test.go: SupportsLifecycleVerb
	// probes the registry, which is empty without this.
	_ "github.com/WiseLabz/wiselabz/internal/connector/all"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	s := apitest.NewStore(t)
	cfg := &config.Config{Encryption: config.EncryptionSettings{Key: "YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWE="}}
	jwtSvc := auth.NewService("test-secret-test-secret-test-secret", time.Hour, time.Hour)
	connH := connectors.NewHandler(s, sync.NewEngine(s, nil, nil, nil, cfg.Encryption.Key), cfg, jwtSvc, nil)
	return NewHandler(s, connH)
}

func TestListMutuallyExclusiveFilters(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/runbooks?changeType=deploy&alertSeverity=high", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestGetNotFound(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/runbooks/missing", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestCreate(t *testing.T) {
	h := newTestHandler(t)

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/runbooks", strings.NewReader(`{`))
		rr := httptest.NewRecorder()
		h.Create(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing required fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/runbooks", strings.NewReader(`{"title":""}`))
		rr := httptest.NewRecorder()
		h.Create(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid target type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/runbooks", strings.NewReader(`{"title":"t","targetType":"bogus","targetValue":"v"}`))
		rr := httptest.NewRecorder()
		h.Create(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("happy path then conflict on duplicate target", func(t *testing.T) {
		body := `{"title":"Deploy runbook","body":"steps","targetType":"change_type","targetValue":"deploy"}`
		req := httptest.NewRequest(http.MethodPost, "/api/runbooks", strings.NewReader(body))
		rr := httptest.NewRecorder()
		h.Create(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
		}

		dupReq := httptest.NewRequest(http.MethodPost, "/api/runbooks", strings.NewReader(body))
		dupRR := httptest.NewRecorder()
		h.Create(dupRR, dupReq)
		if dupRR.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d; body=%s", dupRR.Code, http.StatusConflict, dupRR.Body.String())
		}
	})
}

func TestUpdateAndDelete(t *testing.T) {
	h := newTestHandler(t)

	createReq := httptest.NewRequest(http.MethodPost, "/api/runbooks",
		strings.NewReader(`{"title":"Original","body":"b","targetType":"alert_severity","targetValue":"high"}`))
	createRR := httptest.NewRecorder()
	h.Create(createRR, createReq)
	var created map[string]any
	if err := json.Unmarshal(createRR.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create: %v", err)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("no id in create response: %s", createRR.Body.String())
	}

	t.Run("update invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/runbooks/"+id, strings.NewReader(`{`))
		req.SetPathValue("id", id)
		rr := httptest.NewRecorder()
		h.Update(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("update invalid field type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/runbooks/"+id, strings.NewReader(`{"title":123}`))
		req.SetPathValue("id", id)
		rr := httptest.NewRecorder()
		h.Update(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("update invalid target type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/runbooks/"+id, strings.NewReader(`{"targetType":"bogus"}`))
		req.SetPathValue("id", id)
		rr := httptest.NewRecorder()
		h.Update(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("update not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/runbooks/missing", strings.NewReader(`{"title":"x"}`))
		req.SetPathValue("id", "missing")
		rr := httptest.NewRecorder()
		h.Update(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("update happy path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/runbooks/"+id, strings.NewReader(`{"title":"Updated","docId":null}`))
		req.SetPathValue("id", id)
		rr := httptest.NewRecorder()
		h.Update(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
	})

	t.Run("delete not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/runbooks/missing", nil)
		req.SetPathValue("id", "missing")
		rr := httptest.NewRecorder()
		h.Delete(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("delete happy path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/runbooks/"+id, nil)
		req.SetPathValue("id", id)
		rr := httptest.NewRecorder()
		h.Delete(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusNoContent, rr.Body.String())
		}
	})
}

func seedProxmoxConnector(t *testing.T, h *Handler) string {
	t.Helper()
	c := &store.ConnectorRecord{Name: "Proxmox", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
	if err := h.Store.CreateConnector(context.Background(), c); err != nil {
		t.Fatalf("CreateConnector() error: %v", err)
	}
	return c.ID
}

func TestCreateStepsValidation(t *testing.T) {
	h := newTestHandler(t)
	connID := seedProxmoxConnector(t, h)

	cases := []struct {
		name string
		step string
	}{
		{"missing title", `{"connectorId":"` + connID + `","verb":"restart"}`},
		{"invalid verb", `{"title":"t","connectorId":"` + connID + `","verb":"reboot"}`},
		{"unknown connector", `{"title":"t","connectorId":"missing","verb":"restart"}`},
		{"bad entityRef", `{"title":"t","connectorId":"` + connID + `","verb":"restart","entityRef":"../etc"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := `{"title":"t","targetType":"change_type","targetValue":"x","steps":[` + c.step + `]}`
			req := httptest.NewRequest(http.MethodPost, "/api/runbooks", strings.NewReader(body))
			rr := httptest.NewRecorder()
			h.Create(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
			}
		})
	}

	t.Run("too many steps", func(t *testing.T) {
		steps := "["
		for i := 0; i < 21; i++ {
			if i > 0 {
				steps += ","
			}
			steps += `{"title":"t","connectorId":"` + connID + `","verb":"restart"}`
		}
		steps += "]"
		body := `{"title":"t","targetType":"change_type","targetValue":"y","steps":` + steps + `}`
		req := httptest.NewRequest(http.MethodPost, "/api/runbooks", strings.NewReader(body))
		rr := httptest.NewRecorder()
		h.Create(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("happy path with steps and canExecute false without grant", func(t *testing.T) {
		body := `{"title":"t","targetType":"change_type","targetValue":"z","steps":[{"title":"Restart it","connectorId":"` + connID + `","verb":"restart","entityRef":"100"}]}`
		req := httptest.NewRequest(http.MethodPost, "/api/runbooks", strings.NewReader(body))
		user := apitest.NewUser(t, h.Store, "viewer")
		req = req.WithContext(auth.ContextWithUser(req.Context(), user, false))
		apitest.GrantConnectorRole(t, h.Store, user, connID, "viewer")
		rr := httptest.NewRecorder()
		h.Create(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
		}
		var resp runbookResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(resp.Steps) != 1 {
			t.Fatalf("len(steps) = %d, want 1", len(resp.Steps))
		}
		if resp.Steps[0].CanExecute {
			t.Error("canExecute should be false without an operator grant")
		}
		if resp.Steps[0].ExecuteBlockedReason != "no_operator_grant" {
			t.Errorf("executeBlockedReason = %q, want no_operator_grant", resp.Steps[0].ExecuteBlockedReason)
		}
		if resp.Steps[0].ConnectorName != "Proxmox" {
			t.Errorf("connectorName = %q, want Proxmox", resp.Steps[0].ConnectorName)
		}

		apitest.GrantConnectorRole(t, h.Store, user, connID, "operator")
		getReq := httptest.NewRequest(http.MethodGet, "/api/runbooks/"+resp.ID, nil)
		getReq.SetPathValue("id", resp.ID)
		getReq = getReq.WithContext(auth.ContextWithUser(getReq.Context(), user, false))
		getRR := httptest.NewRecorder()
		h.Get(getRR, getReq)
		var got runbookResponse
		if err := json.Unmarshal(getRR.Body.Bytes(), &got); err != nil {
			t.Fatalf("unmarshal get: %v", err)
		}
		if !got.Steps[0].CanExecute {
			t.Error("canExecute should be true after granting operator")
		}
	})
}

func TestExecuteStepForbiddenWithoutOperatorGrant(t *testing.T) {
	h := newTestHandler(t)
	connID := seedProxmoxConnector(t, h)

	body := `{"title":"t","targetType":"change_type","targetValue":"exec","steps":[{"title":"Restart","connectorId":"` + connID + `","verb":"restart"}]}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/runbooks", strings.NewReader(body))
	createRR := httptest.NewRecorder()
	h.Create(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body=%s", createRR.Code, createRR.Body.String())
	}
	var created runbookResponse
	if err := json.Unmarshal(createRR.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create: %v", err)
	}
	stepID := created.Steps[0].ID

	user := apitest.NewUser(t, h.Store, "viewer") // no connector grant at all
	req := httptest.NewRequest(http.MethodPost, "/api/runbooks/"+created.ID+"/steps/"+stepID+"/execute", nil)
	req.SetPathValue("id", created.ID)
	req.SetPathValue("stepId", stepID)
	req = req.WithContext(auth.ContextWithUser(req.Context(), user, false))
	rr := httptest.NewRecorder()
	h.ExecuteStep(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rr.Code, rr.Body.String())
	}
}

func TestExecuteStepNotFound(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/runbooks/missing/steps/missing/execute", nil)
	req.SetPathValue("id", "missing")
	req.SetPathValue("stepId", "missing")
	rr := httptest.NewRecorder()
	h.ExecuteStep(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rr.Code, rr.Body.String())
	}
}

func TestStepsRedactedWithoutViewerGrant(t *testing.T) {
	h := newTestHandler(t)
	connID := seedProxmoxConnector(t, h)
	body := `{"title":"t","targetType":"change_type","targetValue":"redact","steps":[{"title":"Restart it","connectorId":"` + connID + `","verb":"restart","entityRef":"100"}]}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/runbooks", strings.NewReader(body))
	createRR := httptest.NewRecorder()
	h.Create(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body=%s", createRR.Code, createRR.Body.String())
	}
	var created runbookResponse
	if err := json.Unmarshal(createRR.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	stranger := apitest.NewUser(t, h.Store, "viewer")
	req := httptest.NewRequest(http.MethodGet, "/api/runbooks/"+created.ID, nil)
	req.SetPathValue("id", created.ID)
	req = req.WithContext(auth.ContextWithUser(req.Context(), stranger, false))
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	var got runbookResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	st := got.Steps[0]
	if st.ConnectorID != "" || st.ConnectorName != "" || st.EntityRef != "" || st.Verb != "" || st.Title == "Restart it" || st.CanExecute {
		t.Errorf("step not redacted: %+v", st)
	}
}

// createWithSteps posts a runbook with the given raw steps JSON and returns
// the recorder.
// userID, when set, is the calling user (whose grants shape the response).
func createWithSteps(t *testing.T, h *Handler, userID, targetValue, steps string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"title":"t","targetType":"change_type","targetValue":"` + targetValue + `","steps":[` + steps + `]}`
	req := httptest.NewRequest(http.MethodPost, "/api/runbooks", strings.NewReader(body))
	if userID != "" {
		req = req.WithContext(auth.ContextWithUser(req.Context(), userID, false))
	}
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	return rr
}

// operatorOn creates a user holding an operator grant on connID.
func operatorOn(t *testing.T, h *Handler, connID string) string {
	t.Helper()
	user := apitest.NewUser(t, h.Store, "viewer")
	apitest.GrantConnectorRole(t, h.Store, user, connID, "operator")
	return user
}

func decodeRunbook(t *testing.T, rr *httptest.ResponseRecorder) runbookResponse {
	t.Helper()
	var resp runbookResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, rr.Body.String())
	}
	return resp
}

func fieldErrorFields(t *testing.T, rr *httptest.ResponseRecorder) []string {
	t.Helper()
	var resp struct {
		Details []struct {
			Field string `json:"field"`
		} `json:"details"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error body: %v; body=%s", err, rr.Body.String())
	}
	fields := make([]string, 0, len(resp.Details))
	for _, d := range resp.Details {
		fields = append(fields, d.Field)
	}
	return fields
}

func TestCreateStepKinds(t *testing.T) {
	h := newTestHandler(t)
	connID := seedProxmoxConnector(t, h)

	rr := createWithSteps(t, h, operatorOn(t, h, connID), "kinds", `
		{"title":"Restart","connectorId":"`+connID+`","verb":"restart"},
		{"kind":"lifecycle","title":"Stop","connectorId":"`+connID+`","verb":"stop","entityRef":"100"},
		{"kind":"sync_and_wait","title":"Sync","connectorId":"`+connID+`"},
		{"kind":"wait_until_healthy","title":"Healthy","connectorId":"`+connID+`","timeoutSeconds":600},
		{"kind":"manual","title":"Check the console"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}
	steps := decodeRunbook(t, rr).Steps
	if len(steps) != 5 {
		t.Fatalf("len(steps) = %d, want 5", len(steps))
	}
	want := []struct {
		kind    string
		timeout int
	}{
		{"lifecycle", 0},
		{"lifecycle", 0},
		{"sync_and_wait", 300},
		{"wait_until_healthy", 600},
		{"manual", 0},
	}
	for i, w := range want {
		if steps[i].Kind != w.kind || steps[i].TimeoutSeconds != w.timeout {
			t.Errorf("step %d kind/timeout = %q/%d, want %q/%d", i, steps[i].Kind, steps[i].TimeoutSeconds, w.kind, w.timeout)
		}
	}
	manual := steps[4]
	if manual.Title != "Check the console" || manual.ConnectorID != "" || manual.Verb != "" {
		t.Errorf("manual step = %+v, want visible title and no connector or verb", manual)
	}
	if manual.CanExecute || manual.ExecuteBlockedReason != "not_lifecycle" {
		t.Errorf("manual canExecute/reason = %v/%q, want false/not_lifecycle", manual.CanExecute, manual.ExecuteBlockedReason)
	}
	if steps[2].ConnectorName != "Proxmox" {
		t.Errorf("sync step connectorName = %q, want Proxmox", steps[2].ConnectorName)
	}
}

func TestStepKindsOperatorCanExecute(t *testing.T) {
	h := newTestHandler(t)
	connID := seedProxmoxConnector(t, h)
	rr := createWithSteps(t, h, "", "canexec", `
		{"title":"Restart","connectorId":"`+connID+`","verb":"restart"},
		{"kind":"sync_and_wait","title":"Sync","connectorId":"`+connID+`"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", rr.Code, rr.Body.String())
	}
	created := decodeRunbook(t, rr)

	user := apitest.NewUser(t, h.Store, "viewer")
	apitest.GrantConnectorRole(t, h.Store, user, connID, "operator")
	req := httptest.NewRequest(http.MethodGet, "/api/runbooks/"+created.ID, nil)
	req.SetPathValue("id", created.ID)
	req = req.WithContext(auth.ContextWithUser(req.Context(), user, false))
	getRR := httptest.NewRecorder()
	h.Get(getRR, req)
	steps := decodeRunbook(t, getRR).Steps
	if !steps[0].CanExecute || steps[0].ExecuteBlockedReason != "" {
		t.Errorf("lifecycle step canExecute/reason = %v/%q, want true/empty", steps[0].CanExecute, steps[0].ExecuteBlockedReason)
	}
	if steps[1].CanExecute || steps[1].ExecuteBlockedReason != "not_lifecycle" {
		t.Errorf("sync step canExecute/reason = %v/%q, want false/not_lifecycle", steps[1].CanExecute, steps[1].ExecuteBlockedReason)
	}
}

func TestCreateStepKindsValidation(t *testing.T) {
	h := newTestHandler(t)
	connID := seedProxmoxConnector(t, h)

	cases := []struct {
		name  string
		step  string
		field string
	}{
		{"unknown kind", `{"kind":"reboot","title":"t","connectorId":"` + connID + `","verb":"restart"}`, "steps[0].kind"},
		{"lifecycle without connector", `{"title":"t","verb":"restart"}`, "steps[0].connectorId"},
		{"lifecycle without verb", `{"kind":"lifecycle","title":"t","connectorId":"` + connID + `"}`, "steps[0].verb"},
		{"sync without connector", `{"kind":"sync_and_wait","title":"t"}`, "steps[0].connectorId"},
		{"sync with unknown connector", `{"kind":"sync_and_wait","title":"t","connectorId":"missing"}`, "steps[0].connectorId"},
		{"sync with verb", `{"kind":"sync_and_wait","title":"t","connectorId":"` + connID + `","verb":"restart"}`, "steps[0].verb"},
		{"health with verb", `{"kind":"wait_until_healthy","title":"t","connectorId":"` + connID + `","verb":"stop"}`, "steps[0].verb"},
		{"health with entityRef", `{"kind":"wait_until_healthy","title":"t","connectorId":"` + connID + `","entityRef":"100"}`, "steps[0].entityRef"},
		{"health timeout over 30 minutes", `{"kind":"wait_until_healthy","title":"t","connectorId":"` + connID + `","timeoutSeconds":2700}`, "steps[0].timeoutSeconds"},
		{"sync timeout under 10 seconds", `{"kind":"sync_and_wait","title":"t","connectorId":"` + connID + `","timeoutSeconds":9}`, "steps[0].timeoutSeconds"},
		{"sync timeout zero", `{"kind":"sync_and_wait","title":"t","connectorId":"` + connID + `","timeoutSeconds":0}`, "steps[0].timeoutSeconds"},
		{"lifecycle with timeout", `{"title":"t","connectorId":"` + connID + `","verb":"restart","timeoutSeconds":60}`, "steps[0].timeoutSeconds"},
		{"manual with timeout", `{"kind":"manual","title":"t","timeoutSeconds":60}`, "steps[0].timeoutSeconds"},
		{"manual with connector", `{"kind":"manual","title":"t","connectorId":"` + connID + `"}`, "steps[0].connectorId"},
		{"manual with verb", `{"kind":"manual","title":"t","verb":"restart"}`, "steps[0].verb"},
		{"manual without title", `{"kind":"manual"}`, "steps[0].title"},
		{"manual with entityRef", `{"kind":"manual","title":"t","entityRef":"100"}`, "steps[0].entityRef"},
		{"sync with entityRef", `{"kind":"sync_and_wait","title":"t","connectorId":"` + connID + `","entityRef":"100"}`, "steps[0].entityRef"},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rr := createWithSteps(t, h, "", "val"+string(rune('a'+i)), c.step)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
			}
			found := false
			for _, f := range fieldErrorFields(t, rr) {
				if f == c.field {
					found = true
				}
			}
			if !found {
				t.Errorf("field errors %v do not include %q; body=%s", fieldErrorFields(t, rr), c.field, rr.Body.String())
			}
		})
	}

	t.Run("error is located on the offending step", func(t *testing.T) {
		rr := createWithSteps(t, h, "", "located", `{"title":"ok","connectorId":"`+connID+`","verb":"restart"},
			{"kind":"manual","title":"ok"},
			{"kind":"wait_until_healthy","title":"bad","connectorId":"`+connID+`","timeoutSeconds":2700}`)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
		}
		fields := fieldErrorFields(t, rr)
		if len(fields) != 1 || fields[0] != "steps[2].timeoutSeconds" {
			t.Errorf("field errors = %v, want [steps[2].timeoutSeconds]", fields)
		}
	})

	t.Run("boundary timeouts are accepted", func(t *testing.T) {
		rr := createWithSteps(t, h, operatorOn(t, h, connID), "bounds", `
			{"kind":"sync_and_wait","title":"min","connectorId":"`+connID+`","timeoutSeconds":10},
			{"kind":"wait_until_healthy","title":"max","connectorId":"`+connID+`","timeoutSeconds":1800}`)
		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
		}
		steps := decodeRunbook(t, rr).Steps
		if steps[0].TimeoutSeconds != 10 || steps[1].TimeoutSeconds != 1800 {
			t.Errorf("timeouts = %d/%d, want 10/1800", steps[0].TimeoutSeconds, steps[1].TimeoutSeconds)
		}
	})

	t.Run("manual step without connector", func(t *testing.T) {
		rr := createWithSteps(t, h, "", "manualonly", `{"kind":"manual","title":"Confirm the failover"}`)
		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
		}
	})
}

func TestUpdateStepKinds(t *testing.T) {
	h := newTestHandler(t)
	connID := seedProxmoxConnector(t, h)
	op := operatorOn(t, h, connID)
	rr := createWithSteps(t, h, op, "upd", `{"title":"Restart","connectorId":"`+connID+`","verb":"restart"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body=%s", rr.Code, rr.Body.String())
	}
	created := decodeRunbook(t, rr)

	put := func(steps string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/api/runbooks/"+created.ID, strings.NewReader(`{"steps":[`+steps+`]}`))
		req.SetPathValue("id", created.ID)
		req = req.WithContext(auth.ContextWithUser(req.Context(), op, false))
		rec := httptest.NewRecorder()
		h.Update(rec, req)
		return rec
	}

	bad := put(`{"kind":"sync_and_wait","title":"s","connectorId":"` + connID + `","timeoutSeconds":5}`)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", bad.Code, bad.Body.String())
	}

	// A client that round-trips the response (timeoutSeconds 0 on lifecycle
	// and manual steps) must be accepted.
	ok := put(`{"id":"` + created.Steps[0].ID + `","kind":"lifecycle","title":"Restart","connectorId":"` + connID + `","verb":"restart","timeoutSeconds":0},
		{"kind":"manual","title":"Confirm","timeoutSeconds":0},
		{"kind":"wait_until_healthy","title":"Healthy","connectorId":"` + connID + `"}`)
	if ok.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", ok.Code, ok.Body.String())
	}
	steps := decodeRunbook(t, ok).Steps
	if steps[0].ID != created.Steps[0].ID {
		t.Errorf("lifecycle step id changed across update")
	}
	if steps[2].Kind != "wait_until_healthy" || steps[2].TimeoutSeconds != 300 {
		t.Errorf("health step = %q/%d, want wait_until_healthy/300 default", steps[2].Kind, steps[2].TimeoutSeconds)
	}
}

// TestLegacyStepTimeoutNormalised covers rows written before step kinds
// existed: the column default left 300 on lifecycle steps, new rows hold 0,
// and the API must report the same value for both.
func TestLegacyStepTimeoutNormalised(t *testing.T) {
	h := newTestHandler(t)
	connID := seedProxmoxConnector(t, h)
	op := operatorOn(t, h, connID)
	rr := createWithSteps(t, h, op, "legacy", `{"title":"Restart","connectorId":"`+connID+`","verb":"restart"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body=%s", rr.Code, rr.Body.String())
	}
	created := decodeRunbook(t, rr)
	if created.Steps[0].TimeoutSeconds != 0 {
		t.Fatalf("new lifecycle timeout = %d, want 0", created.Steps[0].TimeoutSeconds)
	}

	if _, err := h.Store.DB().ExecContext(context.Background(), `UPDATE runbook_steps SET timeout_seconds = 300, kind = 'lifecycle' WHERE runbook_id = ?`, created.ID); err != nil {
		t.Fatalf("simulate legacy row: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/runbooks/"+created.ID, nil)
	req.SetPathValue("id", created.ID)
	req = req.WithContext(auth.ContextWithUser(req.Context(), op, false))
	getRR := httptest.NewRecorder()
	h.Get(getRR, req)
	got := decodeRunbook(t, getRR).Steps[0]
	if got.Kind != "lifecycle" || got.TimeoutSeconds != 0 {
		t.Errorf("legacy step kind/timeout = %q/%d, want lifecycle/0", got.Kind, got.TimeoutSeconds)
	}
}

func TestExecuteStepRejectsNonLifecycle(t *testing.T) {
	h := newTestHandler(t)
	connID := seedProxmoxConnector(t, h)
	rr := createWithSteps(t, h, "", "execkinds", `
		{"kind":"sync_and_wait","title":"Sync","connectorId":"`+connID+`"},
		{"kind":"wait_until_healthy","title":"Healthy","connectorId":"`+connID+`"},
		{"kind":"manual","title":"Confirm"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body=%s", rr.Code, rr.Body.String())
	}
	created := decodeRunbook(t, rr)

	// An operator on the connector, so the rejection cannot be a grant failure.
	user := apitest.NewUser(t, h.Store, "viewer")
	apitest.GrantConnectorRole(t, h.Store, user, connID, "operator")
	for _, dryRun := range []string{"", "?dryRun=true"} {
		for _, st := range created.Steps {
			req := httptest.NewRequest(http.MethodPost, "/api/runbooks/"+created.ID+"/steps/"+st.ID+"/execute"+dryRun, nil)
			req.SetPathValue("id", created.ID)
			req.SetPathValue("stepId", st.ID)
			req = req.WithContext(auth.ContextWithUser(req.Context(), user, false))
			rec := httptest.NewRecorder()
			h.ExecuteStep(rec, req)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "unsupported_step_kind") {
				t.Errorf("%s dryRun=%q: status = %d body=%s, want 400 unsupported_step_kind", st.Kind, dryRun, rec.Code, rec.Body.String())
			}
		}
	}
}

// TestExecuteStepRejectsMalformedLifecycle covers a lifecycle step that
// reached the database without a connector or verb (a backup import), which
// must never run as a lifecycle operation.
func TestExecuteStepRejectsMalformedLifecycle(t *testing.T) {
	h := newTestHandler(t)
	connID := seedProxmoxConnector(t, h)
	rr := createWithSteps(t, h, "", "malformed", `
		{"title":"No connector","connectorId":"`+connID+`","verb":"restart"},
		{"title":"No verb","connectorId":"`+connID+`","verb":"restart"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body=%s", rr.Code, rr.Body.String())
	}
	created := decodeRunbook(t, rr)

	db := h.Store.DB()
	if _, err := db.ExecContext(context.Background(), `UPDATE runbook_steps SET connector_id = NULL WHERE id = ?`, created.Steps[0].ID); err != nil {
		t.Fatalf("clear connector: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `UPDATE runbook_steps SET verb = NULL WHERE id = ?`, created.Steps[1].ID); err != nil {
		t.Fatalf("clear verb: %v", err)
	}

	user := apitest.NewUser(t, h.Store, "viewer")
	apitest.GrantConnectorRole(t, h.Store, user, connID, "operator")
	for _, st := range created.Steps {
		req := httptest.NewRequest(http.MethodPost, "/api/runbooks/"+created.ID+"/steps/"+st.ID+"/execute?dryRun=true", nil)
		req.SetPathValue("id", created.ID)
		req.SetPathValue("stepId", st.ID)
		req = req.WithContext(auth.ContextWithUser(req.Context(), user, false))
		rec := httptest.NewRecorder()
		h.ExecuteStep(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_step") {
			t.Errorf("step %q: status = %d body=%s, want 400 invalid_step", st.Title, rec.Code, rec.Body.String())
		}
	}
}

// TestExecuteStepNonLifecycleNeedsGrantFirst checks that a caller without an
// operator grant on the step's connector gets the same 403 whatever the
// step's kind, so the kind of a step they cannot view is not revealed. A
// manual step has no connector and is rejected with a 400 directly.
func TestExecuteStepNonLifecycleNeedsGrantFirst(t *testing.T) {
	h := newTestHandler(t)
	connID := seedProxmoxConnector(t, h)
	rr := createWithSteps(t, h, "", "grantfirst", `
		{"title":"Restart","connectorId":"`+connID+`","verb":"restart"},
		{"kind":"sync_and_wait","title":"Sync","connectorId":"`+connID+`"},
		{"kind":"wait_until_healthy","title":"Healthy","connectorId":"`+connID+`"},
		{"kind":"manual","title":"Confirm"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body=%s", rr.Code, rr.Body.String())
	}
	created := decodeRunbook(t, rr)

	execute := func(user, stepID, query string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/runbooks/"+created.ID+"/steps/"+stepID+"/execute"+query, nil)
		req.SetPathValue("id", created.ID)
		req.SetPathValue("stepId", stepID)
		req = req.WithContext(auth.ContextWithUser(req.Context(), user, false))
		rec := httptest.NewRecorder()
		h.ExecuteStep(rec, req)
		return rec
	}

	stranger := apitest.NewUser(t, h.Store, "viewer")
	viewer := apitest.NewUser(t, h.Store, "viewer")
	apitest.GrantConnectorRole(t, h.Store, viewer, connID, "viewer")
	for _, query := range []string{"", "?dryRun=true"} {
		var bodies []string
		for _, st := range created.Steps[:3] {
			rec := execute(stranger, st.ID, query)
			if rec.Code != http.StatusForbidden {
				t.Errorf("no grant, %s, query=%q: status = %d body=%s, want 403", st.Kind, query, rec.Code, rec.Body.String())
			}
			bodies = append(bodies, rec.Body.String())

			rec = execute(viewer, st.ID, query)
			if rec.Code != http.StatusForbidden {
				t.Errorf("viewer grant, %s, query=%q: status = %d body=%s, want 403", st.Kind, query, rec.Code, rec.Body.String())
			}
		}
		if bodies[0] != bodies[1] || bodies[0] != bodies[2] {
			t.Errorf("query=%q: 403 bodies differ across step kinds: %q", query, bodies)
		}

		rec := execute(stranger, created.Steps[3].ID, query)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "unsupported_step_kind") {
			t.Errorf("manual, query=%q: status = %d body=%s, want 400 unsupported_step_kind", query, rec.Code, rec.Body.String())
		}
	}
}

// TestStepKindsRedactedWithoutViewerGrant checks that a caller who cannot view
// a step's connector learns neither its kind nor its timeout, while a manual
// step (no connector) stays visible.
func TestStepKindsRedactedWithoutViewerGrant(t *testing.T) {
	h := newTestHandler(t)
	connID := seedProxmoxConnector(t, h)
	rr := createWithSteps(t, h, "", "kindsredact", `
		{"kind":"sync_and_wait","title":"Sync","connectorId":"`+connID+`","timeoutSeconds":600},
		{"kind":"wait_until_healthy","title":"Healthy","connectorId":"`+connID+`"},
		{"kind":"manual","title":"Confirm the failover"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body=%s", rr.Code, rr.Body.String())
	}
	created := decodeRunbook(t, rr)

	stranger := apitest.NewUser(t, h.Store, "viewer")
	req := httptest.NewRequest(http.MethodGet, "/api/runbooks/"+created.ID, nil)
	req.SetPathValue("id", created.ID)
	req = req.WithContext(auth.ContextWithUser(req.Context(), stranger, false))
	getRR := httptest.NewRecorder()
	h.Get(getRR, req)
	if getRR.Code != http.StatusOK {
		t.Fatalf("get status = %d; body=%s", getRR.Code, getRR.Body.String())
	}
	steps := decodeRunbook(t, getRR).Steps
	if len(steps) != 3 {
		t.Fatalf("len(steps) = %d, want 3", len(steps))
	}

	for _, st := range steps[:2] {
		if st.Title != "Restricted step" || st.Kind != "" || st.ConnectorID != "" || st.ConnectorName != "" ||
			st.Verb != "" || st.EntityRef != "" || st.TimeoutSeconds != 0 || st.CanExecute ||
			st.ExecuteBlockedReason != "no_viewer_grant" {
			t.Errorf("step not redacted: %+v", st)
		}
	}

	var raw struct {
		Steps []map[string]any `json:"steps"`
	}
	if err := json.Unmarshal(getRR.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	for _, st := range raw.Steps[:2] {
		if _, ok := st["kind"]; ok {
			t.Errorf("redacted step has a kind key: %v", st)
		}
	}

	manual := steps[2]
	if manual.Title != "Confirm the failover" || manual.Kind != "manual" || manual.ExecuteBlockedReason != "not_lifecycle" {
		t.Errorf("manual step = %+v, want visible title, kind manual and reason not_lifecycle", manual)
	}
}

type fakePusher struct {
	typ    string
	fields []connector.ConfigField
}

func (f *fakePusher) Name() string     { return "Fake Pusher" }
func (f *fakePusher) Type() string     { return f.typ }
func (f *fakePusher) Category() string { return "virtualization" }
func (f *fakePusher) Fetch(_ context.Context, _ map[string]any) (*connector.ServiceSnapshot, error) {
	return &connector.ServiceSnapshot{}, nil
}
func (f *fakePusher) Validate(_ context.Context, _ map[string]any) error { return nil }
func (f *fakePusher) WritableFields() []connector.ConfigField            { return f.fields }
func (f *fakePusher) ConfigPush(_ context.Context, _ map[string]any, _, _ string, _ any) error {
	return nil
}

func seedCustomPusherConnector(t *testing.T, h *Handler, fields []connector.ConfigField) string {
	t.Helper()
	typ := fmt.Sprintf("fake_pusher_%d", time.Now().UnixNano())
	fake := &fakePusher{typ: typ, fields: fields}
	connector.Register(connector.TypeSchema{Type: typ, Name: "Fake", Category: "virtualization"},
		func(map[string]any) (connector.Connector, error) {
			return fake, nil
		})
	c := &store.ConnectorRecord{Name: "Fake Pusher", Category: "virtualization", Type: typ, URL: "https://example.com"}
	if err := h.Store.CreateConnector(context.Background(), c); err != nil {
		t.Fatalf("CreateConnector error: %v", err)
	}
	return c.ID
}

func seedDockerConnector(t *testing.T, h *Handler) string {
	t.Helper()
	c := &store.ConnectorRecord{Name: "Docker", Category: "containers_paas", Type: "docker", URL: "https://example.com"}
	if err := h.Store.CreateConnector(context.Background(), c); err != nil {
		t.Fatalf("CreateConnector error: %v", err)
	}
	return c.ID
}

func seedTrueNASConnector(t *testing.T, h *Handler) string {
	t.Helper()
	c := &store.ConnectorRecord{Name: "TrueNAS", Category: "storage", Type: "truenas", URL: "https://example.com"}
	if err := h.Store.CreateConnector(context.Background(), c); err != nil {
		t.Fatalf("CreateConnector error: %v", err)
	}
	return c.ID
}

func TestAuthoringConfigPushHappyPath(t *testing.T) {
	h := newTestHandler(t)
	proxmoxID := seedProxmoxConnector(t, h)
	dockerID := seedDockerConnector(t, h)
	fakeID := seedCustomPusherConnector(t, h, []connector.ConfigField{
		{Key: "enabled", Label: "Enabled", Type: "toggle", EntityScope: false},
		{Key: "hostname", Label: "Hostname", Type: "text", EntityScope: true},
	})

	op := operatorOn(t, h, proxmoxID)
	apitest.GrantConnectorRole(t, h.Store, op, dockerID, "operator")
	apitest.GrantConnectorRole(t, h.Store, op, fakeID, "operator")

	rr := createWithSteps(t, h, op, "push_happy", `
		{"kind":"config_push","title":"Set Memory","connectorId":"`+proxmoxID+`","entityRef":"100","fieldKey":"memory","targetValue":4096},
		{"kind":"config_push","title":"Set Cores","connectorId":"`+proxmoxID+`","entityRef":"100","fieldKey":"cores","targetValue":"4"},
		{"kind":"config_push","title":"Set Restart","connectorId":"`+dockerID+`","entityRef":"web","fieldKey":"restartPolicy","targetValue":"unless-stopped"},
		{"kind":"config_push","title":"Global Toggle","connectorId":"`+fakeID+`","fieldKey":"enabled","targetValue":true},
		{"kind":"config_push","title":"Scoped Text","connectorId":"`+fakeID+`","entityRef":"node-1","fieldKey":"hostname","targetValue":"app-node"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}

	created := decodeRunbook(t, rr)
	if len(created.Steps) != 5 {
		t.Fatalf("len(steps) = %d, want 5", len(created.Steps))
	}

	want := []struct {
		kind        string
		fieldKey    string
		targetValue string
		entityRef   string
	}{
		{"config_push", "memory", "4096", "100"},
		{"config_push", "cores", "4", "100"},
		{"config_push", "restartPolicy", `"unless-stopped"`, "web"},
		{"config_push", "enabled", "true", ""},
		{"config_push", "hostname", `"app-node"`, "node-1"},
	}

	for i, w := range want {
		st := created.Steps[i]
		if st.Kind != w.kind || st.FieldKey != w.fieldKey || st.TargetValue != w.targetValue || st.EntityRef != w.entityRef {
			t.Errorf("step %d = %+v, want kind=%q fieldKey=%q targetValue=%q entityRef=%q", i, st, w.kind, w.fieldKey, w.targetValue, w.entityRef)
		}
		if st.CanExecute || st.ExecuteBlockedReason != "not_lifecycle" {
			t.Errorf("step %d canExecute/reason = %v/%q, want false/not_lifecycle", i, st.CanExecute, st.ExecuteBlockedReason)
		}
		if st.TimeoutSeconds != 0 {
			t.Errorf("step %d timeout = %d, want 0", i, st.TimeoutSeconds)
		}
	}
}

func TestAuthoringConfigPushValidation(t *testing.T) {
	h := newTestHandler(t)
	proxmoxID := seedProxmoxConnector(t, h)
	dockerID := seedDockerConnector(t, h)
	truenasID := seedTrueNASConnector(t, h)
	fakeID := seedCustomPusherConnector(t, h, []connector.ConfigField{
		{Key: "global_toggle", Label: "Global Toggle", Type: "toggle", EntityScope: false},
		{Key: "secret_token", Label: "Secret Token", Type: "password", EntityScope: false},
		{Key: "scoped_toggle", Label: "Scoped Toggle", Type: "toggle", EntityScope: true},
	})

	cases := []struct {
		name  string
		step  string
		field string
	}{
		{"connector does not support config push", `{"kind":"config_push","title":"t","connectorId":"` + truenasID + `","fieldKey":"f","targetValue":"v"}`, "steps[0].connectorId"},
		{"missing fieldKey", `{"kind":"config_push","title":"t","connectorId":"` + proxmoxID + `","entityRef":"100","targetValue":4096}`, "steps[0].fieldKey"},
		{"field not writable", `{"kind":"config_push","title":"t","connectorId":"` + proxmoxID + `","entityRef":"100","fieldKey":"disk_size","targetValue":4096}`, "steps[0].fieldKey"},
		{"entity-scoped field missing entity", `{"kind":"config_push","title":"t","connectorId":"` + proxmoxID + `","fieldKey":"memory","targetValue":4096}`, "steps[0].entityRef"},
		{"global field with entityRef", `{"kind":"config_push","title":"t","connectorId":"` + fakeID + `","fieldKey":"global_toggle","entityRef":"100","targetValue":true}`, "steps[0].entityRef"},
		{"secret field type rejected", `{"kind":"config_push","title":"t","connectorId":"` + fakeID + `","fieldKey":"secret_token","targetValue":"secret123"}`, "steps[0].fieldKey"},
		{"missing targetValue", `{"kind":"config_push","title":"t","connectorId":"` + proxmoxID + `","entityRef":"100","fieldKey":"memory"}`, "steps[0].targetValue"},
		{"invalid JSON targetValue", `{"kind":"config_push","title":"t","connectorId":"` + proxmoxID + `","entityRef":"100","fieldKey":"memory","targetValue":"{bad"}`, "steps[0].targetValue"},
		{"toggle with invalid non-bool", `{"kind":"config_push","title":"t","connectorId":"` + fakeID + `","fieldKey":"global_toggle","targetValue":"invalid_bool"}`, "steps[0].targetValue"},
		{"number with invalid non-number", `{"kind":"config_push","title":"t","connectorId":"` + proxmoxID + `","entityRef":"100","fieldKey":"memory","targetValue":"abc"}`, "steps[0].targetValue"},
		{"docker select with invalid option", `{"kind":"config_push","title":"t","connectorId":"` + dockerID + `","entityRef":"web","fieldKey":"restartPolicy","targetValue":"sometimes"}`, "steps[0].targetValue"},
		{"verb present", `{"kind":"config_push","title":"t","connectorId":"` + proxmoxID + `","entityRef":"100","fieldKey":"memory","targetValue":4096,"verb":"restart"}`, "steps[0].verb"},
		{"timeout present", `{"kind":"config_push","title":"t","connectorId":"` + proxmoxID + `","entityRef":"100","fieldKey":"memory","targetValue":4096,"timeoutSeconds":120}`, "steps[0].timeoutSeconds"},
		{"attribute present", `{"kind":"config_push","title":"t","connectorId":"` + proxmoxID + `","entityRef":"100","fieldKey":"memory","targetValue":4096,"attribute":"status"}`, "steps[0].attribute"},
		{"operator present", `{"kind":"config_push","title":"t","connectorId":"` + proxmoxID + `","entityRef":"100","fieldKey":"memory","targetValue":4096,"operator":"eq"}`, "steps[0].operator"},
		{"expectedValue present", `{"kind":"config_push","title":"t","connectorId":"` + proxmoxID + `","entityRef":"100","fieldKey":"memory","targetValue":4096,"expectedValue":"val"}`, "steps[0].expectedValue"},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rr := createWithSteps(t, h, "", fmt.Sprintf("val_push_%d", i), c.step)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
			}
			found := false
			for _, f := range fieldErrorFields(t, rr) {
				if f == c.field {
					found = true
				}
			}
			if !found {
				t.Errorf("field errors %v do not include %q; body=%s", fieldErrorFields(t, rr), c.field, rr.Body.String())
			}
		})
	}
}

func TestAuthoringWaitForEntityHappyPath(t *testing.T) {
	h := newTestHandler(t)
	connID := seedProxmoxConnector(t, h)
	op := operatorOn(t, h, connID)

	rr := createWithSteps(t, h, op, "wait_happy", `
		{"kind":"wait_for_entity","title":"Wait Running","connectorId":"`+connID+`","entityRef":"100","attribute":"status","operator":"eq","expectedValue":"running"},
		{"kind":"wait_for_entity","title":"Wait Not Stopped","connectorId":"`+connID+`","entityRef":"100","attribute":"status","operator":"neq","expectedValue":"stopped","timeoutSeconds":60},
		{"kind":"wait_for_entity","title":"Wait Contains","connectorId":"`+connID+`","entityRef":"100","attribute":"name","operator":"contains","expectedValue":"prod","timeoutSeconds":1800},
		{"kind":"wait_for_entity","title":"Wait Regex","connectorId":"`+connID+`","entityRef":"100","attribute":"version","operator":"regex","expectedValue":"^v[0-9]+$","timeoutSeconds":120},
		{"kind":"wait_for_entity","title":"Wait GT","connectorId":"`+connID+`","entityRef":"100","attribute":"cores","operator":"gt","expectedValue":2,"timeoutSeconds":120},
		{"kind":"wait_for_entity","title":"Wait LT","connectorId":"`+connID+`","entityRef":"100","attribute":"memory","operator":"lt","expectedValue":"8192","timeoutSeconds":120},
		{"kind":"wait_for_entity","title":"Undeclared Free Text Attribute","connectorId":"`+connID+`","entityRef":"100","attribute":"custom_undeclared_attr","operator":"eq","expectedValue":"ok"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}

	created := decodeRunbook(t, rr)
	if len(created.Steps) != 7 {
		t.Fatalf("len(steps) = %d, want 7", len(created.Steps))
	}

	want := []struct {
		attribute     string
		operator      string
		expectedValue string
		timeout       int
	}{
		{"status", "eq", `"running"`, 300},
		{"status", "neq", `"stopped"`, 60},
		{"name", "contains", `"prod"`, 1800},
		{"version", "regex", `"^v[0-9]+$"`, 120},
		{"cores", "gt", "2", 120},
		{"memory", "lt", "8192", 120},
		{"custom_undeclared_attr", "eq", `"ok"`, 300},
	}

	for i, w := range want {
		st := created.Steps[i]
		if st.Kind != "wait_for_entity" || st.Attribute != w.attribute || st.Operator != w.operator || st.ExpectedValue != w.expectedValue || st.TimeoutSeconds != w.timeout {
			t.Errorf("step %d = %+v, want attr=%q op=%q exp=%q timeout=%d", i, st, w.attribute, w.operator, w.expectedValue, w.timeout)
		}
		if st.CanExecute || st.ExecuteBlockedReason != "not_lifecycle" {
			t.Errorf("step %d canExecute/reason = %v/%q, want false/not_lifecycle", i, st.CanExecute, st.ExecuteBlockedReason)
		}
	}
}

func TestAuthoringWaitForEntityValidation(t *testing.T) {
	h := newTestHandler(t)
	connID := seedProxmoxConnector(t, h)

	cases := []struct {
		name  string
		step  string
		field string
	}{
		{"missing connector", `{"kind":"wait_for_entity","title":"t","entityRef":"100","attribute":"status","operator":"eq","expectedValue":"running"}`, "steps[0].connectorId"},
		{"missing entityRef", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","attribute":"status","operator":"eq","expectedValue":"running"}`, "steps[0].entityRef"},
		{"invalid composite entityRef", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","entityRef":"../invalid","attribute":"status","operator":"eq","expectedValue":"running"}`, "steps[0].entityRef"},
		{"missing attribute", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","entityRef":"100","operator":"eq","expectedValue":"running"}`, "steps[0].attribute"},
		{"missing operator", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","entityRef":"100","attribute":"status","expectedValue":"running"}`, "steps[0].operator"},
		{"unsupported operator startswith", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","entityRef":"100","attribute":"status","operator":"startswith","expectedValue":"run"}`, "steps[0].operator"},
		{"unsupported operator days_left_gt", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","entityRef":"100","attribute":"status","operator":"days_left_gt","expectedValue":"30"}`, "steps[0].operator"},
		{"missing expectedValue", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","entityRef":"100","attribute":"status","operator":"eq"}`, "steps[0].expectedValue"},
		{"regex non-string expectedValue", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","entityRef":"100","attribute":"status","operator":"regex","expectedValue":123}`, "steps[0].expectedValue"},
		{"invalid uncompilable regex", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","entityRef":"100","attribute":"status","operator":"regex","expectedValue":"[a-"}`, "steps[0].expectedValue"},
		{"regex length over 256", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","entityRef":"100","attribute":"status","operator":"regex","expectedValue":"` + strings.Repeat("a", 257) + `"}`, "steps[0].expectedValue"},
		{"gt non-numeric expectedValue", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","entityRef":"100","attribute":"cores","operator":"gt","expectedValue":"two"}`, "steps[0].expectedValue"},
		{"lt non-numeric expectedValue", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","entityRef":"100","attribute":"cores","operator":"lt","expectedValue":true}`, "steps[0].expectedValue"},
		{"timeout below 60 seconds", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","entityRef":"100","attribute":"status","operator":"eq","expectedValue":"running","timeoutSeconds":30}`, "steps[0].timeoutSeconds"},
		{"timeout 0 seconds", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","entityRef":"100","attribute":"status","operator":"eq","expectedValue":"running","timeoutSeconds":0}`, "steps[0].timeoutSeconds"},
		{"timeout above 1800 seconds", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","entityRef":"100","attribute":"status","operator":"eq","expectedValue":"running","timeoutSeconds":2700}`, "steps[0].timeoutSeconds"},
		{"verb present", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","entityRef":"100","attribute":"status","operator":"eq","expectedValue":"running","verb":"restart"}`, "steps[0].verb"},
		{"fieldKey present", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","entityRef":"100","attribute":"status","operator":"eq","expectedValue":"running","fieldKey":"cores"}`, "steps[0].fieldKey"},
		{"targetValue present", `{"kind":"wait_for_entity","title":"t","connectorId":"` + connID + `","entityRef":"100","attribute":"status","operator":"eq","expectedValue":"running","targetValue":"4"}`, "steps[0].targetValue"},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rr := createWithSteps(t, h, "", fmt.Sprintf("val_wait_%d", i), c.step)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
			}
			found := false
			for _, f := range fieldErrorFields(t, rr) {
				if f == c.field {
					found = true
				}
			}
			if !found {
				t.Errorf("field errors %v do not include %q; body=%s", fieldErrorFields(t, rr), c.field, rr.Body.String())
			}
		})
	}
}

func TestNewStepKindsMultipleErrorsCollected(t *testing.T) {
	h := newTestHandler(t)
	connID := seedProxmoxConnector(t, h)

	rr := createWithSteps(t, h, "", "multi_err", `{"kind":"wait_for_entity","title":"Bad Step","connectorId":"`+connID+`","entityRef":"100","attribute":"status","operator":"startswith","expectedValue":"run","timeoutSeconds":30}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	fields := fieldErrorFields(t, rr)
	hasOp := false
	hasTimeout := false
	for _, f := range fields {
		if f == "steps[0].operator" {
			hasOp = true
		}
		if f == "steps[0].timeoutSeconds" {
			hasTimeout = true
		}
	}
	if !hasOp || !hasTimeout {
		t.Errorf("expected both steps[0].operator and steps[0].timeoutSeconds, got %v", fields)
	}
}

func TestExecuteStepRejectsNewKinds(t *testing.T) {
	h := newTestHandler(t)
	connID := seedProxmoxConnector(t, h)

	rr := createWithSteps(t, h, "", "exec_new_kinds", `
		{"kind":"config_push","title":"Push","connectorId":"`+connID+`","entityRef":"100","fieldKey":"memory","targetValue":4096},
		{"kind":"wait_for_entity","title":"Wait","connectorId":"`+connID+`","entityRef":"100","attribute":"status","operator":"eq","expectedValue":"running"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body=%s", rr.Code, rr.Body.String())
	}
	created := decodeRunbook(t, rr)

	user := apitest.NewUser(t, h.Store, "operator")
	apitest.GrantConnectorRole(t, h.Store, user, connID, "operator")

	for _, dryRun := range []string{"", "?dryRun=true"} {
		for _, st := range created.Steps {
			req := httptest.NewRequest(http.MethodPost, "/api/runbooks/"+created.ID+"/steps/"+st.ID+"/execute"+dryRun, nil)
			req.SetPathValue("id", created.ID)
			req.SetPathValue("stepId", st.ID)
			req = req.WithContext(auth.ContextWithUser(req.Context(), user, false))
			rec := httptest.NewRecorder()
			h.ExecuteStep(rec, req)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "unsupported_step_kind") {
				t.Errorf("%s dryRun=%q: status = %d body=%s, want 400 unsupported_step_kind", st.Kind, dryRun, rec.Code, rec.Body.String())
			}
		}
	}

	// Caller without operator grant gets 403 first, hiding the kind.
	stranger := apitest.NewUser(t, h.Store, "stranger")
	for _, dryRun := range []string{"", "?dryRun=true"} {
		for _, st := range created.Steps {
			req := httptest.NewRequest(http.MethodPost, "/api/runbooks/"+created.ID+"/steps/"+st.ID+"/execute"+dryRun, nil)
			req.SetPathValue("id", created.ID)
			req.SetPathValue("stepId", st.ID)
			req = req.WithContext(auth.ContextWithUser(req.Context(), stranger, false))
			rec := httptest.NewRecorder()
			h.ExecuteStep(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Errorf("stranger %s dryRun=%q: status = %d body=%s, want 403", st.Kind, dryRun, rec.Code, rec.Body.String())
			}
		}
	}
}

func TestNewStepKindsRedactionWithoutViewerGrant(t *testing.T) {
	h := newTestHandler(t)
	connID := seedProxmoxConnector(t, h)

	rr := createWithSteps(t, h, "", "new_kinds_redact", `
		{"kind":"config_push","title":"Push","connectorId":"`+connID+`","entityRef":"100","fieldKey":"memory","targetValue":4096},
		{"kind":"wait_for_entity","title":"Wait","connectorId":"`+connID+`","entityRef":"100","attribute":"status","operator":"eq","expectedValue":"running"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body=%s", rr.Code, rr.Body.String())
	}
	created := decodeRunbook(t, rr)

	stranger := apitest.NewUser(t, h.Store, "stranger")
	req := httptest.NewRequest(http.MethodGet, "/api/runbooks/"+created.ID, nil)
	req.SetPathValue("id", created.ID)
	req = req.WithContext(auth.ContextWithUser(req.Context(), stranger, false))
	getRR := httptest.NewRecorder()
	h.Get(getRR, req)
	if getRR.Code != http.StatusOK {
		t.Fatalf("get status = %d; body=%s", getRR.Code, getRR.Body.String())
	}

	steps := decodeRunbook(t, getRR).Steps
	if len(steps) != 2 {
		t.Fatalf("len(steps) = %d, want 2", len(steps))
	}
	for i, st := range steps {
		if st.Title != "Restricted step" || st.Kind != "" || st.ConnectorID != "" || st.ConnectorName != "" ||
			st.Verb != "" || st.EntityRef != "" || st.TimeoutSeconds != 0 || st.CanExecute ||
			st.ExecuteBlockedReason != "no_viewer_grant" || st.FieldKey != "" || st.TargetValue != "" ||
			st.Attribute != "" || st.Operator != "" || st.ExpectedValue != "" {
			t.Errorf("step %d not properly redacted: %+v", i, st)
		}
	}

	var raw struct {
		Steps []map[string]any `json:"steps"`
	}
	if err := json.Unmarshal(getRR.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	for i, st := range raw.Steps {
		for _, key := range []string{"kind", "fieldKey", "targetValue", "attribute", "operator", "expectedValue"} {
			if _, ok := st[key]; ok {
				t.Errorf("step %d contains redacted key %q: %v", i, key, st)
			}
		}
	}
}
