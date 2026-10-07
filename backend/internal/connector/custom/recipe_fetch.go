package custom

import (
	"bytes"
	"context"
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
	"github.com/WiseLabz/wiselabz/internal/httpx"
)

func (c *Connector) validateRecipeConnection(ctx context.Context, config map[string]any, recipe *Recipe) error {
	baseURL, err := recipeBaseURL(config)
	if err != nil {
		return err
	}
	if len(recipe.Endpoints) == 0 {
		return &connector.ConfigValidationError{Field: "recipe.endpoints", Message: "must contain at least one endpoint"}
	}
	req, err := buildRecipeRequest(ctx, baseURL, recipe, recipe.Endpoints[0], config)
	if err != nil {
		return err
	}
	resp, err := c.recipeHTTPClient(config).Do(req) // codeql[go/request-forgery]
	if err != nil {
		return recipeEndpointError(recipe.Endpoints[0].Name, connector.MapTransportError(err))
	}
	defer resp.Body.Close() //nolint:errcheck
	body, readErr := connector.ReadBody(resp.Body)
	if statusErr := connector.CheckStatus(resp.StatusCode, nil); statusErr != nil {
		return recipeEndpointError(recipe.Endpoints[0].Name, statusErr)
	}
	if statusErr := checkRecipeSuccessStatus(resp.StatusCode); statusErr != nil {
		return recipeEndpointError(recipe.Endpoints[0].Name, statusErr)
	}
	if readErr != nil {
		return recipeEndpointError(recipe.Endpoints[0].Name, connector.NewMalformedResponseError(readErr))
	}
	_ = body // The connection check validates reachability, not the response shape.
	return nil
}

func (c *Connector) fetchRecipe(ctx context.Context, config map[string]any, recipe *Recipe) (*connector.ServiceSnapshot, error) {
	baseURL, err := recipeBaseURL(config)
	if err != nil {
		return nil, err
	}
	if len(recipe.Endpoints) == 0 {
		return nil, &connector.ConfigValidationError{Field: "recipe.endpoints", Message: "must contain at least one endpoint"}
	}
	requests := make([]*http.Request, len(recipe.Endpoints))
	for i, endpoint := range recipe.Endpoints {
		request, err := buildRecipeRequest(ctx, baseURL, recipe, endpoint, config)
		if err != nil {
			return nil, err
		}
		requests[i] = request
	}

	client := c.recipeHTTPClient(config)
	entities := make([]connector.SnapshotEntity, 0)
	dependencies := make([]connector.ServiceDependency, 0)
	seenEntities := make(map[string]struct{})
	seenDependencies := make(map[string]struct{})
	metadata := map[string]string{"url": connector.RedactURL(baseURL.String())}
	sections := make([]connector.SnapshotSection, 0, len(recipe.Endpoints))
	for i, endpoint := range recipe.Endpoints {
		response, err := client.Do(requests[i]) // codeql[go/request-forgery]
		if err != nil {
			return nil, recipeEndpointError(endpoint.Name, connector.MapTransportError(err))
		}
		body, readErr := connector.ReadBody(response.Body)
		closeErr := response.Body.Close()
		if statusErr := connector.CheckStatus(response.StatusCode, nil); statusErr != nil {
			return nil, recipeEndpointError(endpoint.Name, statusErr)
		}
		if statusErr := checkRecipeSuccessStatus(response.StatusCode); statusErr != nil {
			return nil, recipeEndpointError(endpoint.Name, statusErr)
		}
		if readErr != nil {
			return nil, recipeEndpointError(endpoint.Name, connector.NewMalformedResponseError(readErr))
		}
		if closeErr != nil {
			return nil, recipeEndpointError(endpoint.Name, fmt.Errorf("close response body: %w", closeErr))
		}
		if !json.Valid(body) {
			return nil, recipeEndpointError(endpoint.Name, connector.NewMalformedResponseError(errors.New("response is not valid JSON")))
		}

		mapped, err := mapEndpoint(recipe, endpoint, body, seenEntities)
		if err != nil {
			return nil, recipeEndpointError(endpoint.Name, connector.NewMalformedResponseError(err))
		}
		entities = append(entities, mapped.Entities...)
		for _, dependency := range mapped.Dependencies {
			key := dependency.Kind + "\x00" + dependency.Name
			if _, ok := seenDependencies[key]; ok {
				continue
			}
			seenDependencies[key] = struct{}{}
			dependencies = append(dependencies, dependency)
		}
		prefix := "endpoint." + endpoint.Name + "."
		metadata[prefix+"status_code"] = strconv.Itoa(response.StatusCode)
		metadata[prefix+"items"] = strconv.Itoa(mapped.Items)
		metadata[prefix+"skipped"] = strconv.Itoa(mapped.Skipped)
		sections = append(sections, connector.SnapshotSection{
			Title:   endpoint.Name,
			Content: fmt.Sprintf("%d items mapped; %d items skipped", mapped.Items-mapped.Skipped, mapped.Skipped),
		})
	}

	return &connector.ServiceSnapshot{
		ServiceName:  "Custom: " + connector.RedactURL(baseURL.String()),
		Type:         typeName,
		Sections:     sections,
		Dependencies: dependencies,
		Entities:     entities,
		Metadata:     metadata,
		FetchedAt:    time.Now(),
	}, nil
}

