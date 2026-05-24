package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agenticflow/agenticflow/internal/middleware"
	"github.com/agenticflow/agenticflow/internal/realtime"
	"github.com/agenticflow/agenticflow/internal/service"
	db "github.com/agenticflow/agenticflow/pkg/db/generated"
)

// DaemonHandler holds dependencies for daemon API handlers.
type DaemonHandler struct {
	Queries              *db.Queries
	Hub                  *realtime.Hub
	AgentStatusService   *service.AgentStatusService
	SessionStateManager  *service.SessionStateManager
}

// NewDaemonHandler creates a new DaemonHandler.
func NewDaemonHandler(queries *db.Queries, hub *realtime.Hub) *DaemonHandler {
	return &DaemonHandler{Queries: queries, Hub: hub}
}

// ---------------------------------------------------------------------------
// Request/Response types
// ---------------------------------------------------------------------------

// AgentInfo represents a single agent entry in the register request.
type AgentInfo struct {
	Path    string `json:"path"`
	Model   string `json:"model"`
	Version string `json:"version"`
}

// DaemonRegisterReq is the request body for POST /api/daemon/register.
type DaemonRegisterReq struct {
	DaemonID   string               `json:"daemon_id"`
	DeviceName string               `json:"device_name"`
	CLIVersion string               `json:"cli_version"`
	Agents     map[string]AgentInfo `json:"agents"`
}

// DaemonDeregisterReq is the request body for POST /api/daemon/deregister.
type DaemonDeregisterReq struct {
	DaemonID string `json:"daemon_id"`
}

// DaemonHeartbeatReq is the request body for POST /api/daemon/heartbeat.
type DaemonHeartbeatReq struct {
	DaemonID string `json:"daemon_id"`
}

// TaskCompleteReq is the request body for POST /api/daemon/tasks/{taskId}/complete.
type TaskCompleteReq struct {
	Output   string `json:"output"`
	ExitCode int32  `json:"exit_code"`
}

// TaskFailReq is the request body for POST /api/daemon/tasks/{taskId}/fail.
type TaskFailReq struct {
	ErrorMessage string `json:"error_message"`
	ExitCode     int32  `json:"exit_code"`
}

// TaskMessageEntry represents a single message in the messages request.
type TaskMessageEntry struct {
	Sequence int32  `json:"sequence"`
	Stream   string `json:"stream"`
	Content  string `json:"content"`
}

// TaskMessagesReq is the request body for POST /api/daemon/tasks/{taskId}/messages.
type TaskMessagesReq struct {
	Messages []TaskMessageEntry `json:"messages"`
}

