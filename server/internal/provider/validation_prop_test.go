package provider

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// TestPropertyProviderNameValidation verifies that ValidateProviderName accepts
// names with length 1-128 and rejects names with length 0 or > 128.
//
// **Validates: Requirements 1.2, 1.15**
func TestPropertyProviderNameValidation(t *testing.T) {
	t.Run("names_with_valid_length_accepted", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			length := rapid.IntRange(1, 128).Draw(t, "length")
			// Generate a string of exactly the desired length using printable ASCII
			name := rapid.StringMatching(`[a-zA-Z0-9 _\-!@#]{1,128}`).Draw(t, "name")
			// Trim or pad to exact length
			if len(name) > length {
				name = name[:length]
			}
			for len(name) < length {
				name += "x"
			}

			err := ValidateProviderName(name)
			if err != nil {
				t.Fatalf("expected name of length %d to be accepted, got error: %v", len(name), err)
			}
		})
	})

	t.Run("empty_name_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			err := ValidateProviderName("")
			if err == nil {
				t.Fatal("expected empty name to be rejected")
			}
			if !strings.Contains(err.Error(), "name must be between 1 and 128 characters") {
				t.Fatalf("unexpected error message: %v", err)
			}
		})
	})

	t.Run("names_exceeding_max_length_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			length := rapid.IntRange(129, 200).Draw(t, "length")
			// Build a string that exceeds 128 chars
			name := strings.Repeat("a", length)

			err := ValidateProviderName(name)
			if err == nil {
				t.Fatalf("expected name of length %d to be rejected", len(name))
			}
			if !strings.Contains(err.Error(), "name must be between 1 and 128 characters") {
				t.Fatalf("unexpected error message: %v", err)
			}
		})
	})

	t.Run("boundary_length_1_accepted", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Single character names should always be accepted
			ch := rapid.StringMatching(`[a-zA-Z0-9]`).Draw(t, "char")

			err := ValidateProviderName(ch)
			if err != nil {
				t.Fatalf("expected single-char name %q to be accepted, got error: %v", ch, err)
			}
		})
	})

	t.Run("boundary_length_128_accepted", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Exactly 128 characters should always be accepted
			name := strings.Repeat("a", 128)

			err := ValidateProviderName(name)
			if err != nil {
				t.Fatalf("expected 128-char name to be accepted, got error: %v", err)
			}
		})
	})

	t.Run("boundary_length_129_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Exactly 129 characters should always be rejected
			name := strings.Repeat("b", 129)

			err := ValidateProviderName(name)
			if err == nil {
				t.Fatal("expected 129-char name to be rejected")
			}
		})
	})
}

// TestPropertyProviderTypeValidation verifies that ValidateProviderType accepts
// only the allowed provider types and rejects all others.
//
// **Validates: Requirements 1.2, 1.15**
func TestPropertyProviderTypeValidation(t *testing.T) {
	t.Run("allowed_types_accepted", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			idx := rapid.IntRange(0, len(AllowedProviderTypes)-1).Draw(t, "idx")
			providerType := AllowedProviderTypes[idx]

			err := ValidateProviderType(providerType)
			if err != nil {
				t.Fatalf("expected allowed type %q to be accepted, got error: %v", providerType, err)
			}
		})
	})

	t.Run("disallowed_types_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate random strings that are NOT in the allowed set
			candidate := rapid.StringMatching(`[a-z0-9_]{0,50}`).Draw(t, "candidate")

			// Skip if the generated string happens to be a valid type
			for _, allowed := range AllowedProviderTypes {
				if candidate == allowed {
					return
				}
			}

			err := ValidateProviderType(candidate)
			if err == nil {
				t.Fatalf("expected type %q to be rejected", candidate)
			}
			if !strings.Contains(err.Error(), "provider_type must be one of") {
				t.Fatalf("unexpected error message: %v", err)
			}
		})
	})

	t.Run("empty_type_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			err := ValidateProviderType("")
			if err == nil {
				t.Fatal("expected empty type to be rejected")
			}
		})
	})

	t.Run("case_sensitive_rejection", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Uppercase variants of valid types should be rejected
			idx := rapid.IntRange(0, len(AllowedProviderTypes)-1).Draw(t, "idx")
			providerType := strings.ToUpper(AllowedProviderTypes[idx])

			// Skip if uppercasing didn't change anything (shouldn't happen with our types)
			if providerType == AllowedProviderTypes[idx] {
				return
			}

			err := ValidateProviderType(providerType)
			if err == nil {
				t.Fatalf("expected uppercase type %q to be rejected (validation is case-sensitive)", providerType)
			}
		})
	})
}

