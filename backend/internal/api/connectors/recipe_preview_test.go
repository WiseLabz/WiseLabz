package connectors

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector/custom"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func previewRequest(t *testing.T, h *Handler, user string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	r := recipeRequestAs(t, http.MethodPost, "recipe-preview", body, user, true)
	w := httptest.NewRecorder()
	h.RecipePreview(w, r)
	return w
}

func assertPreviewWrites(t *testing.T, h *Handler, connectors int) {
	t.Helper()
	for table, want := range map[string]int{
		"connectors": connectors, "service_snapshots": 0, "sync_runs": 0, "changes": 0, "alerts": 0,
		"entities": 0, "entity_members": 0, "entity_index": 0, "health_checks": 0, "topology_edges": 0,
		"notification_deliveries": 0, "in_app_notifications": 0,
	} {
		var count int
		if err := h.Store.DB().QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != want {
			t.Errorf("%s count=%d want=%d", table, count, want)
		}
	}
}

func assertPreviewAudit(t *testing.T, h *Handler, actor, id, target string, secrets ...string) {
	t.Helper()
	rows, total, err := h.Store.ListAuditRecords(context.Background(), "connector.recipe_preview", "connector", "", "", 0, 20)
	if err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("audit rows=%+v total=%d err=%v", rows, total, err)
	}
	entry := rows[0]
	if entry.ActorUserID != actor || entry.ActorRole != "admin" || entry.TargetID != id {
		t.Errorf("audit=%+v", entry)
	}
	var detail map[string]string
	if err := json.Unmarshal([]byte(entry.Detail), &detail); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(detail, map[string]string{"url": target}) {
		t.Errorf("detail=%+v want URL=%s only", detail, target)
	}
	for _, secret := range secrets {
		if strings.Contains(entry.Detail, secret) {
			t.Errorf("audit contains credential %q", secret)
		}
	}
}

func TestRecipePreviewUnsavedContinuesAndWritesOnlyAudit(t *testing.T) {
	h := newTestHandler(t)
	actor := apitest.NewUser(t, h.Store, "operator")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/down" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.URL.Query().Get("apikey") != "preview-private-token" {
			t.Error("token absent from request")
		}
		_, _ = fmt.Fprint(w, `[{"id":"1","title":"Invented title","ready":true},{"title":"Skipped"}]`)
	}))
	defer server.Close()
	recipe := strings.Replace(apiRecipe("media"), "auth: {mode: none}", "auth: {mode: query, name: apikey}", 1)
	recipe += `  - name: down
    path: /down
    method: GET
    items: '@this'
    entity: {kind: item, name: title, external_id: id}
  - name: after
    path: /after
    method: GET
    items: '@this'
    entity: {kind: other, name: title, external_id: id}
dependencies: [{kind: storage, const: invented-storage}]
`
	w := previewRequest(t, h, actor, map[string]any{
		"url": server.URL, "config": map[string]any{"recipe": recipe, "auth_token": "preview-private-token"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d: %s", w.Code, w.Body.String())
	}
	var got custom.RecipePreviewResult
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Endpoints) != 3 || got.Endpoints[0].Count != 1 || got.Endpoints[0].Skipped != 1 ||
		got.Endpoints[1].Error == "" || got.Endpoints[2].Count != 1 || len(got.Errors) != 1 || len(got.Dependencies) != 1 {
		t.Fatalf("preview=%+v", got)
	}
	assertPreviewWrites(t, h, 0)
	assertPreviewAudit(t, h, actor, "", server.URL, "preview-private-token", "Invented title")
}

