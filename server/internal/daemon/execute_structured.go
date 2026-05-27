package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/agenticflow/agenticflow/internal/daemon/execenv"
	"github.com/agenticflow/agenticflow/pkg/agent"
)

// executeTaskStructured runs a task using the structured agent backend.
// It replaces the raw ExecEnv.RunWithStdin() approach with agent.New() →
// Backend.Execute() → Session.Messages → BatchReporter → structured events.
//
// This method is called from executeTask when the agent type is supported
// by the new backend architecture.
//
// Routing priority:
//  1. If the poll response contains a deliverable_type field, the daemon uses
//     the conversational execution path (session-based, stdout = deliverable).
//  2. If the poll response contains a current_stage field, the daemon executes
//     only that single stage (legacy staged execution with approval gates).
//  3. Otherwise, the daemon uses the existing single-pass flow (backward compat).
func (d *Daemon) executeTaskStructured(ctx context.Context, task *PollResponse) {
	// Branch: conversational task (deliverable_type present).
	if task.DeliverableType != "" {
		d.executeConversationalStage(ctx, task)
		return
	}

	// Branch: legacy staged execution vs single-pass.
	if task.CurrentStage != nil {
		d.executeStage(ctx, task, task.CurrentStage, task.PriorStages)
		return
	}

	// Single-pass execution (existing flow, backward compatible).
	d.executeTaskStructuredSinglePass(ctx, task)
}

// executeTaskStructuredSinglePass is the original single-pass execution flow.
// It is called when the task has no workflow stages (backward compatibility).
func (d *Daemon) executeTaskStructuredSinglePass(ctx context.Context, task *PollResponse) {
	taskID := task.TaskID
	logger := d.logger.With("task_id", taskID, "agent_type", task.AgentType)

	// Resolve the binary path for the agent.
	agentEntry, ok := d.cfg.Agents[task.AgentType]
	if !ok {
		logger.Error("no agent entry found for type", "agent_type", task.AgentType)
		d.reportTaskFailure(ctx, taskID, "agent type not found: "+task.AgentType, -1)
		return
	}

	// Resolve agent configuration from the claim response.
	prompt := task.Prompt
	model := task.Model
	var customEnv map[string]string
	var customArgs []string
	var systemPrompt string

	if task.Agent != nil {
		logger = logger.With("agent_id", task.Agent.ID, "agent_name", task.Agent.Name)
		if task.Agent.Model != "" {
			model = task.Agent.Model
		}
		customEnv = task.Agent.CustomEnv
		customArgs = task.Agent.CustomArgs
		if task.Agent.Instructions != "" {
			systemPrompt = task.Agent.Instructions
		}
	}
	if task.EnvVars != nil {
		if customEnv == nil {
			customEnv = task.EnvVars
		} else {
			// Task-level env overrides agent-level.
			for k, v := range task.EnvVars {
				customEnv[k] = v
			}
		}
	}

	// Setup workspace directory via ExecEnv (retained for workspace management).
	execTask := execenv.Task{
		ID:        taskID,
		AgentType: task.AgentType,
		Prompt:    prompt,
	}
	execCfg := execenv.Config{
		WorkspacesRoot: d.cfg.WorkspacesRoot,
		AgentTimeout:   d.cfg.AgentTimeout,
	}
	env, err := execenv.NewExecEnv(execTask, execCfg, agentEntry.Path, logger)
	if err != nil {
		logger.Error("failed to create exec environment", "error", err)
		d.reportTaskFailure(ctx, taskID, fmt.Sprintf("failed to create exec environment: %v", err), -1)
		return
	}
	if err := env.Setup(); err != nil {
		logger.Error("workspace setup failed", "error", err)
		d.reportTaskFailure(ctx, taskID, fmt.Sprintf("workspace setup failed: %v", err), -1)
		return
	}

	// Report task start to server.
	if err := d.client.StartTask(ctx, taskID); err != nil {
		logger.Warn("failed to report task start", "error", err)
	}

	// Instantiate the agent backend.
	cfg := agent.Config{
		ExecutablePath: agentEntry.Path,
		Env:            customEnv,
		Logger:         logger,
	}
	backend, err := agent.New(task.AgentType, cfg)
	if err != nil {
		logger.Error("failed to create agent backend", "error", err)
		d.reportTaskFailure(ctx, taskID, fmt.Sprintf("unsupported agent type: %v", err), -1)
		return
	}

	// Build execution options.
	timeout := d.cfg.AgentTimeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}
	opts := agent.ExecOptions{
		Cwd:          env.WorkspaceDir,
		Model:        model,
		SystemPrompt: systemPrompt,
		Timeout:      timeout,
		CustomArgs:   customArgs,
		CustomEnv:    customEnv,
	}

	// Execute the agent.
	session, err := backend.Execute(ctx, prompt, opts)
	if err != nil {
		logger.Error("agent execution failed to start", "error", err)
		d.reportTaskFailure(ctx, taskID, fmt.Sprintf("agent execution failed: %v", err), -1)
		return
	}

	// Create BatchReporter to accumulate and send structured messages.
	reporter := &realHTTPMessageReporter{client: d.client}
	batchReporter := NewBatchReporter(reporter, taskID, defaultFlushInterval, logger)

	// Read from Session.Messages and feed each Message into BatchReporter.
	go func() {
		for msg := range session.Messages {
			batchReporter.Feed(msg)
		}
	}()

	// Wait for the final Result.
	result := <-session.Result

	// Close the batch reporter (final flush).
	batchReporter.Close()

	// Report token usage if available.
	if len(result.Usage) > 0 {
		d.reportTokenUsage(ctx, taskID, result.Usage, logger)
	}

	// Report task completion or failure to server.
	switch result.Status {
	case "completed":
		logger.Info("task completed successfully (structured)", "duration_ms", result.DurationMs)
		if err := d.client.CompleteTask(ctx, taskID, result.Output, 0); err != nil {
			logger.Warn("failed to report task completion", "error", err)
		}
	case "failed":
		logger.Info("task failed (structured)", "error", result.Error, "duration_ms", result.DurationMs)
		d.reportTaskFailure(ctx, taskID, result.Error, 1)
	case "aborted":
		logger.Info("task aborted (structured)", "duration_ms", result.DurationMs)
		// Don't report if daemon is shutting down.
		if ctx.Err() == nil {
			d.reportTaskFailure(ctx, taskID, "execution aborted", -1)
		}
	case "timeout":
		logger.Warn("task timed out (structured)", "duration_ms", result.DurationMs)
		d.reportTaskFailure(ctx, taskID, result.Error, -1)
	default:
		logger.Warn("task ended with unknown status (structured)", "status", result.Status)
		d.reportTaskFailure(ctx, taskID, result.Error, -1)
	}
}

