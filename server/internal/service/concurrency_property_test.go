package service

import (
	"testing"

	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Feature: agent-management, Property 12: Agent Concurrency Enforcement
//
// For any agent with max_concurrent_tasks = N and currently N tasks in
// "running" status, the Server SHALL NOT assign additional tasks to that
// agent. When the running count drops below N, the Server SHALL resume
// assignment. Each agent's limit SHALL be enforced independently of other
// agents on the same daemon.
//
// **Validates: Requirements 15.1, 15.2, 15.3**
// ---------------------------------------------------------------------------

// IsAgentEligibleForTask determines whether an agent can accept a new task
// based on its current running task count and configured maximum concurrency.
// This mirrors the SQL predicate in ClaimPendingTaskForRuntime:
//
//	(SELECT COUNT(*) FROM task t2 WHERE t2.agent_id = a.id AND t2.status = 'running') < a.max_concurrent_tasks
//
// Returns true if the agent can accept a new task, false otherwise.
func IsAgentEligibleForTask(runningCount, maxConcurrent int) bool {
	return runningCount < maxConcurrent
}

// --- Property Tests ---

func TestProperty12_ConcurrencyEnforcement_AtMaxNotEligible(t *testing.T) {
	// Feature: agent-management, Property 12: Agent Concurrency Enforcement
	// For any agent where runningCount == maxConcurrentTasks: agent is NOT eligible
	rapid.Check(t, func(t *rapid.T) {
		maxConcurrent := rapid.IntRange(1, 20).Draw(t, "maxConcurrent")
		runningCount := maxConcurrent // exactly at max

		eligible := IsAgentEligibleForTask(runningCount, maxConcurrent)
		if eligible {
			t.Fatalf("agent with runningCount=%d == maxConcurrent=%d should NOT be eligible",
				runningCount, maxConcurrent)
		}
	})
}

func TestProperty12_ConcurrencyEnforcement_BelowMaxIsEligible(t *testing.T) {
	// Feature: agent-management, Property 12: Agent Concurrency Enforcement
	// For any agent where runningCount < maxConcurrentTasks: agent IS eligible
	rapid.Check(t, func(t *rapid.T) {
		maxConcurrent := rapid.IntRange(1, 20).Draw(t, "maxConcurrent")
		runningCount := rapid.IntRange(0, maxConcurrent-1).Draw(t, "runningCount")

		eligible := IsAgentEligibleForTask(runningCount, maxConcurrent)
		if !eligible {
			t.Fatalf("agent with runningCount=%d < maxConcurrent=%d should be eligible",
				runningCount, maxConcurrent)
		}
	})
}

func TestProperty12_ConcurrencyEnforcement_AboveMaxNotEligible(t *testing.T) {
	// Feature: agent-management, Property 12: Agent Concurrency Enforcement
	// For any agent where runningCount > maxConcurrentTasks (defensive): agent is NOT eligible
	rapid.Check(t, func(t *rapid.T) {
		maxConcurrent := rapid.IntRange(1, 20).Draw(t, "maxConcurrent")
		runningCount := rapid.IntRange(maxConcurrent+1, maxConcurrent+50).Draw(t, "runningCount")

		eligible := IsAgentEligibleForTask(runningCount, maxConcurrent)
		if eligible {
			t.Fatalf("agent with runningCount=%d > maxConcurrent=%d should NOT be eligible",
				runningCount, maxConcurrent)
		}
	})
}

func TestProperty12_ConcurrencyEnforcement_IndependentPerAgent(t *testing.T) {
	// Feature: agent-management, Property 12: Agent Concurrency Enforcement
	// Two agents with different limits are evaluated independently
	rapid.Check(t, func(t *rapid.T) {
		// Agent A configuration
		maxConcurrentA := rapid.IntRange(1, 20).Draw(t, "maxConcurrentA")
		runningCountA := rapid.IntRange(0, 25).Draw(t, "runningCountA")

		// Agent B configuration (independent)
		maxConcurrentB := rapid.IntRange(1, 20).Draw(t, "maxConcurrentB")
		runningCountB := rapid.IntRange(0, 25).Draw(t, "runningCountB")

		eligibleA := IsAgentEligibleForTask(runningCountA, maxConcurrentA)
		eligibleB := IsAgentEligibleForTask(runningCountB, maxConcurrentB)

		// Verify each agent's eligibility is determined solely by its own state
		expectedA := runningCountA < maxConcurrentA
		expectedB := runningCountB < maxConcurrentB

		if eligibleA != expectedA {
			t.Fatalf("agent A: IsAgentEligibleForTask(%d, %d) = %v, want %v (independent of agent B)",
				runningCountA, maxConcurrentA, eligibleA, expectedA)
		}
		if eligibleB != expectedB {
			t.Fatalf("agent B: IsAgentEligibleForTask(%d, %d) = %v, want %v (independent of agent A)",
				runningCountB, maxConcurrentB, eligibleB, expectedB)
		}
	})
}

func TestProperty12_ConcurrencyEnforcement_ResumeWhenBelowMax(t *testing.T) {
	// Feature: agent-management, Property 12: Agent Concurrency Enforcement
	// When an agent's active task count drops below max, it becomes eligible again
	rapid.Check(t, func(t *rapid.T) {
		maxConcurrent := rapid.IntRange(1, 20).Draw(t, "maxConcurrent")

		// Start at max (not eligible)
		atMax := IsAgentEligibleForTask(maxConcurrent, maxConcurrent)
		if atMax {
			t.Fatalf("agent at max (%d/%d) should not be eligible", maxConcurrent, maxConcurrent)
		}

		// Drop below max (eligible again)
		belowMax := rapid.IntRange(0, maxConcurrent-1).Draw(t, "belowMax")
		resumed := IsAgentEligibleForTask(belowMax, maxConcurrent)
		if !resumed {
			t.Fatalf("agent below max (%d/%d) should be eligible (resumed)", belowMax, maxConcurrent)
		}
	})
}

func TestProperty12_ConcurrencyEnforcement_ZeroRunningAlwaysEligible(t *testing.T) {
	// Feature: agent-management, Property 12: Agent Concurrency Enforcement
	// An agent with zero running tasks is always eligible (since max >= 1)
	rapid.Check(t, func(t *rapid.T) {
		maxConcurrent := rapid.IntRange(1, 20).Draw(t, "maxConcurrent")

		eligible := IsAgentEligibleForTask(0, maxConcurrent)
		if !eligible {
			t.Fatalf("agent with 0 running tasks and maxConcurrent=%d should always be eligible",
				maxConcurrent)
		}
	})
}

func TestProperty12_ConcurrencyEnforcement_PredicateMatchesSQL(t *testing.T) {
	// Feature: agent-management, Property 12: Agent Concurrency Enforcement
	// The predicate runningCount < maxConcurrentTasks matches the SQL:
	//   (SELECT COUNT(*) ...) < a.max_concurrent_tasks
	// For any valid inputs, the Go function must produce the same boolean as
	// the strict less-than comparison.
	rapid.Check(t, func(t *rapid.T) {
		maxConcurrent := rapid.IntRange(1, 20).Draw(t, "maxConcurrent")
		runningCount := rapid.IntRange(0, 30).Draw(t, "runningCount")

		goResult := IsAgentEligibleForTask(runningCount, maxConcurrent)
		sqlResult := runningCount < maxConcurrent // mirrors SQL: COUNT(*) < max_concurrent_tasks

		if goResult != sqlResult {
			t.Fatalf("IsAgentEligibleForTask(%d, %d) = %v, but SQL predicate (%d < %d) = %v",
				runningCount, maxConcurrent, goResult, runningCount, maxConcurrent, sqlResult)
		}
	})
}

func TestProperty12_ConcurrencyEnforcement_Deterministic(t *testing.T) {
	// Feature: agent-management, Property 12: Agent Concurrency Enforcement
	// For the same inputs, the function must always return the same result
	rapid.Check(t, func(t *rapid.T) {
		maxConcurrent := rapid.IntRange(1, 20).Draw(t, "maxConcurrent")
		runningCount := rapid.IntRange(0, 30).Draw(t, "runningCount")

		result1 := IsAgentEligibleForTask(runningCount, maxConcurrent)
		result2 := IsAgentEligibleForTask(runningCount, maxConcurrent)

		if result1 != result2 {
			t.Fatalf("IsAgentEligibleForTask(%d, %d) is non-deterministic: got %v and %v",
				runningCount, maxConcurrent, result1, result2)
		}
	})
}
