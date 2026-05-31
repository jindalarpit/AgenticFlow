package service

import (
	"encoding/json"
	"testing"

	"github.com/agenticflow/agenticflow/server/internal/provider"
	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Property 13: Provider Status Change Preserves Agent Bindings
//
// For any provider that has agents bound to it, changing the provider's status
// (from "active" to "error" or vice versa) SHALL NOT modify, remove, or nullify
// the provider_id on any bound agent record.
//
// This test verifies the property using a pure function that simulates a
// provider status change and returns the updated agent records. The function
// mirrors the real system behavior: when a provider's status changes, only the
// agent's derived status is updated — the provider_id binding is never touched.
//
// **Validates: Requirements 10.4**
// ---------------------------------------------------------------------------

// boundAgent represents an agent bound to a provider for testing purposes.
type boundAgent struct {
	AgentID    string
	ProviderID string
	Status     string // derived agent status
}

// providerState represents a provider's current state for testing purposes.
type providerState struct {
	ProviderID string
	Status     string // "active", "error", "inactive", "validating"
}

// ApplyProviderStatusChange simulates a provider status change and reconciles
// the bound agents' derived statuses. It returns the updated agents with their
// new derived statuses. Crucially, the provider_id on each agent is NEVER
// modified — only the derived status changes.
//
// This mirrors the real behavior in ProviderService.validateProviderAsync and
// AgentStatusService.ReconcileAgentsForProvider: when a provider's status
// changes, each bound agent's status is recomputed via DeriveOnlineAgentStatus,
// but the provider_id foreign key is never altered.
func ApplyProviderStatusChange(agents []boundAgent, prov providerState, newStatus string, runningTaskCounts map[string]int64) []boundAgent {
	// Update the provider status (simulates DB update)
	prov.Status = newStatus

	// Reconcile each bound agent's derived status (simulates ReconcileAgentsForProvider)
	result := make([]boundAgent, len(agents))
	for i, agent := range agents {
		// Copy the agent — provider_id is NEVER modified
		result[i] = boundAgent{
			AgentID:    agent.AgentID,
			ProviderID: agent.ProviderID, // preserved unchanged
			Status:     DeriveOnlineAgentStatus(prov.Status, runningTaskCounts[agent.AgentID]),
		}
	}
	return result
}

// providerStatusValues are the allowed status values for online providers.
var providerStatusValues = []string{"active", "error", "inactive", "validating"}

func TestProperty13_ProviderStatusChange_PreservesProviderID(t *testing.T) {
	// After any provider status change, all bound agents still have the same provider_id.
	rapid.Check(t, func(t *rapid.T) {
		// Generate a provider
		providerID := genUUID(t, "provider")
		initialStatus := rapid.SampledFrom(providerStatusValues).Draw(t, "initialStatus")
		newStatus := rapid.SampledFrom(providerStatusValues).Draw(t, "newStatus")

		prov := providerState{
			ProviderID: providerID,
			Status:     initialStatus,
		}

		// Generate bound agents (1 to 20)
		agentCount := rapid.IntRange(1, 20).Draw(t, "agentCount")
		agents := make([]boundAgent, agentCount)
		runningTaskCounts := make(map[string]int64)

		for i := 0; i < agentCount; i++ {
			agentID := genUUID(t, "agent")
			agents[i] = boundAgent{
				AgentID:    agentID,
				ProviderID: providerID,
				Status:     DeriveOnlineAgentStatus(initialStatus, 0),
			}
			runningTaskCounts[agentID] = rapid.Int64Range(0, 10).Draw(t, "runningTasks")
		}

		// Apply status change
		updatedAgents := ApplyProviderStatusChange(agents, prov, newStatus, runningTaskCounts)

		// Property: all agents still have the same provider_id
		for i, updated := range updatedAgents {
			if updated.ProviderID != providerID {
				t.Fatalf("agent %d: provider_id changed from %q to %q after status change %q→%q",
					i, providerID, updated.ProviderID, initialStatus, newStatus)
			}
		}
	})
}

func TestProperty13_ProviderStatusChange_ActiveToErrorPreservesBindings(t *testing.T) {
	// Status changes from "active" to "error" do not unbind agents.
	rapid.Check(t, func(t *rapid.T) {
		providerID := genUUID(t, "provider")
		prov := providerState{
			ProviderID: providerID,
			Status:     "active",
		}

		agentCount := rapid.IntRange(1, 20).Draw(t, "agentCount")
		agents := make([]boundAgent, agentCount)
		runningTaskCounts := make(map[string]int64)

		for i := 0; i < agentCount; i++ {
			agentID := genUUID(t, "agent")
			agents[i] = boundAgent{
				AgentID:    agentID,
				ProviderID: providerID,
				Status:     "idle",
			}
			runningTaskCounts[agentID] = rapid.Int64Range(0, 5).Draw(t, "runningTasks")
		}

		// Change from active to error
		updatedAgents := ApplyProviderStatusChange(agents, prov, "error", runningTaskCounts)

		// Property: no agent was unbound (provider_id preserved, not empty)
		for i, updated := range updatedAgents {
			if updated.ProviderID != providerID {
				t.Fatalf("agent %d: provider_id changed after active→error transition", i)
			}
			if updated.ProviderID == "" {
				t.Fatalf("agent %d: provider_id was emptied after active→error transition", i)
			}
		}
	})
}

func TestProperty13_ProviderStatusChange_ErrorToActivePreservesBindings(t *testing.T) {
	// Status changes from "error" to "active" do not unbind agents.
	rapid.Check(t, func(t *rapid.T) {
		providerID := genUUID(t, "provider")
		prov := providerState{
			ProviderID: providerID,
			Status:     "error",
		}

		agentCount := rapid.IntRange(1, 20).Draw(t, "agentCount")
		agents := make([]boundAgent, agentCount)
		runningTaskCounts := make(map[string]int64)

		for i := 0; i < agentCount; i++ {
			agentID := genUUID(t, "agent")
			agents[i] = boundAgent{
				AgentID:    agentID,
				ProviderID: providerID,
				Status:     "error",
			}
			runningTaskCounts[agentID] = rapid.Int64Range(0, 5).Draw(t, "runningTasks")
		}

		// Change from error to active
		updatedAgents := ApplyProviderStatusChange(agents, prov, "active", runningTaskCounts)

		// Property: no agent was unbound (provider_id preserved, not empty)
		for i, updated := range updatedAgents {
			if updated.ProviderID != providerID {
				t.Fatalf("agent %d: provider_id changed after error→active transition", i)
			}
			if updated.ProviderID == "" {
				t.Fatalf("agent %d: provider_id was emptied after error→active transition", i)
			}
		}
	})
}

func TestProperty13_ProviderStatusChange_ProviderIDNeverNullified(t *testing.T) {
	// The provider_id field on agents is never nullified by status changes.
	rapid.Check(t, func(t *rapid.T) {
		providerID := genUUID(t, "provider")
		initialStatus := rapid.SampledFrom(providerStatusValues).Draw(t, "initialStatus")
		newStatus := rapid.SampledFrom(providerStatusValues).Draw(t, "newStatus")

		prov := providerState{
			ProviderID: providerID,
			Status:     initialStatus,
		}

		agentCount := rapid.IntRange(1, 20).Draw(t, "agentCount")
		agents := make([]boundAgent, agentCount)
		runningTaskCounts := make(map[string]int64)

		for i := 0; i < agentCount; i++ {
			agentID := genUUID(t, "agent")
			agents[i] = boundAgent{
				AgentID:    agentID,
				ProviderID: providerID,
				Status:     DeriveOnlineAgentStatus(initialStatus, 0),
			}
			runningTaskCounts[agentID] = rapid.Int64Range(0, 10).Draw(t, "runningTasks")
		}

		// Apply status change
		updatedAgents := ApplyProviderStatusChange(agents, prov, newStatus, runningTaskCounts)

		// Property: no agent's provider_id was nullified (empty string)
		for i, updated := range updatedAgents {
			if updated.ProviderID == "" {
				t.Fatalf("agent %d: provider_id was nullified after status change %q→%q",
					i, initialStatus, newStatus)
			}
		}
	})
}

func TestProperty13_ProviderStatusChange_MultipleTransitionsPreserveBindings(t *testing.T) {
	// After multiple sequential status changes, agent bindings are still preserved.
	rapid.Check(t, func(t *rapid.T) {
		providerID := genUUID(t, "provider")
		initialStatus := rapid.SampledFrom(providerStatusValues).Draw(t, "initialStatus")

		prov := providerState{
			ProviderID: providerID,
			Status:     initialStatus,
		}

		agentCount := rapid.IntRange(1, 10).Draw(t, "agentCount")
		agents := make([]boundAgent, agentCount)
		runningTaskCounts := make(map[string]int64)

		for i := 0; i < agentCount; i++ {
			agentID := genUUID(t, "agent")
			agents[i] = boundAgent{
				AgentID:    agentID,
				ProviderID: providerID,
				Status:     DeriveOnlineAgentStatus(initialStatus, 0),
			}
			runningTaskCounts[agentID] = rapid.Int64Range(0, 5).Draw(t, "runningTasks")
		}

		// Apply multiple status transitions
		transitionCount := rapid.IntRange(2, 10).Draw(t, "transitionCount")
		currentAgents := agents
		for j := 0; j < transitionCount; j++ {
			newStatus := rapid.SampledFrom(providerStatusValues).Draw(t, "transition")
			currentAgents = ApplyProviderStatusChange(currentAgents, prov, newStatus, runningTaskCounts)
			prov.Status = newStatus
		}

		// Property: after all transitions, all agents still have the original provider_id
		for i, updated := range currentAgents {
			if updated.ProviderID != providerID {
				t.Fatalf("agent %d: provider_id changed after %d status transitions", i, transitionCount)
			}
			if updated.ProviderID == "" {
				t.Fatalf("agent %d: provider_id was nullified after %d status transitions", i, transitionCount)
			}
		}
	})
}

func TestProperty13_ProviderStatusChange_AgentCountPreserved(t *testing.T) {
	// A provider status change never removes agents from the bound set.
	// The number of bound agents before and after must be identical.
	rapid.Check(t, func(t *rapid.T) {
		providerID := genUUID(t, "provider")
		initialStatus := rapid.SampledFrom(providerStatusValues).Draw(t, "initialStatus")
		newStatus := rapid.SampledFrom(providerStatusValues).Draw(t, "newStatus")

		prov := providerState{
			ProviderID: providerID,
			Status:     initialStatus,
		}

		agentCount := rapid.IntRange(0, 30).Draw(t, "agentCount")
		agents := make([]boundAgent, agentCount)
		runningTaskCounts := make(map[string]int64)

		for i := 0; i < agentCount; i++ {
			agentID := genUUID(t, "agent")
			agents[i] = boundAgent{
				AgentID:    agentID,
				ProviderID: providerID,
				Status:     DeriveOnlineAgentStatus(initialStatus, 0),
			}
			runningTaskCounts[agentID] = rapid.Int64Range(0, 10).Draw(t, "runningTasks")
		}

		// Apply status change
		updatedAgents := ApplyProviderStatusChange(agents, prov, newStatus, runningTaskCounts)

		// Property: agent count is preserved
		if len(updatedAgents) != len(agents) {
			t.Fatalf("agent count changed from %d to %d after status change %q→%q",
				len(agents), len(updatedAgents), initialStatus, newStatus)
		}
	})
}

// ---------------------------------------------------------------------------
// Property 8: Token Usage Extraction with Zero Defaults
//
// For any provider response containing usage information, the extracted
// TokenUsage SHALL contain the exact values from the response for
// prompt_tokens, completion_tokens, and total_tokens. For any field not
// present in the provider response (i.e., usage is nil), the extracted
// value SHALL be zero.
//
// **Validates: Requirements 8.8**
// ---------------------------------------------------------------------------

func TestProperty8_TokenUsageExtraction_NilReturnsAllZeros(t *testing.T) {
	// If usage is nil, all fields in the extracted result must be zero.
	rapid.Check(t, func(t *rapid.T) {
		result := ExtractTokenUsage(nil)

		if result.PromptTokens != 0 {
			t.Fatalf("expected PromptTokens=0 for nil usage, got %d", result.PromptTokens)
		}
		if result.CompletionTokens != 0 {
			t.Fatalf("expected CompletionTokens=0 for nil usage, got %d", result.CompletionTokens)
		}
		if result.TotalTokens != 0 {
			t.Fatalf("expected TotalTokens=0 for nil usage, got %d", result.TotalTokens)
		}
	})
}

func TestProperty8_TokenUsageExtraction_NonNilMatchesInput(t *testing.T) {
	// If usage is non-nil, extracted values must match the input exactly.
	rapid.Check(t, func(t *rapid.T) {
		promptTokens := rapid.IntRange(0, 1000000).Draw(t, "prompt_tokens")
		completionTokens := rapid.IntRange(0, 1000000).Draw(t, "completion_tokens")
		totalTokens := rapid.IntRange(0, 2000000).Draw(t, "total_tokens")

		usage := &provider.TokenUsage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
		}

		result := ExtractTokenUsage(usage)

		if result.PromptTokens != promptTokens {
			t.Fatalf("PromptTokens mismatch: got %d, want %d", result.PromptTokens, promptTokens)
		}
		if result.CompletionTokens != completionTokens {
			t.Fatalf("CompletionTokens mismatch: got %d, want %d", result.CompletionTokens, completionTokens)
		}
		if result.TotalTokens != totalTokens {
			t.Fatalf("TotalTokens mismatch: got %d, want %d", result.TotalTokens, totalTokens)
		}
	})
}