// TaskInputStateReq is the request body for POST /api/daemon/tasks/{taskId}/input-state.
type TaskInputStateReq struct {
	State string `json:"state"` // "waiting" or "cleared"
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// Register handles POST /api/daemon/register.
// It upserts the daemon and its agent runtimes, returning the runtime IDs.
func (h *DaemonHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req DaemonRegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.DaemonID = strings.TrimSpace(req.DaemonID)
	req.DeviceName = strings.TrimSpace(req.DeviceName)

	if req.DaemonID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "daemon_id is required")
		return
	}
	if len(req.Agents) == 0 {
		writeErrorJSON(w, http.StatusBadRequest, "at least one agent is required")
		return
	}

	userID := middleware.DaemonUserIDFromContext(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	// Upsert daemon.
	daemon, err := h.Queries.UpsertDaemon(r.Context(), db.UpsertDaemonParams{
		UserID:     userUUID,
		DaemonID:   req.DaemonID,
		DeviceName: req.DeviceName,
		CliVersion: pgtype.Text{String: req.CLIVersion, Valid: req.CLIVersion != ""},
	})
	if err != nil {
		slog.Error("daemon register: upsert daemon failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to register daemon")
		return
	}

	// Upsert each agent runtime.
	runtimeIDs := make(map[string]string, len(req.Agents))
	for provider, agent := range req.Agents {
		provider = strings.TrimSpace(provider)
		if provider == "" {
			continue
		}
		name := provider
		if req.DeviceName != "" {
			name = fmt.Sprintf("%s (%s)", provider, req.DeviceName)
		}

		rt, err := h.Queries.UpsertRuntime(r.Context(), db.UpsertRuntimeParams{
			DaemonID:   daemon.ID,
			Provider:   provider,
			Name:       name,
			Version:    pgtype.Text{String: agent.Version, Valid: agent.Version != ""},
			BinaryPath: pgtype.Text{String: agent.Path, Valid: agent.Path != ""},
			Status:     "available",
		})
		if err != nil {
			slog.Error("daemon register: upsert runtime failed", "provider", provider, "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "failed to register runtime: "+provider)
			return
		}
		runtimeIDs[provider] = uuidToString(rt.ID)
	}

	slog.Info("daemon registered",
		"daemon_id", req.DaemonID,
		"user_id", userID,
		"runtimes_count", len(runtimeIDs),
	)

	// Create default "Nexus" agent if the user has no agents yet.
	// This is non-blocking: errors are logged and do not affect the registration response.
	service.EnsureDefaultAgent(r.Context(), h.Queries, userUUID, runtimeIDs)

	// Broadcast daemon_connected event via WebSocket.
	if h.Hub != nil {
		h.Hub.Broadcast(realtime.Event{
			Type: "daemon_connected",
			Payload: map[string]interface{}{
				"daemon_id":   req.DaemonID,
				"device_name": req.DeviceName,
				"runtime_ids": runtimeIDs,
			},
		})
	}

	// Trigger agent status recomputation for all agents bound to this daemon's
	// runtimes. This runs asynchronously and completes within 2 seconds.
	if h.AgentStatusService != nil {
		h.AgentStatusService.ReconcileAgentsForDaemon(r.Context(), daemon.ID)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"daemon_db_id": uuidToString(daemon.ID),
		"runtime_ids":  runtimeIDs,
	})
}

