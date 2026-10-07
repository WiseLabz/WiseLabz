package custom

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/tidwall/gjson"
)

type recipeEndpointResult struct {
	Entities     []connector.SnapshotEntity
	Dependencies []connector.ServiceDependency
	Items        int
	Skipped      int
}

// recipeBudget bounds the data one endpoint maps. A recipe selects and repeats
// fields, so without a bound it could multiply a response many times over.
// used may start above zero when earlier pages or endpoints already spent part
// of a shared limit; the error always reports the limit itself.
type recipeBudget struct {
	endpoint string
	limit    int
	used     int
}

func (b *recipeBudget) add(n int) error {
	b.used += n
	if b.used > b.limit {
		return fmt.Errorf("endpoint %q maps more than %d bytes of data", b.endpoint, b.limit)
	}
	return nil
}

// mapEndpoint maps one endpoint response atomically. The caller shares seen
// across endpoints so duplicate identifiers are detected across the sync. The
// mapped data is bounded by the response size limit.
func mapEndpoint(recipe *Recipe, endpoint RecipeEndpoint, body []byte, seen map[string]struct{}) (recipeEndpointResult, error) {
	return mapEndpointLimited(recipe, endpoint, body, seen, connector.MaxResponseBytes)
}

func mapEndpointLimited(recipe *Recipe, endpoint RecipeEndpoint, body []byte, seen map[string]struct{}, limit int) (recipeEndpointResult, error) {
	return mapEndpointBudgeted(recipe, endpoint, body, seen, limit, 0)
}

// mapEndpointBudgeted is mapEndpointLimited with used bytes already charged
// against limit.
func mapEndpointBudgeted(recipe *Recipe, endpoint RecipeEndpoint, body []byte, seen map[string]struct{}, limit, used int) (recipeEndpointResult, error) {
	if recipe == nil {
		return recipeEndpointResult{}, fmt.Errorf("endpoint %q: recipe is required", endpoint.Name)
	}
	if !gjson.ValidBytes(body) {
		return recipeEndpointResult{}, fmt.Errorf("endpoint %q: response is not valid JSON", endpoint.Name)
	}
	items := jsonPathValue(body, endpoint.Items)
	if !items.Exists() || !items.IsArray() {
		return recipeEndpointResult{}, fmt.Errorf("endpoint %q items path %q did not select a list", endpoint.Name, endpoint.Items)
	}

	known := make(map[string]struct{}, len(seen))
	for key := range seen {
		known[key] = struct{}{}
	}
	budget := &recipeBudget{endpoint: endpoint.Name, limit: limit, used: used}
	result := recipeEndpointResult{Items: len(items.Array())}
	for _, item := range items.Array() {
		entity, externalID, skip, err := mapRecipeEntity(endpoint, []byte(item.Raw), budget)
		if err != nil {
			return recipeEndpointResult{}, err
		}
		if skip {
			result.Skipped++
			continue
		}

		key := endpoint.Entity.Kind + "\x00" + externalID
		if _, ok := known[key]; ok {
			return recipeEndpointResult{}, fmt.Errorf("endpoint %q has duplicate identifier %q for kind %q", endpoint.Name, externalID, endpoint.Entity.Kind)
		}
		known[key] = struct{}{}
		result.Entities = append(result.Entities, entity)
	}

	dependencies := make(map[string]connector.ServiceDependency)
	if err := addRecipeDependencies(dependencies, recipe.Dependencies, body, endpoint.Name); err != nil {
		return recipeEndpointResult{}, err
	}
	if err := addRecipeDependencies(dependencies, endpoint.Dependencies, body, endpoint.Name); err != nil {
		return recipeEndpointResult{}, err
	}
	dependencyKeys := make([]string, 0, len(dependencies))
	for key := range dependencies {
		dependencyKeys = append(dependencyKeys, key)
	}
	sort.Strings(dependencyKeys)
	for _, key := range dependencyKeys {
		result.Dependencies = append(result.Dependencies, dependencies[key])
	}

	if seen != nil {
		for key := range known {
			if _, existed := seen[key]; !existed {
				seen[key] = struct{}{}
			}
		}
	}
	return result, nil
}

