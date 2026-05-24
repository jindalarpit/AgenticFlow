/**
 * Pure filtering functions for the Agent List Page.
 *
 * Each function takes an array of agents (plus filter criteria) and returns
 * a new filtered array without mutating the input.
 */

/* ─── Types ─── */

/** Minimal agent shape needed by the filtering functions. */
export interface AgentListItem {
  id: string;
  name: string;
  description: string;
  owner_id: string;
  visibility: "private" | "shared";
  archived_at: string | null;
}

/** Presence detail for an agent, keyed by agent ID in a Map. */
export interface AgentPresenceDetail {
  availability: "online" | "unstable" | "offline";
}

/** Availability filter options. */
export type AvailabilityFilter = "all" | "online" | "unstable" | "offline";

/** Scope filter options. */
export type Scope = "mine" | "all";

/* ─── Functions ─── */

/**
 * Filters agents by a case-insensitive search string matched against name or description.
 * Returns all agents when search is empty.
 *
 * Property 1: For any non-empty search, the result contains exactly those agents
 * whose name or description includes the search string (case-insensitive).
 */
export function filterBySearch<T extends Pick<AgentListItem, "name" | "description">>(
  agents: T[],
  search: string
): T[] {
  if (search === "") return agents;
  const lower = search.toLowerCase();
  return agents.filter(
    (agent) =>
      agent.name.toLowerCase().includes(lower) ||
      agent.description.toLowerCase().includes(lower)
  );
}

/**
 * Filters agents by ownership scope.
 * "all" returns all agents; "mine" returns only agents owned by the current user.
 *
 * Property 2: When scope is "mine", result contains only agents where
 * owner_id === currentUserId. When scope is "all", result equals the input.
 */
export function filterByScope<T extends Pick<AgentListItem, "owner_id">>(
  agents: T[],
  scope: Scope,
  currentUserId: string
): T[] {
  if (scope === "all") return agents;
  return agents.filter((agent) => agent.owner_id === currentUserId);
}

/**
 * Filters agents by their availability status using a presence map.
 * "all" returns all agents; specific values return only matching agents.
 *
 * Property 3: When filter is not "all", result contains exactly those agents
 * whose presence availability matches the filter. Agents without presence data
 * are excluded unless filter is "all".
 */
export function filterByAvailability<T extends Pick<AgentListItem, "id">>(
  agents: T[],
  presenceMap: Map<string, AgentPresenceDetail>,
  filter: AvailabilityFilter
): T[] {
  if (filter === "all") return agents;
  return agents.filter((agent) => {
    const presence = presenceMap.get(agent.id);
    return presence?.availability === filter;
  });
}

/**
 * Filters out private agents owned by other users, unless the current user is an admin.
 * Admins see all agents regardless of visibility.
 *
 * Property 5: Non-admin users cannot see agents where visibility === "private"
 * AND owner_id !== currentUserId. Admins see all agents.
 */
export function filterByVisibility<
  T extends Pick<AgentListItem, "visibility" | "owner_id">
>(agents: T[], currentUserId: string, isAdmin: boolean): T[] {
  if (isAdmin) return agents;
  return agents.filter(
    (agent) =>
      agent.visibility !== "private" || agent.owner_id === currentUserId
  );
}
