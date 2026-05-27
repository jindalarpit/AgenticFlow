package handler

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// ---------------------------------------------------------------------------
// Unit tests for poll response construction logic (task 5.4)
// Requirements: 9.1, 9.4, 9.5
// ---------------------------------------------------------------------------

// TestPollResponse_StagedTask_IncludesCurrentStageAndPriorStages verifies that
// when a task has stages with one pending, the response includes current_stage
// with name/order/status, and prior_stages with completed stage outputs.
// Requirements: 9.1
func TestPollResponse_StagedTask_IncludesCurrentStageAndPriorStages(t *testing.T) {
	taskID := makeTestUUID(1)

	task := db.Task{
		ID:            taskID,
		AgentType:     "claude",
		Prompt:        "Build a REST API",
		Status:        "running",
		WorkspaceMode: "isolated",
	}

	// Build base response.
	response := buildPollBaseResponse(task)

	// Simulate the next pending stage (design, order 2).
	nextStage := db.TaskStage{
		ID:         makeTestUUID(10),
		TaskID:     taskID,
		StageName:  "design",
		StageOrder: 2,
		Status:     "pending",
	}

	// Simulate completed prior stages (plan, order 1 — approved with output).
	completedStages := []db.TaskStage{
		{
			ID:         makeTestUUID(11),
			TaskID:     taskID,
			StageName:  "plan",
			StageOrder: 1,
			Status:     "approved",
			OutputContent: pgtype.Text{
				String: "# Plan\n\nStep 1: Design the API schema\nStep 2: Implement endpoints",
				Valid:  true,
			},
		},
	}

	// Enrich response with stage fields.
	enrichResponseWithStageFields(response, nextStage, completedStages)

	// Verify current_stage is present with correct fields.
	currentStage, ok := response["current_stage"].(map[string]interface{})
	if !ok {
		t.Fatal("expected current_stage to be present in response")
	}
	if currentStage["name"] != "design" {
		t.Errorf("current_stage.name = %v, want %q", currentStage["name"], "design")
	}
	if currentStage["order"] != int32(2) {
		t.Errorf("current_stage.order = %v, want %d", currentStage["order"], 2)
	}
	if currentStage["status"] != "pending" {
		t.Errorf("current_stage.status = %v, want %q", currentStage["status"], "pending")
	}

	// Verify prior_stages is present with completed stage output.
	priorStages, ok := response["prior_stages"].([]map[string]interface{})
	if !ok {
		t.Fatal("expected prior_stages to be present in response")
	}
	if len(priorStages) != 1 {
		t.Fatalf("expected 1 prior stage, got %d", len(priorStages))
	}
	if priorStages[0]["name"] != "plan" {
		t.Errorf("prior_stages[0].name = %v, want %q", priorStages[0]["name"], "plan")
	}
	if priorStages[0]["order"] != int32(1) {
		t.Errorf("prior_stages[0].order = %v, want %d", priorStages[0]["order"], 1)
	}
	if priorStages[0]["status"] != "approved" {
		t.Errorf("prior_stages[0].status = %v, want %q", priorStages[0]["status"], "approved")
	}
	if priorStages[0]["output_content"] != "# Plan\n\nStep 1: Design the API schema\nStep 2: Implement endpoints" {
		t.Errorf("prior_stages[0].output_content = %v, want plan content", priorStages[0]["output_content"])
	}
}

// TestPollResponse_StagedTask_MultiplePriorStages verifies that when multiple
// stages are completed, all their outputs are included in prior_stages.
// Requirements: 9.1
func TestPollResponse_StagedTask_MultiplePriorStages(t *testing.T) {
	taskID := makeTestUUID(2)

	task := db.Task{
		ID:            taskID,
		AgentType:     "claude",
		Prompt:        "Build a web app",
		Status:        "running",
		WorkspaceMode: "existing",
		WorkspacePath: pgtype.Text{String: "/home/user/project", Valid: true},
	}

	response := buildPollBaseResponse(task)

	// Current stage: execution (order 4).
	nextStage := db.TaskStage{
		ID:         makeTestUUID(20),
		TaskID:     taskID,
		StageName:  "execution",
		StageOrder: 4,
		Status:     "pending",
	}

	// Prior stages: plan (approved), design (approved), tasks (completed).
	completedStages := []db.TaskStage{
		{
			ID:            makeTestUUID(21),
			TaskID:        taskID,
			StageName:     "plan",
			StageOrder:    1,
			Status:        "approved",
			OutputContent: pgtype.Text{String: "Plan content here", Valid: true},
		},
		{
			ID:            makeTestUUID(22),
			TaskID:        taskID,
			StageName:     "design",
			StageOrder:    2,
			Status:        "approved",
			OutputContent: pgtype.Text{String: "Design content here", Valid: true},
		},
		{
			ID:            makeTestUUID(23),
			TaskID:        taskID,
			StageName:     "tasks",
			StageOrder:    3,
			Status:        "completed",
			OutputContent: pgtype.Text{String: "Tasks content here", Valid: true},
		},
	}

	enrichResponseWithStageFields(response, nextStage, completedStages)

	// Verify workspace fields.
	if response["workspace_mode"] != "existing" {
		t.Errorf("workspace_mode = %v, want %q", response["workspace_mode"], "existing")
	}
	if response["workspace_path"] != "/home/user/project" {
		t.Errorf("workspace_path = %v, want %q", response["workspace_path"], "/home/user/project")
	}

	// Verify current_stage.
	currentStage := response["current_stage"].(map[string]interface{})
	if currentStage["name"] != "execution" {
		t.Errorf("current_stage.name = %v, want %q", currentStage["name"], "execution")
	}
	if currentStage["order"] != int32(4) {
		t.Errorf("current_stage.order = %v, want %d", currentStage["order"], 4)
	}

	// Verify all 3 prior stages are present.
	priorStages := response["prior_stages"].([]map[string]interface{})
	if len(priorStages) != 3 {
		t.Fatalf("expected 3 prior stages, got %d", len(priorStages))
	}

	expectedNames := []string{"plan", "design", "tasks"}
	expectedOutputs := []string{"Plan content here", "Design content here", "Tasks content here"}
	for i, ps := range priorStages {
		if ps["name"] != expectedNames[i] {
			t.Errorf("prior_stages[%d].name = %v, want %q", i, ps["name"], expectedNames[i])
		}
		if ps["output_content"] != expectedOutputs[i] {
			t.Errorf("prior_stages[%d].output_content = %v, want %q", i, ps["output_content"], expectedOutputs[i])
		}
	}
}

