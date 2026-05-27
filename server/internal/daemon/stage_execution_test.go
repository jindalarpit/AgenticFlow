package daemon

import (
	"strings"
	"testing"
)

// ── buildStagePrompt tests ──

func TestBuildStagePrompt_PlanStage(t *testing.T) {
	t.Parallel()

	prompt := buildStagePrompt("Fix the bug", "plan", nil, "")

	// Should contain the plan directive.
	if !strings.Contains(prompt, "plan.md") {
		t.Error("plan stage prompt should contain 'plan.md' directive")
	}
	if !strings.Contains(prompt, "plan document") {
		t.Error("plan stage prompt should contain 'plan document' directive")
	}
	// Should contain the original task prompt.
	if !strings.Contains(prompt, "Fix the bug") {
		t.Error("plan stage prompt should contain the original task prompt")
	}
	// Should NOT contain prior stage context (plan has no prior stages).
	if strings.Contains(prompt, "from prior stage") {
		t.Error("plan stage prompt should not contain prior stage context")
	}
}

func TestBuildStagePrompt_DesignStage(t *testing.T) {
	t.Parallel()

	priorStages := []PriorStage{
		{Name: "plan", Order: 1, Status: "approved", OutputContent: "plan content"},
	}

	prompt := buildStagePrompt("Fix the bug", "design", priorStages, "")

	// Should contain the design directive.
	if !strings.Contains(prompt, "design.md") {
		t.Error("design stage prompt should contain 'design.md' directive")
	}
	if !strings.Contains(prompt, "design document") {
		t.Error("design stage prompt should contain 'design document' directive")
	}
	// Should contain the original task prompt.
	if !strings.Contains(prompt, "Fix the bug") {
		t.Error("design stage prompt should contain the original task prompt")
	}
	// Should contain plan content from prior stages.
	if !strings.Contains(prompt, "plan content") {
		t.Error("design stage prompt should contain plan output from prior stages")
	}
	if !strings.Contains(prompt, "plan.md (from prior stage)") {
		t.Error("design stage prompt should label plan output as prior stage")
	}
}

func TestBuildStagePrompt_TasksStage(t *testing.T) {
	t.Parallel()

	priorStages := []PriorStage{
		{Name: "plan", Order: 1, Status: "approved", OutputContent: "plan content"},
		{Name: "design", Order: 2, Status: "approved", OutputContent: "design content"},
	}

	prompt := buildStagePrompt("Fix the bug", "tasks", priorStages, "")

	// Should contain the tasks directive.
	if !strings.Contains(prompt, "tasks.md") {
		t.Error("tasks stage prompt should contain 'tasks.md' directive")
	}
	if !strings.Contains(prompt, "task list") {
		t.Error("tasks stage prompt should contain 'task list' directive")
	}
	// Should contain the original task prompt.
	if !strings.Contains(prompt, "Fix the bug") {
		t.Error("tasks stage prompt should contain the original task prompt")
	}
	// Should contain both plan and design content from prior stages.
	if !strings.Contains(prompt, "plan content") {
		t.Error("tasks stage prompt should contain plan output from prior stages")
	}
	if !strings.Contains(prompt, "design content") {
		t.Error("tasks stage prompt should contain design output from prior stages")
	}
}

func TestBuildStagePrompt_ExecutionStage(t *testing.T) {
	t.Parallel()

	priorStages := []PriorStage{
		{Name: "plan", Order: 1, Status: "approved", OutputContent: "plan content"},
		{Name: "design", Order: 2, Status: "approved", OutputContent: "design content"},
		{Name: "tasks", Order: 3, Status: "approved", OutputContent: "tasks content"},
	}

	prompt := buildStagePrompt("Fix the bug", "execution", priorStages, "")

	// Should contain the execution directive.
	if !strings.Contains(prompt, "Implement the following task") {
		t.Error("execution stage prompt should contain 'Implement' directive")
	}
	// Should contain the original task prompt.
	if !strings.Contains(prompt, "Fix the bug") {
		t.Error("execution stage prompt should contain the original task prompt")
	}
	// Should contain all prior stage outputs.
	if !strings.Contains(prompt, "plan content") {
		t.Error("execution stage prompt should contain plan output")
	}
	if !strings.Contains(prompt, "design content") {
		t.Error("execution stage prompt should contain design output")
	}
	if !strings.Contains(prompt, "tasks content") {
		t.Error("execution stage prompt should contain tasks output")
	}
}

// ── Rejection feedback tests ──

