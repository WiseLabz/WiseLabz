package custom

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/tidwall/gjson"
	"go.yaml.in/yaml/v3"
)

const (
	maxRecipeBytes     = 64 * 1024
	maxRecipeEndpoints = 20
)

// Recipe describes a data-only mapping from same-origin HTTP endpoints to
// connector snapshot entities. Credentials are deliberately kept in config
// fields outside the recipe.
type Recipe struct {
	Version      int                `yaml:"version"`
	Category     string             `yaml:"category"`
	Auth         RecipeAuth         `yaml:"auth"`
	Endpoints    []RecipeEndpoint   `yaml:"endpoints"`
	Dependencies []RecipeDependency `yaml:"dependencies"`
}

// RecipeAuth selects how a recipe request receives credentials.
type RecipeAuth struct {
	Mode   string `yaml:"mode"`
	Name   string `yaml:"name"`
	Prefix string `yaml:"prefix"`
}

// RecipeEndpoint describes one request and its response mapping.
type RecipeEndpoint struct {
	Name         string             `yaml:"name"`
	Path         string             `yaml:"path"`
	Method       string             `yaml:"method"`
	Query        map[string]string  `yaml:"query"`
	Headers      map[string]string  `yaml:"headers"`
	Body         *yaml.Node         `yaml:"body"`
	Items        string             `yaml:"items"`
	Pagination   *RecipePagination  `yaml:"pagination"`
	Entity       RecipeEntity       `yaml:"entity"`
	Dependencies []RecipeDependency `yaml:"dependencies"`
}

// RecipePagination describes the future pagination formats. The current
// recipe runner validates these fields and rejects the block until the
// pagination implementation is available.
type RecipePagination struct {
	Type       string `yaml:"type"`
	Param      string `yaml:"param"`
	SizeParam  string `yaml:"size_param"`
	Size       int    `yaml:"size"`
	Start      int    `yaml:"start"`
	CursorPath string `yaml:"cursor_path"`
	NextPath   string `yaml:"next_path"`
	LinkHeader bool   `yaml:"link_header"`
}

// RecipeEntity defines the fixed kind and fields mapped from each item.
type RecipeEntity struct {
	Kind       string                     `yaml:"kind"`
	Name       string                     `yaml:"name"`
	ExternalID string                     `yaml:"external_id"`
	Hostname   string                     `yaml:"hostname"`
	IP         string                     `yaml:"ip"`
	MAC        string                     `yaml:"mac"`
	Aliases    string                     `yaml:"aliases"`
	Attributes map[string]RecipeAttribute `yaml:"attributes"`
}

// RecipeAttribute defines one attribute's source and optional conversion.
type RecipeAttribute struct {
	Path        string         `yaml:"path"`
	Const       any            `yaml:"const"`
	Template    string         `yaml:"template"`
	Type        string         `yaml:"type"`
	Map         map[string]any `yaml:"map"`
	Default     any            `yaml:"default"`
	HasPath     bool           `yaml:"-"`
	HasConst    bool           `yaml:"-"`
	HasTemplate bool           `yaml:"-"`
	HasDefault  bool           `yaml:"-"`
}

// RecipeDependency maps a dependency name from a constant or a response path.
type RecipeDependency struct {
	Kind     string `yaml:"kind"`
	Path     string `yaml:"path"`
	Const    any    `yaml:"const"`
	HasPath  bool   `yaml:"-"`
	HasConst bool   `yaml:"-"`
}

// RecipeIssue is a validation problem and its dotted location in the recipe.
type RecipeIssue struct {
	Location string
	Message  string
}

// RecipeValidationError carries every validation problem found in a recipe.
type RecipeValidationError struct {
	Issues []RecipeIssue
}

func (e *RecipeValidationError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return "invalid recipe"
	}
	lines := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		location := issue.Location
		if location == "" {
			location = "recipe"
		}
		lines = append(lines, redactEmbeddedRecipeURLs(location+": "+issue.Message))
	}
	return strings.Join(lines, "; ")
}

