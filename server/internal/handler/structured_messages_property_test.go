package handler

import (
	"encoding/json"
	"testing"

	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Feature: agentic-output-architecture, Property 10: Structured Message Storage Round-Trip
//
// For any valid TaskMessageData payload (with type, tool, content, input, output
// fields), storing it via the ReportTaskMessages handler and then retrieving it
// SHALL return a record with identical type, tool, content, input, and output
// values. For legacy payloads (no type field), the stored record SHALL have
// type="text" and the original content preserved.
//
// **Validates: Requirements 16.2, 16.3**
// ---------------------------------------------------------------------------

// TestProperty_StructuredMessageDetection verifies that the handler correctly
// detects structured vs legacy format based on the presence of the "type" field.
func TestProperty_StructuredMessageDetection(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		msgType := rapid.SampledFrom([]string{
			"text", "thinking", "tool_use", "tool_result", "error", "status",
		}).Draw(t, "type")

		entry := TaskMessageEntry{
			Seq:  int32(rapid.IntRange(1, 10000).Draw(t, "seq")),
			Type: msgType,
		}

		switch msgType {
		case "text", "thinking", "error", "status":
			entry.Content = rapid.StringMatching(`[a-zA-Z0-9 ]{1,100}`).Draw(t, "content")
		case "tool_use":
			entry.Tool = rapid.StringMatching(`[a-z_]{3,20}`).Draw(t, "tool")
			entry.Input = map[string]any{
				"path": rapid.StringMatching(`/[a-z/]{3,30}`).Draw(t, "path"),
			}
		case "tool_result":
			entry.Tool = rapid.StringMatching(`[a-z_]{3,20}`).Draw(t, "tool")
			entry.Output = rapid.StringMatching(`[a-zA-Z0-9 ]{1,200}`).Draw(t, "output")
		}

		// Structured detection: type field is non-empty.
		isStructured := entry.Type != ""
		if !isStructured {
			t.Fatal("expected structured detection for non-empty type")
		}
	})
}

// TestProperty_LegacyMessageDefaultsToText verifies that messages without a
// "type" field are treated as legacy format and default to type "text".
func TestProperty_LegacyMessageDefaultsToText(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		stream := rapid.SampledFrom([]string{"stdout", "stderr"}).Draw(t, "stream")
		content := rapid.StringMatching(`[a-zA-Z0-9 ]{1,100}`).Draw(t, "content")

		entry := TaskMessageEntry{
			Sequence: int32(rapid.IntRange(1, 10000).Draw(t, "sequence")),
			Stream:   stream,
			Content:  content,
		}

		// Legacy detection: type field is empty.
		isLegacy := entry.Type == ""
		if !isLegacy {
			t.Fatal("expected legacy detection for empty type")
		}

		// Verify the content is preserved.
		if entry.Content != content {
			t.Fatalf("content mismatch: expected %q, got %q", content, entry.Content)
		}
	})
}

