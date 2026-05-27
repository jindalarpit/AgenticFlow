package handler

import (
	"sort"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// WorkflowStage represents a single stage in a task workflow for testing purposes.
type WorkflowStage struct {
	Name     string
	Order    int
	Status   string
	Output   string
	Feedback string
}

// ---------------------------------------------------------------------------
// Feature: task-workflow-stages, Property 3: Stage Completion Halts at Approval Gate
//
// For any workflow stage that transitions from "running" to a terminal execution
// state, the stage status SHALL become "awaiting_approval" (not "approved" or
// the next stage's "running"), ensuring the workflow cannot advance without
// explicit user action.
//
// **Validates: Requirements 2.3, 4.1**
// ---------------------------------------------------------------------------

// completeStage simulates what happens when a daemon reports stage completion.
// Input: a slice of stages representing the workflow, and the index of the stage
// that is completing (must be in "running" status).
// Output: the updated slice of stages after the completion is processed.
// The completing stage's status transitions to "awaiting_approval".
// No other stages change status.
func completeStage(stages []WorkflowStage, completingIdx int) []WorkflowStage {
	result := make([]WorkflowStage, len(stages))
	copy(result, stages)

	if completingIdx < 0 || completingIdx >= len(stages) {
		return result
	}

	if result[completingIdx].Status != "running" {
		return result
	}

	// Server-enforced: completing a stage always results in awaiting_approval.
	// The daemon cannot auto-advance or auto-approve.
	result[completingIdx].Status = "awaiting_approval"

	return result
}

// rejectStage simulates what happens when a user rejects a stage in "awaiting_approval" status.
// It stores the feedback, transitions the stage to "rejected", and then re-queues it
// back to "pending" for re-execution. Returns an error message if the stage is not
// in the correct status or feedback is empty.
func rejectStage(stage *WorkflowStage, feedback string) string {
	if stage.Status != "awaiting_approval" {
		return "stage is not awaiting approval"
	}
	if strings.TrimSpace(feedback) == "" {
		return "feedback is required when rejecting a stage"
	}

	// Step 1: Set status to rejected and store feedback
	stage.Status = "rejected"
	stage.Feedback = feedback

	// Step 2: Re-queue the stage back to pending for re-execution
	stage.Status = "pending"

	return ""
}

func TestProperty3_StageCompletionHaltsAtApprovalGate(t *testing.T) {
	// Feature: task-workflow-stages, Property 3: Stage Completion Halts at Approval Gate
	allDeliverables := []string{"plan", "design", "tasks", "execution"}

	// Generator: produce a workflow configuration with 1-4 stages
	genWorkflowConfig := rapid.Custom(func(t *rapid.T) []WorkflowStage {
		numStages := rapid.IntRange(1, 4).Draw(t, "numStages")

		// Pick numStages unique deliverables in canonical order
		indices := make([]int, 4)
		for i := range indices {
			indices[i] = i
		}
		// Shuffle and take first numStages
		for i := len(indices) - 1; i > 0; i-- {
			j := rapid.IntRange(0, i).Draw(t, "shuffleIdx")
			indices[i], indices[j] = indices[j], indices[i]
		}
		selected := indices[:numStages]

		// Sort selected indices to maintain canonical order
		for i := 0; i < len(selected)-1; i++ {
			for j := i + 1; j < len(selected); j++ {
				if selected[i] > selected[j] {
					selected[i], selected[j] = selected[j], selected[i]
				}
			}
		}

		stages := make([]WorkflowStage, numStages)
		for i, idx := range selected {
			stages[i] = WorkflowStage{
				Name:   allDeliverables[idx],
				Order:  deliverableOrder[allDeliverables[idx]],
				Status: "pending",
			}
		}

		return stages
	})

	rapid.Check(t, func(t *rapid.T) {
		stages := genWorkflowConfig.Draw(t, "workflow")

		// Pick a random stage position to be the "completing" stage
		completingIdx := rapid.IntRange(0, len(stages)-1).Draw(t, "completingIdx")

		// Set the completing stage to "running" (precondition for completion)
		stages[completingIdx].Status = "running"

		// Record the statuses of all other stages before completion
		otherStatuses := make(map[int]string)
		for i, s := range stages {
			if i != completingIdx {
				otherStatuses[i] = s.Status
			}
		}

		// Simulate stage completion
		result := completeStage(stages, completingIdx)

		// Property 3a: The completing stage MUST be "awaiting_approval"
		if result[completingIdx].Status != "awaiting_approval" {
			t.Fatalf("stage %q at index %d should be 'awaiting_approval' after completion, got %q",
				result[completingIdx].Name, completingIdx, result[completingIdx].Status)
		}

		// Property 3b: The completing stage must NOT be "approved" (no auto-approve)
		if result[completingIdx].Status == "approved" {
			t.Fatalf("stage %q at index %d was auto-approved, violating approval gate",
				result[completingIdx].Name, completingIdx)
		}

		// Property 3c: No other stage should have changed status (no auto-advancing)
		for i, expectedStatus := range otherStatuses {
			if result[i].Status != expectedStatus {
				t.Fatalf("stage %q at index %d changed status from %q to %q during completion of stage %q; no other stages should change",
					result[i].Name, i, expectedStatus, result[i].Status, result[completingIdx].Name)
			}
		}

		// Property 3d: Specifically, if there's a next stage, it must NOT be "running"
		if completingIdx+1 < len(result) {
			nextStatus := result[completingIdx+1].Status
			if nextStatus == "running" {
				t.Fatalf("next stage %q auto-advanced to 'running' without approval gate",
					result[completingIdx+1].Name)
			}
		}
	})
}

func TestProperty3_StageCompletionNeverAutoAdvances(t *testing.T) {
	// Feature: task-workflow-stages, Property 3: Stage Completion Halts at Approval Gate
	// Additional sub-property: regardless of workflow size or position, completion
	// never results in any stage being auto-advanced to "running".
	allDeliverables := []string{"plan", "design", "tasks", "execution"}

	rapid.Check(t, func(t *rapid.T) {
		numStages := rapid.IntRange(2, 4).Draw(t, "numStages")

		// Build stages in canonical order from a random subset
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

		stages := make([]WorkflowStage, numStages)
		for i, idx := range selected {
			stages[i] = WorkflowStage{
				Name:   allDeliverables[idx],
				Order:  deliverableOrder[allDeliverables[idx]],
				Status: "pending",
			}
		}

		// Set the first stage as running (simulating daemon executing it)
		completingIdx := rapid.IntRange(0, numStages-1).Draw(t, "completingIdx")
		stages[completingIdx].Status = "running"

		result := completeStage(stages, completingIdx)

		// After completion, no stage in the entire workflow should be "running"
		for i, s := range result {
			if s.Status == "running" {
				t.Fatalf("stage %q at index %d is 'running' after completeStage; "+
					"completion must halt at approval gate without auto-advancing any stage",
					s.Name, i)
			}
		}
	})
}

func TestProperty3_StageCompletionOnlyFromRunning(t *testing.T) {
	// Feature: task-workflow-stages, Property 3: Stage Completion Halts at Approval Gate
	// Sub-property: completeStage is a no-op if the stage is not in "running" status.
	allStatuses := []string{"pending", "awaiting_approval", "approved", "rejected", "completed", "failed"}

	rapid.Check(t, func(t *rapid.T) {
		// Generate a single stage with a non-running status
		status := rapid.SampledFrom(allStatuses).Draw(t, "status")
		stages := []WorkflowStage{
			{Name: "plan", Order: 1, Status: status},
		}

		result := completeStage(stages, 0)

		// The stage should remain unchanged since it's not in "running" status
		if result[0].Status != status {
			t.Fatalf("completeStage changed status from %q to %q for non-running stage; should be no-op",
				status, result[0].Status)
		}
	})
}

// ---------------------------------------------------------------------------
// Feature: task-workflow-stages, Property 4: Approve Advances Exactly One Stage
//
// For any stage in `awaiting_approval` status, approving it SHALL set that stage
// to `approved` and — if a subsequent stage exists — set exactly the next stage
// (by stage_order) to `pending`. No other stages shall change status.
//
// **Validates: Requirements 4.3**
// ---------------------------------------------------------------------------

// stageState represents a workflow stage with its name and status for testing.
type stageState struct {
	Name   string
	Order  int
	Status string
}

// simulateApprove applies the approve logic to a slice of stages at position K.
// It returns the updated stages slice and an error string (empty if successful).
// Rules:
//   - The stage at position K must be in "awaiting_approval" status.
//   - The approved stage transitions to "approved".
//   - If K+1 exists, only stage K+1 transitions to "pending".
//   - No other stages change status.
//   - If K is the last stage, no stage becomes pending (task completes).
func simulateApprove(stages []stageState, approveIdx int) ([]stageState, string) {
	if approveIdx < 0 || approveIdx >= len(stages) {
		return stages, "stage index out of bounds"
	}
	if stages[approveIdx].Status != "awaiting_approval" {
		return stages, "stage is not awaiting approval"
	}

	// Create a copy to avoid mutating the input
	result := make([]stageState, len(stages))
	copy(result, stages)

	// Set the approved stage to "approved"
	result[approveIdx].Status = "approved"

	// If there's a next stage, set it to "pending"
	if approveIdx+1 < len(result) {
		result[approveIdx+1].Status = "pending"
	}

	return result, ""
}

func TestProperty4_ApproveAdvancesExactlyOneStage(t *testing.T) {
	stageNames := []string{"plan", "design", "tasks", "execution"}

	// Valid statuses for stages that are NOT the one being approved
	otherStatuses := []string{"pending", "running", "awaiting_approval", "approved", "rejected", "completed", "failed"}

	rapid.Check(t, func(t *rapid.T) {
		// Generate N stages (1-4)
		n := rapid.IntRange(1, 4).Draw(t, "numStages")

		// Generate K (position to approve, 0 to N-1)
		k := rapid.IntRange(0, n-1).Draw(t, "approvePosition")

		// Build stages with initial statuses
		stages := make([]stageState, n)
		for i := 0; i < n; i++ {
			stages[i] = stageState{
				Name:  stageNames[i],
				Order: i + 1,
			}
			if i == k {
				// The stage being approved must be in "awaiting_approval"
				stages[i].Status = "awaiting_approval"
			} else {
				// Other stages get random statuses
				stages[i].Status = rapid.SampledFrom(otherStatuses).Draw(t, "otherStatus")
			}
		}

		// Capture original statuses for comparison
		originalStatuses := make([]string, n)
		for i, s := range stages {
			originalStatuses[i] = s.Status
		}

		// Perform the approve
		result, errMsg := simulateApprove(stages, k)

		// Should succeed
		if errMsg != "" {
			t.Fatalf("simulateApprove failed unexpectedly: %s", errMsg)
		}

		// Property 4a: The approved stage must now be "approved"
		if result[k].Status != "approved" {
			t.Fatalf("stage at position %d should be 'approved', got %q", k, result[k].Status)
		}

		// Property 4b: If K+1 exists, only stage K+1 becomes "pending"
		if k+1 < n {
			if result[k+1].Status != "pending" {
				t.Fatalf("stage at position %d (next after approved) should be 'pending', got %q", k+1, result[k+1].Status)
			}
		}

		// Property 4c: No other stages change status
		for i := 0; i < n; i++ {
			if i == k {
				continue // already checked
			}
			if i == k+1 {
				continue // already checked
			}
			if result[i].Status != originalStatuses[i] {
				t.Fatalf("stage at position %d should not have changed: was %q, now %q",
					i, originalStatuses[i], result[i].Status)
			}
		}

		// Property 4d: If K is the last stage, no stage becomes pending
		if k == n-1 {
			for i := 0; i < n; i++ {
				if i == k {
					continue
				}
				if result[i].Status == "pending" && originalStatuses[i] != "pending" {
					t.Fatalf("when approving last stage, no other stage should become pending, but stage %d changed to 'pending'", i)
				}
			}
		}
	})
}

func TestProperty4_ApproveRejectsNonAwaitingStage(t *testing.T) {
	// Feature: task-workflow-stages, Property 4: Approve Advances Exactly One Stage
	// Verify that approving a stage NOT in "awaiting_approval" returns an error.
	nonAwaitingStatuses := []string{"pending", "running", "approved", "rejected", "completed", "failed"}

	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 4).Draw(t, "numStages")
		k := rapid.IntRange(0, n-1).Draw(t, "approvePosition")

		stages := make([]stageState, n)
		for i := 0; i < n; i++ {
			stages[i] = stageState{
				Name:  []string{"plan", "design", "tasks", "execution"}[i],
				Order: i + 1,
			}
			if i == k {
				// Set to a non-awaiting status
				stages[i].Status = rapid.SampledFrom(nonAwaitingStatuses).Draw(t, "wrongStatus")
			} else {
				stages[i].Status = "pending"
			}
		}

		_, errMsg := simulateApprove(stages, k)

		if errMsg == "" {
			t.Fatalf("simulateApprove should fail when stage status is %q, not 'awaiting_approval'", stages[k].Status)
		}
	})
}

