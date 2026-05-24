package handler

import (
	"errors"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/agenticflow/agenticflow/internal/middleware"
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

// ---------------------------------------------------------------------------
// GET /api/agents/activity — GetAgentsActivity
// ---------------------------------------------------------------------------

// AgentActivityBucket represents a single day's task counts for an agent.
type AgentActivityBucket struct {
	Date      string `json:"date"`
	Completed int64  `json:"completed"`
	Failed    int64  `json:"failed"`
}

// AgentActivityResponse represents the 7-day activity for a single agent.
type AgentActivityResponse struct {
	AgentID string                `json:"agent_id"`
	Buckets []AgentActivityBucket `json:"buckets"`
}

// GetAgentsActivity handles GET /api/agents/activity.
// It returns 7-day daily task completion/failure counts grouped by agent_id
// for all agents visible to the authenticated user.
func (h *AgentHandler) GetAgentsActivity(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rows, err := h.Queries.GetAgentsActivity7d(r.Context())
	if err != nil {
		slog.Error("get agents activity: query failed", "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to get agent activity")
		return
	}

	// Group rows by agent_id and build the response.
	agentMap := make(map[string]*AgentActivityResponse)
	for _, row := range rows {
		agentID := uuidToString(row.AgentID)
		if agentID == "" {
			continue
		}

		entry, exists := agentMap[agentID]
		if !exists {
			entry = &AgentActivityResponse{
				AgentID: agentID,
				Buckets: []AgentActivityBucket{},
			}
			agentMap[agentID] = entry
		}

		dateStr := ""
		if row.ActivityDate.Valid {
			dateStr = row.ActivityDate.Time.Format(time.DateOnly)
		}

		entry.Buckets = append(entry.Buckets, AgentActivityBucket{
			Date:      dateStr,
			Completed: row.Completed,
			Failed:    row.Failed,
		})
	}

	// Convert map to slice.
	resp := make([]AgentActivityResponse, 0, len(agentMap))
	for _, entry := range agentMap {
		resp = append(resp, *entry)
	}

	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// GET /api/agents/run-counts — GetAgentsRunCounts
// ---------------------------------------------------------------------------

// AgentRunCountResponse represents the 30-day completed task count for a single agent.
type AgentRunCountResponse struct {
	AgentID  string `json:"agent_id"`
	RunCount int64  `json:"run_count"`
}

// GetAgentsRunCounts handles GET /api/agents/run-counts.
// It returns the 30-day completed task count per agent for all agents
// that have at least one completed task in the last 30 days.
func (h *AgentHandler) GetAgentsRunCounts(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rows, err := h.Queries.GetAgentsRunCounts30d(r.Context())
	if err != nil {
		slog.Error("get agents run counts: query failed", "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to get agent run counts")
		return
	}

	resp := make([]AgentRunCountResponse, 0, len(rows))
	for _, row := range rows {
		agentID := uuidToString(row.AgentID)
		if agentID == "" {
			continue
		}
		resp = append(resp, AgentRunCountResponse{
			AgentID:  agentID,
			RunCount: row.RunCount,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}
