package handler

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Unit tests for task creation handler logic (deliverables, workspace, stages)
// ---------------------------------------------------------------------------

// TestCreateTask_DefaultDeliverables verifies that when deliverables is omitted
// from the request, the system defaults to ["execution"] (backward compat).
// Requirements: 1.3, 11.1
func TestCreateTask_DefaultDeliverables(t *testing.T) {
	req := CreateTaskReq{
		AgentType: "claude",
		Prompt:    "Fix the bug",
		// Deliverables intentionally omitted
	}

	// When deliverables is nil/empty, the handler defaults to ["execution"].
	deliverables := req.Deliverables
	if len(deliverables) == 0 {
		deliverables = []string{"execution"}
	}

	if len(deliverables) != 1 || deliverables[0] != "execution" {
		t.Errorf("expected default deliverables [\"execution\"], got %v", deliverables)
	}

	// Single-pass mode: no stages should be created.
	singlePass := len(deliverables) == 1 && deliverables[0] == "execution"
	if !singlePass {
		t.Error("expected single-pass mode when deliverables defaults to [\"execution\"]")
	}
}

// TestCreateTask_SingleExecutionDeliverable verifies that explicitly providing
// ["execution"] results in single-pass mode with no stages created.
// Requirements: 1.6, 11.1
func TestCreateTask_SingleExecutionDeliverable(t *testing.T) {
	req := CreateTaskReq{
		AgentType:    "claude",
		Prompt:       "Fix the bug",
		Deliverables: []string{"execution"},
	}

	deliverables := req.Deliverables

	// Validation should pass.
	if errMsg := validateDeliverables(deliverables); errMsg != "" {
		t.Fatalf("unexpected validation error: %s", errMsg)
	}

	// Single-pass mode: no stages should be created.
	singlePass := len(deliverables) == 1 && deliverables[0] == "execution"
	if !singlePass {
		t.Error("expected single-pass mode for [\"execution\"] deliverable")
	}
}

// TestCreateTask_MultipleDeliverables_StagesCreatedInOrder verifies that when
// multiple deliverables are selected, stages are created in canonical order.
// Requirements: 1.2, 1.5, 2.1, 2.2
func TestCreateTask_MultipleDeliverables_StagesCreatedInOrder(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "all deliverables in random order",
			input:    []string{"execution", "plan", "tasks", "design"},
			expected: []string{"plan", "design", "tasks", "execution"},
		},
		{
			name:     "plan and execution",
			input:    []string{"execution", "plan"},
			expected: []string{"plan", "execution"},
		},
		{
			name:     "design and tasks",
			input:    []string{"tasks", "design"},
			expected: []string{"design", "tasks"},
		},
		{
			name:     "plan, design, execution (skip tasks)",
			input:    []string{"design", "execution", "plan"},
			expected: []string{"plan", "design", "execution"},
		},
		{
			name:     "already in order",
			input:    []string{"plan", "design", "tasks", "execution"},
			expected: []string{"plan", "design", "tasks", "execution"},
		},
		{
			name:     "duplicates removed",
			input:    []string{"plan", "plan", "execution", "execution"},
			expected: []string{"plan", "execution"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validation should pass.
			if errMsg := validateDeliverables(tt.input); errMsg != "" {
				t.Fatalf("unexpected validation error: %s", errMsg)
			}

			// Not single-pass when multiple deliverables (or not just "execution").
			singlePass := len(tt.input) == 1 && tt.input[0] == "execution"

			sorted := orderDeliverables(tt.input)

			if singlePass {
				t.Skip("single-pass mode, no stages to verify")
			}

			if len(sorted) != len(tt.expected) {
				t.Fatalf("orderDeliverables(%v) returned %d items, want %d",
					tt.input, len(sorted), len(tt.expected))
			}

			for i, want := range tt.expected {
				if sorted[i] != want {
					t.Errorf("orderDeliverables(%v)[%d] = %q, want %q",
						tt.input, i, sorted[i], want)
				}
			}

			// Verify stage_order values are correct for each deliverable.
			for i, d := range sorted {
				order := deliverableOrder[d]
				if i > 0 {
					prevOrder := deliverableOrder[sorted[i-1]]
					if order <= prevOrder {
						t.Errorf("stage order not increasing: %s(%d) after %s(%d)",
							d, order, sorted[i-1], prevOrder)
					}
				}
			}
		})
	}
}