// ParseRecipe parses and validates a recipe document. It rejects unknown
// fields, malformed YAML, and every semantic error before callers make HTTP
// requests.
func ParseRecipe(raw string) (*Recipe, error) {
	if len([]byte(raw)) > maxRecipeBytes {
		return nil, &RecipeValidationError{Issues: dedupeIssues([]RecipeIssue{{Location: "recipe", Message: fmt.Sprintf("must be at most %d bytes", maxRecipeBytes)}})}
	}

	var document yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(raw))
	if err := decoder.Decode(&document); err != nil {
		return nil, &RecipeValidationError{Issues: dedupeIssues([]RecipeIssue{{Location: "recipe", Message: "invalid YAML: " + err.Error()}})}
	}
	if len(document.Content) == 0 {
		return nil, &RecipeValidationError{Issues: dedupeIssues([]RecipeIssue{{Location: "recipe", Message: "must be a YAML mapping"}})}
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); err != nil && !errors.Is(err, io.EOF) {
		return nil, &RecipeValidationError{Issues: dedupeIssues([]RecipeIssue{{Location: "recipe", Message: "invalid YAML: " + err.Error()}})}
	} else if err == nil && len(trailing.Content) > 0 {
		return nil, &RecipeValidationError{Issues: dedupeIssues([]RecipeIssue{{Location: "recipe", Message: "must contain exactly one YAML document"}})}
	}

	root := document.Content[0]
	if graphIssues := validateYAMLGraph(root); len(graphIssues) != 0 {
		return nil, &RecipeValidationError{Issues: dedupeIssues(graphIssues)}
	}
	issues := validateRecipeNode(root, "recipe")
	var recipe Recipe
	strictDecoder := yaml.NewDecoder(strings.NewReader(raw))
	strictDecoder.KnownFields(true)
	if err := strictDecoder.Decode(&recipe); err != nil {
		var typeErr *yaml.TypeError
		if errors.As(err, &typeErr) {
			for _, detail := range typeErr.Errors {
				if strings.Contains(detail, "not found in type") {
					continue // already reported with its dotted node location above.
				}
				issues = append(issues, RecipeIssue{Location: locationForYAMLError(root, detail), Message: detail})
			}
		} else {
			issues = append(issues, RecipeIssue{Location: "recipe", Message: "invalid YAML: " + err.Error()})
		}
	}
	markRecipePresence(root, &recipe)
	issues = append(issues, validateRecipe(&recipe)...)
	if len(issues) != 0 {
		return nil, &RecipeValidationError{Issues: dedupeIssues(issues)}
	}
	return &recipe, nil
}

// CategoryForConfig returns the category encoded by a custom connector's
// recipe, defaulting to virtualization when no recipe is configured.
func CategoryForConfig(config map[string]any) (string, error) {
	raw, ok := config["recipe"]
	if !ok || raw == nil || raw == "" {
		return "virtualization", nil
	}
	text, ok := raw.(string)
	if !ok {
		return "", &connector.ConfigValidationError{Field: "recipe", Message: "must be a YAML string"}
	}
	if strings.TrimSpace(text) == "" {
		return "virtualization", nil
	}
	recipe, err := ParseRecipe(text)
	if err != nil {
		return "", recipeConfigErrors(err)
	}
	return recipe.Category, nil
}

func recipeAuthMode(raw string) string {
	var partial struct {
		Auth RecipeAuth `yaml:"auth"`
	}
	if yaml.Unmarshal([]byte(raw), &partial) != nil {
		return ""
	}
	return partial.Auth.Mode
}

