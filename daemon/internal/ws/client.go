// Package ws implements the WebSocket client handler for real-time events from the server.
package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/agenticflow/agenticflow/shared/api"
	"github.com/gorilla/websocket"
)

// EventHandler is called when a WebSocket event is received from the server.
type EventHandler func(event api.WebSocketEvent)

// ConnectionStateHandler is called when the WebSocket connection state changes.
// connected=true means the connection is established; connected=false means disconnected.
type ConnectionStateHandler func(connected bool)

// Client manages a WebSocket connection to the AgenticFlow server for
// receiving real-time events (e.g., task_input for stdin relay, task_available for push).
type Client struct {
	serverURL string
	token     string
	daemonID  string
	handler   EventHandler
	onState   ConnectionStateHandler

	mu   sync.Mutex
	conn *websocket.Conn
}

// NewClient creates a new WebSocket client.
// The handler is called for each received event.
// The onState callback (optional) is called when connection state changes.
func NewClient(serverURL, token, daemonID string, handler EventHandler, onState ConnectionStateHandler) *Client {
	return &Client{
		serverURL: strings.TrimRight(serverURL, "/"),
		token:     token,
		daemonID:  daemonID,
		handler:   handler,
		onState:   onState,
	}
}

// Connect establishes the WebSocket connection to the server.
// It blocks until the context is cancelled, reconnecting on failures
// with exponential backoff (1s, 2s, 4s, 8s, 16s, capped at 30s).
func (c *Client) Connect(ctx context.Context) error {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			c.close()
			return ctx.Err()
		default:
		}

		err := c.connectOnce(ctx)
		if err != nil {
			slog.Warn("websocket connection failed, reconnecting",
				"error", err,
				"backoff", backoff,
			)
		}

		// Notify disconnected state.
		if c.onState != nil {
			c.onState(false)
		}

		// Wait with exponential backoff before reconnecting.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		// Increase backoff for next attempt.
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// Send sends a WebSocket event to the server.
func (c *Client) Send(event api.WebSocketEvent) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("websocket not connected")
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	return conn.WriteMessage(websocket.TextMessage, data)
}

// connectOnce establishes a single WebSocket connection and reads messages
// until the connection is closed or an error occurs.
func (c *Client) connectOnce(ctx context.Context) error {
	wsURL := c.buildWSURL()

	// Use Sec-WebSocket-Protocol header for authentication (same as frontend).
	// The server expects "access_token.<token>" as a sub-protocol value.
	header := http.Header{}
	if c.daemonID != "" {
		header.Set("X-Daemon-ID", c.daemonID)
	}

	subprotocols := []string{}
	if c.token != "" {
		subprotocols = append(subprotocols, "access_token."+c.token)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		Subprotocols:     subprotocols,
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return fmt.Errorf("dial websocket: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	slog.Info("websocket connected", "url", wsURL)

	// Notify connected state and reset backoff on successful connection.
	if c.onState != nil {
		c.onState(true)
	}

	defer func() {
		c.mu.Lock()
		c.conn = nil
		c.mu.Unlock()
		conn.Close()
	}()

	// Read messages until connection closes.
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil
			}
			return fmt.Errorf("read message: %w", err)
		}

		var event api.WebSocketEvent
		if err := json.Unmarshal(message, &event); err != nil {
			slog.Warn("failed to parse websocket event", "error", err)
			continue
		}

		if c.handler != nil {
			c.handler(event)
		}
	}
}

// buildWSURL constructs the WebSocket URL from the server URL.
func (c *Client) buildWSURL() string {
	url := c.serverURL
	// Convert http(s) to ws(s).
	url = strings.Replace(url, "https://", "wss://", 1)
	url = strings.Replace(url, "http://", "ws://", 1)
	return url + "/ws"
}

// close cleanly closes the WebSocket connection.
func (c *Client) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		c.conn.Close()
		c.conn = nil
	}
}
