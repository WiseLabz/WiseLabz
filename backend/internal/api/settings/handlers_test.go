package settings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/ai"
	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		Auth: config.AuthSettings{
			Secret: "test-secret",
			OIDC: []config.OIDCProvider{
				{ID: "okta", DisplayName: "Okta", IssuerURL: "https://okta.example.com", ClientID: "abc", ClientSecret: "shh"},
			},
		},
		// Fixed valid base64 32-byte AES-256 key, so AI-config tests exercise
		// real encryption/decryption instead of erroring out.
		Encryption: config.EncryptionSettings{Key: "YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWE="},
	}
}

func TestGetAuthConfig(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s, testConfig(), ai.NewRegistry())

	req := httptest.NewRequest(http.MethodGet, "/api/auth/config", nil)
	rr := httptest.NewRecorder()
	h.GetAuthConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	providers, _ := resp["oidcProviders"].([]any)
	if len(providers) != 1 {
		t.Fatalf("oidcProviders len = %d, want 1", len(providers))
	}
}

func TestUpdateAuthConfig(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s, testConfig(), ai.NewRegistry())

	t.Run("no fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/auth/config", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.UpdateAuthConfig(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/auth/config", strings.NewReader(`{`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.UpdateAuthConfig(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("updates and reads back", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/auth/config", strings.NewReader(`{"localEnabled":false,"stepUpForDestructive":true}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.UpdateAuthConfig(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp["localEnabled"] != false {
			t.Errorf("localEnabled = %v, want false", resp["localEnabled"])
		}
		if resp["stepUpForDestructive"] != true {
			t.Errorf("stepUpForDestructive = %v, want true", resp["stepUpForDestructive"])
		}
	})
}

func TestUpdateProviderEnabled(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s, testConfig(), ai.NewRegistry())

	t.Run("unknown provider", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/auth/providers/nope/enabled", strings.NewReader(`{"enabled":false}`))
		req.SetPathValue("providerId", "nope")
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.UpdateProviderEnabled(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/auth/providers/okta/enabled", strings.NewReader(`{`))
		req.SetPathValue("providerId", "okta")
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.UpdateProviderEnabled(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("disable revokes sessions and audits", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/auth/providers/okta/enabled", strings.NewReader(`{"enabled":false}`))
		req.SetPathValue("providerId", "okta")
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.UpdateProviderEnabled(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp["enabled"] != false {
			t.Errorf("enabled = %v, want false", resp["enabled"])
		}
	})

	t.Run("re-enable", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/auth/providers/okta/enabled", strings.NewReader(`{"enabled":true}`))
		req.SetPathValue("providerId", "okta")
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.UpdateProviderEnabled(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
	})
}

func TestAIConfigRoundTrip(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s, testConfig(), ai.NewRegistry())

	t.Run("get default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ai/config", nil)
		rr := httptest.NewRecorder()
		h.GetAIConfig(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("no fields to update", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/ai/config", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.UpdateAIConfig(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("update encrypts api key and never leaks plaintext in JSON response", func(t *testing.T) {
		body := `{"enabled":true,"provider":"anthropic","model":"claude","apiKey":"sk-super-secret","baseUrl":"https://api.anthropic.com"}`
		req := httptest.NewRequest(http.MethodPut, "/api/ai/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.UpdateAIConfig(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
		if strings.Contains(rr.Body.String(), "sk-super-secret") {
			t.Errorf("response leaked plaintext API key: %s", rr.Body.String())
		}

		got := h.GetDecryptedAPIKey()
		if got != "sk-super-secret" {
			t.Errorf("GetDecryptedAPIKey() = %q, want sk-super-secret", got)
		}
	})
}

func TestUpdateAIConfigEncryptsEmbedAPIKey(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s, testConfig(), ai.NewRegistry())
	req := httptest.NewRequest(http.MethodPut, "/api/ai/config", strings.NewReader(`{"embedApiKey":"embed-super-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.UpdateAIConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "embed-super-secret") {
		t.Fatalf("response leaked plaintext embed API key: %s", rr.Body.String())
	}
	if got := h.GetDecryptedEmbedAPIKey(); got != "embed-super-secret" {
		t.Fatalf("GetDecryptedEmbedAPIKey() = %q, want embed-super-secret", got)
	}
}

func TestNotificationsConfigRoundTrip(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s, testConfig(), ai.NewRegistry())

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/notifications/config", strings.NewReader(`{`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.UpdateNotificationsConfig(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("update and read back", func(t *testing.T) {
		body := `{"channels":[{"id":"c1","type":"in_app"}],"routing":[]}`
		req := httptest.NewRequest(http.MethodPut, "/api/notifications/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.UpdateNotificationsConfig(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}

		getReq := httptest.NewRequest(http.MethodGet, "/api/notifications/config", nil)
		getRR := httptest.NewRecorder()
		h.GetNotificationsConfig(getRR, getReq)
		if !strings.Contains(getRR.Body.String(), `"c1"`) {
			t.Errorf("GetNotificationsConfig() did not reflect update: %s", getRR.Body.String())
		}
	})

	t.Run("test notification missing channel", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/notifications/config/test", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.TestNotificationsConfig(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("test notification ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/notifications/config/test", strings.NewReader(`{"channel":"in_app"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.TestNotificationsConfig(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("test notification unconfigured channel", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/notifications/config/test", strings.NewReader(`{"channel":"slack"}`))
		rr := httptest.NewRecorder()
		h.TestNotificationsConfig(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("test notification reports real sender failure", func(t *testing.T) {
		body := `{"channels":[{"id":"c2","type":"slack","enabled":true,"config":{}}],"routing":[]}`
		put := httptest.NewRequest(http.MethodPut, "/api/notifications/config", strings.NewReader(body))
		h.UpdateNotificationsConfig(httptest.NewRecorder(), put)

		req := httptest.NewRequest(http.MethodPost, "/api/notifications/config/test", strings.NewReader(`{"channel":"slack"}`))
		rr := httptest.NewRecorder()
		h.TestNotificationsConfig(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if !strings.Contains(rr.Body.String(), `"ok":false`) || !strings.Contains(rr.Body.String(), "slack url not configured") {
			t.Errorf("body = %s, want ok:false with sender error", rr.Body.String())
		}
	})
}

func TestGetDecryptedAPIKeyNoKeyStored(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s, testConfig(), ai.NewRegistry())
	if got := h.GetDecryptedAPIKey(); got != "" {
		t.Errorf("GetDecryptedAPIKey() = %q, want empty when nothing stored", got)
	}
}

func TestNotificationsConfigSigningSecret(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s, testConfig(), ai.NewRegistry())

	put := func(body string) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodPut, "/api/notifications/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.UpdateNotificationsConfig(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d; body=%s", rr.Code, rr.Body.String())
		}
		return rr.Body.String()
	}
	stored := func() string {
		t.Helper()
		var raw string
		if err := s.DB().QueryRow(`SELECT config_json FROM notification_config WHERE id = 1`).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		return raw
	}

	resp := put(`{"channels":[{"type":"webhook","enabled":true,"config":{"url":"https://x.test/h","secret":"hunter2"}}],"routing":[]}`)
	if strings.Contains(resp, "hunter2") || strings.Contains(resp, "secretEncrypted") || !strings.Contains(resp, `"secretSet":true`) {
		t.Errorf("response must mask the secret, got %s", resp)
	}
	raw := stored()
	if strings.Contains(raw, "hunter2") || !strings.Contains(raw, "secretEncrypted") {
		t.Errorf("secret must be stored encrypted, got %s", raw)
	}

	// Omitting the secret (e.g. a GET-then-PUT round trip) keeps the stored one.
	put(`{"channels":[{"type":"webhook","enabled":true,"config":{"url":"https://x.test/h2","secretSet":true}}],"routing":[]}`)
	if !strings.Contains(stored(), "secretEncrypted") {
		t.Error("omitted secret should preserve the stored one")
	}

	// An empty string clears it.
	put(`{"channels":[{"type":"webhook","enabled":true,"config":{"url":"https://x.test/h2","secret":""}}],"routing":[]}`)
	if strings.Contains(stored(), "secretEncrypted") {
		t.Error("empty secret should clear the stored one")
	}
}

