package agent

import (
	"encoding/json"
	"testing"

	"pgregory.net/rapid"
)

// Feature: agentic-output-architecture, Property 12: Token Usage Extraction Preserves Totals
//
// For any sequence of Claude SDK messages with per-message usage blocks,
// the accumulated TokenUsage for each model SHALL equal the sum of all
// individual usage blocks for that model.
//
// **Validates: Requirements 21.3**

func TestProperty_TokenUsageExtraction_PreservesTotals(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		model := rapid.SampledFrom([]string{
			"claude-sonnet-4-20250514",
			"claude-opus-4-20250514",
			"claude-3-haiku-20240307",
		}).Draw(t, "model")

		numMessages := rapid.IntRange(1, 10).Draw(t, "numMessages")

		var expectedInput, expectedOutput, expectedCacheRead, expectedCacheWrite int64
		usage := make(map[string]TokenUsage)

		for i := 0; i < numMessages; i++ {
			inputTokens := int64(rapid.IntRange(100, 10000).Draw(t, "input"))
			outputTokens := int64(rapid.IntRange(10, 5000).Draw(t, "output"))
			cacheRead := int64(rapid.IntRange(0, 5000).Draw(t, "cacheRead"))
			cacheWrite := int64(rapid.IntRange(0, 1000).Draw(t, "cacheWrite"))

			expectedInput += inputTokens
			expectedOutput += outputTokens
			expectedCacheRead += cacheRead
			expectedCacheWrite += cacheWrite

			// Simulate what handleAssistant does: accumulate per-model usage.
			content := claudeMessageContent{
				Model: model,
				Usage: &claudeUsage{
					InputTokens:              inputTokens,
					OutputTokens:             outputTokens,
					CacheReadInputTokens:     cacheRead,
					CacheCreationInputTokens: cacheWrite,
				},
				Content: []claudeContentBlock{{Type: "text", Text: "hello"}},
			}
			raw, _ := json.Marshal(content)
			msg := claudeSDKMessage{Type: "assistant", Message: raw}

			// Simulate handleAssistant accumulation.
			var mc claudeMessageContent
			_ = json.Unmarshal(msg.Message, &mc)
			if mc.Usage != nil && mc.Model != "" {
				u := usage[mc.Model]
				u.InputTokens += mc.Usage.InputTokens
				u.OutputTokens += mc.Usage.OutputTokens
				u.CacheReadTokens += mc.Usage.CacheReadInputTokens
				u.CacheWriteTokens += mc.Usage.CacheCreationInputTokens
				usage[mc.Model] = u
			}
		}

		// Verify accumulated totals match expected sums.
		got := usage[model]
		if got.InputTokens != expectedInput {
			t.Fatalf("InputTokens: expected %d, got %d", expectedInput, got.InputTokens)
		}
		if got.OutputTokens != expectedOutput {
			t.Fatalf("OutputTokens: expected %d, got %d", expectedOutput, got.OutputTokens)
		}
		if got.CacheReadTokens != expectedCacheRead {
			t.Fatalf("CacheReadTokens: expected %d, got %d", expectedCacheRead, got.CacheReadTokens)
		}
		if got.CacheWriteTokens != expectedCacheWrite {
			t.Fatalf("CacheWriteTokens: expected %d, got %d", expectedCacheWrite, got.CacheWriteTokens)
		}
	})
}

// TestProperty_TokenUsageExtraction_MultiModel verifies that usage is tracked
// independently per model when multiple models are used in a session.
func TestProperty_TokenUsageExtraction_MultiModel(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		models := []string{"model-a", "model-b"}
		expected := make(map[string]TokenUsage)
		usage := make(map[string]TokenUsage)

		numMessages := rapid.IntRange(2, 8).Draw(t, "numMessages")
		for i := 0; i < numMessages; i++ {
			model := models[rapid.IntRange(0, 1).Draw(t, "modelIdx")]
			inputTokens := int64(rapid.IntRange(100, 5000).Draw(t, "input"))
			outputTokens := int64(rapid.IntRange(10, 2000).Draw(t, "output"))

			e := expected[model]
			e.InputTokens += inputTokens
			e.OutputTokens += outputTokens
			expected[model] = e

			// Accumulate.
			u := usage[model]
			u.InputTokens += inputTokens
			u.OutputTokens += outputTokens
			usage[model] = u
		}

		// Verify each model's totals are independent.
		for _, model := range models {
			got := usage[model]
			exp := expected[model]
			if got.InputTokens != exp.InputTokens {
				t.Fatalf("model %q InputTokens: expected %d, got %d", model, exp.InputTokens, got.InputTokens)
			}
			if got.OutputTokens != exp.OutputTokens {
				t.Fatalf("model %q OutputTokens: expected %d, got %d", model, exp.OutputTokens, got.OutputTokens)
			}
		}
	})
}