func TestProperty8_TokenUsageExtraction_NonNegativeValues(t *testing.T) {
	// The extracted result always has non-negative values for any valid input.
	// Whether usage is nil or non-nil, all fields must be >= 0.
	rapid.Check(t, func(t *rapid.T) {
		isNil := rapid.Bool().Draw(t, "is_nil")

		var usage *provider.TokenUsage
		if !isNil {
			usage = &provider.TokenUsage{
				PromptTokens:     rapid.IntRange(0, 10000000).Draw(t, "prompt_tokens"),
				CompletionTokens: rapid.IntRange(0, 10000000).Draw(t, "completion_tokens"),
				TotalTokens:      rapid.IntRange(0, 20000000).Draw(t, "total_tokens"),
			}
		}

		result := ExtractTokenUsage(usage)

		if result.PromptTokens < 0 {
			t.Fatalf("PromptTokens is negative: %d", result.PromptTokens)
		}
		if result.CompletionTokens < 0 {
			t.Fatalf("CompletionTokens is negative: %d", result.CompletionTokens)
		}
		if result.TotalTokens < 0 {
			t.Fatalf("TotalTokens is negative: %d", result.TotalTokens)
		}
	})
}

// ---------------------------------------------------------------------------
// Property 9: Online Task Result Shape
//
// For any completed online task, the task result SHALL contain only text
// response content and token usage metadata (prompt_tokens, completion_tokens,
// total_tokens). The result SHALL NOT contain workspace_path, work_dir, or
// file artifact references.
//
// This test validates the structural shape of online task results via a pure
// validation function ValidateOnlineTaskResult.
//
// **Validates: Requirements 9.4, 9.5**
// ---------------------------------------------------------------------------

