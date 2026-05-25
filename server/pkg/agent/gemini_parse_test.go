package agent

import (
	"encoding/json"
	"testing"

	"pgregory.net/rapid"
)

// Feature: agentic-output-architecture, Property 3: Gemini Message Mapping Preserves Content
//
// For any valid Gemini NDJSON event, parsing through the Gemini backend SHALL
// produce Message structs where:
// - message events with role "assistant" map to Type=text
// - tool_use events map to Type=tool_use with correct Tool/CallID/Input
// - tool_result events map to Type=tool_result with correct CallID/Output
// - error events map to Type=error
//
// **Validates: Requirements 3.3, 3.4, 3.5, 3.6**

func TestProperty_GeminiMessageMapping_AssistantText(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		content := rapid.StringMatching(`[a-zA-Z0-9 _.!?]{1,100}`).Draw(t, "content")

		evt := geminiStreamEvent{
			Type:    "message",
			Role:    "assistant",
			Content: content,
		}
		msg := mapGeminiEvent(evt)

		if msg == nil {
			t.Fatal("expected non-nil message for assistant message event")
		}
		if msg.Type != MessageText {
			t.Fatalf("expected Type=%q, got %q", MessageText, msg.Type)
		}
		if msg.Content != content {
			t.Fatalf("expected Content=%q, got %q", content, msg.Content)
		}
	})
}

func TestProperty_GeminiMessageMapping_ToolUse(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		toolName := rapid.StringMatching(`[a-z][a-z_]{0,19}`).Draw(t, "toolName")
		toolID := rapid.StringMatching(`[a-zA-Z0-9]{8,16}`).Draw(t, "toolID")

		// Generate random input parameters.
		numKeys := rapid.IntRange(1, 4).Draw(t, "numKeys")
		input := make(map[string]any, numKeys)
		for i := 0; i < numKeys; i++ {
			key := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "key")
			val := rapid.StringMatching(`[a-zA-Z0-9]{1,20}`).Draw(t, "val")
			input[key] = val
		}
		paramsJSON, _ := json.Marshal(input)

		evt := geminiStreamEvent{
			Type:       "tool_use",
			ToolName:   toolName,
			ToolID:     toolID,
			Parameters: json.RawMessage(paramsJSON),
		}
		msg := mapGeminiEvent(evt)

		if msg == nil {
			t.Fatal("expected non-nil message for tool_use event")
		}
		if msg.Type != MessageToolUse {
			t.Fatalf("expected Type=%q, got %q", MessageToolUse, msg.Type)
		}
		if msg.Tool != toolName {
			t.Fatalf("expected Tool=%q, got %q", toolName, msg.Tool)
		}
		if msg.CallID != toolID {
			t.Fatalf("expected CallID=%q, got %q", toolID, msg.CallID)
		}

		// Verify input round-trips correctly.
		gotJSON, _ := json.Marshal(msg.Input)
		expectedJSON, _ := json.Marshal(input)
		if string(gotJSON) != string(expectedJSON) {
			t.Fatalf("expected Input=%s, got %s", expectedJSON, gotJSON)
		}
	})
}

func TestProperty_GeminiMessageMapping_ToolResult(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		toolID := rapid.StringMatching(`[a-zA-Z0-9]{8,16}`).Draw(t, "toolID")
		output := rapid.StringMatching(`[a-zA-Z0-9 \n_.]{1,200}`).Draw(t, "output")

		evt := geminiStreamEvent{
			Type:   "tool_result",
			ToolID: toolID,
			Output: output,
		}
		msg := mapGeminiEvent(evt)

		if msg == nil {
			t.Fatal("expected non-nil message for tool_result event")
		}
		if msg.Type != MessageToolResult {
			t.Fatalf("expected Type=%q, got %q", MessageToolResult, msg.Type)
		}
		if msg.CallID != toolID {
			t.Fatalf("expected CallID=%q, got %q", toolID, msg.CallID)
		}
		if msg.Output != output {
			t.Fatalf("expected Output=%q, got %q", output, msg.Output)
		}
	})
}

func TestProperty_GeminiMessageMapping_Error(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		errMsg := rapid.StringMatching(`[a-zA-Z0-9 :_.]{1,100}`).Draw(t, "errorMessage")

		evt := geminiStreamEvent{
			Type:    "error",
			Message: errMsg,
		}
		msg := mapGeminiEvent(evt)

		if msg == nil {
			t.Fatal("expected non-nil message for error event")
		}
		if msg.Type != MessageError {
			t.Fatalf("expected Type=%q, got %q", MessageError, msg.Type)
		}
		if msg.Content != errMsg {
			t.Fatalf("expected Content=%q, got %q", errMsg, msg.Content)
		}
	})
}

func TestProperty_GeminiMessageMapping_NonAssistantMessageIgnored(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		// Messages with role != "assistant" should not produce a message.
		role := rapid.SampledFrom([]string{"user", "system", ""}).Draw(t, "role")
		content := rapid.StringMatching(`[a-zA-Z0-9]{1,50}`).Draw(t, "content")

		evt := geminiStreamEvent{
			Type:    "message",
			Role:    role,
			Content: content,
		}
		msg := mapGeminiEvent(evt)

		if msg != nil {
			t.Fatalf("expected nil message for role=%q, got %+v", role, msg)
		}
	})
}
