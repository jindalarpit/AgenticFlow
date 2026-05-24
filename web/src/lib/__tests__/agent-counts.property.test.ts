// Feature: agent-management-ui, Property 6: Count accuracy
// **Validates: Requirements 5.3, 17.3, 18.1**

import { describe, it, expect } from "vitest";
import fc from "fast-check";
import { computeCounts } from "../agent-counts";
import {
  filterByScope,
  filterBySearch,
  filterByAvailability,
} from "../agent-filters";
import type {
  AgentListItem,
  AgentPresenceDetail,
} from "../agent-filters";

/* ─── Generators ─── */

const arbAvailability = fc.constantFrom(
  "online" as const,
  "unstable" as const,
  "offline" as const
);

const arbAvailabilityFilter = fc.constantFrom(
  "all" as const,
  "online" as const,
  "unstable" as const,
  "offline" as const
);

const arbScope = fc.constantFrom("mine" as const, "all" as const);

const arbAgent = fc.record({
  id: fc.uuid(),
  name: fc.string({ minLength: 1, maxLength: 64 }),
  description: fc.string({ minLength: 0, maxLength: 255 }),
  owner_id: fc.uuid(),
  visibility: fc.constantFrom("private" as const, "shared" as const),
  archived_at: fc.constant(null as string | null),
});

function arbPresenceMap(
  agents: AgentListItem[]
): fc.Arbitrary<Map<string, AgentPresenceDetail>> {
  if (agents.length === 0) return fc.constant(new Map());
  return fc
    .array(arbAvailability, {
      minLength: agents.length,
      maxLength: agents.length,
    })
    .map((availabilities) => {
      const map = new Map<string, AgentPresenceDetail>();
      agents.forEach((agent, i) => {
        map.set(agent.id, { availability: availabilities[i]! });
      });
      return map;
    });
}

/* ─── Tests ─── */

