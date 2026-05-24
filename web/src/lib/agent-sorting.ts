/**
 * Pure sorting functions for the Agent List UI.
 *
 * Sorts agents by different criteria: recent activity, name, run count, or creation date.
 * All functions return new arrays without mutating the input.
 */

/* ─── Types ─── */

/** Available sort keys for the agent list. */
export type SortKey = "recent" | "name" | "runs" | "created";

/** A single day's activity bucket for an agent. */
export interface AgentActivityBucket {
  completed: number;
  failed: number;
}

/** 7-day activity data for an agent. */
export interface AgentActivity {
  buckets: AgentActivityBucket[];
}

/** Minimal agent shape required for sorting. */
export interface SortableAgent {
  id: string;
  name: string;
  created_at: string;
}

/* ─── Functions ─── */

/**
 * Computes the total activity sum (completed + failed) across all buckets.
 */
function getActivitySum(activity: AgentActivity | undefined): number {
  if (!activity) return 0;
  return activity.buckets.reduce(
    (sum, bucket) => sum + bucket.completed + bucket.failed,
    0
  );
}

/**
 * Sorts agents by the specified sort key.
 *
 * Returns a new sorted array — the input is never mutated.
 *
 * Sort key behaviors:
 * - "name": ascending alphabetical via localeCompare
 * - "runs": descending by 30-day run count
 * - "created": descending by created_at (newest first)
 * - "recent": primary by activity sum (descending), tiebreak by run count (descending),
 *   then by created_at (newest first)
 */
export function sortAgents<T extends SortableAgent>(
  agents: T[],
  sortKey: SortKey,
  activityMap: Map<string, AgentActivity>,
  runCountMap: Map<string, number>
): T[] {
  const sorted = [...agents];

  switch (sortKey) {
    case "name":
      sorted.sort((a, b) => a.name.localeCompare(b.name));
      break;

    case "runs":
      sorted.sort((a, b) => {
        const runsA = runCountMap.get(a.id) ?? 0;
        const runsB = runCountMap.get(b.id) ?? 0;
        return runsB - runsA;
      });
      break;

    case "created":
      sorted.sort((a, b) => {
        // Newest first (descending)
        return b.created_at.localeCompare(a.created_at);
      });
      break;

    case "recent":
      sorted.sort((a, b) => {
        // Primary: activity sum descending
        const activityA = getActivitySum(activityMap.get(a.id));
        const activityB = getActivitySum(activityMap.get(b.id));
        if (activityA !== activityB) return activityB - activityA;

        // Tiebreak: run count descending
        const runsA = runCountMap.get(a.id) ?? 0;
        const runsB = runCountMap.get(b.id) ?? 0;
        if (runsA !== runsB) return runsB - runsA;

        // Final tiebreak: created_at newest first
        return b.created_at.localeCompare(a.created_at);
      });
      break;
  }

  return sorted;
}
