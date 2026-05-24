/**
 * Count computation for the Agent List toolbar.
 *
 * Computes visible/total counts and per-scope/per-availability breakdowns
 * used by the toolbar's "visible of total" indicator and availability chip badges.
 *
 * Validates: Requirements 5.3, 17.3, 18.1
 */

import type { AgentListItem, AgentPresenceDetail, AvailabilityFilter } from "./agent-filters";
import { filterByScope, filterBySearch, filterByAvailability } from "./agent-filters";

/* ─── Return Type ─── */

export interface AgentCounts {
  /** Number of agents after all filters (scope + search + availability) */
  visible: number;
  /** Number of agents in the current scope (before search and availability) */
  total: number;
  /** Agent counts per scope option (before search/availability) */
  scopeCounts: { all: number; mine: number };
  /** Agent counts per availability state within the current scope (before search) */
  availabilityCounts: { online: number; unstable: number; offline: number };
}

/* ─── Main Function ─── */

/**
 * Computes all count values needed by the Agent List toolbar.
 *
 * Pipeline:
 *   1. Scope filtering → determines `total` and `scopeCounts`
 *   2. Availability counting → counts agents per availability within scope (before search)
 *   3. Search + availability filtering → determines `visible`
 *
 * @param agents - The full list of agents already filtered by visibility and view (active/archived)
 * @param presenceMap - Map of agent ID → presence detail (availability, workload, etc.)
 * @param scope - Current scope selection ("mine" or "all")
 * @param currentUserId - The current user's ID for ownership filtering
 * @param search - Current search text (empty string means no search filter)
 * @param availabilityFilter - Current availability chip selection
 */
export function computeCounts(
  agents: AgentListItem[],
  presenceMap: Map<string, AgentPresenceDetail>,
  scope: "mine" | "all",
  currentUserId: string,
  search: string,
  availabilityFilter: AvailabilityFilter
): AgentCounts {
  // Step 1: Compute scope counts (before search/availability)
  const allInScope = filterByScope(agents, "all", currentUserId);
  const mineInScope = filterByScope(agents, "mine", currentUserId);

  const scopeCounts = {
    all: allInScope.length,
    mine: mineInScope.length,
  };

  // Step 2: Get agents in the current scope
  const inScope = scope === "mine" ? mineInScope : allInScope;
  const total = inScope.length;

  // Step 3: Compute availability counts within the current scope (before search)
  const availabilityCounts = { online: 0, unstable: 0, offline: 0 };
  for (const agent of inScope) {
    const presence = presenceMap.get(agent.id);
    const availability = presence?.availability ?? "offline";
    availabilityCounts[availability]++;
  }

  // Step 4: Apply search and availability filters to get visible count
  const afterSearch = filterBySearch(inScope, search);
  const afterAvailability = filterByAvailability(afterSearch, presenceMap, availabilityFilter);
  const visible = afterAvailability.length;

  return { visible, total, scopeCounts, availabilityCounts };
}
