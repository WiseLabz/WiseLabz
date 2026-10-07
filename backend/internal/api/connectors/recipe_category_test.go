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
	"github.com/WiseLabz/wiselabz/internal/connector"
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
	return recipeRequestAs(t, method, id, body, "recipe-admin", true)
}

// recipeRequestAs is recipeRequest for an arbitrary caller; non-admin callers
// exercise the instance-admin gates.
func recipeRequestAs(t *testing.T, method, id string, body map[string]any, userID string, admin bool) *http.Request {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, "/api/connectors/"+id, strings.NewReader(string(data)))
	req.SetPathValue("id", id)
	return req.WithContext(auth.ContextWithUser(req.Context(), userID, admin))
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

// putAs sends a PUT /api/connectors/{id} as an admin or a plain operator.
func putAs(t *testing.T, h *Handler, id string, body map[string]any, admin bool) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	h.Update(rr, recipeRequestAs(t, "PUT", id, body, "recipe-user", admin))
	return rr
}

func requireStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rr.Code != want {
		t.Fatalf("status %d, want %d: %s", rr.Code, want, rr.Body.String())
	}
}

func storedCategory(t *testing.T, h *Handler, id string) string {
	t.Helper()
	stored, err := h.Store.GetConnector(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return stored.Category
}

// apiRecipeOf reads the recipe back through the GET handler.
func apiRecipeOf(t *testing.T, h *Handler, id string) string {
	t.Helper()
	rr := httptest.NewRecorder()
	h.Get(rr, recipeRequest(t, "GET", id, nil))
	requireStatus(t, rr, 200)
	var got struct {
		Config map[string]any `json:"config"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	recipe, _ := got.Config["recipe"].(string)
	return recipe
}

func TestLegacyCustomConnectorKeepsCategoryOnConfigUpdate(t *testing.T) {
	h := newTestHandler(t)
	c := seedCoverageConnector(t, h, "legacy", "virtualization")
	if err := h.Store.UpdateConnector(context.Background(), c.ID, map[string]any{"category": "monitoring"}); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name   string
		body   map[string]any
		admin  bool
		status int
		want   string
	}{
		{"operator config edit", map[string]any{"config": map[string]any{"method": "GET"}}, false, 200, "monitoring"},
		{"admin config edit", map[string]any{"config": map[string]any{"method": "GET"}}, true, 200, "monitoring"},
		{"admin resends stored category", map[string]any{"category": "monitoring"}, true, 200, "monitoring"},
		{"admin other category", map[string]any{"category": "storage"}, true, 400, "monitoring"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rr := putAs(t, h, c.ID, tt.body, tt.admin)
			requireStatus(t, rr, tt.status)
			if tt.status == 400 {
				requireCategoryFieldError(t, rr)
			}
			if got := storedCategory(t, h, c.ID); got != tt.want {
				t.Fatalf("stored category %s, want %s", got, tt.want)
			}
		})
	}
}

func TestRecipeConnectorUpdatesKeepRecipeAndCategory(t *testing.T) {
	h := newTestHandler(t)
	recipe := apiRecipe("media")
	rr := httptest.NewRecorder()
	h.Create(rr, recipeRequest(t, "POST", "", map[string]any{"name": "media", "type": "custom", "url": "https://api.example", "config": map[string]any{"recipe": recipe}}))
	requireStatus(t, rr, 201)
	var created struct{ ID string }
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	requireUnchanged := func(t *testing.T) {
		t.Helper()
		if got := apiRecipeOf(t, h, created.ID); got != recipe {
			t.Fatalf("recipe changed to %q", got)
		}
		if got := storedCategory(t, h, created.ID); got != "media" {
			t.Fatalf("category changed to %s", got)
		}
	}

	t.Run("operator cannot change the recipe category", func(t *testing.T) {
		requireStatus(t, putAs(t, h, created.ID, map[string]any{"config": map[string]any{"recipe": apiRecipe("monitoring")}}, false), 403)
		requireUnchanged(t)
	})
	t.Run("operator save without the recipe keeps it", func(t *testing.T) {
		requireStatus(t, putAs(t, h, created.ID, map[string]any{"config": map[string]any{}}, false), 200)
		requireUnchanged(t)
	})
	t.Run("admin rename without the recipe keeps it", func(t *testing.T) {
		requireStatus(t, putAs(t, h, created.ID, map[string]any{"name": "renamed", "config": map[string]any{}}, true), 200)
		requireUnchanged(t)
	})
	t.Run("admin clears the recipe explicitly", func(t *testing.T) {
		requireStatus(t, putAs(t, h, created.ID, map[string]any{"config": map[string]any{"recipe": ""}}, true), 200)
		if got := apiRecipeOf(t, h, created.ID); got != "" {
			t.Fatalf("recipe = %q, want cleared", got)
		}
		if got := storedCategory(t, h, created.ID); got != "virtualization" {
			t.Fatalf("category %s, want virtualization", got)
		}
	})
}

// A type that derives its category from plain config and declares no
// EndpointConfigKeys: only the explicit category gate stops an operator.
func TestDerivedCategoryChangeRequiresInstanceAdmin(t *testing.T) {
	const typ = "category_gate_test"
	connector.Register(connector.TypeSchema{
		Type:     typ,
		Name:     "Category gate",
		Category: "other",
		Fields:   []connector.SchemaField{{Key: "tier", Label: "Tier", Type: "text"}},
		CategoryForConfig: func(config map[string]any) (string, error) {
			if tier, _ := config["tier"].(string); tier != "" {
				return tier, nil
			}
			return "other", nil
		},
	}, func(map[string]any) (connector.Connector, error) { return nil, fmt.Errorf("not used") })
	h := newTestHandler(t)
	rr := httptest.NewRecorder()
	h.Create(rr, recipeRequest(t, "POST", "", map[string]any{"name": "gate", "type": typ, "url": "https://api.example", "config": map[string]any{"tier": "media"}}))
	requireStatus(t, rr, 201)
	var created struct{ ID string }
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	requireStatus(t, putAs(t, h, created.ID, map[string]any{"config": map[string]any{"tier": "storage"}}, false), 403)
	if got := storedCategory(t, h, created.ID); got != "media" {
		t.Fatalf("operator changed category to %s", got)
	}
	requireStatus(t, putAs(t, h, created.ID, map[string]any{"config": map[string]any{"tier": "media"}}, false), 200)
	requireStatus(t, putAs(t, h, created.ID, map[string]any{"config": map[string]any{"tier": "storage"}}, true), 200)
	if got := storedCategory(t, h, created.ID); got != "storage" {
		t.Fatalf("admin category %s, want storage", got)
	}
}
