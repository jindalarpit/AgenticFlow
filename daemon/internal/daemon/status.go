package daemon

import (
	"fmt"
	"strings"
	"time"
)

// AgentRuntimeInfo holds information about a detected agent runtime for status display.
type AgentRuntimeInfo struct {
	Name    string
	Version string
	Status  string // "available", "busy", "unavailable"
}

// HeartbeatStatus holds heartbeat connection information for status display.
type HeartbeatStatus struct {
	LastTimestamp    time.Time
	ConnectionState string // "connected" or "disconnected"
}

// DaemonStatus holds the complete daemon state for status output.
type DaemonStatus struct {
	Running       bool
	PID           int
	Uptime        time.Duration
	AgentRuntimes []AgentRuntimeInfo
	Heartbeat     HeartbeatStatus
}

// FormatStatus formats a DaemonStatus into a human-readable string.
func FormatStatus(status DaemonStatus) string {
	var b strings.Builder

	if status.Running {
		b.WriteString("Status: running\n")
	} else {
		b.WriteString("Status: stopped\n")
	}
	if status.Running {
		b.WriteString(fmt.Sprintf("PID: %d\n", status.PID))
	}
	b.WriteString(fmt.Sprintf("Uptime: %s\n", formatDuration(status.Uptime)))
	b.WriteString("Agent Runtimes:\n")
	if len(status.AgentRuntimes) == 0 {
		b.WriteString("  (none detected)\n")
	} else {
		for _, agent := range status.AgentRuntimes {
			b.WriteString(fmt.Sprintf("  - %s (version: %s, status: %s)\n",
				agent.Name, agent.Version, agent.Status))
		}
	}
	b.WriteString("Heartbeat:\n")
	b.WriteString(fmt.Sprintf("  Connection: %s\n", status.Heartbeat.ConnectionState))
	if !status.Heartbeat.LastTimestamp.IsZero() {
		b.WriteString(fmt.Sprintf("  Last heartbeat: %s\n",
			status.Heartbeat.LastTimestamp.Format(time.RFC3339)))
	} else {
		b.WriteString("  Last heartbeat: never\n")
	}
	return b.String()
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
