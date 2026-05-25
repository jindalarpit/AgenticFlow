package daemon

import (
	"testing"

	"github.com/agenticflow/agenticflow/pkg/agent"
)

// ── isStructuredBackendSupported tests ──

func TestIsStructuredBackendSupported_SupportedTypes(t *testing.T) {
	t.Parallel()

	for _, agentType := range agent.SupportedTypes {
		if !isStructuredBackendSupported(agentType) {
			t.Errorf("expected %q to be supported", agentType)
		}
	}
}

func TestIsStructuredBackendSupported_UnsupportedTypes(t *testing.T) {
	t.Parallel()

	unsupported := []string{"unknown", "gpt4", "llama", "", "CLAUDE"}
	for _, agentType := range unsupported {
		if isStructuredBackendSupported(agentType) {
			t.Errorf("expected %q to NOT be supported", agentType)
		}
	}
}

// ── formatStructuredContent tests ──

func TestFormatStructuredContent_TextType(t *testing.T) {
	t.Parallel()

	msg := TaskMessageData{Type: "text", Content: "hello world"}
	got := formatStructuredContent(msg)
	if got != "hello world" {
		t.Fatalf("expected 'hello world', got %q", got)
	}
}

func TestFormatStructuredContent_ThinkingType(t *testing.T) {
	t.Parallel()

	msg := TaskMessageData{Type: "thinking", Content: "let me think..."}
	got := formatStructuredContent(msg)
	if got != "let me think..." {
		t.Fatalf("expected 'let me think...', got %q", got)
	}
}

func TestFormatStructuredContent_ErrorType(t *testing.T) {
	t.Parallel()

	msg := TaskMessageData{Type: "error", Content: "something failed"}
	got := formatStructuredContent(msg)
	if got != "something failed" {
		t.Fatalf("expected 'something failed', got %q", got)
	}
}

func TestFormatStructuredContent_ToolUseType(t *testing.T) {
	t.Parallel()

	msg := TaskMessageData{Type: "tool_use", Tool: "read_file", Input: map[string]any{"path": "/test"}}
	got := formatStructuredContent(msg)
	if got != "[tool_use] read_file" {
		t.Fatalf("expected '[tool_use] read_file', got %q", got)
	}
}

func TestFormatStructuredContent_ToolResultType(t *testing.T) {
	t.Parallel()

	msg := TaskMessageData{Type: "tool_result", Tool: "read_file", Output: "file contents here"}
	got := formatStructuredContent(msg)
	expected := "[tool_result] read_file: file contents here"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestFormatStructuredContent_ToolResultTruncation(t *testing.T) {
	t.Parallel()

	// Output longer than 200 chars should be truncated with "..."
	longOutput := ""
	for i := 0; i < 250; i++ {
		longOutput += "x"
	}
	msg := TaskMessageData{Type: "tool_result", Tool: "exec", Output: longOutput}
	got := formatStructuredContent(msg)
	// "[tool_result] exec: " (20 chars) + 200 chars + "..." (3 chars) = 223 max
	if len(got) > 223 {
		t.Fatalf("expected truncated output (max 223), got length %d", len(got))
	}
	if got[len(got)-3:] != "..." {
		t.Fatalf("expected '...' suffix, got %q", got[len(got)-10:])
	}
}

// ── realHTTPMessageReporter tests ──

func TestRealHTTPMessageReporter_NilClient(t *testing.T) {
	t.Parallel()

	reporter := &realHTTPMessageReporter{client: nil}
	err := reporter.ReportTaskMessages("task-1", []TaskMessageData{
		{Seq: 1, Type: "text", Content: "hello"},
	})
	if err != nil {
		t.Fatalf("expected nil error for nil client, got %v", err)
	}
}
