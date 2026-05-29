package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
	"github.com/agenticflow/agenticflow/server/internal/realtime"
	"github.com/agenticflow/agenticflow/shared/constants"
	"github.com/agenticflow/agenticflow/shared/pgutil"
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
	bgCtx   context.Context // parent context for background goroutines; checked for cancellation
	wg      sync.WaitGroup  // tracks in-flight reconciliation goroutines
}

// NewAgentStatusService creates a new AgentStatusService.
// The bgCtx is the parent context for all background reconciliation goroutines;
// when cancelled, goroutines will exit promptly.
func NewAgentStatusService(queries *db.Queries, hub *realtime.Hub, bgCtx context.Context) *AgentStatusService {
	return &AgentStatusService{
		queries: queries,
		hub:     hub,
		bgCtx:   bgCtx,
	}
}

// Wait blocks until all in-flight reconciliation goroutines have completed.
func (s *AgentStatusService) Wait() {
	s.wg.Wait()
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

	// Persist FIRST — update the database before broadcasting.
	err = s.queries.UpdateAgentStatus(ctx, db.UpdateAgentStatusParams{
		ID:     agentID,
		Status: string(newStatus),
	})
	if err != nil {
		slog.Error("failed to persist agent status", "error", err, "agent_id", agentID)
		return // skip broadcast on DB failure
	}

	// Then broadcast the status change event.
	agentIDStr := pgutil.UUIDToString(agentID)
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
	// Check if the parent context is already cancelled (shutdown in progress).
	if s.bgCtx.Err() != nil {
		slog.Debug("skipping reconciliation for daemon: shutdown in progress",
			"daemon_id", pgutil.UUIDToString(daemonDBID))
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		// Use a context derived from the parent background context with a 2-second deadline.
		// This ensures the goroutine exits promptly when shutdown is signaled.
		reconcileCtx, cancel := context.WithTimeout(s.bgCtx, 2*time.Second)
		defer cancel()

		agents, err := s.queries.ListAgentsByDaemon(reconcileCtx, daemonDBID)
		if err != nil {
			if s.bgCtx.Err() != nil {
				// Context cancelled due to shutdown — not an error.
				return
			}
			slog.Error("reconcile agents for daemon: list agents failed",
				"daemon_id", pgutil.UUIDToString(daemonDBID), "error", err)
			return
		}

		for _, agent := range agents {
			// Check for cancellation between iterations.
			if s.bgCtx.Err() != nil {
				return
			}
			s.ReconcileAndBroadcast(reconcileCtx, agent.ID)
		}

		if len(agents) > 0 {
			slog.Info("reconciled agent statuses for daemon",
				"daemon_id", pgutil.UUIDToString(daemonDBID),
				"agents_count", len(agents),
			)
		}
	}()
}

// ReconcileAgentForTask recomputes and broadcasts the owning agent's status
// when a task transitions to running/completed/failed/cancelled. It runs
// asynchronously to meet the 2-second requirement for status recomputation.
func (s *AgentStatusService) ReconcileAgentForTask(ctx context.Context, taskID pgtype.UUID) {
	// Check if the parent context is already cancelled (shutdown in progress).
	if s.bgCtx.Err() != nil {
		slog.Debug("skipping reconciliation for task: shutdown in progress",
			"task_id", pgutil.UUIDToString(taskID))
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		// Use a context derived from the parent background context with a 2-second deadline.
		// This ensures the goroutine exits promptly when shutdown is signaled.
		reconcileCtx, cancel := context.WithTimeout(s.bgCtx, 2*time.Second)
		defer cancel()

		agent, err := s.queries.GetAgentByTaskID(reconcileCtx, taskID)
		if err != nil {
			if s.bgCtx.Err() != nil {
				// Context cancelled due to shutdown — not an error.
				return
			}
			// Task may not have an associated agent (legacy tasks or tasks without agent_id).
			// This is not an error condition.
			slog.Debug("reconcile agent for task: no agent found",
				"task_id", pgutil.UUIDToString(taskID), "error", err)
			return
		}

		s.ReconcileAndBroadcast(reconcileCtx, agent.ID)
	}()
}

// uuidToString converts a pgtype.UUID to its string representation.
// This is a convenience wrapper around pgutil.UUIDToString for use within the service package.
func uuidToString(id pgtype.UUID) string {
	return pgutil.UUIDToString(id)
}