// realHTTPMessageReporter adapts the RealHTTPClient to the MessageReporter interface.
type realHTTPMessageReporter struct {
	client HTTPClient
}

func (r *realHTTPMessageReporter) ReportTaskMessages(taskID string, messages []TaskMessageData) error {
	if r.client == nil {
		return nil
	}
	// Convert TaskMessageData to TaskMessage for the legacy ReportMessages endpoint.
	// The server handler detects structured format based on the "type" field.
	legacyMsgs := make([]TaskMessage, 0, len(messages))
	for _, m := range messages {
		legacyMsgs = append(legacyMsgs, TaskMessage{
			Sequence: int(m.Seq),
			Stream:   "stdout",
			Content:  formatStructuredContent(m),
		})
	}
	return r.client.ReportMessages(context.Background(), taskID, legacyMsgs)
}

// formatStructuredContent serializes a TaskMessageData for the legacy message format.
// This is a temporary bridge until the server fully supports structured payloads.
func formatStructuredContent(m TaskMessageData) string {
	switch m.Type {
	case "text", "thinking", "error", "status":
		return m.Content
	case "tool_use":
		return fmt.Sprintf("[tool_use] %s", m.Tool)
	case "tool_result":
		if len(m.Output) > 200 {
			return fmt.Sprintf("[tool_result] %s: %s...", m.Tool, m.Output[:200])
		}
		return fmt.Sprintf("[tool_result] %s: %s", m.Tool, m.Output)
	default:
		return m.Content
	}
}

// isStructuredBackendSupported returns true if the agent type has a structured
// backend implementation available.
func isStructuredBackendSupported(agentType string) bool {
	for _, t := range agent.SupportedTypes {
		if t == agentType {
			return true
		}
	}
	return false
}

// reportTokenUsage sends token usage data to the server after task completion.
func (d *Daemon) reportTokenUsage(ctx context.Context, taskID string, usage map[string]agent.TokenUsage, logger *slog.Logger) {
	type usageEntry struct {
		Model            string `json:"model"`
		InputTokens      int64  `json:"input_tokens"`
		OutputTokens     int64  `json:"output_tokens"`
		CacheReadTokens  int64  `json:"cache_read_tokens,omitempty"`
		CacheWriteTokens int64  `json:"cache_write_tokens,omitempty"`
	}

	entries := make([]usageEntry, 0, len(usage))
	for model, u := range usage {
		entries = append(entries, usageEntry{
			Model:            model,
			InputTokens:      u.InputTokens,
			OutputTokens:     u.OutputTokens,
			CacheReadTokens:  u.CacheReadTokens,
			CacheWriteTokens: u.CacheWriteTokens,
		})
	}

	logger.Info("reporting token usage", "task_id", taskID, "models", len(entries))
	// Token usage reporting is best-effort — don't fail the task if it fails.
	// The server endpoint POST /api/daemon/tasks/{id}/usage will be created
	// when the server handler is updated.
	_ = entries // TODO: POST to /api/daemon/tasks/{id}/usage when endpoint exists
}
