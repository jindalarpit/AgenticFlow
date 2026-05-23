package auth

import "strings"

const (
	// MaxEmailLength is the maximum acceptable email length per RFC 5321.
	MaxEmailLength = 254
)

// ValidateEmail checks whether an email address meets the format requirements:
// - Contains exactly one '@'
// - Domain part (after '@') contains at least one '.'
// - Total length does not exceed MaxEmailLength (254 chars)
// Returns true if the email is valid.
func ValidateEmail(email string) bool {
	if len(email) == 0 || len(email) > MaxEmailLength {
		return false
	}

	// Must contain exactly one '@'.
	atIdx := strings.IndexByte(email, '@')
	if atIdx < 1 {
		return false
	}
	// Check there's no second '@'.
	if strings.IndexByte(email[atIdx+1:], '@') != -1 {
		return false
	}

	// Domain part must have at least one '.'.
	domain := email[atIdx+1:]
	if len(domain) == 0 {
		return false
	}
	if strings.IndexByte(domain, '.') == -1 {
		return false
	}

	return true
}
