package handler

import (
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// buildPollBaseResponse constructs the base poll response map for a claimed task.
// This is the common response structure for both staged and single-pass tasks.
func buildPollBaseResponse(task db.Task) map[string]interface{} {
	response := map[string]interface{}{
		"id":         uuidToString(task.ID),
		"agent_type": task.AgentType,
		"prompt":     task.Prompt,
		"status":     task.Status,
	}

	// Always include workspace_mode and workspace_path for all tasks.
	response["workspace_mode"] = task.WorkspaceMode
	if task.WorkspacePath.Valid && task.WorkspacePath.String != "" {
		response["workspace_path"] = task.WorkspacePath.String
	}

	return response
}

// buildCurrentStageField constructs the current_stage map from a TaskStage.
func buildCurrentStageField(stage db.TaskStage) map[string]interface{} {
	return map[string]interface{}{
		"name":   stage.StageName,
		"order":  stage.StageOrder,
		"status": stage.Status,
	}
}

// buildPriorStagesField constructs the prior_stages array from completed/approved stages.
func buildPriorStagesField(completedStages []db.TaskStage) []map[string]interface{} {
	priorStages := make([]map[string]interface{}, 0, len(completedStages))
	for _, stage := range completedStages {
		stageEntry := map[string]interface{}{
			"name":   stage.StageName,
			"order":  stage.StageOrder,
			"status": stage.Status,
		}
		if stage.OutputContent.Valid {
			stageEntry["output_content"] = stage.OutputContent.String
		}
		priorStages = append(priorStages, stageEntry)
	}
	return priorStages
}

// enrichResponseWithStageFields adds current_stage and prior_stages to the response map.
// This is the pure logic extracted from enrichPollResponseWithStages for testability.
func enrichResponseWithStageFields(response map[string]interface{}, nextStage db.TaskStage, completedStages []db.TaskStage) {
	response["current_stage"] = buildCurrentStageField(nextStage)
	response["prior_stages"] = buildPriorStagesField(completedStages)
}

// isTaskEligibleForStagedPoll checks whether a task's stages make it eligible
// for the staged poll claim. A task is eligible only if it has at least one
// stage with status "pending". Tasks with all stages in awaiting_approval,
// running, approved, rejected, or completed are NOT eligible.
func isTaskEligibleForStagedPoll(stages []db.TaskStage) bool {
	for _, s := range stages {
		if s.Status == "pending" {
			return true
		}
	}
	return false
}

// makeTestUUID creates a pgtype.UUID from a byte value for testing purposes.
func makeTestUUID(b byte) pgtype.UUID {
	var u pgtype.UUID
	u.Valid = true
	u.Bytes[0] = b
	return u
}

// ---------------------------------------------------------------------------
// Completion params construction helpers (extracted for testability)
// ---------------------------------------------------------------------------

// buildStageCompletionParams constructs the UpdateStageCompletionParams from
// a stage ID and a TaskCompleteReq. This is the same logic used in
// completeConversationalStage, extracted for unit testing.
func buildStageCompletionParams(stageID pgtype.UUID, req TaskCompleteReq) db.UpdateStageCompletionParams {
	return db.UpdateStageCompletionParams{
		ID:            stageID,
		OutputContent: pgtype.Text{String: req.Output, Valid: req.Output != ""},
		SessionID:     pgtype.Text{String: req.SessionID, Valid: req.SessionID != ""},
		WorkDir:       pgtype.Text{String: req.WorkDir, Valid: req.WorkDir != ""},
	}
}

// buildPromptHistoryParams constructs the CreatePromptHistoryEntryParams from
// a stage, task, and completion request. This is the same logic used in
// completeConversationalStage, extracted for unit testing.
func buildPromptHistoryParams(stageID, taskID pgtype.UUID, promptText string, output string) db.CreatePromptHistoryEntryParams {
	return db.CreatePromptHistoryEntryParams{
		TaskStageID: stageID,
		TaskID:      taskID,
		PromptText:  promptText,
		OutputText:  pgtype.Text{String: output, Valid: output != ""},
	}
}

// isConversationalCompletion checks whether a set of stages indicates a
// conversational task completion. Returns the deliverable type and the running
// stage if found, or empty string and zero-value stage if not.
func isConversationalCompletion(stages []db.TaskStage) (string, db.TaskStage) {
	for _, stage := range stages {
		if stage.Status == "running" && ValidDeliverableTypes[stage.StageName] {
			return stage.StageName, stage
		}
	}
	return "", db.TaskStage{}
}

// buildCompletionBroadcastPayload constructs the WebSocket broadcast payload
// for a task_completed event. For conversational tasks, it includes
// deliverable_type and output_content.
func buildCompletionBroadcastPayload(taskID string, exitCode int32, deliverableType string, output string) map[string]interface{} {
	payload := map[string]interface{}{
		"task_id":   taskID,
		"exit_code": exitCode,
	}
	if deliverableType != "" {
		payload["deliverable_type"] = deliverableType
		payload["output_content"] = output
	}
	return payload
}

// parseDeliverableTypeFromStages extracts the deliverable type from a task's
// stages. Used to determine if a task is conversational during completion.
func parseDeliverableTypeFromStages(stages []db.TaskStage) string {
	for _, stage := range stages {
		if ValidDeliverableTypes[stage.StageName] {
			return stage.StageName
		}
	}
	return ""
}

// stageCompletionStatusFromSQL returns the status that UpdateStageCompletion
// sets. This is always "completed" — the SQL hardcodes it. This function
// exists for documentation and test verification purposes.
func stageCompletionStatusFromSQL() string {
	// The SQL query: SET status = 'completed'
	// This is intentional: conversational tasks go directly to "completed",
	// never to "awaiting_approval".
	return "completed"
}

// parsePriorSessionIDFromDeliverables extracts prior_session_id from the
// deliverables JSON column (object format).
func parsePriorSessionIDFromDeliverables(deliverables []byte) string {
	if len(deliverables) == 0 {
		return ""
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(deliverables, &obj); err != nil {
		return ""
	}
	if sessionID, ok := obj["prior_session_id"].(string); ok {
		return sessionID
	}
	return ""
}
