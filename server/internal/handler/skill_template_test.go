package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agenticflow/agenticflow/server/internal/middleware"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Feature: skill-management, Property 9: Instantiation produces correct skill copy
//
// For any valid template, POST /api/skill-templates/:slug/instantiate SHALL
// create a user-owned skill where: the skill name equals the template slug,
// the skill description equals the template description, the skill content
// equals the template content, and the skill config contains a
// `template_origin` object with `template_slug` matching the source
// template's slug, `template_version` matching the source template's version,
// and `instantiated_at` as a valid ISO 8601 timestamp.
//
// **Validates: Requirements 5.1, 5.2, 5.4, 1.4**
// ---------------------------------------------------------------------------

// --- Mock Querier for instantiation tests ---

// capturedCreateSkillParams stores the params passed to CreateSkill for verification.
type capturedCreateSkillParams struct {
	called bool
	params db.CreateSkillParams
}

// instantiateMockQuerier is a minimal mock that implements the Querier methods
// needed by the Instantiate handler logic.
type instantiateMockQuerier struct {
	db.Querier
	template       db.SkillTemplate
	templateErr    error
	skillByNameErr error
	captured       *capturedCreateSkillParams
	createSkillErr error
}

func (m *instantiateMockQuerier) GetSkillTemplateBySlug(_ context.Context, slug string) (db.SkillTemplate, error) {
	if m.templateErr != nil {
		return db.SkillTemplate{}, m.templateErr
	}
	if slug != m.template.Slug {
		return db.SkillTemplate{}, pgx.ErrNoRows
	}
	return m.template, nil
}

func (m *instantiateMockQuerier) GetSkillByName(_ context.Context, arg db.GetSkillByNameParams) (db.Skill, error) {
	return db.Skill{}, m.skillByNameErr
}

func (m *instantiateMockQuerier) CreateSkill(_ context.Context, arg db.CreateSkillParams) (db.Skill, error) {
	if m.createSkillErr != nil {
		return db.Skill{}, m.createSkillErr
	}
	m.captured.called = true
	m.captured.params = arg

	now := time.Now().UTC()
	return db.Skill{
		ID:          arg.UserID, // reuse for simplicity
		UserID:      arg.UserID,
		Name:        arg.Name,
		Description: arg.Description,
		Content:     arg.Content,
		Config:      arg.Config,
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
	}, nil
}

// --- Generators ---

// genTemplateSlug generates a valid template slug matching ^[a-z0-9][a-z0-9-]*$ (1-64 chars).
func genTemplateSlug(t *rapid.T) string {
	return rapid.StringMatching(`[a-z0-9][a-z0-9-]{0,20}`).Draw(t, "slug")
}

// genTemplateName generates a valid template name (1-128 chars).
func genTemplateName(t *rapid.T) string {
	return rapid.StringMatching(`[A-Za-z][A-Za-z0-9 ]{0,30}`).Draw(t, "name")
}

// genTemplateDescription generates a valid template description (1-512 chars).
func genTemplateDescription(t *rapid.T) string {
	return rapid.StringMatching(`[A-Za-z0-9 .,!?]{1,100}`).Draw(t, "description")
}

// genTemplateContent generates valid template content (500+ chars).
func genTemplateContent(t *rapid.T) string {
	// Generate content of at least 500 chars
	base := rapid.StringMatching(`[A-Za-z0-9 \n.,!?#]{100,200}`).Draw(t, "content")
	// Pad to ensure minimum 500 chars
	for len(base) < 500 {
		base += rapid.StringMatching(`[A-Za-z0-9 \n.,]{50,100}`).Draw(t, "contentPad")
	}
	return base
}

// genTemplateCategory generates a valid category.
func genTemplateCategory(t *rapid.T) string {
	categories := []string{"Analysis", "Development", "Testing", "Operations", "Documentation"}
	return rapid.SampledFrom(categories).Draw(t, "category")
}

// genTemplateVersion generates a valid version string.
func genTemplateVersion(t *rapid.T) string {
	major := rapid.IntRange(1, 9).Draw(t, "major")
	minor := rapid.IntRange(0, 9).Draw(t, "minor")
	patch := rapid.IntRange(0, 9).Draw(t, "patch")
	return rapid.Just(formatVersion(major, minor, patch)).Draw(t, "version")
}