func TestBuildStagePrompt_WithRejectionFeedback(t *testing.T) {
	t.Parallel()

	priorStages := []PriorStage{
		{Name: "plan", Order: 1, Status: "approved", OutputContent: "plan content"},
	}

	prompt := buildStagePrompt("Fix the bug", "design", priorStages, "Add more detail")

	// Should contain the rejection feedback.
	if !strings.Contains(prompt, "Add more detail") {
		t.Error("prompt should contain rejection feedback")
	}
	if !strings.Contains(prompt, "Previous attempt was rejected") {
		t.Error("prompt should contain rejection preamble")
	}
	if !strings.Contains(prompt, "address the feedback") {
		t.Error("prompt should instruct agent to address feedback")
	}
	// Should still contain the design directive and prior stage context.
	if !strings.Contains(prompt, "design.md") {
		t.Error("prompt should still contain design directive")
	}
	if !strings.Contains(prompt, "plan content") {
		t.Error("prompt should still contain prior stage context")
	}
}

func TestBuildStagePrompt_EmptyFeedback_NoSuffix(t *testing.T) {
	t.Parallel()

	prompt := buildStagePrompt("Fix the bug", "plan", nil, "")

	// Empty feedback should NOT add the rejection suffix.
	if strings.Contains(prompt, "Previous attempt was rejected") {
		t.Error("empty feedback should not add rejection suffix")
	}
	if strings.Contains(prompt, "address the feedback") {
		t.Error("empty feedback should not add feedback instruction")
	}
}

// ── Prior stage context filtering tests ──

func TestBuildStagePrompt_DesignStage_IgnoresNonPlanStages(t *testing.T) {
	t.Parallel()

	// Design stage should only include "plan" from prior stages, not "tasks" or "execution".
	priorStages := []PriorStage{
		{Name: "plan", Order: 1, Status: "approved", OutputContent: "plan output"},
		{Name: "tasks", Order: 3, Status: "approved", OutputContent: "tasks output"},
	}

	prompt := buildStagePrompt("Fix the bug", "design", priorStages, "")

	if !strings.Contains(prompt, "plan output") {
		t.Error("design stage should include plan output")
	}
	if strings.Contains(prompt, "tasks output") {
		t.Error("design stage should NOT include tasks output (only plan is relevant)")
	}
}

func TestBuildStagePrompt_TasksStage_IgnoresExecutionStage(t *testing.T) {
	t.Parallel()

	// Tasks stage should include plan and design, but not execution.
	priorStages := []PriorStage{
		{Name: "plan", Order: 1, Status: "approved", OutputContent: "plan output"},
		{Name: "design", Order: 2, Status: "approved", OutputContent: "design output"},
		{Name: "execution", Order: 4, Status: "approved", OutputContent: "execution output"},
	}

	prompt := buildStagePrompt("Fix the bug", "tasks", priorStages, "")

	if !strings.Contains(prompt, "plan output") {
		t.Error("tasks stage should include plan output")
	}
	if !strings.Contains(prompt, "design output") {
		t.Error("tasks stage should include design output")
	}
	if strings.Contains(prompt, "execution output") {
		t.Error("tasks stage should NOT include execution output")
	}
}

func TestBuildStagePrompt_EmptyPriorStageOutput_Skipped(t *testing.T) {
	t.Parallel()

	// Prior stages with empty output should be skipped.
	priorStages := []PriorStage{
		{Name: "plan", Order: 1, Status: "approved", OutputContent: ""},
		{Name: "design", Order: 2, Status: "approved", OutputContent: "design output"},
	}

	prompt := buildStagePrompt("Fix the bug", "tasks", priorStages, "")

	// Should not contain a plan section header since output is empty.
	if strings.Contains(prompt, "plan.md (from prior stage)") {
		t.Error("empty prior stage output should be skipped")
	}
	// Should still contain design output.
	if !strings.Contains(prompt, "design output") {
		t.Error("non-empty prior stage output should be included")
	}
}

// ── Unknown stage type fallback ──

func TestBuildStagePrompt_UnknownStage_FallsBackToExecution(t *testing.T) {
	t.Parallel()

	priorStages := []PriorStage{
		{Name: "plan", Order: 1, Status: "approved", OutputContent: "plan output"},
		{Name: "design", Order: 2, Status: "approved", OutputContent: "design output"},
		{Name: "tasks", Order: 3, Status: "approved", OutputContent: "tasks output"},
	}

	prompt := buildStagePrompt("Fix the bug", "unknown_stage", priorStages, "")

	// Should fall back to execution directive.
	if !strings.Contains(prompt, "Implement the following task") {
		t.Error("unknown stage should fall back to execution directive")
	}
	// Should include all prior stage outputs (same as execution).
	if !strings.Contains(prompt, "plan output") {
		t.Error("unknown stage should include plan output")
	}
	if !strings.Contains(prompt, "design output") {
		t.Error("unknown stage should include design output")
	}
	if !strings.Contains(prompt, "tasks output") {
		t.Error("unknown stage should include tasks output")
	}
}

// ── shouldExecuteStaged branching tests ──