// TestPollResponse_StagedTask_NoPriorStages verifies that when the first stage
// is pending (no completed stages yet), prior_stages is an empty array.
// Requirements: 9.1
func TestPollResponse_StagedTask_NoPriorStages(t *testing.T) {
	taskID := makeTestUUID(3)

	task := db.Task{
		ID:            taskID,
		AgentType:     "claude",
		Prompt:        "Create a plan",
		Status:        "running",
		WorkspaceMode: "isolated",
	}

	response := buildPollBaseResponse(task)

	// First stage is pending — no prior stages.
	nextStage := db.TaskStage{
		ID:         makeTestUUID(30),
		TaskID:     taskID,
		StageName:  "plan",
		StageOrder: 1,
		Status:     "pending",
	}

	enrichResponseWithStageFields(response, nextStage, nil)

	// Verify current_stage is present.
	currentStage := response["current_stage"].(map[string]interface{})
	if currentStage["name"] != "plan" {
		t.Errorf("current_stage.name = %v, want %q", currentStage["name"], "plan")
	}

	// Verify prior_stages is empty array (not nil).
	priorStages := response["prior_stages"].([]map[string]interface{})
	if len(priorStages) != 0 {
		t.Errorf("expected empty prior_stages, got %d entries", len(priorStages))
	}
}

// TestPollResponse_StagedTask_PriorStageWithoutOutput verifies that when a
// completed stage has no output_content, the output_content field is omitted.
// Requirements: 9.1
func TestPollResponse_StagedTask_PriorStageWithoutOutput(t *testing.T) {
	taskID := makeTestUUID(4)

	completedStages := []db.TaskStage{
		{
			ID:            makeTestUUID(40),
			TaskID:        taskID,
			StageName:     "plan",
			StageOrder:    1,
			Status:        "approved",
			OutputContent: pgtype.Text{Valid: false}, // No output content.
		},
	}

	priorStages := buildPriorStagesField(completedStages)

	if len(priorStages) != 1 {
		t.Fatalf("expected 1 prior stage, got %d", len(priorStages))
	}

	// output_content should NOT be present when OutputContent is not valid.
	if _, exists := priorStages[0]["output_content"]; exists {
		t.Error("expected output_content to be omitted when not valid")
	}
}

// ---------------------------------------------------------------------------
// Tests for single-pass tasks (no stages) — Requirements: 9.4
// ---------------------------------------------------------------------------

// TestPollResponse_SinglePassTask_OmitsStageFields verifies that for single-pass
// tasks (no stages), current_stage and prior_stages are NOT included in the
// response, maintaining backward compatibility.
// Requirements: 9.4
func TestPollResponse_SinglePassTask_OmitsStageFields(t *testing.T) {
	task := db.Task{
		ID:            makeTestUUID(5),
		AgentType:     "claude",
		Prompt:        "Fix the login bug",
		Status:        "running",
		WorkspaceMode: "isolated",
	}

	// Build base response only (no stage enrichment for single-pass tasks).
	response := buildPollBaseResponse(task)

	// Verify base fields are present.
	if response["id"] == "" {
		t.Error("expected id to be present")
	}
	if response["agent_type"] != "claude" {
		t.Errorf("agent_type = %v, want %q", response["agent_type"], "claude")
	}
	if response["prompt"] != "Fix the login bug" {
		t.Errorf("prompt = %v, want %q", response["prompt"], "Fix the login bug")
	}
	if response["status"] != "running" {
		t.Errorf("status = %v, want %q", response["status"], "running")
	}
	if response["workspace_mode"] != "isolated" {
		t.Errorf("workspace_mode = %v, want %q", response["workspace_mode"], "isolated")
	}

	// Verify stage fields are NOT present.
	if _, exists := response["current_stage"]; exists {
		t.Error("expected current_stage to be absent for single-pass task")
	}
	if _, exists := response["prior_stages"]; exists {
		t.Error("expected prior_stages to be absent for single-pass task")
	}
}

// TestPollResponse_SinglePassTask_WorkspacePathOmittedWhenEmpty verifies that
// workspace_path is not included when it's empty/invalid.
// Requirements: 9.4
func TestPollResponse_SinglePassTask_WorkspacePathOmittedWhenEmpty(t *testing.T) {
	task := db.Task{
		ID:            makeTestUUID(6),
		AgentType:     "claude",
		Prompt:        "Do something",
		Status:        "running",
		WorkspaceMode: "isolated",
		WorkspacePath: pgtype.Text{Valid: false}, // Not set.
	}

	response := buildPollBaseResponse(task)

	if _, exists := response["workspace_path"]; exists {
		t.Error("expected workspace_path to be absent when not set")
	}
}

// TestPollResponse_SinglePassTask_WorkspacePathIncludedWhenSet verifies that
// workspace_path IS included when it has a valid value.
// Requirements: 9.4
func TestPollResponse_SinglePassTask_WorkspacePathIncludedWhenSet(t *testing.T) {
	task := db.Task{
		ID:            makeTestUUID(7),
		AgentType:     "claude",
		Prompt:        "Work on project",
		Status:        "running",
		WorkspaceMode: "existing",
		WorkspacePath: pgtype.Text{String: "/home/user/myproject", Valid: true},
	}

	response := buildPollBaseResponse(task)

	if response["workspace_path"] != "/home/user/myproject" {
		t.Errorf("workspace_path = %v, want %q", response["workspace_path"], "/home/user/myproject")
	}
}

// ---------------------------------------------------------------------------
// Tests for poll eligibility logic — Requirements: 9.5
// ---------------------------------------------------------------------------

// TestPollEligibility_TaskWithPendingStage_IsEligible verifies that a task
// with at least one stage in "pending" status is eligible for the staged poll.
// Requirements: 9.5
func TestPollEligibility_TaskWithPendingStage_IsEligible(t *testing.T) {
	stages := []db.TaskStage{
		{StageName: "plan", StageOrder: 1, Status: "approved"},
		{StageName: "design", StageOrder: 2, Status: "pending"},
		{StageName: "tasks", StageOrder: 3, Status: "pending"},
	}

	if !isTaskEligibleForStagedPoll(stages) {
		t.Error("expected task with pending stage to be eligible")
	}
}

// TestPollEligibility_TaskWithAwaitingApproval_NotEligible verifies that a task
// where all non-completed stages are in "awaiting_approval" is NOT eligible.
// The SQL query ClaimPendingTaskWithStage only picks tasks where the next
// stage by order has status "pending". A task with its next stage in
// "awaiting_approval" should NOT be returned by the poll.
// Requirements: 9.5
func TestPollEligibility_TaskWithAwaitingApproval_NotEligible(t *testing.T) {
	// All stages are either completed or awaiting_approval — no pending stage.
	stages := []db.TaskStage{
		{StageName: "plan", StageOrder: 1, Status: "approved"},
		{StageName: "design", StageOrder: 2, Status: "awaiting_approval"},
	}

	if isTaskEligibleForStagedPoll(stages) {
		t.Error("expected task with only awaiting_approval stages to NOT be eligible")
	}
}

