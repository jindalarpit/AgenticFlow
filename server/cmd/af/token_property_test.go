package main

import (
	"testing"

	"pgregory.net/rapid"
)

// Feature: cli-auth-daemon, Property 12: Token display truncation
// For any token string, displayed value is token[:12] + "..." if len >= 12, else full token.
// Validates: Requirements 12.1
func TestProperty_TokenDisplayTruncation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random string of arbitrary length (0..100 chars).
		token := rapid.StringMatching(`.{0,100}`).Draw(t, "token")

		result := TruncateToken(token)

		if len(token) >= 12 {
			// Should be truncated to first 12 chars + "..."
			expected := token[:12] + "..."
			if result != expected {
				t.Fatalf("token len=%d: TruncateToken(%q) = %q, want %q",
					len(token), token, result, expected)
			}
		} else {
			// Should return the full token unchanged
			if result != token {
				t.Fatalf("token len=%d: TruncateToken(%q) = %q, want full token",
					len(token), token, result)
			}
		}
	})
}
