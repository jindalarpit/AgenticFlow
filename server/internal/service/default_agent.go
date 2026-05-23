package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/agenticflow/agenticflow/pkg/db/generated"
)

const (
	// DefaultAgentName is the name of the default agent created on first setup.
	DefaultAgentName = "Nexus"
	// DefaultAgentDescription is the description for the default agent.
	DefaultAgentDescription = "Your local AI coding agent"
)

// EnsureDefaultAgent creates the default "Nexus" agent for a user if they have
// no agents yet. This is intended to be called during daemon registration and
// is non-blocking: any errors are logged and do not affect the caller.
//
// Parameters:
//   - ctx: request context
//   - queries: database queries interface
//   - userID: the user's UUID
//   - runtimeIDs: map of provider → runtime UUID string from the registration payload
//
// The function:
//  1. Checks if the user already has any agents (skip if yes)
//  2. Checks if "Nexus" already exists for the user (skip if yes, race guard)
//  3. Picks the first valid runtime ID from the registration payload
//  4. Creates the "Nexus" agent with default settings
//  5. Logs and returns on any DB error (non-blocking)
func EnsureDefaultAgent(ctx context.Context, queries *db.Queries, userID pgtype.UUID, runtimeIDs map[string]string) {
	userIDStr := uuidToString(userID)

	// Check if the user already has any agents.
	count, err := queries.CountAgentsByUser(ctx, userID)
	if err != nil {
		slog.Warn("default agent: failed to count user agents", "user_id", userIDStr, "error", err)
		return
	}
	if count > 0 {
		// User already has agents — skip.
		return
	}

	// Double-check: skip if "Nexus" already exists (race condition guard).
	_, err = queries.GetAgentByName(ctx, db.GetAgentByNameParams{
		UserID: userID,
		Name:   DefaultAgentName,
	})
	if err == nil {
		// "Nexus" already exists — skip.
		return
	}

	// Pick the first valid runtime ID from the registration payload.
	var firstRuntimeID pgtype.UUID
	for _, rid := range runtimeIDs {
		parsed, parseErr := parseUUID(rid)
		if parseErr == nil {
			firstRuntimeID = parsed
			break
		}
	}
	if !firstRuntimeID.Valid {
		slog.Warn("default agent: no valid runtime ID available for Nexus creation", "user_id", userIDStr)
		return
	}

	// Create the default "Nexus" agent.
	_, err = queries.CreateAgent(ctx, db.CreateAgentParams{
		UserID:             userID,
		Name:               DefaultAgentName,
		Description:        DefaultAgentDescription,
		Instructions:       "",
		RuntimeID:          firstRuntimeID,
		Model:              pgtype.Text{Valid: false},
		CustomEnv:          []byte("{}"),
		CustomArgs:         []byte("[]"),
		MaxConcurrentTasks: 1,
		Visibility:         "private",
		AvatarUrl:          pgtype.Text{Valid: false},
	})
	if err != nil {
		slog.Warn("default agent: failed to create Nexus agent", "user_id", userIDStr, "error", err)
		return
	}

	slog.Info("default agent created", "name", DefaultAgentName, "user_id", userIDStr)
}

// parseUUID converts a string UUID to pgtype.UUID.
func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if s == "" {
		return u, fmt.Errorf("empty UUID string")
	}
	if err := u.Scan(s); err != nil {
		return u, fmt.Errorf("invalid UUID %q: %w", s, err)
	}
	return u, nil
}