func TestRecipePreviewStoredSecretsAndRedactedEcho(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(fmt.Sprint("empty=", empty), func(t *testing.T) {
			h := newTestHandler(t)
			actor := apitest.NewUser(t, h.Store, "operator")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, password, _ := r.BasicAuth()
				if user != "private-user" || password != "private-password" || r.Header.Get("X-Legacy") != "private-header" {
					t.Errorf("stored credentials not sent")
				}
				_, _ = fmt.Fprint(w, `[{"id":"private-password","title":"private-user","alias":["private-header"],"echo":"https://api.example/items?arbitrary=private-password#private-user"}]`)
			}))
			defer server.Close()
			recipe := strings.Replace(apiRecipe("media"), "auth: {mode: none}", "auth: {mode: basic}", 1)
			recipe = strings.Replace(recipe, "external_id: id}", "external_id: id, aliases: alias, attributes: {echo: {path: echo}, private-password: {const: private-header}}}", 1)
			cfg := map[string]any{"recipe": recipe, "auth_username": "private-user", "auth_password": "private-password", "headers": `{"X-Legacy":"private-header"}`}
			data, err := store.MarshalConnectorConfig("custom", cfg, h.Config.Encryption.Key)
			if err != nil {
				t.Fatal(err)
			}
			rec := &store.ConnectorRecord{Name: "Preview", Type: "custom", Category: "media", URL: server.URL,
				ConfigData: data, VerifyTLS: true, Status: "online", LastSyncError: "previous failure"}
			if err := h.Store.CreateConnector(context.Background(), rec); err != nil {
				t.Fatal(err)
			}
			before, err := h.Store.GetConnector(context.Background(), rec.ID)
			if err != nil {
				t.Fatal(err)
			}
			requestCfg := map[string]any{"recipe": recipe}
			if empty {
				requestCfg["auth_username"], requestCfg["auth_password"], requestCfg["headers"] = "", "", ""
			}
			w := previewRequest(t, h, actor, map[string]any{"connectorId": rec.ID, "url": server.URL + "?token=private-query", "config": requestCfg})
			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			for _, secret := range []string{"private-user", "private-password", "private-header", "private-query"} {
				if strings.Contains(w.Body.String(), secret) {
					t.Errorf("response contains credential %q", secret)
				}
			}
			after, err := h.Store.GetConnector(context.Background(), rec.ID)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("connector changed: before=%+v after=%+v err=%v", before, after, err)
			}
			assertPreviewWrites(t, h, 1)
			assertPreviewAudit(t, h, actor, rec.ID, server.URL, "private-password", "private-user", "private-header", "private-query")
		})
	}
}

func TestRecipePreviewValidationLocatedAndAudited(t *testing.T) {
	h := newTestHandler(t)
	actor := apitest.NewUser(t, h.Store, "operator")
	recipe := strings.Replace(apiRecipe("media"), "external_id: id", "external_id: ''", 1)
	w := previewRequest(t, h, actor, map[string]any{"url": "https://api.example?apikey=private-query",
		"config": map[string]any{"recipe": recipe, "auth_token": "private-token"}})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d: %s", w.Code, w.Body.String())
	}
	var response struct{ Details []httputil.FieldError }
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, detail := range response.Details {
		found = found || detail.Field == "config.recipe.endpoints[0].entity.external_id"
	}
	if !found {
		t.Fatalf("located error absent: %+v", response.Details)
	}
	assertPreviewWrites(t, h, 0)
	assertPreviewAudit(t, h, actor, "", "https://api.example", "private-query", "private-token")
}

func TestRecipePreviewAdminCheckBeforeBodyAndStore(t *testing.T) {
	// A nil store and malformed body prove the authorization check runs first.
	h := &Handler{}
	r := httptest.NewRequest(http.MethodPost, "/api/connectors/recipe-preview", strings.NewReader("{"))
	r = r.WithContext(auth.ContextWithUser(r.Context(), "viewer", false))
	w := httptest.NewRecorder()
	h.RecipePreview(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d: %s", w.Code, w.Body.String())
	}
}

func TestRecipePreviewBodyAndConcurrencyBounds(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest(http.MethodPost, "/api/connectors/recipe-preview",
		strings.NewReader(`{"config":{"recipe":"`+strings.Repeat("x", httputil.MaxJSONBodyBytes)+`"}}`))
	r = r.WithContext(auth.ContextWithUser(r.Context(), "admin", true))
	w := httptest.NewRecorder()
	h.RecipePreview(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body status=%d", w.Code)
	}
	for range cap(recipePreviewSlots) {
		recipePreviewSlots <- struct{}{}
	}
	defer func() {
		for range cap(recipePreviewSlots) {
			<-recipePreviewSlots
		}
	}()
	w = httptest.NewRecorder()
	h.RecipePreview(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("concurrency status=%d", w.Code)
	}
}

