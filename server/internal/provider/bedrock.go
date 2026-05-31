package provider

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// bedrockCredentials holds the parsed AWS credentials for Bedrock.
type bedrockCredentials struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	Region          string `json:"region"`
	SessionToken    string `json:"session_token,omitempty"`
}

func parseBedrockCredentials(creds json.RawMessage) (*bedrockCredentials, error) {
	var c bedrockCredentials
	if err := json.Unmarshal(creds, &c); err != nil {
		return nil, fmt.Errorf("aws_bedrock: invalid credentials JSON: %w", err)
	}
	if c.AccessKeyID == "" {
		return nil, fmt.Errorf("aws_bedrock: access_key_id is required")
	}
	if c.SecretAccessKey == "" {
		return nil, fmt.Errorf("aws_bedrock: secret_access_key is required")
	}
	if c.Region == "" {
		return nil, fmt.Errorf("aws_bedrock: region is required")
	}
	return &c, nil
}

// BedrockAdapter implements ProviderAdapter for the AWS Bedrock API.
type BedrockAdapter struct{}

func (a *BedrockAdapter) ProviderType() string {
	return "aws_bedrock"
}

// ChatCompletion sends a non-streaming chat completion request using the Bedrock Converse API.
func (a *BedrockAdapter) ChatCompletion(ctx context.Context, creds json.RawMessage, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	c, err := parseBedrockCredentials(creds)
	if err != nil {
		return nil, err
	}

	body := buildConverseRequestBody(req)
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("aws_bedrock: failed to marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/converse", c.Region, req.Model)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("aws_bedrock: failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	signAWSRequest(httpReq, payload, c, "bedrock")

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("aws_bedrock: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("aws_bedrock: failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := string(respBody)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, fmt.Errorf("aws_bedrock: API returned status %d: %s", resp.StatusCode, preview)
	}

	return parseConverseResponse(respBody)
}

// ChatCompletionStream sends a streaming chat completion request using the Bedrock ConverseStream API.
func (a *BedrockAdapter) ChatCompletionStream(ctx context.Context, creds json.RawMessage, req ChatCompletionRequest) (<-chan ChatCompletionChunk, <-chan error) {
	chunkCh := make(chan ChatCompletionChunk, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunkCh)
		defer close(errCh)

		c, err := parseBedrockCredentials(creds)
		if err != nil {
			errCh <- err
			return
		}

		body := buildConverseRequestBody(req)
		payload, err := json.Marshal(body)
		if err != nil {
			errCh <- fmt.Errorf("aws_bedrock: failed to marshal request: %w", err)
			return
		}

		endpoint := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/converse-stream", c.Region, req.Model)

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			errCh <- fmt.Errorf("aws_bedrock: failed to create request: %w", err)
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "application/vnd.amazon.eventstream")

		signAWSRequest(httpReq, payload, c, "bedrock")

		client := &http.Client{Timeout: 300 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			errCh <- fmt.Errorf("aws_bedrock: stream request failed: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			respBody, _ := io.ReadAll(resp.Body)
			preview := string(respBody)
			if len(preview) > 200 {
				preview = preview[:200]
			}
			errCh <- fmt.Errorf("aws_bedrock: API returned status %d: %s", resp.StatusCode, preview)
			return
		}

		// Parse the event stream response. Bedrock ConverseStream returns
		// newline-delimited JSON events (each line is a JSON object with a type field).
		scanner := bufio.NewScanner(resp.Body)
		// Increase buffer for potentially large events
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			// Try to parse as JSON event
			var event map[string]json.RawMessage
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				// Not JSON, skip
				continue
			}

			// Handle contentBlockDelta events
			if deltaRaw, ok := event["contentBlockDelta"]; ok {
				var delta struct {
					Delta struct {
						Text string `json:"text"`
					} `json:"delta"`
				}
				if err := json.Unmarshal(deltaRaw, &delta); err == nil && delta.Delta.Text != "" {
					chunkCh <- ChatCompletionChunk{
						Content: delta.Delta.Text,
					}
				}
			}

			// Handle messageStop events
			if _, ok := event["messageStop"]; ok {
				chunkCh <- ChatCompletionChunk{
					FinishReason: "stop",
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("aws_bedrock: error reading stream: %w", err)
		}
	}()

	return chunkCh, errCh
}

// ValidateCredentials validates AWS credentials by calling the Bedrock ListFoundationModels API.
func (a *BedrockAdapter) ValidateCredentials(ctx context.Context, creds json.RawMessage) ([]string, error) {
	return a.ListModels(ctx, creds)
}

