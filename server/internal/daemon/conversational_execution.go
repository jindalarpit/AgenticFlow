package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/agenticflow/agenticflow/internal/daemon/execenv"
	"github.com/agenticflow/agenticflow/pkg/agent"
)

// Stage directive templates for conversational tasks.
// Each directive instructs the agent on what type of output to produce.
const (
	conversationalPlanDirective      = "You are a planning assistant. Produce a detailed plan document as your response."
	conversationalDesignDirective    = "You are a technical design assistant. Produce a technical design document as your response."
	conversationalTasksDirective     = "You are a task breakdown assistant. Produce an implementation task list as your response."
	conversationalExecutionDirective = "Implement the following task. Work in the configured workspace directory."
)

// stageDirective returns the directive string for a given deliverable type.
// Returns an empty string for unknown types.
func stageDirective(deliverableType string) string {
	switch deliverableType {
	case "plan":
		return conversationalPlanDirective
	case "design":
		return conversationalDesignDirective
	case "tasks":
		return conversationalTasksDirective
	case "execution":
		return conversationalExecutionDirective
	default:
		return ""
	}
}

// BuildConversationalPrompt constructs the prompt to send to the agent CLI
// for a conversational task.
//
// Branching logic:
//   - Follow-up message (priorSessionID is non-empty): return ONLY the user prompt.
//     The agent session already has full context from prior turns via --resume.
//   - First message (priorSessionID is empty): construct a full prompt with:
//     1. Stage directive (based on deliverableType)
//     2. User prompt
//     3. Prior context (if any, from previously completed deliverables)
func BuildConversationalPrompt(deliverableType string, userPrompt string, priorContext []string, priorSessionID string) string {
	// Follow-up: session has context, just pass the user's new prompt.
	if priorSessionID != "" {
		return userPrompt
	}

	// First message: build full prompt with directive + user prompt + prior context.
	var sb strings.Builder

	// 1. Stage directive.
	directive := stageDirective(deliverableType)
	if directive != "" {
		sb.WriteString(directive)
		sb.WriteString("\n\n")
	}

	// 2. User prompt.
	sb.WriteString(userPrompt)

	// 3. Prior context from previously completed deliverables.
	if len(priorContext) > 0 {
		sb.WriteString("\n\n--- Prior Context ---")
		for i, ctx := range priorContext {
			sb.WriteString(fmt.Sprintf("\n\n[Context %d]:\n%s", i+1, ctx))
		}
	}

	return sb.String()
}