describe("computeCounts — Property 6: Count accuracy", () => {
  it("visible count equals length of (scope-filtered → search-filtered → availability-filtered) list", () => {
    fc.assert(
      fc.property(
        fc.array(arbAgent, { minLength: 0, maxLength: 30 }),
        arbScope,
        fc.uuid(),
        fc.string({ minLength: 0, maxLength: 10 }),
        arbAvailabilityFilter,
        (agents, scope, currentUserId, search, availabilityFilter) => {
          // Build presence map for all agents
          const presenceMap = new Map<string, AgentPresenceDetail>();
          const availabilities: Array<"online" | "unstable" | "offline"> = [
            "online",
            "unstable",
            "offline",
          ];
          agents.forEach((agent, i) => {
            presenceMap.set(agent.id, {
              availability: availabilities[i % 3]!,
            });
          });

          const result = computeCounts(
            agents,
            presenceMap,
            scope,
            currentUserId,
            search,
            availabilityFilter
          );

          // Manually compute expected visible count
          const inScope = filterByScope(agents, scope, currentUserId);
          const afterSearch = filterBySearch(inScope, search);
          const afterAvailability = filterByAvailability(
            afterSearch,
            presenceMap,
            availabilityFilter
          );

          expect(result.visible).toBe(afterAvailability.length);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("total count equals length of scope-filtered list (before search and availability)", () => {
    fc.assert(
      fc.property(
        fc.array(arbAgent, { minLength: 0, maxLength: 30 }),
        arbScope,
        fc.uuid(),
        fc.string({ minLength: 0, maxLength: 10 }),
        arbAvailabilityFilter,
        (agents, scope, currentUserId, search, availabilityFilter) => {
          const presenceMap = new Map<string, AgentPresenceDetail>();
          const availabilities: Array<"online" | "unstable" | "offline"> = [
            "online",
            "unstable",
            "offline",
          ];
          agents.forEach((agent, i) => {
            presenceMap.set(agent.id, {
              availability: availabilities[i % 3]!,
            });
          });

          const result = computeCounts(
            agents,
            presenceMap,
            scope,
            currentUserId,
            search,
            availabilityFilter
          );

          // Total should be the scope-filtered count (before search/availability)
          const inScope = filterByScope(agents, scope, currentUserId);
          expect(result.total).toBe(inScope.length);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("scopeCounts.all equals agents.length and scopeCounts.mine equals agents filtered to owner_id match", () => {
    fc.assert(
      fc.property(
        fc.array(arbAgent, { minLength: 0, maxLength: 30 }),
        arbScope,
        fc.uuid(),
        fc.string({ minLength: 0, maxLength: 10 }),
        arbAvailabilityFilter,
        (agents, scope, currentUserId, search, availabilityFilter) => {
          const presenceMap = new Map<string, AgentPresenceDetail>();
          agents.forEach((agent) => {
            presenceMap.set(agent.id, { availability: "online" });
          });

          const result = computeCounts(
            agents,
            presenceMap,
            scope,
            currentUserId,
            search,
            availabilityFilter
          );

          // scopeCounts.all should equal total agents (since "all" scope returns everything)
          expect(result.scopeCounts.all).toBe(agents.length);

          // scopeCounts.mine should equal agents owned by currentUserId
          const mineCount = agents.filter(
            (a) => a.owner_id === currentUserId
          ).length;
          expect(result.scopeCounts.mine).toBe(mineCount);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("availabilityCounts[status] equals count of agents in scope with that availability", () => {
    fc.assert(
      fc.property(
        fc.array(arbAgent, { minLength: 0, maxLength: 30 }).chain((agents) =>
          fc.tuple(fc.constant(agents), arbPresenceMap(agents))
        ),
        arbScope,
        fc.uuid(),
        fc.string({ minLength: 0, maxLength: 10 }),
        arbAvailabilityFilter,
        ([agents, presenceMap], scope, currentUserId, search, availabilityFilter) => {
          const result = computeCounts(
            agents,
            presenceMap,
            scope,
            currentUserId,
            search,
            availabilityFilter
          );

          // Compute expected availability counts within the current scope
          const inScope = filterByScope(agents, scope, currentUserId);
          const expectedCounts = { online: 0, unstable: 0, offline: 0 };
          for (const agent of inScope) {
            const presence = presenceMap.get(agent.id);
            const availability = presence?.availability ?? "offline";
            expectedCounts[availability]++;
          }

          expect(result.availabilityCounts.online).toBe(expectedCounts.online);
          expect(result.availabilityCounts.unstable).toBe(
            expectedCounts.unstable
          );
          expect(result.availabilityCounts.offline).toBe(
            expectedCounts.offline
          );
        }
      ),
      { numRuns: 100 }
    );
  });

  it("visible count is always <= total count", () => {
    fc.assert(
      fc.property(
        fc.array(arbAgent, { minLength: 0, maxLength: 30 }).chain((agents) =>
          fc.tuple(fc.constant(agents), arbPresenceMap(agents))
        ),
        arbScope,
        fc.uuid(),
        fc.string({ minLength: 0, maxLength: 10 }),
        arbAvailabilityFilter,
        ([agents, presenceMap], scope, currentUserId, search, availabilityFilter) => {
          const result = computeCounts(
            agents,
            presenceMap,
            scope,
            currentUserId,
            search,
            availabilityFilter
          );

          expect(result.visible).toBeLessThanOrEqual(result.total);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("sum of availabilityCounts equals total count", () => {
    fc.assert(
      fc.property(
        fc.array(arbAgent, { minLength: 0, maxLength: 30 }).chain((agents) =>
          fc.tuple(fc.constant(agents), arbPresenceMap(agents))
        ),
        arbScope,
        fc.uuid(),
        ([ agents, presenceMap], scope, currentUserId) => {
          const result = computeCounts(
            agents,
            presenceMap,
            scope,
            currentUserId,
            "",
            "all"
          );

          const sumAvailability =
            result.availabilityCounts.online +
            result.availabilityCounts.unstable +
            result.availabilityCounts.offline;

          expect(sumAvailability).toBe(result.total);
        }
      ),
      { numRuns: 100 }
    );
  });
});