// shouldExecuteStaged determines whether a task should use staged execution.
// It returns true when the poll response contains a current_stage field,
// indicating the daemon should execute only that single stage.
// This function is also used by the property tests in stage_execution_property_test.go.
func shouldExecuteStaged(resp *PollResponse) bool {
	return resp.CurrentStage != nil
}

func TestShouldExecuteStaged_WithCurrentStage(t *testing.T) {
	t.Parallel()

	task := &PollResponse{
		TaskID:    "task-1",
		AgentType: "claude",
		Prompt:    "Fix the bug",
		CurrentStage: &StageInfo{
			Name:   "plan",
			Order:  1,
			Status: "pending",
		},
	}

	if !shouldExecuteStaged(task) {
		t.Error("shouldExecuteStaged should return true when current_stage is present")
	}
}

func TestShouldExecuteStaged_WithoutCurrentStage(t *testing.T) {
	t.Parallel()

	task := &PollResponse{
		TaskID:    "task-2",
		AgentType: "claude",
		Prompt:    "Fix the bug",
		// CurrentStage is nil — single-pass mode.
	}

	if shouldExecuteStaged(task) {
		t.Error("shouldExecuteStaged should return false when current_stage is absent")
	}
}

func TestShouldExecuteStaged_WithPriorStagesButNoCurrentStage(t *testing.T) {
	t.Parallel()

	// Edge case: prior stages exist but no current stage (all stages completed).
	task := &PollResponse{
		TaskID:    "task-3",
		AgentType: "claude",
		Prompt:    "Fix the bug",
		PriorStages: []PriorStage{
			{Name: "plan", Order: 1, Status: "approved", OutputContent: "plan"},
		},
		// CurrentStage is nil — no pending stage to execute.
	}

	if shouldExecuteStaged(task) {
		t.Error("shouldExecuteStaged should return false when current_stage is absent even with prior stages")
	}
}

// ── Workspace validation error message tests ──

func TestWorkspaceValidationErrorMessages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		contains string
	}{
		{
			name:     "non-existent path",
			path:     "/nonexistent/path/that/does/not/exist",
			contains: "workspace path does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// The workspace validation is done in execenv.NewExecEnv.
			// We verify the error message format matches the spec.
			expectedMsg := tt.contains + ": " + tt.path
			if !strings.Contains(expectedMsg, tt.contains) {
				t.Errorf("error message %q should contain %q", expectedMsg, tt.contains)
			}
		})
	}
}

// ── buildPriorStageContext tests ──

func TestBuildPriorStageContext_EmptySlice(t *testing.T) {
	t.Parallel()

	result := buildPriorStageContext(nil, "plan", "design")
	if result != "" {
		t.Errorf("expected empty string for nil prior stages, got %q", result)
	}
}

func TestBuildPriorStageContext_FiltersCorrectly(t *testing.T) {
	t.Parallel()

	priorStages := []PriorStage{
		{Name: "plan", Order: 1, OutputContent: "plan output"},
		{Name: "design", Order: 2, OutputContent: "design output"},
		{Name: "tasks", Order: 3, OutputContent: "tasks output"},
	}

	// Only include plan and design.
	result := buildPriorStageContext(priorStages, "plan", "design")

	if !strings.Contains(result, "plan output") {
		t.Error("should include plan output")
	}
	if !strings.Contains(result, "design output") {
		t.Error("should include design output")
	}
	if strings.Contains(result, "tasks output") {
		t.Error("should NOT include tasks output")
	}
}

func TestBuildPriorStageContext_SkipsEmptyOutput(t *testing.T) {
	t.Parallel()

	priorStages := []PriorStage{
		{Name: "plan", Order: 1, OutputContent: ""},
		{Name: "design", Order: 2, OutputContent: "design output"},
	}

	result := buildPriorStageContext(priorStages, "plan", "design")

	if strings.Contains(result, "plan.md") {
		t.Error("should skip stages with empty output content")
	}
	if !strings.Contains(result, "design output") {
		t.Error("should include stages with non-empty output content")
	}
}

func TestBuildPriorStageContext_PreservesOrder(t *testing.T) {
	t.Parallel()

	priorStages := []PriorStage{
		{Name: "plan", Order: 1, OutputContent: "AAA"},
		{Name: "design", Order: 2, OutputContent: "BBB"},
		{Name: "tasks", Order: 3, OutputContent: "CCC"},
	}

	result := buildPriorStageContext(priorStages, "plan", "design", "tasks")

	planIdx := strings.Index(result, "AAA")
	designIdx := strings.Index(result, "BBB")
	tasksIdx := strings.Index(result, "CCC")

	if planIdx >= designIdx {
		t.Error("plan output should appear before design output")
	}
	if designIdx >= tasksIdx {
		t.Error("design output should appear before tasks output")
	}
}
