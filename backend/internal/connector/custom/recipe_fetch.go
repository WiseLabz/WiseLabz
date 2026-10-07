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
	"github.com/tidwall/gjson"
)

const (
	maxRecipePages    = 100
	maxRecipeEntities = 10_000
	maxPreviewSamples = 20
	// maxRecipePaginationValueBytes bounds a cursor or next link taken from a
	// response. Each one is remembered to detect cycles and sent back to the
	// server, so an unbounded value could pin up to a response body per page.
	maxRecipePaginationValueBytes = 8 << 10
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
	run, err := c.runRecipe(ctx, config, recipe, false)
	if err != nil {
		return nil, err
	}

	return &connector.ServiceSnapshot{
		ServiceName:  "Custom: " + connector.RedactURL(run.baseURL.String()),
		Type:         typeName,
		Sections:     run.sections,
		Dependencies: run.dependencies,
		Entities:     run.entities,
		Metadata:     run.metadata,
		FetchedAt:    time.Now(),
	}, nil
}

// RecipePreviewResult contains mapped preview data without persisting a snapshot.
type RecipePreviewResult struct {
	Endpoints    []RecipeEndpointPreview       `json:"endpoints"`
	Dependencies []connector.ServiceDependency `json:"dependencies"`
	Errors       []string                      `json:"errors"`
}

// RecipeEndpointPreview summarizes one endpoint's preview run.
type RecipeEndpointPreview struct {
	Name         string                        `json:"name"`
	Items        int                           `json:"items"`
	Count        int                           `json:"count"`
	Skipped      int                           `json:"skipped"`
	Samples      []connector.SnapshotEntity    `json:"samples"`
	Dependencies []connector.ServiceDependency `json:"dependencies"`
	Error        string                        `json:"error,omitempty"`
}

type recipeRunResult struct {
	baseURL      *url.URL
	entities     []connector.SnapshotEntity
	dependencies []connector.ServiceDependency
	metadata     map[string]string
	sections     []connector.SnapshotSection
	preview      RecipePreviewResult
}

type recipeRunState struct {
	seenEntities      map[string]struct{}
	seenDependencies  map[string]struct{}
	entityCount       int
	mappedOutputBytes int
	entities          []connector.SnapshotEntity
	dependencies      []connector.ServiceDependency
}

// PreviewRecipe validates and runs a recipe without persisting connector state.
func (c *Connector) PreviewRecipe(ctx context.Context, config map[string]any) (*RecipePreviewResult, error) {
	if err := validateCustomConfig(config); err != nil {
		return nil, redactURLsInRecipeError(err)
	}
	recipe, err := recipeFromConfig(config)
	if err != nil {
		return nil, err
	}
	run, err := c.runRecipe(ctx, config, recipe, true)
	if err != nil {
		return nil, redactURLsInRecipeError(sanitizeRecipeError(config, err))
	}
	return &run.preview, nil
}

