package api

// WebSocket event type constants for server-to-client communication.
// These define the event types broadcast over WebSocket connections.
const (
	// EventTaskCreated is broadcast when a new task is created.
	EventTaskCreated = "task_created"

	// EventTaskStarted is broadcast when a daemon begins executing a task.
	EventTaskStarted = "task_started"

	// EventTaskOutput is broadcast when new task output (stdout, stderr, or stdin) is available.
	EventTaskOutput = "task_output"

	// EventTaskCompleted is broadcast when a task finishes successfully.
	EventTaskCompleted = "task_completed"

	// EventTaskFailed is broadcast when a task fails.
	EventTaskFailed = "task_failed"

	// EventInputRequested is broadcast when the daemon detects the CLI is waiting for input.
	EventInputRequested = "input_requested"

	// EventInputCleared is broadcast when the CLI resumes output after waiting for input.
	EventInputCleared = "input_cleared"

	// EventSessionStateChanged is broadcast when a task's session state transitions.
	EventSessionStateChanged = "session_state_changed"
)

// WebSocket event type constants for server-to-daemon communication.
const (
	// EventTaskInput is sent from the server to a daemon to relay user input
	// to a running task's stdin pipe.
	EventTaskInput = "task_input"
)

// WebSocketEvent is the envelope for all WebSocket messages.
type WebSocketEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// TaskCreatedPayload is the payload for task_created events.
type TaskCreatedPayload struct {
	TaskID    string `json:"task_id"`
	AgentType string `json:"agent_type"`
	Prompt    string `json:"prompt"`
	Status    string `json:"status"`
}

// TaskStartedPayload is the payload for task_started events.
type TaskStartedPayload struct {
	TaskID          string `json:"task_id"`
	DaemonID        string `json:"daemon_id"`
	DeliverableType string `json:"deliverable_type,omitempty"`
}

// TaskOutputPayload is the payload for task_output events.
type TaskOutputPayload struct {
	TaskID   string `json:"task_id"`
	Stream   string `json:"stream"`
	Content  string `json:"content"`
	Sequence int    `json:"sequence,omitempty"`
}

// TaskCompletedPayload is the payload for task_completed events.
type TaskCompletedPayload struct {
	TaskID          string `json:"task_id"`
	ExitCode        int32  `json:"exit_code"`
	DeliverableType string `json:"deliverable_type,omitempty"`
	OutputContent   string `json:"output_content,omitempty"`
}

// TaskFailedPayload is the payload for task_failed events.
type TaskFailedPayload struct {
	TaskID          string `json:"task_id"`
	ExitCode        int32  `json:"exit_code"`
	ErrorMessage    string `json:"error_message"`
	DeliverableType string `json:"deliverable_type,omitempty"`
}

// TaskInputPayload is the payload for task_input events (server → daemon).
type TaskInputPayload struct {
	TaskID  string `json:"task_id"`
	Content string `json:"content"`
}