func recipeBaseURL(config map[string]any) (*url.URL, error) {
	raw, ok := config["url"].(string)
	if !ok || strings.TrimSpace(raw) == "" {
		return nil, &connector.ConfigValidationError{Field: "url", Message: "is required"}
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return nil, &connector.ConfigValidationError{Field: "url", Message: "must be a valid HTTP URL"}
	}
	if scheme := strings.ToLower(u.Scheme); scheme != "http" && scheme != "https" {
		return nil, &connector.ConfigValidationError{Field: "url", Message: "scheme must be http or https"}
	}
	if u.Hostname() == "" || u.Opaque != "" {
		return nil, &connector.ConfigValidationError{Field: "url", Message: "must include a host"}
	}
	if u.User != nil {
		return nil, &connector.ConfigValidationError{Field: "url", Message: "must not contain credentials when a recipe is configured"}
	}
	if u.Fragment != "" {
		return nil, &connector.ConfigValidationError{Field: "url", Message: "must not contain a fragment when a recipe is configured"}
	}
	if port := u.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return nil, &connector.ConfigValidationError{Field: "url", Message: "must contain a valid port"}
		}
	}
	u.Scheme = strings.ToLower(u.Scheme)
	return u, nil
}

func buildRecipeRequest(ctx context.Context, baseURL *url.URL, recipe *Recipe, endpoint RecipeEndpoint, config map[string]any) (*http.Request, error) {
	if err := validateEndpointPath(endpoint.Path); err != nil {
		return nil, recipeEndpointError(endpoint.Name, &connector.ConfigValidationError{Field: "recipe.endpoints.path", Message: err.Error()})
	}
	reference, err := url.Parse(endpoint.Path)
	if err != nil {
		return nil, recipeEndpointError(endpoint.Name, &connector.ConfigValidationError{Field: "recipe.endpoints.path", Message: "is malformed"})
	}
	resolved := baseURL.ResolveReference(reference)
	if !sameOrigin(baseURL, resolved) {
		return nil, fmt.Errorf("endpoint %q resolves outside connector origin: %s", endpoint.Name, connector.RedactURL(resolved.String()))
	}
	query := baseURL.Query()
	for key, values := range reference.Query() {
		query[key] = append([]string(nil), values...)
	}
	for key, value := range endpoint.Query {
		query.Set(key, value)
	}
	if recipe.Auth.Mode == "query" {
		token, _ := config["auth_token"].(string)
		query.Set(recipe.Auth.Name, token)
	}
	resolved.RawQuery = query.Encode()
	resolved.ForceQuery = false
	body, err := recipeBody(endpoint)
	if err != nil {
		return nil, recipeEndpointError(endpoint.Name, &connector.ConfigValidationError{Field: "recipe.endpoints.body", Message: "must be a static JSON value"})
	}
	var requestBodyReader io.Reader
	if body != nil {
		requestBodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, endpoint.Method, resolved.String(), requestBodyReader)
	if err != nil {
		return nil, recipeEndpointError(endpoint.Name, fmt.Errorf("cannot create request for %s", connector.RedactURL(resolved.String())))
	}
	setHeaders(req, config)
	for name, value := range endpoint.Headers {
		req.Header.Set(name, value)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	switch recipe.Auth.Mode {
	case "header":
		token, _ := config["auth_token"].(string)
		req.Header.Set(recipe.Auth.Name, recipe.Auth.Prefix+token)
	case "basic":
		username, _ := config["auth_username"].(string)
		password, _ := config["auth_password"].(string)
		req.SetBasicAuth(username, password)
	}
	return req, nil
}

func sameOrigin(base, target *url.URL) bool {
	if base == nil || target == nil || !strings.EqualFold(base.Scheme, target.Scheme) || !strings.EqualFold(base.Hostname(), target.Hostname()) {
		return false
	}
	return normalizedPort(base) == normalizedPort(target)
}

func normalizedPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func (c *Connector) recipeHTTPClient(config map[string]any) *http.Client {
	client := c.client
	if skipTLS, ok := config["verify_tls"].(bool); ok && !skipTLS {
		client = connector.NewHTTPClient(connector.HTTPClientOptions{SkipTLSVerify: true})
	}
	if client == nil {
		client = connector.NewHTTPClient(connector.HTTPClientOptions{})
	}
	recipeClient := *client
	recipeClient.Transport = httpx.Unwrap(client.Transport)
	recipeClient.CheckRedirect = httpx.NoRedirect
	return &recipeClient
}

func checkRecipeSuccessStatus(status int) error {
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		return nil
	}
	return fmt.Errorf("API returned %d (expected a 2xx status)", status)
}

func recipeEndpointError(name string, err error) error {
	return fmt.Errorf("endpoint %q: %w", name, err)
}

type sanitizedRecipeError struct {
	message string
	err     error
}

func (e *sanitizedRecipeError) Error() string { return e.message }
func (e *sanitizedRecipeError) Unwrap() error { return e.err }

func sanitizeRecipeError(config map[string]any, err error) error {
	if err == nil {
		return nil
	}
	type secret struct{ value string }
	secrets := make([]secret, 0, 3)
	for _, key := range []string{"auth_token", "auth_username", "auth_password"} {
		if value, ok := config[key].(string); ok && value != "" {
			secrets = append(secrets, secret{value: value})
		}
	}
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i].value) > len(secrets[j].value) })
	message := err.Error()
	for _, secret := range secrets {
		message = strings.ReplaceAll(message, secret.value, "[REDACTED]")
	}
	if message == err.Error() {
		return err
	}
	return &sanitizedRecipeError{message: message, err: err}
}
