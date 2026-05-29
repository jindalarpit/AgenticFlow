package realtime

import (
	"encoding/json"
	"log/slog"
)

// Agent event type constants for WebSocket broadcasting.
const (
	EventAgentCreated       = "agent_created"
	EventAgentUpdated       = "agent_updated"
	EventAgentDeleted       = "agent_deleted"
	EventAgentStatusChanged = "agent_status_changed"
)

// AgentStatusChangedPayload is the payload for agent_status_changed events.
type AgentStatusChangedPayload struct {
	AgentID string `json:"agent_id"`
	Status  string `json:"status"`
}

// BroadcastAgentCreated broadcasts an agent_created event to all connected clients.
// The payload should be the full AgentResponse struct.
func (h *Hub) BroadcastAgentCreated(payload interface{}) {
	h.broadcastAgentEvent(EventAgentCreated, map[string]interface{}{"agent": payload})
}

// BroadcastAgentUpdated broadcasts an agent_updated event to all connected clients.
// The payload should be the full AgentResponse struct.
func (h *Hub) BroadcastAgentUpdated(payload interface{}) {
	h.broadcastAgentEvent(EventAgentUpdated, map[string]interface{}{"agent": payload})
}

// BroadcastAgentDeleted broadcasts an agent_deleted event to all connected clients.
func (h *Hub) BroadcastAgentDeleted(agentID string) {
	h.broadcastAgentEvent(EventAgentDeleted, map[string]interface{}{"agent_id": agentID})
}

// BroadcastAgentStatusChanged broadcasts an agent_status_changed event to all
// connected clients. This is called when an agent's derived status changes due
// to daemon connect/disconnect or task status transitions.
func (h *Hub) BroadcastAgentStatusChanged(agentID, status string) {
	h.broadcastAgentEvent(EventAgentStatusChanged, AgentStatusChangedPayload{
		AgentID: agentID,
		Status:  status,
	})
}

// broadcastAgentEvent is a helper that marshals and broadcasts an agent event.
func (h *Hub) broadcastAgentEvent(eventType string, payload interface{}) {
	event := Event{
		Type:    eventType,
		Payload: payload,
	}

	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal agent event", "type", eventType, "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, conns := range h.clients {
		for _, client := range conns {
			select {
			case client.send <- data:
			default:
				slog.Warn("client send channel full during agent event broadcast",
					"user_id", client.UserID, "type", eventType)
			}
		}
	}

	for _, client := range h.daemons {
		select {
		case client.send <- data:
		default:
			slog.Warn("daemon send channel full during agent event broadcast",
				"daemon_id", client.ID, "type", eventType)
		}
	}
}