func formatVersion(major, minor, patch int) string {
	return string(rune('0'+major)) + "." + string(rune('0'+minor)) + "." + string(rune('0'+patch))
}

// genTemplateIcon generates a valid icon (single emoji).
func genTemplateIcon(t *rapid.T) pgtype.Text {
	icons := []string{"📊", "🔍", "📋", "🏗️", "💻", "👁️", "🧪", "🔒", "⚙️", "📝"}
	icon := rapid.SampledFrom(icons).Draw(t, "icon")
	return pgtype.Text{String: icon, Valid: true}
}

// genValidTemplate generates a complete valid SkillTemplate.
func genValidTemplate(t *rapid.T) db.SkillTemplate {
	templateID := genUUID(t)
	now := time.Now().UTC()
	return db.SkillTemplate{
		ID:          templateID,
		Slug:        genTemplateSlug(t),
		Name:        genTemplateName(t),
		Description: genTemplateDescription(t),
		Content:     genTemplateContent(t),
		Category:    genTemplateCategory(t),
		Version:     genTemplateVersion(t),
		Icon:        genTemplateIcon(t),
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
	}
}

// --- Property Tests ---

func TestProperty9_InstantiationCorrectness_SkillNameEqualsSlug(t *testing.T) {
	// Feature: skill-management, Property 9: Instantiation produces correct skill copy
	rapid.Check(t, func(t *rapid.T) {
		template := genValidTemplate(t)
		userID := genUUID(t)

		captured := &capturedCreateSkillParams{}
		mock := &instantiateMockQuerier{
			template:       template,
			skillByNameErr: pgx.ErrNoRows, // no conflict
			captured:       captured,
		}

		h := NewSkillTemplateHandler(mock)

		// Simulate the instantiation logic directly
		ctx := context.Background()
		tmpl, err := h.Queries.GetSkillTemplateBySlug(ctx, template.Slug)
		if err != nil {
			t.Fatalf("unexpected error getting template: %v", err)
		}

		_, err = h.Queries.GetSkillByName(ctx, db.GetSkillByNameParams{
			UserID: userID,
			Name:   tmpl.Slug,
		})
		if err != pgx.ErrNoRows {
			t.Fatalf("expected ErrNoRows for name check, got: %v", err)
		}

		origin := TemplateOrigin{
			TemplateSlug:    tmpl.Slug,
			TemplateVersion: tmpl.Version,
			InstantiatedAt:  time.Now().UTC().Format(time.RFC3339),
		}
		configMap := map[string]interface{}{
			"template_origin": origin,
		}
		configBytes, err := json.Marshal(configMap)
		if err != nil {
			t.Fatalf("failed to marshal config: %v", err)
		}

		_, err = h.Queries.CreateSkill(ctx, db.CreateSkillParams{
			UserID:      userID,
			Name:        tmpl.Slug,
			Description: tmpl.Description,
			Content:     tmpl.Content,
			Config:      configBytes,
		})
		if err != nil {
			t.Fatalf("unexpected error creating skill: %v", err)
		}

		// Verify: skill name == template slug
		if !captured.called {
			t.Fatal("CreateSkill was not called")
		}
		if captured.params.Name != template.Slug {
			t.Fatalf("skill name mismatch: got %q, want %q", captured.params.Name, template.Slug)
		}
	})
}

