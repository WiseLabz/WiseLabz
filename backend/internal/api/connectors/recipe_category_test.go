package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
)

func apiRecipe(category string) string {
	return fmt.Sprintf(`version: 1
category: %s
auth: {mode: none}
endpoints:
  - name: items
    path: /items
    method: GET
    items: '@this'
    entity: {kind: item, name: title, external_id: id}
`, category)
}

func recipeRequest(t *testing.T, method, id string, body map[string]any) *http.Request {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, "/api/connectors/"+id, strings.NewReader(string(data)))
	req.SetPathValue("id", id)
	return req.WithContext(auth.ContextWithUser(req.Context(), "recipe-admin", true))
}

func TestRecipeCategoryCreateUpdate(t *testing.T) {
	h := newTestHandler(t)
	for _, tt := range []struct {
		name, category, recipe, want string
		status                       int
	}{
		{"recipe derives", "", apiRecipe("media"), "media", 201},
		{"matching category", "storage", apiRecipe("storage"), "storage", 201},
		{"conflict", "storage", apiRecipe("media"), "", 400},
		{"legacy derives", "", "", "virtualization", 201},
		{"legacy conflicting", "media", "", "", 400},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := map[string]any{"name": tt.name, "type": "custom", "url": "https://api.example", "config": map[string]any{"recipe": tt.recipe}}
			if tt.category != "" {
				body["category"] = tt.category
			}
			rr := httptest.NewRecorder()
			h.Create(rr, recipeRequest(t, "POST", "", body))
			if rr.Code != tt.status {
				t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
			}
			if tt.status == 400 {
				requireCategoryFieldError(t, rr)
				return
			}
			var created struct{ ID, Category string }
			if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
				t.Fatal(err)
			}
			if created.Category != tt.want {
				t.Fatalf("category=%s want=%s", created.Category, tt.want)
			}
			put := func(body map[string]any) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				h.Update(rr, recipeRequest(t, "PUT", created.ID, body))
				return rr
			}
			rr = put(map[string]any{"config": map[string]any{"recipe": apiRecipe("monitoring")}})
			if rr.Code != 200 {
				t.Fatalf("update status %d: %s", rr.Code, rr.Body.String())
			}
			stored, err := h.Store.GetConnector(context.Background(), created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.Category != "monitoring" {
				t.Fatalf("stored category %s", stored.Category)
			}
			rr = put(map[string]any{"category": "storage"})
			requireCategoryFieldError(t, rr)
			rr = put(map[string]any{"config": map[string]any{"recipe": ""}})
			if rr.Code != 200 {
				t.Fatalf("remove recipe status %d: %s", rr.Code, rr.Body.String())
			}
			stored, err = h.Store.GetConnector(context.Background(), created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.Category != "virtualization" {
				t.Fatalf("recipe-less category %s", stored.Category)
			}
		})
	}
}

func TestRecipeAPIReportsEveryLocatedError(t *testing.T) {
	h := newTestHandler(t)
	recipe := strings.ReplaceAll(apiRecipe("media"), "external_id: id", "external_id: ''")
	recipe = strings.ReplaceAll(recipe, "method: GET", "method: DELETE") + "unexpected: true\n"
	rr := httptest.NewRecorder()
	h.Create(rr, recipeRequest(t, "POST", "", map[string]any{"name": "invalid", "type": "custom", "url": "https://api.example", "config": map[string]any{"recipe": recipe}}))
	if rr.Code != 400 {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var got struct{ Details []httputil.FieldError }
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"config.recipe.unexpected", "config.recipe.endpoints[0].method", "config.recipe.endpoints[0].entity.external_id"} {
		found := false
		for _, d := range got.Details {
			if d.Field == want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing %s in %+v", want, got.Details)
		}
	}
}

func TestRecipeAPIConfigReturnsRecipeAndHidesCredentials(t *testing.T) {
	h := newTestHandler(t)
	recipe := strings.ReplaceAll(apiRecipe("media"), "auth: {mode: none}", "auth: {mode: basic}")
	cfg := map[string]any{"recipe": recipe, "auth_username": "private-user", "auth_password": "private-pass"}
	rr := httptest.NewRecorder()
	h.Create(rr, recipeRequest(t, "POST", "", map[string]any{"name": "private", "type": "custom", "url": "https://api.example", "config": cfg}))
	if rr.Code != 201 {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var created struct{ ID string }
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	stored, err := h.Store.GetConnector(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private-user", "private-pass"} {
		if strings.Contains(stored.ConfigData, secret) {
			t.Errorf("stored secret %s", secret)
		}
	}
	rr = httptest.NewRecorder()
	h.Get(rr, recipeRequest(t, "GET", created.ID, nil))
	if rr.Code != 200 {
		t.Fatalf("config status %d: %s", rr.Code, rr.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	config, _ := got["config"].(map[string]any)
	if config["recipe"] != recipe {
		t.Errorf("recipe absent or changed: %s", rr.Body.String())
	}
	for _, secret := range []string{"private-user", "private-pass"} {
		if strings.Contains(rr.Body.String(), secret) {
			t.Errorf("API exposes credential %s", secret)
		}
	}
}
