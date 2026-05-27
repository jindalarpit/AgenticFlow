package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agenticflow/agenticflow/server/internal/middleware"
	"github.com/agenticflow/agenticflow/server/internal/realtime"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// Validation constants for agent fields.
const (
	maxAgentNameLength        = 64
	maxAgentDescriptionLength = 255
	maxAgentInstructionsLength = 50000
	maxAgentModelLength       = 100
	maxCustomEnvPairs         = 20
	maxCustomEnvKeyLength     = 64
	maxCustomEnvValueLength   = 1024
	maxCustomArgs             = 20
	maxCustomArgLength        = 256
	minConcurrentTasks        = 1
	maxConcurrentTasks        = 20
)

// agentNameRegex validates agent names: starts with alphanumeric,
// followed by alphanumeric, hyphens, or underscores. 1-64 chars total.
var agentNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// AgentHandler holds dependencies for agent CRUD HTTP handlers.
type AgentHandler struct {
	Queries *db.Queries
	Hub     *realtime.Hub
}

// NewAgentHandler creates a new AgentHandler.
func NewAgentHandler(queries *db.Queries, hub *realtime.Hub) *AgentHandler {
	return &AgentHandler{Queries: queries, Hub: hub}
}

// ---------------------------------------------------------------------------
// Request/Response types
// ---------------------------------------------------------------------------

// CreateAgentRequest is the JSON body for POST /api/agents.
type CreateAgentRequest struct {
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	Instructions       string            `json:"instructions"`
	AvatarURL          *string           `json:"avatar_url"`
	RuntimeID          string            `json:"runtime_id"`
	CustomEnv          map[string]string `json:"custom_env"`
	CustomArgs         []string          `json:"custom_args"`
	Model              string            `json:"model"`
	Visibility         string            `json:"visibility"`
	MaxConcurrentTasks int32             `json:"max_concurrent_tasks"`
	MCPConfig          json.RawMessage   `json:"mcp_config"`
}

// AgentResponse is the public representation of an agent.
type AgentResponse struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	Instructions       string            `json:"instructions"`
	AvatarURL          *string           `json:"avatar_url"`
	RuntimeID          string            `json:"runtime_id"`
	CustomEnv          map[string]string `json:"custom_env"`
	CustomArgs         []string          `json:"custom_args"`
	Model              string            `json:"model"`
	Visibility         string            `json:"visibility"`
	Status             string            `json:"status"`
	MaxConcurrentTasks int32             `json:"max_concurrent_tasks"`
	OwnerID            string            `json:"owner_id"`
	MCPConfig          json.RawMessage   `json:"mcp_config"`
	ArchivedAt         *string           `json:"archived_at"`
	CreatedAt          string            `json:"created_at"`
	UpdatedAt          string            `json:"updated_at"`
}

// toAgentResponse converts a db.Agent to the public AgentResponse.
func toAgentResponse(a db.Agent) AgentResponse {
	resp := AgentResponse{
		ID:                 uuidToString(a.ID),
		Name:               a.Name,
		Description:        a.Description,
		Instructions:       a.Instructions,
		RuntimeID:          uuidToString(a.RuntimeID),
		CustomEnv:          make(map[string]string),
		CustomArgs:         []string{},
		Model:              "",
		Visibility:         a.Visibility,
		Status:             a.Status,
		MaxConcurrentTasks: a.MaxConcurrentTasks,
		OwnerID:            uuidToString(a.UserID),
		CreatedAt:          a.CreatedAt.Time.UTC().Format(time.RFC3339),
		UpdatedAt:          a.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}

	if a.Model.Valid {
		resp.Model = a.Model.String
	}
	if a.AvatarUrl.Valid {
		s := a.AvatarUrl.String
		resp.AvatarURL = &s
	}
	if a.ArchivedAt.Valid {
		s := a.ArchivedAt.Time.UTC().Format(time.RFC3339)
		resp.ArchivedAt = &s
	}
	if len(a.CustomEnv) > 0 {
		_ = json.Unmarshal(a.CustomEnv, &resp.CustomEnv)
	}
	if len(a.CustomArgs) > 0 {
		_ = json.Unmarshal(a.CustomArgs, &resp.CustomArgs)
	}
	if len(a.McpConfig) > 0 {
		resp.MCPConfig = json.RawMessage(a.McpConfig)
	}

	return resp
}