func TestRecipePreviewDeadlineAndGuardedRedirect(t *testing.T) {
	h := newTestHandler(t)
	actor := apitest.NewUser(t, h.Store, "operator")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data?token=private-token", http.StatusFound)
	}))
	defer server.Close()
	body := map[string]any{"url": server.URL, "config": map[string]any{"recipe": apiRecipe("media")}}
	w := previewRequest(t, h, actor, body)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "302") || strings.Contains(w.Body.String(), "private-token") {
		t.Fatalf("redirect preview=%d %s", w.Code, w.Body.String())
	}
	ctx, cancel := context.WithTimeout(auth.ContextWithUser(context.Background(), actor, true), time.Nanosecond)
	defer cancel()
	<-ctx.Done()
	r := recipeRequest(t, http.MethodPost, "recipe-preview", body).WithContext(ctx)
	w = httptest.NewRecorder()
	h.RecipePreview(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "timeout") {
		t.Fatalf("deadline preview=%d %s", w.Code, w.Body.String())
	}
}

func TestPreviewRedactionURLsHeadersAndKeys(t *testing.T) {
	basic := base64.StdEncoding.EncodeToString([]byte("private-user:private-password"))
	redact := previewRedactor(map[string]any{"auth_username": "private-user", "auth_password": "private-password",
		"auth_token": "private token", "headers": `{"Authorization":"Bearer legacy-secret"}`}, "https://api.example?custom=secret-query")
	value := map[string]any{"private-password": []any{basic, "private+token", "legacy-secret", "secret-query", "https://u:p@api.example/items?arbitrary=sensitive#fragment"}}
	data, err := json.Marshal(redactPreviewValue(value, redact))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private", "legacy-secret", "secret-query", "sensitive", "fragment", "u:p@"} {
		if strings.Contains(string(data), secret) {
			t.Errorf("redaction leaked %s: %s", secret, data)
		}
	}
}

func TestPreviewRedactionLegacyBasicCredentials(t *testing.T) {
	basic := base64.StdEncoding.EncodeToString([]byte("legacy-user:legacy-password"))
	redact := previewRedactor(map[string]any{"headers": `{"Authorization":"Basic ` + basic + `"}`}, "https://api.example")
	for _, value := range []string{basic, "legacy-user", "legacy-password"} {
		if strings.Contains(redact(value), value) {
			t.Errorf("legacy basic credential leaked: %s", redact(value))
		}
	}
}

func TestRecipePreviewEditedRecipeUsesStoredToken(t *testing.T) {
	h := newTestHandler(t)
	actor := apitest.NewUser(t, h.Store, "operator")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/edited-items" || r.Header.Get("X-Api-Key") != "stored-private-token" {
			t.Errorf("edited preview request path/auth=%s/%s", r.URL.Path, r.Header.Get("X-Api-Key"))
		}
		_, _ = fmt.Fprint(w, `[{"id":1,"title":"Edited item"}]`)
	}))
	defer server.Close()
	recipe := strings.Replace(apiRecipe("media"), "auth: {mode: none}", "auth: {mode: header, name: X-Api-Key}", 1)
	data, err := store.MarshalConnectorConfig("custom", map[string]any{"recipe": recipe, "auth_token": "stored-private-token"}, h.Config.Encryption.Key)
	if err != nil {
		t.Fatal(err)
	}
	rec := &store.ConnectorRecord{Name: "Saved recipe", Type: "custom", Category: "media", URL: server.URL, ConfigData: data}
	if err := h.Store.CreateConnector(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	w := previewRequest(t, h, actor, map[string]any{"connectorId": rec.ID, "url": server.URL,
		"config": map[string]any{"recipe": strings.Replace(recipe, "/items", "/edited-items", 1), "auth_token": ""}})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Edited item") || strings.Contains(w.Body.String(), "stored-private-token") {
		t.Fatalf("stored token preview=%d: %s", w.Code, w.Body.String())
	}
	stored, err := h.Store.GetConnector(context.Background(), rec.ID)
	if err != nil || stored.ConfigData != data {
		t.Fatalf("preview changed config: %v", err)
	}
	assertPreviewWrites(t, h, 1)
	assertPreviewAudit(t, h, actor, rec.ID, server.URL, "stored-private-token")
}