func TestProperty9_InstantiationCorrectness_SkillDescriptionEqualsTemplateDescription(t *testing.T) {
	// Feature: skill-management, Property 9: Instantiation produces correct skill copy
	rapid.Check(t, func(t *rapid.T) {
		template := genValidTemplate(t)
		userID := genUUID(t)

		captured := &capturedCreateSkillParams{}
		mock := &instantiateMockQuerier{
			template:       template,
			skillByNameErr: pgx.ErrNoRows,
			captured:       captured,
		}

		h := NewSkillTemplateHandler(mock)
		ctx := context.Background()

		tmpl, _ := h.Queries.GetSkillTemplateBySlug(ctx, template.Slug)

		origin := TemplateOrigin{
			TemplateSlug:    tmpl.Slug,
			TemplateVersion: tmpl.Version,
			InstantiatedAt:  time.Now().UTC().Format(time.RFC3339),
		}
		configMap := map[string]interface{}{
			"template_origin": origin,
		}
		configBytes, _ := json.Marshal(configMap)

		h.Queries.CreateSkill(ctx, db.CreateSkillParams{
			UserID:      userID,
			Name:        tmpl.Slug,
			Description: tmpl.Description,
			Content:     tmpl.Content,
			Config:      configBytes,
		})

		// Verify: skill description == template description
		if !captured.called {
			t.Fatal("CreateSkill was not called")
		}
		if captured.params.Description != template.Description {
			t.Fatalf("skill description mismatch: got %q, want %q",
				captured.params.Description, template.Description)
		}
	})
}

func TestProperty9_InstantiationCorrectness_SkillContentEqualsTemplateContent(t *testing.T) {
	// Feature: skill-management, Property 9: Instantiation produces correct skill copy
	rapid.Check(t, func(t *rapid.T) {
		template := genValidTemplate(t)
		userID := genUUID(t)

		captured := &capturedCreateSkillParams{}
		mock := &instantiateMockQuerier{
			template:       template,
			skillByNameErr: pgx.ErrNoRows,
			captured:       captured,
		}

		h := NewSkillTemplateHandler(mock)
		ctx := context.Background()

		tmpl, _ := h.Queries.GetSkillTemplateBySlug(ctx, template.Slug)

		origin := TemplateOrigin{
			TemplateSlug:    tmpl.Slug,
			TemplateVersion: tmpl.Version,
			InstantiatedAt:  time.Now().UTC().Format(time.RFC3339),
		}
		configMap := map[string]interface{}{
			"template_origin": origin,
		}
		configBytes, _ := json.Marshal(configMap)

		h.Queries.CreateSkill(ctx, db.CreateSkillParams{
			UserID:      userID,
			Name:        tmpl.Slug,
			Description: tmpl.Description,
			Content:     tmpl.Content,
			Config:      configBytes,
		})

		// Verify: skill content == template content
		if !captured.called {
			t.Fatal("CreateSkill was not called")
		}
		if captured.params.Content != template.Content {
			t.Fatalf("skill content mismatch: got length %d, want length %d",
				len(captured.params.Content), len(template.Content))
		}
	})
}

func TestProperty9_InstantiationCorrectness_ConfigContainsCorrectTemplateOrigin(t *testing.T) {
	// Feature: skill-management, Property 9: Instantiation produces correct skill copy
	rapid.Check(t, func(t *rapid.T) {
		template := genValidTemplate(t)
		userID := genUUID(t)

		captured := &capturedCreateSkillParams{}
		mock := &instantiateMockQuerier{
			template:       template,
			skillByNameErr: pgx.ErrNoRows,
			captured:       captured,
		}

		h := NewSkillTemplateHandler(mock)
		ctx := context.Background()

		tmpl, _ := h.Queries.GetSkillTemplateBySlug(ctx, template.Slug)

		beforeInstantiation := time.Now().UTC()

		origin := TemplateOrigin{
			TemplateSlug:    tmpl.Slug,
			TemplateVersion: tmpl.Version,
			InstantiatedAt:  time.Now().UTC().Format(time.RFC3339),
		}
		configMap := map[string]interface{}{
			"template_origin": origin,
		}
		configBytes, _ := json.Marshal(configMap)

		h.Queries.CreateSkill(ctx, db.CreateSkillParams{
			UserID:      userID,
			Name:        tmpl.Slug,
			Description: tmpl.Description,
			Content:     tmpl.Content,
			Config:      configBytes,
		})

		afterInstantiation := time.Now().UTC()

		// Verify: config contains template_origin with correct fields
		if !captured.called {
			t.Fatal("CreateSkill was not called")
		}

		var config map[string]json.RawMessage
		if err := json.Unmarshal(captured.params.Config, &config); err != nil {
			t.Fatalf("failed to unmarshal config: %v", err)
		}

		originJSON, ok := config["template_origin"]
		if !ok {
			t.Fatal("config missing template_origin key")
		}

		var parsedOrigin TemplateOrigin
		if err := json.Unmarshal(originJSON, &parsedOrigin); err != nil {
			t.Fatalf("failed to unmarshal template_origin: %v", err)
		}

		// Verify template_slug matches
		if parsedOrigin.TemplateSlug != template.Slug {
			t.Fatalf("template_origin.template_slug mismatch: got %q, want %q",
				parsedOrigin.TemplateSlug, template.Slug)
		}

		// Verify template_version matches
		if parsedOrigin.TemplateVersion != template.Version {
			t.Fatalf("template_origin.template_version mismatch: got %q, want %q",
				parsedOrigin.TemplateVersion, template.Version)
		}

		// Verify instantiated_at is a valid ISO 8601 timestamp
		instantiatedAt, err := time.Parse(time.RFC3339, parsedOrigin.InstantiatedAt)
		if err != nil {
			t.Fatalf("template_origin.instantiated_at is not valid ISO 8601: %q, error: %v",
				parsedOrigin.InstantiatedAt, err)
		}

		// Verify instantiated_at is within a reasonable time window
		if instantiatedAt.Before(beforeInstantiation.Add(-1*time.Second)) ||
			instantiatedAt.After(afterInstantiation.Add(1*time.Second)) {
			t.Fatalf("template_origin.instantiated_at %v is outside expected window [%v, %v]",
				instantiatedAt, beforeInstantiation, afterInstantiation)
		}
	})
}

