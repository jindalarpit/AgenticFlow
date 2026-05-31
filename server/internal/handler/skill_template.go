package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agenticflow/agenticflow/server/internal/middleware"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// SkillTemplateHandler holds dependencies for skill template HTTP handlers.
type SkillTemplateHandler struct {
	Queries db.Querier
}

// NewSkillTemplateHandler creates a new SkillTemplateHandler.
func NewSkillTemplateHandler(queries db.Querier) *SkillTemplateHandler {
	return &SkillTemplateHandler{Queries: queries}
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

// SkillTemplateSummaryResponse is the public representation of a skill template
// in list responses (no content field).
type SkillTemplateSummaryResponse struct {
	ID          string  `json:"id"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Version     string  `json:"version"`
	Icon        *string `json:"icon"`
}

// SkillTemplateDetailResponse is the full public representation of a skill
// template including the content field.
type SkillTemplateDetailResponse struct {
	ID          string  `json:"id"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Version     string  `json:"version"`
	Icon        *string `json:"icon"`
	Content     string  `json:"content"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// TemplateOrigin is stored in a skill's config JSON when instantiated from a template.
type TemplateOrigin struct {
	TemplateSlug    string `json:"template_slug"`
	TemplateVersion string `json:"template_version"`
	InstantiatedAt  string `json:"instantiated_at"`
}

// ---------------------------------------------------------------------------
// GET /api/skill-templates — List
// ---------------------------------------------------------------------------

// List handles GET /api/skill-templates with optional category query param.
func (h *SkillTemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	category := r.URL.Query().Get("category")

	if category != "" {
		templates, err := h.Queries.ListSkillTemplatesByCategory(r.Context(), category)
		if err != nil {
			slog.Error("list skill templates by category: query failed", "category", category, "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "internal server error")
			return
		}

		resp := make([]SkillTemplateSummaryResponse, 0, len(templates))
		for _, t := range templates {
			resp = append(resp, SkillTemplateSummaryResponse{
				ID:          uuidToString(t.ID),
				Slug:        t.Slug,
				Name:        t.Name,
				Description: t.Description,
				Category:    t.Category,
				Version:     t.Version,
				Icon:        pgTextToStringPtr(t.Icon),
			})
		}

		writeJSON(w, http.StatusOK, resp)
		return
	}

	templates, err := h.Queries.ListSkillTemplates(r.Context())
	if err != nil {
		slog.Error("list skill templates: query failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := make([]SkillTemplateSummaryResponse, 0, len(templates))
	for _, t := range templates {
		resp = append(resp, SkillTemplateSummaryResponse{
			ID:          uuidToString(t.ID),
			Slug:        t.Slug,
			Name:        t.Name,
			Description: t.Description,
			Category:    t.Category,
			Version:     t.Version,
			Icon:        pgTextToStringPtr(t.Icon),
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// GET /api/skill-templates/{slug} — GetBySlug
// ---------------------------------------------------------------------------

// GetBySlug handles GET /api/skill-templates/{slug}.
func (h *SkillTemplateHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	slug := chi.URLParam(r, "slug")
	if slug == "" {
		writeErrorJSON(w, http.StatusBadRequest, "slug is required")
		return
	}

	template, err := h.Queries.GetSkillTemplateBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErrorJSON(w, http.StatusNotFound, "template not found")
			return
		}
		slog.Error("get skill template by slug: query failed", "slug", slug, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := SkillTemplateDetailResponse{
		ID:          uuidToString(template.ID),
		Slug:        template.Slug,
		Name:        template.Name,
		Description: template.Description,
		Category:    template.Category,
		Version:     template.Version,
		Icon:        pgTextToStringPtr(template.Icon),
		Content:     template.Content,
		CreatedAt:   template.CreatedAt.Time.UTC().Format(time.RFC3339),
		UpdatedAt:   template.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}

	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// POST /api/skill-templates/{slug}/instantiate — Instantiate
// ---------------------------------------------------------------------------

// Instantiate handles POST /api/skill-templates/{slug}/instantiate.
func (h *SkillTemplateHandler) Instantiate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	slug := chi.URLParam(r, "slug")
	if slug == "" {
		writeErrorJSON(w, http.StatusBadRequest, "slug is required")
		return
	}

	// Look up template by slug.
	template, err := h.Queries.GetSkillTemplateBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErrorJSON(w, http.StatusNotFound, "template not found")
			return
		}
		slog.Error("instantiate template: get template failed", "slug", slug, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "internal server error")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	// Check if user already has a skill with the same name (slug).
	_, err = h.Queries.GetSkillByName(r.Context(), db.GetSkillByNameParams{
		UserID: userUUID,
		Name:   template.Slug,
	})
	if err == nil {
		writeErrorJSON(w, http.StatusConflict, "a skill with this name already exists")
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		slog.Error("instantiate template: check duplicate name failed", "slug", slug, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to check skill name")
		return
	}

	// Build config with template_origin.
	origin := TemplateOrigin{
		TemplateSlug:    template.Slug,
		TemplateVersion: template.Version,
		InstantiatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	configMap := map[string]interface{}{
		"template_origin": origin,
	}
	configBytes, err := json.Marshal(configMap)
	if err != nil {
		slog.Error("instantiate template: marshal config failed", "slug", slug, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to create skill")
		return
	}

	// Create the skill with template's content, description, and slug as name.
	skill, err := h.Queries.CreateSkill(r.Context(), db.CreateSkillParams{
		UserID:      userUUID,
		Name:        template.Slug,
		Description: template.Description,
		Content:     template.Content,
		Config:      configBytes,
	})
	if err != nil {
		slog.Error("instantiate template: create skill failed", "slug", slug, "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to create skill")
		return
	}

	slog.Info("skill instantiated from template",
		"skill_id", uuidToString(skill.ID),
		"template_slug", template.Slug,
		"user_id", userID,
	)

	resp := toSkillResponse(skill, []SkillFileResponse{}, 0)
	writeJSON(w, http.StatusCreated, resp)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// pgTextToStringPtr converts a pgtype.Text to a *string.
// Returns nil if the text is not valid (NULL in the database).
func pgTextToStringPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}
