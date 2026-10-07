// Package compliance evaluates user-defined rules against connector snapshots.
package compliance

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	maxConditions     = 20
	maxRegexLength    = 256
	maxRelatedClauses = 5
)

// Related clause modes.
const (
	ModeRequires = "requires"
	ModeForbids  = "forbids"
)

// AttributeFieldPrefix marks a join field that names a catalog attribute
// (attributes.mac) rather than a typed entity field (mac). The prefix keeps the
// two namespaces apart: some connectors emit an attribute called "mac" next to
// the typed MAC field.
const AttributeFieldPrefix = "attributes."

// JoinFields lists the typed entity fields a related clause may join on.
var JoinFields = []string{"external_id", "name", "ip", "hostname", "mac"}

// Rule is a compliance rule stored by the application.
type Rule struct {
	ID            string      `json:"id" yaml:"id"`
	Name          string      `json:"name" yaml:"name"`
	ConnectorType string      `json:"connectorType" yaml:"connectorType"`
	EntityKind    string      `json:"entityKind" yaml:"entityKind"`
	Conditions    []Condition `json:"conditions" yaml:"conditions"`
	// Related clauses are ANDed. They are evaluated against entities of other
	// connectors; see EvaluateWithRelated.
	Related         []RelatedClause `json:"related,omitempty" yaml:"related,omitempty"`
	Severity        string          `json:"severity" yaml:"severity"`
	Title           string          `json:"title" yaml:"title"`
	RemediationLink string          `json:"remediationLink" yaml:"remediationLink"`
	Enabled         bool            `json:"enabled" yaml:"enabled"`
}

