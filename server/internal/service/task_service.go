package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agenticflow/agenticflow/server/internal/realtime"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// maxPromptLength is the maximum allowed prompt length for task creation.
const maxPromptLength = 32000

// TaskService encapsulates business logic for task creation, listing,
// retrieval, cancellation, and status transitions.
type TaskService struct {
	q   db.Querier
	hub *realtime.Hub
}

// NewTaskService creates a new TaskService.
func NewTaskService(q db.Querier, hub *realtime.Hub) *TaskService {
	return &TaskService{q: q, hub: hub}
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

// CreateTaskParams holds the validated parameters for creating a task.
type CreateTaskParams struct {
	UserID    pgtype.UUID
	AgentType string
	Prompt    string
	AgentID   pgtype.UUID
	// Legacy multi-stage fields
	Deliverables  []string
	WorkspaceMode string
	WorkspacePath string
}

// Create validates inputs and creates a new task. It resolves the agent's
// runtime to determine agent_type, creates task_stage rows for multi-stage
// workflows, and broadcasts a task_created WebSocket event.
func (s *TaskService) Create(ctx context.Context, params CreateTaskParams) (db.Task, *ServiceError) {
	// Validate prompt.
	prompt := strings.TrimSpace(params.Prompt)
	if prompt == "" {
		return db.Task{}, Validation("prompt is required")
	}
	if len(prompt) > maxPromptLength {
		return db.Task{}, Validation("prompt exceeds 32000 character limit")
	}

	agentType := strings.TrimSpace(params.AgentType)

	// Resolve agent runtime if agent_id is provided.
	if params.AgentID.Valid {
		agent, err := s.q.GetAgent(ctx, params.AgentID)
		if err != nil {
			return db.Task{}, Validation("agent not found")
		}

		runtime, err := s.q.GetRuntimeByID(ctx, agent.RuntimeID)
		if err != nil {
			slog.Warn("create task: agent runtime not found, using provided agent_type",
				"agent_id", uuidToString(params.AgentID),
				"runtime_id", uuidToString(agent.RuntimeID),
				"error", err)
		} else {
			agentType = runtime.Provider
		}
	}

	if agentType == "" {
		return db.Task{}, Validation("agent_type is required")
	}

	// Default deliverables.
	deliverables := params.Deliverables
	if len(deliverables) == 0 {
		deliverables = []string{"execution"}
	} else {
		if errMsg := validateDeliverables(deliverables); errMsg != "" {
			return db.Task{}, Validation(errMsg)
		}
	}

	// Default workspace_mode.
	workspaceMode := params.WorkspaceMode
	if workspaceMode == "" {
		workspaceMode = "isolated"
	} else {
		if errMsg := validateWorkspaceMode(workspaceMode); errMsg != "" {
			return db.Task{}, Validation(errMsg)
		}
	}

	// Validate workspace_path.
	if errMsg := validateWorkspacePath(workspaceMode, params.WorkspacePath); errMsg != "" {
		return db.Task{}, Validation(errMsg)
	}

	// Determine if single-pass (no stages needed).
	singlePass := len(deliverables) == 1 && deliverables[0] == "execution"

	// Serialize deliverables to JSON.
	deliverablesJSON, err := json.Marshal(deliverables)
	if err != nil {
		slog.Error("create task: marshal deliverables failed", "error", err)
		return db.Task{}, Internal("failed to create task")
	}

	// Create the task.
	task, err := s.q.CreateTaskWithWorkflow(ctx, db.CreateTaskWithWorkflowParams{
		UserID:        params.UserID,
		AgentType:     agentType,
		Prompt:        prompt,
		AgentID:       params.AgentID,
		Deliverables:  deliverablesJSON,
		WorkspaceMode: workspaceMode,
		WorkspacePath: pgtype.Text{String: params.WorkspacePath, Valid: params.WorkspacePath != ""},
	})
	if err != nil {
		slog.Error("create task: insert failed", "error", err)
		return db.Task{}, Internal("failed to create task")
	}

	taskIDStr := uuidToString(task.ID)

	// Create task_stage rows for multi-stage workflows.
	if !singlePass {
		sortedDeliverables := orderDeliverables(deliverables)
		for _, d := range sortedDeliverables {
			_, err := s.q.CreateTaskStage(ctx, db.CreateTaskStageParams{
				TaskID:     task.ID,
				StageName:  d,
				StageOrder: int32(deliverableOrder[d]),
			})
			if err != nil {
				slog.Error("create task: create stage failed",
					"task_id", taskIDStr, "stage", d, "error", err)
				return db.Task{}, Internal("failed to create task stages")
			}
		}
	}

	// Broadcast task_created event.
	if s.hub != nil {
		payload := map[string]interface{}{
			"task_id":    taskIDStr,
			"agent_type": agentType,
			"prompt":     prompt,
			"status":     task.Status,
		}
		if params.AgentID.Valid {
			payload["agent_id"] = uuidToString(params.AgentID)
		}
		s.hub.Broadcast(realtime.Event{
			Type:    "task_created",
			Payload: payload,
		})
	}

	// Notify the target daemon via WebSocket push if connected.
	if s.hub != nil && params.AgentID.Valid {
		s.notifyDaemon(ctx, task)
	}

	return task, nil
}

// ---------------------------------------------------------------------------
// notifyDaemon
// ---------------------------------------------------------------------------

// notifyDaemon resolves the target daemon for a task (via agent → runtime → daemon)
// and sends a task_available WebSocket push event if the daemon is connected.
// This enables near-instant task pickup instead of relying on polling.
// Errors are logged but do not affect task creation — push is best-effort.
func (s *TaskService) notifyDaemon(ctx context.Context, task db.Task) {
	if !task.AgentID.Valid {
		return
	}

	agent, err := s.q.GetAgent(ctx, task.AgentID)
	if err != nil {
		slog.Warn("notifyDaemon: failed to get agent",
			"task_id", uuidToString(task.ID),
			"agent_id", uuidToString(task.AgentID),
			"error", err)
		return
	}

	if !agent.RuntimeID.Valid {
		slog.Warn("notifyDaemon: agent has no runtime bound",
			"task_id", uuidToString(task.ID),
			"agent_id", uuidToString(task.AgentID))
		return
	}

	runtime, err := s.q.GetRuntimeByID(ctx, agent.RuntimeID)
	if err != nil {
		slog.Warn("notifyDaemon: failed to get runtime",
			"task_id", uuidToString(task.ID),
			"runtime_id", uuidToString(agent.RuntimeID),
			"error", err)
		return
	}

	daemon, err := s.q.GetDaemonByID(ctx, runtime.DaemonID)
	if err != nil {
		slog.Warn("notifyDaemon: failed to get daemon",
			"task_id", uuidToString(task.ID),
			"daemon_id", uuidToString(runtime.DaemonID),
			"error", err)
		return
	}

	if s.hub.IsDaemonConnected(daemon.DaemonID) {
		s.hub.SendToDaemon(daemon.DaemonID, realtime.Event{
			Type:    "task_available",
			Payload: map[string]string{"task_id": uuidToString(task.ID)},
		})
		slog.Debug("notifyDaemon: sent task_available push",
			"task_id", uuidToString(task.ID),
			"daemon_id", daemon.DaemonID)
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

// ListTasksParams holds parameters for listing tasks.
type ListTasksParams struct {
	UserID  pgtype.UUID
	AgentID pgtype.UUID // optional filter
	Limit   int32
	Offset  int32
}

// List returns tasks for the given user with pagination. If AgentID is set,
// filters to tasks for that agent.
func (s *TaskService) List(ctx context.Context, params ListTasksParams) ([]db.Task, *ServiceError) {
	var tasks []db.Task
	var err error

	if params.AgentID.Valid {
		tasks, err = s.q.ListTasksByAgent(ctx, db.ListTasksByAgentParams{
			AgentID: params.AgentID,
			UserID:  params.UserID,
			Limit:   params.Limit,
			Offset:  params.Offset,
		})
	} else {
		tasks, err = s.q.ListTasksByUser(ctx, db.ListTasksByUserParams{
			UserID: params.UserID,
			Limit:  params.Limit,
			Offset: params.Offset,
		})
	}

	if err != nil {
		slog.Error("list tasks: query failed", "error", err)
		return nil, Internal("failed to list tasks")
	}

	return tasks, nil
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

// Get retrieves a single task by ID. Returns NotFound if the task does not
// exist, and Forbidden if the task does not belong to the given user.
func (s *TaskService) Get(ctx context.Context, taskID pgtype.UUID, userID string) (db.Task, *ServiceError) {
	task, err := s.q.GetTaskByID(ctx, taskID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return db.Task{}, NotFound("task not found")
		}
		slog.Error("get task: query failed", "error", err)
		return db.Task{}, Internal("failed to get task")
	}

	if uuidToString(task.UserID) != userID {
		return db.Task{}, NotFound("task not found")
	}

	return task, nil
}

// ---------------------------------------------------------------------------
// Cancel
// ---------------------------------------------------------------------------

// Cancel cancels a pending or running task. Returns NotFound if the task does
// not exist or does not belong to the user. The underlying SQL query enforces
// that only pending/running tasks can be cancelled.
func (s *TaskService) Cancel(ctx context.Context, taskID pgtype.UUID, userID pgtype.UUID) *ServiceError {
	err := s.q.CancelTask(ctx, db.CancelTaskParams{
		ID:     taskID,
		UserID: userID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return NotFound("task not found or not cancellable")
		}
		slog.Error("cancel task: update failed", "task_id", uuidToString(taskID), "error", err)
		return Internal("failed to cancel task")
	}

	return nil
}

// ---------------------------------------------------------------------------
// TransitionStatus
// ---------------------------------------------------------------------------

// TransitionStatus transitions a task to a new status. This is used by the
// daemon-facing handlers for start, complete, and fail transitions.
// It validates the transition is allowed and returns appropriate errors.
//
// Allowed transitions:
//   - pending  → running  (start)
//   - running  → completed (complete)
//   - running  → failed    (fail)
//   - pending  → cancelled (cancel — handled by Cancel method)
//   - running  → cancelled (cancel — handled by Cancel method)
func (s *TaskService) TransitionStatus(ctx context.Context, taskID pgtype.UUID, newStatus string) *ServiceError {
	// Validate the target status.
	validStatuses := map[string]bool{
		"running":   true,
		"completed": true,
		"failed":    true,
		"cancelled": true,
	}
	if !validStatuses[newStatus] {
		return Validation(fmt.Sprintf("invalid target status: %s", newStatus))
	}

	// Get the current task to check the transition is valid.
	task, err := s.q.GetTaskByID(ctx, taskID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return NotFound("task not found")
		}
		slog.Error("transition status: get task failed", "error", err)
		return Internal("failed to get task")
	}

	// Validate the state transition.
	if svcErr := validateTransition(task.Status, newStatus); svcErr != nil {
		return svcErr
	}

	// Perform the status update.
	err = s.q.UpdateTaskStatus(ctx, db.UpdateTaskStatusParams{
		ID:     taskID,
		Status: newStatus,
	})
	if err != nil {
		slog.Error("transition status: update failed",
			"task_id", uuidToString(taskID),
			"new_status", newStatus,
			"error", err)
		return Internal("failed to update task status")
	}

	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// validateTransition checks that the state transition from current to target
// is allowed.
func validateTransition(current, target string) *ServiceError {
	allowed := map[string][]string{
		"pending": {"running", "cancelled"},
		"running": {"completed", "failed", "cancelled"},
	}

	targets, ok := allowed[current]
	if !ok {
		return Conflict(fmt.Sprintf("task in terminal status %q cannot be transitioned", current))
	}

	for _, t := range targets {
		if t == target {
			return nil
		}
	}

	return Conflict(fmt.Sprintf("cannot transition from %q to %q", current, target))
}

// validDeliverables is the set of accepted deliverable values.
var validDeliverables = map[string]bool{
	"plan":      true,
	"design":    true,
	"tasks":     true,
	"execution": true,
}

// deliverableOrder defines the canonical execution order for deliverables.
var deliverableOrder = map[string]int{
	"plan":      1,
	"design":    2,
	"tasks":     3,
	"execution": 4,
}

// validateDeliverables checks that the deliverables array is non-empty and
// contains only valid values.
func validateDeliverables(deliverables []string) string {
	if len(deliverables) == 0 {
		return "deliverables must contain at least one valid value"
	}
	for _, d := range deliverables {
		if !validDeliverables[d] {
			return fmt.Sprintf("invalid deliverable: %s. Valid values: plan, design, tasks, execution", d)
		}
	}
	return ""
}

// validateWorkspaceMode checks that workspace_mode is either "isolated" or "existing".
func validateWorkspaceMode(mode string) string {
	if mode != "isolated" && mode != "existing" {
		return "workspace_mode must be 'isolated' or 'existing'"
	}
	return ""
}

// validateWorkspacePath checks workspace_path requirements based on the workspace_mode.
func validateWorkspacePath(mode, path string) string {
	if mode != "existing" {
		return ""
	}
	if strings.TrimSpace(path) == "" {
		return "workspace_path is required when workspace_mode is 'existing'"
	}
	if !strings.HasPrefix(path, "/") {
		return "workspace_path must be an absolute path"
	}
	return ""
}

// orderDeliverables sorts deliverables by canonical order and removes duplicates.
func orderDeliverables(deliverables []string) []string {
	if len(deliverables) == 0 {
		return []string{}
	}

	seen := make(map[string]bool, len(deliverables))
	unique := make([]string, 0, len(deliverables))
	for _, d := range deliverables {
		if !seen[d] {
			seen[d] = true
			unique = append(unique, d)
		}
	}

	// Sort by canonical order using insertion sort (small slice).
	for i := 1; i < len(unique); i++ {
		for j := i; j > 0 && deliverableOrder[unique[j]] < deliverableOrder[unique[j-1]]; j-- {
			unique[j], unique[j-1] = unique[j-1], unique[j]
		}
	}

	return unique
}