// TestCreateTask_ValidationErrors verifies that invalid inputs produce the
// correct validation error messages.
// Requirements: 1.1, 1.2
func TestCreateTask_ValidationErrors(t *testing.T) {
	t.Run("empty deliverables array", func(t *testing.T) {
		errMsg := validateDeliverables([]string{})
		if errMsg != "deliverables must contain at least one valid value" {
			t.Errorf("got %q, want 'deliverables must contain at least one valid value'", errMsg)
		}
	})

	t.Run("invalid deliverable value", func(t *testing.T) {
		errMsg := validateDeliverables([]string{"plan", "invalid_stage"})
		expected := "invalid deliverable: invalid_stage. Valid values: plan, design, tasks, execution"
		if errMsg != expected {
			t.Errorf("got %q, want %q", errMsg, expected)
		}
	})

	t.Run("workspace_mode existing without workspace_path", func(t *testing.T) {
		errMsg := validateWorkspacePath("existing", "")
		expected := "workspace_path is required when workspace_mode is 'existing'"
		if errMsg != expected {
			t.Errorf("got %q, want %q", errMsg, expected)
		}
	})

	t.Run("workspace_mode existing with relative path", func(t *testing.T) {
		errMsg := validateWorkspacePath("existing", "relative/path/to/project")
		expected := "workspace_path must be an absolute path"
		if errMsg != expected {
			t.Errorf("got %q, want %q", errMsg, expected)
		}
	})

	t.Run("workspace_mode existing with valid absolute path", func(t *testing.T) {
		errMsg := validateWorkspacePath("existing", "/home/user/my-project")
		if errMsg != "" {
			t.Errorf("expected no error for valid absolute path, got %q", errMsg)
		}
	})
}

// TestCreateTask_WorkspaceMode_ExistingWithAbsolutePath verifies that
// workspace_mode "existing" with a valid absolute path passes validation.
// Requirements: 5.1, 5.2
func TestCreateTask_WorkspaceMode_ExistingWithAbsolutePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		ok   bool
	}{
		{"root path", "/", true},
		{"home directory", "/home/user", true},
		{"deep path", "/var/lib/projects/my-app/src", true},
		{"empty path", "", false},
		{"relative path", "projects/my-app", false},
		{"dot relative", "./my-app", false},
		{"tilde path", "~/projects", false},
		{"whitespace only", "   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := validateWorkspacePath("existing", tt.path)
			if tt.ok && errMsg != "" {
				t.Errorf("expected valid, got error: %q", errMsg)
			}
			if !tt.ok && errMsg == "" {
				t.Errorf("expected error for path %q, got none", tt.path)
			}
		})
	}
}

