// Package execenv provides task execution environment with workspace isolation.
// Each task gets its own directory under the configured workspaces root,
// ensuring concurrent tasks cannot interfere with each other's filesystem state.
package execenv

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Task represents the minimal task information needed to set up an execution
// environment. This mirrors the design's Task type from internal/daemon/types.go.
type Task struct {
	ID           string
	AgentType    string
	Prompt       string
	Model        string
	ArgsTemplate string
	EnvVars      map[string]string
	CustomArgs   []string // Agent-level custom arguments appended after provider args.
}

// Config holds the daemon configuration fields relevant to execution environments.
type Config struct {
	WorkspacesRoot string
	AgentTimeout   time.Duration
}

// ExecEnv represents an isolated execution environment for a single task.
// It manages workspace creation, agent process spawning, and cleanup.
type ExecEnv struct {
	TaskID       string
	WorkspaceDir string
	Provider     string
	Prompt       string
	Model        string
	EnvVars      map[string]string
	ArgsTemplate string
	CustomArgs   []string // Agent-level custom arguments appended after provider args.
	SystemPrompt string   // Runtime_Brief for providers that support system prompt flags.
	BinaryPath   string
	Logger       *slog.Logger
}

// NewExecEnv creates an ExecEnv from a task and daemon config.
// It resolves the workspace directory to cfg.WorkspacesRoot/<task-id>/.
func NewExecEnv(task Task, cfg Config, binaryPath string, logger *slog.Logger) (*ExecEnv, error) {
	if cfg.WorkspacesRoot == "" {
		return nil, fmt.Errorf("execenv: workspaces root is required")
	}
	if task.ID == "" {
		return nil, fmt.Errorf("execenv: task ID is required")
	}
	if binaryPath == "" {
		return nil, fmt.Errorf("execenv: binary path is required")
	}

	workspaceDir := filepath.Join(cfg.WorkspacesRoot, task.ID)

	envVars := task.EnvVars
	if envVars == nil {
		envVars = make(map[string]string)
	}

	return &ExecEnv{
		TaskID:       task.ID,
		WorkspaceDir: workspaceDir,
		Provider:     task.AgentType,
		Prompt:       task.Prompt,
		Model:        task.Model,
		EnvVars:      envVars,
		ArgsTemplate: task.ArgsTemplate,
		CustomArgs:   task.CustomArgs,
		BinaryPath:   binaryPath,
		Logger:       logger,
	}, nil
}

// Setup creates the workspace directory for the task.
// If the directory already exists, it is removed and recreated to ensure
// a clean state (Requirement 10.6).
func (e *ExecEnv) Setup() error {
	// If workspace already exists, remove it first.
	if _, err := os.Stat(e.WorkspaceDir); err == nil {
		e.Logger.Info("execenv: removing existing workspace", "path", e.WorkspaceDir)
		if err := os.RemoveAll(e.WorkspaceDir); err != nil {
			return fmt.Errorf("execenv: remove existing workspace %s: %w", e.WorkspaceDir, err)
		}
	}

	// Create the workspace directory.
	if err := os.MkdirAll(e.WorkspaceDir, 0o755); err != nil {
		return fmt.Errorf("execenv: create workspace %s: %w", e.WorkspaceDir, err)
	}

	e.Logger.Info("execenv: workspace created", "path", e.WorkspaceDir)
	return nil
}

// Run spawns the agent CLI process in the isolated workspace.
// It sets the working directory, environment variables, and resolves the
// args template with variable substitution. stdout and stderr are piped
// to the provided writers.
//
// Returns the process exit code and any error. On context cancellation,
// the process receives SIGTERM followed by SIGKILL after 10 seconds.
func (e *ExecEnv) Run(ctx context.Context, stdout, stderr io.Writer) (int, error) {
	args := buildProviderArgs(e.Provider, e.Prompt, e.WorkspaceDir, e.Model, e.ArgsTemplate, e.SystemPrompt, e.CustomArgs)

	cmd := exec.CommandContext(ctx, e.BinaryPath, args...)
	cmd.Dir = e.WorkspaceDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Build environment: inherit current env + overlay task-specific vars.
	env := os.Environ()
	for k, v := range e.EnvVars {
		env = append(env, k+"="+v)
	}
	// Inject provider-specific environment variables.
	for k, v := range providerEnvVars(e.Provider) {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	// Disable automatic process killing on context cancellation so we can
	// handle graceful shutdown ourselves (SIGTERM → wait → SIGKILL).
	cmd.Cancel = nil

	e.Logger.Info("execenv: starting process",
		"binary", e.BinaryPath,
		"args", args,
		"workdir", e.WorkspaceDir,
	)

	if err := cmd.Start(); err != nil {
		return -1, fmt.Errorf("execenv: start process: %w", err)
	}

	// Wait for the process to complete or context to be cancelled.
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	select {
	case err := <-waitDone:
		// Process exited normally (or with error).
		return exitCode(cmd, err), err

	case <-ctx.Done():
		// Context cancelled — initiate graceful shutdown.
		e.Logger.Info("execenv: context cancelled, sending SIGTERM", "task_id", e.TaskID)

		// Send SIGTERM for graceful shutdown.
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}

		// Wait up to 10 seconds for the process to exit.
		killTimer := time.NewTimer(10 * time.Second)
		defer killTimer.Stop()

		select {
		case err := <-waitDone:
			// Process exited after SIGTERM.
			return exitCode(cmd, err), ctx.Err()

		case <-killTimer.C:
			// Process didn't exit in time — force kill.
			e.Logger.Warn("execenv: process did not exit after SIGTERM, sending SIGKILL", "task_id", e.TaskID)
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			// Wait for the kill to take effect.
			err := <-waitDone
			return exitCode(cmd, err), ctx.Err()
		}
	}
}

