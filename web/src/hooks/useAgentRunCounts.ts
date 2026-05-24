import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { apiFetch } from "../lib/api";

interface AgentRunCountResponse {
  agent_id: string;
  run_count: number;
}

/**
 * Fetch 30-day run counts for all visible agents.
 * GET /api/agents/run-counts
 *
 * Returns a Map keyed by agent_id → run_count.
 * Gracefully degrades on failure: returns an empty map instead of throwing.
 */
export function useAgentRunCounts(): UseQueryResult<Map<string, number>> {
  return useQuery<Map<string, number>>({
    queryKey: ["agents", "run-counts"],
    queryFn: async () => {
      try {
        const data = await apiFetch<AgentRunCountResponse[]>(
          "/api/agents/run-counts"
        );
        const map = new Map<string, number>();
        for (const item of data) {
          map.set(item.agent_id, item.run_count);
        }
        return map;
      } catch {
        return new Map<string, number>();
      }
    },
  });
}