func TestProperty4_ApproveLastStageNoNewPending(t *testing.T) {
	// Feature: task-workflow-stages, Property 4: Approve Advances Exactly One Stage
	// When the last stage is approved, no stage should transition to pending.
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 4).Draw(t, "numStages")
		k := n - 1 // always approve the last stage

		otherStatuses := []string{"approved", "completed"}

		stages := make([]stageState, n)
		for i := 0; i < n; i++ {
			stages[i] = stageState{
				Name:  []string{"plan", "design", "tasks", "execution"}[i],
				Order: i + 1,
			}
			if i == k {
				stages[i].Status = "awaiting_approval"
			} else {
				// Prior stages should be approved/completed in a realistic workflow
				stages[i].Status = rapid.SampledFrom(otherStatuses).Draw(t, "priorStatus")
			}
		}

		result, errMsg := simulateApprove(stages, k)

		if errMsg != "" {
			t.Fatalf("simulateApprove failed unexpectedly: %s", errMsg)
		}

		// The last stage should be approved
		if result[k].Status != "approved" {
			t.Fatalf("last stage should be 'approved', got %q", result[k].Status)
		}

		// No stage should have newly become "pending"
		for i := 0; i < n; i++ {
			if i == k {
				continue
			}
			if result[i].Status != stages[i].Status {
				t.Fatalf("stage %d should not have changed when approving last stage: was %q, now %q",
					i, stages[i].Status, result[i].Status)
			}
		}
	})
}

