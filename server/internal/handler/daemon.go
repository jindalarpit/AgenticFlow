package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agenticflow/agenticflow/server/internal/middleware"
	"github.com/agenticflow/agenticflow/server/internal/realtime"
	"github.com/agenticflow/agenticflow/server/internal/service"
	"github.com/agenticflow/agenticflow/shared/api"
	"github.com/agenticflow/agenticflow/shared/constants"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// DaemonHandler holds dependencies for daemon API handlers.
type DaemonHandler struct {
	Queries              db.Querier
	Hub                  *realtime.Hub
	AgentStatusService   *service.AgentStatusService
	SessionStateManager  *service.SessionStateManager
}

// NewDaemonHandler creates a new DaemonHandler.
func NewDaemonHandler(queries db.Querier, hub *realtime.Hub) *DaemonHandler {
	return &DaemonHandler{Queries: queries, Hub: hub}
}

// ---------------------------------------------------------------------------
// Request/Response types — aliased from shared/api for backward compatibility
// ---------------------------------------------------------------------------

// AgentInfo represents a single agent entry in the register request.
// Aliased from shared/api.AgentInfo.
type AgentInfo = api.AgentInfo

// DaemonRegisterReq is the request body for POST /api/daemon/register.
// Aliased from shared/api.DaemonRegisterRequest.
type DaemonRegisterReq = api.DaemonRegisterRequest

// DaemonDeregisterReq is the request body for POST /api/daemon/deregister.
type DaemonDeregisterReq struct {
	DaemonID string `json:"daemon_id"`
}

// DaemonHeartbeatReq is the request body for POST /api/daemon/heartbeat.
type DaemonHeartbeatReq struct {
	DaemonID string `json:"daemon_id"`
}

// TaskCompleteReq is the request body for POST /api/daemon/tasks/{taskId}/complete.
// Aliased from shared/api.TaskCompleteRequest.
type TaskCompleteReq = api.TaskCompleteRequest

// TaskFailReq is the request body for POST /api/daemon/tasks/{taskId}/fail.
// Aliased from shared/api.TaskFailRequest.
type TaskFailReq = api.TaskFailRequest

// TaskMessageEntry represents a single message in the messages request.
// Aliased from shared/api.TaskMessageEntry.
type TaskMessageEntry = api.TaskMessageEntry

