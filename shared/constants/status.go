package constants

// Task statuses
const (
	TaskStatusPending   = "pending"
	TaskStatusRunning   = "running"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
)

// Agent statuses
const (
	AgentStatusIdle    = "idle"
	AgentStatusWorking = "working"
	AgentStatusOffline = "offline"
	AgentStatusError   = "error"
)

// Daemon statuses
const (
	DaemonStatusOnline  = "online"
	DaemonStatusOffline = "offline"
)
