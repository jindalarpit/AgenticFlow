package seed

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"testing"
	"time"

	"pgregory.net/rapid"

	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// Feature: skill-management, Property 3: Seeding idempotence
//
// For any built-in template already present in the database at the same version,
// running the seeding process SHALL NOT modify the template record (updated_at
// remains unchanged).
//
// **Validates: Requirements 3.3**

// fakeQuerier is a minimal fake implementation of db.Querier that simulates
// the UpsertSkillTemplate SQL behavior: it only updates a record when the
// version differs from the existing one (matching the WHERE clause
// `skill_template.version <> EXCLUDED.version`).
type fakeQuerier struct {
	db.Querier // embed interface to satisfy all methods (panics on unimplemented)

	mu      sync.Mutex
	records map[string]*templateRecord // keyed by slug
	// upsertCalls tracks every call to UpsertSkillTemplate for assertion purposes.
	upsertCalls []db.UpsertSkillTemplateParams
	// updateCount tracks how many times a record was actually modified.
	updateCount int
}

type templateRecord struct {
	params    db.UpsertSkillTemplateParams
	createdAt time.Time
	updatedAt time.Time
}

func newFakeQuerier() *fakeQuerier {
	return &fakeQuerier{
		records: make(map[string]*templateRecord),
	}
}

// UpsertSkillTemplate simulates the PostgreSQL upsert with version check:
// INSERT ... ON CONFLICT (slug) DO UPDATE ... WHERE skill_template.version <> EXCLUDED.version
func (f *fakeQuerier) UpsertSkillTemplate(_ context.Context, arg db.UpsertSkillTemplateParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.upsertCalls = append(f.upsertCalls, arg)

	existing, exists := f.records[arg.Slug]
	if !exists {
		// INSERT path: new record
		now := time.Now()
		f.records[arg.Slug] = &templateRecord{
			params:    arg,
			createdAt: now,
			updatedAt: now,
		}
		f.updateCount++
		return nil
	}

	// ON CONFLICT path: only update if version differs
	if existing.params.Version != arg.Version {
		existing.params = arg
		existing.updatedAt = time.Now()
		f.updateCount++
	}
	// If version matches, do nothing (idempotent skip)

	return nil
}

func TestProperty3_SeedingIdempotence(t *testing.T) {
	// Feature: skill-management, Property 3: Seeding idempotence
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random set of templates with various versions
		numTemplates := rapid.IntRange(1, 20).Draw(t, "numTemplates")
		templates := make([]BuiltinTemplate, numTemplates)

		for i := range templates {
			templates[i] = BuiltinTemplate{
				Slug:        rapid.StringMatching(`^[a-z][a-z0-9-]{2,20}$`).Draw(t, "slug"),
				Name:        rapid.StringMatching(`^[A-Z][a-zA-Z ]{2,30}$`).Draw(t, "name"),
				Description: rapid.StringMatching(`^[a-zA-Z0-9 .,]{10,100}$`).Draw(t, "description"),
				Content:     rapid.StringMatching(`^[a-zA-Z0-9 \n.,#]{50,200}$`).Draw(t, "content"),
				Category:    rapid.SampledFrom([]string{"Analysis", "Development", "Testing", "Operations", "Documentation"}).Draw(t, "category"),
				Version:     rapid.SampledFrom([]string{"1.0.0", "1.1.0", "2.0.0", "1.0.1", "3.0.0"}).Draw(t, "version"),
				Icon:        rapid.SampledFrom([]string{"📊", "💻", "🧪", "⚙️", "📝"}).Draw(t, "icon"),
			}
		}

		// Temporarily replace BuiltinTemplates for seeding
		originalTemplates := BuiltinTemplates
		BuiltinTemplates = templates
		defer func() { BuiltinTemplates = originalTemplates }()

		ctx := context.Background()
		fq := newFakeQuerier()

		// First seed: inserts all templates
		SeedTemplates(ctx, fq)

		// Record the updated_at timestamps after first seed
		type snapshot struct {
			updatedAt time.Time
			params    db.UpsertSkillTemplateParams
		}
		firstSeedSnapshots := make(map[string]snapshot)
		fq.mu.Lock()
		for slug, rec := range fq.records {
			firstSeedSnapshots[slug] = snapshot{
				updatedAt: rec.updatedAt,
				params:    rec.params,
			}
		}
		firstUpdateCount := fq.updateCount
		fq.mu.Unlock()

		// Small delay to ensure time.Now() would differ if updated_at were changed
		time.Sleep(1 * time.Millisecond)

		// Reset the update counter to track only second-seed modifications
		fq.mu.Lock()
		fq.updateCount = 0
		fq.mu.Unlock()

		// Second seed with SAME templates (same versions)
		SeedTemplates(ctx, fq)

		// Verify: no records were modified on the second seed
		fq.mu.Lock()
		secondUpdateCount := fq.updateCount
		for slug, rec := range fq.records {
			firstSnap, exists := firstSeedSnapshots[slug]
			if !exists {
				continue
			}
			// updated_at must remain unchanged
			if !rec.updatedAt.Equal(firstSnap.updatedAt) {
				t.Fatalf("template %q: updated_at changed on second seed (was %v, now %v) despite same version",
					slug, firstSnap.updatedAt, rec.updatedAt)
			}
			// All fields must remain unchanged
			if rec.params.Name != firstSnap.params.Name {
				t.Fatalf("template %q: name changed on second seed", slug)
			}
			if rec.params.Description != firstSnap.params.Description {
				t.Fatalf("template %q: description changed on second seed", slug)
			}
			if rec.params.Content != firstSnap.params.Content {
				t.Fatalf("template %q: content changed on second seed", slug)
			}
			if rec.params.Category != firstSnap.params.Category {
				t.Fatalf("template %q: category changed on second seed", slug)
			}
			if rec.params.Version != firstSnap.params.Version {
				t.Fatalf("template %q: version changed on second seed", slug)
			}
		}
		fq.mu.Unlock()

		// The second seed should have zero actual updates
		if secondUpdateCount != 0 {
			t.Fatalf("expected 0 updates on second seed with same versions, got %d (first seed had %d)",
				secondUpdateCount, firstUpdateCount)
		}
	})
}

