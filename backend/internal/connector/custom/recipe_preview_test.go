package custom

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestPreviewRecipeContinuesAfterEndpointFailureAndLimitsSamples(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	var firstResponse strings.Builder
	firstResponse.WriteString(`{"items":[`)
	for i := range 25 {
		if i > 0 {
			firstResponse.WriteByte(',')
		}
		fmt.Fprintf(&firstResponse, `{"id":"item-%d","name":"Item %d","active":true}`, i, i)
	}
	firstResponse.WriteString(`]}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/first":
			if _, err := w.Write([]byte(firstResponse.String())); err != nil {
				t.Error(err)
			}
		case "/broken":
			http.Error(w, "fixture endpoint failure", http.StatusServiceUnavailable)
		case "/last":
			if _, err := w.Write([]byte(`{"items":[{"id":"last-item","name":"Last item"}]}`)); err != nil {
				t.Error(err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	config := map[string]any{
		"url": server.URL,
		"recipe": `version: 1
category: other
auth: {mode: none}
dependencies:
  - {kind: network, const: recipe-network}
endpoints:
  - name: first
    path: /first
    method: GET
    items: items
    entity: {kind: record, name: name, external_id: id, attributes: {active: {path: active, type: bool}}}
    dependencies:
      - {kind: storage, const: first-root}
  - name: broken
    path: /broken
    method: GET
    items: items
    entity: {kind: record, name: name, external_id: id}
  - name: last
    path: /last
    method: GET
    items: items
    entity: {kind: record, name: name, external_id: id}
    dependencies:
      - {kind: upstream_service, const: last-source}
`,
	}
	instance, err := connector.Get(typeName, config)
	if err != nil {
		t.Fatal(err)
	}
	customConnector, ok := instance.(*Connector)
	if !ok {
		t.Fatalf("connector type = %T, want *Connector", instance)
	}
	result, err := customConnector.PreviewRecipe(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Endpoints) != 3 {
		t.Fatalf("endpoint results = %d, want 3", len(result.Endpoints))
	}
	if got := result.Endpoints[0]; got.Items != 25 || got.Count != 25 || got.Skipped != 0 {
		t.Fatalf("first endpoint counts = items %d, count %d, skipped %d", got.Items, got.Count, got.Skipped)
	}
	if len(result.Endpoints[0].Samples) != 18 || result.Endpoints[0].Samples[0].Attributes["active"] != true {
		t.Fatalf("first endpoint samples = %d, first sample %#v", len(result.Endpoints[0].Samples), result.Endpoints[0].Samples[0])
	}
	if result.Endpoints[1].Error == "" || !strings.Contains(result.Endpoints[1].Error, `endpoint "broken"`) {
		t.Fatalf("broken endpoint error = %q", result.Endpoints[1].Error)
	}
	if got := result.Endpoints[2]; got.Count != 1 || len(got.Samples) != 1 || got.Samples[0].Name != "Last item" {
		t.Fatalf("last endpoint result = %+v", got)
	}
	totalSamples := 0
	for _, endpoint := range result.Endpoints {
		totalSamples += len(endpoint.Samples)
	}
	if totalSamples > maxPreviewSamples {
		t.Fatalf("total samples = %d, limit %d", totalSamples, maxPreviewSamples)
	}
	if len(result.Errors) != 1 || len(result.Dependencies) != 3 {
		t.Fatalf("errors/dependencies = %#v/%#v", result.Errors, result.Dependencies)
	}
	if len(result.Endpoints[0].Dependencies) != 2 || len(result.Endpoints[2].Dependencies) != 2 {
		t.Fatalf("endpoint dependencies = %#v / %#v", result.Endpoints[0].Dependencies, result.Endpoints[2].Dependencies)
	}
}

func TestPreviewRecipeCanceledContextReportsEveryEndpointWithoutRequests(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()
	config := map[string]any{
		"url": server.URL,
		"recipe": `version: 1
category: other
auth: {mode: none}
endpoints:
  - name: first
    path: /first
    method: GET
    items: items
    entity: {kind: record, name: name, external_id: id}
  - name: second
    path: /second
    method: GET
    items: items
    entity: {kind: record, name: name, external_id: id}
`,
	}
	instance, err := connector.Get(typeName, config)
	if err != nil {
		t.Fatal(err)
	}
	customConnector, ok := instance.(*Connector)
	if !ok {
		t.Fatalf("connector type = %T, want *Connector", instance)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := customConnector.PreviewRecipe(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 0 || len(result.Endpoints) != 2 {
		t.Fatalf("requests/results = %d/%d, want 0/2", requests.Load(), len(result.Endpoints))
	}
	for _, endpoint := range result.Endpoints {
		if endpoint.Error == "" || endpoint.Items != 0 || endpoint.Count != 0 {
			t.Errorf("canceled endpoint result = %+v", endpoint)
		}
	}
}

func TestPreviewRecipeStillBlocksLoopbackFromGuardedFactory(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()
	config := map[string]any{
		"url": server.URL,
		"recipe": `version: 1
category: other
auth: {mode: none}
endpoints:
  - name: local
    path: /items
    method: GET
    items: items
    entity: {kind: record, name: name, external_id: id}
`,
	}
	instance, err := connector.Get(typeName, config)
	if err != nil {
		t.Fatal(err)
	}
	customConnector, ok := instance.(*Connector)
	if !ok {
		t.Fatalf("connector type = %T, want *Connector", instance)
	}
	result, err := customConnector.PreviewRecipe(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 0 || len(result.Endpoints) != 1 {
		t.Fatalf("requests/results = %d/%d, want 0/1", requests.Load(), len(result.Endpoints))
	}
	if !strings.Contains(strings.ToLower(result.Endpoints[0].Error), "blocked") {
		t.Fatalf("loopback endpoint error = %q, want blocked address", result.Endpoints[0].Error)
	}
}

func TestPreviewRecipeFailedPageDoesNotReserveIdentifiers(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/paged":
			switch r.URL.Query().Get("page") {
			case "1":
				_, _ = w.Write([]byte(`{"items":[{"id":"page-one","name":"Page one"}]}`))
			case "2":
				_, _ = w.Write([]byte(`{"items":[{"id":"new-page-two","name":"New item"},{"id":"page-one","name":"Duplicate"}]}`))
			default:
				http.Error(w, "unexpected page", http.StatusBadRequest)
			}
		case "/after":
			_, _ = w.Write([]byte(`{"items":[{"id":"new-page-two","name":"Accepted later"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	config := map[string]any{
		"url": server.URL,
		"recipe": `version: 1
category: other
auth: {mode: none}
endpoints:
  - name: paged
    path: /paged
    method: GET
    items: items
    pagination: {type: page, param: page, size_param: pageSize, size: 1}
    entity: {kind: record, name: name, external_id: id}
  - name: after
    path: /after
    method: GET
    items: items
    entity: {kind: record, name: name, external_id: id}
`,
	}
	instance, err := connector.Get(typeName, config)
	if err != nil {
		t.Fatal(err)
	}
	customConnector, ok := instance.(*Connector)
	if !ok {
		t.Fatalf("connector type = %T, want *Connector", instance)
	}
	result, err := customConnector.PreviewRecipe(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Endpoints) != 2 || result.Endpoints[0].Error == "" || result.Endpoints[0].Count != 1 {
		t.Fatalf("paged endpoint = %+v", result.Endpoints)
	}
	if got := result.Endpoints[1]; got.Error != "" || got.Count != 1 || len(got.Samples) != 1 || got.Samples[0].Name != "Accepted later" {
		t.Fatalf("later endpoint = %+v", got)
	}
}