// executeConversationalStage executes a conversational task using the session-based
// model. Unlike the legacy staged execution, this path:
//   - Captures agent stdout as the deliverable output (no file reading for plan/design/tasks)
//   - Reports session_id and work_dir on completion via the enhanced completion endpoint
//   - Passes --resume <prior_session_id> to the agent CLI when PriorSessionID is present
//   - Falls back to a fresh session if resume fails
//
// Requirements: 4.1, 4.2, 4.4, 4.5, 10.1, 10.3
func (d *Daemon) executeConversationalStage(ctx context.Context, task *PollResponse) {
	taskID := task.TaskID
	logger := d.logger.With(
		"task_id", taskID,
		"agent_type", task.AgentType,
		"deliverable_type", task.DeliverableType,
	)

	logger.Info("executing conversational task")

	// Resolve the binary path for the agent.
	agentEntry, ok := d.cfg.Agents[task.AgentType]
	if !ok {
		logger.Error("no agent entry found for type", "agent_type", task.AgentType)
		d.reportTaskFailure(ctx, taskID, "agent type not found: "+task.AgentType, -1)
		return
	}

	// Resolve agent configuration from the claim response.
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
			for k, v := range task.EnvVars {
				customEnv[k] = v
			}
		}
	}

	// Determine workspace directory.
	// For execution-type tasks, use the workspace_config's local_directory_path
	// with git clone support if the directory does not exist.
	// For plan/design/tasks, use the standard workspace directory.
	var workspaceDir string
	if task.DeliverableType == "execution" && task.WorkspaceConfig != nil && task.WorkspaceConfig.LocalDirectoryPath != "" {
		localDir := task.WorkspaceConfig.LocalDirectoryPath

		// Check if local_directory_path exists.
		if _, err := os.Stat(localDir); err == nil {
			// Directory exists — use as-is (Requirements 5.2, 5.5).
			workspaceDir = localDir
			logger.Info("using existing local directory as workspace", "path", localDir)
		} else if os.IsNotExist(err) {
			// Directory does not exist — check if we can clone.
			if task.WorkspaceConfig.GitRepoURL != "" {
				// Clone the repository to local_directory_path (Requirement 5.3).
				logger.Info("cloning repository to local directory",
					"git_repo_url", task.WorkspaceConfig.GitRepoURL,
					"path", localDir,
				)
				cloneCmd := exec.CommandContext(ctx, "git", "clone", task.WorkspaceConfig.GitRepoURL, localDir)
				cloneOutput, cloneErr := cloneCmd.CombinedOutput()
				if cloneErr != nil {
					errMsg := fmt.Sprintf("failed to clone repository: %s", strings.TrimSpace(string(cloneOutput)))
					logger.Error("git clone failed", "error", cloneErr, "output", string(cloneOutput))
					d.reportTaskFailure(ctx, taskID, errMsg, -1)
					return
				}
				workspaceDir = localDir
				logger.Info("repository cloned successfully", "path", localDir)
			} else {
				// No git_repo_url provided — fail the task (Requirement 5.4).
				errMsg := "local directory does not exist and no git_repo_url provided for cloning"
				logger.Error(errMsg, "path", localDir)
				d.reportTaskFailure(ctx, taskID, errMsg, -1)
				return
			}
		} else {
			// os.Stat returned an error other than NotExist (e.g., permission denied).
			errMsg := fmt.Sprintf("failed to access local directory: %v", err)
			logger.Error(errMsg, "path", localDir)
			d.reportTaskFailure(ctx, taskID, errMsg, -1)
			return
		}
	} else if task.PriorWorkDir != "" {
		// Reuse the workspace from a prior task on this stage.
		workspaceDir = task.PriorWorkDir
	}

	// If no explicit workspace, set up via ExecEnv (standard workspace management).
	if workspaceDir == "" {
		execTask := execenv.Task{
			ID:        taskID,
			AgentType: task.AgentType,
			Prompt:    task.Prompt,
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
		workspaceDir = env.WorkspaceDir
	}

	// Build the conversational prompt.
	stagePrompt := BuildConversationalPrompt(
		task.DeliverableType,
		task.Prompt,
		task.PriorContext,
		task.PriorSessionID,
	)

	// Report task start to server.
	if err := d.client.StartTask(ctx, taskID); err != nil {
		logger.Warn("failed to report task start", "error", err)
	}

	// Execute with session resume if PriorSessionID is present.
	result, sessionID := d.executeConversationalAgent(ctx, task, agentEntry.Path, stagePrompt, workspaceDir, model, systemPrompt, customEnv, customArgs, task.PriorSessionID, logger)

	// Handle session resume fallback: if execution failed with a resume session,
	// retry without --resume (fresh session).
	if result.Status == "failed" && task.PriorSessionID != "" {
		logger.Warn("session resume failed, retrying with fresh session",
			"prior_session_id", task.PriorSessionID,
			"error", result.Error,
		)
		result, sessionID = d.executeConversationalAgent(ctx, task, agentEntry.Path, stagePrompt, workspaceDir, model, systemPrompt, customEnv, customArgs, "", logger)
	}

	// Report result.
	switch result.Status {
	case "completed":
		logger.Info("conversational task completed successfully",
			"deliverable_type", task.DeliverableType,
			"duration_ms", result.DurationMs,
			"session_id", sessionID,
		)
		// For plan/design/tasks: stdout IS the deliverable output.
		// For execution: stdout is the summary.
		// In both cases, result.Output contains the agent's stdout text.
		output := result.Output

		// Report completion with session_id and work_dir for future follow-ups.
		if err := d.client.CompleteTaskConversational(ctx, taskID, output, sessionID, workspaceDir); err != nil {
			logger.Warn("failed to report conversational task completion", "error", err)
		}

	case "failed":
		logger.Info("conversational task failed",
			"error", result.Error,
			"duration_ms", result.DurationMs,
		)
		d.reportTaskFailure(ctx, taskID, fmt.Sprintf("conversational task failed: %s", result.Error), 1)

	case "aborted":
		logger.Info("conversational task aborted", "duration_ms", result.DurationMs)
		if ctx.Err() == nil {
			d.reportTaskFailure(ctx, taskID, "conversational task execution aborted", -1)
		}

	case "timeout":
		logger.Warn("conversational task timed out", "duration_ms", result.DurationMs)
		d.reportTaskFailure(ctx, taskID, fmt.Sprintf("conversational task timed out: %s", result.Error), -1)

	default:
		logger.Warn("conversational task ended with unknown status", "status", result.Status)
		d.reportTaskFailure(ctx, taskID, result.Error, -1)
	}
}

// executeConversationalAgent runs the agent backend for a conversational task.
// It returns the agent Result and the session ID (from the result or empty).
// The resumeSessionID parameter controls whether --resume is passed to the agent CLI.
func (d *Daemon) executeConversationalAgent(
	ctx context.Context,
	task *PollResponse,
	executablePath string,
	prompt string,
	workspaceDir string,
	model string,
	systemPrompt string,
	customEnv map[string]string,
	customArgs []string,
	resumeSessionID string,
	logger *slog.Logger,
) (agent.Result, string) {
	// Instantiate the agent backend.
	cfg := agent.Config{
		ExecutablePath: executablePath,
		Env:            customEnv,
		Logger:         logger,
	}
	backend, err := agent.New(task.AgentType, cfg)
	if err != nil {
		logger.Error("failed to create agent backend", "error", err)
		return agent.Result{
			Status: "failed",
			Error:  fmt.Sprintf("unsupported agent type: %v", err),
		}, ""
	}

	// Build execution options.
	timeout := d.cfg.AgentTimeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}
	opts := agent.ExecOptions{
		Cwd:             workspaceDir,
		Model:           model,
		SystemPrompt:    systemPrompt,
		Timeout:         timeout,
		CustomArgs:      customArgs,
		CustomEnv:       customEnv,
		ResumeSessionID: resumeSessionID,
	}

	// Execute the agent.
	session, err := backend.Execute(ctx, prompt, opts)
	if err != nil {
		logger.Error("agent execution failed to start", "error", err)
		return agent.Result{
			Status: "failed",
			Error:  fmt.Sprintf("agent execution failed: %v", err),
		}, ""
	}

	// Create BatchReporter to accumulate and send structured messages.
	reporter := &realHTTPMessageReporter{client: d.client}
	batchReporter := NewBatchReporter(reporter, task.TaskID, defaultFlushInterval, logger)

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
		d.reportTokenUsage(ctx, task.TaskID, result.Usage, logger)
	}

	return result, result.SessionID
}
