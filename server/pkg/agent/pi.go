package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// piBackend implements Backend for the Pi CLI.
type piBackend struct {
	cfg Config
}

func (b *piBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	execPath := b.cfg.ExecutablePath
	if execPath == "" {
		execPath = "pi"
	}
	if _, err := exec.LookPath(execPath); err != nil {
		return nil, fmt.Errorf("pi executable not found at %q: %w", execPath, err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)

	args := buildPiArgs(prompt, opts, b.cfg.Logger)

	cmd := exec.CommandContext(runCtx, execPath, args...)
	cmd.WaitDelay = 10 * time.Second
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	cmd.Env = buildEnv(b.cfg.Env)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("pi stdout pipe: %w", err)
	}
	stderrBuf := newStderrTail(newLogWriter(b.cfg.Logger, "[pi:stderr] "), stderrTailSize)
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start pi: %w", err)
	}

	msgCh := make(chan Message, 256)
	resCh := make(chan Result, 1)

	closeOnCancel(runCtx, stdout)

	go func() {
		defer cancel()
		defer close(msgCh)
		defer close(resCh)

		startTime := time.Now()
		var output strings.Builder
		finalStatus := "completed"
		var finalError string
		usage := make(map[string]TokenUsage)

		scanner := newLineScanner(stdout)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			var evt piEvent
			if err := json.Unmarshal([]byte(line), &evt); err != nil {
				continue
			}

			switch evt.Type {
			case "tool_execution_start":
				var input map[string]any
				if evt.Input != nil {
					_ = json.Unmarshal(evt.Input, &input)
				}
				trySend(msgCh, Message{
					Type:   MessageToolUse,
					Tool:   evt.Tool,
					CallID: evt.CallID,
					Input:  input,
				})
			case "tool_execution_end":
				trySend(msgCh, Message{
					Type:   MessageToolResult,
					Tool:   evt.Tool,
					CallID: evt.CallID,
					Output: evt.Output,
				})
			case "message_update":
				switch evt.DeltaType {
				case "text_delta":
					if evt.Content != "" {
						output.WriteString(evt.Content)
						trySend(msgCh, Message{Type: MessageText, Content: evt.Content})
					}
				case "thinking_delta":
					if evt.Content != "" {
						trySend(msgCh, Message{Type: MessageThinking, Content: evt.Content})
					}
				}
			case "error":
				trySend(msgCh, Message{Type: MessageError, Content: evt.Error})
			case "usage":
				if evt.Usage.InputTokens > 0 || evt.Usage.OutputTokens > 0 {
					model := "default"
					u := usage[model]
					u.InputTokens += evt.Usage.InputTokens
					u.OutputTokens += evt.Usage.OutputTokens
					usage[model] = u
				}
			}
		}

		waitErr := cmd.Wait()
		duration := time.Since(startTime)

		if runCtx.Err() == context.DeadlineExceeded {
			finalStatus = "timeout"
			finalError = fmt.Sprintf("pi timed out after %s", timeout)
		} else if runCtx.Err() == context.Canceled {
			finalStatus = "aborted"
			finalError = "execution cancelled"
		} else if waitErr != nil && finalStatus == "completed" {
			finalStatus = "failed"
			finalError = fmt.Sprintf("pi exited with error: %v", waitErr)
		}

		if finalError != "" {
			finalError = withStderrTail(finalError, "pi", stderrBuf.Tail())
		}

		resCh <- Result{
			Status:     finalStatus,
			Output:     output.String(),
			Error:      finalError,
			DurationMs: duration.Milliseconds(),
			Usage:      usage,
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}

// ── Pi event types ──

type piEvent struct {
	Type      string          `json:"type"`
	DeltaType string          `json:"deltaType,omitempty"`
	Content   string          `json:"content,omitempty"`
	Tool      string          `json:"tool,omitempty"`
	CallID    string          `json:"callId,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Output    string          `json:"output,omitempty"`
	Error     string          `json:"error,omitempty"`
	Usage     piUsage         `json:"usage,omitempty"`
}

type piUsage struct {
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
}

// ── Pi arg building ──

var piBlockedArgs = map[string]blockedArgMode{
	"-p":        blockedStandalone,
	"--mode":    blockedWithValue,
	"--session": blockedWithValue,
}

func buildPiArgs(prompt string, opts ExecOptions, logger *slog.Logger) []string {
	args := []string{
		"-p",
		"--mode", "json",
	}
	// Derive session path from working directory.
	sessionPath := ".pi-session"
	if opts.Cwd != "" {
		sessionPath = filepath.Join(opts.Cwd, ".pi-session")
	}
	if opts.ResumeSessionID != "" {
		sessionPath = opts.ResumeSessionID
	}
	args = append(args, "--session", sessionPath)
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	args = append(args, filterCustomArgs(opts.CustomArgs, piBlockedArgs, logger)...)
	// Prompt as positional argument.
	args = append(args, prompt)
	return args
}
