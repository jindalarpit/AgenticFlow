package realtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// newTestClient creates a Client for testing with a buffered send channel.
func newTestClient(id, userID string, isDaemon bool) *Client {
	return &Client{
		ID:       id,
		UserID:   userID,
		IsDaemon: isDaemon,
		send:     make(chan []byte, 16),
	}
}

func TestSendToDaemon_TaskInput(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Register a daemon client.
	daemonClient := newTestClient("daemon-1", "user-1", true)
	hub.register <- daemonClient

	// Give the hub time to process the registration.
	time.Sleep(50 * time.Millisecond)

	// Send a task_input event to the daemon.
	event := Event{
		Type: EventTaskInput,
		Payload: map[string]interface{}{
			"task_id": "task-abc",
			"text":    "yes",
		},
	}
	hub.SendToDaemon("daemon-1", event)

	// Read the message from the daemon's send channel.
	select {
	case msg := <-daemonClient.send:
		var received Event
		if err := json.Unmarshal(msg, &received); err != nil {
			t.Fatalf("failed to unmarshal sent message: %v", err)
		}
		if received.Type != EventTaskInput {
			t.Errorf("expected event type %q, got %q", EventTaskInput, received.Type)
		}
		// Verify payload contains task_id and text.
		payload, ok := received.Payload.(map[string]interface{})
		if !ok {
			t.Fatalf("expected payload to be map, got %T", received.Payload)
		}
		if payload["task_id"] != "task-abc" {
			t.Errorf("expected task_id %q, got %v", "task-abc", payload["task_id"])
		}
		if payload["text"] != "yes" {
			t.Errorf("expected text %q, got %v", "yes", payload["text"])
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message on daemon send channel")
	}
}

func TestSendToDaemon_RoutesToCorrectDaemon(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Register two daemon clients.
	daemon1 := newTestClient("daemon-1", "user-1", true)
	daemon2 := newTestClient("daemon-2", "user-2", true)
	hub.register <- daemon1
	hub.register <- daemon2

	time.Sleep(50 * time.Millisecond)

	// Send a task_input event to daemon-2 only.
	event := Event{
		Type: EventTaskInput,
		Payload: map[string]interface{}{
			"task_id": "task-xyz",
			"text":    "hello daemon 2",
		},
	}
	hub.SendToDaemon("daemon-2", event)

	// daemon-2 should receive the message.
	select {
	case msg := <-daemon2.send:
		var received Event
		if err := json.Unmarshal(msg, &received); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if received.Type != EventTaskInput {
			t.Errorf("expected type %q, got %q", EventTaskInput, received.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("daemon-2 did not receive the message")
	}

	// daemon-1 should NOT receive any message.
	select {
	case msg := <-daemon1.send:
		t.Fatalf("daemon-1 should not have received a message, got: %s", msg)
	case <-time.After(100 * time.Millisecond):
		// Expected: no message for daemon-1.
	}
}

func TestSendToDaemon_NonExistentDaemon(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	time.Sleep(50 * time.Millisecond)

	// Sending to a non-existent daemon should not panic.
	event := Event{
		Type: EventTaskInput,
		Payload: map[string]interface{}{
			"task_id": "task-123",
			"text":    "hello",
		},
	}
	hub.SendToDaemon("non-existent-daemon", event)
	// No panic = success.
}

func TestIsDaemonConnected(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Initially no daemons connected.
	if hub.IsDaemonConnected("daemon-1") {
		t.Error("expected daemon-1 to not be connected initially")
	}

	// Register a daemon.
	daemonClient := newTestClient("daemon-1", "user-1", true)
	hub.register <- daemonClient
	time.Sleep(50 * time.Millisecond)

	// Now it should be connected.
	if !hub.IsDaemonConnected("daemon-1") {
		t.Error("expected daemon-1 to be connected after registration")
	}

	// Unregister the daemon.
	hub.unregister <- daemonClient
	time.Sleep(50 * time.Millisecond)

	// Should no longer be connected.
	if hub.IsDaemonConnected("daemon-1") {
		t.Error("expected daemon-1 to not be connected after unregistration")
	}
}

func TestSendToDaemon_DoesNotSendToUsers(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Register a user client (not a daemon).
	userClient := newTestClient("user-1", "user-1", false)
	hub.register <- userClient
	time.Sleep(50 * time.Millisecond)

	// SendToDaemon with the user's ID should not reach the user client.
	event := Event{
		Type: EventTaskInput,
		Payload: map[string]interface{}{
			"task_id": "task-123",
			"text":    "hello",
		},
	}
	hub.SendToDaemon("user-1", event)

	// User should NOT receive the message (SendToDaemon only looks in daemons map).
	select {
	case msg := <-userClient.send:
		t.Fatalf("user client should not receive SendToDaemon messages, got: %s", msg)
	case <-time.After(100 * time.Millisecond):
		// Expected: no message for user.
	}
}
