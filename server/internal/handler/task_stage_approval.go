package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agenticflow/agenticflow/internal/middleware"
	db "github.com/agenticflow/agenticflow/pkg/db/generated"
)

// ---------------------------------------------------------------------------
// POST /api/tasks/{taskId}/stages/{stageName}/approve
// ---------------------------------------------------------------------------

// ApproveStage approves a workflow stage that is in awaiting_approval status.
// If a next stage exists, it is set to pending. If this is the last stage,
// the overall task is marked as completed.
func (h *UserHandler) ApproveStage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

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

	// Verify task exists and belongs to the user.
	task, err := h.Queries.GetTaskByID(r.Context(), taskUUID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeErrorJSON(w, http.StatusNotFound, "task not found")
			return
		}
		slog.Error("approve stage: get task failed", "task_id", taskID, "error", err)
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
		slog.Error("approve stage: list stages failed", "task_id", taskID, "error", err)
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
	var targetIdx int
	for i := range stages {
		if stages[i].StageName == stageName {
			targetStage = &stages[i]
			targetIdx = i
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

	// Set stage status to approved.
	if err := h.Queries.UpdateStageStatus(r.Context(), db.UpdateStageStatusParams{
		ID:     targetStage.ID,
		Status: "approved",
	}); err != nil {
		slog.Error("approve stage: update status failed", "task_id", taskID, "stage", stageName, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to approve stage")
		return
	}

	// Determine if there is a next stage.
	if targetIdx+1 < len(stages) {
		// Set the next stage to pending.
		nextStage := stages[targetIdx+1]
		if err := h.Queries.UpdateStageStatus(r.Context(), db.UpdateStageStatusParams{
			ID:     nextStage.ID,
			Status: "pending",
		}); err != nil {
			slog.Error("approve stage: advance next stage failed",
				"task_id", taskID, "next_stage", nextStage.StageName, "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "failed to advance to next stage")
			return
		}
	} else {
		// This is the last stage — mark the overall task as completed.
		if err := h.Queries.UpdateTaskCompleted(r.Context(), db.UpdateTaskCompletedParams{
			ID:            taskUUID,
			ExitCode:      pgtype.Int4{Int32: 0, Valid: true},
			OutputPreview: pgtype.Text{},
		}); err != nil {
			slog.Error("approve stage: complete task failed", "task_id", taskID, "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "failed to complete task")
			return
		}
	}

	// Broadcast stage_approved WebSocket event.
	if h.Hub != nil {
		h.Hub.BroadcastStageApproved(taskID, stageName)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"task_id":    taskID,
		"stage_name": stageName,
		"status":     "approved",
	})
}
