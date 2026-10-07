package connectors

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/custom"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

const recipePreviewTimeout = 30 * time.Second

var recipePreviewSlots = make(chan struct{}, 4)

// RecipePreview handles POST /api/connectors/recipe-preview without changing sync state.
func (h *Handler) RecipePreview(w http.ResponseWriter, r *http.Request) {
	if !auth.InstanceAdminFromContext(r.Context()) {
		httputil.Error(w, http.StatusForbidden, "forbidden", "Recipe preview requires an instance admin")
		return
	}
	select {
	case recipePreviewSlots <- struct{}{}:
		defer func() { <-recipePreviewSlots }()
	default:
		httputil.Error(w, http.StatusTooManyRequests, "preview_busy", "Recipe preview concurrency limit reached")
		return
	}
	req, ok := httputil.DecodeJSON[struct {
		ConnectorID string         `json:"connectorId"`
		URL         string         `json:"url"`
		VerifyTLS   *bool          `json:"verifyTls"`
		Config      map[string]any `json:"config"`
	}](w, r)
	if !ok {
		return
	}
	cfg := req.Config
	if cfg == nil {
		cfg = map[string]any{}
	}
	defer func() {
		// Preview cancellation must not cancel its audit write.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
		defer cancel()
		redact := previewRedactor(cfg, req.URL)
		if err := h.Store.RecordAuditFromContext(
			ctx,
			"connector.recipe_preview",
			"connector",
			req.ConnectorID,
			map[string]any{"url": redact(connector.RedactURL(req.URL))},
		); err != nil {
			slog.Error("failed to record audit", "action", "connector.recipe_preview", "error", err)
		}
	}()
	if req.ConnectorID != "" {
		rec, err := h.Store.GetConnector(r.Context(), req.ConnectorID)
		if err != nil {
			httputil.HandleStoreError(w, err)
			return
		}
		if rec.Type != "custom" {
			httputil.Error(w, http.StatusBadRequest, "invalid_request", "Recipe preview requires a custom connector")
			return
		}
		stored, err := store.ParseConnectorConfig(rec.Type, rec.ConfigData, h.Config.Encryption.Key)
		if err != nil {
			httputil.Errorf(w, err)
			return
		}
		schema, err := connector.GetTypeSchema("custom")
		if err != nil {
			httputil.Errorf(w, err)
			return
		}
		for _, field := range schema.Fields {
			value, sent := cfg[field.Key]
			if store.IsSecretFieldType(field.Type) && (!sent || value == nil || value == "") {
				cfg[field.Key] = stored[field.Key]
			}
		}
	}
	verifyTLS := req.VerifyTLS == nil || *req.VerifyTLS
	connector.ApplyRecordConfig(cfg, req.URL, verifyTLS)
	redact := previewRedactor(cfg, req.URL)
	recipe, recipePresent := cfg["recipe"]
	recipeText, recipeIsText := recipe.(string)
	if !recipePresent || recipe == nil || (recipeIsText && strings.TrimSpace(recipeText) == "") {
		writePreviewRejection(w, &connector.ConfigValidationError{Field: "recipe", Message: "is required"}, redact)
		return
	}
	if err := validateConnectorConfig("custom", req.URL, verifyTLS, cfg); err != nil {
		writePreviewRejection(w, err, redact)
		return
	}
	conn, err := connector.Get("custom", cfg)
	if err != nil {
		writePreviewRejection(w, err, redact)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), recipePreviewTimeout)
	defer cancel()
	result, err := conn.(*custom.Connector).PreviewRecipe(ctx, cfg)
	if err != nil {
		writePreviewRejection(w, err, redact)
		return
	}
	// A service can echo credentials into mapped values, names or dependencies.
	// Sanitize the complete JSON tree, including object keys, at the API boundary.
	data, err := json.Marshal(result)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	var output any
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err := decoder.Decode(&output); err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, redactPreviewValue(output, redact))
}

func writePreviewRejection(w http.ResponseWriter, err error, redact func(string) string) {
	details := configErrorDetails(err)
	for i := range details {
		details[i].Field = redact(details[i].Field)
		details[i].Msg = redact(details[i].Msg)
	}
	if len(details) == 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", redact(err.Error()))
		return
	}
	httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", redact(err.Error()), details)
}

var previewURLPattern = regexp.MustCompile("(?i)https?://[^\\s\\\"'`<>]+")

func previewRedactor(cfg map[string]any, targetURL string) func(string) string {
	secrets := []string{}
	for _, key := range []string{"auth_token", "auth_username", "auth_password"} {
		if value, ok := cfg[key].(string); ok && value != "" {
			secrets = append(secrets, value)
		}
	}
	if headers, ok := cfg["headers"].(string); ok && headers != "" {
		secrets = append(secrets, headers)
		values := map[string]string{}
		if json.Unmarshal([]byte(headers), &values) == nil {
			for _, value := range values {
				if value == "" {
					continue
				}
				secrets = append(secrets, value)
				if prefix, token, found := strings.Cut(value, " "); found &&
					(strings.EqualFold(prefix, "Bearer") || strings.EqualFold(prefix, "Basic")) && token != "" {
					secrets = append(secrets, token)
					if strings.EqualFold(prefix, "Basic") {
						decoded, err := base64.StdEncoding.DecodeString(token)
						if err == nil {
							for _, part := range strings.SplitN(string(decoded), ":", 2) {
								if part != "" {
									secrets = append(secrets, part)
								}
							}
						}
					}
				}
			}
		}
	}
	username, _ := cfg["auth_username"].(string)
	password, _ := cfg["auth_password"].(string)
	if username != "" || password != "" {
		secrets = append(secrets, base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}
	if parsed, err := url.Parse(targetURL); err == nil {
		for _, values := range parsed.Query() {
			for _, value := range values {
				if value != "" {
					secrets = append(secrets, value)
				}
			}
		}
	}
	for _, secret := range append([]string{}, secrets...) {
		secrets = append(secrets, url.QueryEscape(secret), url.PathEscape(secret))
	}
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
	replacements := make([]string, 0, 2*len(secrets))
	for _, secret := range secrets {
		replacements = append(replacements, secret, "[REDACTED]")
	}
	// One pass keeps redaction bounded even with many legacy header values.
	replacer := strings.NewReplacer(replacements...)
	return func(value string) string {
		value = previewURLPattern.ReplaceAllStringFunc(value, connector.RedactURL)
		return replacer.Replace(value)
	}
}

func redactPreviewValue(value any, redact func(string) string) any {
	switch typed := value.(type) {
	case string:
		return redact(typed)
	case []any:
		for i := range typed {
			typed[i] = redactPreviewValue(typed[i], redact)
		}
	case map[string]any:
		output := make(map[string]any, len(typed))
		for key, child := range typed {
			output[redact(key)] = redactPreviewValue(child, redact)
		}
		return output
	}
	return value
}
