package agent

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// openclawBackend implements Backend for the OpenClaw CLI.
type openclawBackend struct {
	cfg Config
}

func (b *openclawBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	execPath := b.cfg.ExecutablePath
	if execPath == "" {
		execPath = "openclaw"
	}
	if _, err := exec.LookPath(execPath); err != nil {
		return nil, fmt.Errorf("openclaw executable not found at %q: %w", execPath, err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)

	args := buildOpenclawArgs(prompt, opts, b.cfg.Logger)

	cmd := exec.CommandContext(runCtx, execPath, args...)
	cmd.WaitDelay = 10 * time.Second
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	cmd.Env = buildEnv(b.cfg.Env)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("openclaw stdout pipe: %w", err)
	}
	stderrBuf := newStderrTail(newLogWriter(b.cfg.Logger, "[openclaw:stderr] "), stderrTailSize)
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start openclaw: %w", err)
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

			var evt openclawEvent
			if err := json.Unmarshal([]byte(line), &evt); err != nil {
				continue
			}

			switch evt.Type {
			case "tool_use":
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
			case "tool_result":
				trySend(msgCh, Message{
					Type:   MessageToolResult,
					Tool:   evt.Tool,
					CallID: evt.CallID,
					Output: evt.Output,
				})
			case "text":
				if evt.Content != "" {
					output.WriteString(evt.Content)
					trySend(msgCh, Message{Type: MessageText, Content: evt.Content})
				}
			case "error", "failed", "cancelled":
				errMsg := evt.Error
				if errMsg == "" {
					errMsg = evt.Content
				}
				if errMsg != "" {
					trySend(msgCh, Message{Type: MessageError, Content: errMsg})
					if evt.Type == "failed" || evt.Type == "cancelled" {
						finalStatus = evt.Type
						finalError = errMsg
					}
				}
			case "result":
				// Single final-result JSON blob mode.
				if evt.Content != "" {
					output.WriteString(evt.Content)
					trySend(msgCh, Message{Type: MessageText, Content: evt.Content})
				}
				if evt.Status != "" {
					finalStatus = evt.Status
				}
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
			finalError = fmt.Sprintf("openclaw timed out after %s", timeout)
		} else if runCtx.Err() == context.Canceled {
			finalStatus = "aborted"
			finalError = "execution cancelled"
		} else if waitErr != nil && finalStatus == "completed" {
			finalStatus = "failed"
			finalError = fmt.Sprintf("openclaw exited with error: %v", waitErr)
		}

		if finalError != "" {
			finalError = withStderrTail(finalError, "openclaw", stderrBuf.Tail())
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

// ── OpenClaw event types ──

type openclawEvent struct {
	Type    string          `json:"type"`
	Content string          `json:"content,omitempty"`
	Tool    string          `json:"tool,omitempty"`
	CallID  string          `json:"callId,omitempty"`
	Input   json.RawMessage `json:"input,omitempty"`
	Output  string          `json:"output,omitempty"`
	Error   string          `json:"error,omitempty"`
	Status  string          `json:"status,omitempty"`
	Usage   openclawUsage   `json:"usage,omitempty"`
}

type openclawUsage struct {
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
}

// ── OpenClaw arg building ──

var openclawBlockedArgs = map[string]blockedArgMode{
	"--local":      blockedStandalone,
	"--json":       blockedStandalone,
	"--session-id": blockedWithValue,
	"--message":    blockedWithValue,
}

func buildOpenclawArgs(prompt string, opts ExecOptions, logger *slog.Logger) []string {
	sessionID := opts.ResumeSessionID
	if sessionID == "" {
		sessionID = newSessionID()
	}

	args := []string{
		"agent",
		"--local",
		"--json",
		"--session-id", sessionID,
		"--message", prompt,
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	args = append(args, filterCustomArgs(opts.CustomArgs, openclawBlockedArgs, logger)...)
	return args
}

// newSessionID generates a random UUID v4 string for session identification.
func newSessionID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
