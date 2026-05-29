package handler

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/agenticflow/agenticflow/server/internal/middleware"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// Validation constants for skill fields.
const (
	maxSkillNameLength    = 64
	maxSkillContentSize   = 200 * 1024  // 200KB
	maxSkillFileSize      = 1024 * 1024 // 1MB
	maxSkillDescLength    = 255
)

// skillNameRegex validates skill names: lowercase alphanumeric with hyphens,
// starting with an alphanumeric character. 1-64 chars total.
var skillNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// SkillHandler holds dependencies for skill CRUD HTTP handlers.
type SkillHandler struct {
	Queries db.Querier
}

// NewSkillHandler creates a new SkillHandler.
func NewSkillHandler(queries db.Querier) *SkillHandler {
	return &SkillHandler{Queries: queries}
}

// ---------------------------------------------------------------------------
// Request/Response types
// ---------------------------------------------------------------------------

// CreateSkillRequest is the JSON body for POST /api/skills.
type CreateSkillRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Content     string            `json:"content"`
	Config      json.RawMessage   `json:"config,omitempty"`
	Files       []SkillFileInput  `json:"files,omitempty"`
}

// SkillFileInput represents a file to create within a skill.
type SkillFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// UpdateSkillRequest is the JSON body for PUT /api/skills/:id.
// All fields are pointers; nil means "do not change".
type UpdateSkillRequest struct {
	Name        *string           `json:"name"`
	Description *string           `json:"description"`
	Content     *string           `json:"content"`
	Config      json.RawMessage   `json:"config,omitempty"`
	Files       *[]SkillFileInput `json:"files,omitempty"`
}

// SkillResponse is the public representation of a skill.
type SkillResponse struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Content     string              `json:"content"`
	Config      json.RawMessage     `json:"config,omitempty"`
	Files       []SkillFileResponse `json:"files"`
	AgentCount  int32               `json:"agent_count"`
	CreatedAt   string              `json:"created_at"`
	UpdatedAt   string              `json:"updated_at"`
}

// SkillFileResponse is the public representation of a skill file.
type SkillFileResponse struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
}

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

// ValidateSkillName checks if a skill name matches the required pattern and length.
func ValidateSkillName(name string) bool {
	if len(name) == 0 || len(name) > maxSkillNameLength {
		return false
	}
	return skillNameRegex.MatchString(name)
}

// ---------------------------------------------------------------------------
// POST /api/skills — Create
// ---------------------------------------------------------------------------

