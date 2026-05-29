package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/agenticflow/agenticflow/shared/api"
	"github.com/gorilla/websocket"
)

// TestClient_ConnectsWithSecWebSocketProtocol verifies that the WS client
// sends the PAT token via Sec-WebSocket-Protocol header (not query params).
func TestClient_ConnectsWithSecWebSocketProtocol(t *testing.T) {
	var receivedProtocol string
	var mu sync.Mutex

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
		Subprotocols: []string{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the sub-protocols from the request.
		protocols := websocket.Subprotocols(r)
		mu.Lock()
		if len(protocols) > 0 {
			receivedProtocol = protocols[0]
		}
		mu.Unlock()

		// Echo back the sub-protocol to complete handshake.
		upgrader.Subprotocols = protocols
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Keep connection open briefly.
		time.Sleep(100 * time.Millisecond)
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}))
	defer server.Close()

	token := "af_test_token_12345"
	handler := func(event api.WebSocketEvent) {}
	onState := func(connected bool) {}

	client := NewClient(server.URL, token, "daemon-123", handler, onState)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Run connect in background (it will reconnect loop until ctx cancelled).
	go client.Connect(ctx)

	// Wait for connection to be established.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	proto := receivedProtocol
	mu.Unlock()

	expected := "access_token." + token
	if proto != expected {
		t.Errorf("expected Sec-WebSocket-Protocol %q, got %q", expected, proto)
	}
}

// TestClient_ReceivesTaskAvailableEvent verifies that the client dispatches
// task_available events to the handler.
func TestClient_ReceivesTaskAvailableEvent(t *testing.T) {
	var receivedEvent api.WebSocketEvent
	var mu sync.Mutex
	eventReceived := make(chan struct{}, 1)

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		protocols := websocket.Subprotocols(r)
		upgrader.Subprotocols = protocols
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Send a task_available event.
		event := api.WebSocketEvent{
			Type:    "task_available",
			Payload: map[string]string{"task_id": "task-abc-123"},
		}
		data, _ := json.Marshal(event)
		conn.WriteMessage(websocket.TextMessage, data)

		// Keep connection open briefly.
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	handler := func(event api.WebSocketEvent) {
		mu.Lock()
		receivedEvent = event
		mu.Unlock()
		select {
		case eventReceived <- struct{}{}:
		default:
		}
	}
	onState := func(connected bool) {}

	client := NewClient(server.URL, "af_test_token", "daemon-123", handler, onState)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go client.Connect(ctx)

	// Wait for the event to be received.
	select {
	case <-eventReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for task_available event")
	}

	mu.Lock()
	defer mu.Unlock()

	if receivedEvent.Type != "task_available" {
		t.Errorf("expected event type 'task_available', got %q", receivedEvent.Type)
	}
}

// TestClient_ConnectionStateCallbacks verifies that the onState callback
// is called with true on connect and false on disconnect.
func TestClient_ConnectionStateCallbacks(t *testing.T) {
	var states []bool
	var mu sync.Mutex
	stateChanged := make(chan struct{}, 10)

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		protocols := websocket.Subprotocols(r)
		upgrader.Subprotocols = protocols
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		// Close immediately to trigger disconnect.
		time.Sleep(50 * time.Millisecond)
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		conn.Close()
	}))
	defer server.Close()

	handler := func(event api.WebSocketEvent) {}
	onState := func(connected bool) {
		mu.Lock()
		states = append(states, connected)
		mu.Unlock()
		select {
		case stateChanged <- struct{}{}:
		default:
		}
	}

	client := NewClient(server.URL, "af_test_token", "daemon-123", handler, onState)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go client.Connect(ctx)

	// Wait for at least 2 state changes (connected=true, then disconnected=false).
	for i := 0; i < 2; i++ {
		select {
		case <-stateChanged:
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for state change %d", i+1)
		}
	}

	cancel() // Stop the client.

	mu.Lock()
	defer mu.Unlock()

	if len(states) < 2 {
		t.Fatalf("expected at least 2 state changes, got %d", len(states))
	}
	if states[0] != true {
		t.Errorf("expected first state to be connected (true), got %v", states[0])
	}
	if states[1] != false {
		t.Errorf("expected second state to be disconnected (false), got %v", states[1])
	}
}

// TestClient_ExponentialBackoff verifies that reconnection attempts use
// increasing delays.
func TestClient_ExponentialBackoff(t *testing.T) {
	var connectAttempts []time.Time
	var mu sync.Mutex

	// Server that always rejects connections (returns 403).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		connectAttempts = append(connectAttempts, time.Now())
		mu.Unlock()
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	handler := func(event api.WebSocketEvent) {}
	onState := func(connected bool) {}

	client := NewClient(server.URL, "af_test_token", "daemon-123", handler, onState)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go client.Connect(ctx)

	// Wait for at least 3 connection attempts.
	deadline := time.After(4500 * time.Millisecond)
	for {
		mu.Lock()
		count := len(connectAttempts)
		mu.Unlock()
		if count >= 3 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for 3 connection attempts, got %d", count)
		case <-time.After(100 * time.Millisecond):
		}
	}

	cancel()

	mu.Lock()
	defer mu.Unlock()

	// Verify that delays increase (backoff).
	if len(connectAttempts) >= 3 {
		delay1 := connectAttempts[1].Sub(connectAttempts[0])
		delay2 := connectAttempts[2].Sub(connectAttempts[1])

		// First delay should be ~1s, second should be ~2s.
		// Allow some tolerance for test execution jitter.
		if delay1 < 800*time.Millisecond {
			t.Errorf("first backoff delay too short: %v (expected ~1s)", delay1)
		}
		if delay2 < delay1 {
			t.Errorf("backoff should increase: delay1=%v, delay2=%v", delay1, delay2)
		}
	}
}