// TestPollEligibility_TaskWithAllApproved_NotEligible verifies that a task
// where all stages are approved/completed is NOT eligible for polling.
// Requirements: 9.5
func TestPollEligibility_TaskWithAllApproved_NotEligible(t *testing.T) {
	stages := []db.TaskStage{
		{StageName: "plan", StageOrder: 1, Status: "approved"},
		{StageName: "design", StageOrder: 2, Status: "approved"},
		{StageName: "execution", StageOrder: 4, Status: "completed"},
	}

	if isTaskEligibleForStagedPoll(stages) {
		t.Error("expected task with all approved/completed stages to NOT be eligible")
	}
}

// TestPollEligibility_TaskWithRunningStage_NotEligible verifies that a task
// where the next stage is "running" is NOT eligible (already being executed).
// Requirements: 9.5
func TestPollEligibility_TaskWithRunningStage_NotEligible(t *testing.T) {
	stages := []db.TaskStage{
		{StageName: "plan", StageOrder: 1, Status: "approved"},
		{StageName: "design", StageOrder: 2, Status: "running"},
	}

	if isTaskEligibleForStagedPoll(stages) {
		t.Error("expected task with running stage (no pending) to NOT be eligible")
	}
}

// TestPollEligibility_TaskWithRejectedStage_NotEligible verifies that a task
// where stages are in "rejected" status (before re-queue to pending) is NOT eligible.
// Requirements: 9.5
func TestPollEligibility_TaskWithRejectedStage_NotEligible(t *testing.T) {
	stages := []db.TaskStage{
		{StageName: "plan", StageOrder: 1, Status: "approved"},
		{StageName: "design", StageOrder: 2, Status: "rejected"},
	}

	if isTaskEligibleForStagedPoll(stages) {
		t.Error("expected task with rejected stage (not yet re-queued) to NOT be eligible")
	}
}

// TestPollEligibility_EmptyStages_NotEligible verifies that a task with no
// stages is not eligible for the staged poll (it's a single-pass task).
// Requirements: 9.5
func TestPollEligibility_EmptyStages_NotEligible(t *testing.T) {
	if isTaskEligibleForStagedPoll(nil) {
		t.Error("expected task with no stages to NOT be eligible for staged poll")
	}

	if isTaskEligibleForStagedPoll([]db.TaskStage{}) {
		t.Error("expected task with empty stages to NOT be eligible for staged poll")
	}
}

// ---------------------------------------------------------------------------
// Test buildCurrentStageField directly
// ---------------------------------------------------------------------------

// TestBuildCurrentStageField verifies the current_stage map construction.
func TestBuildCurrentStageField(t *testing.T) {
	tests := []struct {
		name       string
		stage      db.TaskStage
		wantName   string
		wantOrder  int32
		wantStatus string
	}{
		{
			name:       "plan stage pending",
			stage:      db.TaskStage{StageName: "plan", StageOrder: 1, Status: "pending"},
			wantName:   "plan",
			wantOrder:  1,
			wantStatus: "pending",
		},
		{
			name:       "design stage pending",
			stage:      db.TaskStage{StageName: "design", StageOrder: 2, Status: "pending"},
			wantName:   "design",
			wantOrder:  2,
			wantStatus: "pending",
		},
		{
			name:       "execution stage pending",
			stage:      db.TaskStage{StageName: "execution", StageOrder: 4, Status: "pending"},
			wantName:   "execution",
			wantOrder:  4,
			wantStatus: "pending",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildCurrentStageField(tt.stage)

			if result["name"] != tt.wantName {
				t.Errorf("name = %v, want %q", result["name"], tt.wantName)
			}
			if result["order"] != tt.wantOrder {
				t.Errorf("order = %v, want %d", result["order"], tt.wantOrder)
			}
			if result["status"] != tt.wantStatus {
				t.Errorf("status = %v, want %q", result["status"], tt.wantStatus)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test buildPriorStagesField directly
// ---------------------------------------------------------------------------

// TestBuildPriorStagesField verifies the prior_stages array construction.
func TestBuildPriorStagesField(t *testing.T) {
	t.Run("nil input returns empty slice", func(t *testing.T) {
		result := buildPriorStagesField(nil)
		if len(result) != 0 {
			t.Errorf("expected empty slice, got %d entries", len(result))
		}
	})

	t.Run("empty input returns empty slice", func(t *testing.T) {
		result := buildPriorStagesField([]db.TaskStage{})
		if len(result) != 0 {
			t.Errorf("expected empty slice, got %d entries", len(result))
		}
	})

	t.Run("stages with output include output_content", func(t *testing.T) {
		stages := []db.TaskStage{
			{
				StageName:     "plan",
				StageOrder:    1,
				Status:        "approved",
				OutputContent: pgtype.Text{String: "plan output", Valid: true},
			},
			{
				StageName:     "design",
				StageOrder:    2,
				Status:        "completed",
				OutputContent: pgtype.Text{String: "design output", Valid: true},
			},
		}

		result := buildPriorStagesField(stages)
		if len(result) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(result))
		}

		if result[0]["output_content"] != "plan output" {
			t.Errorf("result[0].output_content = %v, want %q", result[0]["output_content"], "plan output")
		}
		if result[1]["output_content"] != "design output" {
			t.Errorf("result[1].output_content = %v, want %q", result[1]["output_content"], "design output")
		}
	})

	t.Run("stages without output omit output_content", func(t *testing.T) {
		stages := []db.TaskStage{
			{
				StageName:     "plan",
				StageOrder:    1,
				Status:        "approved",
				OutputContent: pgtype.Text{Valid: false},
			},
		}

		result := buildPriorStagesField(stages)
		if len(result) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result))
		}

		if _, exists := result[0]["output_content"]; exists {
			t.Error("expected output_content to be absent when not valid")
		}
	})
}

// ---------------------------------------------------------------------------
// Tests for conversational task poll response — Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 8.2
// ---------------------------------------------------------------------------

