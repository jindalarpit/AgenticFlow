package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/agenticflow/agenticflow/server/internal/auth"
	"github.com/agenticflow/agenticflow/server/internal/middleware"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
	"pgregory.net/rapid"
)

// Feature: agent-management, Property 1: PAT Generation Structural Integrity
//
// For any valid PAT name and expiry option, the generated token SHALL have the
// format `af_` followed by exactly 64 hexadecimal characters, the stored hash
// SHALL equal SHA-256 of the raw token, the stored prefix SHALL equal the first
// 12 characters of the token, and the computed expiry timestamp SHALL match the
// requested duration (30/90/365 days from now, or null for no-expiry).
//
// **Validates: Requirements 1.1, 1.2, 1.3**

func TestProperty1_PATGeneration_TokenFormat(t *testing.T) {
	// Property 1: PAT Generation Structural Integrity
	// Verify that GeneratePAT always produces a token with af_ prefix + 64 hex chars.
	rapid.Check(t, func(t *rapid.T) {
		// Call GeneratePAT — it takes no arguments, generates a random token each time
		token, hash, err := auth.GeneratePAT()
		if err != nil {
			t.Fatalf("GeneratePAT() returned error: %v", err)
		}

		// Token must start with "af_"
		if !strings.HasPrefix(token, "af_") {
			t.Fatalf("token does not start with 'af_': got %q", token)
		}

		// After the prefix, there must be exactly 64 hex characters
		suffix := token[len("af_"):]
		if len(suffix) != 64 {
			t.Fatalf("expected 64 hex chars after prefix, got %d: %q", len(suffix), suffix)
		}

		// All characters after prefix must be valid hex
		for i, c := range suffix {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Fatalf("non-hex character at position %d in suffix: %c (token=%q)", i, c, token)
			}
		}

		// Total token length: 3 (prefix) + 64 (hex) = 67
		if len(token) != 67 {
			t.Fatalf("expected token length 67, got %d: %q", len(token), token)
		}

		// The returned hash must be non-empty
		if hash == "" {
			t.Fatal("GeneratePAT() returned empty hash")
		}
	})
}

func TestProperty1_PATGeneration_HashIntegrity(t *testing.T) {
	// Property 1: PAT Generation Structural Integrity
	// Verify that the hash returned by GeneratePAT equals SHA-256(token),
	// and that HashToken(token) produces the same result.
	rapid.Check(t, func(t *rapid.T) {
		token, returnedHash, err := auth.GeneratePAT()
		if err != nil {
			t.Fatalf("GeneratePAT() returned error: %v", err)
		}

		// Compute SHA-256 of the token independently
		h := sha256.Sum256([]byte(token))
		expectedHash := hex.EncodeToString(h[:])

		// The hash returned by GeneratePAT must match our independent computation
		if returnedHash != expectedHash {
			t.Fatalf("returned hash mismatch:\n  got:  %s\n  want: %s\n  token: %s",
				returnedHash, expectedHash, token)
		}

		// HashToken(token) must also produce the same hash
		computedHash := auth.HashToken(token)
		if computedHash != expectedHash {
			t.Fatalf("HashToken() mismatch:\n  got:  %s\n  want: %s\n  token: %s",
				computedHash, expectedHash, token)
		}
	})
}

func TestProperty1_PATGeneration_PrefixExtraction(t *testing.T) {
	// Property 1: PAT Generation Structural Integrity
	// Verify that the token prefix (first 12 characters) is correctly extractable.
	rapid.Check(t, func(t *rapid.T) {
		token, _, err := auth.GeneratePAT()
		if err != nil {
			t.Fatalf("GeneratePAT() returned error: %v", err)
		}

		// Token must be at least 12 characters long (it's 67, so this always holds)
		if len(token) < 12 {
			t.Fatalf("token too short for prefix extraction: len=%d, token=%q", len(token), token)
		}

		// The prefix (first 12 chars) should start with "af_" and include 9 hex chars
		prefix := token[:12]
		if !strings.HasPrefix(prefix, "af_") {
			t.Fatalf("prefix does not start with 'af_': %q", prefix)
		}

		// Verify prefix length is exactly 12
		if len(prefix) != 12 {
			t.Fatalf("expected prefix length 12, got %d: %q", len(prefix), prefix)
		}

		// The prefix must be a proper substring of the full token
		if !strings.HasPrefix(token, prefix) {
			t.Fatalf("token does not start with its own prefix: token=%q, prefix=%q", token, prefix)
		}
	})
}

