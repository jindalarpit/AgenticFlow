package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ── buildClaudeArgs tests ──

func TestBuildClaudeArgs_BaseFlags(t *testing.T) {
	t.Parallel()

	args := buildClaudeArgs(ExecOptions{}, slog.Default())

	// Must always include the protocol-critical flags.
	expected := []string{
		"-p",
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--verbose",
	}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, want := range expected {
		if args[i] != want {
			t.Fatalf("args[%d] = %q, want %q", i, args[i], want)
		}
	}
}

func TestBuildClaudeArgs_WithModel(t *testing.T) {
	t.Parallel()

	args := buildClaudeArgs(ExecOptions{Model: "claude-sonnet-4-20250514"}, slog.Default())

	found := false
	for i, a := range args {
		if a == "--model" && i+1 < len(args) && args[i+1] == "claude-sonnet-4-20250514" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected --model claude-sonnet-4-20250514 in args: %v", args)
	}
}

func TestBuildClaudeArgs_WithSystemPrompt(t *testing.T) {
	t.Parallel()

	args := buildClaudeArgs(ExecOptions{SystemPrompt: "You are helpful"}, slog.Default())

	found := false
	for i, a := range args {
		if a == "--append-system-prompt" && i+1 < len(args) && args[i+1] == "You are helpful" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected --append-system-prompt in args: %v", args)
	}
}

func TestBuildClaudeArgs_WithResumeSession(t *testing.T) {
	t.Parallel()

	args := buildClaudeArgs(ExecOptions{ResumeSessionID: "sess-abc123"}, slog.Default())

	found := false
	for i, a := range args {
		if a == "--resume" && i+1 < len(args) && args[i+1] == "sess-abc123" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected --resume sess-abc123 in args: %v", args)
	}
}

func TestBuildClaudeArgs_CustomArgsPassThrough(t *testing.T) {
	t.Parallel()

	args := buildClaudeArgs(ExecOptions{
		CustomArgs: []string{"--max-turns", "50", "--model", "o3"},
	}, slog.Default())

	// Custom args should appear at the end.
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--max-turns 50") {
		t.Fatalf("expected --max-turns 50 in args: %v", args)
	}
	if !strings.Contains(joined, "--model o3") {
		t.Fatalf("expected --model o3 in args: %v", args)
	}
}

func TestBuildClaudeArgs_BlockedArgsFiltered(t *testing.T) {
	t.Parallel()

	args := buildClaudeArgs(ExecOptions{
		CustomArgs: []string{"--output-format", "text", "--model", "o3"},
	}, slog.Default())

	// --output-format text should be stripped (blocked with value).
	for i, a := range args {
		if a == "--output-format" && i+1 < len(args) && args[i+1] == "text" {
			t.Fatalf("blocked --output-format text should have been filtered: %v", args)
		}
	}

	// --model o3 should pass through.
	foundModel := false
	for i, a := range args {
		if a == "--model" && i+1 < len(args) && args[i+1] == "o3" {
			foundModel = true
		}
	}
	if !foundModel {
		t.Fatalf("expected --model o3 in args but it was missing: %v", args)
	}
}

func TestBuildClaudeArgs_BlockedStandaloneFlag(t *testing.T) {
	t.Parallel()

	// -p is blocked as standalone — it should be removed but not consume the next arg.
	args := buildClaudeArgs(ExecOptions{
		CustomArgs: []string{"-p", "--max-turns", "10"},
	}, slog.Default())

	// The custom -p should be filtered, but --max-turns 10 should remain.
	// Note: the base args already include -p, so we check the custom portion.
	foundMaxTurns := false
	for i, a := range args {
		if a == "--max-turns" && i+1 < len(args) && args[i+1] == "10" {
			foundMaxTurns = true
		}
	}
	if !foundMaxTurns {
		t.Fatalf("expected --max-turns 10 to pass through after blocking -p: %v", args)
	}
}

func TestBuildClaudeArgs_BlockedVerboseFlag(t *testing.T) {
	t.Parallel()

	// --verbose is blocked as standalone.
	args := buildClaudeArgs(ExecOptions{
		CustomArgs: []string{"--verbose", "--max-turns", "5"},
	}, slog.Default())

	// Count occurrences of --verbose — should be exactly 1 (the base one).
	count := 0
	for _, a := range args {
		if a == "--verbose" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 --verbose (base), got %d in: %v", count, args)
	}

	// --max-turns 5 should still be present.
	foundMaxTurns := false
	for i, a := range args {
		if a == "--max-turns" && i+1 < len(args) && args[i+1] == "5" {
			foundMaxTurns = true
		}
	}
	if !foundMaxTurns {
		t.Fatalf("expected --max-turns 5 in args: %v", args)
	}
}

