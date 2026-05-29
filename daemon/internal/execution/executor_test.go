package execution

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/agenticflow/agenticflow/daemon/internal/execution/execenv"
	"github.com/agenticflow/agenticflow/shared/api"
)

// mockReporter implements Reporter for testing.
type mockReporter struct {
	messages  []api.TaskMessageEntry
	completed *api.TaskCompleteRequest
	failed    *api.TaskFailRequest
}

func (m *mockReporter) ReportMessages(_ context.Context, _ string, msgs []api.TaskMessageEntry) error {
	m.messages = append(m.messages, msgs...)
	return nil
}

func (m *mockReporter) CompleteTask(_ context.Context, _ string, req api.TaskCompleteRequest) error {
	m.completed = &req
	return nil
}

func (m *mockReporter) FailTask(_ context.Context, _ string, req api.TaskFailRequest) error {
	m.failed = &req
	return nil
}

func testExecutorLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestExecute_SuccessfulProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()
	reporter := &mockReporter{}
	executor := NewExecutor(testExecutorLogger(), reporter)

	cfg := TaskConfig{
		TaskID:         "task-exec-1",
		AgentType:      "claude",
		Prompt:         "hello",
		BinaryPath:     "/bin/echo",
		WorkspaceMode:  execenv.WorkspaceModeIsolated,
		WorkspacesRoot: tmpDir,
		AgentTimeout:   30 * time.Second,
	}

	result := executor.Execute(context.Background(), cfg)

	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.Error)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	if reporter.completed == nil {
		t.Fatal("expected CompleteTask to be called")
	}
	if reporter.completed.ExitCode != 0 {
		t.Errorf("reported exit code = %d, want 0", reporter.completed.ExitCode)
	}
}

func TestExecute_FailedProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()

	// Create a script that exits with code 1.
	script := filepath.Join(tmpDir, "fail.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho 'error output' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	reporter := &mockReporter{}
	executor := NewExecutor(testExecutorLogger(), reporter)

	cfg := TaskConfig{
		TaskID:         "task-exec-2",
		AgentType:      "custom",
		Prompt:         "",
		BinaryPath:     script,
		WorkspaceMode:  execenv.WorkspaceModeIsolated,
		WorkspacesRoot: tmpDir,
		AgentTimeout:   30 * time.Second,
	}

	result := executor.Execute(context.Background(), cfg)

	if result.Success {
		t.Fatal("expected failure, got success")
	}
	if result.ExitCode != 1 {
		t.Errorf("exit code = %d, want 1", result.ExitCode)
	}
	if reporter.failed == nil {
		t.Fatal("expected FailTask to be called")
	}
	if reporter.failed.ExitCode != 1 {
		t.Errorf("reported exit code = %d, want 1", reporter.failed.ExitCode)
	}
}

func TestExecute_StreamsOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()

	// Create a script that writes to both stdout and stderr.
	script := filepath.Join(tmpDir, "output.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho 'stdout line'\necho 'stderr line' >&2\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	reporter := &mockReporter{}
	executor := NewExecutor(testExecutorLogger(), reporter)

	cfg := TaskConfig{
		TaskID:         "task-exec-3",
		AgentType:      "custom",
		Prompt:         "",
		BinaryPath:     script,
		WorkspaceMode:  execenv.WorkspaceModeIsolated,
		WorkspacesRoot: tmpDir,
		AgentTimeout:   30 * time.Second,
	}

	result := executor.Execute(context.Background(), cfg)

	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.Error)
	}

	// Verify messages were streamed.
	if len(reporter.messages) == 0 {
		t.Fatal("expected streaming messages to be reported")
	}

	// Check that we have both stdout and stderr messages.
	hasStdout := false
	hasStderr := false
	for _, msg := range reporter.messages {
		if msg.Stream == "stdout" {
			hasStdout = true
		}
		if msg.Stream == "stderr" {
			hasStderr = true
		}
	}
	if !hasStdout {
		t.Error("expected stdout messages")
	}
	if !hasStderr {
		t.Error("expected stderr messages")
	}
}

func TestExecute_CustomEnvMerged(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()

	// Create a script that prints a custom env var.
	script := filepath.Join(tmpDir, "env.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$CUSTOM_VAR\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	reporter := &mockReporter{}
	executor := NewExecutor(testExecutorLogger(), reporter)

	cfg := TaskConfig{
		TaskID:         "task-exec-4",
		AgentType:      "custom",
		Prompt:         "",
		BinaryPath:     script,
		CustomEnv:      map[string]string{"CUSTOM_VAR": "custom_value"},
		WorkspaceMode:  execenv.WorkspaceModeIsolated,
		WorkspacesRoot: tmpDir,
		AgentTimeout:   30 * time.Second,
	}

	result := executor.Execute(context.Background(), cfg)

	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.Error)
	}
	if result.Output != "custom_value\n" {
		t.Errorf("output = %q, want %q", result.Output, "custom_value\n")
	}
}