// TestPollResponse_ConversationalTask_IncludesDeliverableType verifies that
// when a conversational task is claimed, the poll response includes the
// deliverable_type field matching the stage_name.
// Requirements: 9.1
func TestPollResponse_ConversationalTask_IncludesDeliverableType(t *testing.T) {
	taskID := makeTestUUID(20)

	task := db.Task{
		ID:            taskID,
		AgentType:     "claude",
		Prompt:        "Create a plan for the project",
		Status:        "running",
		WorkspaceMode: "isolated",
		Deliverables:  []byte("[]"),
	}

	response := buildPollBaseResponse(task)

	// Simulate a conversational task stage (plan).
	stage := db.TaskStage{
		ID:         makeTestUUID(21),
		TaskID:     taskID,
		StageName:  "plan",
		StageOrder: 1,
		Status:     "pending",
	}

	h := &DaemonHandler{}
	h.enrichPollResponseForConversationalTask(response, task, stage)

	// Verify deliverable_type is present.
	if response["deliverable_type"] != "plan" {
		t.Errorf("deliverable_type = %v, want %q", response["deliverable_type"], "plan")
	}

	// Verify legacy stage fields are NOT present.
	if _, exists := response["current_stage"]; exists {
		t.Error("expected current_stage to be absent for conversational task")
	}
	if _, exists := response["prior_stages"]; exists {
		t.Error("expected prior_stages to be absent for conversational task")
	}
}

// TestPollResponse_ConversationalTask_IncludesPriorContext verifies that
// when a conversational task has prior_context stored as a JSON array in
// the deliverables column, the poll response includes the prior_context field.
// Requirements: 9.3
func TestPollResponse_ConversationalTask_IncludesPriorContext(t *testing.T) {
	taskID := makeTestUUID(22)

	task := db.Task{
		ID:            taskID,
		AgentType:     "claude",
		Prompt:        "Design the system",
		Status:        "running",
		WorkspaceMode: "isolated",
		Deliverables:  []byte(`["Plan output: step 1, step 2, step 3"]`),
	}

	response := buildPollBaseResponse(task)

	stage := db.TaskStage{
		ID:         makeTestUUID(23),
		TaskID:     taskID,
		StageName:  "design",
		StageOrder: 2,
		Status:     "pending",
	}

	h := &DaemonHandler{}
	h.enrichPollResponseForConversationalTask(response, task, stage)

	// Verify deliverable_type.
	if response["deliverable_type"] != "design" {
		t.Errorf("deliverable_type = %v, want %q", response["deliverable_type"], "design")
	}

	// Verify prior_context is present.
	priorContext, ok := response["prior_context"].([]string)
	if !ok {
		t.Fatal("expected prior_context to be present as []string")
	}
	if len(priorContext) != 1 {
		t.Fatalf("expected 1 prior_context entry, got %d", len(priorContext))
	}
	if priorContext[0] != "Plan output: step 1, step 2, step 3" {
		t.Errorf("prior_context[0] = %q, want plan output", priorContext[0])
	}

	// Verify prior_session_id is NOT present (first message, not follow-up).
	if _, exists := response["prior_session_id"]; exists {
		t.Error("expected prior_session_id to be absent for first message")
	}
}

// TestPollResponse_ConversationalTask_IncludesPriorSessionID verifies that
// when a follow-up task has prior_session_id stored in the deliverables JSON
// object, the poll response includes the prior_session_id field.
// Requirements: 9.2
func TestPollResponse_ConversationalTask_IncludesPriorSessionID(t *testing.T) {
	taskID := makeTestUUID(24)

	task := db.Task{
		ID:            taskID,
		AgentType:     "claude",
		Prompt:        "Refine the plan",
		Status:        "running",
		WorkspaceMode: "isolated",
		Deliverables:  []byte(`{"prior_session_id":"session-abc-123"}`),
	}

	response := buildPollBaseResponse(task)

	stage := db.TaskStage{
		ID:         makeTestUUID(25),
		TaskID:     taskID,
		StageName:  "plan",
		StageOrder: 1,
		Status:     "pending",
	}

	h := &DaemonHandler{}
	h.enrichPollResponseForConversationalTask(response, task, stage)

	// Verify prior_session_id is present.
	if response["prior_session_id"] != "session-abc-123" {
		t.Errorf("prior_session_id = %v, want %q", response["prior_session_id"], "session-abc-123")
	}

	// Verify prior_context is NOT present (follow-up, not first message).
	if _, exists := response["prior_context"]; exists {
		t.Error("expected prior_context to be absent for follow-up task")
	}
}

// TestPollResponse_ConversationalTask_IncludesWorkspaceConfig verifies that
// when a conversational task is of type execution, the poll response includes
// workspace_config with git_repo_url and local_directory_path.
// Requirements: 9.4
func TestPollResponse_ConversationalTask_IncludesWorkspaceConfig(t *testing.T) {
	taskID := makeTestUUID(26)

	task := db.Task{
		ID:            taskID,
		AgentType:     "claude",
		Prompt:        "Implement the feature",
		Status:        "running",
		WorkspaceMode: "existing",
		WorkspacePath: pgtype.Text{String: "/home/user/project", Valid: true},
		GitRepoUrl:    pgtype.Text{String: "https://github.com/user/repo.git", Valid: true},
		Deliverables:  []byte(`["Plan output","Design output","Tasks output"]`),
	}

	response := buildPollBaseResponse(task)

	stage := db.TaskStage{
		ID:         makeTestUUID(27),
		TaskID:     taskID,
		StageName:  "execution",
		StageOrder: 4,
		Status:     "pending",
	}

	h := &DaemonHandler{}
	h.enrichPollResponseForConversationalTask(response, task, stage)

	// Verify deliverable_type.
	if response["deliverable_type"] != "execution" {
		t.Errorf("deliverable_type = %v, want %q", response["deliverable_type"], "execution")
	}

	// Verify workspace_config is present.
	wsConfig, ok := response["workspace_config"].(map[string]interface{})
	if !ok {
		t.Fatal("expected workspace_config to be present")
	}
	if wsConfig["local_directory_path"] != "/home/user/project" {
		t.Errorf("workspace_config.local_directory_path = %v, want %q", wsConfig["local_directory_path"], "/home/user/project")
	}
	if wsConfig["git_repo_url"] != "https://github.com/user/repo.git" {
		t.Errorf("workspace_config.git_repo_url = %v, want %q", wsConfig["git_repo_url"], "https://github.com/user/repo.git")
	}

	// Verify prior_context is present (3 entries).
	priorContext, ok := response["prior_context"].([]string)
	if !ok {
		t.Fatal("expected prior_context to be present as []string")
	}
	if len(priorContext) != 3 {
		t.Fatalf("expected 3 prior_context entries, got %d", len(priorContext))
	}
}

