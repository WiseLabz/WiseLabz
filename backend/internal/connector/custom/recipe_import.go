package custom

import (
	"errors"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// ValidateRecipeImportConfig validates the structural parts of a custom
// connector recipe in a backup. Exports redact recipe credentials, so import
// validation deliberately does not require auth_token, auth_username, or
// auth_password to be present.
func ValidateRecipeImportConfig(config map[string]any) error {
	if !recipeConfigured(config) {
		return nil
	}
	raw, ok := config["recipe"].(string)
	if !ok {
		return &connector.ConfigValidationError{Field: "recipe", Message: "must be a YAML string"}
	}
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	_, parseErr := ParseRecipe(raw)
	var issues []error
	issues = append(issues, validateLegacyHeaders(config))
	_, urlErr := recipeBaseURL(config)
	issues = append(issues, urlErr)
	if parseErr != nil {
		issues = append(issues, recipeConfigErrors(parseErr))
	}
	return redactURLsInRecipeError(errors.Join(issues...))
}