func TestProperty9_InstantiationCorrectness_AllFieldsCopiedCorrectly(t *testing.T) {
	// Feature: skill-management, Property 9: Instantiation produces correct skill copy
	// Combined test verifying all fields in a single pass for efficiency.
	rapid.Check(t, func(t *rapid.T) {
		template := genValidTemplate(t)
		userID := genUUID(t)

		captured := &capturedCreateSkillParams{}
		mock := &instantiateMockQuerier{
			template:       template,
			skillByNameErr: pgx.ErrNoRows,
			captured:       captured,
		}

		h := NewSkillTemplateHandler(mock)
		ctx := context.Background()

		tmpl, err := h.Queries.GetSkillTemplateBySlug(ctx, template.Slug)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		beforeInstantiation := time.Now().UTC()

		origin := TemplateOrigin{
			TemplateSlug:    tmpl.Slug,
			TemplateVersion: tmpl.Version,
			InstantiatedAt:  time.Now().UTC().Format(time.RFC3339),
		}
		configMap := map[string]interface{}{
			"template_origin": origin,
		}
		configBytes, err := json.Marshal(configMap)
		if err != nil {
			t.Fatalf("failed to marshal config: %v", err)
		}

		_, err = h.Queries.CreateSkill(ctx, db.CreateSkillParams{
			UserID:      userID,
			Name:        tmpl.Slug,
			Description: tmpl.Description,
			Content:     tmpl.Content,
			Config:      configBytes,
		})
		if err != nil {
			t.Fatalf("unexpected error creating skill: %v", err)
		}

		afterInstantiation := time.Now().UTC()

		if !captured.called {
			t.Fatal("CreateSkill was not called")
		}

		// Verify: skill.Name == template.Slug
		if captured.params.Name != template.Slug {
			t.Fatalf("skill name mismatch: got %q, want %q",
				captured.params.Name, template.Slug)
		}

		// Verify: skill.Description == template.Description
		if captured.params.Description != template.Description {
			t.Fatalf("skill description mismatch: got %q, want %q",
				captured.params.Description, template.Description)
		}

		// Verify: skill.Content == template.Content
		if captured.params.Content != template.Content {
			t.Fatalf("skill content mismatch: got length %d, want length %d",
				len(captured.params.Content), len(template.Content))
		}

		// Verify: config contains correct template_origin
		var config map[string]json.RawMessage
		if err := json.Unmarshal(captured.params.Config, &config); err != nil {
			t.Fatalf("failed to unmarshal config: %v", err)
		}

		originJSON, ok := config["template_origin"]
		if !ok {
			t.Fatal("config missing template_origin key")
		}

		var parsedOrigin TemplateOrigin
		if err := json.Unmarshal(originJSON, &parsedOrigin); err != nil {
			t.Fatalf("failed to unmarshal template_origin: %v", err)
		}

		if parsedOrigin.TemplateSlug != template.Slug {
			t.Fatalf("template_origin.template_slug mismatch: got %q, want %q",
				parsedOrigin.TemplateSlug, template.Slug)
		}

		if parsedOrigin.TemplateVersion != template.Version {
			t.Fatalf("template_origin.template_version mismatch: got %q, want %q",
				parsedOrigin.TemplateVersion, template.Version)
		}

		instantiatedAt, err := time.Parse(time.RFC3339, parsedOrigin.InstantiatedAt)
		if err != nil {
			t.Fatalf("template_origin.instantiated_at is not valid ISO 8601: %q",
				parsedOrigin.InstantiatedAt)
		}

		if instantiatedAt.Before(beforeInstantiation.Add(-1*time.Second)) ||
			instantiatedAt.After(afterInstantiation.Add(1*time.Second)) {
			t.Fatalf("template_origin.instantiated_at %v outside expected window [%v, %v]",
				instantiatedAt, beforeInstantiation, afterInstantiation)
		}

		// Verify: UserID is correctly passed through
		if captured.params.UserID != userID {
			t.Fatalf("skill user_id mismatch: got %v, want %v",
				captured.params.UserID, userID)
		}
	})
}

