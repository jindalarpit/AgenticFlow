// Feature: agent-detail-ui, Property 7: Duplicate key detection is correct
// **Validates: Requirements 14.7**

import { describe, it, expect } from "vitest";
import fc from "fast-check";
import { hasDuplicateKeys, computeSuccessRate, splitArgs } from "../agent-utils";

describe("hasDuplicateKeys — Property 7: Duplicate key detection is correct", () => {
  it("returns false for lists with all unique trimmed keys", () => {
    fc.assert(
      fc.property(
        fc.uniqueArray(
          fc.string({ minLength: 1 }).map((s) => s.trim()).filter((s) => s.length > 0),
          { minLength: 0, maxLength: 20, comparator: (a, b) => a === b }
        ),
        (uniqueKeys) => {
          const entries = uniqueKeys.map((key) => ({
            key,
            value: "v",
          }));
          expect(hasDuplicateKeys(entries)).toBe(false);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("returns true for lists where at least two entries have the same trimmed key", () => {
    fc.assert(
      fc.property(
        fc.array(
          fc.record({
            key: fc.string({ minLength: 1 }),
            value: fc.string(),
          }),
          { minLength: 1, maxLength: 19 }
        ),
        fc.nat({ max: 19 }),
        (baseEntries, insertIdx) => {
          // Pick a key from the existing entries and duplicate it (possibly with whitespace)
          const sourceIdx = insertIdx % baseEntries.length;
          const duplicateKey = "  " + baseEntries[sourceIdx]!.key.trim() + "  ";
          const duplicateEntry = { key: duplicateKey, value: "dup" };

          // Insert the duplicate at a different position
          const entries = [...baseEntries, duplicateEntry];

          // Only assert true if the trimmed key actually matches an existing trimmed key
          const trimmedKeys = entries.map((e) => e.key.trim());
          const hasDup = trimmedKeys.length !== new Set(trimmedKeys).size;
          expect(hasDuplicateKeys(entries)).toBe(hasDup);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("returns false for an empty list", () => {
    fc.assert(
      fc.property(fc.constant([] as { key: string; value: string }[]), (entries) => {
        expect(hasDuplicateKeys(entries)).toBe(false);
      }),
      { numRuns: 100 }
    );
  });

  it("returns false for a single entry", () => {
    fc.assert(
      fc.property(
        fc.record({ key: fc.string(), value: fc.string() }),
        (entry) => {
          expect(hasDuplicateKeys([entry])).toBe(false);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("returns true when whitespace-only differences in keys trim to the same value", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1 }).filter((s) => s.trim().length > 0),
        fc.string(),
        fc.string(),
        (baseKey, value1, value2) => {
          const trimmedKey = baseKey.trim();
          const entries = [
            { key: trimmedKey, value: value1 },
            { key: "  " + trimmedKey + "  ", value: value2 },
          ];
          expect(hasDuplicateKeys(entries)).toBe(true);
        }
      ),
      { numRuns: 100 }
    );
  });
});

// Feature: agent-detail-ui, Property 4: Success rate calculation is bounded and correct
describe("computeSuccessRate — Property 4: Success rate calculation is bounded and correct", () => {
  it("returns 0 when total_terminal === 0", () => {
    fc.assert(
      fc.property(
        fc.nat(),
        (completed) => {
          expect(computeSuccessRate(completed, 0)).toBe(0);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("returns Math.round((completed / total_terminal) * 100) when total_terminal > 0", () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 1, max: 10000 }).chain((totalTerminal) =>
          fc.tuple(
            fc.nat({ max: totalTerminal }),
            fc.constant(totalTerminal)
          )
        ),
        ([completed, totalTerminal]) => {
          const expected = Math.round((completed / totalTerminal) * 100);
          expect(computeSuccessRate(completed, totalTerminal)).toBe(expected);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("result is always in [0, 100]", () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 1, max: 10000 }).chain((totalTerminal) =>
          fc.tuple(
            fc.nat({ max: totalTerminal }),
            fc.constant(totalTerminal)
          )
        ),
        ([completed, totalTerminal]) => {
          const result = computeSuccessRate(completed, totalTerminal);
          expect(result).toBeGreaterThanOrEqual(0);
          expect(result).toBeLessThanOrEqual(100);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("result is always an integer", () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 1, max: 10000 }).chain((totalTerminal) =>
          fc.tuple(
            fc.nat({ max: totalTerminal }),
            fc.constant(totalTerminal)
          )
        ),
        ([completed, totalTerminal]) => {
          const result = computeSuccessRate(completed, totalTerminal);
          expect(Number.isInteger(result)).toBe(true);
        }
      ),
      { numRuns: 100 }
    );
  });
});

// Feature: agent-detail-ui, Property 8: Custom args space-splitting produces correct tokens
/**
 * Property 8: Custom args space-splitting produces correct tokens
 *
 * For any non-empty string s, splitting by whitespace SHALL produce an array of
 * non-empty tokens where joining them with a single space produces a string equal
 * to s.trim().replace(/\s+/g, ' '). Empty strings SHALL produce an empty array.
 *
 * **Validates: Requirements 15.6**
 */
describe("splitArgs — Property 8: Custom args space-splitting produces correct tokens", () => {
  it("empty string produces empty array", () => {
    fc.assert(
      fc.property(fc.constant(""), (s) => {
        expect(splitArgs(s)).toEqual([]);
      }),
      { numRuns: 100 }
    );
  });

  it("whitespace-only string produces empty array", () => {
    fc.assert(
      fc.property(
        fc.array(fc.constantFrom(" ", "\t", "\n", "\r"), { minLength: 1, maxLength: 20 }).map(
          (chars) => chars.join("")
        ),
        (s) => {
          expect(splitArgs(s)).toEqual([]);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("for any non-empty string: joining result with single space equals s.trim().replace(/\\s+/g, ' ')", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1 }).filter((s) => s.trim().length > 0),
        (s) => {
          const result = splitArgs(s);
          const joined = result.join(" ");
          const expected = s.trim().replace(/\s+/g, " ");
          expect(joined).toBe(expected);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("every token in the result is non-empty (no empty strings in output)", () => {
    fc.assert(
      fc.property(fc.string(), (s) => {
        const result = splitArgs(s);
        for (const token of result) {
          expect(token.length).toBeGreaterThan(0);
        }
      }),
      { numRuns: 100 }
    );
  });

  it("result length equals number of whitespace-separated tokens", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1 }).filter((s) => s.trim().length > 0),
        (s) => {
          const result = splitArgs(s);
          const expectedTokens = s.trim().split(/\s+/);
          expect(result.length).toBe(expectedTokens.length);
        }
      ),
      { numRuns: 100 }
    );
  });
});
