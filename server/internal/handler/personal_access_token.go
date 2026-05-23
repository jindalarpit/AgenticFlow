package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agenticflow/agenticflow/internal/auth"
	"github.com/agenticflow/agenticflow/internal/middleware"
	db "github.com/agenticflow/agenticflow/pkg/db/generated"
)

const (
	// patPrefix is the token prefix for AgenticFlow PATs.
	patPrefix = "af_"
	// patTokenLen is the number of hex characters after the prefix.
	patTokenLen = 64
	// patPrefixLen is the number of characters stored for display.
	patPrefixLen = 12
	// patMaxName is the maximum allowed PAT name length.
	patMaxName = 64
)

// validExpiryDays defines the allowed expiry options for PAT creation.
var validExpiryDays = map[int]bool{
	30:  true,
	90:  true,
	365: true,
}

// CreatePATRequest is the JSON body for POST /api/tokens.
type CreatePATRequest struct {
	Name          string `json:"name"`
	ExpiresInDays *int   `json:"expires_in_days"` // 30, 90, 365, or nil (no expiry)
}

// PATResponse is the public representation of a PAT (without the raw token).
type PATResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"token_prefix"`
	ExpiresAt  *string `json:"expires_at"`
	LastUsedAt *string `json:"last_used_at"`
	CreatedAt  string  `json:"created_at"`
}

// CreatePATResponse includes the full token value (returned exactly once).
type CreatePATResponse struct {
	PATResponse
	Token string `json:"token"`
}

// PATHandler holds dependencies for PAT HTTP handlers.
type PATHandler struct {
	Queries  *db.Queries
	PATCache *middleware.PATCache
}

// NewPATHandler creates a new PATHandler.
func NewPATHandler(queries *db.Queries, cache *middleware.PATCache) *PATHandler {
	return &PATHandler{Queries: queries, PATCache: cache}
}

// CreatePersonalAccessToken handles POST /api/tokens.
// It generates a cryptographically random token with the af_ prefix,
// stores its SHA-256 hash and prefix, and returns the full token exactly once.
func (h *PATHandler) CreatePersonalAccessToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreatePATRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate name: non-empty after trimming, max 64 chars.
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeErrorJSON(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(name) > patMaxName {
		writeErrorJSON(w, http.StatusBadRequest, "name exceeds maximum length of 64 characters")
		return
	}

	// Validate expiry option: must be 30, 90, 365, or nil.
	if req.ExpiresInDays != nil {
		if !validExpiryDays[*req.ExpiresInDays] {
			writeErrorJSON(w, http.StatusBadRequest, "expires_in_days must be 30, 90, or 365")
			return
		}
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	// Generate token: af_ + 64 random hex chars.
	rawToken, _, err := auth.GeneratePAT()
	if err != nil {
		slog.Error("create PAT: generate failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// Compute SHA-256 hash of the full token string.
	tokenHash := auth.HashToken(rawToken)

	// Store prefix: first 12 characters of the token.
	prefix := rawToken
	if len(prefix) > patPrefixLen {
		prefix = prefix[:patPrefixLen]
	}

	// Compute expiry timestamp.
	var expiresAt pgtype.Timestamptz
	if req.ExpiresInDays != nil {
		expiresAt = pgtype.Timestamptz{
			Time:  time.Now().Add(time.Duration(*req.ExpiresInDays) * 24 * time.Hour),
			Valid: true,
		}
	}

	// Store in database.
	pat, err := h.Queries.CreatePersonalAccessToken(r.Context(), db.CreatePersonalAccessTokenParams{
		UserID:      userUUID,
		Name:        name,
		TokenHash:   tokenHash,
		TokenPrefix: prefix,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		slog.Error("create PAT: insert failed", "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	// Return full token value exactly once.
	writeJSON(w, http.StatusCreated, CreatePATResponse{
		PATResponse: toPATResponse(pat),
		Token:       rawToken,
	})
}

// ListPersonalAccessTokens handles GET /api/tokens.
// It returns all non-revoked PATs for the authenticated user.
func (h *PATHandler) ListPersonalAccessTokens(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	tokens, err := h.Queries.ListTokensByUser(r.Context(), userUUID)
	if err != nil {
		slog.Error("list PATs: query failed", "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to list tokens")
		return
	}

	result := make([]PATResponse, 0, len(tokens))
	for _, t := range tokens {
		result = append(result, toPATResponse(t))
	}
	writeJSON(w, http.StatusOK, result)
}

// RevokePersonalAccessToken handles DELETE /api/tokens/{id}.
// It deletes the token record (idempotent — returns success even if not found).
func (h *PATHandler) RevokePersonalAccessToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	tokenID := chi.URLParam(r, "id")
	if tokenID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "id is required")
		return
	}

	tokenUUID, err := parseUUID(tokenID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid token id")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	// Delete is idempotent — no error if token doesn't exist or doesn't belong to user.
	// Use DeleteTokenReturningHash to get the hash for cache invalidation.
	tokenHash, err := h.Queries.DeleteTokenReturningHash(r.Context(), db.DeleteTokenReturningHashParams{
		ID:     tokenUUID,
		UserID: userUUID,
	})
	if err != nil {
		// pgx returns ErrNoRows if the token doesn't exist — that's fine (idempotent).
		// Only log and fail on actual DB errors.
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("revoke PAT: delete failed", "token_id", tokenID, "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "failed to revoke token")
			return
		}
	}

	// Invalidate the cache entry so the token is rejected immediately.
	if tokenHash != "" && h.PATCache != nil {
		h.PATCache.Invalidate(tokenHash)
	}

	w.WriteHeader(http.StatusNoContent)
}

// toPATResponse converts a db.PersonalAccessToken to the public PATResponse.
func toPATResponse(pat db.PersonalAccessToken) PATResponse {
	resp := PATResponse{
		ID:        uuidToString(pat.ID),
		Name:      pat.Name,
		Prefix:    pat.TokenPrefix,
		CreatedAt: pat.CreatedAt.Time.UTC().Format(time.RFC3339),
	}
	if pat.ExpiresAt.Valid {
		s := pat.ExpiresAt.Time.UTC().Format(time.RFC3339)
		resp.ExpiresAt = &s
	}
	if pat.LastUsedAt.Valid {
		s := pat.LastUsedAt.Time.UTC().Format(time.RFC3339)
		resp.LastUsedAt = &s
	}
	return resp
}
