package custom

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestMapEndpointMapsEntityAndAttributeSources(t *testing.T) {
	recipe := &Recipe{}
	endpoint := RecipeEndpoint{
		Name:  "series",
		Items: "items",
		Entity: RecipeEntity{
			Kind:       "series",
			Name:       "title",
			ExternalID: "id",
			IP:         "address.ip",
			Hostname:   "address.hostname",
			MAC:        "address.mac",
			Aliases:    "aliases",
			Attributes: map[string]RecipeAttribute{
				"monitored": {Path: "monitored", HasPath: true, Type: "boolean"},
				"status":    {Path: "status", HasPath: true, Type: "string", Map: map[string]any{"1": "enabled"}, Default: "unknown", HasDefault: true},
				"address":   {Template: "{{device}} {address.hostname}:{port}", HasTemplate: true},
				"source":    {Const: "library", HasConst: true},
				"port_text": {Path: "port", HasPath: true, Type: "string"},
				"count":     {Path: "count", HasPath: true, Type: "number"},
				"tags":      {Path: "tags", HasPath: true, Type: "list"},
				"optional":  {Path: "missing.value", HasPath: true},
				"clear":     {Path: "clear_state", HasPath: true, Map: map[string]any{"1": "enabled"}, HasDefault: true},
			},
		},
	}
	body := []byte(`{"items":[{"id":123,"title":"Example","address":{"ip":"10.0.0.5","hostname":"nas","mac":"02:00:00:00:00:01"},"aliases":["nas.local","storage"],"monitored":true,"status":"1","clear_state":"2","port":8096,"count":"12.5","tags":["tv","library"]}]}`)

	result, err := mapEndpoint(recipe, endpoint, body, make(map[string]struct{}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Items != 1 || result.Skipped != 0 {
		t.Fatalf("counts = items %d, skipped %d; want 1, 0", result.Items, result.Skipped)
	}
	want := []connector.SnapshotEntity{{
		Kind:       "series",
		Name:       "Example",
		ExternalID: "123",
		IP:         "10.0.0.5",
		Hostname:   "nas",
		MAC:        "02:00:00:00:00:01",
		Aliases:    []string{"nas.local", "storage"},
		Attributes: map[string]any{
			"monitored": true,
			"status":    "enabled",
			"address":   "{device} nas:8096",
			"source":    "library",
			"port_text": "8096",
			"count":     json.Number("12.5"),
			"tags":      []string{"tv", "library"},
			"clear":     nil,
		},
	}}
	if !reflect.DeepEqual(result.Entities, want) {
		t.Fatalf("entities = %#v, want %#v", result.Entities, want)
	}
}

func TestMapEndpointSkipsMissingNullAndEmptyIdentifiers(t *testing.T) {
	endpoint := RecipeEndpoint{
		Name:  "devices",
		Items: "items",
		Entity: RecipeEntity{
			Kind:       "device",
			Name:       "name",
			ExternalID: "id",
		},
	}
	result, err := mapEndpoint(&Recipe{}, endpoint, []byte(`{"items":[{"id":9007199254740993,"name":"one"},{"id":"","name":"empty"},{"name":"missing"},{"id":null,"name":"null"}]}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Items != 4 || result.Skipped != 3 || len(result.Entities) != 1 {
		t.Fatalf("counts/entities = %d/%d/%d, want 4/3/1", result.Items, result.Skipped, len(result.Entities))
	}
	if got := result.Entities[0].ExternalID; got != "9007199254740993" {
		t.Fatalf("numeric external ID = %q, want exact decimal digits", got)
	}
}

func TestMapEndpointDetectsDuplicateIdentifiersAcrossEndpointsAtomically(t *testing.T) {
	seen := make(map[string]struct{})
	first := RecipeEndpoint{
		Name: "first", Items: "items",
		Entity: RecipeEntity{Kind: "device", Name: "name", ExternalID: "id"},
	}
	if _, err := mapEndpoint(&Recipe{}, first, []byte(`{"items":[{"id":"same","name":"one"}]}`), seen); err != nil {
		t.Fatal(err)
	}
	second := first
	second.Name = "second"
	result, err := mapEndpoint(&Recipe{}, second, []byte(`{"items":[{"id":"same","name":"two"}]}`), seen)
	if err == nil || !strings.Contains(err.Error(), `endpoint "second"`) || !strings.Contains(err.Error(), `"same"`) {
		t.Fatalf("duplicate error = %v, want endpoint and identifier", err)
	}
	if len(result.Entities) != 0 {
		t.Fatalf("duplicate returned partial entities: %#v", result.Entities)
	}
	if len(seen) != 1 {
		t.Fatalf("failed endpoint changed seen set: %#v", seen)
	}

	otherKind := second
	otherKind.Name = "other-kind"
	otherKind.Entity.Kind = "host"
	if _, err := mapEndpoint(&Recipe{}, otherKind, []byte(`{"items":[{"id":"same","name":"host"}]}`), seen); err != nil {
		t.Fatalf("same ID with another kind should be allowed: %v", err)
	}
}

func TestMapEndpointAttributeScenarios(t *testing.T) {
	tests := []struct {
		name       string
		definition RecipeAttribute
		item       string
		want       any
		wantAbsent bool
		wantError  string
	}{
		{
			name:       "missing path is omitted",
			definition: RecipeAttribute{Path: "missing", HasPath: true},
			item:       `{"id":"1"}`,
			wantAbsent: true,
		},
		{
			name:       "typed boolean",
			definition: RecipeAttribute{Path: "enabled", HasPath: true, Type: "bool"},
			item:       `{"id":"1","enabled":true}`,
			want:       true,
		},
		{
			name:       "value map",
			definition: RecipeAttribute{Path: "state", HasPath: true, Type: "string", Map: map[string]any{"1": "enabled"}},
			item:       `{"id":"1","state":1}`,
			want:       "enabled",
		},
		{
			name:       "unmapped value uses default",
			definition: RecipeAttribute{Path: "state", HasPath: true, Type: "string", Map: map[string]any{"1": "enabled"}, Default: "unknown", HasDefault: true},
			item:       `{"id":"1","state":2}`,
			want:       "unknown",
		},
		{
			name:       "mapped value converts after replacement",
			definition: RecipeAttribute{Path: "state", HasPath: true, Type: "number", Map: map[string]any{"small": "12.5"}},
			item:       `{"id":"1","state":"small"}`,
			want:       json.Number("12.5"),
		},
		{
			name:       "default converts after replacement",
			definition: RecipeAttribute{Path: "state", HasPath: true, Type: "number", Map: map[string]any{"1": "2"}, Default: "3.5", HasDefault: true},
			item:       `{"id":"1","state":9}`,
			want:       json.Number("3.5"),
		},
		{
			name:       "empty map uses default",
			definition: RecipeAttribute{Path: "state", HasPath: true, Map: map[string]any{}, Default: "unknown", HasDefault: true},
			item:       `{"id":"1","state":"anything"}`,
			want:       "unknown",
		},
		{
			name:       "constant",
			definition: RecipeAttribute{Const: "sonarr", HasConst: true},
			item:       `{"id":"1"}`,
			want:       "sonarr",
		},
		{
			name:       "template and brace escapes",
			definition: RecipeAttribute{Template: "{{{host}}}:{port}", HasTemplate: true},
			item:       `{"id":"1","host":"nas","port":8096}`,
			want:       "{nas}:8096",
		},
		{
			name:       "missing template path is omitted",
			definition: RecipeAttribute{Template: "{host}:{port}", HasTemplate: true},
			item:       `{"id":"1","host":"nas"}`,
			wantAbsent: true,
		},
		{
			name:       "string array alias type",
			definition: RecipeAttribute{Path: "aliases", HasPath: true, Type: "string_array"},
			item:       `{"id":"1","aliases":["nas","nas.local"]}`,
			want:       []string{"nas", "nas.local"},
		},
		{
			name:       "number conversion failure",
			definition: RecipeAttribute{Path: "value", HasPath: true, Type: "number"},
			item:       `{"id":"1","value":"abc"}`,
			wantError:  `endpoint "records" attribute "broken":`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			endpoint := RecipeEndpoint{
				Name:  "records",
				Items: "items",
				Entity: RecipeEntity{
					Kind: "record", Name: "name", ExternalID: "id",
					Attributes: map[string]RecipeAttribute{"broken": test.definition},
				},
			}
			body := []byte(`{"items":[` + test.item + `]}`)
			result, err := mapEndpoint(&Recipe{}, endpoint, body, nil)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want it to contain %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, exists := result.Entities[0].Attributes["broken"]
			if test.wantAbsent {
				if exists {
					t.Fatalf("attribute unexpectedly present: %#v", got)
				}
				return
			}
			if !exists || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("attribute = %#v (present %t), want %#v", got, exists, test.want)
			}
		})
	}
}

func TestMapEndpointRejectsInvalidTypedAttributeValues(t *testing.T) {
	tests := []struct {
		name string
		attr RecipeAttribute
		item string
	}{
		{name: "boolean", attr: RecipeAttribute{Path: "value", HasPath: true, Type: "boolean"}, item: `{"id":"1","value":"yes"}`},
		{name: "list element", attr: RecipeAttribute{Path: "value", HasPath: true, Type: "list"}, item: `{"id":"1","value":["ok",1]}`},
		{name: "nested object", attr: RecipeAttribute{Path: "value", HasPath: true}, item: `{"id":"1","value":{"unsafe":true}}`},
		{name: "number overflow", attr: RecipeAttribute{Path: "value", HasPath: true}, item: `{"id":"1","value":1e400}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			endpoint := RecipeEndpoint{
				Name:  "typed",
				Items: "items",
				Entity: RecipeEntity{
					Kind: "device", Name: "name", ExternalID: "id",
					Attributes: map[string]RecipeAttribute{"value": test.attr},
				},
			}
			_, err := mapEndpoint(&Recipe{}, endpoint, []byte(`{"items":[`+test.item+`]}`), nil)
			if err == nil || !strings.Contains(err.Error(), `endpoint "typed" attribute "value"`) {
				t.Fatalf("error = %v, want endpoint and attribute in error", err)
			}
		})
	}
}

func TestMapEndpointMapsAndDeduplicatesRootAndEndpointDependencies(t *testing.T) {
	recipe := &Recipe{Dependencies: []RecipeDependency{
		{Kind: "upstream_service", Path: "services.#.name", HasPath: true},
		{Kind: "storage", Const: "tank", HasConst: true},
	}}
	endpoint := RecipeEndpoint{
		Name: "services", Items: "items",
		Entity: RecipeEntity{Kind: "service", Name: "name", ExternalID: "id"},
		Dependencies: []RecipeDependency{
			{Kind: "upstream_service", Path: "upstreams.#.name", HasPath: true},
			{Kind: "storage", Const: "tank", HasConst: true},
			{Kind: "network", Const: "backend", HasConst: true},
		},
	}
	body := []byte(`{"items":[{"id":"1","name":"one"}],"services":[{"name":"qbittorrent"},{"name":"sabnzbd"},{"name":"qbittorrent"}],"upstreams":[{"name":"qbittorrent"},{"name":"prowlarr"}]}`)
	result, err := mapEndpoint(recipe, endpoint, body, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]connector.ServiceDependency{
		"upstream_service\x00qbittorrent": {Kind: "upstream_service", Name: "qbittorrent"},
		"upstream_service\x00sabnzbd":     {Kind: "upstream_service", Name: "sabnzbd"},
		"upstream_service\x00prowlarr":    {Kind: "upstream_service", Name: "prowlarr"},
		"storage\x00tank":                 {Kind: "storage", Name: "tank"},
		"network\x00backend":              {Kind: "network", Name: "backend"},
	}
	if len(result.Dependencies) != len(want) {
		t.Fatalf("dependencies = %#v, want %d unique values", result.Dependencies, len(want))
	}
	for _, dependency := range result.Dependencies {
		key := dependency.Kind + "\x00" + dependency.Name
		if got, ok := want[key]; !ok || got != dependency {
			t.Errorf("unexpected dependency %#v", dependency)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Errorf("missing dependencies: %#v", want)
	}
}

func TestMapEndpointRequiresItemsList(t *testing.T) {
	endpoint := RecipeEndpoint{
		Name: "records", Items: "items",
		Entity: RecipeEntity{Kind: "record", Name: "name", ExternalID: "id"},
	}
	_, err := mapEndpoint(&Recipe{}, endpoint, []byte(`{"items":{"id":"1"}}`), nil)
	if err == nil || !strings.Contains(err.Error(), `endpoint "records"`) || !strings.Contains(err.Error(), `items`) {
		t.Fatalf("error = %v, want endpoint and items path", err)
	}
}