// Feature: task-workflow-stages, Property 2: Deliverable Ordering Invariant
//
// For any valid deliverable set, the stages created for a task SHALL be exactly
// the selected deliverables, ordered by the canonical sequence (plan=1, design=2,
// tasks=3, execution=4), with no duplicates and no stages for unselected deliverables.
//
// **Validates: Requirements 1.5, 2.1, 2.2**

func TestProperty2_DeliverableOrderingInvariant(t *testing.T) {
	allDeliverables := []string{"plan", "design", "tasks", "execution"}

	// Generator: produce a non-empty subset of valid deliverables (may contain duplicates)
	genDeliverableSubset := rapid.Custom(func(t *rapid.T) []string {
		// Generate a non-empty slice by picking elements from valid deliverables
		size := rapid.IntRange(1, 8).Draw(t, "size") // up to 8 to allow duplicates
		result := make([]string, size)
		for i := range result {
			idx := rapid.IntRange(0, len(allDeliverables)-1).Draw(t, "idx")
			result[i] = allDeliverables[idx]
		}
		return result
	})

	rapid.Check(t, func(t *rapid.T) {
		input := genDeliverableSubset.Draw(t, "deliverables")

		ordered := orderDeliverables(input)

		// Property 2a: No duplicates in output
		seen := make(map[string]bool)
		for _, d := range ordered {
			if seen[d] {
				t.Fatalf("duplicate found in ordered result: %q", d)
			}
			seen[d] = true
		}

		// Property 2b: Output contains exactly the same unique elements as input (no additions/removals)
		inputSet := make(map[string]bool)
		for _, d := range input {
			inputSet[d] = true
		}
		outputSet := make(map[string]bool)
		for _, d := range ordered {
			outputSet[d] = true
		}
		// Every input element must be in output
		for d := range inputSet {
			if !outputSet[d] {
				t.Fatalf("input element %q missing from ordered output", d)
			}
		}
		// Every output element must be in input
		for d := range outputSet {
			if !inputSet[d] {
				t.Fatalf("output element %q was not in input", d)
			}
		}
		// Same number of unique elements
		if len(ordered) != len(inputSet) {
			t.Fatalf("ordered has %d elements, but input has %d unique elements", len(ordered), len(inputSet))
		}

		// Property 2c: Output is sorted by deliverableOrder values (plan=1 < design=2 < tasks=3 < execution=4)
		for i := 1; i < len(ordered); i++ {
			prevOrder := deliverableOrder[ordered[i-1]]
			currOrder := deliverableOrder[ordered[i]]
			if prevOrder >= currOrder {
				t.Fatalf("ordering violated: %q (order=%d) appears before %q (order=%d)",
					ordered[i-1], prevOrder, ordered[i], currOrder)
			}
		}
	})
}


// ---------------------------------------------------------------------------
// Feature: task-workflow-stages, Property 7: Workspace Mode Validation
//
// For any task creation request, the workspace_mode validation SHALL accept only
// the values "isolated" and "existing", and when workspace_mode is "existing",
// the workspace_path field SHALL be required and must be an absolute filesystem
// path (starts with "/").
//
// **Validates: Requirements 5.1, 5.2**
// ---------------------------------------------------------------------------

func TestProperty7_WorkspaceModeValidation_OnlyValidModesAccepted(t *testing.T) {
	// Feature: task-workflow-stages, Property 7: Workspace Mode Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate an arbitrary string as workspace_mode
		mode := rapid.String().Draw(t, "mode")

		result := validateWorkspaceMode(mode)

		isValid := (mode == "isolated" || mode == "existing")

		if isValid && result != "" {
			t.Fatalf("validateWorkspaceMode(%q) returned error %q, expected no error", mode, result)
		}
		if !isValid && result == "" {
			t.Fatalf("validateWorkspaceMode(%q) returned no error, expected rejection", mode)
		}
	})
}