func (c *Connector) runRecipe(ctx context.Context, config map[string]any, recipe *Recipe, preview bool) (*recipeRunResult, error) {
	baseURL, err := recipeBaseURL(config)
	if err != nil {
		return nil, err
	}
	if len(recipe.Endpoints) == 0 {
		return nil, &connector.ConfigValidationError{Field: "recipe.endpoints", Message: "must contain at least one endpoint"}
	}
	initialRequests := make([]*http.Request, len(recipe.Endpoints))
	for i, endpoint := range recipe.Endpoints {
		request, err := buildRecipeRequest(ctx, baseURL, recipe, endpoint, config)
		if err != nil {
			return nil, err
		}
		initialRequests[i] = request
	}
	run := &recipeRunResult{
		baseURL:  baseURL,
		metadata: map[string]string{"url": connector.RedactURL(baseURL.String())},
		sections: make([]connector.SnapshotSection, 0, len(recipe.Endpoints)),
		preview: RecipePreviewResult{
			Endpoints: make([]RecipeEndpointPreview, 0, len(recipe.Endpoints)),
			Errors:    make([]string, 0),
		},
	}
	state := &recipeRunState{
		seenEntities:     make(map[string]struct{}),
		seenDependencies: make(map[string]struct{}),
		entities:         make([]connector.SnapshotEntity, 0),
		dependencies:     make([]connector.ServiceDependency, 0),
	}
	client := c.recipeHTTPClient(config)
	for i, endpoint := range recipe.Endpoints {
		sampleLimit := 0
		if preview {
			sampleLimit = maxPreviewSamples
		}
		endpointRun, err := runRecipeEndpoint(ctx, recipeEndpointRunInput{
			baseURL:     baseURL,
			client:      client,
			config:      config,
			recipe:      recipe,
			endpoint:    endpoint,
			request:     initialRequests[i],
			state:       state,
			preview:     preview,
			sampleLimit: sampleLimit,
		})
		if err != nil {
			if !preview {
				return nil, err
			}
			endpointRun.preview.Error = redactURLsInRecipeError(sanitizeRecipeError(config, err)).Error()
			run.preview.Errors = append(run.preview.Errors, endpointRun.preview.Error)
		}
		if preview {
			run.preview.Endpoints = append(run.preview.Endpoints, endpointRun.preview)
		}
		prefix := "endpoint." + endpoint.Name + "."
		run.metadata[prefix+"status_code"] = strconv.Itoa(endpointRun.statusCode)
		run.metadata[prefix+"items"] = strconv.Itoa(endpointRun.preview.Items)
		run.metadata[prefix+"skipped"] = strconv.Itoa(endpointRun.preview.Skipped)
		run.sections = append(run.sections, connector.SnapshotSection{
			Title:   endpoint.Name,
			Content: fmt.Sprintf("%d items mapped; %d items skipped", endpointRun.preview.Count, endpointRun.preview.Skipped),
		})
	}
	run.entities = state.entities
	run.dependencies = state.dependencies
	run.preview.Dependencies = state.dependencies
	return run, nil
}

type recipeEndpointRunResult struct {
	preview    RecipeEndpointPreview
	statusCode int
}

type recipeEndpointRunInput struct {
	baseURL     *url.URL
	client      *http.Client
	config      map[string]any
	recipe      *Recipe
	endpoint    RecipeEndpoint
	request     *http.Request
	state       *recipeRunState
	preview     bool
	sampleLimit int
}

type fetchedRecipePage struct {
	response *http.Response
	body     []byte
}

func runRecipeEndpoint(ctx context.Context, input recipeEndpointRunInput) (recipeEndpointRunResult, error) {
	result := recipeEndpointRunResult{preview: RecipeEndpointPreview{
		Name:         input.endpoint.Name,
		Dependencies: make([]connector.ServiceDependency, 0),
	}}
	if input.preview {
		result.preview.Samples = make([]connector.SnapshotEntity, 0, input.sampleLimit)
	}
	strategy, err := newRecipePaginationStrategy(input.baseURL, input.recipe, input.endpoint, input.config, input.request)
	if err != nil {
		return result, recipeEndpointError(input.endpoint.Name, err)
	}
	var endpointDependencies map[string]struct{}
	if input.preview {
		endpointDependencies = make(map[string]struct{})
	}
	for pageNumber := 1; ; pageNumber++ {
		fail := func(err error) error {
			if input.endpoint.Pagination != nil {
				err = fmt.Errorf("page %d: %w", pageNumber, err)
			}
			return recipeEndpointError(input.endpoint.Name, err)
		}
		if err := ctx.Err(); err != nil {
			return result, fail(connector.MapTransportError(err))
		}
		page, err := fetchRecipePage(ctx, input.client, input.request)
		if err != nil {
			return result, fail(err)
		}
		pageItems, err := input.acceptPage(pageNumber, page.body, &result, endpointDependencies)
		if err != nil {
			return result, fail(err)
		}
		result.statusCode = page.response.StatusCode

		next, hasNext, err := strategy.next(ctx, recipePageResponse{
			request: input.request,
			header:  page.response.Header,
			body:    page.body,
			items:   pageItems,
		})
		if ctx.Err() != nil {
			return result, fail(connector.MapTransportError(ctx.Err()))
		}
		if err != nil {
			return result, fail(err)
		}
		if !hasNext {
			break
		}
		if pageNumber > maxRecipePages {
			return result, fail(fmt.Errorf("pagination exceeds the %d-page limit", maxRecipePages))
		}
		input.request = next
	}
	return result, nil
}