func TestBuildClaudeArgs_BlockedResumeFromCustomArgs(t *testing.T) {
	t.Parallel()

	// --resume is blocked with value — user cannot override it via custom_args.
	args := buildClaudeArgs(ExecOptions{
		ResumeSessionID: "sess-real",
		CustomArgs:      []string{"--resume", "sess-evil", "--max-turns", "3"},
	}, slog.Default())

	// Should have exactly one --resume with the real session ID.
	resumeCount := 0
	for i, a := range args {
		if a == "--resume" {
			resumeCount++
			if i+1 < len(args) && args[i+1] == "sess-evil" {
				t.Fatalf("blocked --resume sess-evil should have been filtered: %v", args)
			}
		}
	}
	if resumeCount != 1 {
		t.Fatalf("expected exactly 1 --resume, got %d in: %v", resumeCount, args)
	}
}

// ── writeClaudeInput tests ──

func TestWriteClaudeInput_EncodesUserMessage(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := writeClaudeInput(&buf, "say pong")
	if err != nil {
		t.Fatalf("writeClaudeInput: %v", err)
	}

	data := buf.Bytes()
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("expected newline-terminated payload, got %q", data)
	}

	var payload map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["type"] != "user" {
		t.Fatalf("expected type user, got %v", payload["type"])
	}

	message, ok := payload["message"].(map[string]any)
	if !ok {
		t.Fatalf("expected message object, got %T", payload["message"])
	}
	if message["role"] != "user" {
		t.Fatalf("expected role user, got %v", message["role"])
	}

	content, ok := message["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("expected one content block, got %v", message["content"])
	}
	block, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("expected content block object, got %T", content[0])
	}
	if block["type"] != "text" || block["text"] != "say pong" {
		t.Fatalf("unexpected content block: %v", block)
	}
}

func TestWriteClaudeInput_EmptyPrompt(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := writeClaudeInput(&buf, "")
	if err != nil {
		t.Fatalf("writeClaudeInput with empty prompt: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	message := payload["message"].(map[string]any)
	content := message["content"].([]any)
	block := content[0].(map[string]any)
	if block["text"] != "" {
		t.Fatalf("expected empty text, got %q", block["text"])
	}
}

// ── resolveClaudeSessionID tests ──

func TestResolveClaudeSessionID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		requested string
		emitted   string
		failed    bool
		want      string
	}{
		{
			name:      "no resume requested propagates emitted",
			requested: "",
			emitted:   "fresh-abc",
			failed:    false,
			want:      "fresh-abc",
		},
		{
			name:      "resume succeeded keeps matching id",
			requested: "sess-old",
			emitted:   "sess-old",
			failed:    false,
			want:      "sess-old",
		},
		{
			name:      "resume succeeded but run failed mid-turn keeps id",
			requested: "sess-old",
			emitted:   "sess-old",
			failed:    true,
			want:      "sess-old",
		},
		{
			name:      "resume did not land and run failed clears id",
			requested: "sess-dead",
			emitted:   "fresh-new",
			failed:    true,
			want:      "",
		},
		{
			name:      "resume did not land but run succeeded keeps fresh id",
			requested: "sess-dead",
			emitted:   "fresh-new",
			failed:    false,
			want:      "fresh-new",
		},
		{
			name:      "no emitted id leaves result empty",
			requested: "sess-old",
			emitted:   "",
			failed:    true,
			want:      "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := resolveClaudeSessionID(tc.requested, tc.emitted, tc.failed)
			if got != tc.want {
				t.Fatalf("resolveClaudeSessionID(%q, %q, %v) = %q, want %q",
					tc.requested, tc.emitted, tc.failed, got, tc.want)
			}
		})
	}
}

// ── filterCustomArgs tests (Claude-specific blocked set) ──

func TestFilterCustomArgs_ClaudeBlockedSet(t *testing.T) {
	t.Parallel()
	logger := slog.Default()

	// Blocks flag with separate value.
	result := filterCustomArgs([]string{"--output-format", "text", "--model", "o3"}, claudeBlockedArgs, logger)
	if len(result) != 2 || result[0] != "--model" || result[1] != "o3" {
		t.Fatalf("expected [--model o3], got %v", result)
	}

	// Blocks flag=value form.
	result = filterCustomArgs([]string{"--input-format=text", "--max-turns", "5"}, claudeBlockedArgs, logger)
	if len(result) != 2 || result[0] != "--max-turns" || result[1] != "5" {
		t.Fatalf("expected [--max-turns 5], got %v", result)
	}

	// Blocks standalone short flags without consuming next arg.
	result = filterCustomArgs([]string{"-p", "--max-turns", "10"}, claudeBlockedArgs, logger)
	if len(result) != 2 || result[0] != "--max-turns" || result[1] != "10" {
		t.Fatalf("expected [--max-turns 10], got %v", result)
	}

	// Passes through non-blocked args.
	result = filterCustomArgs([]string{"--model", "o3", "--max-turns", "50"}, claudeBlockedArgs, logger)
	if len(result) != 4 {
		t.Fatalf("expected all 4 args to pass through, got %v", result)
	}

	// Handles nil blocked map.
	result = filterCustomArgs([]string{"--anything"}, nil, logger)
	if len(result) != 1 {
		t.Fatalf("expected args to pass through with nil blocked map, got %v", result)
	}

	// Handles empty args.
	result = filterCustomArgs(nil, claudeBlockedArgs, logger)
	if result != nil {
		t.Fatalf("expected nil for nil input, got %v", result)
	}
}

