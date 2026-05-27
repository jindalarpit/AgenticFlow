package handler

import (
	"testing"

	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Feature: task-workflow-stages, Property 10: Poll Filtering Excludes Non-Pending Tasks
//
// For any set of tasks in the database, the poll endpoint SHALL only return tasks
// where either (a) the task has no stages and status is "pending", or (b) the task
// has stages and the next stage by order has status "pending". Tasks with stages in
// awaiting_approval, rejected, or running status SHALL NOT be returned.
//
// **Validates: Requirements 9.5**
// ---------------------------------------------------------------------------

// StageState represents the status of a single workflow stage for poll filtering logic.
type StageState struct {
	Name   string
	Order  int
	Status string
}

// isTaskEligibleForPoll determines whether a task is eligible to be returned by
// the poll endpoint. A task is eligible if and only if:
//   - It has no stages and the task status is "pending" (single-pass mode), OR
//   - It has stages and the next stage by order has status "pending"
//
// Tasks with stages where the next stage is in awaiting_approval, rejected, running,
// approved, completed, or failed status are NOT eligible.
func isTaskEligibleForPoll(stages []StageState, taskStatus string) bool {
	// Case (a): No stages — single-pass task. Eligible only if task status is "pending".
	if len(stages) == 0 {
		return taskStatus == "pending"
	}

	// Case (b): Has stages — find the next stage by order (lowest order with status
	// that indicates it needs execution). The "next stage" is the lowest-order stage
	// that has not yet been approved/completed.
	// We look for the first stage (by order) that is in "pending" status.
	// If no stage is pending, the task is not eligible.
	var nextStage *StageState
	for i := range stages {
		s := &stages[i]
		// Find the lowest-order stage that is not yet approved/completed
		// (i.e., the "next" stage in the workflow)
		if s.Status != "approved" && s.Status != "completed" {
			if nextStage == nil || s.Order < nextStage.Order {
				nextStage = s
			}
			break // stages are expected to be processed in order
		}
	}

	// If we found a next stage, it must be "pending" for the task to be eligible
	if nextStage == nil {
		// All stages are approved/completed — no pending work, not eligible
		return false
	}

	return nextStage.Status == "pending"
}

func TestProperty10_PollFilteringExcludesNonPendingTasks(t *testing.T) {
	// Feature: task-workflow-stages, Property 10: Poll Filtering Excludes Non-Pending Tasks

	allStageNames := []string{"plan", "design", "tasks", "execution"}
	allStageStatuses := []string{"pending", "running", "awaiting_approval", "approved", "rejected", "completed", "failed"}
	allTaskStatuses := []string{"pending", "running", "completed", "failed"}

	// Generator: produce a task with stages in various statuses
	genStages := rapid.Custom(func(t *rapid.T) []StageState {
		numStages := rapid.IntRange(0, 4).Draw(t, "numStages")
		if numStages == 0 {
			return nil
		}

		// Pick numStages unique deliverables in canonical order
		indices := []int{0, 1, 2, 3}
		// Shuffle to pick random subset
		for i := len(indices) - 1; i > 0; i-- {
			j := rapid.IntRange(0, i).Draw(t, "shuffleIdx")
			indices[i], indices[j] = indices[j], indices[i]
		}
		selected := indices[:numStages]

		// Sort to maintain canonical order
		for i := 0; i < len(selected)-1; i++ {
			for j := i + 1; j < len(selected); j++ {
				if selected[i] > selected[j] {
					selected[i], selected[j] = selected[j], selected[i]
				}
			}
		}

		stages := make([]StageState, numStages)
		for i, idx := range selected {
			stages[i] = StageState{
				Name:   allStageNames[idx],
				Order:  deliverableOrder[allStageNames[idx]],
				Status: rapid.SampledFrom(allStageStatuses).Draw(t, "stageStatus"),
			}
		}
		return stages
	})

	rapid.Check(t, func(t *rapid.T) {
		stages := genStages.Draw(t, "stages")
		taskStatus := rapid.SampledFrom(allTaskStatuses).Draw(t, "taskStatus")

		eligible := isTaskEligibleForPoll(stages, taskStatus)

		if len(stages) == 0 {
			// Case (a): No stages — eligible only if task status is "pending"
			if taskStatus == "pending" && !eligible {
				t.Fatalf("single-pass task with status 'pending' should be eligible for poll")
			}
			if taskStatus != "pending" && eligible {
				t.Fatalf("single-pass task with status %q should NOT be eligible for poll", taskStatus)
			}
		} else {
			// Case (b): Has stages — find the next stage (first non-approved/non-completed)
			var nextStage *StageState
			for i := range stages {
				if stages[i].Status != "approved" && stages[i].Status != "completed" {
					nextStage = &stages[i]
					break
				}
			}

			if nextStage == nil {
				// All stages approved/completed — not eligible
				if eligible {
					t.Fatalf("task with all stages approved/completed should NOT be eligible for poll")
				}
			} else if nextStage.Status == "pending" {
				// Next stage is pending — eligible
				if !eligible {
					t.Fatalf("task with next stage %q in 'pending' status should be eligible for poll, stages: %+v",
						nextStage.Name, stages)
				}
			} else {
				// Next stage is NOT pending (running, awaiting_approval, rejected, failed) — NOT eligible
				if eligible {
					t.Fatalf("task with next stage %q in %q status should NOT be eligible for poll, stages: %+v",
						nextStage.Name, nextStage.Status, stages)
				}
			}
		}
	})
}

func TestProperty10_SinglePassPendingTaskIsEligible(t *testing.T) {
	// Feature: task-workflow-stages, Property 10: Poll Filtering Excludes Non-Pending Tasks
	// Sub-property: A single-pass task (no stages) with status "pending" is always eligible.

	rapid.Check(t, func(t *rapid.T) {
		eligible := isTaskEligibleForPoll(nil, "pending")
		if !eligible {
			t.Fatal("single-pass task with status 'pending' must always be eligible for poll")
		}
	})
}

func TestProperty10_SinglePassNonPendingTaskIsNotEligible(t *testing.T) {
	// Feature: task-workflow-stages, Property 10: Poll Filtering Excludes Non-Pending Tasks
	// Sub-property: A single-pass task (no stages) with any non-pending status is never eligible.

	nonPendingStatuses := []string{"running", "completed", "failed"}

	rapid.Check(t, func(t *rapid.T) {
		status := rapid.SampledFrom(nonPendingStatuses).Draw(t, "status")
		eligible := isTaskEligibleForPoll(nil, status)
		if eligible {
			t.Fatalf("single-pass task with status %q must NOT be eligible for poll", status)
		}
	})
}

func TestProperty10_StagedTaskWithPendingNextStageIsEligible(t *testing.T) {
	// Feature: task-workflow-stages, Property 10: Poll Filtering Excludes Non-Pending Tasks
	// Sub-property: A staged task where the next stage (by order) is "pending" is eligible.

	allStageNames := []string{"plan", "design", "tasks", "execution"}

	rapid.Check(t, func(t *rapid.T) {
		numStages := rapid.IntRange(1, 4).Draw(t, "numStages")

		// Pick stages in canonical order
		indices := []int{0, 1, 2, 3}
		for i := len(indices) - 1; i > 0; i-- {
			j := rapid.IntRange(0, i).Draw(t, "shuffleIdx")
			indices[i], indices[j] = indices[j], indices[i]
		}
		selected := indices[:numStages]
		for i := 0; i < len(selected)-1; i++ {
			for j := i + 1; j < len(selected); j++ {
				if selected[i] > selected[j] {
					selected[i], selected[j] = selected[j], selected[i]
				}
			}
		}

		// Pick a random position K where the "next" stage will be pending
		// All stages before K are approved/completed, stage at K is pending
		k := rapid.IntRange(0, numStages-1).Draw(t, "pendingPosition")

		stages := make([]StageState, numStages)
		for i, idx := range selected {
			stages[i] = StageState{
				Name:  allStageNames[idx],
				Order: deliverableOrder[allStageNames[idx]],
			}
			if i < k {
				// Prior stages are approved or completed
				if rapid.Bool().Draw(t, "approvedOrCompleted") {
					stages[i].Status = "approved"
				} else {
					stages[i].Status = "completed"
				}
			} else if i == k {
				stages[i].Status = "pending"
			} else {
				// Stages after K are pending (haven't been reached yet)
				stages[i].Status = "pending"
			}
		}

		eligible := isTaskEligibleForPoll(stages, "running")
		if !eligible {
			t.Fatalf("staged task with next stage %q in 'pending' status should be eligible, stages: %+v",
				stages[k].Name, stages)
		}
	})
}

func TestProperty10_StagedTaskWithNonPendingNextStageIsNotEligible(t *testing.T) {
	// Feature: task-workflow-stages, Property 10: Poll Filtering Excludes Non-Pending Tasks
	// Sub-property: A staged task where the next stage is NOT "pending" is never eligible.
	// This covers awaiting_approval, rejected, running, and failed statuses.

	allStageNames := []string{"plan", "design", "tasks", "execution"}
	nonPendingNonTerminalStatuses := []string{"running", "awaiting_approval", "rejected", "failed"}

	rapid.Check(t, func(t *rapid.T) {
		numStages := rapid.IntRange(1, 4).Draw(t, "numStages")

		// Pick stages in canonical order
		indices := []int{0, 1, 2, 3}
		for i := len(indices) - 1; i > 0; i-- {
			j := rapid.IntRange(0, i).Draw(t, "shuffleIdx")
			indices[i], indices[j] = indices[j], indices[i]
		}
		selected := indices[:numStages]
		for i := 0; i < len(selected)-1; i++ {
			for j := i + 1; j < len(selected); j++ {
				if selected[i] > selected[j] {
					selected[i], selected[j] = selected[j], selected[i]
				}
			}
		}

		// Pick a random position K where the "next" stage will be non-pending
		k := rapid.IntRange(0, numStages-1).Draw(t, "blockingPosition")

		stages := make([]StageState, numStages)
		for i, idx := range selected {
			stages[i] = StageState{
				Name:  allStageNames[idx],
				Order: deliverableOrder[allStageNames[idx]],
			}
			if i < k {
				// Prior stages are approved or completed
				if rapid.Bool().Draw(t, "approvedOrCompleted") {
					stages[i].Status = "approved"
				} else {
					stages[i].Status = "completed"
				}
			} else if i == k {
				// The blocking stage — NOT pending
				stages[i].Status = rapid.SampledFrom(nonPendingNonTerminalStatuses).Draw(t, "blockingStatus")
			} else {
				// Stages after K are pending
				stages[i].Status = "pending"
			}
		}

		eligible := isTaskEligibleForPoll(stages, "running")
		if eligible {
			t.Fatalf("staged task with next stage %q in %q status should NOT be eligible, stages: %+v",
				stages[k].Name, stages[k].Status, stages)
		}
	})
}

func TestProperty10_AllStagesApprovedOrCompletedIsNotEligible(t *testing.T) {
	// Feature: task-workflow-stages, Property 10: Poll Filtering Excludes Non-Pending Tasks
	// Sub-property: A task where all stages are approved/completed has no pending work
	// and should NOT be eligible for poll.

	allStageNames := []string{"plan", "design", "tasks", "execution"}
	terminalStatuses := []string{"approved", "completed"}

	rapid.Check(t, func(t *rapid.T) {
		numStages := rapid.IntRange(1, 4).Draw(t, "numStages")

		// Pick stages in canonical order
		indices := []int{0, 1, 2, 3}
		for i := len(indices) - 1; i > 0; i-- {
			j := rapid.IntRange(0, i).Draw(t, "shuffleIdx")
			indices[i], indices[j] = indices[j], indices[i]
		}
		selected := indices[:numStages]
		for i := 0; i < len(selected)-1; i++ {
			for j := i + 1; j < len(selected); j++ {
				if selected[i] > selected[j] {
					selected[i], selected[j] = selected[j], selected[i]
				}
			}
		}

		stages := make([]StageState, numStages)
		for i, idx := range selected {
			stages[i] = StageState{
				Name:   allStageNames[idx],
				Order:  deliverableOrder[allStageNames[idx]],
				Status: rapid.SampledFrom(terminalStatuses).Draw(t, "terminalStatus"),
			}
		}

		eligible := isTaskEligibleForPoll(stages, "completed")
		if eligible {
			t.Fatalf("task with all stages approved/completed should NOT be eligible for poll, stages: %+v", stages)
		}
	})
}