// --- Property 3: Provider Credential Schema Validation ---

// genValidHTTPSURL generates a valid HTTPS URL within the given max length.
func genValidHTTPSURL(t *rapid.T, name string, maxLen int) string {
	// Generate a domain-like host portion
	host := rapid.StringMatching(`[a-z][a-z0-9]{2,30}\.[a-z]{2,4}`).Draw(t, name+"_host")
	url := "https://" + host
	if len(url) > maxLen {
		url = url[:maxLen]
	}
	return url
}

// genNonEmptyString generates a non-empty string within the given max length.
func genNonEmptyString(t *rapid.T, name string, maxLen int) string {
	upper := maxLen
	if upper > 64 {
		upper = 64
	}
	length := rapid.IntRange(1, upper).Draw(t, name+"_len")
	s := strings.Repeat("a", length)
	// Add some randomness
	suffix := rapid.StringMatching(`[a-zA-Z0-9]{0,10}`).Draw(t, name+"_sfx")
	s = suffix + s
	if len(s) > length {
		s = s[:length]
	}
	if len(s) == 0 {
		s = "x"
	}
	return s
}

// TestPropertyCredentialSchemaValidation verifies Property 3: for any provider type
// and credential JSON, the validation function accepts if and only if all required
// fields are present, non-empty, and within length constraints, and URL fields are
// valid HTTPS URLs.
//
// **Validates: Requirements 1.3, 1.4, 1.5, 1.6, 1.7**
func TestPropertyCredentialSchemaValidation(t *testing.T) {
	t.Run("valid_credentials_always_accepted", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Pick a random provider type
			providerType := rapid.SampledFrom(AllowedProviderTypes).Draw(t, "providerType")
			schema := credentialSchemas[providerType]

			// Build valid credentials: all required fields present, non-empty, within length
			fields := make(map[string]string)
			for _, field := range schema {
				if field.Required {
					if field.IsURL {
						fields[field.Name] = genValidHTTPSURL(t, field.Name, field.MaxLen)
					} else {
						fields[field.Name] = genNonEmptyString(t, field.Name, field.MaxLen)
					}
				} else {
					// Optionally include optional fields
					include := rapid.Bool().Draw(t, "include_"+field.Name)
					if include {
						if field.IsURL {
							fields[field.Name] = genValidHTTPSURL(t, field.Name, field.MaxLen)
						} else {
							fields[field.Name] = genNonEmptyString(t, field.Name, field.MaxLen)
						}
					}
				}
			}

			creds, err := json.Marshal(fields)
			if err != nil {
				t.Fatalf("failed to marshal credentials: %v", err)
			}

			err = ValidateCredentials(providerType, json.RawMessage(creds))
			if err != nil {
				t.Fatalf("expected valid credentials to pass for %s, got error: %v\ncreds: %s",
					providerType, err, string(creds))
			}
		})
	})

	t.Run("missing_required_field_always_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Pick a random provider type
			providerType := rapid.SampledFrom(AllowedProviderTypes).Draw(t, "providerType")
			schema := credentialSchemas[providerType]

			// Collect required fields
			var requiredFields []credentialField
			for _, field := range schema {
				if field.Required {
					requiredFields = append(requiredFields, field)
				}
			}
			if len(requiredFields) == 0 {
				return
			}

			// Pick a random required field to omit
			fieldToOmit := rapid.SampledFrom(requiredFields).Draw(t, "fieldToOmit")

			// Build credentials with all required fields EXCEPT the omitted one
			fields := make(map[string]string)
			for _, field := range schema {
				if field.Name == fieldToOmit.Name {
					continue
				}
				if field.Required {
					if field.IsURL {
						fields[field.Name] = genValidHTTPSURL(t, field.Name, field.MaxLen)
					} else {
						fields[field.Name] = genNonEmptyString(t, field.Name, field.MaxLen)
					}
				}
			}

			creds, err := json.Marshal(fields)
			if err != nil {
				t.Fatalf("failed to marshal credentials: %v", err)
			}

			err = ValidateCredentials(providerType, json.RawMessage(creds))
			if err == nil {
				t.Fatalf("expected validation to fail when required field %q is missing for %s, but it passed",
					fieldToOmit.Name, providerType)
			}

			// Verify error message mentions the missing field
			if !strings.Contains(err.Error(), fieldToOmit.Name) {
				t.Fatalf("error message should mention field %q, got: %v", fieldToOmit.Name, err)
			}
		})
	})

	t.Run("empty_required_field_always_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Pick a random provider type
			providerType := rapid.SampledFrom(AllowedProviderTypes).Draw(t, "providerType")
			schema := credentialSchemas[providerType]

			// Collect required fields
			var requiredFields []credentialField
			for _, field := range schema {
				if field.Required {
					requiredFields = append(requiredFields, field)
				}
			}
			if len(requiredFields) == 0 {
				return
			}

			// Pick a random required field to set to empty string
			fieldToEmpty := rapid.SampledFrom(requiredFields).Draw(t, "fieldToEmpty")

			// Build credentials with all required fields, but one is empty
			fields := make(map[string]string)
			for _, field := range schema {
				if field.Name == fieldToEmpty.Name {
					fields[field.Name] = "" // empty value
					continue
				}
				if field.Required {
					if field.IsURL {
						fields[field.Name] = genValidHTTPSURL(t, field.Name, field.MaxLen)
					} else {
						fields[field.Name] = genNonEmptyString(t, field.Name, field.MaxLen)
					}
				}
			}

			creds, err := json.Marshal(fields)
			if err != nil {
				t.Fatalf("failed to marshal credentials: %v", err)
			}

			err = ValidateCredentials(providerType, json.RawMessage(creds))
			if err == nil {
				t.Fatalf("expected validation to fail when required field %q is empty for %s, but it passed",
					fieldToEmpty.Name, providerType)
			}

			// Verify error message mentions the field
			if !strings.Contains(err.Error(), fieldToEmpty.Name) {
				t.Fatalf("error message should mention field %q, got: %v", fieldToEmpty.Name, err)
			}
		})
	})

	t.Run("field_exceeding_max_length_always_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Pick a random provider type
			providerType := rapid.SampledFrom(AllowedProviderTypes).Draw(t, "providerType")
			schema := credentialSchemas[providerType]

			// Pick a random field to overflow
			fieldToOverflow := rapid.SampledFrom(schema).Draw(t, "fieldToOverflow")

			// Build valid credentials first
			fields := make(map[string]string)
			for _, field := range schema {
				if field.Required || field.Name == fieldToOverflow.Name {
					if field.IsURL {
						fields[field.Name] = genValidHTTPSURL(t, field.Name, field.MaxLen)
					} else {
						fields[field.Name] = genNonEmptyString(t, field.Name, field.MaxLen)
					}
				}
			}

			// Overflow the chosen field
			overflowLen := rapid.IntRange(fieldToOverflow.MaxLen+1, fieldToOverflow.MaxLen+100).Draw(t, "overflowLen")
			if fieldToOverflow.IsURL {
				// For URL fields, generate a long HTTPS URL that exceeds max length
				fields[fieldToOverflow.Name] = "https://" + strings.Repeat("a", overflowLen-8)
			} else {
				fields[fieldToOverflow.Name] = strings.Repeat("x", overflowLen)
			}

			creds, err := json.Marshal(fields)
			if err != nil {
				t.Fatalf("failed to marshal credentials: %v", err)
			}

			err = ValidateCredentials(providerType, json.RawMessage(creds))
			if err == nil {
				t.Fatalf("expected validation to fail when field %q exceeds max length %d for %s, but it passed",
					fieldToOverflow.Name, fieldToOverflow.MaxLen, providerType)
			}

			// Verify error message mentions the field
			if !strings.Contains(err.Error(), fieldToOverflow.Name) {
				t.Fatalf("error message should mention field %q, got: %v", fieldToOverflow.Name, err)
			}
		})
	})

	t.Run("invalid_url_format_always_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Pick a random provider type
			providerType := rapid.SampledFrom(AllowedProviderTypes).Draw(t, "providerType")
			schema := credentialSchemas[providerType]

			// Collect URL fields for this provider
			var urlFields []credentialField
			for _, field := range schema {
				if field.IsURL {
					urlFields = append(urlFields, field)
				}
			}
			if len(urlFields) == 0 {
				return // skip providers with no URL fields (aws_bedrock)
			}

			// Pick a random URL field to make invalid
			fieldToInvalidate := rapid.SampledFrom(urlFields).Draw(t, "fieldToInvalidate")

			// Build valid credentials first
			fields := make(map[string]string)
			for _, field := range schema {
				if field.Required || field.Name == fieldToInvalidate.Name {
					if field.IsURL {
						fields[field.Name] = genValidHTTPSURL(t, field.Name, field.MaxLen)
					} else {
						fields[field.Name] = genNonEmptyString(t, field.Name, field.MaxLen)
					}
				}
			}

			// Generate an invalid URL (not HTTPS)
			invalidURLType := rapid.SampledFrom([]string{"http", "ftp", "no_scheme", "empty_scheme"}).Draw(t, "invalidURLType")
			var invalidURL string
			switch invalidURLType {
			case "http":
				invalidURL = "http://example.com/path"
			case "ftp":
				invalidURL = "ftp://files.example.com"
			case "no_scheme":
				invalidURL = "example.com/api"
			case "empty_scheme":
				invalidURL = "://broken.url"
			}
			fields[fieldToInvalidate.Name] = invalidURL

			creds, err := json.Marshal(fields)
			if err != nil {
				t.Fatalf("failed to marshal credentials: %v", err)
			}

			err = ValidateCredentials(providerType, json.RawMessage(creds))
			if err == nil {
				t.Fatalf("expected validation to fail for invalid URL in field %q for %s, but it passed\nURL: %s",
					fieldToInvalidate.Name, providerType, invalidURL)
			}

			// Verify error message mentions the field and HTTPS
			if !strings.Contains(err.Error(), fieldToInvalidate.Name) {
				t.Fatalf("error message should mention field %q, got: %v", fieldToInvalidate.Name, err)
			}
			if !strings.Contains(err.Error(), "HTTPS URL") {
				t.Fatalf("error message should mention HTTPS URL requirement, got: %v", err)
			}
		})
	})
}