func TestPreviewRedactionScalarCredentialAttributesPreservesCounts(t *testing.T) {
	redact := previewRedactor(map[string]any{"auth_token": "987654", "auth_password": "true"}, "https://api.example")
	value := map[string]any{
		"count": json.Number("987654"),
		"attributes": map[string]any{
			"token": json.Number("987654"), "password": true, "year": json.Number("2032"),
		},
	}
	got := redactPreviewValue(value, redact).(map[string]any)
	attributes := got["attributes"].(map[string]any)
	if attributes["token"] != "[REDACTED]" || attributes["password"] != "[REDACTED]" {
		t.Fatalf("scalar credentials leaked: %+v", attributes)
	}
	if attributes["year"] != json.Number("2032") || got["count"] != json.Number("987654") {
		t.Fatalf("non-credential scalars changed: %+v", got)
	}
}

// tokenServer counts requests and records the X-Api-Key header it receives.
func tokenServer(t *testing.T) (*httptest.Server, *atomic.Int32, *atomic.Value) {
	t.Helper()
	var requests atomic.Int32
	var key atomic.Value
	key.Store("")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		key.Store(r.Header.Get("X-Api-Key"))
		_, _ = fmt.Fprint(w, `[{"id":1,"title":"Item"}]`)
	}))
	t.Cleanup(server.Close)
	return server, &requests, &key
}

func createStoredTokenConnector(t *testing.T, h *Handler, targetURL string, verifyTLS bool, managedBy string) (*store.ConnectorRecord, string) {
	t.Helper()
	recipe := strings.Replace(apiRecipe("media"), "auth: {mode: none}", "auth: {mode: header, name: X-Api-Key}", 1)
	data, err := store.MarshalConnectorConfig("custom", map[string]any{"recipe": recipe, "auth_token": "stored-private-token"}, h.Config.Encryption.Key)
	if err != nil {
		t.Fatal(err)
	}
	rec := &store.ConnectorRecord{Name: "Saved " + managedBy, Type: "custom", Category: "media", URL: targetURL,
		ConfigData: data, VerifyTLS: verifyTLS, ManagedBy: managedBy}
	if err := h.Store.CreateConnector(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	return rec, recipe
}

func assertPreviewRejectedField(t *testing.T, w *httptest.ResponseRecorder, field string) {
	t.Helper()
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d: %s", w.Code, w.Body.String())
	}
	var response struct{ Details []httputil.FieldError }
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Details) != 1 || response.Details[0].Field != field {
		t.Fatalf("details=%+v want field %q", response.Details, field)
	}
	if strings.Contains(w.Body.String(), "stored-private-token") {
		t.Errorf("response contains stored credential: %s", w.Body.String())
	}
}

func TestRecipePreviewStoredSecretsRejectOtherOriginWithoutRequests(t *testing.T) {
	for _, managedBy := range []string{store.ManagedByUI, store.ManagedByConfig} {
		t.Run(managedBy, func(t *testing.T) {
			h := newTestHandler(t)
			actor := apitest.NewUser(t, h.Store, "operator")
			serverA, requestsA, _ := tokenServer(t)
			serverB, requestsB, _ := tokenServer(t)
			rec, recipe := createStoredTokenConnector(t, h, serverA.URL, true, managedBy)
			w := previewRequest(t, h, actor, map[string]any{"connectorId": rec.ID, "url": serverB.URL,
				"config": map[string]any{"recipe": recipe, "auth_token": ""}})
			assertPreviewRejectedField(t, w, "url")
			if requestsA.Load() != 0 || requestsB.Load() != 0 {
				t.Fatalf("requests A/B = %d/%d, want 0/0", requestsA.Load(), requestsB.Load())
			}
			assertPreviewWrites(t, h, 1)
			assertPreviewAudit(t, h, actor, rec.ID, serverB.URL, "stored-private-token")
		})
	}
}

func TestRecipePreviewStoredSecretsRejectWeakenedTLS(t *testing.T) {
	h := newTestHandler(t)
	actor := apitest.NewUser(t, h.Store, "operator")
	server, requests, _ := tokenServer(t)
	rec, recipe := createStoredTokenConnector(t, h, server.URL, true, store.ManagedByUI)
	w := previewRequest(t, h, actor, map[string]any{"connectorId": rec.ID, "url": server.URL, "verifyTls": false,
		"config": map[string]any{"recipe": recipe}})
	assertPreviewRejectedField(t, w, "verifyTls")
	if requests.Load() != 0 {
		t.Fatalf("requests = %d, want 0", requests.Load())
	}
	assertPreviewWrites(t, h, 1)
	assertPreviewAudit(t, h, actor, rec.ID, server.URL, "stored-private-token")
}