func TestProperty3_SeedingIdempotence_MultipleRuns(t *testing.T) {
	// Feature: skill-management, Property 3: Seeding idempotence
	// Verify idempotence holds across multiple consecutive seed runs (not just two).
	rapid.Check(t, func(t *rapid.T) {
		numTemplates := rapid.IntRange(1, 10).Draw(t, "numTemplates")
		templates := make([]BuiltinTemplate, numTemplates)

		for i := range templates {
			templates[i] = BuiltinTemplate{
				Slug:        rapid.StringMatching(`^[a-z][a-z0-9-]{2,15}$`).Draw(t, "slug"),
				Name:        rapid.StringMatching(`^[A-Z][a-zA-Z ]{2,20}$`).Draw(t, "name"),
				Description: rapid.StringMatching(`^[a-zA-Z0-9 .,]{10,50}$`).Draw(t, "description"),
				Content:     rapid.StringMatching(`^[a-zA-Z0-9 \n.,#]{50,150}$`).Draw(t, "content"),
				Category:    rapid.SampledFrom([]string{"Analysis", "Development", "Testing", "Operations", "Documentation"}).Draw(t, "category"),
				Version:     rapid.SampledFrom([]string{"1.0.0", "2.0.0", "3.0.0"}).Draw(t, "version"),
				Icon:        rapid.SampledFrom([]string{"📊", "💻", "🧪", "⚙️", "📝"}).Draw(t, "icon"),
			}
		}

		originalTemplates := BuiltinTemplates
		BuiltinTemplates = templates
		defer func() { BuiltinTemplates = originalTemplates }()

		ctx := context.Background()
		fq := newFakeQuerier()

		// First seed
		SeedTemplates(ctx, fq)

		// Snapshot after first seed
		fq.mu.Lock()
		firstSnapshots := make(map[string]time.Time)
		for slug, rec := range fq.records {
			firstSnapshots[slug] = rec.updatedAt
		}
		fq.mu.Unlock()

		// Run seed N more times (3-5 additional runs)
		numExtraRuns := rapid.IntRange(3, 5).Draw(t, "numExtraRuns")
		for run := 0; run < numExtraRuns; run++ {
			time.Sleep(1 * time.Millisecond)
			SeedTemplates(ctx, fq)
		}

		// Verify updated_at unchanged across all runs
		fq.mu.Lock()
		for slug, rec := range fq.records {
			firstTime, exists := firstSnapshots[slug]
			if !exists {
				continue
			}
			if !rec.updatedAt.Equal(firstTime) {
				t.Fatalf("template %q: updated_at changed after %d extra seed runs (was %v, now %v)",
					slug, numExtraRuns, firstTime, rec.updatedAt)
			}
		}
		fq.mu.Unlock()
	})
}

