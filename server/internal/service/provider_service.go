package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agenticflow/agenticflow/server/internal/crypto"
	"github.com/agenticflow/agenticflow/server/internal/provider"
	"github.com/agenticflow/agenticflow/server/internal/realtime"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// ProviderService encapsulates business logic for online AI provider CRUD
// operations, credential encryption, validation triggering, and real-time
// event broadcasting.
type ProviderService struct {
	q                  db.Querier
	hub                *realtime.Hub
	encryptor          *crypto.CredentialEncryptor
	registry           *provider.Registry
	agentStatusService *AgentStatusService
}

// NewProviderService creates a new ProviderService with the given dependencies.
func NewProviderService(q db.Querier, hub *realtime.Hub, encryptor *crypto.CredentialEncryptor, registry *provider.Registry) *ProviderService {
	return &ProviderService{
		q:         q,
		hub:       hub,
		encryptor: encryptor,
		registry:  registry,
	}
}

// SetAgentStatusService sets the AgentStatusService used to reconcile agent
// statuses when a provider's status changes. This is set after construction
// to avoid circular initialization dependencies.
func (s *ProviderService) SetAgentStatusService(svc *AgentStatusService) {
	s.agentStatusService = svc
}

// ---------------------------------------------------------------------------
// Request/Response types
// ---------------------------------------------------------------------------

// CreateProviderParams holds the validated parameters for creating a provider.
type CreateProviderParams struct {
	UserID       string
	Name         string
	ProviderType string
	Credentials  json.RawMessage
	Models       []string // optional manual model override
}

// UpdateProviderParams holds the parameters for updating a provider.
type UpdateProviderParams struct {
	UserID      string
	ProviderID  string
	Name        *string
	Credentials json.RawMessage // nil or empty means no change
}

