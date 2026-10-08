package custom

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func recipeWithActions(entityNames, serviceNames []string) string {
	raw := validRecipe
	if len(entityNames) > 0 {
		var block strings.Builder
		block.WriteString("      actions:\n")
		for _, name := range entityNames {
			fmt.Fprintf(&block, "        %s:\n          method: POST\n          path: /api/{external_id}/%s\n", name, name)
		}
		raw = strings.Replace(raw, "      external_id: id\n", "      external_id: id\n"+block.String(), 1)
	}
	if len(serviceNames) > 0 {
		raw += "actions:\n"
		for _, name := range serviceNames {
			raw += fmt.Sprintf("  %s:\n    method: POST\n    path: /api/%s\n", name, name)
		}
	}
	return raw
}

func validActionRecipeForFormat() string {
	raw := recipeWithActions([]string{"rescan"}, []string{"restart"})
	raw = strings.Replace(raw, "      external_id: id\n", "      external_id: id\n      attributes:\n        node: {path: node}\n", 1)
	raw = strings.Replace(raw, "          path: /api/{external_id}/rescan\n", "          path: /api/nodes/{attr.node}/containers/{external_id}/rescan\n          query: {source: \"{attr.node}\"}\n          body: {reason: \"Rescan {attr.node}\"}\n", 1)
	return raw
}

func TestParseRecipeActions(t *testing.T) {
	base := validActionRecipeForFormat()
	cases := []struct {
		name, recipe, want string
	}{
		{"invalid action name", strings.Replace(base, "rescan:", "Re Scan:", 1), "endpoints[0].entity.actions.Re Scan"},
		{"name must start lowercase", strings.Replace(base, "rescan:", "1scan:", 1), "endpoints[0].entity.actions.1scan"},
		{"name max length", strings.Replace(base, "rescan:", strings.Repeat("a", 33)+":", 1), "endpoints[0].entity.actions." + strings.Repeat("a", 33)},
		{"method", strings.Replace(base, "method: POST\n          path: /api/nodes", "method: GET\n          path: /api/nodes", 1), "endpoints[0].entity.actions.rescan.method"},
		{"absolute path", strings.Replace(base, "/api/nodes/{attr.node}", "https://other.example/api/nodes/{attr.node}", 1), "endpoints[0].entity.actions.rescan.path"},
		{"scheme relative path", strings.Replace(base, "/api/nodes/{attr.node}", "//other.example/api/nodes/{attr.node}", 1), "endpoints[0].entity.actions.rescan.path"},
		{"fragment", strings.Replace(base, "/rescan\n", "/rescan#fragment\n", 1), "endpoints[0].entity.actions.rescan.path"},
		{"invalid header name", strings.Replace(base, "          body:", "          headers: {\"Bad Header\": value}\n          body:", 1), "endpoints[0].entity.actions.rescan.headers.Bad Header"},
		{"invalid header value", strings.Replace(base, "          body:", "          headers: {X-Test: \"bad\\nvalue\"}\n          body:", 1), "endpoints[0].entity.actions.rescan.headers.X-Test"},
		{"non JSON body", strings.Replace(base, "\"Rescan {attr.node}\"", "2026-10-08", 1), "endpoints[0].entity.actions.rescan.body"},
		{"unknown action field", strings.Replace(base, "          method: POST", "          methd: POST", 1), "endpoints[0].entity.actions.rescan.methd"},
		{"unknown service action field", strings.Replace(base, "  restart:\n    method: POST", "  restart:\n    methd: POST", 1), "actions.restart.methd"},
		{"params are not supported", strings.Replace(base, "          method: POST", "          method: POST\n          params: {}", 1), "endpoints[0].entity.actions.rescan.params"},
		{"query must be a map", strings.Replace(base, "query: {source: \"{attr.node}\"}", "query: [source]", 1), "endpoints[0].entity.actions.rescan.query"},
		{"unsupported placeholder", strings.Replace(base, "{attr.node}", "{node}", 1), "unsupported placeholder"},
		{"unmapped attribute", strings.Replace(base, "{attr.node}", "{attr.other}", 1), "does not name an attribute mapped by this endpoint"},
		{"unclosed placeholder", strings.Replace(base, "{external_id}", "{external_id", 1), "has an unclosed placeholder"},
		{"unmatched brace", strings.Replace(base, "{external_id}", "external_id}", 1), "has an unmatched closing brace"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseRecipe(tt.recipe)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ParseRecipe() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestParseRecipeActionCountAndEntityKinds(t *testing.T) {
	for _, place := range []string{"entity", "service"} {
		for _, count := range []int{10, 11} {
			t.Run(fmt.Sprintf("%s/%d", place, count), func(t *testing.T) {
				names := make([]string, count)
				for i := range names {
					names[i] = fmt.Sprintf("action%d", i)
				}
				var entityNames, serviceNames []string
				if place == "entity" {
					entityNames = names
				} else {
					serviceNames = names
				}
				_, err := ParseRecipe(recipeWithActions(entityNames, serviceNames))
				if count == 10 && err != nil {
					t.Fatalf("ten actions rejected: %v", err)
				}
				if count == 11 && (err == nil || !strings.Contains(err.Error(), "must contain at most 10 actions")) {
					t.Fatalf("eleven actions error = %v", err)
				}
			})
		}
	}

	raw := recipeWithActions([]string{"rescan"}, nil)
	second := "  - name: other\n    path: /other\n    method: GET\n    items: items\n    entity:\n" +
		"      kind: media_item\n      name: name\n      external_id: id\n      actions:\n        restart:\n" +
		"          method: POST\n          path: /restart\n"
	raw += second
	_, err := ParseRecipe(raw)
	if err == nil || !strings.Contains(err.Error(), "endpoints[1].entity.actions") {
		t.Fatalf("duplicate entity kind error = %v", err)
	}
}

func TestParseRecipeAcceptsEveryActionMethod(t *testing.T) {
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			raw := strings.Replace(validActionRecipeForFormat(), "method: POST\n          path:", "method: "+method+"\n          path:", 1)
			if _, err := ParseRecipe(raw); err != nil {
				t.Fatalf("ParseRecipe() rejected %s: %v", method, err)
			}
		})
	}
}