// Create handles POST /api/skills.
func (h *SkillHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate name.
	if req.Name == "" {
		writeErrorJSON(w, http.StatusBadRequest, "name is required")
		return
	}
	if !ValidateSkillName(req.Name) {
		writeErrorJSON(w, http.StatusBadRequest,
			"name must match pattern: lowercase alphanumeric with hyphens, starting with alphanumeric")
		return
	}

	// Validate description length.
	if len(req.Description) > maxSkillDescLength {
		writeErrorJSON(w, http.StatusBadRequest, "description must be 255 characters or fewer")
		return
	}

	// Validate content size.
	if len(req.Content) > maxSkillContentSize {
		writeErrorJSON(w, http.StatusBadRequest, "content must be 200KB or smaller")
		return
	}

	// Validate file sizes.
	for _, f := range req.Files {
		if len(f.Content) > maxSkillFileSize {
			writeErrorJSON(w, http.StatusBadRequest, "file content must be 1MB or smaller")
			return
		}
		if f.Path == "" {
			writeErrorJSON(w, http.StatusBadRequest, "file path is required")
			return
		}
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	// Check for duplicate name.
	_, err = h.Queries.GetSkillByName(r.Context(), db.GetSkillByNameParams{
		UserID: userUUID,
		Name:   req.Name,
	})
	if err == nil {
		writeErrorJSON(w, http.StatusConflict, "a skill with this name already exists")
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		slog.Error("create skill: check duplicate name failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to check skill name")
		return
	}

	// Create the skill.
	var configBytes []byte
	if len(req.Config) > 0 {
		configBytes = req.Config
	}

	skill, err := h.Queries.CreateSkill(r.Context(), db.CreateSkillParams{
		UserID:      userUUID,
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
		Config:      configBytes,
	})
	if err != nil {
		slog.Error("create skill: insert failed", "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to create skill")
		return
	}

	// Create skill files.
	fileResponses := make([]SkillFileResponse, 0, len(req.Files))
	for _, f := range req.Files {
		sf, err := h.Queries.CreateSkillFile(r.Context(), db.CreateSkillFileParams{
			SkillID: skill.ID,
			Path:    f.Path,
			Content: f.Content,
		})
		if err != nil {
			slog.Error("create skill: create file failed", "skill_id", uuidToString(skill.ID), "path", f.Path, "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "failed to create skill file")
			return
		}
		fileResponses = append(fileResponses, SkillFileResponse{
			ID:      uuidToString(sf.ID),
			Path:    sf.Path,
			Content: sf.Content,
		})
	}

	slog.Info("skill created", "skill_id", uuidToString(skill.ID), "name", skill.Name, "user_id", userID)

	resp := toSkillResponse(skill, fileResponses, 0)
	writeJSON(w, http.StatusCreated, resp)
}

// ---------------------------------------------------------------------------
// GET /api/skills — List
// ---------------------------------------------------------------------------

// List handles GET /api/skills.
func (h *SkillHandler) List(w http.ResponseWriter, r *http.Request) {
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

	skills, err := h.Queries.ListSkillsByUser(r.Context(), userUUID)
	if err != nil {
		slog.Error("list skills: query failed", "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to list skills")
		return
	}

	resp := make([]SkillResponse, 0, len(skills))
	for _, s := range skills {
		// For list, get file metadata only (no content).
		files, err := h.Queries.ListSkillFiles(r.Context(), s.ID)
		if err != nil {
			slog.Error("list skills: list files failed", "skill_id", uuidToString(s.ID), "error", err)
			// Continue without files rather than failing the whole request.
			files = nil
		}

		fileResponses := make([]SkillFileResponse, 0, len(files))
		for _, f := range files {
			fileResponses = append(fileResponses, SkillFileResponse{
				ID:   uuidToString(f.ID),
				Path: f.Path,
				// Content omitted in list view.
			})
		}

		resp = append(resp, SkillResponse{
			ID:          uuidToString(s.ID),
			Name:        s.Name,
			Description: s.Description,
			Content:     s.Content,
			Config:      normalizeConfig(s.Config),
			Files:       fileResponses,
			AgentCount:  s.AgentCount,
			CreatedAt:   s.CreatedAt.Time.UTC().Format(time.RFC3339),
			UpdatedAt:   s.UpdatedAt.Time.UTC().Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// GET /api/skills/{id} — Get
// ---------------------------------------------------------------------------

// Get handles GET /api/skills/{id}.
func (h *SkillHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	skillID := chi.URLParam(r, "id")
	if skillID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "skill id is required")
		return
	}

	skillUUID, err := parseUUID(skillID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid skill id format")
		return
	}

	skill, err := h.Queries.GetSkillByID(r.Context(), skillUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErrorJSON(w, http.StatusNotFound, "skill not found")
			return
		}
		slog.Error("get skill: query failed", "skill_id", skillID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to get skill")
		return
	}

	// Ownership check.
	if uuidToString(skill.UserID) != userID {
		writeErrorJSON(w, http.StatusForbidden, "forbidden")
		return
	}

	// Get files with content.
	files, err := h.Queries.GetSkillFilesWithContent(r.Context(), skill.ID)
	if err != nil {
		slog.Error("get skill: get files failed", "skill_id", skillID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to get skill files")
		return
	}

	fileResponses := make([]SkillFileResponse, 0, len(files))
	for _, f := range files {
		fileResponses = append(fileResponses, SkillFileResponse{
			ID:      uuidToString(f.ID),
			Path:    f.Path,
			Content: f.Content,
		})
	}

	// Get agent count.
	agentCount, err := h.Queries.CountAgentsBySkill(r.Context(), skill.ID)
	if err != nil {
		slog.Error("get skill: count agents failed", "skill_id", skillID, "error", err)
		agentCount = 0
	}

	resp := toSkillResponse(skill, fileResponses, int32(agentCount))
	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// PUT /api/skills/{id} — Update
// ---------------------------------------------------------------------------

// Update handles PUT /api/skills/{id}.
func (h *SkillHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	skillID := chi.URLParam(r, "id")
	if skillID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "skill id is required")
		return
	}

	skillUUID, err := parseUUID(skillID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid skill id format")
		return
	}

	// Fetch existing skill for ownership check.
	existing, err := h.Queries.GetSkillByID(r.Context(), skillUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErrorJSON(w, http.StatusNotFound, "skill not found")
			return
		}
		slog.Error("update skill: get failed", "skill_id", skillID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to get skill")
		return
	}

	// Ownership check.
	if uuidToString(existing.UserID) != userID {
		writeErrorJSON(w, http.StatusForbidden, "forbidden")
		return
	}

	var req UpdateSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate name if provided.
	if req.Name != nil {
		if *req.Name == "" {
			writeErrorJSON(w, http.StatusBadRequest, "name is required")
			return
		}
		if !ValidateSkillName(*req.Name) {
			writeErrorJSON(w, http.StatusBadRequest,
				"name must match pattern: lowercase alphanumeric with hyphens, starting with alphanumeric")
			return
		}

		// Check for duplicate name if name is changing.
		if *req.Name != existing.Name {
			userUUID, _ := parseUUID(userID)
			_, err := h.Queries.GetSkillByName(r.Context(), db.GetSkillByNameParams{
				UserID: userUUID,
				Name:   *req.Name,
			})
			if err == nil {
				writeErrorJSON(w, http.StatusConflict, "a skill with this name already exists")
				return
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				slog.Error("update skill: check duplicate name failed", "error", err)
				writeErrorJSON(w, http.StatusInternalServerError, "failed to check skill name")
				return
			}
		}
	}

	// Validate description if provided.
	if req.Description != nil && len(*req.Description) > maxSkillDescLength {
		writeErrorJSON(w, http.StatusBadRequest, "description must be 255 characters or fewer")
		return
	}

	// Validate content size if provided.
	if req.Content != nil && len(*req.Content) > maxSkillContentSize {
		writeErrorJSON(w, http.StatusBadRequest, "content must be 200KB or smaller")
		return
	}

	// Validate file sizes if provided.
	if req.Files != nil {
		for _, f := range *req.Files {
			if len(f.Content) > maxSkillFileSize {
				writeErrorJSON(w, http.StatusBadRequest, "file content must be 1MB or smaller")
				return
			}
			if f.Path == "" {
				writeErrorJSON(w, http.StatusBadRequest, "file path is required")
				return
			}
		}
	}

	// Build update params.
	var nameVal interface{}
	if req.Name != nil {
		nameVal = *req.Name
	} else {
		nameVal = ""
	}

	var contentVal interface{}
	if req.Content != nil {
		contentVal = *req.Content
	} else {
		contentVal = ""
	}

	var descVal string
	if req.Description != nil {
		descVal = *req.Description
	} else {
		descVal = existing.Description
	}

	var configBytes []byte
	if len(req.Config) > 0 {
		configBytes = req.Config
	} else {
		configBytes = existing.Config
	}

	updated, err := h.Queries.UpdateSkill(r.Context(), db.UpdateSkillParams{
		ID:          skillUUID,
		Column2:     nameVal,
		Description: descVal,
		Column4:     contentVal,
		Config:      configBytes,
	})
	if err != nil {
		slog.Error("update skill: query failed", "skill_id", skillID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to update skill")
		return
	}

	// Replace files if provided.
	if req.Files != nil {
		// Delete existing files.
		if err := h.Queries.DeleteSkillFilesBySkill(r.Context(), skillUUID); err != nil {
			slog.Error("update skill: delete files failed", "skill_id", skillID, "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "failed to update skill files")
			return
		}

		// Create new files.
		for _, f := range *req.Files {
			_, err := h.Queries.CreateSkillFile(r.Context(), db.CreateSkillFileParams{
				SkillID: skillUUID,
				Path:    f.Path,
				Content: f.Content,
			})
			if err != nil {
				slog.Error("update skill: create file failed", "skill_id", skillID, "path", f.Path, "error", err)
				writeErrorJSON(w, http.StatusInternalServerError, "failed to create skill file")
				return
			}
		}
	}

	// Get updated files with content.
	files, err := h.Queries.GetSkillFilesWithContent(r.Context(), skillUUID)
	if err != nil {
		slog.Error("update skill: get files failed", "skill_id", skillID, "error", err)
		files = nil
	}

	fileResponses := make([]SkillFileResponse, 0, len(files))
	for _, f := range files {
		fileResponses = append(fileResponses, SkillFileResponse{
			ID:      uuidToString(f.ID),
			Path:    f.Path,
			Content: f.Content,
		})
	}

	// Get agent count.
	agentCount, err := h.Queries.CountAgentsBySkill(r.Context(), skillUUID)
	if err != nil {
		agentCount = 0
	}

	slog.Info("skill updated", "skill_id", skillID, "user_id", userID)

	resp := toSkillResponse(updated, fileResponses, int32(agentCount))
	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// DELETE /api/skills/{id} — Delete
// ---------------------------------------------------------------------------

// Delete handles DELETE /api/skills/{id}.
func (h *SkillHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	skillID := chi.URLParam(r, "id")
	if skillID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "skill id is required")
		return
	}

	skillUUID, err := parseUUID(skillID)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid skill id format")
		return
	}

	// Fetch existing skill for ownership check.
	existing, err := h.Queries.GetSkillByID(r.Context(), skillUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErrorJSON(w, http.StatusNotFound, "skill not found")
			return
		}
		slog.Error("delete skill: get failed", "skill_id", skillID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to get skill")
		return
	}

	// Ownership check.
	if uuidToString(existing.UserID) != userID {
		writeErrorJSON(w, http.StatusForbidden, "forbidden")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	err = h.Queries.DeleteSkill(r.Context(), db.DeleteSkillParams{
		ID:     skillUUID,
		UserID: userUUID,
	})
	if err != nil {
		slog.Error("delete skill: query failed", "skill_id", skillID, "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to delete skill")
		return
	}

	slog.Info("skill deleted", "skill_id", skillID, "user_id", userID)
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// toSkillResponse converts a db.Skill and file responses to the public SkillResponse.
func toSkillResponse(s db.Skill, files []SkillFileResponse, agentCount int32) SkillResponse {
	return SkillResponse{
		ID:          uuidToString(s.ID),
		Name:        s.Name,
		Description: s.Description,
		Content:     s.Content,
		Config:      normalizeConfig(s.Config),
		Files:       files,
		AgentCount:  agentCount,
		CreatedAt:   s.CreatedAt.Time.UTC().Format(time.RFC3339),
		UpdatedAt:   s.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}
}

// normalizeConfig returns nil for empty/null config bytes, otherwise the raw JSON.
func normalizeConfig(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}

// ---------------------------------------------------------------------------
// POST /api/skills/import — Import from URL
// ---------------------------------------------------------------------------

// maxImportSize is the maximum response body size when fetching a skill from a URL.
const maxImportSize = 200 * 1024 // 200KB

// githubBlobRegex matches GitHub file URLs in the form:
// github.com/:owner/:repo/blob/:ref/:path
var githubBlobRegex = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/blob/(.+)$`)

// NormalizeGitHubURL converts a GitHub blob URL to a raw.githubusercontent.com URL.
// If the URL is not a GitHub blob URL, it is returned unchanged.
func NormalizeGitHubURL(rawURL string) string {
	matches := githubBlobRegex.FindStringSubmatch(rawURL)
	if matches == nil {
		return rawURL
	}
	// matches[1] = owner, matches[2] = repo, matches[3] = ref/path
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", matches[1], matches[2], matches[3])
}

// ImportSkillRequest is the JSON body for POST /api/skills/import.
type ImportSkillRequest struct {
	URL string `json:"url"`
}

// Import handles POST /api/skills/import.
func (h *SkillHandler) Import(w http.ResponseWriter, r *http.Request) {
	userID := middleware.ContextUserID(r.Context())
	if userID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req ImportSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.URL == "" {
		writeErrorJSON(w, http.StatusBadRequest, "url is required")
		return
	}

	// Normalize GitHub URLs.
	fetchURL := NormalizeGitHubURL(req.URL)

	// Fetch content from URL with timeout.
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(fetchURL)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "failed to fetch skill from URL: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeErrorJSON(w, http.StatusBadRequest,
			fmt.Sprintf("failed to fetch skill from URL: HTTP %d", resp.StatusCode))
		return
	}

	// Read body with size limit.
	limitedReader := io.LimitReader(resp.Body, maxImportSize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "failed to fetch skill from URL: "+err.Error())
		return
	}
	if len(body) > maxImportSize {
		writeErrorJSON(w, http.StatusBadRequest, "content exceeds maximum size")
		return
	}

	content := string(body)

	// Validate that content looks like markdown (has some text content).
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		writeErrorJSON(w, http.StatusBadRequest, "URL does not contain valid markdown content")
		return
	}

	// Parse YAML frontmatter for name and description.
	name, description := parseFrontmatterFields(content)

	// If no name in frontmatter, derive from URL path.
	if name == "" {
		name = deriveNameFromURL(req.URL)
	}

	// Validate the derived/parsed name.
	if !ValidateSkillName(name) {
		// Sanitize the name to make it valid.
		name = sanitizeToSkillName(name)
	}

	if name == "" || !ValidateSkillName(name) {
		writeErrorJSON(w, http.StatusBadRequest, "could not derive a valid skill name from URL")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	// Check for duplicate name.
	_, err = h.Queries.GetSkillByName(r.Context(), db.GetSkillByNameParams{
		UserID: userUUID,
		Name:   name,
	})
	if err == nil {
		writeErrorJSON(w, http.StatusConflict, "a skill with this name already exists")
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		slog.Error("import skill: check duplicate name failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to check skill name")
		return
	}

	// Build config with origin URL.
	configMap := map[string]string{"origin": req.URL}
	configBytes, _ := json.Marshal(configMap)

	// Create the skill.
	skill, err := h.Queries.CreateSkill(r.Context(), db.CreateSkillParams{
		UserID:      userUUID,
		Name:        name,
		Description: description,
		Content:     content,
		Config:      configBytes,
	})
	if err != nil {
		slog.Error("import skill: insert failed", "user_id", userID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "failed to create skill")
		return
	}

	slog.Info("skill imported", "skill_id", uuidToString(skill.ID), "name", skill.Name, "url", req.URL)

	resp2 := toSkillResponse(skill, []SkillFileResponse{}, 0)
	writeJSON(w, http.StatusCreated, resp2)
}

// ---------------------------------------------------------------------------
// Frontmatter parsing helpers (for import)
// ---------------------------------------------------------------------------

// parseFrontmatterFields extracts name and description from YAML frontmatter.
// Returns empty strings if no frontmatter is found.
func parseFrontmatterFields(content string) (name, description string) {
	if !strings.HasPrefix(strings.TrimSpace(content), "---") {
		return "", ""
	}

	scanner := bufio.NewScanner(strings.NewReader(content))

	// Skip leading whitespace lines and the opening ---.
	foundOpen := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			if foundOpen {
				// Closing delimiter — done.
				break
			}
			foundOpen = true
			continue
		}
		if !foundOpen {
			continue
		}

		// Simple YAML key: value parsing.
		parts := strings.SplitN(scanner.Text(), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Remove surrounding quotes if present.
		value = strings.Trim(value, "\"'")

		switch key {
		case "name":
			name = value
		case "description":
			description = value
		}
	}

	return name, description
}

// deriveNameFromURL extracts a skill name from the last path segment of a URL.
func deriveNameFromURL(rawURL string) string {
	// Get the path component.
	urlPath := rawURL
	if idx := strings.Index(rawURL, "://"); idx != -1 {
		urlPath = rawURL[idx+3:]
	}
	if idx := strings.Index(urlPath, "/"); idx != -1 {
		urlPath = urlPath[idx:]
	}

	// Get the last segment.
	base := path.Base(urlPath)

	// Remove file extension.
	if ext := path.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}

	return sanitizeToSkillName(base)
}

// sanitizeToSkillName converts a string to a valid skill name.
func sanitizeToSkillName(s string) string {
	s = strings.ToLower(s)

	// Replace underscores and spaces with hyphens.
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")

	// Remove any characters that aren't lowercase alphanumeric or hyphens.
	var result strings.Builder
	for _, ch := range s {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			result.WriteRune(ch)
		}
	}
	s = result.String()

	// Remove leading hyphens.
	s = strings.TrimLeft(s, "-")

	// Collapse multiple hyphens.
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}

	// Truncate to max length.
	if len(s) > maxSkillNameLength {
		s = s[:maxSkillNameLength]
	}

	// Remove trailing hyphens after truncation.
	s = strings.TrimRight(s, "-")

	return s
}