func TestProperty3_SeedingIdempotence_UniqueSlugDedup(t *testing.T) {
	// Feature: skill-management, Property 3: Seeding idempotence
	// When multiple templates share the same slug (edge case in generated data),
	// the last one wins on first seed, but subsequent seeds remain idempotent.
	rapid.Check(t, func(t *rapid.T) {
		// Generate templates where some may share slugs
		slug := rapid.StringMatching(`^[a-z][a-z0-9-]{2,10}$`).Draw(t, "slug")
		version := rapid.SampledFrom([]string{"1.0.0", "2.0.0"}).Draw(t, "version")

		numDuplicates := rapid.IntRange(2, 5).Draw(t, "numDuplicates")
		templates := make([]BuiltinTemplate, numDuplicates)
		for i := range templates {
			templates[i] = BuiltinTemplate{
				Slug:        slug,
				Name:        rapid.StringMatching(`^[A-Z][a-zA-Z ]{2,20}$`).Draw(t, "name"),
				Description: rapid.StringMatching(`^[a-zA-Z0-9 .,]{10,50}$`).Draw(t, "description"),
				Content:     rapid.StringMatching(`^[a-zA-Z0-9 \n.,#]{50,100}$`).Draw(t, "content"),
				Category:    rapid.SampledFrom([]string{"Analysis", "Development"}).Draw(t, "category"),
				Version:     version, // same version for all duplicates
				Icon:        "📊",
			}
		}

		originalTemplates := BuiltinTemplates
		BuiltinTemplates = templates
		defer func() { BuiltinTemplates = originalTemplates }()

		ctx := context.Background()
		fq := newFakeQuerier()

		// First seed
		SeedTemplates(ctx, fq)

		fq.mu.Lock()
		firstUpdatedAt := fq.records[slug].updatedAt
		fq.updateCount = 0
		fq.mu.Unlock()

		time.Sleep(1 * time.Millisecond)

		// Second seed
		SeedTemplates(ctx, fq)

		fq.mu.Lock()
		if !fq.records[slug].updatedAt.Equal(firstUpdatedAt) {
			t.Fatalf("template %q: updated_at changed on second seed despite same version", slug)
		}
		if fq.updateCount != 0 {
			t.Fatalf("expected 0 updates on second seed, got %d", fq.updateCount)
		}
		fq.mu.Unlock()
	})
}

// ---------------------------------------------------------------------------
// Feature: skill-management, Property 1: Template data integrity
//
// For any skill template in the system, the template SHALL have: a slug
// matching `^[a-z0-9][a-z0-9-]*$` (1-64 chars), a non-empty name (max 128
// chars), a category from the valid set, a description of 1-512 characters,
// content of 500-204800 bytes, and an icon that is a non-empty string (single
// grapheme cluster).
//
// **Validates: Requirements 2.2, 2.3, 2.4**
// ---------------------------------------------------------------------------

// validCategories is the set of allowed categories for skill templates.
var validCategories = map[string]bool{
	"Analysis":      true,
	"Development":   true,
	"Testing":       true,
	"Operations":    true,
	"Documentation": true,
}

