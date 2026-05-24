// Feature: agent-management-ui, Property 1: Search filtering correctness
// **Validates: Requirements 2.2, 2.3**

import { describe, it, expect } from "vitest";
import fc from "fast-check";
import { filterBySearch } from "../agent-filters";

/* ─── Generators ─── */

/** Generates a minimal agent with random name and description. */
const arbitraryAgent = () =>
  fc.record({
    id: fc.uuid(),
    name: fc.string({ minLength: 0, maxLength: 64 }),
    description: fc.string({ minLength: 0, maxLength: 255 }),
  });

/** Generates a list of agents. */
const arbitraryAgentList = () =>
  fc.array(arbitraryAgent(), { minLength: 0, maxLength: 30 });

/** Generates a non-empty search string. */
const arbitraryNonEmptySearch = () =>
  fc.string({ minLength: 1, maxLength: 20 });

describe("filterBySearch — Property 1: Search filtering correctness", () => {
  it("for any non-empty search, result contains exactly agents whose name or description includes search (case-insensitive)", () => {
    fc.assert(
      fc.property(
        arbitraryAgentList(),
        arbitraryNonEmptySearch(),
        (agents, search) => {
          const result = filterBySearch(agents, search);
          const lower = search.toLowerCase();

          // Expected: agents whose name or description includes the search (case-insensitive)
          const expected = agents.filter(
            (a) =>
              a.name.toLowerCase().includes(lower) ||
              a.description.toLowerCase().includes(lower)
          );

          // Result should contain exactly the expected agents (same length, same elements)
          expect(result.length).toBe(expected.length);
          expect(result).toEqual(expected);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("when search is empty, all agents are returned unchanged", () => {
    fc.assert(
      fc.property(arbitraryAgentList(), (agents) => {
        const result = filterBySearch(agents, "");
        expect(result).toBe(agents); // same reference — no filtering applied
      }),
      { numRuns: 100 }
    );
  });

  it("search is case-insensitive: filtering with search.toUpperCase() yields same results as search.toLowerCase()", () => {
    fc.assert(
      fc.property(
        arbitraryAgentList(),
        fc.string({ minLength: 1, maxLength: 10 }).filter((s) => s.trim().length > 0),
        (agents, search) => {
          const resultUpper = filterBySearch(agents, search.toUpperCase());
          const resultLower = filterBySearch(agents, search.toLowerCase());
          expect(resultUpper).toEqual(resultLower);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("every agent in the result actually matches the search in name or description", () => {
    fc.assert(
      fc.property(
        arbitraryAgentList(),
        arbitraryNonEmptySearch(),
        (agents, search) => {
          const result = filterBySearch(agents, search);
          const lower = search.toLowerCase();

          for (const agent of result) {
            const matches =
              agent.name.toLowerCase().includes(lower) ||
              agent.description.toLowerCase().includes(lower);
            expect(matches).toBe(true);
          }
        }
      ),
      { numRuns: 100 }
    );
  });

  it("no agent excluded from the result would have matched the search", () => {
    fc.assert(
      fc.property(
        arbitraryAgentList(),
        arbitraryNonEmptySearch(),
        (agents, search) => {
          const result = filterBySearch(agents, search);
          const resultIds = new Set(result.map((a) => a.id));
          const lower = search.toLowerCase();

          for (const agent of agents) {
            if (!resultIds.has(agent.id)) {
              const matches =
                agent.name.toLowerCase().includes(lower) ||
                agent.description.toLowerCase().includes(lower);
              expect(matches).toBe(false);
            }
          }
        }
      ),
      { numRuns: 100 }
    );
  });
});
