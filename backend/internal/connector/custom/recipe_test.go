package custom

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const validRecipe = `version: 1
category: media
auth:
  mode: none
endpoints:
  - name: items
    path: /api/items
    method: GET
    items: items
    entity:
      kind: media_item
      name: title
      external_id: id
`

func TestParseRecipeValid(t *testing.T) {
	recipe, err := ParseRecipe(validRecipe)
	if err != nil {
		t.Fatalf("ParseRecipe(): %v", err)
	}
	if recipe.Category != "media" || len(recipe.Endpoints) != 1 {
		t.Fatalf("recipe = %+v", recipe)
	}
}

func TestParseRecipeValidationLocations(t *testing.T) {
	tests := []struct {
		name   string
		recipe string
		want   string
	}{
		{name: "unknown key", recipe: strings.Replace(validRecipe, "    method: GET", "    methd: GET", 1), want: "endpoints[0].methd: unknown field"},
		{name: "unsupported version", recipe: strings.Replace(validRecipe, "version: 1", "version: 2", 1), want: "version: must be 1"},
		{name: "unknown category", recipe: strings.Replace(validRecipe, "category: media", "category: gaming", 1), want: "category: must be one of"},
		{name: "unknown auth mode", recipe: strings.Replace(validRecipe, "mode: none", "mode: kerberos", 1), want: "auth.mode: must be one of"},
		{name: "unsupported method", recipe: strings.Replace(validRecipe, "method: GET", "method: DELETE", 1), want: "endpoints[0].method: must be GET or POST"},
		{name: "malformed item path", recipe: strings.Replace(validRecipe, "items: items", "items: items[", 1), want: "endpoints[0].items: path expression is malformed"},
		{name: "absolute endpoint URL", recipe: strings.Replace(validRecipe, "/api/items", "https://other.example/items", 1), want: "endpoints[0].path: must be relative to the connector URL"},
		{name: "missing name mapping", recipe: strings.Replace(validRecipe, "      name: title\n", "", 1), want: "endpoints[0].entity.name: mapping is required"},
		{name: "missing identifier mapping", recipe: strings.Replace(validRecipe, "      external_id: id\n", "", 1), want: "endpoints[0].entity.external_id: mapping is required"},
		{name: "pagination unsupported", recipe: strings.Replace(validRecipe, "    items: items", "    items: items\n    pagination: {type: page, param: page, size: 100}", 1), want: "endpoints[0].pagination: pagination is not supported yet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseRecipe(tt.recipe)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ParseRecipe() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestParseRecipeAggregatesValidationErrors(t *testing.T) {
	raw := strings.Replace(validRecipe, "    method: GET", "    method: DELETE", 1)
	raw = strings.Replace(raw, "items: items", "items: items[", 1)
	_, err := ParseRecipe(raw)
	if err == nil {
		t.Fatal("ParseRecipe() error = nil")
	}
	for _, want := range []string{"endpoints[0].method", "endpoints[0].items"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ParseRecipe() error %q does not include %q", err, want)
		}
	}
}

func TestParseRecipeLimits(t *testing.T) {
	t.Run("size", func(t *testing.T) {
		accepted := validRecipe + strings.Repeat(" ", maxRecipeBytes-len(validRecipe))
		if _, err := ParseRecipe(accepted); err != nil {
			t.Fatalf("ParseRecipe(exactly %d bytes): %v", maxRecipeBytes, err)
		}
		raw := accepted + " "
		_, err := ParseRecipe(raw)
		if err == nil || !strings.Contains(err.Error(), "must be at most 65536 bytes") {
			t.Fatalf("ParseRecipe() error = %v", err)
		}
	})
	t.Run("endpoint count", func(t *testing.T) {
		if _, err := ParseRecipe(recipeWithEndpointCount(maxRecipeEndpoints)); err != nil {
			t.Fatalf("ParseRecipe(exactly %d endpoints): %v", maxRecipeEndpoints, err)
		}
		raw := recipeWithEndpointCount(maxRecipeEndpoints + 1)
		_, err := ParseRecipe(raw)
		if err == nil || !strings.Contains(err.Error(), "must contain at most 20 endpoints") {
			t.Fatalf("ParseRecipe() error = %v", err)
		}
	})
}

func recipeWithEndpointCount(count int) string {
	var endpoints strings.Builder
	for i := 0; i < count; i++ {
		fmt.Fprintf(&endpoints, "  - name: endpoint-%02d\n    path: /api\n    method: GET\n    items: items\n    entity: {kind: service, name: name, external_id: id}\n", i)
	}
	return "version: 1\ncategory: other\nauth: {mode: none}\nendpoints:\n" + endpoints.String()
}

func TestParseRecipeRejectsMalformedAndMultipleDocuments(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "malformed YAML", raw: "version: [", want: "invalid YAML"},
		{name: "multiple documents", raw: validRecipe + "---\nversion: 1\n", want: "exactly one YAML document"},
		{name: "zero endpoints", raw: "version: 1\ncategory: other\nauth: {mode: none}\nendpoints: []\n", want: "endpoints: must contain at least one endpoint"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseRecipe(test.raw)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ParseRecipe() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestParseRecipeRejectsDuplicateEndpointNames(t *testing.T) {
	raw := recipeWithEndpointCount(2)
	raw = strings.Replace(raw, "endpoint-01", "endpoint-00", 1)
	_, err := ParseRecipe(raw)
	if err == nil || !strings.Contains(err.Error(), "endpoints[1].name: must be unique") {
		t.Fatalf("ParseRecipe() error = %v, want duplicate endpoint location", err)
	}
}

func TestParseRecipeUnknownKeysHaveDottedLocations(t *testing.T) {
	raw := strings.Replace(validRecipe, "  mode: none", "  mode: none\n  extra: true", 1)
	raw = strings.Replace(raw, "    method: GET", "    method: GET\n    pagination: {type: page, param: page, size: 10, extra: true}", 1)
	raw = strings.Replace(raw, "    entity:\n", "    dependencies:\n      - kind: host\n        const: host-1\n        extra: true\n    entity:\n", 1)
	_, err := ParseRecipe(raw)
	if err == nil {
		t.Fatal("ParseRecipe() error = nil")
	}
	for _, want := range []string{"auth.extra: unknown field", "endpoints[0].pagination.extra: unknown field", "endpoints[0].dependencies[0].extra: unknown field"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ParseRecipe() error %q does not include %q", err, want)
		}
	}
}

func TestParseRecipeAttributeValidation(t *testing.T) {
	tests := []struct {
		name string
		attr string
		want string
	}{
		{name: "source count", attr: "enabled: {path: enabled, const: true}", want: "attributes.enabled: must define exactly one"},
		{name: "source missing", attr: "enabled: {type: bool}", want: "attributes.enabled: must define exactly one"},
		{name: "unsupported type", attr: "enabled: {const: true, type: object}", want: "attributes.enabled.type: must be string"},
		{name: "malformed path", attr: "enabled: {path: 'enabled[', type: bool}", want: "attributes.enabled.path: path expression is malformed"},
		{name: "malformed template", attr: "enabled: {template: '{enabled', type: string}", want: "attributes.enabled.template: template has an unclosed placeholder"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := strings.Replace(validRecipe, "      external_id: id", "      external_id: id\n      attributes:\n        "+test.attr, 1)
			_, err := ParseRecipe(raw)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ParseRecipe() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestParseRecipeGETBodyAndStaticPOSTBody(t *testing.T) {
	t.Run("GET null body is omitted", func(t *testing.T) {
		raw := strings.Replace(validRecipe, "    method: GET", "    method: GET\n    body: null", 1)
		if _, err := ParseRecipe(raw); err != nil {
			t.Fatalf("ParseRecipe(GET body null): %v", err)
		}
	})
	t.Run("GET non-null body rejected", func(t *testing.T) {
		raw := strings.Replace(validRecipe, "    method: GET", "    method: GET\n    body: {active: true}", 1)
		_, err := ParseRecipe(raw)
		if err == nil || !strings.Contains(err.Error(), "endpoints[0].body: is only allowed with POST") {
			t.Fatalf("ParseRecipe(GET body): %v", err)
		}
	})
	t.Run("POST static JSON body accepted", func(t *testing.T) {
		raw := strings.Replace(validRecipe, "    method: GET", "    method: POST\n    body: {active: true, tags: [a, b]}", 1)
		if _, err := ParseRecipe(raw); err != nil {
			t.Fatalf("ParseRecipe(POST body): %v", err)
		}
	})
	t.Run("POST timestamp rejected", func(t *testing.T) {
		raw := strings.Replace(validRecipe, "    method: GET", "    method: POST\n    body: {created: 2026-10-06}", 1)
		_, err := ParseRecipe(raw)
		if err == nil || !strings.Contains(err.Error(), "endpoints[0].body: must be a static JSON value") {
			t.Fatalf("ParseRecipe(POST timestamp): %v", err)
		}
	})
	t.Run("cyclic alias rejected", func(t *testing.T) {
		raw := strings.Replace(validRecipe, "    method: GET", "    method: POST\n    body: &loop {self: *loop}", 1)
		_, err := ParseRecipe(raw)
		if err == nil || !strings.Contains(err.Error(), "cyclic YAML aliases are not supported") {
			t.Fatalf("ParseRecipe(cyclic alias): %v", err)
		}
	})
}

func TestParseRecipePaginationBlocksAreValidatedAndRejected(t *testing.T) {
	blocks := []string{
		"{type: page, param: page, size: 50}",
		"{type: offset, param: offset, size_param: limit, size: 50}",
		"{type: cursor, param: cursor, cursor_path: next}",
		"{type: next_link, next_path: next}",
	}
	for _, block := range blocks {
		raw := strings.Replace(validRecipe, "    items: items", "    items: items\n    pagination: "+block, 1)
		_, err := ParseRecipe(raw)
		if err == nil || !strings.Contains(err.Error(), "endpoints[0].pagination: pagination is not supported yet") {
			t.Errorf("ParseRecipe(pagination %s) error = %v", block, err)
		}
	}
}

func TestParseRecipeAcceptsGJSONPathForms(t *testing.T) {
	for _, path := range []string{`items.#(active==true)`, `items.#.name`, `{items.0.name,items.1.name}`, `meta.a\.b`, `meta.\.leading`, `items.#(name==\"one\").id`, `@this`} {
		if err := validateGJSONPath(path); err != nil {
			t.Errorf("validateGJSONPath(%q): %v", path, err)
		}
	}
	if err := validateEndpointPath("/items?format=json"); err != nil {
		t.Errorf("validateEndpointPath(relative query): %v", err)
	}
}

func TestParseRecipeRejectsUnknownNestedKeysTogether(t *testing.T) {
	raw := strings.Replace(validRecipe, "    method: GET", "    method: PATCH\n    extra: true", 1)
	raw = strings.Replace(raw, "      external_id: id", "      external_id: id\n      attributes:\n        enabled: {path: isEnabled, typ: bool}", 1)
	_, err := ParseRecipe(raw)
	if err == nil {
		t.Fatal("ParseRecipe() error = nil")
	}
	for _, want := range []string{"endpoints[0].method", "endpoints[0].extra", "endpoints[0].entity.attributes.enabled.typ"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ParseRecipe() error %q does not include %q", err, want)
		}
	}
}

func TestCategoryForConfig(t *testing.T) {
	category, err := CategoryForConfig(map[string]any{})
	if err != nil || category != "virtualization" {
		t.Fatalf("CategoryForConfig(empty) = %q, %v", category, err)
	}
	category, err = CategoryForConfig(map[string]any{"recipe": validRecipe})
	if err != nil || category != "media" {
		t.Fatalf("CategoryForConfig(recipe) = %q, %v", category, err)
	}
	_, err = CategoryForConfig(map[string]any{"recipe": "not: [yaml"})
	var fieldErr *connector.ConfigValidationError
	if !errors.As(err, &fieldErr) || fieldErr.Field != "recipe" {
		t.Fatalf("CategoryForConfig(invalid) = %v, want recipe field error", err)
	}
}

func TestValidateCustomConfigRecipeCredentials(t *testing.T) {
	tests := []struct {
		name        string
		mode        string
		credentials map[string]any
		wantError   string
	}{
		{name: "none", mode: "none"},
		{name: "header missing token", mode: "header", wantError: "auth_token"},
		{name: "header token", mode: "header", credentials: map[string]any{"auth_token": "token"}},
		{name: "query missing token", mode: "query", wantError: "auth_token"},
		{name: "query token", mode: "query", credentials: map[string]any{"auth_token": "token"}},
		{name: "basic reports both missing fields", mode: "basic", wantError: "auth_username"},
		{name: "basic username only", mode: "basic", credentials: map[string]any{"auth_username": "operator"}, wantError: "auth_password"},
		{name: "basic credentials", mode: "basic", credentials: map[string]any{"auth_username": "operator", "auth_password": "secret"}},
		{name: "basic username colon", mode: "basic", credentials: map[string]any{"auth_username": "bad:name", "auth_password": "secret"}, wantError: "auth_username"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := strings.Replace(validRecipe, "mode: none", "mode: "+test.mode, 1)
			if test.mode == "header" {
				raw = strings.Replace(raw, "mode: header", "mode: header\n  name: Authorization\n  prefix: 'Bearer '", 1)
			}
			if test.mode == "query" {
				raw = strings.Replace(raw, "mode: query", "mode: query\n  name: api_key", 1)
			}
			config := map[string]any{"recipe": raw}
			for key, value := range test.credentials {
				config[key] = value
			}
			err := validateCustomConfig(config)
			if test.wantError == "" && err != nil {
				t.Fatalf("validateCustomConfig() = %v, want nil", err)
			}
			if test.wantError != "" && (err == nil || !strings.Contains(err.Error(), test.wantError)) {
				t.Fatalf("validateCustomConfig() = %v, want containing %q", err, test.wantError)
			}
		})
	}
}

func TestValidateCustomConfigURLCredentialsRejected(t *testing.T) {
	err := validateCustomConfig(map[string]any{
		"url":    "https://operator:secret@api.example.test",
		"recipe": validRecipe,
	})
	if err == nil || !strings.Contains(err.Error(), "must not contain credentials") {
		t.Fatalf("validateCustomConfig() = %v, want URL credential rejection", err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error exposes URL password: %v", err)
	}
}

func TestCustomRecipeAndCredentialsUseExpectedFieldKinds(t *testing.T) {
	schema, err := connector.GetTypeSchema(typeName)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		typ string
		max int
	}{
		"recipe":        {typ: "textarea", max: maxRecipeBytes},
		"auth_token":    {typ: "password"},
		"auth_username": {typ: "password"},
		"auth_password": {typ: "password"},
	}
	for _, field := range schema.Fields {
		if expectation, ok := want[field.Key]; ok {
			if field.Type != expectation.typ || (expectation.max != 0 && field.MaxLength != expectation.max) {
				t.Errorf("field %q = type %q max %d; want type %q max %d", field.Key, field.Type, field.MaxLength, expectation.typ, expectation.max)
			}
			delete(want, field.Key)
		}
	}
	if len(want) != 0 {
		t.Errorf("missing schema fields %v", want)
	}
}

func TestCategoryForConfigAggregatesLocatedErrors(t *testing.T) {
	raw := strings.Replace(validRecipe, "    method: GET", "    method: DELETE", 1)
	raw = strings.Replace(raw, "    items: items", "    items: items[", 1)
	_, err := CategoryForConfig(map[string]any{"recipe": raw})
	if err == nil {
		t.Fatal("CategoryForConfig() error = nil")
	}
	for _, want := range []string{`field "recipe.endpoints[0].method"`, `field "recipe.endpoints[0].items"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("CategoryForConfig() error %q does not include %q", err, want)
		}
	}
}

func TestRecipeValidationErrorsRedactEmbeddedURLQueries(t *testing.T) {
	raw := strings.Replace(validRecipe, "    method: GET", "    method: GET\n    'https://api.example.test/items?token=query-secret': true", 1)
	_, err := ParseRecipe(raw)
	if err == nil || strings.Contains(err.Error(), "query-secret") {
		t.Fatalf("ParseRecipe() error = %v; want unknown key with query redacted", err)
	}
	var fieldErr *connector.ConfigValidationError
	if !errors.As(recipeConfigErrors(err), &fieldErr) {
		t.Fatalf("recipeConfigErrors() = %v; want located config error", recipeConfigErrors(err))
	}
	if strings.Contains(fieldErr.Field, "query-secret") || strings.Contains(fieldErr.Message, "query-secret") {
		t.Fatalf("structured error exposes URL query token: %#v", fieldErr)
	}
}

func postRecipeWithBody(body string) string {
	return strings.Replace(validRecipe, "    method: GET", "    method: POST\n    body: "+body, 1)
}

func TestParseRecipeBoundsYAMLAliasExpansion(t *testing.T) {
	const want = "YAML alias expansion exceeds the validation limit"
	long := strings.Repeat("x", 20*1024)
	t.Run("aliased values", func(t *testing.T) {
		raw := postRecipeWithBody(`[&a "` + long + `"` + strings.Repeat(", *a", 3000) + "]")
		if len(raw) >= maxRecipeBytes {
			t.Fatalf("test recipe is %d bytes, want under %d", len(raw), maxRecipeBytes)
		}
		_, err := ParseRecipe(raw)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("ParseRecipe() error = %v, want %q", err, want)
		}
	})
	t.Run("aliased mapping keys", func(t *testing.T) {
		raw := postRecipeWithBody(`[{? &k "` + long + `" : 1}` + strings.Repeat(", {*k : 1}", 3000) + "]")
		if len(raw) >= maxRecipeBytes {
			t.Fatalf("test recipe is %d bytes, want under %d", len(raw), maxRecipeBytes)
		}
		_, err := ParseRecipe(raw)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("ParseRecipe() error = %v, want %q", err, want)
		}
	})
	t.Run("aliased attribute values", func(t *testing.T) {
		raw := strings.Replace(validRecipe, "      external_id: id", "      external_id: id\n      attributes:\n        a: {const: &a \""+long+"\"}\n"+
			strings.Repeat("        b: {map: {k: *a}}\n", 1), 1)
		raw = strings.Replace(raw, "        b: {map: {k: *a}}\n", "        b: {map: {"+strings.TrimPrefix(strings.Repeat(", k: *a", 3000), ", ")+"}}\n", 1)
		_, err := ParseRecipe(raw)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("ParseRecipe() error = %v, want %q", err, want)
		}
	})
	t.Run("small alias still works", func(t *testing.T) {
		recipe, err := ParseRecipe(postRecipeWithBody("{a: &x one, b: *x}"))
		if err != nil {
			t.Fatalf("ParseRecipe(): %v", err)
		}
		baseURL, err := url.Parse("https://api.example.test")
		if err != nil {
			t.Fatal(err)
		}
		request, err := buildRecipeRequest(context.Background(), baseURL, recipe, recipe.Endpoints[0], nil)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != `{"a":"one","b":"one"}` {
			t.Fatalf("request body = %s", body)
		}
	})
}

func TestParseRecipeRejectsMergeAndDuplicateKeys(t *testing.T) {
	tests := []struct {
		name   string
		recipe string
		want   string
	}{
		{
			name:   "merge key in endpoint",
			recipe: "version: 1\ncategory: media\nauth: {mode: none}\nendpoints:\n  - &base {name: base, path: /api, method: GET, items: items, entity: {kind: k, name: n, external_id: id}}\n  - <<: *base\n    name: other\n",
			want:   "endpoints[1]: mapping keys must be strings",
		},
		{
			name:   "merge key in entity",
			recipe: strings.Replace(validRecipe, "    entity:\n      kind: media_item\n", "    entity:\n      <<: {kind: media_item}\n", 1),
			want:   "endpoints[0].entity: mapping keys must be strings",
		},
		{
			name:   "duplicate endpoint key",
			recipe: strings.Replace(validRecipe, "    method: GET", "    method: GET\n    method: POST", 1),
			want:   "endpoints[0]",
		},
		{
			name:   "duplicate query key",
			recipe: strings.Replace(validRecipe, "    method: GET", "    method: GET\n    query:\n      page: '1'\n      page: '2'", 1),
			want:   "endpoints[0].query",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseRecipe(test.recipe)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ParseRecipe() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestValidateGJSONPathModifiersAndLength(t *testing.T) {
	for _, path := range []string{`@this`, `items.@reverse`, `items.@flatten`, `items.@keys`, `meta.\@type`, `items|@values`, `{a:@this}`, `[hostname,fqdn]`, `[a,b,c,d,e,f,g,h]`, `items.#[active==true]`, `items.#(email=="a\u0040b.example")`, `items.#(name=="x").id`} {
		if err := validateGJSONPath(path); err != nil {
			t.Errorf("validateGJSONPath(%q): %v", path, err)
		}
	}
	for _, path := range []string{`@tostr`, `items|@tostr|@tostr`, `@pretty:{"indent":"xxxx"}`, `{a:@ugly}`, `items.#(@valid==true)`, `@fromstr`, strings.Repeat("a", maxGJSONPathBytes+1),
		`x'|@tostr|@tostr|y'`, `x"|@tostr|y"`, `items.#(email=="a@b.example")`, `@flatten:{"deep":true}`,
		`@this|[@this,@this]|[@this,@this]`, `{a:{b,c}}`, `[a,b].[c,d]`, `[a,a,a,a,a,a,a,a,a]`} {
		if err := validateGJSONPath(path); err == nil {
			t.Errorf("validateGJSONPath(%.40q) error = nil, want rejection", path)
		}
	}
	if err := validateGJSONPath(strings.Repeat("a", maxGJSONPathBytes)); err != nil {
		t.Errorf("validateGJSONPath(%d bytes): %v", maxGJSONPathBytes, err)
	}
	err := validateGJSONPath(strings.Repeat("a", maxGJSONPathBytes+1))
	if err == nil || err.Error() != "path expression is too long" {
		t.Errorf("long path error = %v", err)
	}
	err = validateGJSONPath(`items|@tostr`)
	if err == nil || err.Error() != `path expression uses unsupported modifier "@tostr"` {
		t.Errorf("modifier error = %v", err)
	}
	_, err = ParseRecipe(strings.Replace(validRecipe, "items: items", "items: '@tostr'", 1))
	if err == nil || !strings.Contains(err.Error(), `endpoints[0].items: path expression uses unsupported modifier "@tostr"`) {
		t.Fatalf("ParseRecipe() error = %v", err)
	}
}

func TestValidateGJSONPathMultipathErrors(t *testing.T) {
	tests := map[string]string{
		`@flatten:{"deep":true}`:        "path expression uses unsupported modifier arguments",
		`[a,b].[c,d]`:                   "path expression may contain at most one multipath group",
		`[a,a,a,a,a,a,a,a,a]`:           "path expression may select at most 8 values",
		`items.#(email=="a@b.example")`: `path expression uses unsupported modifier "@b"`,
		`x'|@tostr|@tostr|y'`:           `path expression uses unsupported modifier "@tostr"`,
	}
	for path, want := range tests {
		if err := validateGJSONPath(path); err == nil || err.Error() != want {
			t.Errorf("validateGJSONPath(%q) error = %v, want %q", path, err, want)
		}
	}
}

func TestMapEndpointEvaluatesMultipathAliasesAndUnicodeEscapedQuery(t *testing.T) {
	raw := strings.Replace(validRecipe, "      external_id: id\n", "      external_id: id\n      aliases: '[hostname,fqdn]'\n", 1)
	recipe, err := ParseRecipe(raw)
	if err != nil {
		t.Fatalf("ParseRecipe(): %v", err)
	}
	body := []byte(`{"items":[{"id":"1","name":"n","title":"n","hostname":"h","fqdn":"h.example"}]}`)
	result, err := mapEndpoint(recipe, recipe.Endpoints[0], body, make(map[string]struct{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entities) != 1 || !reflect.DeepEqual(result.Entities[0].Aliases, []string{"h", "h.example"}) {
		t.Fatalf("result = %+v, want aliases [h h.example]", result)
	}
	got := jsonPathValue([]byte(`{"items":[{"email":"a@b.example","id":"x"}]}`), `items.#(email=="a\u0040b.example").id`)
	if got.String() != "x" {
		t.Errorf("unicode-escaped query value = %q, want x", got.String())
	}
}

func TestParseRecipeValidatesStaticAttributeValues(t *testing.T) {
	tests := []struct {
		name string
		attr string
		want string
	}{
		{name: "nested const", attr: "x: {const: {a: 1}}", want: "attributes.x.const: "},
		{name: "timestamp const", attr: "x: {const: 2026-10-06}", want: "attributes.x.const: "},
		{name: "infinite const", attr: "x: {const: .inf}", want: "attributes.x.const: value is not a finite number"},
		{name: "non-numeric const", attr: "x: {const: abc, type: number}", want: "attributes.x.const: value is not a number"},
		{name: "bad map value", attr: "x: {path: state, type: number, map: {up: 1, down: abc}}", want: "attributes.x.map.down: value is not a number"},
		{name: "bad default", attr: "x: {path: state, type: bool, map: {up: true}, default: maybe}", want: "attributes.x.default: value is not a boolean"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := strings.Replace(validRecipe, "      external_id: id", "      external_id: id\n      attributes:\n        "+test.attr, 1)
			_, err := ParseRecipe(raw)
			if err == nil || !strings.Contains(err.Error(), "endpoints[0].entity."+test.want) {
				t.Fatalf("ParseRecipe() error = %v, want containing %q", err, test.want)
			}
		})
	}
	valid := `
        a: {const: 3, type: number}
        b: {path: state, type: bool, map: {up: true, down: false}, default: false}
        c: {const: [x, y], type: list}
        d: {const: text}`
	raw := strings.Replace(validRecipe, "      external_id: id", "      external_id: id\n      attributes:"+valid, 1)
	if _, err := ParseRecipe(raw); err != nil {
		t.Fatalf("ParseRecipe(valid static values): %v", err)
	}
}
