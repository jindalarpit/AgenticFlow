package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
	"github.com/agenticflow/agenticflow/shared/pgutil"
)

const (
	// PATPrefix is the prefix for all AgenticFlow personal access tokens.
	PATPrefix = "af_"

	// PATExpiry is the default lifetime for a PAT (90 days).
	PATExpiry = 90 * 24 * time.Hour

	// patRandomBytes is the number of random bytes used to generate a PAT.
	patRandomBytes = 32
)

// GeneratePAT creates a new personal access token with the af_ prefix.
// It returns the raw token (to be shown to the user once) and the SHA-256
// hash (to be stored in the database).
func GeneratePAT() (token string, hash string, err error) {
	b := make([]byte, patRandomBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate PAT: %w", err)
	}
	token = PATPrefix + hex.EncodeToString(b)
	hash = HashToken(token)
	return token, hash, nil
}

// HashToken returns the hex-encoded SHA-256 hash of the given token string.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// ValidatePAT looks up a token by its hash in the database, checks that it
// has not expired, and returns the owning user's ID. Returns an error if the
// token is not found or has expired (the SQL query filters expired tokens).
func ValidatePAT(ctx context.Context, queries *db.Queries, token string) (userID string, err error) {
	hash := HashToken(token)
	pat, err := queries.GetTokenByHash(ctx, hash)
	if err != nil {
		return "", fmt.Errorf("invalid or expired token: %w", err)
	}
	return pgutil.UUIDToString(pat.UserID), nil
}
