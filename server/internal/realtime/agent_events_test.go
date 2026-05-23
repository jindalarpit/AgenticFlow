package realtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// testClient creates a minimal Client for testing with a buffered send channel.
func testClient(userID string, isDaemon bool) *Client {
	return &Client{
		ID:       userID,
		UserID:   userID,
		IsDaemon: isDaemon,
		send:     make(chan []byte, 16),
	}
}

func TestBroadcastAgentCreated(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Register a user client.
	client := testClient("user-1", false)
	client.hub = hub
	hub.register <- client
	time.Sleep(10 * time.Millisecond) // let the hub process

	// Broadcast agent_created event.
	payload := map[string]string{"id": "agent-123", "name": "Nexus"}
	hub.BroadcastAgentCreated(payload)
	time.Sleep(10 * time.Millisecond)

	// Verify the client received the event.
	select {
	case msg := <-client.send:
		var event Event
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventAgentCreated {
			t.Errorf("expected event type %q, got %q", EventAgentCreated, event.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for agent_created event")
	}
}

func TestBroadcastAgentUpdated(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := testClient("user-2", false)
	client.hub = hub
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	payload := map[string]string{"id": "agent-456", "name": "Updated"}
	hub.BroadcastAgentUpdated(payload)
	time.Sleep(10 * time.Millisecond)

	select {
	case msg := <-client.send:
		var event Event
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventAgentUpdated {
			t.Errorf("expected event type %q, got %q", EventAgentUpdated, event.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for agent_updated event")
	}
}

func TestBroadcastAgentDeleted(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := testClient("user-3", false)
	client.hub = hub
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastAgentDeleted("agent-789")
	time.Sleep(10 * time.Millisecond)

	select {
	case msg := <-client.send:
		var event Event
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventAgentDeleted {
			t.Errorf("expected event type %q, got %q", EventAgentDeleted, event.Type)
		}
		// Verify payload contains agent_id.
		payloadMap, ok := event.Payload.(map[string]interface{})
		if !ok {
			t.Fatal("payload is not a map")
		}
		if payloadMap["agent_id"] != "agent-789" {
			t.Errorf("expected agent_id %q, got %v", "agent-789", payloadMap["agent_id"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for agent_deleted event")
	}
}

func TestBroadcastAgentStatusChanged(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := testClient("user-4", false)
	client.hub = hub
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastAgentStatusChanged("agent-abc", "working")
	time.Sleep(10 * time.Millisecond)

	select {
	case msg := <-client.send:
		var event struct {
			Type    string                    `json:"type"`
			Payload AgentStatusChangedPayload `json:"payload"`
		}
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventAgentStatusChanged {
			t.Errorf("expected event type %q, got %q", EventAgentStatusChanged, event.Type)
		}
		if event.Payload.AgentID != "agent-abc" {
			t.Errorf("expected agent_id %q, got %q", "agent-abc", event.Payload.AgentID)
		}
		if event.Payload.Status != "working" {
			t.Errorf("expected status %q, got %q", "working", event.Payload.Status)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for agent_status_changed event")
	}
}

func TestBroadcastAgentEvents_ReachesDaemons(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Register a daemon client.
	daemon := testClient("daemon-1", true)
	daemon.hub = hub
	hub.register <- daemon
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastAgentStatusChanged("agent-xyz", "offline")
	time.Sleep(10 * time.Millisecond)

	select {
	case msg := <-daemon.send:
		var event struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != EventAgentStatusChanged {
			t.Errorf("expected event type %q, got %q", EventAgentStatusChanged, event.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for agent_status_changed event on daemon")
	}
}

func TestBroadcastAgentEvents_NoClientsNoPanic(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// These should not panic even with no clients.
	hub.BroadcastAgentCreated(map[string]string{"id": "test"})
	hub.BroadcastAgentUpdated(map[string]string{"id": "test"})
	hub.BroadcastAgentDeleted("test")
	hub.BroadcastAgentStatusChanged("test", "idle")
}
