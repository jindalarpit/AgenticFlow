package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// --------------------------------------------------------------------------
// Property 6: Multi-connection fan-out
// For any user with N active WebSocket connections (1 ≤ N ≤ 5), when the Hub
// broadcasts an event to that user, all N connections SHALL receive the event.
// **Validates: Requirements 15.2, 15.3**
// --------------------------------------------------------------------------

func TestProperty_MultiConnectionFanOut(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(t, "numConnections")
		userID := fmt.Sprintf("user-%s", rapid.StringMatching(`[a-z0-9]{8}`).Draw(t, "userID"))
		eventType := rapid.SampledFrom([]string{
			EventTaskStarted, EventTaskCompleted, EventTaskFailed, EventTaskOutput,
		}).Draw(t, "eventType")

		hub := NewHub()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go hub.Run(ctx)

		// Register N connections for the same user.
		clients := make([]*Client, n)
		for i := 0; i < n; i++ {
			clients[i] = newTestClient(fmt.Sprintf("%s-conn-%d", userID, i), userID, false)
			hub.register <- clients[i]
		}
		// Wait for all registrations to be processed.
		time.Sleep(50 * time.Millisecond)

		// Send an event to the user.
		hub.SendToUser(userID, Event{
			Type:    eventType,
			Payload: map[string]string{"msg": "hello"},
		})

		// Verify all N connections receive the event.
		for i, client := range clients {
			select {
			case msg := <-client.send:
				var received Event
				if err := json.Unmarshal(msg, &received); err != nil {
					t.Fatalf("connection %d: failed to unmarshal: %v", i, err)
				}
				if received.Type != eventType {
					t.Fatalf("connection %d: expected event type %q, got %q", i, eventType, received.Type)
				}
			case <-time.After(time.Second):
				t.Fatalf("connection %d: timed out waiting for event", i)
			}
		}
	})
}

func TestProperty_MultiConnectionFanOut_Broadcast(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(t, "numConnections")
		userID := fmt.Sprintf("user-%s", rapid.StringMatching(`[a-z0-9]{8}`).Draw(t, "userID"))

		hub := NewHub()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go hub.Run(ctx)

		// Register N connections for the same user.
		clients := make([]*Client, n)
		for i := 0; i < n; i++ {
			clients[i] = newTestClient(fmt.Sprintf("%s-conn-%d", userID, i), userID, false)
			hub.register <- clients[i]
		}
		time.Sleep(50 * time.Millisecond)

		// Broadcast an event to all users.
		hub.Broadcast(Event{
			Type:    "test_broadcast",
			Payload: map[string]string{"data": "broadcast_msg"},
		})

		// Wait for broadcast to be processed.
		time.Sleep(50 * time.Millisecond)

		// Verify all N connections receive the broadcast.
		for i, client := range clients {
			select {
			case msg := <-client.send:
				var received Event
				if err := json.Unmarshal(msg, &received); err != nil {
					t.Fatalf("connection %d: failed to unmarshal: %v", i, err)
				}
				if received.Type != "test_broadcast" {
					t.Fatalf("connection %d: expected event type %q, got %q", i, "test_broadcast", received.Type)
				}
			case <-time.After(time.Second):
				t.Fatalf("connection %d: timed out waiting for broadcast", i)
			}
		}
	})
}

// --------------------------------------------------------------------------
// Property 7: Connection limit enforcement
// For any user attempting to open more than 5 concurrent WebSocket connections,
// the Hub SHALL maintain at most 5 active connections for that user.
// **Validates: Requirements 15.5**
// --------------------------------------------------------------------------

func TestProperty_ConnectionLimitEnforcement(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate between 6 and 10 connection attempts.
		numAttempts := rapid.IntRange(6, 10).Draw(t, "numAttempts")
		userID := fmt.Sprintf("user-%s", rapid.StringMatching(`[a-z0-9]{8}`).Draw(t, "userID"))

		hub := NewHub()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go hub.Run(ctx)

		// Register more than maxConnectionsPerUser connections.
		clients := make([]*Client, numAttempts)
		for i := 0; i < numAttempts; i++ {
			clients[i] = newTestClient(fmt.Sprintf("%s-conn-%d", userID, i), userID, false)
			hub.register <- clients[i]
		}
		// Wait for all registrations to be processed.
		time.Sleep(100 * time.Millisecond)

		// Verify the hub maintains at most maxConnectionsPerUser active connections.
		hub.mu.RLock()
		activeConns := len(hub.clients[userID])
		hub.mu.RUnlock()

		if activeConns > maxConnectionsPerUser {
			t.Fatalf("expected at most %d active connections, got %d", maxConnectionsPerUser, activeConns)
		}
		if activeConns != maxConnectionsPerUser {
			t.Fatalf("expected exactly %d active connections (limit reached), got %d", maxConnectionsPerUser, activeConns)
		}

		// Verify the most recent 5 connections are the ones kept (oldest evicted).
		hub.mu.RLock()
		conns := hub.clients[userID]
		hub.mu.RUnlock()

		// The last maxConnectionsPerUser clients should be the active ones.
		expectedStart := numAttempts - maxConnectionsPerUser
		for i, c := range conns {
			expectedID := fmt.Sprintf("%s-conn-%d", userID, expectedStart+i)
			if c.ID != expectedID {
				t.Fatalf("connection %d: expected ID %q, got %q", i, expectedID, c.ID)
			}
		}
	})
}