func TestRecipePreviewConfigManagedConnectorSameOriginSendsStoredToken(t *testing.T) {
	h := newTestHandler(t)
	actor := apitest.NewUser(t, h.Store, "operator")
	server, requests, key := tokenServer(t)
	rec, recipe := createStoredTokenConnector(t, h, server.URL, true, store.ManagedByConfig)
	w := previewRequest(t, h, actor, map[string]any{"connectorId": rec.ID, "url": server.URL + "/other?x=1",
		"config": map[string]any{"recipe": recipe}})
	if w.Code != http.StatusOK || requests.Load() != 1 || key.Load() != "stored-private-token" {
		t.Fatalf("status=%d requests=%d key=%v: %s", w.Code, requests.Load(), key.Load(), w.Body.String())
	}
	if strings.Contains(w.Body.String(), "stored-private-token") {
		t.Errorf("response contains stored credential: %s", w.Body.String())
	}
}

func TestRecipePreviewOtherOriginAllowedWhenCallerSuppliesCredentials(t *testing.T) {
	h := newTestHandler(t)
	actor := apitest.NewUser(t, h.Store, "operator")
	serverA, requestsA, _ := tokenServer(t)
	serverB, requestsB, keyB := tokenServer(t)
	rec, recipe := createStoredTokenConnector(t, h, serverA.URL, true, store.ManagedByUI)
	w := previewRequest(t, h, actor, map[string]any{"connectorId": rec.ID, "url": serverB.URL, "verifyTls": false,
		"config": map[string]any{"recipe": recipe, "auth_token": "caller-token"}})
	if w.Code != http.StatusOK || requestsA.Load() != 0 || requestsB.Load() != 1 || keyB.Load() != "caller-token" {
		t.Fatalf("status=%d requests A/B=%d/%d key=%v: %s", w.Code, requestsA.Load(), requestsB.Load(), keyB.Load(), w.Body.String())
	}
}

func TestRecipePreviewAuditBoundsCallerControlledValues(t *testing.T) {
	h := newTestHandler(t)
	actor := apitest.NewUser(t, h.Store, "operator")
	id := strings.Repeat("é", 200)
	w := previewRequest(t, h, actor, map[string]any{"connectorId": id, "url": "https://api.example/" + strings.Repeat("é", 3000),
		"config": map[string]any{"recipe": apiRecipe("media")}})
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d: %s", w.Code, w.Body.String())
	}
	rows, total, err := h.Store.ListAuditRecords(context.Background(), "connector.recipe_preview", "connector", "", "", 0, 20)
	if err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("audit rows=%+v total=%d err=%v", rows, total, err)
	}
	var detail map[string]string
	if err := json.Unmarshal([]byte(rows[0].Detail), &detail); err != nil {
		t.Fatal(err)
	}
	if len(rows[0].TargetID) > maxPreviewAuditIDBytes || !utf8.ValidString(rows[0].TargetID) || rows[0].TargetID == "" {
		t.Errorf("target id length=%d valid=%t", len(rows[0].TargetID), utf8.ValidString(rows[0].TargetID))
	}
	if len(detail["url"]) > maxPreviewAuditURLBytes || !utf8.ValidString(detail["url"]) || !strings.HasPrefix(detail["url"], "https://api.example/") {
		t.Errorf("url length=%d valid=%t", len(detail["url"]), utf8.ValidString(detail["url"]))
	}
}

func TestPreviewRedactionWhitespaceTrimmedCredentials(t *testing.T) {
	redact := previewRedactor(map[string]any{"auth_token": "  abc ", "headers": `{"X-Legacy":" legacy-secret\t"}`}, "https://api.example")
	for _, value := range []string{"Bearer abc", "abc", "legacy-secret"} {
		if got := redact(value); strings.Contains(got, "abc") || strings.Contains(got, "legacy-secret") {
			t.Errorf("trimmed credential leaked in %q: %q", value, got)
		}
	}
}