// RunResult holds the outcome of a process started by RunWithStdin.
// The caller receives this via the channel returned by RunWithStdin.
type RunResult struct {
	ExitCode int
	Err      error
}

// RunWithStdin spawns the agent CLI process with stdin pipe support.
// Unlike Run, this method starts the process and returns immediately with:
//   - stdinPipe: a writable pipe connected to the process's stdin
//   - done: a channel that receives the RunResult when the process exits
//   - err: non-nil if the process could not be started
//
// The caller is responsible for closing the stdin pipe when the task completes.
// On context cancellation, the process receives SIGTERM followed by SIGKILL
// after 10 seconds (same graceful shutdown as Run).
func (e *ExecEnv) RunWithStdin(ctx context.Context, stdout, stderr io.Writer) (io.WriteCloser, <-chan RunResult, error) {
	args := buildProviderArgs(e.Provider, e.Prompt, e.WorkspaceDir, e.Model, e.ArgsTemplate, e.SystemPrompt, e.CustomArgs)

	cmd := exec.CommandContext(ctx, e.BinaryPath, args...)
	cmd.Dir = e.WorkspaceDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Build environment: inherit current env + overlay task-specific vars.
	env := os.Environ()
	for k, v := range e.EnvVars {
		env = append(env, k+"="+v)
	}
	// Inject provider-specific environment variables.
	for k, v := range providerEnvVars(e.Provider) {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	// Disable automatic process killing on context cancellation so we can
	// handle graceful shutdown ourselves (SIGTERM → wait → SIGKILL).
	cmd.Cancel = nil

	// Create stdin pipe for bidirectional communication.
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("execenv: create stdin pipe: %w", err)
	}

	e.Logger.Info("execenv: starting process with stdin pipe",
		"binary", e.BinaryPath,
		"args", args,
		"workdir", e.WorkspaceDir,
	)

	if err := cmd.Start(); err != nil {
		stdinPipe.Close()
		return nil, nil, fmt.Errorf("execenv: start process: %w", err)
	}

	// Launch a goroutine to wait for process completion and handle graceful shutdown.
	done := make(chan RunResult, 1)
	go func() {
		waitDone := make(chan error, 1)
		go func() {
			waitDone <- cmd.Wait()
		}()

		select {
		case err := <-waitDone:
			// Process exited normally (or with error).
			done <- RunResult{ExitCode: exitCode(cmd, err), Err: err}

		case <-ctx.Done():
			// Context cancelled — initiate graceful shutdown.
			e.Logger.Info("execenv: context cancelled, sending SIGTERM", "task_id", e.TaskID)

			// Send SIGTERM for graceful shutdown.
			if cmd.Process != nil {
				_ = cmd.Process.Signal(syscall.SIGTERM)
			}

			// Wait up to 10 seconds for the process to exit.
			killTimer := time.NewTimer(10 * time.Second)
			defer killTimer.Stop()

			select {
			case err := <-waitDone:
				// Process exited after SIGTERM.
				done <- RunResult{ExitCode: exitCode(cmd, err), Err: ctx.Err()}

			case <-killTimer.C:
				// Process didn't exit in time — force kill.
				e.Logger.Warn("execenv: process did not exit after SIGTERM, sending SIGKILL", "task_id", e.TaskID)
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				// Wait for the kill to take effect.
				err := <-waitDone
				done <- RunResult{ExitCode: exitCode(cmd, err), Err: ctx.Err()}
			}
		}
	}()

	return stdinPipe, done, nil
}

// Cleanup removes the workspace directory and all its contents.
func (e *ExecEnv) Cleanup() error {
	if err := os.RemoveAll(e.WorkspaceDir); err != nil {
		e.Logger.Warn("execenv: cleanup failed", "path", e.WorkspaceDir, "error", err)
		return fmt.Errorf("execenv: cleanup workspace %s: %w", e.WorkspaceDir, err)
	}
	e.Logger.Debug("execenv: workspace cleaned up", "path", e.WorkspaceDir)
	return nil
}

