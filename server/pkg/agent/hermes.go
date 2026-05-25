package agent

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// hermesBackend implements Backend for the Hermes CLI via ACP protocol.
type hermesBackend struct {
	cfg Config
}

func (b *hermesBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	execPath := b.cfg.ExecutablePath
	if execPath == "" {
		execPath = "hermes"
	}
	if _, err := exec.LookPath(execPath); err != nil {
		return nil, fmt.Errorf("hermes executable not found at %q: %w", execPath, err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)

	args := []string{"acp"}

	cmd := exec.CommandContext(runCtx, execPath, args...)
	cmd.WaitDelay = 10 * time.Second
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	// Set HERMES_YOLO_MODE=1 for auto-approval.
	env := make(map[string]string, len(b.cfg.Env)+1)
	for k, v := range b.cfg.Env {
		env[k] = v
	}
	env["HERMES_YOLO_MODE"] = "1"
	cmd.Env = buildEnv(env)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("hermes stdout pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("hermes stdin pipe: %w", err)
	}
	stderrBuf := newStderrTail(newLogWriter(b.cfg.Logger, "[hermes:stderr] "), stderrTailSize)
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start hermes: %w", err)
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
		if msg.Type == MessageToolUse && msg.Tool != "" {
			msg.Tool = NormalizeACPToolName(msg.Tool)
		}
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

		// ACP handshake.
		_, err := client.request(runCtx, "initialize", map[string]any{
			"clientInfo": map[string]any{"name": "agenticflow-daemon", "version": "1.0.0"},
		})
		if err != nil {
			finalStatus = "failed"
			finalError = fmt.Sprintf("hermes ACP initialize failed: %v", err)
			_ = cmd.Process.Kill()
			resCh <- Result{Status: finalStatus, Error: finalError, DurationMs: time.Since(startTime).Milliseconds()}
			return
		}

		var sessionResult []byte
		if opts.ResumeSessionID != "" {
			sessionResult, err = client.request(runCtx, "session/resume", map[string]any{"sessionId": opts.ResumeSessionID})
		} else {
			sessionResult, err = client.request(runCtx, "session/new", map[string]any{})
		}
		if err != nil {
			finalStatus = "failed"
			finalError = fmt.Sprintf("hermes ACP session failed: %v", err)
			_ = cmd.Process.Kill()
			resCh <- Result{Status: finalStatus, Error: finalError, DurationMs: time.Since(startTime).Milliseconds()}
			return
		}
		sessionID = extractACPSessionID(sessionResult)

		// Send prompt.
		client.acceptNotification = func(updateType string) bool { return true }
		_, err = client.request(runCtx, "session/prompt", map[string]any{"prompt": prompt})
		if err != nil && runCtx.Err() == nil {
			finalStatus = "failed"
			finalError = fmt.Sprintf("hermes ACP prompt failed: %v", err)
		}

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
			finalError = fmt.Sprintf("hermes timed out after %s", timeout)
		} else if runCtx.Err() == context.Canceled && finalStatus == "completed" {
			finalStatus = "aborted"
			finalError = "execution cancelled"
		}

		if finalError != "" {
			finalError = withStderrTail(finalError, "hermes", stderrBuf.Tail())
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
