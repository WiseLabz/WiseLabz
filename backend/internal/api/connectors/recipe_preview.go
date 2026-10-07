package connectors

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/custom"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

const (
	recipePreviewTimeout = 30 * time.Second
	// The request body may be 4 MiB; the audit entry keeps only a bounded echo.
	maxPreviewAuditIDBytes  = 128
	maxPreviewAuditURLBytes = 2048
)

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
			truncatePreviewAudit(req.ConnectorID, maxPreviewAuditIDBytes),
			map[string]any{"url": truncatePreviewAudit(redact(connector.RedactURL(req.URL)), maxPreviewAuditURLBytes)},
		); err != nil {
			slog.Error("failed to record audit", "action", "connector.recipe_preview", "error", err)
		}
	}()
	verifyTLS := req.VerifyTLS == nil || *req.VerifyTLS
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
		fill := map[string]string{}
		for _, field := range schema.Fields {
			value, sent := cfg[field.Key]
			secret, _ := stored[field.Key].(string)
			if store.IsSecretFieldType(field.Type) && (!sent || value == nil || value == "") && secret != "" {
				fill[field.Key] = secret
			}
		}
		// Stored secrets may only go to the saved target, whoever owns its settings.
		if len(fill) > 0 {
			var rejection *connector.ConfigValidationError
			switch {
			case !custom.SameOrigin(rec.URL, req.URL):
				rejection = &connector.ConfigValidationError{Field: "url",
					Message: "must match the saved connector URL to use its stored credentials; re-enter the credentials or save the new URL first"}
			case rec.VerifyTLS && !verifyTLS:
				rejection = &connector.ConfigValidationError{Field: "verify_tls",
					Message: "cannot be turned off while using the saved connector's stored credentials; re-enter the credentials"}
			}
			if rejection != nil {
				writePreviewRejection(w, rejection, previewRedactor(cfg, req.URL))
				return
			}
		}
		for key, secret := range fill {
			cfg[key] = secret
		}
	}
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
	customConn, ok := conn.(*custom.Connector)
	if !ok {
		httputil.Errorf(w, fmt.Errorf("recipe preview: unexpected connector type %T", conn))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), recipePreviewTimeout)
	defer cancel()
	result, err := customConn.PreviewRecipe(ctx, cfg)
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

// truncatePreviewAudit caps s at limit bytes without splitting a rune.
func truncatePreviewAudit(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	s = s[:limit]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
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
	// net/http trims header values on the wire, so a stored " abc " is echoed as "abc".
	for _, secret := range append([]string{}, secrets...) {
		if trimmed := strings.TrimSpace(secret); trimmed != "" && trimmed != secret {
			secrets = append(secrets, trimmed)
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
	return redactPreviewData(value, redact, false)
}

func redactPreviewData(value any, redact func(string) string, attribute bool) any {
	switch typed := value.(type) {
	case string:
		return redact(typed)
	case json.Number:
		if attribute {
			if masked := redact(typed.String()); masked != typed.String() {
				return masked
			}
		}
	case bool:
		if attribute {
			text := strconv.FormatBool(typed)
			if masked := redact(text); masked != text {
				return masked
			}
		}
	case []any:
		for i := range typed {
			typed[i] = redactPreviewData(typed[i], redact, attribute)
		}
	case map[string]any:
		output := make(map[string]any, len(typed))
		for key, child := range typed {
			// Counts are computed by the runner; attributes can echo any scalar.
			output[redact(key)] = redactPreviewData(child, redact, attribute || key == "attributes")
		}
		return output
	}
	return value
}
