// Feature: agent-management-ui, Property 4: Sorting correctness
// **Validates: Requirements 4.2, 4.3, 4.4, 4.5**

import { describe, it, expect } from "vitest";
import fc from "fast-check";
import { sortAgents } from "../agent-sorting";
import type { AgentActivity, SortableAgent, SortKey } from "../agent-sorting";

/* ─── Generators ─── */

/** Generates a random ISO date string within a reasonable range. */
const arbIsoDate = fc
  .integer({
    min: new Date("2020-01-01T00:00:00Z").getTime(),
    max: new Date("2026-01-01T00:00:00Z").getTime(),
  })
  .map((ts) => new Date(ts).toISOString());

/** Generates a random agent with a unique ID, random name, and random created_at. */
const arbAgent: fc.Arbitrary<SortableAgent> = fc.record({
  id: fc.uuid(),
  name: fc.string({ minLength: 1, maxLength: 64 }),
  created_at: arbIsoDate,
});

/** Generates a list of agents with unique IDs. */
const arbAgentList = fc.array(arbAgent, { minLength: 0, maxLength: 30 });

/** Generates a random activity map for a list of agents. */
function arbActivityMap(agents: SortableAgent[]): fc.Arbitrary<Map<string, AgentActivity>> {
  if (agents.length === 0) return fc.constant(new Map());
  return fc
    .array(
      fc.array(
        fc.record({ completed: fc.nat({ max: 50 }), failed: fc.nat({ max: 20 }) }),
        { minLength: 0, maxLength: 7 }
      ),
      { minLength: agents.length, maxLength: agents.length }
    )
    .map((bucketArrays) => {
      const map = new Map<string, AgentActivity>();
      agents.forEach((agent, i) => {
        // Randomly include or exclude agents from the map (some may have no activity data)
        if (bucketArrays[i]!.length > 0) {
          map.set(agent.id, { buckets: bucketArrays[i]! });
        }
      });
      return map;
    });
}

/** Generates a random run count map for a list of agents. */
function arbRunCountMap(agents: SortableAgent[]): fc.Arbitrary<Map<string, number>> {
  if (agents.length === 0) return fc.constant(new Map());
  return fc
    .array(fc.nat({ max: 500 }), { minLength: agents.length, maxLength: agents.length })
    .map((counts) => {
      const map = new Map<string, number>();
      agents.forEach((agent, i) => {
        map.set(agent.id, counts[i]!);
      });
      return map;
    });
}

/* ─── Helpers ─── */

function getActivitySum(activityMap: Map<string, AgentActivity>, agentId: string): number {
  const activity = activityMap.get(agentId);
  if (!activity) return 0;
  return activity.buckets.reduce((sum, b) => sum + b.completed + b.failed, 0);
}

/**
 * Checks that every adjacent pair in the sorted array satisfies the ordering invariant.
 */
function checkAdjacentPairs<T>(arr: T[], compareFn: (a: T, b: T) => boolean): boolean {
  for (let i = 0; i < arr.length - 1; i++) {
    if (!compareFn(arr[i]!, arr[i + 1]!)) {
      return false;
    }
  }
  return true;
}

/* ─── Property Tests ─── */

describe("sortAgents — Property 4: Sorting correctness", () => {
  it('sort by "name": for every adjacent pair (a, b), a.name.localeCompare(b.name) <= 0', () => {
    fc.assert(
      fc.property(arbAgentList, (agents) => {
        const activityMap = new Map<string, AgentActivity>();
        const runCountMap = new Map<string, number>();
        const sorted = sortAgents(agents, "name", activityMap, runCountMap);

        expect(
          checkAdjacentPairs(sorted, (a, b) => a.name.localeCompare(b.name) <= 0)
        ).toBe(true);
      }),
      { numRuns: 100 }
    );
  });

  it('sort by "runs": for every adjacent pair (a, b), runCount(a) >= runCount(b)', () => {
    fc.assert(
      fc.property(
        arbAgentList.chain((agents) =>
          fc.tuple(fc.constant(agents), arbRunCountMap(agents))
        ),
        ([agents, runCountMap]) => {
          const activityMap = new Map<string, AgentActivity>();
          const sorted = sortAgents(agents, "runs", activityMap, runCountMap);

          expect(
            checkAdjacentPairs(sorted, (a, b) => {
              const runsA = runCountMap.get(a.id) ?? 0;
              const runsB = runCountMap.get(b.id) ?? 0;
              return runsA >= runsB;
            })
          ).toBe(true);
        }
      ),
      { numRuns: 100 }
    );
  });

  it('sort by "created": for every adjacent pair (a, b), a.created_at >= b.created_at', () => {
    fc.assert(
      fc.property(arbAgentList, (agents) => {
        const activityMap = new Map<string, AgentActivity>();
        const runCountMap = new Map<string, number>();
        const sorted = sortAgents(agents, "created", activityMap, runCountMap);

        expect(
          checkAdjacentPairs(sorted, (a, b) => a.created_at >= b.created_at)
        ).toBe(true);
      }),
      { numRuns: 100 }
    );
  });

  it('sort by "recent": for every adjacent pair (a, b), activitySum(a) > activitySum(b) OR (equal AND runCount(a) >= runCount(b)) OR (both equal AND a.created_at >= b.created_at)', () => {
    fc.assert(
      fc.property(
        arbAgentList.chain((agents) =>
          fc.tuple(
            fc.constant(agents),
            arbActivityMap(agents),
            arbRunCountMap(agents)
          )
        ),
        ([agents, activityMap, runCountMap]) => {
          const sorted = sortAgents(agents, "recent", activityMap, runCountMap);

          expect(
            checkAdjacentPairs(sorted, (a, b) => {
              const actA = getActivitySum(activityMap, a.id);
              const actB = getActivitySum(activityMap, b.id);

              if (actA > actB) return true;
              if (actA < actB) return false;

              // Activity sums are equal — check run count
              const runsA = runCountMap.get(a.id) ?? 0;
              const runsB = runCountMap.get(b.id) ?? 0;

              if (runsA > runsB) return true;
              if (runsA < runsB) return false;

              // Both equal — check created_at
              return a.created_at >= b.created_at;
            })
          ).toBe(true);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("sorted output has the same length as input for all sort keys", () => {
    const arbSortKey: fc.Arbitrary<SortKey> = fc.constantFrom("name", "runs", "created", "recent");

    fc.assert(
      fc.property(
        arbAgentList,
        arbSortKey,
        (agents, sortKey) => {
          const activityMap = new Map<string, AgentActivity>();
          const runCountMap = new Map<string, number>();
          const sorted = sortAgents(agents, sortKey, activityMap, runCountMap);
          expect(sorted.length).toBe(agents.length);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("sorted output contains the same elements as input (no additions or removals)", () => {
    const arbSortKey: fc.Arbitrary<SortKey> = fc.constantFrom("name", "runs", "created", "recent");

    fc.assert(
      fc.property(
        arbAgentList.chain((agents) =>
          fc.tuple(
            fc.constant(agents),
            arbActivityMap(agents),
            arbRunCountMap(agents),
            arbSortKey
          )
        ),
        ([agents, activityMap, runCountMap, sortKey]) => {
          const sorted = sortAgents(agents, sortKey, activityMap, runCountMap);
          const inputIds = new Set(agents.map((a) => a.id));
          const outputIds = new Set(sorted.map((a) => a.id));
          expect(outputIds).toEqual(inputIds);
        }
      ),
      { numRuns: 100 }
    );
  });
});
