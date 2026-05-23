package daemon

import (
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// Feature: agent-management, Property 14: Runtime_Brief Construction Completeness
// For any agent with non-empty instructions, the constructed Runtime_Brief SHALL
// contain: the agent's name (sanitized for markdown), the full instructions text,
// and the workspace context (if non-empty). When instructions are empty, the brief
// SHALL be empty string.
// **Validates: Requirements 13.2, 13.9**

func TestProperty_RuntimeBriefEmptyWhenInstructionsEmpty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary agent name and workspace context.
		agentName := rapid.String().Draw(t, "agentName")
		workspaceContext := rapid.String().Draw(t, "workspaceContext")

		// When instructions are empty, BuildRuntimeBrief must return empty string.
		result := BuildRuntimeBrief(agentName, "", workspaceContext)

		if result != "" {
			t.Fatalf("expected empty brief when instructions are empty, got:\n%s", result)
		}
	})
}

func TestProperty_RuntimeBriefContainsAgentName(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a non-empty agent name.
		agentName := rapid.StringMatching(`[a-zA-Z0-9][a-zA-Z0-9_\-]{0,63}`).Draw(t, "agentName")
		// Generate non-empty instructions.
		instructions := rapid.StringMatching(`.+`).Draw(t, "instructions")
		workspaceContext := rapid.String().Draw(t, "workspaceContext")

		result := BuildRuntimeBrief(agentName, instructions, workspaceContext)

		// The brief must contain the sanitized agent name.
		sanitized := sanitizeName(agentName)
		if !strings.Contains(result, sanitized) {
			t.Fatalf("brief does not contain sanitized agent name %q:\n%s", sanitized, result)
		}
	})
}

func TestProperty_RuntimeBriefContainsInstructions(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		agentName := rapid.String().Draw(t, "agentName")
		// Generate non-empty instructions (at least 1 char).
		instructions := rapid.StringMatching(`.+`).Draw(t, "instructions")
		workspaceContext := rapid.String().Draw(t, "workspaceContext")

		result := BuildRuntimeBrief(agentName, instructions, workspaceContext)

		// The brief must contain the full instructions text.
		if !strings.Contains(result, instructions) {
			t.Fatalf("brief does not contain full instructions text %q:\n%s", instructions, result)
		}
	})
}

func TestProperty_RuntimeBriefContainsWorkspaceContextWhenNonEmpty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		agentName := rapid.String().Draw(t, "agentName")
		// Generate non-empty instructions.
		instructions := rapid.StringMatching(`.+`).Draw(t, "instructions")
		// Generate non-empty workspace context.
		workspaceContext := rapid.StringMatching(`.+`).Draw(t, "workspaceContext")

		result := BuildRuntimeBrief(agentName, instructions, workspaceContext)

		// The brief must contain the workspace context.
		if !strings.Contains(result, workspaceContext) {
			t.Fatalf("brief does not contain workspace context %q:\n%s", workspaceContext, result)
		}
		// The brief must contain the "## Workspace Context" header.
		if !strings.Contains(result, "## Workspace Context") {
			t.Fatalf("brief missing '## Workspace Context' header when workspace context is non-empty:\n%s", result)
		}
	})
}

func TestProperty_RuntimeBriefOmitsWorkspaceContextWhenEmpty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		agentName := rapid.String().Draw(t, "agentName")
		// Generate non-empty instructions.
		instructions := rapid.StringMatching(`.+`).Draw(t, "instructions")

		// Empty workspace context.
		result := BuildRuntimeBrief(agentName, instructions, "")

		// The brief must NOT contain the "## Workspace Context" header.
		if strings.Contains(result, "## Workspace Context") {
			t.Fatalf("brief contains '## Workspace Context' header when workspace context is empty:\n%s", result)
		}
	})
}