func markRecipePresence(root *yaml.Node, recipe *Recipe) {
	root = mappingContent(root)
	if root == nil {
		return
	}
	if node := dereferenceYAMLNode(mappingValue(root, "endpoints")); node != nil && node.Kind == yaml.SequenceNode {
		for i, endpointNode := range node.Content {
			endpoint := mappingContent(endpointNode)
			if endpoint == nil || i >= len(recipe.Endpoints) {
				continue
			}
			if body := mappingValue(endpoint, "body"); body != nil {
				recipe.Endpoints[i].Body = body
			}
			entity := mappingContent(mappingValue(endpoint, "entity"))
			if entity != nil {
				attributes := mappingContent(mappingValue(entity, "attributes"))
				for j := 0; attributes != nil && j+1 < len(attributes.Content); j += 2 {
					name := attributes.Content[j].Value
					if attr, ok := recipe.Endpoints[i].Entity.Attributes[name]; ok && attributes.Content[j+1].Kind == yaml.MappingNode {
						node := mappingContent(attributes.Content[j+1])
						attr.HasPath = mappingValue(node, "path") != nil
						attr.HasConst = mappingValue(node, "const") != nil
						attr.HasTemplate = mappingValue(node, "template") != nil
						attr.HasDefault = mappingValue(node, "default") != nil
						recipe.Endpoints[i].Entity.Attributes[name] = attr
					}
				}
			}
			markDependencyPresence(dereferenceYAMLNode(mappingValue(endpoint, "dependencies")), recipe.Endpoints[i].Dependencies)
		}
	}
	markDependencyPresence(dereferenceYAMLNode(mappingValue(root, "dependencies")), recipe.Dependencies)
}

func markDependencyPresence(node *yaml.Node, dependencies []RecipeDependency) {
	node = dereferenceYAMLNode(node)
	if node == nil || node.Kind != yaml.SequenceNode {
		return
	}
	for i, item := range node.Content {
		if i < len(dependencies) {
			mapping := mappingContent(item)
			dependencies[i].HasPath = mappingValue(mapping, "path") != nil
			dependencies[i].HasConst = mappingValue(mapping, "const") != nil
		}
	}
}

func validateRecipe(recipe *Recipe) []RecipeIssue {
	var issues []RecipeIssue
	add := func(location, message string) {
		issues = append(issues, RecipeIssue{Location: location, Message: message})
	}
	if recipe.Version != 1 {
		add("version", "must be 1")
	}
	if !validCategory(recipe.Category) {
		add("category", fmt.Sprintf("must be one of %v", connector.Categories()))
	}
	switch recipe.Auth.Mode {
	case "none":
		if recipe.Auth.Name != "" {
			add("auth.name", "is only allowed for header or query authentication")
		}
		if recipe.Auth.Prefix != "" {
			add("auth.prefix", "is only allowed for header authentication")
		}
	case "header":
		if !validHeaderName(recipe.Auth.Name) {
			add("auth.name", "must be a valid HTTP header name")
		}
		if !validHeaderValue(recipe.Auth.Prefix) {
			add("auth.prefix", "contains an invalid HTTP header control byte")
		}
	case "basic":
		if recipe.Auth.Name != "" {
			add("auth.name", "is not used by basic authentication")
		}
		if recipe.Auth.Prefix != "" {
			add("auth.prefix", "is not used by basic authentication")
		}
	case "query":
		if strings.TrimSpace(recipe.Auth.Name) == "" {
			add("auth.name", "is required for query authentication")
		} else if hasHeaderControl(recipe.Auth.Name) {
			add("auth.name", "must not contain line breaks")
		}
		if recipe.Auth.Prefix != "" {
			add("auth.prefix", "is only allowed for header authentication")
		}
	default:
		add("auth.mode", "must be one of none, header, basic, query")
	}
	if len(recipe.Endpoints) == 0 {
		add("endpoints", "must contain at least one endpoint")
	}
	if len(recipe.Endpoints) > maxRecipeEndpoints {
		add("endpoints", fmt.Sprintf("must contain at most %d endpoints", maxRecipeEndpoints))
	}
	seenNames := make(map[string]bool, len(recipe.Endpoints))
	for i := range recipe.Endpoints {
		endpoint := &recipe.Endpoints[i]
		base := fmt.Sprintf("endpoints[%d]", i)
		if strings.TrimSpace(endpoint.Name) == "" {
			add(base+".name", "is required")
		} else if seenNames[endpoint.Name] {
			add(base+".name", "must be unique")
		} else {
			seenNames[endpoint.Name] = true
		}
		if err := validateEndpointPath(endpoint.Path); err != nil {
			add(base+".path", err.Error())
		}
		if endpoint.Method != "GET" && endpoint.Method != "POST" {
			add(base+".method", "must be GET or POST")
		}
		if endpoint.Method == "GET" && endpoint.Body != nil && !yamlNodeIsNull(endpoint.Body) {
			add(base+".body", "is only allowed with POST")
		}
		if endpoint.Method == "POST" && endpoint.Body != nil {
			if _, err := yamlBodyJSON(endpoint.Body); err != nil {
				add(base+".body", "must be a static JSON value: "+err.Error())
			}
		}
		if err := validateGJSONPath(endpoint.Items); err != nil {
			add(base+".items", err.Error())
		}
		for name, value := range endpoint.Headers {
			if !validHeaderName(name) {
				add(base+".headers."+name, "must be a valid HTTP header name")
			}
			if !validHeaderValue(value) {
				add(base+".headers."+name, "contains an invalid HTTP header control byte")
			}
		}
		issues = append(issues, validateRecipeEntity(endpoint.Entity, base+".entity")...)
		issues = append(issues, validateDependencies(endpoint.Dependencies, base+".dependencies")...)
		if endpoint.Pagination != nil {
			issues = append(issues, validatePagination(endpoint.Pagination, base+".pagination")...)
			add(base+".pagination", "pagination is not supported yet")
		}
	}
	issues = append(issues, validateDependencies(recipe.Dependencies, "dependencies")...)
	return issues
}

