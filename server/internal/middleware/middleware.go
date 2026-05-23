// Package middleware provides HTTP middleware for authentication and request processing.
//
// This package contains:
//   - Auth: middleware for user PAT authentication
//   - DaemonAuth: middleware for daemon endpoint authentication
//   - PATCache: in-memory cache for validated tokens
//   - Context helpers: ContextUserID, WithUserID, DaemonUserIDFromContext, etc.
package middleware
