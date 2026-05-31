package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/agenticflow/agenticflow/daemon/internal/execution/execenv"
	"github.com/agenticflow/agenticflow/daemon/pkg/agent"
)

const (
	conversationalPlanDirective      = "You are a planning assistant. Produce a detailed plan document as your response."
	conversationalDesignDirective    = "You are a technical design assistant. Produce a technical design document as your response."
	conversationalTasksDirective     = "You are a task breakdown assistant. Produce an implementation task list as your response."
	conversationalExecutionDirective = "Implement the following task. Work in the configured workspace directory."
)

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

// BuildConversationalPrompt constructs the prompt for a conversational task.
func BuildConversationalPrompt(deliverableType string, userPrompt string, priorContext []string, priorSessionID string) string {
	if priorSessionID != "" {
		return userPrompt
	}

	var sb strings.Builder
	directive := stageDirective(deliverableType)
	if directive != "" {
		sb.WriteString(directive)
		sb.WriteString("\n\n")
	}
	sb.WriteString(userPrompt)
	if len(priorContext) > 0 {
		sb.WriteString("\n\n--- Prior Context ---")
		for i, ctx := range priorContext {
			sb.WriteString(fmt.Sprintf("\n\n[Context %d]:\n%s", i+1, ctx))
		}
	}
	return sb.String()
}

// executeConversationalStage executes a conversational task using the session-based model.
func (d *Daemon) executeConversationalStage(ctx context.Context, task *PollResponse) {
	taskID := task.TaskID
	logger := d.logger.With(
		"task_id", taskID,
		"agent_type", task.AgentType,
		"deliverable_type", task.DeliverableType,
	)

	logger.Info("executing conversational task")

	agentEntry, ok := d.cfg.Agents[task.AgentType]
	if !ok {
		logger.Error("no agent entry found for type", "agent_type", task.AgentType)
		d.reportTaskFailure(ctx, taskID, "agent type not found: "+task.AgentType, -1)
		return
	}

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
	// Fallback to daemon-detected model (from AF_<AGENT>_MODEL env var)
	if model == "" {
		model = agentEntry.Model
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

	var workspaceDir string
	if task.DeliverableType == "execution" && task.WorkspaceConfig != nil && task.WorkspaceConfig.LocalDirectoryPath != "" {
		localDir := task.WorkspaceConfig.LocalDirectoryPath

		if _, err := os.Stat(localDir); err == nil {
			workspaceDir = localDir
			logger.Info("using existing local directory as workspace", "path", localDir)
		} else if os.IsNotExist(err) {
			if task.WorkspaceConfig.GitRepoURL != "" {
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
			} else {
				errMsg := "local directory does not exist and no git_repo_url provided for cloning"
				logger.Error(errMsg, "path", localDir)
				d.reportTaskFailure(ctx, taskID, errMsg, -1)
				return
			}
		} else {
			errMsg := fmt.Sprintf("failed to access local directory: %v", err)
			logger.Error(errMsg, "path", localDir)
			d.reportTaskFailure(ctx, taskID, errMsg, -1)
			return
		}
	} else if task.PriorWorkDir != "" {
		workspaceDir = task.PriorWorkDir
	}

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

	stagePrompt := BuildConversationalPrompt(
		task.DeliverableType,
		task.Prompt,
		task.PriorContext,
		task.PriorSessionID,
	)

	if err := d.client.StartTask(ctx, taskID); err != nil {
		logger.Warn("failed to report task start", "error", err)
	}

	// Inject skills and MCP config before execution.
	injResult, mcpArgs, injErr := injectSkillsAndMCP(workspaceDir, task.AgentType, task.Agent, logger)
	if injErr != nil {
		logger.Error("skill/MCP injection failed", "error", injErr)
		d.reportTaskFailure(ctx, taskID, fmt.Sprintf("skill/MCP injection failed: %v", injErr), -1)
		return
	}
	defer cleanupInjection(injResult, logger)

	// Append MCP args to custom args.
	if len(mcpArgs) > 0 {
		customArgs = append(customArgs, mcpArgs...)
	}

	result, sessionID := d.executeConversationalAgent(ctx, task, agentEntry.Path, stagePrompt, workspaceDir, model, systemPrompt, customEnv, customArgs, task.PriorSessionID, logger)

	// Retry without resume if session resume failed.
	if result.Status == "failed" && task.PriorSessionID != "" {
		logger.Warn("session resume failed, retrying with fresh session",
			"prior_session_id", task.PriorSessionID,
			"error", result.Error,
		)
		result, sessionID = d.executeConversationalAgent(ctx, task, agentEntry.Path, stagePrompt, workspaceDir, model, systemPrompt, customEnv, customArgs, "", logger)
	}

	switch result.Status {
	case "completed":
		logger.Info("conversational task completed successfully",
			"deliverable_type", task.DeliverableType,
			"duration_ms", result.DurationMs,
			"session_id", sessionID,
		)
		output := result.Output
		if err := d.client.CompleteTaskConversational(ctx, taskID, output, sessionID, workspaceDir); err != nil {
			logger.Warn("failed to report conversational task completion", "error", err)
		}
	case "failed":
		logger.Info("conversational task failed", "error", result.Error, "duration_ms", result.DurationMs)
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

	session, err := backend.Execute(ctx, prompt, opts)
	if err != nil {
		logger.Error("agent execution failed to start", "error", err)
		return agent.Result{
			Status: "failed",
			Error:  fmt.Sprintf("agent execution failed: %v", err),
		}, ""
	}

	reporter := &realHTTPMessageReporter{client: d.client}
	batchReporter := NewBatchReporter(reporter, task.TaskID, defaultFlushInterval, logger)

	msgsDone := make(chan struct{})
	go func() {
		defer close(msgsDone)
		for msg := range session.Messages {
			batchReporter.Feed(msg)
		}
	}()

	result := <-session.Result
	<-msgsDone
	batchReporter.Close()

	if len(result.Usage) > 0 {
		d.reportTokenUsage(ctx, task.TaskID, result.Usage, logger)
	}

	return result, result.SessionID
}
