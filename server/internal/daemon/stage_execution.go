package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/agenticflow/agenticflow/internal/daemon/execenv"
	"github.com/agenticflow/agenticflow/pkg/agent"
)

// readStageOutputFile reads the generated output file for a stage from the workspace.
// For plan/design/tasks stages, the agent is instructed to create plan.md, design.md,
// or tasks.md respectively. Returns the file content as a string, or empty string if
// the file does not exist or cannot be read.
func readStageOutputFile(workspaceDir string, stageName string, logger *slog.Logger) string {
	filename := stageName + ".md"
	filePath := filepath.Join(workspaceDir, filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("failed to read stage output file",
				"file", filePath,
				"error", err,
			)
		}
		return ""
	}

	logger.Info("read stage output file",
		"file", filePath,
		"size_bytes", len(data),
	)
	return string(data)
}

// StageInfo represents a workflow stage returned in the poll response.
// When present in the PollResponse, it indicates the daemon should execute
// only this single stage rather than the full task.
type StageInfo struct {
	Name   string `json:"name"`   // "plan", "design", "tasks", "execution"
	Order  int    `json:"order"`  // 1-4
	Status string `json:"status"` // "pending"
}

// PriorStage represents a completed/approved stage with its output content.
// Prior stages provide context for subsequent stage execution.
type PriorStage struct {
	Name          string `json:"name"`
	Order         int    `json:"order"`
	Status        string `json:"status"`
	OutputContent string `json:"output_content,omitempty"`
}

// Stage prompt directive templates for each stage type.
const (
	planDirective = `Create a detailed plan document (plan.md) for the following task. Focus on approach, steps, and considerations.

Task:
%s`

	designDirective = `Create a technical design document (design.md) for the following task.

Task:
%s`

	tasksDirective = `Create an implementation task list (tasks.md) breaking down the work into discrete steps.

Task:
%s`

	executionDirective = `Implement the following task according to the plan and design.

Task:
%s`

	rejectionFeedbackSuffix = `

[Previous attempt was rejected with feedback: "%s"]
Please address the feedback and produce an improved version.`
)

// buildStagePrompt constructs the agent prompt for a specific stage,
// incorporating prior stage outputs and any rejection feedback.
//
// For each stage type, the prompt includes:
//   - plan: original task prompt only
//   - design: task prompt + plan.md content from prior stages
//   - tasks: task prompt + plan.md + design.md from prior stages
//   - execution: task prompt + all prior stage outputs
//
// If feedback is non-empty (re-execution after rejection), it is appended
// as additional context instructing the agent to address the feedback.
func buildStagePrompt(originalPrompt string, stageName string, priorStages []PriorStage, feedback string) string {
	var prompt string

	switch stageName {
	case "plan":
		prompt = fmt.Sprintf(planDirective, originalPrompt)
	case "design":
		prompt = fmt.Sprintf(designDirective, originalPrompt)
		prompt += buildPriorStageContext(priorStages, "plan")
	case "tasks":
		prompt = fmt.Sprintf(tasksDirective, originalPrompt)
		prompt += buildPriorStageContext(priorStages, "plan", "design")
	case "execution":
		prompt = fmt.Sprintf(executionDirective, originalPrompt)
		prompt += buildPriorStageContext(priorStages, "plan", "design", "tasks")
	default:
		// Unknown stage — use execution directive as fallback.
		prompt = fmt.Sprintf(executionDirective, originalPrompt)
		prompt += buildPriorStageContext(priorStages, "plan", "design", "tasks")
	}

	// Append rejection feedback if present (re-execution after rejection).
	if feedback != "" {
		prompt += fmt.Sprintf(rejectionFeedbackSuffix, feedback)
	}

	return prompt
}

// buildPriorStageContext appends the output content from prior stages
// in the specified order. Only stages whose names match the filter and
// have non-empty output content are included.
func buildPriorStageContext(priorStages []PriorStage, includeNames ...string) string {
	if len(priorStages) == 0 {
		return ""
	}

	// Build a set of names to include.
	nameSet := make(map[string]bool, len(includeNames))
	for _, n := range includeNames {
		nameSet[n] = true
	}

	var context string
	for _, stage := range priorStages {
		if !nameSet[stage.Name] {
			continue
		}
		if stage.OutputContent == "" {
			continue
		}
		context += fmt.Sprintf("\n\n--- %s.md (from prior stage) ---\n%s", stage.Name, stage.OutputContent)
	}

	return context
}

