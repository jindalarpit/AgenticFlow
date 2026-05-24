// Feature: task-tool-chain-ui, Property 7: Filter correctness
// **Validates: Requirements 7.1, 7.2**

import { describe, it, expect } from "vitest";
import fc from "fast-check";
import { filterItems } from "../useTimeline";
import type { TimelineItem, TimelineItemType } from "../../lib/tool-chain-parser";

/**
 * Property 7: Filter correctness
 *
 * For any set of TimelineItems and any non-empty set of active filter values,
 * the filtered output SHALL contain exactly those items whose type or
 * `tool:${toolName}` key is in the filter set — no matching items are excluded
 * and no non-matching items are included.
 */

// ─── Generators ──────────────────────────────────────────────────────────────

/** Valid timeline item types */
const validTypes: TimelineItemType[] = [
  "tool_use",
  "tool_result",
  "thinking",
  "text",
  "error",
];

/** Generate a tool name */
const arbToolName = fc
  .string({ minLength: 1, maxLength: 30 })
  .filter((s) => s.trim().length > 0);

/** Generate a single TimelineItem with a given seq */
function arbTimelineItem(seq: number): fc.Arbitrary<TimelineItem> {
  return fc.constantFrom(...validTypes).chain((type) => {
    if (type === "tool_use" || type === "tool_result") {
      // Items with tool names
      return arbToolName.map((tool) => ({
        seq,
        type,
        tool,
        ...(type === "tool_use" ? { input: {} } : { output: "result" }),
      }));
    }
    // Items without tool names
    return fc.constant({
      seq,
      type,
      content: `content-${seq}`,
    } as TimelineItem);
  });
}

/** Generate a non-empty array of TimelineItems with sequential seq values */
const arbTimelineItems: fc.Arbitrary<TimelineItem[]> = fc
  .integer({ min: 1, max: 30 })
  .chain((length) => {
    const arbs = Array.from({ length }, (_, i) => arbTimelineItem(i));
    return fc.tuple(...arbs);
  })
  .map((items) => [...items]);

/**
 * Generate a non-empty filter set that contains valid filter values
 * (type names and/or `tool:${toolName}` keys).
 */
const arbFilterSet: fc.Arbitrary<Set<string>> = fc
  .array(
    fc.oneof(
      // Type-based filters
      fc.constantFrom(...validTypes) as fc.Arbitrary<string>,
      // Tool-specific filters
      arbToolName.map((name) => `tool:${name}`)
    ),
    { minLength: 1, maxLength: 8 }
  )
  .map((values) => new Set(values));

/**
 * Generate a filter set derived from the actual items (ensures some matches).
 * This creates more interesting test cases where filters actually match items.
 */
function arbFilterSetFromItems(items: TimelineItem[]): fc.Arbitrary<Set<string>> {
  // Collect all possible filter values from the items
  const possibleValues: string[] = [];
  const seenTypes = new Set<string>();
  const seenTools = new Set<string>();

  for (const item of items) {
    if (!seenTypes.has(item.type)) {
      seenTypes.add(item.type);
      possibleValues.push(item.type);
    }
    if (item.tool && !seenTools.has(item.tool)) {
      seenTools.add(item.tool);
      possibleValues.push(`tool:${item.tool}`);
    }
  }

  if (possibleValues.length === 0) {
    // Fallback: generate a random filter
    return arbFilterSet;
  }

  // Pick a non-empty subset of the possible values
  return fc
    .subarray(possibleValues, { minLength: 1 })
    .map((values) => new Set(values));
}

// ─── Helper: reference implementation of filter logic ────────────────────────

/**
 * Reference implementation: determines if an item matches the filter set.
 * An item matches if its type is in the filter set OR `tool:${item.tool}` is in the set.
 */
function itemMatchesFilter(item: TimelineItem, filters: Set<string>): boolean {
  if (filters.has(item.type)) {
    return true;
  }
  if (item.tool && filters.has(`tool:${item.tool}`)) {
    return true;
  }
  return false;
}