// ---------------------------------------------------------------------------
// POST /api/agents — CreateAgent
// ---------------------------------------------------------------------------

// CreateAgent handles POST /api/agents.
// It validates all fields, verifies the runtime exists, rejects duplicate
// names per user, and creates the agent.
func (h *AgentHandler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// --- Validate name ---
	if req.Name == "" {
		writeErrorJSON(w, http.StatusBadRequest, "name is required")
		return
	}
	if !agentNameRegex.MatchString(req.Name) {
		writeErrorJSON(w, http.StatusBadRequest,
			"name must start with an alphanumeric character and contain only alphanumeric characters, hyphens, and underscores (1-64 characters)")
		return
	}

	// --- Validate description ---
	if utf8.RuneCountInString(req.Description) > maxAgentDescriptionLength {
		writeErrorJSON(w, http.StatusBadRequest,
			fmt.Sprintf("description must be %d characters or fewer", maxAgentDescriptionLength))
		return
	}

	// --- Validate instructions ---
	if utf8.RuneCountInString(req.Instructions) > maxAgentInstructionsLength {
		writeErrorJSON(w, http.StatusBadRequest,
			fmt.Sprintf("instructions must be %d characters or fewer", maxAgentInstructionsLength))
		return
	}

	// --- Validate model ---
	if utf8.RuneCountInString(req.Model) > maxAgentModelLength {
		writeErrorJSON(w, http.StatusBadRequest,
			fmt.Sprintf("model must be %d characters or fewer", maxAgentModelLength))
		return
	}

	// --- Validate custom_env ---
	if len(req.CustomEnv) > maxCustomEnvPairs {
		writeErrorJSON(w, http.StatusBadRequest,
			fmt.Sprintf("custom_env must have %d or fewer key-value pairs", maxCustomEnvPairs))
		return
	}
	for key, value := range req.CustomEnv {
		keyLen := utf8.RuneCountInString(key)
		if keyLen < 1 || keyLen > maxCustomEnvKeyLength {
			writeErrorJSON(w, http.StatusBadRequest,
				fmt.Sprintf("custom_env key must be between 1 and %d characters", maxCustomEnvKeyLength))
			return
		}
		valueLen := utf8.RuneCountInString(value)
		if valueLen < 1 || valueLen > maxCustomEnvValueLength {
			writeErrorJSON(w, http.StatusBadRequest,
				fmt.Sprintf("custom_env value must be between 1 and %d characters", maxCustomEnvValueLength))
			return
		}
	}

	// --- Validate custom_args ---
	if len(req.CustomArgs) > maxCustomArgs {
		writeErrorJSON(w, http.StatusBadRequest,
			fmt.Sprintf("custom_args must have %d or fewer items", maxCustomArgs))
		return
	}
	for _, arg := range req.CustomArgs {
		if utf8.RuneCountInString(arg) > maxCustomArgLength {
			writeErrorJSON(w, http.StatusBadRequest,
				fmt.Sprintf("each custom_args item must be %d characters or fewer", maxCustomArgLength))
			return
		}
	}

	// --- Validate max_concurrent_tasks ---
	if req.MaxConcurrentTasks == 0 {
		req.MaxConcurrentTasks = 1 // default
	}
	if req.MaxConcurrentTasks < minConcurrentTasks || req.MaxConcurrentTasks > maxConcurrentTasks {
		writeErrorJSON(w, http.StatusBadRequest,
			fmt.Sprintf("max_concurrent_tasks must be between %d and %d", minConcurrentTasks, maxConcurrentTasks))
		return
	}

	// --- Validate visibility ---
	if req.Visibility == "" {
		req.Visibility = "private" // default
	}
	if req.Visibility != "private" && req.Visibility != "shared" {
		writeErrorJSON(w, http.StatusBadRequest,
			"visibility must be \"private\" or \"shared\"")
		return
	}

	// --- Validate mcp_config (if provided) ---
	var mcpConfigBytes []byte
	if len(req.MCPConfig) > 0 && string(req.MCPConfig) != "null" {
		if !json.Valid(req.MCPConfig) {
			writeErrorJSON(w, http.StatusBadRequest, "mcp_config must be valid JSON")
			return
		}
		mcpConfigBytes = []byte(req.MCPConfig)
	}

	// --- Validate runtime_id exists ---
	if req.RuntimeID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "runtime_id is required")
		return
	}
	runtimeUUID, err := parseUUID(req.RuntimeID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid runtime_id format")
		return
	}
	_, err = h.Queries.GetRuntimeByID(r.Context(), runtimeUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErrorJSON(w, http.StatusBadRequest, "runtime_id does not reference an existing runtime")
			return
		}
		slog.Error("create agent: get runtime failed", "runtime_id", req.RuntimeID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to verify runtime")
		return
	}

	// --- Reject duplicate names per user ---
	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	_, err = h.Queries.GetAgentByName(r.Context(), db.GetAgentByNameParams{
		UserID: userUUID,
		Name:   req.Name,
	})
	if err == nil {
		// Agent with this name already exists for this user.
		writeErrorJSON(w, http.StatusConflict, "an agent with this name already exists")
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		slog.Error("create agent: check duplicate name failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to check agent name")
		return
	}

	// --- Marshal JSON fields ---
	customEnvJSON, err := json.Marshal(req.CustomEnv)
	if err != nil {
		customEnvJSON = []byte("{}")
	}
	if req.CustomEnv == nil {
		customEnvJSON = []byte("{}")
	}

	customArgsJSON, err := json.Marshal(req.CustomArgs)
	if err != nil {
		customArgsJSON = []byte("[]")
	}
	if req.CustomArgs == nil {
		customArgsJSON = []byte("[]")
	}

	// --- Create agent ---
	agent, err := h.Queries.CreateAgent(r.Context(), db.CreateAgentParams{
		UserID:             userUUID,
		Name:               req.Name,
		Description:        req.Description,
		Instructions:       req.Instructions,
		RuntimeID:          runtimeUUID,
		Model:              pgtype.Text{String: req.Model, Valid: req.Model != ""},
		CustomEnv:          customEnvJSON,
		CustomArgs:         customArgsJSON,
		MaxConcurrentTasks: req.MaxConcurrentTasks,
		Visibility:         req.Visibility,
		AvatarUrl:          pgtype.Text{String: ptrToString(req.AvatarURL), Valid: req.AvatarURL != nil},
		McpConfig:          mcpConfigBytes,
	})
	if err != nil {
		slog.Error("create agent: insert failed", "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to create agent")
		return
	}

	slog.Info("agent created", "agent_id", uuidToString(agent.ID), "name", agent.Name, "user_id", userID)

	resp := toAgentResponse(agent)

	// Broadcast agent_created event via WebSocket.
	if h.Hub != nil {
		h.Hub.BroadcastAgentCreated(resp)
	}

	writeJSON(w, http.StatusCreated, resp)
}