func TestProperty_ConnectionLimitEnforcement_EventDelivery(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate between 6 and 10 connection attempts.
		numAttempts := rapid.IntRange(6, 10).Draw(t, "numAttempts")
		userID := fmt.Sprintf("user-%s", rapid.StringMatching(`[a-z0-9]{8}`).Draw(t, "userID"))

		hub := NewHub()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go hub.Run(ctx)

		// Register more than maxConnectionsPerUser connections.
		clients := make([]*Client, numAttempts)
		for i := 0; i < numAttempts; i++ {
			clients[i] = newTestClient(fmt.Sprintf("%s-conn-%d", userID, i), userID, false)
			hub.register <- clients[i]
		}
		time.Sleep(100 * time.Millisecond)

		// Send an event to the user.
		hub.SendToUser(userID, Event{
			Type:    "test_event",
			Payload: map[string]string{"key": "value"},
		})

		// Only the last maxConnectionsPerUser clients should receive the event.
		expectedStart := numAttempts - maxConnectionsPerUser
		for i := expectedStart; i < numAttempts; i++ {
			select {
			case msg := <-clients[i].send:
				var received Event
				if err := json.Unmarshal(msg, &received); err != nil {
					t.Fatalf("client %d: failed to unmarshal: %v", i, err)
				}
				if received.Type != "test_event" {
					t.Fatalf("client %d: expected type %q, got %q", i, "test_event", received.Type)
				}
			case <-time.After(time.Second):
				t.Fatalf("client %d: timed out waiting for event", i)
			}
		}
	})
}

// --------------------------------------------------------------------------
// Property 8: Selective connection removal
// For any user with N active connections (N > 1), closing one specific connection
// SHALL leave exactly N-1 connections active, and those remaining connections
// SHALL continue to receive broadcast events.
// **Validates: Requirements 15.4**
// --------------------------------------------------------------------------

func TestProperty_SelectiveConnectionRemoval(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(2, 5).Draw(t, "numConnections")
		removeIdx := rapid.IntRange(0, n-1).Draw(t, "removeIndex")
		userID := fmt.Sprintf("user-%s", rapid.StringMatching(`[a-z0-9]{8}`).Draw(t, "userID"))

		hub := NewHub()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go hub.Run(ctx)

		// Register N connections for the same user.
		clients := make([]*Client, n)
		for i := 0; i < n; i++ {
			clients[i] = newTestClient(fmt.Sprintf("%s-conn-%d", userID, i), userID, false)
			hub.register <- clients[i]
		}
		time.Sleep(50 * time.Millisecond)

		// Unregister one specific connection.
		hub.unregister <- clients[removeIdx]
		time.Sleep(50 * time.Millisecond)

		// Verify N-1 connections remain.
		hub.mu.RLock()
		activeConns := len(hub.clients[userID])
		hub.mu.RUnlock()

		if activeConns != n-1 {
			t.Fatalf("expected %d active connections after removal, got %d", n-1, activeConns)
		}

		// Send an event to the user and verify remaining connections receive it.
		hub.SendToUser(userID, Event{
			Type:    "post_removal_event",
			Payload: map[string]string{"check": "alive"},
		})

		for i, client := range clients {
			if i == removeIdx {
				// The removed client's send channel was closed by the hub.
				// We should NOT receive new messages on it (channel is closed).
				continue
			}
			select {
			case msg := <-client.send:
				var received Event
				if err := json.Unmarshal(msg, &received); err != nil {
					t.Fatalf("connection %d: failed to unmarshal: %v", i, err)
				}
				if received.Type != "post_removal_event" {
					t.Fatalf("connection %d: expected type %q, got %q", i, "post_removal_event", received.Type)
				}
			case <-time.After(time.Second):
				t.Fatalf("connection %d: timed out waiting for event after removal", i)
			}
		}
	})
}

func TestProperty_SelectiveConnectionRemoval_OthersUnaffected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(2, 5).Draw(t, "numConnections")
		removeIdx := rapid.IntRange(0, n-1).Draw(t, "removeIndex")
		userID := fmt.Sprintf("user-%s", rapid.StringMatching(`[a-z0-9]{8}`).Draw(t, "userID"))

		hub := NewHub()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go hub.Run(ctx)

		// Register N connections.
		clients := make([]*Client, n)
		for i := 0; i < n; i++ {
			clients[i] = newTestClient(fmt.Sprintf("%s-conn-%d", userID, i), userID, false)
			hub.register <- clients[i]
		}
		time.Sleep(50 * time.Millisecond)

		// Unregister one connection.
		hub.unregister <- clients[removeIdx]
		time.Sleep(50 * time.Millisecond)

		// Verify the remaining connections are exactly the ones we expect.
		hub.mu.RLock()
		conns := hub.clients[userID]
		hub.mu.RUnlock()

		// Build expected set of remaining client IDs.
		expectedIDs := make(map[string]bool)
		for i := 0; i < n; i++ {
			if i != removeIdx {
				expectedIDs[clients[i].ID] = true
			}
		}

		if len(conns) != len(expectedIDs) {
			t.Fatalf("expected %d remaining connections, got %d", len(expectedIDs), len(conns))
		}

		for _, c := range conns {
			if !expectedIDs[c.ID] {
				t.Fatalf("unexpected connection %q in remaining list", c.ID)
			}
		}
	})
}

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
