// Package handler provides HTTP route handlers for the AgenticFlow API.
package handler

import (
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agenticflow/agenticflow/server/internal/service"
	"github.com/agenticflow/agenticflow/shared/httputil"
	"github.com/agenticflow/agenticflow/shared/pgutil"
)

// parseUUID parses a UUID string into a pgtype.UUID.
func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if s == "" {
		return u, fmt.Errorf("empty uuid")
	}
	if err := u.Scan(s); err != nil {
		return u, fmt.Errorf("invalid uuid %q: %w", s, err)
	}
	return u, nil
}

// uuidToString converts a pgtype.UUID to its string representation.
// This is a convenience wrapper around pgutil.UUIDToString for use within the handler package.
func uuidToString(u pgtype.UUID) string {
	return pgutil.UUIDToString(u)
}

// writeJSON writes a JSON response with the given status code.
// This is a convenience wrapper around httputil.WriteJSON for use within the handler package.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	httputil.WriteJSON(w, status, v)
}

// writeErrorJSON writes a JSON error response.
// This is a convenience wrapper around httputil.WriteErrorJSON for use within the handler package.
func writeErrorJSON(w http.ResponseWriter, status int, message string) {
	httputil.WriteErrorJSON(w, status, message)
}

// handleServiceError maps a *service.ServiceError to the appropriate HTTP
// response. It uses ErrorKind.HTTPStatus() to determine the status code and
// writes the error message as a JSON body.
func handleServiceError(w http.ResponseWriter, err *service.ServiceError) {
	writeErrorJSON(w, err.Kind.HTTPStatus(), err.Message)
}