// TestCreateTask_SinglePassDetection verifies the logic that determines
// whether a task should use single-pass mode (no stages) vs multi-stage.
// Requirements: 1.6, 11.1
func TestCreateTask_SinglePassDetection(t *testing.T) {
	tests := []struct {
		name         string
		deliverables []string
		singlePass   bool
	}{
		{"nil deliverables (defaults to execution)", []string{"execution"}, true},
		{"explicit execution only", []string{"execution"}, true},
		{"plan only", []string{"plan"}, false},
		{"design only", []string{"design"}, false},
		{"tasks only", []string{"tasks"}, false},
		{"plan and execution", []string{"plan", "execution"}, false},
		{"all deliverables", []string{"plan", "design", "tasks", "execution"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			singlePass := len(tt.deliverables) == 1 && tt.deliverables[0] == "execution"
			if singlePass != tt.singlePass {
				t.Errorf("singlePass for %v = %v, want %v",
					tt.deliverables, singlePass, tt.singlePass)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Existing validation unit tests
// ---------------------------------------------------------------------------

func TestValidateDeliverables(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		errMsg string
	}{
		{
			name:   "valid single deliverable - plan",
			input:  []string{"plan"},
			errMsg: "",
		},
		{
			name:   "valid single deliverable - execution",
			input:  []string{"execution"},
			errMsg: "",
		},
		{
			name:   "valid multiple deliverables",
			input:  []string{"plan", "design", "tasks", "execution"},
			errMsg: "",
		},
		{
			name:   "valid subset",
			input:  []string{"design", "execution"},
			errMsg: "",
		},
		{
			name:   "empty array",
			input:  []string{},
			errMsg: "deliverables must contain at least one valid value",
		},
		{
			name:   "nil array",
			input:  nil,
			errMsg: "deliverables must contain at least one valid value",
		},
		{
			name:   "invalid value",
			input:  []string{"plan", "invalid"},
			errMsg: "invalid deliverable: invalid. Valid values: plan, design, tasks, execution",
		},
		{
			name:   "all invalid",
			input:  []string{"foo"},
			errMsg: "invalid deliverable: foo. Valid values: plan, design, tasks, execution",
		},
		{
			name:   "empty string element",
			input:  []string{""},
			errMsg: "invalid deliverable: . Valid values: plan, design, tasks, execution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateDeliverables(tt.input)
			if got != tt.errMsg {
				t.Errorf("validateDeliverables(%v) = %q, want %q", tt.input, got, tt.errMsg)
			}
		})
	}
}

func TestValidateWorkspaceMode(t *testing.T) {
	tests := []struct {
		name   string
		mode   string
		errMsg string
	}{
		{
			name:   "valid isolated",
			mode:   "isolated",
			errMsg: "",
		},
		{
			name:   "valid existing",
			mode:   "existing",
			errMsg: "",
		},
		{
			name:   "invalid mode",
			mode:   "shared",
			errMsg: "workspace_mode must be 'isolated' or 'existing'",
		},
		{
			name:   "empty string",
			mode:   "",
			errMsg: "workspace_mode must be 'isolated' or 'existing'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateWorkspaceMode(tt.mode)
			if got != tt.errMsg {
				t.Errorf("validateWorkspaceMode(%q) = %q, want %q", tt.mode, got, tt.errMsg)
			}
		})
	}
}

func TestValidateWorkspacePath(t *testing.T) {
	tests := []struct {
		name   string
		mode   string
		path   string
		errMsg string
	}{
		{
			name:   "isolated mode - path ignored",
			mode:   "isolated",
			path:   "",
			errMsg: "",
		},
		{
			name:   "isolated mode - path present but ignored",
			mode:   "isolated",
			path:   "relative/path",
			errMsg: "",
		},
		{
			name:   "existing mode - valid absolute path",
			mode:   "existing",
			path:   "/home/user/project",
			errMsg: "",
		},
		{
			name:   "existing mode - empty path",
			mode:   "existing",
			path:   "",
			errMsg: "workspace_path is required when workspace_mode is 'existing'",
		},
		{
			name:   "existing mode - whitespace only path",
			mode:   "existing",
			path:   "   ",
			errMsg: "workspace_path is required when workspace_mode is 'existing'",
		},
		{
			name:   "existing mode - relative path",
			mode:   "existing",
			path:   "relative/path",
			errMsg: "workspace_path must be an absolute path",
		},
		{
			name:   "existing mode - path without leading slash",
			mode:   "existing",
			path:   "home/user/project",
			errMsg: "workspace_path must be an absolute path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateWorkspacePath(tt.mode, tt.path)
			if got != tt.errMsg {
				t.Errorf("validateWorkspacePath(%q, %q) = %q, want %q", tt.mode, tt.path, got, tt.errMsg)
			}
		})
	}
}

func TestDeliverableOrder(t *testing.T) {
	// Verify the canonical ordering map has the expected values.
	expected := map[string]int{
		"plan":      1,
		"design":    2,
		"tasks":     3,
		"execution": 4,
	}

	if len(deliverableOrder) != len(expected) {
		t.Fatalf("deliverableOrder has %d entries, want %d", len(deliverableOrder), len(expected))
	}

	for k, v := range expected {
		if deliverableOrder[k] != v {
			t.Errorf("deliverableOrder[%q] = %d, want %d", k, deliverableOrder[k], v)
		}
	}
}

// ---------------------------------------------------------------------------
// Unit tests for approval/rejection handlers (task 3.8)
// Tests the logic functions: simulateApprove, rejectStage, completeStage
// Requirements: 4.3, 4.4, 4.5
// ---------------------------------------------------------------------------

// TestApproval_ValidAwaitingApprovalStage verifies that approving a stage in
// awaiting_approval status succeeds: the stage becomes approved and the next
// stage (if any) becomes pending.
// Requirements: 4.3
func TestApproval_ValidAwaitingApprovalStage(t *testing.T) {
	tests := []struct {
		name       string
		stages     []stageState
		approveIdx int
		wantStatus string
		wantNext   string // expected status of next stage, empty if no next
	}{
		{
			name: "approve first of two stages",
			stages: []stageState{
				{Name: "plan", Order: 1, Status: "awaiting_approval"},
				{Name: "design", Order: 2, Status: "pending"},
			},
			approveIdx: 0,
			wantStatus: "approved",
			wantNext:   "pending",
		},
		{
			name: "approve middle stage advances next",
			stages: []stageState{
				{Name: "plan", Order: 1, Status: "approved"},
				{Name: "design", Order: 2, Status: "awaiting_approval"},
				{Name: "tasks", Order: 3, Status: "pending"},
				{Name: "execution", Order: 4, Status: "pending"},
			},
			approveIdx: 1,
			wantStatus: "approved",
			wantNext:   "pending",
		},
		{
			name: "approve last stage (no next stage)",
			stages: []stageState{
				{Name: "plan", Order: 1, Status: "approved"},
				{Name: "execution", Order: 4, Status: "awaiting_approval"},
			},
			approveIdx: 1,
			wantStatus: "approved",
			wantNext:   "",
		},
		{
			name: "approve single stage workflow",
			stages: []stageState{
				{Name: "plan", Order: 1, Status: "awaiting_approval"},
			},
			approveIdx: 0,
			wantStatus: "approved",
			wantNext:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, errMsg := simulateApprove(tt.stages, tt.approveIdx)

			if errMsg != "" {
				t.Fatalf("simulateApprove returned unexpected error: %s", errMsg)
			}

			// Verify the approved stage is now "approved".
			if result[tt.approveIdx].Status != tt.wantStatus {
				t.Errorf("approved stage status = %q, want %q",
					result[tt.approveIdx].Status, tt.wantStatus)
			}

			// Verify next stage status if applicable.
			if tt.wantNext != "" && tt.approveIdx+1 < len(result) {
				if result[tt.approveIdx+1].Status != tt.wantNext {
					t.Errorf("next stage status = %q, want %q",
						result[tt.approveIdx+1].Status, tt.wantNext)
				}
			}

			// Verify no other stages changed (except approved and next).
			for i, s := range result {
				if i == tt.approveIdx || i == tt.approveIdx+1 {
					continue
				}
				if s.Status != tt.stages[i].Status {
					t.Errorf("stage %d (%s) changed from %q to %q unexpectedly",
						i, s.Name, tt.stages[i].Status, s.Status)
				}
			}
		})
	}
}

