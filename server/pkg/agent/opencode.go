package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// opencodeBackend implements Backend for the OpenCode CLI.
type opencodeBackend struct {
	cfg Config
}

func (b *opencodeBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	execPath := b.cfg.ExecutablePath
	if execPath == "" {
		execPath = "opencode"
	}
	if _, err := exec.LookPath(execPath); err != nil {
		return nil, fmt.Errorf("opencode executable not found at %q: %w", execPath, err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)

	args := buildOpencodeArgs(prompt, opts, b.cfg.Logger)

	cmd := exec.CommandContext(runCtx, execPath, args...)
	cmd.WaitDelay = 10 * time.Second
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	cmd.Env = buildEnv(b.cfg.Env)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("opencode stdout pipe: %w", err)
	}
	stderrBuf := newStderrTail(newLogWriter(b.cfg.Logger, "[opencode:stderr] "), stderrTailSize)
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start opencode: %w", err)
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

			var evt opencodeEvent
			if err := json.Unmarshal([]byte(line), &evt); err != nil {
				continue
			}

			switch evt.Type {
			case "text":
				if evt.Part.Text != "" {
					output.WriteString(evt.Part.Text)
					trySend(msgCh, Message{Type: MessageText, Content: evt.Part.Text})
				}
			case "tool_use":
				var input map[string]any
				if evt.Part.State.Input != nil {
					_ = json.Unmarshal(evt.Part.State.Input, &input)
				}
				trySend(msgCh, Message{
					Type:   MessageToolUse,
					Tool:   evt.Part.Tool,
					CallID: evt.Part.CallID,
					Input:  input,
				})
				// If completed, also emit tool_result.
				if evt.Part.State.Status == "completed" {
					trySend(msgCh, Message{
						Type:   MessageToolResult,
						Tool:   evt.Part.Tool,
						CallID: evt.Part.CallID,
						Output: evt.Part.State.Output,
					})
				}
			case "error":
				trySend(msgCh, Message{Type: MessageError, Content: evt.Error})
			case "step_finish":
				// Accumulate token usage.
				if evt.Usage.InputTokens > 0 || evt.Usage.OutputTokens > 0 {
					model := "default"
					u := usage[model]
					u.InputTokens += evt.Usage.InputTokens
					u.OutputTokens += evt.Usage.OutputTokens
					u.CacheReadTokens += evt.Usage.CacheTokens
					usage[model] = u
				}
			}
		}

		waitErr := cmd.Wait()
		duration := time.Since(startTime)

		if runCtx.Err() == context.DeadlineExceeded {
			finalStatus = "timeout"
			finalError = fmt.Sprintf("opencode timed out after %s", timeout)
		} else if runCtx.Err() == context.Canceled {
			finalStatus = "aborted"
			finalError = "execution cancelled"
		} else if waitErr != nil && finalStatus == "completed" {
			finalStatus = "failed"
			finalError = fmt.Sprintf("opencode exited with error: %v", waitErr)
		}

		if finalError != "" {
			finalError = withStderrTail(finalError, "opencode", stderrBuf.Tail())
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

// ── OpenCode event types ──

type opencodeEvent struct {
	Type  string           `json:"type"`
	Part  opencodePartData `json:"part,omitempty"`
	Error string           `json:"error,omitempty"`
	Usage opencodeUsage    `json:"usage,omitempty"`
}

type opencodePartData struct {
	Text   string            `json:"text,omitempty"`
	Tool   string            `json:"tool,omitempty"`
	CallID string            `json:"callID,omitempty"`
	State  opencodeToolState `json:"state,omitempty"`
}

type opencodeToolState struct {
	Status string          `json:"status,omitempty"`
	Input  json.RawMessage `json:"input,omitempty"`
	Output string          `json:"output,omitempty"`
}

type opencodeUsage struct {
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
	CacheTokens  int64 `json:"cacheTokens"`
}

// ── OpenCode arg building ──

var opencodeBlockedArgs = map[string]blockedArgMode{
	"--format":                       blockedWithValue,
	"--dangerously-skip-permissions": blockedStandalone,
	"--dir":                          blockedWithValue,
}

func buildOpencodeArgs(prompt string, opts ExecOptions, logger *slog.Logger) []string {
	args := []string{
		"run",
		"--format", "json",
		"--dangerously-skip-permissions",
	}
	if opts.Cwd != "" {
		args = append(args, "--dir", opts.Cwd)
	}
	args = append(args, filterCustomArgs(opts.CustomArgs, opencodeBlockedArgs, logger)...)
	args = append(args, prompt)
	return args
}