// ─── Property Tests ──────────────────────────────────────────────────────────

describe("Property 7: Filter correctness", () => {
  // ─── Sub-property: Every item in filtered output matches a filter criterion ─

  it("every item in the filtered output matches at least one filter criterion", () => {
    fc.assert(
      fc.property(arbTimelineItems, arbFilterSet, (items, filters) => {
        const filtered = filterItems(items, filters);

        for (const item of filtered) {
          const matchesType = filters.has(item.type);
          const matchesTool = item.tool
            ? filters.has(`tool:${item.tool}`)
            : false;
          expect(matchesType || matchesTool).toBe(true);
        }
      }),
      { numRuns: 150 }
    );
  });

  // ─── Sub-property: No matching items are excluded ────────────────────────

  it("every item in the original array that matches a filter is present in the output", () => {
    fc.assert(
      fc.property(arbTimelineItems, arbFilterSet, (items, filters) => {
        const filtered = filterItems(items, filters);

        // Find all items that should match
        const expectedMatches = items.filter((item) =>
          itemMatchesFilter(item, filters)
        );

        // Every expected match must be in the filtered output
        expect(filtered.length).toBe(expectedMatches.length);
        for (const expected of expectedMatches) {
          const found = filtered.some((f) => f.seq === expected.seq);
          expect(found).toBe(true);
        }
      }),
      { numRuns: 150 }
    );
  });

  // ─── Sub-property: When filters is empty, all items are returned ─────────

  it("when filters is empty, all items are returned", () => {
    fc.assert(
      fc.property(arbTimelineItems, (items) => {
        const emptyFilters = new Set<string>();
        const filtered = filterItems(items, emptyFilters);

        expect(filtered.length).toBe(items.length);
        // Items should be the same (identity or equal)
        for (let i = 0; i < items.length; i++) {
          expect(filtered[i]).toBe(items[i]);
        }
      }),
      { numRuns: 100 }
    );
  });

  // ─── Sub-property: Filtered output is a subset of original items ─────────

  it("the filtered output is a subset of the original items (no new items created)", () => {
    fc.assert(
      fc.property(arbTimelineItems, arbFilterSet, (items, filters) => {
        const filtered = filterItems(items, filters);

        // Every item in filtered must exist in the original array (by reference)
        for (const filteredItem of filtered) {
          const existsInOriginal = items.some(
            (item) => item === filteredItem
          );
          expect(existsInOriginal).toBe(true);
        }
      }),
      { numRuns: 150 }
    );
  });

  // ─── Sub-property: Filter preserves relative order ───────────────────────

  it("filtered output preserves the relative order of items from the original array", () => {
    fc.assert(
      fc.property(arbTimelineItems, arbFilterSet, (items, filters) => {
        const filtered = filterItems(items, filters);

        // Check that seq values in filtered are in the same relative order as in items
        if (filtered.length < 2) return;

        for (let i = 0; i < filtered.length - 1; i++) {
          const idxA = items.findIndex((item) => item === filtered[i]);
          const idxB = items.findIndex((item) => item === filtered[i + 1]);
          expect(idxA).toBeLessThan(idxB);
        }
      }),
      { numRuns: 150 }
    );
  });

  // ─── Sub-property: Filter with derived filter set produces exact matches ──

  it("filtering with values derived from items produces exactly the matching items", () => {
    fc.assert(
      fc.property(
        arbTimelineItems.chain((items) =>
          fc.tuple(fc.constant(items), arbFilterSetFromItems(items))
        ),
        ([items, filters]) => {
          const filtered = filterItems(items, filters);

          // Manually compute expected
          const expected = items.filter((item) =>
            itemMatchesFilter(item, filters)
          );

          expect(filtered.length).toBe(expected.length);
          for (let i = 0; i < expected.length; i++) {
            expect(filtered[i]!.seq).toBe(expected[i]!.seq);
            expect(filtered[i]!.type).toBe(expected[i]!.type);
          }
        }
      ),
      { numRuns: 150 }
    );
  });

  // ─── Sub-property: Filtering by a single type returns only that type ─────

  it("filtering by a single type returns only items of that type", () => {
    fc.assert(
      fc.property(
        arbTimelineItems,
        fc.constantFrom(...validTypes),
        (items, filterType) => {
          const filters = new Set([filterType]);
          const filtered = filterItems(items, filters);

          // All filtered items must have the specified type
          for (const item of filtered) {
            expect(item.type).toBe(filterType);
          }

          // All items of that type must be in the filtered output
          const expectedCount = items.filter(
            (item) => item.type === filterType
          ).length;
          expect(filtered.length).toBe(expectedCount);
        }
      ),
      { numRuns: 100 }
    );
  });

  // ─── Sub-property: Filtering by tool name returns items with that tool ────

  it("filtering by tool:name returns items with that specific tool name", () => {
    fc.assert(
      fc.property(
        arbTimelineItems,
        arbToolName,
        (items, toolName) => {
          const filters = new Set([`tool:${toolName}`]);
          const filtered = filterItems(items, filters);

          // All filtered items must have the specified tool name
          for (const item of filtered) {
            expect(item.tool).toBe(toolName);
          }

          // All items with that tool name must be in the filtered output
          const expectedCount = items.filter(
            (item) => item.tool === toolName
          ).length;
          expect(filtered.length).toBe(expectedCount);
        }
      ),
      { numRuns: 100 }
    );
  });
});


