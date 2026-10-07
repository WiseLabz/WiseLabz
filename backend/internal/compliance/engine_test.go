package compliance

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func testCatalog() Catalog {
	return Catalog{
		"docker": {"container": {
			{Name: "name", Type: "string"},
			{Name: "tags", Type: "string_array"},
			{Name: "ports", Type: "number"},
			{Name: "privileged", Type: "boolean"},
		}},
		"proxmox": {"vm": {
			{Name: "external_id", Type: "string"},
			{Name: "tags", Type: "string_array"},
			{Name: "memory_gb", Type: "number"},
		}},
		"pbs": {"backup_job": {
			{Name: "external_id", Type: "string"},
			{Name: "last_run_days_ago", Type: "number"},
		}},
	}
}

func TestEvaluateOperators(t *testing.T) {
	entity := Entity{Kind: "container", Name: "web", Attributes: map[string]any{
		"name": "web-server", "tags": []any{"prod", "public"}, "ports": json.Number("8080"), "privileged": true,
	}}
	tests := []struct {
		name      string
		condition Condition
		want      bool
	}{
		{"string eq", Condition{"name", "eq", "web-server"}, true},
		{"string neq", Condition{"name", "neq", "db"}, true},
		{"string contains", Condition{"name", "contains", "server"}, true},
		{"string regex", Condition{"name", "regex", "^web-"}, true},
		{"string exists", Condition{"name", "exists", true}, true},
		{"array eq decoded", Condition{"tags", "eq", []string{"prod", "public"}}, true},
		{"array neq", Condition{"tags", "neq", []string{"prod"}}, true},
		{"array contains", Condition{"tags", "contains", "prod"}, true},
		{"array exists", Condition{"tags", "exists", true}, true},
		{"number eq json", Condition{"ports", "eq", float64(8080)}, true},
		{"number neq", Condition{"ports", "neq", 443}, true},
		{"number gt", Condition{"ports", "gt", 1024}, true},
		{"number lt", Condition{"ports", "lt", 9000}, true},
		{"number exists", Condition{"ports", "exists", true}, true},
		{"boolean eq", Condition{"privileged", "eq", true}, true},
		{"boolean neq", Condition{"privileged", "neq", false}, true},
		{"boolean exists", Condition{"privileged", "exists", true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := Rule{EntityKind: "container", Conditions: []Condition{tt.condition}}
			got := Evaluate(rule, Snapshot{Entities: []Entity{entity}})
			if (len(got) == 1) != tt.want {
				t.Fatalf("matches = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateEdgeCases(t *testing.T) {
	entity := Entity{Kind: "container", Name: "web", Attributes: map[string]any{
		"null": nil, "name": "web", "tags": []string{"prod"}, "ports": "8080", "privileged": "true",
	}}
	tests := []struct {
		name      string
		condition Condition
		want      bool
	}{
		{"missing eq", Condition{"missing", "eq", "x"}, false},
		{"missing neq", Condition{"missing", "neq", "x"}, true},
		{"missing exists true", Condition{"missing", "exists", true}, false},
		{"missing exists false", Condition{"missing", "exists", false}, true},
		{"null eq", Condition{"null", "eq", nil}, false},
		{"null neq", Condition{"null", "neq", "x"}, false},
		{"null exists", Condition{"null", "exists", true}, true},
		{"wrong contains value", Condition{"tags", "contains", 2}, false},
		{"string regex mismatch", Condition{"name", "regex", "^db"}, false},
		{"invalid regex", Condition{"name", "regex", "["}, false},
		{"number type mismatch", Condition{"ports", "gt", 80}, false},
		{"boolean type mismatch", Condition{"privileged", "eq", true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(Rule{EntityKind: "container", Conditions: []Condition{tt.condition}}, Snapshot{Entities: []Entity{entity}})
			if (len(got) == 1) != tt.want {
				t.Fatalf("matches = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateAndKindAndOrder(t *testing.T) {
	rule := Rule{EntityKind: "container", Conditions: []Condition{
		{Attribute: "name", Op: "contains", Value: "web"},
		{Attribute: "privileged", Op: "eq", Value: true},
	}}
	snapshot := Snapshot{Entities: []Entity{
		{Kind: "vm", Name: "wrong-kind", Attributes: map[string]any{"name": "web", "privileged": true}},
		{Kind: "container", Name: "not-privileged", Attributes: map[string]any{"name": "web", "privileged": false}},
		{Kind: "container", Name: "second", Attributes: map[string]any{"name": "web2", "privileged": true}},
		{Kind: "container", Name: "third", Attributes: map[string]any{"name": "web3", "privileged": true}},
	}}
	got := Evaluate(rule, snapshot)
	if len(got) != 2 || got[0].Name != "second" || got[1].Name != "third" {
		t.Fatalf("matches = %#v", got)
	}
}

func TestEvaluateLargeSnapshot(t *testing.T) {
	entities := make([]Entity, 0, 1001)
	for range 1000 {
		entities = append(entities, Entity{Kind: "container", Name: "no", Attributes: map[string]any{"privileged": false}})
	}
	entities = append(entities, Entity{Kind: "container", Name: "yes", Attributes: map[string]any{"privileged": true}})
	got := Evaluate(Rule{EntityKind: "container", Conditions: []Condition{{Attribute: "privileged", Op: "eq", Value: true}}}, Snapshot{Entities: entities})
	if len(got) != 1 || got[0].Name != "yes" {
		t.Fatalf("matches = %#v", got)
	}
}

func TestValidate(t *testing.T) {
	valid := Rule{ConnectorType: "docker", EntityKind: "container", Conditions: []Condition{{Attribute: "name", Op: "eq", Value: "web"}}}
	tests := []struct {
		name string
		rule Rule
		want string
	}{
		{"valid", valid, ""},
		{"unknown connector", Rule{ConnectorType: "nope", EntityKind: "container", Conditions: valid.Conditions}, "unknown connector"},
		{"unknown kind", Rule{ConnectorType: "docker", EntityKind: "vm", Conditions: valid.Conditions}, "unknown entity kind"},
		{"unknown attribute", Rule{ConnectorType: "docker", EntityKind: "container", Conditions: []Condition{{Attribute: "nope", Op: "eq", Value: "x"}}}, "unknown attribute"},
		{"invalid string op", Rule{ConnectorType: "docker", EntityKind: "container", Conditions: []Condition{{Attribute: "name", Op: "gt", Value: "x"}}}, "not valid"},
		{"invalid array op", Rule{ConnectorType: "docker", EntityKind: "container", Conditions: []Condition{{Attribute: "tags", Op: "regex", Value: "x"}}}, "not valid"},
		{"invalid number op", Rule{ConnectorType: "docker", EntityKind: "container", Conditions: []Condition{{Attribute: "ports", Op: "contains", Value: "x"}}}, "not valid"},
		{"invalid boolean op", Rule{ConnectorType: "docker", EntityKind: "container", Conditions: []Condition{{Attribute: "privileged", Op: "contains", Value: "x"}}}, "not valid"},
		{"empty", Rule{ConnectorType: "docker", EntityKind: "container"}, "at least"},
		{"invalid regex", Rule{ConnectorType: "docker", EntityKind: "container", Conditions: []Condition{{Attribute: "name", Op: "regex", Value: "["}}}, "invalid regex"},
		{"non-string regex", Rule{ConnectorType: "docker", EntityKind: "container", Conditions: []Condition{{Attribute: "name", Op: "regex", Value: 1}}}, "must be a string"},
		{"long regex", Rule{ConnectorType: "docker", EntityKind: "container", Conditions: []Condition{{Attribute: "name", Op: "regex", Value: strings.Repeat("a", maxRegexLength+1)}}}, "exceeds"},
	}
	tooMany := valid
	tooMany.Conditions = make([]Condition, maxConditions+1)
	for i := range tooMany.Conditions {
		tooMany.Conditions[i] = valid.Conditions[0]
	}
	tests = append(tests, struct {
		name string
		rule Rule
		want string
	}{"too many", tooMany, "at most"})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.rule, testCatalog())
			if tt.want == "" && err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Fatalf("Validate error = %v, want %q", err, tt.want)
			}
		})
	}
}

// TestValidateFieldAttribution pins the rule field each rejection names, so
// the API layer can surface it as a {field, msg} detail without re-deriving
// it from the message text.
func TestValidateFieldAttribution(t *testing.T) {
	ok := Condition{Attribute: "name", Op: "eq", Value: "web"}
	rule := func(conditions ...Condition) Rule {
		return Rule{ConnectorType: "docker", EntityKind: "container", Conditions: conditions}
	}
	tests := []struct {
		name  string
		rule  Rule
		field string
	}{
		{"unknown connector", Rule{ConnectorType: "nope", EntityKind: "container", Conditions: []Condition{ok}}, "connectorType"},
		{"unknown kind", Rule{ConnectorType: "docker", EntityKind: "vm", Conditions: []Condition{ok}}, "entityKind"},
		{"empty conditions", rule(), "conditions"},
		{"unknown attribute", rule(ok, Condition{Attribute: "nope", Op: "eq", Value: "x"}), "conditions[1].attribute"},
		{"invalid op", rule(ok, Condition{Attribute: "name", Op: "gt", Value: "x"}), "conditions[1].op"},
		{"invalid regex", rule(Condition{Attribute: "name", Op: "regex", Value: "["}), "conditions[0].value"},
		{"non-string regex", rule(ok, ok, Condition{Attribute: "name", Op: "regex", Value: 1}), "conditions[2].value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.rule, testCatalog())

			var invalid *ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("Validate error = %v, want *ValidationError", err)
			}
			if len(invalid.Fields) != 1 {
				t.Fatalf("Fields = %+v, want exactly one", invalid.Fields)
			}
			if invalid.Fields[0].Field != tt.field {
				t.Errorf("Field = %q, want %q", invalid.Fields[0].Field, tt.field)
			}
			if invalid.Fields[0].Msg == "" {
				t.Error("Msg is empty")
			}
			// Error() must still read as the sentence callers already log.
			if invalid.Error() != invalid.Message {
				t.Errorf("Error() = %q, want %q", invalid.Error(), invalid.Message)
			}
		})
	}
}

// TestValidateRegexErrorUnwraps keeps the regexp compile failure reachable
// through the ValidationError wrapper.
func TestValidateRegexErrorUnwraps(t *testing.T) {
	err := Validate(
		Rule{ConnectorType: "docker", EntityKind: "container", Conditions: []Condition{{Attribute: "name", Op: "regex", Value: "["}}},
		testCatalog(),
	)
	var invalid *ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("Validate error = %v, want *ValidationError", err)
	}
	if errors.Unwrap(invalid) == nil {
		t.Error("Unwrap() = nil, want the regexp compile error")
	}
}

// TestNotContains tests the not_contains operator
func TestNotContains(t *testing.T) {
	tests := []struct {
		name      string
		entity    Entity
		condition Condition
		want      bool
	}{
		{"string negation", Entity{Kind: "container", Attributes: map[string]any{"name": "server"}}, Condition{"name", "not_contains", "db"}, true},
		{"string contains match", Entity{Kind: "container", Attributes: map[string]any{"name": "database"}}, Condition{"name", "not_contains", "base"}, false},
		{"string array substring is not an element", Entity{Kind: "container", Attributes: map[string]any{"tags": []any{"no-backup-yet"}}}, Condition{"tags", "not_contains", "no-backup"}, true},
		{"string array exact element any", Entity{Kind: "container", Attributes: map[string]any{"tags": []any{"no-backup"}}}, Condition{"tags", "not_contains", "no-backup"}, false},
		{"missing attribute with non-string needle", Entity{Kind: "container", Attributes: map[string]any{}}, Condition{"missing", "not_contains", 1}, false},
		{"string array exact element", Entity{Kind: "container", Attributes: map[string]any{"tags": []string{"prod", "backup"}}}, Condition{"tags", "not_contains", "prod"}, false},
		{"string array no element", Entity{Kind: "container", Attributes: map[string]any{"tags": []string{"dev", "test"}}}, Condition{"tags", "not_contains", "prod"}, true},
		{"missing attribute", Entity{Kind: "container", Attributes: map[string]any{}}, Condition{"missing", "not_contains", "value"}, true},
		{"null attribute", Entity{Kind: "container", Attributes: map[string]any{"attr": nil}}, Condition{"attr", "not_contains", "value"}, true},
		{"non-string needle", Entity{Kind: "container", Attributes: map[string]any{"name": "server"}}, Condition{"name", "not_contains", 123}, false},
		{"array with any types", Entity{Kind: "container", Attributes: map[string]any{"tags": []any{"a", "b", "c"}}}, Condition{"tags", "not_contains", "d"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := Rule{EntityKind: "container", Conditions: []Condition{tt.condition}}
			got := Evaluate(rule, Snapshot{Entities: []Entity{tt.entity}})
			if (len(got) == 1) != tt.want {
				t.Errorf("matches = %v, want %v", len(got) == 1, tt.want)
			}
		})
	}
}

// TestEvaluateWithRelatedBasic tests EvaluateWithRelated with simple requires and forbids
func TestEvaluateWithRelatedBasic(t *testing.T) {
	sourceRule := Rule{
		EntityKind: "vm",
		Conditions: []Condition{{Attribute: "memory_gb", Op: "gt", Value: float64(2)}},
		Related: []RelatedClause{{
			Mode:          ModeRequires,
			ConnectorType: "pbs",
			EntityKind:    "backup_job",
			Join: Join{
				SourceField:  "external_id",
				RelatedField: "external_id",
			},
		}},
	}
	sourceSnapshot := Snapshot{Entities: []Entity{
		{Kind: "vm", Name: "vm1", ExternalID: "pve-vm1", Attributes: map[string]any{"memory_gb": 4.0}},
		{Kind: "vm", Name: "vm2", ExternalID: "pve-vm2", Attributes: map[string]any{"memory_gb": 8.0}},
	}}
	relatedEntities := RelatedEntities{
		"pbs": {
			{Kind: "backup_job", Name: "backup1", ExternalID: "pve-vm1", Attributes: map[string]any{}},
		},
	}

	got, skipped := EvaluateWithRelated(sourceRule, sourceSnapshot, relatedEntities)
	if skipped {
		t.Fatal("skipped = true, want false")
	}
	if len(got) != 1 || got[0].Name != "vm2" {
		t.Fatalf("matches = %v, want vm2 (violated requires)", got)
	}
}

// TestEvaluateWithRelatedForbids tests forbids mode
func TestEvaluateWithRelatedForbids(t *testing.T) {
	sourceRule := Rule{
		EntityKind: "vm",
		Conditions: []Condition{{Attribute: "memory_gb", Op: "gt", Value: float64(0)}},
		Related: []RelatedClause{{
			Mode:          ModeForbids,
			ConnectorType: "pbs",
			EntityKind:    "backup_job",
			Join: Join{
				SourceField:  "external_id",
				RelatedField: "external_id",
			},
		}},
	}
	sourceSnapshot := Snapshot{Entities: []Entity{
		{Kind: "vm", Name: "vm1", ExternalID: "pve-vm1", Attributes: map[string]any{"memory_gb": 4.0}},
		{Kind: "vm", Name: "vm2", ExternalID: "pve-vm2", Attributes: map[string]any{"memory_gb": 8.0}},
	}}
	relatedEntities := RelatedEntities{
		"pbs": {
			{Kind: "backup_job", Name: "backup1", ExternalID: "pve-vm1", Attributes: map[string]any{}},
		},
	}

	got, skipped := EvaluateWithRelated(sourceRule, sourceSnapshot, relatedEntities)
	if skipped {
		t.Fatal("skipped = true, want false")
	}
	if len(got) != 1 || got[0].Name != "vm1" {
		t.Fatalf("matches = %v, want vm1 (violates forbids)", got)
	}
}

// TestEvaluateWithRelatedMultipleClausesANDed tests that multiple clauses are ANDed
func TestEvaluateWithRelatedMultipleClauses(t *testing.T) {
	sourceRule := Rule{
		EntityKind: "vm",
		Conditions: []Condition{{Attribute: "memory_gb", Op: "gt", Value: float64(0)}},
		Related: []RelatedClause{
			{
				Mode:          ModeRequires,
				ConnectorType: "pbs",
				EntityKind:    "backup_job",
				Join:          Join{SourceField: "external_id", RelatedField: "external_id"},
			},
			{
				Mode:          ModeRequires,
				ConnectorType: "docker",
				EntityKind:    "container",
				Join:          Join{SourceField: "name", RelatedField: "name"},
			},
		},
	}
	sourceSnapshot := Snapshot{Entities: []Entity{
		{Kind: "vm", Name: "db", ExternalID: "pve-db", Attributes: map[string]any{"memory_gb": 4.0}},
	}}
	relatedEntities := RelatedEntities{
		"pbs": {
			{Kind: "backup_job", Name: "bk", ExternalID: "pve-db", Attributes: map[string]any{}},
		},
		"docker": {
			// No container named "db", so second requires is violated
		},
	}

	got, skipped := EvaluateWithRelated(sourceRule, sourceSnapshot, relatedEntities)
	if skipped {
		t.Fatal("skipped = true, want false")
	}
	if len(got) != 1 {
		t.Fatalf("matches = %v, want db (second requires violated)", got)
	}
}

// TestEvaluateWithRelatedSkipped tests skipping when connector type is absent
func TestEvaluateWithRelatedSkipped(t *testing.T) {
	sourceRule := Rule{
		EntityKind: "vm",
		Conditions: []Condition{{Attribute: "memory_gb", Op: "gt", Value: float64(0)}},
		Related: []RelatedClause{{
			Mode:          ModeRequires,
			ConnectorType: "pbs",
			EntityKind:    "backup_job",
			Join:          Join{SourceField: "external_id", RelatedField: "external_id"},
		}},
	}
	sourceSnapshot := Snapshot{Entities: []Entity{
		{Kind: "vm", Name: "vm1", ExternalID: "pve-vm1", Attributes: map[string]any{"memory_gb": 4.0}},
	}}
	relatedEntities := RelatedEntities{
		// pbs not present
	}

	got, skipped := EvaluateWithRelated(sourceRule, sourceSnapshot, relatedEntities)
	if !skipped || got != nil {
		t.Fatalf("skipped = %v, got = %v, want skipped=true, got=nil", skipped, got)
	}
}

// TestEvaluateWithRelatedEmptySlice tests empty slice (snapshot exists but no entities)
func TestEvaluateWithRelatedEmptySlice(t *testing.T) {
	sourceRule := Rule{
		EntityKind: "vm",
		Conditions: []Condition{{Attribute: "memory_gb", Op: "gt", Value: float64(0)}},
		Related: []RelatedClause{{
			Mode:          ModeRequires,
			ConnectorType: "pbs",
			EntityKind:    "backup_job",
			Join:          Join{SourceField: "external_id", RelatedField: "external_id"},
		}},
	}
	sourceSnapshot := Snapshot{Entities: []Entity{
		{Kind: "vm", Name: "vm1", ExternalID: "pve-vm1", Attributes: map[string]any{"memory_gb": 4.0}},
	}}
	relatedEntities := RelatedEntities{
		"pbs": {}, // Empty slice, not missing
	}

	got, skipped := EvaluateWithRelated(sourceRule, sourceSnapshot, relatedEntities)
	if skipped {
		t.Fatal("skipped = true, want false (empty slice means snapshot exists)")
	}
	if len(got) != 1 {
		t.Fatalf("matches = %v, want vm1 (requires violated, no backup entities)", got)
	}
}

// TestEvaluateWithRelatedWrongKindIgnored tests that related entities of wrong kind are ignored
func TestEvaluateWithRelatedWrongKind(t *testing.T) {
	sourceRule := Rule{
		EntityKind: "vm",
		Conditions: []Condition{{Attribute: "memory_gb", Op: "gt", Value: float64(0)}},
		Related: []RelatedClause{{
			Mode:          ModeRequires,
			ConnectorType: "pbs",
			EntityKind:    "backup_job",
			Join:          Join{SourceField: "external_id", RelatedField: "external_id"},
		}},
	}
	sourceSnapshot := Snapshot{Entities: []Entity{
		{Kind: "vm", Name: "vm1", ExternalID: "pve-vm1", Attributes: map[string]any{"memory_gb": 4.0}},
	}}
	relatedEntities := RelatedEntities{
		"pbs": {
			{Kind: "storage", Name: "storage1", ExternalID: "pve-vm1", Attributes: map[string]any{}},
		},
	}

	got, skipped := EvaluateWithRelated(sourceRule, sourceSnapshot, relatedEntities)
	if skipped {
		t.Fatal("skipped = true, want false")
	}
	if len(got) != 1 {
		t.Fatalf("matches = %v, want vm1 (wrong kind ignored)", got)
	}
}

// TestEvaluateWithRelatedConditionFilters tests that related entity conditions filter entities
func TestEvaluateWithRelatedConditionFilters(t *testing.T) {
	sourceRule := Rule{
		EntityKind: "vm",
		Conditions: []Condition{{Attribute: "memory_gb", Op: "gt", Value: float64(0)}},
		Related: []RelatedClause{{
			Mode:          ModeRequires,
			ConnectorType: "pbs",
			EntityKind:    "backup_job",
			Join:          Join{SourceField: "external_id", RelatedField: "external_id"},
			Conditions:    []Condition{{Attribute: "last_run_days_ago", Op: "lt", Value: float64(7)}},
		}},
	}
	sourceSnapshot := Snapshot{Entities: []Entity{
		{Kind: "vm", Name: "vm1", ExternalID: "pve-vm1", Attributes: map[string]any{"memory_gb": 4.0}},
	}}
	relatedEntities := RelatedEntities{
		"pbs": {
			{Kind: "backup_job", Name: "backup1", ExternalID: "pve-vm1", Attributes: map[string]any{"last_run_days_ago": 14.0}},
		},
	}

	got, skipped := EvaluateWithRelated(sourceRule, sourceSnapshot, relatedEntities)
	if skipped {
		t.Fatal("skipped = true, want false")
	}
	if len(got) != 1 {
		t.Fatalf("matches = %v, want vm1 (backup older than 7 days)", got)
	}
}

// TestEvaluateWithRelatedJoinTypedFields tests join on typed fields
func TestEvaluateWithRelatedJoinTypedFields(t *testing.T) {
	tests := []struct {
		name        string
		joinSource  string
		joinRelated string
		sourceVal   string
		relatedVal  string
		shouldMatch bool
	}{
		{"external_id", "external_id", "external_id", "pve-vm1", "pve-vm1", true},
		{"name", "name", "name", "backup1", "backup1", true},
		{"ip", "ip", "ip", "192.168.1.1", "192.168.1.1", true},
		{"hostname", "hostname", "hostname", "host1", "host1", true},
		{"mac", "mac", "mac", "00:11:22:33:44:55", "00:11:22:33:44:55", true},
		{"case insensitive", "external_id", "external_id", "PVE-VM1", "pve-vm1", true},
		{"external_id mismatch", "external_id", "external_id", "pve-vm1", "pve-vm2", false},
		{"name mismatch", "name", "name", "backup1", "backup2", false},
		{"ip mismatch", "ip", "ip", "192.168.1.1", "192.168.1.2", false},
		{"hostname mismatch", "hostname", "hostname", "host1", "host2", false},
		{"mac mismatch", "mac", "mac", "00:11:22:33:44:55", "00:11:22:33:44:66", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceRule := Rule{
				EntityKind: "vm",
				Conditions: []Condition{{Attribute: "memory_gb", Op: "gt", Value: float64(0)}},
				Related: []RelatedClause{{
					Mode:          ModeRequires,
					ConnectorType: "pbs",
					EntityKind:    "backup_job",
					Join:          Join{SourceField: tt.joinSource, RelatedField: tt.joinRelated},
				}},
			}

			var sourceEntity Entity
			var relatedEntity Entity
			switch tt.joinSource {
			case "external_id":
				sourceEntity = Entity{Kind: "vm", Name: "vm1", ExternalID: tt.sourceVal, Attributes: map[string]any{"memory_gb": 4.0}}
				relatedEntity = Entity{Kind: "backup_job", Name: "bk", ExternalID: tt.relatedVal}
			case "name":
				sourceEntity = Entity{Kind: "vm", Name: tt.sourceVal, ExternalID: "pve-vm1", Attributes: map[string]any{"memory_gb": 4.0}}
				relatedEntity = Entity{Kind: "backup_job", Name: tt.relatedVal, ExternalID: "pve-bk1"}
			case "ip":
				sourceEntity = Entity{Kind: "vm", Name: "vm1", IP: tt.sourceVal, Attributes: map[string]any{"memory_gb": 4.0}}
				relatedEntity = Entity{Kind: "backup_job", Name: "bk", IP: tt.relatedVal}
			case "hostname":
				sourceEntity = Entity{Kind: "vm", Name: "vm1", Hostname: tt.sourceVal, Attributes: map[string]any{"memory_gb": 4.0}}
				relatedEntity = Entity{Kind: "backup_job", Name: "bk", Hostname: tt.relatedVal}
			case "mac":
				sourceEntity = Entity{Kind: "vm", Name: "vm1", MAC: tt.sourceVal, Attributes: map[string]any{"memory_gb": 4.0}}
				relatedEntity = Entity{Kind: "backup_job", Name: "bk", MAC: tt.relatedVal}
			}

			sourceSnapshot := Snapshot{Entities: []Entity{sourceEntity}}
			relatedEntities := RelatedEntities{"pbs": {relatedEntity}}

			got, skipped := EvaluateWithRelated(sourceRule, sourceSnapshot, relatedEntities)
			if skipped {
				t.Fatal("skipped = true, want false")
			}
			if tt.shouldMatch && len(got) != 0 {
				t.Fatalf("matches = %v, want empty (join matched)", got)
			}
			if !tt.shouldMatch && len(got) != 1 {
				t.Fatalf("matches = %v, want vm1 (join didn't match)", got)
			}
		})
	}
}

// TestEvaluateWithRelatedJoinAttributes tests join on attributes
func TestEvaluateWithRelatedJoinAttributes(t *testing.T) {
	sourceRule := Rule{
		EntityKind: "vm",
		Conditions: []Condition{{Attribute: "memory_gb", Op: "gt", Value: float64(0)}},
		Related: []RelatedClause{{
			Mode:          ModeRequires,
			ConnectorType: "pbs",
			EntityKind:    "backup_job",
			Join:          Join{SourceField: "attributes.backup_id", RelatedField: "external_id"},
		}},
	}
	sourceSnapshot := Snapshot{Entities: []Entity{
		{Kind: "vm", Name: "vm1", ExternalID: "pve-vm1", Attributes: map[string]any{"memory_gb": 4.0, "backup_id": "bk-123"}},
	}}
	relatedEntities := RelatedEntities{
		"pbs": {
			{Kind: "backup_job", Name: "backup1", ExternalID: "bk-123", Attributes: map[string]any{}},
		},
	}

	got, skipped := EvaluateWithRelated(sourceRule, sourceSnapshot, relatedEntities)
	if skipped {
		t.Fatal("skipped = true, want false")
	}
	if len(got) != 0 {
		t.Fatalf("matches = %v, want empty (join on attributes matched)", got)
	}
}

// TestEvaluateWithRelatedNumberConversion tests number-to-string conversion in join
func TestEvaluateWithRelatedNumberConversion(t *testing.T) {
	sourceRule := Rule{
		EntityKind: "vm",
		Conditions: []Condition{{Attribute: "memory_gb", Op: "gt", Value: float64(0)}},
		Related: []RelatedClause{{
			Mode:          ModeRequires,
			ConnectorType: "pbs",
			EntityKind:    "backup_job",
			Join:          Join{SourceField: "attributes.vm_id", RelatedField: "attributes.vm_id"},
		}},
	}
	sourceSnapshot := Snapshot{Entities: []Entity{
		{Kind: "vm", Name: "vm1", Attributes: map[string]any{"memory_gb": 4.0, "vm_id": 104.0}},
	}}
	relatedEntities := RelatedEntities{
		"pbs": {
			{Kind: "backup_job", Name: "backup1", Attributes: map[string]any{"vm_id": "104"}},
		},
	}

	got, skipped := EvaluateWithRelated(sourceRule, sourceSnapshot, relatedEntities)
	if skipped {
		t.Fatal("skipped = true, want false")
	}
	if len(got) != 0 {
		t.Fatalf("matches = %v, want empty (number 104 matches string '104')", got)
	}
}

// TestEvaluateWithRelatedEmptyJoinValue tests that empty join values never match
func TestEvaluateWithRelatedEmptyJoinValue(t *testing.T) {
	sourceRule := Rule{
		EntityKind: "vm",
		Conditions: []Condition{{Attribute: "memory_gb", Op: "gt", Value: float64(0)}},
		Related: []RelatedClause{{
			Mode:          ModeRequires,
			ConnectorType: "pbs",
			EntityKind:    "backup_job",
			Join:          Join{SourceField: "external_id", RelatedField: "external_id"},
		}},
	}
	sourceSnapshot := Snapshot{Entities: []Entity{
		{Kind: "vm", Name: "vm1", ExternalID: "", Attributes: map[string]any{"memory_gb": 4.0}},
	}}
	relatedEntities := RelatedEntities{
		"pbs": {
			{Kind: "backup_job", Name: "backup1", ExternalID: "", Attributes: map[string]any{}},
		},
	}

	got, skipped := EvaluateWithRelated(sourceRule, sourceSnapshot, relatedEntities)
	if skipped {
		t.Fatal("skipped = true, want false")
	}
	if len(got) != 1 {
		t.Fatalf("matches = %v, want vm1 (empty source join never matches)", got)
	}
}

// TestEvaluateWithRelatedSourceConditionFilter tests that source conditions still filter first
func TestEvaluateWithRelatedSourceConditionFilter(t *testing.T) {
	sourceRule := Rule{
		EntityKind: "vm",
		Conditions: []Condition{{Attribute: "memory_gb", Op: "lt", Value: float64(8)}},
		Related: []RelatedClause{{
			Mode:          ModeRequires,
			ConnectorType: "pbs",
			EntityKind:    "backup_job",
			Join:          Join{SourceField: "external_id", RelatedField: "external_id"},
		}},
	}
	sourceSnapshot := Snapshot{Entities: []Entity{
		{Kind: "vm", Name: "vm1", ExternalID: "pve-vm1", Attributes: map[string]any{"memory_gb": 4.0}},
		{Kind: "vm", Name: "vm2", ExternalID: "pve-vm2", Attributes: map[string]any{"memory_gb": 16.0}},
	}}
	relatedEntities := RelatedEntities{
		"pbs": {}, // No backups
	}

	got, skipped := EvaluateWithRelated(sourceRule, sourceSnapshot, relatedEntities)
	if skipped {
		t.Fatal("skipped = true, want false")
	}
	if len(got) != 1 || got[0].Name != "vm1" {
		t.Fatalf("matches = %v, want vm1 (vm2 filtered by source condition)", got)
	}
}

// TestEvaluateWithRelatedNoRelatedClauses tests that no related clauses returns Evaluate's result
func TestEvaluateWithRelatedNoRelated(t *testing.T) {
	sourceRule := Rule{
		EntityKind: "vm",
		Conditions: []Condition{{Attribute: "memory_gb", Op: "gt", Value: float64(4)}},
		Related:    []RelatedClause{},
	}
	sourceSnapshot := Snapshot{Entities: []Entity{
		{Kind: "vm", Name: "vm1", Attributes: map[string]any{"memory_gb": 2.0}},
		{Kind: "vm", Name: "vm2", Attributes: map[string]any{"memory_gb": 8.0}},
	}}
	relatedEntities := RelatedEntities{}

	got, skipped := EvaluateWithRelated(sourceRule, sourceSnapshot, relatedEntities)
	if skipped {
		t.Fatal("skipped = true, want false")
	}
	if len(got) != 1 || got[0].Name != "vm2" {
		t.Fatalf("matches = %v, want vm2", got)
	}
}

// TestEvaluateWithRelatedContainsStringArray tests contains on string_array
func TestContainsStringArray(t *testing.T) {
	entity := Entity{Kind: "container", Attributes: map[string]any{"tags": []string{"no-backup-yet", "prod"}}}
	// Exact element match, not substring
	rule := Rule{EntityKind: "container", Conditions: []Condition{{Attribute: "tags", Op: "contains", Value: "no-backup"}}}
	got := Evaluate(rule, Snapshot{Entities: []Entity{entity}})
	if len(got) != 0 {
		t.Fatalf("should not match substring in string_array, got %v", got)
	}

	rule2 := Rule{EntityKind: "container", Conditions: []Condition{{Attribute: "tags", Op: "contains", Value: "no-backup-yet"}}}
	got2 := Evaluate(rule2, Snapshot{Entities: []Entity{entity}})
	if len(got2) != 1 {
		t.Fatalf("should match exact element, got %v", got2)
	}
}

// TestValidateNotContains tests Validate with not_contains
func TestValidateNotContains(t *testing.T) {
	valid := Rule{ConnectorType: "docker", EntityKind: "container", Conditions: []Condition{{Attribute: "name", Op: "not_contains", Value: "test"}}}
	err := Validate(valid, testCatalog())
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// not_contains on number should fail
	invalid := Rule{ConnectorType: "docker", EntityKind: "container", Conditions: []Condition{{Attribute: "ports", Op: "not_contains", Value: "test"}}}
	err = Validate(invalid, testCatalog())
	if err == nil || !strings.Contains(err.Error(), "not valid") {
		t.Fatalf("not_contains on number should be invalid, got %v", err)
	}

	// not_contains on boolean should fail
	invalid2 := Rule{ConnectorType: "docker", EntityKind: "container", Conditions: []Condition{{Attribute: "privileged", Op: "not_contains", Value: "test"}}}
	err = Validate(invalid2, testCatalog())
	if err == nil || !strings.Contains(err.Error(), "not valid") {
		t.Fatalf("not_contains on boolean should be invalid, got %v", err)
	}
}

// TestValidateRelatedTooMany tests validation of too many related clauses
func TestValidateRelatedTooMany(t *testing.T) {
	rule := Rule{
		ConnectorType: "docker",
		EntityKind:    "container",
		Conditions:    []Condition{{Attribute: "name", Op: "eq", Value: "test"}},
	}
	for i := 0; i < maxRelatedClauses+1; i++ {
		rule.Related = append(rule.Related, RelatedClause{
			Mode:          ModeRequires,
			ConnectorType: "docker",
			EntityKind:    "container",
			Join:          Join{SourceField: "name", RelatedField: "name"},
		})
	}

	err := Validate(rule, testCatalog())
	if err == nil || !strings.Contains(err.Error(), "at most") {
		t.Fatalf("should reject too many clauses, got %v", err)
	}

	var invalid *ValidationError
	if !errors.As(err, &invalid) || len(invalid.Fields) != 1 {
		t.Fatalf("should have exactly one field error, got %v", invalid)
	}
	if invalid.Fields[0].Field != "related" {
		t.Errorf("Field = %q, want 'related'", invalid.Fields[0].Field)
	}
}

// TestValidateRelatedBadMode tests validation of invalid mode
func TestValidateRelatedBadMode(t *testing.T) {
	rule := Rule{
		ConnectorType: "docker",
		EntityKind:    "container",
		Conditions:    []Condition{{Attribute: "name", Op: "eq", Value: "test"}},
		Related: []RelatedClause{{
			Mode:          "invalid",
			ConnectorType: "docker",
			EntityKind:    "container",
			Join:          Join{SourceField: "name", RelatedField: "name"},
		}},
	}

	err := Validate(rule, testCatalog())
	var invalid *ValidationError
	if !errors.As(err, &invalid) || len(invalid.Fields) != 1 {
		t.Fatalf("should have exactly one field error, got %v", invalid)
	}
	if invalid.Fields[0].Field != "related[0].mode" {
		t.Errorf("Field = %q, want 'related[0].mode'", invalid.Fields[0].Field)
	}
}

// TestValidateRelatedUnknownConnectorType tests validation of unknown connector type in clause
func TestValidateRelatedUnknownConnectorType(t *testing.T) {
	rule := Rule{
		ConnectorType: "docker",
		EntityKind:    "container",
		Conditions:    []Condition{{Attribute: "name", Op: "eq", Value: "test"}},
		Related: []RelatedClause{{
			Mode:          ModeRequires,
			ConnectorType: "unknown",
			EntityKind:    "container",
			Join:          Join{SourceField: "name", RelatedField: "name"},
		}},
	}

	err := Validate(rule, testCatalog())
	var invalid *ValidationError
	if !errors.As(err, &invalid) || len(invalid.Fields) != 1 {
		t.Fatalf("should have exactly one field error, got %v", invalid)
	}
	if invalid.Fields[0].Field != "related[0].connectorType" {
		t.Errorf("Field = %q, want 'related[0].connectorType'", invalid.Fields[0].Field)
	}
}

// TestValidateRelatedUnknownEntityKind tests validation of unknown entity kind in clause
func TestValidateRelatedUnknownEntityKind(t *testing.T) {
	rule := Rule{
		ConnectorType: "docker",
		EntityKind:    "container",
		Conditions:    []Condition{{Attribute: "name", Op: "eq", Value: "test"}},
		Related: []RelatedClause{{
			Mode:          ModeRequires,
			ConnectorType: "docker",
			EntityKind:    "unknown",
			Join:          Join{SourceField: "name", RelatedField: "name"},
		}},
	}

	err := Validate(rule, testCatalog())
	var invalid *ValidationError
	if !errors.As(err, &invalid) || len(invalid.Fields) != 1 {
		t.Fatalf("should have exactly one field error, got %v", invalid)
	}
	if invalid.Fields[0].Field != "related[0].entityKind" {
		t.Errorf("Field = %q, want 'related[0].entityKind'", invalid.Fields[0].Field)
	}
}

// TestValidateRelatedUnknownAttribute tests validation of unknown attribute in clause conditions
func TestValidateRelatedUnknownAttribute(t *testing.T) {
	rule := Rule{
		ConnectorType: "docker",
		EntityKind:    "container",
		Conditions:    []Condition{{Attribute: "name", Op: "eq", Value: "test"}},
		Related: []RelatedClause{{
			Mode:          ModeRequires,
			ConnectorType: "proxmox",
			EntityKind:    "vm",
			Join:          Join{SourceField: "name", RelatedField: "name"},
			Conditions:    []Condition{{Attribute: "unknown_attr", Op: "eq", Value: "x"}},
		}},
	}

	err := Validate(rule, testCatalog())
	var invalid *ValidationError
	if !errors.As(err, &invalid) || len(invalid.Fields) != 1 {
		t.Fatalf("should have exactly one field error, got %v", invalid)
	}
	if invalid.Fields[0].Field != "related[0].conditions[0].attribute" {
		t.Errorf("Field = %q, want 'related[0].conditions[0].attribute'", invalid.Fields[0].Field)
	}
}

// TestValidateRelatedClauseConditionUsesRelatedCatalog proves clause conditions
// are checked against the related type: "privileged" exists on the source
// (docker container) but not on pbs backup_job.
func TestValidateRelatedClauseConditionUsesRelatedCatalog(t *testing.T) {
	rule := Rule{
		ConnectorType: "docker",
		EntityKind:    "container",
		Conditions:    []Condition{{Attribute: "name", Op: "eq", Value: "test"}},
		Related: []RelatedClause{{
			Mode:          ModeRequires,
			ConnectorType: "pbs",
			EntityKind:    "backup_job",
			Join:          Join{SourceField: "name", RelatedField: "name"},
			Conditions:    []Condition{{Attribute: "privileged", Op: "eq", Value: true}},
		}},
	}

	err := Validate(rule, testCatalog())
	var invalid *ValidationError
	if !errors.As(err, &invalid) || invalid.Fields[0].Field != "related[0].conditions[0].attribute" {
		t.Fatalf("Validate() = %v, want an attribute error at related[0].conditions[0].attribute", err)
	}

	rule.Related[0].Conditions = []Condition{{Attribute: "last_run_days_ago", Op: "lt", Value: float64(7)}}
	if err := Validate(rule, testCatalog()); err != nil {
		t.Fatalf("attribute of the related type must validate: %v", err)
	}
}

// TestValidateRelatedBadJoinSourceField tests validation of invalid join source field
func TestValidateRelatedBadJoinSourceField(t *testing.T) {
	rule := Rule{
		ConnectorType: "docker",
		EntityKind:    "container",
		Conditions:    []Condition{{Attribute: "name", Op: "eq", Value: "test"}},
		Related: []RelatedClause{{
			Mode:          ModeRequires,
			ConnectorType: "docker",
			EntityKind:    "container",
			Join:          Join{SourceField: "unknown_field", RelatedField: "name"},
		}},
	}

	err := Validate(rule, testCatalog())
	var invalid *ValidationError
	if !errors.As(err, &invalid) || len(invalid.Fields) != 1 {
		t.Fatalf("should have exactly one field error, got %v", invalid)
	}
	if invalid.Fields[0].Field != "related[0].join.sourceField" {
		t.Errorf("Field = %q, want 'related[0].join.sourceField'", invalid.Fields[0].Field)
	}
}

// TestValidateRelatedBadJoinRelatedField tests validation of invalid join related field
func TestValidateRelatedBadJoinRelatedField(t *testing.T) {
	rule := Rule{
		ConnectorType: "docker",
		EntityKind:    "container",
		Conditions:    []Condition{{Attribute: "name", Op: "eq", Value: "test"}},
		Related: []RelatedClause{{
			Mode:          ModeRequires,
			ConnectorType: "docker",
			EntityKind:    "container",
			Join:          Join{SourceField: "name", RelatedField: "unknown_field"},
		}},
	}

	err := Validate(rule, testCatalog())
	var invalid *ValidationError
	if !errors.As(err, &invalid) || len(invalid.Fields) != 1 {
		t.Fatalf("should have exactly one field error, got %v", invalid)
	}
	if invalid.Fields[0].Field != "related[0].join.relatedField" {
		t.Errorf("Field = %q, want 'related[0].join.relatedField'", invalid.Fields[0].Field)
	}
}

// TestValidateRelatedValidTypedJoin tests validation with valid typed join field
func TestValidateRelatedValidTypedJoin(t *testing.T) {
	for _, field := range JoinFields {
		rule := Rule{
			ConnectorType: "docker",
			EntityKind:    "container",
			Conditions:    []Condition{{Attribute: "name", Op: "eq", Value: "test"}},
			Related: []RelatedClause{{
				Mode:          ModeRequires,
				ConnectorType: "proxmox",
				EntityKind:    "vm",
				Join:          Join{SourceField: field, RelatedField: field},
			}},
		}
		err := Validate(rule, testCatalog())
		if err != nil {
			t.Fatalf("should accept typed field %q: %v", field, err)
		}
	}
}

// TestValidateRelatedValidAttributeJoin tests validation with valid attribute join field
func TestValidateRelatedValidAttributeJoin(t *testing.T) {
	rule := Rule{
		ConnectorType: "docker",
		EntityKind:    "container",
		Conditions:    []Condition{{Attribute: "name", Op: "eq", Value: "test"}},
		Related: []RelatedClause{{
			Mode:          ModeRequires,
			ConnectorType: "proxmox",
			EntityKind:    "vm",
			Join:          Join{SourceField: "attributes.tags", RelatedField: "attributes.memory_gb"},
		}},
	}
	err := Validate(rule, testCatalog())
	if err != nil {
		t.Fatalf("should accept attribute join: %v", err)
	}
}

// TestValidateRelatedCompleteValid tests a fully valid rule with related clauses
func TestValidateRelatedCompleteValid(t *testing.T) {
	rule := Rule{
		ConnectorType: "proxmox",
		EntityKind:    "vm",
		Conditions:    []Condition{{Attribute: "memory_gb", Op: "gt", Value: float64(2)}},
		Related: []RelatedClause{
			{
				Mode:          ModeRequires,
				ConnectorType: "pbs",
				EntityKind:    "backup_job",
				Join:          Join{SourceField: "external_id", RelatedField: "external_id"},
				Conditions:    []Condition{{Attribute: "last_run_days_ago", Op: "lt", Value: float64(7)}},
			},
		},
	}

	err := Validate(rule, testCatalog())
	if err != nil {
		t.Fatalf("valid rule should pass: %v", err)
	}
}

// TestEvaluateWithRelatedClausesAreANDed pins the all-clauses-must-hold
// semantics: satisfied on every clause means compliant, one violated clause of
// either mode makes the entity a violation.
func TestEvaluateWithRelatedClausesAreANDed(t *testing.T) {
	rule := Rule{
		EntityKind: "vm",
		Conditions: []Condition{{Attribute: "memory_gb", Op: "gt", Value: float64(0)}},
		Related: []RelatedClause{
			{Mode: ModeRequires, ConnectorType: "pbs", EntityKind: "backup_job", Join: Join{SourceField: "external_id", RelatedField: "external_id"}},
			{Mode: ModeForbids, ConnectorType: "docker", EntityKind: "container", Join: Join{SourceField: "name", RelatedField: "name"}},
		},
	}
	source := func(name, id string) Entity {
		return Entity{Kind: "vm", Name: name, ExternalID: id, Attributes: map[string]any{"memory_gb": 4.0}}
	}
	snapshot := Snapshot{Entities: []Entity{
		source("ok", "1"),
		source("no-backup", "2"),
		source("forbidden", "3"),
	}}
	related := RelatedEntities{
		"pbs":    {{Kind: "backup_job", ExternalID: "1"}, {Kind: "backup_job", ExternalID: "3"}},
		"docker": {{Kind: "container", Name: "forbidden"}},
	}
	got, skipped := EvaluateWithRelated(rule, snapshot, related)
	if skipped {
		t.Fatal("skipped = true, want false")
	}
	names := make([]string, len(got))
	for i, e := range got {
		names[i] = e.Name
	}
	if strings.Join(names, ",") != "no-backup,forbidden" {
		t.Fatalf("violations = %v, want [no-backup forbidden]", names)
	}
}

// TestEvaluateWithRelatedSkipsWhenAnyClauseTypeMissing: one absent type skips
// the whole rule even when the other clause's type has data.
func TestEvaluateWithRelatedSkipsWhenAnyClauseTypeMissing(t *testing.T) {
	rule := Rule{
		EntityKind: "vm",
		Conditions: []Condition{{Attribute: "memory_gb", Op: "gt", Value: float64(0)}},
		Related: []RelatedClause{
			{Mode: ModeRequires, ConnectorType: "pbs", EntityKind: "backup_job", Join: Join{SourceField: "external_id", RelatedField: "external_id"}},
			{Mode: ModeRequires, ConnectorType: "docker", EntityKind: "container", Join: Join{SourceField: "name", RelatedField: "name"}},
		},
	}
	snapshot := Snapshot{Entities: []Entity{{Kind: "vm", Name: "vm1", Attributes: map[string]any{"memory_gb": 4.0}}}}
	got, skipped := EvaluateWithRelated(rule, snapshot, RelatedEntities{"pbs": {}})
	if !skipped || got != nil {
		t.Fatalf("got %v skipped %v, want nil and skipped", got, skipped)
	}
}

func TestEvaluateDaysLeft(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	evaluator := Evaluator{Now: func() time.Time { return now }}
	tests := []struct {
		name       string
		value      any
		conditions []Condition
		want       bool
	}{
		{"inside window", now.Add(5*24*time.Hour + 3*time.Hour).Format(time.RFC3339), []Condition{{"not_after", "days_left_lt", 8}}, true},
		{"expired", now.Add(-2 * 24 * time.Hour).Format(time.RFC3339), []Condition{{"not_after", "days_left_lt", 2}}, true},
		{"negative floor", now.Add(-time.Hour).Format(time.RFC3339), []Condition{{"not_after", "days_left_lt", 0}}, true},
		{"negative floor gt", now.Add(-time.Hour).Format(time.RFC3339), []Condition{{"not_after", "days_left_gt", -1}}, false},
		{"band excludes partial second day", now.Add(36 * time.Hour).Format(time.RFC3339), []Condition{{"not_after", "days_left_lt", 8}, {"not_after", "days_left_gt", 1}}, false},
		{"band lt 2 gt 0 at 1d12h", now.Add(36 * time.Hour).Format(time.RFC3339), []Condition{{"not_after", "days_left_lt", 2}, {"not_after", "days_left_gt", 0}}, true},
		{"greater", now.Add(48 * time.Hour).Format(time.RFC3339), []Condition{{"not_after", "days_left_gt", 1}}, true},
		{"less strict boundary", now.Add(8 * 24 * time.Hour).Format(time.RFC3339), []Condition{{"not_after", "days_left_lt", 8}}, false},
		{"timezone", "2026-10-09T15:00:00+03:00", []Condition{{"not_after", "days_left_gt", 1}}, true},
		{"missing lt", nil, []Condition{{"not_after", "days_left_lt", 8}}, false},
		{"missing gt", nil, []Condition{{"not_after", "days_left_gt", -1}}, false},
		{"invalid lt", "soon", []Condition{{"not_after", "days_left_lt", 8}}, false},
		{"invalid gt", "soon", []Condition{{"not_after", "days_left_gt", -1}}, false},
		{"non string", 42, []Condition{{"not_after", "days_left_lt", 8}}, false},
		{"fractional threshold", now.Format(time.RFC3339), []Condition{{"not_after", "days_left_lt", 1.5}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attributes := map[string]any{}
			if tt.value != nil {
				attributes["not_after"] = tt.value
			}
			got := evaluator.Evaluate(Rule{EntityKind: "certificate", Conditions: tt.conditions}, Snapshot{Entities: []Entity{{Kind: "certificate", Attributes: attributes}}})
			if (len(got) == 1) != tt.want {
				t.Fatalf("matches = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateDaysLeft(t *testing.T) {
	for _, op := range []string{"days_left_lt", "days_left_gt"} {
		for _, value := range []any{8, -1, float64(2), json.Number("3"), "soon", "8", 1.5, true, nil, math.NaN(), math.Inf(1)} {
			rule := Rule{ConnectorType: "docker", EntityKind: "container", Conditions: []Condition{{"name", op, value}}}
			err := Validate(rule, testCatalog())
			valid := value == 8 || value == -1 || value == float64(2) || value == json.Number("3")
			if valid {
				if err != nil {
					t.Fatalf("%s %v: %v", op, value, err)
				}
				continue
			}
			var invalid *ValidationError
			if !errors.As(err, &invalid) || len(invalid.Fields) != 1 || invalid.Fields[0].Field != "conditions[0].value" {
				t.Fatalf("%s %v: error = %v, want condition value field error", op, value, err)
			}
		}
	}
}

func TestEvaluateDaysLeftRelatedUsesOneClockReading(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	calls := 0
	evaluator := Evaluator{Now: func() time.Time { calls++; return now.Add(time.Duration(calls-1) * 24 * time.Hour) }}
	rule := Rule{EntityKind: "certificate", Conditions: []Condition{{"not_after", "days_left_gt", 1}}, Related: []RelatedClause{{
		Mode: ModeForbids, ConnectorType: "npm", EntityKind: "certificate", Join: Join{SourceField: "name", RelatedField: "name"},
		Conditions: []Condition{{"not_after", "days_left_gt", 1}},
	}}}
	entity := Entity{Kind: "certificate", Name: "lab", Attributes: map[string]any{"not_after": now.Add(48 * time.Hour).Format(time.RFC3339)}}
	matches, skipped := evaluator.EvaluateWithRelated(rule, Snapshot{Entities: []Entity{entity}}, RelatedEntities{"npm": {entity}})
	if skipped || len(matches) != 1 || calls != 1 {
		t.Fatalf("matches = %v, skipped = %v, clock calls = %d", matches, skipped, calls)
	}
}