func validateRecipeEntity(entity RecipeEntity, base string) []RecipeIssue {
	var issues []RecipeIssue
	add := func(location, message string) {
		issues = append(issues, RecipeIssue{Location: location, Message: message})
	}
	if strings.TrimSpace(entity.Kind) == "" {
		add(base+".kind", "is required")
	}
	for _, field := range []struct{ name, path string }{
		{"name", entity.Name}, {"external_id", entity.ExternalID},
		{"hostname", entity.Hostname}, {"ip", entity.IP}, {"mac", entity.MAC}, {"aliases", entity.Aliases},
	} {
		if (field.name == "name" || field.name == "external_id") && strings.TrimSpace(field.path) == "" {
			add(base+"."+field.name, "mapping is required")
		} else if field.path != "" {
			if err := validateGJSONPath(field.path); err != nil {
				add(base+"."+field.name, err.Error())
			}
		}
	}
	keys := make([]string, 0, len(entity.Attributes))
	for key := range entity.Attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, name := range keys {
		attr := entity.Attributes[name]
		location := base + ".attributes." + name
		count := 0
		for _, present := range []bool{attr.HasPath, attr.HasConst, attr.HasTemplate} {
			if present {
				count++
			}
		}
		if count != 1 {
			add(location, "must define exactly one of path, const, or template")
		}
		if attr.HasPath {
			if err := validateGJSONPath(attr.Path); err != nil {
				add(location+".path", err.Error())
			}
		}
		if attr.HasTemplate {
			if _, err := templatePaths(attr.Template); err != nil {
				add(location+".template", err.Error())
			}
		}
		if attr.Type != "" && !validAttributeType(attr.Type) {
			add(location+".type", "must be string, number, bool, boolean, or list")
		}
		for key := range attr.Map {
			if strings.ContainsAny(key, "\r\n") {
				add(location+".map", "keys must not contain line breaks")
				break
			}
		}
	}
	return issues
}

func validateDependencies(dependencies []RecipeDependency, base string) []RecipeIssue {
	var issues []RecipeIssue
	for i, dependency := range dependencies {
		location := fmt.Sprintf("%s[%d]", base, i)
		switch dependency.Kind {
		case "host", "network", "storage", "upstream_service":
		default:
			issues = append(issues, RecipeIssue{Location: location + ".kind", Message: "must be host, network, storage, or upstream_service"})
		}
		if dependency.HasPath == dependency.HasConst {
			issues = append(issues, RecipeIssue{Location: location, Message: "must define exactly one of path or const"})
		}
		if dependency.HasPath {
			if err := validateGJSONPath(dependency.Path); err != nil {
				issues = append(issues, RecipeIssue{Location: location + ".path", Message: err.Error()})
			}
		}
		if dependency.HasConst {
			if value, ok := dependency.Const.(string); !ok || strings.TrimSpace(value) == "" {
				issues = append(issues, RecipeIssue{Location: location + ".const", Message: "must be a non-empty string"})
			}
		}
	}
	return issues
}

