// Feature: agent-management-ui, Property 7: Duplicate pre-population correctness
// **Validates: Requirements 7.2, 9.12**

import { describe, it, expect } from "vitest";
import fc from "fast-check";
import { buildDuplicatePayload, type AgentListItem } from "../agent-duplicate";

/* ─── Generators ─── */

/**
 * Generates an arbitrary AgentListItem with random values for all fields.
 * Names are 1-64 alphanumeric chars, descriptions 0-255 chars, etc.
 */
function arbitraryAgent(): fc.Arbitrary<AgentListItem> {
  return fc.record({
    name: fc.string({ minLength: 1, maxLength: 64 }).filter((s) => s.length > 0),
    description: fc.string({ minLength: 0, maxLength: 255 }),
    instructions: fc.string({ minLength: 0, maxLength: 500 }),
    runtime_id: fc.uuid(),
    model: fc.oneof(fc.constant(""), fc.string({ minLength: 1, maxLength: 64 })),
    visibility: fc.constantFrom("private" as const, "shared" as const),
    avatar_url: fc.oneof(
      fc.constant(null),
      fc.webUrl().map((url) => url)
    ),
    custom_env: fc.oneof(
      fc.constant({} as Record<string, string>),
      fc.dictionary(
        fc.string({ minLength: 1, maxLength: 20 }).filter((s) => s.trim().length > 0),
        fc.string({ minLength: 0, maxLength: 50 }),
        { minKeys: 1, maxKeys: 5 }
      )
    ),
    custom_args: fc.array(fc.string({ minLength: 1, maxLength: 30 }), {
      minLength: 0,
      maxLength: 5,
    }),
    max_concurrent_tasks: fc.integer({ min: 0, max: 20 }),
  });
}

/* ─── Property Tests ─── */

describe("buildDuplicatePayload — Property 7: Duplicate pre-population correctness", () => {
  it('payload.name === source.name + " copy" (always)', () => {
    fc.assert(
      fc.property(arbitraryAgent(), (source) => {
        const payload = buildDuplicatePayload(source);
        expect(payload.name).toBe(source.name + " copy");
      }),
      { numRuns: 100 }
    );
  });

  it("payload.description === source.description (always)", () => {
    fc.assert(
      fc.property(arbitraryAgent(), (source) => {
        const payload = buildDuplicatePayload(source);
        expect(payload.description).toBe(source.description);
      }),
      { numRuns: 100 }
    );
  });

  it("payload.runtime_id === source.runtime_id (always)", () => {
    fc.assert(
      fc.property(arbitraryAgent(), (source) => {
        const payload = buildDuplicatePayload(source);
        expect(payload.runtime_id).toBe(source.runtime_id);
      }),
      { numRuns: 100 }
    );
  });

  it("payload.visibility === source.visibility (always)", () => {
    fc.assert(
      fc.property(arbitraryAgent(), (source) => {
        const payload = buildDuplicatePayload(source);
        expect(payload.visibility).toBe(source.visibility);
      }),
      { numRuns: 100 }
    );
  });

  it("if source.instructions is non-empty, payload.instructions === source.instructions", () => {
    fc.assert(
      fc.property(
        arbitraryAgent().filter((a) => a.instructions.length > 0),
        (source) => {
          const payload = buildDuplicatePayload(source);
          expect(payload.instructions).toBe(source.instructions);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("if source.model is non-empty, payload.model === source.model", () => {
    fc.assert(
      fc.property(
        arbitraryAgent().filter((a) => a.model.length > 0),
        (source) => {
          const payload = buildDuplicatePayload(source);
          expect(payload.model).toBe(source.model);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("if source.custom_env has keys, payload.custom_env === source.custom_env", () => {
    fc.assert(
      fc.property(
        arbitraryAgent().filter((a) => Object.keys(a.custom_env).length > 0),
        (source) => {
          const payload = buildDuplicatePayload(source);
          expect(payload.custom_env).toEqual(source.custom_env);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("if source.custom_args is non-empty, payload.custom_args === source.custom_args", () => {
    fc.assert(
      fc.property(
        arbitraryAgent().filter((a) => a.custom_args.length > 0),
        (source) => {
          const payload = buildDuplicatePayload(source);
          expect(payload.custom_args).toEqual(source.custom_args);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("if source.max_concurrent_tasks > 0, payload.max_concurrent_tasks === source.max_concurrent_tasks", () => {
    fc.assert(
      fc.property(
        arbitraryAgent().filter((a) => a.max_concurrent_tasks > 0),
        (source) => {
          const payload = buildDuplicatePayload(source);
          expect(payload.max_concurrent_tasks).toBe(source.max_concurrent_tasks);
        }
      ),
      { numRuns: 100 }
    );
  });
});