// --- Property 15: Model Identifier Pattern Validation ---

// modelPatternRef is the reference regex used to verify the ValidateModelIdentifier function.
var modelPatternRef = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_./:@-]*$`)

// TestPropertyModelIdentifierPatternValidation verifies that for any random string,
// ValidateModelIdentifier accepts it if and only if it matches the pattern
// ^[a-zA-Z0-9][a-zA-Z0-9_./:@-]*$ and has length between 1 and 128.
//
// **Validates: Requirements 3.7**
func TestPropertyModelIdentifierPatternValidation(t *testing.T) {
	t.Run("valid_pattern_and_length_accepted", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate random strings that cover valid and invalid model identifiers
			s := rapid.StringMatching(`[a-zA-Z0-9_./:@\-]{1,128}`).Draw(t, "model")

			err := ValidateModelIdentifier(s)
			shouldAccept := len(s) >= 1 && len(s) <= 128 && modelPatternRef.MatchString(s)

			if shouldAccept && err != nil {
				t.Fatalf("expected acceptance for %q (len=%d, matches pattern), got error: %v", s, len(s), err)
			}
			if !shouldAccept && err == nil {
				t.Fatalf("expected rejection for %q (len=%d, matches=%v), got nil error", s, len(s), modelPatternRef.MatchString(s))
			}
		})
	})

	t.Run("valid_identifiers_always_accepted", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate a valid model identifier: starts with alphanumeric, followed by valid chars
			length := rapid.IntRange(1, 128).Draw(t, "length")
			firstChar := rapid.StringMatching(`[a-zA-Z0-9]`).Draw(t, "firstChar")

			var model string
			if length == 1 {
				model = firstChar
			} else {
				rest := rapid.StringMatching(fmt.Sprintf(`[a-zA-Z0-9_./:@-]{%d}`, length-1)).Draw(t, "rest")
				model = firstChar + rest
			}

			err := ValidateModelIdentifier(model)
			if err != nil {
				t.Fatalf("valid model %q (len=%d) should be accepted, got error: %v", model, len(model), err)
			}
		})
	})

	t.Run("empty_string_always_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			err := ValidateModelIdentifier("")
			if err == nil {
				t.Fatal("empty string should always be rejected")
			}
		})
	})

	t.Run("too_long_always_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate valid-pattern strings that exceed 128 chars
			length := rapid.IntRange(129, 200).Draw(t, "length")
			model := "a" + strings.Repeat("b", length-1)

			err := ValidateModelIdentifier(model)
			if err == nil {
				t.Fatalf("model with length %d should be rejected, got nil error", len(model))
			}
		})
	})

	t.Run("non_alphanumeric_start_always_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate strings that start with a non-alphanumeric character
			invalidStarts := []string{"-", ".", "/", ":", "@", "_", " ", "$", "!"}
			invalidStart := rapid.SampledFrom(invalidStarts).Draw(t, "invalidStart")
			restLen := rapid.IntRange(0, 50).Draw(t, "restLen")
			rest := strings.Repeat("a", restLen)
			model := invalidStart + rest

			// Only test if within length bounds (to isolate the start-char property)
			if len(model) >= 1 && len(model) <= 128 {
				err := ValidateModelIdentifier(model)
				if err == nil {
					t.Fatalf("model %q starting with non-alphanumeric should be rejected", model)
				}
			}
		})
	})

	t.Run("invalid_chars_always_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate a string with a valid start but containing at least one invalid char
			prefixLen := rapid.IntRange(1, 20).Draw(t, "prefixLen")
			prefix := "a" + strings.Repeat("b", prefixLen-1)
			invalidChars := []string{" ", "$", "!", "#", "%", "^", "&", "*", "(", ")", "+", "=", "{", "}", "|", "<", ">", ",", "?", "~"}
			invalidChar := rapid.SampledFrom(invalidChars).Draw(t, "invalidChar")
			suffixLen := rapid.IntRange(0, 20).Draw(t, "suffixLen")
			suffix := strings.Repeat("c", suffixLen)
			model := prefix + invalidChar + suffix

			// Only test if within length bounds
			if len(model) >= 1 && len(model) <= 128 {
				err := ValidateModelIdentifier(model)
				if err == nil {
					t.Fatalf("model %q containing invalid char %q should be rejected", model, invalidChar)
				}
			}
		})
	})
}
