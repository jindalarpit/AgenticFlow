package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// ── JSON-RPC 2.0 Types ──

// rpcRequest is a JSON-RPC 2.0 request message sent from client to server.
type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// rpcResponse is a JSON-RPC 2.0 response message received from server.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.Number     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is the error object in a JSON-RPC 2.0 response.
type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// rpcNotification is a JSON-RPC 2.0 notification (no id field).
type rpcNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// pendingRPC tracks an in-flight JSON-RPC request awaiting a response.
type pendingRPC struct {
	ch     chan rpcResult
	method string
}

// rpcResult carries the outcome of a JSON-RPC request.
type rpcResult struct {
	result json.RawMessage
	err    error
}

// ── ACP Client (shared JSON-RPC 2.0 transport) ──

// acpClient implements the ACP JSON-RPC 2.0 transport used by Kiro, Hermes,
// and Kimi backends. It manages request/response correlation, notification
// dispatch, and the session lifecycle.
type acpClient struct {
	cfg   Config
	stdin interface{ Write([]byte) (int, error) }

	writeMu sync.Mutex // serialises stdin.Write calls across goroutines
	mu      sync.Mutex
	nextID  int
	pending map[int]*pendingRPC

	sessionID string

	// pendingTools buffers tool calls whose input streams in across
	// multiple ACP tool_call_update messages. We defer emitting
	// MessageToolUse until we see status=completed/failed or have
	// a full arg set.
	toolMu       sync.Mutex
	pendingTools map[string]*acpPendingToolCall

	usageMu sync.Mutex
	usage   TokenUsage

	// Callbacks set by the backend.
	onMessage    func(Message)
	onPromptDone func(acpPromptResult)
	// acceptNotification gates session updates so history replay
	// during resume is dropped.
	acceptNotification func(updateType string) bool
}

// acpPendingToolCall buffers state for a tool call while its arguments
// are streaming in.
type acpPendingToolCall struct {
	toolName string
	input    map[string]any
	argsText string
	emitted  bool
}

// acpPromptResult carries the outcome of a session/prompt response.
type acpPromptResult struct {
	stopReason string
	usage      TokenUsage
}

// ── ACP Client Methods ──

// writeLine serialises concurrent JSON-RPC writes so request() and
// handleAgentRequest() don't interleave frames on the stdin pipe.
func (c *acpClient) writeLine(data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := c.stdin.Write(data)
	return err
}

// request sends a JSON-RPC 2.0 request and waits for the response.
// It blocks until the response arrives or the context is cancelled.
func (c *acpClient) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	pr := &pendingRPC{ch: make(chan rpcResult, 1), method: method}
	c.pending[id] = pr
	c.mu.Unlock()

	msg := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}
	data = append(data, '\n')
	if err := c.writeLine(data); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("write %s: %w", method, err)
	}

	select {
	case res := <-pr.ch:
		return res.result, res.err
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}

// closeAllPending resolves all pending requests with the given error.
// Called when the agent process exits.
func (c *acpClient) closeAllPending(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, pr := range c.pending {
		pr.ch <- rpcResult{err: err}
		delete(c.pending, id)
	}
}

// handleLine parses a single line from the ACP agent's stdout and
// dispatches it as a response, agent→client request, or notification.
func (c *acpClient) handleLine(line string) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return
	}

	// Agent → client request: has id + method (no result/error).
	if _, hasID := raw["id"]; hasID {
		if _, hasResult := raw["result"]; hasResult {
			c.handleResponse(raw)
			return
		}
		if _, hasError := raw["error"]; hasError {
			c.handleResponse(raw)
			return
		}
		if _, hasMethod := raw["method"]; hasMethod {
			c.handleAgentRequest(raw)
			return
		}
	}

	// Notification (no id, has method) — session updates.
	if _, hasMethod := raw["method"]; hasMethod {
		c.handleNotification(raw)
	}
}