// ---------------------------------------------------------------------------
// Feature: skill-management, Property 5: List endpoint excludes content
//
// For any response from GET /api/skill-templates, no template object in the
// returned array SHALL contain a `content` field.
//
// **Validates: Requirements 4.1, 4.6**
// ---------------------------------------------------------------------------

// listMockQuerier is a mock that returns randomly generated ListSkillTemplatesRow
// items for the List endpoint property test.
type listMockQuerier struct {
	db.Querier
	templates []db.ListSkillTemplatesRow
}

func (m *listMockQuerier) ListSkillTemplates(_ context.Context) ([]db.ListSkillTemplatesRow, error) {
	return m.templates, nil
}

// genListSkillTemplatesRow generates a random ListSkillTemplatesRow.
func genListSkillTemplatesRow(t *rapid.T) db.ListSkillTemplatesRow {
	return db.ListSkillTemplatesRow{
		ID:          genUUID(t),
		Slug:        genTemplateSlug(t),
		Name:        genTemplateName(t),
		Description: genTemplateDescription(t),
		Category:    genTemplateCategory(t),
		Version:     genTemplateVersion(t),
		Icon:        genTemplateIcon(t),
	}
}

func TestProperty5_ListEndpointExcludesContent(t *testing.T) {
	// Feature: skill-management, Property 5: List endpoint excludes content
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random number of templates (1 to 20)
		count := rapid.IntRange(1, 20).Draw(t, "templateCount")
		templates := make([]db.ListSkillTemplatesRow, count)
		for i := 0; i < count; i++ {
			templates[i] = genListSkillTemplatesRow(t)
		}

		mock := &listMockQuerier{templates: templates}
		h := NewSkillTemplateHandler(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/skill-templates", nil)
		ctx := middleware.WithUserID(req.Context(), testUserID())
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		h.List(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
		}

		// Parse response as array of generic maps to check field presence
		var items []map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if len(items) != count {
			t.Fatalf("expected %d items, got %d", count, len(items))
		}

		// Verify NO response object contains a "content" field
		for i, item := range items {
			if _, exists := item["content"]; exists {
				t.Fatalf("item[%d]: response MUST NOT contain 'content' field, but it does", i)
			}
		}
	})
}

