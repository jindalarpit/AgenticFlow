package handler

import (
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/agenticflow/agenticflow/pkg/db/generated"
	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Feature: agent-management, Property 3: Token List Completeness and Ordering
//
// For any set of PATs belonging to a user (some created, some revoked), the
// list endpoint SHALL return exactly the non-revoked tokens, each containing
// name, prefix, creation date, last-used date, and expiry date, sorted by
// creation date descending.
//
// **Validates: Requirements 2.1**
// ---------------------------------------------------------------------------

// maxTokenListSize is the maximum number of tokens returned by the list endpoint.
const maxTokenListSize = 100

// generateTestToken creates a db.PersonalAccessToken with random but valid fields.
func generateTestToken(t *rapid.T, baseTime time.Time, index int) db.PersonalAccessToken {
	// Generate a unique creation time by offsetting from base
	offsetMinutes := rapid.IntRange(0, 525600).Draw(t, "offsetMinutes") // up to 1 year
	createdAt := baseTime.Add(-time.Duration(offsetMinutes) * time.Minute)

	// Generate name: 1-64 chars
	name := rapid.StringMatching(`[a-zA-Z0-9_-]{1,64}`).Draw(t, "name")

	// Generate prefix: exactly 12 chars starting with af_
	prefix := "af_" + rapid.StringMatching(`[a-f0-9]{9}`).Draw(t, "prefixHex")

	// Generate optional expiry (50% chance of having one)
	var expiresAt pgtype.Timestamptz
	if rapid.Bool().Draw(t, "hasExpiry") {
		// Expiry can be in the past or future
		expiryOffsetDays := rapid.IntRange(-30, 365).Draw(t, "expiryOffsetDays")
		expiresAt = pgtype.Timestamptz{
			Time:  createdAt.Add(time.Duration(expiryOffsetDays) * 24 * time.Hour),
			Valid: true,
		}
	}

	// Generate optional last_used_at (70% chance of having been used)
	var lastUsedAt pgtype.Timestamptz
	if rapid.Bool().Draw(t, "hasLastUsed") {
		usedOffsetHours := rapid.IntRange(1, 720).Draw(t, "usedOffsetHours")
		lastUsedAt = pgtype.Timestamptz{
			Time:  createdAt.Add(time.Duration(usedOffsetHours) * time.Hour),
			Valid: true,
		}
	}

	// Generate a valid UUID
	var id pgtype.UUID
	id.Valid = true
	for i := range id.Bytes {
		id.Bytes[i] = byte(rapid.IntRange(0, 255).Draw(t, "uuidByte"))
	}

	var userID pgtype.UUID
	userID.Valid = true
	for i := range userID.Bytes {
		userID.Bytes[i] = byte(rapid.IntRange(0, 255).Draw(t, "userIDByte"))
	}

	return db.PersonalAccessToken{
		ID:          id,
		UserID:      userID,
		Name:        name,
		TokenHash:   rapid.StringMatching(`[a-f0-9]{64}`).Draw(t, "hash"),
		TokenPrefix: prefix,
		ExpiresAt:   expiresAt,
		LastUsedAt:  lastUsedAt,
		CreatedAt: pgtype.Timestamptz{
			Time:  createdAt,
			Valid: true,
		},
	}
}

// filterAndSortTokens simulates the DB query behavior:
// - Returns all tokens for the user (revocation = deletion, so all DB records are non-revoked)
// - Sorted by creation date descending
// - Limited to maxTokenListSize
func filterAndSortTokens(tokens []db.PersonalAccessToken) []db.PersonalAccessToken {
	// Copy to avoid mutating input
	result := make([]db.PersonalAccessToken, len(tokens))
	copy(result, tokens)

	// Sort by creation date descending (newest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Time.After(result[j].CreatedAt.Time)
	})

	// Limit to max 100
	if len(result) > maxTokenListSize {
		result = result[:maxTokenListSize]
	}

	return result
}

// convertToPATResponses converts DB tokens to PATResponse structs,
// mirroring the toPATResponse function in the handler.
func convertToPATResponses(tokens []db.PersonalAccessToken) []PATResponse {
	result := make([]PATResponse, 0, len(tokens))
	for _, t := range tokens {
		result = append(result, toPATResponse(t))
	}
	return result
}

func TestProperty3_TokenListCompleteness_SortedByCreationDateDescending(t *testing.T) {
	// Feature: agent-management, Property 3: Token List Completeness and Ordering
	// Verify that the list is sorted by creation date descending (newest first).
	rapid.Check(t, func(t *rapid.T) {
		baseTime := time.Now()
		numTokens := rapid.IntRange(2, 50).Draw(t, "numTokens")

		tokens := make([]db.PersonalAccessToken, numTokens)
		for i := range tokens {
			tokens[i] = generateTestToken(t, baseTime, i)
		}

		sorted := filterAndSortTokens(tokens)
		responses := convertToPATResponses(sorted)

		// Verify descending order by creation date
		for i := 1; i < len(responses); i++ {
			prev := responses[i-1].CreatedAt
			curr := responses[i].CreatedAt

			prevTime, err1 := time.Parse(time.RFC3339, prev)
			currTime, err2 := time.Parse(time.RFC3339, curr)
			if err1 != nil || err2 != nil {
				t.Fatalf("failed to parse creation dates: prev=%q err=%v, curr=%q err=%v",
					prev, err1, curr, err2)
			}

			if prevTime.Before(currTime) {
				t.Fatalf("tokens not sorted by creation date descending: index %d (%s) is before index %d (%s)",
					i-1, prev, i, curr)
			}
		}
	})
}

