// Package execution handles task execution, process spawning, and output streaming.
// It provides the Executor type which manages the lifecycle of spawning an agent
// CLI process, capturing its output, and reporting results back to the server.
package execution

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/agenticflow/agenticflow/daemon/internal/execution/execenv"
	"github.com/agenticflow/agenticflow/shared/api"
	"github.com/agenticflow/agenticflow/shared/constants"
)

// Reporter defines the interface for reporting task execution progress
// and results back to the server.
type Reporter interface {
	// ReportMessages sends streaming output messages to the server.
	ReportMessages(ctx context.Context, taskID string, messages []api.TaskMessageEntry) error
	// CompleteTask reports successful task completion.
	CompleteTask(ctx context.Context, taskID string, req api.TaskCompleteRequest) error
	// FailTask reports task failure.
	FailTask(ctx context.Context, taskID string, req api.TaskFailRequest) error
}

// TaskConfig holds the configuration for executing a single task.
type TaskConfig struct {
	TaskID        string
	AgentType     string
	Prompt        string
	Model         string
	ArgsTemplate  string
	BinaryPath    string
	WorkspaceMode execenv.WorkspaceMode
	WorkspacePath string

	// SystemPrompt is an optional system prompt override (e.g., from brief injection).
	SystemPrompt string

	// Agent configuration from the task claim response.
	CustomEnv  map[string]string
	CustomArgs []string

	// Daemon-level configuration.
	WorkspacesRoot string
	AgentTimeout   time.Duration
}

// Result holds the outcome of a task execution.
type Result struct {
	ExitCode int
	Output   string
	Error    string
	Success  bool
}

// Executor manages the execution of agent CLI processes for tasks.
type Executor struct {
	logger   *slog.Logger
	reporter Reporter
}

// NewExecutor creates a new Executor with the given logger and reporter.
func NewExecutor(logger *slog.Logger, reporter Reporter) *Executor {
	return &Executor{
		logger:   logger,
		reporter: reporter,
	}
}

// ExecuteWithHooks runs a task through the full execution lifecycle with optional hooks:
//  1. Apply brief injection hook (if provided)
//  2. Create execution environment (workspace)
//  3. Setup workspace directory
//  4. Spawn agent CLI process with merged env and custom args
//  5. Stream stdout/stderr as TaskMessageEntry chunks, calling OnStdout/OnStderr hooks
//  6. Provide stdin pipe via StdinProvider hook (if provided)
//  7. Handle process completion (exit code 0 = success, non-zero = failure)
//  8. Report result to server
//
// The execution respects the configured agent timeout (defaults to
// constants.DefaultAgentTimeout if not set in TaskConfig).
func (e *Executor) ExecuteWithHooks(ctx context.Context, cfg TaskConfig, hooks ExecutionHooks) Result {
	// Apply brief injection hook if provided.
	if hooks.BriefInjector != nil {
		newPrompt, systemPrompt, err := hooks.BriefInjector(cfg.Prompt)
		if err != nil {
			e.logger.Warn("brief injection failed", "error", err)
		} else {
			cfg.Prompt = newPrompt
			if systemPrompt != "" {
				cfg.SystemPrompt = systemPrompt
			}
		}
	}

	// If stdin pipe is requested, use ExecuteWithStdin path.
	if hooks.StdinProvider != nil {
		return e.executeWithStdinAndHooks(ctx, cfg, hooks)
	}

	return e.executeInternal(ctx, cfg, hooks)
}

// Execute runs a task through the full execution lifecycle:
//  1. Create execution environment (workspace)
//  2. Setup workspace directory
//  3. Spawn agent CLI process with merged env and custom args
//  4. Stream stdout/stderr as TaskMessageEntry chunks
//  5. Handle process completion (exit code 0 = success, non-zero = failure)
//  6. Report result to server
//
// The execution respects the configured agent timeout (defaults to
// constants.DefaultAgentTimeout if not set in TaskConfig).
func (e *Executor) Execute(ctx context.Context, cfg TaskConfig) Result {
	return e.executeInternal(ctx, cfg, ExecutionHooks{})
}

