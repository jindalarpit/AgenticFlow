package service

import (
	"testing"

	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Property 11: Online Agent Status Derivation
//
// For any agent with runtime_mode "online", the derived status SHALL be:
// - "error" if the bound provider's status is "error", "inactive", or "validating"
// - "working" if the provider is "active" and at least one task is running
// - "idle" if the provider is "active" and zero tasks are running
// The derived status SHALL never be "offline" for online agents.
//
// Priority order: error > working > idle (never offline).
//
// **Validates: Requirements 11.1, 11.2, 11.3, 11.4**
// ---------------------------------------------------------------------------

// validOnlineProviderStatuses are the possible provider statuses.
var validOnlineProviderStatuses = []string{"active", "error", "inactive", "validating"}

func TestProperty11_OnlineAgentStatus_ErrorProviderAlwaysError(t *testing.T) {
	// Requirement 11.3: If provider status is "error", "inactive", or "validating",
	// the agent status SHALL be "error" regardless of running task count.
	rapid.Check(t, func(t *rapid.T) {
		providerStatus := rapid.SampledFrom([]string{"error", "inactive", "validating"}).Draw(t, "providerStatus")
		runningTaskCount := rapid.Int64Range(0, 1000).Draw(t, "runningTaskCount")

		result := DeriveOnlineAgentStatus(providerStatus, runningTaskCount)

		if result != "error" {
			t.Fatalf("expected \"error\" for provider status %q with %d running tasks, got %q",
				providerStatus, runningTaskCount, result)
		}
	})
}

func TestProperty11_OnlineAgentStatus_ActiveWithTasksIsWorking(t *testing.T) {
	// Requirement 11.2: If provider is "active" and at least one task is running,
	// the agent status SHALL be "working".
	rapid.Check(t, func(t *rapid.T) {
		runningTaskCount := rapid.Int64Range(1, 1000).Draw(t, "runningTaskCount")

		result := DeriveOnlineAgentStatus("active", runningTaskCount)

		if result != "working" {
			t.Fatalf("expected \"working\" for active provider with %d running tasks, got %q",
				runningTaskCount, result)
		}
	})
}

func TestProperty11_OnlineAgentStatus_ActiveWithZeroTasksIsIdle(t *testing.T) {
	// Requirement 11.1: If provider is "active" and zero tasks running,
	// the agent status SHALL be "idle".
	rapid.Check(t, func(t *rapid.T) {
		// Always zero running tasks
		result := DeriveOnlineAgentStatus("active", 0)

		if result != "idle" {
			t.Fatalf("expected \"idle\" for active provider with 0 running tasks, got %q", result)
		}
	})
}

func TestProperty11_OnlineAgentStatus_NeverOffline(t *testing.T) {
	// Requirement 11.4: The derived status SHALL never be "offline" for online agents.
	rapid.Check(t, func(t *rapid.T) {
		providerStatus := rapid.SampledFrom(validOnlineProviderStatuses).Draw(t, "providerStatus")
		runningTaskCount := rapid.Int64Range(0, 1000).Draw(t, "runningTaskCount")

		result := DeriveOnlineAgentStatus(providerStatus, runningTaskCount)

		if result == "offline" {
			t.Fatalf("DeriveOnlineAgentStatus returned \"offline\" for provider status %q with %d running tasks",
				providerStatus, runningTaskCount)
		}
	})
}

func TestProperty11_OnlineAgentStatus_PriorityErrorOverWorking(t *testing.T) {
	// Priority: error > working > idle.
	// Even with running tasks, error provider status takes precedence.
	rapid.Check(t, func(t *rapid.T) {
		providerStatus := rapid.SampledFrom([]string{"error", "inactive", "validating"}).Draw(t, "providerStatus")
		// Specifically test with running tasks to verify error takes priority over working
		runningTaskCount := rapid.Int64Range(1, 1000).Draw(t, "runningTaskCount")

		result := DeriveOnlineAgentStatus(providerStatus, runningTaskCount)

		if result != "error" {
			t.Fatalf("expected error to take priority over working: provider status %q, %d running tasks, got %q",
				providerStatus, runningTaskCount, result)
		}
	})
}

func TestProperty11_OnlineAgentStatus_PriorityWorkingOverIdle(t *testing.T) {
	// Priority: working > idle.
	// With active provider and running tasks, status is "working" not "idle".
	rapid.Check(t, func(t *rapid.T) {
		runningTaskCount := rapid.Int64Range(1, 1000).Draw(t, "runningTaskCount")

		result := DeriveOnlineAgentStatus("active", runningTaskCount)

		if result == "idle" {
			t.Fatalf("expected working to take priority over idle: active provider with %d running tasks, got \"idle\"",
				runningTaskCount)
		}
		if result != "working" {
			t.Fatalf("expected \"working\" for active provider with %d running tasks, got %q",
				runningTaskCount, result)
		}
	})
}

func TestProperty11_OnlineAgentStatus_ResultAlwaysValid(t *testing.T) {
	// For any combination of inputs, the result is always one of the valid
	// online agent statuses: "error", "working", or "idle" (never "offline").
	rapid.Check(t, func(t *rapid.T) {
		providerStatus := rapid.SampledFrom(validOnlineProviderStatuses).Draw(t, "providerStatus")
		runningTaskCount := rapid.Int64Range(0, 1000).Draw(t, "runningTaskCount")

		result := DeriveOnlineAgentStatus(providerStatus, runningTaskCount)

		validResults := map[string]bool{"error": true, "working": true, "idle": true}
		if !validResults[result] {
			t.Fatalf("DeriveOnlineAgentStatus returned invalid status %q for provider status %q with %d running tasks",
				result, providerStatus, runningTaskCount)
		}
	})
}