// slugPattern validates the slug format constraint.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// validateTemplate checks a single BuiltinTemplate against all data integrity
// constraints. Returns an error describing the first violation found.
func validateTemplate(tmpl BuiltinTemplate) error {
	// Slug: matches pattern and 1-64 chars
	if len(tmpl.Slug) < 1 || len(tmpl.Slug) > 64 {
		return fmt.Errorf("slug %q length %d not in [1,64]", tmpl.Slug, len(tmpl.Slug))
	}
	if !slugPattern.MatchString(tmpl.Slug) {
		return fmt.Errorf("slug %q does not match ^[a-z0-9][a-z0-9-]*$", tmpl.Slug)
	}

	// Name: non-empty, max 128 chars
	if len(tmpl.Name) == 0 {
		return fmt.Errorf("slug %q: name is empty", tmpl.Slug)
	}
	if len(tmpl.Name) > 128 {
		return fmt.Errorf("slug %q: name length %d exceeds 128", tmpl.Slug, len(tmpl.Name))
	}

	// Category: must be from valid set
	if !validCategories[tmpl.Category] {
		return fmt.Errorf("slug %q: category %q not in valid set", tmpl.Slug, tmpl.Category)
	}

	// Description: 1-512 characters
	if len(tmpl.Description) < 1 {
		return fmt.Errorf("slug %q: description is empty", tmpl.Slug)
	}
	if len(tmpl.Description) > 512 {
		return fmt.Errorf("slug %q: description length %d exceeds 512", tmpl.Slug, len(tmpl.Description))
	}

	// Content: 500-204800 bytes
	contentBytes := len([]byte(tmpl.Content))
	if contentBytes < 500 {
		return fmt.Errorf("slug %q: content size %d bytes < 500", tmpl.Slug, contentBytes)
	}
	if contentBytes > 204800 {
		return fmt.Errorf("slug %q: content size %d bytes > 204800", tmpl.Slug, contentBytes)
	}

	// Icon: non-empty string (single grapheme cluster check simplified to non-empty)
	if len(tmpl.Icon) == 0 {
		return fmt.Errorf("slug %q: icon is empty", tmpl.Slug)
	}

	// Version: must be "1.0.0" for initial templates
	if tmpl.Version != "1.0.0" {
		return fmt.Errorf("slug %q: version %q is not 1.0.0", tmpl.Slug, tmpl.Version)
	}

	return nil
}