func validatePagination(pagination *RecipePagination, base string) []RecipeIssue {
	var issues []RecipeIssue
	add := func(suffix, message string) {
		issues = append(issues, RecipeIssue{Location: base + suffix, Message: message})
	}
	switch pagination.Type {
	case "page":
		if pagination.Param == "" {
			add(".param", "is required for page pagination")
		}
		if pagination.Size <= 0 {
			add(".size", "must be greater than zero")
		}
	case "offset":
		if pagination.Param == "" {
			add(".param", "is required for offset pagination")
		}
		if pagination.SizeParam == "" {
			add(".size_param", "is required for offset pagination")
		}
		if pagination.Size <= 0 {
			add(".size", "must be greater than zero")
		}
	case "cursor":
		if pagination.Param == "" {
			add(".param", "is required for cursor pagination")
		}
		if err := validateGJSONPath(pagination.CursorPath); err != nil {
			add(".cursor_path", err.Error())
		}
	case "next_link":
		if pagination.LinkHeader == (pagination.NextPath != "") {
			add("", "must define exactly one of next_path or link_header")
		}
		if pagination.NextPath != "" {
			if err := validateGJSONPath(pagination.NextPath); err != nil {
				add(".next_path", err.Error())
			}
		}
	default:
		add(".type", "must be page, offset, cursor, or next_link")
	}
	if pagination.Start < 0 {
		add(".start", "must not be negative")
	}
	if pagination.SizeParam != "" && pagination.Size <= 0 {
		add(".size", "must be greater than zero when size_param is set")
	}
	return issues
}

func validateEndpointPath(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("is malformed")
	}
	if u.IsAbs() || u.Scheme != "" {
		return errors.New("must be relative to the connector URL")
	}
	if strings.HasPrefix(raw, "//") || u.Host != "" {
		return errors.New("must not be scheme-relative")
	}
	if u.Fragment != "" {
		return errors.New("must not contain a fragment")
	}
	return nil
}

func validateGJSONPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path expression is required")
	}
	if strings.TrimSpace(path) != path || strings.ContainsAny(path, "\r\n\t") {
		return errors.New("path expression has invalid whitespace")
	}
	type delimiter struct {
		char rune
		pos  int
	}
	stack := make([]delimiter, 0, 4)
	var quote rune
	escaped := false
	lastSeparator := false
	for i, r := range path {
		if escaped {
			escaped = false
			lastSeparator = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			continue
		}
		switch r {
		case '[', '(', '{':
			stack = append(stack, delimiter{char: r, pos: i})
			lastSeparator = false
		case ']', ')', '}':
			want := map[rune]rune{']': '[', ')': '(', '}': '{'}[r]
			if len(stack) == 0 || stack[len(stack)-1].char != want {
				return errors.New("path expression has unbalanced delimiters")
			}
			open := stack[len(stack)-1]
			if r == ')' && open.pos+2 == i && open.pos > 0 && path[open.pos-1] == '#' {
				return errors.New("path expression has an empty query")
			}
			stack = stack[:len(stack)-1]
			lastSeparator = false
		case '.', '|':
			if len(stack) == 0 {
				if lastSeparator || i == 0 {
					return errors.New("path expression is malformed")
				}
				lastSeparator = true
			}
		case ' ', '\f', '\v':
			if len(stack) == 0 {
				return errors.New("path expression has invalid whitespace")
			}
		default:
			lastSeparator = false
		}
	}
	if escaped || quote != 0 || len(stack) != 0 || lastSeparator {
		return errors.New("path expression is malformed")
	}
	return nil
}

func templatePaths(template string) ([]string, error) {
	var paths []string
	for i := 0; i < len(template); {
		switch template[i] {
		case '{':
			if i+1 < len(template) && template[i+1] == '{' {
				i += 2
				continue
			}
			end := strings.IndexByte(template[i+1:], '}')
			if end < 0 {
				return nil, errors.New("template has an unclosed placeholder")
			}
			path := template[i+1 : i+1+end]
			if err := validateGJSONPath(path); err != nil {
				return nil, fmt.Errorf("placeholder %q: %w", path, err)
			}
			paths = append(paths, path)
			i += end + 2
		case '}':
			if i+1 < len(template) && template[i+1] == '}' {
				i += 2
			} else {
				return nil, errors.New("template has an unmatched closing brace")
			}
		default:
			i++
		}
	}
	return paths, nil
}

