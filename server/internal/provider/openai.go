package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const defaultOpenAIBaseURL = "https://api.openai.com/v1"

// openAICreds represents the credential JSON for OpenAI providers.
type openAICreds struct {
	APIKey       string `json:"api_key"`
	BaseURL      string `json:"base_url"`
	Organization string `json:"organization"`
}

// openAIChatResponse represents the OpenAI chat completions API response.
type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// openAIStreamChunk represents a single SSE chunk from the OpenAI streaming API.
type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// openAIModelsResponse represents the OpenAI /models endpoint response.
type openAIModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// OpenAIAdapter implements ProviderAdapter for the OpenAI API.
type OpenAIAdapter struct{}

func (a *OpenAIAdapter) ProviderType() string {
	return "openai"
}

// ChatCompletion sends a non-streaming chat completion request to OpenAI.
func (a *OpenAIAdapter) ChatCompletion(ctx context.Context, creds json.RawMessage, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	c, err := parseOpenAICreds(creds)
	if err != nil {
		return nil, err
	}

	// Build request body
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("openai: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL()+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: failed to create request: %w", err)
	}
	c.setHeaders(httpReq)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("openai: API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("openai: failed to decode response: %w", err)
	}

	content := ""
	if len(chatResp.Choices) > 0 {
		content = chatResp.Choices[0].Message.Content
	}

	return &ChatCompletionResponse{
		Content: content,
		Usage: TokenUsage{
			PromptTokens:     chatResp.Usage.PromptTokens,
			CompletionTokens: chatResp.Usage.CompletionTokens,
			TotalTokens:      chatResp.Usage.TotalTokens,
		},
	}, nil
}

// ChatCompletionStream sends a streaming chat completion request to OpenAI.
// It returns a channel of chunks and an error channel.
func (a *OpenAIAdapter) ChatCompletionStream(ctx context.Context, creds json.RawMessage, req ChatCompletionRequest) (<-chan ChatCompletionChunk, <-chan error) {
	chunkCh := make(chan ChatCompletionChunk, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunkCh)
		defer close(errCh)

		c, err := parseOpenAICreds(creds)
		if err != nil {
			errCh <- err
			return
		}

		// Force streaming
		req.Stream = true
		body, err := json.Marshal(req)
		if err != nil {
			errCh <- fmt.Errorf("openai: failed to marshal request: %w", err)
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL()+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			errCh <- fmt.Errorf("openai: failed to create request: %w", err)
			return
		}
		c.setHeaders(httpReq)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			errCh <- fmt.Errorf("openai: request failed: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			errCh <- fmt.Errorf("openai: API returned status %d: %s", resp.StatusCode, string(respBody))
			return
		}

		// Parse SSE stream
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// SSE lines starting with "data: " contain the JSON payload
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			// "[DONE]" signals end of stream
			if data == "[DONE]" {
				return
			}

			var chunk openAIStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				// Skip malformed chunks
				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			finishReason := ""
			if chunk.Choices[0].FinishReason != nil {
				finishReason = *chunk.Choices[0].FinishReason
			}

			chunkCh <- ChatCompletionChunk{
				Content:      chunk.Choices[0].Delta.Content,
				FinishReason: finishReason,
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("openai: error reading stream: %w", err)
		}
	}()

	return chunkCh, errCh
}

// ValidateCredentials validates OpenAI credentials by calling GET /models.
// Returns the list of available model IDs on success.
func (a *OpenAIAdapter) ValidateCredentials(ctx context.Context, creds json.RawMessage) ([]string, error) {
	return a.listModelsInternal(ctx, creds)
}

// ListModels queries the OpenAI API for available models.
func (a *OpenAIAdapter) ListModels(ctx context.Context, creds json.RawMessage) ([]string, error) {
	return a.listModelsInternal(ctx, creds)
}

// listModelsInternal performs the actual GET /models call.
func (a *OpenAIAdapter) listModelsInternal(ctx context.Context, creds json.RawMessage) ([]string, error) {
	c, err := parseOpenAICreds(creds)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL()+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("openai: failed to create request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("openai: API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var modelsResp openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("openai: failed to decode models response: %w", err)
	}

	models := make([]string, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}

	return models, nil
}

// parseOpenAICreds parses and validates the OpenAI credential JSON.
func parseOpenAICreds(creds json.RawMessage) (*openAICreds, error) {
	var c openAICreds
	if err := json.Unmarshal(creds, &c); err != nil {
		return nil, fmt.Errorf("openai: failed to parse credentials: %w", err)
	}
	if c.APIKey == "" {
		return nil, fmt.Errorf("openai: api_key is required")
	}
	return &c, nil
}

// baseURL returns the configured base URL or the default OpenAI API URL.
func (c *openAICreds) baseURL() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return defaultOpenAIBaseURL
}

// setHeaders sets the authorization and optional organization headers on the request.
func (c *openAICreds) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	if c.Organization != "" {
		req.Header.Set("OpenAI-Organization", c.Organization)
	}
}
