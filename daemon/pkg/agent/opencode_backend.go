package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// opencodeBackend implements Backend by spawning `opencode run --format json`
// and reading streaming JSON events from stdout.
type opencodeBackend struct {
	cfg Config
}

func (b *opencodeBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	execPath := b.cfg.ExecutablePath
	if execPath == "" {
		execPath = "opencode"
	}
	resolved, err := exec.LookPath(execPath)
	if err != nil {
		return nil, fmt.Errorf("opencode executable not found at %q: %w", execPath, err)
	}
	if runtime.GOOS == "windows" {
		if native := resolveOpenCodeNativeFromShim(resolved, os.Stat); native != "" {
			b.cfg.Logger.Info("opencode resolved to native binary", "shim", resolved, "native", native)
			resolved = native
		}
	}
	execPath = resolved

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)

	args := []string{"run", "--format", "json", "--dangerously-skip-permissions"}
	if opts.Cwd != "" {
		args = append(args, "--dir", opts.Cwd)
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.ResumeSessionID != "" {
		args = append(args, "--session", opts.ResumeSessionID)
	}
	if len(opts.CustomArgs) > 0 {
		args = append(args, opts.CustomArgs...)
	}
	// Message is the positional argument — must come last.
	// If there's a system prompt, prepend it to the message.
	message := prompt
	if opts.SystemPrompt != "" {
		message = opts.SystemPrompt + "\n\n" + prompt
	}
	args = append(args, message)

	cmd := exec.CommandContext(runCtx, execPath, args...)
	b.cfg.Logger.Info("agent command", "exec", execPath, "args", args)
	cmd.WaitDelay = 10 * time.Second
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}

	env := buildAgentEnv(b.cfg.Env)
	if opts.Cwd != "" {
		env = append(env, "PWD="+opts.Cwd)
	}
	for k, v := range opts.CustomEnv {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("opencode stdout pipe: %w", err)
	}
	stderrBuf := &strings.Builder{}
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start opencode: %w", err)
	}

	b.cfg.Logger.Info("opencode started", "pid", cmd.Process.Pid, "cwd", opts.Cwd, "model", opts.Model)

	msgCh := make(chan Message, 256)
	resCh := make(chan Result, 1)

	go func() {
		<-runCtx.Done()
		_ = stdout.Close()
	}()

	go func() {
		defer cancel()
		defer close(msgCh)
		defer close(resCh)

		startTime := time.Now()
		scanResult := b.processEvents(stdout, msgCh)

		exitErr := cmd.Wait()
		duration := time.Since(startTime)

		if runCtx.Err() == context.DeadlineExceeded {
			scanResult.status = "timeout"
			scanResult.errMsg = fmt.Sprintf("opencode timed out after %s", timeout)
		} else if runCtx.Err() == context.Canceled {
			scanResult.status = "aborted"
			scanResult.errMsg = "execution cancelled"
		} else if exitErr != nil && scanResult.status == "completed" {
			scanResult.status = "failed"
			stderr := stderrBuf.String()
			if stderr != "" {
				scanResult.errMsg = fmt.Sprintf("opencode exited with error: %v\nstderr: %s", exitErr, stderr)
			} else {
				scanResult.errMsg = fmt.Sprintf("opencode exited with error: %v", exitErr)
			}
		}

		b.cfg.Logger.Info("opencode finished", "pid", cmd.Process.Pid, "status", scanResult.status, "duration", duration.Round(time.Millisecond).String())

		var usage map[string]TokenUsage
		u := scanResult.usage
		if u.InputTokens > 0 || u.OutputTokens > 0 || u.CacheReadTokens > 0 || u.CacheWriteTokens > 0 {
			model := opts.Model
			if model == "" {
				model = "unknown"
			}
			usage = map[string]TokenUsage{model: u}
		}

		resCh <- Result{
			Status:     scanResult.status,
			Output:     scanResult.output,
			Error:      scanResult.errMsg,
			DurationMs: duration.Milliseconds(),
			SessionID:  scanResult.sessionID,
			Usage:      usage,
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}

type opencodeEventResult struct {
	status    string
	errMsg    string
	output    string
	sessionID string
	usage     TokenUsage
}

func (b *opencodeBackend) processEvents(r io.Reader, ch chan<- Message) opencodeEventResult {
	var output strings.Builder
	var sessionID string
	var usage TokenUsage
	finalStatus := "completed"
	var finalError string

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event opencodeEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		if event.SessionID != "" {
			sessionID = event.SessionID
		}

		switch event.Type {
		case "text":
			text := event.Part.Text
			if text != "" {
				output.WriteString(text)
				trySendMsg(ch, Message{Type: MessageText, Content: text})
			}

		case "tool_use":
			var input map[string]any
			if event.Part.State != nil && event.Part.State.Input != nil {
				_ = json.Unmarshal(event.Part.State.Input, &input)
			}
			trySendMsg(ch, Message{
				Type:   MessageToolUse,
				Tool:   event.Part.Tool,
				CallID: event.Part.CallID,
				Input:  input,
			})
			if event.Part.State != nil && event.Part.State.Status == "completed" {
				outputStr := extractOCToolOutput(event.Part.State.Output)
				trySendMsg(ch, Message{
					Type:   MessageToolResult,
					Tool:   event.Part.Tool,
					CallID: event.Part.CallID,
					Output: outputStr,
				})
			}

		case "error":
			errMsg := ""
			if event.Error != nil {
				errMsg = event.Error.Message()
			}
			if errMsg == "" {
				errMsg = "unknown opencode error"
			}
			b.cfg.Logger.Warn("opencode error event", "error", errMsg)
			trySendMsg(ch, Message{Type: MessageError, Content: errMsg})
			finalStatus = "failed"
			finalError = errMsg

		case "step_start":
			trySendMsg(ch, Message{Type: MessageStatus, Status: "running"})

		case "step_finish":
			if t := event.Part.Tokens; t != nil {
				usage.InputTokens += t.Input
				usage.OutputTokens += t.Output
				if t.Cache != nil {
					usage.CacheReadTokens += t.Cache.Read
					usage.CacheWriteTokens += t.Cache.Write
				}
			}
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		b.cfg.Logger.Warn("opencode stdout scanner error", "error", scanErr)
		if finalStatus == "completed" {
			finalStatus = "failed"
			finalError = fmt.Sprintf("stdout read error: %v", scanErr)
		}
	}

	return opencodeEventResult{
		status:    finalStatus,
		errMsg:    finalError,
		output:    output.String(),
		sessionID: sessionID,
		usage:     usage,
	}
}

func extractOCToolOutput(output any) string {
	if output == nil {
		return ""
	}
	if s, ok := output.(string); ok {
		return s
	}
	data, _ := json.Marshal(output)
	return string(data)
}

// resolveOpenCodeNativeFromShim resolves the native opencode binary on Windows
// to avoid .cmd shim argv truncation issues with multi-line prompts.
func resolveOpenCodeNativeFromShim(shimPath string, statFn func(string) (os.FileInfo, error)) string {
	if !strings.EqualFold(filepath.Ext(shimPath), ".cmd") {
		return ""
	}
	prefix := filepath.Dir(shimPath)
	for _, pkg := range opencodeWindowsPackageCandidates(runtime.GOARCH) {
		candidate := filepath.Join(prefix, "node_modules", "opencode-ai", "node_modules", pkg, "bin", "opencode.exe")
		if _, err := statFn(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func opencodeWindowsPackageCandidates(goarch string) []string {
	switch goarch {
	case "arm64":
		return []string{"opencode-windows-arm64", "opencode-windows-x64", "opencode-windows-x64-baseline"}
	default:
		return []string{"opencode-windows-x64", "opencode-windows-x64-baseline", "opencode-windows-arm64"}
	}
}

// ── JSON types for `opencode run --format json` stdout events ──

type opencodeEvent struct {
	Type      string            `json:"type"`
	Timestamp int64             `json:"timestamp,omitempty"`
	SessionID string            `json:"sessionID,omitempty"`
	Part      opencodeEventPart `json:"part"`
	Error     *opencodeError    `json:"error,omitempty"`
}

type opencodeEventPart struct {
	ID        string `json:"id,omitempty"`
	MessageID string `json:"messageID,omitempty"`
	SessionID string `json:"sessionID,omitempty"`
	Type      string `json:"type,omitempty"`
	Text      string `json:"text,omitempty"`
	Tool      string `json:"tool,omitempty"`
	CallID    string `json:"callID,omitempty"`
	State     *opencodeToolState `json:"state,omitempty"`
	Tokens    *opencodeTokens    `json:"tokens,omitempty"`
}

type opencodeTokens struct {
	Input  int64              `json:"input"`
	Output int64              `json:"output"`
	Cache  *opencodeCacheTokens `json:"cache,omitempty"`
}

type opencodeCacheTokens struct {
	Read  int64 `json:"read"`
	Write int64 `json:"write"`
}

type opencodeToolState struct {
	Status string          `json:"status,omitempty"`
	Input  json.RawMessage `json:"input,omitempty"`
	Output any             `json:"output,omitempty"`
}

type opencodeError struct {
	Name string           `json:"name,omitempty"`
	Data *opencodeErrData `json:"data,omitempty"`
}

func (e *opencodeError) Message() string {
	if e.Data != nil && e.Data.Message != "" {
		return e.Data.Message
	}
	if e.Name != "" {
		return e.Name
	}
	return ""
}

type opencodeErrData struct {
	Message string `json:"message,omitempty"`
}
