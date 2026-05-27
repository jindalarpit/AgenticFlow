package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"github.com/agenticflow/agenticflow/server/internal/auth"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// loginRequest is the JSON body for POST /auth/login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// registerRequest is the JSON body for POST /auth/register.
type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// authResponse is the JSON response for login and register.
type authResponse struct {
	Token     string       `json:"token"`
	ExpiresAt string       `json:"expires_at"`
	User      userResponse `json:"user"`
}

// userResponse is the public representation of a user.
type userResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatar_url"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// AuthHandler holds dependencies for authentication HTTP handlers.
type AuthHandler struct {
	Queries *db.Queries
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(queries *db.Queries) *AuthHandler {
	return &AuthHandler{Queries: queries}
}

// Login handles POST /auth/login.
// It validates email/password credentials and returns a PAT.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email is required"})
		return
	}
	if req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password is required"})
		return
	}

	// Look up user by email.
	user, err := h.Queries.GetUserByEmail(r.Context(), email)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		return
	}

	// Verify password hash.
	if !user.PasswordHash.Valid || user.PasswordHash.String == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash.String), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		return
	}

	// Generate PAT.
	token, hash, err := auth.GeneratePAT()
	if err != nil {
		slog.Error("failed to generate PAT", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	// Store token with 90-day expiry.
	expiresAt := time.Now().Add(auth.PATExpiry)
	_, err = h.Queries.CreateToken(r.Context(), db.CreateTokenParams{
		UserID:    user.ID,
		Name:      "login",
		TokenHash: hash,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		slog.Error("failed to store token", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create token"})
		return
	}

	slog.Info("user logged in", "user_id", uuidToString(user.ID), "email", user.Email)
	writeJSON(w, http.StatusOK, authResponse{
		Token:     token,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
		User:      toUserResponse(user),
	})
}

// maxNameLength is the maximum allowed length for a user name after trimming.
const maxNameLength = 128

// Register handles POST /auth/register.
// It creates a new user with a hashed password and returns a PAT.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := req.Password

	// Validate name: 1-128 characters after trimming.
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if len(name) > maxNameLength {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("name must not exceed %d characters", maxNameLength),
		})
		return
	}

	// Validate email format: one '@', domain with '.', max 254 chars.
	if !auth.ValidateEmail(email) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid email format"})
		return
	}

	// Validate password: 8-128 characters.
	if !auth.ValidatePassword(password) {
		if len(password) < auth.MinPasswordLength {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("password must be at least %d characters", auth.MinPasswordLength),
			})
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("password must not exceed %d characters", auth.MaxPasswordLength),
			})
		}
		return
	}

	// Hash password with bcrypt.
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to process password"})
		return
	}

	// Create user.
	user, err := h.Queries.CreateUser(r.Context(), db.CreateUserParams{
		Name:         name,
		Email:        email,
		PasswordHash: pgtype.Text{String: string(hashedPassword), Valid: true},
		AvatarUrl:    pgtype.Text{},
	})
	if err != nil {
		// Check for unique constraint violation (duplicate email).
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "23505") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "email already registered"})
			return
		}
		slog.Error("failed to create user", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create user"})
		return
	}

	// Generate PAT.
	token, hash, err := auth.GeneratePAT()
	if err != nil {
		slog.Error("failed to generate PAT", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	// Store token with 90-day expiry.
	expiresAt := time.Now().Add(auth.PATExpiry)
	_, err = h.Queries.CreateToken(r.Context(), db.CreateTokenParams{
		UserID:    user.ID,
		Name:      "registration",
		TokenHash: hash,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		slog.Error("failed to store token", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create token"})
		return
	}

	slog.Info("user registered", "user_id", uuidToString(user.ID), "email", user.Email)
	writeJSON(w, http.StatusCreated, authResponse{
		Token:     token,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
		User:      toUserResponse(user),
	})
}

// OAuthCallback handles GET /auth/callback/{provider}.
// This is a placeholder that returns 501 Not Implemented.
func (h *AuthHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error":    "OAuth callback not implemented",
		"provider": provider,
		"message":  fmt.Sprintf("OAuth callback for provider %q is not yet implemented", provider),
	})
}

// toUserResponse converts a db.User to the public userResponse.
func toUserResponse(u db.User) userResponse {
	resp := userResponse{
		ID:        uuidToString(u.ID),
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt.Time.UTC().Format(time.RFC3339),
		UpdatedAt: u.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}
	if u.AvatarUrl.Valid {
		resp.AvatarURL = &u.AvatarUrl.String
	}
	return resp
}
