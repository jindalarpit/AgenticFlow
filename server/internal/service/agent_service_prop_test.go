package service

import (
	"strings"
	"testing"
	"unicode/utf8"

	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Property 16: Online Agent Requires Model
//
// For any agent creation or update request with runtime_mode "online", the
// model field SHALL be required (non-empty) and at most 100 characters. An
// empty model for an online agent SHALL always be rejected with a 422 status.
//
// For agents with runtime_mode "local", the model field is optional (can be
// empty) and any value up to 100 characters is accepted.
//
// This test verifies the validation logic in isolation via a pure helper
// function ValidateOnlineAgentModel.
//
// **Validates: Requirements 6.6**
// ---------------------------------------------------------------------------

// ValidateOnlineAgentModel validates the model field for agent creation/update
// based on the runtime mode. Returns a ServiceError if validation fails:
//   - runtime_mode "online" + empty model → rejected (422 Unprocessable)
//   - runtime_mode "online" + model > 100 chars → rejected (400 Validation)
//   - runtime_mode "online" + model 1-100 chars → accepted
//   - runtime_mode "local" + empty model → accepted (model is optional)
//   - runtime_mode "local" + model > 100 chars → rejected (400 Validation)
//   - runtime_mode "local" + model 1-100 chars → accepted
func ValidateOnlineAgentModel(runtimeMode, model string) *ServiceError {
	if runtimeMode == "online" {
		if model == "" {
			return Unprocessable("model is required for online agents")
		}
	}
	if utf8.RuneCountInString(model) > maxAgentModelLength {
		return Validation("model must be 100 characters or fewer")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Test: online mode + empty model → always rejected (422)
// ---------------------------------------------------------------------------

func TestProperty16_OnlineAgent_EmptyModel_AlwaysRejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// For any online agent request with an empty model, it must be rejected
		svcErr := ValidateOnlineAgentModel("online", "")
		if svcErr == nil {
			t.Fatal("expected empty model for online agent to be rejected, got nil")
		}
		if svcErr.Kind != ErrUnprocessable {
			t.Fatalf("expected ErrUnprocessable (422), got %v", svcErr.Kind)
		}
		if !strings.Contains(svcErr.Message, "model is required for online agents") {
			t.Fatalf("unexpected error message: %v", svcErr.Message)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: online mode + model > 100 chars → always rejected
// ---------------------------------------------------------------------------

func TestProperty16_OnlineAgent_ModelExceeds100Chars_Rejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a model string with length > 100 runes
		modelLen := rapid.IntRange(101, 200).Draw(t, "modelLen")
		model := genModelString(t, "model", modelLen)

		svcErr := ValidateOnlineAgentModel("online", model)
		if svcErr == nil {
			t.Fatalf("expected model of length %d for online agent to be rejected", utf8.RuneCountInString(model))
		}
		if svcErr.Kind != ErrValidation {
			t.Fatalf("expected ErrValidation, got %v", svcErr.Kind)
		}
		if !strings.Contains(svcErr.Message, "model must be 100 characters or fewer") {
			t.Fatalf("unexpected error message: %v", svcErr.Message)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: online mode + model 1-100 chars → always accepted
// ---------------------------------------------------------------------------

func TestProperty16_OnlineAgent_ValidModel_Accepted(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a model string with valid length 1-100 runes
		modelLen := rapid.IntRange(1, 100).Draw(t, "modelLen")
		model := genModelString(t, "model", modelLen)

		svcErr := ValidateOnlineAgentModel("online", model)
		if svcErr != nil {
			t.Fatalf("expected valid model (len=%d) for online agent to be accepted, got error: %v",
				utf8.RuneCountInString(model), svcErr)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: local mode + empty model → always accepted (model is optional)
// ---------------------------------------------------------------------------

func TestProperty16_LocalAgent_EmptyModel_Accepted(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// For any local agent request with an empty model, it must be accepted
		svcErr := ValidateOnlineAgentModel("local", "")
		if svcErr != nil {
			t.Fatalf("expected empty model for local agent to be accepted, got error: %v", svcErr)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: local mode + any model 1-100 chars → always accepted
// ---------------------------------------------------------------------------

func TestProperty16_LocalAgent_AnyValidModel_Accepted(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a model string with valid length 0-100 runes
		modelLen := rapid.IntRange(0, 100).Draw(t, "modelLen")
		model := genModelString(t, "model", modelLen)

		svcErr := ValidateOnlineAgentModel("local", model)
		if svcErr != nil {
			t.Fatalf("expected model (len=%d) for local agent to be accepted, got error: %v",
				utf8.RuneCountInString(model), svcErr)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: local mode + model > 100 chars → rejected (length validation applies)
// ---------------------------------------------------------------------------

func TestProperty16_LocalAgent_ModelExceeds100Chars_Rejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a model string with length > 100 runes
		modelLen := rapid.IntRange(101, 200).Draw(t, "modelLen")
		model := genModelString(t, "model", modelLen)

		svcErr := ValidateOnlineAgentModel("local", model)
		if svcErr == nil {
			t.Fatalf("expected model of length %d for local agent to be rejected", utf8.RuneCountInString(model))
		}
		if svcErr.Kind != ErrValidation {
			t.Fatalf("expected ErrValidation, got %v", svcErr.Kind)
		}
		if !strings.Contains(svcErr.Message, "model must be 100 characters or fewer") {
			t.Fatalf("unexpected error message: %v", svcErr.Message)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: boundary - model of exactly 100 chars for online → accepted
// ---------------------------------------------------------------------------

func TestProperty16_OnlineAgent_BoundaryModel100Chars_Accepted(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Exactly 100 characters should always be accepted for online agents
		model := genModelString(t, "model", 100)

		svcErr := ValidateOnlineAgentModel("online", model)
		if svcErr != nil {
			t.Fatalf("expected 100-char model for online agent to be accepted, got error: %v", svcErr)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: boundary - model of exactly 101 chars for online → rejected
// ---------------------------------------------------------------------------

func TestProperty16_OnlineAgent_BoundaryModel101Chars_Rejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Exactly 101 characters should always be rejected
		model := genModelString(t, "model", 101)

		svcErr := ValidateOnlineAgentModel("online", model)
		if svcErr == nil {
			t.Fatal("expected 101-char model for online agent to be rejected")
		}
		if svcErr.Kind != ErrValidation {
			t.Fatalf("expected ErrValidation, got %v", svcErr.Kind)
		}
	})
}

// ---------------------------------------------------------------------------
// Helper: genModelString generates a string of exactly runeCount runes using
// characters typical of model identifiers (alphanumeric, dots, slashes, colons).
// ---------------------------------------------------------------------------

func genModelString(t *rapid.T, label string, runeCount int) string {
	if runeCount == 0 {
		return ""
	}
	// For large strings, use a repeating pattern for performance
	if runeCount > 50 {
		base := "gpt-4o-mini"
		baseRunes := utf8.RuneCountInString(base)
		repeats := runeCount / baseRunes
		remainder := runeCount % baseRunes
		result := strings.Repeat(base, repeats)
		if remainder > 0 {
			runes := []rune(base)
			result += string(runes[:remainder])
		}
		return result
	}
	// For smaller strings, generate with some randomness
	chars := []rune("abcdefghijklmnopqrstuvwxyz0123456789-_./:")
	var sb strings.Builder
	for i := 0; i < runeCount; i++ {
		idx := rapid.IntRange(0, len(chars)-1).Draw(t, label+"_char")
		sb.WriteRune(chars[idx])
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Property 5: Agent Runtime Mode Mutual Exclusivity
//
// For any agent record, if runtime_mode is "local" then runtime_id SHALL be
// non-NULL and provider_id SHALL be NULL; if runtime_mode is "online" then
// provider_id SHALL be non-NULL and runtime_id SHALL be NULL. No agent SHALL
// ever have both fields set or both fields NULL.
//
// This test validates the pure validation logic via ValidateRuntimeModeExclusivity,
// a helper function that encapsulates the mutual exclusivity constraint without
// requiring database access.
//
// **Validates: Requirements 6.2, 6.3**
// ---------------------------------------------------------------------------

// ValidateRuntimeModeExclusivity checks the mutual exclusivity constraint between
// runtime_mode, runtime_id, and provider_id for agent configurations.
// Returns nil if the configuration is valid, or a ServiceError describing the violation.
//
// Rules:
//   - runtime_mode "local": runtime_id must be non-empty, provider_id must be empty
//   - runtime_mode "online": provider_id must be non-empty, runtime_id must be empty
//   - Both runtime_id and provider_id set simultaneously is always rejected
//   - Both runtime_id and provider_id empty (for valid modes) is always rejected
func ValidateRuntimeModeExclusivity(runtimeMode, runtimeID, providerID string) *ServiceError {
	// Reject both set simultaneously
	if runtimeID != "" && providerID != "" {
		return Unprocessable("only one of provider_id or runtime_id may be specified")
	}

	switch runtimeMode {
	case "local":
		if runtimeID == "" {
			return Validation("runtime_id is required")
		}
		if providerID != "" {
			return Unprocessable("only one of provider_id or runtime_id may be specified")
		}
	case "online":
		if providerID == "" {
			return Unprocessable("provider_id is required for online agents")
		}
		if runtimeID != "" {
			return Unprocessable("only one of provider_id or runtime_id may be specified")
		}
	default:
		return Validation("runtime_mode must be \"local\" or \"online\"")
	}

	return nil
}

// genUUID generates a random UUID-like string for testing purposes.
func genUUID(t *rapid.T, label string) string {
	return rapid.StringMatching(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`).Draw(t, label)
}

// ---------------------------------------------------------------------------
// Test: local + runtime_id set + provider_id empty → always accepted
// ---------------------------------------------------------------------------

func TestProperty5_RuntimeModeExclusivity_LocalAccepted(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		runtimeID := genUUID(t, "runtime_id")

		err := ValidateRuntimeModeExclusivity("local", runtimeID, "")
		if err != nil {
			t.Fatalf("expected valid config (local + runtime_id=%q + provider_id empty) to be accepted, got error: %v", runtimeID, err)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: local + runtime_id empty → always rejected
// ---------------------------------------------------------------------------

func TestProperty5_RuntimeModeExclusivity_LocalRejectedWithoutRuntimeID(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		err := ValidateRuntimeModeExclusivity("local", "", "")
		if err == nil {
			t.Fatal("expected config (local + runtime_id empty + provider_id empty) to be rejected, but was accepted")
		}
		if err.Kind != ErrValidation {
			t.Fatalf("expected ErrValidation, got %v", err.Kind)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: online + provider_id set + runtime_id empty → always accepted
// ---------------------------------------------------------------------------

func TestProperty5_RuntimeModeExclusivity_OnlineAccepted(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		providerID := genUUID(t, "provider_id")

		err := ValidateRuntimeModeExclusivity("online", "", providerID)
		if err != nil {
			t.Fatalf("expected valid config (online + provider_id=%q + runtime_id empty) to be accepted, got error: %v", providerID, err)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: online + provider_id empty → always rejected
// ---------------------------------------------------------------------------

func TestProperty5_RuntimeModeExclusivity_OnlineRejectedWithoutProviderID(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		err := ValidateRuntimeModeExclusivity("online", "", "")
		if err == nil {
			t.Fatal("expected config (online + provider_id empty + runtime_id empty) to be rejected, but was accepted")
		}
		if err.Kind != ErrUnprocessable {
			t.Fatalf("expected ErrUnprocessable, got %v", err.Kind)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: Both runtime_id and provider_id set → always rejected regardless of mode
// ---------------------------------------------------------------------------

func TestProperty5_RuntimeModeExclusivity_BothSetAlwaysRejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		runtimeMode := rapid.SampledFrom([]string{"local", "online"}).Draw(t, "runtime_mode")
		runtimeID := genUUID(t, "runtime_id")
		providerID := genUUID(t, "provider_id")

		err := ValidateRuntimeModeExclusivity(runtimeMode, runtimeID, providerID)
		if err == nil {
			t.Fatalf("expected config (mode=%q + runtime_id=%q + provider_id=%q) to be rejected (both set), but was accepted",
				runtimeMode, runtimeID, providerID)
		}
		if err.Kind != ErrUnprocessable {
			t.Fatalf("expected ErrUnprocessable, got %v", err.Kind)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: invalid runtime_mode values are always rejected
// ---------------------------------------------------------------------------

func TestProperty5_RuntimeModeExclusivity_InvalidModeAlwaysRejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a mode that is neither "local" nor "online"
		mode := rapid.StringMatching(`[a-z]{1,20}`).Filter(func(s string) bool {
			return s != "local" && s != "online"
		}).Draw(t, "invalid_mode")

		// Use empty IDs to isolate the invalid mode check (avoid triggering "both set" first)
		err := ValidateRuntimeModeExclusivity(mode, "", "")
		if err == nil {
			t.Fatalf("expected invalid mode %q to be rejected, but was accepted", mode)
		}
		if err.Kind != ErrValidation {
			t.Fatalf("expected ErrValidation for invalid mode, got %v", err.Kind)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: For any accepted configuration, exactly one of runtime_id or provider_id
// is set — the core mutual exclusivity invariant.
// ---------------------------------------------------------------------------

func TestProperty5_RuntimeModeExclusivity_ValidConfigsNeverHaveBothNullOrBothSet(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		runtimeMode := rapid.SampledFrom([]string{"local", "online"}).Draw(t, "runtime_mode")
		runtimeID := rapid.SampledFrom([]string{"", genUUID(t, "runtime_id")}).Draw(t, "runtime_choice")
		providerID := rapid.SampledFrom([]string{"", genUUID(t, "provider_id")}).Draw(t, "provider_choice")

		err := ValidateRuntimeModeExclusivity(runtimeMode, runtimeID, providerID)
		if err == nil {
			// If accepted, exactly one must be set
			runtimeSet := runtimeID != ""
			providerSet := providerID != ""

			if runtimeSet == providerSet {
				t.Fatalf("accepted config violates mutual exclusivity: runtime_id set=%v, provider_id set=%v (mode=%q)",
					runtimeSet, providerSet, runtimeMode)
			}

			// Additionally verify the correct field is set for the mode
			if runtimeMode == "local" && !runtimeSet {
				t.Fatalf("accepted local config has no runtime_id")
			}
			if runtimeMode == "local" && providerSet {
				t.Fatalf("accepted local config has provider_id set")
			}
			if runtimeMode == "online" && !providerSet {
				t.Fatalf("accepted online config has no provider_id")
			}
			if runtimeMode == "online" && runtimeSet {
				t.Fatalf("accepted online config has runtime_id set")
			}
		}
	})
}