// executeInternal is the core execution logic shared by Execute and ExecuteWithHooks.
func (e *Executor) executeInternal(ctx context.Context, cfg TaskConfig, hooks ExecutionHooks) Result {
	taskLogger := e.logger.With("task_id", cfg.TaskID, "agent_type", cfg.AgentType)

	// Resolve timeout.
	timeout := cfg.AgentTimeout
	if timeout == 0 {
		timeout = constants.DefaultAgentTimeout
	}

	// Merge environment variables: agent custom_env with system env.
	mergedEnv := execenv.MergeEnv(nil, cfg.CustomEnv, nil, taskLogger)

	// Create the execution environment.
	task := execenv.Task{
		ID:            cfg.TaskID,
		AgentType:     cfg.AgentType,
		Prompt:        cfg.Prompt,
		Model:         cfg.Model,
		ArgsTemplate:  cfg.ArgsTemplate,
		EnvVars:       mergedEnv,
		CustomArgs:    cfg.CustomArgs,
		WorkspaceMode: cfg.WorkspaceMode,
		WorkspacePath: cfg.WorkspacePath,
	}

	execCfg := execenv.Config{
		WorkspacesRoot: cfg.WorkspacesRoot,
		AgentTimeout:   timeout,
	}

	env, err := execenv.NewExecEnv(task, execCfg, cfg.BinaryPath, taskLogger)
	if err != nil {
		taskLogger.Error("failed to create exec environment", "error", err)
		e.reportFailure(ctx, cfg.TaskID, fmt.Sprintf("failed to create exec environment: %v", err), -1)
		return Result{ExitCode: -1, Error: err.Error(), Success: false}
	}

	// Apply system prompt if provided (e.g., from brief injection).
	if cfg.SystemPrompt != "" {
		env.SystemPrompt = cfg.SystemPrompt
	}

	// Setup workspace.
	if err := env.Setup(); err != nil {
		taskLogger.Error("workspace setup failed", "error", err)
		e.reportFailure(ctx, cfg.TaskID, fmt.Sprintf("workspace setup failed: %v", err), -1)
		return Result{ExitCode: -1, Error: err.Error(), Success: false}
	}

	// Create streaming writers that capture output and report to server.
	var sequence atomic.Int32
	stdoutBuf := &truncatingBuffer{maxBytes: maxStdoutBytes}
	stderrBuf := &tailBuffer{maxChars: maxStderrChars}

	// Create backpressure buffer and flusher for non-blocking streaming.
	buffer := NewBackpressureBuffer(100, 1*1024*1024)
	flusher := NewFlusher(buffer, e.reporter, cfg.TaskID, taskLogger)
	flusher.Run(ctx)

	stdoutWriter := &streamingWriter{
		inner:    stdoutBuf,
		buffer:   buffer,
		stream:   "stdout",
		sequence: &sequence,
		onWrite:  hooks.OnStdout,
	}

	stderrWriter := &streamingWriter{
		inner:    stderrBuf,
		buffer:   buffer,
		stream:   "stderr",
		sequence: &sequence,
		onWrite:  hooks.OnStderr,
	}

	// Run the agent process with timeout.
	taskCtx, taskCancel := context.WithTimeout(ctx, timeout)
	defer taskCancel()

	exitCode, runErr := env.Run(taskCtx, stdoutWriter, stderrWriter)

	// Stop the flusher to drain remaining buffered messages before reporting.
	flusher.Stop()

	// Determine outcome.
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if taskCtx.Err() == context.DeadlineExceeded {
		// Task timed out.
		taskLogger.Warn("task timed out", "timeout", timeout)
		errMsg := fmt.Sprintf("task timed out after %s", timeout)
		if stderr != "" {
			errMsg += "\nstderr: " + stderr
		}
		e.reportFailure(ctx, cfg.TaskID, errMsg, exitCode)
		return Result{ExitCode: exitCode, Output: stdout, Error: errMsg, Success: false}
	}

	if runErr != nil && ctx.Err() != nil {
		// Parent context cancelled (daemon shutting down).
		taskLogger.Info("task interrupted by daemon shutdown")
		return Result{ExitCode: exitCode, Output: stdout, Error: "interrupted", Success: false}
	}

	if exitCode == 0 && runErr == nil {
		// Success.
		taskLogger.Info("task completed successfully")
		if e.reporter != nil {
			req := api.TaskCompleteRequest{
				Output:   stdout,
				ExitCode: int32(exitCode),
			}
			if err := e.reporter.CompleteTask(ctx, cfg.TaskID, req); err != nil {
				taskLogger.Warn("failed to report task completion", "error", err)
			}
		}
		return Result{ExitCode: exitCode, Output: stdout, Success: true}
	}

	// Failure.
	taskLogger.Info("task failed", "exit_code", exitCode, "error", runErr)
	errMsg := stderr
	if errMsg == "" && runErr != nil {
		errMsg = runErr.Error()
	}
	e.reportFailure(ctx, cfg.TaskID, errMsg, exitCode)
	return Result{ExitCode: exitCode, Output: stdout, Error: errMsg, Success: false}
}