func TestParseRecipeActionMetadataAndDowntime(t *testing.T) {
	base := validActionRecipeForFormat()
	for _, tt := range []struct{ name, field, value string }{
		{"label limit", "label", strings.Repeat("a", 61)},
		{"description limit", "description", strings.Repeat("a", 301)},
		{"negative downtime", "downtime_seconds", "-1"},
		{"large downtime", "downtime_seconds", "3601"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := strings.Replace(base, "          body:", fmt.Sprintf("          %s: %s\n          body:", tt.field, tt.value), 1)
			_, err := ParseRecipe(raw)
			if err == nil || !strings.Contains(err.Error(), "endpoints[0].entity.actions.rescan."+tt.field) {
				t.Fatalf("ParseRecipe() error = %v", err)
			}
		})
	}
	recipe, err := ParseRecipe(base)
	if err != nil {
		t.Fatal(err)
	}
	if got := ActionDowntime("restart", recipe.Actions["restart"]); got != 30 {
		t.Errorf("restart downtime = %d, want 30", got)
	}
	if got := ActionDowntime("rescan", recipe.Endpoints[0].Entity.Actions["rescan"]); got != 0 {
		t.Errorf("rescan downtime = %d, want 0", got)
	}
}

func TestParseRecipeActionPlaceholders(t *testing.T) {
	base := validActionRecipeForFormat()
	for _, tt := range []struct{ name, action, want string }{
		{"service path", "path: /api/{external_id}", ".path"},
		{"service query", "path: /api/restart\n    query: {id: \"{external_id}\"}", ".query.id"},
		{"service body", "path: /api/restart\n    body: {id: \"{external_id}\"}", ".body"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := strings.Replace(base, "    path: /api/restart", "    "+tt.action, 1)
			_, err := ParseRecipe(raw)
			if err == nil || !strings.Contains(err.Error(), "service actions cannot contain placeholders") ||
				!strings.Contains(err.Error(), "actions.restart"+tt.want) {
				t.Fatalf("ParseRecipe() error = %v", err)
			}
		})
	}
	escaped := strings.Replace(base, "{external_id}/rescan", "{{literal}}/{external_id}/rescan", 1)
	if _, err := ParseRecipe(escaped); err != nil {
		t.Fatalf("escaped braces or valid path/query/body placeholder rejected: %v", err)
	}
}

