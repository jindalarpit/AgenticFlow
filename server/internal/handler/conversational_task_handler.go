package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agenticflow/agenticflow/server/internal/middleware"
	"github.com/agenticflow/agenticflow/server/internal/realtime"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// createConversationalTask handles the conversational task creation flow.
// It is called from CreateTask when the request includes a deliverable_type field.
// This creates a task + single task_stage row (status=pending, stage_name=deliverable_type),
// stores prior_context as JSON in the deliverables column, and stores git_repo_url on the task.
func (h *UserHandler) createConversationalTask(w http.ResponseWriter, r *http.Request, req CreateTaskReq, userUUID pgtype.UUID, agentID pgtype.UUID) {
	// Build a ConversationalTaskCreateRequest for validation.
	convReq := &ConversationalTaskCreateRequest{
		AgentID:            req.AgentID,
		Prompt:             req.Prompt,
		DeliverableType:    req.DeliverableType,
		PriorContext:       req.PriorContext,
		GitRepoURL:         req.GitRepoURL,
		LocalDirectoryPath: req.LocalDirectoryPath,
	}

	// Validate the conversational task request.
	if err := convReq.Validate(); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	// Serialize prior_context as JSON for storage in the deliverables column.
	// The deliverables column is repurposed to store prior_context for conversational tasks.
	var deliverablesJSON []byte
	if len(req.PriorContext) > 0 {
		var err error
		deliverablesJSON, err = json.Marshal(req.PriorContext)
		if err != nil {
			slog.Error("create conversational task: marshal prior_context failed", "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "failed to create task")
			return
		}
	} else {
		deliverablesJSON = []byte("[]")
	}

	// Determine workspace_mode based on deliverable_type.
	// For execution tasks with a local_directory_path, use "existing" mode.
	workspaceMode := "isolated"
	var workspacePath pgtype.Text
	if req.DeliverableType == "execution" && req.LocalDirectoryPath != "" {
		workspaceMode = "existing"
		workspacePath = pgtype.Text{String: req.LocalDirectoryPath, Valid: true}
	}

	// Create the task using the conversational task query.
	task, err := h.Queries.CreateConversationalTask(r.Context(), db.CreateConversationalTaskParams{
		UserID:        userUUID,
		AgentType:     req.AgentType,
		Prompt:        req.Prompt,
		AgentID:       agentID,
		Deliverables:  deliverablesJSON,
		WorkspaceMode: workspaceMode,
		WorkspacePath: workspacePath,
		GitRepoUrl:    pgtype.Text{String: req.GitRepoURL, Valid: req.GitRepoURL != ""},
	})
	if err != nil {
		slog.Error("create conversational task: insert failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to create task")
		return
	}

	taskIDStr := uuidToString(task.ID)

	// Create a single task_stage row for the deliverable_type.
	_, err = h.Queries.CreateConversationalTaskStage(r.Context(), db.CreateConversationalTaskStageParams{
		TaskID:     task.ID,
		StageName:  req.DeliverableType,
		StageOrder: int32(deliverableOrder[req.DeliverableType]),
	})
	if err != nil {
		slog.Error("create conversational task: create stage failed",
			"task_id", taskIDStr, "stage", req.DeliverableType, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to create task stage")
		return
	}

	// Broadcast task_created event via WebSocket.
	if h.Hub != nil {
		payload := map[string]interface{}{
			"task_id":         taskIDStr,
			"agent_type":      req.AgentType,
			"prompt":          req.Prompt,
			"status":          task.Status,
			"deliverable_type": req.DeliverableType,
		}
		if agentID.Valid {
			payload["agent_id"] = req.AgentID
		}
		h.Hub.Broadcast(realtime.Event{
			Type:    "task_created",
			Payload: payload,
		})
	}

	writeJSON(w, http.StatusCreated, toTaskResponse(task))
}

// ---------------------------------------------------------------------------
// GET /api/tasks/{taskId}/stages/{stageName}/history
// ---------------------------------------------------------------------------

// GetStageHistory returns the prompt history for a specific task stage,
// ordered chronologically (oldest first).
func (h *UserHandler) GetStageHistory(w http.ResponseWriter, r *http.Request) {
	// Extract authenticated user ID.
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Extract task ID and stage name from URL params.
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "taskId is required")
		return
	}

	stageName := chi.URLParam(r, "stageName")
	if stageName == "" {
		writeErrorJSON(w, http.StatusBadRequest, "stageName is required")
		return
	}

	taskUUID, err := parseUUID(taskID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid task id")
		return
	}

	// Verify task exists and belongs to the authenticated user.
	task, err := h.Queries.GetTaskByID(r.Context(), taskUUID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeErrorJSON(w, http.StatusNotFound, "task not found")
			return
		}
		slog.Error("get stage history: get task failed", "task_id", taskID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "internal error")
		return
	}

	if uuidToString(task.UserID) != userID {
		writeErrorJSON(w, http.StatusForbidden, "forbidden")
		return
	}

	// Look up the task_stage using GetTaskStageByTaskAndName.
	stage, err := h.Queries.GetTaskStageByTaskAndName(r.Context(), db.GetTaskStageByTaskAndNameParams{
		TaskID:    taskUUID,
		StageName: stageName,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeErrorJSON(w, http.StatusNotFound, "stage not found")
			return
		}
		slog.Error("get stage history: get stage failed", "task_id", taskID, "stage", stageName, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Query prompt_history for the stage, ordered by created_at ASC.
	entries, err := h.Queries.ListPromptHistoryForStage(r.Context(), stage.ID)
	if err != nil {
		slog.Error("get stage history: list prompt history failed", "task_id", taskID, "stage", stageName, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Convert to response type.
	result := make([]PromptHistoryEntry, len(entries))
	for i, e := range entries {
		result[i] = PromptHistoryEntry{
			ID:          uuidToString(e.ID),
			TaskStageID: uuidToString(e.TaskStageID),
			TaskID:      uuidToString(e.TaskID),
			PromptText:  e.PromptText,
			CreatedAt:   e.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		}
		if e.OutputText.Valid {
			result[i].OutputText = &e.OutputText.String
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// POST /api/tasks/{taskId}/stages/{stageName}/follow-up
// ---------------------------------------------------------------------------

// FollowUpStage handles sending a follow-up message to refine a completed
// deliverable's output. It creates a new task with prior_session_id set to
// the stage's session_id, preserves workspace config from the original task,
// resets the stage status to pending for re-execution, and broadcasts a
// task_created WebSocket event.
func (h *UserHandler) FollowUpStage(w http.ResponseWriter, r *http.Request) {
	// Extract authenticated user ID.
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Extract task ID and stage name from URL params.
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "taskId is required")
		return
	}

	stageName := chi.URLParam(r, "stageName")
	if stageName == "" {
		writeErrorJSON(w, http.StatusBadRequest, "stageName is required")
		return
	}

	taskUUID, err := parseUUID(taskID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid task id")
		return
	}

	// Parse request body.
	var req FollowUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate prompt is non-empty.
	if err := req.Validate(); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	// Verify task exists and belongs to the user.
	task, err := h.Queries.GetTaskByID(r.Context(), taskUUID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeErrorJSON(w, http.StatusNotFound, "task not found")
			return
		}
		slog.Error("follow-up stage: get task failed", "task_id", taskID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "internal error")
		return
	}

	if uuidToString(task.UserID) != userID {
		writeErrorJSON(w, http.StatusForbidden, "forbidden")
		return
	}

	// Look up the task_stage using GetTaskStageByTaskAndName.
	stage, err := h.Queries.GetTaskStageByTaskAndName(r.Context(), db.GetTaskStageByTaskAndNameParams{
		TaskID:    taskUUID,
		StageName: stageName,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeErrorJSON(w, http.StatusNotFound, "stage not found")
			return
		}
		slog.Error("follow-up stage: get stage failed", "task_id", taskID, "stage", stageName, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Check stage status == "completed" (409 if not).
	if stage.Status != "completed" {
		writeErrorJSON(w, http.StatusConflict, "stage must be in completed status to send follow-up")
		return
	}

	// Get session_id and work_dir from the stage.
	sessionInfo, err := h.Queries.GetLatestSessionForStage(r.Context(), stage.ID)
	if err != nil {
		slog.Error("follow-up stage: get session info failed", "task_id", taskID, "stage", stageName, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Build the deliverables JSON storing the prior_session_id for the new task.
	// We store it as a JSON object with prior_session_id so the daemon can pick it up.
	deliverables := map[string]interface{}{
		"prior_session_id": sessionInfo.SessionID.String,
	}
	if sessionInfo.WorkDir.Valid {
		deliverables["prior_work_dir"] = sessionInfo.WorkDir.String
	}
	deliverablesJSON, err := json.Marshal(deliverables)
	if err != nil {
		slog.Error("follow-up stage: marshal deliverables failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to create follow-up task")
		return
	}

	// Create a new follow-up task preserving workspace config from the original task.
	followUpTask, err := h.Queries.CreateFollowUpTask(r.Context(), db.CreateFollowUpTaskParams{
		UserID:        task.UserID,
		AgentType:     task.AgentType,
		Prompt:        req.Prompt,
		AgentID:       task.AgentID,
		Deliverables:  deliverablesJSON,
		WorkspaceMode: task.WorkspaceMode,
		WorkspacePath: task.WorkspacePath,
		GitRepoUrl:    task.GitRepoUrl,
	})
	if err != nil {
		slog.Error("follow-up stage: create task failed", "task_id", taskID, "stage", stageName, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to create follow-up task")
		return
	}

	followUpTaskIDStr := uuidToString(followUpTask.ID)

	// Update task_stage status back to "pending" for re-execution.
	if err := h.Queries.UpdateStageStatus(r.Context(), db.UpdateStageStatusParams{
		ID:     stage.ID,
		Status: "pending",
	}); err != nil {
		slog.Error("follow-up stage: reset stage status failed",
			"task_id", taskID, "stage", stageName, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to reset stage for re-execution")
		return
	}

	// Broadcast task_created WebSocket event.
	if h.Hub != nil {
		payload := map[string]interface{}{
			"task_id":          followUpTaskIDStr,
			"agent_type":       followUpTask.AgentType,
			"prompt":           followUpTask.Prompt,
			"status":           followUpTask.Status,
			"deliverable_type": stageName,
			"is_follow_up":     true,
			"parent_task_id":   taskID,
		}
		if followUpTask.AgentID.Valid {
			payload["agent_id"] = uuidToString(followUpTask.AgentID)
		}
		h.Hub.Broadcast(realtime.Event{
			Type:    "task_created",
			Payload: payload,
		})
	}

	writeJSON(w, http.StatusCreated, toTaskResponse(followUpTask))
}
