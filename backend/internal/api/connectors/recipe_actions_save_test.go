package connectors

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
)

func saveActionRecipe(path string) string {
	return apiRecipe("other") + "actions:\n  rescan: {method: POST, path: " + path + "}\n"
}

func TestRecipeActionsSaveElevation(t *testing.T) {
	h := newTestHandler(t)
	createBody := map[string]any{"name": "Actions", "type": "custom", "url": "https://api.example", "config": map[string]any{"recipe": saveActionRecipe("/rescan")}}
	create := func(token string) *httptest.ResponseRecorder {
		req := recipeRequest(t, http.MethodPost, "", createBody)
		if token != "" {
			req.Header.Set("X-Elevation-Token", token)
		}
		rr := httptest.NewRecorder()
		h.Create(rr, req)
		return rr
	}
	rr := create("")
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "elevation_required") {
		t.Fatalf("create without elevation: %d %s", rr.Code, rr.Body.String())
	}
	rows, _, err := h.Store.ListConnectors(context.Background(), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatal("rejected create wrote a connector")
	}
	token, err := h.JWT.IssueElevation("recipe-admin", "connector.recipeActions")
	if err != nil {
		t.Fatal(err)
	}
	rr = create(token.Token)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	var created struct {
		ID           string
		Capabilities connector.CapabilityDescriptor
		Actions      []connector.ActionDescriptor
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if len(created.Actions) != 1 || created.Capabilities.Restart {
		t.Fatalf("operations = %+v %+v", created.Capabilities, created.Actions)
	}
	update := func(recipe, target string, elevate bool, admin bool) *httptest.ResponseRecorder {
		req := recipeRequestAs(t, http.MethodPut, created.ID, map[string]any{"config": map[string]any{"recipe": recipe}}, "recipe-admin", admin)
		if elevate {
			token, err := h.JWT.IssueElevationBound("recipe-admin", "connector.recipeActions", auth.ElevationBinding{Target: target})
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("X-Elevation-Token", token.Token)
		}
		rr := httptest.NewRecorder()
		h.Update(rr, req)
		return rr
	}
	for _, tc := range []struct {
		name, recipe, target string
		elevate, admin       bool
		status               int
	}{
		{"missing", saveActionRecipe("/new"), "", false, true, 400},
		{"wrong target", saveActionRecipe("/new"), "other", true, true, 401},
		{"not admin", saveActionRecipe("/new"), created.ID, true, false, 403},
		{"comments", saveActionRecipe("/rescan") + "# unchanged\n", "", false, true, 200},
		{"mapping", strings.Replace(saveActionRecipe("/rescan"), "name: title", "name: other_title", 1), "", false, true, 200},
		{"matching", saveActionRecipe("/new"), created.ID, true, true, 200},
		{"remove all", apiRecipe("other"), "", false, true, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, err := h.Store.GetConnector(context.Background(), created.ID)
			if err != nil {
				t.Fatal(err)
			}
			rr := update(tc.recipe, tc.target, tc.elevate, tc.admin)
			if rr.Code != tc.status {
				t.Fatalf("update: %d %s", rr.Code, rr.Body.String())
			}
			after, err := h.Store.GetConnector(context.Background(), created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if tc.status != 200 && before.ConfigData != after.ConfigData {
				t.Fatal("rejected update changed recipe")
			}
		})
	}
	audits, _, err := h.Store.ListAuditRecords(context.Background(), "connector.recipe_actions_changed", "connector", "", "", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(audits) != 3 {
		t.Fatalf("action audits = %d, want create/change/remove", len(audits))
	}
	h.JWT.SetSettingsSource(func() (auth.RuntimeSettings, bool) { return auth.RuntimeSettings{StepUpForDestructive: false}, true })
	rr = update(saveActionRecipe("/disabled"), "", false, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("disabled step-up: %d %s", rr.Code, rr.Body.String())
	}
}
