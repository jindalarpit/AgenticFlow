package daemon

import (
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Feature: task-workflow-stages, Property 6: Prior Stage Context Accumulation
//
// For any stage at position N in a workflow, when it is executed, the context
// provided to the agent SHALL include the output_content from all stages at
// positions 1..N-1 that have status "approved" or "completed".
//
// **Validates: Requirements 2.4, 3.3, 3.4, 3.5**
// ---------------------------------------------------------------------------

// canonicalStages defines the fixed stage ordering used in workflows.
var canonicalStages = []struct {
	Name  string
	Order int
}{
	{"plan", 1},
	{"design", 2},
	{"tasks", 3},
	{"execution", 4},
}

func TestProperty6_PriorStageContextAccumulation(t *testing.T) {
	// Feature: task-workflow-stages, Property 6: Prior Stage Context Accumulation
	//
	// Generate workflows with 2-4 stages at various positions (stage N being executed).
	// Simulate prior stages (positions 1..N-1) having approved/completed status with output content.
	// Call buildStagePrompt and verify the resulting prompt includes ALL prior stage outputs.
	// Verify that the prompt does NOT include outputs from stages at position N or later.

	rapid.Check(t, func(t *rapid.T) {
		// Generate a workflow with 2-4 stages.
		numStages := rapid.IntRange(2, 4).Draw(t, "numStages")

		// Select numStages stages from the canonical set in order.
		// We pick a random subset of size numStages from indices [0..3].
		indices := []int{0, 1, 2, 3}
		// Shuffle and take first numStages
		for i := len(indices) - 1; i > 0; i-- {
			j := rapid.IntRange(0, i).Draw(t, fmt.Sprintf("shuffle_%d", i))
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

		// Build the workflow stages with unique output content.
		type workflowStage struct {
			name   string
			order  int
			output string
		}
		stages := make([]workflowStage, numStages)
		for i, idx := range selected {
			stages[i] = workflowStage{
				name:  canonicalStages[idx].Name,
				order: canonicalStages[idx].Order,
				output: fmt.Sprintf("output_for_%s_%d",
					canonicalStages[idx].Name,
					rapid.IntRange(1000, 9999).Draw(t, fmt.Sprintf("outputId_%d", i))),
			}
		}

		// Pick which stage position N is being executed (1-indexed within the workflow).
		// N must be >= 2 so there's at least one prior stage.
		executingIdx := rapid.IntRange(1, numStages-1).Draw(t, "executingIdx")
		currentStageName := stages[executingIdx].name

		// Generate a task prompt.
		taskPrompt := rapid.StringMatching(`[a-zA-Z0-9 ]{5,50}`).Draw(t, "taskPrompt")

		// Build prior stages: all stages before executingIdx with approved/completed status.
		priorStages := make([]PriorStage, 0, executingIdx)
		for i := 0; i < executingIdx; i++ {
			status := rapid.SampledFrom([]string{"approved", "completed"}).Draw(t, fmt.Sprintf("priorStatus_%d", i))
			priorStages = append(priorStages, PriorStage{
				Name:          stages[i].name,
				Order:         stages[i].order,
				Status:        status,
				OutputContent: stages[i].output,
			})
		}

		// Call buildStagePrompt with no feedback (normal execution).
		result := buildStagePrompt(taskPrompt, currentStageName, priorStages, "")

		// Property 6a: The prompt MUST include the output_content from ALL prior stages
		// (positions 1..N-1) that are relevant to the current stage type.
		// The relevance depends on the stage type:
		//   - plan: no prior context needed
		//   - design: includes plan output
		//   - tasks: includes plan + design outputs
		//   - execution: includes plan + design + tasks outputs
		expectedPriorNames := priorStageNamesForStage(currentStageName)
		for _, ps := range priorStages {
			if containsStr(expectedPriorNames, ps.Name) && ps.OutputContent != "" {
				if !strings.Contains(result, ps.OutputContent) {
					t.Fatalf("prompt for stage %q at position %d missing prior stage %q output %q.\nPrompt:\n%s",
						currentStageName, executingIdx, ps.Name, ps.OutputContent, result)
				}
			}
		}

		// Property 6b: The prompt must NOT include outputs from stages at position N or later.
		for i := executingIdx; i < numStages; i++ {
			// The output of stages at or after the current position should NOT appear.
			if strings.Contains(result, stages[i].output) {
				t.Fatalf("prompt for stage %q at position %d incorrectly includes output from stage %q (position %d): %q.\nPrompt:\n%s",
					currentStageName, executingIdx, stages[i].name, i, stages[i].output, result)
			}
		}

		// Property 6c: The prompt MUST include the original task prompt.
		if !strings.Contains(result, taskPrompt) {
			t.Fatalf("prompt for stage %q missing original task prompt %q.\nPrompt:\n%s",
				currentStageName, taskPrompt, result)
		}
	})
}

func TestProperty6_PriorStageContextAccumulation_AllPriorIncluded(t *testing.T) {
	// Feature: task-workflow-stages, Property 6: Prior Stage Context Accumulation
	//
	// Specifically test that for the "execution" stage (which includes all prior outputs),
	// every prior stage's output is present in the prompt.

	rapid.Check(t, func(t *rapid.T) {
		// Generate a full 4-stage workflow: plan → design → tasks → execution.
		taskPrompt := rapid.StringMatching(`[a-zA-Z0-9 ]{10,80}`).Draw(t, "taskPrompt")

		// Generate unique outputs for each prior stage.
		planOutput := fmt.Sprintf("PLAN_OUTPUT_%d", rapid.IntRange(10000, 99999).Draw(t, "planId"))
		designOutput := fmt.Sprintf("DESIGN_OUTPUT_%d", rapid.IntRange(10000, 99999).Draw(t, "designId"))
		tasksOutput := fmt.Sprintf("TASKS_OUTPUT_%d", rapid.IntRange(10000, 99999).Draw(t, "tasksId"))

		priorStages := []PriorStage{
			{Name: "plan", Order: 1, Status: "approved", OutputContent: planOutput},
			{Name: "design", Order: 2, Status: "completed", OutputContent: designOutput},
			{Name: "tasks", Order: 3, Status: "approved", OutputContent: tasksOutput},
		}

		// Build prompt for the execution stage (position 4, all priors included).
		result := buildStagePrompt(taskPrompt, "execution", priorStages, "")

		// All three prior outputs MUST be present.
		if !strings.Contains(result, planOutput) {
			t.Fatalf("execution stage prompt missing plan output %q.\nPrompt:\n%s", planOutput, result)
		}
		if !strings.Contains(result, designOutput) {
			t.Fatalf("execution stage prompt missing design output %q.\nPrompt:\n%s", designOutput, result)
		}
		if !strings.Contains(result, tasksOutput) {
			t.Fatalf("execution stage prompt missing tasks output %q.\nPrompt:\n%s", tasksOutput, result)
		}

		// The original task prompt MUST be present.
		if !strings.Contains(result, taskPrompt) {
			t.Fatalf("execution stage prompt missing task prompt %q.\nPrompt:\n%s", taskPrompt, result)
		}
	})
}

func TestProperty6_PriorStageContextAccumulation_DesignIncludesPlanOnly(t *testing.T) {
	// Feature: task-workflow-stages, Property 6: Prior Stage Context Accumulation
	//
	// For the "design" stage, only the plan output should be included.
	// Outputs from "tasks" or "execution" stages (if somehow present) should NOT appear.

	rapid.Check(t, func(t *rapid.T) {
		taskPrompt := rapid.StringMatching(`[a-zA-Z0-9 ]{10,80}`).Draw(t, "taskPrompt")

		planOutput := fmt.Sprintf("PLAN_CONTENT_%d", rapid.IntRange(10000, 99999).Draw(t, "planId"))
		tasksOutput := fmt.Sprintf("TASKS_CONTENT_%d", rapid.IntRange(10000, 99999).Draw(t, "tasksId"))

		// Provide prior stages including one that shouldn't be used (tasks).
		priorStages := []PriorStage{
			{Name: "plan", Order: 1, Status: "approved", OutputContent: planOutput},
			{Name: "tasks", Order: 3, Status: "approved", OutputContent: tasksOutput},
		}

		result := buildStagePrompt(taskPrompt, "design", priorStages, "")

		// Plan output MUST be included for design stage.
		if !strings.Contains(result, planOutput) {
			t.Fatalf("design stage prompt missing plan output %q.\nPrompt:\n%s", planOutput, result)
		}

		// Tasks output must NOT be included (design only uses plan).
		if strings.Contains(result, tasksOutput) {
			t.Fatalf("design stage prompt incorrectly includes tasks output %q.\nPrompt:\n%s", tasksOutput, result)
		}
	})
}

func TestProperty6_PriorStageContextAccumulation_TasksIncludesPlanAndDesign(t *testing.T) {
	// Feature: task-workflow-stages, Property 6: Prior Stage Context Accumulation
	//
	// For the "tasks" stage, plan and design outputs should be included,
	// but execution output (if somehow present) should NOT appear.

	rapid.Check(t, func(t *rapid.T) {
		taskPrompt := rapid.StringMatching(`[a-zA-Z0-9 ]{10,80}`).Draw(t, "taskPrompt")

		planOutput := fmt.Sprintf("PLAN_DATA_%d", rapid.IntRange(10000, 99999).Draw(t, "planId"))
		designOutput := fmt.Sprintf("DESIGN_DATA_%d", rapid.IntRange(10000, 99999).Draw(t, "designId"))
		executionOutput := fmt.Sprintf("EXEC_DATA_%d", rapid.IntRange(10000, 99999).Draw(t, "execId"))

		priorStages := []PriorStage{
			{Name: "plan", Order: 1, Status: "completed", OutputContent: planOutput},
			{Name: "design", Order: 2, Status: "approved", OutputContent: designOutput},
			{Name: "execution", Order: 4, Status: "approved", OutputContent: executionOutput},
		}

		result := buildStagePrompt(taskPrompt, "tasks", priorStages, "")

		// Plan and design outputs MUST be included.
		if !strings.Contains(result, planOutput) {
			t.Fatalf("tasks stage prompt missing plan output %q.\nPrompt:\n%s", planOutput, result)
		}
		if !strings.Contains(result, designOutput) {
			t.Fatalf("tasks stage prompt missing design output %q.\nPrompt:\n%s", designOutput, result)
		}

		// Execution output must NOT be included.
		if strings.Contains(result, executionOutput) {
			t.Fatalf("tasks stage prompt incorrectly includes execution output %q.\nPrompt:\n%s", executionOutput, result)
		}
	})
}

func TestProperty6_PriorStageContextAccumulation_PlanHasNoPriorContext(t *testing.T) {
	// Feature: task-workflow-stages, Property 6: Prior Stage Context Accumulation
	//
	// For the "plan" stage (position 1), no prior stage outputs should be included,
	// even if prior stages are provided (edge case).

	rapid.Check(t, func(t *rapid.T) {
		taskPrompt := rapid.StringMatching(`[a-zA-Z0-9 ]{10,80}`).Draw(t, "taskPrompt")

		// Even if we pass prior stages, plan should not include them.
		designOutput := fmt.Sprintf("DESIGN_EXTRA_%d", rapid.IntRange(10000, 99999).Draw(t, "designId"))
		tasksOutput := fmt.Sprintf("TASKS_EXTRA_%d", rapid.IntRange(10000, 99999).Draw(t, "tasksId"))

		priorStages := []PriorStage{
			{Name: "design", Order: 2, Status: "approved", OutputContent: designOutput},
			{Name: "tasks", Order: 3, Status: "approved", OutputContent: tasksOutput},
		}

		result := buildStagePrompt(taskPrompt, "plan", priorStages, "")

		// Plan stage should NOT include any prior stage outputs.
		if strings.Contains(result, designOutput) {
			t.Fatalf("plan stage prompt incorrectly includes design output %q.\nPrompt:\n%s", designOutput, result)
		}
		if strings.Contains(result, tasksOutput) {
			t.Fatalf("plan stage prompt incorrectly includes tasks output %q.\nPrompt:\n%s", tasksOutput, result)
		}

		// But the task prompt MUST be present.
		if !strings.Contains(result, taskPrompt) {
			t.Fatalf("plan stage prompt missing task prompt %q.\nPrompt:\n%s", taskPrompt, result)
		}
	})
}

func TestProperty6_PriorStageContextAccumulation_EmptyOutputsExcluded(t *testing.T) {
	// Feature: task-workflow-stages, Property 6: Prior Stage Context Accumulation
	//
	// Prior stages with empty output_content should not add any context markers
	// to the prompt. Only non-empty outputs are included.

	rapid.Check(t, func(t *rapid.T) {
		taskPrompt := rapid.StringMatching(`[a-zA-Z0-9 ]{10,80}`).Draw(t, "taskPrompt")

		// Create prior stages where some have empty output.
		planOutput := fmt.Sprintf("PLAN_REAL_%d", rapid.IntRange(10000, 99999).Draw(t, "planId"))

		priorStages := []PriorStage{
			{Name: "plan", Order: 1, Status: "approved", OutputContent: planOutput},
			{Name: "design", Order: 2, Status: "approved", OutputContent: ""}, // empty
		}

		result := buildStagePrompt(taskPrompt, "tasks", priorStages, "")

		// Plan output MUST be included.
		if !strings.Contains(result, planOutput) {
			t.Fatalf("tasks stage prompt missing plan output %q.\nPrompt:\n%s", planOutput, result)
		}

		// The design stage marker should NOT appear since output is empty.
		if strings.Contains(result, "design.md (from prior stage)") {
			t.Fatalf("tasks stage prompt includes design stage marker despite empty output.\nPrompt:\n%s", result)
		}
	})
}

// priorStageNamesForStage returns the stage names whose outputs should be included
// as context for the given stage type, based on the design specification.
func priorStageNamesForStage(stageName string) []string {
	switch stageName {
	case "plan":
		return nil // plan has no prior context
	case "design":
		return []string{"plan"}
	case "tasks":
		return []string{"plan", "design"}
	case "execution":
		return []string{"plan", "design", "tasks"}
	default:
		return []string{"plan", "design", "tasks"}
	}
}

// containsStr checks if a string slice contains a given string.
func containsStr(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
