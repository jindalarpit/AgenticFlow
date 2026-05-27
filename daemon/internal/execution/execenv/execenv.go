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

// WorkspaceMode defines how the execution workspace is determined.
type WorkspaceMode string

const (
	WorkspaceModeIsolated WorkspaceMode = "isolated"
	WorkspaceModeExisting WorkspaceMode = "existing"
)

// Task represents the minimal task information needed to set up an execution environment.
type Task struct {
	ID            string
	AgentType     string
	Prompt        string
	Model         string
	ArgsTemplate  string
	EnvVars       map[string]string
	CustomArgs    []string
	WorkspaceMode WorkspaceMode
	WorkspacePath string
}

// Config holds the daemon configuration fields relevant to execution environments.
type Config struct {
	WorkspacesRoot string
	AgentTimeout   time.Duration
}

// ExecEnv represents an isolated execution environment for a single task.
type ExecEnv struct {
	TaskID        string
	WorkspaceDir  string
	Provider      string
	Prompt        string
	Model         string
	EnvVars       map[string]string
	ArgsTemplate  string
	CustomArgs    []string
	SystemPrompt  string
	BinaryPath    string
	WorkspaceMode WorkspaceMode
	Logger        *slog.Logger
}

// NewExecEnv creates an ExecEnv from a task and daemon config.
func NewExecEnv(task Task, cfg Config, binaryPath string, logger *slog.Logger) (*ExecEnv, error) {
	if task.ID == "" {
		return nil, fmt.Errorf("execenv: task ID is required")
	}
	if binaryPath == "" {
		return nil, fmt.Errorf("execenv: binary path is required")
	}
	mode := task.WorkspaceMode
	if mode == "" {
		mode = WorkspaceModeIsolated
	}
	var workspaceDir string
	switch mode {
	case WorkspaceModeExisting:
		if task.WorkspacePath == "" {
			return nil, fmt.Errorf("execenv: workspace path is required for existing mode")
		}
		if err := validateExistingWorkspace(task.WorkspacePath); err != nil {
			return nil, err
		}
		workspaceDir = task.WorkspacePath
	case WorkspaceModeIsolated:
		if cfg.WorkspacesRoot == "" {
			return nil, fmt.Errorf("execenv: workspaces root is required")
		}
		workspaceDir = filepath.Join(cfg.WorkspacesRoot, task.ID)
	default:
		return nil, fmt.Errorf("execenv: invalid workspace mode: %q", mode)
	}
	envVars := task.EnvVars
	if envVars == nil {
		envVars = make(map[string]string)
	}
	return &ExecEnv{
		TaskID:        task.ID,
		WorkspaceDir:  workspaceDir,
		Provider:      task.AgentType,
		Prompt:        task.Prompt,
		Model:         task.Model,
		EnvVars:       envVars,
		ArgsTemplate:  task.ArgsTemplate,
		CustomArgs:    task.CustomArgs,
		BinaryPath:    binaryPath,
		WorkspaceMode: mode,
		Logger:        logger,
	}, nil
}

func validateExistingWorkspace(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("workspace path does not exist: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("workspace path is not a directory: %s", path)
	}
	testFile := filepath.Join(path, ".agenticflow-write-test")
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("workspace path is not writable: %s", path)
	}
	f.Close()
	os.Remove(testFile)
	return nil
}

// Setup creates the workspace directory for the task.
func (e *ExecEnv) Setup() error {
	if e.WorkspaceMode == WorkspaceModeExisting {
		e.Logger.Info("execenv: using existing workspace", "path", e.WorkspaceDir)
		return nil
	}
	if _, err := os.Stat(e.WorkspaceDir); err == nil {
		e.Logger.Info("execenv: removing existing workspace", "path", e.WorkspaceDir)
		if err := os.RemoveAll(e.WorkspaceDir); err != nil {
			return fmt.Errorf("execenv: remove existing workspace %s: %w", e.WorkspaceDir, err)
		}
	}
	if err := os.MkdirAll(e.WorkspaceDir, 0o755); err != nil {
		return fmt.Errorf("execenv: create workspace %s: %w", e.WorkspaceDir, err)
	}
	e.Logger.Info("execenv: workspace created", "path", e.WorkspaceDir)
	return nil
}

// Cleanup removes the workspace directory for the task.
func (e *ExecEnv) Cleanup() error {
	if e.WorkspaceMode == WorkspaceModeExisting {
		e.Logger.Info("execenv: skipping cleanup for existing workspace", "path", e.WorkspaceDir)
		return nil
	}
	e.Logger.Info("execenv: cleaning up workspace", "path", e.WorkspaceDir)
	if err := os.RemoveAll(e.WorkspaceDir); err != nil {
		return fmt.Errorf("execenv: cleanup workspace %s: %w", e.WorkspaceDir, err)
	}
	return nil
}

// RunResult holds the outcome of a process started by RunWithStdin.
type RunResult struct {
	ExitCode int
	Err      error
}