// ExecuteWithStdin runs a task with stdin pipe support for bidirectional communication.
// Returns the stdin pipe, a done channel, and any startup error.
func (e *Executor) ExecuteWithStdin(ctx context.Context, cfg TaskConfig) (io.WriteCloser, <-chan Result, error) {
	taskLogger := e.logger.With("task_id", cfg.TaskID, "agent_type", cfg.AgentType)

	// Resolve timeout.
	timeout := cfg.AgentTimeout
	if timeout == 0 {
		timeout = constants.DefaultAgentTimeout
	}

	// Merge environment variables.
	mergedEnv := execenv.MergeEnv(nil, cfg.CustomEnv, nil, taskLogger)

	// Create the execution environment.
	task := execenv.Task{
		ID:            cfg.TaskID,
		AgentType:     cfg.AgentType,
		Prompt:        cfg.Prompt,
		Model:         cfg.Model,
		ArgsTemplate:  cfg.ArgsTemplate,
		EnvVars:       mergedEnv,
		CustomArgs:    cfg.CustomArgs,
		WorkspaceMode: cfg.WorkspaceMode,
		WorkspacePath: cfg.WorkspacePath,
	}

	execCfg := execenv.Config{
		WorkspacesRoot: cfg.WorkspacesRoot,
		AgentTimeout:   timeout,
	}

	env, err := execenv.NewExecEnv(task, execCfg, cfg.BinaryPath, taskLogger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create exec environment: %w", err)
	}

	// Setup workspace.
	if err := env.Setup(); err != nil {
		return nil, nil, fmt.Errorf("workspace setup failed: %w", err)
	}

	// Create streaming writers.
	var sequence atomic.Int32
	stdoutBuf := &truncatingBuffer{maxBytes: maxStdoutBytes}
	stderrBuf := &tailBuffer{maxChars: maxStderrChars}

	// Create backpressure buffer and flusher for non-blocking streaming.
	buffer := NewBackpressureBuffer(100, 1*1024*1024)
	flusher := NewFlusher(buffer, e.reporter, cfg.TaskID, taskLogger)
	flusher.Run(ctx)

	stdoutWriter := &streamingWriter{
		inner:    stdoutBuf,
		buffer:   buffer,
		stream:   "stdout",
		sequence: &sequence,
	}

	stderrWriter := &streamingWriter{
		inner:    stderrBuf,
		buffer:   buffer,
		stream:   "stderr",
		sequence: &sequence,
	}

	// Run with stdin pipe and timeout.
	taskCtx, taskCancel := context.WithTimeout(ctx, timeout)

	stdinPipe, envDone, startErr := env.RunWithStdin(taskCtx, stdoutWriter, stderrWriter)
	if startErr != nil {
		taskCancel()
		flusher.Stop()
		return nil, nil, fmt.Errorf("failed to start agent process: %w", startErr)
	}

	// Wrap the envDone channel to produce Result values.
	resultCh := make(chan Result, 1)
	go func() {
		defer taskCancel()
		envResult := <-envDone

		// Stop the flusher to drain remaining buffered messages before reporting.
		flusher.Stop()

		stdout := stdoutBuf.String()
		stderr := stderrBuf.String()

		if taskCtx.Err() == context.DeadlineExceeded {
			errMsg := fmt.Sprintf("task timed out after %s", timeout)
			if stderr != "" {
				errMsg += "\nstderr: " + stderr
			}
			resultCh <- Result{ExitCode: envResult.ExitCode, Output: stdout, Error: errMsg, Success: false}
			return
		}

		if envResult.ExitCode == 0 && envResult.Err == nil {
			resultCh <- Result{ExitCode: 0, Output: stdout, Success: true}
		} else {
			errMsg := stderr
			if errMsg == "" && envResult.Err != nil {
				errMsg = envResult.Err.Error()
			}
			resultCh <- Result{ExitCode: envResult.ExitCode, Output: stdout, Error: errMsg, Success: false}
		}
	}()

	return stdinPipe, resultCh, nil
}

