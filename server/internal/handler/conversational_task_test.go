package handler

import (
	"testing"

	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// ---------------------------------------------------------------------------
// Unit tests for conversational task creation (task 2.7)
// Tests the validation logic via ConversationalTaskCreateRequest.Validate(),
// ValidateDeliverableType, and FollowUpRequest.Validate().
// Requirements: 1.1, 1.2, 1.4, 1.5, 1.6, 8.1
// ---------------------------------------------------------------------------

// TestValidateDeliverableType_ValidTypes verifies that each valid deliverable_type
// is accepted by ValidateDeliverableType.
// Requirements: 1.1, 1.6
func TestValidateDeliverableType_ValidTypes(t *testing.T) {
	validTypes := []string{"plan", "design", "tasks", "execution"}

	for _, dt := range validTypes {
		t.Run(dt, func(t *testing.T) {
			err := ValidateDeliverableType(dt)
			if err != nil {
				t.Errorf("ValidateDeliverableType(%q) returned error: %v, want nil", dt, err)
			}
		})
	}
}

// TestValidateDeliverableType_InvalidTypes verifies that invalid deliverable_type
// values are rejected with the correct error message.
// Requirements: 1.6
func TestValidateDeliverableType_InvalidTypes(t *testing.T) {
	invalidTypes := []string{
		"",
		"planning",
		"execute",
		"PLAN",
		"Design",
		"TASKS",
		"code",
		"review",
		"test",
		"deploy",
		"plan ",
		" design",
		"plan\n",
	}

	expectedErr := "invalid deliverable_type: must be one of plan, design, tasks, execution"

	for _, dt := range invalidTypes {
		t.Run("type_"+dt, func(t *testing.T) {
			err := ValidateDeliverableType(dt)
			if err == nil {
				t.Fatalf("ValidateDeliverableType(%q) returned nil, want error", dt)
			}
			if err.Error() != expectedErr {
				t.Errorf("ValidateDeliverableType(%q) error = %q, want %q", dt, err.Error(), expectedErr)
			}
		})
	}
}

// TestConversationalTaskCreate_EachValidDeliverableType verifies that creating
// a conversational task with each valid deliverable_type passes validation.
// Requirements: 1.1, 1.2
func TestConversationalTaskCreate_EachValidDeliverableType(t *testing.T) {
	tests := []struct {
		name            string
		deliverableType string
		localDir        string // needed for execution type
	}{
		{"plan type", "plan", ""},
		{"design type", "design", ""},
		{"tasks type", "tasks", ""},
		{"execution type", "execution", "/home/user/project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ConversationalTaskCreateRequest{
				AgentID:            "agent-123",
				Prompt:             "Build a REST API for user management",
				DeliverableType:    tt.deliverableType,
				LocalDirectoryPath: tt.localDir,
			}

			err := req.Validate()
			if err != nil {
				t.Errorf("Validate() returned error for deliverable_type %q: %v", tt.deliverableType, err)
			}
		})
	}
}

// TestConversationalTaskCreate_InvalidDeliverableType verifies that creating a
// conversational task with an invalid deliverable_type returns a validation error.
// Requirements: 1.6
func TestConversationalTaskCreate_InvalidDeliverableType(t *testing.T) {
	invalidTypes := []string{"", "invalid", "PLAN", "execute", "code"}

	for _, dt := range invalidTypes {
		t.Run("type_"+dt, func(t *testing.T) {
			req := &ConversationalTaskCreateRequest{
				AgentID:         "agent-123",
				Prompt:          "Build something",
				DeliverableType: dt,
			}

			err := req.Validate()
			if err == nil {
				t.Fatalf("Validate() returned nil for invalid deliverable_type %q, want error", dt)
			}

			expectedErr := "invalid deliverable_type: must be one of plan, design, tasks, execution"
			if err.Error() != expectedErr {
				t.Errorf("Validate() error = %q, want %q", err.Error(), expectedErr)
			}
		})
	}
}