func validateRecipeNode(node *yaml.Node, path string) []RecipeIssue {
	node = dereferenceYAMLNode(node)
	if node == nil || node.Kind != yaml.MappingNode {
		return []RecipeIssue{{Location: path, Message: "must be a mapping"}}
	}
	var issues []RecipeIssue
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
			issues = append(issues, RecipeIssue{Location: path, Message: "mapping keys must be strings"})
			continue
		}
		childPath := path + "." + key.Value
		schemaPath := normalizeRecipePath(path)
		if !knownRecipeField(schemaPath, key.Value) {
			issues = append(issues, RecipeIssue{Location: childPath, Message: "unknown field"})
			continue
		}
		if nested := knownRecipeChildren(path, key.Value, value); nested != nil {
			issues = append(issues, nested...)
		}
	}
	return issues
}

func knownRecipeField(path, key string) bool {
	var fields string
	switch path {
	case "recipe":
		fields = "version category auth endpoints dependencies"
	case "recipe.auth":
		fields = "mode name prefix"
	case "recipe.endpoints[]":
		fields = "name path method query headers body items pagination entity dependencies"
	case "recipe.endpoints[].pagination":
		fields = "type param size_param size start cursor_path next_path link_header"
	case "recipe.endpoints[].entity":
		fields = "kind name external_id hostname ip mac aliases attributes"
	case "recipe.endpoints[].entity.attributes[]":
		fields = "path const template type map default"
	case "recipe.endpoints[].dependencies[]", "recipe.dependencies[]":
		fields = "kind path const"
	default:
		return true
	}
	for _, allowed := range strings.Fields(fields) {
		if key == allowed {
			return true
		}
	}
	return false
}

func knownRecipeChildren(parent, key string, node *yaml.Node) []RecipeIssue {
	node = dereferenceYAMLNode(node)
	schemaParent := normalizeRecipePath(parent)
	path := parent + "." + key
	switch {
	case schemaParent == "recipe" && key == "auth":
		return validateRecipeNode(node, path)
	case schemaParent == "recipe" && key == "endpoints":
		if node.Kind != yaml.SequenceNode {
			return []RecipeIssue{{Location: path, Message: "must be a list"}}
		}
		var issues []RecipeIssue
		for i, endpoint := range node.Content {
			issues = append(issues, validateRecipeNode(endpoint, fmt.Sprintf("%s[%d]", path, i))...)
		}
		return issues
	case schemaParent == "recipe.endpoints[]" && key == "pagination":
		return validateRecipeNode(node, path)
	case schemaParent == "recipe.endpoints[]" && key == "entity":
		return validateRecipeNode(node, path)
	case schemaParent == "recipe.endpoints[]" && key == "dependencies", schemaParent == "recipe" && key == "dependencies":
		if node.Kind != yaml.SequenceNode {
			return []RecipeIssue{{Location: path, Message: "must be a list"}}
		}
		var issues []RecipeIssue
		for i, dependency := range node.Content {
			issues = append(issues, validateRecipeNode(dependency, fmt.Sprintf("%s[%d]", path, i))...)
		}
		return issues
	case schemaParent == "recipe.endpoints[].entity" && key == "attributes":
		if node.Kind != yaml.MappingNode {
			return []RecipeIssue{{Location: path, Message: "must be a mapping"}}
		}
		var issues []RecipeIssue
		for i := 0; i+1 < len(node.Content); i += 2 {
			name := node.Content[i].Value
			issues = append(issues, validateRecipeNode(node.Content[i+1], path+"."+name)...)
		}
		return issues
	case schemaParent == "recipe.endpoints[].entity.attributes[]":
		if key == "map" {
			if node.Kind != yaml.MappingNode {
				return []RecipeIssue{{Location: path, Message: "must be a mapping"}}
			}
		}
	}
	return nil
}