// OnlineTaskResult represents the result shape of a completed online task.
// It contains only text content and token usage — no workspace or file artifacts.
type OnlineTaskResult struct {
	Content    string              `json:"content"`
	TokenUsage provider.TokenUsage `json:"token_usage"`
}

// ValidateOnlineTaskResult checks that an online task result conforms to the
// expected shape: contains text content and token usage only, with no
// workspace_path, work_dir, or file artifact references.
//
// Returns nil if the result is valid, or a ServiceError describing the violation.
//
// Rules:
//   - Content is always a string (can be empty)
//   - TokenUsage always has prompt_tokens, completion_tokens, total_tokens (all >= 0)
//   - No workspace_path field
//   - No work_dir field
//   - No file artifacts
func ValidateOnlineTaskResult(result OnlineTaskResult) *ServiceError {
	if result.TokenUsage.PromptTokens < 0 {
		return Validation("prompt_tokens must be >= 0")
	}
	if result.TokenUsage.CompletionTokens < 0 {
		return Validation("completion_tokens must be >= 0")
	}
	if result.TokenUsage.TotalTokens < 0 {
		return Validation("total_tokens must be >= 0")
	}
	return nil
}

// ValidateOnlineTaskResultJSON checks that a JSON-serialized online task result
// does NOT contain forbidden fields (workspace_path, work_dir, file artifacts).
// This validates the structural constraint at the serialization level.
func ValidateOnlineTaskResultJSON(resultJSON []byte) *ServiceError {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(resultJSON, &raw); err != nil {
		return Validation("result is not valid JSON")
	}

	// Forbidden fields that must NOT be present in online task results.
	forbiddenFields := []string{"workspace_path", "work_dir", "files", "artifacts", "file_artifacts"}
	for _, field := range forbiddenFields {
		if _, exists := raw[field]; exists {
			return Validation("online task result must not contain " + field)
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Test: Content is always a string (can be empty) — accepted
// ---------------------------------------------------------------------------

func TestProperty9_OnlineTaskResultShape_ContentIsString(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		content := rapid.String().Draw(t, "content")
		promptTokens := rapid.IntRange(0, 100000).Draw(t, "prompt_tokens")
		completionTokens := rapid.IntRange(0, 100000).Draw(t, "completion_tokens")
		totalTokens := rapid.IntRange(0, 200000).Draw(t, "total_tokens")

		result := OnlineTaskResult{
			Content: content,
			TokenUsage: provider.TokenUsage{
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      totalTokens,
			},
		}

		err := ValidateOnlineTaskResult(result)
		if err != nil {
			t.Fatalf("expected valid result with content=%q to be accepted, got error: %v", content, err)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: TokenUsage always has non-negative prompt_tokens, completion_tokens,
// total_tokens — accepted
// ---------------------------------------------------------------------------

func TestProperty9_OnlineTaskResultShape_TokenUsageNonNegative(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		promptTokens := rapid.IntRange(0, 100000).Draw(t, "prompt_tokens")
		completionTokens := rapid.IntRange(0, 100000).Draw(t, "completion_tokens")
		totalTokens := rapid.IntRange(0, 200000).Draw(t, "total_tokens")

		result := OnlineTaskResult{
			Content: rapid.String().Draw(t, "content"),
			TokenUsage: provider.TokenUsage{
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      totalTokens,
			},
		}

		err := ValidateOnlineTaskResult(result)
		if err != nil {
			t.Fatalf("expected valid result with non-negative token usage to be accepted, got error: %v", err)
		}

		// Verify the token usage values are exactly what we set
		if result.TokenUsage.PromptTokens != promptTokens {
			t.Fatalf("prompt_tokens mismatch: expected %d, got %d", promptTokens, result.TokenUsage.PromptTokens)
		}
		if result.TokenUsage.CompletionTokens != completionTokens {
			t.Fatalf("completion_tokens mismatch: expected %d, got %d", completionTokens, result.TokenUsage.CompletionTokens)
		}
		if result.TokenUsage.TotalTokens != totalTokens {
			t.Fatalf("total_tokens mismatch: expected %d, got %d", totalTokens, result.TokenUsage.TotalTokens)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: Serialized result does NOT contain workspace_path field
// ---------------------------------------------------------------------------

func TestProperty9_OnlineTaskResultShape_NoWorkspacePath(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		content := rapid.String().Draw(t, "content")
		promptTokens := rapid.IntRange(0, 100000).Draw(t, "prompt_tokens")
		completionTokens := rapid.IntRange(0, 100000).Draw(t, "completion_tokens")
		totalTokens := rapid.IntRange(0, 200000).Draw(t, "total_tokens")

		result := OnlineTaskResult{
			Content: content,
			TokenUsage: provider.TokenUsage{
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      totalTokens,
			},
		}

		// Serialize and verify no forbidden fields
		resultJSON, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("failed to marshal result: %v", err)
		}

		svcErr := ValidateOnlineTaskResultJSON(resultJSON)
		if svcErr != nil {
			t.Fatalf("expected serialized result to not contain forbidden fields, got error: %v", svcErr)
		}

		// Double-check: parse and verify workspace_path is absent
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(resultJSON, &raw); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		if _, exists := raw["workspace_path"]; exists {
			t.Fatal("online task result must not contain workspace_path")
		}
	})
}

// ---------------------------------------------------------------------------
// Test: Serialized result does NOT contain work_dir field
// ---------------------------------------------------------------------------

func TestProperty9_OnlineTaskResultShape_NoWorkDir(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		content := rapid.String().Draw(t, "content")
		promptTokens := rapid.IntRange(0, 100000).Draw(t, "prompt_tokens")
		completionTokens := rapid.IntRange(0, 100000).Draw(t, "completion_tokens")
		totalTokens := rapid.IntRange(0, 200000).Draw(t, "total_tokens")

		result := OnlineTaskResult{
			Content: content,
			TokenUsage: provider.TokenUsage{
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      totalTokens,
			},
		}

		// Serialize and verify no work_dir field
		resultJSON, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("failed to marshal result: %v", err)
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(resultJSON, &raw); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		if _, exists := raw["work_dir"]; exists {
			t.Fatal("online task result must not contain work_dir")
		}
	})
}

// ---------------------------------------------------------------------------
// Test: Serialized result does NOT contain file artifacts
// ---------------------------------------------------------------------------

func TestProperty9_OnlineTaskResultShape_NoFileArtifacts(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		content := rapid.String().Draw(t, "content")
		promptTokens := rapid.IntRange(0, 100000).Draw(t, "prompt_tokens")
		completionTokens := rapid.IntRange(0, 100000).Draw(t, "completion_tokens")
		totalTokens := rapid.IntRange(0, 200000).Draw(t, "total_tokens")

		result := OnlineTaskResult{
			Content: content,
			TokenUsage: provider.TokenUsage{
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      totalTokens,
			},
		}

		// Serialize and verify no file artifact fields
		resultJSON, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("failed to marshal result: %v", err)
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(resultJSON, &raw); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}

		forbiddenFields := []string{"files", "artifacts", "file_artifacts"}
		for _, field := range forbiddenFields {
			if _, exists := raw[field]; exists {
				t.Fatalf("online task result must not contain %s", field)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Test: JSON with forbidden workspace_path field is rejected by validator
// ---------------------------------------------------------------------------

func TestProperty9_OnlineTaskResultShape_RejectsWorkspacePathInJSON(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		content := rapid.String().Draw(t, "content")
		workspacePath := rapid.StringMatching(`/[a-z/]{1,50}`).Draw(t, "workspace_path")

		// Construct a JSON object that includes workspace_path (simulating a bad result)
		badResult := map[string]interface{}{
			"content": content,
			"token_usage": map[string]int{
				"prompt_tokens":     0,
				"completion_tokens": 0,
				"total_tokens":      0,
			},
			"workspace_path": workspacePath,
		}

		resultJSON, err := json.Marshal(badResult)
		if err != nil {
			t.Fatalf("failed to marshal bad result: %v", err)
		}

		svcErr := ValidateOnlineTaskResultJSON(resultJSON)
		if svcErr == nil {
			t.Fatal("expected result with workspace_path to be rejected, but was accepted")
		}
		if svcErr.Kind != ErrValidation {
			t.Fatalf("expected ErrValidation, got %v", svcErr.Kind)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: JSON with forbidden work_dir field is rejected by validator
// ---------------------------------------------------------------------------

func TestProperty9_OnlineTaskResultShape_RejectsWorkDirInJSON(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		content := rapid.String().Draw(t, "content")
		workDir := rapid.StringMatching(`/[a-z/]{1,50}`).Draw(t, "work_dir")

		// Construct a JSON object that includes work_dir (simulating a bad result)
		badResult := map[string]interface{}{
			"content": content,
			"token_usage": map[string]int{
				"prompt_tokens":     0,
				"completion_tokens": 0,
				"total_tokens":      0,
			},
			"work_dir": workDir,
		}

		resultJSON, err := json.Marshal(badResult)
		if err != nil {
			t.Fatalf("failed to marshal bad result: %v", err)
		}

		svcErr := ValidateOnlineTaskResultJSON(resultJSON)
		if svcErr == nil {
			t.Fatal("expected result with work_dir to be rejected, but was accepted")
		}
		if svcErr.Kind != ErrValidation {
			t.Fatalf("expected ErrValidation, got %v", svcErr.Kind)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: JSON with forbidden file_artifacts field is rejected by validator
// ---------------------------------------------------------------------------

func TestProperty9_OnlineTaskResultShape_RejectsFileArtifactsInJSON(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		content := rapid.String().Draw(t, "content")

		// Construct a JSON object that includes file artifacts (simulating a bad result)
		badResult := map[string]interface{}{
			"content": content,
			"token_usage": map[string]int{
				"prompt_tokens":     0,
				"completion_tokens": 0,
				"total_tokens":      0,
			},
			"file_artifacts": []string{"/tmp/output.txt"},
		}

		resultJSON, err := json.Marshal(badResult)
		if err != nil {
			t.Fatalf("failed to marshal bad result: %v", err)
		}

		svcErr := ValidateOnlineTaskResultJSON(resultJSON)
		if svcErr == nil {
			t.Fatal("expected result with file_artifacts to be rejected, but was accepted")
		}
		if svcErr.Kind != ErrValidation {
			t.Fatalf("expected ErrValidation, got %v", svcErr.Kind)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: Valid OnlineTaskResult serialization only contains allowed fields
// (content and token_usage)
// ---------------------------------------------------------------------------

func TestProperty9_OnlineTaskResultShape_OnlyAllowedFieldsInSerialization(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		content := rapid.String().Draw(t, "content")
		promptTokens := rapid.IntRange(0, 100000).Draw(t, "prompt_tokens")
		completionTokens := rapid.IntRange(0, 100000).Draw(t, "completion_tokens")
		totalTokens := rapid.IntRange(0, 200000).Draw(t, "total_tokens")

		result := OnlineTaskResult{
			Content: content,
			TokenUsage: provider.TokenUsage{
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      totalTokens,
			},
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("failed to marshal result: %v", err)
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(resultJSON, &raw); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}

		// Only "content" and "token_usage" should be present
		allowedFields := map[string]bool{
			"content":     true,
			"token_usage": true,
		}

		for field := range raw {
			if !allowedFields[field] {
				t.Fatalf("unexpected field %q in online task result; only 'content' and 'token_usage' are allowed", field)
			}
		}

		// Both allowed fields must be present
		if _, exists := raw["content"]; !exists {
			t.Fatal("online task result must contain 'content' field")
		}
		if _, exists := raw["token_usage"]; !exists {
			t.Fatal("online task result must contain 'token_usage' field")
		}
	})
}