// executeStage executes a single workflow stage for a task.
// It builds the stage-specific prompt, runs the agent, captures output,
// and reports stage completion to the server.
func (d *Daemon) executeStage(ctx context.Context, task *PollResponse, currentStage *StageInfo, priorStages []PriorStage) {
	taskID := task.TaskID
	logger := d.logger.With(
		"task_id", taskID,
		"agent_type", task.AgentType,
		"stage_name", currentStage.Name,
		"stage_order", currentStage.Order,
	)

	logger.Info("executing workflow stage")

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
			for k, v := range task.EnvVars {
				customEnv[k] = v
			}
		}
	}

	// Determine rejection feedback from the current stage (if re-executing after rejection).
	feedback := task.StageFeedback

	// Build the stage-specific prompt.
	stagePrompt := buildStagePrompt(prompt, currentStage.Name, priorStages, feedback)

	// Setup workspace directory via ExecEnv.
	execTask := execenv.Task{
		ID:            taskID,
		AgentType:     task.AgentType,
		Prompt:        stagePrompt,
		WorkspaceMode: execenv.WorkspaceMode(task.WorkspaceMode),
		WorkspacePath: task.WorkspacePath,
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

	// Execute the agent with the stage-specific prompt.
	session, err := backend.Execute(ctx, stagePrompt, opts)
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

	// Handle the result.
	switch result.Status {
	case "completed":
		logger.Info("stage completed successfully",
			"stage_name", currentStage.Name,
			"duration_ms", result.DurationMs,
		)
		// For plan/design/tasks stages, read the generated file from the workspace.
		// The agent is instructed to create plan.md, design.md, or tasks.md.
		// For execution stages, use the agent's stdout output.
		outputContent := result.Output
		if currentStage.Name != "execution" {
			fileContent := readStageOutputFile(env.WorkspaceDir, currentStage.Name, logger)
			if fileContent != "" {
				outputContent = fileContent
			}
		}

		// Report stage completion to the server with the output content.
		d.reportStageCompletion(ctx, taskID, currentStage.Name, outputContent, logger)

	case "failed":
		logger.Info("stage failed", "error", result.Error, "duration_ms", result.DurationMs)
		d.reportTaskFailure(ctx, taskID, fmt.Sprintf("stage %q failed: %s", currentStage.Name, result.Error), 1)

	case "aborted":
		logger.Info("stage aborted", "duration_ms", result.DurationMs)
		if ctx.Err() == nil {
			d.reportTaskFailure(ctx, taskID, fmt.Sprintf("stage %q execution aborted", currentStage.Name), -1)
		}

	case "timeout":
		logger.Warn("stage timed out", "duration_ms", result.DurationMs)
		d.reportTaskFailure(ctx, taskID, fmt.Sprintf("stage %q timed out: %s", currentStage.Name, result.Error), -1)

	default:
		logger.Warn("stage ended with unknown status", "status", result.Status)
		d.reportTaskFailure(ctx, taskID, fmt.Sprintf("stage %q ended with unknown status: %s", currentStage.Name, result.Error), -1)
	}
}

// reportStageCompletion POSTs the stage output to the server's stage completion endpoint.
// POST /api/daemon/tasks/{taskId}/stages/{stageName}/complete
func (d *Daemon) reportStageCompletion(ctx context.Context, taskID, stageName, outputContent string, logger *slog.Logger) {
	if d.client == nil {
		logger.Warn("no HTTP client configured, cannot report stage completion")
		return
	}

	err := d.client.ReportStageCompletion(ctx, taskID, stageName, outputContent)
	if err != nil {
		logger.Error("failed to report stage completion",
			"task_id", taskID,
			"stage_name", stageName,
			"error", err,
		)
		// If we can't report stage completion, fail the task.
		d.reportTaskFailure(ctx, taskID, fmt.Sprintf("failed to report stage %q completion: %v", stageName, err), -1)
	} else {
		logger.Info("stage completion reported",
			"task_id", taskID,
			"stage_name", stageName,
		)
	}
}
