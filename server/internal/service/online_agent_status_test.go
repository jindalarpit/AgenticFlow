package service

import (
	"testing"

	"github.com/agenticflow/agenticflow/shared/constants"
)

func TestDeriveOnlineAgentStatus_ErrorProviderStatuses(t *testing.T) {
	tests := []struct {
		name             string
		providerStatus   string
		runningTaskCount int64
		expected         string
	}{
		{"error status with no tasks", "error", 0, constants.AgentStatusError},
		{"error status with running tasks", "error", 5, constants.AgentStatusError},
		{"inactive status with no tasks", "inactive", 0, constants.AgentStatusError},
		{"inactive status with running tasks", "inactive", 3, constants.AgentStatusError},
		{"validating status with no tasks", "validating", 0, constants.AgentStatusError},
		{"validating status with running tasks", "validating", 2, constants.AgentStatusError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeriveOnlineAgentStatus(tt.providerStatus, tt.runningTaskCount)
			if result != tt.expected {
				t.Errorf("DeriveOnlineAgentStatus(%q, %d) = %q, want %q",
					tt.providerStatus, tt.runningTaskCount, result, tt.expected)
			}
		})
	}
}

func TestDeriveOnlineAgentStatus_WorkingWithRunningTasks(t *testing.T) {
	tests := []struct {
		name             string
		runningTaskCount int64
	}{
		{"one running task", 1},
		{"multiple running tasks", 5},
		{"many running tasks", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeriveOnlineAgentStatus("active", tt.runningTaskCount)
			if result != constants.AgentStatusWorking {
				t.Errorf("DeriveOnlineAgentStatus(%q, %d) = %q, want %q",
					"active", tt.runningTaskCount, result, constants.AgentStatusWorking)
			}
		})
	}
}

func TestDeriveOnlineAgentStatus_IdleWithNoTasks(t *testing.T) {
	result := DeriveOnlineAgentStatus("active", 0)
	if result != constants.AgentStatusIdle {
		t.Errorf("DeriveOnlineAgentStatus(%q, %d) = %q, want %q",
			"active", 0, result, constants.AgentStatusIdle)
	}
}

func TestDeriveOnlineAgentStatus_NeverReturnsOffline(t *testing.T) {
	// Test all possible provider statuses to ensure "offline" is never returned
	statuses := []string{"active", "error", "inactive", "validating", "unknown", ""}
	taskCounts := []int64{0, 1, 5, 100}

	for _, status := range statuses {
		for _, count := range taskCounts {
			result := DeriveOnlineAgentStatus(status, count)
			if result == constants.AgentStatusOffline {
				t.Errorf("DeriveOnlineAgentStatus(%q, %d) returned %q, which should never happen for online agents",
					status, count, constants.AgentStatusOffline)
			}
		}
	}
}

func TestDeriveOnlineAgentStatus_PriorityOrder(t *testing.T) {
	// Error takes precedence over working
	result := DeriveOnlineAgentStatus("error", 10)
	if result != constants.AgentStatusError {
		t.Errorf("error should take precedence over working: got %q", result)
	}

	// Working takes precedence over idle (when provider is active)
	result = DeriveOnlineAgentStatus("active", 1)
	if result != constants.AgentStatusWorking {
		t.Errorf("working should take precedence over idle: got %q", result)
	}

	// Idle is the default when provider is active and no tasks running
	result = DeriveOnlineAgentStatus("active", 0)
	if result != constants.AgentStatusIdle {
		t.Errorf("idle should be default: got %q", result)
	}
}
