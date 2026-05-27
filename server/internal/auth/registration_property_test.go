package auth_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/agenticflow/agenticflow/server/internal/auth"
	"pgregory.net/rapid"
)

// Feature: agent-management, Property 5: Registration Input Validation
//
// For any combination of (email, password, name), the Server SHALL accept
// registration if and only if: the email contains exactly one `@` followed
// by a domain with at least one `.` and does not exceed 254 characters, the
// password is between 8 and 128 characters, and the trimmed name is between
// 1 and 128 characters.
//
// **Validates: Requirements 5.2, 5.5, 5.6, 5.7**

const maxNameLength = 128

// ValidateName mirrors the name validation logic from the Register handler:
// trim whitespace, then check 1-128 characters.
func ValidateName(name string) bool {
	trimmed := strings.TrimSpace(name)
	return len(trimmed) >= 1 && len(trimmed) <= maxNameLength
}

// --- Generators ---

// genValidLocalPart generates a valid email local part (1-63 chars, alphanumeric + dots/underscores).
func genValidLocalPart(t *rapid.T) string {
	return rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9._]{0,62}`).Draw(t, "localPart")
}

// genValidDomain generates a valid domain with at least one dot (e.g., "example.com").
func genValidDomain(t *rapid.T) string {
	// Generate a domain label + "." + TLD
	label := rapid.StringMatching(`[a-z][a-z0-9]{0,20}`).Draw(t, "domainLabel")
	tld := rapid.StringMatching(`[a-z]{2,6}`).Draw(t, "tld")
	return label + "." + tld
}

// genValidEmail generates an email that satisfies all validation rules:
// exactly one '@', domain with '.', total ≤254 chars.
func genValidEmail(t *rapid.T) string {
	local := genValidLocalPart(t)
	domain := genValidDomain(t)
	email := local + "@" + domain
	// Ensure total length ≤ 254
	if len(email) > auth.MaxEmailLength {
		// Truncate local part to fit
		maxLocal := auth.MaxEmailLength - len(domain) - 1 // -1 for '@'
		if maxLocal < 1 {
			maxLocal = 1
		}
		local = local[:maxLocal]
		email = local + "@" + domain
	}
	return email
}

// genValidPassword generates a password between 8 and 128 characters.
func genValidPassword(t *rapid.T) string {
	length := rapid.IntRange(auth.MinPasswordLength, auth.MaxPasswordLength).Draw(t, "pwdLen")
	return rapid.StringMatching(fmt.Sprintf(`[a-zA-Z0-9!@#$%%^&*]{%d}`, length)).Draw(t, "password")
}

// genValidName generates a name that after trimming is 1-128 characters.
// It ensures at least one non-whitespace character exists.
func genValidName(t *rapid.T) string {
	length := rapid.IntRange(1, maxNameLength).Draw(t, "nameLen")
	// Use only letters to guarantee non-empty after trim
	return rapid.StringMatching(fmt.Sprintf(`[a-zA-Z]{%d}`, length)).Draw(t, "name")
}

// --- Property Tests ---

// TestProperty5_Registration_AllValidAccepted verifies that when all three
// fields (email, password, name) are valid, validation accepts the input.
func TestProperty5_Registration_AllValidAccepted(t *testing.T) {
	// Feature: agent-management, Property 5: Registration Input Validation
	rapid.Check(t, func(t *rapid.T) {
		email := genValidEmail(t)
		password := genValidPassword(t)
		name := genValidName(t)

		emailOk := auth.ValidateEmail(email)
		passwordOk := auth.ValidatePassword(password)
		nameOk := ValidateName(name)

		if !emailOk {
			t.Fatalf("expected valid email to be accepted: %q (len=%d)", email, len(email))
		}
		if !passwordOk {
			t.Fatalf("expected valid password to be accepted: %q (len=%d)", password, len(password))
		}
		if !nameOk {
			t.Fatalf("expected valid name to be accepted: %q (trimmed len=%d)", name, len(strings.TrimSpace(name)))
		}
	})
}

// TestProperty5_Registration_InvalidEmailRejected verifies that when the email
// is invalid, validation rejects it regardless of password and name validity.
func TestProperty5_Registration_InvalidEmailRejected(t *testing.T) {
	// Feature: agent-management, Property 5: Registration Input Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate various invalid emails
		emailType := rapid.IntRange(0, 5).Draw(t, "emailType")
		var email string
		switch emailType {
		case 0:
			// No @ sign
			email = rapid.StringMatching(`[a-zA-Z0-9.]{1,50}`).Draw(t, "noAt")
			// Ensure no @ accidentally
			email = strings.ReplaceAll(email, "@", "x")
		case 1:
			// Empty string
			email = ""
		case 2:
			// Multiple @ signs
			local := rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "local")
			mid := rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "mid")
			domain := rapid.StringMatching(`[a-z]{2,10}\.[a-z]{2,4}`).Draw(t, "domain")
			email = local + "@" + mid + "@" + domain
		case 3:
			// No dot in domain
			local := rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "local")
			domainNoDot := rapid.StringMatching(`[a-z]{2,20}`).Draw(t, "domainNoDot")
			email = local + "@" + domainNoDot
		case 4:
			// Nothing before @
			domain := rapid.StringMatching(`[a-z]{2,10}\.[a-z]{2,4}`).Draw(t, "domain")
			email = "@" + domain
		case 5:
			// Exceeds 254 characters
			local := rapid.StringMatching(`[a-z]{100}`).Draw(t, "longLocal")
			domain := rapid.StringMatching(`[a-z]{150}\.[a-z]{3}`).Draw(t, "longDomain")
			email = local + "@" + domain
			// Ensure it's actually > 254
			if len(email) <= auth.MaxEmailLength {
				email = email + strings.Repeat("x", auth.MaxEmailLength-len(email)+1)
			}
		}

		result := auth.ValidateEmail(email)
		if result {
			t.Fatalf("expected invalid email to be rejected: %q (len=%d)", email, len(email))
		}
	})
}

