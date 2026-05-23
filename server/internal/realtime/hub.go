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

// Hub manages WebSocket connections and broadcasts events to connected clients and daemons.
type Hub struct {
	// clients maps userID -> *Client for user connections.
	clients map[string]*Client

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
		clients:    make(map[string]*Client),
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
			for _, client := range h.clients {
				close(client.send)
			}
			for _, client := range h.daemons {
				close(client.send)
			}
			h.clients = make(map[string]*Client)
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
				// If there's an existing user connection, close it.
				if existing, ok := h.clients[client.UserID]; ok {
					close(existing.send)
				}
				h.clients[client.UserID] = client
				slog.Info("user connected to hub", "user_id", client.UserID)
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
				if existing, ok := h.clients[client.UserID]; ok && existing == client {
					delete(h.clients, client.UserID)
					close(client.send)
					slog.Info("user disconnected from hub", "user_id", client.UserID)
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

// SendToUser sends an event to a specific user's client connection.
func (h *Hub) SendToUser(userID string, event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal event for user", "error", err, "user_id", userID, "type", event.Type)
		return
	}

	h.mu.RLock()
	client, ok := h.clients[userID]
	h.mu.RUnlock()

	if !ok {
		return
	}

	select {
	case client.send <- data:
	default:
		slog.Warn("user send channel full, dropping message", "user_id", userID, "type", event.Type)
	}
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

	for _, client := range h.clients {
		select {
		case client.send <- data:
		default:
			slog.Warn("client send channel full during broadcast", "user_id", client.UserID, "type", event.Type)
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
