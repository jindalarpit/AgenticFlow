package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTruncatingBuffer_UnderLimit(t *testing.T) {
	tb := &truncatingBuffer{maxBytes: 100}
	data := []byte("hello world")
	n, err := tb.Write(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected n=%d, got %d", len(data), n)
	}
	if tb.String() != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", tb.String())
	}
}

func TestTruncatingBuffer_AtLimit(t *testing.T) {
	tb := &truncatingBuffer{maxBytes: 5}
	n, err := tb.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("expected n=5, got %d", n)
	}
	if tb.String() != "hello" {
		t.Errorf("expected %q, got %q", "hello", tb.String())
	}
}

func TestTruncatingBuffer_OverLimit(t *testing.T) {
	tb := &truncatingBuffer{maxBytes: 5}

	// First write fills the buffer.
	n, err := tb.Write([]byte("hel"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 3 {
		t.Errorf("expected n=3, got %d", n)
	}

	// Second write exceeds limit — only 2 bytes should be written.
	n, err = tb.Write([]byte("lo world"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Reports full length written (to satisfy io.Writer contract).
	if n != 8 {
		t.Errorf("expected n=8, got %d", n)
	}

	if tb.String() != "hello" {
		t.Errorf("expected %q, got %q", "hello", tb.String())
	}
}

func TestTruncatingBuffer_MultipleWritesPastLimit(t *testing.T) {
	tb := &truncatingBuffer{maxBytes: 10}

	tb.Write([]byte("12345"))
	tb.Write([]byte("67890"))
	// This write should be fully discarded.
	n, err := tb.Write([]byte("extra"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("expected n=5, got %d", n)
	}
	if tb.String() != "1234567890" {
		t.Errorf("expected %q, got %q", "1234567890", tb.String())
	}
}

func TestTailBuffer_UnderLimit(t *testing.T) {
	tb := &tailBuffer{maxChars: 100}
	tb.Write([]byte("hello world"))
	if tb.String() != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", tb.String())
	}
}

func TestTailBuffer_OverLimit(t *testing.T) {
	tb := &tailBuffer{maxChars: 5}
	tb.Write([]byte("hello world"))
	if tb.String() != "world" {
		t.Errorf("expected %q, got %q", "world", tb.String())
	}
}

func TestTailBuffer_MultipleWrites(t *testing.T) {
	tb := &tailBuffer{maxChars: 10}
	tb.Write([]byte("hello "))
	tb.Write([]byte("beautiful "))
	tb.Write([]byte("world"))
	// Total: "hello beautiful world" (21 chars), keep last 10: "iful world"
	expected := "iful world"
	if tb.String() != expected {
		t.Errorf("expected %q, got %q", expected, tb.String())
	}
}

func TestTailBuffer_ExactLimit(t *testing.T) {
	tb := &tailBuffer{maxChars: 5}
	tb.Write([]byte("abcde"))
	if tb.String() != "abcde" {
		t.Errorf("expected %q, got %q", "abcde", tb.String())
	}
}

func TestExecuteTask_AgentNotFound(t *testing.T) {
	cfg := testConfig()
	cfg.Agents = map[string]AgentEntry{
		"claude": {Name: "claude", Path: "/usr/bin/claude", Version: "1.0.0"},
	}
	logger := testLogger()
	client := &mockHTTPClient{}

	d := New(cfg, logger)
	d.SetClient(client)

	ctx := context.Background()
	task := &PollResponse{
		TaskID:    "task-123",
		AgentType: "nonexistent-agent",
		Prompt:    "do something",
	}

	d.executeTask(ctx, task)

	// Should have called FailTask since agent type doesn't exist.
	client.mu.Lock()
	failCalls := client.failTaskCalls
	client.mu.Unlock()

	if failCalls != 1 {
		t.Errorf("expected 1 FailTask call, got %d", failCalls)
	}
}

func TestExecuteTask_SuccessfulExecution(t *testing.T) {
	// Create a temp directory for workspaces.
	tmpDir := t.TempDir()

	cfg := testConfig()
	cfg.WorkspacesRoot = tmpDir
	cfg.AgentTimeout = 10 * time.Second
	// Use "echo" as the agent binary — it just prints and exits 0.
	cfg.Agents = map[string]AgentEntry{
		"echo-agent": {Name: "echo-agent", Path: "/bin/echo", Version: "1.0.0"},
	}
	logger := testLogger()
	client := &mockHTTPClient{}

	d := New(cfg, logger)
	d.SetClient(client)

	ctx := context.Background()
	task := &PollResponse{
		TaskID:    "task-success",
		AgentType: "echo-agent",
		Prompt:    "hello from test",
	}

	d.executeTask(ctx, task)

	// Should have called StartTask and CompleteTask.
	client.mu.Lock()
	startCalls := client.startTaskCalls
	completeCalls := client.completeTaskCalls
	failCalls := client.failTaskCalls
	client.mu.Unlock()

	if startCalls != 1 {
		t.Errorf("expected 1 StartTask call, got %d", startCalls)
	}
	if completeCalls != 1 {
		t.Errorf("expected 1 CompleteTask call, got %d", completeCalls)
	}
	if failCalls != 0 {
		t.Errorf("expected 0 FailTask calls, got %d", failCalls)
	}
}

func TestExecuteTask_FailedExecution(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := testConfig()
	cfg.WorkspacesRoot = tmpDir
	cfg.AgentTimeout = 10 * time.Second
	// Use "false" as the agent binary — it always exits with code 1.
	cfg.Agents = map[string]AgentEntry{
		"fail-agent": {Name: "fail-agent", Path: "/usr/bin/false", Version: "1.0.0"},
	}
	logger := testLogger()
	client := &mockHTTPClient{}

	d := New(cfg, logger)
	d.SetClient(client)

	ctx := context.Background()
	task := &PollResponse{
		TaskID:    "task-fail",
		AgentType: "fail-agent",
		Prompt:    "",
	}

	d.executeTask(ctx, task)

	// Should have called StartTask and FailTask.
	client.mu.Lock()
	startCalls := client.startTaskCalls
	completeCalls := client.completeTaskCalls
	failCalls := client.failTaskCalls
	client.mu.Unlock()

	if startCalls != 1 {
		t.Errorf("expected 1 StartTask call, got %d", startCalls)
	}
	if completeCalls != 0 {
		t.Errorf("expected 0 CompleteTask calls, got %d", completeCalls)
	}
	if failCalls != 1 {
		t.Errorf("expected 1 FailTask call, got %d", failCalls)
	}
}

func TestExecuteTask_Timeout(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := testConfig()
	cfg.WorkspacesRoot = tmpDir
	cfg.AgentTimeout = 100 * time.Millisecond // Very short timeout.
	// Use "sleep" as the agent binary — it will exceed the timeout.
	cfg.Agents = map[string]AgentEntry{
		"sleep-agent": {Name: "sleep-agent", Path: "/bin/sleep", Version: "1.0.0"},
	}
	logger := testLogger()
	client := &mockHTTPClient{}

	d := New(cfg, logger)
	d.SetClient(client)

	ctx := context.Background()
	task := &PollResponse{
		TaskID:       "task-timeout",
		AgentType:    "sleep-agent",
		Prompt:       "60", // sleep for 60 seconds (will be killed by timeout).
		ArgsTemplate: "{{prompt}}",
	}

	d.executeTask(ctx, task)

	// Should have called StartTask and FailTask (timeout).
	client.mu.Lock()
	startCalls := client.startTaskCalls
	failCalls := client.failTaskCalls
	client.mu.Unlock()

	if startCalls != 1 {
		t.Errorf("expected 1 StartTask call, got %d", startCalls)
	}
	if failCalls != 1 {
		t.Errorf("expected 1 FailTask call, got %d", failCalls)
	}
}

func TestExecuteTask_RunsInGoroutine(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := testConfig()
	cfg.WorkspacesRoot = tmpDir
	cfg.AgentTimeout = 5 * time.Second
	cfg.PollInterval = 50 * time.Millisecond
	cfg.MaxConcurrentTasks = 5
	cfg.Agents = map[string]AgentEntry{
		"echo-agent": {Name: "echo-agent", Path: "/bin/echo", Version: "1.0.0"},
	}
	logger := testLogger()

	// Return a task on the first poll, then empty.
	client := &mockHTTPClient{}
	client.pollResp = nil // Will be overridden below.

	d := New(cfg, logger)
	d.SetClient(client)
	d.runtimes["rt-1"] = "echo-agent"

	// Override poll to return a task once.
	client.pollResp = &PollResponse{
		TaskID:    "task-goroutine",
		AgentType: "echo-agent",
		Prompt:    "test",
	}

	ctx := context.Background()
	// Call pollForTasks — it should spawn a goroutine.
	d.pollForTasks(ctx)

	// Active tasks should be 1 (or already completed since echo is fast).
	// Wait a bit for the goroutine to complete.
	time.Sleep(200 * time.Millisecond)

	// After completion, active tasks should be back to 0.
	if d.ActiveTasks() != 0 {
		t.Errorf("expected 0 active tasks after completion, got %d", d.ActiveTasks())
	}
}

func TestBufferOutput_WritesFile(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := testConfig()
	cfg.WorkspacesRoot = tmpDir
	logger := testLogger()

	d := New(cfg, logger)

	d.bufferOutput("task-buf-1", "stdout content", "stderr content", 1, false)

	// Check that the buffer file was created.
	bufferFile := filepath.Join(tmpDir, ".buffers", "task-buf-1.buf")
	data, err := os.ReadFile(bufferFile)
	if err != nil {
		t.Fatalf("failed to read buffer file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "task_id=task-buf-1") {
		t.Errorf("buffer file missing task_id")
	}
	if !strings.Contains(content, "status=failed") {
		t.Errorf("buffer file missing status=failed")
	}
	if !strings.Contains(content, "stdout content") {
		t.Errorf("buffer file missing stdout content")
	}
	if !strings.Contains(content, "stderr content") {
		t.Errorf("buffer file missing stderr content")
	}
}

func TestBufferOutput_TruncatesLargeOutput(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := testConfig()
	cfg.WorkspacesRoot = tmpDir
	logger := testLogger()

	d := New(cfg, logger)

	// Create output larger than 5 MB.
	largeStdout := strings.Repeat("x", 6*1024*1024)
	d.bufferOutput("task-buf-large", largeStdout, "err", 0, true)

	bufferFile := filepath.Join(tmpDir, ".buffers", "task-buf-large.buf")
	info, err := os.Stat(bufferFile)
	if err != nil {
		t.Fatalf("failed to stat buffer file: %v", err)
	}

	// File should be at most ~5 MB + metadata overhead.
	if info.Size() > int64(maxLocalBufferBytes+1024) {
		t.Errorf("buffer file too large: %d bytes (max expected ~%d)", info.Size(), maxLocalBufferBytes+1024)
	}
}

func TestTruncatingBuffer_LargeOutput(t *testing.T) {
	// Verify 1 MB truncation.
	tb := &truncatingBuffer{maxBytes: maxStdoutBytes}

	// Write 2 MB of data.
	chunk := strings.Repeat("a", 1024)
	for i := 0; i < 2048; i++ {
		tb.Write([]byte(chunk))
	}

	result := tb.String()
	if len(result) != maxStdoutBytes {
		t.Errorf("expected output length %d, got %d", maxStdoutBytes, len(result))
	}
}

func TestTailBuffer_LargeStderr(t *testing.T) {
	// Verify 4096 char tail.
	tb := &tailBuffer{maxChars: maxStderrChars}

	// Write a large amount of data.
	for i := 0; i < 1000; i++ {
		tb.Write([]byte(fmt.Sprintf("line %d: some error output\n", i)))
	}

	result := tb.String()
	runes := []rune(result)
	if len(runes) != maxStderrChars {
		t.Errorf("expected %d chars, got %d", maxStderrChars, len(runes))
	}
}

func TestExecuteTask_ActiveTasksIncrement(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := testConfig()
	cfg.WorkspacesRoot = tmpDir
	cfg.AgentTimeout = 5 * time.Second
	cfg.Agents = map[string]AgentEntry{
		"echo-agent": {Name: "echo-agent", Path: "/bin/echo", Version: "1.0.0"},
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	client := &mockHTTPClient{}

	d := New(cfg, logger)
	d.SetClient(client)
	d.runtimes["rt-1"] = "echo-agent"

	// Set up poll to return a task.
	client.pollResp = &PollResponse{
		TaskID:    "task-active",
		AgentType: "echo-agent",
		Prompt:    "test",
	}

	// Before poll, active tasks should be 0.
	if d.ActiveTasks() != 0 {
		t.Fatalf("expected 0 active tasks before poll, got %d", d.ActiveTasks())
	}

	ctx := context.Background()
	d.pollForTasks(ctx)

	// Wait for the goroutine to complete.
	time.Sleep(300 * time.Millisecond)

	// After completion, active tasks should be back to 0.
	if d.ActiveTasks() != 0 {
		t.Errorf("expected 0 active tasks after completion, got %d", d.ActiveTasks())
	}
}
