package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agenticflow/agenticflow/internal/auth"
	"github.com/agenticflow/agenticflow/internal/middleware"
	"github.com/agenticflow/agenticflow/internal/realtime"
	"github.com/agenticflow/agenticflow/internal/service"
	db "github.com/agenticflow/agenticflow/pkg/db/generated"
)

// customAgentNameRegex validates custom agent names.
var customAgentNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// maxPromptLength is the maximum allowed prompt length.
const maxPromptLength = 32000

// UserHandler holds dependencies for user-facing API handlers.
type UserHandler struct {
	Queries            *db.Queries
	Hub                *realtime.Hub
	AgentStatusService *service.AgentStatusService
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(queries *db.Queries, hub *realtime.Hub) *UserHandler {
	return &UserHandler{Queries: queries, Hub: hub}
}

// ---------------------------------------------------------------------------
// GET /api/me
// ---------------------------------------------------------------------------

// GetMe returns the current authenticated user's info.
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	user, err := h.Queries.GetUserByID(r.Context(), userUUID)
	if err != nil {
		slog.Error("get me: user not found", "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, toUserResponse(user))
}

// ---------------------------------------------------------------------------
// GET /api/daemons
// ---------------------------------------------------------------------------

// ListDaemons returns the user's registered daemons.
func (h *UserHandler) ListDaemons(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	daemons, err := h.Queries.ListDaemonsByUser(r.Context(), userUUID)
	if err != nil {
		slog.Error("list daemons: query failed", "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to list daemons")
		return
	}

	result := make([]daemonResponse, 0, len(daemons))
	for _, d := range daemons {
		resp := toDaemonResponse(d)
		// Fetch runtimes for this daemon.
		runtimes, err := h.Queries.ListRuntimesByDaemon(r.Context(), d.ID)
		if err != nil {
			slog.Error("list daemons: list runtimes failed",
				"daemon_id", uuidToString(d.ID), "error", err)
		} else {
			for _, rt := range runtimes {
				resp.AgentRuntimes = append(resp.AgentRuntimes, toRuntimeResponse(rt, d))
			}
		}
		if resp.AgentRuntimes == nil {
			resp.AgentRuntimes = []runtimeResponse{}
		}
		result = append(result, resp)
	}
	writeJSON(w, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// GET /api/agents
// ---------------------------------------------------------------------------

// ListAgentRuntimes returns agent runtimes across the user's daemons.
func (h *UserHandler) ListAgentRuntimes(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	daemons, err := h.Queries.ListDaemonsByUser(r.Context(), userUUID)
	if err != nil {
		slog.Error("list agents: list daemons failed", "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to list agents")
		return
	}

	var result []runtimeResponse
	for _, d := range daemons {
		runtimes, err := h.Queries.ListRuntimesByDaemon(r.Context(), d.ID)
		if err != nil {
			slog.Error("list agents: list runtimes failed",
				"daemon_id", uuidToString(d.ID), "error", err)
			continue
		}
		for _, rt := range runtimes {
			result = append(result, toRuntimeResponse(rt, d))
		}
	}

	if result == nil {
		result = []runtimeResponse{}
	}
	writeJSON(w, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// POST /api/tasks
// ---------------------------------------------------------------------------

// CreateTaskReq is the request body for POST /api/tasks.
type CreateTaskReq struct {
	AgentType string `json:"agent_type"`
	Prompt    string `json:"prompt"`
	AgentID   string `json:"agent_id,omitempty"`
}

// CreateTask creates a new task for the authenticated user.
func (h *UserHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.AgentType = strings.TrimSpace(req.AgentType)
	req.Prompt = strings.TrimSpace(req.Prompt)

	if req.Prompt == "" {
		writeErrorJSON(w, http.StatusBadRequest, "prompt is required")
		return
	}
	if len(req.Prompt) > maxPromptLength {
		writeErrorJSON(w, http.StatusBadRequest, "prompt exceeds 32000 character limit")
		return
	}

	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	// Parse optional agent_id and resolve agent's bound runtime.
	var agentID pgtype.UUID
	if req.AgentID != "" {
		parsed, err := parseUUID(req.AgentID)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "invalid agent_id")
			return
		}

		// Look up the agent to validate it exists and resolve its runtime.
		agent, err := h.Queries.GetAgent(r.Context(), parsed)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "agent not found")
			return
		}

		// Resolve the agent's bound runtime to determine the agent_type (provider).
		// This allows the task to be routed to the correct runtime when claimed.
		runtime, err := h.Queries.GetRuntimeByID(r.Context(), agent.RuntimeID)
		if err != nil {
			// Runtime not found — this shouldn't happen if the agent was created
			// correctly, but we still accept the task. The agent_type from the
			// request (if provided) will be used as fallback.
			slog.Warn("create task: agent runtime not found, using request agent_type",
				"agent_id", req.AgentID, "runtime_id", uuidToString(agent.RuntimeID), "error", err)
		} else {
			// Override agent_type with the runtime's provider for correct routing.
			req.AgentType = runtime.Provider
		}

		agentID = parsed
	}

	// agent_type is required when no agent_id is provided (backward compatibility).
	if req.AgentType == "" {
		writeErrorJSON(w, http.StatusBadRequest, "agent_type is required")
		return
	}

	// Create the task with status "pending". The task will be picked up by the
	// daemon when it polls, even if the agent's runtime is currently offline.
	task, err := h.Queries.CreateTask(r.Context(), db.CreateTaskParams{
		UserID:    userUUID,
		AgentType: req.AgentType,
		Prompt:    req.Prompt,
		AgentID:   agentID,
	})
	if err != nil {
		slog.Error("create task: insert failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to create task")
		return
	}

	taskIDStr := uuidToString(task.ID)

	// Broadcast task_created event via WebSocket.
	if h.Hub != nil {
		payload := map[string]interface{}{
			"task_id":    taskIDStr,
			"agent_type": req.AgentType,
			"prompt":     req.Prompt,
			"status":     task.Status,
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
// GET /api/tasks
// ---------------------------------------------------------------------------

// ListTasks returns the user's tasks with pagination.
func (h *UserHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit := int32(25)
	offset := int32(0)

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = int32(n)
		}
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	var tasks []db.Task

	// Support filtering by agent_id.
	if agentIDStr := r.URL.Query().Get("agent_id"); agentIDStr != "" {
		agentUUID, err := parseUUID(agentIDStr)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "invalid agent_id")
			return
		}
		tasks, err = h.Queries.ListTasksByAgent(r.Context(), db.ListTasksByAgentParams{
			AgentID: agentUUID,
			UserID:  userUUID,
			Limit:   limit,
			Offset:  offset,
		})
		if err != nil {
			slog.Error("list tasks by agent: query failed", "user_id", userID, "agent_id", agentIDStr, "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "failed to list tasks")
			return
		}
	} else {
		tasks, err = h.Queries.ListTasksByUser(r.Context(), db.ListTasksByUserParams{
			UserID: userUUID,
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			slog.Error("list tasks: query failed", "user_id", userID, "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "failed to list tasks")
			return
		}
	}

	// Build a cache of agent names to avoid repeated lookups.
	agentNames := make(map[string]string)
	result := make([]taskResponse, 0, len(tasks))
	for _, t := range tasks {
		resp := toTaskResponse(t)
		if t.AgentID.Valid {
			agentIDStr := uuidToString(t.AgentID)
			if name, ok := agentNames[agentIDStr]; ok {
				resp.AgentName = &name
			} else {
				agent, err := h.Queries.GetAgent(r.Context(), t.AgentID)
				if err == nil {
					agentNames[agentIDStr] = agent.Name
					resp.AgentName = &agent.Name
				}
			}
		}
		result = append(result, resp)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": result,
		"total": len(result), // TODO: add a count query for accurate total
	})
}

// ---------------------------------------------------------------------------
// GET /api/tasks/{taskId}
// ---------------------------------------------------------------------------

// GetTask returns a single task by ID.
func (h *UserHandler) GetTask(w http.ResponseWriter, r *http.Request) {
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

	taskUUID, err := parseUUID(taskID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid task id")
		return
	}

	task, err := h.Queries.GetTaskByID(r.Context(), taskUUID)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "task not found")
		return
	}

	// Ensure the task belongs to the requesting user.
	if uuidToString(task.UserID) != userID {
		writeErrorJSON(w, http.StatusNotFound, "task not found")
		return
	}

	resp := toTaskResponse(task)

	// Enrich with agent name if agent_id is set.
	if task.AgentID.Valid {
		agent, err := h.Queries.GetAgent(r.Context(), task.AgentID)
		if err == nil {
			resp.AgentName = &agent.Name
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// GET /api/tasks/{taskId}/messages
// ---------------------------------------------------------------------------

// ListTaskMessages returns messages for a task.
func (h *UserHandler) ListTaskMessages(w http.ResponseWriter, r *http.Request) {
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

	taskUUID, err := parseUUID(taskID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid task id")
		return
	}

	// Verify task ownership.
	task, err := h.Queries.GetTaskByID(r.Context(), taskUUID)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "task not found")
		return
	}
	if uuidToString(task.UserID) != userID {
		writeErrorJSON(w, http.StatusNotFound, "task not found")
		return
	}

	messages, err := h.Queries.ListTaskMessagesByTask(r.Context(), taskUUID)
	if err != nil {
		slog.Error("list task messages: query failed",
			"task_id", taskID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to list messages")
		return
	}

	result := make([]taskMessageResponse, 0, len(messages))
	for _, m := range messages {
		result = append(result, toTaskMessageResponse(m))
	}
	writeJSON(w, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// POST /api/tasks/{taskId}/cancel
// ---------------------------------------------------------------------------

// CancelTask cancels a pending or running task.
func (h *UserHandler) CancelTask(w http.ResponseWriter, r *http.Request) {
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

	taskUUID, err := parseUUID(taskID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid task id")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	if err := h.Queries.CancelTask(r.Context(), db.CancelTaskParams{
		ID:     taskUUID,
		UserID: userUUID,
	}); err != nil {
		slog.Error("cancel task: update failed", "task_id", taskID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to cancel task")
		return
	}

	// Trigger agent status recomputation for the task's owning agent.
	// The agent may transition from working → idle after cancellation.
	if h.AgentStatusService != nil {
		h.AgentStatusService.ReconcileAgentForTask(r.Context(), taskUUID)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// ---------------------------------------------------------------------------
// POST /api/custom-agents
// ---------------------------------------------------------------------------

// createCustomAgentRequest is the JSON body for POST /api/custom-agents.
type createCustomAgentRequest struct {
	Name          string            `json:"name"`
	ModelOverride string            `json:"model_override"`
	EnvVars       map[string]string `json:"env_vars"`
}

// CreateCustomAgent creates a new custom agent for the user.
func (h *UserHandler) CreateCustomAgent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createCustomAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if !customAgentNameRegex.MatchString(req.Name) {
		writeErrorJSON(w, http.StatusBadRequest,
			"name must match ^[a-zA-Z0-9_-]{1,64}$")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	envVarsJSON, err := json.Marshal(req.EnvVars)
	if err != nil {
		envVarsJSON = []byte("{}")
	}

	// Resolve the first available runtime for this user.
	daemons, err := h.Queries.ListDaemonsByUser(r.Context(), userUUID)
	if err != nil || len(daemons) == 0 {
		writeErrorJSON(w, http.StatusBadRequest, "no daemon available; register a daemon first")
		return
	}
	runtimes, err := h.Queries.ListRuntimesByDaemon(r.Context(), daemons[0].ID)
	if err != nil || len(runtimes) == 0 {
		writeErrorJSON(w, http.StatusBadRequest, "no runtime available; register a runtime first")
		return
	}

	agent, err := h.Queries.CreateAgent(r.Context(), db.CreateAgentParams{
		UserID:             userUUID,
		Name:               req.Name,
		Description:        "",
		Instructions:       "",
		RuntimeID:          runtimes[0].ID,
		Model:              pgtype.Text{String: req.ModelOverride, Valid: req.ModelOverride != ""},
		CustomEnv:          envVarsJSON,
		CustomArgs:         []byte("[]"),
		MaxConcurrentTasks: 1,
		Visibility:         "private",
		AvatarUrl:          pgtype.Text{},
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "23505") {
			writeErrorJSON(w, http.StatusConflict, "agent name already exists")
			return
		}
		slog.Error("create agent: insert failed", "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to create agent")
		return
	}

	writeJSON(w, http.StatusCreated, toCustomAgentResponse(agent))
}

// ---------------------------------------------------------------------------
// GET /api/custom-agents
// ---------------------------------------------------------------------------

// ListCustomAgents returns the user's agents.
func (h *UserHandler) ListCustomAgents(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	agents, err := h.Queries.ListAgentsByUser(r.Context(), userUUID)
	if err != nil {
		slog.Error("list agents: query failed", "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to list agents")
		return
	}

	result := make([]customAgentResponse, 0, len(agents))
	for _, a := range agents {
		result = append(result, toCustomAgentResponse(a))
	}
	writeJSON(w, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// PUT /api/custom-agents/{id}
// ---------------------------------------------------------------------------

// updateCustomAgentRequest is the JSON body for PUT /api/custom-agents/{id}.
type updateCustomAgentRequest struct {
	Name          string            `json:"name"`
	ModelOverride string            `json:"model_override"`
	EnvVars       map[string]string `json:"env_vars"`
}

// UpdateCustomAgent updates an existing agent.
func (h *UserHandler) UpdateCustomAgent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	agentID := chi.URLParam(r, "id")
	if agentID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "id is required")
		return
	}

	agentUUID, err := parseUUID(agentID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid agent id")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	var req updateCustomAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name != "" && !customAgentNameRegex.MatchString(req.Name) {
		writeErrorJSON(w, http.StatusBadRequest,
			"name must match ^[a-zA-Z0-9_-]{1,64}$")
		return
	}

	params := db.UpdateAgentParams{
		ID:     agentUUID,
		UserID: userUUID,
	}
	if req.Name != "" {
		params.Name = pgtype.Text{String: req.Name, Valid: true}
	}
	if req.ModelOverride != "" {
		params.Model = pgtype.Text{String: req.ModelOverride, Valid: true}
	}
	if req.EnvVars != nil {
		envJSON, _ := json.Marshal(req.EnvVars)
		params.CustomEnv = envJSON
	}

	agent, err := h.Queries.UpdateAgent(r.Context(), params)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "23505") {
			writeErrorJSON(w, http.StatusConflict, "agent name already exists")
			return
		}
		slog.Error("update agent: failed", "agent_id", agentID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to update agent")
		return
	}

	writeJSON(w, http.StatusOK, toCustomAgentResponse(agent))
}

// ---------------------------------------------------------------------------
// DELETE /api/custom-agents/{id}
// ---------------------------------------------------------------------------

// DeleteCustomAgent deletes an agent owned by the user.
func (h *UserHandler) DeleteCustomAgent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	agentID := chi.URLParam(r, "id")
	if agentID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "id is required")
		return
	}

	agentUUID, err := parseUUID(agentID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid agent id")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	if err := h.Queries.DeleteAgent(r.Context(), db.DeleteAgentParams{
		ID:     agentUUID,
		UserID: userUUID,
	}); err != nil {
		slog.Error("delete agent: failed", "agent_id", agentID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to delete agent")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ---------------------------------------------------------------------------
// GET /api/tokens
// ---------------------------------------------------------------------------

// ListTokens returns the user's personal access tokens (without hashes).
func (h *UserHandler) ListTokens(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	tokens, err := h.Queries.ListTokensByUser(r.Context(), userUUID)
	if err != nil {
		slog.Error("list tokens: query failed", "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to list tokens")
		return
	}

	result := make([]tokenResponse, 0, len(tokens))
	for _, t := range tokens {
		result = append(result, toTokenResponse(t))
	}
	writeJSON(w, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// POST /api/tokens
// ---------------------------------------------------------------------------

// createTokenRequest is the JSON body for POST /api/tokens.
type createTokenRequest struct {
	Name string `json:"name"`
}

// CreateToken generates a new PAT for the user.
func (h *UserHandler) CreateToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeErrorJSON(w, http.StatusBadRequest, "name is required")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	token, hash, err := auth.GeneratePAT()
	if err != nil {
		slog.Error("create token: generate failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	expiresAt := time.Now().Add(auth.PATExpiry)
	pat, err := h.Queries.CreateToken(r.Context(), db.CreateTokenParams{
		UserID:    userUUID,
		Name:      req.Name,
		TokenHash: hash,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		slog.Error("create token: insert failed", "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":         uuidToString(pat.ID),
		"name":       pat.Name,
		"token":      token,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
		"created_at": pat.CreatedAt.Time.UTC().Format(time.RFC3339),
	})
}

// ---------------------------------------------------------------------------
// DELETE /api/tokens/{id}
// ---------------------------------------------------------------------------

// RevokeToken deletes a personal access token.
func (h *UserHandler) RevokeToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	tokenID := chi.URLParam(r, "id")
	if tokenID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "id is required")
		return
	}

	tokenUUID, err := parseUUID(tokenID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid token id")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	if err := h.Queries.DeleteToken(r.Context(), db.DeleteTokenParams{
		ID:     tokenUUID,
		UserID: userUUID,
	}); err != nil {
		slog.Error("revoke token: delete failed", "token_id", tokenID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to revoke token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// ---------------------------------------------------------------------------
// Response types and converters
// ---------------------------------------------------------------------------

type daemonResponse struct {
	ID              string            `json:"id"`
	DaemonID        string            `json:"daemon_id"`
	DeviceName      string            `json:"device_name"`
	Status          string            `json:"status"`
	LastHeartbeatAt *string           `json:"last_heartbeat_at"`
	CLIVersion      *string           `json:"cli_version"`
	AgentRuntimes   []runtimeResponse `json:"agent_runtimes"`
	CreatedAt       string            `json:"created_at"`
	UpdatedAt       string            `json:"updated_at"`
}

func toDaemonResponse(d db.Daemon) daemonResponse {
	resp := daemonResponse{
		ID:         uuidToString(d.ID),
		DaemonID:   d.DaemonID,
		DeviceName: d.DeviceName,
		Status:     d.Status,
		CreatedAt:  d.CreatedAt.Time.UTC().Format(time.RFC3339),
		UpdatedAt:  d.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}
	if d.LastHeartbeatAt.Valid {
		s := d.LastHeartbeatAt.Time.UTC().Format(time.RFC3339)
		resp.LastHeartbeatAt = &s
	}
	if d.CliVersion.Valid {
		resp.CLIVersion = &d.CliVersion.String
	}
	return resp
}

type runtimeResponse struct {
	ID         string  `json:"id"`
	DaemonID   string  `json:"daemon_id"`
	Provider   string  `json:"provider"`
	Name       string  `json:"name"`
	Version    *string `json:"version"`
	BinaryPath *string `json:"binary_path"`
	Status     string  `json:"status"`
	DeviceName string  `json:"device_name"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

func toRuntimeResponse(rt db.AgentRuntime, d db.Daemon) runtimeResponse {
	resp := runtimeResponse{
		ID:         uuidToString(rt.ID),
		DaemonID:   uuidToString(rt.DaemonID),
		Provider:   rt.Provider,
		Name:       rt.Name,
		Status:     rt.Status,
		DeviceName: d.DeviceName,
		CreatedAt:  rt.CreatedAt.Time.UTC().Format(time.RFC3339),
		UpdatedAt:  rt.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}
	if rt.Version.Valid {
		resp.Version = &rt.Version.String
	}
	if rt.BinaryPath.Valid {
		resp.BinaryPath = &rt.BinaryPath.String
	}
	return resp
}

type taskResponse struct {
	ID             string  `json:"id"`
	AgentType      string  `json:"agent_type"`
	Prompt         string  `json:"prompt"`
	Status         string  `json:"status"`
	ExitCode       *int32  `json:"exit_code"`
	ErrorMessage   *string `json:"error_message"`
	OutputPreview  *string `json:"output_preview"`
	AgentID        *string `json:"agent_id"`
	AgentName      *string `json:"agent_name"`
	AgentRuntimeID *string `json:"agent_runtime_id"`
	DaemonID       *string `json:"daemon_id"`
	StartedAt      *string `json:"started_at"`
	CompletedAt    *string `json:"completed_at"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

func toTaskResponse(t db.Task) taskResponse {
	resp := taskResponse{
		ID:        uuidToString(t.ID),
		AgentType: t.AgentType,
		Prompt:    t.Prompt,
		Status:    t.Status,
		CreatedAt: t.CreatedAt.Time.UTC().Format(time.RFC3339),
		UpdatedAt: t.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}
	if t.ExitCode.Valid {
		resp.ExitCode = &t.ExitCode.Int32
	}
	if t.ErrorMessage.Valid {
		resp.ErrorMessage = &t.ErrorMessage.String
	}
	if t.OutputPreview.Valid {
		resp.OutputPreview = &t.OutputPreview.String
	}
	if t.AgentID.Valid {
		s := uuidToString(t.AgentID)
		resp.AgentID = &s
	}
	if t.AgentRuntimeID.Valid {
		s := uuidToString(t.AgentRuntimeID)
		resp.AgentRuntimeID = &s
	}
	if t.DaemonID.Valid {
		s := uuidToString(t.DaemonID)
		resp.DaemonID = &s
	}
	if t.StartedAt.Valid {
		s := t.StartedAt.Time.UTC().Format(time.RFC3339)
		resp.StartedAt = &s
	}
	if t.CompletedAt.Valid {
		s := t.CompletedAt.Time.UTC().Format(time.RFC3339)
		resp.CompletedAt = &s
	}
	return resp
}

type taskMessageResponse struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	Sequence  int32  `json:"sequence"`
	Stream    string `json:"stream"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

func toTaskMessageResponse(m db.TaskMessage) taskMessageResponse {
	return taskMessageResponse{
		ID:        uuidToString(m.ID),
		TaskID:    uuidToString(m.TaskID),
		Sequence:  m.Sequence,
		Stream:    m.Stream.String,
		Content:   m.Content.String,
		CreatedAt: m.CreatedAt.Time.UTC().Format(time.RFC3339),
	}
}

type customAgentResponse struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Instructions  string            `json:"instructions"`
	Model         *string           `json:"model"`
	EnvVars       map[string]string `json:"env_vars"`
	Visibility    string            `json:"visibility"`
	Status        string            `json:"status"`
	CreatedAt     string            `json:"created_at"`
	UpdatedAt     string            `json:"updated_at"`
}

func toCustomAgentResponse(a db.Agent) customAgentResponse {
	resp := customAgentResponse{
		ID:           uuidToString(a.ID),
		Name:         a.Name,
		Description:  a.Description,
		Instructions: a.Instructions,
		EnvVars:      make(map[string]string),
		Visibility:   a.Visibility,
		Status:       a.Status,
		CreatedAt:    a.CreatedAt.Time.UTC().Format(time.RFC3339),
		UpdatedAt:    a.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}
	if a.Model.Valid {
		resp.Model = &a.Model.String
	}
	if len(a.CustomEnv) > 0 {
		_ = json.Unmarshal(a.CustomEnv, &resp.EnvVars)
	}
	return resp
}

type tokenResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	ExpiresAt  string  `json:"expires_at"`
	LastUsedAt *string `json:"last_used_at"`
	CreatedAt  string  `json:"created_at"`
}

func toTokenResponse(t db.PersonalAccessToken) tokenResponse {
	resp := tokenResponse{
		ID:        uuidToString(t.ID),
		Name:      t.Name,
		ExpiresAt: t.ExpiresAt.Time.UTC().Format(time.RFC3339),
		CreatedAt: t.CreatedAt.Time.UTC().Format(time.RFC3339),
	}
	if t.LastUsedAt.Valid {
		s := t.LastUsedAt.Time.UTC().Format(time.RFC3339)
		resp.LastUsedAt = &s
	}
	return resp
}
