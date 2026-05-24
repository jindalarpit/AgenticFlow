package execenv

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewExecEnv_Success(t *testing.T) {
	task := Task{
		ID:           "task-123",
		AgentType:    "claude",
		Prompt:       "fix the bug",
		Model:        "claude-sonnet-4-20250514",
		ArgsTemplate: "{{prompt}}",
		EnvVars:      map[string]string{"KEY": "value"},
	}
	cfg := Config{
		WorkspacesRoot: "/tmp/workspaces",
		AgentTimeout:   2 * time.Hour,
	}

	env, err := NewExecEnv(task, cfg, "/usr/bin/claude", testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.TaskID != "task-123" {
		t.Errorf("TaskID = %q, want %q", env.TaskID, "task-123")
	}
	if env.WorkspaceDir != filepath.Join("/tmp/workspaces", "task-123") {
		t.Errorf("WorkspaceDir = %q, want %q", env.WorkspaceDir, filepath.Join("/tmp/workspaces", "task-123"))
	}
	if env.Provider != "claude" {
		t.Errorf("Provider = %q, want %q", env.Provider, "claude")
	}
	if env.Prompt != "fix the bug" {
		t.Errorf("Prompt = %q, want %q", env.Prompt, "fix the bug")
	}
	if env.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want %q", env.Model, "claude-sonnet-4-20250514")
	}
	if env.BinaryPath != "/usr/bin/claude" {
		t.Errorf("BinaryPath = %q, want %q", env.BinaryPath, "/usr/bin/claude")
	}
	if env.EnvVars["KEY"] != "value" {
		t.Errorf("EnvVars[KEY] = %q, want %q", env.EnvVars["KEY"], "value")
	}
}

func TestNewExecEnv_MissingWorkspacesRoot(t *testing.T) {
	task := Task{ID: "task-123", AgentType: "claude"}
	cfg := Config{WorkspacesRoot: ""}

	_, err := NewExecEnv(task, cfg, "/usr/bin/claude", testLogger())
	if err == nil {
		t.Fatal("expected error for missing workspaces root")
	}
}

func TestNewExecEnv_MissingTaskID(t *testing.T) {
	task := Task{ID: "", AgentType: "claude"}
	cfg := Config{WorkspacesRoot: "/tmp/workspaces"}

	_, err := NewExecEnv(task, cfg, "/usr/bin/claude", testLogger())
	if err == nil {
		t.Fatal("expected error for missing task ID")
	}
}

func TestNewExecEnv_MissingBinaryPath(t *testing.T) {
	task := Task{ID: "task-123", AgentType: "claude"}
	cfg := Config{WorkspacesRoot: "/tmp/workspaces"}

	_, err := NewExecEnv(task, cfg, "", testLogger())
	if err == nil {
		t.Fatal("expected error for missing binary path")
	}
}

func TestNewExecEnv_NilEnvVars(t *testing.T) {
	task := Task{ID: "task-123", AgentType: "claude", EnvVars: nil}
	cfg := Config{WorkspacesRoot: "/tmp/workspaces"}

	env, err := NewExecEnv(task, cfg, "/usr/bin/claude", testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.EnvVars == nil {
		t.Fatal("EnvVars should be initialized to empty map, not nil")
	}
}

func TestSetup_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	env := &ExecEnv{
		TaskID:       "task-setup-1",
		WorkspaceDir: filepath.Join(tmpDir, "task-setup-1"),
		Logger:       testLogger(),
	}

	if err := env.Setup(); err != nil {
		t.Fatalf("Setup() error: %v", err)
	}

	info, err := os.Stat(env.WorkspaceDir)
	if err != nil {
		t.Fatalf("workspace dir does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("workspace path is not a directory")
	}
}

func TestSetup_RemovesExistingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "task-existing")

	// Create existing workspace with a file inside.
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, "old-file.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	env := &ExecEnv{
		TaskID:       "task-existing",
		WorkspaceDir: wsDir,
		Logger:       testLogger(),
	}

	if err := env.Setup(); err != nil {
		t.Fatalf("Setup() error: %v", err)
	}

	// Verify old file is gone.
	if _, err := os.Stat(filepath.Join(wsDir, "old-file.txt")); !os.IsNotExist(err) {
		t.Fatal("expected old file to be removed after Setup()")
	}

	// Verify directory still exists (recreated).
	info, err := os.Stat(wsDir)
	if err != nil {
		t.Fatalf("workspace dir does not exist after Setup(): %v", err)
	}
	if !info.IsDir() {
		t.Fatal("workspace path is not a directory after Setup()")
	}
}

