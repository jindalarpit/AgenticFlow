package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/agenticflow/agenticflow/server/internal/middleware"
	"github.com/agenticflow/agenticflow/server/internal/service"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// DeliverableTypeHandler holds dependencies for deliverable type CRUD HTTP handlers.
type DeliverableTypeHandler struct {
	Service *service.DeliverableTypeService
}

// NewDeliverableTypeHandler creates a new DeliverableTypeHandler.
func NewDeliverableTypeHandler(queries db.Querier) *DeliverableTypeHandler {
	return &DeliverableTypeHandler{
		Service: service.NewDeliverableTypeService(queries),
	}
}

// RegisterRoutes registers all deliverable type routes on the given Chi router.
// All routes require auth middleware to be applied by the caller.
func (h *DeliverableTypeHandler) RegisterRoutes(r chi.Router) {
	r.Post("/api/deliverable-types", h.Create)
	r.Get("/api/deliverable-types", h.List)
	r.Get("/api/deliverable-types/{id}", h.Get)
	r.Put("/api/deliverable-types/{id}", h.Update)
	r.Delete("/api/deliverable-types/{id}", h.Delete)
}

// ---------------------------------------------------------------------------
// Request/Response types
// ---------------------------------------------------------------------------

// CreateDeliverableTypeRequest is the JSON body for POST /api/deliverable-types.
type CreateDeliverableTypeRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	OutputFormat string `json:"output_format"`
}

// UpdateDeliverableTypeRequest is the JSON body for PUT /api/deliverable-types/{id}.
type UpdateDeliverableTypeRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	OutputFormat string `json:"output_format"`
}

// DeliverableTypeResponse is the public representation of a deliverable type.
type DeliverableTypeResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	OutputFormat string `json:"output_format"`
	IsSystem     bool   `json:"is_system"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// toDeliverableTypeResponse converts a db.DeliverableType to the public response.
func toDeliverableTypeResponse(dt db.DeliverableType) DeliverableTypeResponse {
	return DeliverableTypeResponse{
		ID:           uuidToString(dt.ID),
		Name:         dt.Name,
		Description:  dt.Description,
		OutputFormat: dt.OutputFormat,
		IsSystem:     dt.IsSystem,
		CreatedAt:    dt.CreatedAt.Time.UTC().Format(time.RFC3339),
		UpdatedAt:    dt.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}
}

// ---------------------------------------------------------------------------
// POST /api/deliverable-types — Create
// ---------------------------------------------------------------------------

// Create handles POST /api/deliverable-types.
func (h *DeliverableTypeHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateDeliverableTypeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	dt, svcErr := h.Service.Create(r.Context(), service.CreateDeliverableTypeParams{
		UserID:       userID,
		Name:         req.Name,
		Description:  req.Description,
		OutputFormat: req.OutputFormat,
	})
	if svcErr != nil {
		handleServiceError(w, svcErr)
		return
	}

	writeJSON(w, http.StatusCreated, toDeliverableTypeResponse(dt))
}

// ---------------------------------------------------------------------------
// GET /api/deliverable-types — List
// ---------------------------------------------------------------------------

// List handles GET /api/deliverable-types.
func (h *DeliverableTypeHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	types, svcErr := h.Service.List(r.Context(), userID)
	if svcErr != nil {
		handleServiceError(w, svcErr)
		return
	}

	resp := make([]DeliverableTypeResponse, 0, len(types))
	for _, dt := range types {
		resp = append(resp, toDeliverableTypeResponse(dt))
	}

	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// GET /api/deliverable-types/{id} — Get
// ---------------------------------------------------------------------------

// Get handles GET /api/deliverable-types/{id}.
func (h *DeliverableTypeHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeErrorJSON(w, http.StatusBadRequest, "deliverable type id is required")
		return
	}

	dt, svcErr := h.Service.Get(r.Context(), id, userID)
	if svcErr != nil {
		handleServiceError(w, svcErr)
		return
	}

	writeJSON(w, http.StatusOK, toDeliverableTypeResponse(dt))
}

// ---------------------------------------------------------------------------
// PUT /api/deliverable-types/{id} — Update
// ---------------------------------------------------------------------------

// Update handles PUT /api/deliverable-types/{id}.
func (h *DeliverableTypeHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeErrorJSON(w, http.StatusBadRequest, "deliverable type id is required")
		return
	}

	var req UpdateDeliverableTypeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	dt, svcErr := h.Service.Update(r.Context(), service.UpdateDeliverableTypeParams{
		UserID:       userID,
		ID:           id,
		Name:         req.Name,
		Description:  req.Description,
		OutputFormat: req.OutputFormat,
	})
	if svcErr != nil {
		handleServiceError(w, svcErr)
		return
	}

	writeJSON(w, http.StatusOK, toDeliverableTypeResponse(dt))
}

// ---------------------------------------------------------------------------
// DELETE /api/deliverable-types/{id} — Delete
// ---------------------------------------------------------------------------

// Delete handles DELETE /api/deliverable-types/{id}.
func (h *DeliverableTypeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeErrorJSON(w, http.StatusBadRequest, "deliverable type id is required")
		return
	}

	svcErr := h.Service.Delete(r.Context(), id, userID)
	if svcErr != nil {
		handleServiceError(w, svcErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
