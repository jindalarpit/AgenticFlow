package constants

import "time"

const (
	DefaultServerPort         = 8080
	DefaultDaemonHealthPort   = 8081
	DefaultPollInterval       = 3 * time.Second
	DefaultHeartbeatInterval  = 15 * time.Second
	DefaultAgentTimeout       = 2 * time.Hour
	DefaultMaxConcurrentTasks = 5

	// FallbackPollInterval is the reduced poll interval used when the daemon
	// has an active WebSocket connection to the server. Push-based task
	// assignment is preferred; polling serves as a fallback.
	FallbackPollInterval = 30 * time.Second
)

