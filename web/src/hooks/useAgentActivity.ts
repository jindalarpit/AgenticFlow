import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { apiFetch } from "../lib/api";

export interface AgentActivityBucket {
  date: string;
  completed: number;
  failed: number;
}

export interface AgentActivity {
  buckets: AgentActivityBucket[];
}

interface AgentActivityResponse {
  agent_id: string;
  buckets: AgentActivityBucket[];
}

/**
 * Fetch 7-day activity data (daily task completion counts) for all visible agents.
 * GET /api/agents/activity
 *
 * Returns a Map keyed by agent_id → AgentActivity.
 * Gracefully degrades on failure: returns an empty map instead of throwing.
 */
export function useAgentActivity(): UseQueryResult<Map<string, AgentActivity>> {
  return useQuery<Map<string, AgentActivity>>({
    queryKey: ["agents", "activity"],
    queryFn: async () => {
      try {
        const data = await apiFetch<AgentActivityResponse[]>(
          "/api/agents/activity"
        );
        const map = new Map<string, AgentActivity>();
        for (const item of data) {
          map.set(item.agent_id, { buckets: item.buckets });
        }
        return map;
      } catch {
        return new Map<string, AgentActivity>();
      }
    },
  });
}