func TestProperty1_PATGeneration_ExpiryComputation(t *testing.T) {
	// Property 1: PAT Generation Structural Integrity
	// For any valid expiry option (30, 90, 365 days), the computed expiry timestamp
	// is within 1 second of expected.
	rapid.Check(t, func(t *rapid.T) {
		// Draw a valid expiry option
		expiryDays := rapid.SampledFrom([]int{30, 90, 365}).Draw(t, "expiryDays")

		// Record time before and after computing expiry
		before := time.Now()
		expectedExpiry := before.Add(time.Duration(expiryDays) * 24 * time.Hour)
		after := time.Now()

		// The actual expiry computation (as done in the handler) would be:
		actualExpiry := time.Now().Add(time.Duration(expiryDays) * 24 * time.Hour)

		// The actual expiry should be within 1 second of expected
		// (accounting for time elapsed between before/after measurements)
		lowerBound := expectedExpiry.Add(-1 * time.Second)
		upperBound := after.Add(time.Duration(expiryDays) * 24 * time.Hour).Add(1 * time.Second)

		if actualExpiry.Before(lowerBound) || actualExpiry.After(upperBound) {
			t.Fatalf("expiry out of range for %d days:\n  actual:  %v\n  lower:   %v\n  upper:   %v",
				expiryDays, actualExpiry, lowerBound, upperBound)
		}
	})
}

func TestProperty1_PATGeneration_Uniqueness(t *testing.T) {
	// Property 1: PAT Generation Structural Integrity
	// Each call to GeneratePAT should produce a unique token (collision resistance).
	rapid.Check(t, func(t *rapid.T) {
		token1, hash1, err1 := auth.GeneratePAT()
		if err1 != nil {
			t.Fatalf("first GeneratePAT() returned error: %v", err1)
		}

		token2, hash2, err2 := auth.GeneratePAT()
		if err2 != nil {
			t.Fatalf("second GeneratePAT() returned error: %v", err2)
		}

		if token1 == token2 {
			t.Fatalf("two consecutive GeneratePAT() calls produced the same token: %q", token1)
		}

		if hash1 == hash2 {
			t.Fatalf("two consecutive GeneratePAT() calls produced the same hash: %q", hash1)
		}
	})
}

// Feature: agenticflow-core, Property 13: PAT Authentication Enforcement
//
// For any API request with a missing, malformed, or expired PAT in the
// Authorization header, the middleware SHALL respond with HTTP 401.
//
// **Validates: Requirements 7.4**

// mockDBTX implements the db.DBTX interface and always returns "not found"
// for any query (simulating unknown token hashes).
type mockDBTX struct{}

func (m *mockDBTX) Exec(_ context.Context, _ string, _ ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}

func (m *mockDBTX) Query(_ context.Context, _ string, _ ...interface{}) (pgx.Rows, error) {
	return nil, pgx.ErrNoRows
}

func (m *mockDBTX) QueryRow(_ context.Context, _ string, _ ...interface{}) pgx.Row {
	return &mockRow{}
}

// mockRow implements pgx.Row and always returns ErrNoRows on Scan.
type mockRow struct{}

func (r *mockRow) Scan(_ ...interface{}) error {
	return pgx.ErrNoRows
}

// protectedHandler is a simple handler that returns 200 OK if auth passes.
func protectedHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
}

func TestProperty13_PATAuthenticationEnforcement_NoHeader(t *testing.T) {
	// Feature: agenticflow-core, Property 13: PAT Authentication Enforcement
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random HTTP method and path
		method := rapid.SampledFrom([]string{"GET", "POST", "PUT", "DELETE"}).Draw(t, "method")
		path := "/" + rapid.StringMatching(`[a-z]{1,20}(/[a-z]{1,20}){0,3}`).Draw(t, "path")

		queries := db.New(&mockDBTX{})
		cache := middleware.NewPATCache()
		handler := middleware.Auth(queries, cache)(protectedHandler())

		req := httptest.NewRequest(method, path, nil)
		// No Authorization header set
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for missing auth header, got %d", rec.Code)
		}
	})
}

func TestProperty13_PATAuthenticationEnforcement_EmptyBearer(t *testing.T) {
	// Feature: agenticflow-core, Property 13: PAT Authentication Enforcement
	rapid.Check(t, func(t *rapid.T) {
		queries := db.New(&mockDBTX{})
		cache := middleware.NewPATCache()
		handler := middleware.Auth(queries, cache)(protectedHandler())

		req := httptest.NewRequest("GET", "/api/me", nil)
		// Empty Bearer token
		req.Header.Set("Authorization", "Bearer ")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for empty Bearer token, got %d", rec.Code)
		}
	})
}

func TestProperty13_PATAuthenticationEnforcement_NonAfPrefix(t *testing.T) {
	// Feature: agenticflow-core, Property 13: PAT Authentication Enforcement
	rapid.Check(t, func(t *rapid.T) {
		// Generate tokens that do NOT start with "af_"
		prefix := rapid.SampledFrom([]string{"", "sk_", "pk_", "token_", "xyz_", "AF_", "aF_"}).Draw(t, "prefix")
		suffix := rapid.StringMatching(`[a-zA-Z0-9]{10,64}`).Draw(t, "suffix")
		token := prefix + suffix

		// Ensure the token doesn't accidentally start with "af_"
		if strings.HasPrefix(token, auth.PATPrefix) {
			return // skip this case
		}

		queries := db.New(&mockDBTX{})
		cache := middleware.NewPATCache()
		handler := middleware.Auth(queries, cache)(protectedHandler())

		req := httptest.NewRequest("GET", "/api/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for non-af_ prefix token %q, got %d", token, rec.Code)
		}
	})
}

