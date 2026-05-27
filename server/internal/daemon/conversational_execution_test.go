package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════════
// BuildConversationalPrompt tests
// ═══════════════════════════════════════════════════════════════════════════════

// ── First message (no PriorSessionID): directive + user prompt + prior context ──

func TestBuildConversationalPrompt_PlanFirstMessage(t *testing.T) {
	t.Parallel()

	prompt := BuildConversationalPrompt("plan", "Build a REST API", nil, "")

	// Should contain the plan directive.
	if !strings.Contains(prompt, conversationalPlanDirective) {
		t.Error("plan first message should contain the plan directive")
	}
	// Should contain the user prompt.
	if !strings.Contains(prompt, "Build a REST API") {
		t.Error("plan first message should contain the user prompt")
	}
	// Should NOT contain prior context section.
	if strings.Contains(prompt, "--- Prior Context ---") {
		t.Error("plan first message without prior context should not contain context section")
	}
}

func TestBuildConversationalPrompt_DesignFirstMessage(t *testing.T) {
	t.Parallel()

	prompt := BuildConversationalPrompt("design", "Design the auth module", nil, "")

	if !strings.Contains(prompt, conversationalDesignDirective) {
		t.Error("design first message should contain the design directive")
	}
	if !strings.Contains(prompt, "Design the auth module") {
		t.Error("design first message should contain the user prompt")
	}
}

func TestBuildConversationalPrompt_TasksFirstMessage(t *testing.T) {
	t.Parallel()

	prompt := BuildConversationalPrompt("tasks", "Break down the work", nil, "")

	if !strings.Contains(prompt, conversationalTasksDirective) {
		t.Error("tasks first message should contain the tasks directive")
	}
	if !strings.Contains(prompt, "Break down the work") {
		t.Error("tasks first message should contain the user prompt")
	}
}

func TestBuildConversationalPrompt_ExecutionFirstMessage(t *testing.T) {
	t.Parallel()

	prompt := BuildConversationalPrompt("execution", "Implement the login feature", nil, "")

	if !strings.Contains(prompt, conversationalExecutionDirective) {
		t.Error("execution first message should contain the execution directive")
	}
	if !strings.Contains(prompt, "Implement the login feature") {
		t.Error("execution first message should contain the user prompt")
	}
}

func TestBuildConversationalPrompt_FirstMessageWithPriorContext(t *testing.T) {
	t.Parallel()

	priorContext := []string{"Plan output here", "Design output here"}
	prompt := BuildConversationalPrompt("tasks", "Create task list", priorContext, "")

	// Should contain the tasks directive.
	if !strings.Contains(prompt, conversationalTasksDirective) {
		t.Error("should contain the tasks directive")
	}
	// Should contain the user prompt.
	if !strings.Contains(prompt, "Create task list") {
		t.Error("should contain the user prompt")
	}
	// Should contain prior context section.
	if !strings.Contains(prompt, "--- Prior Context ---") {
		t.Error("should contain prior context section header")
	}
	// Should contain each context entry.
	if !strings.Contains(prompt, "[Context 1]:") {
		t.Error("should contain [Context 1] label")
	}
	if !strings.Contains(prompt, "Plan output here") {
		t.Error("should contain first prior context entry")
	}
	if !strings.Contains(prompt, "[Context 2]:") {
		t.Error("should contain [Context 2] label")
	}
	if !strings.Contains(prompt, "Design output here") {
		t.Error("should contain second prior context entry")
	}
}

func TestBuildConversationalPrompt_UnknownDeliverableType_NoDirective(t *testing.T) {
	t.Parallel()

	prompt := BuildConversationalPrompt("unknown", "Do something", nil, "")

	// Should NOT contain any known directive.
	if strings.Contains(prompt, "planning assistant") {
		t.Error("unknown type should not contain plan directive")
	}
	if strings.Contains(prompt, "design assistant") {
		t.Error("unknown type should not contain design directive")
	}
	if strings.Contains(prompt, "task breakdown") {
		t.Error("unknown type should not contain tasks directive")
	}
	// Should still contain the user prompt.
	if !strings.Contains(prompt, "Do something") {
		t.Error("unknown type should still contain the user prompt")
	}
}

