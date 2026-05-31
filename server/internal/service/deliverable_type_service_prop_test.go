package service

import (
	"strings"
	"testing"
	"unicode/utf8"

	"pgregory.net/rapid"
)

// TestPropertyDeliverableTypeFieldValidation verifies Property 14: for any
// deliverable type creation or update request, the name SHALL be accepted if
// and only if it is between 1 and 64 characters, the description SHALL be
// accepted if and only if it is at most 255 characters, and the output_format
// SHALL be accepted if and only if it is at most 10000 characters.
//
// **Validates: Requirements 4.2, 4.9**
func TestPropertyDeliverableTypeFieldValidation(t *testing.T) {
	t.Run("valid_name_description_outputformat_accepted", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate name with valid length 1-64
			nameLen := rapid.IntRange(1, 64).Draw(t, "nameLen")
			name := genUTF8String(t, "name", nameLen)

			// Generate description with valid length 0-255
			descLen := rapid.IntRange(0, 255).Draw(t, "descLen")
			description := genUTF8String(t, "description", descLen)

			// Generate output_format with valid length 0-10000
			ofLen := rapid.IntRange(0, 10000).Draw(t, "ofLen")
			outputFormat := genUTF8String(t, "outputFormat", ofLen)

			svcErr := ValidateDeliverableTypeFields(name, description, outputFormat)
			if svcErr != nil {
				t.Fatalf("expected valid fields to be accepted (name len=%d, desc len=%d, of len=%d), got error: %v",
					utf8.RuneCountInString(name), utf8.RuneCountInString(description), utf8.RuneCountInString(outputFormat), svcErr)
			}
		})
	})

	t.Run("empty_name_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Empty name should always be rejected
			description := genUTF8String(t, "description", rapid.IntRange(0, 100).Draw(t, "descLen"))
			outputFormat := genUTF8String(t, "outputFormat", rapid.IntRange(0, 100).Draw(t, "ofLen"))

			svcErr := ValidateDeliverableTypeFields("", description, outputFormat)
			if svcErr == nil {
				t.Fatal("expected empty name to be rejected")
			}
			if svcErr.Kind != ErrValidation {
				t.Fatalf("expected ErrValidation, got %v", svcErr.Kind)
			}
			if !strings.Contains(svcErr.Message, "name must be between 1 and 64 characters") {
				t.Fatalf("unexpected error message: %v", svcErr.Message)
			}
		})
	})

	t.Run("name_exceeding_64_chars_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Name with length > 64 should always be rejected
			nameLen := rapid.IntRange(65, 100).Draw(t, "nameLen")
			name := genUTF8String(t, "name", nameLen)
			description := genUTF8String(t, "description", rapid.IntRange(0, 100).Draw(t, "descLen"))
			outputFormat := genUTF8String(t, "outputFormat", rapid.IntRange(0, 100).Draw(t, "ofLen"))

			svcErr := ValidateDeliverableTypeFields(name, description, outputFormat)
			if svcErr == nil {
				t.Fatalf("expected name of length %d to be rejected", utf8.RuneCountInString(name))
			}
			if svcErr.Kind != ErrValidation {
				t.Fatalf("expected ErrValidation, got %v", svcErr.Kind)
			}
			if !strings.Contains(svcErr.Message, "name must be between 1 and 64 characters") {
				t.Fatalf("unexpected error message: %v", svcErr.Message)
			}
		})
	})

	t.Run("description_exceeding_255_chars_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Valid name, description > 255 chars
			nameLen := rapid.IntRange(1, 64).Draw(t, "nameLen")
			name := genUTF8String(t, "name", nameLen)
			descLen := rapid.IntRange(256, 300).Draw(t, "descLen")
			description := genUTF8String(t, "description", descLen)
			outputFormat := genUTF8String(t, "outputFormat", rapid.IntRange(0, 100).Draw(t, "ofLen"))

			svcErr := ValidateDeliverableTypeFields(name, description, outputFormat)
			if svcErr == nil {
				t.Fatalf("expected description of length %d to be rejected", utf8.RuneCountInString(description))
			}
			if svcErr.Kind != ErrValidation {
				t.Fatalf("expected ErrValidation, got %v", svcErr.Kind)
			}
			if !strings.Contains(svcErr.Message, "description must be at most 255 characters") {
				t.Fatalf("unexpected error message: %v", svcErr.Message)
			}
		})
	})

	t.Run("output_format_exceeding_10000_chars_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Valid name and description, output_format > 10000 chars
			nameLen := rapid.IntRange(1, 64).Draw(t, "nameLen")
			name := genUTF8String(t, "name", nameLen)
			descLen := rapid.IntRange(0, 255).Draw(t, "descLen")
			description := genUTF8String(t, "description", descLen)
			ofLen := rapid.IntRange(10001, 11000).Draw(t, "ofLen")
			outputFormat := genUTF8String(t, "outputFormat", ofLen)

			svcErr := ValidateDeliverableTypeFields(name, description, outputFormat)
			if svcErr == nil {
				t.Fatalf("expected output_format of length %d to be rejected", utf8.RuneCountInString(outputFormat))
			}
			if svcErr.Kind != ErrValidation {
				t.Fatalf("expected ErrValidation, got %v", svcErr.Kind)
			}
			if !strings.Contains(svcErr.Message, "output_format must be at most 10000 characters") {
				t.Fatalf("unexpected error message: %v", svcErr.Message)
			}
		})
	})

	t.Run("boundary_name_length_1_accepted", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Single character name should always be accepted
			name := genUTF8String(t, "name", 1)

			svcErr := ValidateDeliverableTypeFields(name, "", "")
			if svcErr != nil {
				t.Fatalf("expected single-char name to be accepted, got error: %v", svcErr)
			}
		})
	})

	t.Run("boundary_name_length_64_accepted", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Exactly 64 characters should always be accepted
			name := genUTF8String(t, "name", 64)

			svcErr := ValidateDeliverableTypeFields(name, "", "")
			if svcErr != nil {
				t.Fatalf("expected 64-char name to be accepted, got error: %v", svcErr)
			}
		})
	})

	t.Run("boundary_name_length_65_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Exactly 65 characters should always be rejected
			name := genUTF8String(t, "name", 65)

			svcErr := ValidateDeliverableTypeFields(name, "", "")
			if svcErr == nil {
				t.Fatal("expected 65-char name to be rejected")
			}
			if svcErr.Kind != ErrValidation {
				t.Fatalf("expected ErrValidation, got %v", svcErr.Kind)
			}
		})
	})

	t.Run("boundary_description_length_255_accepted", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Exactly 255 characters should always be accepted
			name := genUTF8String(t, "name", 1)
			description := genUTF8String(t, "description", 255)

			svcErr := ValidateDeliverableTypeFields(name, description, "")
			if svcErr != nil {
				t.Fatalf("expected 255-char description to be accepted, got error: %v", svcErr)
			}
		})
	})

	t.Run("boundary_description_length_256_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Exactly 256 characters should always be rejected
			name := genUTF8String(t, "name", 1)
			description := genUTF8String(t, "description", 256)

			svcErr := ValidateDeliverableTypeFields(name, description, "")
			if svcErr == nil {
				t.Fatal("expected 256-char description to be rejected")
			}
			if svcErr.Kind != ErrValidation {
				t.Fatalf("expected ErrValidation, got %v", svcErr.Kind)
			}
		})
	})

	t.Run("boundary_output_format_length_10000_accepted", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Exactly 10000 characters should always be accepted
			name := genUTF8String(t, "name", 1)
			outputFormat := genUTF8String(t, "outputFormat", 10000)

			svcErr := ValidateDeliverableTypeFields(name, "", outputFormat)
			if svcErr != nil {
				t.Fatalf("expected 10000-char output_format to be accepted, got error: %v", svcErr)
			}
		})
	})

	t.Run("boundary_output_format_length_10001_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Exactly 10001 characters should always be rejected
			name := genUTF8String(t, "name", 1)
			outputFormat := genUTF8String(t, "outputFormat", 10001)

			svcErr := ValidateDeliverableTypeFields(name, "", outputFormat)
			if svcErr == nil {
				t.Fatal("expected 10001-char output_format to be rejected")
			}
			if svcErr.Kind != ErrValidation {
				t.Fatalf("expected ErrValidation, got %v", svcErr.Kind)
			}
		})
	})
}