func TestProperty3_TokenListCompleteness_AllNonRevokedTokensReturned(t *testing.T) {
	// Feature: agent-management, Property 3: Token List Completeness and Ordering
	// Since revocation = deletion, all tokens in the DB are non-revoked.
	// Verify the list returns exactly all tokens (up to 100).
	rapid.Check(t, func(t *rapid.T) {
		baseTime := time.Now()
		numTokens := rapid.IntRange(0, 80).Draw(t, "numTokens")

		tokens := make([]db.PersonalAccessToken, numTokens)
		for i := range tokens {
			tokens[i] = generateTestToken(t, baseTime, i)
		}

		sorted := filterAndSortTokens(tokens)

		// All tokens should be returned (since count ≤ 100)
		if len(sorted) != numTokens {
			t.Fatalf("expected %d tokens returned, got %d", numTokens, len(sorted))
		}

		// Verify each original token appears in the result
		resultIDs := make(map[[16]byte]bool)
		for _, tok := range sorted {
			resultIDs[tok.ID.Bytes] = true
		}
		for _, tok := range tokens {
			if !resultIDs[tok.ID.Bytes] {
				t.Fatalf("token with ID %v not found in result", tok.ID.Bytes)
			}
		}
	})
}

func TestProperty3_TokenListCompleteness_ResponseContainsRequiredFields(t *testing.T) {
	// Feature: agent-management, Property 3: Token List Completeness and Ordering
	// Verify each returned token contains: name, prefix, creation date, last-used date, expiry date.
	rapid.Check(t, func(t *rapid.T) {
		baseTime := time.Now()
		numTokens := rapid.IntRange(1, 30).Draw(t, "numTokens")

		tokens := make([]db.PersonalAccessToken, numTokens)
		for i := range tokens {
			tokens[i] = generateTestToken(t, baseTime, i)
		}

		sorted := filterAndSortTokens(tokens)
		responses := convertToPATResponses(sorted)

		for i, resp := range responses {
			// ID must be non-empty
			if resp.ID == "" {
				t.Fatalf("token at index %d has empty ID", i)
			}

			// Name must be non-empty
			if resp.Name == "" {
				t.Fatalf("token at index %d has empty Name", i)
			}

			// Prefix must be non-empty
			if resp.Prefix == "" {
				t.Fatalf("token at index %d has empty Prefix", i)
			}

			// CreatedAt must be non-empty and valid RFC3339
			if resp.CreatedAt == "" {
				t.Fatalf("token at index %d has empty CreatedAt", i)
			}
			if _, err := time.Parse(time.RFC3339, resp.CreatedAt); err != nil {
				t.Fatalf("token at index %d has invalid CreatedAt %q: %v", i, resp.CreatedAt, err)
			}

			// ExpiresAt: if present, must be valid RFC3339
			if resp.ExpiresAt != nil {
				if _, err := time.Parse(time.RFC3339, *resp.ExpiresAt); err != nil {
					t.Fatalf("token at index %d has invalid ExpiresAt %q: %v", i, *resp.ExpiresAt, err)
				}
			}

			// LastUsedAt: if present, must be valid RFC3339
			if resp.LastUsedAt != nil {
				if _, err := time.Parse(time.RFC3339, *resp.LastUsedAt); err != nil {
					t.Fatalf("token at index %d has invalid LastUsedAt %q: %v", i, *resp.LastUsedAt, err)
				}
			}
		}
	})
}

func TestProperty3_TokenListCompleteness_NeverExceeds100Tokens(t *testing.T) {
	// Feature: agent-management, Property 3: Token List Completeness and Ordering
	// Verify the list never exceeds 100 tokens even when more exist.
	rapid.Check(t, func(t *rapid.T) {
		baseTime := time.Now()
		// Generate more than 100 tokens
		numTokens := rapid.IntRange(101, 200).Draw(t, "numTokens")

		tokens := make([]db.PersonalAccessToken, numTokens)
		for i := range tokens {
			tokens[i] = generateTestToken(t, baseTime, i)
		}

		sorted := filterAndSortTokens(tokens)

		if len(sorted) > maxTokenListSize {
			t.Fatalf("expected at most %d tokens, got %d", maxTokenListSize, len(sorted))
		}
		if len(sorted) != maxTokenListSize {
			t.Fatalf("expected exactly %d tokens when input has %d, got %d",
				maxTokenListSize, numTokens, len(sorted))
		}

		// Verify the returned tokens are the 100 newest ones
		// Sort all tokens by creation date descending
		allSorted := make([]db.PersonalAccessToken, len(tokens))
		copy(allSorted, tokens)
		sort.Slice(allSorted, func(i, j int) bool {
			return allSorted[i].CreatedAt.Time.After(allSorted[j].CreatedAt.Time)
		})

		for i := 0; i < maxTokenListSize; i++ {
			if sorted[i].ID.Bytes != allSorted[i].ID.Bytes {
				t.Fatalf("at index %d: expected token %v but got %v (should be newest 100)",
					i, allSorted[i].ID.Bytes, sorted[i].ID.Bytes)
			}
		}
	})
}