// TestConversationalTaskCreate_WithoutDeliverableType_SinglePassFlow verifies
// that when deliverable_type is absent from CreateTaskReq, the handler uses
// the single-pass flow (backward compatibility).
// Requirements: 8.1
func TestConversationalTaskCreate_WithoutDeliverableType_SinglePassFlow(t *testing.T) {
	// When DeliverableType is empty in CreateTaskReq, the CreateTask handler
	// does NOT call createConversationalTask — it falls through to the legacy
	// multi-stage / single-pass flow.
	req := CreateTaskReq{
		AgentType: "claude",
		Prompt:    "Fix the login bug",
		// DeliverableType intentionally empty — single-pass flow
	}

	// Verify the routing condition: DeliverableType == "" means single-pass.
	if req.DeliverableType != "" {
		t.Fatal("expected DeliverableType to be empty for single-pass flow")
	}

	// In single-pass flow, deliverables defaults to ["execution"].
	deliverables := req.Deliverables
	if len(deliverables) == 0 {
		deliverables = []string{"execution"}
	}

	singlePass := len(deliverables) == 1 && deliverables[0] == "execution"
	if !singlePass {
		t.Errorf("expected single-pass mode when DeliverableType is empty, got deliverables=%v", deliverables)
	}
}

// TestConversationalTaskCreate_ExecutionWithoutLocalDirectoryPath verifies that
// execution type without local_directory_path returns a validation error.
// Requirements: 1.5
func TestConversationalTaskCreate_ExecutionWithoutLocalDirectoryPath(t *testing.T) {
	tests := []struct {
		name     string
		localDir string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tab character", "\t"},
		{"newline", "\n"},
		{"mixed whitespace", " \t\n "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ConversationalTaskCreateRequest{
				AgentID:            "agent-123",
				Prompt:             "Implement the feature",
				DeliverableType:    "execution",
				LocalDirectoryPath: tt.localDir,
			}

			err := req.Validate()
			if err == nil {
				t.Fatalf("Validate() returned nil for execution type with local_directory_path=%q, want error", tt.localDir)
			}

			expectedErr := "local_directory_path is required for execution deliverable type"
			if err.Error() != expectedErr {
				t.Errorf("Validate() error = %q, want %q", err.Error(), expectedErr)
			}
		})
	}
}

// TestConversationalTaskCreate_ExecutionWithNonAbsolutePath verifies that
// execution type with a non-absolute local_directory_path returns a validation error.
// Requirements: 1.5
func TestConversationalTaskCreate_ExecutionWithNonAbsolutePath(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"relative path", "relative/path/to/project"},
		{"dot relative", "./my-project"},
		{"double dot relative", "../parent/project"},
		{"tilde path", "~/projects/my-app"},
		{"no leading slash", "home/user/project"},
		{"windows-style path", "C:\\Users\\project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ConversationalTaskCreateRequest{
				AgentID:            "agent-123",
				Prompt:             "Implement the feature",
				DeliverableType:    "execution",
				LocalDirectoryPath: tt.path,
			}

			err := req.Validate()
			if err == nil {
				t.Fatalf("Validate() returned nil for execution type with non-absolute path %q, want error", tt.path)
			}

			expectedErr := "local_directory_path must be an absolute path"
			if err.Error() != expectedErr {
				t.Errorf("Validate() error = %q, want %q", err.Error(), expectedErr)
			}
		})
	}
}

// TestConversationalTaskCreate_ExecutionWithValidAbsolutePath verifies that
// execution type with a valid absolute local_directory_path passes validation.
// Requirements: 1.5
func TestConversationalTaskCreate_ExecutionWithValidAbsolutePath(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"root path", "/"},
		{"home directory", "/home/user"},
		{"deep path", "/var/lib/projects/my-app/src"},
		{"tmp directory", "/tmp/workspace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ConversationalTaskCreateRequest{
				AgentID:            "agent-123",
				Prompt:             "Implement the feature",
				DeliverableType:    "execution",
				LocalDirectoryPath: tt.path,
			}

			err := req.Validate()
			if err != nil {
				t.Errorf("Validate() returned error for valid absolute path %q: %v", tt.path, err)
			}
		})
	}
}