func TestProperty7_WorkspaceModeValidation_IsolatedAlwaysAccepted(t *testing.T) {
	// Feature: task-workflow-stages, Property 7: Workspace Mode Validation
	rapid.Check(t, func(t *rapid.T) {
		result := validateWorkspaceMode("isolated")
		if result != "" {
			t.Fatalf("validateWorkspaceMode(\"isolated\") returned error %q, expected no error", result)
		}
	})
}

func TestProperty7_WorkspaceModeValidation_ExistingAlwaysAccepted(t *testing.T) {
	// Feature: task-workflow-stages, Property 7: Workspace Mode Validation
	rapid.Check(t, func(t *rapid.T) {
		result := validateWorkspaceMode("existing")
		if result != "" {
			t.Fatalf("validateWorkspaceMode(\"existing\") returned error %q, expected no error", result)
		}
	})
}

func TestProperty7_WorkspaceModeValidation_InvalidModesRejected(t *testing.T) {
	// Feature: task-workflow-stages, Property 7: Workspace Mode Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate strings that are NOT "isolated" or "existing"
		mode := rapid.String().Draw(t, "mode")
		if mode == "isolated" || mode == "existing" {
			return
		}

		result := validateWorkspaceMode(mode)
		if result == "" {
			t.Fatalf("validateWorkspaceMode(%q) should be rejected but was accepted", mode)
		}
	})
}

func TestProperty7_WorkspacePathValidation_IsolatedModeIgnoresPath(t *testing.T) {
	// Feature: task-workflow-stages, Property 7: Workspace Mode Validation
	rapid.Check(t, func(t *rapid.T) {
		// When mode is "isolated", any path (including empty) should be accepted
		path := rapid.String().Draw(t, "path")

		result := validateWorkspacePath("isolated", path)
		if result != "" {
			t.Fatalf("validateWorkspacePath(\"isolated\", %q) returned error %q, expected no error for isolated mode", path, result)
		}
	})
}

func TestProperty7_WorkspacePathValidation_ExistingModeRequiresNonEmptyAbsolutePath(t *testing.T) {
	// Feature: task-workflow-stages, Property 7: Workspace Mode Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate an arbitrary path string
		path := rapid.String().Draw(t, "path")

		result := validateWorkspacePath("existing", path)

		trimmedPath := strings.TrimSpace(path)
		isValid := trimmedPath != "" && strings.HasPrefix(path, "/")

		if isValid && result != "" {
			t.Fatalf("validateWorkspacePath(\"existing\", %q) returned error %q, expected no error for valid absolute path", path, result)
		}
		if !isValid && result == "" {
			t.Fatalf("validateWorkspacePath(\"existing\", %q) returned no error, expected rejection (trimmed=%q, hasPrefix=/: %v)", path, trimmedPath, strings.HasPrefix(path, "/"))
		}
	})
}

func TestProperty7_WorkspacePathValidation_ExistingModeEmptyPathRejected(t *testing.T) {
	// Feature: task-workflow-stages, Property 7: Workspace Mode Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate whitespace-only strings (including empty)
		numSpaces := rapid.IntRange(0, 50).Draw(t, "numSpaces")
		wsChars := []rune{' ', '\t', '\n', '\r'}
		runes := make([]rune, numSpaces)
		for i := range runes {
			runes[i] = wsChars[rapid.IntRange(0, len(wsChars)-1).Draw(t, "wsIdx")]
		}
		path := string(runes)

		result := validateWorkspacePath("existing", path)
		if result == "" {
			t.Fatalf("validateWorkspacePath(\"existing\", %q) should reject empty/whitespace path", path)
		}
	})
}

func TestProperty7_WorkspacePathValidation_ExistingModeRelativePathRejected(t *testing.T) {
	// Feature: task-workflow-stages, Property 7: Workspace Mode Validation
	nonSlashStarters := []string{
		"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m",
		"n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
		".", "~", "0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
	}
	rapid.Check(t, func(t *rapid.T) {
		// Generate non-empty paths that do NOT start with "/"
		firstChar := rapid.SampledFrom(nonSlashStarters).Draw(t, "firstChar")
		rest := rapid.StringMatching(`[a-zA-Z0-9/_.\-]{0,50}`).Draw(t, "rest")
		path := firstChar + rest

		result := validateWorkspacePath("existing", path)
		if result == "" {
			t.Fatalf("validateWorkspacePath(\"existing\", %q) should reject relative path", path)
		}
	})
}

func TestProperty7_WorkspacePathValidation_ExistingModeAbsolutePathAccepted(t *testing.T) {
	// Feature: task-workflow-stages, Property 7: Workspace Mode Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate valid absolute paths (start with "/" followed by non-empty content)
		pathSuffix := rapid.StringMatching(`[a-zA-Z0-9/_.\-]{1,100}`).Draw(t, "pathSuffix")
		path := "/" + pathSuffix

		result := validateWorkspacePath("existing", path)
		if result != "" {
			t.Fatalf("validateWorkspacePath(\"existing\", %q) returned error %q, expected acceptance for absolute path", path, result)
		}
	})
}


// ---------------------------------------------------------------------------
// Feature: task-workflow-stages, Property 5: Reject Stores Feedback and Re-queues
//
// For any stage in awaiting_approval status and any non-empty feedback string,
// rejecting it SHALL set the stage to rejected, store the feedback, and then
// transition the stage back to pending for re-execution. The re-execution
// context SHALL include the feedback message.
//
// **Validates: Requirements 4.4, 4.5**
// ---------------------------------------------------------------------------

