package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// Validation constants for deliverable type fields.
const (
	maxDeliverableTypeNameLength         = 64
	maxDeliverableTypeDescriptionLength  = 255
	maxDeliverableTypeOutputFormatLength = 10000
)

// ValidateDeliverableTypeFields validates the name, description, and
// output_format fields for a deliverable type creation or update request.
// Returns nil if all fields are within acceptable bounds, or a ServiceError
// describing the first validation failure encountered.
func ValidateDeliverableTypeFields(name, description, outputFormat string) *ServiceError {
	nameLen := utf8.RuneCountInString(name)
	if nameLen < 1 || nameLen > maxDeliverableTypeNameLength {
		return Validation("name must be between 1 and 64 characters")
	}

	if utf8.RuneCountInString(description) > maxDeliverableTypeDescriptionLength {
		return Validation("description must be at most 255 characters")
	}

	if utf8.RuneCountInString(outputFormat) > maxDeliverableTypeOutputFormatLength {
		return Validation("output_format must be at most 10000 characters")
	}

	return nil
}

// DeliverableTypeService encapsulates business logic for deliverable type
// CRUD operations including validation, ownership checks, and system type
// protection.
type DeliverableTypeService struct {
	q db.Querier
}

// NewDeliverableTypeService creates a new DeliverableTypeService.
func NewDeliverableTypeService(q db.Querier) *DeliverableTypeService {
	return &DeliverableTypeService{q: q}
}

// ---------------------------------------------------------------------------
// Request types
// ---------------------------------------------------------------------------

// CreateDeliverableTypeParams holds the validated parameters for creating a
// deliverable type.
type CreateDeliverableTypeParams struct {
	UserID       string
	Name         string
	Description  string
	OutputFormat string
}