// ListModels calls the Bedrock ListFoundationModels API and returns model IDs.
func (a *BedrockAdapter) ListModels(ctx context.Context, creds json.RawMessage) ([]string, error) {
	c, err := parseBedrockCredentials(creds)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("https://bedrock.%s.amazonaws.com/foundation-models", c.Region)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("aws_bedrock: failed to create request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")

	signAWSRequest(httpReq, nil, c, "bedrock")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("aws_bedrock: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("aws_bedrock: failed to read response: %w", err)
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, fmt.Errorf("aws_bedrock: invalid credentials: authentication failed")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := string(respBody)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, fmt.Errorf("aws_bedrock: API returned status %d: %s", resp.StatusCode, preview)
	}

	var result struct {
		ModelSummaries []struct {
			ModelID string `json:"modelId"`
		} `json:"modelSummaries"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("aws_bedrock: failed to parse models response: %w", err)
	}

	models := make([]string, 0, len(result.ModelSummaries))
	for _, m := range result.ModelSummaries {
		if m.ModelID != "" {
			models = append(models, m.ModelID)
		}
	}

	return models, nil
}

// --- Converse API request/response types ---

type converseRequest struct {
	Messages     []converseMessage      `json:"messages"`
	System       []converseSystemBlock  `json:"system,omitempty"`
	InferenceConfig *converseInferenceConfig `json:"inferenceConfig,omitempty"`
}

type converseMessage struct {
	Role    string                `json:"role"`
	Content []converseContentBlock `json:"content"`
}

type converseContentBlock struct {
	Text string `json:"text,omitempty"`
}

type converseSystemBlock struct {
	Text string `json:"text"`
}

type converseInferenceConfig struct {
	MaxTokens   int      `json:"maxTokens,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
}

func buildConverseRequestBody(req ChatCompletionRequest) converseRequest {
	cr := converseRequest{}

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			cr.System = append(cr.System, converseSystemBlock{Text: msg.Content})
			continue
		}
		role := msg.Role
		if role == "assistant" {
			role = "assistant"
		}
		cr.Messages = append(cr.Messages, converseMessage{
			Role: role,
			Content: []converseContentBlock{
				{Text: msg.Content},
			},
		})
	}

	if req.MaxTokens > 0 || req.Temperature != nil {
		cr.InferenceConfig = &converseInferenceConfig{
			MaxTokens:   req.MaxTokens,
			Temperature: req.Temperature,
		}
	}

	return cr
}

func parseConverseResponse(body []byte) (*ChatCompletionResponse, error) {
	var resp struct {
		Output struct {
			Message struct {
				Role    string `json:"role"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
			TotalTokens  int `json:"totalTokens"`
		} `json:"usage"`
		StopReason string `json:"stopReason"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("aws_bedrock: failed to parse response: %w", err)
	}

	var content strings.Builder
	for _, block := range resp.Output.Message.Content {
		content.WriteString(block.Text)
	}

	return &ChatCompletionResponse{
		Content: content.String(),
		Usage: TokenUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

// --- AWS Signature V4 signing ---

// signAWSRequest signs an HTTP request using AWS Signature Version 4.
func signAWSRequest(req *http.Request, payload []byte, creds *bedrockCredentials, service string) {
	now := time.Now().UTC()
	datestamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	req.Header.Set("X-Amz-Date", amzDate)
	if creds.SessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", creds.SessionToken)
	}

	// Ensure Host header is set
	if req.Header.Get("Host") == "" {
		req.Header.Set("Host", req.URL.Host)
	}

	// Step 1: Create canonical request
	payloadHash := hashSHA256(payload)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	canonicalHeaders, signedHeaders := buildCanonicalHeaders(req)

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI(req),
		canonicalQueryString(req),
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	// Step 2: Create string to sign
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", datestamp, creds.Region, service)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		hashSHA256([]byte(canonicalRequest)),
	}, "\n")

	// Step 3: Calculate signature
	signingKey := deriveSigningKey(creds.SecretAccessKey, datestamp, creds.Region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	// Step 4: Add authorization header
	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		creds.AccessKeyID, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)
}

func canonicalURI(req *http.Request) string {
	path := req.URL.Path
	if path == "" {
		path = "/"
	}
	return path
}

func canonicalQueryString(req *http.Request) string {
	params := req.URL.Query()
	if len(params) == 0 {
		return ""
	}

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		for _, v := range params[k] {
			pairs = append(pairs, fmt.Sprintf("%s=%s", uriEncode(k), uriEncode(v)))
		}
	}
	return strings.Join(pairs, "&")
}

func buildCanonicalHeaders(req *http.Request) (canonicalHeaders string, signedHeaders string) {
	// Collect headers to sign
	headers := make(map[string]string)
	headerKeys := make([]string, 0)

	for key := range req.Header {
		lowerKey := strings.ToLower(key)
		// Sign content-type, host, and all x-amz-* headers
		if lowerKey == "content-type" || lowerKey == "host" || strings.HasPrefix(lowerKey, "x-amz-") {
			headers[lowerKey] = strings.TrimSpace(req.Header.Get(key))
			headerKeys = append(headerKeys, lowerKey)
		}
	}

	sort.Strings(headerKeys)

	var canonicalBuf strings.Builder
	for _, k := range headerKeys {
		canonicalBuf.WriteString(k)
		canonicalBuf.WriteString(":")
		canonicalBuf.WriteString(headers[k])
		canonicalBuf.WriteString("\n")
	}

	return canonicalBuf.String(), strings.Join(headerKeys, ";")
}

func deriveSigningKey(secret, datestamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(datestamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func hashSHA256(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func uriEncode(s string) string {
	var buf strings.Builder
	for _, b := range []byte(s) {
		if isUnreserved(b) {
			buf.WriteByte(b)
		} else {
			buf.WriteString(fmt.Sprintf("%%%02X", b))
		}
	}
	return buf.String()
}

func isUnreserved(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~'
}
