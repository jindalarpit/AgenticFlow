package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIAdapter_ProviderType(t *testing.T) {
	adapter := &OpenAIAdapter{}
	if got := adapter.ProviderType(); got != "openai" {
		t.Errorf("ProviderType() = %q, want %q", got, "openai")
	}
}

func TestOpenAIAdapter_ChatCompletion_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("OpenAI-Organization") != "test-org" {
			t.Errorf("unexpected org header: %s", r.Header.Get("OpenAI-Organization"))
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"choices": [{"message": {"content": "Hello!"}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
		}`)
	}))
	defer server.Close()

	adapter := &OpenAIAdapter{}
	creds := json.RawMessage(fmt.Sprintf(`{"api_key":"test-key","base_url":"%s","organization":"test-org"}`, server.URL))

	resp, err := adapter.ChatCompletion(context.Background(), creds, ChatCompletionRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello!" {
		t.Errorf("Content = %q, want %q", resp.Content, "Hello!")
	}
	if resp.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 5 {
		t.Errorf("CompletionTokens = %d, want 5", resp.Usage.CompletionTokens)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", resp.Usage.TotalTokens)
	}
}

func TestOpenAIAdapter_ChatCompletion_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"invalid api key"}}`)
	}))
	defer server.Close()

	adapter := &OpenAIAdapter{}
	creds := json.RawMessage(fmt.Sprintf(`{"api_key":"bad-key","base_url":"%s"}`, server.URL))

	_, err := adapter.ChatCompletion(context.Background(), creds, ChatCompletionRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should contain status code 401, got: %v", err)
	}
}

func TestOpenAIAdapter_ChatCompletionStream_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"},\"finish_reason\":null}]}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\" world\"},\"finish_reason\":null}]}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"\"},\"finish_reason\":\"stop\"}]}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	adapter := &OpenAIAdapter{}
	creds := json.RawMessage(fmt.Sprintf(`{"api_key":"test-key","base_url":"%s"}`, server.URL))

	chunkCh, errCh := adapter.ChatCompletionStream(context.Background(), creds, ChatCompletionRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	})

	var chunks []ChatCompletionChunk
	for chunk := range chunkCh {
		chunks = append(chunks, chunk)
	}

	// Check for errors
	for err := range errCh {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[0].Content != "Hello" {
		t.Errorf("chunk[0].Content = %q, want %q", chunks[0].Content, "Hello")
	}
	if chunks[1].Content != " world" {
		t.Errorf("chunk[1].Content = %q, want %q", chunks[1].Content, " world")
	}
	if chunks[2].FinishReason != "stop" {
		t.Errorf("chunk[2].FinishReason = %q, want %q", chunks[2].FinishReason, "stop")
	}
}

func TestOpenAIAdapter_ChatCompletionStream_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"message":"rate limited"}}`)
	}))
	defer server.Close()

	adapter := &OpenAIAdapter{}
	creds := json.RawMessage(fmt.Sprintf(`{"api_key":"test-key","base_url":"%s"}`, server.URL))

	chunkCh, errCh := adapter.ChatCompletionStream(context.Background(), creds, ChatCompletionRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	})

	// Drain chunks
	for range chunkCh {
	}

	var gotErr error
	for err := range errCh {
		gotErr = err
	}
	if gotErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(gotErr.Error(), "429") {
		t.Errorf("error should contain status code 429, got: %v", gotErr)
	}
}

func TestOpenAIAdapter_ValidateCredentials_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/models" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"id":"gpt-4"},{"id":"gpt-3.5-turbo"},{"id":""}]}`)
	}))
	defer server.Close()

	adapter := &OpenAIAdapter{}
	creds := json.RawMessage(fmt.Sprintf(`{"api_key":"test-key","base_url":"%s"}`, server.URL))

	models, err := adapter.ValidateCredentials(context.Background(), creds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d: %v", len(models), models)
	}
	if models[0] != "gpt-4" {
		t.Errorf("models[0] = %q, want %q", models[0], "gpt-4")
	}
	if models[1] != "gpt-3.5-turbo" {
		t.Errorf("models[1] = %q, want %q", models[1], "gpt-3.5-turbo")
	}
}

func TestOpenAIAdapter_ListModels_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"id":"gpt-4o"},{"id":"gpt-4o-mini"}]}`)
	}))
	defer server.Close()

	adapter := &OpenAIAdapter{}
	creds := json.RawMessage(fmt.Sprintf(`{"api_key":"test-key","base_url":"%s"}`, server.URL))

	models, err := adapter.ListModels(context.Background(), creds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
}

func TestOpenAIAdapter_ValidateCredentials_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"Incorrect API key"}}`)
	}))
	defer server.Close()

	adapter := &OpenAIAdapter{}
	creds := json.RawMessage(fmt.Sprintf(`{"api_key":"bad-key","base_url":"%s"}`, server.URL))

	_, err := adapter.ValidateCredentials(context.Background(), creds)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should contain 401, got: %v", err)
	}
}

func TestOpenAIAdapter_InvalidCredentials(t *testing.T) {
	adapter := &OpenAIAdapter{}

	// Missing api_key
	creds := json.RawMessage(`{"base_url":"https://api.openai.com/v1"}`)
	_, err := adapter.ValidateCredentials(context.Background(), creds)
	if err == nil {
		t.Fatal("expected error for missing api_key")
	}
	if !strings.Contains(err.Error(), "api_key is required") {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid JSON
	creds = json.RawMessage(`not json`)
	_, err = adapter.ValidateCredentials(context.Background(), creds)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestOpenAIAdapter_DefaultBaseURL(t *testing.T) {
	c := &openAICreds{APIKey: "test"}
	if got := c.baseURL(); got != defaultOpenAIBaseURL {
		t.Errorf("baseURL() = %q, want %q", got, defaultOpenAIBaseURL)
	}
}

func TestOpenAIAdapter_CustomBaseURL_TrailingSlash(t *testing.T) {
	c := &openAICreds{APIKey: "test", BaseURL: "https://custom.api.com/v1/"}
	if got := c.baseURL(); got != "https://custom.api.com/v1" {
		t.Errorf("baseURL() = %q, want %q", got, "https://custom.api.com/v1")
	}
}
