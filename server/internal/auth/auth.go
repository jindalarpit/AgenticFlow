// Package auth provides token management, PAT generation/validation, and OAuth flow.
//
// This package contains:
//   - GeneratePAT: creates a new personal access token with af_ prefix
//   - HashToken: SHA-256 hashes a token for storage/lookup
//   - ValidatePAT: validates a token against the database
package auth
