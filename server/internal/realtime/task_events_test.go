package realtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestBroadcastTaskStarted_IncludesDeliverableType(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Register a user client.
	client := testClient("user-task-1", false)
	client.hub = hub
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast task_started event with deliverable_type.
	hub.BroadcastTaskStarted("task-100", "daemon-1", "plan")
	time.Sleep(10 * time.Millisecond)

	// Verify the client received the event with correct type and payload.
	select {
	case msg := <-client.send:
		var event struct {
			Type    string             `json:"type"`
			Payload TaskStartedPayload `json:"payload"`
		}
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventTaskStarted {
			t.Errorf("expected event type %q, got %q", EventTaskStarted, event.Type)
		}
		if event.Payload.TaskID != "task-100" {
			t.Errorf("expected task_id %q, got %q", "task-100", event.Payload.TaskID)
		}
		if event.Payload.DaemonID != "daemon-1" {
			t.Errorf("expected daemon_id %q, got %q", "daemon-1", event.Payload.DaemonID)
		}
		if event.Payload.DeliverableType != "plan" {
			t.Errorf("expected deliverable_type %q, got %q", "plan", event.Payload.DeliverableType)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for task_started event")
	}
}

func TestBroadcastTaskStarted_WithoutDeliverableType(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := testClient("user-task-1b", false)
	client.hub = hub
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast task_started without deliverable_type (single-pass task).
	hub.BroadcastTaskStarted("task-101", "daemon-2", "")
	time.Sleep(10 * time.Millisecond)

	select {
	case msg := <-client.send:
		var event struct {
			Type    string             `json:"type"`
			Payload TaskStartedPayload `json:"payload"`
		}
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventTaskStarted {
			t.Errorf("expected event type %q, got %q", EventTaskStarted, event.Type)
		}
		if event.Payload.TaskID != "task-101" {
			t.Errorf("expected task_id %q, got %q", "task-101", event.Payload.TaskID)
		}
		if event.Payload.DeliverableType != "" {
			t.Errorf("expected empty deliverable_type, got %q", event.Payload.DeliverableType)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for task_started event")
	}
}

func TestBroadcastTaskCompleted_IncludesOutputContent(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := testClient("user-task-2", false)
	client.hub = hub
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast task_completed event with deliverable_type and output_content.
	hub.BroadcastTaskCompleted("task-200", 0, "design", "# Design Document\n\nThis is the design output.")
	time.Sleep(10 * time.Millisecond)

	select {
	case msg := <-client.send:
		var event struct {
			Type    string               `json:"type"`
			Payload TaskCompletedPayload `json:"payload"`
		}
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventTaskCompleted {
			t.Errorf("expected event type %q, got %q", EventTaskCompleted, event.Type)
		}
		if event.Payload.TaskID != "task-200" {
			t.Errorf("expected task_id %q, got %q", "task-200", event.Payload.TaskID)
		}
		if event.Payload.ExitCode != 0 {
			t.Errorf("expected exit_code 0, got %d", event.Payload.ExitCode)
		}
		if event.Payload.DeliverableType != "design" {
			t.Errorf("expected deliverable_type %q, got %q", "design", event.Payload.DeliverableType)
		}
		if event.Payload.OutputContent != "# Design Document\n\nThis is the design output." {
			t.Errorf("expected output_content %q, got %q", "# Design Document\n\nThis is the design output.", event.Payload.OutputContent)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for task_completed event")
	}
}

func TestBroadcastTaskCompleted_WithoutConversationalFields(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := testClient("user-task-2b", false)
	client.hub = hub
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast task_completed without conversational fields (single-pass task).
	hub.BroadcastTaskCompleted("task-201", 0, "", "")
	time.Sleep(10 * time.Millisecond)

	select {
	case msg := <-client.send:
		var event struct {
			Type    string               `json:"type"`
			Payload TaskCompletedPayload `json:"payload"`
		}
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventTaskCompleted {
			t.Errorf("expected event type %q, got %q", EventTaskCompleted, event.Type)
		}
		if event.Payload.TaskID != "task-201" {
			t.Errorf("expected task_id %q, got %q", "task-201", event.Payload.TaskID)
		}
		if event.Payload.DeliverableType != "" {
			t.Errorf("expected empty deliverable_type, got %q", event.Payload.DeliverableType)
		}
		if event.Payload.OutputContent != "" {
			t.Errorf("expected empty output_content, got %q", event.Payload.OutputContent)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for task_completed event")
	}
}

func TestBroadcastTaskFailed_IncludesErrorInfo(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := testClient("user-task-3", false)
	client.hub = hub
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast task_failed event with error info and deliverable_type.
	hub.BroadcastTaskFailed("task-300", 1, "agent process exited with code 1", "execution")
	time.Sleep(10 * time.Millisecond)

	select {
	case msg := <-client.send:
		var event struct {
			Type    string            `json:"type"`
			Payload TaskFailedPayload `json:"payload"`
		}
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventTaskFailed {
			t.Errorf("expected event type %q, got %q", EventTaskFailed, event.Type)
		}
		if event.Payload.TaskID != "task-300" {
			t.Errorf("expected task_id %q, got %q", "task-300", event.Payload.TaskID)
		}
		if event.Payload.ExitCode != 1 {
			t.Errorf("expected exit_code 1, got %d", event.Payload.ExitCode)
		}
		if event.Payload.ErrorMessage != "agent process exited with code 1" {
			t.Errorf("expected error_message %q, got %q", "agent process exited with code 1", event.Payload.ErrorMessage)
		}
		if event.Payload.DeliverableType != "execution" {
			t.Errorf("expected deliverable_type %q, got %q", "execution", event.Payload.DeliverableType)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for task_failed event")
	}
}

func TestBroadcastTaskFailed_WithoutDeliverableType(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := testClient("user-task-3b", false)
	client.hub = hub
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast task_failed without deliverable_type (single-pass task).
	hub.BroadcastTaskFailed("task-301", 127, "command not found", "")
	time.Sleep(10 * time.Millisecond)

	select {
	case msg := <-client.send:
		var event struct {
			Type    string            `json:"type"`
			Payload TaskFailedPayload `json:"payload"`
		}
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventTaskFailed {
			t.Errorf("expected event type %q, got %q", EventTaskFailed, event.Type)
		}
		if event.Payload.TaskID != "task-301" {
			t.Errorf("expected task_id %q, got %q", "task-301", event.Payload.TaskID)
		}
		if event.Payload.ExitCode != 127 {
			t.Errorf("expected exit_code 127, got %d", event.Payload.ExitCode)
		}
		if event.Payload.ErrorMessage != "command not found" {
			t.Errorf("expected error_message %q, got %q", "command not found", event.Payload.ErrorMessage)
		}
		if event.Payload.DeliverableType != "" {
			t.Errorf("expected empty deliverable_type, got %q", event.Payload.DeliverableType)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for task_failed event")
	}
}

func TestBroadcastTaskOutput_StreamingEvents(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := testClient("user-task-4", false)
	client.hub = hub
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast multiple task_output events simulating streaming.
	hub.BroadcastTaskOutput("task-400", "stdout", "Analyzing code...\n", 1)
	hub.BroadcastTaskOutput("task-400", "stdout", "Found 3 issues.\n", 2)
	hub.BroadcastTaskOutput("task-400", "stderr", "Warning: deprecated API\n", 3)
	time.Sleep(10 * time.Millisecond)

	// Verify all three events are received in order.
	expected := []struct {
		stream   string
		content  string
		sequence int
	}{
		{"stdout", "Analyzing code...\n", 1},
		{"stdout", "Found 3 issues.\n", 2},
		{"stderr", "Warning: deprecated API\n", 3},
	}

	for i, exp := range expected {
		select {
		case msg := <-client.send:
			var event struct {
				Type    string            `json:"type"`
				Payload TaskOutputPayload `json:"payload"`
			}
			if err := json.Unmarshal(msg, &event); err != nil {
				t.Fatalf("event %d: failed to unmarshal: %v", i, err)
			}
			if event.Type != EventTaskOutput {
				t.Errorf("event %d: expected type %q, got %q", i, EventTaskOutput, event.Type)
			}
			if event.Payload.TaskID != "task-400" {
				t.Errorf("event %d: expected task_id %q, got %q", i, "task-400", event.Payload.TaskID)
			}
			if event.Payload.Stream != exp.stream {
				t.Errorf("event %d: expected stream %q, got %q", i, exp.stream, event.Payload.Stream)
			}
			if event.Payload.Content != exp.content {
				t.Errorf("event %d: expected content %q, got %q", i, exp.content, event.Payload.Content)
			}
			if event.Payload.Sequence != exp.sequence {
				t.Errorf("event %d: expected sequence %d, got %d", i, exp.sequence, event.Payload.Sequence)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("event %d: timeout waiting for task_output event", i)
		}
	}
}

func TestBroadcastTaskOutput_SingleEvent(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := testClient("user-task-4b", false)
	client.hub = hub
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast a single task_output event.
	hub.BroadcastTaskOutput("task-401", "stdout", "Hello world\n", 0)
	time.Sleep(10 * time.Millisecond)

	select {
	case msg := <-client.send:
		var event struct {
			Type    string            `json:"type"`
			Payload TaskOutputPayload `json:"payload"`
		}
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventTaskOutput {
			t.Errorf("expected event type %q, got %q", EventTaskOutput, event.Type)
		}
		if event.Payload.TaskID != "task-401" {
			t.Errorf("expected task_id %q, got %q", "task-401", event.Payload.TaskID)
		}
		if event.Payload.Stream != "stdout" {
			t.Errorf("expected stream %q, got %q", "stdout", event.Payload.Stream)
		}
		if event.Payload.Content != "Hello world\n" {
			t.Errorf("expected content %q, got %q", "Hello world\n", event.Payload.Content)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for task_output event")
	}
}

func TestBroadcastTaskEvents_ReachesDaemons(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Register a daemon client.
	daemon := testClient("daemon-task-1", true)
	daemon.hub = hub
	hub.register <- daemon
	time.Sleep(10 * time.Millisecond)

	// Broadcast task_started — should reach daemon too.
	hub.BroadcastTaskStarted("task-500", "daemon-task-1", "tasks")
	time.Sleep(10 * time.Millisecond)

	select {
	case msg := <-daemon.send:
		var event struct {
			Type    string             `json:"type"`
			Payload TaskStartedPayload `json:"payload"`
		}
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventTaskStarted {
			t.Errorf("expected event type %q, got %q", EventTaskStarted, event.Type)
		}
		if event.Payload.DeliverableType != "tasks" {
			t.Errorf("expected deliverable_type %q, got %q", "tasks", event.Payload.DeliverableType)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for task_started event on daemon")
	}
}

func TestBroadcastTaskEvents_ReachesBothUsersAndDaemons(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Register both a user and a daemon.
	user := testClient("user-task-5", false)
	user.hub = hub
	hub.register <- user

	daemon := testClient("daemon-task-2", true)
	daemon.hub = hub
	hub.register <- daemon
	time.Sleep(10 * time.Millisecond)

	// Broadcast task_completed.
	hub.BroadcastTaskCompleted("task-600", 0, "plan", "# Plan\n\nStep 1...")
	time.Sleep(10 * time.Millisecond)

	// Both should receive the event.
	for _, c := range []*Client{user, daemon} {
		select {
		case msg := <-c.send:
			var event struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(msg, &event); err != nil {
				t.Fatalf("failed to unmarshal event for %s: %v", c.ID, err)
			}
			if event.Type != EventTaskCompleted {
				t.Errorf("client %s: expected event type %q, got %q", c.ID, EventTaskCompleted, event.Type)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timeout waiting for task_completed event on client %s", c.ID)
		}
	}
}

func TestBroadcastTaskEvents_NoClientsNoPanic(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// These should not panic even with no clients connected.
	hub.BroadcastTaskStarted("task-x", "daemon-x", "plan")
	hub.BroadcastTaskCompleted("task-x", 0, "design", "output")
	hub.BroadcastTaskFailed("task-x", 1, "error", "execution")
	hub.BroadcastTaskOutput("task-x", "stdout", "content", 1)
}

func TestTaskStartedPayload_JSONSerialization(t *testing.T) {
	payload := TaskStartedPayload{
		TaskID:          "task-json-1",
		DaemonID:        "daemon-json-1",
		DeliverableType: "design",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal TaskStartedPayload: %v", err)
	}

	var decoded TaskStartedPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal TaskStartedPayload: %v", err)
	}

	if decoded.TaskID != "task-json-1" {
		t.Errorf("expected task_id %q, got %q", "task-json-1", decoded.TaskID)
	}
	if decoded.DaemonID != "daemon-json-1" {
		t.Errorf("expected daemon_id %q, got %q", "daemon-json-1", decoded.DaemonID)
	}
	if decoded.DeliverableType != "design" {
		t.Errorf("expected deliverable_type %q, got %q", "design", decoded.DeliverableType)
	}
}

func TestTaskStartedPayload_OmitsEmptyDeliverableType(t *testing.T) {
	payload := TaskStartedPayload{
		TaskID:   "task-json-2",
		DaemonID: "daemon-json-2",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal TaskStartedPayload: %v", err)
	}

	// Verify deliverable_type is omitted from JSON when empty.
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}
	if _, exists := raw["deliverable_type"]; exists {
		t.Error("expected deliverable_type to be omitted from JSON when empty")
	}
}

func TestTaskCompletedPayload_JSONSerialization(t *testing.T) {
	payload := TaskCompletedPayload{
		TaskID:          "task-json-3",
		ExitCode:        0,
		DeliverableType: "tasks",
		OutputContent:   "## Task List\n\n1. Implement feature A\n2. Write tests",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal TaskCompletedPayload: %v", err)
	}

	var decoded TaskCompletedPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal TaskCompletedPayload: %v", err)
	}

	if decoded.TaskID != "task-json-3" {
		t.Errorf("expected task_id %q, got %q", "task-json-3", decoded.TaskID)
	}
	if decoded.ExitCode != 0 {
		t.Errorf("expected exit_code 0, got %d", decoded.ExitCode)
	}
	if decoded.DeliverableType != "tasks" {
		t.Errorf("expected deliverable_type %q, got %q", "tasks", decoded.DeliverableType)
	}
	if decoded.OutputContent != "## Task List\n\n1. Implement feature A\n2. Write tests" {
		t.Errorf("expected output_content %q, got %q", "## Task List\n\n1. Implement feature A\n2. Write tests", decoded.OutputContent)
	}
}

func TestTaskCompletedPayload_OmitsEmptyFields(t *testing.T) {
	payload := TaskCompletedPayload{
		TaskID:   "task-json-4",
		ExitCode: 0,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal TaskCompletedPayload: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}
	if _, exists := raw["deliverable_type"]; exists {
		t.Error("expected deliverable_type to be omitted from JSON when empty")
	}
	if _, exists := raw["output_content"]; exists {
		t.Error("expected output_content to be omitted from JSON when empty")
	}
}

func TestTaskFailedPayload_JSONSerialization(t *testing.T) {
	payload := TaskFailedPayload{
		TaskID:          "task-json-5",
		ExitCode:        1,
		ErrorMessage:    "execution timed out after 5m",
		DeliverableType: "execution",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal TaskFailedPayload: %v", err)
	}

	var decoded TaskFailedPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal TaskFailedPayload: %v", err)
	}

	if decoded.TaskID != "task-json-5" {
		t.Errorf("expected task_id %q, got %q", "task-json-5", decoded.TaskID)
	}
	if decoded.ExitCode != 1 {
		t.Errorf("expected exit_code 1, got %d", decoded.ExitCode)
	}
	if decoded.ErrorMessage != "execution timed out after 5m" {
		t.Errorf("expected error_message %q, got %q", "execution timed out after 5m", decoded.ErrorMessage)
	}
	if decoded.DeliverableType != "execution" {
		t.Errorf("expected deliverable_type %q, got %q", "execution", decoded.DeliverableType)
	}
}

func TestTaskFailedPayload_OmitsEmptyDeliverableType(t *testing.T) {
	payload := TaskFailedPayload{
		TaskID:       "task-json-6",
		ExitCode:     1,
		ErrorMessage: "some error",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal TaskFailedPayload: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}
	if _, exists := raw["deliverable_type"]; exists {
		t.Error("expected deliverable_type to be omitted from JSON when empty")
	}
}

func TestTaskOutputPayload_JSONSerialization(t *testing.T) {
	payload := TaskOutputPayload{
		TaskID:   "task-json-7",
		Stream:   "stdout",
		Content:  "Processing file main.go...\n",
		Sequence: 42,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal TaskOutputPayload: %v", err)
	}

	var decoded TaskOutputPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal TaskOutputPayload: %v", err)
	}

	if decoded.TaskID != "task-json-7" {
		t.Errorf("expected task_id %q, got %q", "task-json-7", decoded.TaskID)
	}
	if decoded.Stream != "stdout" {
		t.Errorf("expected stream %q, got %q", "stdout", decoded.Stream)
	}
	if decoded.Content != "Processing file main.go...\n" {
		t.Errorf("expected content %q, got %q", "Processing file main.go...\n", decoded.Content)
	}
	if decoded.Sequence != 42 {
		t.Errorf("expected sequence 42, got %d", decoded.Sequence)
	}
}

func TestTaskOutputPayload_OmitsZeroSequence(t *testing.T) {
	payload := TaskOutputPayload{
		TaskID:  "task-json-8",
		Stream:  "stderr",
		Content: "error output",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal TaskOutputPayload: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}
	if _, exists := raw["sequence"]; exists {
		t.Error("expected sequence to be omitted from JSON when zero")
	}
}