func fetchRecipePage(ctx context.Context, client *http.Client, request *http.Request) (fetchedRecipePage, error) {
	response, err := client.Do(request) // codeql[go/request-forgery]
	if err != nil {
		return fetchedRecipePage{}, connector.MapTransportError(err)
	}
	body, readErr := connector.ReadBody(response.Body)
	closeErr := response.Body.Close()
	if statusErr := connector.CheckStatus(response.StatusCode, nil); statusErr != nil {
		return fetchedRecipePage{}, statusErr
	}
	if statusErr := checkRecipeSuccessStatus(response.StatusCode); statusErr != nil {
		return fetchedRecipePage{}, statusErr
	}
	if readErr != nil {
		if ctx.Err() != nil {
			return fetchedRecipePage{}, connector.MapTransportError(ctx.Err())
		}
		return fetchedRecipePage{}, connector.NewMalformedResponseError(readErr)
	}
	if closeErr != nil {
		return fetchedRecipePage{}, fmt.Errorf("close response body: %w", closeErr)
	}
	if !json.Valid(body) {
		return fetchedRecipePage{}, connector.NewMalformedResponseError(errors.New("response is not valid JSON"))
	}
	return fetchedRecipePage{response: response, body: body}, nil
}

func (input recipeEndpointRunInput) acceptPage(pageNumber int, body []byte, result *recipeEndpointRunResult, endpointDependencies map[string]struct{}) (int, error) {
	mapped, err := mapEndpointBudgeted(input.recipe, input.endpoint, body, input.state.seenEntities, connector.MaxResponseBytes, input.state.mappedOutputBytes)
	if err != nil {
		return 0, connector.NewMalformedResponseError(err)
	}
	// Page maxRecipePages+1 is only an end probe: it may confirm that
	// pagination ended, but it must not return more data.
	var capErr error
	if pageNumber > maxRecipePages && mapped.Items > 0 {
		capErr = fmt.Errorf("pagination exceeds the %d-page limit", maxRecipePages)
	}
	if capErr == nil && input.state.entityCount+len(mapped.Entities) > maxRecipeEntities {
		capErr = fmt.Errorf("recipe exceeds the %d-entity limit", maxRecipeEntities)
	}
	pageOutputBytes := recipeEntitiesSize(mapped.Entities)
	for _, dependency := range mapped.Dependencies {
		key := dependency.Kind + "\x00" + dependency.Name
		if _, ok := input.state.seenDependencies[key]; !ok {
			pageOutputBytes += len(dependency.Kind) + len(dependency.Name)
		}
		if input.preview {
			if _, ok := endpointDependencies[key]; !ok {
				pageOutputBytes += len(dependency.Kind) + len(dependency.Name)
			}
		}
	}
	if capErr == nil && input.state.mappedOutputBytes+pageOutputBytes > connector.MaxResponseBytes {
		capErr = fmt.Errorf("recipe maps more than %d bytes of data", connector.MaxResponseBytes)
	}
	if capErr != nil {
		rollbackRecipeEntities(input.state.seenEntities, mapped.Entities)
		return 0, capErr
	}

	input.state.mappedOutputBytes += pageOutputBytes
	input.state.entityCount += len(mapped.Entities)
	result.preview.Items += mapped.Items
	result.preview.Count += len(mapped.Entities)
	result.preview.Skipped += mapped.Skipped
	if input.preview {
		for _, entity := range mapped.Entities {
			if len(result.preview.Samples) >= input.sampleLimit {
				break
			}
			result.preview.Samples = append(result.preview.Samples, entity)
		}
	} else {
		input.state.entities = append(input.state.entities, mapped.Entities...)
	}
	for _, dependency := range mapped.Dependencies {
		key := dependency.Kind + "\x00" + dependency.Name
		if input.preview {
			if _, ok := endpointDependencies[key]; !ok {
				endpointDependencies[key] = struct{}{}
				result.preview.Dependencies = append(result.preview.Dependencies, dependency)
			}
		}
		if _, ok := input.state.seenDependencies[key]; ok {
			continue
		}
		input.state.seenDependencies[key] = struct{}{}
		input.state.dependencies = append(input.state.dependencies, dependency)
	}
	return mapped.Items, nil
}

func rollbackRecipeEntities(seen map[string]struct{}, entities []connector.SnapshotEntity) {
	for _, entity := range entities {
		delete(seen, entity.Kind+"\x00"+entity.ExternalID)
	}
}

type recipePageResponse struct {
	request *http.Request
	header  http.Header
	body    []byte
	items   int
}

