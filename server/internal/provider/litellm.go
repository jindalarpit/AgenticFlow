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

// liteLLMCredentials holds the parsed credential fields for a LiteLLM provider.
type liteLLMCredentials struct {
	APIKey            string `json:"api_key"`
	APIBase           string `json:"api_base"`
	CustomLLMProvider string `json:"custom_llm_provider"`
}

// LiteLLMAdapter implements ProviderAdapter for LiteLLM-compatible endpoints.
type LiteLLMAdapter struct{}

func (a *LiteLLMAdapter) ProviderType() string {
	return "litellm"
}

func (a *LiteLLMAdapter) ChatCompletion(ctx context.Context, creds json.RawMessage, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	parsed, err := parseLiteLLMCredentials(creds)
	if err != nil {
		return nil, err
	}

	model := resolveModel(req.Model, parsed.CustomLLMProvider)

	// Build OpenAI-compatible request body.
	body := map[string]interface{}{
		"model":    model,
		"messages": req.Messages,
		"stream":   false,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("litellm: failed to marshal request: %w", err)
	}

	url := strings.TrimRight(parsed.APIBase, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("litellm: failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+parsed.APIKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("litellm: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("litellm: provider returned error: %d %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("litellm: failed to decode response: %w", err)
	}

	content := ""
	if len(result.Choices) > 0 {
		content = result.Choices[0].Message.Content
	}

	return &ChatCompletionResponse{
		Content: content,
		Usage: TokenUsage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}, nil
}

func (a *LiteLLMAdapter) ChatCompletionStream(ctx context.Context, creds json.RawMessage, req ChatCompletionRequest) (<-chan ChatCompletionChunk, <-chan error) {
	chunkCh := make(chan ChatCompletionChunk, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunkCh)
		defer close(errCh)

		parsed, err := parseLiteLLMCredentials(creds)
		if err != nil {
			errCh <- err
			return
		}

		model := resolveModel(req.Model, parsed.CustomLLMProvider)

		body := map[string]interface{}{
			"model":    model,
			"messages": req.Messages,
			"stream":   true,
		}
		if req.MaxTokens > 0 {
			body["max_tokens"] = req.MaxTokens
		}
		if req.Temperature != nil {
			body["temperature"] = *req.Temperature
		}

		payload, err := json.Marshal(body)
		if err != nil {
			errCh <- fmt.Errorf("litellm: failed to marshal request: %w", err)
			return
		}

		url := strings.TrimRight(parsed.APIBase, "/") + "/chat/completions"
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			errCh <- fmt.Errorf("litellm: failed to create request: %w", err)
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+parsed.APIKey)

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			errCh <- fmt.Errorf("litellm: request failed: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			errCh <- fmt.Errorf("litellm: provider returned error: %d %s", resp.StatusCode, string(respBody))
			return
		}

		// Parse SSE stream (OpenAI-compatible format).
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines and comments.
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}

			// SSE data lines start with "data: ".
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			// Stream termination signal.
			if data == "[DONE]" {
				return
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				// Skip malformed chunks.
				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			choice := chunk.Choices[0]
			finishReason := ""
			if choice.FinishReason != nil {
				finishReason = *choice.FinishReason
			}

			// Only send chunks that have content or a finish reason.
			if choice.Delta.Content != "" || finishReason != "" {
				chunkCh <- ChatCompletionChunk{
					Content:      choice.Delta.Content,
					FinishReason: finishReason,
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("litellm: error reading stream: %w", err)
		}
	}()

	return chunkCh, errCh
}

func (a *LiteLLMAdapter) ValidateCredentials(ctx context.Context, creds json.RawMessage) ([]string, error) {
	return a.ListModels(ctx, creds)
}

func (a *LiteLLMAdapter) ListModels(ctx context.Context, creds json.RawMessage) ([]string, error) {
	parsed, err := parseLiteLLMCredentials(creds)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(parsed.APIBase, "/") + "/models"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("litellm: failed to create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+parsed.APIKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("litellm: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("litellm: provider returned error: %d %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("litellm: failed to decode models response: %w", err)
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}

	return models, nil
}

// parseLiteLLMCredentials extracts and validates LiteLLM credentials from raw JSON.
func parseLiteLLMCredentials(creds json.RawMessage) (*liteLLMCredentials, error) {
	var parsed liteLLMCredentials
	if err := json.Unmarshal(creds, &parsed); err != nil {
		return nil, fmt.Errorf("litellm: failed to parse credentials: %w", err)
	}
	if parsed.APIKey == "" {
		return nil, fmt.Errorf("litellm: api_key is required")
	}
	if parsed.APIBase == "" {
		return nil, fmt.Errorf("litellm: api_base is required")
	}
	return &parsed, nil
}

// resolveModel prepends the custom_llm_provider prefix to the model name if set.
func resolveModel(model, customLLMProvider string) string {
	if customLLMProvider != "" {
		return customLLMProvider + "/" + model
	}
	return model
}