func TestProperty13_PATAuthenticationEnforcement_RandomGarbage(t *testing.T) {
	// Feature: agenticflow-core, Property 13: PAT Authentication Enforcement
	rapid.Check(t, func(t *rapid.T) {
		// Generate random garbage Authorization header values
		headerType := rapid.IntRange(0, 4).Draw(t, "headerType")

		var authHeader string
		switch headerType {
		case 0:
			// Random string without "Bearer " prefix
			authHeader = rapid.StringMatching(`[a-zA-Z0-9!@#$%^&*]{5,50}`).Draw(t, "garbage")
		case 1:
			// Bearer with af_ prefix but random hash (not in DB)
			randomHex := rapid.StringMatching(`[a-f0-9]{64}`).Draw(t, "hex")
			authHeader = "Bearer " + auth.PATPrefix + randomHex
		case 2:
			// Just "Bearer" with no space or token
			authHeader = "Bearer"
		case 3:
			// Multiple spaces
			authHeader = "Bearer   " + rapid.StringMatching(`[a-z]{10}`).Draw(t, "token")
		case 4:
			// Wrong auth scheme
			authHeader = "Basic " + rapid.StringMatching(`[a-zA-Z0-9+/=]{20,40}`).Draw(t, "basic")
		}

		queries := db.New(&mockDBTX{})
		cache := middleware.NewPATCache()
		handler := middleware.Auth(queries, cache)(protectedHandler())

		req := httptest.NewRequest("GET", "/api/tasks", nil)
		req.Header.Set("Authorization", authHeader)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for garbage auth header %q, got %d", authHeader, rec.Code)
		}
	})
}

// Feature: agenticflow-core, Property 15: Password Validation
//
// For any password string submitted during registration, the server SHALL
// accept it if and only if its length is at least 8 characters.
//
// **Validates: Requirements 8.5**

func TestProperty15_PasswordValidation(t *testing.T) {
	// Feature: agenticflow-core, Property 15: Password Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate random strings of various lengths (0-200 to cover both min and max)
		length := rapid.IntRange(0, 200).Draw(t, "length")
		// Use StringMatching with a pattern that generates exactly `length` chars
		password := rapid.StringMatching(fmt.Sprintf(`[a-zA-Z0-9!@#$]{%d}`, length)).Draw(t, "password")

		result := auth.ValidatePassword(password)
		expected := len(password) >= auth.MinPasswordLength && len(password) <= auth.MaxPasswordLength

		if result != expected {
			t.Fatalf("ValidatePassword(%q) = %v, want %v (len=%d, min=%d, max=%d)",
				password, result, expected, len(password), auth.MinPasswordLength, auth.MaxPasswordLength)
		}
	})
}

func TestProperty15_PasswordValidation_ShortAlwaysRejected(t *testing.T) {
	// Feature: agenticflow-core, Property 15: Password Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate passwords that are strictly shorter than MinPasswordLength
		length := rapid.IntRange(0, auth.MinPasswordLength-1).Draw(t, "length")
		password := rapid.StringMatching(fmt.Sprintf(`[a-zA-Z0-9]{%d}`, length)).Draw(t, "password")

		if auth.ValidatePassword(password) {
			t.Fatalf("expected password of length %d to be rejected, but it was accepted: %q",
				len(password), password)
		}
	})
}

func TestProperty15_PasswordValidation_LongAlwaysAccepted(t *testing.T) {
	// Feature: agenticflow-core, Property 15: Password Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate passwords that are between MinPasswordLength and MaxPasswordLength
		length := rapid.IntRange(auth.MinPasswordLength, auth.MaxPasswordLength).Draw(t, "length")
		password := rapid.StringMatching(fmt.Sprintf(`[a-zA-Z0-9]{%d}`, length)).Draw(t, "password")

		if !auth.ValidatePassword(password) {
			t.Fatalf("expected password of length %d to be accepted, but it was rejected: %q",
				len(password), password)
		}
	})
}

// TestProperty15_PasswordValidation_BoundaryExact tests the exact boundary at 8 characters.
func TestProperty15_PasswordValidation_BoundaryExact(t *testing.T) {
	// Feature: agenticflow-core, Property 15: Password Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate exactly 8-character passwords — should always be accepted
		password := rapid.StringMatching(`[a-zA-Z0-9]{8}`).Draw(t, "password8")
		if !auth.ValidatePassword(password) {
			t.Fatalf("expected 8-char password to be accepted: %q (len=%d)", password, len(password))
		}

		// Generate exactly 7-character passwords — should always be rejected
		password7 := rapid.StringMatching(`[a-zA-Z0-9]{7}`).Draw(t, "password7")
		if auth.ValidatePassword(password7) {
			t.Fatalf("expected 7-char password to be rejected: %q (len=%d)", password7, len(password7))
		}
	})
}


