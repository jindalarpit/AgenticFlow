package crypto

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// TestPropertyCredentialEncryptionRoundTrip verifies that for any random byte slice,
// encrypting then decrypting produces the exact same bytes.
//
// **Validates: Requirements 1.8**
func TestPropertyCredentialEncryptionRoundTrip(t *testing.T) {
	// Fixed valid 64-char hex key (32 bytes) for the test encryptor
	const testKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	enc, err := NewCredentialEncryptor(testKey)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	rapid.Check(t, func(t *rapid.T) {
		// Generate random byte slices of varying lengths (0 to ~10000 bytes)
		plaintext := rapid.SliceOfN(rapid.Byte(), 0, 10000).Draw(t, "plaintext")

		// Encrypt the plaintext
		ciphertext, err := enc.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encrypt failed: %v", err)
		}

		// Decrypt the ciphertext
		decrypted, err := enc.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("Decrypt failed: %v", err)
		}

		// Verify byte-for-byte equality
		if !bytes.Equal(plaintext, decrypted) {
			t.Fatalf("round-trip failed: plaintext length=%d, decrypted length=%d",
				len(plaintext), len(decrypted))
		}
	})
}

// TestPropertyCredentialMaskingNeverLeaksSecrets verifies that for any random string,
// the masking function follows the rules:
// - For strings with len >= 4: last 4 chars of masked output equal last 4 chars of original
// - For strings with len < 4: all chars are asterisks
// - Masked output length always equals input length
// - For strings with len >= 5: masked output never equals the full original value
//
// **Validates: Requirements 1.9**
func TestPropertyCredentialMaskingNeverLeaksSecrets(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random strings of varying lengths (0 to ~200 chars)
		length := rapid.IntRange(0, 200).Draw(t, "length")
		var value string
		if length == 0 {
			value = ""
		} else {
			value = rapid.StringMatching(fmt.Sprintf(`[a-zA-Z0-9!@#$%%^&*]{%d}`, length)).Draw(t, "value")
		}

		masked := MaskCredentialValue(value)
		n := len(value)

		// Property: masked output length always equals input length
		if len(masked) != n {
			t.Fatalf("masked length %d != input length %d for value %q", len(masked), n, value)
		}

		if n == 0 {
			// Empty string should return empty
			if masked != "" {
				t.Fatalf("expected empty string for empty input, got %q", masked)
			}
			return
		}

		if n < 4 {
			// For strings with len < 4: all chars are asterisks
			expected := strings.Repeat("*", n)
			if masked != expected {
				t.Fatalf("for len %d, expected all asterisks %q, got %q", n, expected, masked)
			}
		} else {
			// For strings with len >= 4: last 4 chars of masked output equal last 4 chars of original
			last4Original := value[n-4:]
			last4Masked := masked[len(masked)-4:]
			if last4Masked != last4Original {
				t.Fatalf("last 4 chars mismatch: original %q, masked %q", last4Original, last4Masked)
			}

			// Prefix should be all asterisks
			prefix := masked[:len(masked)-4]
			expectedPrefix := strings.Repeat("*", n-4)
			if prefix != expectedPrefix {
				t.Fatalf("prefix should be all asterisks, got %q", prefix)
			}
		}

		// For strings with len >= 5: masked output never equals the full original value
		if n >= 5 && masked == value {
			t.Fatalf("masked output should never equal original for len >= 5, value: %q", value)
		}

		// General property: masked output never contains the full original value as a substring
		// (only relevant for non-trivial strings where masking actually changes something)
		if n >= 5 && strings.Contains(masked, value) {
			t.Fatalf("masked output should never contain the full original value, masked: %q, original: %q", masked, value)
		}
	})
}
