package custom

import (
	"errors"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestParseRecipeRedactsRequestURLsInLocatedErrors(t *testing.T) {
	const token = "unreported-query-token"
	raw := validRecipe + "'https://api.example/items?api_token=" + token + "': true\n"
	_, err := ParseRecipe(raw)
	var invalid *RecipeValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("ParseRecipe() = %v, want validation error", err)
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("validation error contains query token: %v", err)
	}
	for _, issue := range invalid.Issues {
		if strings.Contains(issue.Location, token) || strings.Contains(issue.Message, token) {
			t.Fatalf("located issue contains query token: %+v", issue)
		}
	}
	configErr := recipeConfigErrors(err)
	if strings.Contains(configErr.Error(), token) {
		t.Fatalf("config error contains query token: %v", configErr)
	}
}

func TestRecipeURLRedactionPreservesErrorClassification(t *testing.T) {
	original := connector.NewAuthError(errors.New("request to https://user:password@api.example/items?arbitrary=private-token#fragment failed"))
	err := redactURLsInRecipeError(original)
	var auth *connector.AuthError
	if !errors.As(err, &auth) {
		t.Fatalf("redacted error = %v, want authentication classification", err)
	}
	for _, secret := range []string{"user", "password", "private-token", "fragment"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("redacted error contains %q: %v", secret, err)
		}
	}
	if !strings.Contains(err.Error(), "https://api.example/items") {
		t.Fatalf("redacted error lost safe request URL: %v", err)
	}
}
