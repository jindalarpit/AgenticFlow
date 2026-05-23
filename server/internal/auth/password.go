package auth

const (
	// MinPasswordLength is the minimum acceptable password length for registration.
	MinPasswordLength = 8
	// MaxPasswordLength is the maximum acceptable password length for registration.
	MaxPasswordLength = 128
)

// ValidatePassword checks whether a password meets the length requirements.
// Returns true if the password is between MinPasswordLength and MaxPasswordLength characters.
func ValidatePassword(password string) bool {
	return len(password) >= MinPasswordLength && len(password) <= MaxPasswordLength
}