// ---------------------------------------------------------------------------
// GET /api/agents — ListAgents
// ---------------------------------------------------------------------------

// ListAgents handles GET /api/agents.
// It returns all agents visible to the authenticated user (owned + shared),
// sorted by created_at descending.
func (h *AgentHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
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

	resp := make([]AgentResponse, 0, len(agents))
	for _, a := range agents {
		resp = append(resp, toAgentResponse(a))
	}

	writeJSON(w, http.StatusOK, resp)
}

// ptrToString safely dereferences a string pointer, returning "" if nil.
func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ---------------------------------------------------------------------------
// UpdateAgentRequest — partial update (only provided fields are changed)
// ---------------------------------------------------------------------------

// UpdateAgentRequest is the JSON body for PUT /api/agents/{id}.
// All fields are pointers; nil means "do not change".
type UpdateAgentRequest struct {
	Name               *string            `json:"name"`
	Description        *string            `json:"description"`
	Instructions       *string            `json:"instructions"`
	AvatarURL          *string            `json:"avatar_url"`
	RuntimeID          *string            `json:"runtime_id"`
	CustomEnv          *map[string]string `json:"custom_env"`
	CustomArgs         *[]string          `json:"custom_args"`
	Model              *string            `json:"model"`
	Visibility         *string            `json:"visibility"`
	MaxConcurrentTasks *int32             `json:"max_concurrent_tasks"`
	MCPConfig          json.RawMessage    `json:"mcp_config"`
}

// ---------------------------------------------------------------------------
// GET /api/agents/{id} — GetAgent
// ---------------------------------------------------------------------------

