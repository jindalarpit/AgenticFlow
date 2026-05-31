package provider

import (
	"context"
	"encoding/json"
)

// ChatMessage represents a single message in a chat completion request.
type ChatMessage struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"`
}

// ChatCompletionRequest is the unified request format for all providers.
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
}

// ChatCompletionChunk represents a single streaming chunk from the provider.
type ChatCompletionChunk struct {
	Content      string `json:"content"`       // text delta
	FinishReason string `json:"finish_reason"` // "", "stop", "length"
}

// TokenUsage holds token consumption metrics from the provider response.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionResponse is the final aggregated response.
type ChatCompletionResponse struct {
	Content string     `json:"content"`
	Usage   TokenUsage `json:"usage"`
}

// ProviderAdapter defines the interface each provider must implement.
type ProviderAdapter interface {
	// ChatCompletion sends a chat completion request and returns the full response.
	// Used when streaming is not requested or not supported.
	ChatCompletion(ctx context.Context, creds json.RawMessage, req ChatCompletionRequest) (*ChatCompletionResponse, error)

	// ChatCompletionStream sends a streaming chat completion request.
	// Returns a channel of chunks and an error channel.
	// The caller must consume the chunks channel until it is closed.
	ChatCompletionStream(ctx context.Context, creds json.RawMessage, req ChatCompletionRequest) (<-chan ChatCompletionChunk, <-chan error)

	// ValidateCredentials checks if the provided credentials are valid.
	// Returns the list of available models on success.
	ValidateCredentials(ctx context.Context, creds json.RawMessage) ([]string, error)

	// ListModels queries the provider for available models.
	ListModels(ctx context.Context, creds json.RawMessage) ([]string, error)

	// ProviderType returns the provider type string (e.g., "openai").
	ProviderType() string
}
