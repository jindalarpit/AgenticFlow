package daemon

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agenticflow/agenticflow/pkg/agent"
	"pgregory.net/rapid"
)

// mockReporter captures all reported batches for testing.
type mockReporter struct {
	mu      sync.Mutex
	batches [][]TaskMessageData
}

func (m *mockReporter) ReportTaskMessages(taskID string, messages []TaskMessageData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Copy the slice to avoid aliasing.
	cp := make([]TaskMessageData, len(messages))
	copy(cp, messages)
	m.batches = append(m.batches, cp)
	return nil
}

func (m *mockReporter) allMessages() []TaskMessageData {
	m.mu.Lock()
	defer m.mu.Unlock()
	var all []TaskMessageData
	for _, b := range m.batches {
		all = append(all, b...)
	}
	return all
}

// newTestBatchReporter creates a BatchReporter with a very short flush interval
// for testing, using a mock reporter.
func newTestBatchReporter(reporter *mockReporter) *BatchReporter {
	return NewBatchReporter(reporter, "test-task", 10*time.Millisecond, nil)
}

// Feature: agentic-output-architecture, Property 5: Batch Reporter Text Consolidation
//
// For any sequence of N consecutive text Messages followed by a flush,
// the resulting batch SHALL contain exactly one TaskMessageData with type="text"
// whose Content equals the concatenation of all N messages' Content fields.
// Same for thinking Messages independently.
//
// **Validates: Requirements 13.1, 13.3**

func TestProperty_BatchReporter_TextConsolidation(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		reporter := &mockReporter{}
		br := NewBatchReporter(reporter, "test-task", time.Hour, nil) // Long interval, manual flush

		n := rapid.IntRange(1, 20).Draw(t, "numTextMessages")
		var expected strings.Builder
		for i := 0; i < n; i++ {
			text := rapid.StringMatching(`[a-zA-Z0-9 ]{1,50}`).Draw(t, "text")
			br.Feed(agent.Message{Type: agent.MessageText, Content: text})
			expected.WriteString(text)
		}

		br.Close()

		all := reporter.allMessages()
		// Find text messages.
		var textMsgs []TaskMessageData
		for _, m := range all {
			if m.Type == "text" {
				textMsgs = append(textMsgs, m)
			}
		}

		if len(textMsgs) != 1 {
			t.Fatalf("expected exactly 1 text message, got %d", len(textMsgs))
		}
		if textMsgs[0].Content != expected.String() {
			t.Fatalf("expected consolidated content %q, got %q", expected.String(), textMsgs[0].Content)
		}
	})
}

func TestProperty_BatchReporter_ThinkingConsolidation(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		reporter := &mockReporter{}
		br := NewBatchReporter(reporter, "test-task", time.Hour, nil)

		n := rapid.IntRange(1, 20).Draw(t, "numThinkingMessages")
		var expected strings.Builder
		for i := 0; i < n; i++ {
			text := rapid.StringMatching(`[a-zA-Z0-9 ]{1,50}`).Draw(t, "thinking")
			br.Feed(agent.Message{Type: agent.MessageThinking, Content: text})
			expected.WriteString(text)
		}

		br.Close()

		all := reporter.allMessages()
		var thinkingMsgs []TaskMessageData
		for _, m := range all {
			if m.Type == "thinking" {
				thinkingMsgs = append(thinkingMsgs, m)
			}
		}

		if len(thinkingMsgs) != 1 {
			t.Fatalf("expected exactly 1 thinking message, got %d", len(thinkingMsgs))
		}
		if thinkingMsgs[0].Content != expected.String() {
			t.Fatalf("expected consolidated content %q, got %q", expected.String(), thinkingMsgs[0].Content)
		}
	})
}

// Feature: agentic-output-architecture, Property 6: Batch Reporter Sequence Monotonicity
//
// For any sequence of Messages fed to the BatchReporter, the sequence numbers
// assigned to the resulting TaskMessageData entries SHALL be strictly
// monotonically increasing.
//
// **Validates: Requirements 13.4**

