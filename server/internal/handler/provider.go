package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/agenticflow/agenticflow/server/internal/middleware"
	"github.com/agenticflow/agenticflow/server/internal/service"
)

// ProviderHandler holds dependencies for online AI provider HTTP handlers.
type ProviderHandler struct {
	Service *service.ProviderService
}

// NewProviderHandler creates a new ProviderHandler with the given service.
func NewProviderHandler(svc *service.ProviderService) *ProviderHandler {
	return &ProviderHandler{Service: svc}
}

// RegisterRoutes registers all provider routes on the given Chi router.
// All routes require auth middleware to be applied by the caller.
func (h *ProviderHandler) RegisterRoutes(r chi.Router) {
	r.Post("/api/providers", h.CreateProvider)
	r.Get("/api/providers", h.ListProviders)
	r.Get("/api/providers/{id}", h.GetProvider)
	r.Put("/api/providers/{id}", h.UpdateProvider)
	r.Delete("/api/providers/{id}", h.DeleteProvider)
	r.Post("/api/providers/{id}/validate", h.ValidateProvider)
	r.Get("/api/providers/{id}/models", h.ListModels)
	r.Post("/api/providers/{id}/refresh-models", h.RefreshModels)
}

// ---------------------------------------------------------------------------
// Request types
// ---------------------------------------------------------------------------

// CreateProviderHTTPRequest is the JSON body for POST /api/providers.
type CreateProviderHTTPRequest struct {
	Name         string          `json:"name"`
	ProviderType string          `json:"provider_type"`
	Credentials  json.RawMessage `json:"credentials"`
	Models       []string        `json:"models,omitempty"`
}

// UpdateProviderHTTPRequest is the JSON body for PUT /api/providers/:id.
type UpdateProviderHTTPRequest struct {
	Name        *string         `json:"name"`
	Credentials json.RawMessage `json:"credentials,omitempty"`
}

// ---------------------------------------------------------------------------
// POST /api/providers — CreateProvider
// ---------------------------------------------------------------------------

// CreateProvider handles POST /api/providers.
func (h *ProviderHandler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateProviderHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, svcErr := h.Service.Create(r.Context(), service.CreateProviderParams{
		UserID:       userID,
		Name:         req.Name,
		ProviderType: req.ProviderType,
		Credentials:  req.Credentials,
		Models:       req.Models,
	})
	if svcErr != nil {
		writeErrorJSON(w, svcErr.Kind.HTTPStatus(), svcErr.Message)
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

// ---------------------------------------------------------------------------
// GET /api/providers — ListProviders
// ---------------------------------------------------------------------------

// ListProviders handles GET /api/providers.
// Supports optional ?status=active query parameter for filtering.
func (h *ProviderHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	statusFilter := r.URL.Query().Get("status")

	providers, svcErr := h.Service.List(r.Context(), userID, statusFilter)
	if svcErr != nil {
		writeErrorJSON(w, svcErr.Kind.HTTPStatus(), svcErr.Message)
		return
	}

	writeJSON(w, http.StatusOK, providers)
}

// ---------------------------------------------------------------------------
// GET /api/providers/{id} — GetProvider
// ---------------------------------------------------------------------------

// GetProvider handles GET /api/providers/{id}.
func (h *ProviderHandler) GetProvider(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	providerID := chi.URLParam(r, "id")
	if providerID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "provider id is required")
		return
	}

	resp, svcErr := h.Service.Get(r.Context(), providerID, userID)
	if svcErr != nil {
		writeErrorJSON(w, svcErr.Kind.HTTPStatus(), svcErr.Message)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// PUT /api/providers/{id} — UpdateProvider
// ---------------------------------------------------------------------------

// UpdateProvider handles PUT /api/providers/{id}.
func (h *ProviderHandler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	providerID := chi.URLParam(r, "id")
	if providerID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "provider id is required")
		return
	}

	var req UpdateProviderHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, svcErr := h.Service.Update(r.Context(), service.UpdateProviderParams{
		UserID:      userID,
		ProviderID:  providerID,
		Name:        req.Name,
		Credentials: req.Credentials,
	})
	if svcErr != nil {
		writeErrorJSON(w, svcErr.Kind.HTTPStatus(), svcErr.Message)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// DELETE /api/providers/{id} — DeleteProvider
// ---------------------------------------------------------------------------

// DeleteProvider handles DELETE /api/providers/{id}.
func (h *ProviderHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	providerID := chi.URLParam(r, "id")
	if providerID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "provider id is required")
		return
	}

	svcErr := h.Service.Delete(r.Context(), providerID, userID)
	if svcErr != nil {
		writeErrorJSON(w, svcErr.Kind.HTTPStatus(), svcErr.Message)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// POST /api/providers/{id}/validate — ValidateProvider
// ---------------------------------------------------------------------------

// ValidateProvider handles POST /api/providers/{id}/validate.
// It re-triggers credential validation for the specified provider.
func (h *ProviderHandler) ValidateProvider(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	providerID := chi.URLParam(r, "id")
	if providerID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "provider id is required")
		return
	}

	resp, svcErr := h.Service.Validate(r.Context(), providerID, userID)
	if svcErr != nil {
		writeErrorJSON(w, svcErr.Kind.HTTPStatus(), svcErr.Message)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// GET /api/providers/{id}/models — ListModels
// ---------------------------------------------------------------------------

// ListModels handles GET /api/providers/{id}/models.
// Returns the stored list of model identifiers for the provider.
func (h *ProviderHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	providerID := chi.URLParam(r, "id")
	if providerID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "provider id is required")
		return
	}

	models, svcErr := h.Service.ListModels(r.Context(), providerID, userID)
	if svcErr != nil {
		writeErrorJSON(w, svcErr.Kind.HTTPStatus(), svcErr.Message)
		return
	}

	writeJSON(w, http.StatusOK, map[string][]string{"models": models})
}

// ---------------------------------------------------------------------------
// POST /api/providers/{id}/refresh-models — RefreshModels
// ---------------------------------------------------------------------------

// RefreshModels handles POST /api/providers/{id}/refresh-models.
// Re-queries the provider's API for available models and updates the stored list.
func (h *ProviderHandler) RefreshModels(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	providerID := chi.URLParam(r, "id")
	if providerID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "provider id is required")
		return
	}

	models, svcErr := h.Service.RefreshModels(r.Context(), providerID, userID)
	if svcErr != nil {
		writeErrorJSON(w, svcErr.Kind.HTTPStatus(), svcErr.Message)
		return
	}

	writeJSON(w, http.StatusOK, map[string][]string{"models": models})
}
