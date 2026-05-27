package realtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestBroadcastStageAwaitingApproval(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Register a user client.
	client := testClient("user-stage-1", false)
	client.hub = hub
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast stage_awaiting_approval event.
	hub.BroadcastStageAwaitingApproval("task-001", "design", "# Design Document\n\nThis is the design output.")
	time.Sleep(10 * time.Millisecond)

	// Verify the client received the event with correct type and payload.
	select {
	case msg := <-client.send:
		var event struct {
			Type    string                       `json:"type"`
			Payload StageAwaitingApprovalPayload `json:"payload"`
		}
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventStageAwaitingApproval {
			t.Errorf("expected event type %q, got %q", EventStageAwaitingApproval, event.Type)
		}
		if event.Payload.TaskID != "task-001" {
			t.Errorf("expected task_id %q, got %q", "task-001", event.Payload.TaskID)
		}
		if event.Payload.StageName != "design" {
			t.Errorf("expected stage_name %q, got %q", "design", event.Payload.StageName)
		}
		if event.Payload.OutputContent != "# Design Document\n\nThis is the design output." {
			t.Errorf("expected output_content %q, got %q", "# Design Document\n\nThis is the design output.", event.Payload.OutputContent)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for stage_awaiting_approval event")
	}
}

func TestBroadcastStageApproved(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Register a user client.
	client := testClient("user-stage-2", false)
	client.hub = hub
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast stage_approved event.
	hub.BroadcastStageApproved("task-002", "plan")
	time.Sleep(10 * time.Millisecond)

	// Verify the client received the event with correct type and payload.
	select {
	case msg := <-client.send:
		var event struct {
			Type    string               `json:"type"`
			Payload StageApprovedPayload `json:"payload"`
		}
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventStageApproved {
			t.Errorf("expected event type %q, got %q", EventStageApproved, event.Type)
		}
		if event.Payload.TaskID != "task-002" {
			t.Errorf("expected task_id %q, got %q", "task-002", event.Payload.TaskID)
		}
		if event.Payload.StageName != "plan" {
			t.Errorf("expected stage_name %q, got %q", "plan", event.Payload.StageName)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for stage_approved event")
	}
}

func TestBroadcastStageRejected(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Register a user client.
	client := testClient("user-stage-3", false)
	client.hub = hub
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast stage_rejected event with feedback.
	hub.BroadcastStageRejected("task-003", "tasks", "Please add more detail to the implementation steps")
	time.Sleep(10 * time.Millisecond)

	// Verify the client received the event with correct type and payload including feedback.
	select {
	case msg := <-client.send:
		var event struct {
			Type    string                `json:"type"`
			Payload StageRejectedPayload `json:"payload"`
		}
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventStageRejected {
			t.Errorf("expected event type %q, got %q", EventStageRejected, event.Type)
		}
		if event.Payload.TaskID != "task-003" {
			t.Errorf("expected task_id %q, got %q", "task-003", event.Payload.TaskID)
		}
		if event.Payload.StageName != "tasks" {
			t.Errorf("expected stage_name %q, got %q", "tasks", event.Payload.StageName)
		}
		if event.Payload.Feedback != "Please add more detail to the implementation steps" {
			t.Errorf("expected feedback %q, got %q", "Please add more detail to the implementation steps", event.Payload.Feedback)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for stage_rejected event")
	}
}

func TestBroadcastStageStarted(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Register a user client.
	client := testClient("user-stage-4", false)
	client.hub = hub
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast stage_started event.
	hub.BroadcastStageStarted("task-004", "execution")
	time.Sleep(10 * time.Millisecond)

	// Verify the client received the event with correct type and payload.
	select {
	case msg := <-client.send:
		var event struct {
			Type    string              `json:"type"`
			Payload StageStartedPayload `json:"payload"`
		}
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventStageStarted {
			t.Errorf("expected event type %q, got %q", EventStageStarted, event.Type)
		}
		if event.Payload.TaskID != "task-004" {
			t.Errorf("expected task_id %q, got %q", "task-004", event.Payload.TaskID)
		}
		if event.Payload.StageName != "execution" {
			t.Errorf("expected stage_name %q, got %q", "execution", event.Payload.StageName)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for stage_started event")
	}
}

func TestBroadcastStageEvents_ReachesDaemons(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Register a daemon client.
	daemon := testClient("daemon-stage-1", true)
	daemon.hub = hub
	hub.register <- daemon
	time.Sleep(10 * time.Millisecond)

	// Broadcast stage_awaiting_approval — should reach daemon too.
	hub.BroadcastStageAwaitingApproval("task-005", "plan", "Plan content")
	time.Sleep(10 * time.Millisecond)

	select {
	case msg := <-daemon.send:
		var event struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventStageAwaitingApproval {
			t.Errorf("expected event type %q, got %q", EventStageAwaitingApproval, event.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for stage event on daemon")
	}
}

func TestBroadcastStageEvents_NoClientsNoPanic(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// These should not panic even with no clients connected.
	hub.BroadcastStageAwaitingApproval("task-x", "plan", "output")
	hub.BroadcastStageApproved("task-x", "plan")
	hub.BroadcastStageRejected("task-x", "plan", "feedback")
	hub.BroadcastStageStarted("task-x", "plan")
}
