/**
 * Property-based tests for Skill Library page logic.
 *
 * Feature: skill-management
 *
 * Property 11: Client-side search filter (Validates: Requirements 6.4)
 * Property 13: Already-instantiated detection (Validates: Requirements 6.7, 9.4)
 *
 * Uses fast-check to generate random inputs and verify pure logic functions
 * extracted from SkillLibrary.tsx.
 */
import { describe, it, expect } from "vitest";
import * as fc from "fast-check";

// --- Extracted pure filter function (mirrors SkillLibrary.tsx logic) ---

interface SkillTemplateSummary {
  id: string;
  slug: string;
  name: string;
  description: string;
  category: string;
  version: string;
  icon: string | null;
}

/**
 * Filters templates by search term using case-insensitive substring match
 * on name or description. This is the exact logic used in SkillLibrary.tsx.
 */
function filterTemplates(
  templates: SkillTemplateSummary[],
  search: string
): SkillTemplateSummary[] {
  if (!search.trim()) return templates;
  const term = search.toLowerCase();
  return templates.filter(
    (t) =>
      t.name.toLowerCase().includes(term) ||
      t.description.toLowerCase().includes(term)
  );
}

// --- Generators ---

const templateArb: fc.Arbitrary<SkillTemplateSummary> = fc.record({
  id: fc.uuid(),
  slug: fc.stringMatching(/^[a-z0-9][a-z0-9-]{0,15}$/),
  name: fc.string({ minLength: 1, maxLength: 64 }),
  description: fc.string({ minLength: 0, maxLength: 256 }),
  category: fc.constantFrom("Analysis", "Development", "Testing", "Operations", "Documentation"),
  version: fc.constant("1.0.0"),
  icon: fc.constantFrom("📊", "🔍", "📋", "🏗️", "💻", "👁️", "🧪", "🔒", "⚙️", "📝", null),
});

const templateArrayArb = fc.array(templateArb, { minLength: 0, maxLength: 30 });

// Search term: non-empty string (trimmed) to exercise the filter path
const searchTermArb = fc.string({ minLength: 1, maxLength: 20 }).filter(
  (s) => s.trim().length > 0
);

// --- Property Tests ---