func TestExecute_CustomArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()
	reporter := &mockReporter{}
	executor := NewExecutor(testExecutorLogger(), reporter)

	// Use echo with custom args — the custom args are appended after provider args.
	// For "custom" agent type with no template, prompt is the first arg.
	cfg := TaskConfig{
		TaskID:         "task-exec-5",
		AgentType:      "custom-agent",
		Prompt:         "hello",
		BinaryPath:     "/bin/echo",
		CustomArgs:     []string{"world"},
		WorkspaceMode:  execenv.WorkspaceModeIsolated,
		WorkspacesRoot: tmpDir,
		AgentTimeout:   30 * time.Second,
	}

	result := executor.Execute(context.Background(), cfg)

	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.Error)
	}
	// For unknown provider with no template, args = [prompt] + customArgs = ["hello", "world"]
	if result.Output != "hello world\n" {
		t.Errorf("output = %q, want %q", result.Output, "hello world\n")
	}
}

func TestExecute_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()
	reporter := &mockReporter{}
	executor := NewExecutor(testExecutorLogger(), reporter)

	cfg := TaskConfig{
		TaskID:         "task-exec-timeout",
		AgentType:      "custom",
		Prompt:         "60",
		BinaryPath:     "/bin/sleep",
		WorkspaceMode:  execenv.WorkspaceModeIsolated,
		WorkspacesRoot: tmpDir,
		AgentTimeout:   500 * time.Millisecond,
	}

	result := executor.Execute(context.Background(), cfg)

	if result.Success {
		t.Fatal("expected failure due to timeout")
	}
	if reporter.failed == nil {
		t.Fatal("expected FailTask to be called on timeout")
	}
}

func TestExecute_InvalidBinaryPath(t *testing.T) {
	tmpDir := t.TempDir()
	reporter := &mockReporter{}
	executor := NewExecutor(testExecutorLogger(), reporter)

	cfg := TaskConfig{
		TaskID:         "task-exec-invalid",
		AgentType:      "custom",
		Prompt:         "test",
		BinaryPath:     "/nonexistent/binary",
		WorkspaceMode:  execenv.WorkspaceModeIsolated,
		WorkspacesRoot: tmpDir,
		AgentTimeout:   30 * time.Second,
	}

	result := executor.Execute(context.Background(), cfg)

	if result.Success {
		t.Fatal("expected failure for invalid binary path")
	}
	if result.ExitCode != -1 {
		t.Errorf("exit code = %d, want -1", result.ExitCode)
	}
}

func TestExecuteWithHooks_BriefInjection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()

	// Create a script that echoes its arguments (the prompt becomes an arg).
	script := filepath.Join(tmpDir, "echo_prompt.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	reporter := &mockReporter{}
	executor := NewExecutor(testExecutorLogger(), reporter)

	cfg := TaskConfig{
		TaskID:         "task-hooks-brief",
		AgentType:      "custom-agent",
		Prompt:         "original prompt",
		BinaryPath:     script,
		WorkspaceMode:  execenv.WorkspaceModeIsolated,
		WorkspacesRoot: tmpDir,
		AgentTimeout:   30 * time.Second,
	}

	hooks := ExecutionHooks{
		BriefInjector: func(prompt string) (string, string, error) {
			return "injected: " + prompt, "system-prompt-value", nil
		},
	}

	result := executor.ExecuteWithHooks(context.Background(), cfg, hooks)

	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.Error)
	}
	// The prompt should have been modified by the brief injector.
	if result.Output != "injected: original prompt\n" {
		t.Errorf("output = %q, want %q", result.Output, "injected: original prompt\n")
	}
}

func TestExecuteWithHooks_OnStdoutHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()
	reporter := &mockReporter{}
	executor := NewExecutor(testExecutorLogger(), reporter)

	cfg := TaskConfig{
		TaskID:         "task-hooks-stdout",
		AgentType:      "custom-agent",
		Prompt:         "hello world",
		BinaryPath:     "/bin/echo",
		WorkspaceMode:  execenv.WorkspaceModeIsolated,
		WorkspacesRoot: tmpDir,
		AgentTimeout:   30 * time.Second,
	}

	var capturedOutput []byte
	hooks := ExecutionHooks{
		OnStdout: func(p []byte) {
			capturedOutput = append(capturedOutput, p...)
		},
	}

	result := executor.ExecuteWithHooks(context.Background(), cfg, hooks)

	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.Error)
	}
	if len(capturedOutput) == 0 {
		t.Error("expected OnStdout hook to be called with output")
	}
	if string(capturedOutput) != "hello world\n" {
		t.Errorf("captured output = %q, want %q", string(capturedOutput), "hello world\n")
	}
}

func TestExecuteWithHooks_StdinProvider(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()

	// Create a script that reads from stdin and echoes it.
	script := filepath.Join(tmpDir, "cat_stdin.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nread line\necho \"got: $line\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	reporter := &mockReporter{}
	executor := NewExecutor(testExecutorLogger(), reporter)

	cfg := TaskConfig{
		TaskID:         "task-hooks-stdin",
		AgentType:      "custom-agent",
		Prompt:         "",
		BinaryPath:     script,
		WorkspaceMode:  execenv.WorkspaceModeIsolated,
		WorkspacesRoot: tmpDir,
		AgentTimeout:   30 * time.Second,
	}

	hooks := ExecutionHooks{
		StdinProvider: func(stdin io.WriteCloser) {
			// Write to stdin and close it.
			stdin.Write([]byte("hello from stdin\n"))
			stdin.Close()
		},
	}

	result := executor.ExecuteWithHooks(context.Background(), cfg, hooks)

	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.Error)
	}
	if result.Output != "got: hello from stdin\n" {
		t.Errorf("output = %q, want %q", result.Output, "got: hello from stdin\n")
	}
}
