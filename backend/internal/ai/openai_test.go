package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAICompatibleSuggest(t *testing.T) {
	maxTokens := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("Authorization = %q", got)
		}
		var body struct {
			Model     string `json:"model"`
			MaxTokens int    `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.Model != "model" {
			t.Fatalf("request = %+v", body)
		}
		maxTokens = body.MaxTokens
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"suggestion"}}]}`))
	}))
	defer server.Close()

	p := &openAICompatibleProvider{name: "openai", baseURL: server.URL, apiKey: "secret", model: "model", client: server.Client()}
	got, err := p.Suggest(context.Background(), &SuggestRequest{SystemPrompt: "system", UserPrompt: "user", MaxTokens: 42})
	if err != nil || got != "suggestion" {
		t.Fatalf("Suggest() = %q, %v", got, err)
	}
	if maxTokens != 42 {
		t.Fatalf("max_tokens = %d, want 42", maxTokens)
	}

	ch, err := p.SuggestStream(context.Background(), &SuggestRequest{})
	if err != nil {
		t.Fatalf("SuggestStream: %v", err)
	}
	chunk := <-ch
	if !chunk.Done || chunk.ContentDelta != "suggestion" || chunk.Error != "" {
		t.Fatalf("chunk = %+v", chunk)
	}
}

func TestOpenAICompatibleSuggestErrors(t *testing.T) {
	tests := []struct {
		name string
		code int
		body string
		want string
	}{
		{name: "status", code: http.StatusUnauthorized, body: `bad token`, want: "ai provider returned 401: bad token"},
		{name: "invalid JSON", code: http.StatusOK, body: `{`, want: "decode ai response"},
		{name: "no choices", code: http.StatusOK, body: `{}`, want: "ai provider returned no choices"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.code)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			p := &openAICompatibleProvider{baseURL: server.URL, client: server.Client()}
			_, err := p.Suggest(context.Background(), &SuggestRequest{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Suggest() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestRegisterOpenAICompatibleDefaults(t *testing.T) {
	r := NewRegistry()
	RegisterOpenAICompatible(r)
	p, err := r.Get("ollama", map[string]any{})
	if err != nil {
		t.Fatalf("Get(ollama): %v", err)
	}
	provider := p.(*openAICompatibleProvider)
	if provider.Name() != "ollama" || provider.baseURL != "https://api.openai.com/v1" || provider.model != "gpt-4o-mini" {
		t.Fatalf("provider = %+v", provider)
	}
	if _, err := r.Get("missing", nil); err == nil {
		t.Fatal("Get(missing) error = nil")
	}
}