// Deregister handles POST /api/daemon/deregister.
// It marks the daemon offline and deletes its runtimes.
func (h *DaemonHandler) Deregister(w http.ResponseWriter, r *http.Request) {
	var req DaemonDeregisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.DaemonID = strings.TrimSpace(req.DaemonID)
	if req.DaemonID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "daemon_id is required")
		return
	}

	userID := middleware.DaemonUserIDFromContext(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	// Look up the daemon by user_id + daemon_id.
	daemon, err := h.Queries.GetDaemonByDaemonID(r.Context(), db.GetDaemonByDaemonIDParams{
		UserID:   userUUID,
		DaemonID: req.DaemonID,
	})
	if err != nil {
		slog.Warn("daemon deregister: daemon not found", "daemon_id", req.DaemonID, "error", err)
		writeErrorJSON(w, http.StatusNotFound, "daemon not found")
		return
	}

	// Mark daemon offline.
	if err := h.Queries.UpdateDaemonStatus(r.Context(), db.UpdateDaemonStatusParams{
		ID:     daemon.ID,
		Status: "offline",
	}); err != nil {
		slog.Error("daemon deregister: update status failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to update daemon status")
		return
	}

	// Delete runtimes.
	if err := h.Queries.DeleteRuntimesByDaemon(r.Context(), daemon.ID); err != nil {
		slog.Error("daemon deregister: delete runtimes failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to delete runtimes")
		return
	}

	slog.Info("daemon deregistered", "daemon_id", req.DaemonID, "user_id", userID)

	// Broadcast daemon_disconnected event via WebSocket.
	if h.Hub != nil {
		h.Hub.Broadcast(realtime.Event{
			Type: "daemon_disconnected",
			Payload: map[string]interface{}{
				"daemon_id": req.DaemonID,
			},
		})
	}

	// Trigger agent status recomputation for all agents bound to this daemon's
	// runtimes. This runs asynchronously and completes within 2 seconds.
	// Note: Even though runtimes are deleted, agents still reference runtime_ids
	// that belonged to this daemon. The status derivation will detect the daemon
	// is offline and set agents to "offline" status.
	if h.AgentStatusService != nil {
		h.AgentStatusService.ReconcileAgentsForDaemon(r.Context(), daemon.ID)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Heartbeat handles POST /api/daemon/heartbeat.
// It updates the daemon's last_heartbeat_at timestamp.
func (h *DaemonHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	var req DaemonHeartbeatReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.DaemonID = strings.TrimSpace(req.DaemonID)
	if req.DaemonID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "daemon_id is required")
		return
	}

	userID := middleware.DaemonUserIDFromContext(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	// Look up the daemon to get its DB ID.
	daemon, err := h.Queries.GetDaemonByDaemonID(r.Context(), db.GetDaemonByDaemonIDParams{
		UserID:   userUUID,
		DaemonID: req.DaemonID,
	})
	if err != nil {
		slog.Warn("daemon heartbeat: daemon not found", "daemon_id", req.DaemonID, "error", err)
		writeErrorJSON(w, http.StatusNotFound, "daemon not found")
		return
	}

	// Update heartbeat.
	if err := h.Queries.UpdateDaemonHeartbeat(r.Context(), daemon.ID); err != nil {
		slog.Error("daemon heartbeat: update failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "heartbeat update failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PollTasks handles GET /api/daemon/tasks/poll.
// It claims a pending task matching the daemon's runtimes, enforcing per-agent
// concurrency limits via ClaimPendingTaskForRuntime. Each agent's
// max_concurrent_tasks is checked independently at the SQL level.
func (h *DaemonHandler) PollTasks(w http.ResponseWriter, r *http.Request) {
	userID := middleware.DaemonUserIDFromContext(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Get daemon ID from context (set by X-Daemon-ID header in middleware).
	daemonIDStr := middleware.DaemonIDFromContext(r.Context())
	if daemonIDStr == "" {
		writeErrorJSON(w, http.StatusBadRequest, "X-Daemon-ID header is required")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	// Look up the daemon to get its DB UUID.
	daemon, err := h.Queries.GetDaemonByDaemonID(r.Context(), db.GetDaemonByDaemonIDParams{
		UserID:   userUUID,
		DaemonID: daemonIDStr,
	})
	if err != nil {
		slog.Warn("poll tasks: daemon not found", "daemon_id", daemonIDStr, "error", err)
		writeErrorJSON(w, http.StatusNotFound, "daemon not found")
		return
	}

	// Get all runtimes registered for this daemon.
	runtimes, err := h.Queries.ListRuntimesByDaemon(r.Context(), daemon.ID)
	if err != nil || len(runtimes) == 0 {
		// No runtimes registered — nothing to claim.
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Try to claim a pending task for each runtime. The ClaimPendingTaskForRuntime
	// query enforces per-agent concurrency: it skips agents whose active running
	// task count equals their max_concurrent_tasks, and resumes assignment when
	// the count drops below the limit. Each agent's limit is enforced independently.
	var task db.Task
	var claimed bool
	for _, rt := range runtimes {
		task, err = h.Queries.ClaimPendingTaskForRuntime(r.Context(), db.ClaimPendingTaskForRuntimeParams{
			RuntimeID: rt.ID,
			DaemonID:  daemon.ID,
		})
		if err == nil {
			claimed = true
			break
		}
		// pgx.ErrNoRows means no eligible pending task for this runtime — try next.
	}

	if !claimed {
		// No pending task found for any runtime — return 204.
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Build the response. If the task has an associated agent, include the
	// agent configuration so the daemon can inject instructions, env, args, model.
	response := map[string]interface{}{
		"id":         uuidToString(task.ID),
		"agent_type": task.AgentType,
		"prompt":     task.Prompt,
		"status":     task.Status,
	}

	if task.AgentID.Valid {
		agent, agentErr := h.Queries.GetAgent(r.Context(), task.AgentID)
		if agentErr == nil {
			agentData := map[string]interface{}{
				"id":           uuidToString(agent.ID),
				"name":         agent.Name,
				"instructions": agent.Instructions,
			}
			if agent.Model.Valid && agent.Model.String != "" {
				agentData["model"] = agent.Model.String
			}
			// Parse custom_env from JSONB.
			if len(agent.CustomEnv) > 0 {
				var customEnv map[string]string
				if json.Unmarshal(agent.CustomEnv, &customEnv) == nil && len(customEnv) > 0 {
					agentData["custom_env"] = customEnv
				}
			}
			// Parse custom_args from JSONB.
			if len(agent.CustomArgs) > 0 {
				var customArgs []string
				if json.Unmarshal(agent.CustomArgs, &customArgs) == nil && len(customArgs) > 0 {
					agentData["custom_args"] = customArgs
				}
			}
			response["agent"] = agentData
		} else {
			slog.Warn("poll tasks: failed to load agent for claimed task",
				"task_id", uuidToString(task.ID),
				"agent_id", uuidToString(task.AgentID),
				"error", agentErr,
			)
		}
	}

	writeJSON(w, http.StatusOK, response)
}

// StartTask handles POST /api/daemon/tasks/{taskId}/start.
// It marks the task as running.
func (h *DaemonHandler) StartTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "taskId is required")
		return
	}

	taskUUID, err := parseUUID(taskID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid task id")
		return
	}

	// Get daemon context for the daemon_id and runtime_id.
	daemonIDStr := middleware.DaemonIDFromContext(r.Context())
	userID := middleware.DaemonUserIDFromContext(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	// Look up daemon.
	daemon, err := h.Queries.GetDaemonByDaemonID(r.Context(), db.GetDaemonByDaemonIDParams{
		UserID:   userUUID,
		DaemonID: daemonIDStr,
	})
	if err != nil {
		slog.Warn("start task: daemon not found", "daemon_id", daemonIDStr, "error", err)
		writeErrorJSON(w, http.StatusNotFound, "daemon not found")
		return
	}

	// Get first runtime for this daemon.
	runtimes, err := h.Queries.ListRuntimesByDaemon(r.Context(), daemon.ID)
	if err != nil || len(runtimes) == 0 {
		writeErrorJSON(w, http.StatusInternalServerError, "no runtimes found")
		return
	}

	if err := h.Queries.UpdateTaskStarted(r.Context(), db.UpdateTaskStartedParams{
		DaemonID:       daemon.ID,
		AgentRuntimeID: runtimes[0].ID,
		ID:             taskUUID,
	}); err != nil {
		slog.Error("start task: update failed", "task_id", taskID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to start task")
		return
	}

	// Broadcast task_started event via WebSocket.
	if h.Hub != nil {
		h.Hub.Broadcast(realtime.Event{
			Type: "task_started",
			Payload: map[string]interface{}{
				"task_id":   taskID,
				"daemon_id": daemonIDStr,
			},
		})
	}

	// Trigger agent status recomputation for the task's owning agent.
	// The agent may transition from idle → working.
	if h.AgentStatusService != nil {
		h.AgentStatusService.ReconcileAgentForTask(r.Context(), taskUUID)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

// CompleteTask handles POST /api/daemon/tasks/{taskId}/complete.
// It marks the task as completed with output and exit code.
func (h *DaemonHandler) CompleteTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "taskId is required")
		return
	}

	taskUUID, err := parseUUID(taskID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid task id")
		return
	}

	var req TaskCompleteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Truncate output preview to 1024 chars.
	outputPreview := req.Output
	if len(outputPreview) > 1024 {
		outputPreview = outputPreview[:1024]
	}

	if err := h.Queries.UpdateTaskCompleted(r.Context(), db.UpdateTaskCompletedParams{
		ID:            taskUUID,
		ExitCode:      pgtype.Int4{Int32: req.ExitCode, Valid: true},
		OutputPreview: pgtype.Text{String: outputPreview, Valid: outputPreview != ""},
	}); err != nil {
		slog.Error("complete task: update failed", "task_id", taskID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to complete task")
		return
	}

	// Clear session state on terminal transition.
	if h.SessionStateManager != nil {
		h.SessionStateManager.ClearState(taskID)
	}

	// Broadcast task_completed event via WebSocket.
	if h.Hub != nil {
		h.Hub.Broadcast(realtime.Event{
			Type: "task_completed",
			Payload: map[string]interface{}{
				"task_id":   taskID,
				"exit_code": req.ExitCode,
			},
		})
	}

	// Trigger agent status recomputation for the task's owning agent.
	// The agent may transition from working → idle.
	if h.AgentStatusService != nil {
		h.AgentStatusService.ReconcileAgentForTask(r.Context(), taskUUID)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

// FailTask handles POST /api/daemon/tasks/{taskId}/fail.
// It marks the task as failed with an error message and exit code.
func (h *DaemonHandler) FailTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "taskId is required")
		return
	}

	taskUUID, err := parseUUID(taskID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid task id")
		return
	}

	var req TaskFailReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.Queries.UpdateTaskFailed(r.Context(), db.UpdateTaskFailedParams{
		ID:           taskUUID,
		ExitCode:     pgtype.Int4{Int32: req.ExitCode, Valid: true},
		ErrorMessage: pgtype.Text{String: req.ErrorMessage, Valid: req.ErrorMessage != ""},
	}); err != nil {
		slog.Error("fail task: update failed", "task_id", taskID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to fail task")
		return
	}

	// Clear session state on terminal transition.
	if h.SessionStateManager != nil {
		h.SessionStateManager.ClearState(taskID)
	}

	// Broadcast task_failed event via WebSocket.
	if h.Hub != nil {
		h.Hub.Broadcast(realtime.Event{
			Type: "task_failed",
			Payload: map[string]interface{}{
				"task_id":       taskID,
				"exit_code":     req.ExitCode,
				"error_message": req.ErrorMessage,
			},
		})
	}

	// Trigger agent status recomputation for the task's owning agent.
	// The agent may transition from working → idle.
	if h.AgentStatusService != nil {
		h.AgentStatusService.ReconcileAgentForTask(r.Context(), taskUUID)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "failed"})
}

// ReportTaskMessages handles POST /api/daemon/tasks/{taskId}/messages.
// It stores streaming output messages for a task.
func (h *DaemonHandler) ReportTaskMessages(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "taskId is required")
		return
	}

	taskUUID, err := parseUUID(taskID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid task id")
		return
	}

	var req TaskMessagesReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Messages) == 0 {
		writeErrorJSON(w, http.StatusBadRequest, "messages array is required")
		return
	}

	for _, msg := range req.Messages {
		stream := strings.TrimSpace(msg.Stream)
		if stream != "stdout" && stream != "stderr" && stream != "stdin" {
			stream = "stdout"
		}

		_, err := h.Queries.CreateTaskMessageIdempotent(r.Context(), db.CreateTaskMessageIdempotentParams{
			TaskID:   taskUUID,
			Sequence: msg.Sequence,
			Stream:   stream,
			Content:  msg.Content,
		})
		if err != nil {
			slog.Error("report messages: create failed",
				"task_id", taskID,
				"sequence", msg.Sequence,
				"error", err,
			)
			writeErrorJSON(w, http.StatusInternalServerError, "failed to store message")
			return
		}

		// Broadcast task_output event for each message via WebSocket.
		if h.Hub != nil {
			h.Hub.Broadcast(realtime.Event{
				Type: "task_output",
				Payload: map[string]interface{}{
					"task_id":  taskID,
					"sequence": msg.Sequence,
					"stream":   stream,
					"content":  msg.Content,
				},
			})
		}
	}

	// Update session state to "producing_output" since we received new task output.
	if h.SessionStateManager != nil {
		h.SessionStateManager.SetState(taskID, service.SessionStateProducingOutput)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"received": len(req.Messages),
	})
}

// ReportInputState handles POST /api/daemon/tasks/{taskId}/input-state.
// Called by the daemon when the InputDetector signals a state change.
func (h *DaemonHandler) ReportInputState(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "taskId is required")
		return
	}

	var req TaskInputStateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.State = strings.TrimSpace(req.State)
	if req.State != "waiting" && req.State != "cleared" {
		writeErrorJSON(w, http.StatusBadRequest, "state must be \"waiting\" or \"cleared\"")
		return
	}

	// Update session state and broadcast appropriate WebSocket event.
	switch req.State {
	case "waiting":
		if h.SessionStateManager != nil {
			h.SessionStateManager.SetState(taskID, service.SessionStateWaitingForInput)
		}
		if h.Hub != nil {
			h.Hub.Broadcast(realtime.Event{
				Type: "input_requested",
				Payload: map[string]interface{}{
					"task_id": taskID,
				},
			})
		}
	case "cleared":
		if h.SessionStateManager != nil {
			h.SessionStateManager.SetState(taskID, service.SessionStateProducingOutput)
		}
		if h.Hub != nil {
			h.Hub.Broadcast(realtime.Event{
				Type: "input_cleared",
				Payload: map[string]interface{}{
					"task_id": taskID,
				},
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// writeErrorJSON writes a JSON error response.
func writeErrorJSON(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
