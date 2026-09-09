package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClaudeSuggest(t *testing.T) {
	maxTokens := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "secret" {
			t.Fatalf("x-api-key = %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Fatalf("anthropic-version = %q", got)
		}
		var body struct {
			Model     string              `json:"model"`
			MaxTokens int                 `json:"max_tokens"`
			System    string              `json:"system"`
			Messages  []map[string]string `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.Model != "model" {
			t.Fatalf("request = %+v", body)
		}
		maxTokens = body.MaxTokens
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"suggestion"}]}`))
	}))
	defer server.Close()

	p := &claudeProvider{name: "claude", baseURL: server.URL, apiKey: "secret", model: "model", client: server.Client()}
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

func TestClaudeSuggestDefaultMaxTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MaxTokens int `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.MaxTokens != 4096 {
			t.Fatalf("max_tokens = %d, want 4096", body.MaxTokens)
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"response"}]}`))
	}))
	defer server.Close()

	p := &claudeProvider{name: "claude", baseURL: server.URL, model: "model", client: server.Client()}
	got, err := p.Suggest(context.Background(), &SuggestRequest{UserPrompt: "user"})
	if err != nil || got != "response" {
		t.Fatalf("Suggest() = %q, %v", got, err)
	}
}

func TestClaudeSuggestMultipleContentBlocks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"hello"},{"type":"text","text":" world"}]}`))
	}))
	defer server.Close()

	p := &claudeProvider{baseURL: server.URL, client: server.Client()}
	got, err := p.Suggest(context.Background(), &SuggestRequest{UserPrompt: "user"})
	if err != nil || got != "hello world" {
		t.Fatalf("Suggest() = %q, %v", got, err)
	}
}

func TestClaudeSuggestErrors(t *testing.T) {
	tests := []struct {
		name string
		code int
		body string
		want string
	}{
		{name: "status", code: http.StatusUnauthorized, body: `bad token`, want: "ai provider returned 401: bad token"},
		{name: "invalid JSON", code: http.StatusOK, body: `{`, want: "decode ai response"},
		{name: "no content", code: http.StatusOK, body: `{"content":[]}`, want: "ai provider returned no content"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.code)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			p := &claudeProvider{baseURL: server.URL, client: server.Client()}
			_, err := p.Suggest(context.Background(), &SuggestRequest{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Suggest() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestRegisterClaudeDefaults(t *testing.T) {
	r := NewRegistry()
	RegisterClaude(r)
	p, err := r.Get("claude", map[string]any{})
	if err != nil {
		t.Fatalf("Get(claude): %v", err)
	}
	provider := p.(*claudeProvider)
	if provider.Name() != "claude" || provider.baseURL != "https://api.anthropic.com" || provider.model != "claude-sonnet-5" {
		t.Fatalf("provider = %+v", provider)
	}
	if _, err := r.Get("missing", nil); err == nil {
		t.Fatal("Get(missing) error = nil")
	}
}
