// Package realtime provides the WebSocket hub for real-time event broadcasting.
package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
)

// Event represents a real-time event to be broadcast or sent to specific clients.
type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
	UserID  string      `json:"-"` // target user (empty = broadcast to all)
}

// maxConnectionsPerUser is the maximum number of concurrent WebSocket connections
// allowed per user. When exceeded, the oldest connection is closed.
const maxConnectionsPerUser = 5

// Hub manages WebSocket connections and broadcasts events to connected clients and daemons.
type Hub struct {
	// clients maps userID -> list of *Client for user connections (supports multiple tabs).
	clients map[string][]*Client

	// daemons maps daemonID -> *Client for daemon connections.
	daemons map[string]*Client

	// broadcast channel for events to all connected clients.
	broadcast chan Event

	// register channel for new client connections.
	register chan *Client

	// unregister channel for disconnecting clients.
	unregister chan *Client

	// mu protects clients and daemons maps.
	mu sync.RWMutex
}

// NewHub creates a new Hub instance with initialized channels and maps.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string][]*Client),
		daemons:    make(map[string]*Client),
		broadcast:  make(chan Event, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub event loop. It processes register, unregister, and broadcast
// channels until the context is cancelled.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			for _, conns := range h.clients {
				for _, client := range conns {
					close(client.send)
				}
			}
			for _, client := range h.daemons {
				close(client.send)
			}
			h.clients = make(map[string][]*Client)
			h.daemons = make(map[string]*Client)
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			if client.IsDaemon {
				// If there's an existing daemon connection, close it.
				if existing, ok := h.daemons[client.ID]; ok {
					close(existing.send)
				}
				h.daemons[client.ID] = client
				slog.Info("daemon connected to hub", "daemon_id", client.ID)
			} else {
				conns := h.clients[client.UserID]
				if len(conns) >= maxConnectionsPerUser {
					// Close the oldest connection to make room.
					oldest := conns[0]
					close(oldest.send)
					conns = conns[1:]
					slog.Info("user connection limit reached, closed oldest", "user_id", client.UserID)
				}
				h.clients[client.UserID] = append(conns, client)
				slog.Info("user connected to hub", "user_id", client.UserID, "connections", len(h.clients[client.UserID]))
			}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if client.IsDaemon {
				if existing, ok := h.daemons[client.ID]; ok && existing == client {
					delete(h.daemons, client.ID)
					close(client.send)
					slog.Info("daemon disconnected from hub", "daemon_id", client.ID)
				}
			} else {
				conns := h.clients[client.UserID]
				for i, c := range conns {
					if c == client {
						// Remove this specific connection from the slice.
						h.clients[client.UserID] = append(conns[:i], conns[i+1:]...)
						close(client.send)
						slog.Info("user disconnected from hub", "user_id", client.UserID, "remaining", len(h.clients[client.UserID]))
						break
					}
				}
				// Clean up empty slices.
				if len(h.clients[client.UserID]) == 0 {
					delete(h.clients, client.UserID)
				}
			}
			h.mu.Unlock()

		case event := <-h.broadcast:
			h.broadcastEvent(event)
		}
	}
}

// Broadcast sends an event to all connected clients (both users and daemons).
func (h *Hub) Broadcast(event Event) {
	select {
	case h.broadcast <- event:
	default:
		slog.Warn("broadcast channel full, dropping event", "type", event.Type)
	}
}

// SendToUser sends an event to all active connections for a specific user.
func (h *Hub) SendToUser(userID string, event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal event for user", "error", err, "user_id", userID, "type", event.Type)
		return
	}

	h.mu.RLock()
	conns := h.clients[userID]
	h.mu.RUnlock()

	for _, client := range conns {
		select {
		case client.send <- data:
		default:
			slog.Warn("user send channel full, dropping message", "user_id", userID, "type", event.Type)
		}
	}
}

// IsDaemonConnected returns true if the daemon with the given ID has an active
// WebSocket connection to the hub.
func (h *Hub) IsDaemonConnected(daemonID string) bool {
	h.mu.RLock()
	_, ok := h.daemons[daemonID]
	h.mu.RUnlock()
	return ok
}

// Register adds a client to the hub's registration queue.
// This is primarily used for testing from external packages.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// SendToDaemon sends an event to a specific daemon's client connection.
func (h *Hub) SendToDaemon(daemonID string, event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal event for daemon", "error", err, "daemon_id", daemonID, "type", event.Type)
		return
	}

	h.mu.RLock()
	client, ok := h.daemons[daemonID]
	h.mu.RUnlock()

	if !ok {
		return
	}

	select {
	case client.send <- data:
	default:
		slog.Warn("daemon send channel full, dropping message", "daemon_id", daemonID, "type", event.Type)
	}
}

// broadcastEvent sends an event to all connected clients and daemons.
func (h *Hub) broadcastEvent(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal broadcast event", "error", err, "type", event.Type)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, conns := range h.clients {
		for _, client := range conns {
			select {
			case client.send <- data:
			default:
				slog.Warn("client send channel full during broadcast", "user_id", client.UserID, "type", event.Type)
			}
		}
	}

	for _, client := range h.daemons {
		select {
		case client.send <- data:
		default:
			slog.Warn("daemon send channel full during broadcast", "daemon_id", client.ID, "type", event.Type)
		}
	}
}
