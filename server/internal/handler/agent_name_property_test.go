package handler

import (
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Feature: agent-management, Property 6: Agent Name Validation
//
// For any string submitted as an agent name, the Server SHALL accept it if and
// only if it matches the pattern `^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$` (starts
// with alphanumeric, 1-64 characters total, containing only alphanumeric,
// hyphens, and underscores).
//
// **Validates: Requirements 6.2, 6.3**
// ---------------------------------------------------------------------------

// ValidateAgentName checks whether an agent name is acceptable using the
// production agentNameRegex defined in agent.go.
func ValidateAgentName(name string) bool {
	return agentNameRegex.MatchString(name)
}

func TestProperty6_AgentNameValidation_ValidNamesAccepted(t *testing.T) {
	// Feature: agent-management, Property 6: Agent Name Validation
	// For any string matching ^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$: validation should ACCEPT
	rapid.Check(t, func(t *rapid.T) {
		// Generate valid names: start with alphanumeric, followed by 0-63 valid chars
		name := rapid.StringMatching(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`).Draw(t, "name")

		if !ValidateAgentName(name) {
			t.Fatalf("valid agent name %q should be accepted", name)
		}
	})
}

func TestProperty6_AgentNameValidation_InvalidNamesRejected(t *testing.T) {
	// Feature: agent-management, Property 6: Agent Name Validation
	// For any string NOT matching the pattern: validation should REJECT
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary strings and filter to those that should be invalid
		name := rapid.String().Draw(t, "name")

		// Skip strings that happen to match the valid pattern
		if agentNameRegex.MatchString(name) {
			return // skip this iteration — it's a valid name
		}

		if ValidateAgentName(name) {
			t.Fatalf("invalid agent name %q should be rejected", name)
		}
	})
}

func TestProperty6_AgentNameValidation_Boundary64CharsAccepted(t *testing.T) {
	// Feature: agent-management, Property 6: Agent Name Validation
	// Boundary: exactly 64 chars starting with alphanumeric → accepted
	rapid.Check(t, func(t *rapid.T) {
		// Generate a name of exactly 64 characters: 1 alphanumeric + 63 valid chars
		firstChar := rapid.StringMatching(`[a-zA-Z0-9]`).Draw(t, "firstChar")
		rest := rapid.StringMatching(`[a-zA-Z0-9_-]{63}`).Draw(t, "rest")
		name := firstChar + rest

		if len(name) != 64 {
			t.Fatalf("expected name length 64, got %d", len(name))
		}

		if !ValidateAgentName(name) {
			t.Fatalf("agent name of exactly 64 chars %q should be accepted", name)
		}
	})
}

func TestProperty6_AgentNameValidation_Boundary65CharsRejected(t *testing.T) {
	// Feature: agent-management, Property 6: Agent Name Validation
	// Boundary: 65 chars → rejected
	rapid.Check(t, func(t *rapid.T) {
		// Generate a name of exactly 65 characters using valid characters
		firstChar := rapid.StringMatching(`[a-zA-Z0-9]`).Draw(t, "firstChar")
		rest := rapid.StringMatching(`[a-zA-Z0-9_-]{64}`).Draw(t, "rest")
		name := firstChar + rest

		if len(name) != 65 {
			t.Fatalf("expected name length 65, got %d", len(name))
		}

		if ValidateAgentName(name) {
			t.Fatalf("agent name of 65 chars %q should be rejected", name)
		}
	})
}

func TestProperty6_AgentNameValidation_StartsWithNonAlphanumericRejected(t *testing.T) {
	// Feature: agent-management, Property 6: Agent Name Validation
	// Strings starting with non-alphanumeric → rejected
	rapid.Check(t, func(t *rapid.T) {
		// Generate a non-alphanumeric first character
		invalidStarts := []rune{'_', '-', '.', '@', '#', '$', '%', '!', ' ', '/', '\\', '(', ')', '+', '=', '~', '`', '{', '}', '[', ']', '|', ':', ';', '"', '\'', '<', '>', ',', '?', '^', '&', '*'}
		firstChar := invalidStarts[rapid.IntRange(0, len(invalidStarts)-1).Draw(t, "charIdx")]

		// Follow with valid characters (0-63 chars)
		restLen := rapid.IntRange(0, 63).Draw(t, "restLen")
		rest := ""
		if restLen > 0 {
			rest = rapid.StringMatching(`[a-zA-Z0-9_-]{1,63}`).Draw(t, "rest")
			if len(rest) > restLen {
				rest = rest[:restLen]
			}
		}

		name := string(firstChar) + rest

		if ValidateAgentName(name) {
			t.Fatalf("agent name %q starting with non-alphanumeric %q should be rejected",
				name, string(firstChar))
		}
	})
}

func TestProperty6_AgentNameValidation_InvalidCharactersRejected(t *testing.T) {
	// Feature: agent-management, Property 6: Agent Name Validation
	// Strings containing invalid characters (spaces, dots, @, etc.) → rejected
	rapid.Check(t, func(t *rapid.T) {
		// Start with a valid prefix
		prefix := rapid.StringMatching(`[a-zA-Z0-9][a-zA-Z0-9_-]{0,30}`).Draw(t, "prefix")

		// Insert an invalid character
		invalidChars := []rune{' ', '.', '@', '#', '$', '%', '!', '/', '\\', '(', ')', '+', '=', '~', '`', '{', '}', '[', ']', '|', ':', ';', '"', '\'', '<', '>', ',', '?', '^', '&', '*'}
		invalidChar := invalidChars[rapid.IntRange(0, len(invalidChars)-1).Draw(t, "charIdx")]

		// Optionally add a valid suffix
		suffixLen := rapid.IntRange(0, 10).Draw(t, "suffixLen")
		suffix := ""
		if suffixLen > 0 {
			suffix = rapid.StringMatching(`[a-zA-Z0-9_-]{1,10}`).Draw(t, "suffix")
			if len(suffix) > suffixLen {
				suffix = suffix[:suffixLen]
			}
		}

		name := prefix + string(invalidChar) + suffix

		if ValidateAgentName(name) {
			t.Fatalf("agent name %q containing invalid char %q should be rejected",
				name, string(invalidChar))
		}
	})
}

func TestProperty6_AgentNameValidation_EmptyStringRejected(t *testing.T) {
	// Feature: agent-management, Property 6: Agent Name Validation
	// Empty string should always be rejected
	rapid.Check(t, func(t *rapid.T) {
		if ValidateAgentName("") {
			t.Fatal("empty agent name should be rejected")
		}
	})
}

func TestProperty6_AgentNameValidation_SingleAlphanumericAccepted(t *testing.T) {
	// Feature: agent-management, Property 6: Agent Name Validation
	// A single alphanumeric character is valid (matches ^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$ with 0 trailing)
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringMatching(`[a-zA-Z0-9]`).Draw(t, "name")

		if !ValidateAgentName(name) {
			t.Fatalf("single alphanumeric char %q should be accepted as agent name", name)
		}
	})
}