// GetAgent handles GET /api/agents/{id}.
// It returns a single agent by ID with its derived status.
func (h *AgentHandler) GetAgent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	agentID := chi.URLParam(r, "id")
	if agentID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "agent id is required")
		return
	}

	agentUUID, err := parseUUID(agentID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid agent id format")
		return
	}

	agent, err := h.Queries.GetAgent(r.Context(), agentUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErrorJSON(w, http.StatusNotFound, "agent not found")
			return
		}
		slog.Error("get agent: query failed", "agent_id", agentID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to get agent")
		return
	}

	resp := toAgentResponse(agent)
	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// PUT /api/agents/{id} — UpdateAgent
// ---------------------------------------------------------------------------

// UpdateAgent handles PUT /api/agents/{id}.
// It validates changed fields, persists the update, and broadcasts an
// agent_updated event via WebSocket.
func (h *AgentHandler) UpdateAgent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	agentID := chi.URLParam(r, "id")
	if agentID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "agent id is required")
		return
	}

	agentUUID, err := parseUUID(agentID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid agent id format")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	var req UpdateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Determine if mcp_config was explicitly present in the request body.
	// json.RawMessage is nil when the field is absent from JSON,
	// and contains the literal bytes "null" when explicitly set to null.
	mcpConfigPresent := req.MCPConfig != nil

	// --- Validate name (if provided) ---
	if req.Name != nil {
		if *req.Name == "" {
			writeErrorJSON(w, http.StatusBadRequest, "name is required")
			return
		}
		if !agentNameRegex.MatchString(*req.Name) {
			writeErrorJSON(w, http.StatusBadRequest,
				"name must start with an alphanumeric character and contain only alphanumeric characters, hyphens, and underscores (1-64 characters)")
			return
		}
	}

	// --- Validate description (if provided) ---
	if req.Description != nil {
		if utf8.RuneCountInString(*req.Description) > maxAgentDescriptionLength {
			writeErrorJSON(w, http.StatusBadRequest,
				fmt.Sprintf("description must be %d characters or fewer", maxAgentDescriptionLength))
			return
		}
	}

	// --- Validate instructions (if provided) ---
	if req.Instructions != nil {
		if utf8.RuneCountInString(*req.Instructions) > maxAgentInstructionsLength {
			writeErrorJSON(w, http.StatusBadRequest,
				fmt.Sprintf("instructions must be %d characters or fewer", maxAgentInstructionsLength))
			return
		}
	}

	// --- Validate model (if provided) ---
	if req.Model != nil {
		if utf8.RuneCountInString(*req.Model) > maxAgentModelLength {
			writeErrorJSON(w, http.StatusBadRequest,
				fmt.Sprintf("model must be %d characters or fewer", maxAgentModelLength))
			return
		}
	}

	// --- Validate custom_env (if provided) ---
	if req.CustomEnv != nil {
		if len(*req.CustomEnv) > maxCustomEnvPairs {
			writeErrorJSON(w, http.StatusBadRequest,
				fmt.Sprintf("custom_env must have %d or fewer key-value pairs", maxCustomEnvPairs))
			return
		}
		for key, value := range *req.CustomEnv {
			keyLen := utf8.RuneCountInString(key)
			if keyLen < 1 || keyLen > maxCustomEnvKeyLength {
				writeErrorJSON(w, http.StatusBadRequest,
					fmt.Sprintf("custom_env key must be between 1 and %d characters", maxCustomEnvKeyLength))
				return
			}
			valueLen := utf8.RuneCountInString(value)
			if valueLen < 1 || valueLen > maxCustomEnvValueLength {
				writeErrorJSON(w, http.StatusBadRequest,
					fmt.Sprintf("custom_env value must be between 1 and %d characters", maxCustomEnvValueLength))
				return
			}
		}
	}

	// --- Validate custom_args (if provided) ---
	if req.CustomArgs != nil {
		if len(*req.CustomArgs) > maxCustomArgs {
			writeErrorJSON(w, http.StatusBadRequest,
				fmt.Sprintf("custom_args must have %d or fewer items", maxCustomArgs))
			return
		}
		for _, arg := range *req.CustomArgs {
			if utf8.RuneCountInString(arg) > maxCustomArgLength {
				writeErrorJSON(w, http.StatusBadRequest,
					fmt.Sprintf("each custom_args item must be %d characters or fewer", maxCustomArgLength))
				return
			}
		}
	}

	// --- Validate max_concurrent_tasks (if provided) ---
	if req.MaxConcurrentTasks != nil {
		if *req.MaxConcurrentTasks < minConcurrentTasks || *req.MaxConcurrentTasks > maxConcurrentTasks {
			writeErrorJSON(w, http.StatusBadRequest,
				fmt.Sprintf("max_concurrent_tasks must be between %d and %d", minConcurrentTasks, maxConcurrentTasks))
			return
		}
	}

	// --- Validate visibility (if provided) ---
	if req.Visibility != nil {
		if *req.Visibility != "private" && *req.Visibility != "shared" {
			writeErrorJSON(w, http.StatusBadRequest,
				"visibility must be \"private\" or \"shared\"")
			return
		}
	}

	// --- Validate runtime_id (if provided) ---
	var runtimeUUID pgtype.UUID
	if req.RuntimeID != nil {
		if *req.RuntimeID == "" {
			writeErrorJSON(w, http.StatusBadRequest, "runtime_id cannot be empty")
			return
		}
		var err error
		runtimeUUID, err = parseUUID(*req.RuntimeID)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "invalid runtime_id format")
			return
		}
		_, err = h.Queries.GetRuntimeByID(r.Context(), runtimeUUID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeErrorJSON(w, http.StatusBadRequest, "runtime_id does not reference an existing runtime")
				return
			}
			slog.Error("update agent: get runtime failed", "runtime_id", *req.RuntimeID, "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "failed to verify runtime")
			return
		}
	}

	// --- Build update params (COALESCE-based: nil = keep existing) ---
	params := db.UpdateAgentParams{
		ID:     agentUUID,
		UserID: userUUID,
	}

	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.Instructions != nil {
		params.Instructions = pgtype.Text{String: *req.Instructions, Valid: true}
	}
	if req.RuntimeID != nil {
		params.RuntimeID = runtimeUUID
	}
	if req.Model != nil {
		params.Model = pgtype.Text{String: *req.Model, Valid: true}
	}
	if req.CustomEnv != nil {
		envJSON, err := json.Marshal(*req.CustomEnv)
		if err != nil {
			envJSON = []byte("{}")
		}
		params.CustomEnv = envJSON
	}
	if req.CustomArgs != nil {
		argsJSON, err := json.Marshal(*req.CustomArgs)
		if err != nil {
			argsJSON = []byte("[]")
		}
		params.CustomArgs = argsJSON
	}
	if req.MaxConcurrentTasks != nil {
		params.MaxConcurrentTasks = pgtype.Int4{Int32: *req.MaxConcurrentTasks, Valid: true}
	}
	if req.Visibility != nil {
		params.Visibility = pgtype.Text{String: *req.Visibility, Valid: true}
	}
	if req.AvatarURL != nil {
		params.AvatarUrl = pgtype.Text{String: *req.AvatarURL, Valid: true}
	}

	// --- Handle mcp_config tri-state ---
	// mcpConfigPresent=false → field omitted → no change (SetMcpConfig=false)
	// mcpConfigPresent=true, value is "null" → clear (SetMcpConfig=true, McpConfig=nil)
	// mcpConfigPresent=true, value is JSON object → validate and store
	if mcpConfigPresent {
		params.SetMcpConfig = true
		if string(req.MCPConfig) == "null" {
			params.McpConfig = nil
		} else {
			if !json.Valid(req.MCPConfig) {
				writeErrorJSON(w, http.StatusBadRequest, "mcp_config must be valid JSON")
				return
			}
			params.McpConfig = []byte(req.MCPConfig)
		}
	}

	agent, err := h.Queries.UpdateAgent(r.Context(), params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErrorJSON(w, http.StatusNotFound, "agent not found")
			return
		}
		slog.Error("update agent: query failed", "agent_id", agentID, "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to update agent")
		return
	}

	slog.Info("agent updated", "agent_id", agentID, "user_id", userID)

	resp := toAgentResponse(agent)

	// Broadcast agent_updated event via WebSocket.
	if h.Hub != nil {
		h.Hub.BroadcastAgentUpdated(resp)
	}

	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// POST /api/agents/{id}/restore — RestoreAgent