func mappingContent(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}
	if node.Kind == yaml.AliasNode {
		return mappingContent(node.Alias)
	}
	if node.Kind != yaml.MappingNode {
		return nil
	}
	return node
}

func dereferenceYAMLNode(node *yaml.Node) *yaml.Node {
	for node != nil && node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	return node
}

func validateYAMLGraph(root *yaml.Node) []RecipeIssue {
	const maxDepth = 128
	const maxNodes = 100_000
	active := make(map[*yaml.Node]bool)
	remaining := maxNodes
	var issues []RecipeIssue
	var visit func(node *yaml.Node, path string, depth int)
	visit = func(node *yaml.Node, path string, depth int) {
		if node == nil {
			return
		}
		if depth > maxDepth {
			issues = append(issues, RecipeIssue{Location: path, Message: fmt.Sprintf("YAML nesting exceeds %d levels", maxDepth)})
			return
		}
		remaining--
		if remaining < 0 {
			issues = append(issues, RecipeIssue{Location: path, Message: "YAML alias expansion exceeds the validation limit"})
			return
		}
		if active[node] {
			issues = append(issues, RecipeIssue{Location: path, Message: "cyclic YAML aliases are not supported"})
			return
		}
		active[node] = true
		defer delete(active, node)
		if node.Kind == yaml.AliasNode {
			visit(node.Alias, path, depth+1)
			return
		}
		switch node.Kind {
		case yaml.MappingNode:
			for i := 0; i+1 < len(node.Content); i += 2 {
				key, value := node.Content[i], node.Content[i+1]
				childPath := path
				if key.Kind == yaml.ScalarNode && key.Tag == "!!str" {
					childPath += "." + key.Value
				}
				visit(value, childPath, depth+1)
			}
		case yaml.SequenceNode:
			for i, value := range node.Content {
				visit(value, fmt.Sprintf("%s[%d]", path, i), depth+1)
			}
		}
	}
	visit(root, "recipe", 0)
	return issues
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func locationForYAMLError(root *yaml.Node, detail string) string {
	line := 0
	if strings.HasPrefix(detail, "line ") {
		if token, _, ok := strings.Cut(strings.TrimPrefix(detail, "line "), ":"); ok {
			line, _ = strconv.Atoi(token)
		}
	}
	if line == 0 {
		return "recipe"
	}
	if location := findYAMLLine(root, "recipe", line); location != "" {
		return location
	}
	return "recipe"
}

func findYAMLLine(node *yaml.Node, path string, line int) string {
	if node == nil {
		return ""
	}
	if node.Line == line {
		return path
	}
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			childPath := path + "." + key.Value
			if key.Line == line {
				return childPath
			}
			if result := findYAMLLine(value, childPath, line); result != "" {
				return result
			}
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			if result := findYAMLLine(child, fmt.Sprintf("%s[%d]", path, i), line); result != "" {
				return result
			}
		}
	}
	return ""
}

func dedupeIssues(issues []RecipeIssue) []RecipeIssue {
	seen := make(map[string]bool, len(issues))
	out := make([]RecipeIssue, 0, len(issues))
	for _, issue := range issues {
		issue.Location = redactEmbeddedRecipeURLs(issue.Location)
		issue.Message = redactEmbeddedRecipeURLs(issue.Message)
		key := issue.Location + "\x00" + issue.Message
		if !seen[key] {
			seen[key] = true
			out = append(out, issue)
		}
	}
	return out
}

func validCategory(category string) bool {
	return connector.ValidCategory(category)
}

func validAttributeType(typ string) bool {
	switch typ {
	case "string", "number", "bool", "boolean", "list", "string_array":
		return true
	default:
		return false
	}
}

