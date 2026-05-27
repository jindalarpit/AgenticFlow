package agent

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// cliBackend implements Backend by spawning the agent CLI process and
// streaming its stdout output as text messages. This is a generic backend
// that works for all supported agent types by building provider-specific
// arguments based on the agent type.
type cliBackend struct {
	cfg       Config
	agentType string
}

func (b *cliBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	execPath := b.cfg.ExecutablePath
	if execPath == "" {
		def := LookupByType(b.agentType)
		if def != nil {
			execPath = def.CLIName
		} else {
			execPath = b.agentType
		}
	}

	if _, err := exec.LookPath(execPath); err != nil {
		return nil, fmt.Errorf("%s executable not found at %q: %w", b.agentType, execPath, err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)

	args := buildCLIArgs(b.agentType, prompt, opts, b.cfg.Logger)

	cmd := exec.CommandContext(runCtx, execPath, args...)
	cmd.WaitDelay = 10 * time.Second
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	cmd.Env = buildCLIEnv(b.agentType, b.cfg.Env, opts.CustomEnv, opts.Cwd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("%s stdout pipe: %w", b.agentType, err)
	}
	stderrBuf := &strings.Builder{}
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start %s: %w", b.agentType, err)
	}

	b.cfg.Logger.Info("agent started", "type", b.agentType, "pid", cmd.Process.Pid, "cwd", opts.Cwd, "model", opts.Model)

	msgCh := make(chan Message, 256)
	resCh := make(chan Result, 1)

	// Close stdout when context is cancelled so scanner unblocks.
	go func() {
		<-runCtx.Done()
		_ = stdout.Close()
	}()

	go func() {
		defer cancel()
		defer close(msgCh)
		defer close(resCh)

		startTime := time.Now()
		var output strings.Builder
		finalStatus := "completed"
		var finalError string

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

		for scanner.Scan() {
			line := scanner.Text()
			output.WriteString(line)
			output.WriteByte('\n')
			trySendMsg(msgCh, Message{Type: MessageText, Content: line + "\n"})
		}

		waitErr := cmd.Wait()
		duration := time.Since(startTime)

		if runCtx.Err() == context.DeadlineExceeded {
			finalStatus = "timeout"
			finalError = fmt.Sprintf("%s timed out after %s", b.agentType, timeout)
		} else if runCtx.Err() == context.Canceled {
			finalStatus = "aborted"
			finalError = "execution cancelled"
		} else if waitErr != nil {
			finalStatus = "failed"
			stderr := stderrBuf.String()
			if stderr != "" {
				finalError = stderr
			} else {
				finalError = fmt.Sprintf("%s exited with error: %v", b.agentType, waitErr)
			}
		}

		b.cfg.Logger.Info("agent finished", "type", b.agentType, "pid", cmd.Process.Pid, "status", finalStatus, "duration", duration.Round(time.Millisecond).String())

		resCh <- Result{
			Status:     finalStatus,
			Output:     output.String(),
			Error:      finalError,
			DurationMs: duration.Milliseconds(),
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}

// trySendMsg sends a message to the channel without blocking if the channel is full.
func trySendMsg(ch chan<- Message, msg Message) {
	select {
	case ch <- msg:
	default:
	}
}

// buildCLIArgs assembles the CLI arguments for the given agent type.
func buildCLIArgs(agentType, prompt string, opts ExecOptions, logger *slog.Logger) []string {
	var args []string

	switch agentType {
	case "claude":
		args = []string{"-p", prompt, "--output-format", "text", "--verbose", "--permission-mode", "bypassPermissions"}
		if opts.Model != "" {
			args = append(args, "--model", opts.Model)
		}
		if opts.SystemPrompt != "" {
			args = append(args, "--append-system-prompt", opts.SystemPrompt)
		}
		if opts.ResumeSessionID != "" {
			args = append(args, "--resume", opts.ResumeSessionID)
		}

	case "pi":
		args = []string{"-p", prompt, "--output-format", "text"}
		if opts.Model != "" {
			args = append(args, "--model", opts.Model)
		}
		if opts.SystemPrompt != "" {
			args = append(args, "--append-system-prompt", opts.SystemPrompt)
		}
		if opts.ResumeSessionID != "" {
			args = append(args, "--resume", opts.ResumeSessionID)
		}

	case "gemini":
		args = []string{"-p", prompt, "--yolo"}
		if opts.Model != "" {
			args = append(args, "-m", opts.Model)
		}
		if opts.ResumeSessionID != "" {
			args = append(args, "-r", opts.ResumeSessionID)
		}

	case "opencode":
		args = []string{"run", "--format", "json", "--dangerously-skip-permissions"}
		if opts.Cwd != "" {
			args = append(args, "--dir", opts.Cwd)
		}
		if opts.Model != "" {
			args = append(args, "--model", opts.Model)
		}
		if opts.SystemPrompt != "" {
			args = append(args, "--prompt", opts.SystemPrompt)
		}
		if opts.ResumeSessionID != "" {
			args = append(args, "--session", opts.ResumeSessionID)
		}
		args = append(args, prompt)

	case "codex":
		args = []string{"--quiet", "--full-auto"}
		if opts.Model != "" {
			args = append(args, "--model", opts.Model)
		}
		args = append(args, prompt)

	case "copilot":
		args = []string{"-p", prompt}
		if opts.Model != "" {
			args = append(args, "--model", opts.Model)
		}

	case "kiro":
		args = []string{"-p", prompt, "--output-format", "text"}
		if opts.Model != "" {
			args = append(args, "--model", opts.Model)
		}

	case "hermes":
		args = []string{"-p", prompt}
		if opts.Model != "" {
			args = append(args, "--model", opts.Model)
		}

	case "kimi":
		args = []string{"-p", prompt}
		if opts.Model != "" {
			args = append(args, "--model", opts.Model)
		}

	case "cursor":
		args = []string{"-p", prompt}
		if opts.Model != "" {
			args = append(args, "--model", opts.Model)
		}

	case "openclaw":
		args = []string{"-p", prompt}
		if opts.Model != "" {
			args = append(args, "--model", opts.Model)
		}

	default:
		// Unknown agent — just pass the prompt as the argument
		if prompt != "" {
			args = []string{prompt}
		}
	}

	// Append custom args
	if len(opts.CustomArgs) > 0 {
		args = append(args, opts.CustomArgs...)
	}

	return args
}

// buildCLIEnv builds the environment for the CLI process.
func buildCLIEnv(agentType string, cfgEnv, optsEnv map[string]string, cwd string) []string {
	env := os.Environ()

	// Add provider-specific env vars
	switch agentType {
	case "gemini":
		env = append(env, "GEMINI_CLI_TRUST_WORKSPACE=true")
	case "opencode":
		// OpenCode reads PWD for project discovery (AGENTS.md, .opencode/skills).
		// Override it to the task workspace so it doesn't inherit the daemon's cwd.
		if cwd != "" {
			env = append(env, "PWD="+cwd)
		}
	}

	// Add config-level env vars
	for k, v := range cfgEnv {
		env = append(env, k+"="+v)
	}

	// Add opts-level env vars (highest precedence)
	for k, v := range optsEnv {
		env = append(env, k+"="+v)
	}

	return env
}