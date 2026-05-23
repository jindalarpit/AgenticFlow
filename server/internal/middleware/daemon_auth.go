package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/agenticflow/agenticflow/internal/auth"
	db "github.com/agenticflow/agenticflow/pkg/db/generated"
)

// Daemon context keys.
type daemonContextKey int

const (
	ctxKeyDaemonUserID daemonContextKey = iota
	ctxKeyDaemonDaemonID
)

// DaemonUserIDFromContext returns the user ID set by DaemonAuth middleware.
func DaemonUserIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyDaemonUserID).(string)
	return id
}

// DaemonIDFromContext returns the daemon ID set by DaemonAuth middleware.
func DaemonIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyDaemonDaemonID).(string)
	return id
}

// WithDaemonContext returns a new context with daemon user ID and daemon ID set.
// This is used by tests to simulate daemon token authentication.
func WithDaemonContext(ctx context.Context, userID, daemonID string) context.Context {
	ctx = context.WithValue(ctx, ctxKeyDaemonUserID, userID)
	ctx = context.WithValue(ctx, ctxKeyDaemonDaemonID, daemonID)
	return ctx
}

// DaemonAuth validates daemon authentication. Daemons authenticate using the
// same PAT tokens (af_ prefix) as users. The middleware validates the token,
// extracts the user ID, and sets it in the context for downstream handlers.
//
// The daemon ID is extracted from the X-Daemon-ID header which the daemon
// includes in every request.
//
// Returns HTTP 401 for missing, malformed, or expired tokens.
func DaemonAuth(queries *db.Queries, cache *PATCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := extractBearerToken(r)
			if tokenString == "" {
				slog.Debug("daemon_auth: missing authorization header", "path", r.URL.Path)
				writeJSON(w, http.StatusUnauthorized, `{"error":"missing authorization"}`)
				return
			}

			// Only accept tokens with the af_ prefix.
			if !strings.HasPrefix(tokenString, auth.PATPrefix) {
				slog.Debug("daemon_auth: invalid token prefix", "path", r.URL.Path)
				writeJSON(w, http.StatusUnauthorized, `{"error":"invalid token"}`)
				return
			}

			hash := auth.HashToken(tokenString)

			// Check cache first.
			if userID, ok := cache.Get(hash); ok {
				daemonID := r.Header.Get("X-Daemon-ID")
				ctx := WithDaemonContext(r.Context(), userID, daemonID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Cache miss — look up in database.
			pat, err := queries.GetTokenByHash(r.Context(), hash)
			if err != nil {
				slog.Warn("daemon_auth: invalid PAT", "path", r.URL.Path, "error", err)
				writeJSON(w, http.StatusUnauthorized, `{"error":"invalid or expired token"}`)
				return
			}

			userID := pgUUIDToString(pat.UserID)

			// Cache the result with appropriate TTL.
			var expiresAt time.Time
			if pat.ExpiresAt.Valid {
				expiresAt = pat.ExpiresAt.Time
			}
			cache.Set(hash, userID, TTLForExpiry(time.Now(), expiresAt))

			// Update last_used_at asynchronously.
			go func() {
				_ = queries.UpdateTokenLastUsed(context.Background(), pat.ID)
			}()

			daemonID := r.Header.Get("X-Daemon-ID")
			ctx := WithDaemonContext(r.Context(), userID, daemonID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