type recipePaginationStrategy interface {
	next(context.Context, recipePageResponse) (*http.Request, bool, error)
}

type singlePageStrategy struct{}

func (singlePageStrategy) next(context.Context, recipePageResponse) (*http.Request, bool, error) {
	return nil, false, nil
}

type numericPaginationStrategy struct {
	baseURL    *url.URL
	recipe     *Recipe
	endpoint   RecipeEndpoint
	config     map[string]any
	pagination RecipePagination
	value      int
	step       int
}

func (s *numericPaginationStrategy) next(ctx context.Context, previous recipePageResponse) (*http.Request, bool, error) {
	if previous.items == 0 {
		return nil, false, nil
	}
	maxInt := int(^uint(0) >> 1)
	if s.value > maxInt-s.step {
		return nil, false, errors.New("pagination value exceeds the integer limit")
	}
	value := s.value + s.step
	request, err := buildRecipeRequestForPaginationValue(
		ctx,
		s.baseURL,
		s.recipe,
		s.endpoint,
		s.config,
		previous.request.URL,
		s.pagination,
		value,
	)
	if err != nil {
		return nil, false, err
	}
	s.value = value
	return request, true, nil
}

type cursorPaginationStrategy struct {
	baseURL    *url.URL
	recipe     *Recipe
	endpoint   RecipeEndpoint
	config     map[string]any
	pagination RecipePagination
	sent       string
	seen       map[string]struct{}
}

func (s *cursorPaginationStrategy) next(ctx context.Context, previous recipePageResponse) (*http.Request, bool, error) {
	cursor, found, err := recipePathString(previous.body, s.pagination.CursorPath)
	if err != nil {
		return nil, false, fmt.Errorf("cursor path did not resolve to a scalar: %w", err)
	}
	if !found || cursor == "" {
		return nil, false, nil
	}
	if len(cursor) > maxRecipePaginationValueBytes {
		return nil, false, fmt.Errorf("cursor exceeds the %d-byte limit", maxRecipePaginationValueBytes)
	}
	if cursor == s.sent {
		return nil, false, errors.New("cursor did not advance")
	}
	if _, exists := s.seen[cursor]; exists {
		return nil, false, errors.New("cursor repeated")
	}
	request, err := buildRecipeRequestForPaginationValue(
		ctx,
		s.baseURL,
		s.recipe,
		s.endpoint,
		s.config,
		previous.request.URL,
		s.pagination,
		cursor,
	)
	if err != nil {
		return nil, false, err
	}
	s.sent = cursor
	s.seen[cursor] = struct{}{}
	return request, true, nil
}

type nextLinkPaginationStrategy struct {
	baseURL    *url.URL
	recipe     *Recipe
	endpoint   RecipeEndpoint
	config     map[string]any
	pagination RecipePagination
	seen       map[string]struct{}
}

func (s *nextLinkPaginationStrategy) next(ctx context.Context, previous recipePageResponse) (*http.Request, bool, error) {
	var link string
	var found bool
	if s.pagination.LinkHeader {
		link, found = recipeNextLink(previous.header)
	} else {
		result := jsonPathValue(previous.body, s.pagination.NextPath)
		if !result.Exists() || result.Type == gjson.Null {
			return nil, false, nil
		}
		if result.Type != gjson.String {
			return nil, false, errors.New("next link path must resolve to a string")
		}
		link, found = result.Str, true
	}
	if !found || strings.TrimSpace(link) == "" {
		return nil, false, nil
	}
	if len(strings.TrimSpace(link)) > maxRecipePaginationValueBytes {
		return nil, false, fmt.Errorf("next link exceeds the %d-byte limit", maxRecipePaginationValueBytes)
	}
	request, err := buildRecipeRequestForNextLink(ctx, s.baseURL, s.recipe, s.endpoint, s.config, link)
	if err != nil {
		return nil, false, err
	}
	key := recipeRequestURLKey(request.URL)
	if _, exists := s.seen[key]; exists {
		return nil, false, errors.New("next link repeated")
	}
	s.seen[key] = struct{}{}
	return request, true, nil
}

