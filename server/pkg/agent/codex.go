package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// codexBackend implements Backend for the Codex CLI via JSON-RPC over stdio.
type codexBackend struct {
	cfg Config
}

func (b *codexBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	execPath := b.cfg.ExecutablePath
	if execPath == "" {
		execPath = "codex"
	}
	if _, err := exec.LookPath(execPath); err != nil {
		return nil, fmt.Errorf("codex executable not found at %q: %w", execPath, err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)

	args := []string{"app-server", "--listen", "stdio://"}

	cmd := exec.CommandContext(runCtx, execPath, args...)
	cmd.WaitDelay = 10 * time.Second
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	cmd.Env = buildEnv(b.cfg.Env)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("codex stdout pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("codex stdin pipe: %w", err)
	}
	stderrBuf := newStderrTail(newLogWriter(b.cfg.Logger, "[codex:stderr] "), stderrTailSize)
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start codex: %w", err)
	}

	msgCh := make(chan Message, 256)
	resCh := make(chan Result, 1)

	client := &acpClient{
		cfg:     b.cfg,
		stdin:   stdin,
		nextID:  1,
		pending: make(map[int]*pendingRPC),
	}
	client.onMessage = func(msg Message) {
		trySend(msgCh, msg)
	}

	go func() {
		defer cancel()
		defer close(msgCh)
		defer close(resCh)

		startTime := time.Now()
		var output strings.Builder
		finalStatus := "completed"
		var finalError string
		var sessionID string

		go func() {
			<-runCtx.Done()
			_ = stdout.Close()
		}()

		scanner := newLineScanner(stdout)

		// Codex handshake: initialize → thread/start → turn/start.
		_, err := client.request(runCtx, "initialize", map[string]any{
			"clientInfo": map[string]any{"name": "agenticflow-daemon", "version": "1.0.0"},
		})
		if err != nil {
			finalStatus = "failed"
			finalError = fmt.Sprintf("codex initialize failed: %v", err)
			_ = cmd.Process.Kill()
			resCh <- Result{Status: finalStatus, Error: finalError, DurationMs: time.Since(startTime).Milliseconds()}
			return
		}

		threadResult, err := client.request(runCtx, "thread/start", map[string]any{})
		if err != nil {
			finalStatus = "failed"
			finalError = fmt.Sprintf("codex thread/start failed: %v", err)
			_ = cmd.Process.Kill()
			resCh <- Result{Status: finalStatus, Error: finalError, DurationMs: time.Since(startTime).Milliseconds()}
			return
		}
		// Extract thread ID as session ID.
		var threadResp struct {
			ThreadID string `json:"threadId"`
		}
		_ = json.Unmarshal(threadResult, &threadResp)
		sessionID = threadResp.ThreadID

		// Start turn with prompt.
		_, err = client.request(runCtx, "turn/start", map[string]any{
			"message": prompt,
		})
		if err != nil && runCtx.Err() == nil {
			finalStatus = "failed"
			finalError = fmt.Sprintf("codex turn/start failed: %v", err)
		}

		// Process notifications from codex.
		// Codex sends: item/started, item/completed, turn/completed
		// We handle these via the ACP client's notification handler.
		client.acceptNotification = func(updateType string) bool { return true }

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			// Handle codex-specific notifications.
			var raw map[string]json.RawMessage
			if err := json.Unmarshal([]byte(line), &raw); err != nil {
				continue
			}

			// Check for agent→client requests (approval requests).
			if _, hasID := raw["id"]; hasID {
				if _, hasMethod := raw["method"]; hasMethod {
					// Auto-approve execution and patch requests.
					b.handleCodexApproval(raw, client)
					continue
				}
			}

			// Dispatch to ACP client for response correlation.
			client.handleLine(line)

			// Also parse codex-specific notification types.
			b.handleCodexNotification(raw, msgCh, &output)
		}

		_ = cmd.Wait()
		duration := time.Since(startTime)

		if runCtx.Err() == context.DeadlineExceeded {
			finalStatus = "timeout"
			finalError = fmt.Sprintf("codex timed out after %s", timeout)
		} else if runCtx.Err() == context.Canceled && finalStatus == "completed" {
			finalStatus = "aborted"
			finalError = "execution cancelled"
		}

		if finalError != "" {
			finalError = withStderrTail(finalError, "codex", stderrBuf.Tail())
		}

		resCh <- Result{
			Status:     finalStatus,
			Output:     output.String(),
			Error:      finalError,
			DurationMs: duration.Milliseconds(),
			SessionID:  sessionID,
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}

func (b *codexBackend) handleCodexApproval(raw map[string]json.RawMessage, client *acpClient) {
	// Auto-approve all execution and patch requests.
	client.handleAgentRequest(raw)
}

func (b *codexBackend) handleCodexNotification(raw map[string]json.RawMessage, msgCh chan<- Message, output *strings.Builder) {
	methodRaw, ok := raw["method"]
	if !ok {
		return
	}
	var method string
	_ = json.Unmarshal(methodRaw, &method)

	paramsRaw := raw["params"]

	switch method {
	case "item/completed":
		var params struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		_ = json.Unmarshal(paramsRaw, &params)
		if params.Type == "agentMessage" && params.Text != "" {
			output.WriteString(params.Text)
			trySend(msgCh, Message{Type: MessageText, Content: params.Text})
		}
	}
}