func mapRecipeEntity(endpoint RecipeEndpoint, item []byte, budget *recipeBudget) (connector.SnapshotEntity, string, bool, error) {
	entityConfig := endpoint.Entity
	externalID, exists, err := recipePathString(item, entityConfig.ExternalID)
	if err != nil {
		return connector.SnapshotEntity{}, "", false, fmt.Errorf("endpoint %q entity external_id path: %w", endpoint.Name, err)
	}
	if !exists || strings.TrimSpace(externalID) == "" {
		return connector.SnapshotEntity{}, "", true, nil
	}

	name, _, err := recipePathString(item, entityConfig.Name)
	if err != nil {
		return connector.SnapshotEntity{}, "", false, fmt.Errorf("endpoint %q entity name path: %w", endpoint.Name, err)
	}
	entity := connector.SnapshotEntity{
		Kind:       entityConfig.Kind,
		Name:       name,
		ExternalID: externalID,
	}
	for _, field := range []struct {
		name string
		path string
		dest *string
	}{
		{name: "ip", path: entityConfig.IP, dest: &entity.IP},
		{name: "hostname", path: entityConfig.Hostname, dest: &entity.Hostname},
		{name: "mac", path: entityConfig.MAC, dest: &entity.MAC},
	} {
		if field.path == "" {
			continue
		}
		value, found, err := recipePathString(item, field.path)
		if err != nil {
			return connector.SnapshotEntity{}, "", false, fmt.Errorf("endpoint %q entity %s path: %w", endpoint.Name, field.name, err)
		}
		if found {
			*field.dest = value
		}
	}
	if entityConfig.Aliases != "" {
		aliases, found, err := recipePathStrings(item, entityConfig.Aliases)
		if err != nil {
			return connector.SnapshotEntity{}, "", false, fmt.Errorf("endpoint %q entity aliases path: %w", endpoint.Name, err)
		}
		if found {
			entity.Aliases = aliases
		}
	}

	size := len(entity.Kind) + len(entity.Name) + len(entity.ExternalID) + len(entity.IP) + len(entity.Hostname) + len(entity.MAC)
	for _, alias := range entity.Aliases {
		size += len(alias)
	}
	if err := budget.add(size); err != nil {
		return connector.SnapshotEntity{}, "", false, err
	}

	if len(entityConfig.Attributes) > 0 {
		entity.Attributes = make(map[string]any, len(entityConfig.Attributes))
		for _, name := range sortedRecipeAttributeNames(entityConfig.Attributes) {
			attribute, include, err := mapRecipeAttribute(item, entityConfig.Attributes[name], budget.limit)
			if err != nil {
				return connector.SnapshotEntity{}, "", false, fmt.Errorf("endpoint %q attribute %q: %w", endpoint.Name, name, err)
			}
			if include {
				if err := budget.add(len(name) + recipeValueSize(attribute)); err != nil {
					return connector.SnapshotEntity{}, "", false, err
				}
				entity.Attributes[name] = attribute
			}
		}
		if len(entity.Attributes) == 0 {
			entity.Attributes = nil
		}
	}
	return entity, externalID, false, nil
}

func sortedRecipeAttributeNames(attributes map[string]RecipeAttribute) []string {
	names := make([]string, 0, len(attributes))
	for name := range attributes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func mapRecipeAttribute(item []byte, attribute RecipeAttribute, limit int) (any, bool, error) {
	hasPath := attribute.HasPath || attribute.Path != ""
	hasConst := attribute.HasConst || attribute.Const != nil
	hasTemplate := attribute.HasTemplate || attribute.Template != ""
	if boolCount(hasPath, hasConst, hasTemplate) != 1 {
		return nil, false, errors.New("must define exactly one of path, const, or template")
	}

	var value any
	if hasPath {
		resolved, err := recipePathValue(item, attribute.Path)
		if err != nil {
			return nil, false, err
		}
		if !resolved.found {
			return nil, false, nil
		}
		value = resolved.value
	} else if hasConst {
		value = attribute.Const
	} else {
		template, found, err := expandRecipeTemplate(item, attribute.Template, limit)
		if err != nil {
			return nil, false, err
		}
		if !found {
			return nil, false, nil
		}
		value = template
	}

	if attribute.Map != nil {
		key, err := recipeMapKey(value)
		if err != nil {
			return nil, false, err
		}
		if mapped, ok := attribute.Map[key]; ok {
			value = mapped
		} else if attribute.HasDefault || attribute.Default != nil {
			value = attribute.Default
		}
	}

	converted, err := convertRecipeAttribute(value, attribute.Type)
	if err != nil {
		return nil, false, err
	}
	return converted, true, nil
}

// recipeValueSize approximates the bytes an attribute value adds to a snapshot.
func recipeValueSize(value any) int {
	switch v := value.(type) {
	case string:
		return len(v)
	case json.Number:
		return len(v)
	case []string:
		size := 0
		for _, item := range v {
			size += len(item)
		}
		return size
	default:
		return 8
	}
}

func boolCount(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}

type recipeResolvedValue struct {
	value any
	found bool
}

func recipePathValue(data []byte, path string) (recipeResolvedValue, error) {
	if strings.TrimSpace(path) == "" {
		return recipeResolvedValue{}, errors.New("path expression is empty")
	}
	result := jsonPathValue(data, path)
	if !result.Exists() || result.Type == gjson.Null {
		return recipeResolvedValue{}, nil
	}
	value, err := recipeGJSONValue(result)
	if err != nil {
		return recipeResolvedValue{}, err
	}
	return recipeResolvedValue{value: value, found: true}, nil
}

func recipeGJSONValue(result gjson.Result) (any, error) {
	switch result.Type {
	case gjson.String:
		return result.Str, nil
	case gjson.Number:
		return json.Number(result.Raw), nil
	case gjson.True:
		return true, nil
	case gjson.False:
		return false, nil
	case gjson.JSON:
		decoder := json.NewDecoder(bytes.NewBufferString(result.Raw))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("decode JSON value: %w", err)
		}
		return value, nil
	default:
		return nil, errors.New("path did not resolve to a value")
	}
}