func TestProperty1_TemplateDataIntegrity_BuiltinTemplates(t *testing.T) {
	// Feature: skill-management, Property 1: Template data integrity
	// Validate all actual BuiltinTemplates against the data integrity constraints.
	if len(BuiltinTemplates) == 0 {
		t.Fatal("BuiltinTemplates is empty; expected at least 10 SDLC templates")
	}

	for _, tmpl := range BuiltinTemplates {
		t.Run(tmpl.Slug, func(t *testing.T) {
			if err := validateTemplate(tmpl); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestProperty1_TemplateDataIntegrity_RandomValid(t *testing.T) {
	// Feature: skill-management, Property 1: Template data integrity
	// Use rapid to generate random templates that meet the constraints and verify
	// they pass validation — confirming the validator itself is consistent.
	rapid.Check(t, func(t *rapid.T) {
		categories := []string{"Analysis", "Development", "Testing", "Operations", "Documentation"}

		slug := rapid.StringMatching(`^[a-z0-9][a-z0-9-]{0,63}$`).Draw(t, "slug")
		// Ensure slug is 1-64 chars (the regex already guarantees this)
		if len(slug) > 64 {
			slug = slug[:64]
		}

		name := rapid.StringMatching(`^[A-Z][a-zA-Z ]{0,126}$`).Draw(t, "name")
		if len(name) == 0 {
			name = "A"
		}

		category := rapid.SampledFrom(categories).Draw(t, "category")

		// Description: 1-512 chars
		descLen := rapid.IntRange(1, 512).Draw(t, "descLen")
		desc := rapid.StringMatching(`^[a-zA-Z0-9 .,]+$`).Draw(t, "descBase")
		if len(desc) > descLen {
			desc = desc[:descLen]
		}
		if len(desc) == 0 {
			desc = "A valid description."
		}
		if len(desc) > 512 {
			desc = desc[:512]
		}

		// Content: 500-204800 bytes — generate at least 500 chars
		contentLen := rapid.IntRange(500, 2000).Draw(t, "contentLen")
		content := make([]byte, contentLen)
		for i := range content {
			content[i] = byte('a' + (i % 26))
		}

		icon := rapid.SampledFrom([]string{"📊", "🔍", "📋", "🏗️", "💻", "👁️", "🧪", "🔒", "⚙️", "📝"}).Draw(t, "icon")

		tmpl := BuiltinTemplate{
			Slug:        slug,
			Name:        name,
			Description: desc,
			Content:     string(content),
			Category:    category,
			Version:     "1.0.0",
			Icon:        icon,
		}

		if err := validateTemplate(tmpl); err != nil {
			t.Fatalf("randomly generated valid template failed validation: %v", err)
		}
	})
}

func TestProperty1_TemplateDataIntegrity_InvalidSlugDetected(t *testing.T) {
	// Feature: skill-management, Property 1: Template data integrity
	// Verify that templates with invalid slugs are correctly rejected.
	rapid.Check(t, func(t *rapid.T) {
		// Generate slugs that violate the pattern (start with hyphen, contain uppercase, etc.)
		invalidSlug := rapid.SampledFrom([]string{
			"-invalid",
			"UPPERCASE",
			"has spaces",
			"has_underscore",
			"",
			rapid.StringMatching(`^[A-Z][a-z]{0,10}$`).Draw(t, "upperSlug"),
		}).Draw(t, "invalidSlug")

		tmpl := BuiltinTemplate{
			Slug:        invalidSlug,
			Name:        "Valid Name",
			Description: "A valid description for testing purposes.",
			Content:     string(make([]byte, 500)),
			Category:    "Development",
			Version:     "1.0.0",
			Icon:        "💻",
		}

		err := validateTemplate(tmpl)
		if err == nil && invalidSlug != "" {
			// Only fail if the slug truly doesn't match the pattern
			if !slugPattern.MatchString(invalidSlug) || len(invalidSlug) < 1 || len(invalidSlug) > 64 {
				t.Fatalf("expected validation error for invalid slug %q, got nil", invalidSlug)
			}
		}
	})
}

// pgtype helpers for the fake querier (unused methods satisfy the interface via embedding).
var _ db.Querier = (*fakeQuerier)(nil)

// Ensure the fakeQuerier's UpsertSkillTemplate handles the pgtype.Text icon field correctly.
func TestProperty3_SeedingIdempotence_IconHandling(t *testing.T) {
	// Feature: skill-management, Property 3: Seeding idempotence
	// Verify that templates with various icon values (including empty) remain idempotent.
	rapid.Check(t, func(t *rapid.T) {
		icon := rapid.SampledFrom([]string{"", "📊", "💻", "🧪", "⚙️", "📝", "🔍", "🏗️"}).Draw(t, "icon")

		templates := []BuiltinTemplate{
			{
				Slug:        rapid.StringMatching(`^[a-z][a-z0-9-]{2,10}$`).Draw(t, "slug"),
				Name:        "Test Template",
				Description: "A test template for icon handling",
				Content:     "This is test content that is long enough to be valid for the property test.",
				Category:    "Testing",
				Version:     "1.0.0",
				Icon:        icon,
			},
		}

		originalTemplates := BuiltinTemplates
		BuiltinTemplates = templates
		defer func() { BuiltinTemplates = originalTemplates }()

		ctx := context.Background()
		fq := newFakeQuerier()

		// First seed
		SeedTemplates(ctx, fq)

		fq.mu.Lock()
		slug := templates[0].Slug
		firstUpdatedAt := fq.records[slug].updatedAt
		fq.updateCount = 0
		fq.mu.Unlock()

		time.Sleep(1 * time.Millisecond)

		// Second seed
		SeedTemplates(ctx, fq)

		fq.mu.Lock()
		if !fq.records[slug].updatedAt.Equal(firstUpdatedAt) {
			t.Fatalf("template %q with icon %q: updated_at changed on second seed", slug, icon)
		}
		if fq.updateCount != 0 {
			t.Fatalf("expected 0 updates on second seed with icon %q, got %d", icon, fq.updateCount)
		}
		fq.mu.Unlock()
	})
}

// ---------------------------------------------------------------------------
// Feature: skill-management, Property 4: Seeding version update
//
// For any built-in template where the embedded version differs from the
// database version, running the seeding process SHALL update the record's
// name, description, content, category, icon, and version to match the
// embedded values.
//
// **Validates: Requirements 3.2**
// ---------------------------------------------------------------------------

func TestProperty4_SeedingVersionUpdate_RecordUpdatedOnVersionChange(t *testing.T) {
	// Feature: skill-management, Property 4: Seeding version update
	// When the embedded version differs from the DB version, the record is updated.
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random slug
		slug := rapid.StringMatching(`^[a-z][a-z0-9-]{2,15}$`).Draw(t, "slug")

		// Generate old version (what's in the DB)
		oldVersion := rapid.SampledFrom([]string{"1.0.0", "1.1.0", "2.0.0", "0.9.0"}).Draw(t, "oldVersion")

		// Generate new version (what's embedded) — must differ from old
		newVersion := rapid.SampledFrom([]string{"1.0.0", "1.1.0", "2.0.0", "3.0.0", "2.1.0"}).Draw(t, "newVersion")
		if newVersion == oldVersion {
			newVersion = oldVersion + ".1"
		}

		// Generate new template fields
		newName := rapid.StringMatching(`^[A-Z][a-zA-Z ]{2,30}$`).Draw(t, "newName")
		newDescription := rapid.StringMatching(`^[A-Za-z ]{10,50}$`).Draw(t, "newDescription")
		newContent := rapid.StringMatching(`^[A-Za-z #\n]{50,200}$`).Draw(t, "newContent")
		newCategory := rapid.SampledFrom([]string{"Analysis", "Development", "Testing", "Operations", "Documentation"}).Draw(t, "newCategory")
		newIcon := rapid.SampledFrom([]string{"📊", "🔍", "📋", "🏗️", "💻", "👁️", "🧪", "🔒", "⚙️", "📝"}).Draw(t, "newIcon")

		// Set up fake querier with existing record at old version
		fq := newFakeQuerier()
		now := time.Now()
		fq.records[slug] = &templateRecord{
			params: db.UpsertSkillTemplateParams{
				Slug:        slug,
				Name:        "Old Name",
				Description: "Old description",
				Content:     "Old content that was previously seeded",
				Category:    "OldCategory",
				Version:     oldVersion,
			},
			createdAt: now,
			updatedAt: now,
		}

		// Override BuiltinTemplates for this test
		origTemplates := BuiltinTemplates
		BuiltinTemplates = []BuiltinTemplate{
			{
				Slug:        slug,
				Name:        newName,
				Description: newDescription,
				Content:     newContent,
				Category:    newCategory,
				Version:     newVersion,
				Icon:        newIcon,
			},
		}
		defer func() { BuiltinTemplates = origTemplates }()

		// Small delay so updated_at would differ
		time.Sleep(1 * time.Millisecond)

		// Run seeding
		SeedTemplates(context.Background(), fq)

		// Verify the record was updated with new values
		fq.mu.Lock()
		defer fq.mu.Unlock()

		record, exists := fq.records[slug]
		if !exists {
			t.Fatal("record should exist after seeding")
		}
		if record.params.Version != newVersion {
			t.Fatalf("version not updated: got %q, want %q", record.params.Version, newVersion)
		}
		if record.params.Name != newName {
			t.Fatalf("name not updated: got %q, want %q", record.params.Name, newName)
		}
		if record.params.Description != newDescription {
			t.Fatalf("description not updated: got %q, want %q", record.params.Description, newDescription)
		}
		if record.params.Content != newContent {
			t.Fatalf("content not updated: got %q, want %q", record.params.Content, newContent)
		}
		if record.params.Category != newCategory {
			t.Fatalf("category not updated: got %q, want %q", record.params.Category, newCategory)
		}
		if record.updatedAt.After(now) {
			// Good: updated_at was refreshed
		} else {
			t.Fatalf("updated_at was not refreshed after version change (still %v)", record.updatedAt)
		}
	})
}

func TestProperty4_SeedingVersionUpdate_AllFieldsMatchEmbedded(t *testing.T) {
	// Feature: skill-management, Property 4: Seeding version update
	// After seeding with a different version, ALL fields in the record must
	// exactly match the embedded template values.
	rapid.Check(t, func(t *rapid.T) {
		// Generate multiple templates with version changes
		numTemplates := rapid.IntRange(1, 5).Draw(t, "numTemplates")
		templates := make([]BuiltinTemplate, numTemplates)
		slugSet := make(map[string]bool)

		fq := newFakeQuerier()
		now := time.Now()

		for i := 0; i < numTemplates; i++ {
			slug := rapid.StringMatching(`^[a-z][a-z0-9-]{2,10}$`).Draw(t, "slug")
			// Ensure unique slugs
			for slugSet[slug] {
				slug = slug + rapid.StringMatching(`^[a-z]{1,3}$`).Draw(t, "suffix")
			}
			slugSet[slug] = true

			oldVersion := rapid.SampledFrom([]string{"1.0.0", "1.1.0", "2.0.0"}).Draw(t, "oldVer")
			newVersion := rapid.SampledFrom([]string{"2.1.0", "3.0.0", "4.0.0"}).Draw(t, "newVer")
			if newVersion == oldVersion {
				newVersion = oldVersion + ".1"
			}

			newName := rapid.StringMatching(`^[A-Z][a-zA-Z ]{2,20}$`).Draw(t, "name")
			newDesc := rapid.StringMatching(`^[A-Za-z ]{10,40}$`).Draw(t, "desc")
			newContent := rapid.StringMatching(`^[A-Za-z #\n]{50,200}$`).Draw(t, "content")
			newCategory := rapid.SampledFrom([]string{"Analysis", "Development", "Testing", "Operations", "Documentation"}).Draw(t, "category")
			newIcon := rapid.SampledFrom([]string{"📊", "🔍", "📋", "🏗️", "💻"}).Draw(t, "icon")

			// Preload with old version
			fq.records[slug] = &templateRecord{
				params: db.UpsertSkillTemplateParams{
					Slug:        slug,
					Name:        "OldName",
					Description: "OldDesc",
					Content:     "OldContent",
					Category:    "OldCat",
					Version:     oldVersion,
				},
				createdAt: now,
				updatedAt: now,
			}

			templates[i] = BuiltinTemplate{
				Slug:        slug,
				Name:        newName,
				Description: newDesc,
				Content:     newContent,
				Category:    newCategory,
				Version:     newVersion,
				Icon:        newIcon,
			}
		}

		// Override BuiltinTemplates
		origTemplates := BuiltinTemplates
		BuiltinTemplates = templates
		defer func() { BuiltinTemplates = origTemplates }()

		time.Sleep(1 * time.Millisecond)

		// Run seeding
		SeedTemplates(context.Background(), fq)

		// Verify each template was updated to match embedded values
		fq.mu.Lock()
		defer fq.mu.Unlock()

		for _, tmpl := range templates {
			record, exists := fq.records[tmpl.Slug]
			if !exists {
				t.Fatalf("record for slug %q should exist", tmpl.Slug)
			}
			if record.params.Name != tmpl.Name {
				t.Fatalf("slug %q: name mismatch: got %q, want %q", tmpl.Slug, record.params.Name, tmpl.Name)
			}
			if record.params.Description != tmpl.Description {
				t.Fatalf("slug %q: description mismatch: got %q, want %q", tmpl.Slug, record.params.Description, tmpl.Description)
			}
			if record.params.Content != tmpl.Content {
				t.Fatalf("slug %q: content mismatch: got %q, want %q", tmpl.Slug, record.params.Content, tmpl.Content)
			}
			if record.params.Category != tmpl.Category {
				t.Fatalf("slug %q: category mismatch: got %q, want %q", tmpl.Slug, record.params.Category, tmpl.Category)
			}
			if record.params.Version != tmpl.Version {
				t.Fatalf("slug %q: version mismatch: got %q, want %q", tmpl.Slug, record.params.Version, tmpl.Version)
			}
		}
	})
}

func TestProperty4_SeedingVersionUpdate_UpsertCalledWithNewVersion(t *testing.T) {
	// Feature: skill-management, Property 4: Seeding version update
	// SeedTemplates always passes the new embedded version to UpsertSkillTemplate,
	// regardless of what's currently in the DB. The DB-level WHERE clause handles
	// the version comparison.
	rapid.Check(t, func(t *rapid.T) {
		slug := rapid.StringMatching(`^[a-z][a-z0-9-]{2,12}$`).Draw(t, "slug")
		oldVersion := rapid.SampledFrom([]string{"1.0.0", "1.1.0", "2.0.0"}).Draw(t, "oldVersion")
		newVersion := rapid.SampledFrom([]string{"2.1.0", "3.0.0", "4.0.0", "5.0.0"}).Draw(t, "newVersion")
		if newVersion == oldVersion {
			newVersion = oldVersion + "-bump"
		}

		newName := rapid.StringMatching(`^[A-Z][a-zA-Z ]{2,20}$`).Draw(t, "name")
		newContent := rapid.StringMatching(`^[A-Za-z #]{50,200}$`).Draw(t, "content")

		fq := newFakeQuerier()
		now := time.Now()
		fq.records[slug] = &templateRecord{
			params: db.UpsertSkillTemplateParams{
				Slug:        slug,
				Name:        "Old",
				Description: "Old",
				Content:     "Old",
				Category:    "Old",
				Version:     oldVersion,
			},
			createdAt: now,
			updatedAt: now,
		}

		origTemplates := BuiltinTemplates
		BuiltinTemplates = []BuiltinTemplate{
			{
				Slug:        slug,
				Name:        newName,
				Description: "New desc",
				Content:     newContent,
				Category:    "Development",
				Version:     newVersion,
				Icon:        "💻",
			},
		}
		defer func() { BuiltinTemplates = origTemplates }()

		SeedTemplates(context.Background(), fq)

		// Verify UpsertSkillTemplate was called with the new version
		fq.mu.Lock()
		defer fq.mu.Unlock()

		if len(fq.upsertCalls) != 1 {
			t.Fatalf("expected 1 upsert call, got %d", len(fq.upsertCalls))
		}
		call := fq.upsertCalls[0]
		if call.Version != newVersion {
			t.Fatalf("upsert called with version %q, want %q", call.Version, newVersion)
		}
		if call.Name != newName {
			t.Fatalf("upsert called with name %q, want %q", call.Name, newName)
		}
		if call.Slug != slug {
			t.Fatalf("upsert called with slug %q, want %q", call.Slug, slug)
		}
	})
}

func TestProperty4_SeedingVersionUpdate_OnlyDifferentVersionTriggersUpdate(t *testing.T) {
	// Feature: skill-management, Property 4: Seeding version update
	// Verify that the update count is exactly equal to the number of templates
	// whose version differs from the DB version.
	rapid.Check(t, func(t *rapid.T) {
		numTemplates := rapid.IntRange(2, 8).Draw(t, "numTemplates")
		templates := make([]BuiltinTemplate, numTemplates)
		slugSet := make(map[string]bool)

		fq := newFakeQuerier()
		now := time.Now()
		expectedUpdates := 0

		for i := 0; i < numTemplates; i++ {
			slug := rapid.StringMatching(`^[a-z][a-z0-9-]{2,10}$`).Draw(t, "slug")
			for slugSet[slug] {
				slug = slug + rapid.StringMatching(`^[a-z]{1,3}$`).Draw(t, "suffix")
			}
			slugSet[slug] = true

			dbVersion := rapid.SampledFrom([]string{"1.0.0", "2.0.0", "3.0.0"}).Draw(t, "dbVersion")
			// Randomly decide if this template has a version change
			hasVersionChange := rapid.Bool().Draw(t, "hasVersionChange")

			var embeddedVersion string
			if hasVersionChange {
				embeddedVersion = rapid.SampledFrom([]string{"4.0.0", "5.0.0", "6.0.0"}).Draw(t, "newVer")
				if embeddedVersion == dbVersion {
					embeddedVersion = dbVersion + ".1"
				}
				expectedUpdates++
			} else {
				embeddedVersion = dbVersion // same version, no update expected
			}

			// Preload DB record
			fq.records[slug] = &templateRecord{
				params: db.UpsertSkillTemplateParams{
					Slug:    slug,
					Name:    "OldName",
					Version: dbVersion,
				},
				createdAt: now,
				updatedAt: now,
			}

			templates[i] = BuiltinTemplate{
				Slug:        slug,
				Name:        rapid.StringMatching(`^[A-Z][a-zA-Z ]{2,15}$`).Draw(t, "name"),
				Description: "Desc",
				Content:     "Content for testing purposes that is long enough",
				Category:    "Development",
				Version:     embeddedVersion,
				Icon:        "💻",
			}
		}

		origTemplates := BuiltinTemplates
		BuiltinTemplates = templates
		defer func() { BuiltinTemplates = origTemplates }()

		// Reset update count (preload increments it)
		fq.mu.Lock()
		fq.updateCount = 0
		fq.mu.Unlock()

		SeedTemplates(context.Background(), fq)

		fq.mu.Lock()
		defer fq.mu.Unlock()

		if fq.updateCount != expectedUpdates {
			t.Fatalf("expected %d updates (templates with version changes), got %d",
				expectedUpdates, fq.updateCount)
		}
	})
}
