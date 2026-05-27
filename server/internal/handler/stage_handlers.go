package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agenticflow/agenticflow/internal/middleware"
	db "github.com/agenticflow/agenticflow/pkg/db/generated"
)

// isConversationalTask checks whether the given stages belong to a conversational
// task by checking if any stage_name is a valid deliverable type (plan, design,
// tasks, execution). Conversational tasks use the follow-up model instead of
// approval gates.
func isConversationalTask(stages []db.TaskStage) bool {
	for _, s := range stages {
		if ValidDeliverableTypes[s.StageName] {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// POST /api/tasks/{taskId}/stages/{stageName}/reject
// ---------------------------------------------------------------------------

// StageRejectRequest is the request body for rejecting a stage.
type StageRejectRequest struct {
	Feedback string `json:"feedback"`
}

// RejectStage handles POST /api/tasks/{taskId}/stages/{stageName}/reject.
// It verifies the stage exists and is in awaiting_approval status, requires
// non-empty feedback, sets the stage to rejected with feedback stored, then
// transitions it back to pending for re-execution, and broadcasts a
// stage_rejected WebSocket event.
func (h *UserHandler) RejectStage(w http.ResponseWriter, r *http.Request) {
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
	var req StageRejectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate feedback is non-empty.
	if strings.TrimSpace(req.Feedback) == "" {
		writeErrorJSON(w, http.StatusBadRequest, "feedback is required when rejecting a stage")
		return
	}

	// Verify task exists and belongs to the user.
	task, err := h.Queries.GetTaskByID(r.Context(), taskUUID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeErrorJSON(w, http.StatusNotFound, "task not found")
			return
		}
		slog.Error("reject stage: get task failed", "task_id", taskID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "internal error")
		return
	}

	if uuidToString(task.UserID) != userID {
		writeErrorJSON(w, http.StatusForbidden, "forbidden")
		return
	}

	// List all stages for the task.
	stages, err := h.Queries.ListStagesForTask(r.Context(), taskUUID)
	if err != nil {
		slog.Error("reject stage: list stages failed", "task_id", taskID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Guard: conversational tasks do not support approval gates.
	if isConversationalTask(stages) {
		writeErrorJSON(w, http.StatusConflict, "approval gates not supported for conversational tasks")
		return
	}

	// Find the target stage by name.
	var targetStage *db.TaskStage
	for i := range stages {
		if stages[i].StageName == stageName {
			targetStage = &stages[i]
			break
		}
	}

	if targetStage == nil {
		writeErrorJSON(w, http.StatusNotFound, "stage not found")
		return
	}

	// Verify stage is in awaiting_approval status.
	if targetStage.Status != "awaiting_approval" {
		writeErrorJSON(w, http.StatusConflict, "stage is not awaiting approval")
		return
	}

	// Step 1: Set stage status to "rejected".
	if err := h.Queries.UpdateStageStatus(r.Context(), db.UpdateStageStatusParams{
		ID:     targetStage.ID,
		Status: "rejected",
	}); err != nil {
		slog.Error("reject stage: update status to rejected failed",
			"task_id", taskID, "stage", stageName, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to reject stage")
		return
	}

	// Step 2: Store feedback.
	if err := h.Queries.UpdateStageFeedback(r.Context(), db.UpdateStageFeedbackParams{
		ID:       targetStage.ID,
		Feedback: pgtype.Text{String: req.Feedback, Valid: true},
	}); err != nil {
		slog.Error("reject stage: store feedback failed",
			"task_id", taskID, "stage", stageName, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to store feedback")
		return
	}

	// Step 3: Transition stage back to "pending" for re-execution.
	if err := h.Queries.UpdateStageStatus(r.Context(), db.UpdateStageStatusParams{
		ID:     targetStage.ID,
		Status: "pending",
	}); err != nil {
		slog.Error("reject stage: transition to pending failed",
			"task_id", taskID, "stage", stageName, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to re-queue stage")
		return
	}

	// Step 4: Broadcast stage_rejected WebSocket event.
	if h.Hub != nil {
		h.Hub.BroadcastStageRejected(taskID, stageName, req.Feedback)
	}

	// Return success response.
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"task_id":    taskID,
		"stage_name": stageName,
		"status":     "pending",
		"feedback":   req.Feedback,
	})
}