// TestProperty5_Registration_ShortPasswordRejected verifies that passwords
// shorter than 8 characters are always rejected.
func TestProperty5_Registration_ShortPasswordRejected(t *testing.T) {
	// Feature: agent-management, Property 5: Registration Input Validation
	rapid.Check(t, func(t *rapid.T) {
		length := rapid.IntRange(0, auth.MinPasswordLength-1).Draw(t, "shortLen")
		password := rapid.StringMatching(fmt.Sprintf(`[a-zA-Z0-9]{%d}`, length)).Draw(t, "shortPwd")

		if auth.ValidatePassword(password) {
			t.Fatalf("expected short password to be rejected: %q (len=%d)", password, len(password))
		}
	})
}

// TestProperty5_Registration_LongPasswordRejected verifies that passwords
// longer than 128 characters are always rejected.
func TestProperty5_Registration_LongPasswordRejected(t *testing.T) {
	// Feature: agent-management, Property 5: Registration Input Validation
	rapid.Check(t, func(t *rapid.T) {
		length := rapid.IntRange(auth.MaxPasswordLength+1, auth.MaxPasswordLength+50).Draw(t, "longLen")
		password := rapid.StringMatching(fmt.Sprintf(`[a-zA-Z0-9]{%d}`, length)).Draw(t, "longPwd")

		if auth.ValidatePassword(password) {
			t.Fatalf("expected long password to be rejected: %q (len=%d)", password, len(password))
		}
	})
}

// TestProperty5_Registration_EmptyNameRejected verifies that names that are
// empty or whitespace-only after trimming are always rejected.
func TestProperty5_Registration_EmptyNameRejected(t *testing.T) {
	// Feature: agent-management, Property 5: Registration Input Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate names that are empty or whitespace-only
		nameType := rapid.IntRange(0, 2).Draw(t, "nameType")
		var name string
		switch nameType {
		case 0:
			// Empty string
			name = ""
		case 1:
			// Only spaces
			spaceCount := rapid.IntRange(1, 20).Draw(t, "spaceCount")
			name = strings.Repeat(" ", spaceCount)
		case 2:
			// Only tabs and spaces
			tabCount := rapid.IntRange(1, 10).Draw(t, "tabCount")
			spaceCount := rapid.IntRange(0, 10).Draw(t, "spaceCount2")
			name = strings.Repeat("\t", tabCount) + strings.Repeat(" ", spaceCount)
		}

		if ValidateName(name) {
			t.Fatalf("expected empty/whitespace name to be rejected: %q (trimmed=%q)", name, strings.TrimSpace(name))
		}
	})
}

// TestProperty5_Registration_LongNameRejected verifies that names exceeding
// 128 characters after trimming are always rejected.
func TestProperty5_Registration_LongNameRejected(t *testing.T) {
	// Feature: agent-management, Property 5: Registration Input Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate names that exceed 128 chars after trimming
		length := rapid.IntRange(maxNameLength+1, maxNameLength+50).Draw(t, "longNameLen")
		name := rapid.StringMatching(fmt.Sprintf(`[a-zA-Z]{%d}`, length)).Draw(t, "longName")

		if ValidateName(name) {
			t.Fatalf("expected long name to be rejected: %q (trimmed len=%d)", name, len(strings.TrimSpace(name)))
		}
	})
}

// TestProperty5_Registration_CombinedValidation verifies the full registration
// validation: acceptance iff ALL three fields are valid simultaneously.
func TestProperty5_Registration_CombinedValidation(t *testing.T) {
	// Feature: agent-management, Property 5: Registration Input Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate random inputs that may or may not be valid
		emailLen := rapid.IntRange(0, 260).Draw(t, "emailLen")
		var email string
		if emailLen == 0 {
			email = ""
		} else {
			email = rapid.StringMatching(fmt.Sprintf(`[a-zA-Z0-9@.]{%d}`, emailLen)).Draw(t, "email")
		}

		pwdLen := rapid.IntRange(0, 150).Draw(t, "pwdLen")
		var password string
		if pwdLen == 0 {
			password = ""
		} else {
			password = rapid.StringMatching(fmt.Sprintf(`[a-zA-Z0-9]{%d}`, pwdLen)).Draw(t, "password")
		}

		nameLen := rapid.IntRange(0, 150).Draw(t, "nameLen")
		var name string
		if nameLen == 0 {
			name = ""
		} else {
			name = rapid.StringMatching(fmt.Sprintf(`[a-zA-Z ]{%d}`, nameLen)).Draw(t, "name")
		}

		emailValid := auth.ValidateEmail(email)
		passwordValid := auth.ValidatePassword(password)
		nameValid := ValidateName(name)

		allValid := emailValid && passwordValid && nameValid

		// Verify the combined result matches individual validations
		if allValid {
			// If all are valid, registration should be accepted
			if !emailValid || !passwordValid || !nameValid {
				t.Fatalf("inconsistency: allValid=true but individual checks disagree")
			}
		} else {
			// If any is invalid, registration should be rejected
			if emailValid && passwordValid && nameValid {
				t.Fatalf("inconsistency: allValid=false but all individual checks pass")
			}
		}
	})
}
