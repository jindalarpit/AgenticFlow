package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
)

// mockPipe is a simple io.WriteCloser for testing stdin writes.
type mockPipe struct {
	mu      sync.Mutex
	data    []byte
	closed  bool
	writeErr error
}

func (p *mockPipe) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.writeErr != nil {
		return 0, p.writeErr
	}
	p.data = append(p.data, b...)
	return len(b), nil
}

func (p *mockPipe) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return nil
}

func (p *mockPipe) Written() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return string(p.data)
}

// trackingClient extends mockHTTPClient to capture reported messages.
type trackingClient struct {
	mockHTTPClient
	mu              sync.Mutex
	reportedMsgs    []TaskMessage
	reportedTaskIDs []string
}

func (c *trackingClient) ReportMessages(_ context.Context, taskID string, messages []TaskMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reportedTaskIDs = append(c.reportedTaskIDs, taskID)
	c.reportedMsgs = append(c.reportedMsgs, messages...)
	return c.reportMsgErr
}

func (c *trackingClient) getReportedMessages() []TaskMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]TaskMessage{}, c.reportedMsgs...)
}

func (c *trackingClient) getReportedTaskIDs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string{}, c.reportedTaskIDs...)
}

func testDaemonWithClient(client HTTPClient) *Daemon {
	cfg := Config{
		ServerURL:          "http://localhost:8080",
		DaemonID:           "test-daemon",
		DeviceName:         "test-machine",
		Agents:             map[string]AgentEntry{"claude": {Name: "claude", Path: "/usr/bin/claude", Version: "1.0.0"}},
		WorkspacesRoot:     os.TempDir(),
		PollInterval:       50,
		HeartbeatInterval:  50,
		AgentTimeout:       60,
		MaxConcurrentTasks: 5,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(cfg, logger)
	d.SetClient(client)
	return d
}

func TestHandleTaskInput_Success(t *testing.T) {
	client := &trackingClient{}
	d := testDaemonWithClient(client)

	// Register a pipe and sequence for the task.
	pipe := &mockPipe{}
	taskID := "task-123"
	d.stdinManager.Register(taskID, pipe)
	seq := 0
	d.sequences.Register(taskID, &seq)

	// Handle the input.
	d.handleTaskInput(taskID, "hello world")

	// Verify the pipe received the text with newline.
	written := pipe.Written()
	if written != "hello world\n" {
		t.Errorf("expected pipe to contain %q, got %q", "hello world\n", written)
	}

	// Verify the message was reported to the server.
	msgs := client.getReportedMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 reported message, got %d", len(msgs))
	}
	if msgs[0].Stream != "stdin" {
		t.Errorf("expected stream %q, got %q", "stdin", msgs[0].Stream)
	}
	if msgs[0].Content != "hello world" {
		t.Errorf("expected content %q, got %q", "hello world", msgs[0].Content)
	}
	if msgs[0].Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", msgs[0].Sequence)
	}

	// Verify the task ID was correct.
	taskIDs := client.getReportedTaskIDs()
	if len(taskIDs) != 1 || taskIDs[0] != taskID {
		t.Errorf("expected task_id %q, got %v", taskID, taskIDs)
	}
}

func TestHandleTaskInput_PipeNotFound(t *testing.T) {
	client := &trackingClient{}
	d := testDaemonWithClient(client)

	// Don't register any pipe — should fail gracefully.
	seq := 0
	d.sequences.Register("task-missing", &seq)

	d.handleTaskInput("task-missing", "hello")

	// Should report failure as stderr message.
	msgs := client.getReportedMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 failure message, got %d", len(msgs))
	}
	if msgs[0].Stream != "stderr" {
		t.Errorf("expected stream %q, got %q", "stderr", msgs[0].Stream)
	}
	if msgs[0].Content == "" {
		t.Error("expected non-empty failure content")
	}
}

func TestHandleTaskInput_PipeWriteError(t *testing.T) {
	client := &trackingClient{}
	d := testDaemonWithClient(client)

	// Register a pipe that returns an error on write.
	pipe := &mockPipe{writeErr: errors.New("broken pipe")}
	taskID := "task-broken"
	d.stdinManager.Register(taskID, pipe)
	seq := 0
	d.sequences.Register(taskID, &seq)

	d.handleTaskInput(taskID, "hello")

	// Should report failure as stderr message.
	msgs := client.getReportedMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 failure message, got %d", len(msgs))
	}
	if msgs[0].Stream != "stderr" {
		t.Errorf("expected stream %q for failure, got %q", "stderr", msgs[0].Stream)
	}
}

