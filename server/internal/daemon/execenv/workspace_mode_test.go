package execenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewExecEnv_ExistingMode_ValidPath(t *testing.T) {
	tmpDir := t.TempDir()

	task := Task{
		ID:            "task-existing-1",
		AgentType:     "claude",
		Prompt:        "fix the bug",
		WorkspaceMode: WorkspaceModeExisting,
		WorkspacePath: tmpDir,
	}
	cfg := Config{
		WorkspacesRoot: "/unused",
		AgentTimeout:   2 * time.Hour,
	}

	env, err := NewExecEnv(task, cfg, "/usr/bin/claude", testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.WorkspaceDir != tmpDir {
		t.Errorf("WorkspaceDir = %q, want %q", env.WorkspaceDir, tmpDir)
	}
	if env.WorkspaceMode != WorkspaceModeExisting {
		t.Errorf("WorkspaceMode = %q, want %q", env.WorkspaceMode, WorkspaceModeExisting)
	}
}

func TestNewExecEnv_ExistingMode_PathDoesNotExist(t *testing.T) {
	task := Task{
		ID:            "task-existing-2",
		AgentType:     "claude",
		WorkspaceMode: WorkspaceModeExisting,
		WorkspacePath: "/nonexistent/path/that/does/not/exist",
	}
	cfg := Config{WorkspacesRoot: "/unused"}

	_, err := NewExecEnv(task, cfg, "/usr/bin/claude", testLogger())
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
	expected := "workspace path does not exist: /nonexistent/path/that/does/not/exist"
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestNewExecEnv_ExistingMode_PathIsNotDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "not-a-dir.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	task := Task{
		ID:            "task-existing-3",
		AgentType:     "claude",
		WorkspaceMode: WorkspaceModeExisting,
		WorkspacePath: filePath,
	}
	cfg := Config{WorkspacesRoot: "/unused"}

	_, err := NewExecEnv(task, cfg, "/usr/bin/claude", testLogger())
	if err == nil {
		t.Fatal("expected error for non-directory path")
	}
	expected := "workspace path is not a directory: " + filePath
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestNewExecEnv_ExistingMode_PathNotWritable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping: running as root (root can write anywhere)")
	}

	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0o555); err != nil {
		t.Fatal(err)
	}
	// Ensure we can clean up after the test.
	t.Cleanup(func() {
		os.Chmod(readOnlyDir, 0o755)
	})

	task := Task{
		ID:            "task-existing-4",
		AgentType:     "claude",
		WorkspaceMode: WorkspaceModeExisting,
		WorkspacePath: readOnlyDir,
	}
	cfg := Config{WorkspacesRoot: "/unused"}

	_, err := NewExecEnv(task, cfg, "/usr/bin/claude", testLogger())
	if err == nil {
		t.Fatal("expected error for non-writable path")
	}
	expected := "workspace path is not writable: " + readOnlyDir
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestNewExecEnv_ExistingMode_MissingWorkspacePath(t *testing.T) {
	task := Task{
		ID:            "task-existing-5",
		AgentType:     "claude",
		WorkspaceMode: WorkspaceModeExisting,
		WorkspacePath: "",
	}
	cfg := Config{WorkspacesRoot: "/unused"}

	_, err := NewExecEnv(task, cfg, "/usr/bin/claude", testLogger())
	if err == nil {
		t.Fatal("expected error for missing workspace path")
	}
	if !strings.Contains(err.Error(), "workspace path is required") {
		t.Errorf("error = %q, want to contain 'workspace path is required'", err.Error())
	}
}