// ─── Property 8: Sort direction reversal ─────────────────────────────────────
// Feature: task-tool-chain-ui, Property 8: Sort direction reversal

/**
 * **Validates: Requirements 7.4**
 *
 * For any array of TimelineItems, applying "newest_first" sort SHALL produce
 * the exact reverse of the "chronological" sort order.
 */

import { sortItems } from "../useTimeline";

// ─── Generators (reusing types from Property 7 above) ────────────────────────

/** Generate a TimelineItem with a given seq for sort testing */
function arbTimelineItemForSort(seq: number): fc.Arbitrary<TimelineItem> {
  return fc.constantFrom(...validTypes).chain((type) => {
    if (type === "tool_use" || type === "tool_result") {
      return fc
        .string({ minLength: 1, maxLength: 20 })
        .filter((s) => s.trim().length > 0)
        .map((tool) => ({
          seq,
          type,
          tool,
          ...(type === "tool_use" ? { input: {} } : { output: "result" }),
        }));
    }
    return fc.constant({
      seq,
      type,
      content: `content-${seq}`,
    } as TimelineItem);
  });
}

/** Generate a non-empty array of TimelineItems for sort testing */
const arbTimelineItemsForSort: fc.Arbitrary<TimelineItem[]> = fc
  .integer({ min: 1, max: 50 })
  .chain((length) => {
    const arbs = Array.from({ length }, (_, i) => arbTimelineItemForSort(i));
    return fc.tuple(...arbs);
  })
  .map((items) => [...items]);

/** Generate an array that may be empty for sort testing */
const _arbTimelineItemsMaybeEmptyForSort: fc.Arbitrary<TimelineItem[]> = fc.oneof(
  fc.constant([] as TimelineItem[]),
  arbTimelineItemsForSort
);
void _arbTimelineItemsMaybeEmptyForSort; // suppress unused warning

// ─── Property Tests ──────────────────────────────────────────────────────────

