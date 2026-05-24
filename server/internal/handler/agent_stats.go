package handler

import (
	"errors"
	"log/slog"
	"math"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// AgentStatsResponse is the JSON response for GET /api/agents/{id}/stats.
type AgentStatsResponse struct {
	TotalRuns     int64 `json:"total_runs"`
	SuccessRate   int64 `json:"success_rate"`
	AvgDurationMs int64 `json:"avg_duration_ms"`
	TotalTerminal int64 `json:"total_terminal"`
}

// GetAgentStats handles GET /api/agents/{id}/stats.
// It returns pre-computed 30-day aggregate statistics for the given agent.
func (h *AgentHandler) GetAgentStats(w http.ResponseWriter, r *http.Request) {
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

	// Verify the agent exists.
	_, err = h.Queries.GetAgent(r.Context(), agentUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErrorJSON(w, http.StatusNotFound, "agent not found")
			return
		}
		slog.Error("get agent stats: agent lookup failed", "agent_id", agentID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to get agent")
		return
	}

	// Fetch 30-day stats.
	stats, err := h.Queries.GetAgentStats30d(r.Context(), agentUUID)
	if err != nil {
		slog.Error("get agent stats: query failed", "agent_id", agentID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to get agent stats")
		return
	}

	// Compute success rate: Math.round((total_completed / total_terminal) * 100).
	// Return 0 when total_terminal is 0.
	var successRate int64
	if stats.TotalTerminal > 0 {
		successRate = int64(math.Round(float64(stats.TotalCompleted) / float64(stats.TotalTerminal) * 100))
	}

	resp := AgentStatsResponse{
		TotalRuns:     stats.TotalCompleted,
		SuccessRate:   successRate,
		AvgDurationMs: stats.AvgDurationMs,
		TotalTerminal: stats.TotalTerminal,
	}

	writeJSON(w, http.StatusOK, resp)
}
