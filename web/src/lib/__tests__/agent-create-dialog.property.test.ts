// Feature: agent-management-ui, Property 9: Create button disabled state
// **Validates: Requirements 9.8**

import { describe, it, expect } from "vitest";
import fc from "fast-check";
import { isCreateDisabled } from "../agent-create-validation";

describe("isCreateDisabled — Property 9: Create button disabled state", () => {
  it("disabled iff name.trim() === '' OR selectedRuntimeId === ''", () => {
    fc.assert(
      fc.property(
        fc.string(),
        fc.string(),
        (name, selectedRuntimeId) => {
          const result = isCreateDisabled(name, selectedRuntimeId);
          const expected = name.trim() === "" || selectedRuntimeId === "";
          expect(result).toBe(expected);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("returns true when name is empty string", () => {
    fc.assert(
      fc.property(
        fc.string(),
        (runtimeId) => {
          expect(isCreateDisabled("", runtimeId)).toBe(true);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("returns true when name is whitespace-only", () => {
    fc.assert(
      fc.property(
        fc.array(fc.constantFrom(" ", "\t", "\n", "\r"), { minLength: 1, maxLength: 20 }).map(
          (chars) => chars.join("")
        ),
        fc.string({ minLength: 1 }),
        (whitespaceOnlyName, runtimeId) => {
          expect(isCreateDisabled(whitespaceOnlyName, runtimeId)).toBe(true);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("returns true when selectedRuntimeId is empty string", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1 }).filter((s) => s.trim().length > 0),
        (name) => {
          expect(isCreateDisabled(name, "")).toBe(true);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("returns false when name has non-whitespace content AND runtimeId is non-empty", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1 }).filter((s) => s.trim().length > 0),
        fc.string({ minLength: 1 }),
        (name, runtimeId) => {
          expect(isCreateDisabled(name, runtimeId)).toBe(false);
        }
      ),
      { numRuns: 100 }
    );
  });
});