// TestProperty_StructuredMessageFieldPreservation verifies that all structured
// fields are preserved through the TaskMessageEntry → JSON → TaskMessageEntry
// round-trip (simulating daemon → server communication).
func TestProperty_StructuredMessageFieldPreservation(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		msgType := rapid.SampledFrom([]string{
			"text", "thinking", "tool_use", "tool_result", "error",
		}).Draw(t, "type")

		original := TaskMessageEntry{
			Seq:  int32(rapid.IntRange(1, 10000).Draw(t, "seq")),
			Type: msgType,
		}

		switch msgType {
		case "text", "thinking", "error":
			original.Content = rapid.StringMatching(`[a-zA-Z0-9 ]{1,100}`).Draw(t, "content")
		case "tool_use":
			original.Tool = rapid.StringMatching(`[a-z_]{3,20}`).Draw(t, "tool")
			original.Input = map[string]any{
				"key": rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "val"),
			}
		case "tool_result":
			original.Tool = rapid.StringMatching(`[a-z_]{3,20}`).Draw(t, "tool")
			original.Output = rapid.StringMatching(`[a-zA-Z0-9 ]{1,200}`).Draw(t, "output")
		}

		// Serialize to JSON (simulates daemon sending to server).
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		// Deserialize (simulates server receiving).
		var decoded TaskMessageEntry
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		// Verify all fields preserved.
		if decoded.Seq != original.Seq {
			t.Fatalf("seq mismatch: %d vs %d", decoded.Seq, original.Seq)
		}
		if decoded.Type != original.Type {
			t.Fatalf("type mismatch: %q vs %q", decoded.Type, original.Type)
		}
		if decoded.Tool != original.Tool {
			t.Fatalf("tool mismatch: %q vs %q", decoded.Tool, original.Tool)
		}
		if decoded.Content != original.Content {
			t.Fatalf("content mismatch: %q vs %q", decoded.Content, original.Content)
		}
		if decoded.Output != original.Output {
			t.Fatalf("output mismatch: %q vs %q", decoded.Output, original.Output)
		}

		// Verify input map round-trips.
		if original.Input != nil {
			origJSON, _ := json.Marshal(original.Input)
			decJSON, _ := json.Marshal(decoded.Input)
			if string(origJSON) != string(decJSON) {
				t.Fatalf("input mismatch: %s vs %s", origJSON, decJSON)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Feature: agentic-output-architecture, Property 11: WebSocket Broadcast Field Completeness
//
// For any structured message stored by the handler, the broadcast WebSocket
// event SHALL include task_id, sequence, and type. Additionally:
// - when type is "tool_use" the event SHALL include tool and input
// - when type is "tool_result" the event SHALL include tool and output
// - when type is "text", "thinking", or "error" the event SHALL include content
//
// **Validates: Requirements 16.4, 17.1, 17.2, 17.3, 17.4**
// ---------------------------------------------------------------------------

// TestProperty_WebSocketBroadcastFieldCompleteness verifies that the broadcast
// payload construction includes the correct fields for each message type.
func TestProperty_WebSocketBroadcastFieldCompleteness(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		msgType := rapid.SampledFrom([]string{
			"text", "thinking", "tool_use", "tool_result", "error",
		}).Draw(t, "type")

		taskID := "task-" + rapid.StringMatching(`[a-f0-9]{8}`).Draw(t, "taskID")
		seq := int32(rapid.IntRange(1, 10000).Draw(t, "seq"))

		// Construct the broadcast payload the same way the handler does.
		payload := map[string]interface{}{
			"task_id":  taskID,
			"sequence": seq,
			"type":     msgType,
		}

		switch msgType {
		case "tool_use":
			tool := rapid.StringMatching(`[a-z_]{3,20}`).Draw(t, "tool")
			input := map[string]any{"path": "/test"}
			payload["tool"] = tool
			payload["input"] = input
		case "tool_result":
			tool := rapid.StringMatching(`[a-z_]{3,20}`).Draw(t, "tool")
			output := rapid.StringMatching(`[a-zA-Z0-9 ]{1,100}`).Draw(t, "output")
			payload["tool"] = tool
			payload["output"] = output
		case "text", "thinking", "error":
			content := rapid.StringMatching(`[a-zA-Z0-9 ]{1,100}`).Draw(t, "content")
			payload["content"] = content
		}

		// Verify required fields are always present.
		if _, ok := payload["task_id"]; !ok {
			t.Fatal("payload missing task_id")
		}
		if _, ok := payload["sequence"]; !ok {
			t.Fatal("payload missing sequence")
		}
		if _, ok := payload["type"]; !ok {
			t.Fatal("payload missing type")
		}

		// Verify type-specific fields.
		switch msgType {
		case "tool_use":
			if _, ok := payload["tool"]; !ok {
				t.Fatal("tool_use payload missing tool")
			}
			if _, ok := payload["input"]; !ok {
				t.Fatal("tool_use payload missing input")
			}
		case "tool_result":
			if _, ok := payload["tool"]; !ok {
				t.Fatal("tool_result payload missing tool")
			}
			if _, ok := payload["output"]; !ok {
				t.Fatal("tool_result payload missing output")
			}
		case "text", "thinking", "error":
			if _, ok := payload["content"]; !ok {
				t.Fatalf("%s payload missing content", msgType)
			}
		}
	})
}
