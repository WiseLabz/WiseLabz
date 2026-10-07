package custom_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/connector"
	_ "github.com/WiseLabz/wiselabz/internal/connector/custom"
	"go.yaml.in/yaml/v3"
)

func TestDocumentedDeclaredRecipe(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	doc, err := os.ReadFile("../../../../docs/CONNECTORS_IN_CONFIG.md")
	if err != nil {
		t.Fatal(err)
	}
	_, after, ok := strings.Cut(string(doc), "<!-- custom-rest-recipe-example -->")
	if !ok {
		t.Fatal("missing example marker")
	}
	_, after, ok = strings.Cut(after, "```yaml\n")
	if !ok {
		t.Fatal("missing yaml fence")
	}
	snippet, _, ok := strings.Cut(after, "```")
	if !ok {
		t.Fatal("missing closing fence")
	}
	var parsed struct {
		Connectors []struct {
			Name, Type, URL string
			Config          map[string]any
		}
	}
	if err := yaml.Unmarshal([]byte(snippet), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Connectors) != 1 {
		t.Fatalf("connectors %d", len(parsed.Connectors))
	}
	entry := parsed.Connectors[0]
	t.Setenv("LIBRARY_TOKEN", "documented-secret")
	declared := (&config.Config{Connectors: []config.ConnectorEntry{{Name: entry.Name, Type: entry.Type, URL: entry.URL, Config: entry.Config}}}).ResolveConnectors()[0]
	if declared.Err != nil {
		t.Fatal(declared.Err)
	}
	if err := connector.ValidateDeclared(declared.Type, declared.Config); err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/api/items" || r.Header.Get("X-Api-Key") != "documented-secret" {
			t.Errorf("request path/auth %s/%s", r.URL.Path, r.Header.Get("X-Api-Key"))
		}
		_, err := w.Write([]byte(`{"items":[{"id":1,"title":"One","enabled":true,"downloadClient":"downloader"},{"id":2,"title":"Two","enabled":false,"downloadClient":"downloader"}]}`))
		if err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	cfg := declared.Config
	cfg["url"] = server.URL
	inst, err := connector.Get("custom", cfg)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := inst.Fetch(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 || len(snapshot.Entities) != 2 {
		t.Fatalf("requests/entities %d/%d", requests, len(snapshot.Entities))
	}
	if snapshot.Entities[0].Name != "One" || snapshot.Entities[0].ExternalID != "1" || snapshot.Entities[0].Attributes["enabled"] != true || snapshot.Entities[0].Attributes["source"] != "library" {
		t.Fatalf("entity %+v", snapshot.Entities[0])
	}
	if len(snapshot.Dependencies) != 2 {
		t.Fatalf("dependencies %+v", snapshot.Dependencies)
	}
}
