package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/agenticflow/agenticflow/internal/middleware"
	"github.com/agenticflow/agenticflow/internal/realtime"
)

// maxInputTextLength is the maximum allowed length for task input text.
const maxInputTextLength = 10000

// TaskInputRequest is the request body for POST /api/tasks/{id}/input.
type TaskInputRequest struct {
	Text string `json:"text"`
}

// TaskInputResponse is the response for successful input relay.
type TaskInputResponse struct {
	Status    string `json:"status"`    // "delivered"
	TaskID    string `json:"task_id"`
	Timestamp string `json:"timestamp"`
}

// SendTaskInput handles POST /api/tasks/{id}/input.
// It validates the request, checks task ownership and status, then relays
// the input to the daemon via WebSocket.
func (h *UserHandler) SendTaskInput(w http.ResponseWriter, r *http.Request) {
	// Extract authenticated user ID.
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Extract task ID from URL.
	taskID := chi.URLParam(r, "id")
	if taskID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "task id is required")
		return
	}

	taskUUID, err := parseUUID(taskID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid task id")
		return
	}

	// 1. Parse and validate request body.
	var req TaskInputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// 2. Validate text: non-empty, ≤ 10,000 characters.
	trimmedText := strings.TrimSpace(req.Text)
	if trimmedText == "" {
		writeErrorJSON(w, http.StatusBadRequest, "text must be between 1 and 10000 characters")
		return
	}
	if utf8.RuneCountInString(req.Text) > maxInputTextLength {
		writeErrorJSON(w, http.StatusBadRequest, "text must be between 1 and 10000 characters")
		return
	}

	// 3. Load task from DB, verify user ownership.
	task, err := h.Queries.GetTaskByID(r.Context(), taskUUID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeErrorJSON(w, http.StatusNotFound, "task not found")
			return
		}
		slog.Error("send task input: get task failed", "task_id", taskID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Verify ownership: the authenticated user must own the task.
	if uuidToString(task.UserID) != userID {
		writeErrorJSON(w, http.StatusForbidden, "forbidden")
		return
	}

	// 4. Verify task status == "running".
	if task.Status != "running" {
		writeErrorJSON(w, http.StatusConflict, "task is not running")
		return
	}

	// 5. Resolve daemon_id, check daemon online.
	if !task.DaemonID.Valid {
		writeErrorJSON(w, http.StatusBadGateway, "daemon is unreachable")
		return
	}

	// Look up the daemon to get its daemon_id string (used for WebSocket routing).
	daemon, err := h.Queries.GetDaemonByID(r.Context(), task.DaemonID)
	if err != nil {
		slog.Error("send task input: get daemon failed", "task_id", taskID, "error", err)
		writeErrorJSON(w, http.StatusBadGateway, "daemon is unreachable")
		return
	}

	// Check if the daemon is connected to the WebSocket hub.
	if h.Hub == nil || !h.Hub.IsDaemonConnected(daemon.DaemonID) {
		writeErrorJSON(w, http.StatusBadGateway, "daemon is unreachable")
		return
	}

	// 6. Send task_input event to daemon via Hub.SendToDaemon().
	h.Hub.SendToDaemon(daemon.DaemonID, realtime.Event{
		Type: realtime.EventTaskInput,
		Payload: map[string]interface{}{
			"task_id": taskID,
			"text":    req.Text,
		},
	})

	// 7. Return HTTP 202 with confirmation payload.
	writeJSON(w, http.StatusAccepted, TaskInputResponse{
		Status:    "delivered",
		TaskID:    taskID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