// handleResponse correlates a JSON-RPC response with its pending request.
func (c *acpClient) handleResponse(raw map[string]json.RawMessage) {
	var id int
	if err := json.Unmarshal(raw["id"], &id); err != nil {
		// Try float (JSON numbers are floats by default).
		var fid float64
		if err := json.Unmarshal(raw["id"], &fid); err != nil {
			return
		}
		id = int(fid)
	}

	c.mu.Lock()
	pr, ok := c.pending[id]
	if ok {
		delete(c.pending, id)
	}
	c.mu.Unlock()

	if !ok {
		return
	}

	if errData, hasErr := raw["error"]; hasErr {
		var rpcErr rpcError
		_ = json.Unmarshal(errData, &rpcErr)
		detail := ""
		if len(rpcErr.Data) > 0 && string(rpcErr.Data) != "null" {
			var s string
			if err := json.Unmarshal(rpcErr.Data, &s); err == nil {
				detail = s
			} else {
				detail = string(rpcErr.Data)
			}
		}
		if detail != "" {
			pr.ch <- rpcResult{err: fmt.Errorf("%s: %s (code=%d, data=%s)", pr.method, rpcErr.Message, rpcErr.Code, detail)}
		} else {
			pr.ch <- rpcResult{err: fmt.Errorf("%s: %s (code=%d)", pr.method, rpcErr.Message, rpcErr.Code)}
		}
	} else {
		// If this is a prompt response, extract usage and stop reason.
		if pr.method == "session/prompt" {
			c.extractPromptResult(raw["result"])
		}
		pr.ch <- rpcResult{result: raw["result"]}
	}
}

// handleAgentRequest replies to JSON-RPC requests the agent sends us
// (agent → client direction). The primary case is session/request_permission:
// the daemon is headless, so we auto-approve every action with
// "approve_for_session" so subsequent identical actions don't round-trip.
func (c *acpClient) handleAgentRequest(raw map[string]json.RawMessage) {
	var method string
	_ = json.Unmarshal(raw["method"], &method)

	rawID, ok := raw["id"]
	if !ok {
		return
	}

	var resp map[string]any
	switch method {
	case "session/request_permission":
		resp = map[string]any{
			"jsonrpc": "2.0",
			"id":      json.RawMessage(rawID),
			"result": map[string]any{
				"outcome": map[string]any{
					"outcome":  "selected",
					"optionId": "approve_for_session",
				},
			},
		}
		c.cfg.Logger.Debug("auto-approved agent permission request", "method", method)
	default:
		// Unknown agent→client method — reply with "method not found"
		// so the agent doesn't block waiting for us.
		resp = map[string]any{
			"jsonrpc": "2.0",
			"id":      json.RawMessage(rawID),
			"error": map[string]any{
				"code":    -32601,
				"message": "method not found: " + method,
			},
		}
		c.cfg.Logger.Debug("unhandled agent→client request", "method", method)
	}

	data, err := json.Marshal(resp)
	if err != nil {
		c.cfg.Logger.Warn("marshal agent-request response", "method", method, "error", err)
		return
	}
	data = append(data, '\n')
	if err := c.writeLine(data); err != nil {
		c.cfg.Logger.Warn("write agent-request response", "method", method, "error", err)
	}
}

// handleNotification processes ACP session update notifications.
func (c *acpClient) handleNotification(raw map[string]json.RawMessage) {
	var method string
	_ = json.Unmarshal(raw["method"], &method)

	if method != "session/update" && method != "session/notification" {
		return
	}

	var params struct {
		SessionID string          `json:"sessionId"`
		Update    json.RawMessage `json:"update"`
	}
	if p, ok := raw["params"]; ok {
		_ = json.Unmarshal(p, &params)
	}
	if len(params.Update) == 0 {
		return
	}

	updateType, updateData := normalizeACPUpdate(params.Update)
	if c.acceptNotification != nil && !c.acceptNotification(updateType) {
		return
	}

	switch updateType {
	case "agent_message_chunk":
		c.handleAgentMessage(updateData)
	case "agent_thought_chunk":
		c.handleAgentThought(updateData)
	case "tool_call":
		c.handleToolCallStart(updateData)
	case "tool_call_update":
		c.handleToolCallUpdate(updateData)
	case "usage_update":
		c.handleUsageUpdate(updateData)
	case "turn_end":
		c.extractPromptResult(updateData)
	}
}

// normalizeACPUpdate extracts the update type and data from an ACP
// session update payload. Handles multiple serialization forms:
// - {sessionUpdate: "agentMessageChunk", ...}
// - {type: "agent_message_chunk", ...}
// - {"agentMessageChunk": {...}} (externally tagged)
func normalizeACPUpdate(data json.RawMessage) (string, json.RawMessage) {
	var updateType struct {
		SessionUpdate string `json:"sessionUpdate"`
		Type          string `json:"type"`
	}
	_ = json.Unmarshal(data, &updateType)
	if updateType.SessionUpdate != "" {
		return normalizeACPUpdateType(updateType.SessionUpdate), data
	}
	if updateType.Type != "" {
		return normalizeACPUpdateType(updateType.Type), data
	}

	// Externally tagged object: {"agentMessageChunk": {"content": ...}}.
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper) == 1 {
		for k, v := range wrapper {
			return normalizeACPUpdateType(k), v
		}
	}

	return "", data
}