func TestCleanup_RemovesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "task-cleanup")

	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	env := &ExecEnv{
		TaskID:       "task-cleanup",
		WorkspaceDir: wsDir,
		Logger:       testLogger(),
	}

	if err := env.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error: %v", err)
	}

	if _, err := os.Stat(wsDir); !os.IsNotExist(err) {
		t.Fatal("expected workspace dir to be removed after Cleanup()")
	}
}

func TestCleanup_NonexistentDirectory(t *testing.T) {
	env := &ExecEnv{
		TaskID:       "task-nonexistent",
		WorkspaceDir: "/tmp/nonexistent-workspace-dir-xyz",
		Logger:       testLogger(),
	}

	// Cleanup on a non-existent directory should not error (RemoveAll is idempotent).
	if err := env.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error on non-existent dir: %v", err)
	}
}

func TestResolveArgs_BasicSubstitution(t *testing.T) {
	args := resolveArgs("{{prompt}}", "hello world", "/workspace", "gpt-4")
	if len(args) != 2 || args[0] != "hello" || args[1] != "world" {
		t.Errorf("resolveArgs basic = %v, want [hello world]", args)
	}
}

func TestResolveArgs_AllVariables(t *testing.T) {
	args := resolveArgs("--prompt {{prompt}} --workspace {{workspace}} --model {{model}}", "fix bug", "/ws", "gpt-4")
	expected := []string{"--prompt", "fix", "bug", "--workspace", "/ws", "--model", "gpt-4"}
	if len(args) != len(expected) {
		t.Fatalf("resolveArgs all vars: got %v, want %v", args, expected)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("resolveArgs[%d] = %q, want %q", i, a, expected[i])
		}
	}
}

func TestResolveArgs_EmptyModel(t *testing.T) {
	args := resolveArgs("--model {{model}} {{prompt}}", "test", "/ws", "")
	// {{model}} resolves to empty string, --model and empty are split by Fields
	// so we get: ["--model", "test"]
	expected := []string{"--model", "test"}
	if len(args) != len(expected) {
		t.Fatalf("resolveArgs empty model: got %v, want %v", args, expected)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("resolveArgs[%d] = %q, want %q", i, a, expected[i])
		}
	}
}

func TestResolveArgs_EmptyTemplate(t *testing.T) {
	args := resolveArgs("", "hello", "/ws", "model")
	if len(args) != 1 || args[0] != "hello" {
		t.Errorf("resolveArgs empty template = %v, want [hello]", args)
	}
}

func TestResolveArgs_EmptyTemplateAndPrompt(t *testing.T) {
	args := resolveArgs("", "", "/ws", "model")
	if args != nil {
		t.Errorf("resolveArgs empty template+prompt = %v, want nil", args)
	}
}

func TestRun_SuccessfulProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "task-run")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	env := &ExecEnv{
		TaskID:       "task-run",
		WorkspaceDir: wsDir,
		BinaryPath:   "/bin/echo",
		Prompt:       "hello",
		ArgsTemplate: "{{prompt}}",
		EnvVars:      map[string]string{},
		Logger:       testLogger(),
	}

	var stdout, stderr bytes.Buffer
	code, err := env.Run(context.Background(), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if got := stdout.String(); got != "hello\n" {
		t.Errorf("stdout = %q, want %q", got, "hello\n")
	}
}

