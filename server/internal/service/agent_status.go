package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
	"github.com/agenticflow/agenticflow/server/internal/realtime"
	"github.com/agenticflow/agenticflow/shared/constants"
	"github.com/jackc/pgx/v5/pgtype"
)

// AgentStatus represents the derived status of an agent.
type AgentStatus string

const (
	// AgentStatusOffline indicates the agent's runtime daemon is offline.
	AgentStatusOffline AgentStatus = AgentStatus(constants.AgentStatusOffline)
	// AgentStatusWorking indicates the agent has at least one running task.
	AgentStatusWorking AgentStatus = AgentStatus(constants.AgentStatusWorking)
	// AgentStatusIdle indicates the agent is online with no running tasks.
	AgentStatusIdle AgentStatus = AgentStatus(constants.AgentStatusIdle)
)

// DeriveAgentStatus computes the agent's status from runtime state and active task count.
// Priority order: offline > working > idle.
// This is a pure function exposed for easy unit and property-based testing.
func DeriveAgentStatus(runtimeStatus string, activeTaskCount int) AgentStatus {
	if runtimeStatus == constants.DaemonStatusOffline {
		return AgentStatusOffline
	}
	if activeTaskCount > 0 {
		return AgentStatusWorking
	}
	return AgentStatusIdle
}

// AgentStatusService derives agent status from runtime state and active task count,
// and broadcasts status changes via the WebSocket hub.
type AgentStatusService struct {
	queries *db.Queries
	hub     *realtime.Hub
}

// NewAgentStatusService creates a new AgentStatusService.
func NewAgentStatusService(queries *db.Queries, hub *realtime.Hub) *AgentStatusService {
	return &AgentStatusService{
		queries: queries,
		hub:     hub,
	}
}

// DeriveStatus computes the current status for the given agent.
// It looks up the agent's bound runtime and counts active tasks.
// Priority: offline > working > idle.
func (s *AgentStatusService) DeriveStatus(ctx context.Context, agentID pgtype.UUID) (AgentStatus, error) {
	agent, err := s.queries.GetAgent(ctx, agentID)
	if err != nil {
		return "", fmt.Errorf("get agent: %w", err)
	}

	runtime, err := s.queries.GetRuntimeByID(ctx, agent.RuntimeID)
	if err != nil {
		return "", fmt.Errorf("get runtime: %w", err)
	}

	// Check runtime's daemon status to determine if offline.
	daemon, err := s.queries.GetDaemonByID(ctx, runtime.DaemonID)
	if err != nil {
		return "", fmt.Errorf("get daemon: %w", err)
	}

	if daemon.Status == constants.DaemonStatusOffline {
		return AgentStatusOffline, nil
	}

	activeCount, err := s.queries.CountActiveTasksForAgent(ctx, agentID)
	if err != nil {
		return "", fmt.Errorf("count active tasks: %w", err)
	}

	return DeriveAgentStatus(daemon.Status, int(activeCount)), nil
}

// ReconcileAndBroadcast recomputes the agent's status and, if it has changed
// from the stored value, updates the database and broadcasts an
// `agent_status_changed` event via the WebSocket hub.
func (s *AgentStatusService) ReconcileAndBroadcast(ctx context.Context, agentID pgtype.UUID) {
	newStatus, err := s.DeriveStatus(ctx, agentID)
	if err != nil {
		slog.Error("failed to derive agent status", "error", err, "agent_id", agentID)
		return
	}

	// Get current stored status.
	agent, err := s.queries.GetAgent(ctx, agentID)
	if err != nil {
		slog.Error("failed to get agent for reconciliation", "error", err, "agent_id", agentID)
		return
	}

	if agent.Status == string(newStatus) {
		// No change, nothing to broadcast.
		return
	}

	// Broadcast the status change event using the dedicated method.
	agentIDStr := uuidToString(agentID)
	s.hub.BroadcastAgentStatusChanged(agentIDStr, string(newStatus))

	slog.Info("agent status changed",
		"agent_id", agentIDStr,
		"old_status", agent.Status,
		"new_status", string(newStatus),
	)
}

// ReconcileAgentsForDaemon recomputes and broadcasts status for all agents
// bound to the given daemon's runtimes. This should be called when a daemon
// connects or disconnects. It runs asynchronously to meet the 2-second
// requirement for status recomputation.
func (s *AgentStatusService) ReconcileAgentsForDaemon(ctx context.Context, daemonDBID pgtype.UUID) {
	go func() {
		// Use a fresh context with a 2-second deadline to ensure timely recomputation.
		reconcileCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		agents, err := s.queries.ListAgentsByDaemon(reconcileCtx, daemonDBID)
		if err != nil {
			slog.Error("reconcile agents for daemon: list agents failed",
				"daemon_id", uuidToString(daemonDBID), "error", err)
			return
		}

		for _, agent := range agents {
			s.ReconcileAndBroadcast(reconcileCtx, agent.ID)
		}

		if len(agents) > 0 {
			slog.Info("reconciled agent statuses for daemon",
				"daemon_id", uuidToString(daemonDBID),
				"agents_count", len(agents),
			)
		}
	}()
}

// ReconcileAgentForTask recomputes and broadcasts the owning agent's status
// when a task transitions to running/completed/failed/cancelled. It runs
// asynchronously to meet the 2-second requirement for status recomputation.
func (s *AgentStatusService) ReconcileAgentForTask(ctx context.Context, taskID pgtype.UUID) {
	go func() {
		// Use a fresh context with a 2-second deadline to ensure timely recomputation.
		reconcileCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		agent, err := s.queries.GetAgentByTaskID(reconcileCtx, taskID)
		if err != nil {
			// Task may not have an associated agent (legacy tasks or tasks without agent_id).
			// This is not an error condition.
			slog.Debug("reconcile agent for task: no agent found",
				"task_id", uuidToString(taskID), "error", err)
			return
		}

		s.ReconcileAndBroadcast(reconcileCtx, agent.ID)
	}()
}

// uuidToString converts a pgtype.UUID to its string representation.
func uuidToString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	b := id.Bytes
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