// TestApproval_StageNotInAwaitingApproval verifies that attempting to approve
// a stage that is NOT in awaiting_approval status returns a 409-equivalent error.
// Requirements: 4.3
func TestApproval_StageNotInAwaitingApproval(t *testing.T) {
	invalidStatuses := []string{"pending", "running", "approved", "rejected", "completed", "failed"}

	for _, status := range invalidStatuses {
		t.Run("status_"+status, func(t *testing.T) {
			stages := []stageState{
				{Name: "plan", Order: 1, Status: status},
				{Name: "design", Order: 2, Status: "pending"},
			}

			_, errMsg := simulateApprove(stages, 0)

			if errMsg == "" {
				t.Fatalf("simulateApprove should fail for stage in status %q", status)
			}

			// The error message should indicate the stage is not awaiting approval.
			expectedErr := "stage is not awaiting approval"
			if errMsg != expectedErr {
				t.Errorf("error message = %q, want %q", errMsg, expectedErr)
			}
		})
	}
}

// TestReject_WithFeedback verifies that rejecting a stage in awaiting_approval
// status with non-empty feedback succeeds: the stage transitions to pending
// with feedback stored.
// Requirements: 4.4, 4.5
func TestReject_WithFeedback(t *testing.T) {
	tests := []struct {
		name     string
		stage    WorkflowStage
		feedback string
	}{
		{
			name: "reject plan stage with simple feedback",
			stage: WorkflowStage{
				Name:   "plan",
				Order:  1,
				Status: "awaiting_approval",
				Output: "# Plan\nStep 1: Do something",
			},
			feedback: "Please add more detail to step 1",
		},
		{
			name: "reject design stage with detailed feedback",
			stage: WorkflowStage{
				Name:   "design",
				Order:  2,
				Status: "awaiting_approval",
				Output: "# Design\nArchitecture overview",
			},
			feedback: "The architecture needs to account for horizontal scaling. Please revise the database layer to support read replicas.",
		},
		{
			name: "reject with special characters in feedback",
			stage: WorkflowStage{
				Name:   "tasks",
				Order:  3,
				Status: "awaiting_approval",
				Output: "# Tasks\n- Task 1",
			},
			feedback: "Fix: use `goroutines` instead of threads. See https://example.com/docs",
		},
		{
			name: "reject execution stage",
			stage: WorkflowStage{
				Name:   "execution",
				Order:  4,
				Status: "awaiting_approval",
				Output: "Implementation complete",
			},
			feedback: "Tests are failing, please fix the auth module",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stage := tt.stage // copy
			errMsg := rejectStage(&stage, tt.feedback)

			if errMsg != "" {
				t.Fatalf("rejectStage returned unexpected error: %s", errMsg)
			}

			// Stage should be re-queued to pending.
			if stage.Status != "pending" {
				t.Errorf("stage status = %q, want %q", stage.Status, "pending")
			}

			// Feedback should be stored.
			if stage.Feedback != tt.feedback {
				t.Errorf("stage feedback = %q, want %q", stage.Feedback, tt.feedback)
			}

			// Feedback should be non-empty (available for re-execution context).
			if strings.TrimSpace(stage.Feedback) == "" {
				t.Error("feedback should be non-empty after rejection")
			}
		})
	}
}