func TestNewExecEnv_IsolatedMode_DefaultBehavior(t *testing.T) {
	task := Task{
		ID:            "task-isolated-1",
		AgentType:     "claude",
		Prompt:        "fix the bug",
		WorkspaceMode: WorkspaceModeIsolated,
	}
	cfg := Config{
		WorkspacesRoot: "/tmp/workspaces",
		AgentTimeout:   2 * time.Hour,
	}

	env, err := NewExecEnv(task, cfg, "/usr/bin/claude", testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedDir := filepath.Join("/tmp/workspaces", "task-isolated-1")
	if env.WorkspaceDir != expectedDir {
		t.Errorf("WorkspaceDir = %q, want %q", env.WorkspaceDir, expectedDir)
	}
	if env.WorkspaceMode != WorkspaceModeIsolated {
		t.Errorf("WorkspaceMode = %q, want %q", env.WorkspaceMode, WorkspaceModeIsolated)
	}
}

func TestNewExecEnv_EmptyMode_DefaultsToIsolated(t *testing.T) {
	task := Task{
		ID:        "task-default-1",
		AgentType: "claude",
		Prompt:    "fix the bug",
		// WorkspaceMode intentionally left empty
	}
	cfg := Config{
		WorkspacesRoot: "/tmp/workspaces",
		AgentTimeout:   2 * time.Hour,
	}

	env, err := NewExecEnv(task, cfg, "/usr/bin/claude", testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.WorkspaceMode != WorkspaceModeIsolated {
		t.Errorf("WorkspaceMode = %q, want %q (default)", env.WorkspaceMode, WorkspaceModeIsolated)
	}
	expectedDir := filepath.Join("/tmp/workspaces", "task-default-1")
	if env.WorkspaceDir != expectedDir {
		t.Errorf("WorkspaceDir = %q, want %q", env.WorkspaceDir, expectedDir)
	}
}

func TestNewExecEnv_InvalidMode(t *testing.T) {
	task := Task{
		ID:            "task-invalid-1",
		AgentType:     "claude",
		WorkspaceMode: "bogus",
	}
	cfg := Config{WorkspacesRoot: "/tmp/workspaces"}

	_, err := NewExecEnv(task, cfg, "/usr/bin/claude", testLogger())
	if err == nil {
		t.Fatal("expected error for invalid workspace mode")
	}
	if !strings.Contains(err.Error(), "invalid workspace mode") {
		t.Errorf("error = %q, want to contain 'invalid workspace mode'", err.Error())
	}
}

func TestShouldCleanup_IsolatedMode(t *testing.T) {
	env := &ExecEnv{
		WorkspaceMode: WorkspaceModeIsolated,
		Logger:        testLogger(),
	}
	if !env.ShouldCleanup() {
		t.Error("ShouldCleanup() = false, want true for isolated mode")
	}
}

func TestShouldCleanup_ExistingMode(t *testing.T) {
	env := &ExecEnv{
		WorkspaceMode: WorkspaceModeExisting,
		Logger:        testLogger(),
	}
	if env.ShouldCleanup() {
		t.Error("ShouldCleanup() = true, want false for existing mode")
	}
}

func TestCleanup_ExistingMode_PreservesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "important-file.txt")
	if err := os.WriteFile(testFile, []byte("do not delete"), 0o644); err != nil {
		t.Fatal(err)
	}

	env := &ExecEnv{
		TaskID:        "task-existing-cleanup",
		WorkspaceDir:  tmpDir,
		WorkspaceMode: WorkspaceModeExisting,
		Logger:        testLogger(),
	}

	if err := env.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error: %v", err)
	}

	// Directory and file should still exist.
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Fatal("expected workspace dir to still exist after Cleanup() in existing mode")
	}
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatal("expected file to still exist after Cleanup() in existing mode")
	}
}

func TestCleanup_IsolatedMode_RemovesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "task-isolated-cleanup")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	env := &ExecEnv{
		TaskID:        "task-isolated-cleanup",
		WorkspaceDir:  wsDir,
		WorkspaceMode: WorkspaceModeIsolated,
		Logger:        testLogger(),
	}

	if err := env.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error: %v", err)
	}

	if _, err := os.Stat(wsDir); !os.IsNotExist(err) {
		t.Fatal("expected workspace dir to be removed after Cleanup() in isolated mode")
	}
}

func TestSetup_ExistingMode_NoOp(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "existing-file.txt")
	if err := os.WriteFile(testFile, []byte("preserve me"), 0o644); err != nil {
		t.Fatal(err)
	}

	env := &ExecEnv{
		TaskID:        "task-existing-setup",
		WorkspaceDir:  tmpDir,
		WorkspaceMode: WorkspaceModeExisting,
		Logger:        testLogger(),
	}

	if err := env.Setup(); err != nil {
		t.Fatalf("Setup() error: %v", err)
	}

	// File should still exist (Setup is a no-op for existing mode).
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("file should still exist: %v", err)
	}
	if string(data) != "preserve me" {
		t.Errorf("file content = %q, want %q", string(data), "preserve me")
	}
}

func TestSetup_IsolatedMode_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "task-isolated-setup")

	env := &ExecEnv{
		TaskID:        "task-isolated-setup",
		WorkspaceDir:  wsDir,
		WorkspaceMode: WorkspaceModeIsolated,
		Logger:        testLogger(),
	}

	if err := env.Setup(); err != nil {
		t.Fatalf("Setup() error: %v", err)
	}

	info, err := os.Stat(wsDir)
	if err != nil {
		t.Fatalf("workspace dir does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("workspace path is not a directory")
	}
}

func TestExistingMode_WorkspacesRootNotRequired(t *testing.T) {
	tmpDir := t.TempDir()

	task := Task{
		ID:            "task-existing-no-root",
		AgentType:     "claude",
		WorkspaceMode: WorkspaceModeExisting,
		WorkspacePath: tmpDir,
	}
	// WorkspacesRoot is empty — should be fine for existing mode.
	cfg := Config{WorkspacesRoot: ""}

	env, err := NewExecEnv(task, cfg, "/usr/bin/claude", testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.WorkspaceDir != tmpDir {
		t.Errorf("WorkspaceDir = %q, want %q", env.WorkspaceDir, tmpDir)
	}
}