// normalizeACPUpdateType maps various casing/formatting conventions
// for ACP update types to canonical internal names.
func normalizeACPUpdateType(t string) string {
	key := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(t), "_", ""), "-", ""))
	switch key {
	case "agentmessagechunk":
		return "agent_message_chunk"
	case "agentthoughtchunk":
		return "agent_thought_chunk"
	case "toolcall":
		return "tool_call"
	case "toolcallupdate":
		return "tool_call_update"
	case "usageupdate":
		return "usage_update"
	case "turnend", "endturn":
		return "turn_end"
	default:
		return ""
	}
}

// ── ACP Notification Handlers ──

func (c *acpClient) handleAgentMessage(data json.RawMessage) {
	var msg struct {
		Content struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &msg); err != nil || msg.Content.Text == "" {
		return
	}
	if c.onMessage != nil {
		c.onMessage(Message{Type: MessageText, Content: msg.Content.Text})
	}
}

func (c *acpClient) handleAgentThought(data json.RawMessage) {
	var msg struct {
		Content struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &msg); err != nil || msg.Content.Text == "" {
		return
	}
	if c.onMessage != nil {
		c.onMessage(Message{Type: MessageThinking, Content: msg.Content.Text})
	}
}

func (c *acpClient) handleToolCallStart(data json.RawMessage) {
	var msg struct {
		ToolCallID string            `json:"toolCallId"`
		Name       string            `json:"name"`
		Title      string            `json:"title"`
		Kind       string            `json:"kind"`
		RawInput   map[string]any    `json:"rawInput"`
		Input      map[string]any    `json:"input"`
		Parameters map[string]any    `json:"parameters"`
		Content    []json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	toolName := msg.Title
	if toolName == "" {
		toolName = msg.Name
	}
	rawInput := msg.RawInput
	if rawInput == nil {
		rawInput = msg.Input
	}
	if rawInput == nil {
		rawInput = msg.Parameters
	}

	// If rawInput is available, emit MessageToolUse immediately.
	if rawInput != nil {
		c.trackTool(msg.ToolCallID, &acpPendingToolCall{
			toolName: toolName,
			input:    rawInput,
			emitted:  true,
		})
		if c.onMessage != nil {
			c.onMessage(Message{
				Type:   MessageToolUse,
				Tool:   toolName,
				CallID: msg.ToolCallID,
				Input:  rawInput,
			})
		}
		return
	}

	// Buffer the tool and defer emission until completion.
	c.trackTool(msg.ToolCallID, &acpPendingToolCall{
		toolName: toolName,
		argsText: extractACPToolCallText(msg.Content),
		emitted:  false,
	})
}

func (c *acpClient) handleToolCallUpdate(data json.RawMessage) {
	var msg struct {
		ToolCallID string            `json:"toolCallId"`
		Status     string            `json:"status"`
		Name       string            `json:"name"`
		Title      string            `json:"title"`
		Kind       string            `json:"kind"`
		RawInput   map[string]any    `json:"rawInput"`
		Input      map[string]any    `json:"input"`
		Parameters map[string]any    `json:"parameters"`
		RawOutput  string            `json:"rawOutput"`
		Output     string            `json:"output"`
		Content    []json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	rawInput := msg.RawInput
	if rawInput == nil {
		rawInput = msg.Input
	}
	if rawInput == nil {
		rawInput = msg.Parameters
	}
	title := msg.Title
	if title == "" {
		title = msg.Name
	}

	// Mid-stream: only buffer updates.
	if msg.Status != "completed" && msg.Status != "failed" {
		if pending := c.getPendingTool(msg.ToolCallID); pending != nil && !pending.emitted {
			if text := extractACPToolCallText(msg.Content); text != "" {
				pending.argsText = text
			}
		}
		return
	}

	// Completion: emit any deferred MessageToolUse first, then the result.
	pending := c.takePendingTool(msg.ToolCallID)
	c.emitDeferredToolUse(pending, msg.ToolCallID, title, rawInput)

	output := msg.RawOutput
	if output == "" {
		output = msg.Output
	}
	if output == "" {
		output = extractACPToolCallText(msg.Content)
	}
	if c.onMessage != nil {
		c.onMessage(Message{
			Type:   MessageToolResult,
			CallID: msg.ToolCallID,
			Output: output,
		})
	}
}

func (c *acpClient) handleUsageUpdate(data json.RawMessage) {
	var msg struct {
		Usage struct {
			InputTokens      int64 `json:"inputTokens"`
			OutputTokens     int64 `json:"outputTokens"`
			CachedReadTokens int64 `json:"cachedReadTokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	c.usageMu.Lock()
	// Usage updates from ACP are cumulative snapshots — take the latest.
	if msg.Usage.InputTokens > c.usage.InputTokens {
		c.usage.InputTokens = msg.Usage.InputTokens
	}
	if msg.Usage.OutputTokens > c.usage.OutputTokens {
		c.usage.OutputTokens = msg.Usage.OutputTokens
	}
	if msg.Usage.CachedReadTokens > c.usage.CacheReadTokens {
		c.usage.CacheReadTokens = msg.Usage.CachedReadTokens
	}
	c.usageMu.Unlock()
}

// extractPromptResult parses the session/prompt response for stop reason
// and usage, then invokes the onPromptDone callback.
func (c *acpClient) extractPromptResult(data json.RawMessage) {
	var resp struct {
		StopReason string `json:"stopReason"`
		Usage      *struct {
			InputTokens      int64 `json:"inputTokens"`
			OutputTokens     int64 `json:"outputTokens"`
			CachedReadTokens int64 `json:"cachedReadTokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return
	}

	pr := acpPromptResult{
		stopReason: resp.StopReason,
	}
	if resp.Usage != nil {
		pr.usage = TokenUsage{
			InputTokens:     resp.Usage.InputTokens,
			OutputTokens:    resp.Usage.OutputTokens,
			CacheReadTokens: resp.Usage.CachedReadTokens,
		}
	}

	if c.onPromptDone != nil {
		c.onPromptDone(pr)
	}
}

// ── Tool Call Tracking ──

func (c *acpClient) trackTool(callID string, p *acpPendingToolCall) {
	c.toolMu.Lock()
	defer c.toolMu.Unlock()
	if c.pendingTools == nil {
		c.pendingTools = make(map[string]*acpPendingToolCall)
	}
	c.pendingTools[callID] = p
}

func (c *acpClient) getPendingTool(callID string) *acpPendingToolCall {
	c.toolMu.Lock()
	defer c.toolMu.Unlock()
	if c.pendingTools == nil {
		return nil
	}
	return c.pendingTools[callID]
}

func (c *acpClient) takePendingTool(callID string) *acpPendingToolCall {
	c.toolMu.Lock()
	defer c.toolMu.Unlock()
	if c.pendingTools == nil {
		return nil
	}
	p := c.pendingTools[callID]
	delete(c.pendingTools, callID)
	return p
}

// emitDeferredToolUse emits a buffered MessageToolUse right before the
// matching MessageToolResult. Handles three cases:
//   - Already emitted on tool_call → skip
//   - Streamed args → parse accumulated JSON as Input
//   - No start frame → synthesize from update fields
func (c *acpClient) emitDeferredToolUse(
	p *acpPendingToolCall,
	callID, updateTitle string,
	updateRawInput map[string]any,
) {
	if p != nil && p.emitted {
		return
	}

	var toolName string
	var input map[string]any

	switch {
	case p != nil && p.input != nil:
		toolName = p.toolName
		input = p.input
	case p != nil:
		toolName = p.toolName
		input = parseACPToolArgsJSON(p.argsText)
	default:
		toolName = updateTitle
		input = updateRawInput
	}

	if c.onMessage == nil {
		return
	}
	c.onMessage(Message{
		Type:   MessageToolUse,
		Tool:   toolName,
		CallID: callID,
		Input:  input,
	})
}

// ── ACP Content Helpers ──

// parseACPToolArgsJSON turns accumulated args text into a structured map.
// ACP agents send args as JSON-encoded objects; on malformed input we
// preserve the raw text under a "text" key.
func parseACPToolArgsJSON(argsText string) map[string]any {
	argsText = strings.TrimSpace(argsText)
	if argsText == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(argsText), &m); err == nil {
		return m
	}
	return map[string]any{"text": argsText}
}

// extractACPToolCallText concatenates the rendered text of every ACP
// block in a tool_call / tool_call_update's content array.
func extractACPToolCallText(blocks []json.RawMessage) string {
	var b strings.Builder
	for _, raw := range blocks {
		var kind struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &kind); err != nil {
			continue
		}
		switch kind.Type {
		case "content":
			var outer struct {
				Content json.RawMessage `json:"content"`
			}
			if err := json.Unmarshal(raw, &outer); err != nil || len(outer.Content) == 0 {
				continue
			}
			var inner struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if err := json.Unmarshal(outer.Content, &inner); err != nil {
				continue
			}
			if inner.Type != "text" || inner.Text == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(inner.Text)
		}
	}
	return b.String()
}

// ── ACP Session Helpers ──

// extractACPSessionID pulls sessionId from a session/new or session/load
// response. Shared by all ACP backends.
func extractACPSessionID(result json.RawMessage) string {
	var r struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(result, &r); err != nil {
		return ""
	}
	return r.SessionID
}

// resolveResumedSessionID picks which session id to use after a
// session/resume round-trip. If the server returns a different id
// (e.g., after state reset), we use the server's canonical id.
func resolveResumedSessionID(requested string, response json.RawMessage) (string, bool) {
	got := extractACPSessionID(response)
	if got == "" {
		return requested, false
	}
	return got, got != requested
}

// ── ACP Tool Name Normalization ──

// NormalizeACPToolName converts ACP tool titles (e.g., "Read file",
// "Run command") into consistent snake_case identifiers (e.g., "read_file",
// "terminal"). This is used by Kiro, Hermes, and Kimi backends to produce
// uniform tool names for the UI.
//
// The function:
// 1. Strips any suffix after ":" (e.g., "Read file: /path" → "Read file")
// 2. Checks a known-title lookup table for common ACP tool names
// 3. Falls back to lowercasing and replacing spaces with underscores
//
// The result is idempotent: normalizing an already-normalized name returns
// the same value.
func NormalizeACPToolName(title string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		return ""
	}

	// Strip any suffix after ":" (e.g., "Read file: /path/to/file").
	if idx := strings.Index(t, ":"); idx > 0 {
		t = strings.TrimSpace(t[:idx])
	}

	lower := strings.ToLower(t)

	// Known title mappings — covers the common ACP tool names across
	// Kiro, Hermes, and Kimi.
	switch lower {
	case "read", "read file":
		return "read_file"
	case "write", "write file":
		return "write_file"
	case "edit", "patch":
		return "edit_file"
	case "shell", "bash", "terminal", "run command", "run shell command":
		return "terminal"
	case "grep", "search", "find", "search files":
		return "search_files"
	case "glob":
		return "glob"
	case "code":
		return "code"
	case "web search":
		return "web_search"
	case "fetch", "web fetch":
		return "web_fetch"
	case "todo", "todo write", "todo list", "todo_list":
		return "todo_write"
	case "list directory", "list dir":
		return "list_directory"
	case "execute code":
		return "execute_code"
	case "delegate", "delegate task":
		return "delegate_task"
	}

	// Fallback: lowercase and replace spaces with underscores.
	return strings.ReplaceAll(lower, " ", "_")
}

// ── Blocked Args Filtering ──

// blockedArgMode specifies whether a blocked arg takes a value or is standalone.
type blockedArgMode int

const (
	blockedWithValue  blockedArgMode = iota // flag takes a value (next arg or =value)
	blockedStandalone                       // flag is boolean, no value
)

// filterCustomArgs removes protocol-critical flags from user-configured
// custom args to prevent breaking daemon↔agent communication. Each backend
// defines its own blocked set (the flags it hardcodes).
func filterCustomArgs(args []string, blocked map[string]blockedArgMode, logger *slog.Logger) []string {
	if len(args) == 0 {
		return args
	}
	filtered := make([]string, 0, len(args))
	skip := false
	for _, arg := range args {
		if skip {
			skip = false
			continue
		}
		flag := arg
		hasInlineValue := false
		if idx := strings.Index(arg, "="); idx > 0 {
			flag = arg[:idx]
			hasInlineValue = true
		}
		mode, isBlocked := blocked[flag]
		if isBlocked {
			logger.Warn("custom_args: blocked protocol-critical flag, skipping", "flag", flag)
			if mode == blockedWithValue && !hasInlineValue {
				skip = true
			}
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered
}