func TestRun_NonZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "task-fail")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a script that exits with code 42.
	script := filepath.Join(tmpDir, "fail.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 42\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	env := &ExecEnv{
		TaskID:       "task-fail",
		WorkspaceDir: wsDir,
		BinaryPath:   script,
		Prompt:       "",
		ArgsTemplate: "",
		EnvVars:      map[string]string{},
		Logger:       testLogger(),
	}

	var stdout, stderr bytes.Buffer
	code, err := env.Run(context.Background(), &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if code != 42 {
		t.Errorf("exit code = %d, want 42", code)
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "task-cancel")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	env := &ExecEnv{
		TaskID:       "task-cancel",
		WorkspaceDir: wsDir,
		BinaryPath:   "/bin/sleep",
		Prompt:       "",
		ArgsTemplate: "60",
		EnvVars:      map[string]string{},
		Logger:       testLogger(),
	}

	ctx, cancel := context.WithCancel(context.Background())

	var stdout, stderr bytes.Buffer
	done := make(chan struct{})
	var code int
	var runErr error

	go func() {
		code, runErr = env.Run(ctx, &stdout, &stderr)
		close(done)
	}()

	// Give the process time to start.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Process should have been terminated.
		if runErr != context.Canceled {
			t.Errorf("Run() error = %v, want context.Canceled", runErr)
		}
		// Exit code should be non-zero (killed by signal).
		if code == 0 {
			t.Error("expected non-zero exit code after cancellation")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Run() did not return after context cancellation within 15s")
	}
}

func TestRun_WorkingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "task-cwd")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	env := &ExecEnv{
		TaskID:       "task-cwd",
		WorkspaceDir: wsDir,
		BinaryPath:   "/bin/pwd",
		Prompt:       "",
		ArgsTemplate: "",
		EnvVars:      map[string]string{},
		Logger:       testLogger(),
	}

	var stdout, stderr bytes.Buffer
	code, err := env.Run(context.Background(), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	// pwd output should match the workspace dir.
	// On macOS, /var is a symlink to /private/var, so resolve both paths.
	got := filepath.Clean(stdout.String()[:len(stdout.String())-1]) // trim newline
	want := filepath.Clean(wsDir)

	// Resolve symlinks for comparison (macOS /var → /private/var).
	gotResolved, _ := filepath.EvalSymlinks(got)
	wantResolved, _ := filepath.EvalSymlinks(want)
	if gotResolved == "" {
		gotResolved = got
	}
	if wantResolved == "" {
		wantResolved = want
	}

	if gotResolved != wantResolved {
		t.Errorf("working dir = %q, want %q", gotResolved, wantResolved)
	}
}

func TestRun_EnvironmentVariables(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "task-env")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a script that prints the env var.
	script := filepath.Join(tmpDir, "print_env.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$MY_TEST_VAR\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	env := &ExecEnv{
		TaskID:       "task-env",
		WorkspaceDir: wsDir,
		BinaryPath:   script,
		Prompt:       "",
		ArgsTemplate: "",
		EnvVars:      map[string]string{"MY_TEST_VAR": "hello_from_env"},
		Logger:       testLogger(),
	}

	var stdout, stderr bytes.Buffer
	code, err := env.Run(context.Background(), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if got := stdout.String(); got != "hello_from_env\n" {
		t.Errorf("stdout = %q, want %q", got, "hello_from_env\n")
	}
}