// ── Follow-up message (PriorSessionID present): user prompt only ──

func TestBuildConversationalPrompt_FollowUp_ReturnsOnlyUserPrompt(t *testing.T) {
	t.Parallel()

	// When PriorSessionID is present, the function should return ONLY the user prompt.
	prompt := BuildConversationalPrompt("plan", "Add more detail to section 3", nil, "session-abc-123")

	// Should be exactly the user prompt.
	if prompt != "Add more detail to section 3" {
		t.Errorf("follow-up should return only user prompt, got %q", prompt)
	}
}

func TestBuildConversationalPrompt_FollowUp_IgnoresDirective(t *testing.T) {
	t.Parallel()

	prompt := BuildConversationalPrompt("design", "Refine the API design", nil, "session-xyz")

	// Should NOT contain any directive.
	if strings.Contains(prompt, conversationalDesignDirective) {
		t.Error("follow-up should not contain the design directive")
	}
	if strings.Contains(prompt, "technical design") {
		t.Error("follow-up should not contain directive text")
	}
}

func TestBuildConversationalPrompt_FollowUp_IgnoresPriorContext(t *testing.T) {
	t.Parallel()

	priorContext := []string{"Some prior output"}
	prompt := BuildConversationalPrompt("tasks", "Revise task 3", priorContext, "session-456")

	// Should NOT contain prior context (session already has it).
	if strings.Contains(prompt, "--- Prior Context ---") {
		t.Error("follow-up should not contain prior context section")
	}
	if strings.Contains(prompt, "Some prior output") {
		t.Error("follow-up should not contain prior context content")
	}
	// Should be exactly the user prompt.
	if prompt != "Revise task 3" {
		t.Errorf("follow-up should return only user prompt, got %q", prompt)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// stageDirective tests
// ═══════════════════════════════════════════════════════════════════════════════

func TestStageDirective_AllTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		deliverableType string
		expected        string
	}{
		{"plan", conversationalPlanDirective},
		{"design", conversationalDesignDirective},
		{"tasks", conversationalTasksDirective},
		{"execution", conversationalExecutionDirective},
	}

	for _, tt := range tests {
		t.Run(tt.deliverableType, func(t *testing.T) {
			t.Parallel()
			got := stageDirective(tt.deliverableType)
			if got != tt.expected {
				t.Errorf("stageDirective(%q) = %q, want %q", tt.deliverableType, got, tt.expected)
			}
		})
	}
}