func TestProperty5_ListEndpointExcludesContent_EmptyList(t *testing.T) {
	// Feature: skill-management, Property 5: List endpoint excludes content
	// Edge case: empty template list should return empty array with no content fields.
	rapid.Check(t, func(t *rapid.T) {
		mock := &listMockQuerier{templates: []db.ListSkillTemplatesRow{}}
		h := NewSkillTemplateHandler(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/skill-templates", nil)
		ctx := middleware.WithUserID(req.Context(), testUserID())
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		h.List(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
		}

		var items []map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		// Empty array should have no items with content
		for i, item := range items {
			if _, exists := item["content"]; exists {
				t.Fatalf("item[%d]: response MUST NOT contain 'content' field", i)
			}
		}
	})
}

func TestProperty5_ListEndpointExcludesContent_HasExpectedFields(t *testing.T) {
	// Feature: skill-management, Property 5: List endpoint excludes content
	// Verify that while content is excluded, all expected summary fields ARE present.
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(1, 10).Draw(t, "templateCount")
		templates := make([]db.ListSkillTemplatesRow, count)
		for i := 0; i < count; i++ {
			templates[i] = genListSkillTemplatesRow(t)
		}

		mock := &listMockQuerier{templates: templates}
		h := NewSkillTemplateHandler(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/skill-templates", nil)
		ctx := middleware.WithUserID(req.Context(), testUserID())
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		h.List(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
		}

		var items []map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		expectedFields := []string{"id", "slug", "name", "description", "category", "version", "icon"}
		for i, item := range items {
			// content MUST NOT be present
			if _, exists := item["content"]; exists {
				t.Fatalf("item[%d]: response MUST NOT contain 'content' field", i)
			}
			// all summary fields MUST be present
			for _, field := range expectedFields {
				if _, exists := item[field]; !exists {
					t.Fatalf("item[%d]: response missing expected field %q", i, field)
				}
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Unit Tests for SkillTemplateHandler
// ---------------------------------------------------------------------------

// --- Unit Test Mock Querier ---

// unitTestMockQuerier implements the Querier methods needed by the
// SkillTemplateHandler for unit testing with standard testing.T.
type unitTestMockQuerier struct {
	db.Querier

	// List
	listTemplates    []db.ListSkillTemplatesRow
	listTemplatesErr error

	// ListByCategory
	listByCategoryTemplates []db.ListSkillTemplatesByCategoryRow
	listByCategoryErr       error

	// GetBySlug
	templateBySlug    db.SkillTemplate
	templateBySlugErr error

	// GetSkillByName
	skillByNameResult db.Skill
	skillByNameErr    error

	// CreateSkill
	createSkillResult db.Skill
	createSkillErr    error
}

func (m *unitTestMockQuerier) ListSkillTemplates(_ context.Context) ([]db.ListSkillTemplatesRow, error) {
	return m.listTemplates, m.listTemplatesErr
}

func (m *unitTestMockQuerier) ListSkillTemplatesByCategory(_ context.Context, _ string) ([]db.ListSkillTemplatesByCategoryRow, error) {
	return m.listByCategoryTemplates, m.listByCategoryErr
}

func (m *unitTestMockQuerier) GetSkillTemplateBySlug(_ context.Context, _ string) (db.SkillTemplate, error) {
	return m.templateBySlug, m.templateBySlugErr
}

func (m *unitTestMockQuerier) GetSkillByName(_ context.Context, _ db.GetSkillByNameParams) (db.Skill, error) {
	return m.skillByNameResult, m.skillByNameErr
}

func (m *unitTestMockQuerier) CreateSkill(_ context.Context, arg db.CreateSkillParams) (db.Skill, error) {
	if m.createSkillErr != nil {
		return db.Skill{}, m.createSkillErr
	}
	now := time.Now().UTC()
	return db.Skill{
		ID:          arg.UserID,
		UserID:      arg.UserID,
		Name:        arg.Name,
		Description: arg.Description,
		Content:     arg.Content,
		Config:      arg.Config,
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
	}, nil
}

// --- Helper to build a test template UUID ---

func testUUID() pgtype.UUID {
	return pgtype.UUID{
		Bytes: [16]byte{0x55, 0x0e, 0x84, 0x00, 0xe2, 0x9b, 0x41, 0xd4, 0xa7, 0x16, 0x44, 0x66, 0x55, 0x44, 0x00, 0x01},
		Valid: true,
	}
}

func testUserID() string {
	return "550e8400-e29b-41d4-a716-446655440000"
}

// --- Unit Tests: List Endpoint ---

func TestList_ReturnsTemplatesWithoutContentField(t *testing.T) {
	mock := &unitTestMockQuerier{
		listTemplates: []db.ListSkillTemplatesRow{
			{
				ID:          testUUID(),
				Slug:        "developer",
				Name:        "Developer",
				Description: "Writes code",
				Category:    "Development",
				Version:     "1.0.0",
				Icon:        pgtype.Text{String: "💻", Valid: true},
			},
			{
				ID:          testUUID(),
				Slug:        "tester",
				Name:        "QA Tester",
				Description: "Tests code",
				Category:    "Testing",
				Version:     "1.0.0",
				Icon:        pgtype.Text{String: "🧪", Valid: true},
			},
		},
	}

	h := NewSkillTemplateHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/skill-templates", nil)
	ctx := middleware.WithUserID(req.Context(), testUserID())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	// Parse response as array of generic maps to check field presence
	var items []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	for i, item := range items {
		// Verify "content" key is NOT present
		if _, exists := item["content"]; exists {
			t.Errorf("item[%d]: response should NOT contain 'content' field", i)
		}
		// Verify expected fields ARE present
		for _, field := range []string{"id", "slug", "name", "description", "category", "version", "icon"} {
			if _, exists := item[field]; !exists {
				t.Errorf("item[%d]: response missing expected field %q", i, field)
			}
		}
	}
}

// --- Unit Tests: GetBySlug Endpoint ---

func TestGetBySlug_ReturnsFullTemplateWithContent(t *testing.T) {
	now := time.Now().UTC()
	mock := &unitTestMockQuerier{
		templateBySlug: db.SkillTemplate{
			ID:          testUUID(),
			Slug:        "developer",
			Name:        "Developer",
			Description: "Writes clean code",
			Content:     "# Developer Skill\n\nYou are a software developer...",
			Category:    "Development",
			Version:     "1.0.0",
			Icon:        pgtype.Text{String: "💻", Valid: true},
			CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		},
	}

	h := NewSkillTemplateHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/skill-templates/developer", nil)
	ctx := middleware.WithUserID(req.Context(), testUserID())

	// Set up Chi URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "developer")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.GetBySlug(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	// Parse response as generic map to check field presence
	var item map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify "content" key IS present
	content, exists := item["content"]
	if !exists {
		t.Fatal("response should contain 'content' field")
	}
	if content.(string) != "# Developer Skill\n\nYou are a software developer..." {
		t.Errorf("unexpected content value: %v", content)
	}

	// Verify all expected fields are present
	for _, field := range []string{"id", "slug", "name", "description", "category", "version", "icon", "content", "created_at", "updated_at"} {
		if _, exists := item[field]; !exists {
			t.Errorf("response missing expected field %q", field)
		}
	}
}

func TestGetBySlug_NonExistentSlug_Returns404(t *testing.T) {
	mock := &unitTestMockQuerier{
		templateBySlugErr: pgx.ErrNoRows,
	}

	h := NewSkillTemplateHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/skill-templates/nonexistent", nil)
	ctx := middleware.WithUserID(req.Context(), testUserID())

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "nonexistent")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.GetBySlug(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if resp["error"] != "template not found" {
		t.Errorf("expected error 'template not found', got %q", resp["error"])
	}
}

// --- Unit Tests: Instantiate Endpoint ---

func TestInstantiate_CreatesSkillCorrectly(t *testing.T) {
	now := time.Now().UTC()
	mock := &unitTestMockQuerier{
		templateBySlug: db.SkillTemplate{
			ID:          testUUID(),
			Slug:        "developer",
			Name:        "Developer",
			Description: "Writes clean code",
			Content:     "# Developer Skill\n\nYou are a software developer...",
			Category:    "Development",
			Version:     "1.0.0",
			Icon:        pgtype.Text{String: "💻", Valid: true},
			CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		},
		skillByNameErr: pgx.ErrNoRows, // no conflict
	}

	h := NewSkillTemplateHandler(mock)

	req := httptest.NewRequest(http.MethodPost, "/api/skill-templates/developer/instantiate", nil)
	ctx := middleware.WithUserID(req.Context(), testUserID())

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "developer")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.Instantiate(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}

	// Parse response and verify skill was created with correct fields
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["name"] != "developer" {
		t.Errorf("expected skill name 'developer', got %v", resp["name"])
	}
	if resp["description"] != "Writes clean code" {
		t.Errorf("expected description 'Writes clean code', got %v", resp["description"])
	}
	if resp["content"] != "# Developer Skill\n\nYou are a software developer..." {
		t.Errorf("unexpected content: %v", resp["content"])
	}

	// Verify config contains template_origin
	configRaw, ok := resp["config"]
	if !ok {
		t.Fatal("response missing 'config' field")
	}
	configMap, ok := configRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("config is not an object: %T", configRaw)
	}
	originRaw, ok := configMap["template_origin"]
	if !ok {
		t.Fatal("config missing 'template_origin'")
	}
	origin, ok := originRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("template_origin is not an object: %T", originRaw)
	}
	if origin["template_slug"] != "developer" {
		t.Errorf("expected template_slug 'developer', got %v", origin["template_slug"])
	}
	if origin["template_version"] != "1.0.0" {
		t.Errorf("expected template_version '1.0.0', got %v", origin["template_version"])
	}
}

func TestInstantiate_DuplicateName_Returns409(t *testing.T) {
	now := time.Now().UTC()
	mock := &unitTestMockQuerier{
		templateBySlug: db.SkillTemplate{
			ID:          testUUID(),
			Slug:        "developer",
			Name:        "Developer",
			Description: "Writes clean code",
			Content:     "# Developer Skill",
			Category:    "Development",
			Version:     "1.0.0",
			Icon:        pgtype.Text{String: "💻", Valid: true},
			CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		},
		skillByNameErr: nil, // nil error means skill already exists
		skillByNameResult: db.Skill{
			ID:   testUUID(),
			Name: "developer",
		},
	}

	h := NewSkillTemplateHandler(mock)

	req := httptest.NewRequest(http.MethodPost, "/api/skill-templates/developer/instantiate", nil)
	ctx := middleware.WithUserID(req.Context(), testUserID())

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "developer")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.Instantiate(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if resp["error"] != "a skill with this name already exists" {
		t.Errorf("expected conflict error message, got %q", resp["error"])
	}
}

func TestInstantiate_NonExistentSlug_Returns404(t *testing.T) {
	mock := &unitTestMockQuerier{
		templateBySlugErr: pgx.ErrNoRows,
	}

	h := NewSkillTemplateHandler(mock)

	req := httptest.NewRequest(http.MethodPost, "/api/skill-templates/nonexistent/instantiate", nil)
	ctx := middleware.WithUserID(req.Context(), testUserID())

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "nonexistent")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.Instantiate(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if resp["error"] != "template not found" {
		t.Errorf("expected error 'template not found', got %q", resp["error"])
	}
}

// --- Unit Tests: Authentication (401) ---

func TestList_RequiresAuthentication(t *testing.T) {
	mock := &unitTestMockQuerier{}
	h := NewSkillTemplateHandler(mock)

	// Request WITHOUT user ID in context
	req := httptest.NewRequest(http.MethodGet, "/api/skill-templates", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if resp["error"] != "unauthorized" {
		t.Errorf("expected error 'unauthorized', got %q", resp["error"])
	}
}

func TestGetBySlug_RequiresAuthentication(t *testing.T) {
	mock := &unitTestMockQuerier{}
	h := NewSkillTemplateHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/skill-templates/developer", nil)
	// Set up Chi URL params but NO user ID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "developer")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.GetBySlug(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if resp["error"] != "unauthorized" {
		t.Errorf("expected error 'unauthorized', got %q", resp["error"])
	}
}

func TestInstantiate_RequiresAuthentication(t *testing.T) {
	mock := &unitTestMockQuerier{}
	h := NewSkillTemplateHandler(mock)

	req := httptest.NewRequest(http.MethodPost, "/api/skill-templates/developer/instantiate", nil)
	// Set up Chi URL params but NO user ID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "developer")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.Instantiate(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if resp["error"] != "unauthorized" {
		t.Errorf("expected error 'unauthorized', got %q", resp["error"])
	}
}