func TestParseRecipeWithoutActionsIsUnchanged(t *testing.T) {
	recipe, err := ParseRecipe(validRecipe)
	if err != nil {
		t.Fatal(err)
	}
	if recipe.Actions != nil || len(recipe.Endpoints[0].Entity.Actions) != 0 {
		t.Fatalf("actions = %#v / %#v, want no actions", recipe.Actions, recipe.Endpoints[0].Entity.Actions)
	}
}

func TestCanonicalActionsAndDiff(t *testing.T) {
	base := validActionRecipeForFormat()
	original, err := CanonicalActions(map[string]any{"recipe": base})
	if err != nil {
		t.Fatal(err)
	}
	formatted := strings.Replace(base, "    path: /api/items\n    method: GET",
		"    # endpoint comment\n    method: GET\n    path: /api/items", 1)
	formatted = strings.Replace(formatted, "  restart:\n    method: POST\n    path: /api/restart",
		"  restart:\n    # key order differs\n    path: /api/restart\n    method: POST", 1)
	reformatted, err := CanonicalActions(map[string]any{"recipe": formatted})
	if err != nil {
		t.Fatal(err)
	}
	if diff := DiffActions(original, reformatted); !reflect.DeepEqual(diff, ActionDiff{}) {
		t.Fatalf("format-only diff = %#v", diff)
	}
	if len(original) != 2 || original["entity.media_item.rescan"].DowntimeSeconds == nil ||
		*original["entity.media_item.rescan"].DowntimeSeconds != 0 || *original["service.restart"].DowntimeSeconds != 30 {
		t.Fatalf("canonical actions = %#v", original)
	}

	changed := map[string]RecipeAction{
		"entity.media_item.rescan": {Method: "PUT", Path: "/changed"},
		"service.pause":            {Method: "POST", Path: "/pause"},
	}
	got := DiffActions(original, changed)
	want := ActionDiff{Added: []string{"service.pause"}, Changed: []string{"entity.media_item.rescan"}, Removed: []string{"service.restart"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DiffActions() = %#v, want %#v", got, want)
	}

	baseAction := RecipeAction{
		Method: "POST", Path: "/api/run", Query: map[string]string{"mode": "safe"},
		Headers: map[string]string{"X-Mode": "safe"}, Body: map[string]any{"mode": "safe"},
		HasBody: true, Label: "Run", Description: "Runs a task", DowntimeSeconds: intPointer(5),
	}
	changedCases := []struct {
		name   string
		change func(RecipeAction) RecipeAction
	}{
		{"method", func(a RecipeAction) RecipeAction { a.Method = "PUT"; return a }},
		{"path", func(a RecipeAction) RecipeAction { a.Path = "/api/other"; return a }},
		{"query", func(a RecipeAction) RecipeAction { a.Query = map[string]string{"mode": "fast"}; return a }},
		{"headers", func(a RecipeAction) RecipeAction { a.Headers = map[string]string{"X-Mode": "fast"}; return a }},
		{"body", func(a RecipeAction) RecipeAction { a.Body = map[string]any{"mode": "fast"}; return a }},
		{"label", func(a RecipeAction) RecipeAction { a.Label = "Other"; return a }},
		{"description", func(a RecipeAction) RecipeAction { a.Description = "Other task"; return a }},
		{"downtime", func(a RecipeAction) RecipeAction { a.DowntimeSeconds = intPointer(6); return a }},
	}
	for _, tt := range changedCases {
		t.Run("value changed: "+tt.name, func(t *testing.T) {
			got := DiffActions(map[string]RecipeAction{"service.run": baseAction}, map[string]RecipeAction{"service.run": tt.change(baseAction)})
			if !reflect.DeepEqual(got.Changed, []string{"service.run"}) || len(got.Added)+len(got.Removed) != 0 {
				t.Fatalf("DiffActions() = %#v", got)
			}
		})
	}
	if got := DiffActions(
		map[string]RecipeAction{"service.restart": {Method: "POST", Path: "/restart"}},
		map[string]RecipeAction{"service.restart": {Method: "POST", Path: "/restart", DowntimeSeconds: intPointer(30)}},
	); !reflect.DeepEqual(got, ActionDiff{}) {
		t.Fatalf("equivalent default and explicit downtime diff = %#v", got)
	}
}

func TestCanonicalActionsIgnoreMapKeyOrder(t *testing.T) {
	first := recipeWithActions([]string{"rescan", "flush"}, []string{"restart", "stop"})
	second := recipeWithActions([]string{"flush", "rescan"}, []string{"stop", "restart"})
	first = strings.Replace(first, "          path: /api/{external_id}/rescan\n",
		"          path: /api/{external_id}/rescan\n          query: {z: \"1\", a: \"2\"}\n          headers: {X-Z: z, X-A: a}\n          body: {z: z, a: a}\n", 1)
	second = strings.Replace(second, "          path: /api/{external_id}/rescan\n",
		"          path: /api/{external_id}/rescan\n          body: {a: a, z: z}\n          headers: {X-A: a, X-Z: z}\n          query: {a: \"2\", z: \"1\"}\n", 1)
	firstActions, err := CanonicalActions(map[string]any{"recipe": first})
	if err != nil {
		t.Fatal(err)
	}
	secondActions, err := CanonicalActions(map[string]any{"recipe": second})
	if err != nil {
		t.Fatal(err)
	}
	if diff := DiffActions(firstActions, secondActions); !reflect.DeepEqual(diff, ActionDiff{}) {
		t.Fatalf("map-order-only diff = %#v", diff)
	}
}

func TestCanonicalActionsEntityServiceKeysCannotCollide(t *testing.T) {
	raw := recipeWithActions([]string{"restart"}, []string{"restart"})
	raw = strings.Replace(raw, "kind: media_item", "kind: service", 1)
	actions, err := CanonicalActions(map[string]any{"recipe": raw})
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 2 || actions["service.restart"].Method != "POST" || actions["entity.service.restart"].Method != "POST" {
		t.Fatalf("qualified actions = %#v", actions)
	}
}

func intPointer(value int) *int { return &value }

func TestDocumentedRecipeActionsExample(t *testing.T) {
	doc, err := os.ReadFile("../../../../docs/connectors/RECIPE_FORMAT.md")
	if err != nil {
		t.Fatal(err)
	}
	_, after, ok := strings.Cut(string(doc), "<!-- recipe-actions-example-start -->")
	if !ok {
		t.Fatal("missing recipe action example marker")
	}
	fence := strings.Repeat(string(rune(96)), 3)
	_, after, ok = strings.Cut(after, fence+"yaml\n")
	if !ok {
		t.Fatal("missing YAML fence in recipe action example")
	}
	recipeText, _, ok := strings.Cut(after, fence)
	if !ok {
		t.Fatal("missing closing YAML fence in recipe action example")
	}
	recipe, err := ParseRecipe(recipeText)
	if err != nil {
		t.Fatalf("documented action recipe is invalid: %v", err)
	}
	if len(recipe.Endpoints[0].Entity.Actions) != 1 || len(recipe.Actions) != 1 {
		t.Fatalf("documented actions = entity %#v service %#v", recipe.Endpoints[0].Entity.Actions, recipe.Actions)
	}
}