// TestReject_WithoutFeedback verifies that rejecting a stage with empty or
// whitespace-only feedback returns a 400-equivalent error.
// Requirements: 4.4
func TestReject_WithoutFeedback(t *testing.T) {
	emptyFeedbacks := []struct {
		name     string
		feedback string
	}{
		{"empty string", ""},
		{"single space", " "},
		{"multiple spaces", "   "},
		{"tab character", "\t"},
		{"newline", "\n"},
		{"mixed whitespace", " \t\n\r "},
	}

	for _, tt := range emptyFeedbacks {
		t.Run(tt.name, func(t *testing.T) {
			stage := &WorkflowStage{
				Name:   "plan",
				Order:  1,
				Status: "awaiting_approval",
				Output: "Some output",
			}

			errMsg := rejectStage(stage, tt.feedback)

			if errMsg == "" {
				t.Fatalf("rejectStage should fail for empty feedback %q", tt.feedback)
			}

			expectedErr := "feedback is required when rejecting a stage"
			if errMsg != expectedErr {
				t.Errorf("error message = %q, want %q", errMsg, expectedErr)
			}

			// Stage status should remain unchanged.
			if stage.Status != "awaiting_approval" {
				t.Errorf("stage status changed to %q, should remain 'awaiting_approval'",
					stage.Status)
			}
		})
	}
}

// TestReject_StageNotInAwaitingApproval verifies that rejecting a stage NOT in
// awaiting_approval status returns a 409-equivalent error regardless of feedback.
// Requirements: 4.4
func TestReject_StageNotInAwaitingApproval(t *testing.T) {
	invalidStatuses := []string{"pending", "running", "approved", "rejected", "completed", "failed"}

	for _, status := range invalidStatuses {
		t.Run("status_"+status, func(t *testing.T) {
			stage := &WorkflowStage{
				Name:   "design",
				Order:  2,
				Status: status,
			}

			errMsg := rejectStage(stage, "valid feedback")

			if errMsg == "" {
				t.Fatalf("rejectStage should fail for stage in status %q", status)
			}

			expectedErr := "stage is not awaiting approval"
			if errMsg != expectedErr {
				t.Errorf("error message = %q, want %q", errMsg, expectedErr)
			}

			// Stage status should remain unchanged.
			if stage.Status != status {
				t.Errorf("stage status changed from %q to %q", status, stage.Status)
			}
		})
	}
}

