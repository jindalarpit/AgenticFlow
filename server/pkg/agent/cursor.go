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

// cursorBackend implements Backend for the Cursor Agent CLI.
type cursorBackend struct {
	cfg Config
}

func (b *cursorBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	execPath := b.cfg.ExecutablePath
	if execPath == "" {
		execPath = "cursor-agent"
	}
	if _, err := exec.LookPath(execPath); err != nil {
		return nil, fmt.Errorf("cursor-agent executable not found at %q: %w", execPath, err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)

	args := buildCursorArgs(prompt, opts, b.cfg.Logger)

	cmd := exec.CommandContext(runCtx, execPath, args...)
	cmd.WaitDelay = 10 * time.Second
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	cmd.Env = buildEnv(b.cfg.Env)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("cursor stdout pipe: %w", err)
	}
	stderrBuf := newStderrTail(newLogWriter(b.cfg.Logger, "[cursor:stderr] "), stderrTailSize)
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start cursor: %w", err)
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
			line := scanner.Text()

			// Strip "stdout:" or "stderr:" prefixes that cursor-agent emits.
			line = stripCursorPrefix(line)
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var evt cursorEvent
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
			case "text", "output_text":
				if evt.Content != "" {
					output.WriteString(evt.Content)
					trySend(msgCh, Message{Type: MessageText, Content: evt.Content})
				}
			case "thinking":
				if evt.Content != "" {
					trySend(msgCh, Message{Type: MessageThinking, Content: evt.Content})
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
			finalError = fmt.Sprintf("cursor timed out after %s", timeout)
		} else if runCtx.Err() == context.Canceled {
			finalStatus = "aborted"
			finalError = "execution cancelled"
		} else if waitErr != nil && finalStatus == "completed" {
			finalStatus = "failed"
			finalError = fmt.Sprintf("cursor exited with error: %v", waitErr)
		}

		if finalError != "" {
			finalError = withStderrTail(finalError, "cursor", stderrBuf.Tail())
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

// stripCursorPrefix removes "stdout:" or "stderr:" prefixes that
// cursor-agent prepends to its stream-json output lines.
func stripCursorPrefix(line string) string {
	if strings.HasPrefix(line, "stdout:") {
		return strings.TrimPrefix(line, "stdout:")
	}
	if strings.HasPrefix(line, "stderr:") {
		return strings.TrimPrefix(line, "stderr:")
	}
	return line
}

// ── Cursor event types ──

type cursorEvent struct {
	Type    string          `json:"type"`
	Content string          `json:"content,omitempty"`
	Tool    string          `json:"tool,omitempty"`
	CallID  string          `json:"callId,omitempty"`
	Input   json.RawMessage `json:"input,omitempty"`
	Output  string          `json:"output,omitempty"`
	Error   string          `json:"error,omitempty"`
	Usage   cursorUsage     `json:"usage,omitempty"`
}

type cursorUsage struct {
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
}

// ── Cursor arg building ──

var cursorBlockedArgs = map[string]blockedArgMode{
	"-p":              blockedStandalone,
	"--output-format": blockedWithValue,
	"--yolo":          blockedStandalone,
	"--workspace":     blockedWithValue,
}

func buildCursorArgs(prompt string, opts ExecOptions, logger *slog.Logger) []string {
	args := []string{
		"chat",
		"-p", prompt,
		"--output-format", "stream-json",
		"--yolo",
	}
	cwd := opts.Cwd
	if cwd == "" {
		cwd = "."
	}
	args = append(args, "--workspace", cwd)
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	args = append(args, filterCustomArgs(opts.CustomArgs, cursorBlockedArgs, logger)...)
	return args
}
