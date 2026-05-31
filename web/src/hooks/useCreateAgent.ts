import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api";
import type { Agent } from "../lib/agent-detail-types";

/**
 * Request body for creating a new agent.
 * Excludes skill_ids which are managed via a separate endpoint.
 */
export interface CreateAgentRequest {
  name: string;
  description: string;
  instructions: string;
  runtime_mode: "local" | "online";
  runtime_id?: string;
  provider_id?: string;
  deliverable_type_id?: string;
  model: string;
  custom_env: Record<string, string>;
  custom_args: string[];
  max_concurrent_tasks: number;
  visibility: "private" | "shared";
  mcp_config: Record<string, unknown> | null;
}

/**
 * Mutation hook for creating a new agent.
 * POST /api/agents
 *
 * Invalidates the ["agents"] query on success so lists refresh.
 * Returns the created Agent object (with `id`) for subsequent skills call.
 */
export function useCreateAgent() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateAgentRequest) =>
      apiFetch<Agent>("/api/agents", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["agents"] });
    },
  });
}
