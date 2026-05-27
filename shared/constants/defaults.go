package constants

import "time"

const (
	DefaultServerPort         = 8080
	DefaultDaemonHealthPort   = 8081
	DefaultPollInterval       = 3 * time.Second
	DefaultHeartbeatInterval  = 15 * time.Second
	DefaultAgentTimeout       = 2 * time.Hour
	DefaultMaxConcurrentTasks = 5
)