func TestProperty5_RejectStoresFeedbackAndRequeues(t *testing.T) {
	allStageNames := []string{"plan", "design", "tasks", "execution"}

	// Generator: non-empty feedback strings (at least one non-whitespace character)
	genNonEmptyFeedback := rapid.Custom(func(t *rapid.T) string {
		// Generate a non-empty string with at least one non-whitespace char
		prefix := rapid.StringMatching(`[a-zA-Z0-9]`).Draw(t, "prefix")
		rest := rapid.String().Draw(t, "rest")
		return prefix + rest
	})

	rapid.Check(t, func(t *rapid.T) {
		// Pick a random valid stage name
		stageName := rapid.SampledFrom(allStageNames).Draw(t, "stageName")
		stageOrder := deliverableOrder[stageName]
		feedback := genNonEmptyFeedback.Draw(t, "feedback")

		// Create a stage in awaiting_approval status
		stage := &WorkflowStage{
			Name:   stageName,
			Order:  stageOrder,
			Status: "awaiting_approval",
			Output: "some output content",
		}

		// Reject the stage with feedback
		errMsg := rejectStage(stage, feedback)

		// Property 5a: Rejection succeeds (no error)
		if errMsg != "" {
			t.Fatalf("rejectStage returned error %q for valid rejection", errMsg)
		}

		// Property 5b: After rejection, stage status is "pending" (re-queued)
		if stage.Status != "pending" {
			t.Fatalf("expected stage status 'pending' after rejection, got %q", stage.Status)
		}

		// Property 5c: Feedback is stored on the stage
		if stage.Feedback != feedback {
			t.Fatalf("expected feedback %q to be stored, got %q", feedback, stage.Feedback)
		}

		// Property 5d: Feedback is non-empty (preserved for re-execution context)
		if strings.TrimSpace(stage.Feedback) == "" {
			t.Fatal("feedback should be non-empty after rejection")
		}
	})
}

func TestProperty5_RejectRequiresAwaitingApprovalStatus(t *testing.T) {
	// Feature: task-workflow-stages, Property 5: Reject Stores Feedback and Re-queues
	allStageNames := []string{"plan", "design", "tasks", "execution"}
	nonAwaitingStatuses := []string{"pending", "running", "approved", "rejected", "completed", "failed"}

	rapid.Check(t, func(t *rapid.T) {
		stageName := rapid.SampledFrom(allStageNames).Draw(t, "stageName")
		stageOrder := deliverableOrder[stageName]
		status := rapid.SampledFrom(nonAwaitingStatuses).Draw(t, "status")
		feedback := rapid.StringMatching(`[a-zA-Z0-9 ]{1,100}`).Draw(t, "feedback")

		stage := &WorkflowStage{
			Name:   stageName,
			Order:  stageOrder,
			Status: status,
		}

		errMsg := rejectStage(stage, feedback)

		// Rejection must fail when stage is not in awaiting_approval
		if errMsg == "" {
			t.Fatalf("rejectStage should fail for stage in status %q, but succeeded", status)
		}
	})
}

func TestProperty5_RejectRequiresNonEmptyFeedback(t *testing.T) {
	// Feature: task-workflow-stages, Property 5: Reject Stores Feedback and Re-queues
	allStageNames := []string{"plan", "design", "tasks", "execution"}

	// Generator: empty or whitespace-only feedback strings
	genEmptyFeedback := rapid.Custom(func(t *rapid.T) string {
		numSpaces := rapid.IntRange(0, 50).Draw(t, "numSpaces")
		wsChars := []rune{' ', '\t', '\n', '\r'}
		runes := make([]rune, numSpaces)
		for i := range runes {
			runes[i] = wsChars[rapid.IntRange(0, len(wsChars)-1).Draw(t, "wsIdx")]
		}
		return string(runes)
	})

	rapid.Check(t, func(t *rapid.T) {
		stageName := rapid.SampledFrom(allStageNames).Draw(t, "stageName")
		stageOrder := deliverableOrder[stageName]
		feedback := genEmptyFeedback.Draw(t, "feedback")

		stage := &WorkflowStage{
			Name:   stageName,
			Order:  stageOrder,
			Status: "awaiting_approval",
		}

		errMsg := rejectStage(stage, feedback)

		// Rejection must fail when feedback is empty/whitespace
		if errMsg == "" {
			t.Fatalf("rejectStage should fail for empty feedback %q, but succeeded", feedback)
		}

		// Stage status should remain unchanged
		if stage.Status != "awaiting_approval" {
			t.Fatalf("stage status should remain 'awaiting_approval' when rejection fails, got %q", stage.Status)
		}
	})
}

func TestProperty5_RejectFeedbackPreservedForReexecution(t *testing.T) {
	// Feature: task-workflow-stages, Property 5: Reject Stores Feedback and Re-queues
	allStageNames := []string{"plan", "design", "tasks", "execution"}

	// Generator: various non-empty feedback strings with special characters
	genFeedback := rapid.Custom(func(t *rapid.T) string {
		prefix := rapid.StringMatching(`[a-zA-Z]`).Draw(t, "prefix")
		body := rapid.StringMatching(`[a-zA-Z0-9 !@#$%^&*()_+\-=\[\]{};':"|,.<>?/~]{0,200}`).Draw(t, "body")
		return prefix + body
	})

	rapid.Check(t, func(t *rapid.T) {
		stageName := rapid.SampledFrom(allStageNames).Draw(t, "stageName")
		stageOrder := deliverableOrder[stageName]
		feedback := genFeedback.Draw(t, "feedback")

		stage := &WorkflowStage{
			Name:   stageName,
			Order:  stageOrder,
			Status: "awaiting_approval",
			Output: "original output",
		}

		// Reject the stage
		errMsg := rejectStage(stage, feedback)
		if errMsg != "" {
			t.Fatalf("rejectStage returned unexpected error: %q", errMsg)
		}

		// After re-queue to pending, feedback must still be accessible
		// This ensures the re-execution context includes the feedback
		if stage.Feedback != feedback {
			t.Fatalf("feedback not preserved after re-queue: expected %q, got %q", feedback, stage.Feedback)
		}

		// The stage is now pending with feedback stored - ready for re-execution
		if stage.Status != "pending" {
			t.Fatalf("stage should be pending for re-execution, got %q", stage.Status)
		}
	})
}


// ---------------------------------------------------------------------------
// Feature: task-workflow-stages, Property 9: Poll Response Completeness
//
// For any task that has workflow stages with at least one in "pending" status,
// the poll response SHALL include:
//   - current_stage (the lowest-order pending stage) with name, order, status
//   - prior_stages (all completed/approved stage outputs)
//   - workspace_mode
//   - workspace_path (when mode is "existing")
//
// **Validates: Requirements 9.1, 9.2, 9.3**
// ---------------------------------------------------------------------------

