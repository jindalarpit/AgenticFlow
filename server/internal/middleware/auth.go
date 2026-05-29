package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/agenticflow/agenticflow/server/internal/auth"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
	"github.com/agenticflow/agenticflow/shared/httputil"
	"github.com/agenticflow/agenticflow/shared/pgutil"
)

// Context keys for user identity.
type contextKey int

const (
	ctxKeyUserID  contextKey = iota
	ctxKeyIsAdmin contextKey = iota
)

// ContextUserID extracts the authenticated user ID from the request context.
func ContextUserID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyUserID).(string)
	return id
}

// ContextIsAdmin returns whether the authenticated user has admin privileges.
// In AgenticFlow's simple model, this currently always returns false.
// It provides an extension point for future workspace admin role support.
func ContextIsAdmin(ctx context.Context) bool {
	admin, _ := ctx.Value(ctxKeyIsAdmin).(bool)
	return admin
}

// WithUserID returns a new context with the given user ID set.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, userID)
}

// PATCache is an in-memory cache mapping token hashes to user IDs with
// expiration. It uses sync.Map for concurrent access without a global lock.
type PATCache struct {
	store sync.Map // map[string]patCacheEntry
}

type patCacheEntry struct {
	userID    string
	expiresAt time.Time
}

// NewPATCache creates a new in-memory PAT cache.
func NewPATCache() *PATCache {
	return &PATCache{}
}

// Get returns the cached user ID for a token hash. Returns empty string and
// false on cache miss or expired entry.
func (c *PATCache) Get(hash string) (userID string, ok bool) {
	if c == nil {
		return "", false
	}
	v, loaded := c.store.Load(hash)
	if !loaded {
		return "", false
	}
	entry := v.(patCacheEntry)
	if time.Now().After(entry.expiresAt) {
		c.store.Delete(hash)
		return "", false
	}
	return entry.userID, true
}

// Set stores a user ID for a token hash with the given TTL.
func (c *PATCache) Set(hash, userID string, ttl time.Duration) {
	if c == nil || ttl <= 0 {
		return
	}
	c.store.Store(hash, patCacheEntry{
		userID:    userID,
		expiresAt: time.Now().Add(ttl),
	})
}

// Invalidate removes the entry for a token hash. Called on token revocation.
func (c *PATCache) Invalidate(hash string) {
	if c == nil {
		return
	}
	c.store.Delete(hash)
}

// CacheTTL is the default TTL for cached PAT lookups (5 minutes).
// This bounds the maximum time a revoked token could remain valid in cache.
const CacheTTL = 5 * time.Minute

// TTLForExpiry returns the cache TTL for a token given its expiry time.
// Returns the minimum of CacheTTL and the remaining token lifetime.
func TTLForExpiry(now time.Time, expiresAt time.Time) time.Duration {
	if expiresAt.IsZero() {
		return CacheTTL
	}
	remaining := expiresAt.Sub(now)
	if remaining <= 0 {
		return 0
	}
	if remaining < CacheTTL {
		return remaining
	}
	return CacheTTL
}

// Auth middleware validates Personal Access Tokens from the Authorization
// header. It extracts the Bearer token, validates it against the cache or
// database, and sets the user ID in the request context.
//
// The pool parameter is used for bounded asynchronous last_used_at updates.
// If pool is nil, the update is skipped.
//
// Returns HTTP 401 for missing, malformed, or expired tokens.
func Auth(queries *db.Queries, cache *PATCache, pool *TokenUpdatePool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := extractBearerToken(r)
			if tokenString == "" {
				slog.Debug("auth: no token found", "path", r.URL.Path)
				httputil.WriteErrorJSON(w, http.StatusUnauthorized, "missing authorization")
				return
			}

			// Only accept tokens with the af_ prefix.
			if !strings.HasPrefix(tokenString, auth.PATPrefix) {
				slog.Debug("auth: invalid token prefix", "path", r.URL.Path)
				httputil.WriteErrorJSON(w, http.StatusUnauthorized, "invalid token")
				return
			}

			hash := auth.HashToken(tokenString)

			// Check cache first.
			if userID, ok := cache.Get(hash); ok {
				ctx := WithUserID(r.Context(), userID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Cache miss — look up in database.
			pat, err := queries.GetTokenByHash(r.Context(), hash)
			if err != nil {
				slog.Warn("auth: invalid PAT", "path", r.URL.Path, "error", err)
				httputil.WriteErrorJSON(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			userID := pgutil.UUIDToString(pat.UserID)

			// Cache the result with appropriate TTL.
			var expiresAt time.Time
			if pat.ExpiresAt.Valid {
				expiresAt = pat.ExpiresAt.Time
			}
			cache.Set(hash, userID, TTLForExpiry(time.Now(), expiresAt))

			// Update last_used_at asynchronously via bounded worker pool.
			if pool != nil {
				patID := pat.ID
				pool.Submit(func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					_ = queries.UpdateTokenLastUsed(ctx, patID)
				})
			}

			ctx := WithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractBearerToken extracts the token from the Authorization: Bearer <token> header.
func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return ""
	}
	return strings.TrimPrefix(authHeader, prefix)
}