// Run spawns the agent CLI process and waits for it to complete.
// It returns the exit code and any error. This is a synchronous wrapper
// around RunWithStdin that immediately closes stdin.
func (e *ExecEnv) Run(ctx context.Context, stdout, stderr io.Writer) (int, error) {
	stdinPipe, done, err := e.RunWithStdin(ctx, stdout, stderr)
	if err != nil {
		return -1, err
	}
	// Close stdin immediately since we don't need to write to it.
	stdinPipe.Close()
	// Wait for the process to complete.
	result := <-done
	return result.ExitCode, result.Err
}

// RunWithStdin spawns the agent CLI process with stdin pipe support.
func (e *ExecEnv) RunWithStdin(ctx context.Context, stdout, stderr io.Writer) (io.WriteCloser, <-chan RunResult, error) {
	args := buildProviderArgs(e.Provider, e.Prompt, e.WorkspaceDir, e.Model, e.ArgsTemplate, e.SystemPrompt, e.CustomArgs)
	cmd := exec.CommandContext(ctx, e.BinaryPath, args...)
	cmd.Dir = e.WorkspaceDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	env := os.Environ()
	for k, v := range e.EnvVars {
		env = append(env, k+"="+v)
	}
	for k, v := range providerEnvVars(e.Provider) {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	cmd.Cancel = nil
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("execenv: create stdin pipe: %w", err)
	}
	e.Logger.Info("execenv: starting process with stdin pipe", "binary", e.BinaryPath, "args", args, "workdir", e.WorkspaceDir)
	if err := cmd.Start(); err != nil {
		stdinPipe.Close()
		return nil, nil, fmt.Errorf("execenv: start process: %w", err)
	}
	done := make(chan RunResult, 1)
	go func() {
		waitDone := make(chan error, 1)
		go func() { waitDone <- cmd.Wait() }()
		select {
		case waitErr := <-waitDone:
			done <- RunResult{ExitCode: exitCode(cmd, waitErr), Err: waitErr}
		case <-ctx.Done():
			e.Logger.Info("execenv: context cancelled, sending SIGTERM", "task_id", e.TaskID)
			if cmd.Process != nil {
				_ = cmd.Process.Signal(syscall.SIGTERM)
			}
			killTimer := time.NewTimer(10 * time.Second)
			defer killTimer.Stop()
			select {
			case waitErr := <-waitDone:
				done <- RunResult{ExitCode: exitCode(cmd, waitErr), Err: ctx.Err()}
			case <-killTimer.C:
				e.Logger.Warn("execenv: process did not exit after SIGTERM, sending SIGKILL", "task_id", e.TaskID)
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				waitErr := <-waitDone
				done <- RunResult{ExitCode: exitCode(cmd, waitErr), Err: ctx.Err()}
			}
		}
	}()
	return stdinPipe, done, nil
}

func providerEnvVars(provider string) map[string]string {
	switch provider {
	case "gemini":
		return map[string]string{"GEMINI_CLI_TRUST_WORKSPACE": "true"}
	case "opencode":
		return map[string]string{"OPENCODE_AUTO_APPROVE": "true"}
	default:
		return nil
	}
}

func buildProviderArgs(provider, prompt, workspace, model, argsTemplate, systemPrompt string, customArgs []string) []string {
	var args []string
	switch provider {
	case "claude":
		args = []string{"-p", prompt, "--output-format", "text", "--verbose", "--permission-mode", "bypassPermissions"}
		if model != "" {
			args = append(args, "--model", model)
		}
		if systemPrompt != "" {
			args = append(args, "--append-system-prompt", systemPrompt)
		}
	case "pi":
		args = []string{"-p", prompt, "--output-format", "text"}
		if model != "" {
			args = append(args, "--model", model)
		}
		if systemPrompt != "" {
			args = append(args, "--append-system-prompt", systemPrompt)
		}
	case "gemini":
		args = []string{"-p", prompt, "--yolo"}
		if model != "" {
			args = append(args, "-m", model)
		}
	case "opencode":
		args = []string{"run", "--dangerously-skip-permissions"}
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
		args = []string{"--quiet", "--full-auto"}
		if model != "" {
			args = append(args, "--model", model)
		}
		args = append(args, prompt)
	case "copilot":
		args = []string{"-p", prompt}
		if model != "" {
			args = append(args, "--model", model)
		}
	case "kiro":
		args = []string{"-p", prompt, "--output-format", "text"}
		if model != "" {
			args = append(args, "--model", model)
		}
	default:
		args = resolveArgs(argsTemplate, prompt, workspace, model)
	}
	if len(customArgs) > 0 {
		args = append(args, customArgs...)
	}
	return args
}

// ResolveArgs resolves template variables in an args template string.
func ResolveArgs(template, prompt, workspace, model string) []string {
	return resolveArgs(template, prompt, workspace, model)
}

func resolveArgs(template, prompt, workspace, model string) []string {
	if template == "" {
		if prompt != "" {
			return []string{prompt}
		}
		return nil
	}
	resolved := template
	resolved = strings.ReplaceAll(resolved, "{{prompt}}", prompt)
	resolved = strings.ReplaceAll(resolved, "{{workspace}}", workspace)
	resolved = strings.ReplaceAll(resolved, "{{model}}", model)
	return strings.Fields(resolved)
}

func exitCode(cmd *exec.Cmd, err error) int {
	if cmd.ProcessState != nil {
		return cmd.ProcessState.ExitCode()
	}
	if err != nil {
		return -1
	}
	return 0
}