func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', strings.ContainsRune("!#$%&'*+-.^_`|~", r):
		default:
			return false
		}
	}
	return true
}

func hasHeaderControl(value string) bool {
	return strings.ContainsAny(value, "\r\n")
}

func validHeaderValue(value string) bool {
	for i := 0; i < len(value); i++ {
		b := value[i]
		if (b < 0x20 && b != '\t') || b == 0x7f {
			return false
		}
	}
	return true
}

func yamlNodeIsNull(node *yaml.Node) bool {
	for node != nil && node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	return node != nil && node.Kind == yaml.ScalarNode && node.Tag == "!!null"
}

func yamlBodyJSON(node *yaml.Node) ([]byte, error) {
	if node == nil {
		return nil, errors.New("body is missing")
	}
	var value any
	if err := node.Decode(&value); err != nil {
		return nil, err
	}
	if err := validateJSONValue(value, 0); err != nil {
		return nil, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func validateJSONValue(value any, depth int) error {
	if depth > 128 {
		return errors.New("JSON body nesting exceeds 128 levels")
	}
	switch value := value.(type) {
	case nil, bool, string,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return nil
	case time.Time:
		return errors.New("timestamp scalars must be quoted strings")
	case []any:
		for _, item := range value {
			if err := validateJSONValue(item, depth+1); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		for _, item := range value {
			if err := validateJSONValue(item, depth+1); err != nil {
				return err
			}
		}
		return nil
	case map[any]any:
		for key, item := range value {
			if _, ok := key.(string); !ok {
				return errors.New("object keys must be strings")
			}
			if err := validateJSONValue(item, depth+1); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported JSON value %T", value)
	}
}

func recipeBody(endpoint RecipeEndpoint) ([]byte, error) {
	if endpoint.Body == nil || (endpoint.Method == http.MethodGet && yamlNodeIsNull(endpoint.Body)) {
		return nil, nil
	}
	return yamlBodyJSON(endpoint.Body)
}

func jsonPathValue(data []byte, path string) gjson.Result {
	return gjson.GetBytes(data, path)
}

func normalizeRecipePath(path string) string {
	for i := 0; i < len(path); i++ {
		if path[i] != '[' {
			continue
		}
		end := strings.IndexByte(path[i+1:], ']')
		if end < 0 {
			break
		}
		end += i + 1
		if end > i+1 {
			if _, err := strconv.Atoi(path[i+1 : end]); err == nil {
				path = path[:i] + "[]" + path[end+1:]
				i++
			}
		}
	}
	if i := strings.Index(path, ".entity.attributes."); i >= 0 {
		path = path[:i] + ".entity.attributes[]"
	}
	return path
}

func recipeConfigErrors(err error) error {
	var validation *RecipeValidationError
	if !errors.As(err, &validation) {
		return err
	}
	configErrors := make([]error, 0, len(validation.Issues))
	for _, issue := range validation.Issues {
		field := "recipe"
		if issue.Location != "" && issue.Location != "recipe" {
			field += "." + strings.TrimPrefix(issue.Location, "recipe.")
		}
		configErrors = append(configErrors, &connector.ConfigValidationError{
			Field:   redactEmbeddedRecipeURLs(field),
			Message: redactEmbeddedRecipeURLs(issue.Message),
		})
	}
	return errors.Join(configErrors...)
}

func redactURLsInRecipeError(err error) error {
	if err == nil {
		return nil
	}
	message := redactEmbeddedRecipeURLs(err.Error())
	if message == err.Error() {
		return err
	}
	return &sanitizedRecipeError{message: message, err: err}
}

func redactEmbeddedRecipeURLs(message string) string {
	lower := strings.ToLower(message)
	var output strings.Builder
	last := 0
	for searchFrom := 0; searchFrom < len(message); {
		httpAt := strings.Index(lower[searchFrom:], "http://")
		httpsAt := strings.Index(lower[searchFrom:], "https://")
		if httpAt < 0 && httpsAt < 0 {
			break
		}
		relative := httpAt
		if relative < 0 || (httpsAt >= 0 && httpsAt < relative) {
			relative = httpsAt
		}
		start := searchFrom + relative
		end := start
		for end < len(message) {
			r := rune(message[end])
			if strings.ContainsRune(" \t\r\n\"'`<>", r) {
				break
			}
			end++
		}
		if end == start {
			searchFrom = start + 1
			continue
		}
		output.WriteString(message[last:start])
		output.WriteString(connector.RedactURL(message[start:end]))
		last = end
		searchFrom = end
	}
	if last == 0 {
		return message
	}
	output.WriteString(message[last:])
	return output.String()
}
