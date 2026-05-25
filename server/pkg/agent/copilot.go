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

// copilotBackend implements Backend for the GitHub Copilot CLI.
type copilotBackend struct {
	cfg Config
}

func (b *copilotBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	execPath := b.cfg.ExecutablePath
	if execPath == "" {
		execPath = "copilot"
	}
	if _, err := exec.LookPath(execPath); err != nil {
		return nil, fmt.Errorf("copilot executable not found at %q: %w", execPath, err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)

	args := buildCopilotArgs(prompt, opts, b.cfg.Logger)

	cmd := exec.CommandContext(runCtx, execPath, args...)
	cmd.WaitDelay = 10 * time.Second
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	cmd.Env = buildEnv(b.cfg.Env)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("copilot stdout pipe: %w", err)
	}
	stderrBuf := newStderrTail(newLogWriter(b.cfg.Logger, "[copilot:stderr] "), stderrTailSize)
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start copilot: %w", err)
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

			var evt copilotEvent
			if err := json.Unmarshal([]byte(line), &evt); err != nil {
				continue
			}

			switch evt.Type {
			case "assistant.message":
				// If toolRequests are present, emit tool_use for each.
				if len(evt.ToolRequests) > 0 {
					for _, tr := range evt.ToolRequests {
						var input map[string]any
						if tr.Input != nil {
							_ = json.Unmarshal(tr.Input, &input)
						}
						trySend(msgCh, Message{
							Type:   MessageToolUse,
							Tool:   tr.Name,
							CallID: tr.ID,
							Input:  input,
						})
					}
				} else if evt.Content != "" {
					output.WriteString(evt.Content)
					trySend(msgCh, Message{Type: MessageText, Content: evt.Content})
				}
			case "tool.execution_complete":
				trySend(msgCh, Message{
					Type:   MessageToolResult,
					Tool:   evt.ToolName,
					CallID: evt.CallID,
					Output: evt.Output,
				})
			case "assistant.message_delta":
				if evt.Content != "" {
					output.WriteString(evt.Content)
					trySend(msgCh, Message{Type: MessageText, Content: evt.Content})
				}
			case "assistant.reasoning":
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
			finalError = fmt.Sprintf("copilot timed out after %s", timeout)
		} else if runCtx.Err() == context.Canceled {
			finalStatus = "aborted"
			finalError = "execution cancelled"
		} else if waitErr != nil && finalStatus == "completed" {
			finalStatus = "failed"
			finalError = fmt.Sprintf("copilot exited with error: %v", waitErr)
		}

		if finalError != "" {
			finalError = withStderrTail(finalError, "copilot", stderrBuf.Tail())
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

// ── Copilot event types ──

type copilotEvent struct {
	Type         string               `json:"type"`
	Content      string               `json:"content,omitempty"`
	ToolRequests []copilotToolRequest `json:"toolRequests,omitempty"`
	ToolName     string               `json:"toolName,omitempty"`
	CallID       string               `json:"callId,omitempty"`
	Output       string               `json:"output,omitempty"`
	Error        string               `json:"error,omitempty"`
	Usage        copilotUsage         `json:"usage,omitempty"`
}

type copilotToolRequest struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input,omitempty"`
}

type copilotUsage struct {
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
}

// ── Copilot arg building ──

var copilotBlockedArgs = map[string]blockedArgMode{
	"-p":              blockedStandalone,
	"--output-format": blockedWithValue,
	"--allow-all":     blockedStandalone,
	"--no-ask-user":   blockedStandalone,
}

func buildCopilotArgs(prompt string, opts ExecOptions, logger *slog.Logger) []string {
	args := []string{
		"-p", prompt,
		"--output-format", "json",
		"--allow-all",
		"--no-ask-user",
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	args = append(args, filterCustomArgs(opts.CustomArgs, copilotBlockedArgs, logger)...)
	return args
}
