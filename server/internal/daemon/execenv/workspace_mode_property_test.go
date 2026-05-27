package execenv

import (
	"os"
	"path/filepath"
	"testing"

	"pgregory.net/rapid"
)

// Feature: task-workflow-stages, Property 8: Existing Workspace Preservation
// For any task with workspace_mode="existing" and a valid workspace_path,
// the daemon SHALL use that path as the agent CLI working directory,
// SHALL NOT create or remove the directory, and SHALL NOT perform cleanup
// after task completion.
//
// **Validates: Requirements 6.6, 7.1, 7.2, 7.3**
func TestProperty8_ExistingWorkspacePreservation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Create a real temp directory to serve as the existing workspace.
		baseDir := os.TempDir()
		dirName := "af-prop8-" + rapid.StringMatching(`[a-z0-9]{6,12}`).Draw(t, "dirSuffix")
		existingDir := filepath.Join(baseDir, dirName)

		if err := os.MkdirAll(existingDir, 0o755); err != nil {
			t.Fatalf("failed to create test dir: %v", err)
		}
		defer os.RemoveAll(existingDir)

		// Place a marker file in the directory to verify it's not recreated or wiped.
		markerFile := filepath.Join(existingDir, "marker.txt")
		markerContent := rapid.StringMatching(`[a-zA-Z0-9]{10,30}`).Draw(t, "markerContent")
		if err := os.WriteFile(markerFile, []byte(markerContent), 0o644); err != nil {
			t.Fatalf("failed to write marker file: %v", err)
		}

		// Generate a random task ID.
		taskID := rapid.StringMatching(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`).Draw(t, "taskID")

		task := Task{
			ID:            taskID,
			AgentType:     "claude",
			Prompt:        "test prompt",
			WorkspaceMode: WorkspaceModeExisting,
			WorkspacePath: existingDir,
		}
		cfg := Config{
			WorkspacesRoot: "/unused",
		}

		// Property: NewExecEnv uses the specified path as WorkspaceDir.
		env, err := NewExecEnv(task, cfg, "/usr/bin/claude", testLogger())
		if err != nil {
			t.Fatalf("NewExecEnv failed: %v", err)
		}
		if env.WorkspaceDir != existingDir {
			t.Fatalf("WorkspaceDir = %q, want %q", env.WorkspaceDir, existingDir)
		}

		// Property: Setup() does NOT create the directory (it already exists)
		// and does NOT modify existing contents.
		if err := env.Setup(); err != nil {
			t.Fatalf("Setup() error: %v", err)
		}

		// Verify directory still exists with original contents.
		info, err := os.Stat(existingDir)
		if err != nil {
			t.Fatalf("directory should still exist after Setup(): %v", err)
		}
		if !info.IsDir() {
			t.Fatalf("path should still be a directory after Setup()")
		}
		data, err := os.ReadFile(markerFile)
		if err != nil {
			t.Fatalf("marker file should still exist after Setup(): %v", err)
		}
		if string(data) != markerContent {
			t.Fatalf("marker file content = %q, want %q (Setup modified contents)", string(data), markerContent)
		}

		// Property: ShouldCleanup() returns false for existing mode.
		if env.ShouldCleanup() {
			t.Fatalf("ShouldCleanup() = true, want false for existing mode")
		}

		// Property: Cleanup() does NOT remove the directory.
		if err := env.Cleanup(); err != nil {
			t.Fatalf("Cleanup() error: %v", err)
		}

		// Verify directory and contents still exist after Cleanup.
		info, err = os.Stat(existingDir)
		if err != nil {
			t.Fatalf("directory should still exist after Cleanup(): %v", err)
		}
		if !info.IsDir() {
			t.Fatalf("path should still be a directory after Cleanup()")
		}
		data, err = os.ReadFile(markerFile)
		if err != nil {
			t.Fatalf("marker file should still exist after Cleanup(): %v", err)
		}
		if string(data) != markerContent {
			t.Fatalf("marker file content = %q, want %q (Cleanup modified contents)", string(data), markerContent)
		}
	})
}

// Feature: task-workflow-stages, Property 8: Existing Workspace Preservation (contrast)
// For any task with workspace_mode="isolated", ShouldCleanup() returns true
// and Setup() creates the directory.
//
// **Validates: Requirements 7.4** (contrast with existing mode)
func TestProperty8_IsolatedModeContrast(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Create a temp root for isolated workspaces.
		baseDir := os.TempDir()
		rootName := "af-prop8-iso-" + rapid.StringMatching(`[a-z0-9]{6,12}`).Draw(t, "rootSuffix")
		workspacesRoot := filepath.Join(baseDir, rootName)

		if err := os.MkdirAll(workspacesRoot, 0o755); err != nil {
			t.Fatalf("failed to create workspaces root: %v", err)
		}
		defer os.RemoveAll(workspacesRoot)

		// Generate a random task ID.
		taskID := rapid.StringMatching(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`).Draw(t, "taskID")

		task := Task{
			ID:            taskID,
			AgentType:     "claude",
			Prompt:        "test prompt",
			WorkspaceMode: WorkspaceModeIsolated,
		}
		cfg := Config{
			WorkspacesRoot: workspacesRoot,
		}

		env, err := NewExecEnv(task, cfg, "/usr/bin/claude", testLogger())
		if err != nil {
			t.Fatalf("NewExecEnv failed: %v", err)
		}

		expectedDir := filepath.Join(workspacesRoot, taskID)
		if env.WorkspaceDir != expectedDir {
			t.Fatalf("WorkspaceDir = %q, want %q", env.WorkspaceDir, expectedDir)
		}

		// Property: ShouldCleanup() returns true for isolated mode.
		if !env.ShouldCleanup() {
			t.Fatalf("ShouldCleanup() = false, want true for isolated mode")
		}

		// Property: Setup() creates the directory.
		if err := env.Setup(); err != nil {
			t.Fatalf("Setup() error: %v", err)
		}

		info, err := os.Stat(expectedDir)
		if err != nil {
			t.Fatalf("workspace dir should exist after Setup(): %v", err)
		}
		if !info.IsDir() {
			t.Fatalf("workspace path should be a directory after Setup()")
		}

		// Clean up the created directory.
		os.RemoveAll(expectedDir)
	})
}
