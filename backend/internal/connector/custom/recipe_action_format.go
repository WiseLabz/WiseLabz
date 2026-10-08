package custom

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

func validateRecipeActions(recipe *Recipe) []RecipeIssue {
	var issues []RecipeIssue
	issues = append(issues, validateActionSet(recipe.Actions, nil, "actions")...)
	declaredKinds := make(map[string]bool)
	for i := range recipe.Endpoints {
		entity := &recipe.Endpoints[i].Entity
		if len(entity.Actions) == 0 {
			continue
		}
		base := fmt.Sprintf("endpoints[%d].entity.actions", i)
		if declaredKinds[entity.Kind] {
			issues = append(issues, RecipeIssue{Location: base, Message: "actions for this entity kind are already declared on another endpoint"})
		} else {
			declaredKinds[entity.Kind] = true
		}
		issues = append(issues, validateActionSet(entity.Actions, entity, base)...)
	}
	return issues
}

func validateActionSet(actions map[string]RecipeAction, entity *RecipeEntity, base string) []RecipeIssue {
	var issues []RecipeIssue
	if len(actions) > 10 {
		issues = append(issues, RecipeIssue{Location: base, Message: "must contain at most 10 actions"})
	}
	names := make([]string, 0, len(actions))
	for name := range actions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		location := base + "." + name
		if !validActionName(name) {
			issues = append(issues, RecipeIssue{Location: location, Message: "must start with a lowercase letter and contain only lowercase letters, digits, underscores, and hyphens (at most 32 characters)"})
		}
		action := actions[name]
		if action.Method != "POST" && action.Method != "PUT" && action.Method != "PATCH" && action.Method != "DELETE" {
			issues = append(issues, RecipeIssue{Location: location + ".method", Message: "must be POST, PUT, PATCH, or DELETE"})
		}
		if err := validateEndpointPath(action.Path); err != nil {
			issues = append(issues, RecipeIssue{Location: location + ".path", Message: "must be a relative path: " + err.Error()})
		}
		queryKeys := make([]string, 0, len(action.Query))
		for key := range action.Query {
			queryKeys = append(queryKeys, key)
		}
		sort.Strings(queryKeys)
		for _, key := range queryKeys {
			value := action.Query[key]
			issues = append(issues, validateActionTemplate(value, location+".query."+key, entity)...)
		}
		headerNames := make([]string, 0, len(action.Headers))
		for name := range action.Headers {
			headerNames = append(headerNames, name)
		}
		sort.Strings(headerNames)
		for _, name := range headerNames {
			value := action.Headers[name]
			headerLocation := location + ".headers." + name
			if !validHeaderName(name) {
				issues = append(issues, RecipeIssue{Location: headerLocation, Message: "must be a valid HTTP header name"})
			}
			if !validHeaderValue(value) {
				issues = append(issues, RecipeIssue{Location: headerLocation, Message: "contains an invalid HTTP header control byte"})
			}
		}
		if action.HasBody || action.Body != nil {
			if err := validateJSONValue(action.Body, 0); err != nil {
				issues = append(issues, RecipeIssue{Location: location + ".body", Message: "must be a static JSON value: " + err.Error()})
			} else if _, err := json.Marshal(action.Body); err != nil {
				issues = append(issues, RecipeIssue{Location: location + ".body", Message: "must be a static JSON value: " + err.Error()})
			} else {
				issues = append(issues, validateActionBodyTemplates(action.Body, location+".body", entity)...)
			}
		}
		if utf8.RuneCountInString(action.Label) > 60 {
			issues = append(issues, RecipeIssue{Location: location + ".label", Message: "must be at most 60 characters"})
		}
		if utf8.RuneCountInString(action.Description) > 300 {
			issues = append(issues, RecipeIssue{Location: location + ".description", Message: "must be at most 300 characters"})
		}
		if action.DowntimeSeconds != nil && (*action.DowntimeSeconds < 0 || *action.DowntimeSeconds > 3600) {
			issues = append(issues, RecipeIssue{Location: location + ".downtime_seconds", Message: "must be between 0 and 3600 seconds"})
		}
		issues = append(issues, validateActionTemplate(action.Path, location+".path", entity)...)
	}
	return issues
}

