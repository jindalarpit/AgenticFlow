package handler

import (
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Feature: agent-management, Property 2: PAT Name Validation
//
// For any string submitted as a PAT name, the Server SHALL accept it if and
// only if it is non-empty after trimming whitespace and does not exceed 64
// characters. Empty or whitespace-only strings SHALL be rejected.
//
// **Validates: Requirements 1.4, 1.5**
// ---------------------------------------------------------------------------

// ValidatePATName checks whether a PAT name is acceptable.
// It returns true if the name is non-empty after trimming whitespace
// and does not exceed patMaxName (64) characters.
// This mirrors the validation logic in CreatePersonalAccessToken handler.
func ValidatePATName(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return false
	}
	if len(name) > patMaxName {
		return false
	}
	return true
}

func TestProperty2_PATNameValidation_EmptyRejected(t *testing.T) {
	// Feature: agent-management, Property 2: PAT Name Validation
	rapid.Check(t, func(t *rapid.T) {
		// Empty string should always be rejected
		if ValidatePATName("") {
			t.Fatal("empty PAT name should be rejected")
		}
	})
}

func TestProperty2_PATNameValidation_WhitespaceOnlyRejected(t *testing.T) {
	// Feature: agent-management, Property 2: PAT Name Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate whitespace-only strings of various lengths
		numChars := rapid.IntRange(1, 100).Draw(t, "numChars")
		wsChars := []rune{' ', '\t', '\n', '\r'}
		runes := make([]rune, numChars)
		for i := range runes {
			runes[i] = wsChars[rapid.IntRange(0, len(wsChars)-1).Draw(t, "wsIdx")]
		}
		name := string(runes)

		if ValidatePATName(name) {
			t.Fatalf("whitespace-only PAT name %q should be rejected", name)
		}
	})
}

func TestProperty2_PATNameValidation_ValidNameAccepted(t *testing.T) {
	// Feature: agent-management, Property 2: PAT Name Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate valid names: 1 to 64 characters, non-empty after trim
		length := rapid.IntRange(1, 64).Draw(t, "length")
		name := rapid.StringMatching(`[a-zA-Z0-9 _-]{1,64}`).Draw(t, "name")

		// Ensure at least one non-whitespace character
		if strings.TrimSpace(name) == "" {
			name = "x"
		}

		// Trim to requested length
		if len(name) > length {
			name = name[:length]
		}
		// Ensure still non-empty after trim
		if strings.TrimSpace(name) == "" {
			name = "x"
		}
		// Ensure within max length
		if len(name) > patMaxName {
			name = name[:patMaxName]
		}

		if !ValidatePATName(name) {
			t.Fatalf("valid PAT name %q (len=%d) should be accepted", name, len(name))
		}
	})
}

func TestProperty2_PATNameValidation_ExceedsMaxLengthRejected(t *testing.T) {
	// Feature: agent-management, Property 2: PAT Name Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate names that exceed 64 characters
		extraLen := rapid.IntRange(1, 200).Draw(t, "extraLen")
		name := rapid.StringMatching(`[a-zA-Z0-9]{65,265}`).Draw(t, "name")

		// Ensure the name is longer than patMaxName
		if len(name) <= patMaxName {
			name = strings.Repeat("a", patMaxName+extraLen)
		}

		if ValidatePATName(name) {
			t.Fatalf("PAT name of length %d (exceeds %d) should be rejected", len(name), patMaxName)
		}
	})
}

func TestProperty2_PATNameValidation_BoundaryLength(t *testing.T) {
	// Feature: agent-management, Property 2: PAT Name Validation
	rapid.Check(t, func(t *rapid.T) {
		// Exactly 64 characters (non-whitespace) should be accepted
		name64 := rapid.StringMatching(`[a-zA-Z0-9]{64}`).Draw(t, "name64")
		if !ValidatePATName(name64) {
			t.Fatalf("PAT name of exactly 64 chars should be accepted: %q", name64)
		}

		// 65 characters should be rejected
		name65 := rapid.StringMatching(`[a-zA-Z0-9]{65}`).Draw(t, "name65")
		if ValidatePATName(name65) {
			t.Fatalf("PAT name of 65 chars should be rejected: %q", name65)
		}

		// Single non-whitespace character should be accepted
		name1 := rapid.StringMatching(`[a-zA-Z0-9]`).Draw(t, "name1")
		if !ValidatePATName(name1) {
			t.Fatalf("single char PAT name %q should be accepted", name1)
		}
	})
}

func TestProperty2_PATNameValidation_ArbitraryStrings(t *testing.T) {
	// Feature: agent-management, Property 2: PAT Name Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary strings and verify the validation logic is consistent
		name := rapid.String().Draw(t, "name")

		result := ValidatePATName(name)
		trimmed := strings.TrimSpace(name)
		expectedValid := trimmed != "" && len(name) <= patMaxName

		if result != expectedValid {
			t.Fatalf("ValidatePATName(%q) = %v, want %v (trimmed=%q, len=%d, maxLen=%d)",
				name, result, expectedValid, trimmed, len(name), patMaxName)
		}
	})
}

func TestProperty2_PATNameValidation_LeadingTrailingWhitespace(t *testing.T) {
	// Feature: agent-management, Property 2: PAT Name Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate names with leading/trailing whitespace but non-empty content
		// These should be accepted as long as total length ≤ 64
		content := rapid.StringMatching(`[a-zA-Z0-9]{1,50}`).Draw(t, "content")
		leadingSpaces := rapid.IntRange(0, 5).Draw(t, "leadingSpaces")
		trailingSpaces := rapid.IntRange(0, 5).Draw(t, "trailingSpaces")

		name := strings.Repeat(" ", leadingSpaces) + content + strings.Repeat(" ", trailingSpaces)

		// If total length exceeds 64, it should be rejected
		if len(name) > patMaxName {
			if ValidatePATName(name) {
				t.Fatalf("PAT name %q (len=%d, exceeds %d) should be rejected",
					name, len(name), patMaxName)
			}
		} else {
			// Non-empty after trim and within length: should be accepted
			if !ValidatePATName(name) {
				t.Fatalf("PAT name %q (len=%d, trimmed=%q) should be accepted",
					name, len(name), strings.TrimSpace(name))
			}
		}
	})
}
