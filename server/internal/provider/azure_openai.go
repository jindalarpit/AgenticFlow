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

// azureOpenAICreds holds the parsed credential fields for Azure OpenAI.
type azureOpenAICreds struct {
	APIKey         string `json:"api_key"`
	Endpoint       string `json:"endpoint"`
	APIVersion     string `json:"api_version"`
	DeploymentName string `json:"deployment_name"`
}

// AzureOpenAIAdapter implements ProviderAdapter for the Azure OpenAI API.
type AzureOpenAIAdapter struct{}

func (a *AzureOpenAIAdapter) ProviderType() string {
	return "azure_openai"
}

func (a *AzureOpenAIAdapter) ChatCompletion(ctx context.Context, creds json.RawMessage, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	c, err := parseAzureOpenAICreds(creds)
	if err != nil {
		return nil, err
	}

	body := buildAzureChatRequest(req, false)
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("azure_openai: failed to marshal request: %w", err)
	}

	url := azureChatCompletionsURL(c)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("azure_openai: failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", c.APIKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("azure_openai: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("azure_openai: API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result azureChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("azure_openai: failed to decode response: %w", err)
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

func (a *AzureOpenAIAdapter) ChatCompletionStream(ctx context.Context, creds json.RawMessage, req ChatCompletionRequest) (<-chan ChatCompletionChunk, <-chan error) {
	chunkCh := make(chan ChatCompletionChunk, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunkCh)
		defer close(errCh)

		c, err := parseAzureOpenAICreds(creds)
		if err != nil {
			errCh <- err
			return
		}

		body := buildAzureChatRequest(req, true)
		payload, err := json.Marshal(body)
		if err != nil {
			errCh <- fmt.Errorf("azure_openai: failed to marshal request: %w", err)
			return
		}

		url := azureChatCompletionsURL(c)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			errCh <- fmt.Errorf("azure_openai: failed to create request: %w", err)
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("api-key", c.APIKey)

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			errCh <- fmt.Errorf("azure_openai: request failed: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			errCh <- fmt.Errorf("azure_openai: API returned status %d: %s", resp.StatusCode, string(respBody))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}

			var chunk azureStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			delta := chunk.Choices[0].Delta
			finishReason := ""
			if chunk.Choices[0].FinishReason != nil {
				finishReason = *chunk.Choices[0].FinishReason
			}

			if delta.Content != "" || finishReason != "" {
				chunkCh <- ChatCompletionChunk{
					Content:      delta.Content,
					FinishReason: finishReason,
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("azure_openai: error reading stream: %w", err)
		}
	}()

	return chunkCh, errCh
}

func (a *AzureOpenAIAdapter) ValidateCredentials(ctx context.Context, creds json.RawMessage) ([]string, error) {
	c, err := parseAzureOpenAICreds(creds)
	if err != nil {
		return nil, err
	}

	// GET {endpoint}/openai/deployments/{deployment_name}?api-version={api_version}
	url := fmt.Sprintf("%s/openai/deployments/%s?api-version=%s",
		strings.TrimRight(c.Endpoint, "/"), c.DeploymentName, c.APIVersion)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("azure_openai: failed to create validation request: %w", err)
	}
	httpReq.Header.Set("api-key", c.APIKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("azure_openai: validation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("azure_openai: validation failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Azure uses deployments, so return the deployment_name as the single model.
	return []string{c.DeploymentName}, nil
}

func (a *AzureOpenAIAdapter) ListModels(ctx context.Context, creds json.RawMessage) ([]string, error) {
	c, err := parseAzureOpenAICreds(creds)
	if err != nil {
		return nil, err
	}

	// Azure OpenAI uses deployments rather than a model list API.
	// Return the deployment_name as the single available model.
	return []string{c.DeploymentName}, nil
}

// --- Internal helpers ---

func parseAzureOpenAICreds(creds json.RawMessage) (*azureOpenAICreds, error) {
	var c azureOpenAICreds
	if err := json.Unmarshal(creds, &c); err != nil {
		return nil, fmt.Errorf("azure_openai: failed to parse credentials: %w", err)
	}
	if c.APIKey == "" {
		return nil, fmt.Errorf("azure_openai: api_key is required")
	}
	if c.Endpoint == "" {
		return nil, fmt.Errorf("azure_openai: endpoint is required")
	}
	if c.APIVersion == "" {
		return nil, fmt.Errorf("azure_openai: api_version is required")
	}
	if c.DeploymentName == "" {
		return nil, fmt.Errorf("azure_openai: deployment_name is required")
	}
	return &c, nil
}

func azureChatCompletionsURL(c *azureOpenAICreds) string {
	return fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		strings.TrimRight(c.Endpoint, "/"), c.DeploymentName, c.APIVersion)
}

// buildAzureChatRequest constructs the Azure OpenAI chat completions request body.
func buildAzureChatRequest(req ChatCompletionRequest, stream bool) map[string]interface{} {
	messages := make([]map[string]string, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = map[string]string{
			"role":    m.Role,
			"content": m.Content,
		}
	}

	body := map[string]interface{}{
		"messages": messages,
		"stream":   stream,
	}

	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}

	return body
}

// --- Response types for Azure OpenAI ---

type azureChatCompletionResponse struct {
	Choices []azureChoice `json:"choices"`
	Usage   azureUsage    `json:"usage"`
}

type azureChoice struct {
	Message azureMessage `json:"message"`
}

type azureMessage struct {
	Content string `json:"content"`
}

type azureUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// --- Streaming response types ---

type azureStreamChunk struct {
	Choices []azureStreamChoice `json:"choices"`
}

type azureStreamChoice struct {
	Delta        azureStreamDelta `json:"delta"`
	FinishReason *string          `json:"finish_reason"`
}

type azureStreamDelta struct {
	Content string `json:"content"`
}