// providerEnvVars returns provider-specific environment variables needed for
// headless/automated execution.
func providerEnvVars(provider string) map[string]string {
	switch provider {
	case "gemini":
		// Gemini CLI requires workspace trust in headless mode.
		// See: https://geminicli.com/docs/cli/trusted-folders/#headless-and-automated-environments
		return map[string]string{
			"GEMINI_CLI_TRUST_WORKSPACE": "true",
		}
	case "opencode":
		// OpenCode: skip interactive permission prompts in daemon mode.
		return map[string]string{
			"OPENCODE_AUTO_APPROVE": "true",
		}
	default:
		return nil
	}
}

// buildProviderArgs builds the correct CLI arguments for each known agent provider.
// Falls back to the generic resolveArgs for custom/unknown agents.
// When systemPrompt is non-empty, it is injected via the provider-appropriate mechanism.
// customArgs are appended after the provider-specific arguments.
func buildProviderArgs(provider, prompt, workspace, model, argsTemplate, systemPrompt string, customArgs []string) []string {
	var args []string

	switch provider {
	case "claude":
		// Claude Code CLI: non-interactive print mode
		args = []string{
			"-p", prompt,
			"--output-format", "text",
			"--verbose",
			"--permission-mode", "bypassPermissions",
		}
		if model != "" {
			args = append(args, "--model", model)
		}
		if systemPrompt != "" {
			args = append(args, "--append-system-prompt", systemPrompt)
		}

	case "pi":
		// Pi CLI: similar to claude, uses --append-system-prompt
		args = []string{
			"-p", prompt,
			"--output-format", "text",
		}
		if model != "" {
			args = append(args, "--model", model)
		}
		if systemPrompt != "" {
			args = append(args, "--append-system-prompt", systemPrompt)
		}

	case "gemini":
		// Gemini CLI: non-interactive with auto-approve
		args = []string{
			"-p", prompt,
			"--yolo",
		}
		if model != "" {
			args = append(args, "-m", model)
		}

	case "opencode":
		// OpenCode CLI: run mode with JSON output
		args = []string{
			"run",
			"--dangerously-skip-permissions",
		}
		if workspace != "" {
			args = append(args, "--dir", workspace)
		}
		if model != "" {
			args = append(args, "--model", model)
		}
		if systemPrompt != "" {
			args = append(args, "--prompt", systemPrompt)
		}
		args = append(args, prompt)

	case "codex":
		// Codex CLI: quiet mode
		args = []string{
			"--quiet",
			"--full-auto",
		}
		if model != "" {
			args = append(args, "--model", model)
		}
		args = append(args, prompt)

	case "copilot":
		// GitHub Copilot CLI
		args = []string{
			"-p", prompt,
		}
		if model != "" {
			args = append(args, "--model", model)
		}

	case "kiro":
		// Kiro CLI
		args = []string{
			"-p", prompt,
			"--output-format", "text",
		}
		if model != "" {
			args = append(args, "--model", model)
		}

	default:
		// Custom agents or unknown providers: use template-based resolution
		args = resolveArgs(argsTemplate, prompt, workspace, model)
	}

	// Append agent-level custom arguments after provider-specific args.
	if len(customArgs) > 0 {
		args = append(args, customArgs...)
	}

	return args
}

// resolveArgs replaces template variables in the args template string and
// splits the result into individual arguments.
//
// Supported variables:
//   - {{prompt}}    → the task prompt
//   - {{workspace}} → the workspace directory path
//   - {{model}}     → the model override (empty string if not set)
//
// The template is split on whitespace after substitution to produce the
// argument slice.
func resolveArgs(template, prompt, workspace, model string) []string {
	if template == "" {
		// Default: just pass the prompt as the single argument.
		if prompt != "" {
			return []string{prompt}
		}
		return nil
	}

	resolved := template
	resolved = strings.ReplaceAll(resolved, "{{prompt}}", prompt)
	resolved = strings.ReplaceAll(resolved, "{{workspace}}", workspace)
	resolved = strings.ReplaceAll(resolved, "{{model}}", model)

	// Split on whitespace to produce argument list.
	// Filter out empty strings that result from consecutive spaces or
	// empty variable substitutions.
	parts := strings.Fields(resolved)
	return parts
}

// exitCode extracts the exit code from a completed command.
// Returns -1 if the exit code cannot be determined.
func exitCode(cmd *exec.Cmd, err error) int {
	if cmd.ProcessState != nil {
		return cmd.ProcessState.ExitCode()
	}
	if err != nil {
		return -1
	}
	return 0
}
