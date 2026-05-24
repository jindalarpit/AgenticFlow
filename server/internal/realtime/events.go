package realtime

// Server-to-daemon event type constants.
// These are the event types that the server sends to daemon WebSocket connections.
const (
	// EventTaskInput is sent from the server to a daemon to relay user input
	// to a running task's stdin pipe.
	EventTaskInput = "task_input"
)

// Server-to-client event type constants.
// These are the event types that the server broadcasts to user WebSocket connections.
const (
	// EventTaskOutput is broadcast when new task output (stdout, stderr, or stdin) is available.
	EventTaskOutput = "task_output"

	// EventInputRequested is broadcast when the daemon detects the CLI is waiting for input.
	EventInputRequested = "input_requested"

	// EventInputCleared is broadcast when the CLI resumes output after waiting for input.
	EventInputCleared = "input_cleared"

	// EventSessionStateChanged is broadcast when a task's session state transitions.
	EventSessionStateChanged = "session_state_changed"
)
