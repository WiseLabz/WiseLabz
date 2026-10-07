package custom_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/connector"
	_ "github.com/WiseLabz/wiselabz/internal/connector/custom"
	"go.yaml.in/yaml/v3"
)

func TestDocumentedDeclaredRecipe(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	t.Setenv("WL_RECIPE_LITERAL", "must-not-expand")
	oldUnset, wasSet := os.LookupEnv("WL_RECIPE_UNSET")
	if err := os.Unsetenv("WL_RECIPE_UNSET"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv("WL_RECIPE_UNSET", oldUnset)
			return
		}
		_ = os.Unsetenv("WL_RECIPE_UNSET")
	})
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
	if snapshot.Entities[0].Name != "One" || snapshot.Entities[0].ExternalID != "1" || snapshot.Entities[0].Attributes["enabled"] != true || snapshot.Entities[0].Attributes["source"] != "library" || snapshot.Entities[0].Attributes["literal"] != "${WL_RECIPE_LITERAL}/${WL_RECIPE_UNSET}" {
		t.Fatalf("entity %+v", snapshot.Entities[0])
	}
	if len(snapshot.Dependencies) != 2 {
		t.Fatalf("dependencies %+v", snapshot.Dependencies)
	}
}

func TestDocumentedServiceRecipesMapRecordedResponses(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	t.Run("Sonarr", func(t *testing.T) {
		recipe, err := os.ReadFile("../../../../docs/connectors/recipes/sonarr.yaml")
		if err != nil {
			t.Fatal(err)
		}
		response, err := os.ReadFile("testdata/sonarr-series.json")
		if err != nil {
			t.Fatal(err)
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v3/series" || r.Header.Get("X-Api-Key") != "fixture-token" {
				t.Errorf("request path/auth = %q/%q", r.URL.Path, r.Header.Get("X-Api-Key"))
			}
			if _, err := w.Write(response); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		snapshot := fetchDocumentedRecipe(t, server.URL, string(recipe), "fixture-token")
		if len(snapshot.Entities) != 2 {
			t.Fatalf("entities = %d, want 2", len(snapshot.Entities))
		}
		first := snapshot.Entities[0]
		if first.Kind != "media_series" || first.Name != "Clockwork Harbor" || first.ExternalID != "42" {
			t.Fatalf("first entity = %+v", first)
		}
		if first.Attributes["status"] != "active" || first.Attributes["monitored"] != true || first.Attributes["year"] != json.Number("2031") {
			t.Fatalf("first attributes = %#v", first.Attributes)
		}
		wantDependencies := []connector.ServiceDependency{
			{Kind: "storage", Name: "/library/series/amber-signal"},
			{Kind: "storage", Name: "/library/series/clockwork-harbor"},
		}
		if !reflect.DeepEqual(snapshot.Dependencies, wantDependencies) {
			t.Fatalf("dependencies = %#v, want %#v", snapshot.Dependencies, wantDependencies)
		}
	})

	t.Run("Jellyfin", func(t *testing.T) {
		recipe, err := os.ReadFile("../../../../docs/connectors/recipes/jellyfin.yaml")
		if err != nil {
			t.Fatal(err)
		}
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			if r.URL.Path != "/Items" || r.Header.Get("Authorization") != "MediaBrowser Token=fixture-token" {
				t.Errorf("request path/auth = %q/%q", r.URL.Path, r.Header.Get("Authorization"))
			}
			if r.URL.Query().Get("Recursive") != "true" || r.URL.Query().Get("IncludeItemTypes") != "Movie" || r.URL.Query().Get("Limit") != "2" {
				t.Errorf("request query = %s", r.URL.RawQuery)
			}
			var fixture string
			switch r.URL.Query().Get("StartIndex") {
			case "0":
				fixture = "testdata/jellyfin-items-0.json"
			case "2":
				fixture = "testdata/jellyfin-items-2.json"
			case "4":
				fixture = "testdata/jellyfin-items-4.json"
			default:
				t.Errorf("unexpected StartIndex %q", r.URL.Query().Get("StartIndex"))
				http.Error(w, "unexpected page", http.StatusBadRequest)
				return
			}
			response, err := os.ReadFile(fixture)
			if err != nil {
				t.Error(err)
				http.Error(w, "missing fixture", http.StatusInternalServerError)
				return
			}
			if _, err := w.Write(response); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		snapshot := fetchDocumentedRecipe(t, server.URL, string(recipe), "fixture-token")
		if requests != 3 {
			t.Fatalf("requests = %d, want 3 (two pages and an empty end page)", requests)
		}
		if len(snapshot.Entities) != 3 {
			t.Fatalf("entities = %d, want 3", len(snapshot.Entities))
		}
		first := snapshot.Entities[0]
		if first.Kind != "media_item" || first.Name != "Glass Comet" || first.ExternalID != "10000000000000000000000000000001" {
			t.Fatalf("first entity = %+v", first)
		}
		if first.Attributes["media_type"] != "Movie" || first.Attributes["year"] != json.Number("2032") {
			t.Fatalf("first attributes = %#v", first.Attributes)
		}
		wantDependencies := []connector.ServiceDependency{
			{Kind: "storage", Name: "/media/movies/glass-comet.mkv"},
			{Kind: "storage", Name: "/media/movies/quiet-orbit.mkv"},
			{Kind: "storage", Name: "/media/movies/paper-moons.mkv"},
		}
		if !reflect.DeepEqual(snapshot.Dependencies, wantDependencies) {
			t.Fatalf("dependencies = %#v, want %#v", snapshot.Dependencies, wantDependencies)
		}
	})
}

func fetchDocumentedRecipe(t *testing.T, targetURL, recipe, token string) *connector.ServiceSnapshot {
	t.Helper()
	config := map[string]any{
		"url":        targetURL,
		"recipe":     recipe,
		"auth_token": token,
	}
	instance, err := connector.Get("custom", config)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := instance.Fetch(context.Background(), config)
	if err != nil {
		t.Fatalf("fetch documented recipe: %v", err)
	}
	return snapshot
}
