import {
  useQuery,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { apiFetch } from "../lib/api";
import type { Agent, AgentStats, PaginatedTasks } from "../lib/agent-detail-types";

/**
 * Fetch a single agent by ID.
 * GET /api/agents/:id
 */
export function useAgent(id: string) {
  return useQuery<Agent>({
    queryKey: ["agents", id],
    queryFn: () => apiFetch<Agent>(`/api/agents/${id}`),
    enabled: !!id,
  });
}

/**
 * Fetch agent's 30-day stats.
 * GET /api/agents/:id/stats
 */
export function useAgentStats(agentId: string) {
  return useQuery<AgentStats>({
    queryKey: ["agent-stats", agentId],
    queryFn: () => apiFetch<AgentStats>(`/api/agents/${agentId}/stats`),
    enabled: !!agentId,
  });
}

/**
 * Fetch tasks for a specific agent (paginated).
 * GET /api/tasks?agent_id=<agentId>&limit=<limit>&offset=<offset>
 */
export function useAgentTasks(
  agentId: string,
  opts: { limit: number; offset: number }
) {
  return useQuery<PaginatedTasks>({
    queryKey: ["agent-tasks", agentId, opts],
    queryFn: () =>
      apiFetch<PaginatedTasks>(
        `/api/tasks?agent_id=${agentId}&limit=${opts.limit}&offset=${opts.offset}`
      ),
    enabled: !!agentId,
  });
}

/**
 * Optimistic update mutation for agent properties.
 * PUT /api/agents/:id
 *
 * Uses onMutate/onError rollback pattern for instant UI feedback.
 */
export function useUpdateAgent(agentId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: Partial<Agent>) =>
      apiFetch<Agent>(`/api/agents/${agentId}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    onMutate: async (data) => {
      await queryClient.cancelQueries({ queryKey: ["agents", agentId] });
      const previous = queryClient.getQueryData<Agent>(["agents", agentId]);
      queryClient.setQueryData<Agent>(
        ["agents", agentId],
        (old) => (old ? { ...old, ...data } : old) as Agent
      );
      return { previous };
    },
    onError: (_err, _data, context) => {
      if (context?.previous) {
        queryClient.setQueryData(["agents", agentId], context.previous);
      }
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: ["agents", agentId] });
      void queryClient.invalidateQueries({ queryKey: ["agents"] });
    },
  });
}

/**
 * Delete agent mutation with cache invalidation.
 * DELETE /api/agents/:id
 */
export function useDeleteAgent() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/agents/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["agents"] });
    },
  });
}
