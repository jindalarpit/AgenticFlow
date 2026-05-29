package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agenticflow/agenticflow/server/internal/middleware"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// TxBeginner is an interface for beginning database transactions.
// Satisfied by *pgxpool.Pool and *pgx.Conn.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// QuerierWithTx extends db.Querier with the ability to create a
// transaction-scoped Querier. Satisfied by *db.Queries.
type QuerierWithTx interface {
	db.Querier
	WithTx(tx pgx.Tx) *db.Queries
}

// AgentSkillHandler holds dependencies for agent-skill association HTTP handlers.
type AgentSkillHandler struct {
	Queries QuerierWithTx
	Pool    TxBeginner
}

// NewAgentSkillHandler creates a new AgentSkillHandler.
func NewAgentSkillHandler(queries QuerierWithTx, pool TxBeginner) *AgentSkillHandler {
	return &AgentSkillHandler{Queries: queries, Pool: pool}
}

// ---------------------------------------------------------------------------
// Request/Response types
// ---------------------------------------------------------------------------

// SetAgentSkillsRequest is the JSON body for PUT /api/agents/{id}/skills.
type SetAgentSkillsRequest struct {
	SkillIDs []string `json:"skill_ids"`
}

// AgentSkillResponse is the public representation of a skill assigned to an agent.
type AgentSkillResponse struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Content     string          `json:"content"`
	Config      json.RawMessage `json:"config,omitempty"`
}

// ---------------------------------------------------------------------------
// PUT /api/agents/{id}/skills — SetSkills
// ---------------------------------------------------------------------------

// SetSkills handles PUT /api/agents/{id}/skills.
// It replaces all skill associations for the agent with the provided set.
func (h *AgentSkillHandler) SetSkills(w http.ResponseWriter, r *http.Request) {
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

	// Verify agent exists and is owned by the user.
	agent, err := h.Queries.GetAgent(r.Context(), agentUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErrorJSON(w, http.StatusNotFound, "agent not found")
			return
		}
		slog.Error("set agent skills: get agent failed", "agent_id", agentID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to get agent")
		return
	}

	if uuidToString(agent.UserID) != userID {
		writeErrorJSON(w, http.StatusForbidden, "forbidden")
		return
	}

	// Parse request body.
	var req SetAgentSkillsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Allow empty skill_ids to clear all associations.
	if req.SkillIDs == nil {
		req.SkillIDs = []string{}
	}

	// Validate each skill ID: must exist and be owned by the user.
	skillUUIDs := make([]pgtype.UUID, 0, len(req.SkillIDs))
	for _, sid := range req.SkillIDs {
		skillUUID, err := parseUUID(sid)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "invalid skill id: "+sid)
			return
		}

		skill, err := h.Queries.GetSkillByID(r.Context(), skillUUID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeErrorJSON(w, http.StatusBadRequest, "invalid skill id: "+sid)
				return
			}
			slog.Error("set agent skills: get skill failed", "skill_id", sid, "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "failed to validate skill")
			return
		}

		if uuidToString(skill.UserID) != userID {
			writeErrorJSON(w, http.StatusBadRequest, "invalid skill id: "+sid)
			return
		}

		skillUUIDs = append(skillUUIDs, skillUUID)
	}

	// Execute in a transaction: delete all existing + insert new.
	tx, err := h.Pool.Begin(r.Context())
	if err != nil {
		slog.Error("set agent skills: begin tx failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to update agent skills")
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.Queries.WithTx(tx)

	if err := qtx.DeleteAllAgentSkills(r.Context(), agentUUID); err != nil {
		slog.Error("set agent skills: delete failed", "agent_id", agentID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to update agent skills")
		return
	}

	for _, skillUUID := range skillUUIDs {
		if err := qtx.InsertAgentSkill(r.Context(), db.InsertAgentSkillParams{
			AgentID: agentUUID,
			SkillID: skillUUID,
		}); err != nil {
			slog.Error("set agent skills: insert failed", "agent_id", agentID, "skill_id", uuidToString(skillUUID), "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "failed to update agent skills")
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		slog.Error("set agent skills: commit failed", "agent_id", agentID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to update agent skills")
		return
	}

	slog.Info("agent skills updated", "agent_id", agentID, "skill_count", len(skillUUIDs), "user_id", userID)

	// Return the updated skill list.
	skills, err := h.Queries.GetAgentSkills(r.Context(), agentUUID)
	if err != nil {
		slog.Error("set agent skills: get skills failed", "agent_id", agentID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to retrieve updated skills")
		return
	}

	resp := make([]AgentSkillResponse, 0, len(skills))
	for _, s := range skills {
		resp = append(resp, AgentSkillResponse{
			ID:          uuidToString(s.ID),
			Name:        s.Name,
			Description: s.Description,
			Content:     s.Content,
			Config:      normalizeConfig(s.Config),
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// GET /api/agents/{id}/skills — GetSkills
// ---------------------------------------------------------------------------

// GetSkills handles GET /api/agents/{id}/skills.
// It returns all skills assigned to the specified agent.
func (h *AgentSkillHandler) GetSkills(w http.ResponseWriter, r *http.Request) {
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

	// Verify agent exists and is owned by the user.
	agent, err := h.Queries.GetAgent(r.Context(), agentUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErrorJSON(w, http.StatusNotFound, "agent not found")
			return
		}
		slog.Error("get agent skills: get agent failed", "agent_id", agentID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to get agent")
		return
	}

	if uuidToString(agent.UserID) != userID {
		writeErrorJSON(w, http.StatusForbidden, "forbidden")
		return
	}

	// Fetch assigned skills.
	skills, err := h.Queries.GetAgentSkills(r.Context(), agentUUID)
	if err != nil {
		slog.Error("get agent skills: query failed", "agent_id", agentID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to get agent skills")
		return
	}

	resp := make([]AgentSkillResponse, 0, len(skills))
	for _, s := range skills {
		resp = append(resp, AgentSkillResponse{
			ID:          uuidToString(s.ID),
			Name:        s.Name,
			Description: s.Description,
			Content:     s.Content,
			Config:      normalizeConfig(s.Config),
		})
	}

	writeJSON(w, http.StatusOK, resp)
}
