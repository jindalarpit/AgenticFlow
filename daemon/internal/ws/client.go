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

// Client manages a WebSocket connection to the AgenticFlow server for
// receiving real-time events (e.g., task_input for stdin relay).
type Client struct {
	serverURL string
	token     string
	daemonID  string
	handler   EventHandler

	mu   sync.Mutex
	conn *websocket.Conn
}

// NewClient creates a new WebSocket client.
func NewClient(serverURL, token, daemonID string, handler EventHandler) *Client {
	return &Client{
		serverURL: strings.TrimRight(serverURL, "/"),
		token:     token,
		daemonID:  daemonID,
		handler:   handler,
	}
}

// Connect establishes the WebSocket connection to the server.
// It blocks until the context is cancelled, reconnecting on failures.
func (c *Client) Connect(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			c.close()
			return ctx.Err()
		default:
		}

		err := c.connectOnce(ctx)
		if err != nil {
			slog.Warn("websocket connection failed, reconnecting", "error", err)
		}

		// Wait before reconnecting
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
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

	header := http.Header{}
	if c.token != "" {
		header.Set("Authorization", "Bearer "+c.token)
	}
	if c.daemonID != "" {
		header.Set("X-Daemon-ID", c.daemonID)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return fmt.Errorf("dial websocket: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	slog.Info("websocket connected", "url", wsURL)

	defer func() {
		c.mu.Lock()
		c.conn = nil
		c.mu.Unlock()
		conn.Close()
	}()

	// Read messages until connection closes
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
	// Convert http(s) to ws(s)
	url = strings.Replace(url, "https://", "wss://", 1)
	url = strings.Replace(url, "http://", "ws://", 1)
	return url + "/api/daemon/ws"
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
