package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// Feature: agentic-output-architecture, Property 2: Claude Message Mapping Preserves Content
//
// For any valid Claude SDK JSON line containing content blocks (text, thinking,
// tool_use, or tool_result), parsing that line through the Claude backend's
// message handler SHALL produce Message structs where:
// - text blocks map to Type=text with matching Content
// - thinking blocks map to Type=thinking with matching Content
// - tool_use blocks map to Type=tool_use with Tool=block.name, CallID=block.id, Input=block.input
// - tool_result blocks map to Type=tool_result with CallID=block.tool_use_id, Output=block.content
//
// **Validates: Requirements 2.3, 2.4, 2.5, 2.6**

// genNonEmptyString generates a non-empty alphanumeric string suitable for identifiers.
func genNonEmptyString(t *rapid.T, label string) string {
	return rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_]{0,29}`).Draw(t, label)
}

// genTextContent generates a non-empty text string (may contain spaces, punctuation).
func genTextContent(t *rapid.T, label string) string {
	return rapid.StringMatching(`[a-zA-Z0-9 _.!?,:;-]{1,100}`).Draw(t, label)
}

// genToolInput generates a random map[string]any for tool input parameters.
func genToolInput(t *rapid.T) map[string]any {
	numKeys := rapid.IntRange(1, 5).Draw(t, "numInputKeys")
	input := make(map[string]any, numKeys)
	for i := 0; i < numKeys; i++ {
		key := genNonEmptyString(t, "inputKey")
		// Generate simple string values to keep JSON round-tripping predictable.
		val := genTextContent(t, "inputVal")
		input[key] = val
	}
	return input
}

// collectAssistantMessages runs handleAssistant and collects emitted Messages.
func collectAssistantMessages(b *claudeBackend, msg claudeSDKMessage) []Message {
	ch := make(chan Message, 256)
	var output strings.Builder
	usage := make(map[string]TokenUsage)
	b.handleAssistant(msg, ch, &output, usage)
	close(ch)

	var msgs []Message
	for m := range ch {
		msgs = append(msgs, m)
	}
	return msgs
}

// collectUserMessages runs handleUser and collects emitted Messages.
func collectUserMessages(b *claudeBackend, msg claudeSDKMessage) []Message {
	ch := make(chan Message, 256)
	b.handleUser(msg, ch)
	close(ch)

	var msgs []Message
	for m := range ch {
		msgs = append(msgs, m)
	}
	return msgs
}

// buildAssistantSDKMessage constructs a claudeSDKMessage with type "assistant"
// and the given content blocks.
func buildAssistantSDKMessage(blocks []claudeContentBlock) claudeSDKMessage {
	content := claudeMessageContent{
		Role:    "assistant",
		Model:   "claude-sonnet-4-20250514",
		Content: blocks,
	}
	raw, _ := json.Marshal(content)
	return claudeSDKMessage{
		Type:    "assistant",
		Message: raw,
	}
}

// buildUserSDKMessage constructs a claudeSDKMessage with type "user"
// and the given content blocks.
func buildUserSDKMessage(blocks []claudeContentBlock) claudeSDKMessage {
	content := claudeMessageContent{
		Role:    "user",
		Content: blocks,
	}
	raw, _ := json.Marshal(content)
	return claudeSDKMessage{
		Type:    "user",
		Message: raw,
	}
}

func TestProperty_ClaudeMessageMapping_TextBlocks(t *testing.T) {
	t.Parallel()
	b := &claudeBackend{}

	rapid.Check(t, func(t *rapid.T) {
		text := genTextContent(t, "textContent")

		blocks := []claudeContentBlock{
			{Type: "text", Text: text},
		}
		msg := buildAssistantSDKMessage(blocks)
		msgs := collectAssistantMessages(b, msg)

		if len(msgs) != 1 {
			t.Fatalf("expected 1 message, got %d", len(msgs))
		}
		if msgs[0].Type != MessageText {
			t.Fatalf("expected Type=%q, got %q", MessageText, msgs[0].Type)
		}
		if msgs[0].Content != text {
			t.Fatalf("expected Content=%q, got %q", text, msgs[0].Content)
		}
	})
}

func TestProperty_ClaudeMessageMapping_ThinkingBlocks(t *testing.T) {
	t.Parallel()
	b := &claudeBackend{}

	rapid.Check(t, func(t *rapid.T) {
		text := genTextContent(t, "thinkingContent")

		blocks := []claudeContentBlock{
			{Type: "thinking", Text: text},
		}
		msg := buildAssistantSDKMessage(blocks)
		msgs := collectAssistantMessages(b, msg)

		if len(msgs) != 1 {
			t.Fatalf("expected 1 message, got %d", len(msgs))
		}
		if msgs[0].Type != MessageThinking {
			t.Fatalf("expected Type=%q, got %q", MessageThinking, msgs[0].Type)
		}
		if msgs[0].Content != text {
			t.Fatalf("expected Content=%q, got %q", text, msgs[0].Content)
		}
	})
}

func TestProperty_ClaudeMessageMapping_ToolUseBlocks(t *testing.T) {
	t.Parallel()
	b := &claudeBackend{}

	rapid.Check(t, func(t *rapid.T) {
		toolName := genNonEmptyString(t, "toolName")
		toolID := genNonEmptyString(t, "toolID")
		input := genToolInput(t)

		inputJSON, err := json.Marshal(input)
		if err != nil {
			t.Fatalf("failed to marshal input: %v", err)
		}

		blocks := []claudeContentBlock{
			{
				Type:  "tool_use",
				Name:  toolName,
				ID:    toolID,
				Input: json.RawMessage(inputJSON),
			},
		}
		msg := buildAssistantSDKMessage(blocks)
		msgs := collectAssistantMessages(b, msg)

		if len(msgs) != 1 {
			t.Fatalf("expected 1 message, got %d", len(msgs))
		}
		m := msgs[0]
		if m.Type != MessageToolUse {
			t.Fatalf("expected Type=%q, got %q", MessageToolUse, m.Type)
		}
		if m.Tool != toolName {
			t.Fatalf("expected Tool=%q, got %q", toolName, m.Tool)
		}
		if m.CallID != toolID {
			t.Fatalf("expected CallID=%q, got %q", toolID, m.CallID)
		}

		// Verify input fields match. We compare by re-marshaling both sides.
		gotInputJSON, _ := json.Marshal(m.Input)
		expectedInputJSON, _ := json.Marshal(input)
		if string(gotInputJSON) != string(expectedInputJSON) {
			t.Fatalf("expected Input=%s, got %s", expectedInputJSON, gotInputJSON)
		}
	})
}

func TestProperty_ClaudeMessageMapping_ToolResultBlocks(t *testing.T) {
	t.Parallel()
	b := &claudeBackend{}

	rapid.Check(t, func(t *rapid.T) {
		toolUseID := genNonEmptyString(t, "toolUseID")
		resultContent := genTextContent(t, "resultContent")

		// tool_result blocks use the Content field (json.RawMessage) for output.
		// In the real Claude protocol, content is a JSON-encoded value (e.g., a string).
		// The handleUser method does `string(block.Content)` so we store the raw JSON bytes.
		contentRaw, _ := json.Marshal(resultContent)

		blocks := []claudeContentBlock{
			{
				Type:      "tool_result",
				ToolUseID: toolUseID,
				Content:   json.RawMessage(contentRaw),
			},
		}
		msg := buildUserSDKMessage(blocks)
		msgs := collectUserMessages(b, msg)

		if len(msgs) != 1 {
			t.Fatalf("expected 1 message, got %d", len(msgs))
		}
		m := msgs[0]
		if m.Type != MessageToolResult {
			t.Fatalf("expected Type=%q, got %q", MessageToolResult, m.Type)
		}
		if m.CallID != toolUseID {
			t.Fatalf("expected CallID=%q, got %q", toolUseID, m.CallID)
		}
		// The output is string(block.Content) which is the raw JSON bytes as a string.
		expectedOutput := string(contentRaw)
		if m.Output != expectedOutput {
			t.Fatalf("expected Output=%q, got %q", expectedOutput, m.Output)
		}
	})
}

func TestProperty_ClaudeMessageMapping_MixedBlocks(t *testing.T) {
	t.Parallel()
	b := &claudeBackend{}

	rapid.Check(t, func(t *rapid.T) {
		// Generate a mix of text, thinking, and tool_use blocks in a single assistant message.
		textContent := genTextContent(t, "textContent")
		thinkingContent := genTextContent(t, "thinkingContent")
		toolName := genNonEmptyString(t, "toolName")
		toolID := genNonEmptyString(t, "toolID")
		input := genToolInput(t)
		inputJSON, _ := json.Marshal(input)

		blocks := []claudeContentBlock{
			{Type: "text", Text: textContent},
			{Type: "thinking", Text: thinkingContent},
			{Type: "tool_use", Name: toolName, ID: toolID, Input: json.RawMessage(inputJSON)},
		}
		msg := buildAssistantSDKMessage(blocks)
		msgs := collectAssistantMessages(b, msg)

		if len(msgs) != 3 {
			t.Fatalf("expected 3 messages, got %d", len(msgs))
		}

		// Verify text block
		if msgs[0].Type != MessageText {
			t.Fatalf("msg[0]: expected Type=%q, got %q", MessageText, msgs[0].Type)
		}
		if msgs[0].Content != textContent {
			t.Fatalf("msg[0]: expected Content=%q, got %q", textContent, msgs[0].Content)
		}

		// Verify thinking block
		if msgs[1].Type != MessageThinking {
			t.Fatalf("msg[1]: expected Type=%q, got %q", MessageThinking, msgs[1].Type)
		}
		if msgs[1].Content != thinkingContent {
			t.Fatalf("msg[1]: expected Content=%q, got %q", thinkingContent, msgs[1].Content)
		}

		// Verify tool_use block
		if msgs[2].Type != MessageToolUse {
			t.Fatalf("msg[2]: expected Type=%q, got %q", MessageToolUse, msgs[2].Type)
		}
		if msgs[2].Tool != toolName {
			t.Fatalf("msg[2]: expected Tool=%q, got %q", toolName, msgs[2].Tool)
		}
		if msgs[2].CallID != toolID {
			t.Fatalf("msg[2]: expected CallID=%q, got %q", toolID, msgs[2].CallID)
		}
	})
}