func recipePathString(data []byte, path string) (string, bool, error) {
	if strings.TrimSpace(path) == "" {
		return "", false, nil
	}
	result := jsonPathValue(data, path)
	if !result.Exists() || result.Type == gjson.Null {
		return "", false, nil
	}
	value, err := recipeGJSONString(result)
	return value, err == nil, err
}

func recipeGJSONString(result gjson.Result) (string, error) {
	switch result.Type {
	case gjson.String:
		return result.Str, nil
	case gjson.Number:
		return result.Raw, nil
	case gjson.True:
		return "true", nil
	case gjson.False:
		return "false", nil
	default:
		return "", errors.New("path must resolve to a scalar value")
	}
}

func recipePathStrings(data []byte, path string) ([]string, bool, error) {
	result := jsonPathValue(data, path)
	if !result.Exists() || result.Type == gjson.Null {
		return nil, false, nil
	}
	values := result.Array()
	if !result.IsArray() {
		values = []gjson.Result{result}
	}
	stringsOut := make([]string, 0, len(values))
	for _, value := range values {
		if value.Type == gjson.Null {
			continue
		}
		text, err := recipeGJSONString(value)
		if err != nil {
			return nil, false, err
		}
		if text != "" {
			stringsOut = append(stringsOut, text)
		}
	}
	return stringsOut, true, nil
}

func expandRecipeTemplate(data []byte, template string, limit int) (string, bool, error) {
	var output strings.Builder
	for i := 0; i < len(template); {
		switch template[i] {
		case '{':
			if i+1 < len(template) && template[i+1] == '{' {
				output.WriteByte('{')
				i += 2
				continue
			}
			end := strings.IndexByte(template[i+1:], '}')
			if end < 0 {
				return "", false, errors.New("template has an unclosed placeholder")
			}
			path := template[i+1 : i+1+end]
			value, found, err := recipePathString(data, path)
			if err != nil {
				return "", false, fmt.Errorf("placeholder %q: %w", path, err)
			}
			if !found {
				return "", false, nil
			}
			if output.Len()+len(value) > limit {
				return "", false, errors.New("template output is too large")
			}
			output.WriteString(value)
			i += end + 2
		case '}':
			if i+1 < len(template) && template[i+1] == '}' {
				output.WriteByte('}')
				i += 2
				continue
			}
			return "", false, errors.New("template has an unmatched closing brace")
		default:
			output.WriteByte(template[i])
			i++
		}
	}
	return output.String(), true, nil
}

func recipeMapKey(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "null", nil
	case string:
		return v, nil
	case json.Number:
		return string(v), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("format map key: %w", err)
		}
		return string(encoded), nil
	}
}

func convertRecipeAttribute(value any, typ string) (any, error) {
	switch typ {
	case "":
		return jsonSafeRecipeValue(value)
	case "string":
		return recipeValueString(value)
	case "number":
		return recipeValueNumber(value)
	case "bool", "boolean":
		return recipeValueBool(value)
	case "list", "string_array":
		return recipeStringList(value)
	default:
		return nil, fmt.Errorf("unsupported attribute type %q", typ)
	}
}

