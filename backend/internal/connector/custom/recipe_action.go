package custom

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode"
	"unicode/utf8"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"go.yaml.in/yaml/v3"
)

const actionExcerptBytes = 512

// actionCapabilityConfig keeps only the recipe needed to compute per-instance
// capabilities; secrets and unrelated connector settings stay out of it.
func actionCapabilityConfig(config map[string]any) map[string]any {
	if recipe, ok := config["recipe"]; ok {
		return map[string]any{"recipe": recipe}
	}
	return nil
}

// SupportsLifecycleVerb reports whether this custom connector's recipe declares
// the requested lifecycle action at the service or entity scope.
func (c *Connector) SupportsLifecycleVerb(verb string) bool {
	if verb != "restart" && verb != "start" && verb != "stop" {
		return false
	}
	for _, action := range c.DeclaredActions() {
		if action.Name == verb {
			return true
		}
	}
	return false
}

// DeclaredActions returns the service and entity actions in stable order.
func (c *Connector) DeclaredActions() []connector.ActionDescriptor {
	recipe, err := recipeFromConfig(c.capabilityConfig)
	if err != nil {
		return []connector.ActionDescriptor{}
	}
	return actionDescriptors(recipe)
}

func actionDescriptors(recipe *Recipe) []connector.ActionDescriptor {
	var out []connector.ActionDescriptor
	for name, action := range recipe.Actions {
		out = append(out, actionDescriptor(name, "", false, action))
	}
	for _, endpoint := range recipe.Endpoints {
		for name, action := range endpoint.Entity.Actions {
			out = append(out, actionDescriptor(name, endpoint.Entity.Kind, true, action))
		}
	}
	if len(out) == 0 {
		return []connector.ActionDescriptor{}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].EntityScope != out[j].EntityScope {
			return !out[i].EntityScope
		}
		if out[i].EntityKind != out[j].EntityKind {
			return out[i].EntityKind < out[j].EntityKind
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func actionDescriptor(name, kind string, entityScope bool, action RecipeAction) connector.ActionDescriptor {
	label := action.Label
	if label == "" {
		label = name
	}
	return connector.ActionDescriptor{
		Name:            name,
		EntityKind:      kind,
		Label:           label,
		Description:     action.Description,
		EntityScope:     entityScope,
		DowntimeSeconds: ActionDowntime(name, action),
	}
}

// ResolveAction turns a declared service or entity action into its concrete
// request. It does no network I/O and resolves entity values only from snapshot.
func (c *Connector) ResolveAction(config map[string]any, name, entityRef string, snapshot *connector.ServiceSnapshot) (*connector.ResolvedAction, error) {
	recipe, err := recipeFromConfig(config)
	if err != nil {
		return nil, safeActionError(config, err)
	}

	var action RecipeAction
	var descriptor connector.ActionDescriptor
	var scope string
	var entity *connector.SnapshotEntity
	if entityRef == "" {
		var ok bool
		action, ok = recipe.Actions[name]
		if !ok {
			if actionDeclaredForAnyEntity(recipe, name) {
				return nil, fmt.Errorf("action %q requires an entity reference", name)
			}
			return nil, fmt.Errorf("action %q is not declared for the service", name)
		}
		descriptor = actionDescriptor(name, "", false, action)
		scope = "service." + name
	} else {
		if snapshot == nil {
			return nil, fmt.Errorf("latest snapshot is unavailable for entity action %q", name)
		}
		entity, err = findActionEntity(snapshot, entityRef, recipe, name)
		if err != nil {
			return nil, safeActionError(config, err)
		}
		var endpoint *RecipeEndpoint
		for i := range recipe.Endpoints {
			candidate := &recipe.Endpoints[i]
			if candidate.Entity.Kind == entity.Kind {
				if declared, ok := candidate.Entity.Actions[name]; ok {
					action, endpoint = declared, candidate
					break
				}
			}
		}
		if endpoint == nil {
			return nil, fmt.Errorf("action %q is not declared for entity kind %q", name, entity.Kind)
		}
		descriptor = actionDescriptor(name, entity.Kind, true, action)
		scope = "entity." + entity.Kind + "." + name
	}

	resolvedAction, err := resolveRecipeAction(action, entity)
	if err != nil {
		return nil, safeActionError(config, err)
	}
	request, err := buildResolvedActionRequest(config, recipe, name, resolvedAction)
	if err != nil {
		return nil, safeActionError(config, err)
	}
	fingerprint, err := actionFingerprint(scope, name, action)
	if err != nil {
		return nil, safeActionError(config, err)
	}
	return &connector.ResolvedAction{
		Request:     request,
		Descriptor:  descriptor,
		Fingerprint: fingerprint,
	}, nil
}

func actionDeclaredForAnyEntity(recipe *Recipe, name string) bool {
	for _, endpoint := range recipe.Endpoints {
		if _, ok := endpoint.Entity.Actions[name]; ok {
			return true
		}
	}
	return false
}

func findActionEntity(snapshot *connector.ServiceSnapshot, entityRef string, recipe *Recipe, name string) (*connector.SnapshotEntity, error) {
	var found *connector.SnapshotEntity
	for i := range snapshot.Entities {
		entity := &snapshot.Entities[i]
		if entity.ExternalID != entityRef {
			continue
		}
		for _, endpoint := range recipe.Endpoints {
			if endpoint.Entity.Kind == entity.Kind {
				if _, declared := endpoint.Entity.Actions[name]; declared {
					if found != nil {
						return nil, fmt.Errorf("entity reference %q matches more than one target", entityRef)
					}
					found = entity
				}
			}
		}
	}
	if found != nil {
		return found, nil
	}
	for i := range snapshot.Entities {
		if snapshot.Entities[i].ExternalID == entityRef {
			return nil, fmt.Errorf("action %q is not declared for entity kind %q", name, snapshot.Entities[i].Kind)
		}
	}
	return nil, fmt.Errorf("entity reference %q was not found in the latest snapshot", entityRef)
}

func resolveRecipeAction(action RecipeAction, entity *connector.SnapshotEntity) (RecipeAction, error) {
	resolved := action
	values := map[string]string{}
	if entity != nil {
		values["external_id"] = entity.ExternalID
	}
	var err error
	pathTemplate, queryTemplate, hasQuery := strings.Cut(action.Path, "?")
	resolved.Path, err = resolveActionTemplate(pathTemplate, values, entity, true)
	if err != nil {
		return RecipeAction{}, fmt.Errorf("action path placeholder: %w", err)
	}
	if hasQuery {
		// Path-segment escaping leaves & = + alone, so a placeholder value could
		// add query parameters: only a static query string is allowed here.
		if placeholders, _ := parseActionPlaceholders(queryTemplate); len(placeholders) != 0 {
			return RecipeAction{}, fmt.Errorf("action path placeholder: placeholders are not allowed in the query string of path; declare them under query")
		}
		queryString, err := resolveActionTemplate(queryTemplate, values, entity, false)
		if err != nil {
			return RecipeAction{}, fmt.Errorf("action path placeholder: %w", err)
		}
		resolved.Path += "?" + queryString
	}
	if len(action.Query) != 0 {
		resolved.Query = make(map[string]string, len(action.Query))
		for key, value := range action.Query {
			resolved.Query[key], err = resolveActionTemplate(value, values, entity, false)
			if err != nil {
				return RecipeAction{}, fmt.Errorf("action query placeholder %q: %w", key, err)
			}
		}
	}
	if action.HasBody || action.Body != nil {
		resolved.Body, err = resolveActionBody(action.Body, values, entity)
		if err != nil {
			return RecipeAction{}, fmt.Errorf("action body placeholder: %w", err)
		}
	}
	return resolved, nil
}

func resolveActionBody(value any, values map[string]string, entity *connector.SnapshotEntity) (any, error) {
	switch value := value.(type) {
	case string:
		return resolveActionTemplate(value, values, entity, false)
	case []any:
		out := make([]any, len(value))
		for i, item := range value {
			resolved, err := resolveActionBody(item, values, entity)
			if err != nil {
				return nil, err
			}
			out[i] = resolved
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, item := range value {
			resolved, err := resolveActionBody(item, values, entity)
			if err != nil {
				return nil, err
			}
			out[key] = resolved
		}
		return out, nil
	case map[any]any:
		out := make(map[string]any, len(value))
		for key, item := range value {
			name, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("body object keys must be strings")
			}
			resolved, err := resolveActionBody(item, values, entity)
			if err != nil {
				return nil, err
			}
			out[name] = resolved
		}
		return out, nil
	default:
		return value, nil
	}
}

func resolveActionTemplate(value string, values map[string]string, entity *connector.SnapshotEntity, pathSegments bool) (string, error) {
	var out strings.Builder
	for i := 0; i < len(value); {
		switch value[i] {
		case '{':
			if i+1 < len(value) && value[i+1] == '{' {
				out.WriteByte('{')
				i += 2
				continue
			}
			end := strings.IndexByte(value[i+1:], '}')
			if end < 0 {
				return "", fmt.Errorf("has an unclosed placeholder")
			}
			name := value[i+1 : i+1+end]
			resolved, err := actionPlaceholderValue(name, values, entity)
			if err != nil {
				return "", err
			}
			if pathSegments {
				pathValue := resolved
				resolved, err = connector.PathSegment(pathValue)
				if err != nil {
					return "", fmt.Errorf("placeholder {%s} value %q is not a valid path segment: %w", name, pathValue, err)
				}
			}
			out.WriteString(resolved)
			i += end + 2
		case '}':
			if i+1 < len(value) && value[i+1] == '}' {
				out.WriteByte('}')
				i += 2
				continue
			}
			return "", fmt.Errorf("has an unmatched closing brace")
		default:
			out.WriteByte(value[i])
			i++
		}
	}
	return out.String(), nil
}

func actionPlaceholderValue(name string, values map[string]string, entity *connector.SnapshotEntity) (string, error) {
	if name == "external_id" {
		if values[name] == "" {
			return "", fmt.Errorf("external_id is empty")
		}
		return values[name], nil
	}
	if !strings.HasPrefix(name, "attr.") {
		return "", fmt.Errorf("unsupported placeholder {%s}", name)
	}
	attribute := strings.TrimPrefix(name, "attr.")
	if entity == nil {
		return "", fmt.Errorf("service actions cannot use attribute placeholders")
	}
	value, ok := entity.Attributes[attribute]
	if !ok || value == nil {
		return "", fmt.Errorf("entity is missing mapped attribute %q", attribute)
	}
	switch value := value.(type) {
	case float32:
		return strconv.FormatFloat(float64(value), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64), nil
	case string, bool, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, json.Number:
		return fmt.Sprint(value), nil
	default:
		return "", fmt.Errorf("mapped attribute %q must be a scalar value", attribute)
	}
}

func buildResolvedActionRequest(config map[string]any, recipe *Recipe, name string, action RecipeAction) (connector.ActionRequest, error) {
	baseURL, err := recipeBaseURL(config)
	if err != nil {
		return connector.ActionRequest{}, err
	}
	body, err := actionBodyNode(action)
	if err != nil {
		return connector.ActionRequest{}, err
	}
	endpoint := RecipeEndpoint{
		Name:    name,
		Path:    action.Path,
		Method:  action.Method,
		Query:   action.Query,
		Headers: action.Headers,
		Body:    body,
	}
	req, err := buildRecipeRequest(context.Background(), baseURL, recipe, endpoint, config)
	if err != nil {
		return connector.ActionRequest{}, safeActionError(config, err)
	}
	headers := make(map[string]string, len(req.Header))
	for name, values := range req.Header {
		headers[name] = strings.Join(values, ", ")
	}
	var requestBody any
	if action.HasBody || action.Body != nil {
		encoded, err := json.Marshal(action.Body)
		if err != nil {
			return connector.ActionRequest{}, fmt.Errorf("encode action body: %w", err)
		}
		if action.Body == nil {
			requestBody = json.RawMessage(encoded)
		} else {
			requestBody = action.Body
		}
	}
	return connector.ActionRequest{
		Method:  req.Method,
		URL:     req.URL.String(),
		Headers: headers,
		Body:    requestBody,
	}, nil
}

func actionBodyNode(action RecipeAction) (*yaml.Node, error) {
	if !action.HasBody && action.Body == nil {
		return nil, nil
	}
	data, err := json.Marshal(action.Body)
	if err != nil {
		return nil, fmt.Errorf("encode action body: %w", err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("encode action body: %w", err)
	}
	if len(document.Content) == 0 {
		return nil, fmt.Errorf("encode action body: empty JSON value")
	}
	return document.Content[0], nil
}

func actionBodyNodeFromRequest(value any) (*yaml.Node, error) {
	if value == nil {
		return nil, nil
	}
	var data []byte
	var err error
	if raw, ok := value.(json.RawMessage); ok {
		data = raw
	} else {
		data, err = json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("encode action body: %w", err)
		}
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("encode action body: %w", err)
	}
	if len(document.Content) == 0 {
		return nil, fmt.Errorf("encode action body: empty JSON value")
	}
	return document.Content[0], nil
}

func actionFingerprint(scope, name string, action RecipeAction) (string, error) {
	data, err := json.Marshal(canonicalAction(name, action))
	if err != nil {
		return "", fmt.Errorf("encode action definition: %w", err)
	}
	sum := sha256.Sum256(append([]byte(scope+"\x00"), data...))
	return hex.EncodeToString(sum[:]), nil
}

// RedactedActionRequest returns a preview-safe copy of an action request. It
// removes user information and the recipe's configured query-auth parameter,
// while keeping other static query values visible.
func RedactedActionRequest(config map[string]any, action *connector.ResolvedAction) connector.ActionRequest {
	if action == nil {
		return connector.ActionRequest{}
	}
	request := action.Request
	request.Headers = cloneActionHeaders(request.Headers)
	secretHeaders := &http.Request{Header: make(http.Header)}
	setHeaders(secretHeaders, config)
	recipe, recipeErr := recipeFromConfig(config)
	if recipeErr == nil && recipe.Auth.Mode == "header" {
		secretHeaders.Header.Set(recipe.Auth.Name, "")
	}
	for name, value := range request.Headers {
		_, configuredSecret := secretHeaders.Header[http.CanonicalHeaderKey(name)]
		if configuredSecret || isSensitiveActionHeader(name) || isSensitiveActionHeaderValue(value) {
			request.Headers[name] = "[redacted]"
		}
	}
	parsed, err := url.Parse(request.URL)
	if err != nil {
		request.URL = connector.RedactURL(request.URL)
		return request
	}
	parsed.User = nil
	if recipeErr == nil && recipe.Auth.Mode == "query" && recipe.Auth.Name != "" {
		query := parsed.Query()
		query.Del(recipe.Auth.Name)
		parsed.RawQuery = query.Encode()
		parsed.ForceQuery = false
	}
	request.URL = parsed.String()
	return request
}

func isSensitiveActionHeader(name string) bool {
	name = strings.ToLower(strings.ReplaceAll(name, "_", "-"))
	for _, marker := range []string{"auth", "token", "secret", "password", "credential", "api-key", "apikey", "cookie"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func isSensitiveActionHeaderValue(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	for _, prefix := range []string{"bearer ", "basic ", "token ", "jwt "} {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func cloneActionHeaders(headers map[string]string) map[string]string {
	cloned := make(map[string]string, len(headers))
	for name, value := range headers {
		cloned[name] = value
	}
	return cloned
}

func safeActionError(config map[string]any, err error) error {
	return redactURLsInRecipeError(sanitizeRecipeError(config, err))
}

// SendAction sends one resolved request and returns only a bounded plain-text
// excerpt. Once the status line has arrived the status alone decides the
// outcome: a 2xx is success and any other status is a failure, whatever happens
// to the body afterwards. The body is read only for the excerpt, from a bounded
// prefix, and is not drained. The error never includes any response-body bytes.
func (c *Connector) SendAction(ctx context.Context, config map[string]any, action *connector.ResolvedAction) (connector.ActionResult, error) {
	if action == nil {
		return connector.ActionResult{}, fmt.Errorf("action is required")
	}
	if action.Request.Method != http.MethodPost && action.Request.Method != http.MethodPut && action.Request.Method != http.MethodPatch && action.Request.Method != http.MethodDelete {
		return connector.ActionResult{}, fmt.Errorf("action method %q is not supported", action.Request.Method)
	}
	recipe, err := recipeFromConfig(config)
	if err != nil {
		return connector.ActionResult{}, safeActionError(config, err)
	}
	baseURL, err := recipeBaseURL(config)
	if err != nil {
		return connector.ActionResult{}, safeActionError(config, err)
	}
	target, err := url.Parse(action.Request.URL)
	if err != nil || !target.IsAbs() {
		return connector.ActionResult{}, fmt.Errorf("action request URL is invalid")
	}
	if target.User != nil || target.Fragment != "" || target.RawFragment != "" {
		return connector.ActionResult{}, fmt.Errorf("action request URL is invalid")
	}
	body, err := actionBodyNodeFromRequest(action.Request.Body)
	if err != nil {
		return connector.ActionResult{}, safeActionError(config, err)
	}
	endpoint := RecipeEndpoint{
		Name:    action.Descriptor.Name,
		Method:  action.Request.Method,
		Headers: cloneActionHeaders(action.Request.Headers),
		Body:    body,
	}
	req, err := buildRecipeRequestAt(ctx, baseURL, recipe, endpoint, config, target)
	if err != nil {
		return connector.ActionResult{}, safeActionError(config, err)
	}
	// written means request bytes may have reached the service: the headers were
	// flushed, or the whole request was written. A request that fails part-way
	// through a large body still reports it.
	var written atomic.Bool
	trace := &httptrace.ClientTrace{
		WroteHeaders: func() { written.Store(true) },
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err == nil {
				written.Store(true)
			}
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	resp, err := c.recipeHTTPClient(config).Do(req) // codeql[go/request-forgery]
	result := connector.ActionResult{Written: written.Load()}
	if err != nil {
		return result, safeActionError(config, connector.MapTransportError(err))
	}
	defer resp.Body.Close() //nolint:errcheck
	result.Status = resp.StatusCode
	// The status line alone decides the outcome. The body is read only for the
	// operator's excerpt, from a small prefix: whatever was read is kept when the
	// read fails, and the rest of the body is never consumed.
	prefix, _ := io.ReadAll(io.LimitReader(resp.Body, actionExcerptBytes+utf8.UTFMax))
	result.Excerpt = actionTextExcerpt(resp.Header.Get("Content-Type"), prefix)
	if statusErr := connector.CheckStatus(resp.StatusCode, nil); statusErr != nil {
		return result, safeActionError(config, statusErr)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return result, safeActionError(config, fmt.Errorf("API returned %d (expected a 2xx status)", resp.StatusCode))
	}
	return result, nil
}

func actionTextExcerpt(contentType string, body []byte) string {
	if len(body) == 0 || !isActionText(contentType) {
		return ""
	}
	// Only the first actionExcerptBytes bytes are judged. When the body was cut
	// there, drop an incomplete trailing rune before requiring valid UTF-8.
	if len(body) >= actionExcerptBytes {
		body = body[:actionExcerptBytes]
		for drop := 0; drop < utf8.UTFMax-1 && !utf8.Valid(body); drop++ {
			body = body[:len(body)-1]
		}
	}
	if !utf8.Valid(body) {
		return ""
	}
	return dropControlCharacters(string(body))
}

func isActionText(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return true
	}
	mediaType = strings.ToLower(mediaType)
	switch {
	case strings.HasPrefix(mediaType, "image/"), strings.HasPrefix(mediaType, "audio/"), strings.HasPrefix(mediaType, "video/"),
		mediaType == "application/octet-stream", mediaType == "application/pdf", mediaType == "application/zip", mediaType == "application/gzip":
		return false
	}
	return true
}

func dropControlCharacters(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
}

func (c *Connector) runLifecycleAction(ctx context.Context, config map[string]any, verb, entityRef string) error {
	resolved, err := c.ResolveAction(config, verb, entityRef, connector.PreviousSnapshot(config))
	if err != nil {
		return err
	}
	_, err = c.SendAction(ctx, config, resolved)
	return err
}

// Restart sends the recipe-declared restart request.
func (c *Connector) Restart(ctx context.Context, config map[string]any, entityRef string) error {
	return c.runLifecycleAction(ctx, config, "restart", entityRef)
}

// Start sends the recipe-declared start request.
func (c *Connector) Start(ctx context.Context, config map[string]any, entityRef string) error {
	return c.runLifecycleAction(ctx, config, "start", entityRef)
}

// Stop sends the recipe-declared stop request.
func (c *Connector) Stop(ctx context.Context, config map[string]any, entityRef string) error {
	return c.runLifecycleAction(ctx, config, "stop", entityRef)
}

var _ connector.InstanceCapabilities = (*Connector)(nil)
var _ connector.ActionExecutor = (*Connector)(nil)
var _ connector.Restarter = (*Connector)(nil)
var _ connector.Starter = (*Connector)(nil)
var _ connector.Stopper = (*Connector)(nil)