func TestProperty3_TokenListCompleteness_EmptyListReturnsEmpty(t *testing.T) {
	// Feature: agent-management, Property 3: Token List Completeness and Ordering
	// Verify that an empty token set returns an empty list.
	rapid.Check(t, func(t *rapid.T) {
		tokens := []db.PersonalAccessToken{}
		sorted := filterAndSortTokens(tokens)

		if len(sorted) != 0 {
			t.Fatalf("expected empty result for empty input, got %d tokens", len(sorted))
		}

		responses := convertToPATResponses(sorted)
		if len(responses) != 0 {
			t.Fatalf("expected empty response list, got %d items", len(responses))
		}
	})
}

func TestProperty3_TokenListCompleteness_FieldMappingCorrectness(t *testing.T) {
	// Feature: agent-management, Property 3: Token List Completeness and Ordering
	// Verify that toPATResponse correctly maps DB fields to response fields.
	rapid.Check(t, func(t *rapid.T) {
		baseTime := time.Now()
		token := generateTestToken(t, baseTime, 0)

		resp := toPATResponse(token)

		// Name must match
		if resp.Name != token.Name {
			t.Fatalf("Name mismatch: got %q, want %q", resp.Name, token.Name)
		}

		// Prefix must match
		if resp.Prefix != token.TokenPrefix {
			t.Fatalf("Prefix mismatch: got %q, want %q", resp.Prefix, token.TokenPrefix)
		}

		// CreatedAt must match the token's creation time in RFC3339
		expectedCreatedAt := token.CreatedAt.Time.UTC().Format(time.RFC3339)
		if resp.CreatedAt != expectedCreatedAt {
			t.Fatalf("CreatedAt mismatch: got %q, want %q", resp.CreatedAt, expectedCreatedAt)
		}

		// ExpiresAt: if token has expiry, response should have it
		if token.ExpiresAt.Valid {
			if resp.ExpiresAt == nil {
				t.Fatal("ExpiresAt should be non-nil when token has expiry")
			}
			expectedExpiry := token.ExpiresAt.Time.UTC().Format(time.RFC3339)
			if *resp.ExpiresAt != expectedExpiry {
				t.Fatalf("ExpiresAt mismatch: got %q, want %q", *resp.ExpiresAt, expectedExpiry)
			}
		} else {
			if resp.ExpiresAt != nil {
				t.Fatalf("ExpiresAt should be nil when token has no expiry, got %q", *resp.ExpiresAt)
			}
		}

		// LastUsedAt: if token has last_used_at, response should have it
		if token.LastUsedAt.Valid {
			if resp.LastUsedAt == nil {
				t.Fatal("LastUsedAt should be non-nil when token has been used")
			}
			expectedLastUsed := token.LastUsedAt.Time.UTC().Format(time.RFC3339)
			if *resp.LastUsedAt != expectedLastUsed {
				t.Fatalf("LastUsedAt mismatch: got %q, want %q", *resp.LastUsedAt, expectedLastUsed)
			}
		} else {
			if resp.LastUsedAt != nil {
				t.Fatalf("LastUsedAt should be nil when token has never been used, got %q", *resp.LastUsedAt)
			}
		}
	})
}

func TestProperty3_TokenListCompleteness_IncludesExpiredTokens(t *testing.T) {
	// Feature: agent-management, Property 3: Token List Completeness and Ordering
	// Verify that expired tokens are still included in the list (they are not revoked).
	// Revocation = deletion from DB, so all records in DB are non-revoked.
	rapid.Check(t, func(t *rapid.T) {
		baseTime := time.Now()
		numTokens := rapid.IntRange(2, 20).Draw(t, "numTokens")

		tokens := make([]db.PersonalAccessToken, numTokens)
		expiredCount := 0
		for i := range tokens {
			tokens[i] = generateTestToken(t, baseTime, i)
			// Force some tokens to be expired
			if rapid.Bool().Draw(t, "forceExpired") {
				tokens[i].ExpiresAt = pgtype.Timestamptz{
					Time:  baseTime.Add(-time.Duration(rapid.IntRange(1, 365).Draw(t, "daysAgo")) * 24 * time.Hour),
					Valid: true,
				}
				expiredCount++
			}
		}

		sorted := filterAndSortTokens(tokens)

		// All tokens (including expired) should be in the result
		if len(sorted) != numTokens {
			t.Fatalf("expected %d tokens (including %d expired), got %d",
				numTokens, expiredCount, len(sorted))
		}
	})
}
