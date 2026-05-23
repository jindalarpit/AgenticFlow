package service

import (
	"testing"

	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Feature: agent-management, Property 9: Agent Status Derivation
//
// For any agent with a bound runtime, the derived status SHALL be:
// - "offline" if the runtime's daemon status is "offline"
// - "working" if the daemon is online and the agent has at least one task in "running" status
// - "idle" if the daemon is online and the agent has no running tasks
// The priority order (offline > working > idle) SHALL be strictly enforced
// regardless of other state.
//
// **Validates: Requirements 9.1, 9.2, 9.3, 9.6**
// ---------------------------------------------------------------------------

func TestProperty9_AgentStatusDerivation_OfflineAlwaysWins(t *testing.T) {
	// Feature: agent-management, Property 9: Agent Status Derivation
	// For any runtime status == "offline" and any task count: result is "offline"
	rapid.Check(t, func(t *rapid.T) {
		// Generate any non-negative active task count
		activeTaskCount := rapid.IntRange(0, 1000).Draw(t, "activeTaskCount")

		got := DeriveAgentStatus("offline", activeTaskCount)
		if got != AgentStatusOffline {
			t.Fatalf("DeriveAgentStatus(\"offline\", %d) = %q, want %q",
				activeTaskCount, got, AgentStatusOffline)
		}
	})
}

func TestProperty9_AgentStatusDerivation_OnlineWithTasksIsWorking(t *testing.T) {
	// Feature: agent-management, Property 9: Agent Status Derivation
	// For any runtime status != "offline" and task count > 0: result is "working"
	rapid.Check(t, func(t *rapid.T) {
		// Generate a non-"offline" runtime status
		runtimeStatus := rapid.OneOf(
			rapid.Just("online"),
			rapid.Just(""),
			rapid.Just("unknown"),
			rapid.Just("starting"),
			rapid.Just("degraded"),
			rapid.StringMatching(`[a-z]{1,20}`),
		).Draw(t, "runtimeStatus")

		// Skip if we accidentally generated "offline"
		if runtimeStatus == "offline" {
			return
		}

		// Generate a positive active task count
		activeTaskCount := rapid.IntRange(1, 1000).Draw(t, "activeTaskCount")

		got := DeriveAgentStatus(runtimeStatus, activeTaskCount)
		if got != AgentStatusWorking {
			t.Fatalf("DeriveAgentStatus(%q, %d) = %q, want %q",
				runtimeStatus, activeTaskCount, got, AgentStatusWorking)
		}
	})
}

func TestProperty9_AgentStatusDerivation_OnlineNoTasksIsIdle(t *testing.T) {
	// Feature: agent-management, Property 9: Agent Status Derivation
	// For any runtime status != "offline" and task count == 0: result is "idle"
	rapid.Check(t, func(t *rapid.T) {
		// Generate a non-"offline" runtime status
		runtimeStatus := rapid.OneOf(
			rapid.Just("online"),
			rapid.Just(""),
			rapid.Just("unknown"),
			rapid.Just("starting"),
			rapid.Just("degraded"),
			rapid.StringMatching(`[a-z]{1,20}`),
		).Draw(t, "runtimeStatus")

		// Skip if we accidentally generated "offline"
		if runtimeStatus == "offline" {
			return
		}

		got := DeriveAgentStatus(runtimeStatus, 0)
		if got != AgentStatusIdle {
			t.Fatalf("DeriveAgentStatus(%q, 0) = %q, want %q",
				runtimeStatus, got, AgentStatusIdle)
		}
	})
}

func TestProperty9_AgentStatusDerivation_PriorityOfflineOverWorking(t *testing.T) {
	// Feature: agent-management, Property 9: Agent Status Derivation
	// Priority: offline always wins regardless of task count — even with active tasks,
	// offline runtime means offline status
	rapid.Check(t, func(t *rapid.T) {
		// Generate a positive task count that would normally mean "working"
		activeTaskCount := rapid.IntRange(1, 1000).Draw(t, "activeTaskCount")

		got := DeriveAgentStatus("offline", activeTaskCount)
		if got != AgentStatusOffline {
			t.Fatalf("priority violation: DeriveAgentStatus(\"offline\", %d) = %q, want %q (offline > working)",
				activeTaskCount, got, AgentStatusOffline)
		}
	})
}

func TestProperty9_AgentStatusDerivation_AllCombinations(t *testing.T) {
	// Feature: agent-management, Property 9: Agent Status Derivation
	// Verify status computation for all combinations of runtime status and active task count
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary runtime status
		runtimeStatus := rapid.OneOf(
			rapid.Just("offline"),
			rapid.Just("online"),
			rapid.Just(""),
			rapid.Just("unknown"),
			rapid.StringMatching(`[a-zA-Z0-9_-]{0,30}`),
		).Draw(t, "runtimeStatus")

		// Generate arbitrary non-negative task count
		activeTaskCount := rapid.IntRange(0, 1000).Draw(t, "activeTaskCount")

		got := DeriveAgentStatus(runtimeStatus, activeTaskCount)

		// Determine expected status based on the priority rules
		var expected AgentStatus
		switch {
		case runtimeStatus == "offline":
			expected = AgentStatusOffline
		case activeTaskCount > 0:
			expected = AgentStatusWorking
		default:
			expected = AgentStatusIdle
		}

		if got != expected {
			t.Fatalf("DeriveAgentStatus(%q, %d) = %q, want %q",
				runtimeStatus, activeTaskCount, got, expected)
		}
	})
}

func TestProperty9_AgentStatusDerivation_ResultIsAlwaysValidStatus(t *testing.T) {
	// Feature: agent-management, Property 9: Agent Status Derivation
	// For any inputs, the result must always be one of the three valid statuses
	rapid.Check(t, func(t *rapid.T) {
		// Generate completely arbitrary inputs
		runtimeStatus := rapid.String().Draw(t, "runtimeStatus")
		activeTaskCount := rapid.IntRange(-100, 10000).Draw(t, "activeTaskCount")

		got := DeriveAgentStatus(runtimeStatus, activeTaskCount)

		validStatuses := map[AgentStatus]bool{
			AgentStatusOffline: true,
			AgentStatusWorking: true,
			AgentStatusIdle:    true,
		}

		if !validStatuses[got] {
			t.Fatalf("DeriveAgentStatus(%q, %d) returned invalid status %q",
				runtimeStatus, activeTaskCount, got)
		}
	})
}

func TestProperty9_AgentStatusDerivation_Deterministic(t *testing.T) {
	// Feature: agent-management, Property 9: Agent Status Derivation
	// For the same inputs, the function must always return the same result
	rapid.Check(t, func(t *rapid.T) {
		runtimeStatus := rapid.String().Draw(t, "runtimeStatus")
		activeTaskCount := rapid.IntRange(0, 1000).Draw(t, "activeTaskCount")

		result1 := DeriveAgentStatus(runtimeStatus, activeTaskCount)
		result2 := DeriveAgentStatus(runtimeStatus, activeTaskCount)

		if result1 != result2 {
			t.Fatalf("DeriveAgentStatus(%q, %d) is non-deterministic: got %q and %q",
				runtimeStatus, activeTaskCount, result1, result2)
		}
	})
}