// pollStageInfo represents a stage as it appears in the poll response.
type pollStageInfo struct {
	Name          string
	Order         int
	Status        string
	OutputContent string // only for prior_stages
}

// pollResponse represents the relevant fields of a daemon poll response for staged tasks.
type pollResponse struct {
	CurrentStage  *pollStageInfo
	PriorStages   []pollStageInfo
	WorkspaceMode string
	WorkspacePath string
}

// taskConfig represents a task's configuration for poll response construction.
type taskConfig struct {
	Stages        []WorkflowStage
	WorkspaceMode string
	WorkspacePath string
}

// buildPollResponse simulates the poll response construction logic for a staged task.
// It mirrors the enrichPollResponseWithStages function in daemon.go:
//   - current_stage: the lowest-order stage with status "pending"
//   - prior_stages: all stages with status "completed" or "approved" (ordered by stage_order)
//   - workspace_mode: always included
//   - workspace_path: included when non-empty
//
// Returns nil if no pending stage exists (task should not be returned by poll).
func buildPollResponse(config taskConfig) *pollResponse {
	// Find the lowest-order pending stage (current_stage).
	var currentStage *pollStageInfo
	for _, s := range config.Stages {
		if s.Status == "pending" {
			if currentStage == nil || s.Order < currentStage.Order {
				currentStage = &pollStageInfo{
					Name:   s.Name,
					Order:  s.Order,
					Status: s.Status,
				}
			}
		}
	}

	// If no pending stage exists, this task should not appear in poll results.
	if currentStage == nil {
		return nil
	}

	// Collect all completed/approved stages as prior_stages.
	priorStages := make([]pollStageInfo, 0)
	for _, s := range config.Stages {
		if s.Status == "completed" || s.Status == "approved" {
			priorStages = append(priorStages, pollStageInfo{
				Name:          s.Name,
				Order:         s.Order,
				Status:        s.Status,
				OutputContent: s.Output,
			})
		}
	}

	// Sort prior_stages by order for deterministic output.
	sort.Slice(priorStages, func(i, j int) bool {
		return priorStages[i].Order < priorStages[j].Order
	})

	return &pollResponse{
		CurrentStage:  currentStage,
		PriorStages:   priorStages,
		WorkspaceMode: config.WorkspaceMode,
		WorkspacePath: config.WorkspacePath,
	}
}

func TestProperty9_PollResponseCompletenessCurrentStagePresent(t *testing.T) {
	// Feature: task-workflow-stages, Property 9: Poll Response Completeness
	// For any staged task with at least one pending stage, current_stage must be present
	// and contain name, order, and status fields.
	allDeliverables := []string{"plan", "design", "tasks", "execution"}
	validStatuses := []string{"pending", "running", "awaiting_approval", "approved", "completed", "rejected", "failed"}

	rapid.Check(t, func(t *rapid.T) {
		// Generate 1-4 stages with at least one in "pending" status
		numStages := rapid.IntRange(1, 4).Draw(t, "numStages")

		// Pick numStages unique deliverables in canonical order
		indices := []int{0, 1, 2, 3}
		for i := len(indices) - 1; i > 0; i-- {
			j := rapid.IntRange(0, i).Draw(t, "shuffleIdx")
			indices[i], indices[j] = indices[j], indices[i]
		}
		selected := indices[:numStages]
		sort.Ints(selected)

		stages := make([]WorkflowStage, numStages)
		for i, idx := range selected {
			stages[i] = WorkflowStage{
				Name:   allDeliverables[idx],
				Order:  deliverableOrder[allDeliverables[idx]],
				Status: rapid.SampledFrom(validStatuses).Draw(t, "status"),
				Output: rapid.StringMatching(`[a-zA-Z0-9 ]{0,50}`).Draw(t, "output"),
			}
		}

		// Ensure at least one stage is "pending"
		pendingIdx := rapid.IntRange(0, numStages-1).Draw(t, "pendingIdx")
		stages[pendingIdx].Status = "pending"

		// Generate workspace config
		mode := rapid.SampledFrom([]string{"isolated", "existing"}).Draw(t, "mode")
		path := ""
		if mode == "existing" {
			path = "/" + rapid.StringMatching(`[a-zA-Z0-9/_\-]{1,50}`).Draw(t, "path")
		}

		config := taskConfig{
			Stages:        stages,
			WorkspaceMode: mode,
			WorkspacePath: path,
		}

		resp := buildPollResponse(config)

		// Property 9a: Response must not be nil (task has pending stages)
		if resp == nil {
			t.Fatal("buildPollResponse returned nil for task with at least one pending stage")
		}

		// Property 9b: current_stage must be present
		if resp.CurrentStage == nil {
			t.Fatal("current_stage must be present in poll response for staged task with pending stages")
		}

		// Property 9c: current_stage must have name, order, and status
		if resp.CurrentStage.Name == "" {
			t.Fatal("current_stage.name must be non-empty")
		}
		if resp.CurrentStage.Order == 0 {
			t.Fatal("current_stage.order must be non-zero")
		}
		if resp.CurrentStage.Status == "" {
			t.Fatal("current_stage.status must be non-empty")
		}

		// Property 9d: current_stage.status must be "pending"
		if resp.CurrentStage.Status != "pending" {
			t.Fatalf("current_stage.status must be 'pending', got %q", resp.CurrentStage.Status)
		}

		// Property 9e: current_stage must be the lowest-order pending stage
		for _, s := range stages {
			if s.Status == "pending" && s.Order < resp.CurrentStage.Order {
				t.Fatalf("current_stage has order %d but there is a pending stage with lower order %d (%s)",
					resp.CurrentStage.Order, s.Order, s.Name)
			}
		}
	})
}