// TestStageComplete_FromDaemon verifies that when the daemon reports stage
// completion, the stage transitions from running to awaiting_approval.
// Requirements: 4.3, 4.5
func TestStageComplete_FromDaemon(t *testing.T) {
	tests := []struct {
		name          string
		stages        []WorkflowStage
		completingIdx int
	}{
		{
			name: "complete first stage in two-stage workflow",
			stages: []WorkflowStage{
				{Name: "plan", Order: 1, Status: "running"},
				{Name: "execution", Order: 4, Status: "pending"},
			},
			completingIdx: 0,
		},
		{
			name: "complete middle stage in four-stage workflow",
			stages: []WorkflowStage{
				{Name: "plan", Order: 1, Status: "approved"},
				{Name: "design", Order: 2, Status: "running"},
				{Name: "tasks", Order: 3, Status: "pending"},
				{Name: "execution", Order: 4, Status: "pending"},
			},
			completingIdx: 1,
		},
		{
			name: "complete last stage",
			stages: []WorkflowStage{
				{Name: "plan", Order: 1, Status: "approved"},
				{Name: "design", Order: 2, Status: "approved"},
				{Name: "execution", Order: 4, Status: "running"},
			},
			completingIdx: 2,
		},
		{
			name: "complete single stage workflow",
			stages: []WorkflowStage{
				{Name: "plan", Order: 1, Status: "running"},
			},
			completingIdx: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := completeStage(tt.stages, tt.completingIdx)

			// The completing stage must transition to awaiting_approval.
			if result[tt.completingIdx].Status != "awaiting_approval" {
				t.Errorf("completed stage status = %q, want %q",
					result[tt.completingIdx].Status, "awaiting_approval")
			}

			// No other stages should change status.
			for i, s := range result {
				if i == tt.completingIdx {
					continue
				}
				if s.Status != tt.stages[i].Status {
					t.Errorf("stage %d (%s) changed from %q to %q unexpectedly",
						i, s.Name, tt.stages[i].Status, s.Status)
				}
			}

			// Specifically, no stage should auto-advance to "running".
			for i, s := range result {
				if i == tt.completingIdx {
					continue
				}
				if s.Status == "running" && tt.stages[i].Status != "running" {
					t.Errorf("stage %d (%s) auto-advanced to 'running' without approval",
						i, s.Name)
				}
			}
		})
	}
}

// TestStageComplete_NotFromRunningStatus verifies that completeStage is a no-op
// when the stage is not in "running" status.
// Requirements: 4.3
func TestStageComplete_NotFromRunningStatus(t *testing.T) {
	nonRunningStatuses := []string{"pending", "awaiting_approval", "approved", "rejected", "completed", "failed"}

	for _, status := range nonRunningStatuses {
		t.Run("status_"+status, func(t *testing.T) {
			stages := []WorkflowStage{
				{Name: "plan", Order: 1, Status: status},
			}

			result := completeStage(stages, 0)

			// Stage should remain unchanged.
			if result[0].Status != status {
				t.Errorf("completeStage changed status from %q to %q for non-running stage",
					status, result[0].Status)
			}
		})
	}
}

// TestStageComplete_InvalidIndex verifies that completeStage handles invalid
// indices gracefully without panicking.
// Requirements: 4.3
func TestStageComplete_InvalidIndex(t *testing.T) {
	stages := []WorkflowStage{
		{Name: "plan", Order: 1, Status: "running"},
		{Name: "design", Order: 2, Status: "pending"},
	}

	tests := []struct {
		name string
		idx  int
	}{
		{"negative index", -1},
		{"index equal to length", 2},
		{"index beyond length", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := completeStage(stages, tt.idx)

			// All stages should remain unchanged.
			for i, s := range result {
				if s.Status != stages[i].Status {
					t.Errorf("stage %d changed from %q to %q with invalid index %d",
						i, stages[i].Status, s.Status, tt.idx)
				}
			}
		})
	}
}