func recipeValueString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case json.Number:
		return string(v), nil
	case bool:
		return strconv.FormatBool(v), nil
	case int:
		return strconv.Itoa(v), nil
	case int8, int16, int32, int64:
		return fmt.Sprint(v), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(v), nil
	case float32:
		return strconv.FormatFloat(float64(v), 'g', -1, 32), nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "", errors.New("value is not a finite number")
		}
		return strconv.FormatFloat(v, 'g', -1, 64), nil
	default:
		return "", fmt.Errorf("cannot convert %T to string", value)
	}
}

func recipeValueNumber(value any) (json.Number, error) {
	text, err := recipeValueString(value)
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(text)
	if text == "" || !json.Valid([]byte(text)) {
		return "", errors.New("value is not a number")
	}
	var number json.Number
	if err := json.Unmarshal([]byte(text), &number); err != nil {
		return "", errors.New("value is not a number")
	}
	if err := validateRecipeNumber(number); err != nil {
		return "", err
	}
	return number, nil
}

func validateRecipeNumber(value json.Number) error {
	number, err := value.Float64()
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return errors.New("number is outside the supported range")
	}
	return nil
}

func recipeValueBool(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		if v == "true" {
			return true, nil
		}
		if v == "false" {
			return false, nil
		}
		return false, errors.New("value is not a boolean")
	default:
		return false, fmt.Errorf("cannot convert %T to boolean", value)
	}
}

func recipeStringList(value any) ([]string, error) {
	switch values := value.(type) {
	case []string:
		return append([]string(nil), values...), nil
	case []any:
		result := make([]string, len(values))
		for i, value := range values {
			text, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("list item %d is %T, not a string", i, value)
			}
			result[i] = text
		}
		return result, nil
	default:
		return nil, fmt.Errorf("cannot convert %T to a list of strings", value)
	}
}

func jsonSafeRecipeValue(value any) (any, error) {
	switch v := value.(type) {
	case nil, string, bool:
		return v, nil
	case json.Number:
		if err := validateRecipeNumber(v); err != nil {
			return nil, err
		}
		return v, nil
	case int:
		return json.Number(strconv.Itoa(v)), nil
	case int8, int16, int32, int64:
		return json.Number(fmt.Sprint(v)), nil
	case uint, uint8, uint16, uint32, uint64:
		return json.Number(fmt.Sprint(v)), nil
	case float32:
		f := float64(v)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, errors.New("value is not a finite number")
		}
		return json.Number(strconv.FormatFloat(f, 'g', -1, 32)), nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, errors.New("value is not a finite number")
		}
		return json.Number(strconv.FormatFloat(v, 'g', -1, 64)), nil
	case []string:
		return append([]string(nil), v...), nil
	case []any:
		return recipeStringList(v)
	default:
		return nil, fmt.Errorf("%T is not a JSON-safe attribute value", value)
	}
}

func addRecipeDependencies(target map[string]connector.ServiceDependency, definitions []RecipeDependency, body []byte, endpointName string) error {
	for i, definition := range definitions {
		location := fmt.Sprintf("endpoint %q dependency[%d]", endpointName, i)
		hasPath := definition.HasPath || definition.Path != ""
		hasConst := definition.HasConst || definition.Const != nil
		if hasPath == hasConst {
			return fmt.Errorf("%s must define exactly one of path or const", location)
		}
		if hasConst {
			name, ok := definition.Const.(string)
			if !ok || strings.TrimSpace(name) == "" {
				return fmt.Errorf("%s constant name must be a non-empty string", location)
			}
			addRecipeDependency(target, definition.Kind, name)
			continue
		}

		if strings.TrimSpace(definition.Path) == "" {
			return fmt.Errorf("%s path is empty", location)
		}
		selected := jsonPathValue(body, definition.Path)
		if !selected.Exists() || selected.Type == gjson.Null {
			continue
		}
		values := selected.Array()
		if !selected.IsArray() {
			values = []gjson.Result{selected}
		}
		for _, value := range values {
			if value.Type == gjson.Null {
				continue
			}
			name, err := recipeGJSONString(value)
			if err != nil {
				return fmt.Errorf("%s path must resolve to scalar names: %w", location, err)
			}
			if strings.TrimSpace(name) != "" {
				addRecipeDependency(target, definition.Kind, name)
			}
		}
	}
	return nil
}

func addRecipeDependency(target map[string]connector.ServiceDependency, kind, name string) {
	key := kind + "\x00" + name
	target[key] = connector.ServiceDependency{Kind: kind, Name: name}
}