// TaskMessagesReq is the request body for POST /api/daemon/tasks/{taskId}/messages.
// Aliased from shared/api.TaskMessagesRequest.
type TaskMessagesReq = api.TaskMessagesRequest

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
		Status: constants.DaemonStatusOffline,
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
//
// For staged tasks (those with task_stage rows), the response includes:
//   - current_stage: the lowest-order pending stage (name, order, status)
//   - prior_stages: completed/approved stage outputs (name, order, status, output_content)
//
// For all tasks (staged and single-pass), the response includes:
//   - workspace_mode: the task's workspace mode ("isolated" or "existing")
//   - workspace_path: the task's workspace path (if set)
//
// For single-pass tasks (no stages), current_stage and prior_stages are omitted
// for backward compatibility.
//
// Only tasks where the next stage is "pending" are eligible for claiming.
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
	//
	// We first try ClaimPendingTaskWithStage (for staged tasks with a pending next
	// stage), then fall back to ClaimPendingTaskForRuntime (for single-pass tasks).
	var task db.Task
	var claimed bool
	var isStagedTask bool

	// First pass: try to claim a staged task with a pending stage.
	for _, rt := range runtimes {
		task, err = h.Queries.ClaimPendingTaskWithStage(r.Context(), db.ClaimPendingTaskWithStageParams{
			DaemonID:       daemon.ID,
			AgentRuntimeID: rt.ID,
		})
		if err == nil {
			claimed = true
			isStagedTask = true
			break
		}
	}

	// Second pass: try to claim a single-pass task (no stages).
	if !claimed {
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

	// Always include workspace_mode and workspace_path for all tasks.
	response["workspace_mode"] = task.WorkspaceMode
	if task.WorkspacePath.Valid && task.WorkspacePath.String != "" {
		response["workspace_path"] = task.WorkspacePath.String
	}

	// For staged tasks, include current_stage and prior_stages.
	if isStagedTask {
		h.enrichPollResponseWithStages(r, response, task)
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

			// Enrich with agent skills (always include, empty array if none).
			skills := []api.TaskSkill{}
			skillRows, skillErr := h.Queries.GetAgentSkillsWithFiles(r.Context(), agent.ID)
			if skillErr == nil {
				for _, row := range skillRows {
					skill := api.TaskSkill{
						Name:        row.Name,
						Description: row.Description,
						Content:     row.Content,
					}
					// Fetch supporting files for this skill.
					fileRows, fileErr := h.Queries.GetSkillFilesWithContent(r.Context(), row.ID)
					if fileErr == nil && len(fileRows) > 0 {
						for _, f := range fileRows {
							skill.Files = append(skill.Files, api.TaskSkillFile{
								Path:    f.Path,
								Content: f.Content,
							})
						}
					}
					skills = append(skills, skill)
				}
			} else {
				slog.Warn("poll tasks: failed to load skills for agent",
					"agent_id", uuidToString(agent.ID),
					"error", skillErr,
				)
			}
			agentData["skills"] = skills

			// Include mcp_config from agent record (omit when null).
			if len(agent.McpConfig) > 0 {
				agentData["mcp_config"] = json.RawMessage(agent.McpConfig)
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

// enrichPollResponseWithStages adds stage-specific fields to the poll response
// for tasks that have workflow stages. It detects whether the task is a
// conversational task (stage_name is a valid deliverable type) and branches:
//
// For conversational tasks:
//   - deliverable_type: the stage_name (plan, design, tasks, execution)
//   - prior_session_id: from the task's deliverables JSON (if follow-up)
//   - prior_context: from the task's deliverables JSON (if first message)
//   - workspace_config: {git_repo_url, local_directory_path} for execution type
//   - prior_work_dir: from the task_stage (if available from prior execution)
//
// For legacy staged tasks:
//   - current_stage: {name, order, status} of the lowest-order pending stage
//   - prior_stages: [{name, order, status, output_content}] of completed/approved stages
func (h *DaemonHandler) enrichPollResponseWithStages(r *http.Request, response map[string]interface{}, task db.Task) {
	ctx := r.Context()

	// Get the next pending stage (lowest order).
	nextStage, err := h.Queries.GetNextPendingStage(ctx, task.ID)
	if err != nil {
		slog.Warn("poll tasks: failed to get next pending stage",
			"task_id", uuidToString(task.ID),
			"error", err,
		)
		return
	}

	// Check if this is a conversational task by examining the stage_name.
	if ValidDeliverableTypes[nextStage.StageName] {
		h.enrichPollResponseForConversationalTask(response, task, nextStage)
		return
	}

	// Legacy staged task: get completed/approved prior stages for context.
	completedStages, err := h.Queries.GetCompletedStagesForTask(ctx, task.ID)
	if err != nil {
		slog.Warn("poll tasks: failed to get completed stages",
			"task_id", uuidToString(task.ID),
			"error", err,
		)
		// Non-fatal: continue without prior stages.
		completedStages = nil
	}

	// Use the extracted pure functions for response construction.
	enrichResponseWithStageFields(response, nextStage, completedStages)
}

// enrichPollResponseForConversationalTask adds conversational task fields to
// the poll response. This is called when the claimed task has a stage_name
// that is a valid deliverable type (plan, design, tasks, execution).
//
// The deliverables column stores either:
//   - A JSON array of strings (prior_context for first messages)
//   - A JSON object with "prior_session_id" and optionally "prior_work_dir" (for follow-ups)
func (h *DaemonHandler) enrichPollResponseForConversationalTask(response map[string]interface{}, task db.Task, stage db.TaskStage) {
	// Always include deliverable_type.
	response["deliverable_type"] = stage.StageName

	// Parse the deliverables column to extract prior_session_id or prior_context.
	if len(task.Deliverables) > 0 {
		h.parseConversationalDeliverables(response, task.Deliverables)
	}

	// Include workspace_config for execution-type tasks.
	if stage.StageName == "execution" {
		wsConfig := map[string]interface{}{
			"local_directory_path": "",
		}
		if task.WorkspacePath.Valid && task.WorkspacePath.String != "" {
			wsConfig["local_directory_path"] = task.WorkspacePath.String
		}
		if task.GitRepoUrl.Valid && task.GitRepoUrl.String != "" {
			wsConfig["git_repo_url"] = task.GitRepoUrl.String
		}
		response["workspace_config"] = wsConfig
	}

	// Include prior_work_dir from the task_stage if available.
	if stage.WorkDir.Valid && stage.WorkDir.String != "" {
		response["prior_work_dir"] = stage.WorkDir.String
	}
}

// parseConversationalDeliverables parses the task's deliverables JSON and adds
// the appropriate fields to the poll response.
//
// Two formats are supported:
//   - JSON array of strings → prior_context (first message with context from prior deliverables)
//   - JSON object with "prior_session_id" → prior_session_id (follow-up message)
func (h *DaemonHandler) parseConversationalDeliverables(response map[string]interface{}, deliverables []byte) {
	// Try parsing as a JSON array first (prior_context).
	var priorContext []string
	if err := json.Unmarshal(deliverables, &priorContext); err == nil {
		// Successfully parsed as array. Only include if non-empty.
		if len(priorContext) > 0 {
			response["prior_context"] = priorContext
		}
		return
	}

	// Try parsing as a JSON object (follow-up with prior_session_id).
	var obj map[string]interface{}
	if err := json.Unmarshal(deliverables, &obj); err == nil {
		if sessionID, ok := obj["prior_session_id"].(string); ok && sessionID != "" {
			response["prior_session_id"] = sessionID
		}
		// prior_work_dir from deliverables (in case stage doesn't have it yet).
		if workDir, ok := obj["prior_work_dir"].(string); ok && workDir != "" {
			// Only set if not already set from the stage.
			if _, exists := response["prior_work_dir"]; !exists {
				response["prior_work_dir"] = workDir
			}
		}
	}
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

	// For staged tasks, transition the next pending stage to "running" and
	// broadcast stage_started. This is a best-effort operation — if it fails,
	// the task start itself still succeeds.
	var deliverableTypeForEvent string
	nextStage, err := h.Queries.GetNextPendingStage(r.Context(), taskUUID)
	if err == nil {
		// Update stage status to running.
		_ = h.Queries.UpdateStageStatus(r.Context(), db.UpdateStageStatusParams{
			ID:     nextStage.ID,
			Status: "running",
		})

		// Determine if this is a conversational task (stage_name is a valid deliverable type).
		if ValidDeliverableTypes[nextStage.StageName] {
			deliverableTypeForEvent = nextStage.StageName
		} else {
			// Non-conversational staged task: broadcast stage_started event.
			if h.Hub != nil {
				h.Hub.BroadcastStageStarted(taskID, nextStage.StageName)
			}
		}
	}

	// Broadcast task_started event via WebSocket.
	// For conversational tasks, include the deliverable_type.
	if h.Hub != nil {
		h.Hub.BroadcastTaskStarted(taskID, daemonIDStr, deliverableTypeForEvent)
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
//
// For conversational tasks (those with a task_stage whose stage_name is a valid
// deliverable type), this handler additionally:
//   - Updates the task_stage with session_id, work_dir, output_content, status=completed
//   - Inserts a prompt_history entry with the task's prompt and the output
//   - Broadcasts task_completed with deliverable_type and output_content
//
// For non-conversational tasks, the existing completion flow is preserved.
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

	// Check if this is a conversational task by looking for a running stage
	// with a valid deliverable type. If found, perform conversational completion.
	var deliverableType string
	stages, stagesErr := h.Queries.ListStagesForTask(r.Context(), taskUUID)
	if stagesErr == nil && len(stages) > 0 {
		for _, stage := range stages {
			if stage.Status == "running" && ValidDeliverableTypes[stage.StageName] {
				deliverableType = stage.StageName
				h.completeConversationalStage(r, taskUUID, stage, req)
				break
			}
		}
	}

	// Broadcast task_completed event via WebSocket.
	// For conversational tasks, includes deliverable_type and output_content.
	if h.Hub != nil {
		h.Hub.BroadcastTaskCompleted(taskID, req.ExitCode, deliverableType, req.Output)
	}

	// Trigger agent status recomputation for the task's owning agent.
	// The agent may transition from working → idle.
	if h.AgentStatusService != nil {
		h.AgentStatusService.ReconcileAgentForTask(r.Context(), taskUUID)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

// completeConversationalStage handles the conversational-specific completion
// logic: updating the task_stage with session_id, work_dir, and output_content,
// and inserting a prompt_history entry.
func (h *DaemonHandler) completeConversationalStage(r *http.Request, taskUUID pgtype.UUID, stage db.TaskStage, req TaskCompleteReq) {
	ctx := r.Context()

	// Update the task_stage with completion data.
	if err := h.Queries.UpdateStageCompletion(ctx, db.UpdateStageCompletionParams{
		ID:            stage.ID,
		OutputContent: pgtype.Text{String: req.Output, Valid: req.Output != ""},
		SessionID:     pgtype.Text{String: req.SessionID, Valid: req.SessionID != ""},
		WorkDir:       pgtype.Text{String: req.WorkDir, Valid: req.WorkDir != ""},
	}); err != nil {
		slog.Error("complete task: update stage completion failed",
			"task_id", uuidToString(taskUUID),
			"stage_id", uuidToString(stage.ID),
			"error", err,
		)
		// Non-fatal: the task itself is already marked completed.
		return
	}

	// Look up the task to get the prompt text for the history entry.
	task, err := h.Queries.GetTaskByID(ctx, taskUUID)
	if err != nil {
		slog.Error("complete task: failed to get task for prompt history",
			"task_id", uuidToString(taskUUID),
			"error", err,
		)
		return
	}

	// Insert a prompt_history entry recording this turn.
	_, err = h.Queries.CreatePromptHistoryEntry(ctx, db.CreatePromptHistoryEntryParams{
		TaskStageID: stage.ID,
		TaskID:      taskUUID,
		PromptText:  task.Prompt,
		OutputText:  pgtype.Text{String: req.Output, Valid: req.Output != ""},
	})
	if err != nil {
		slog.Error("complete task: failed to create prompt history entry",
			"task_id", uuidToString(taskUUID),
			"stage_id", uuidToString(stage.ID),
			"error", err,
		)
		// Non-fatal: the stage completion itself succeeded.
	}
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

	// Determine deliverable_type for conversational tasks.
	var deliverableType string
	stages, stagesErr := h.Queries.ListStagesForTask(r.Context(), taskUUID)
	if stagesErr == nil && len(stages) > 0 {
		deliverableType = parseDeliverableTypeFromStages(stages)
		// For conversational tasks with a running stage, mark it as failed.
		for _, stage := range stages {
			if stage.Status == "running" && ValidDeliverableTypes[stage.StageName] {
				_ = h.Queries.UpdateStageStatus(r.Context(), db.UpdateStageStatusParams{
					ID:     stage.ID,
					Status: "failed",
				})
				break
			}
		}
	}

	// Broadcast task_failed event via WebSocket.
	// For conversational tasks, includes deliverable_type.
	if h.Hub != nil {
		h.Hub.BroadcastTaskFailed(taskID, req.ExitCode, req.ErrorMessage, deliverableType)
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
		// Detect structured vs legacy format based on presence of "type" field.
		if msg.Type != "" {
			// Structured format: use seq, type, tool, content, input, output.
			seq := msg.Seq
			if seq == 0 {
				seq = msg.Sequence // fallback to legacy field
			}

			var inputJSON []byte
			if msg.Input != nil {
				inputJSON, _ = json.Marshal(msg.Input)
			}

			_, err := h.Queries.CreateStructuredTaskMessage(r.Context(), db.CreateStructuredTaskMessageParams{
				TaskID:   taskUUID,
				Sequence: seq,
				Stream:   pgtype.Text{}, // NULL for structured messages
				Content:  pgtype.Text{String: msg.Content, Valid: msg.Content != ""},
				Type:     msg.Type,
				Tool:     pgtype.Text{String: msg.Tool, Valid: msg.Tool != ""},
				Input:    inputJSON,
				Output:   pgtype.Text{String: msg.Output, Valid: msg.Output != ""},
			})
			if err != nil {
				slog.Error("report messages: create structured failed",
					"task_id", taskID,
					"sequence", seq,
					"error", err,
				)
				writeErrorJSON(w, http.StatusInternalServerError, "failed to store message")
				return
			}

			// Broadcast structured task_output event via WebSocket.
			if h.Hub != nil {
				payload := map[string]interface{}{
					"task_id":  taskID,
					"sequence": seq,
					"type":     msg.Type,
				}
				switch msg.Type {
				case "tool_use":
					payload["tool"] = msg.Tool
					if msg.Input != nil {
						payload["input"] = msg.Input
					}
				case "tool_result":
					payload["tool"] = msg.Tool
					payload["output"] = msg.Output
				case "text", "thinking", "error":
					payload["content"] = msg.Content
				case "status":
					payload["content"] = msg.Content
				}
				h.Hub.Broadcast(realtime.Event{
					Type:    "task_output",
					Payload: payload,
				})
			}
		} else {
			// Legacy format: use sequence, stream, content.
			stream := strings.TrimSpace(msg.Stream)
			if stream != "stdout" && stream != "stderr" && stream != "stdin" {
				stream = "stdout"
			}

			_, err := h.Queries.CreateTaskMessageIdempotent(r.Context(), db.CreateTaskMessageIdempotentParams{
				TaskID:   taskUUID,
				Sequence: msg.Sequence,
				Stream:   pgtype.Text{String: stream, Valid: true},
				Content:  pgtype.Text{String: msg.Content, Valid: true},
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

			// Broadcast legacy task_output event via WebSocket.
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
// POST /api/daemon/tasks/{taskId}/stages/{stageName}/complete
// ---------------------------------------------------------------------------

// StageCompleteReq is the request body for POST /api/daemon/tasks/{taskId}/stages/{stageName}/complete.
type StageCompleteReq struct {
	OutputContent string `json:"output_content"`
}

// CompleteStage handles POST /api/daemon/tasks/{taskId}/stages/{stageName}/complete.
// It marks a stage as awaiting_approval with the provided output content.
// For conversational tasks (stage_name is a valid deliverable type), this endpoint
// returns 409 — conversational tasks complete via POST /api/daemon/tasks/{id}/complete.
func (h *DaemonHandler) CompleteStage(w http.ResponseWriter, r *http.Request) {
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

	var req StageCompleteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Guard: conversational tasks do not use the stage completion endpoint.
	// They complete via POST /api/daemon/tasks/{id}/complete which handles
	// session_id, work_dir, and prompt_history.
	if ValidDeliverableTypes[stageName] {
		writeErrorJSON(w, http.StatusConflict, "conversational tasks complete via task completion endpoint")
		return
	}

	// Look up the stage by task_id and stage_name.
	stage, err := h.Queries.GetStageByTaskAndName(r.Context(), db.GetStageByTaskAndNameParams{
		TaskID:    taskUUID,
		StageName: stageName,
	})
	if err != nil {
		slog.Warn("complete stage: stage not found",
			"task_id", taskID, "stage_name", stageName, "error", err)
		writeErrorJSON(w, http.StatusNotFound, "stage not found")
		return
	}

	// Verify stage is in "running" status.
	if stage.Status != "running" {
		writeErrorJSON(w, http.StatusConflict, "stage is not in running status")
		return
	}

	// Update stage status to "awaiting_approval".
	if err := h.Queries.UpdateStageStatus(r.Context(), db.UpdateStageStatusParams{
		ID:     stage.ID,
		Status: "awaiting_approval",
	}); err != nil {
		slog.Error("complete stage: update status failed",
			"task_id", taskID, "stage_name", stageName, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to update stage status")
		return
	}

	// Store the output content.
	if err := h.Queries.UpdateStageOutput(r.Context(), db.UpdateStageOutputParams{
		ID:            stage.ID,
		OutputContent: pgtype.Text{String: req.OutputContent, Valid: req.OutputContent != ""},
	}); err != nil {
		slog.Error("complete stage: update output failed",
			"task_id", taskID, "stage_name", stageName, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to store stage output")
		return
	}

	// Broadcast stage_awaiting_approval WebSocket event.
	// This is only reached for non-conversational (approval-gate) tasks.
	if h.Hub != nil {
		h.Hub.BroadcastStageAwaitingApproval(taskID, stageName, req.OutputContent)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "awaiting_approval"})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