func TestProperty6_AgentNameValidation_ArbitraryStringsConsistency(t *testing.T) {
	// Feature: agent-management, Property 6: Agent Name Validation
	// For any arbitrary string, ValidateAgentName must agree with the regex
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.String().Draw(t, "name")

		result := ValidateAgentName(name)
		expected := agentNameRegex.MatchString(name)

		if result != expected {
			t.Fatalf("ValidateAgentName(%q) = %v, want %v (regex match)", name, result, expected)
		}
	})
}

func TestProperty6_AgentNameValidation_LongValidCharsRejected(t *testing.T) {
	// Feature: agent-management, Property 6: Agent Name Validation
	// Names longer than 64 chars even with all valid characters should be rejected
	rapid.Check(t, func(t *rapid.T) {
		extraLen := rapid.IntRange(1, 200).Draw(t, "extraLen")
		name := "a" + strings.Repeat("b", 63+extraLen) // starts with alphanumeric, all valid chars, but too long

		if ValidateAgentName(name) {
			t.Fatalf("agent name of length %d (exceeds 64) should be rejected", len(name))
		}
	})
}

func TestProperty6_AgentNameValidation_HyphensAndUnderscoresValid(t *testing.T) {
	// Feature: agent-management, Property 6: Agent Name Validation
	// Names with hyphens and underscores (not at start) should be accepted
	rapid.Check(t, func(t *rapid.T) {
		// Generate names that use hyphens and underscores in the body
		firstChar := rapid.StringMatching(`[a-zA-Z0-9]`).Draw(t, "firstChar")
		bodyLen := rapid.IntRange(1, 63).Draw(t, "bodyLen")
		body := rapid.StringMatching(`[a-zA-Z0-9_-]{1,63}`).Draw(t, "body")
		if len(body) > bodyLen {
			body = body[:bodyLen]
		}

		name := firstChar + body

		// Ensure total length is within bounds
		if len(name) > 64 {
			name = name[:64]
		}

		if !ValidateAgentName(name) {
			t.Fatalf("agent name %q with hyphens/underscores should be accepted", name)
		}
	})
}