func TestProperty_BatchReporter_SequenceMonotonicity(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		reporter := &mockReporter{}
		br := NewBatchReporter(reporter, "test-task", time.Hour, nil)

		n := rapid.IntRange(2, 30).Draw(t, "numMessages")
		for i := 0; i < n; i++ {
			msgType := rapid.SampledFrom([]agent.MessageType{
				agent.MessageText,
				agent.MessageThinking,
				agent.MessageToolUse,
				agent.MessageToolResult,
				agent.MessageError,
			}).Draw(t, "msgType")

			msg := agent.Message{Type: msgType}
			switch msgType {
			case agent.MessageText, agent.MessageThinking, agent.MessageError:
				msg.Content = rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "content")
			case agent.MessageToolUse:
				msg.Tool = "test_tool"
				msg.Input = map[string]any{"key": "val"}
			case agent.MessageToolResult:
				msg.Tool = "test_tool"
				msg.Output = "result"
			}
			br.Feed(msg)
		}

		br.Close()

		all := reporter.allMessages()
		if len(all) == 0 {
			t.Fatal("expected at least one message after feeding")
		}

		// Verify strict monotonicity.
		for i := 1; i < len(all); i++ {
			if all[i].Seq <= all[i-1].Seq {
				t.Fatalf("sequence not monotonically increasing: seq[%d]=%d, seq[%d]=%d",
					i-1, all[i-1].Seq, i, all[i].Seq)
			}
		}
	})
}

// Feature: agentic-output-architecture, Property 7: Batch Reporter Tool Event Immediacy
//
// For any tool_use or tool_result Message fed to the BatchReporter, that message
// SHALL appear in the current batch immediately without merging.
//
// **Validates: Requirements 13.2**

func TestProperty_BatchReporter_ToolEventImmediacy(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		reporter := &mockReporter{}
		br := NewBatchReporter(reporter, "test-task", time.Hour, nil)

		// Feed some text first to ensure tool events aren't merged with text.
		br.Feed(agent.Message{Type: agent.MessageText, Content: "prefix"})

		toolName := rapid.StringMatching(`[a-z_]{3,15}`).Draw(t, "toolName")
		isToolUse := rapid.Bool().Draw(t, "isToolUse")

		if isToolUse {
			br.Feed(agent.Message{
				Type:  agent.MessageToolUse,
				Tool:  toolName,
				Input: map[string]any{"path": "/test"},
			})
		} else {
			br.Feed(agent.Message{
				Type:   agent.MessageToolResult,
				Tool:   toolName,
				Output: "output data",
			})
		}

		// Feed more text after.
		br.Feed(agent.Message{Type: agent.MessageText, Content: "suffix"})

		br.Close()

		all := reporter.allMessages()

		// Find the tool event.
		var toolEvents []TaskMessageData
		for _, m := range all {
			if m.Type == "tool_use" || m.Type == "tool_result" {
				toolEvents = append(toolEvents, m)
			}
		}

		if len(toolEvents) != 1 {
			t.Fatalf("expected exactly 1 tool event, got %d", len(toolEvents))
		}

		// The tool event should NOT be merged with text.
		if toolEvents[0].Content != "" {
			t.Fatalf("tool event should not have content field set, got %q", toolEvents[0].Content)
		}
	})
}

// Feature: agentic-output-architecture, Property 8: Batch Reporter Final Flush Completeness
//
// For any sequence of Messages fed to the BatchReporter, after calling Close(),
// the union of all reported batches SHALL contain every Message's content.
//
// **Validates: Requirements 13.5**