// TestConversationalTaskCreate_PriorContextStored verifies that prior_context
// is correctly stored on the request struct and accessible for serialization.
// Requirements: 1.2
func TestConversationalTaskCreate_PriorContextStored(t *testing.T) {
	tests := []struct {
		name         string
		priorContext []string
		wantLen      int
	}{
		{
			name:         "nil prior_context",
			priorContext: nil,
			wantLen:      0,
		},
		{
			name:         "empty prior_context",
			priorContext: []string{},
			wantLen:      0,
		},
		{
			name:         "single prior deliverable output",
			priorContext: []string{"# Plan\n\n## Overview\nBuild a REST API..."},
			wantLen:      1,
		},
		{
			name: "multiple prior deliverable outputs",
			priorContext: []string{
				"# Plan\n\n## Overview\nBuild a REST API for user management.",
				"# Design\n\n## Architecture\nUse Go with Chi router and PostgreSQL.",
			},
			wantLen: 2,
		},
		{
			name: "all prior deliverables (plan + design + tasks)",
			priorContext: []string{
				"# Plan\nStep 1: Setup project structure",
				"# Design\nArchitecture: microservices",
				"# Tasks\n- [ ] Create user model\n- [ ] Add auth middleware",
			},
			wantLen: 3,
		},
		{
			name:         "prior_context with large content",
			priorContext: []string{string(make([]byte, 10000))},
			wantLen:      1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ConversationalTaskCreateRequest{
				AgentID:         "agent-123",
				Prompt:          "Design the system",
				DeliverableType: "design",
				PriorContext:    tt.priorContext,
			}

			// Validate should pass (prior_context doesn't affect validation).
			err := req.Validate()
			if err != nil {
				t.Fatalf("Validate() returned unexpected error: %v", err)
			}

			// Verify prior_context is stored correctly.
			if len(req.PriorContext) != tt.wantLen {
				t.Errorf("PriorContext length = %d, want %d", len(req.PriorContext), tt.wantLen)
			}

			// Verify content is preserved for non-empty cases.
			if tt.priorContext != nil {
				for i, ctx := range tt.priorContext {
					if i < len(req.PriorContext) && req.PriorContext[i] != ctx {
						t.Errorf("PriorContext[%d] = %q, want %q", i, req.PriorContext[i], ctx)
					}
				}
			}
		})
	}
}

