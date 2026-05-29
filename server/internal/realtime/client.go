package realtime

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// writeWait is the time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// pongWait is the time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// pingPeriod is the interval at which pings are sent. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// maxMessageSize is the maximum message size allowed from peer.
	maxMessageSize = 4096

	// sendBufferSize is the size of the client send channel buffer.
	sendBufferSize = 256
)

// Client represents a single WebSocket connection.
type Client struct {
	// ID is the unique identifier for this client (daemonID for daemons, generated for users).
	ID string

	// UserID is the authenticated user's ID.
	UserID string

	// IsDaemon indicates whether this client is a daemon connection.
	IsDaemon bool

	// conn is the underlying WebSocket connection.
	conn *websocket.Conn

	// send is a buffered channel of outbound messages.
	send chan []byte

	// hub is a reference to the hub this client belongs to.
	hub *Hub
}

// SetSendChan sets the send channel for the client.
// This is primarily used for testing from external packages.
func (c *Client) SetSendChan(ch chan []byte) {
	c.send = ch
}

// SendChan returns the send channel for reading messages sent to this client.
// This is primarily used for testing from external packages.
func (c *Client) SendChan() <-chan []byte {
	return c.send
}

// readPump pumps messages from the WebSocket connection to the hub.
// It runs in its own goroutine and handles connection close detection.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Debug("websocket read error",
					"error", err,
					"user_id", c.UserID,
					"is_daemon", c.IsDaemon,
				)
			}
			break
		}

		// Handle inbound messages (e.g., ping/pong at application level).
		c.handleMessage(message)
	}
}

// handleMessage processes an inbound WebSocket message from the client.
func (c *Client) handleMessage(raw []byte) {
	var msg struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "ping":
		// Respond with application-level pong.
		resp, _ := json.Marshal(map[string]string{"type": "pong"})
		select {
		case c.send <- resp:
		default:
		}
	default:
		// Unknown message types are ignored for forward compatibility.
		slog.Debug("ws: unknown inbound message type", "type", msg.Type, "user_id", c.UserID)
	}
}

// writePump pumps messages from the send channel to the WebSocket connection.
// It runs in its own goroutine and handles ping/pong keepalive.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				slog.Warn("websocket write error",
					"error", err,
					"user_id", c.UserID,
					"is_daemon", c.IsDaemon,
				)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// TokenValidator is an interface for validating authentication tokens.
// Implementations should return the userID and whether the token is valid.
type TokenValidator interface {
	ValidateToken(token string) (userID string, isDaemon bool, daemonID string, ok bool)
}

// extractTokenFromProtocol extracts a bearer token from the Sec-WebSocket-Protocol
// header. The client sends the token as a sub-protocol with the prefix "access_token.".
// Returns the raw token string (without prefix) or empty string if not found.
func extractTokenFromProtocol(r *http.Request) string {
	protocols := websocket.Subprotocols(r)
	for _, p := range protocols {
		if strings.HasPrefix(p, "access_token.") {
			return strings.TrimPrefix(p, "access_token.")
		}
	}
	return ""
}

// HandleWebSocket upgrades an HTTP connection to WebSocket, authenticates the client,
// and registers it with the hub. Authentication is done via the Sec-WebSocket-Protocol
// header with an "access_token.<token>" sub-protocol value.
func HandleWebSocket(hub *Hub, validator TokenValidator, w http.ResponseWriter, r *http.Request) {
	// Extract token from Sec-WebSocket-Protocol header.
	token := extractTokenFromProtocol(r)

	if token == "" {
		http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
		return
	}

	// Validate the token.
	userID, isDaemon, daemonID, ok := validator.ValidateToken(token)
	if !ok {
		http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
		return
	}

	// Build the upgrader with the matched sub-protocol echoed back to complete the handshake.
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		Subprotocols:    []string{"access_token." + token},
		CheckOrigin: func(r *http.Request) bool {
			// In production, restrict origins. For now, allow all for development.
			return true
		},
	}

	// Upgrade to WebSocket.
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	// Determine client ID.
	clientID := userID
	if isDaemon {
		clientID = daemonID
	}

	client := &Client{
		ID:       clientID,
		UserID:   userID,
		IsDaemon: isDaemon,
		conn:     conn,
		send:     make(chan []byte, sendBufferSize),
		hub:      hub,
	}

	// Register the client with the hub.
	hub.register <- client

	// Start the read and write pumps in separate goroutines.
	go client.writePump()
	go client.readPump()
}