// ProviderResponse represents a provider with masked credentials for API responses.
type ProviderResponse struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	ProviderType  string   `json:"provider_type"`
	Status        string   `json:"status"`
	StatusMessage *string  `json:"status_message"`
	Models        []string `json:"models"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

// Create validates all fields, encrypts credentials, inserts the provider row,
// and triggers async credential validation. Returns the created provider response
// or a typed ServiceError.
func (s *ProviderService) Create(ctx context.Context, params CreateProviderParams) (ProviderResponse, *ServiceError) {
	// --- Validate name ---
	if err := provider.ValidateProviderName(params.Name); err != nil {
		return ProviderResponse{}, Validation(err.Error())
	}

	// --- Validate provider_type ---
	if err := provider.ValidateProviderType(params.ProviderType); err != nil {
		return ProviderResponse{}, Validation(err.Error())
	}

	// --- Validate credentials ---
	if len(params.Credentials) == 0 {
		return ProviderResponse{}, Validation("credentials are required")
	}
	if err := provider.ValidateCredentials(params.ProviderType, params.Credentials); err != nil {
		return ProviderResponse{}, Validation(err.Error())
	}

	// --- Validate manual models (if provided) ---
	if len(params.Models) > 50 {
		return ProviderResponse{}, Validation("models must have 50 or fewer entries")
	}
	for _, m := range params.Models {
		if err := provider.ValidateModelIdentifier(m); err != nil {
			return ProviderResponse{}, Validation(err.Error())
		}
	}

	// --- Encrypt credentials ---
	encrypted, err := s.encryptor.Encrypt(params.Credentials)
	if err != nil {
		slog.Error("create provider: encrypt credentials failed", "error", err)
		return ProviderResponse{}, Internal("failed to encrypt credentials")
	}

	// --- Parse user UUID ---
	userUUID, err := parseUUID(params.UserID)
	if err != nil {
		return ProviderResponse{}, Internal("invalid user id")
	}

	// --- Marshal models ---
	models := params.Models
	if models == nil {
		models = []string{}
	}
	modelsJSON, err := json.Marshal(models)
	if err != nil {
		modelsJSON = []byte("[]")
	}

	// --- Insert provider row with initial status "validating" ---
	p, err := s.q.CreateProvider(ctx, db.CreateProviderParams{
		UserID:               userUUID,
		Name:                 params.Name,
		ProviderType:         params.ProviderType,
		CredentialsEncrypted: encrypted,
		Status:               "validating",
		StatusMessage:        pgtype.Text{},
		Models:               modelsJSON,
	})
	if err != nil {
		slog.Error("create provider: insert failed", "user_id", params.UserID, "error", err)
		return ProviderResponse{}, Internal("failed to create provider")
	}

	slog.Info("provider created", "provider_id", uuidToString(p.ID), "name", p.Name, "user_id", params.UserID)

	// --- Trigger async credential validation ---
	go s.validateProviderAsync(p.ID, params.ProviderType, params.Credentials)

	return providerToResponse(p), nil
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

// List returns all providers owned by the user with masked credentials.
// If statusFilter is non-empty, only providers with that status are returned.
func (s *ProviderService) List(ctx context.Context, userID string, statusFilter string) ([]ProviderResponse, *ServiceError) {
	userUUID, err := parseUUID(userID)
	if err != nil {
		return nil, Internal("invalid user id")
	}

	var providers []db.OnlineProvider
	if statusFilter != "" {
		providers, err = s.q.ListProvidersByUserAndStatus(ctx, db.ListProvidersByUserAndStatusParams{
			UserID: userUUID,
			Status: statusFilter,
		})
	} else {
		providers, err = s.q.ListProvidersByUser(ctx, userUUID)
	}
	if err != nil {
		slog.Error("list providers: query failed", "user_id", userID, "error", err)
		return nil, Internal("failed to list providers")
	}

	responses := make([]ProviderResponse, 0, len(providers))
	for _, p := range providers {
		responses = append(responses, providerToResponse(p))
	}

	return responses, nil
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

// Get retrieves a single provider by ID, scoped to the authenticated user.
// Returns the provider with masked credentials or NotFound if it doesn't exist.
func (s *ProviderService) Get(ctx context.Context, providerID, userID string) (ProviderResponse, *ServiceError) {
	providerUUID, err := parseUUID(providerID)
	if err != nil {
		return ProviderResponse{}, Validation("invalid provider id format")
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		return ProviderResponse{}, Internal("invalid user id")
	}

	p, err := s.q.GetProvider(ctx, db.GetProviderParams{
		ID:     providerUUID,
		UserID: userUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProviderResponse{}, NotFound("provider not found")
		}
		slog.Error("get provider: query failed", "provider_id", providerID, "error", err)
		return ProviderResponse{}, Internal("failed to get provider")
	}

	return providerToResponse(p), nil
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// Update validates changed fields, re-encrypts credentials if changed, persists
// the update, and triggers re-validation if credentials were modified. Returns
// the updated provider response or a typed ServiceError.
func (s *ProviderService) Update(ctx context.Context, params UpdateProviderParams) (ProviderResponse, *ServiceError) {
	providerUUID, err := parseUUID(params.ProviderID)
	if err != nil {
		return ProviderResponse{}, Validation("invalid provider id format")
	}

	userUUID, err := parseUUID(params.UserID)
	if err != nil {
		return ProviderResponse{}, Internal("invalid user id")
	}

	// --- Fetch existing provider to get current values ---
	existing, err := s.q.GetProvider(ctx, db.GetProviderParams{
		ID:     providerUUID,
		UserID: userUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProviderResponse{}, NotFound("provider not found")
		}
		slog.Error("update provider: get existing failed", "provider_id", params.ProviderID, "error", err)
		return ProviderResponse{}, Internal("failed to get provider")
	}

	// --- Determine new name ---
	newName := existing.Name
	if params.Name != nil {
		if err := provider.ValidateProviderName(*params.Name); err != nil {
			return ProviderResponse{}, Validation(err.Error())
		}
		newName = *params.Name
	}

	// --- Determine new credentials ---
	credentialsChanged := false
	newEncrypted := existing.CredentialsEncrypted

	if len(params.Credentials) > 0 {
		// Validate new credentials against the provider's type
		if err := provider.ValidateCredentials(existing.ProviderType, params.Credentials); err != nil {
			return ProviderResponse{}, Validation(err.Error())
		}

		// Encrypt new credentials
		encrypted, err := s.encryptor.Encrypt(params.Credentials)
		if err != nil {
			slog.Error("update provider: encrypt credentials failed", "error", err)
			return ProviderResponse{}, Internal("failed to encrypt credentials")
		}
		newEncrypted = encrypted
		credentialsChanged = true
	}

	// --- Persist update ---
	updated, err := s.q.UpdateProvider(ctx, db.UpdateProviderParams{
		ID:                   providerUUID,
		UserID:               userUUID,
		Name:                 newName,
		CredentialsEncrypted: newEncrypted,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProviderResponse{}, NotFound("provider not found")
		}
		slog.Error("update provider: query failed", "provider_id", params.ProviderID, "error", err)
		return ProviderResponse{}, Internal("failed to update provider")
	}

	slog.Info("provider updated", "provider_id", params.ProviderID, "user_id", params.UserID, "credentials_changed", credentialsChanged)

	// --- If credentials changed, set status to "validating" and trigger re-validation ---
	if credentialsChanged {
		_ = s.q.UpdateProviderStatus(ctx, db.UpdateProviderStatusParams{
			ID:            providerUUID,
			Status:        "validating",
			StatusMessage: pgtype.Text{},
		})
		go s.validateProviderAsync(providerUUID, existing.ProviderType, params.Credentials)
	}

	return providerToResponse(updated), nil
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

// Delete removes the provider record after checking that no agents are bound
// to it. Returns a 409 Conflict if agents reference the provider, or NotFound
// if the provider doesn't exist or isn't owned by the user.
func (s *ProviderService) Delete(ctx context.Context, providerID, userID string) *ServiceError {
	providerUUID, err := parseUUID(providerID)
	if err != nil {
		return Validation("invalid provider id format")
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		return Internal("invalid user id")
	}

	// --- Verify provider exists and belongs to user ---
	_, err = s.q.GetProvider(ctx, db.GetProviderParams{
		ID:     providerUUID,
		UserID: userUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NotFound("provider not found")
		}
		slog.Error("delete provider: get failed", "provider_id", providerID, "error", err)
		return Internal("failed to get provider")
	}

	// --- Check for bound agents ---
	count, err := s.q.CountAgentsByProvider(ctx, providerUUID)
	if err != nil {
		slog.Error("delete provider: count agents failed", "provider_id", providerID, "error", err)
		return Internal("failed to check bound agents")
	}
	if count > 0 {
		return Conflict("cannot delete provider while agents reference it")
	}

	// --- Delete provider ---
	err = s.q.DeleteProvider(ctx, db.DeleteProviderParams{
		ID:     providerUUID,
		UserID: userUUID,
	})
	if err != nil {
		slog.Error("delete provider: query failed", "provider_id", providerID, "user_id", userID, "error", err)
		return Internal("failed to delete provider")
	}

	slog.Info("provider deleted", "provider_id", providerID, "user_id", userID)

	return nil
}

// ---------------------------------------------------------------------------
// ListModels
// ---------------------------------------------------------------------------

// ListModels returns the stored models for a provider. The provider must be
// active; otherwise a 422 error is returned.
func (s *ProviderService) ListModels(ctx context.Context, providerID, userID string) ([]string, *ServiceError) {
	providerUUID, err := parseUUID(providerID)
	if err != nil {
		return nil, Validation("invalid provider id format")
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		return nil, Internal("invalid user id")
	}

	p, err := s.q.GetProvider(ctx, db.GetProviderParams{
		ID:     providerUUID,
		UserID: userUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, NotFound("provider not found")
		}
		slog.Error("list models: get provider failed", "provider_id", providerID, "error", err)
		return nil, Internal("failed to get provider")
	}

	if p.Status != "active" {
		return nil, Unprocessable("provider must be active to list models")
	}

	var models []string
	if err := json.Unmarshal(p.Models, &models); err != nil {
		models = []string{}
	}

	return models, nil
}

// ---------------------------------------------------------------------------
// RefreshModels
// ---------------------------------------------------------------------------

// RefreshModels re-queries the provider's API for available models, updates the
// stored list, and returns the new list. The provider must be active; otherwise
// a 422 error is returned. On failure, the provider status is set to "error"
// and a 502 error is returned.
func (s *ProviderService) RefreshModels(ctx context.Context, providerID, userID string) ([]string, *ServiceError) {
	providerUUID, err := parseUUID(providerID)
	if err != nil {
		return nil, Validation("invalid provider id format")
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		return nil, Internal("invalid user id")
	}

	p, err := s.q.GetProvider(ctx, db.GetProviderParams{
		ID:     providerUUID,
		UserID: userUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, NotFound("provider not found")
		}
		slog.Error("refresh models: get provider failed", "provider_id", providerID, "error", err)
		return nil, Internal("failed to get provider")
	}

	if p.Status != "active" {
		return nil, Unprocessable("provider must be active to refresh models")
	}

	// Decrypt credentials
	decrypted, err := s.encryptor.Decrypt(p.CredentialsEncrypted)
	if err != nil {
		slog.Error("refresh models: decrypt credentials failed", "provider_id", providerID, "error", err)
		return nil, Internal("failed to decrypt provider credentials")
	}

	// Get adapter
	adapter, ok := s.registry.Get(p.ProviderType)
	if !ok {
		slog.Error("refresh models: unknown provider type", "provider_type", p.ProviderType)
		return nil, Internal("unsupported provider type")
	}

	// Call adapter.ListModels with 30s timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	models, err := adapter.ListModels(timeoutCtx, json.RawMessage(decrypted))
	if err != nil {
		// Set provider status to "error" with failure message
		statusMsg := "model refresh failed: " + err.Error()
		_ = s.q.UpdateProviderStatus(ctx, db.UpdateProviderStatusParams{
			ID:            providerUUID,
			Status:        "error",
			StatusMessage: pgtype.Text{String: statusMsg, Valid: true},
		})
		// Reconcile agent statuses after provider status change
		s.reconcileAgentsForProvider(providerUUID)
		slog.Warn("refresh models failed", "provider_id", providerID, "error", err)
		return nil, BadGateway("model refresh failed: " + err.Error())
	}

	// Update stored models
	if models == nil {
		models = []string{}
	}
	modelsJSON, err := json.Marshal(models)
	if err != nil {
		modelsJSON = []byte("[]")
	}
	_ = s.q.UpdateProviderModels(ctx, db.UpdateProviderModelsParams{
		ID:     providerUUID,
		Models: modelsJSON,
	})

	slog.Info("models refreshed", "provider_id", providerID, "models_count", len(models))

	return models, nil
}

// ---------------------------------------------------------------------------
// Validate (re-trigger)
// ---------------------------------------------------------------------------

// Validate re-runs credential validation for the specified provider. The
// provider must exist and belong to the authenticated user. It decrypts the
// stored credentials, sets status to "validating", and triggers async
// validation. Returns the updated provider response.
func (s *ProviderService) Validate(ctx context.Context, providerID, userID string) (ProviderResponse, *ServiceError) {
	providerUUID, err := parseUUID(providerID)
	if err != nil {
		return ProviderResponse{}, Validation("invalid provider id format")
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		return ProviderResponse{}, Internal("invalid user id")
	}

	p, err := s.q.GetProvider(ctx, db.GetProviderParams{
		ID:     providerUUID,
		UserID: userUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProviderResponse{}, NotFound("provider not found")
		}
		slog.Error("validate provider: get failed", "provider_id", providerID, "error", err)
		return ProviderResponse{}, Internal("failed to get provider")
	}

	// Decrypt credentials for validation
	decrypted, err := s.encryptor.Decrypt(p.CredentialsEncrypted)
	if err != nil {
		slog.Error("validate provider: decrypt credentials failed", "provider_id", providerID, "error", err)
		return ProviderResponse{}, Internal("failed to decrypt provider credentials")
	}

	// Set status to "validating"
	_ = s.q.UpdateProviderStatus(ctx, db.UpdateProviderStatusParams{
		ID:            providerUUID,
		Status:        "validating",
		StatusMessage: pgtype.Text{},
	})

	// Trigger async validation
	go s.validateProviderAsync(providerUUID, p.ProviderType, json.RawMessage(decrypted))

	// Return the provider with "validating" status
	p.Status = "validating"
	p.StatusMessage = pgtype.Text{}

	return providerToResponse(p), nil
}

// ---------------------------------------------------------------------------
// Async Validation (internal)
// ---------------------------------------------------------------------------

// validateProviderAsync runs credential validation in the background.
// On success, it sets status to "active" and stores discovered models.
// On failure, it sets status to "error" with an appropriate message based
// on the error type: auth failures, timeouts, or network errors.
// After any status change, it reconciles the status of all agents bound
// to this provider and broadcasts status updates via WebSocket.
func (s *ProviderService) validateProviderAsync(providerID pgtype.UUID, providerType string, creds json.RawMessage) {
	// Use a 30-second timeout for the validation call
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	adapter, ok := s.registry.Get(providerType)
	if !ok {
		slog.Error("validate provider: unknown provider type", "provider_type", providerType)
		_ = s.q.UpdateProviderStatus(context.Background(), db.UpdateProviderStatusParams{
			ID:            providerID,
			Status:        "error",
			StatusMessage: pgtype.Text{String: "unsupported provider type", Valid: true},
		})
		s.reconcileAgentsForProvider(providerID)
		return
	}

	models, err := adapter.ValidateCredentials(ctx, creds)
	if err != nil {
		statusMessage := classifyValidationError(err)
		slog.Warn("provider validation failed", "provider_id", uuidToString(providerID), "error", err, "status_message", statusMessage)
		_ = s.q.UpdateProviderStatus(context.Background(), db.UpdateProviderStatusParams{
			ID:            providerID,
			Status:        "error",
			StatusMessage: pgtype.Text{String: statusMessage, Valid: true},
		})
		s.reconcileAgentsForProvider(providerID)
		return
	}

	// Validation succeeded — set status to "active" and store models
	_ = s.q.UpdateProviderStatus(context.Background(), db.UpdateProviderStatusParams{
		ID:            providerID,
		Status:        "active",
		StatusMessage: pgtype.Text{},
	})

	if models == nil {
		models = []string{}
	}
	modelsJSON, err := json.Marshal(models)
	if err != nil {
		modelsJSON = []byte("[]")
	}
	_ = s.q.UpdateProviderModels(context.Background(), db.UpdateProviderModelsParams{
		ID:     providerID,
		Models: modelsJSON,
	})

	slog.Info("provider validation succeeded", "provider_id", uuidToString(providerID), "models_count", len(models))

	// Reconcile agent statuses after provider status change
	s.reconcileAgentsForProvider(providerID)
}

// reconcileAgentsForProvider triggers agent status reconciliation for all agents
// bound to the given provider. This ensures agent statuses are updated when
// a provider's status changes (e.g., from "validating" to "active" or "error").
func (s *ProviderService) reconcileAgentsForProvider(providerID pgtype.UUID) {
	if s.agentStatusService != nil {
		s.agentStatusService.ReconcileAgentsForProvider(context.Background(), providerID)
	}
}

// classifyValidationError inspects the error returned from a provider
// validation call and returns an appropriate user-facing status message.
// It distinguishes between:
//   - Authentication failures (HTTP 401/403)
//   - Timeout errors (context deadline exceeded)
//   - Network errors (DNS, connection refused, TLS failures)
//   - Other errors (returned as-is)
func classifyValidationError(err error) string {
	// Check for timeout (context deadline exceeded)
	if errors.Is(err, context.DeadlineExceeded) {
		return "connection timeout after 30 seconds"
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return "connection timeout after 30 seconds"
		}
		return fmt.Sprintf("provider endpoint unreachable: %s", netErr.Error())
	}

	// Check for DNS or connection errors by inspecting the error message
	errMsg := err.Error()
	errLower := strings.ToLower(errMsg)

	// Check for auth failures (401/403)
	if strings.Contains(errLower, "401") || strings.Contains(errLower, "403") || strings.Contains(errLower, "authentication") || strings.Contains(errLower, "unauthorized") || strings.Contains(errLower, "forbidden") {
		return "invalid credentials: authentication failed"
	}

	// Check for network-related errors by message content
	if strings.Contains(errLower, "connection refused") ||
		strings.Contains(errLower, "no such host") ||
		strings.Contains(errLower, "dns") ||
		strings.Contains(errLower, "tls") ||
		strings.Contains(errLower, "certificate") ||
		strings.Contains(errLower, "dial tcp") ||
		strings.Contains(errLower, "network is unreachable") {
		return fmt.Sprintf("provider endpoint unreachable: %s", errMsg)
	}

	// Default: return the error message as-is
	return errMsg
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// providerToResponse converts a db.OnlineProvider to a ProviderResponse with
// masked credentials (credentials are not included in the response at all,
// only the models and status fields are exposed).
func providerToResponse(p db.OnlineProvider) ProviderResponse {
	var statusMessage *string
	if p.StatusMessage.Valid {
		statusMessage = &p.StatusMessage.String
	}

	// Parse models from JSON
	var models []string
	if err := json.Unmarshal(p.Models, &models); err != nil {
		models = []string{}
	}

	return ProviderResponse{
		ID:            uuidToString(p.ID),
		Name:          p.Name,
		ProviderType:  p.ProviderType,
		Status:        p.Status,
		StatusMessage: statusMessage,
		Models:        models,
		CreatedAt:     p.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     p.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}
