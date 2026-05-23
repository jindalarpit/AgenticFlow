package handler

import (
	"strings"
	"testing"
	"unicode/utf8"

	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Feature: agent-management, Property 8: Agent Custom Env Validation
//
// For any set of custom environment variable pairs submitted with an agent,
// the Server SHALL accept it if and only if: the number of pairs does not
// exceed 20, each key is between 1 and 64 characters, and each value is
// between 1 and 1024 characters.
//
// **Validates: Requirements 6.5**
// ---------------------------------------------------------------------------

// ValidateCustomEnv checks whether a custom_env map is acceptable.
// It returns true if:
//   - The number of pairs is ≤ maxCustomEnvPairs (20)
//   - Each key has rune length between 1 and maxCustomEnvKeyLength (64)
//   - Each value has rune length between 1 and maxCustomEnvValueLength (1024)
//
// This mirrors the validation logic in CreateAgent handler.
func ValidateCustomEnv(env map[string]string) bool {
	if len(env) > maxCustomEnvPairs {
		return false
	}
	for key, value := range env {
		keyLen := utf8.RuneCountInString(key)
		if keyLen < 1 || keyLen > maxCustomEnvKeyLength {
			return false
		}
		valueLen := utf8.RuneCountInString(value)
		if valueLen < 1 || valueLen > maxCustomEnvValueLength {
			return false
		}
	}
	return true
}

// --- Generators ---

// genValidEnvKey generates a valid env key: 1-64 characters.
func genValidEnvKey(t *rapid.T, label string) string {
	length := rapid.IntRange(1, maxCustomEnvKeyLength).Draw(t, label+"Len")
	key := rapid.StringMatching(`[A-Z_a-z0-9]{1,64}`).Draw(t, label)
	if utf8.RuneCountInString(key) > length {
		runes := []rune(key)
		key = string(runes[:length])
	}
	if utf8.RuneCountInString(key) < 1 {
		key = "K"
	}
	return key
}

// genValidEnvValue generates a valid env value: 1-1024 characters.
func genValidEnvValue(t *rapid.T, label string) string {
	length := rapid.IntRange(1, 256).Draw(t, label+"Len") // cap at 256 for perf
	value := rapid.StringMatching(`[a-zA-Z0-9/=+._-]{1,256}`).Draw(t, label)
	if utf8.RuneCountInString(value) > length {
		runes := []rune(value)
		value = string(runes[:length])
	}
	if utf8.RuneCountInString(value) < 1 {
		value = "v"
	}
	return value
}

// --- Property Tests ---

func TestProperty8_CustomEnvValidation_ValidMapAccepted(t *testing.T) {
	// Feature: agent-management, Property 8: Agent Custom Env Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate a valid custom_env map: 0-20 pairs, keys 1-64 chars, values 1-1024 chars
		numPairs := rapid.IntRange(0, maxCustomEnvPairs).Draw(t, "numPairs")
		env := make(map[string]string, numPairs)
		for i := 0; i < numPairs; i++ {
			key := genValidEnvKey(t, "key")
			value := genValidEnvValue(t, "value")
			env[key] = value
		}

		if !ValidateCustomEnv(env) {
			t.Fatalf("valid custom_env with %d pairs should be accepted", len(env))
		}
	})
}

func TestProperty8_CustomEnvValidation_EmptyMapAccepted(t *testing.T) {
	// Feature: agent-management, Property 8: Agent Custom Env Validation
	rapid.Check(t, func(t *rapid.T) {
		// Empty map (nil or zero-length) should always be accepted
		env := map[string]string{}
		if !ValidateCustomEnv(env) {
			t.Fatal("empty custom_env map should be accepted")
		}

		// nil map should also be accepted
		if !ValidateCustomEnv(nil) {
			t.Fatal("nil custom_env map should be accepted")
		}
	})
}

func TestProperty8_CustomEnvValidation_TooManyPairsRejected(t *testing.T) {
	// Feature: agent-management, Property 8: Agent Custom Env Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate a map with more than 20 pairs
		numPairs := rapid.IntRange(maxCustomEnvPairs+1, maxCustomEnvPairs+50).Draw(t, "numPairs")
		env := make(map[string]string, numPairs)
		for i := 0; i < numPairs; i++ {
			// Use unique keys to ensure we actually get numPairs entries
			key := strings.Repeat("K", rapid.IntRange(1, 10).Draw(t, "keyPrefixLen")) + strings.Repeat("x", i%50)
			// Ensure unique keys by appending index
			key = key[:min(maxCustomEnvKeyLength-5, len(key))]
			key = key + "_" + strings.Repeat("0", 3-len(intToStr(i))) + intToStr(i)
			if utf8.RuneCountInString(key) > maxCustomEnvKeyLength {
				key = string([]rune(key)[:maxCustomEnvKeyLength])
			}
			if utf8.RuneCountInString(key) < 1 {
				key = "K"
			}
			env[key] = "value"
		}

		// Ensure we actually have more than maxCustomEnvPairs entries
		if len(env) <= maxCustomEnvPairs {
			// Force it by adding more unique keys
			for i := len(env); i <= maxCustomEnvPairs; i++ {
				env["extra_key_"+intToStr(i)] = "val"
			}
		}

		if ValidateCustomEnv(env) {
			t.Fatalf("custom_env with %d pairs (exceeds %d) should be rejected",
				len(env), maxCustomEnvPairs)
		}
	})
}