// ---------------------------------------------------------------------------

// RestoreAgent handles POST /api/agents/{id}/restore.
// It clears the archived_at timestamp on an archived agent, effectively
// restoring it to the active view. The agent must exist and be currently
// archived. Broadcasts an agent_updated WebSocket event on success.
func (h *AgentHandler) RestoreAgent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	agentID := chi.URLParam(r, "id")
	if agentID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "agent id is required")
		return
	}

	agentUUID, err := parseUUID(agentID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid agent id format")
		return
	}

	// Fetch the agent to validate it exists and is currently archived.
	agent, err := h.Queries.GetAgent(r.Context(), agentUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErrorJSON(w, http.StatusNotFound, "agent not found")
			return
		}
		slog.Error("restore agent: get agent failed", "agent_id", agentID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to get agent")
		return
	}

	// Verify the agent is currently archived.
	if !agent.ArchivedAt.Valid {
		writeErrorJSON(w, http.StatusConflict, "agent is not archived")
		return
	}

	// Restore the agent (set archived_at = NULL).
	restored, err := h.Queries.RestoreAgent(r.Context(), agentUUID)
	if err != nil {
		slog.Error("restore agent: query failed", "agent_id", agentID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to restore agent")
		return
	}

	slog.Info("agent restored", "agent_id", agentID, "user_id", userID)

	resp := toAgentResponse(restored)

	// Broadcast agent_updated event via WebSocket.
	if h.Hub != nil {
		h.Hub.BroadcastAgentUpdated(resp)
	}

	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// DELETE /api/agents/{id} — DeleteAgent
// ---------------------------------------------------------------------------

// DeleteAgent handles DELETE /api/agents/{id}.
// It removes the agent record (scoped to the authenticated user) and
// broadcasts an agent_deleted event. In-flight tasks are allowed to complete
// because the task table uses ON DELETE SET NULL for agent_id.
func (h *AgentHandler) DeleteAgent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	agentID := chi.URLParam(r, "id")
	if agentID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "agent id is required")
		return
	}

	agentUUID, err := parseUUID(agentID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid agent id format")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	err = h.Queries.DeleteAgent(r.Context(), db.DeleteAgentParams{
		ID:     agentUUID,
		UserID: userUUID,
	})
	if err != nil {
		slog.Error("delete agent: query failed", "agent_id", agentID, "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to delete agent")
		return
	}

	slog.Info("agent deleted", "agent_id", agentID, "user_id", userID)

	// Broadcast agent_deleted event via WebSocket.
	if h.Hub != nil {
		h.Hub.BroadcastAgentDeleted(agentID)
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// POST /api/agents/{id}/archive — ArchiveAgent
// ---------------------------------------------------------------------------

// ArchiveAgent handles POST /api/agents/{id}/archive.
// It soft-deletes the agent by setting archived_at = now(). The user must be
// the agent's owner or a workspace admin. Already-archived agents are rejected
// with 409 Conflict.
func (h *AgentHandler) ArchiveAgent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	agentID := chi.URLParam(r, "id")
	if agentID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "agent id is required")
		return
	}

	agentUUID, err := parseUUID(agentID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid agent id format")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	// Fetch the agent to validate existence and check permissions.
	agent, err := h.Queries.GetAgent(r.Context(), agentUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErrorJSON(w, http.StatusNotFound, "agent not found")
			return
		}
		slog.Error("archive agent: get agent failed", "agent_id", agentID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to get agent")
		return
	}

	// Check if agent is already archived.
	if agent.ArchivedAt.Valid {
		writeErrorJSON(w, http.StatusConflict, "agent is already archived")
		return
	}

	// Permission check: user must be the agent owner or a workspace admin.
	// In AgenticFlow's simple model, we treat the owner check as the primary
	// permission gate. The isAdmin flag allows workspace admins to archive
	// any agent.
	isOwner := uuidToString(agent.UserID) == userID
	isAdmin := middleware.ContextIsAdmin(r.Context())

	if !isOwner && !isAdmin {
		writeErrorJSON(w, http.StatusForbidden, "you do not have permission to archive this agent")
		return
	}

	// Perform the archive.
	archived, err := h.Queries.ArchiveAgent(r.Context(), db.ArchiveAgentParams{
		ID:      agentUUID,
		UserID:  userUUID,
		IsAdmin: isAdmin,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErrorJSON(w, http.StatusNotFound, "agent not found")
			return
		}
		slog.Error("archive agent: query failed", "agent_id", agentID, "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to archive agent")
		return
	}

	slog.Info("agent archived", "agent_id", agentID, "user_id", userID)

	resp := toAgentResponse(archived)

	// Broadcast agent_updated event via WebSocket with updated archived_at.
	if h.Hub != nil {
		h.Hub.BroadcastAgentUpdated(resp)
	}

	writeJSON(w, http.StatusOK, resp)
}