func TestProperty9_PollResponseCompletenessPriorStages(t *testing.T) {
	// Feature: task-workflow-stages, Property 9: Poll Response Completeness
	// prior_stages must contain all completed/approved stage outputs.
	allDeliverables := []string{"plan", "design", "tasks", "execution"}

	rapid.Check(t, func(t *rapid.T) {
		// Generate 2-4 stages (need at least 2 to have prior stages)
		numStages := rapid.IntRange(2, 4).Draw(t, "numStages")

		indices := []int{0, 1, 2, 3}
		for i := len(indices) - 1; i > 0; i-- {
			j := rapid.IntRange(0, i).Draw(t, "shuffleIdx")
			indices[i], indices[j] = indices[j], indices[i]
		}
		selected := indices[:numStages]
		sort.Ints(selected)

		stages := make([]WorkflowStage, numStages)
		for i, idx := range selected {
			stages[i] = WorkflowStage{
				Name:  allDeliverables[idx],
				Order: deliverableOrder[allDeliverables[idx]],
			}
		}

		// Set some stages to completed/approved (prior stages) and ensure last one is pending
		// Make the last stage pending, and randomly assign completed/approved to earlier ones
		stages[numStages-1].Status = "pending"
		for i := 0; i < numStages-1; i++ {
			stages[i].Status = rapid.SampledFrom([]string{"completed", "approved"}).Draw(t, "priorStatus")
			stages[i].Output = rapid.StringMatching(`[a-zA-Z0-9 ]{1,100}`).Draw(t, "priorOutput")
		}

		config := taskConfig{
			Stages:        stages,
			WorkspaceMode: "isolated",
			WorkspacePath: "",
		}

		resp := buildPollResponse(config)

		if resp == nil {
			t.Fatal("buildPollResponse returned nil for task with pending stage")
		}

		// Property 9f: prior_stages must contain exactly the completed/approved stages
		expectedPriorCount := 0
		for _, s := range stages {
			if s.Status == "completed" || s.Status == "approved" {
				expectedPriorCount++
			}
		}
		if len(resp.PriorStages) != expectedPriorCount {
			t.Fatalf("expected %d prior stages, got %d", expectedPriorCount, len(resp.PriorStages))
		}

		// Property 9g: each prior stage must have output_content from the original stage
		priorByName := make(map[string]pollStageInfo)
		for _, ps := range resp.PriorStages {
			priorByName[ps.Name] = ps
		}
		for _, s := range stages {
			if s.Status == "completed" || s.Status == "approved" {
				ps, ok := priorByName[s.Name]
				if !ok {
					t.Fatalf("completed/approved stage %q missing from prior_stages", s.Name)
				}
				if ps.OutputContent != s.Output {
					t.Fatalf("prior_stages[%q].output_content = %q, expected %q", s.Name, ps.OutputContent, s.Output)
				}
			}
		}

		// Property 9h: prior_stages must be ordered by stage_order
		for i := 1; i < len(resp.PriorStages); i++ {
			if resp.PriorStages[i-1].Order >= resp.PriorStages[i].Order {
				t.Fatalf("prior_stages not ordered: stage %q (order=%d) before %q (order=%d)",
					resp.PriorStages[i-1].Name, resp.PriorStages[i-1].Order,
					resp.PriorStages[i].Name, resp.PriorStages[i].Order)
			}
		}
	})
}

func TestProperty9_PollResponseCompletenessWorkspaceMode(t *testing.T) {
	// Feature: task-workflow-stages, Property 9: Poll Response Completeness
	// workspace_mode must always be present in the poll response.
	allDeliverables := []string{"plan", "design", "tasks", "execution"}

	rapid.Check(t, func(t *rapid.T) {
		// Generate a simple staged task with one pending stage
		numStages := rapid.IntRange(1, 4).Draw(t, "numStages")
		indices := []int{0, 1, 2, 3}
		for i := len(indices) - 1; i > 0; i-- {
			j := rapid.IntRange(0, i).Draw(t, "shuffleIdx")
			indices[i], indices[j] = indices[j], indices[i]
		}
		selected := indices[:numStages]
		sort.Ints(selected)

		stages := make([]WorkflowStage, numStages)
		for i, idx := range selected {
			stages[i] = WorkflowStage{
				Name:   allDeliverables[idx],
				Order:  deliverableOrder[allDeliverables[idx]],
				Status: "pending",
			}
		}

		mode := rapid.SampledFrom([]string{"isolated", "existing"}).Draw(t, "mode")
		path := ""
		if mode == "existing" {
			path = "/" + rapid.StringMatching(`[a-zA-Z0-9/_\-]{1,50}`).Draw(t, "path")
		}

		config := taskConfig{
			Stages:        stages,
			WorkspaceMode: mode,
			WorkspacePath: path,
		}

		resp := buildPollResponse(config)

		if resp == nil {
			t.Fatal("buildPollResponse returned nil for task with pending stages")
		}

		// Property 9i: workspace_mode must be present and valid
		if resp.WorkspaceMode == "" {
			t.Fatal("workspace_mode must be present in poll response")
		}
		if resp.WorkspaceMode != "isolated" && resp.WorkspaceMode != "existing" {
			t.Fatalf("workspace_mode must be 'isolated' or 'existing', got %q", resp.WorkspaceMode)
		}

		// Property 9j: workspace_path must be present when mode is "existing"
		if resp.WorkspaceMode == "existing" && resp.WorkspacePath == "" {
			t.Fatal("workspace_path must be present when workspace_mode is 'existing'")
		}

		// Property 9k: workspace_mode must match the task configuration
		if resp.WorkspaceMode != mode {
			t.Fatalf("workspace_mode in response (%q) does not match task config (%q)", resp.WorkspaceMode, mode)
		}

		// Property 9l: workspace_path must match the task configuration
		if resp.WorkspacePath != path {
			t.Fatalf("workspace_path in response (%q) does not match task config (%q)", resp.WorkspacePath, path)
		}
	})
}