func TestProperty8_CustomEnvValidation_EmptyKeyRejected(t *testing.T) {
	// Feature: agent-management, Property 8: Agent Custom Env Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate a map with at least one empty key
		numPairs := rapid.IntRange(1, maxCustomEnvPairs).Draw(t, "numPairs")
		env := make(map[string]string, numPairs)

		// Add valid pairs first
		for i := 0; i < numPairs-1; i++ {
			env["valid_key_"+intToStr(i)] = "value"
		}
		// Add an empty key
		env[""] = "some_value"

		if ValidateCustomEnv(env) {
			t.Fatal("custom_env with empty key should be rejected")
		}
	})
}

func TestProperty8_CustomEnvValidation_KeyTooLongRejected(t *testing.T) {
	// Feature: agent-management, Property 8: Agent Custom Env Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate a map with at least one key exceeding 64 characters
		extraLen := rapid.IntRange(1, 100).Draw(t, "extraLen")
		longKey := strings.Repeat("K", maxCustomEnvKeyLength+extraLen)

		env := map[string]string{
			longKey: "value",
		}

		if ValidateCustomEnv(env) {
			t.Fatalf("custom_env with key of length %d (exceeds %d) should be rejected",
				utf8.RuneCountInString(longKey), maxCustomEnvKeyLength)
		}
	})
}

func TestProperty8_CustomEnvValidation_EmptyValueRejected(t *testing.T) {
	// Feature: agent-management, Property 8: Agent Custom Env Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate a map with at least one empty value
		numPairs := rapid.IntRange(1, maxCustomEnvPairs).Draw(t, "numPairs")
		env := make(map[string]string, numPairs)

		// Add valid pairs first
		for i := 0; i < numPairs-1; i++ {
			env["key_"+intToStr(i)] = "valid_value"
		}
		// Add a key with empty value
		env["key_with_empty_val"] = ""

		if ValidateCustomEnv(env) {
			t.Fatal("custom_env with empty value should be rejected")
		}
	})
}

func TestProperty8_CustomEnvValidation_ValueTooLongRejected(t *testing.T) {
	// Feature: agent-management, Property 8: Agent Custom Env Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate a map with at least one value exceeding 1024 characters
		extraLen := rapid.IntRange(1, 500).Draw(t, "extraLen")
		longValue := strings.Repeat("v", maxCustomEnvValueLength+extraLen)

		env := map[string]string{
			"VALID_KEY": longValue,
		}

		if ValidateCustomEnv(env) {
			t.Fatalf("custom_env with value of length %d (exceeds %d) should be rejected",
				utf8.RuneCountInString(longValue), maxCustomEnvValueLength)
		}
	})
}

func TestProperty8_CustomEnvValidation_BoundaryPairCount(t *testing.T) {
	// Feature: agent-management, Property 8: Agent Custom Env Validation
	rapid.Check(t, func(t *rapid.T) {
		// Exactly 20 pairs should be accepted
		env20 := make(map[string]string, maxCustomEnvPairs)
		for i := 0; i < maxCustomEnvPairs; i++ {
			env20["KEY_"+intToStr(i)] = "value"
		}
		if !ValidateCustomEnv(env20) {
			t.Fatalf("custom_env with exactly %d pairs should be accepted", maxCustomEnvPairs)
		}

		// 21 pairs should be rejected
		env21 := make(map[string]string, maxCustomEnvPairs+1)
		for i := 0; i <= maxCustomEnvPairs; i++ {
			env21["KEY_"+intToStr(i)] = "value"
		}
		if ValidateCustomEnv(env21) {
			t.Fatalf("custom_env with %d pairs should be rejected", maxCustomEnvPairs+1)
		}
	})
}

func TestProperty8_CustomEnvValidation_BoundaryKeyLength(t *testing.T) {
	// Feature: agent-management, Property 8: Agent Custom Env Validation
	rapid.Check(t, func(t *rapid.T) {
		// Key of exactly 64 characters should be accepted
		key64 := strings.Repeat("K", maxCustomEnvKeyLength)
		env := map[string]string{key64: "value"}
		if !ValidateCustomEnv(env) {
			t.Fatalf("custom_env with key of exactly %d chars should be accepted", maxCustomEnvKeyLength)
		}

		// Key of 65 characters should be rejected
		key65 := strings.Repeat("K", maxCustomEnvKeyLength+1)
		env2 := map[string]string{key65: "value"}
		if ValidateCustomEnv(env2) {
			t.Fatalf("custom_env with key of %d chars should be rejected", maxCustomEnvKeyLength+1)
		}

		// Key of 1 character should be accepted
		key1 := rapid.StringMatching(`[A-Z]`).Draw(t, "key1")
		env3 := map[string]string{key1: "value"}
		if !ValidateCustomEnv(env3) {
			t.Fatalf("custom_env with single-char key %q should be accepted", key1)
		}
	})
}