describe("Feature: skill-management, Property 11: Client-side search filter", () => {
  it("filtered result contains exactly templates whose name or description includes the search term (case-insensitive)", () => {
    fc.assert(
      fc.property(templateArrayArb, searchTermArb, (templates, search) => {
        const result = filterTemplates(templates, search);
        const term = search.toLowerCase();

        // Every returned template must match
        for (const t of result) {
          const nameMatches = t.name.toLowerCase().includes(term);
          const descMatches = t.description.toLowerCase().includes(term);
          expect(nameMatches || descMatches).toBe(true);
        }

        // Every template that matches must be in the result
        for (const t of templates) {
          const nameMatches = t.name.toLowerCase().includes(term);
          const descMatches = t.description.toLowerCase().includes(term);
          if (nameMatches || descMatches) {
            expect(result).toContain(t);
          }
        }

        // Result length equals the count of matching templates
        const expectedCount = templates.filter(
          (t) =>
            t.name.toLowerCase().includes(term) ||
            t.description.toLowerCase().includes(term)
        ).length;
        expect(result.length).toBe(expectedCount);
      }),
      { numRuns: 200 }
    );
  });

  it("empty or whitespace-only search returns all templates unfiltered", () => {
    fc.assert(
      fc.property(
        templateArrayArb,
        fc.constantFrom("", "   ", "\t", "\n"),
        (templates, search) => {
          const result = filterTemplates(templates, search);
          expect(result).toEqual(templates);
          expect(result.length).toBe(templates.length);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("filter is case-insensitive: searching with any case variant yields same results", () => {
    fc.assert(
      fc.property(templateArrayArb, searchTermArb, (templates, search) => {
        const lower = filterTemplates(templates, search.toLowerCase());
        const upper = filterTemplates(templates, search.toUpperCase());
        const mixed = filterTemplates(templates, search);

        expect(lower.length).toBe(upper.length);
        expect(lower.length).toBe(mixed.length);

        // Same set of templates regardless of case
        for (const t of lower) {
          expect(upper).toContain(t);
          expect(mixed).toContain(t);
        }
      }),
      { numRuns: 100 }
    );
  });

  it("filter result is a subset of the original templates (preserves order)", () => {
    fc.assert(
      fc.property(templateArrayArb, searchTermArb, (templates, search) => {
        const result = filterTemplates(templates, search);

        // Result preserves relative order from original array
        let lastIndex = -1;
        for (const t of result) {
          const idx = templates.indexOf(t);
          expect(idx).toBeGreaterThan(lastIndex);
          lastIndex = idx;
        }
      }),
      { numRuns: 100 }
    );
  });

  it("a template with the search term embedded in its name always appears in results", () => {
    fc.assert(
      fc.property(
        templateArrayArb,
        searchTermArb,
        fc.uuid(),
        (baseTemplates, search, id) => {
          // Create a template guaranteed to match by embedding the search term in the name
          const matchingTemplate: SkillTemplateSummary = {
            id,
            slug: "test-slug",
            name: `prefix-${search}-suffix`,
            description: "unrelated description",
            category: "Development",
            version: "1.0.0",
            icon: "💻",
          };

          const templates = [...baseTemplates, matchingTemplate];
          const result = filterTemplates(templates, search);

          expect(result).toContain(matchingTemplate);
        }
      ),
      { numRuns: 100 }
    );
  });
});


// --- Property 13: Already-Instantiated Detection ---

/**
 * Feature: skill-management, Property 13: Already-instantiated detection
 *
 * Validates: Requirements 6.7, 9.4
 *
 * The detection logic from SkillLibrary.tsx:
 *   userSkillNames = new Set(userSkills.map(s => s.name))
 *   alreadyAdded = userSkillNames.has(template.slug)
 *
 * A template shows "Already Added" if and only if its slug matches
 * a user skill name in the set.
 */

/**
 * Pure function that determines whether a template is already instantiated.
 * Mirrors the exact logic in SkillLibrary.tsx.
 */
function isAlreadyInstantiated(
  templateSlug: string,
  userSkillNames: Set<string>
): boolean {
  return userSkillNames.has(templateSlug);
}

/**
 * Builds the user skill names set from an array of user skills.
 * Mirrors: new Set(userSkills.map(s => s.name))
 */
function buildUserSkillNames(userSkills: { name: string }[]): Set<string> {
  return new Set(userSkills.map((s) => s.name));
}

// --- Generators for Property 13 ---

// Template slug: valid slug pattern (lowercase alphanumeric with hyphens)
const slugArb = fc.stringMatching(/^[a-z0-9][a-z0-9-]{0,15}$/);

// User skill name: can be any string (skills may have arbitrary names)
const userSkillNameArb = fc.string({ minLength: 1, maxLength: 64 });

// Array of user skill objects (with name field)
const userSkillsArb = fc.array(
  fc.record({ name: userSkillNameArb }),
  { minLength: 0, maxLength: 20 }
);

// Array of template slugs
const templateSlugsArb = fc.array(slugArb, { minLength: 1, maxLength: 20 });

describe("Feature: skill-management, Property 13: Already-instantiated detection", () => {
  it("template shows 'Already Added' iff its slug matches a user skill name", () => {
    fc.assert(
      fc.property(templateSlugsArb, userSkillsArb, (slugs, userSkills) => {
        const userSkillNames = buildUserSkillNames(userSkills);

        for (const slug of slugs) {
          const result = isAlreadyInstantiated(slug, userSkillNames);
          const expected = userSkillNames.has(slug);
          expect(result).toBe(expected);
        }
      }),
      { numRuns: 200 }
    );
  });

  it("a template whose slug is in the user skill names set is always detected as already added", () => {
    fc.assert(
      fc.property(
        slugArb,
        userSkillsArb,
        (slug, baseUserSkills) => {
          // Ensure the slug is present in user skills
          const userSkills = [...baseUserSkills, { name: slug }];
          const userSkillNames = buildUserSkillNames(userSkills);

          expect(isAlreadyInstantiated(slug, userSkillNames)).toBe(true);
        }
      ),
      { numRuns: 200 }
    );
  });

  it("a template whose slug is NOT in the user skill names set is never detected as already added", () => {
    fc.assert(
      fc.property(
        slugArb,
        userSkillsArb,
        (slug, userSkills) => {
          // Remove any user skill that matches the slug
          const filteredSkills = userSkills.filter((s) => s.name !== slug);
          const userSkillNames = buildUserSkillNames(filteredSkills);

          expect(isAlreadyInstantiated(slug, userSkillNames)).toBe(false);
        }
      ),
      { numRuns: 200 }
    );
  });

  it("detection is exact match (not substring or case-insensitive)", () => {
    fc.assert(
      fc.property(
        slugArb,
        (slug) => {
          // Variations that should NOT match
          const variations = [
            slug.toUpperCase(),
            slug + "-extra",
            "prefix-" + slug,
            slug + " ",
            " " + slug,
          ].filter((v) => v !== slug); // exclude if variation happens to equal slug

          for (const variant of variations) {
            const userSkillNames = new Set([variant]);
            // The original slug should NOT be detected as already added
            // (unless the variant happens to equal the slug, which we filtered out)
            expect(isAlreadyInstantiated(slug, userSkillNames)).toBe(false);
          }
        }
      ),
      { numRuns: 100 }
    );
  });

  it("detection works correctly with empty user skill set", () => {
    fc.assert(
      fc.property(slugArb, (slug) => {
        const userSkillNames = buildUserSkillNames([]);
        expect(isAlreadyInstantiated(slug, userSkillNames)).toBe(false);
      }),
      { numRuns: 100 }
    );
  });

  it("detection is consistent: building set from skills and checking slug gives same result as direct Set.has", () => {
    fc.assert(
      fc.property(
        templateSlugsArb,
        userSkillsArb,
        (slugs, userSkills) => {
          const userSkillNames = buildUserSkillNames(userSkills);
          const nameArray = userSkills.map((s) => s.name);

          for (const slug of slugs) {
            // Direct array includes check should match Set-based detection
            const arrayResult = nameArray.includes(slug);
            const setResult = isAlreadyInstantiated(slug, userSkillNames);
            expect(setResult).toBe(arrayResult);
          }
        }
      ),
      { numRuns: 100 }
    );
  });
});