describe("Property 8: Sort direction reversal", () => {
  // ─── Sub-property: newest_first is exact reverse of chronological ────────

  it('"newest_first" produces the exact reverse of "chronological" order', () => {
    fc.assert(
      fc.property(arbTimelineItemsForSort, (items) => {
        const chronological = sortItems(items, "chronological");
        const newestFirst = sortItems(items, "newest_first");

        // newest_first should be the exact reverse of chronological
        expect(newestFirst.length).toBe(chronological.length);
        for (let i = 0; i < chronological.length; i++) {
          expect(newestFirst[i]).toEqual(
            chronological[chronological.length - 1 - i]
          );
        }
      }),
      { numRuns: 150 }
    );
  });

  // ─── Sub-property: chronological returns items in same order as input ────

  it('"chronological" returns items in the same order as input', () => {
    fc.assert(
      fc.property(arbTimelineItemsForSort, (items) => {
        const chronological = sortItems(items, "chronological");

        // chronological should preserve the original order
        expect(chronological.length).toBe(items.length);
        for (let i = 0; i < items.length; i++) {
          expect(chronological[i]).toBe(items[i]);
        }
      }),
      { numRuns: 150 }
    );
  });

  // ─── Sub-property: applying newest_first twice returns original order ────

  it("applying newest_first twice returns the original order", () => {
    fc.assert(
      fc.property(arbTimelineItemsForSort, (items) => {
        const firstReverse = sortItems(items, "newest_first");
        const doubleReverse = sortItems(firstReverse, "newest_first");

        // Double reversal should produce the original order
        expect(doubleReverse.length).toBe(items.length);
        for (let i = 0; i < items.length; i++) {
          expect(doubleReverse[i]).toEqual(items[i]);
        }
      }),
      { numRuns: 150 }
    );
  });

  // ─── Sub-property: both sort directions preserve all items ───────────────

  it("both sort directions preserve all items (same length, same elements)", () => {
    fc.assert(
      fc.property(arbTimelineItemsForSort, (items) => {
        const chronological = sortItems(items, "chronological");
        const newestFirst = sortItems(items, "newest_first");

        // Same length
        expect(chronological.length).toBe(items.length);
        expect(newestFirst.length).toBe(items.length);

        // Same elements (by seq) — every item in the input appears in both outputs
        const inputSeqs = new Set(items.map((item) => item.seq));
        const chronoSeqs = new Set(chronological.map((item) => item.seq));
        const newestSeqs = new Set(newestFirst.map((item) => item.seq));

        expect(chronoSeqs).toEqual(inputSeqs);
        expect(newestSeqs).toEqual(inputSeqs);
      }),
      { numRuns: 150 }
    );
  });

  // ─── Sub-property: sort does not mutate the original array ───────────────

  it("sortItems does not mutate the original array", () => {
    fc.assert(
      fc.property(arbTimelineItemsForSort, (items) => {
        // Take a snapshot of the original
        const originalCopy = items.map((item) => ({ ...item }));

        // Apply both sort directions
        sortItems(items, "chronological");
        sortItems(items, "newest_first");

        // Original array should be unchanged
        expect(items.length).toBe(originalCopy.length);
        for (let i = 0; i < items.length; i++) {
          expect(items[i]!.seq).toBe(originalCopy[i]!.seq);
          expect(items[i]!.type).toBe(originalCopy[i]!.type);
        }
      }),
      { numRuns: 100 }
    );
  });

  // ─── Sub-property: empty array produces empty result for both directions ──

  it("empty array produces empty result for both sort directions", () => {
    const chronological = sortItems([], "chronological");
    const newestFirst = sortItems([], "newest_first");

    expect(chronological).toEqual([]);
    expect(newestFirst).toEqual([]);
  });

  // ─── Sub-property: single item produces same result for both directions ──

  it("single item produces same result for both sort directions", () => {
    fc.assert(
      fc.property(arbTimelineItemForSort(0), (item) => {
        const items = [item];
        const chronological = sortItems(items, "chronological");
        const newestFirst = sortItems(items, "newest_first");

        expect(chronological).toHaveLength(1);
        expect(newestFirst).toHaveLength(1);
        expect(chronological[0]).toEqual(newestFirst[0]);
      }),
      { numRuns: 100 }
    );
  });
});