// ── Process lifecycle tests (using mock executables) ──

func TestClaudeExecute_StderrTailOnNonZeroExit(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}

	// Fake claude binary: drains stdin so writeClaudeInput succeeds, writes
	// a diagnostic line to stderr, then exits non-zero before emitting any
	// stream-json to stdout.
	fakePath := filepath.Join(t.TempDir(), "claude")
	script := "#!/bin/sh\n" +
		"cat >/dev/null\n" +
		"echo \"FATAL ERROR: V8 abort: assertion failed\" >&2\n" +
		"exit 3\n"
	writeTestExecutable(t, fakePath, []byte(script))

	backend, err := New("claude", Config{ExecutablePath: fakePath, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("new claude backend: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := backend.Execute(ctx, "prompt-ignored", ExecOptions{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// Drain message stream so the lifecycle goroutine can progress.
	go func() {
		for range session.Messages {
		}
	}()

	select {
	case result, ok := <-session.Result:
		if !ok {
			t.Fatal("result channel closed without a value")
		}
		if result.Status != "failed" {
			t.Fatalf("expected status=failed, got %q (error=%q)", result.Status, result.Error)
		}
		if !strings.Contains(result.Error, "claude exited with error") {
			t.Fatalf("expected error to mention exit, got %q", result.Error)
		}
		if !strings.Contains(result.Error, "V8 abort: assertion failed") {
			t.Fatalf("expected error to include stderr content, got %q", result.Error)
		}
		if !strings.Contains(result.Error, "claude stderr:") {
			t.Fatalf("expected stderr label in error, got %q", result.Error)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for result")
	}
}

func TestClaudeExecute_ContextCancellation(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}

	// Fake claude binary: drains stdin, emits a system message, then sleeps
	// long enough for us to cancel the context.
	fakePath := filepath.Join(t.TempDir(), "claude")
	script := "#!/bin/sh\n" +
		"cat >/dev/null\n" +
		"printf '%s\\n' '{\"type\":\"system\",\"session_id\":\"sess-cancel\"}'\n" +
		"sleep 60\n"
	writeTestExecutable(t, fakePath, []byte(script))

	backend, err := New("claude", Config{ExecutablePath: fakePath, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("new claude backend: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	session, err := backend.Execute(ctx, "prompt-ignored", ExecOptions{Timeout: 30 * time.Second})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Drain messages.
	go func() {
		for range session.Messages {
		}
	}()

	// Give the process a moment to start and emit the system message.
	time.Sleep(200 * time.Millisecond)

	// Cancel the context — this should trigger SIGTERM → SIGKILL sequence.
	cancel()

	select {
	case result, ok := <-session.Result:
		if !ok {
			t.Fatal("result channel closed without a value")
		}
		if result.Status != "aborted" {
			t.Fatalf("expected status=aborted, got %q (error=%q)", result.Status, result.Error)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for result after context cancellation")
	}
}

func TestClaudeExecute_TimeoutTriggersTermination(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}

	// Fake claude binary: drains stdin, emits a system message, then sleeps
	// indefinitely. The short timeout should trigger termination.
	fakePath := filepath.Join(t.TempDir(), "claude")
	script := "#!/bin/sh\n" +
		"cat >/dev/null\n" +
		"printf '%s\\n' '{\"type\":\"system\",\"session_id\":\"sess-timeout\"}'\n" +
		"sleep 60\n"
	writeTestExecutable(t, fakePath, []byte(script))

	backend, err := New("claude", Config{ExecutablePath: fakePath, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("new claude backend: %v", err)
	}

	ctx := context.Background()
	session, err := backend.Execute(ctx, "prompt-ignored", ExecOptions{
		Timeout: 500 * time.Millisecond, // Very short timeout.
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Drain messages.
	go func() {
		for range session.Messages {
		}
	}()

	select {
	case result, ok := <-session.Result:
		if !ok {
			t.Fatal("result channel closed without a value")
		}
		if result.Status != "timeout" {
			t.Fatalf("expected status=timeout, got %q (error=%q)", result.Status, result.Error)
		}
		if !strings.Contains(result.Error, "timed out") {
			t.Fatalf("expected error to mention timeout, got %q", result.Error)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for result")
	}
}

// ── Helper ──

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return data
}
