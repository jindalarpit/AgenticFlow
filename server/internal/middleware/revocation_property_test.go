package middleware_test

import (
	"testing"
	"time"

	"github.com/agenticflow/agenticflow/server/internal/middleware"
	"pgregory.net/rapid"
)

// Feature: agent-management, Property 4: Token Revocation Immediacy
//
// For any valid PAT that has been revoked, subsequent authentication attempts
// using that token SHALL be rejected. Revocation of non-existent token IDs
// SHALL return success (idempotent).
//
// These tests verify the PATCache behavior which is the mechanism for immediate
// revocation: once Invalidate() is called, Get() must return a cache miss.
//
// **Validates: Requirements 3.1, 3.3, 3.4**

func TestProperty4_TokenRevocationImmediacy_InvalidatedTokenRejected(t *testing.T) {
	// Property 4: Token Revocation Immediacy
	// For any token that has been cached and then invalidated via PATCache.Invalidate(),
	// subsequent Get() calls must return a cache miss.
	rapid.Check(t, func(t *rapid.T) {
		cache := middleware.NewPATCache()

		// Generate a random token hash and user ID
		hash := rapid.StringMatching(`[a-f0-9]{64}`).Draw(t, "tokenHash")
		userID := rapid.StringMatching(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`).Draw(t, "userID")
		ttlMinutes := rapid.IntRange(1, 60).Draw(t, "ttlMinutes")
		ttl := time.Duration(ttlMinutes) * time.Minute

		// Set the token in cache
		cache.Set(hash, userID, ttl)

		// Verify it's accessible before invalidation
		gotUserID, ok := cache.Get(hash)
		if !ok {
			t.Fatalf("expected cache hit before invalidation, got miss for hash=%q", hash)
		}
		if gotUserID != userID {
			t.Fatalf("expected userID=%q, got %q", userID, gotUserID)
		}

		// Invalidate (revoke) the token
		cache.Invalidate(hash)

		// Subsequent Get() must return cache miss — revocation is immediate
		_, ok = cache.Get(hash)
		if ok {
			t.Fatalf("expected cache miss after invalidation, but got hit for hash=%q", hash)
		}
	})
}

func TestProperty4_TokenRevocationImmediacy_NonExistentHashReturnsMiss(t *testing.T) {
	// Property 4: Token Revocation Immediacy
	// For any non-existent hash, Get() returns cache miss (no panic, no error).
	rapid.Check(t, func(t *rapid.T) {
		cache := middleware.NewPATCache()

		// Generate a random hash that was never stored
		hash := rapid.StringMatching(`[a-f0-9]{64}`).Draw(t, "nonExistentHash")

		// Get on a non-existent key must return ("", false) without panic
		userID, ok := cache.Get(hash)
		if ok {
			t.Fatalf("expected cache miss for non-existent hash=%q, got hit with userID=%q", hash, userID)
		}
		if userID != "" {
			t.Fatalf("expected empty userID for cache miss, got %q", userID)
		}
	})
}

func TestProperty4_TokenRevocationImmediacy_TTLExpiry(t *testing.T) {
	// Property 4: Token Revocation Immediacy
	// For any token that is set with a TTL, it is accessible within the TTL
	// and inaccessible after the TTL expires.
	rapid.Check(t, func(t *rapid.T) {
		cache := middleware.NewPATCache()

		hash := rapid.StringMatching(`[a-f0-9]{64}`).Draw(t, "tokenHash")
		userID := rapid.StringMatching(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`).Draw(t, "userID")

		// Set with a very short TTL that has already expired (negative duration simulates past expiry)
		cache.Set(hash, userID, -1*time.Millisecond)

		// Token should be inaccessible (expired)
		_, ok := cache.Get(hash)
		if ok {
			t.Fatalf("expected cache miss for expired token (negative TTL), got hit for hash=%q", hash)
		}

		// Set with a valid positive TTL — should be accessible
		cache.Set(hash, userID, 5*time.Minute)
		gotUserID, ok := cache.Get(hash)
		if !ok {
			t.Fatalf("expected cache hit for token with 5min TTL, got miss for hash=%q", hash)
		}
		if gotUserID != userID {
			t.Fatalf("expected userID=%q, got %q", userID, gotUserID)
		}
	})
}

func TestProperty4_TokenRevocationImmediacy_InvalidateOnNilCache(t *testing.T) {
	// Property 4: Token Revocation Immediacy
	// Invalidate() on a nil cache does not panic.
	rapid.Check(t, func(t *rapid.T) {
		var cache *middleware.PATCache

		// Generate a random hash
		hash := rapid.StringMatching(`[a-f0-9]{64}`).Draw(t, "hash")

		// This must not panic
		cache.Invalidate(hash)

		// Get on nil cache must also not panic and return miss
		_, ok := cache.Get(hash)
		if ok {
			t.Fatalf("expected cache miss on nil cache, got hit")
		}
	})
}

func TestProperty4_TokenRevocationImmediacy_InvalidateNonExistentKey(t *testing.T) {
	// Property 4: Token Revocation Immediacy
	// Invalidate() on a non-existent key does not panic and is a no-op.
	rapid.Check(t, func(t *rapid.T) {
		cache := middleware.NewPATCache()

		// Populate cache with some entries
		numEntries := rapid.IntRange(0, 10).Draw(t, "numEntries")
		for i := 0; i < numEntries; i++ {
			h := rapid.StringMatching(`[a-f0-9]{64}`).Draw(t, "existingHash")
			u := rapid.StringMatching(`user-[a-z]{5}`).Draw(t, "existingUser")
			cache.Set(h, u, 5*time.Minute)
		}

		// Invalidate a key that was never stored — must not panic
		nonExistentHash := rapid.StringMatching(`ffffffff[a-f0-9]{56}`).Draw(t, "nonExistentHash")
		cache.Invalidate(nonExistentHash)

		// Verify the cache still works after invalidating a non-existent key
		cache.Set("verify-hash", "verify-user", 5*time.Minute)
		gotUser, ok := cache.Get("verify-hash")
		if !ok || gotUser != "verify-user" {
			t.Fatalf("cache broken after invalidating non-existent key: got %q, ok=%v", gotUser, ok)
		}
	})
}
