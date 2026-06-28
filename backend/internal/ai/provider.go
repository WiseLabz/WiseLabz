// Package ai provides AI provider abstraction for doc suggestions.
package ai

import (
	"context"
	"fmt"
	"sync"
)

// Provider is the interface for AI suggestion providers.
type Provider interface {
	Name() string
	Suggest(ctx context.Context, req *SuggestRequest) (string, error)
	SuggestStream(ctx context.Context, req *SuggestRequest) (<-chan SuggestChunk, error)
}

// SuggestRequest contains the prompt and context for an AI suggestion.
type SuggestRequest struct {
	SystemPrompt string `json:"systemPrompt"`
	UserPrompt   string `json:"userPrompt"`
	DocContent   string `json:"docContent"`
	MaxTokens    int    `json:"maxTokens"`
}

// SuggestChunk is a streaming response chunk from an AI provider.
type SuggestChunk struct {
	ContentDelta string `json:"contentDelta"`
	Done         bool   `json:"done"`
	Error        string `json:"error,omitempty"`
}

// Registry maps provider names to factory functions.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]func(config map[string]any) (Provider, error)
}

// NewRegistry creates a new AI provider registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]func(config map[string]any) (Provider, error)),
	}
}

// Register adds a provider factory to the registry.
func (r *Registry) Register(name string, factory func(config map[string]any) (Provider, error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// Get returns a provider instance by name.
func (r *Registry) Get(name string, config map[string]any) (Provider, error) {
	r.mu.RLock()
	factory, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown AI provider: %q", name)
	}
	return factory(config)
}

// List returns registered provider names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var names []string
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}

// StubProvider is a stub AI provider for when AI is not configured.
type StubProvider struct{}

// Name returns "stub".
func (s *StubProvider) Name() string { return "stub" }

// Suggest returns a stub message.
func (s *StubProvider) Suggest(ctx context.Context, req *SuggestRequest) (string, error) {
	return "AI suggestions are not configured. Please enable an AI provider in settings.", nil
}

// SuggestStream returns a single chunk with the stub message.
func (s *StubProvider) SuggestStream(ctx context.Context, req *SuggestRequest) (<-chan SuggestChunk, error) {
	ch := make(chan SuggestChunk, 1)
	go func() {
		defer close(ch)
		ch <- SuggestChunk{
			ContentDelta: "AI suggestions are not configured. Please enable an AI provider in settings.",
			Done:         true,
		}
	}()
	return ch, nil
}
