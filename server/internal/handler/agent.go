package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/agenticflow/agenticflow/server/internal/middleware"
	"github.com/agenticflow/agenticflow/server/internal/realtime"
	"github.com/agenticflow/agenticflow/server/internal/service"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// AgentHandler holds dependencies for agent CRUD HTTP handlers.
type AgentHandler struct {
	Queries db.Querier
	Hub     *realtime.Hub
	Service *service.AgentService
}

// NewAgentHandler creates a new AgentHandler.
func NewAgentHandler(queries db.Querier, hub *realtime.Hub) *AgentHandler {
	return &AgentHandler{
		Queries: queries,
		Hub:     hub,
		Service: service.NewAgentService(queries, hub),
	}
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
	RuntimeMode        string            `json:"runtime_mode"`
	ProviderID         string            `json:"provider_id"`
	DeliverableTypeID  string            `json:"deliverable_type_id"`
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
	RuntimeMode        string            `json:"runtime_mode"`
	ProviderID         *string           `json:"provider_id"`
	DeliverableTypeID  *string           `json:"deliverable_type_id"`
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
		RuntimeMode:        a.RuntimeMode,
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
	if a.ProviderID.Valid {
		s := uuidToString(a.ProviderID)
		resp.ProviderID = &s
	}
	if a.DeliverableTypeID.Valid {
		s := uuidToString(a.DeliverableTypeID)
		resp.DeliverableTypeID = &s
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

	agent, svcErr := h.Service.Create(r.Context(), service.CreateAgentParams{
		UserID:             userID,
		Name:               req.Name,
		Description:        req.Description,
		Instructions:       req.Instructions,
		AvatarURL:          req.AvatarURL,
		RuntimeID:          req.RuntimeID,
		CustomEnv:          req.CustomEnv,
		CustomArgs:         req.CustomArgs,
		Model:              req.Model,
		Visibility:         req.Visibility,
		MaxConcurrentTasks: req.MaxConcurrentTasks,
		MCPConfig:          req.MCPConfig,
		RuntimeMode:        req.RuntimeMode,
		ProviderID:         req.ProviderID,
		DeliverableTypeID:  req.DeliverableTypeID,
	})
	if svcErr != nil {
		writeErrorJSON(w, svcErr.Kind.HTTPStatus(), svcErr.Message)
		return
	}

	writeJSON(w, http.StatusCreated, toAgentResponse(agent))
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

	agents, svcErr := h.Service.List(r.Context(), userID)
	if svcErr != nil {
		writeErrorJSON(w, svcErr.Kind.HTTPStatus(), svcErr.Message)
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
	RuntimeMode        *string            `json:"runtime_mode"`
	ProviderID         *string            `json:"provider_id"`
	DeliverableTypeID  *string            `json:"deliverable_type_id"`
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

	agent, svcErr := h.Service.Get(r.Context(), agentID)
	if svcErr != nil {
		writeErrorJSON(w, svcErr.Kind.HTTPStatus(), svcErr.Message)
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

	var req UpdateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Determine if mcp_config was explicitly present in the request body.
	mcpConfigPresent := req.MCPConfig != nil

	agent, svcErr := h.Service.Update(r.Context(), service.UpdateAgentParams{
		UserID:             userID,
		AgentID:            agentID,
		Name:               req.Name,
		Description:        req.Description,
		Instructions:       req.Instructions,
		AvatarURL:          req.AvatarURL,
		RuntimeID:          req.RuntimeID,
		CustomEnv:          req.CustomEnv,
		CustomArgs:         req.CustomArgs,
		Model:              req.Model,
		Visibility:         req.Visibility,
		MaxConcurrentTasks: req.MaxConcurrentTasks,
		MCPConfig:          req.MCPConfig,
		MCPConfigPresent:   mcpConfigPresent,
		RuntimeMode:        req.RuntimeMode,
		ProviderID:         req.ProviderID,
		DeliverableTypeID:  req.DeliverableTypeID,
	})
	if svcErr != nil {
		writeErrorJSON(w, svcErr.Kind.HTTPStatus(), svcErr.Message)
		return
	}

	writeJSON(w, http.StatusOK, toAgentResponse(agent))
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

	agent, svcErr := h.Service.Restore(r.Context(), agentID)
	if svcErr != nil {
		writeErrorJSON(w, svcErr.Kind.HTTPStatus(), svcErr.Message)
		return
	}

	writeJSON(w, http.StatusOK, toAgentResponse(agent))
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

	svcErr := h.Service.Delete(r.Context(), agentID, userID)
	if svcErr != nil {
		writeErrorJSON(w, svcErr.Kind.HTTPStatus(), svcErr.Message)
		return
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

	isAdmin := middleware.ContextIsAdmin(r.Context())

	agent, svcErr := h.Service.Archive(r.Context(), agentID, userID, isAdmin)
	if svcErr != nil {
		writeErrorJSON(w, svcErr.Kind.HTTPStatus(), svcErr.Message)
		return
	}

	writeJSON(w, http.StatusOK, toAgentResponse(agent))
}
