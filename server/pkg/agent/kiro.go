package agent

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// kiroBackend implements Backend for the Kiro CLI via ACP protocol.
type kiroBackend struct {
	cfg Config
}

func (b *kiroBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	execPath := b.cfg.ExecutablePath
	if execPath == "" {
		execPath = "kiro-cli"
	}
	if _, err := exec.LookPath(execPath); err != nil {
		return nil, fmt.Errorf("kiro executable not found at %q: %w", execPath, err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)

	args := []string{"acp", "--trust-all-tools"}

	cmd := exec.CommandContext(runCtx, execPath, args...)
	cmd.WaitDelay = 10 * time.Second
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	cmd.Env = buildEnv(b.cfg.Env)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("kiro stdout pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("kiro stdin pipe: %w", err)
	}
	stderrBuf := newStderrTail(newLogWriter(b.cfg.Logger, "[kiro:stderr] "), stderrTailSize)
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start kiro: %w", err)
	}

	msgCh := make(chan Message, 256)
	resCh := make(chan Result, 1)

	// Setup ACP client.
	client := &acpClient{
		cfg:     b.cfg,
		stdin:   stdin,
		nextID:  1,
		pending: make(map[int]*pendingRPC),
	}
	client.onMessage = func(msg Message) {
		// Normalize tool names for Kiro.
		if msg.Type == MessageToolUse && msg.Tool != "" {
			msg.Tool = NormalizeACPToolName(msg.Tool)
		}
		trySend(msgCh, msg)
	}
	client.onPromptDone = func(pr acpPromptResult) {
		// Prompt completed — result will be sent after process exits.
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

		// Read stdout lines and dispatch to ACP client.
		scanner := newLineScanner(stdout)
		go func() {
			<-runCtx.Done()
			_ = stdout.Close()
		}()

		// Perform ACP handshake.
		_, err := client.request(runCtx, "initialize", map[string]any{
			"clientInfo": map[string]any{"name": "agenticflow-daemon", "version": "1.0.0"},
		})
		if err != nil {
			finalStatus = "failed"
			finalError = fmt.Sprintf("kiro ACP initialize failed: %v", err)
			_ = cmd.Process.Kill()
			resCh <- Result{Status: finalStatus, Error: finalError, DurationMs: time.Since(startTime).Milliseconds()}
			return
		}

		// Create or resume session.
		var sessionResult []byte
		if opts.ResumeSessionID != "" {
			sessionResult, err = client.request(runCtx, "session/load", map[string]any{"sessionId": opts.ResumeSessionID})
		} else {
			sessionResult, err = client.request(runCtx, "session/new", map[string]any{})
		}
		if err != nil {
			finalStatus = "failed"
			finalError = fmt.Sprintf("kiro ACP session failed: %v", err)
			_ = cmd.Process.Kill()
			resCh <- Result{Status: finalStatus, Error: finalError, DurationMs: time.Since(startTime).Milliseconds()}
			return
		}
		sessionID = extractACPSessionID(sessionResult)

		// Set model if specified.
		if opts.Model != "" {
			_, _ = client.request(runCtx, "session/set_model", map[string]any{"model": opts.Model})
		}

		// Send prompt.
		client.acceptNotification = func(updateType string) bool { return true }
		_, err = client.request(runCtx, "session/prompt", map[string]any{"prompt": prompt})
		if err != nil && runCtx.Err() == nil {
			finalStatus = "failed"
			finalError = fmt.Sprintf("kiro ACP prompt failed: %v", err)
		}

		// Read remaining stdout.
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				client.handleLine(line)
			}
		}

		_ = cmd.Wait()
		duration := time.Since(startTime)

		if runCtx.Err() == context.DeadlineExceeded {
			finalStatus = "timeout"
			finalError = fmt.Sprintf("kiro timed out after %s", timeout)
		} else if runCtx.Err() == context.Canceled && finalStatus == "completed" {
			finalStatus = "aborted"
			finalError = "execution cancelled"
		}

		if finalError != "" {
			finalError = withStderrTail(finalError, "kiro", stderrBuf.Tail())
		}

		resCh <- Result{
			Status:     finalStatus,
			Output:     output.String(),
			Error:      finalError,
			DurationMs: duration.Milliseconds(),
			SessionID:  sessionID,
		}
	}()

	// Start reading stdout in background for ACP line dispatch.
	// The goroutine above handles the handshake synchronously first,
	// then reads remaining lines.

	return &Session{Messages: msgCh, Result: resCh}, nil
}
