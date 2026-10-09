package connectors

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
)

// recipeActionContractServer answers every request with status and body, so
// the contract tests below can drive the success and failure paths of the
// recipe action routes against a real upstream.
func recipeActionContractServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	return server
}

// serviceRestartRecipe adds a connector-level restart action to the shared
// handler recipe so the lifecycle routes resolve it without an entityRef.
func serviceRestartRecipe() string {
	return strings.Replace(handlerActionRecipe(), "actions:\n  rescan:", "actions:\n  restart: {method: POST, path: /service-restart}\n  rescan:", 1)
}

func decodeContractBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v; body = %s", err, rr.Body.String())
	}
	return got
}

func TestNamedActionResponsesMatchSpec(t *testing.T) {
	t.Run("dry run preview", func(t *testing.T) {
		h := newTestHandler(t)
		server := recipeActionContractServer(t, http.StatusOK, "OK")
		record := seedRecipeActionConnector(t, h, server.URL)
		seedRecipeActionSnapshot(t, h, record.ID)

		path := "/api/connectors/" + record.ID + "/actions/rescan?dryRun=true"
		req := actionHandlerRequest(record.ID, "rescan", path, `{}`, "operator", false)
		rr := httptest.NewRecorder()
		h.Action(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
		}
		got := decodeContractBody(t, rr)
		if got["userDefined"] != true || got["request"] == nil {
			t.Errorf("preview lacks userDefined/request: %v", got)
		}
		apitest.AssertMatchesSpec(t, req, rr.Result())
	})

	t.Run("executed", func(t *testing.T) {
		h := newTestHandler(t)
		server := recipeActionContractServer(t, http.StatusOK, "OK")
		record := seedRecipeActionConnector(t, h, server.URL)
		seedRecipeActionSnapshot(t, h, record.ID)

		path := "/api/connectors/" + record.ID + "/actions/rescan"
		req := actionHandlerRequest(record.ID, "rescan", path, `{}`, "operator", false)
		req.Header.Set("X-Elevation-Token", issueTestElevation(t, h, "operator", "connector.action", record.ID+":rescan"))
		rr := httptest.NewRecorder()
		h.Action(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
		}
		got := decodeContractBody(t, rr)
		if got["status"] != float64(http.StatusOK) || got["excerpt"] != "OK" {
			t.Errorf("result = %v, want status 200 and excerpt OK", got)
		}
		apitest.AssertMatchesSpec(t, req, rr.Result())
	})

	t.Run("upstream failure", func(t *testing.T) {
		h := newTestHandler(t)
		server := recipeActionContractServer(t, http.StatusConflict, "already running")
		record := seedRecipeActionConnector(t, h, server.URL)
		seedRecipeActionSnapshot(t, h, record.ID)

		path := "/api/connectors/" + record.ID + "/actions/rescan"
		req := actionHandlerRequest(record.ID, "rescan", path, `{}`, "operator", false)
		req.Header.Set("X-Elevation-Token", issueTestElevation(t, h, "operator", "connector.action", record.ID+":rescan"))
		rr := httptest.NewRecorder()
		h.Action(rr, req)

		if rr.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502; body = %s", rr.Code, rr.Body.String())
		}
		got := decodeContractBody(t, rr)
		if got["statusCode"] != float64(http.StatusConflict) || got["excerpt"] != "already running" {
			t.Errorf("error body = %v, want statusCode 409 and excerpt", got)
		}
		apitest.AssertMatchesSpec(t, req, rr.Result())
	})

	t.Run("undeclared action", func(t *testing.T) {
		h := newTestHandler(t)
		server := recipeActionContractServer(t, http.StatusOK, "OK")
		record := seedRecipeActionConnector(t, h, server.URL)
		seedRecipeActionSnapshot(t, h, record.ID)

		path := "/api/connectors/" + record.ID + "/actions/missing"
		req := actionHandlerRequest(record.ID, "missing", path, `{}`, "operator", false)
		rr := httptest.NewRecorder()
		h.Action(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", rr.Code, rr.Body.String())
		}
		if got := decodeEnvelope(t, rr); got.Code != "unsupported_operation" {
			t.Errorf("code = %q, want unsupported_operation", got.Code)
		}
		apitest.AssertMatchesSpec(t, req, rr.Result())
	})
}

func TestCustomLifecycleResponsesMatchSpec(t *testing.T) {
	t.Run("restart dry run preview", func(t *testing.T) {
		h := newTestHandler(t)
		server := recipeActionContractServer(t, http.StatusOK, "OK")
		record := seedRecipeActionConnectorWithRecipe(t, h, server.URL, serviceRestartRecipe())
		seedRecipeActionSnapshot(t, h, record.ID)

		path := "/api/connectors/" + record.ID + "/restart?dryRun=true"
		req := actionHandlerRequest(record.ID, "", path, `{}`, "operator", false)
		rr := httptest.NewRecorder()
		h.RestartPreview(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
		}
		got := decodeContractBody(t, rr)
		if got["userDefined"] != true || got["request"] == nil {
			t.Errorf("preview lacks userDefined/request: %v", got)
		}
		apitest.AssertMatchesSpec(t, req, rr.Result())
	})

	t.Run("restart executed", func(t *testing.T) {
		h := newTestHandler(t)
		server := recipeActionContractServer(t, http.StatusOK, "OK")
		record := seedRecipeActionConnectorWithRecipe(t, h, server.URL, serviceRestartRecipe())
		seedRecipeActionSnapshot(t, h, record.ID)

		path := "/api/connectors/" + record.ID + "/restart"
		req := actionHandlerRequest(record.ID, "", path, `{}`, "operator", false)
		req.Header.Set("X-Elevation-Token", issueTestElevation(t, h, "operator", "connector.restart", ""))
		rr := httptest.NewRecorder()
		h.RestartPreview(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
		}
		got := decodeContractBody(t, rr)
		if got["status"] != "restarted" || got["statusCode"] != float64(http.StatusOK) || got["excerpt"] != "OK" {
			t.Errorf("result = %v, want restarted/200/OK", got)
		}
		apitest.AssertMatchesSpec(t, req, rr.Result())
	})

	t.Run("restart upstream failure", func(t *testing.T) {
		h := newTestHandler(t)
		server := recipeActionContractServer(t, http.StatusConflict, "already running")
		record := seedRecipeActionConnectorWithRecipe(t, h, server.URL, serviceRestartRecipe())
		seedRecipeActionSnapshot(t, h, record.ID)

		path := "/api/connectors/" + record.ID + "/restart"
		req := actionHandlerRequest(record.ID, "", path, `{}`, "operator", false)
		req.Header.Set("X-Elevation-Token", issueTestElevation(t, h, "operator", "connector.restart", ""))
		rr := httptest.NewRecorder()
		h.RestartPreview(rr, req)

		if rr.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502; body = %s", rr.Code, rr.Body.String())
		}
		got := decodeContractBody(t, rr)
		if got["statusCode"] != float64(http.StatusConflict) || got["excerpt"] != "already running" {
			t.Errorf("error body = %v, want statusCode 409 and excerpt", got)
		}
		apitest.AssertMatchesSpec(t, req, rr.Result())
	})
}
