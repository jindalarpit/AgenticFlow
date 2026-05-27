package realtime

import (
	"encoding/json"
	"log/slog"
)

// Stage lifecycle event type constants for WebSocket broadcasting.
const (
	// EventStageAwaitingApproval is broadcast when a workflow stage completes
	// execution and enters the awaiting_approval state, waiting for user review.
	EventStageAwaitingApproval = "stage_awaiting_approval"

	// EventStageApproved is broadcast when a user approves a workflow stage,
	// allowing the workflow to advance to the next stage.
	EventStageApproved = "stage_approved"

	// EventStageRejected is broadcast when a user rejects a workflow stage
	// with feedback, causing the stage to be re-queued for re-execution.
	EventStageRejected = "stage_rejected"

	// EventStageStarted is broadcast when a daemon begins executing a
	// workflow stage.
	EventStageStarted = "stage_started"
)

// StageAwaitingApprovalPayload is the payload for stage_awaiting_approval events.
type StageAwaitingApprovalPayload struct {
	TaskID        string `json:"task_id"`
	StageName     string `json:"stage_name"`
	OutputContent string `json:"output_content"`
}

// StageApprovedPayload is the payload for stage_approved events.
type StageApprovedPayload struct {
	TaskID    string `json:"task_id"`
	StageName string `json:"stage_name"`
}

// StageRejectedPayload is the payload for stage_rejected events.
type StageRejectedPayload struct {
	TaskID    string `json:"task_id"`
	StageName string `json:"stage_name"`
	Feedback  string `json:"feedback"`
}

// StageStartedPayload is the payload for stage_started events.
type StageStartedPayload struct {
	TaskID    string `json:"task_id"`
	StageName string `json:"stage_name"`
}

// BroadcastStageAwaitingApproval broadcasts a stage_awaiting_approval event
// to all connected clients when a stage completes and awaits user review.
func (h *Hub) BroadcastStageAwaitingApproval(taskID, stageName, outputContent string) {
	h.broadcastStageEvent(EventStageAwaitingApproval, StageAwaitingApprovalPayload{
		TaskID:        taskID,
		StageName:     stageName,
		OutputContent: outputContent,
	})
}

// BroadcastStageApproved broadcasts a stage_approved event to all connected
// clients when a user approves a workflow stage.
func (h *Hub) BroadcastStageApproved(taskID, stageName string) {
	h.broadcastStageEvent(EventStageApproved, StageApprovedPayload{
		TaskID:    taskID,
		StageName: stageName,
	})
}

// BroadcastStageRejected broadcasts a stage_rejected event to all connected
// clients when a user rejects a workflow stage with feedback.
func (h *Hub) BroadcastStageRejected(taskID, stageName, feedback string) {
	h.broadcastStageEvent(EventStageRejected, StageRejectedPayload{
		TaskID:    taskID,
		StageName: stageName,
		Feedback:  feedback,
	})
}

// BroadcastStageStarted broadcasts a stage_started event to all connected
// clients when a daemon begins executing a workflow stage.
func (h *Hub) BroadcastStageStarted(taskID, stageName string) {
	h.broadcastStageEvent(EventStageStarted, StageStartedPayload{
		TaskID:    taskID,
		StageName: stageName,
	})
}

// broadcastStageEvent is a helper that marshals and broadcasts a stage event.
func (h *Hub) broadcastStageEvent(eventType string, payload interface{}) {
	event := Event{
		Type:    eventType,
		Payload: payload,
	}

	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal stage event", "type", eventType, "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, client := range h.clients {
		select {
		case client.send <- data:
		default:
			slog.Warn("client send channel full during stage event broadcast",
				"user_id", client.UserID, "type", eventType)
		}
	}

	for _, client := range h.daemons {
		select {
		case client.send <- data:
		default:
			slog.Warn("daemon send channel full during stage event broadcast",
				"daemon_id", client.ID, "type", eventType)
		}
	}
}