func TestRunWithStdin_ReturnsStdinPipe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "task-stdin")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a script that reads from stdin and echoes it.
	script := filepath.Join(tmpDir, "read_stdin.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nread line\necho \"got: $line\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	env := &ExecEnv{
		TaskID:       "task-stdin",
		WorkspaceDir: wsDir,
		BinaryPath:   script,
		Prompt:       "",
		ArgsTemplate: "",
		EnvVars:      map[string]string{},
		Logger:       testLogger(),
	}

	var stdout, stderr bytes.Buffer
	stdinPipe, done, err := env.RunWithStdin(context.Background(), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunWithStdin() error: %v", err)
	}
	if stdinPipe == nil {
		t.Fatal("expected non-nil stdin pipe")
	}

	// Write to stdin pipe.
	_, err = stdinPipe.Write([]byte("hello world\n"))
	if err != nil {
		t.Fatalf("stdin write error: %v", err)
	}

	// Wait for process to complete.
	select {
	case result := <-done:
		if result.ExitCode != 0 {
			t.Errorf("exit code = %d, want 0; stderr: %s", result.ExitCode, stderr.String())
		}
		if got := stdout.String(); got != "got: hello world\n" {
			t.Errorf("stdout = %q, want %q", got, "got: hello world\n")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunWithStdin process did not complete within 5s")
	}
}

func TestRunWithStdin_ContextCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "task-stdin-cancel")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	env := &ExecEnv{
		TaskID:       "task-stdin-cancel",
		WorkspaceDir: wsDir,
		BinaryPath:   "/bin/sleep",
		Prompt:       "",
		ArgsTemplate: "60",
		EnvVars:      map[string]string{},
		Logger:       testLogger(),
	}

	ctx, cancel := context.WithCancel(context.Background())

	var stdout, stderr bytes.Buffer
	stdinPipe, done, err := env.RunWithStdin(ctx, &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunWithStdin() error: %v", err)
	}
	if stdinPipe == nil {
		t.Fatal("expected non-nil stdin pipe")
	}

	// Give the process time to start.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case result := <-done:
		if result.Err != context.Canceled {
			t.Errorf("RunWithStdin() error = %v, want context.Canceled", result.Err)
		}
		if result.ExitCode == 0 {
			t.Error("expected non-zero exit code after cancellation")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("RunWithStdin process did not complete after context cancellation within 15s")
	}
}

func TestRunWithStdin_NonZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "task-stdin-fail")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a script that exits with code 7.
	script := filepath.Join(tmpDir, "fail.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 7\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	env := &ExecEnv{
		TaskID:       "task-stdin-fail",
		WorkspaceDir: wsDir,
		BinaryPath:   script,
		Prompt:       "",
		ArgsTemplate: "",
		EnvVars:      map[string]string{},
		Logger:       testLogger(),
	}

	var stdout, stderr bytes.Buffer
	stdinPipe, done, err := env.RunWithStdin(context.Background(), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunWithStdin() error: %v", err)
	}
	if stdinPipe == nil {
		t.Fatal("expected non-nil stdin pipe")
	}

	select {
	case result := <-done:
		if result.ExitCode != 7 {
			t.Errorf("exit code = %d, want 7", result.ExitCode)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunWithStdin process did not complete within 5s")
	}
}

func TestRunWithStdin_EnvironmentVariables(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "task-stdin-env")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a script that prints an env var.
	script := filepath.Join(tmpDir, "print_env.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$STDIN_TEST_VAR\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	env := &ExecEnv{
		TaskID:       "task-stdin-env",
		WorkspaceDir: wsDir,
		BinaryPath:   script,
		Prompt:       "",
		ArgsTemplate: "",
		EnvVars:      map[string]string{"STDIN_TEST_VAR": "stdin_env_value"},
		Logger:       testLogger(),
	}

	var stdout, stderr bytes.Buffer
	stdinPipe, done, err := env.RunWithStdin(context.Background(), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunWithStdin() error: %v", err)
	}
	if stdinPipe == nil {
		t.Fatal("expected non-nil stdin pipe")
	}

	select {
	case result := <-done:
		if result.ExitCode != 0 {
			t.Errorf("exit code = %d, want 0", result.ExitCode)
		}
		if got := stdout.String(); got != "stdin_env_value\n" {
			t.Errorf("stdout = %q, want %q", got, "stdin_env_value\n")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunWithStdin process did not complete within 5s")
	}
}