// executeWithStdinAndHooks runs a task with stdin pipe support and hooks.
// It spawns the process, provides the stdin pipe via the StdinProvider hook,
// waits for completion, and reports the result.
func (e *Executor) executeWithStdinAndHooks(ctx context.Context, cfg TaskConfig, hooks ExecutionHooks) Result {
	taskLogger := e.logger.With("task_id", cfg.TaskID, "agent_type", cfg.AgentType)

	// Resolve timeout.
	timeout := cfg.AgentTimeout
	if timeout == 0 {
		timeout = constants.DefaultAgentTimeout
	}

	// Merge environment variables.
	mergedEnv := execenv.MergeEnv(nil, cfg.CustomEnv, nil, taskLogger)

	// Create the execution environment.
	task := execenv.Task{
		ID:            cfg.TaskID,
		AgentType:     cfg.AgentType,
		Prompt:        cfg.Prompt,
		Model:         cfg.Model,
		ArgsTemplate:  cfg.ArgsTemplate,
		EnvVars:       mergedEnv,
		CustomArgs:    cfg.CustomArgs,
		WorkspaceMode: cfg.WorkspaceMode,
		WorkspacePath: cfg.WorkspacePath,
	}

	execCfg := execenv.Config{
		WorkspacesRoot: cfg.WorkspacesRoot,
		AgentTimeout:   timeout,
	}

	env, err := execenv.NewExecEnv(task, execCfg, cfg.BinaryPath, taskLogger)
	if err != nil {
		taskLogger.Error("failed to create exec environment", "error", err)
		e.reportFailure(ctx, cfg.TaskID, fmt.Sprintf("failed to create exec environment: %v", err), -1)
		return Result{ExitCode: -1, Error: err.Error(), Success: false}
	}

	// Apply system prompt if provided (e.g., from brief injection).
	if cfg.SystemPrompt != "" {
		env.SystemPrompt = cfg.SystemPrompt
	}

	// Setup workspace.
	if err := env.Setup(); err != nil {
		taskLogger.Error("workspace setup failed", "error", err)
		e.reportFailure(ctx, cfg.TaskID, fmt.Sprintf("workspace setup failed: %v", err), -1)
		return Result{ExitCode: -1, Error: err.Error(), Success: false}
	}

	// Create streaming writers with hooks.
	var sequence atomic.Int32
	stdoutBuf := &truncatingBuffer{maxBytes: maxStdoutBytes}
	stderrBuf := &tailBuffer{maxChars: maxStderrChars}

	// Create backpressure buffer and flusher for non-blocking streaming.
	buffer := NewBackpressureBuffer(100, 1*1024*1024)
	flusher := NewFlusher(buffer, e.reporter, cfg.TaskID, taskLogger)
	flusher.Run(ctx)

	stdoutWriter := &streamingWriter{
		inner:    stdoutBuf,
		buffer:   buffer,
		stream:   "stdout",
		sequence: &sequence,
		onWrite:  hooks.OnStdout,
	}

	stderrWriter := &streamingWriter{
		inner:    stderrBuf,
		buffer:   buffer,
		stream:   "stderr",
		sequence: &sequence,
		onWrite:  hooks.OnStderr,
	}

	// Run with stdin pipe and timeout.
	taskCtx, taskCancel := context.WithTimeout(ctx, timeout)
	defer taskCancel()

	stdinPipe, envDone, startErr := env.RunWithStdin(taskCtx, stdoutWriter, stderrWriter)
	if startErr != nil {
		flusher.Stop()
		taskLogger.Error("failed to start agent process", "error", startErr)
		e.reportFailure(ctx, cfg.TaskID, fmt.Sprintf("failed to start agent process: %v", startErr), -1)
		return Result{ExitCode: -1, Error: startErr.Error(), Success: false}
	}

	// Provide the stdin pipe to the caller via the hook.
	if hooks.StdinProvider != nil {
		hooks.StdinProvider(stdinPipe)
	}

	// Wait for the process to complete.
	envResult := <-envDone

	// Stop the flusher to drain remaining buffered messages before reporting.
	flusher.Stop()

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if taskCtx.Err() == context.DeadlineExceeded {
		taskLogger.Warn("task timed out", "timeout", timeout)
		errMsg := fmt.Sprintf("task timed out after %s", timeout)
		if stderr != "" {
			errMsg += "\nstderr: " + stderr
		}
		e.reportFailure(ctx, cfg.TaskID, errMsg, envResult.ExitCode)
		return Result{ExitCode: envResult.ExitCode, Output: stdout, Error: errMsg, Success: false}
	}

	if envResult.Err != nil && ctx.Err() != nil {
		taskLogger.Info("task interrupted by daemon shutdown")
		return Result{ExitCode: envResult.ExitCode, Output: stdout, Error: "interrupted", Success: false}
	}

	if envResult.ExitCode == 0 && envResult.Err == nil {
		taskLogger.Info("task completed successfully")
		if e.reporter != nil {
			req := api.TaskCompleteRequest{
				Output:   stdout,
				ExitCode: int32(envResult.ExitCode),
			}
			if err := e.reporter.CompleteTask(ctx, cfg.TaskID, req); err != nil {
				taskLogger.Warn("failed to report task completion", "error", err)
			}
		}
		return Result{ExitCode: envResult.ExitCode, Output: stdout, Success: true}
	}

	// Failure.
	taskLogger.Info("task failed", "exit_code", envResult.ExitCode, "error", envResult.Err)
	errMsg := stderr
	if errMsg == "" && envResult.Err != nil {
		errMsg = envResult.Err.Error()
	}
	e.reportFailure(ctx, cfg.TaskID, errMsg, envResult.ExitCode)
	return Result{ExitCode: envResult.ExitCode, Output: stdout, Error: errMsg, Success: false}
}

// reportFailure reports a task failure to the server.
func (e *Executor) reportFailure(ctx context.Context, taskID string, errMsg string, exitCode int) {
	if e.reporter == nil {
		return
	}
	req := api.TaskFailRequest{
		ErrorMessage: errMsg,
		ExitCode:     int32(exitCode),
	}
	if err := e.reporter.FailTask(ctx, taskID, req); err != nil {
		e.logger.Warn("failed to report task failure", "task_id", taskID, "error", err)
	}
}