// UpdateDeliverableTypeParams holds the parameters for updating a deliverable
// type.
type UpdateDeliverableTypeParams struct {
	UserID       string
	ID           string
	Name         string
	Description  string
	OutputFormat string
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

// Create validates all fields, checks name uniqueness, and creates the
// deliverable type. Returns the created deliverable type or a typed
// ServiceError.
func (s *DeliverableTypeService) Create(ctx context.Context, params CreateDeliverableTypeParams) (db.DeliverableType, *ServiceError) {
	// --- Validate fields ---
	if svcErr := ValidateDeliverableTypeFields(params.Name, params.Description, params.OutputFormat); svcErr != nil {
		return db.DeliverableType{}, svcErr
	}

	// --- Parse user UUID ---
	userUUID, err := parseUUID(params.UserID)
	if err != nil {
		return db.DeliverableType{}, Internal("invalid user id")
	}

	// --- Insert (DB unique index handles name uniqueness) ---
	dt, err := s.q.CreateDeliverableType(ctx, db.CreateDeliverableTypeParams{
		UserID:       userUUID,
		Name:         params.Name,
		Description:  params.Description,
		OutputFormat: params.OutputFormat,
	})
	if err != nil {
		// Handle unique constraint violation (duplicate name for user or system type).
		if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate") {
			return db.DeliverableType{}, Conflict("a deliverable type with this name already exists")
		}
		slog.Error("create deliverable type: insert failed", "user_id", params.UserID, "error", err)
		return db.DeliverableType{}, Internal("failed to create deliverable type")
	}

	slog.Info("deliverable type created", "id", uuidToString(dt.ID), "name", dt.Name, "user_id", params.UserID)
	return dt, nil
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

// List returns all system deliverable types plus user-created types owned by
// the authenticated user.
func (s *DeliverableTypeService) List(ctx context.Context, userID string) ([]db.DeliverableType, *ServiceError) {
	userUUID, err := parseUUID(userID)
	if err != nil {
		return nil, Internal("invalid user id")
	}

	types, err := s.q.ListDeliverableTypesByUser(ctx, userUUID)
	if err != nil {
		slog.Error("list deliverable types: query failed", "user_id", userID, "error", err)
		return nil, Internal("failed to list deliverable types")
	}

	return types, nil
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

// Get retrieves a single deliverable type by ID. The query returns the type if
// it is owned by the user or is a system type. Returns NotFound otherwise.
func (s *DeliverableTypeService) Get(ctx context.Context, id, userID string) (db.DeliverableType, *ServiceError) {
	dtUUID, err := parseUUID(id)
	if err != nil {
		return db.DeliverableType{}, Validation("invalid deliverable type id format")
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		return db.DeliverableType{}, Internal("invalid user id")
	}

	dt, err := s.q.GetDeliverableType(ctx, db.GetDeliverableTypeParams{
		ID:     dtUUID,
		UserID: userUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.DeliverableType{}, NotFound("deliverable type not found")
		}
		slog.Error("get deliverable type: query failed", "id", id, "user_id", userID, "error", err)
		return db.DeliverableType{}, Internal("failed to get deliverable type")
	}

	return dt, nil
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// Update validates fields and updates the deliverable type. System types
// cannot be modified (403). Returns the updated deliverable type or a typed
// ServiceError.
func (s *DeliverableTypeService) Update(ctx context.Context, params UpdateDeliverableTypeParams) (db.DeliverableType, *ServiceError) {
	dtUUID, err := parseUUID(params.ID)
	if err != nil {
		return db.DeliverableType{}, Validation("invalid deliverable type id format")
	}

	userUUID, err := parseUUID(params.UserID)
	if err != nil {
		return db.DeliverableType{}, Internal("invalid user id")
	}

	// --- Fetch existing to check system type ---
	existing, err := s.q.GetDeliverableType(ctx, db.GetDeliverableTypeParams{
		ID:     dtUUID,
		UserID: userUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.DeliverableType{}, NotFound("deliverable type not found")
		}
		slog.Error("update deliverable type: get failed", "id", params.ID, "error", err)
		return db.DeliverableType{}, Internal("failed to get deliverable type")
	}

	// --- Reject system types ---
	if existing.IsSystem {
		return db.DeliverableType{}, Forbidden("system deliverable types cannot be modified")
	}

	// --- Validate fields ---
	if svcErr := ValidateDeliverableTypeFields(params.Name, params.Description, params.OutputFormat); svcErr != nil {
		return db.DeliverableType{}, svcErr
	}

	// --- Update ---
	dt, err := s.q.UpdateDeliverableType(ctx, db.UpdateDeliverableTypeParams{
		ID:           dtUUID,
		UserID:       userUUID,
		Name:         params.Name,
		Description:  params.Description,
		OutputFormat: params.OutputFormat,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.DeliverableType{}, NotFound("deliverable type not found")
		}
		// Handle unique constraint violation on name update.
		if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate") {
			return db.DeliverableType{}, Conflict("a deliverable type with this name already exists")
		}
		slog.Error("update deliverable type: query failed", "id", params.ID, "user_id", params.UserID, "error", err)
		return db.DeliverableType{}, Internal("failed to update deliverable type")
	}

	slog.Info("deliverable type updated", "id", params.ID, "user_id", params.UserID)
	return dt, nil
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

// Delete removes a deliverable type. System types cannot be deleted (403).
// Types with bound agents cannot be deleted (409). Returns nil on success or a
// typed ServiceError.
func (s *DeliverableTypeService) Delete(ctx context.Context, id, userID string) *ServiceError {
	dtUUID, err := parseUUID(id)
	if err != nil {
		return Validation("invalid deliverable type id format")
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		return Internal("invalid user id")
	}

	// --- Fetch existing to check system type ---
	existing, err := s.q.GetDeliverableType(ctx, db.GetDeliverableTypeParams{
		ID:     dtUUID,
		UserID: userUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NotFound("deliverable type not found")
		}
		slog.Error("delete deliverable type: get failed", "id", id, "error", err)
		return Internal("failed to get deliverable type")
	}

	// --- Reject system types ---
	if existing.IsSystem {
		return Forbidden("system deliverable types cannot be modified")
	}

	// --- Check bound agents ---
	count, err := s.q.CountAgentsByDeliverableType(ctx, dtUUID)
	if err != nil {
		slog.Error("delete deliverable type: count agents failed", "id", id, "error", err)
		return Internal("failed to check agent references")
	}
	if count > 0 {
		return Conflict("cannot delete deliverable type while agents reference it")
	}

	// --- Delete ---
	err = s.q.DeleteDeliverableType(ctx, db.DeleteDeliverableTypeParams{
		ID:     dtUUID,
		UserID: userUUID,
	})
	if err != nil {
		slog.Error("delete deliverable type: query failed", "id", id, "user_id", userID, "error", err)
		return Internal("failed to delete deliverable type")
	}

	slog.Info("deliverable type deleted", "id", id, "user_id", userID)
	return nil
}