// genUTF8String generates a string with exactly the specified number of UTF-8
// runes. It uses a mix of ASCII and multi-byte characters to exercise the
// rune-counting logic.
func genUTF8String(t *rapid.T, label string, runeCount int) string {
	if runeCount == 0 {
		return ""
	}
	// For large strings, use a simple repeating pattern for performance
	if runeCount > 200 {
		// Mix of ASCII and multi-byte chars
		base := "abcéñ中"
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
	var sb strings.Builder
	for i := 0; i < runeCount; i++ {
		// Pick between ASCII and multi-byte characters
		charType := rapid.IntRange(0, 3).Draw(t, label+"_charType")
		switch charType {
		case 0:
			// ASCII letter
			sb.WriteRune(rune(rapid.IntRange('a', 'z').Draw(t, label+"_ascii")))
		case 1:
			// ASCII digit
			sb.WriteRune(rune(rapid.IntRange('0', '9').Draw(t, label+"_digit")))
		case 2:
			// Multi-byte: Latin extended (é, ñ, ü, etc.)
			sb.WriteRune(rune(rapid.IntRange(0xC0, 0xFF).Draw(t, label+"_latin")))
		case 3:
			// Multi-byte: CJK character
			sb.WriteRune(rune(rapid.IntRange(0x4E00, 0x4E50).Draw(t, label+"_cjk")))
		}
	}
	return sb.String()
}