// TestPollResponse_ConversationalTask_NoWorkspaceConfigForNonExecution verifies
// that workspace_config is NOT included for non-execution deliverable types.
// Requirements: 9.4
func TestPollResponse_ConversationalTask_NoWorkspaceConfigForNonExecution(t *testing.T) {
	for _, deliverableType := range []string{"plan", "design", "tasks"} {
		t.Run(deliverableType, func(t *testing.T) {
			taskID := makeTestUUID(28)

			task := db.Task{
				ID:            taskID,
				AgentType:     "claude",
				Prompt:        "Do something",
				Status:        "running",
				WorkspaceMode: "isolated",
				Deliverables:  []byte("[]"),
			}

			response := buildPollBaseResponse(task)

			stage := db.TaskStage{
				ID:         makeTestUUID(29),
				TaskID:     taskID,
				StageName:  deliverableType,
				StageOrder: 1,
				Status:     "pending",
			}

			h := &DaemonHandler{}
			h.enrichPollResponseForConversationalTask(response, task, stage)

			// Verify workspace_config is NOT present.
			if _, exists := response["workspace_config"]; exists {
				t.Errorf("expected workspace_config to be absent for %s type", deliverableType)
			}
		})
	}
}

// TestPollResponse_ConversationalTask_IncludesPriorWorkDir verifies that
// when a task_stage has a work_dir from a prior execution, the poll response
// includes the prior_work_dir field.
// Requirements: 9.5
func TestPollResponse_ConversationalTask_IncludesPriorWorkDir(t *testing.T) {
	taskID := makeTestUUID(30)

	task := db.Task{
		ID:            taskID,
		AgentType:     "claude",
		Prompt:        "Continue working",
		Status:        "running",
		WorkspaceMode: "existing",
		WorkspacePath: pgtype.Text{String: "/home/user/project", Valid: true},
		Deliverables:  []byte(`{"prior_session_id":"session-xyz"}`),
	}

	response := buildPollBaseResponse(task)

	stage := db.TaskStage{
		ID:         makeTestUUID(31),
		TaskID:     taskID,
		StageName:  "execution",
		StageOrder: 4,
		Status:     "pending",
		WorkDir:    pgtype.Text{String: "/home/user/project/workspace", Valid: true},
	}

	h := &DaemonHandler{}
	h.enrichPollResponseForConversationalTask(response, task, stage)

	// Verify prior_work_dir is present from the stage.
	if response["prior_work_dir"] != "/home/user/project/workspace" {
		t.Errorf("prior_work_dir = %v, want %q", response["prior_work_dir"], "/home/user/project/workspace")
	}
}

// TestPollResponse_ConversationalTask_PriorWorkDirFromDeliverables verifies
// that prior_work_dir can come from the deliverables JSON when the stage
// doesn't have it set.
// Requirements: 9.5
func TestPollResponse_ConversationalTask_PriorWorkDirFromDeliverables(t *testing.T) {
	taskID := makeTestUUID(32)

	task := db.Task{
		ID:            taskID,
		AgentType:     "claude",
		Prompt:        "Continue working",
		Status:        "running",
		WorkspaceMode: "existing",
		WorkspacePath: pgtype.Text{String: "/home/user/project", Valid: true},
		Deliverables:  []byte(`{"prior_session_id":"session-xyz","prior_work_dir":"/tmp/work"}`),
	}

	response := buildPollBaseResponse(task)

	stage := db.TaskStage{
		ID:         makeTestUUID(33),
		TaskID:     taskID,
		StageName:  "execution",
		StageOrder: 4,
		Status:     "pending",
		WorkDir:    pgtype.Text{Valid: false}, // No work_dir on stage.
	}

	h := &DaemonHandler{}
	h.enrichPollResponseForConversationalTask(response, task, stage)

	// Verify prior_work_dir comes from deliverables.
	if response["prior_work_dir"] != "/tmp/work" {
		t.Errorf("prior_work_dir = %v, want %q", response["prior_work_dir"], "/tmp/work")
	}
}

// TestPollResponse_ConversationalTask_EmptyPriorContextOmitted verifies that
// when prior_context is an empty array, it is NOT included in the response.
// Requirements: 9.3
func TestPollResponse_ConversationalTask_EmptyPriorContextOmitted(t *testing.T) {
	taskID := makeTestUUID(34)

	task := db.Task{
		ID:            taskID,
		AgentType:     "claude",
		Prompt:        "Create a plan",
		Status:        "running",
		WorkspaceMode: "isolated",
		Deliverables:  []byte("[]"),
	}

	response := buildPollBaseResponse(task)

	stage := db.TaskStage{
		ID:         makeTestUUID(35),
		TaskID:     taskID,
		StageName:  "plan",
		StageOrder: 1,
		Status:     "pending",
	}

	h := &DaemonHandler{}
	h.enrichPollResponseForConversationalTask(response, task, stage)

	// Verify prior_context is NOT present when empty.
	if _, exists := response["prior_context"]; exists {
		t.Error("expected prior_context to be absent when empty array")
	}
}

// TestPollResponse_ConversationalTask_BackwardCompat_NoDeliverableTypeForSinglePass
// verifies that single-pass tasks (no deliverable_type) do NOT get conversational
// fields in the poll response.
// Requirements: 8.2
func TestPollResponse_ConversationalTask_BackwardCompat_NoDeliverableTypeForSinglePass(t *testing.T) {
	task := db.Task{
		ID:            makeTestUUID(36),
		AgentType:     "claude",
		Prompt:        "Fix the bug",
		Status:        "running",
		WorkspaceMode: "isolated",
	}

	// Build base response only (no stage enrichment for single-pass tasks).
	response := buildPollBaseResponse(task)

	// Verify conversational fields are NOT present.
	if _, exists := response["deliverable_type"]; exists {
		t.Error("expected deliverable_type to be absent for single-pass task")
	}
	if _, exists := response["prior_session_id"]; exists {
		t.Error("expected prior_session_id to be absent for single-pass task")
	}
	if _, exists := response["prior_context"]; exists {
		t.Error("expected prior_context to be absent for single-pass task")
	}
	if _, exists := response["workspace_config"]; exists {
		t.Error("expected workspace_config to be absent for single-pass task")
	}
	if _, exists := response["prior_work_dir"]; exists {
		t.Error("expected prior_work_dir to be absent for single-pass task")
	}
}

// ---------------------------------------------------------------------------
// Unit tests for poll and completion enhancements (task 5.7)
// Requirements: 9.1, 9.2, 10.1, 10.2, 8.2
// ---------------------------------------------------------------------------