// RelatedClause requires (or forbids) an entity of another connector type that
// is joined to the rule's source entity.
type RelatedClause struct {
	Mode          string      `json:"mode" yaml:"mode"` // ModeRequires or ModeForbids
	ConnectorType string      `json:"connectorType" yaml:"connectorType"`
	EntityKind    string      `json:"entityKind" yaml:"entityKind"`
	Join          Join        `json:"join" yaml:"join"`
	Conditions    []Condition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

// Join pairs a source-entity field with a related-entity field. Each is a
// typed field from JoinFields or AttributeFieldPrefix + a catalog attribute.
type Join struct {
	SourceField  string `json:"sourceField" yaml:"sourceField"`
	RelatedField string `json:"relatedField" yaml:"relatedField"`
}

// RelatedEntities holds the candidate related entities per connector type: the
// entities of the latest snapshot of every connector of that type, combined. A
// type with no connector snapshot is absent from the map (rules needing it are
// skipped); a type whose snapshots hold no entities maps to an empty slice.
type RelatedEntities map[string][]Entity

// Condition compares one entity attribute with a value.
type Condition struct {
	Attribute string `json:"attribute" yaml:"attribute"`
	Op        string `json:"op" yaml:"op"`
	Value     any    `json:"value" yaml:"value"`
}

// Catalog describes the attributes emitted by each connector and entity kind.
type Catalog map[string]map[string][]AttributeSpec

// AttributeSpec describes one entity attribute.
type AttributeSpec struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Snapshot is the rule-engine view of a connector snapshot.
type Snapshot struct {
	Entities []Entity `json:"entities"`
}

// Entity is a structured item in a snapshot.
type Entity struct {
	Kind       string         `json:"kind"`
	Name       string         `json:"name"`
	ExternalID string         `json:"externalId,omitempty"`
	IP         string         `json:"ip,omitempty"`
	Hostname   string         `json:"hostname,omitempty"`
	MAC        string         `json:"mac,omitempty"`
	Attributes map[string]any `json:"attributes"`
}

// FieldError names one rule field that failed validation, and why.
type FieldError struct {
	Field string
	Msg   string
}

// ValidationError reports the rule fields that failed validation. Message is
// the human-readable sentence on its own; Fields names the offending rule
// fields, so an API layer can return them per-field instead of re-deriving
// them from the message text.
type ValidationError struct {
	Message string
	Fields  []FieldError
	// Err is the underlying cause when a check has one (a regexp compile
	// failure), so errors.Is/As still reach it through the wrapper.
	Err error
}

func (e *ValidationError) Error() string { return e.Message }

func (e *ValidationError) Unwrap() error { return e.Err }

// invalidField builds a single-field ValidationError. format/args produce
// Message, so the sentence each check already returned is preserved exactly.
func invalidField(field, msg, format string, args ...any) *ValidationError {
	return &ValidationError{
		Message: fmt.Sprintf(format, args...),
		Fields:  []FieldError{{Field: field, Msg: msg}},
	}
}

// validateConditions checks a list of conditions against a catalog's attributes.
// pathPrefix is used to build the indexed path (e.g., "conditions" or "related[0].conditions").
func validateConditions(pathPrefix string, attributes []AttributeSpec, conditions []Condition) error {
	if len(conditions) > maxConditions {
		return invalidField(pathPrefix, fmt.Sprintf("must contain at most %d conditions", maxConditions),
			"at most %d conditions are allowed", maxConditions)
	}

	for i, condition := range conditions {
		spec, ok := findAttribute(attributes, condition.Attribute)
		if !ok {
			path := fmt.Sprintf("%s[%d].attribute", pathPrefix, i)
			return invalidField(path, "is not a known attribute for this entity kind",
				"unknown attribute %q", condition.Attribute)
		}
		if !validOp(spec.Type, condition.Op) {
			path := fmt.Sprintf("%s[%d].op", pathPrefix, i)
			return invalidField(path, fmt.Sprintf("is not a valid operator for %s attributes", spec.Type),
				"operator %q is not valid for %s attribute %q", condition.Op, spec.Type, condition.Attribute)
		}
		if condition.Op == "days_left_lt" || condition.Op == "days_left_gt" {
			if _, ok := wholeNumber(condition.Value); !ok {
				path := fmt.Sprintf("%s[%d].value", pathPrefix, i)
				return invalidField(path, "must be a whole number when op is days_left_lt or days_left_gt",
					"days-left value for %q must be a whole number", condition.Attribute)
			}
		}
		if condition.Op != "regex" {
			continue
		}
		pattern, ok := condition.Value.(string)
		if !ok {
			path := fmt.Sprintf("%s[%d].value", pathPrefix, i)
			return invalidField(path, "must be a string when op is regex",
				"regex value for %q must be a string", condition.Attribute)
		}
		if len(pattern) > maxRegexLength {
			path := fmt.Sprintf("%s[%d].value", pathPrefix, i)
			return invalidField(path, fmt.Sprintf("must be at most %d characters", maxRegexLength),
				"regex for %q exceeds %d characters", condition.Attribute, maxRegexLength)
		}
		if _, err := regexp.Compile(pattern); err != nil {
			path := fmt.Sprintf("%s[%d].value", pathPrefix, i)
			invalid := invalidField(path, "is not a valid regular expression",
				"invalid regex for %q: %v", condition.Attribute, err)
			invalid.Err = err
			return invalid
		}
	}
	return nil
}

// Validate checks that a rule can be evaluated with catalog. Empty conditions
// are rejected: matching every entity is not a useful compliance rule.
//
// Every rejection is a *ValidationError naming the rule field at fault; the
// per-condition ones use an indexed path (conditions[2].op) so a caller can
// point at the exact row of a multi-condition rule.
func Validate(rule Rule, catalog Catalog) error {
	kinds, ok := catalog[rule.ConnectorType]
	if !ok {
		return invalidField("connectorType", "is not a known connector type",
			"unknown connector type %q", rule.ConnectorType)
	}
	sourceAttributes, ok := kinds[rule.EntityKind]
	if !ok {
		return invalidField("entityKind", "is not a known entity kind for this connector type",
			"unknown entity kind %q for connector type %q", rule.EntityKind, rule.ConnectorType)
	}
	if len(rule.Conditions) == 0 {
		return invalidField("conditions", "must contain at least one condition",
			"at least one condition is required")
	}

	// Validate rule's own conditions
	if err := validateConditions("conditions", sourceAttributes, rule.Conditions); err != nil {
		return err
	}

	// Validate related clauses
	if len(rule.Related) > maxRelatedClauses {
		return invalidField("related", fmt.Sprintf("must contain at most %d clauses", maxRelatedClauses),
			"at most %d related clauses are allowed", maxRelatedClauses)
	}

	for i, clause := range rule.Related {
		// Validate mode
		if clause.Mode != ModeRequires && clause.Mode != ModeForbids {
			path := fmt.Sprintf("related[%d].mode", i)
			return invalidField(path, "must be 'requires' or 'forbids'",
				"related[%d].mode must be 'requires' or 'forbids', got %q", i, clause.Mode)
		}

		// Validate connector type
		relatedKinds, ok := catalog[clause.ConnectorType]
		if !ok {
			path := fmt.Sprintf("related[%d].connectorType", i)
			return invalidField(path, "is not a known connector type",
				"unknown connector type %q in related[%d]", clause.ConnectorType, i)
		}

		// Validate entity kind
		relatedAttributes, ok := relatedKinds[clause.EntityKind]
		if !ok {
			path := fmt.Sprintf("related[%d].entityKind", i)
			return invalidField(path, "is not a known entity kind for this connector type",
				"unknown entity kind %q for connector type %q in related[%d]", clause.EntityKind, clause.ConnectorType, i)
		}

		// Validate clause conditions against related kind's attributes
		if len(clause.Conditions) > 0 {
			if err := validateConditions(fmt.Sprintf("related[%d].conditions", i), relatedAttributes, clause.Conditions); err != nil {
				return err
			}
		}

		// Validate join source field against source kind's attributes
		if !validJoinField(clause.Join.SourceField, sourceAttributes) {
			path := fmt.Sprintf("related[%d].join.sourceField", i)
			return invalidField(path, "is not a valid field for the source entity kind",
				"invalid source field %q in related[%d].join", clause.Join.SourceField, i)
		}

		// Validate join related field against related kind's attributes
		if !validJoinField(clause.Join.RelatedField, relatedAttributes) {
			path := fmt.Sprintf("related[%d].join.relatedField", i)
			return invalidField(path, "is not a valid field for the related entity kind",
				"invalid related field %q in related[%d].join", clause.Join.RelatedField, i)
		}
	}

	return nil
}

// Evaluator evaluates rules using Now as its clock. A nil Now uses time.Now.
type Evaluator struct {
	Now func() time.Time
}

func (e Evaluator) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

// Evaluate returns matching entities in snapshot order. A missing attribute is
// false for every operator except neq, not_contains and exists. exists uses a boolean value:
// true requires presence and false requires absence.
func Evaluate(rule Rule, snapshot Snapshot) []Entity {
	return (Evaluator{}).Evaluate(rule, snapshot)
}

// Evaluate returns matching entities using the evaluator's clock.
func (e Evaluator) Evaluate(rule Rule, snapshot Snapshot) []Entity {
	return evaluate(rule, snapshot, e.now())
}

func evaluate(rule Rule, snapshot Snapshot, now time.Time) []Entity {
	matches := make([]Entity, 0)
	for _, entity := range snapshot.Entities {
		if entity.Kind != rule.EntityKind || !matchesAll(entity, rule.Conditions, now) {
			continue
		}
		matches = append(matches, entity)
	}
	return matches
}

// fieldValue extracts a typed or attribute field from an entity and returns its
// string value. Typed names (external_id, name, ip, hostname, mac) map directly to
// Entity fields. AttributeFieldPrefix+"name" maps to Attributes[name], converted to
// string as follows: string as is; numbers as strconv (whole numbers as int, else float);
// bool as "true"/"false"; anything else (nil, arrays, maps) is not usable.
// Returns ("", false) if the field is not usable or is empty after trimming.
func fieldValue(e Entity, field string) (string, bool) {
	var value any

	switch field {
	case "external_id":
		value = e.ExternalID
	case "name":
		value = e.Name
	case "ip":
		value = e.IP
	case "hostname":
		value = e.Hostname
	case "mac":
		value = e.MAC
	default:
		if strings.HasPrefix(field, AttributeFieldPrefix) {
			attrName := field[len(AttributeFieldPrefix):]
			var ok bool
			value, ok = e.Attributes[attrName]
			if !ok {
				return "", false
			}
		} else {
			return "", false
		}
	}

	// Convert value to string
	var s string
	switch v := value.(type) {
	case string:
		s = v
	case bool:
		s = fmt.Sprintf("%v", v)
	default:
		n, ok := number(v)
		if !ok {
			return "", false
		}
		// Whole numbers as int, else float
		if n == float64(int64(n)) {
			s = strconv.FormatInt(int64(n), 10)
		} else {
			s = strconv.FormatFloat(n, 'f', -1, 64)
		}
	}

	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return s, true
}

// validJoinField checks whether a field (either a typed field from JoinFields or
// AttributeFieldPrefix+attributeName) is valid for the given entity's attribute catalog.
func validJoinField(field string, attributes []AttributeSpec) bool {
	// Check if it's a typed field
	for _, typedField := range JoinFields {
		if field == typedField {
			return true
		}
	}

	// Check if it's an attribute field
	if strings.HasPrefix(field, AttributeFieldPrefix) {
		attrName := field[len(AttributeFieldPrefix):]
		_, found := findAttribute(attributes, attrName)
		return found
	}

	return false
}

// EvaluateWithRelated is Evaluate for rules with Related clauses. The rule's own
// conditions select source entities; a selected entity is returned when any
// requires clause finds no related entity or any forbids clause finds one.
// skipped is true, and matches nil, when a clause's connector type is absent
// from related. A rule without clauses returns exactly Evaluate's result.
func EvaluateWithRelated(rule Rule, snapshot Snapshot, related RelatedEntities) (matches []Entity, skipped bool) {
	return (Evaluator{}).EvaluateWithRelated(rule, snapshot, related)
}

// EvaluateWithRelated evaluates source and related conditions at the same time.
func (e Evaluator) EvaluateWithRelated(rule Rule, snapshot Snapshot, related RelatedEntities) (matches []Entity, skipped bool) {
	now := e.now()
	if len(rule.Related) == 0 {
		return evaluate(rule, snapshot, now), false
	}

	// Check if any clause refers to a missing connector type; if so, skip the rule.
	for _, clause := range rule.Related {
		if _, ok := related[clause.ConnectorType]; !ok {
			return nil, true
		}
	}

	// Get source entities by filtering on own conditions and kind.
	sources := evaluate(rule, snapshot, now)

	// For each clause, build a map of (lowercased trimmed) join values that satisfy
	// kind + conditions. We build the map once per clause to avoid O(source*related).
	clauseMaps := make([]map[string]bool, len(rule.Related))
	for i, clause := range rule.Related {
		candidateMap := make(map[string]bool)
		for _, relEntity := range related[clause.ConnectorType] {
			if relEntity.Kind != clause.EntityKind || !matchesAll(relEntity, clause.Conditions, now) {
				continue
			}
			joinVal, ok := fieldValue(relEntity, clause.Join.RelatedField)
			if !ok {
				continue
			}
			candidateMap[strings.ToLower(joinVal)] = true
		}
		clauseMaps[i] = candidateMap
	}

	// Filter sources: keep only those where ANY clause is violated.
	result := make([]Entity, 0)
	for _, source := range sources {
		// Check each clause
		clauseViolated := false
		for i, clause := range rule.Related {
			sourceFieldVal, _ := fieldValue(source, clause.Join.SourceField)
			sourceFieldLower := strings.ToLower(sourceFieldVal)

			// If source join field is empty, it can never match: requires is violated, forbids is satisfied.
			if sourceFieldVal == "" {
				if clause.Mode == ModeRequires {
					clauseViolated = true
					break
				}
				continue
			}

			found := clauseMaps[i][sourceFieldLower]
			if (clause.Mode == ModeRequires && !found) || (clause.Mode == ModeForbids && found) {
				clauseViolated = true
				break
			}
		}

		if clauseViolated {
			result = append(result, source)
		}
	}

	return result, false
}

func findAttribute(attributes []AttributeSpec, name string) (AttributeSpec, bool) {
	for _, attribute := range attributes {
		if attribute.Name == name {
			return attribute, true
		}
	}
	return AttributeSpec{}, false
}

func validOp(attributeType, op string) bool {
	switch attributeType {
	case "string":
		return op == "eq" || op == "neq" || op == "contains" || op == "not_contains" || op == "regex" || op == "exists" || op == "days_left_lt" || op == "days_left_gt"
	case "string_array":
		return op == "eq" || op == "neq" || op == "contains" || op == "not_contains" || op == "exists"
	case "number":
		return op == "eq" || op == "neq" || op == "gt" || op == "lt" || op == "exists"
	case "boolean":
		return op == "eq" || op == "neq" || op == "exists"
	default:
		return false
	}
}

// MatchesCondition reports whether entity satisfies one condition, with the
// comparison rules rule evaluation uses. A condition on a missing attribute is
// false for every operator except neq, not_contains and exists.
func MatchesCondition(entity Entity, condition Condition) bool {
	return (Evaluator{}).MatchesCondition(entity, condition)
}

// MatchesCondition is MatchesCondition using the evaluator's clock.
func (e Evaluator) MatchesCondition(entity Entity, condition Condition) bool {
	return matchesAll(entity, []Condition{condition}, e.now())
}

func matchesAll(entity Entity, conditions []Condition, now time.Time) bool {
	for _, condition := range conditions {
		value, present := entity.Attributes[condition.Attribute]
		if !matches(value, present, condition, now) {
			return false
		}
	}
	return true
}

func matches(value any, present bool, condition Condition, now time.Time) bool {
	if condition.Op == "exists" {
		want, ok := condition.Value.(bool)
		return ok && present == want
	}
	if condition.Op == "not_contains" {
		// Like neq, absence satisfies it; a needle that is not a string never
		// matches, mirroring contains.
		needle, ok := condition.Value.(string)
		return ok && (!present || value == nil || !contains(value, needle))
	}
	if !present {
		return condition.Op == "neq"
	}
	if value == nil {
		return false
	}

	switch condition.Op {
	case "eq":
		return equal(value, condition.Value)
	case "neq":
		return !equal(value, condition.Value)
	case "contains":
		return contains(value, condition.Value)
	case "regex":
		pattern, ok := condition.Value.(string)
		if !ok {
			return false
		}
		s, ok := value.(string)
		if !ok {
			return false
		}
		re, err := regexp.Compile(pattern)
		return err == nil && re.MatchString(s)
	case "days_left_lt", "days_left_gt":
		timestamp, ok := value.(string)
		if !ok {
			return false
		}
		expiry, err := time.Parse(time.RFC3339, timestamp)
		threshold, ok := wholeNumber(condition.Value)
		if err != nil || !ok {
			return false
		}
		days := math.Floor(expiry.Sub(now).Hours() / 24)
		return condition.Op == "days_left_lt" && days < threshold || condition.Op == "days_left_gt" && days > threshold
	case "gt", "lt":
		left, leftOK := number(value)
		right, rightOK := number(condition.Value)
		if !leftOK || !rightOK {
			return false
		}
		return condition.Op == "gt" && left > right || condition.Op == "lt" && left < right
	default:
		return false
	}
}

func equal(left, right any) bool {
	leftNumber, leftOK := number(left)
	rightNumber, rightOK := number(right)
	if leftOK || rightOK {
		return leftOK && rightOK && leftNumber == rightNumber
	}
	leftStrings := stringSlice(left)
	rightStrings := stringSlice(right)
	if leftStrings != nil || rightStrings != nil {
		return leftStrings != nil && rightStrings != nil && reflect.DeepEqual(leftStrings, rightStrings)
	}
	return reflect.DeepEqual(left, right)
}

func contains(value, want any) bool {
	needle, ok := want.(string)
	if !ok {
		return false
	}
	switch actual := value.(type) {
	case string:
		return strings.Contains(actual, needle)
	case []string:
		for _, item := range actual {
			if item == needle {
				return true
			}
		}
	case []any:
		for _, item := range actual {
			if item == needle {
				return true
			}
		}
	}
	return false
}

func stringSlice(value any) []string {
	switch items := value.(type) {
	case []string:
		return items
	case []any:
		result := make([]string, 0, len(items))
		for _, item := range items {
			text, ok := item.(string)
			if !ok {
				return nil
			}
			result = append(result, text)
		}
		return result
	default:
		return nil
	}
}

func number(value any) (float64, bool) {
	switch n := value.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		parsed, err := n.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func wholeNumber(value any) (float64, bool) {
	n, ok := number(value)
	return n, ok && !math.IsInf(n, 0) && !math.IsNaN(n) && math.Trunc(n) == n
}
