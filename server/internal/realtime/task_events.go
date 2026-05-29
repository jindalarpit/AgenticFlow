package realtime

import (
	"encoding/json"
	"log/slog"
)

// TaskStartedPayload is the payload for task_started events.
// For conversational tasks, DeliverableType is populated.
type TaskStartedPayload struct {
	TaskID          string `json:"task_id"`
	DaemonID        string `json:"daemon_id"`
	DeliverableType string `json:"deliverable_type,omitempty"`
}

// TaskCompletedPayload is the payload for task_completed events.
// For conversational tasks, DeliverableType and OutputContent are populated.
type TaskCompletedPayload struct {
	TaskID          string `json:"task_id"`
	ExitCode        int32  `json:"exit_code"`
	DeliverableType string `json:"deliverable_type,omitempty"`
	OutputContent   string `json:"output_content,omitempty"`
}

// TaskFailedPayload is the payload for task_failed events.
// For conversational tasks, DeliverableType is populated.
type TaskFailedPayload struct {
	TaskID          string `json:"task_id"`
	ExitCode        int32  `json:"exit_code"`
	ErrorMessage    string `json:"error_message"`
	DeliverableType string `json:"deliverable_type,omitempty"`
}

// TaskOutputPayload is the payload for task_output events.
// Streams stdout/stderr content during task execution.
type TaskOutputPayload struct {
	TaskID   string `json:"task_id"`
	Stream   string `json:"stream"`
	Content  string `json:"content"`
	Sequence int    `json:"sequence,omitempty"`
}

// BroadcastTaskStarted broadcasts a task_started event to all connected clients.
// For conversational tasks, deliverableType should be non-empty.
func (h *Hub) BroadcastTaskStarted(taskID, daemonID, deliverableType string) {
	h.broadcastTaskEvent(EventTaskStarted, TaskStartedPayload{
		TaskID:          taskID,
		DaemonID:        daemonID,
		DeliverableType: deliverableType,
	})
}

// BroadcastTaskCompleted broadcasts a task_completed event to all connected clients.
// For conversational tasks, deliverableType and outputContent should be non-empty.
func (h *Hub) BroadcastTaskCompleted(taskID string, exitCode int32, deliverableType, outputContent string) {
	h.broadcastTaskEvent(EventTaskCompleted, TaskCompletedPayload{
		TaskID:          taskID,
		ExitCode:        exitCode,
		DeliverableType: deliverableType,
		OutputContent:   outputContent,
	})
}

// BroadcastTaskFailed broadcasts a task_failed event to all connected clients.
// For conversational tasks, deliverableType should be non-empty.
func (h *Hub) BroadcastTaskFailed(taskID string, exitCode int32, errorMessage, deliverableType string) {
	h.broadcastTaskEvent(EventTaskFailed, TaskFailedPayload{
		TaskID:          taskID,
		ExitCode:        exitCode,
		ErrorMessage:    errorMessage,
		DeliverableType: deliverableType,
	})
}

// BroadcastTaskOutput broadcasts a task_output event to all connected clients.
// Used for streaming stdout/stderr during task execution.
func (h *Hub) BroadcastTaskOutput(taskID, stream, content string, sequence int) {
	h.broadcastTaskEvent(EventTaskOutput, TaskOutputPayload{
		TaskID:   taskID,
		Stream:   stream,
		Content:  content,
		Sequence: sequence,
	})
}

// broadcastTaskEvent is a helper that marshals and broadcasts a task event.
func (h *Hub) broadcastTaskEvent(eventType string, payload interface{}) {
	event := Event{
		Type:    eventType,
		Payload: payload,
	}

	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal task event", "type", eventType, "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, conns := range h.clients {
		for _, client := range conns {
			select {
			case client.send <- data:
			default:
				slog.Warn("client send channel full during task event broadcast",
					"user_id", client.UserID, "type", eventType)
			}
		}
	}

	for _, client := range h.daemons {
		select {
		case client.send <- data:
		default:
			slog.Warn("daemon send channel full during task event broadcast",
				"daemon_id", client.ID, "type", eventType)
		}
	}
}