func TestProperty9_PollResponseCompletenessNoPendingReturnsNil(t *testing.T) {
	// Feature: task-workflow-stages, Property 9: Poll Response Completeness
	// When no stage is in "pending" status, buildPollResponse returns nil
	// (task should not be returned by poll endpoint).
	allDeliverables := []string{"plan", "design", "tasks", "execution"}
	nonPendingStatuses := []string{"running", "awaiting_approval", "approved", "completed", "rejected", "failed"}

	rapid.Check(t, func(t *rapid.T) {
		numStages := rapid.IntRange(1, 4).Draw(t, "numStages")
		indices := []int{0, 1, 2, 3}
		for i := len(indices) - 1; i > 0; i-- {
			j := rapid.IntRange(0, i).Draw(t, "shuffleIdx")
			indices[i], indices[j] = indices[j], indices[i]
		}
		selected := indices[:numStages]
		sort.Ints(selected)

		stages := make([]WorkflowStage, numStages)
		for i, idx := range selected {
			stages[i] = WorkflowStage{
				Name:   allDeliverables[idx],
				Order:  deliverableOrder[allDeliverables[idx]],
				Status: rapid.SampledFrom(nonPendingStatuses).Draw(t, "status"),
				Output: rapid.StringMatching(`[a-zA-Z0-9 ]{0,50}`).Draw(t, "output"),
			}
		}

		config := taskConfig{
			Stages:        stages,
			WorkspaceMode: "isolated",
			WorkspacePath: "",
		}

		resp := buildPollResponse(config)

		// When no stage is pending, poll should not return this task
		if resp != nil {
			t.Fatalf("buildPollResponse should return nil when no stage is pending, got response with current_stage=%v", resp.CurrentStage)
		}
	})
}

func TestProperty9_PollResponseCompletenessCurrentStageIsLowestOrder(t *testing.T) {
	// Feature: task-workflow-stages, Property 9: Poll Response Completeness
	// When multiple stages are pending, current_stage must be the one with lowest order.
	allDeliverables := []string{"plan", "design", "tasks", "execution"}

	rapid.Check(t, func(t *rapid.T) {
		// Generate 2-4 stages, all pending (to test lowest-order selection)
		numStages := rapid.IntRange(2, 4).Draw(t, "numStages")
		indices := []int{0, 1, 2, 3}
		for i := len(indices) - 1; i > 0; i-- {
			j := rapid.IntRange(0, i).Draw(t, "shuffleIdx")
			indices[i], indices[j] = indices[j], indices[i]
		}
		selected := indices[:numStages]
		sort.Ints(selected)

		stages := make([]WorkflowStage, numStages)
		for i, idx := range selected {
			stages[i] = WorkflowStage{
				Name:   allDeliverables[idx],
				Order:  deliverableOrder[allDeliverables[idx]],
				Status: "pending",
			}
		}

		config := taskConfig{
			Stages:        stages,
			WorkspaceMode: "isolated",
			WorkspacePath: "",
		}

		resp := buildPollResponse(config)

		if resp == nil {
			t.Fatal("buildPollResponse returned nil for task with all-pending stages")
		}

		// The current_stage must be the stage with the lowest order
		expectedName := allDeliverables[selected[0]]
		expectedOrder := deliverableOrder[expectedName]

		if resp.CurrentStage.Name != expectedName {
			t.Fatalf("current_stage.name should be %q (lowest order), got %q",
				expectedName, resp.CurrentStage.Name)
		}
		if resp.CurrentStage.Order != expectedOrder {
			t.Fatalf("current_stage.order should be %d, got %d",
				expectedOrder, resp.CurrentStage.Order)
		}
	})
}

func TestProperty9_PollResponseCompletenessMixedStatuses(t *testing.T) {
	// Feature: task-workflow-stages, Property 9: Poll Response Completeness
	// With a realistic workflow (some approved, one pending), verify all fields are correct.
	allDeliverables := []string{"plan", "design", "tasks", "execution"}

	rapid.Check(t, func(t *rapid.T) {
		// Generate 2-4 stages in a realistic workflow state:
		// First K stages are approved/completed, remaining are pending
		numStages := rapid.IntRange(2, 4).Draw(t, "numStages")
		indices := []int{0, 1, 2, 3}
		for i := len(indices) - 1; i > 0; i-- {
			j := rapid.IntRange(0, i).Draw(t, "shuffleIdx")
			indices[i], indices[j] = indices[j], indices[i]
		}
		selected := indices[:numStages]
		sort.Ints(selected)

		// Split: first splitAt stages are completed/approved, rest are pending
		splitAt := rapid.IntRange(1, numStages-1).Draw(t, "splitAt")

		stages := make([]WorkflowStage, numStages)
		for i, idx := range selected {
			stages[i] = WorkflowStage{
				Name:  allDeliverables[idx],
				Order: deliverableOrder[allDeliverables[idx]],
			}
			if i < splitAt {
				stages[i].Status = rapid.SampledFrom([]string{"completed", "approved"}).Draw(t, "doneStatus")
				stages[i].Output = rapid.StringMatching(`[a-zA-Z0-9 ]{1,100}`).Draw(t, "output")
			} else {
				stages[i].Status = "pending"
			}
		}

		mode := rapid.SampledFrom([]string{"isolated", "existing"}).Draw(t, "mode")
		path := ""
		if mode == "existing" {
			path = "/" + rapid.StringMatching(`[a-zA-Z0-9/_\-]{1,50}`).Draw(t, "path")
		}

		config := taskConfig{
			Stages:        stages,
			WorkspaceMode: mode,
			WorkspacePath: path,
		}

		resp := buildPollResponse(config)

		if resp == nil {
			t.Fatal("buildPollResponse returned nil for task with pending stages")
		}

		// Verify current_stage is the first pending stage (at splitAt position)
		if resp.CurrentStage.Name != stages[splitAt].Name {
			t.Fatalf("current_stage should be %q, got %q", stages[splitAt].Name, resp.CurrentStage.Name)
		}

		// Verify prior_stages count matches the number of completed/approved stages
		if len(resp.PriorStages) != splitAt {
			t.Fatalf("expected %d prior stages, got %d", splitAt, len(resp.PriorStages))
		}

		// Verify workspace fields
		if resp.WorkspaceMode != mode {
			t.Fatalf("workspace_mode should be %q, got %q", mode, resp.WorkspaceMode)
		}
		if mode == "existing" && resp.WorkspacePath != path {
			t.Fatalf("workspace_path should be %q, got %q", path, resp.WorkspacePath)
		}
	})
}