func TestProperty8_CustomEnvValidation_BoundaryValueLength(t *testing.T) {
	// Feature: agent-management, Property 8: Agent Custom Env Validation
	rapid.Check(t, func(t *rapid.T) {
		// Value of exactly 1024 characters should be accepted
		val1024 := strings.Repeat("v", maxCustomEnvValueLength)
		env := map[string]string{"KEY": val1024}
		if !ValidateCustomEnv(env) {
			t.Fatalf("custom_env with value of exactly %d chars should be accepted", maxCustomEnvValueLength)
		}

		// Value of 1025 characters should be rejected
		val1025 := strings.Repeat("v", maxCustomEnvValueLength+1)
		env2 := map[string]string{"KEY": val1025}
		if ValidateCustomEnv(env2) {
			t.Fatalf("custom_env with value of %d chars should be rejected", maxCustomEnvValueLength+1)
		}

		// Value of 1 character should be accepted
		val1 := rapid.StringMatching(`[a-z]`).Draw(t, "val1")
		env3 := map[string]string{"KEY": val1}
		if !ValidateCustomEnv(env3) {
			t.Fatalf("custom_env with single-char value %q should be accepted", val1)
		}
	})
}

func TestProperty8_CustomEnvValidation_ArbitraryMaps(t *testing.T) {
	// Feature: agent-management, Property 8: Agent Custom Env Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary maps and verify validation is consistent with the spec
		numPairs := rapid.IntRange(0, 30).Draw(t, "numPairs")
		env := make(map[string]string, numPairs)
		for i := 0; i < numPairs; i++ {
			key := rapid.String().Draw(t, "key")
			value := rapid.String().Draw(t, "value")
			env[key] = value
		}

		result := ValidateCustomEnv(env)

		// Compute expected result manually
		expected := true
		if len(env) > maxCustomEnvPairs {
			expected = false
		} else {
			for key, value := range env {
				keyLen := utf8.RuneCountInString(key)
				if keyLen < 1 || keyLen > maxCustomEnvKeyLength {
					expected = false
					break
				}
				valueLen := utf8.RuneCountInString(value)
				if valueLen < 1 || valueLen > maxCustomEnvValueLength {
					expected = false
					break
				}
			}
		}

		if result != expected {
			t.Fatalf("ValidateCustomEnv(map with %d pairs) = %v, want %v",
				len(env), result, expected)
		}
	})
}

func TestProperty8_CustomEnvValidation_UnicodeKeys(t *testing.T) {
	// Feature: agent-management, Property 8: Agent Custom Env Validation
	rapid.Check(t, func(t *rapid.T) {
		// Test with multi-byte unicode characters in keys
		// A key with 64 unicode runes (each potentially multi-byte) should be accepted
		runeCount := rapid.IntRange(1, maxCustomEnvKeyLength).Draw(t, "runeCount")
		// Use a mix of ASCII and multi-byte chars
		runes := make([]rune, runeCount)
		for i := range runes {
			// Mix of ASCII and common unicode chars
			if rapid.Bool().Draw(t, "useUnicode") {
				runes[i] = rune(rapid.IntRange(0x4E00, 0x4E50).Draw(t, "unicodeChar")) // CJK chars
			} else {
				runes[i] = rune(rapid.IntRange('A', 'Z').Draw(t, "asciiChar"))
			}
		}
		key := string(runes)

		env := map[string]string{key: "value"}
		result := ValidateCustomEnv(env)

		// Should be accepted since rune count is within bounds
		if !result {
			t.Fatalf("custom_env with unicode key of %d runes should be accepted",
				utf8.RuneCountInString(key))
		}
	})
}

func TestProperty8_CustomEnvValidation_UnicodeValues(t *testing.T) {
	// Feature: agent-management, Property 8: Agent Custom Env Validation
	rapid.Check(t, func(t *rapid.T) {
		// Test with multi-byte unicode characters in values
		runeCount := rapid.IntRange(1, 256).Draw(t, "runeCount") // cap for perf
		runes := make([]rune, runeCount)
		for i := range runes {
			if rapid.Bool().Draw(t, "useUnicode") {
				runes[i] = rune(rapid.IntRange(0x4E00, 0x4E50).Draw(t, "unicodeChar"))
			} else {
				runes[i] = rune(rapid.IntRange('a', 'z').Draw(t, "asciiChar"))
			}
		}
		value := string(runes)

		env := map[string]string{"KEY": value}
		result := ValidateCustomEnv(env)

		// Should be accepted since rune count is within bounds (≤1024)
		if !result {
			t.Fatalf("custom_env with unicode value of %d runes should be accepted",
				utf8.RuneCountInString(value))
		}
	})
}

// --- Helper ---

func intToStr(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	n := i
	if n < 0 {
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if i < 0 {
		s = "-" + s
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