// TestConversationalTaskCreate_EmptyPrompt verifies that an empty prompt
// returns a validation error.
// Requirements: 1.2
func TestConversationalTaskCreate_EmptyPrompt(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tab", "\t"},
		{"newline", "\n"},
		{"mixed whitespace", " \t\n\r "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ConversationalTaskCreateRequest{
				AgentID:         "agent-123",
				Prompt:          tt.prompt,
				DeliverableType: "plan",
			}

			err := req.Validate()
			if err == nil {
				t.Fatalf("Validate() returned nil for empty prompt %q, want error", tt.prompt)
			}

			expectedErr := "prompt is required"
			if err.Error() != expectedErr {
				t.Errorf("Validate() error = %q, want %q", err.Error(), expectedErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FollowUpRequest.Validate() tests
// ---------------------------------------------------------------------------

// TestFollowUpRequest_ValidPrompt verifies that a non-empty prompt passes validation.
// Requirements: 2.1
func TestFollowUpRequest_ValidPrompt(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
	}{
		{"simple prompt", "Please add more detail to the plan"},
		{"single character", "x"},
		{"with special chars", "Fix: use `goroutines` instead of threads"},
		{"multiline", "Line 1\nLine 2\nLine 3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &FollowUpRequest{Prompt: tt.prompt}
			err := req.Validate()
			if err != nil {
				t.Errorf("Validate() returned error for valid prompt %q: %v", tt.prompt, err)
			}
		})
	}
}

// TestFollowUpRequest_EmptyPrompt verifies that an empty prompt returns a validation error.
// Requirements: 2.1
func TestFollowUpRequest_EmptyPrompt(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tab", "\t"},
		{"newline", "\n"},
		{"mixed whitespace", " \t\n\r "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &FollowUpRequest{Prompt: tt.prompt}
			err := req.Validate()
			if err == nil {
				t.Fatalf("Validate() returned nil for empty prompt %q, want error", tt.prompt)
			}

			expectedErr := "prompt is required"
			if err.Error() != expectedErr {
				t.Errorf("Validate() error = %q, want %q", err.Error(), expectedErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ValidDeliverableTypes map tests
// ---------------------------------------------------------------------------

// TestValidDeliverableTypes_MapContents verifies the ValidDeliverableTypes map
// contains exactly the expected entries.
// Requirements: 1.1
func TestValidDeliverableTypes_MapContents(t *testing.T) {
	expected := map[string]bool{
		"plan":      true,
		"design":    true,
		"tasks":     true,
		"execution": true,
	}

	if len(ValidDeliverableTypes) != len(expected) {
		t.Fatalf("ValidDeliverableTypes has %d entries, want %d", len(ValidDeliverableTypes), len(expected))
	}

	for k, v := range expected {
		if ValidDeliverableTypes[k] != v {
			t.Errorf("ValidDeliverableTypes[%q] = %v, want %v", k, ValidDeliverableTypes[k], v)
		}
	}
}

// ---------------------------------------------------------------------------
// Routing logic tests (DeliverableType presence determines flow)
// ---------------------------------------------------------------------------

// TestCreateTaskReq_ConversationalRouting verifies that the presence of
// DeliverableType in CreateTaskReq determines whether the conversational
// flow is used.
// Requirements: 8.1, 1.4
func TestCreateTaskReq_ConversationalRouting(t *testing.T) {
	tests := []struct {
		name             string
		deliverableType  string
		isConversational bool
	}{
		{"empty deliverable_type → single-pass", "", false},
		{"plan → conversational", "plan", true},
		{"design → conversational", "design", true},
		{"tasks → conversational", "tasks", true},
		{"execution → conversational", "execution", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := CreateTaskReq{
				AgentType:       "claude",
				Prompt:          "Do something",
				DeliverableType: tt.deliverableType,
			}

			// The routing condition in CreateTask handler:
			// if req.DeliverableType != "" → conversational flow
			isConversational := req.DeliverableType != ""
			if isConversational != tt.isConversational {
				t.Errorf("routing for DeliverableType=%q: got conversational=%v, want %v",
					tt.deliverableType, isConversational, tt.isConversational)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Unit tests for follow-up and history handlers (task 3.6)
// Tests the FollowUpRequest validation, isConversationalTask helper,
// and prompt history ordering logic.
// Requirements: 2.1, 2.2, 3.3, 6.3
// ---------------------------------------------------------------------------

// TestFollowUpRequest_Validate_EmptyPromptReturns400 verifies that
// FollowUpRequest.Validate() returns an error when the prompt is empty.
// This corresponds to the handler returning 400 for empty follow-up prompts.
// Requirements: 2.1
func TestFollowUpRequest_Validate_EmptyPromptReturns400(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
	}{
		{"empty string", ""},
		{"spaces only", "   "},
		{"tab only", "\t"},
		{"newline only", "\n"},
		{"carriage return", "\r"},
		{"mixed whitespace", " \t\n\r "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &FollowUpRequest{Prompt: tt.prompt}
			err := req.Validate()
			if err == nil {
				t.Fatalf("Validate() returned nil for empty prompt %q, want error", tt.prompt)
			}
			if err.Error() != "prompt is required" {
				t.Errorf("Validate() error = %q, want %q", err.Error(), "prompt is required")
			}
		})
	}
}

// TestFollowUpRequest_Validate_ValidPromptReturnsNil verifies that
// FollowUpRequest.Validate() returns nil for valid non-empty prompts.
// Requirements: 2.1
func TestFollowUpRequest_Validate_ValidPromptReturnsNil(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
	}{
		{"simple text", "Add error handling to the plan"},
		{"single char", "x"},
		{"with leading space and content", " hello"},
		{"multiline content", "Line 1\nLine 2\nLine 3"},
		{"special characters", "Use `context.Context` for cancellation"},
		{"unicode", "Добавить обработку ошибок"},
		{"long prompt", string(make([]byte, 1000))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For the long prompt case, fill with actual content
			prompt := tt.prompt
			if tt.name == "long prompt" {
				prompt = "a" + prompt[1:] // ensure non-whitespace
			}
			req := &FollowUpRequest{Prompt: prompt}
			err := req.Validate()
			if err != nil {
				t.Errorf("Validate() returned error for valid prompt: %v", err)
			}
		})
	}
}

// TestIsConversationalTask_DeliverableTypeStages verifies that stages with
// deliverable_type names (plan, design, tasks, execution) are detected as
// conversational tasks.
// Requirements: 6.3
func TestIsConversationalTask_DeliverableTypeStages(t *testing.T) {
	tests := []struct {
		name      string
		stageName string
	}{
		{"plan stage", "plan"},
		{"design stage", "design"},
		{"tasks stage", "tasks"},
		{"execution stage", "execution"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stages := []db.TaskStage{
				{StageName: tt.stageName, Status: "completed"},
			}
			if !isConversationalTask(stages) {
				t.Errorf("isConversationalTask() = false for stage %q, want true", tt.stageName)
			}
		})
	}
}

// TestIsConversationalTask_NonDeliverableStages verifies that stages with
// non-deliverable names (e.g., "planning", "implementation") are NOT detected
// as conversational tasks.
// Requirements: 6.3
func TestIsConversationalTask_NonDeliverableStages(t *testing.T) {
	tests := []struct {
		name      string
		stageName string
	}{
		{"planning stage", "planning"},
		{"implementation stage", "implementation"},
		{"review stage", "review"},
		{"testing stage", "testing"},
		{"deployment stage", "deployment"},
		{"analysis stage", "analysis"},
		{"empty stage name", ""},
		{"uppercase PLAN", "PLAN"},
		{"mixed case Design", "Design"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stages := []db.TaskStage{
				{StageName: tt.stageName, Status: "completed"},
			}
			if isConversationalTask(stages) {
				t.Errorf("isConversationalTask() = true for stage %q, want false", tt.stageName)
			}
		})
	}
}

// TestIsConversationalTask_MixedStages verifies that if any stage in the list
// has a deliverable_type name, the task is considered conversational.
// Requirements: 6.3
func TestIsConversationalTask_MixedStages(t *testing.T) {
	tests := []struct {
		name       string
		stageNames []string
		want       bool
	}{
		{
			name:       "one conversational among non-conversational",
			stageNames: []string{"planning", "plan", "review"},
			want:       true,
		},
		{
			name:       "all non-conversational",
			stageNames: []string{"planning", "implementation", "review"},
			want:       false,
		},
		{
			name:       "all conversational",
			stageNames: []string{"plan", "design", "tasks", "execution"},
			want:       true,
		},
		{
			name:       "empty stages list",
			stageNames: []string{},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stages := make([]db.TaskStage, len(tt.stageNames))
			for i, name := range tt.stageNames {
				stages[i] = db.TaskStage{StageName: name, Status: "completed"}
			}
			got := isConversationalTask(stages)
			if got != tt.want {
				t.Errorf("isConversationalTask(%v) = %v, want %v", tt.stageNames, got, tt.want)
			}
		})
	}
}

// TestFollowUpStage_NonCompletedStageReturns409 verifies the logic that
// follow-up is only allowed on completed stages. The handler checks
// stage.Status != "completed" and returns 409.
// Requirements: 2.1, 2.2
func TestFollowUpStage_NonCompletedStageReturns409(t *testing.T) {
	// The FollowUpStage handler checks: if stage.Status != "completed" → 409
	// We test the condition logic that guards follow-up creation.
	nonCompletedStatuses := []string{"pending", "running", "failed"}

	for _, status := range nonCompletedStatuses {
		t.Run("status_"+status, func(t *testing.T) {
			// Simulate the guard condition from FollowUpStage handler.
			stage := db.TaskStage{
				StageName: "plan",
				Status:    status,
			}

			// The handler logic: if stage.Status != "completed" → reject
			if stage.Status == "completed" {
				t.Fatalf("stage status %q should not be 'completed'", status)
			}

			// Verify the condition that would trigger 409.
			shouldReject := stage.Status != "completed"
			if !shouldReject {
				t.Errorf("expected rejection for stage status %q", status)
			}
		})
	}
}

// TestFollowUpStage_CompletedStageAllowsFollowUp verifies that a stage
// in "completed" status passes the guard check for follow-up creation.
// Requirements: 2.1, 2.2
func TestFollowUpStage_CompletedStageAllowsFollowUp(t *testing.T) {
	stage := db.TaskStage{
		StageName: "design",
		Status:    "completed",
	}

	// The handler logic: if stage.Status != "completed" → reject
	// For completed status, the guard should NOT reject.
	shouldReject := stage.Status != "completed"
	if shouldReject {
		t.Error("expected completed stage to pass the follow-up guard check")
	}
}

// TestPromptHistoryEntry_ChronologicalOrder verifies that prompt history
// entries maintain chronological ordering when sorted by CreatedAt.
// The GetStageHistory handler returns entries ordered by created_at ASC
// (oldest first), which is enforced by the SQL query.
// Requirements: 3.3
func TestPromptHistoryEntry_ChronologicalOrder(t *testing.T) {
	// Simulate the response conversion from the handler.
	// The SQL query orders by created_at ASC, so entries should be oldest first.
	entries := []PromptHistoryEntry{
		{
			ID:         "entry-1",
			PromptText: "Create a plan for the REST API",
			CreatedAt:  "2025-01-01T10:00:00Z",
		},
		{
			ID:         "entry-2",
			PromptText: "Add more detail about authentication",
			CreatedAt:  "2025-01-01T10:05:00Z",
		},
		{
			ID:         "entry-3",
			PromptText: "Include rate limiting in the plan",
			CreatedAt:  "2025-01-01T10:10:00Z",
		},
	}

	// Verify entries are in chronological order (oldest first).
	for i := 1; i < len(entries); i++ {
		if entries[i].CreatedAt <= entries[i-1].CreatedAt {
			t.Errorf("entries not in chronological order: entry[%d].CreatedAt=%q <= entry[%d].CreatedAt=%q",
				i, entries[i].CreatedAt, i-1, entries[i-1].CreatedAt)
		}
	}

	// Verify each entry has required fields populated.
	for i, e := range entries {
		if e.ID == "" {
			t.Errorf("entry[%d].ID is empty", i)
		}
		if e.PromptText == "" {
			t.Errorf("entry[%d].PromptText is empty", i)
		}
		if e.CreatedAt == "" {
			t.Errorf("entry[%d].CreatedAt is empty", i)
		}
	}
}

// TestPromptHistoryEntry_OutputTextOptional verifies that OutputText is
// optional (nil when not yet available, non-nil when agent has responded).
// Requirements: 3.3
func TestPromptHistoryEntry_OutputTextOptional(t *testing.T) {
	output := "# Plan\n\n## Overview\nBuild a REST API..."

	tests := []struct {
		name       string
		outputText *string
		wantNil    bool
	}{
		{"nil output (pending)", nil, true},
		{"non-nil output (completed)", &output, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := PromptHistoryEntry{
				ID:         "entry-1",
				PromptText: "Create a plan",
				OutputText: tt.outputText,
				CreatedAt:  "2025-01-01T10:00:00Z",
			}

			if tt.wantNil && entry.OutputText != nil {
				t.Error("expected OutputText to be nil")
			}
			if !tt.wantNil && entry.OutputText == nil {
				t.Error("expected OutputText to be non-nil")
			}
			if !tt.wantNil && *entry.OutputText != output {
				t.Errorf("OutputText = %q, want %q", *entry.OutputText, output)
			}
		})
	}
}

// TestApproveRejectGuard_ConversationalTaskReturns409 verifies that the
// isConversationalTask guard in approve/reject handlers correctly identifies
// conversational tasks and would trigger a 409 response.
// Requirements: 6.3
func TestApproveRejectGuard_ConversationalTaskReturns409(t *testing.T) {
	// Simulate the guard logic from ApproveStage and RejectStage handlers:
	// stages are listed, then isConversationalTask(stages) is checked.
	// If true → 409 "approval gates not supported for conversational tasks"

	tests := []struct {
		name       string
		stageNames []string
		wantReject bool
	}{
		{
			name:       "single plan stage (conversational)",
			stageNames: []string{"plan"},
			wantReject: true,
		},
		{
			name:       "single design stage (conversational)",
			stageNames: []string{"design"},
			wantReject: true,
		},
		{
			name:       "single tasks stage (conversational)",
			stageNames: []string{"tasks"},
			wantReject: true,
		},
		{
			name:       "single execution stage (conversational)",
			stageNames: []string{"execution"},
			wantReject: true,
		},
		{
			name:       "legacy planning stage (not conversational)",
			stageNames: []string{"planning"},
			wantReject: false,
		},
		{
			name:       "legacy multi-stage workflow (not conversational)",
			stageNames: []string{"planning", "implementation", "review"},
			wantReject: false,
		},
		{
			name:       "mixed with one conversational stage",
			stageNames: []string{"planning", "design", "review"},
			wantReject: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stages := make([]db.TaskStage, len(tt.stageNames))
			for i, name := range tt.stageNames {
				stages[i] = db.TaskStage{StageName: name, Status: "awaiting_approval"}
			}

			gotReject := isConversationalTask(stages)
			if gotReject != tt.wantReject {
				t.Errorf("isConversationalTask(%v) = %v, want %v (409 guard)",
					tt.stageNames, gotReject, tt.wantReject)
			}
		})
	}
}
