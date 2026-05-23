package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInjectBrief_EmptyBrief_SkipsInjection(t *testing.T) {
	t.Parallel()

	opts := &ExecOptions{}
	prompt := "do something"

	result, err := InjectBrief("claude", "", prompt, "/tmp", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != prompt {
		t.Errorf("expected prompt %q, got %q", prompt, result)
	}
	if opts.SystemPrompt != "" {
		t.Errorf("expected empty SystemPrompt, got %q", opts.SystemPrompt)
	}
}

func TestInjectBrief_Claude_SetsSystemPrompt(t *testing.T) {
	t.Parallel()

	opts := &ExecOptions{}
	brief := "You are a helpful agent."
	prompt := "fix the bug"

	result, err := InjectBrief("claude", brief, prompt, "/tmp", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != prompt {
		t.Errorf("expected prompt unchanged %q, got %q", prompt, result)
	}
	if opts.SystemPrompt != brief {
		t.Errorf("expected SystemPrompt %q, got %q", brief, opts.SystemPrompt)
	}
}

func TestInjectBrief_Pi_SetsSystemPrompt(t *testing.T) {
	t.Parallel()

	opts := &ExecOptions{}
	brief := "You are Pi agent."
	prompt := "write tests"

	result, err := InjectBrief("pi", brief, prompt, "/tmp", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != prompt {
		t.Errorf("expected prompt unchanged %q, got %q", prompt, result)
	}
	if opts.SystemPrompt != brief {
		t.Errorf("expected SystemPrompt %q, got %q", brief, opts.SystemPrompt)
	}
}

func TestInjectBrief_Opencode_SetsSystemPrompt(t *testing.T) {
	t.Parallel()

	opts := &ExecOptions{}
	brief := "OpenCode instructions"
	prompt := "refactor code"

	result, err := InjectBrief("opencode", brief, prompt, "/tmp", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != prompt {
		t.Errorf("expected prompt unchanged %q, got %q", prompt, result)
	}
	if opts.SystemPrompt != brief {
		t.Errorf("expected SystemPrompt %q, got %q", brief, opts.SystemPrompt)
	}
}

func TestInjectBrief_Codex_SetsSystemPrompt(t *testing.T) {
	t.Parallel()

	opts := &ExecOptions{}
	brief := "Codex developer instructions"
	prompt := "implement feature"

	result, err := InjectBrief("codex", brief, prompt, "/tmp", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != prompt {
		t.Errorf("expected prompt unchanged %q, got %q", prompt, result)
	}
	if opts.SystemPrompt != brief {
		t.Errorf("expected SystemPrompt %q, got %q", brief, opts.SystemPrompt)
	}
}

func TestInjectBrief_Openclaw_PrependsToPrompt(t *testing.T) {
	t.Parallel()

	opts := &ExecOptions{}
	brief := "Agent brief content"
	prompt := "do the task"

	result, err := InjectBrief("openclaw", brief, prompt, "/tmp", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := brief + "\n\n---\n\n" + prompt
	if result != expected {
		t.Errorf("expected prompt %q, got %q", expected, result)
	}
	if opts.SystemPrompt != "" {
		t.Errorf("expected empty SystemPrompt for openclaw, got %q", opts.SystemPrompt)
	}
}

func TestInjectBrief_Kiro_PrependsToPrompt(t *testing.T) {
	t.Parallel()

	opts := &ExecOptions{}
	brief := "Kiro brief"
	prompt := "build feature"

	result, err := InjectBrief("kiro", brief, prompt, "/tmp", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := brief + "\n\n---\n\n" + prompt
	if result != expected {
		t.Errorf("expected prompt %q, got %q", expected, result)
	}
	if opts.SystemPrompt != "" {
		t.Errorf("expected empty SystemPrompt for kiro, got %q", opts.SystemPrompt)
	}
}

func TestInjectBrief_Kimi_PrependsToPrompt(t *testing.T) {
	t.Parallel()

	opts := &ExecOptions{}
	brief := "Kimi brief"
	prompt := "analyze code"

	result, err := InjectBrief("kimi", brief, prompt, "/tmp", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := brief + "\n\n---\n\n" + prompt
	if result != expected {
		t.Errorf("expected prompt %q, got %q", expected, result)
	}
	if opts.SystemPrompt != "" {
		t.Errorf("expected empty SystemPrompt for kimi, got %q", opts.SystemPrompt)
	}
}

func TestInjectBrief_Hermes_WritesAgentsMD(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	opts := &ExecOptions{}
	brief := "Hermes agent instructions"
	prompt := "run task"

	result, err := InjectBrief("hermes", brief, prompt, tmpDir, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != prompt {
		t.Errorf("expected prompt unchanged %q, got %q", prompt, result)
	}
	if opts.SystemPrompt != "" {
		t.Errorf("expected empty SystemPrompt for hermes, got %q", opts.SystemPrompt)
	}

	// Verify AGENTS.md was written.
	content, err := os.ReadFile(filepath.Join(tmpDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("failed to read AGENTS.md: %v", err)
	}
	if string(content) != brief {
		t.Errorf("expected AGENTS.md content %q, got %q", brief, string(content))
	}
}

func TestInjectBrief_UnknownProvider_WritesAgentsMD(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	opts := &ExecOptions{}
	brief := "Fallback instructions"
	prompt := "execute"

	result, err := InjectBrief("some-unknown-provider", brief, prompt, tmpDir, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != prompt {
		t.Errorf("expected prompt unchanged %q, got %q", prompt, result)
	}
	if opts.SystemPrompt != "" {
		t.Errorf("expected empty SystemPrompt for unknown provider, got %q", opts.SystemPrompt)
	}

	// Verify AGENTS.md was written.
	content, err := os.ReadFile(filepath.Join(tmpDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("failed to read AGENTS.md: %v", err)
	}
	if string(content) != brief {
		t.Errorf("expected AGENTS.md content %q, got %q", brief, string(content))
	}
}

func TestInjectBrief_Hermes_InvalidDir_ReturnsError(t *testing.T) {
	t.Parallel()

	opts := &ExecOptions{}
	brief := "Some instructions"
	prompt := "task"

	// Use a non-existent directory to trigger a write error.
	_, err := InjectBrief("hermes", brief, prompt, "/nonexistent/path/that/does/not/exist", opts)
	if err == nil {
		t.Fatal("expected error when writing to invalid directory")
	}
}

func TestInjectBrief_UnknownProvider_InvalidDir_ReturnsError(t *testing.T) {
	t.Parallel()

	opts := &ExecOptions{}
	brief := "Some instructions"
	prompt := "task"

	// Use a non-existent directory to trigger a write error.
	_, err := InjectBrief("mystery-agent", brief, prompt, "/nonexistent/path/that/does/not/exist", opts)
	if err == nil {
		t.Fatal("expected error when writing to invalid directory")
	}
}