func newRecipePaginationStrategy(baseURL *url.URL, recipe *Recipe, endpoint RecipeEndpoint, config map[string]any, initial *http.Request) (recipePaginationStrategy, error) {
	pagination := endpoint.Pagination
	if pagination == nil {
		return singlePageStrategy{}, nil
	}
	switch pagination.Type {
	case "page", "offset":
		start := pagination.Start
		if pagination.Type == "page" && !pagination.HasStart {
			start = 1
		}
		step := 1
		if pagination.Type == "offset" {
			step = pagination.Size
		}
		return &numericPaginationStrategy{
			baseURL:    baseURL,
			recipe:     recipe,
			endpoint:   endpoint,
			config:     config,
			pagination: *pagination,
			value:      start,
			step:       step,
		}, nil
	case "cursor":
		initialCursor := initial.URL.Query().Get(pagination.Param)
		seen := make(map[string]struct{})
		if initialCursor != "" {
			seen[initialCursor] = struct{}{}
		}
		return &cursorPaginationStrategy{
			baseURL:    baseURL,
			recipe:     recipe,
			endpoint:   endpoint,
			config:     config,
			pagination: *pagination,
			sent:       initialCursor,
			seen:       seen,
		}, nil
	case "next_link":
		return &nextLinkPaginationStrategy{
			baseURL:    baseURL,
			recipe:     recipe,
			endpoint:   endpoint,
			config:     config,
			pagination: *pagination,
			seen:       map[string]struct{}{recipeRequestURLKey(initial.URL): {}},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported pagination type %q", pagination.Type)
	}
}

func recipeEntitiesSize(entities []connector.SnapshotEntity) int {
	size := 0
	for _, entity := range entities {
		size += len(entity.Kind) + len(entity.Name) + len(entity.ExternalID) + len(entity.IP) + len(entity.Hostname) + len(entity.MAC)
		for _, alias := range entity.Aliases {
			size += len(alias)
		}
		for name, value := range entity.Attributes {
			size += len(name) + recipeValueSize(value)
		}
	}
	return size
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
	setInitialPaginationQuery(query, endpoint.Pagination)
	if recipe.Auth.Mode == "query" {
		token, _ := config["auth_token"].(string)
		query.Set(recipe.Auth.Name, token)
	}
	resolved.RawQuery = query.Encode()
	resolved.ForceQuery = false
	return buildRecipeRequestAt(ctx, baseURL, recipe, endpoint, config, resolved)
}

func setInitialPaginationQuery(query url.Values, pagination *RecipePagination) {
	if pagination == nil {
		return
	}
	if pagination.Type == "cursor" {
		if pagination.SizeParam != "" {
			query.Set(pagination.SizeParam, strconv.Itoa(pagination.Size))
		}
		return
	}
	if pagination.Type != "page" && pagination.Type != "offset" {
		return
	}
	start := pagination.Start
	if pagination.Type == "page" && !pagination.HasStart {
		start = 1
	}
	query.Set(pagination.Param, strconv.Itoa(start))
	if pagination.SizeParam != "" {
		query.Set(pagination.SizeParam, strconv.Itoa(pagination.Size))
	}
}

func buildRecipeRequestForPaginationValue(
	ctx context.Context,
	baseURL *url.URL,
	recipe *Recipe,
	endpoint RecipeEndpoint,
	config map[string]any,
	previousURL *url.URL,
	pagination RecipePagination,
	value any,
) (*http.Request, error) {
	resolved := *previousURL
	query := resolved.Query()
	query.Set(pagination.Param, fmt.Sprint(value))
	if pagination.SizeParam != "" {
		query.Set(pagination.SizeParam, strconv.Itoa(pagination.Size))
	}
	resolved.RawQuery = query.Encode()
	return buildRecipeRequestAt(ctx, baseURL, recipe, endpoint, config, &resolved)
}

func buildRecipeRequestForNextLink(
	ctx context.Context,
	baseURL *url.URL,
	recipe *Recipe,
	endpoint RecipeEndpoint,
	config map[string]any,
	rawLink string,
) (*http.Request, error) {
	reference, err := url.Parse(strings.TrimSpace(rawLink))
	if err != nil {
		return nil, fmt.Errorf("next link is malformed")
	}
	if reference.User != nil || reference.Fragment != "" || reference.RawFragment != "" || strings.Contains(rawLink, "#") {
		return nil, fmt.Errorf("next link must not contain user information or a fragment")
	}
	resolved := baseURL.ResolveReference(reference)
	if !sameOrigin(baseURL, resolved) {
		return nil, fmt.Errorf("next link resolves outside connector origin: %s", connector.RedactURL(resolved.String()))
	}

	query := baseURL.Query()
	endpointReference, err := url.Parse(endpoint.Path)
	if err != nil {
		return nil, fmt.Errorf("endpoint path is malformed")
	}
	for key, values := range endpointReference.Query() {
		query[key] = append([]string(nil), values...)
	}
	for key, value := range endpoint.Query {
		query.Set(key, value)
	}
	for key, values := range reference.Query() {
		query[key] = append([]string(nil), values...)
	}
	resolved.RawQuery = query.Encode()
	return buildRecipeRequestAt(ctx, baseURL, recipe, endpoint, config, resolved)
}

func buildRecipeRequestAt(ctx context.Context, baseURL *url.URL, recipe *Recipe, endpoint RecipeEndpoint, config map[string]any, target *url.URL) (*http.Request, error) {
	resolved := *target
	if !sameOrigin(baseURL, &resolved) {
		return nil, fmt.Errorf("endpoint %q resolves outside connector origin: %s", endpoint.Name, connector.RedactURL(resolved.String()))
	}
	query := resolved.Query()
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

// SameOrigin reports whether two absolute URLs share scheme, host and effective
// port. Unparsable URLs never match.
func SameOrigin(a, b string) bool {
	base, err := url.Parse(a)
	if err != nil {
		return false
	}
	target, err := url.Parse(b)
	if err != nil {
		return false
	}
	return sameOrigin(base, target)
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

func recipeRequestURLKey(u *url.URL) string {
	canonical := *u
	canonical.Scheme = strings.ToLower(canonical.Scheme)
	canonical.Host = strings.ToLower(canonical.Host)
	canonical.RawQuery = canonical.Query().Encode()
	canonical.ForceQuery = false
	canonical.Fragment = ""
	canonical.RawFragment = ""
	return canonical.String()
}

func recipeNextLink(header http.Header) (string, bool) {
	for _, value := range header.Values("Link") {
		for _, segment := range splitLinkHeader(value) {
			href, relation := parseRecipeLinkSegment(segment)
			for _, rel := range strings.Fields(relation) {
				if strings.EqualFold(rel, "next") && href != "" {
					return href, true
				}
			}
		}
	}
	return "", false
}

func splitLinkHeader(value string) []string {
	var segments []string
	start := 0
	inAngle, inQuote, escaped := false, false, false
	for i, r := range value {
		if inQuote {
			if escaped {
				escaped = false
				continue
			}
			switch r {
			case '\\':
				escaped = true
			case '"':
				inQuote = false
			}
			continue
		}
		switch r {
		case '"':
			inQuote = true
		case '<':
			inAngle = true
		case '>':
			inAngle = false
		case ',':
			if !inAngle {
				segments = append(segments, strings.TrimSpace(value[start:i]))
				start = i + 1
			}
		}
	}
	segments = append(segments, strings.TrimSpace(value[start:]))
	return segments
}

func parseRecipeLinkSegment(segment string) (string, string) {
	segment = strings.TrimSpace(segment)
	if !strings.HasPrefix(segment, "<") {
		return "", ""
	}
	closingAngle := strings.IndexByte(segment, '>')
	if closingAngle < 1 {
		return "", ""
	}
	href := segment[1:closingAngle]
	var relation string
	for _, parameter := range splitLinkParameters(segment[closingAngle+1:]) {
		name, value, ok := strings.Cut(strings.TrimSpace(parameter), "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(name), "rel") {
			continue
		}
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "\"") {
			if len(value) < 2 || !strings.HasSuffix(value, "\"") {
				continue
			}
			decoded, err := strconv.Unquote(value)
			if err != nil {
				continue
			}
			value = decoded
		}
		relation = value
	}
	return href, relation
}

func splitLinkParameters(value string) []string {
	var parameters []string
	start := 0
	inQuote, escaped := false, false
	for i, r := range value {
		if inQuote {
			if escaped {
				escaped = false
				continue
			}
			switch r {
			case '\\':
				escaped = true
			case '"':
				inQuote = false
			}
			continue
		}
		switch r {
		case '"':
			inQuote = true
		case ';':
			parameters = append(parameters, value[start:i])
			start = i + 1
		}
	}
	parameters = append(parameters, value[start:])
	return parameters
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