func validActionName(name string) bool {
	if len(name) < 1 || len(name) > 32 || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for i := 1; i < len(name); i++ {
		c := name[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

type actionPlaceholder struct {
	name string
}

func parseActionPlaceholders(value string) ([]actionPlaceholder, []string) {
	var placeholders []actionPlaceholder
	var issues []string
	for i := 0; i < len(value); {
		switch value[i] {
		case '{':
			if i+1 < len(value) && value[i+1] == '{' {
				i += 2
				continue
			}
			end := strings.IndexByte(value[i+1:], '}')
			if end < 0 {
				issues = append(issues, "has an unclosed placeholder")
				return placeholders, issues
			}
			name := value[i+1 : i+1+end]
			if name != "external_id" && !strings.HasPrefix(name, "attr.") {
				issues = append(issues, fmt.Sprintf("contains unsupported placeholder {%s}", name))
			} else if strings.HasPrefix(name, "attr.") && len(name) == len("attr.") {
				issues = append(issues, "attribute placeholder must name a mapped attribute")
			} else {
				placeholders = append(placeholders, actionPlaceholder{name: name})
			}
			i += end + 2
		case '}':
			if i+1 < len(value) && value[i+1] == '}' {
				i += 2
				continue
			}
			issues = append(issues, "has an unmatched closing brace")
			i++
		default:
			i++
		}
	}
	return placeholders, issues
}

func validateActionTemplate(value, location string, entity *RecipeEntity) []RecipeIssue {
	placeholders, parseIssues := parseActionPlaceholders(value)
	var issues []RecipeIssue
	for _, message := range parseIssues {
		issues = append(issues, RecipeIssue{Location: location, Message: message})
	}
	if len(placeholders) == 0 {
		return issues
	}
	if entity == nil {
		return append(issues, RecipeIssue{Location: location, Message: "service actions cannot contain placeholders"})
	}
	for _, placeholder := range placeholders {
		if placeholder.name == "external_id" {
			continue
		}
		name := strings.TrimPrefix(placeholder.name, "attr.")
		if _, ok := entity.Attributes[name]; !ok {
			issues = append(issues, RecipeIssue{Location: location, Message: fmt.Sprintf("placeholder {attr.%s} does not name an attribute mapped by this endpoint", name)})
		}
	}
	return issues
}

func validateActionBodyTemplates(value any, location string, entity *RecipeEntity) []RecipeIssue {
	var issues []RecipeIssue
	switch value := value.(type) {
	case string:
		return validateActionTemplate(value, location, entity)
	case []any:
		for i, item := range value {
			issues = append(issues, validateActionBodyTemplates(item, fmt.Sprintf("%s[%d]", location, i), entity)...)
		}
	case map[string]any:
		for key, item := range value {
			issues = append(issues, validateActionBodyTemplates(item, location+"."+key, entity)...)
		}
	case map[any]any:
		for key, item := range value {
			if name, ok := key.(string); ok {
				issues = append(issues, validateActionBodyTemplates(item, location+"."+name, entity)...)
			}
		}
	}
	return issues
}

// ActionDowntime returns the declared estimate or the recipe format default.
func ActionDowntime(name string, action RecipeAction) int {
	if action.DowntimeSeconds != nil {
		return *action.DowntimeSeconds
	}
	if separator := strings.LastIndexByte(name, '.'); separator >= 0 {
		name = name[separator+1:]
	}
	if name == "restart" {
		return 30
	}
	return 0
}

// CanonicalActions returns the declared actions from config's recipe. Service
// actions use service.<name>; entity actions use entity.<kind>.<name>.
func CanonicalActions(config map[string]any) (map[string]RecipeAction, error) {
	actions := make(map[string]RecipeAction)
	rawValue, exists := config["recipe"]
	if !exists || rawValue == nil || rawValue == "" {
		return actions, nil
	}
	raw, ok := rawValue.(string)
	if !ok {
		return nil, errors.New("recipe must be a YAML string")
	}
	if strings.TrimSpace(raw) == "" {
		return actions, nil
	}
	recipe, err := ParseRecipe(raw)
	if err != nil {
		return nil, err
	}
	for name, action := range recipe.Actions {
		actions["service."+name] = canonicalAction(name, action)
	}
	for _, endpoint := range recipe.Endpoints {
		for name, action := range endpoint.Entity.Actions {
			actions["entity."+endpoint.Entity.Kind+"."+name] = canonicalAction(name, action)
		}
	}
	return actions, nil
}

func canonicalAction(name string, action RecipeAction) RecipeAction {
	if action.Label == "" {
		action.Label = name
	}
	if len(action.Query) == 0 {
		action.Query = nil
	}
	if len(action.Headers) == 0 {
		action.Headers = nil
	}
	action.HasBody = action.HasBody || action.Body != nil
	downtime := ActionDowntime(name, action)
	action.DowntimeSeconds = &downtime
	return action
}

// ActionDiff lists action names that were added, changed, or removed.
type ActionDiff struct {
	Added   []string
	Changed []string
	Removed []string
}

// DiffActions compares action meanings and returns sorted qualified names.
func DiffActions(before, after map[string]RecipeAction) ActionDiff {
	var diff ActionDiff
	for name, action := range after {
		previous, exists := before[name]
		if !exists {
			diff.Added = append(diff.Added, name)
		} else if !sameAction(name, previous, action) {
			diff.Changed = append(diff.Changed, name)
		}
	}
	for name := range before {
		if _, exists := after[name]; !exists {
			diff.Removed = append(diff.Removed, name)
		}
	}
	sort.Strings(diff.Added)
	sort.Strings(diff.Changed)
	sort.Strings(diff.Removed)
	return diff
}

func sameAction(name string, left, right RecipeAction) bool {
	left, right = canonicalAction(name, left), canonicalAction(name, right)
	leftBody, leftErr := json.Marshal(left.Body)
	rightBody, rightErr := json.Marshal(right.Body)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return left.Method == right.Method && left.Path == right.Path &&
		mapsEqual(left.Query, right.Query) && mapsEqual(left.Headers, right.Headers) &&
		string(leftBody) == string(rightBody) && left.HasBody == right.HasBody &&
		left.Label == right.Label && left.Description == right.Description &&
		ActionDowntime(name, left) == ActionDowntime(name, right)
}

func mapsEqual(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		other, exists := right[key]
		if !exists || other != value {
			return false
		}
	}
	return true
}