func TestGetAIConfigNeverReturnsPlaintextKey(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s, testConfig(), ai.NewRegistry())

	req := httptest.NewRequest(http.MethodGet, "/api/ai/config", nil)
	rr := httptest.NewRecorder()
	h.GetAIConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	response := rr.Body.String()
	if strings.Contains(response, "api_key") || strings.Contains(response, "apiKey") || strings.Contains(response, "secret") {
		t.Errorf("GetAIConfig response must not contain API key field names: %s", response)
	}

	if strings.Contains(response, "sk-") || strings.Contains(response, "sk_") {
		t.Error("GetAIConfig response must not contain plaintext API keys")
	}
}

func TestUpdateAIConfigEncryptsAPIKey(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s, testConfig(), ai.NewRegistry())
	ctx := context.Background()

	updateReq := httptest.NewRequest(
		http.MethodPut,
		"/api/ai/config",
		strings.NewReader(`{"enabled":true,"provider":"openai","model":"gpt-4","apiKey":"sk-test-secret-key-1234567890"}`),
	)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRR := httptest.NewRecorder()
	h.UpdateAIConfig(updateRR, updateReq)

	if updateRR.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200; body=%s", updateRR.Code, updateRR.Body.String())
	}

	var storedConfig struct {
		Enabled   int
		Provider  string
		APIKeyEnc string
	}
	err := s.DB().QueryRowContext(ctx, `
		SELECT enabled, provider, api_key_encrypted FROM ai_config WHERE id = 1
	`).Scan(&storedConfig.Enabled, &storedConfig.Provider, &storedConfig.APIKeyEnc)
	if err != nil {
		t.Fatalf("query stored config: %v", err)
	}

	if storedConfig.APIKeyEnc == "" {
		t.Error("API key should be encrypted and stored")
	}

	if strings.Contains(storedConfig.APIKeyEnc, "sk-test-secret") {
		t.Error("stored API key must not contain plaintext secret")
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/ai/config", nil)
	getRR := httptest.NewRecorder()
	h.GetAIConfig(getRR, getReq)

	if strings.Contains(getRR.Body.String(), "sk-test-secret") {
		t.Error("GetAIConfig must not return plaintext API key in response")
	}
}

func TestUpdateAIConfigAcceptsDisabledProvider(t *testing.T) {
	s := apitest.NewStore(t)
	h := NewHandler(s, testConfig(), ai.NewRegistry())

	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "valid openai provider",
			body:    `{"enabled":true,"provider":"openai","model":"gpt-4","apiKey":"sk-test"}`,
			wantErr: false,
		},
		{
			name:    "disabled provider is accepted",
			body:    `{"enabled":false,"provider":"","model":"","apiKey":""}`,
			wantErr: false,
		},
		{
			name:    "invalid json",
			body:    `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/ai/config", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.UpdateAIConfig(rr, req)

			if tt.wantErr && rr.Code == http.StatusOK {
				t.Errorf("status = %d, want error; body=%s", rr.Code, rr.Body.String())
			}
			if !tt.wantErr && rr.Code != http.StatusOK {
				t.Errorf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}