// TestPollResponse_ConversationalTask_AllFieldsPresent verifies that when a
// conversational task is polled, the response includes all expected conversational
// fields: deliverable_type, prior_context (if present), prior_session_id (if
// follow-up), workspace_config (if execution), and prior_work_dir (if available).
// Requirements: 9.1, 9.2
func TestPollResponse_ConversationalTask_AllFieldsPresent(t *testing.T) {
	tests := []struct {
		name              string
		stageName         string
		deliverables      []byte
		workspacePath     pgtype.Text
		gitRepoUrl        pgtype.Text
		stageWorkDir      pgtype.Text
		wantDeliverable   string
		wantPriorSession  string
		wantPriorContext  []string
		wantWorkspaceConf bool
		wantPriorWorkDir  string
	}{
		{
			name:            "plan type first message with context",
			stageName:       "plan",
			deliverables:    []byte(`["prior output 1"]`),
			wantDeliverable: "plan",
			wantPriorContext: []string{"prior output 1"},
		},
		{
			name:             "design type follow-up with session",
			stageName:        "design",
			deliverables:     []byte(`{"prior_session_id":"sess-abc"}`),
			wantDeliverable:  "design",
			wantPriorSession: "sess-abc",
		},
		{
			name:              "execution type with workspace config",
			stageName:         "execution",
			deliverables:      []byte(`["plan","design","tasks"]`),
			workspacePath:     pgtype.Text{String: "/home/user/project", Valid: true},
			gitRepoUrl:        pgtype.Text{String: "https://github.com/user/repo", Valid: true},
			wantDeliverable:   "execution",
			wantPriorContext:  []string{"plan", "design", "tasks"},
			wantWorkspaceConf: true,
		},
		{
			name:             "tasks type follow-up with prior_work_dir on stage",
			stageName:        "tasks",
			deliverables:     []byte(`{"prior_session_id":"sess-xyz"}`),
			stageWorkDir:     pgtype.Text{String: "/tmp/workspace", Valid: true},
			wantDeliverable:  "tasks",
			wantPriorSession: "sess-xyz",
			wantPriorWorkDir: "/tmp/workspace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskID := makeTestUUID(50)
			task := db.Task{
				ID:            taskID,
				AgentType:     "claude",
				Prompt:        "Do something",
				Status:        "running",
				WorkspaceMode: "isolated",
				Deliverables:  tt.deliverables,
				WorkspacePath: tt.workspacePath,
				GitRepoUrl:    tt.gitRepoUrl,
			}

			response := buildPollBaseResponse(task)

			stage := db.TaskStage{
				ID:        makeTestUUID(51),
				TaskID:    taskID,
				StageName: tt.stageName,
				Status:    "pending",
				WorkDir:   tt.stageWorkDir,
			}

			h := &DaemonHandler{}
			h.enrichPollResponseForConversationalTask(response, task, stage)

			// Verify deliverable_type is always present.
			if response["deliverable_type"] != tt.wantDeliverable {
				t.Errorf("deliverable_type = %v, want %q", response["deliverable_type"], tt.wantDeliverable)
			}

			// Verify prior_session_id.
			if tt.wantPriorSession != "" {
				if response["prior_session_id"] != tt.wantPriorSession {
					t.Errorf("prior_session_id = %v, want %q", response["prior_session_id"], tt.wantPriorSession)
				}
			} else {
				if _, exists := response["prior_session_id"]; exists {
					t.Error("expected prior_session_id to be absent")
				}
			}

			// Verify prior_context.
			if tt.wantPriorContext != nil {
				ctx, ok := response["prior_context"].([]string)
				if !ok {
					t.Fatal("expected prior_context to be present as []string")
				}
				if len(ctx) != len(tt.wantPriorContext) {
					t.Fatalf("prior_context length = %d, want %d", len(ctx), len(tt.wantPriorContext))
				}
			} else {
				if _, exists := response["prior_context"]; exists {
					t.Error("expected prior_context to be absent")
				}
			}

			// Verify workspace_config.
			if tt.wantWorkspaceConf {
				if _, ok := response["workspace_config"].(map[string]interface{}); !ok {
					t.Error("expected workspace_config to be present")
				}
			} else {
				if _, exists := response["workspace_config"]; exists {
					t.Error("expected workspace_config to be absent")
				}
			}

			// Verify prior_work_dir.
			if tt.wantPriorWorkDir != "" {
				if response["prior_work_dir"] != tt.wantPriorWorkDir {
					t.Errorf("prior_work_dir = %v, want %q", response["prior_work_dir"], tt.wantPriorWorkDir)
				}
			}
		})
	}
}

// TestPollResponse_SinglePassTask_OmitsAllConversationalFields verifies that
// buildPollBaseResponse for a single-pass task (no stages, no deliverable_type)
// does NOT include any conversational fields.
// Requirements: 8.2
func TestPollResponse_SinglePassTask_OmitsAllConversationalFields(t *testing.T) {
	task := db.Task{
		ID:            makeTestUUID(52),
		AgentType:     "claude",
		Prompt:        "Fix the login bug in auth.go",
		Status:        "running",
		WorkspaceMode: "isolated",
	}

	response := buildPollBaseResponse(task)

	conversationalFields := []string{
		"deliverable_type",
		"prior_session_id",
		"prior_context",
		"workspace_config",
		"prior_work_dir",
	}

	for _, field := range conversationalFields {
		if _, exists := response[field]; exists {
			t.Errorf("expected %q to be absent for single-pass task, but it was present", field)
		}
	}

	// Verify base fields ARE present.
	if response["agent_type"] != "claude" {
		t.Errorf("agent_type = %v, want %q", response["agent_type"], "claude")
	}
	if response["prompt"] != "Fix the login bug in auth.go" {
		t.Errorf("prompt = %v, want expected value", response["prompt"])
	}
}