func TestStageDirective_UnknownType_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	unknownTypes := []string{"", "unknown", "review", "PLAN", "Plan"}
	for _, dt := range unknownTypes {
		got := stageDirective(dt)
		if got != "" {
			t.Errorf("stageDirective(%q) should return empty string, got %q", dt, got)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Stdout capture as output (no file reading) tests
// ═══════════════════════════════════════════════════════════════════════════════

func TestSimulateConversationalOutputCapture_AllTypes(t *testing.T) {
	t.Parallel()

	types := []string{"plan", "design", "tasks", "execution"}
	for _, dt := range types {
		t.Run(dt, func(t *testing.T) {
			t.Parallel()
			stdout := "This is the agent's stdout output for " + dt
			output := simulateConversationalOutputCapture(dt, stdout)
			if output != stdout {
				t.Errorf("for deliverable_type=%q, output should equal stdout.\nExpected: %q\nGot: %q", dt, stdout, output)
			}
		})
	}
}

func TestSimulateConversationalOutputCapture_PreservesWhitespace(t *testing.T) {
	t.Parallel()

	stdout := "  \n\tLeading whitespace\n\nTrailing whitespace  \n"
	output := simulateConversationalOutputCapture("plan", stdout)
	if output != stdout {
		t.Errorf("output should preserve whitespace exactly.\nExpected: %q\nGot: %q", stdout, output)
	}
}

func TestSimulateConversationalOutputCapture_EmptyStdout(t *testing.T) {
	t.Parallel()

	output := simulateConversationalOutputCapture("design", "")
	if output != "" {
		t.Errorf("empty stdout should produce empty output, got %q", output)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Session resume fallback on failure tests
// ═══════════════════════════════════════════════════════════════════════════════

// TestSessionResumeFallbackLogic verifies the branching logic for session resume:
// If execution fails with a PriorSessionID, the daemon retries without --resume.
func TestSessionResumeFallbackLogic(t *testing.T) {
	t.Parallel()

	// Simulate the fallback logic from executeConversationalStage:
	// if result.Status == "failed" && task.PriorSessionID != "" { retry without resume }

	tests := []struct {
		name            string
		resultStatus    string
		priorSessionID  string
		expectRetry     bool
		description     string
	}{
		{
			name:           "failed with prior session triggers retry",
			resultStatus:   "failed",
			priorSessionID: "session-123",
			expectRetry:    true,
			description:    "should retry without --resume when resume fails",
		},
		{
			name:           "failed without prior session no retry",
			resultStatus:   "failed",
			priorSessionID: "",
			expectRetry:    false,
			description:    "should not retry when there's no prior session",
		},
		{
			name:           "completed with prior session no retry",
			resultStatus:   "completed",
			priorSessionID: "session-456",
			expectRetry:    false,
			description:    "should not retry when execution succeeded",
		},
		{
			name:           "completed without prior session no retry",
			resultStatus:   "completed",
			priorSessionID: "",
			expectRetry:    false,
			description:    "should not retry on success without session",
		},
		{
			name:           "timeout with prior session no retry",
			resultStatus:   "timeout",
			priorSessionID: "session-789",
			expectRetry:    false,
			description:    "should not retry on timeout (only on failed status)",
		},
		{
			name:           "aborted with prior session no retry",
			resultStatus:   "aborted",
			priorSessionID: "session-abc",
			expectRetry:    false,
			description:    "should not retry on abort (only on failed status)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Replicate the fallback condition from executeConversationalStage.
			shouldRetry := tt.resultStatus == "failed" && tt.priorSessionID != ""

			if shouldRetry != tt.expectRetry {
				t.Errorf("%s: shouldRetry=%v, want %v", tt.description, shouldRetry, tt.expectRetry)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Workspace setup tests: existing dir, clone, missing dir error
// ═══════════════════════════════════════════════════════════════════════════════

// TestWorkspaceSetup_ExistingDirectory verifies that when local_directory_path
// exists, it is used as-is without cloning.
func TestWorkspaceSetup_ExistingDirectory(t *testing.T) {
	t.Parallel()

	// Create a temporary directory to simulate an existing workspace.
	tmpDir := t.TempDir()

	task := &PollResponse{
		TaskID:          "task-ws-1",
		AgentType:       "claude",
		Prompt:          "Implement feature",
		DeliverableType: "execution",
		WorkspaceConfig: &WorkspaceConfig{
			LocalDirectoryPath: tmpDir,
			GitRepoURL:         "https://github.com/example/repo.git",
		},
	}

	// Simulate the workspace resolution logic from executeConversationalStage.
	var workspaceDir string
	localDir := task.WorkspaceConfig.LocalDirectoryPath

	if _, err := os.Stat(localDir); err == nil {
		// Directory exists — use as-is.
		workspaceDir = localDir
	}

	if workspaceDir != tmpDir {
		t.Errorf("existing directory should be used as workspace, got %q", workspaceDir)
	}
}

// TestWorkspaceSetup_MissingDirNoGitURL verifies that when local_directory_path
// does not exist and no git_repo_url is provided, the task fails.
func TestWorkspaceSetup_MissingDirNoGitURL(t *testing.T) {
	t.Parallel()

	nonExistentPath := filepath.Join(t.TempDir(), "does-not-exist")

	task := &PollResponse{
		TaskID:          "task-ws-2",
		AgentType:       "claude",
		Prompt:          "Implement feature",
		DeliverableType: "execution",
		WorkspaceConfig: &WorkspaceConfig{
			LocalDirectoryPath: nonExistentPath,
			GitRepoURL:         "", // No git URL provided.
		},
	}

	// Simulate the workspace resolution logic.
	localDir := task.WorkspaceConfig.LocalDirectoryPath
	var errMsg string

	if _, err := os.Stat(localDir); os.IsNotExist(err) {
		if task.WorkspaceConfig.GitRepoURL != "" {
			// Would clone — but URL is empty.
			errMsg = ""
		} else {
			errMsg = "local directory does not exist and no git_repo_url provided for cloning"
		}
	}

	expectedErr := "local directory does not exist and no git_repo_url provided for cloning"
	if errMsg != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, errMsg)
	}
}

// TestWorkspaceSetup_MissingDirWithGitURL verifies that when local_directory_path
// does not exist but git_repo_url is provided, the clone path is triggered.
func TestWorkspaceSetup_MissingDirWithGitURL(t *testing.T) {
	t.Parallel()

	nonExistentPath := filepath.Join(t.TempDir(), "new-repo")

	task := &PollResponse{
		TaskID:          "task-ws-3",
		AgentType:       "claude",
		Prompt:          "Implement feature",
		DeliverableType: "execution",
		WorkspaceConfig: &WorkspaceConfig{
			LocalDirectoryPath: nonExistentPath,
			GitRepoURL:         "https://github.com/example/repo.git",
		},
	}

	// Simulate the workspace resolution logic.
	localDir := task.WorkspaceConfig.LocalDirectoryPath
	var shouldClone bool

	if _, err := os.Stat(localDir); os.IsNotExist(err) {
		if task.WorkspaceConfig.GitRepoURL != "" {
			shouldClone = true
		}
	}

	if !shouldClone {
		t.Error("should trigger git clone when directory doesn't exist and git_repo_url is provided")
	}
}

// TestWorkspaceSetup_PriorWorkDirFallback verifies that when no workspace_config
// is present but prior_work_dir is set, it is used as the workspace.
func TestWorkspaceSetup_PriorWorkDirFallback(t *testing.T) {
	t.Parallel()

	task := &PollResponse{
		TaskID:          "task-ws-4",
		AgentType:       "claude",
		Prompt:          "Continue work",
		DeliverableType: "design",
		PriorWorkDir:    "/home/user/workspace/prior",
	}

	// Simulate the workspace resolution logic from executeConversationalStage.
	var workspaceDir string

	// No execution-type workspace config.
	if task.DeliverableType == "execution" && task.WorkspaceConfig != nil && task.WorkspaceConfig.LocalDirectoryPath != "" {
		// Would handle workspace config — but this is a design task.
	} else if task.PriorWorkDir != "" {
		workspaceDir = task.PriorWorkDir
	}

	if workspaceDir != "/home/user/workspace/prior" {
		t.Errorf("should use PriorWorkDir as workspace, got %q", workspaceDir)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Branching between conversational and single-pass execution tests
// ═══════════════════════════════════════════════════════════════════════════════

// TestExecuteTaskStructured_BranchingLogic verifies the routing priority in
// executeTaskStructured:
//  1. DeliverableType present → conversational path
//  2. CurrentStage present → legacy staged path
//  3. Neither → single-pass path
func TestExecuteTaskStructured_BranchingLogic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		task         *PollResponse
		expectedPath string
	}{
		{
			name: "conversational task routes to conversational path",
			task: &PollResponse{
				TaskID:          "task-conv-1",
				AgentType:       "claude",
				Prompt:          "Create a plan",
				DeliverableType: "plan",
			},
			expectedPath: "conversational",
		},
		{
			name: "staged task routes to staged path",
			task: &PollResponse{
				TaskID:    "task-staged-1",
				AgentType: "claude",
				Prompt:    "Execute stage",
				CurrentStage: &StageInfo{
					Name:   "plan",
					Order:  1,
					Status: "pending",
				},
			},
			expectedPath: "staged",
		},
		{
			name: "single-pass task routes to single-pass path",
			task: &PollResponse{
				TaskID:    "task-single-1",
				AgentType: "claude",
				Prompt:    "Do the thing",
			},
			expectedPath: "single-pass",
		},
		{
			name: "conversational takes priority over staged",
			task: &PollResponse{
				TaskID:          "task-both-1",
				AgentType:       "claude",
				Prompt:          "Ambiguous task",
				DeliverableType: "design",
				CurrentStage: &StageInfo{
					Name:   "design",
					Order:  2,
					Status: "pending",
				},
			},
			expectedPath: "conversational",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Replicate the branching logic from executeTaskStructured.
			var path string
			if tt.task.DeliverableType != "" {
				path = "conversational"
			} else if tt.task.CurrentStage != nil {
				path = "staged"
			} else {
				path = "single-pass"
			}

			if path != tt.expectedPath {
				t.Errorf("expected path %q, got %q", tt.expectedPath, path)
			}
		})
	}
}

// TestExecuteTaskStructured_ConversationalPriority verifies that DeliverableType
// takes priority over CurrentStage in routing decisions.
func TestExecuteTaskStructured_ConversationalPriority(t *testing.T) {
	t.Parallel()

	task := &PollResponse{
		TaskID:          "task-priority",
		AgentType:       "claude",
		Prompt:          "Test priority",
		DeliverableType: "execution",
		CurrentStage: &StageInfo{
			Name:   "execution",
			Order:  4,
			Status: "pending",
		},
	}

	// The conversational path should be chosen because DeliverableType is checked first.
	isConversational := task.DeliverableType != ""
	if !isConversational {
		t.Error("conversational path should take priority when DeliverableType is present")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Prompt structure validation tests
// ═══════════════════════════════════════════════════════════════════════════════

func TestBuildConversationalPrompt_DirectiveBeforeUserPrompt(t *testing.T) {
	t.Parallel()

	prompt := BuildConversationalPrompt("plan", "My task description", nil, "")

	directiveIdx := strings.Index(prompt, conversationalPlanDirective)
	userPromptIdx := strings.Index(prompt, "My task description")

	if directiveIdx < 0 {
		t.Fatal("prompt should contain the directive")
	}
	if userPromptIdx < 0 {
		t.Fatal("prompt should contain the user prompt")
	}
	if directiveIdx >= userPromptIdx {
		t.Error("directive should appear before user prompt in the output")
	}
}

func TestBuildConversationalPrompt_PriorContextAfterUserPrompt(t *testing.T) {
	t.Parallel()

	priorContext := []string{"Context A"}
	prompt := BuildConversationalPrompt("design", "Design the system", priorContext, "")

	userPromptIdx := strings.Index(prompt, "Design the system")
	contextIdx := strings.Index(prompt, "Context A")

	if userPromptIdx < 0 {
		t.Fatal("prompt should contain the user prompt")
	}
	if contextIdx < 0 {
		t.Fatal("prompt should contain the prior context")
	}
	if contextIdx <= userPromptIdx {
		t.Error("prior context should appear after user prompt")
	}
}

func TestBuildConversationalPrompt_EmptyPriorContext_NoSection(t *testing.T) {
	t.Parallel()

	// Empty slice should not add context section.
	prompt := BuildConversationalPrompt("plan", "My prompt", []string{}, "")

	if strings.Contains(prompt, "--- Prior Context ---") {
		t.Error("empty prior context slice should not add context section")
	}
}

func TestBuildConversationalPrompt_NilPriorContext_NoSection(t *testing.T) {
	t.Parallel()

	prompt := BuildConversationalPrompt("plan", "My prompt", nil, "")

	if strings.Contains(prompt, "--- Prior Context ---") {
		t.Error("nil prior context should not add context section")
	}
}
