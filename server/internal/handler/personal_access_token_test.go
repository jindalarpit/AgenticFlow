package handler

import (
	"testing"
	"time"

	"github.com/agenticflow/agenticflow/internal/middleware"
)

func TestRevokePersonalAccessToken_CacheInvalidation(t *testing.T) {
	// Verify that the PATCache.Invalidate method removes the entry.
	cache := middleware.NewPATCache()

	// Simulate a cached token hash.
	tokenHash := "abc123def456"
	cache.Set(tokenHash, "user-id-1", 5*time.Minute)

	// Verify it's cached.
	if uid, ok := cache.Get(tokenHash); !ok || uid != "user-id-1" {
		t.Fatalf("expected cache hit for %q, got ok=%v uid=%q", tokenHash, ok, uid)
	}

	// Invalidate.
	cache.Invalidate(tokenHash)

	// Verify it's gone.
	if _, ok := cache.Get(tokenHash); ok {
		t.Fatalf("expected cache miss after invalidation for %q", tokenHash)
	}
}

func TestRevokePersonalAccessToken_CacheInvalidateNonExistent(t *testing.T) {
	// Invalidating a non-existent key should not panic.
	cache := middleware.NewPATCache()
	cache.Invalidate("does-not-exist")
}

func TestRevokePersonalAccessToken_CacheInvalidateNilCache(t *testing.T) {
	// Invalidating on a nil cache should not panic.
	var cache *middleware.PATCache
	cache.Invalidate("some-hash") // should be a no-op
}
