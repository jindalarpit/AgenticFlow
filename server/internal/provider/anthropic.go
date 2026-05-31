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

const (
	anthropicDefaultBaseURL = "https://api.anthropic.com/v1"
	anthropicAPIVersion     = "2023-06-01"
)

// anthropicKnownModels is the hardcoded list of known Anthropic models.
var anthropicKnownModels = []string{
	"claude-sonnet-4-20250514",
	"claude-3-5-sonnet-20241022",
	"claude-3-5-haiku-20241022",
	"claude-3-opus-20240229",
	"claude-3-haiku-20240307",
}

// anthropicCredentials represents the credential JSON for Anthropic.
type anthropicCredentials struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
}

func (c *anthropicCredentials) baseURL() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return anthropicDefaultBaseURL
}

// anthropicRequest is the Anthropic messages API request body.
type anthropicRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	System    string        `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Stream    bool          `json:"stream"`
}

// anthropicMessage is a message in the Anthropic messages array (user/assistant only).
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse is the non-streaming response from the Anthropic messages API.
type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
	Usage   anthropicUsage          `json:"usage"`
	StopReason string              `json:"stop_reason"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// AnthropicAdapter implements ProviderAdapter for the Anthropic API.
type AnthropicAdapter struct{}

func (a *AnthropicAdapter) ProviderType() string {
	return "anthropic"
}

func (a *AnthropicAdapter) ChatCompletion(ctx context.Context, creds json.RawMessage, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	c, err := parseAnthropicCredentials(creds)
	if err != nil {
		return nil, err
	}

	body, err := buildAnthropicRequest(req, false)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL()+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: failed to create request: %w", err)
	}
	setAnthropicHeaders(httpReq, c.APIKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("anthropic: API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var anthropicResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, fmt.Errorf("anthropic: failed to decode response: %w", err)
	}

	var content strings.Builder
	for _, block := range anthropicResp.Content {
		if block.Type == "text" {
			content.WriteString(block.Text)
		}
	}

	return &ChatCompletionResponse{
		Content: content.String(),
		Usage: TokenUsage{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		},
	}, nil
}

func (a *AnthropicAdapter) ChatCompletionStream(ctx context.Context, creds json.RawMessage, req ChatCompletionRequest) (<-chan ChatCompletionChunk, <-chan error) {
	chunkCh := make(chan ChatCompletionChunk, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunkCh)
		defer close(errCh)

		c, err := parseAnthropicCredentials(creds)
		if err != nil {
			errCh <- err
			return
		}

		body, err := buildAnthropicRequest(req, true)
		if err != nil {
			errCh <- err
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL()+"/messages", bytes.NewReader(body))
		if err != nil {
			errCh <- fmt.Errorf("anthropic: failed to create request: %w", err)
			return
		}
		setAnthropicHeaders(httpReq, c.APIKey)

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			errCh <- fmt.Errorf("anthropic: request failed: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			errCh <- fmt.Errorf("anthropic: API returned status %d: %s", resp.StatusCode, string(respBody))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		var currentEvent string

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "event: ") {
				currentEvent = strings.TrimPrefix(line, "event: ")
				continue
			}

			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")

				switch currentEvent {
				case "content_block_delta":
					var delta struct {
						Delta struct {
							Type string `json:"type"`
							Text string `json:"text"`
						} `json:"delta"`
					}
					if err := json.Unmarshal([]byte(data), &delta); err == nil && delta.Delta.Text != "" {
						chunkCh <- ChatCompletionChunk{
							Content: delta.Delta.Text,
						}
					}

				case "message_delta":
					var msgDelta struct {
						Delta struct {
							StopReason string `json:"stop_reason"`
						} `json:"delta"`
						Usage *anthropicUsage `json:"usage"`
					}
					if err := json.Unmarshal([]byte(data), &msgDelta); err == nil {
						finishReason := ""
						if msgDelta.Delta.StopReason == "end_turn" || msgDelta.Delta.StopReason == "stop" {
							finishReason = "stop"
						} else if msgDelta.Delta.StopReason == "max_tokens" {
							finishReason = "length"
						} else if msgDelta.Delta.StopReason != "" {
							finishReason = msgDelta.Delta.StopReason
						}
						if finishReason != "" {
							chunkCh <- ChatCompletionChunk{
								FinishReason: finishReason,
							}
						}
					}

				case "message_stop":
					// Stream is complete
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("anthropic: stream read error: %w", err)
		}
	}()

	return chunkCh, errCh
}

func (a *AnthropicAdapter) ValidateCredentials(ctx context.Context, creds json.RawMessage) ([]string, error) {
	c, err := parseAnthropicCredentials(creds)
	if err != nil {
		return nil, err
	}

	// Send a minimal request to validate credentials.
	// Non-401/403 responses mean credentials are valid.
	validateReq := anthropicRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 1,
		Messages: []anthropicMessage{
			{Role: "user", Content: "hi"},
		},
		Stream: false,
	}

	body, err := json.Marshal(validateReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: failed to marshal validation request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL()+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: failed to create request: %w", err)
	}
	setAnthropicHeaders(httpReq, c.APIKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: request failed: %w", err)
	}
	defer resp.Body.Close()

	// 401 or 403 means invalid credentials
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("anthropic: invalid credentials: authentication failed")
	}

	// Any other status (including 400, 200, 429, etc.) means credentials are valid
	return a.ListModels(ctx, creds)
}

func (a *AnthropicAdapter) ListModels(ctx context.Context, creds json.RawMessage) ([]string, error) {
	// Anthropic does not have a public models listing API.
	// Return a hardcoded list of known models.
	models := make([]string, len(anthropicKnownModels))
	copy(models, anthropicKnownModels)
	return models, nil
}

// parseAnthropicCredentials parses and validates the credential JSON.
func parseAnthropicCredentials(creds json.RawMessage) (*anthropicCredentials, error) {
	var c anthropicCredentials
	if err := json.Unmarshal(creds, &c); err != nil {
		return nil, fmt.Errorf("anthropic: failed to parse credentials: %w", err)
	}
	if c.APIKey == "" {
		return nil, fmt.Errorf("anthropic: api_key is required")
	}
	return &c, nil
}

// buildAnthropicRequest converts a ChatCompletionRequest to the Anthropic API format.
func buildAnthropicRequest(req ChatCompletionRequest, stream bool) ([]byte, error) {
	ar := anthropicRequest{
		Model:  req.Model,
		Stream: stream,
	}

	// Default max_tokens to 4096 if not specified
	if req.MaxTokens > 0 {
		ar.MaxTokens = req.MaxTokens
	} else {
		ar.MaxTokens = 4096
	}

	// Separate system message from user/assistant messages.
	// Anthropic requires system message in a top-level field, not in the messages array.
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if ar.System != "" {
				ar.System += "\n"
			}
			ar.System += msg.Content
		} else {
			ar.Messages = append(ar.Messages, anthropicMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// Ensure at least one user message exists
	if len(ar.Messages) == 0 {
		ar.Messages = []anthropicMessage{{Role: "user", Content: ""}}
	}

	return json.Marshal(ar)
}

// setAnthropicHeaders sets the required headers for Anthropic API requests.
func setAnthropicHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)
}