func TestProperty_BatchReporter_FinalFlushCompleteness(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		reporter := &mockReporter{}
		br := NewBatchReporter(reporter, "test-task", time.Hour, nil)

		n := rapid.IntRange(1, 20).Draw(t, "numMessages")
		var allText strings.Builder
		var allThinking strings.Builder
		toolUseCount := 0
		toolResultCount := 0

		for i := 0; i < n; i++ {
			msgType := rapid.SampledFrom([]agent.MessageType{
				agent.MessageText,
				agent.MessageThinking,
				agent.MessageToolUse,
				agent.MessageToolResult,
			}).Draw(t, "msgType")

			switch msgType {
			case agent.MessageText:
				text := rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "text")
				br.Feed(agent.Message{Type: agent.MessageText, Content: text})
				allText.WriteString(text)
			case agent.MessageThinking:
				text := rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "thinking")
				br.Feed(agent.Message{Type: agent.MessageThinking, Content: text})
				allThinking.WriteString(text)
			case agent.MessageToolUse:
				br.Feed(agent.Message{Type: agent.MessageToolUse, Tool: "t", Input: map[string]any{"k": "v"}})
				toolUseCount++
			case agent.MessageToolResult:
				br.Feed(agent.Message{Type: agent.MessageToolResult, Tool: "t", Output: "out"})
				toolResultCount++
			}
		}

		br.Close()

		all := reporter.allMessages()

		// Verify all text content is present.
		var gotText strings.Builder
		var gotThinking strings.Builder
		gotToolUse := 0
		gotToolResult := 0
		for _, m := range all {
			switch m.Type {
			case "text":
				gotText.WriteString(m.Content)
			case "thinking":
				gotThinking.WriteString(m.Content)
			case "tool_use":
				gotToolUse++
			case "tool_result":
				gotToolResult++
			}
		}

		if gotText.String() != allText.String() {
			t.Fatalf("text content mismatch: expected %q, got %q", allText.String(), gotText.String())
		}
		if gotThinking.String() != allThinking.String() {
			t.Fatalf("thinking content mismatch: expected %q, got %q", allThinking.String(), gotThinking.String())
		}
		if gotToolUse != toolUseCount {
			t.Fatalf("tool_use count mismatch: expected %d, got %d", toolUseCount, gotToolUse)
		}
		if gotToolResult != toolResultCount {
			t.Fatalf("tool_result count mismatch: expected %d, got %d", toolResultCount, gotToolResult)
		}
	})
}

// Feature: agentic-output-architecture, Property 9: Tool Result Truncation
//
// For any tool_result Message whose Output length exceeds 8192 bytes,
// the BatchReporter SHALL produce a TaskMessageData whose Output field is
// exactly 8192 bytes long. For outputs ≤ 8192 bytes, the Output SHALL be
// preserved unchanged.
//
// **Validates: Requirements 13.8**

func TestProperty_BatchReporter_ToolResultTruncation(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		reporter := &mockReporter{}
		br := NewBatchReporter(reporter, "test-task", time.Hour, nil)

		// Generate output of random length (some above, some below threshold).
		length := rapid.IntRange(1, 20000).Draw(t, "outputLength")
		output := strings.Repeat("x", length)

		br.Feed(agent.Message{
			Type:   agent.MessageToolResult,
			Tool:   "test_tool",
			Output: output,
		})

		br.Close()

		all := reporter.allMessages()
		var toolResults []TaskMessageData
		for _, m := range all {
			if m.Type == "tool_result" {
				toolResults = append(toolResults, m)
			}
		}

		if len(toolResults) != 1 {
			t.Fatalf("expected 1 tool_result, got %d", len(toolResults))
		}

		got := toolResults[0].Output
		if length > maxToolResultOutput {
			if len(got) != maxToolResultOutput {
				t.Fatalf("expected truncated output of %d bytes, got %d", maxToolResultOutput, len(got))
			}
		} else {
			if got != output {
				t.Fatalf("expected output preserved unchanged (len=%d), got len=%d", length, len(got))
			}
		}
	})
}