// TestCompletion_StoresSessionIDAndWorkDir verifies that buildStageCompletionParams
// correctly constructs the params with session_id and work_dir from the
// TaskCompleteReq, matching the logic in completeConversationalStage.
// Requirements: 10.1, 10.2
func TestCompletion_StoresSessionIDAndWorkDir(t *testing.T) {
	tests := []struct {
		name          string
		req           TaskCompleteReq
		wantSessionID pgtype.Text
		wantWorkDir   pgtype.Text
		wantOutput    pgtype.Text
	}{
		{
			name: "both session_id and work_dir present",
			req: TaskCompleteReq{
				Output:    "# Plan\n\nStep 1: Setup project",
				SessionID: "session-abc-123",
				WorkDir:   "/home/user/workspace",
				ExitCode:  0,
			},
			wantSessionID: pgtype.Text{String: "session-abc-123", Valid: true},
			wantWorkDir:   pgtype.Text{String: "/home/user/workspace", Valid: true},
			wantOutput:    pgtype.Text{String: "# Plan\n\nStep 1: Setup project", Valid: true},
		},
		{
			name: "session_id present, work_dir empty",
			req: TaskCompleteReq{
				Output:    "Design document content",
				SessionID: "session-xyz-789",
				WorkDir:   "",
				ExitCode:  0,
			},
			wantSessionID: pgtype.Text{String: "session-xyz-789", Valid: true},
			wantWorkDir:   pgtype.Text{String: "", Valid: false},
			wantOutput:    pgtype.Text{String: "Design document content", Valid: true},
		},
		{
			name: "no session_id (agent did not return one)",
			req: TaskCompleteReq{
				Output:    "Task list output",
				SessionID: "",
				WorkDir:   "/tmp/work",
				ExitCode:  0,
			},
			wantSessionID: pgtype.Text{String: "", Valid: false},
			wantWorkDir:   pgtype.Text{String: "/tmp/work", Valid: true},
			wantOutput:    pgtype.Text{String: "Task list output", Valid: true},
		},
		{
			name: "neither session_id nor work_dir",
			req: TaskCompleteReq{
				Output:    "Simple output",
				SessionID: "",
				WorkDir:   "",
				ExitCode:  0,
			},
			wantSessionID: pgtype.Text{String: "", Valid: false},
			wantWorkDir:   pgtype.Text{String: "", Valid: false},
			wantOutput:    pgtype.Text{String: "Simple output", Valid: true},
		},
		{
			name: "empty output",
			req: TaskCompleteReq{
				Output:    "",
				SessionID: "session-empty-output",
				WorkDir:   "/workspace",
				ExitCode:  0,
			},
			wantSessionID: pgtype.Text{String: "session-empty-output", Valid: true},
			wantWorkDir:   pgtype.Text{String: "/workspace", Valid: true},
			wantOutput:    pgtype.Text{String: "", Valid: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stageID := makeTestUUID(60)
			params := buildStageCompletionParams(stageID, tt.req)

			// Verify stage ID is passed through.
			if params.ID != stageID {
				t.Errorf("params.ID = %v, want %v", params.ID, stageID)
			}

			// Verify session_id.
			if params.SessionID != tt.wantSessionID {
				t.Errorf("params.SessionID = %+v, want %+v", params.SessionID, tt.wantSessionID)
			}

			// Verify work_dir.
			if params.WorkDir != tt.wantWorkDir {
				t.Errorf("params.WorkDir = %+v, want %+v", params.WorkDir, tt.wantWorkDir)
			}

			// Verify output_content.
			if params.OutputContent != tt.wantOutput {
				t.Errorf("params.OutputContent = %+v, want %+v", params.OutputContent, tt.wantOutput)
			}
		})
	}
}

// TestCompletion_CreatesPromptHistoryEntry verifies that buildPromptHistoryParams
// correctly constructs the params for inserting a prompt_history entry,
// matching the logic in completeConversationalStage.
// Requirements: 10.1, 10.2
func TestCompletion_CreatesPromptHistoryEntry(t *testing.T) {
	tests := []struct {
		name       string
		promptText string
		output     string
		wantOutput pgtype.Text
	}{
		{
			name:       "normal completion with output",
			promptText: "Create a plan for the REST API",
			output:     "# Plan\n\n## Overview\nBuild a REST API for user management.",
			wantOutput: pgtype.Text{String: "# Plan\n\n## Overview\nBuild a REST API for user management.", Valid: true},
		},
		{
			name:       "completion with empty output",
			promptText: "Execute the implementation",
			output:     "",
			wantOutput: pgtype.Text{String: "", Valid: false},
		},
		{
			name:       "follow-up completion",
			promptText: "Add more detail about authentication",
			output:     "# Updated Plan\n\n## Authentication\nUse JWT tokens...",
			wantOutput: pgtype.Text{String: "# Updated Plan\n\n## Authentication\nUse JWT tokens...", Valid: true},
		},
		{
			name:       "large output",
			promptText: "Generate detailed design",
			output:     string(make([]byte, 5000)),
			wantOutput: pgtype.Text{String: string(make([]byte, 5000)), Valid: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stageID := makeTestUUID(70)
			taskID := makeTestUUID(71)

			params := buildPromptHistoryParams(stageID, taskID, tt.promptText, tt.output)

			// Verify task_stage_id.
			if params.TaskStageID != stageID {
				t.Errorf("params.TaskStageID = %v, want %v", params.TaskStageID, stageID)
			}

			// Verify task_id.
			if params.TaskID != taskID {
				t.Errorf("params.TaskID = %v, want %v", params.TaskID, taskID)
			}

			// Verify prompt_text is the user's original prompt.
			if params.PromptText != tt.promptText {
				t.Errorf("params.PromptText = %q, want %q", params.PromptText, tt.promptText)
			}

			// Verify output_text.
			if params.OutputText != tt.wantOutput {
				t.Errorf("params.OutputText = %+v, want %+v", params.OutputText, tt.wantOutput)
			}
		})
	}
}

// TestCompletion_SetsStatusToCompleted verifies that the UpdateStageCompletion
// SQL query always sets status to "completed" (never "awaiting_approval").
// This is a key behavioral difference from the old approval-gate model.
// Requirements: 10.1, 10.2
func TestCompletion_SetsStatusToCompleted(t *testing.T) {
	// The SQL query hardcodes: SET status = 'completed'
	// This function documents and verifies that invariant.
	status := stageCompletionStatusFromSQL()

	if status != "completed" {
		t.Errorf("stageCompletionStatusFromSQL() = %q, want %q", status, "completed")
	}

	// Verify it's NOT "awaiting_approval" — the old approval-gate status.
	if status == "awaiting_approval" {
		t.Error("stage completion must NOT set status to 'awaiting_approval' for conversational tasks")
	}
}

// TestCompletion_IsConversationalCompletion verifies that isConversationalCompletion
// correctly identifies conversational task completions by finding a running stage
// with a valid deliverable type name.
// Requirements: 10.1
func TestCompletion_IsConversationalCompletion(t *testing.T) {
	tests := []struct {
		name             string
		stages           []db.TaskStage
		wantDeliverable  string
		wantFound        bool
	}{
		{
			name: "running plan stage (conversational)",
			stages: []db.TaskStage{
				{StageName: "plan", Status: "running"},
			},
			wantDeliverable: "plan",
			wantFound:       true,
		},
		{
			name: "running design stage (conversational)",
			stages: []db.TaskStage{
				{StageName: "design", Status: "running"},
			},
			wantDeliverable: "design",
			wantFound:       true,
		},
		{
			name: "running tasks stage (conversational)",
			stages: []db.TaskStage{
				{StageName: "tasks", Status: "running"},
			},
			wantDeliverable: "tasks",
			wantFound:       true,
		},
		{
			name: "running execution stage (conversational)",
			stages: []db.TaskStage{
				{StageName: "execution", Status: "running"},
			},
			wantDeliverable: "execution",
			wantFound:       true,
		},
		{
			name: "running non-deliverable stage (not conversational)",
			stages: []db.TaskStage{
				{StageName: "planning", Status: "running"},
			},
			wantDeliverable: "",
			wantFound:       false,
		},
		{
			name: "completed plan stage (not running, so not a completion target)",
			stages: []db.TaskStage{
				{StageName: "plan", Status: "completed"},
			},
			wantDeliverable: "",
			wantFound:       false,
		},
		{
			name: "pending plan stage (not running)",
			stages: []db.TaskStage{
				{StageName: "plan", Status: "pending"},
			},
			wantDeliverable: "",
			wantFound:       false,
		},
		{
			name:            "empty stages (single-pass task)",
			stages:          []db.TaskStage{},
			wantDeliverable: "",
			wantFound:       false,
		},
		{
			name:            "nil stages",
			stages:          nil,
			wantDeliverable: "",
			wantFound:       false,
		},
		{
			name: "multiple stages, one running deliverable",
			stages: []db.TaskStage{
				{StageName: "plan", Status: "completed"},
				{StageName: "design", Status: "running"},
				{StageName: "tasks", Status: "pending"},
			},
			wantDeliverable: "design",
			wantFound:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deliverableType, stage := isConversationalCompletion(tt.stages)

			if deliverableType != tt.wantDeliverable {
				t.Errorf("deliverableType = %q, want %q", deliverableType, tt.wantDeliverable)
			}

			if tt.wantFound {
				if stage.StageName != tt.wantDeliverable {
					t.Errorf("stage.StageName = %q, want %q", stage.StageName, tt.wantDeliverable)
				}
			}
		})
	}
}