func TestHandleTaskInput_NoClient(t *testing.T) {
	cfg := Config{
		ServerURL:          "http://localhost:8080",
		DaemonID:           "test-daemon",
		DeviceName:         "test-machine",
		Agents:             map[string]AgentEntry{"claude": {Name: "claude", Path: "/usr/bin/claude", Version: "1.0.0"}},
		WorkspacesRoot:     os.TempDir(),
		PollInterval:       50,
		HeartbeatInterval:  50,
		AgentTimeout:       60,
		MaxConcurrentTasks: 5,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(cfg, logger)
	// Don't set client — should not panic.

	pipe := &mockPipe{}
	taskID := "task-no-client"
	d.stdinManager.Register(taskID, pipe)
	seq := 0
	d.sequences.Register(taskID, &seq)

	// Should not panic even without a client.
	d.handleTaskInput(taskID, "hello")

	// Pipe should still receive the write.
	if pipe.Written() != "hello\n" {
		t.Errorf("expected pipe to contain %q, got %q", "hello\n", pipe.Written())
	}
}

func TestHandleWSMessage_TaskInput(t *testing.T) {
	client := &trackingClient{}
	d := testDaemonWithClient(client)

	// Register a pipe and sequence for the task.
	pipe := &mockPipe{}
	taskID := "task-ws-1"
	d.stdinManager.Register(taskID, pipe)
	seq := 0
	d.sequences.Register(taskID, &seq)

	// Construct a valid task_input WebSocket message.
	event := WSEvent{
		Type: "task_input",
		Payload: mustMarshal(t, TaskInputPayload{
			TaskID: taskID,
			Text:   "ws input",
		}),
	}
	msg, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	d.HandleWSMessage(msg)

	// Verify the pipe received the text.
	if pipe.Written() != "ws input\n" {
		t.Errorf("expected pipe to contain %q, got %q", "ws input\n", pipe.Written())
	}

	// Verify the message was reported.
	msgs := client.getReportedMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 reported message, got %d", len(msgs))
	}
	if msgs[0].Stream != "stdin" {
		t.Errorf("expected stream %q, got %q", "stdin", msgs[0].Stream)
	}
}

func TestHandleWSMessage_InvalidJSON(t *testing.T) {
	client := &trackingClient{}
	d := testDaemonWithClient(client)

	// Should not panic on invalid JSON.
	d.HandleWSMessage([]byte("not json"))

	// No messages should be reported.
	msgs := client.getReportedMessages()
	if len(msgs) != 0 {
		t.Errorf("expected 0 reported messages, got %d", len(msgs))
	}
}

func TestHandleWSMessage_InvalidPayload(t *testing.T) {
	client := &trackingClient{}
	d := testDaemonWithClient(client)

	// Valid event type but invalid payload.
	msg := `{"type":"task_input","payload":"not an object"}`
	d.HandleWSMessage([]byte(msg))

	// No messages should be reported.
	msgs := client.getReportedMessages()
	if len(msgs) != 0 {
		t.Errorf("expected 0 reported messages, got %d", len(msgs))
	}
}

func TestHandleWSMessage_UnknownEventType(t *testing.T) {
	client := &trackingClient{}
	d := testDaemonWithClient(client)

	// Unknown event type should be handled gracefully.
	msg := `{"type":"unknown_event","payload":{}}`
	d.HandleWSMessage([]byte(msg))

	// No messages should be reported.
	msgs := client.getReportedMessages()
	if len(msgs) != 0 {
		t.Errorf("expected 0 reported messages, got %d", len(msgs))
	}
}

func TestHandleTaskInput_SequenceIncrement(t *testing.T) {
	client := &trackingClient{}
	d := testDaemonWithClient(client)

	pipe := &mockPipe{}
	taskID := "task-seq"
	d.stdinManager.Register(taskID, pipe)
	seq := 5 // Start at 5 to simulate existing stdout/stderr messages.
	d.sequences.Register(taskID, &seq)

	// Send two inputs.
	d.handleTaskInput(taskID, "first")
	d.handleTaskInput(taskID, "second")

	// Verify sequences are 6 and 7.
	msgs := client.getReportedMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 reported messages, got %d", len(msgs))
	}
	if msgs[0].Sequence != 6 {
		t.Errorf("expected first sequence 6, got %d", msgs[0].Sequence)
	}
	if msgs[1].Sequence != 7 {
		t.Errorf("expected second sequence 7, got %d", msgs[1].Sequence)
	}
}

func TestHandleTaskInput_ClosedPipe(t *testing.T) {
	client := &trackingClient{}
	d := testDaemonWithClient(client)

	// Register and then close the pipe.
	pipe := &mockPipe{}
	taskID := "task-closed"
	d.stdinManager.Register(taskID, pipe)
	d.stdinManager.Close(taskID)

	seq := 0
	d.sequences.Register(taskID, &seq)

	// Should fail gracefully since pipe is closed/removed.
	d.handleTaskInput(taskID, "hello")

	// Should report failure.
	msgs := client.getReportedMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 failure message, got %d", len(msgs))
	}
	if msgs[0].Stream != "stderr" {
		t.Errorf("expected stream %q for failure, got %q", "stderr", msgs[0].Stream)
	}
}

func TestSequenceTracker(t *testing.T) {
	tracker := newSequenceTracker()

	// Unregistered task returns 0.
	if got := tracker.Next("unknown"); got != 0 {
		t.Errorf("expected 0 for unregistered task, got %d", got)
	}

	// Register and increment.
	seq := 0
	tracker.Register("task-1", &seq)

	if got := tracker.Next("task-1"); got != 1 {
		t.Errorf("expected 1, got %d", got)
	}
	if got := tracker.Next("task-1"); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}

	// Verify the underlying counter was modified.
	if seq != 2 {
		t.Errorf("expected seq to be 2, got %d", seq)
	}

	// Remove and verify.
	tracker.Remove("task-1")
	if got := tracker.Next("task-1"); got != 0 {
		t.Errorf("expected 0 after removal, got %d", got)
	}
}

func mustMarshal(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