// TestCompletion_BroadcastPayload_ConversationalVsSinglePass verifies that
// the broadcast payload includes deliverable_type and output_content for
// conversational tasks, but omits them for single-pass tasks.
// Requirements: 10.1, 8.2
func TestCompletion_BroadcastPayload_ConversationalVsSinglePass(t *testing.T) {
	tests := []struct {
		name            string
		deliverableType string
		output          string
		wantFields      []string
		wantAbsent      []string
	}{
		{
			name:            "conversational task (plan)",
			deliverableType: "plan",
			output:          "# Plan\nStep 1...",
			wantFields:      []string{"task_id", "exit_code", "deliverable_type", "output_content"},
			wantAbsent:      nil,
		},
		{
			name:            "conversational task (execution)",
			deliverableType: "execution",
			output:          "Implementation complete.",
			wantFields:      []string{"task_id", "exit_code", "deliverable_type", "output_content"},
			wantAbsent:      nil,
		},
		{
			name:            "single-pass task (no deliverable_type)",
			deliverableType: "",
			output:          "Done.",
			wantFields:      []string{"task_id", "exit_code"},
			wantAbsent:      []string{"deliverable_type", "output_content"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := buildCompletionBroadcastPayload("task-123", 0, tt.deliverableType, tt.output)

			for _, field := range tt.wantFields {
				if _, exists := payload[field]; !exists {
					t.Errorf("expected %q to be present in payload", field)
				}
			}

			for _, field := range tt.wantAbsent {
				if _, exists := payload[field]; exists {
					t.Errorf("expected %q to be absent in payload", field)
				}
			}

			// Verify specific values for conversational tasks.
			if tt.deliverableType != "" {
				if payload["deliverable_type"] != tt.deliverableType {
					t.Errorf("payload[deliverable_type] = %v, want %q", payload["deliverable_type"], tt.deliverableType)
				}
				if payload["output_content"] != tt.output {
					t.Errorf("payload[output_content] = %v, want %q", payload["output_content"], tt.output)
				}
			}
		})
	}
}

// TestParseConversationalDeliverables_ArrayFormat verifies parsing of the
// JSON array format (prior_context for first messages).
func TestParseConversationalDeliverables_ArrayFormat(t *testing.T) {
	h := &DaemonHandler{}

	tests := []struct {
		name         string
		deliverables []byte
		wantContext  []string
		wantSession  string
	}{
		{
			name:         "single context entry",
			deliverables: []byte(`["Plan: do X, Y, Z"]`),
			wantContext:  []string{"Plan: do X, Y, Z"},
		},
		{
			name:         "multiple context entries",
			deliverables: []byte(`["Plan output","Design output"]`),
			wantContext:  []string{"Plan output", "Design output"},
		},
		{
			name:         "empty array",
			deliverables: []byte(`[]`),
			wantContext:  nil, // empty array should not be included
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := map[string]interface{}{}
			h.parseConversationalDeliverables(response, tt.deliverables)

			if tt.wantContext != nil {
				ctx, ok := response["prior_context"].([]string)
				if !ok {
					t.Fatal("expected prior_context to be present")
				}
				if len(ctx) != len(tt.wantContext) {
					t.Fatalf("prior_context length = %d, want %d", len(ctx), len(tt.wantContext))
				}
				for i, v := range tt.wantContext {
					if ctx[i] != v {
						t.Errorf("prior_context[%d] = %q, want %q", i, ctx[i], v)
					}
				}
			} else {
				if _, exists := response["prior_context"]; exists {
					t.Error("expected prior_context to be absent")
				}
			}

			// Should not have prior_session_id for array format.
			if _, exists := response["prior_session_id"]; exists {
				t.Error("expected prior_session_id to be absent for array format")
			}
		})
	}
}

// TestParseConversationalDeliverables_ObjectFormat verifies parsing of the
// JSON object format (follow-up with prior_session_id).
func TestParseConversationalDeliverables_ObjectFormat(t *testing.T) {
	h := &DaemonHandler{}

	tests := []struct {
		name        string
		deliverables []byte
		wantSession string
		wantWorkDir string
	}{
		{
			name:         "session_id only",
			deliverables: []byte(`{"prior_session_id":"sess-123"}`),
			wantSession:  "sess-123",
		},
		{
			name:         "session_id and work_dir",
			deliverables: []byte(`{"prior_session_id":"sess-456","prior_work_dir":"/tmp/work"}`),
			wantSession:  "sess-456",
			wantWorkDir:  "/tmp/work",
		},
		{
			name:         "empty session_id",
			deliverables: []byte(`{"prior_session_id":""}`),
			wantSession:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := map[string]interface{}{}
			h.parseConversationalDeliverables(response, tt.deliverables)

			if tt.wantSession != "" {
				if response["prior_session_id"] != tt.wantSession {
					t.Errorf("prior_session_id = %v, want %q", response["prior_session_id"], tt.wantSession)
				}
			} else {
				if _, exists := response["prior_session_id"]; exists {
					t.Error("expected prior_session_id to be absent when empty")
				}
			}

			if tt.wantWorkDir != "" {
				if response["prior_work_dir"] != tt.wantWorkDir {
					t.Errorf("prior_work_dir = %v, want %q", response["prior_work_dir"], tt.wantWorkDir)
				}
			}

			// Should not have prior_context for object format.
			if _, exists := response["prior_context"]; exists {
				t.Error("expected prior_context to be absent for object format")
			}
		})
	}
}
