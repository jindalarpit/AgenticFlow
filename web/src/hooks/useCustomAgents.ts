import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api";

export interface CustomAgent {
  id: string;
  user_id: string;
  name: string;
  command: string;
  args_template: string;
  model_override: string | null;
  env_vars: Record<string, string>;
  created_at: string;
  updated_at: string;
}

interface CreateCustomAgentInput {
  name: string;
  command: string;
  args_template: string;
  model_override?: string;
  env_vars?: Record<string, string>;
}

interface UpdateCustomAgentInput {
  name?: string;
  command?: string;
  args_template?: string;
  model_override?: string;
  env_vars?: Record<string, string>;
}

/**
 * Fetch all custom agents for the current user.
 * GET /api/custom-agents
 */
export function useCustomAgents() {
  return useQuery({
    queryKey: ["custom-agents"],
    queryFn: () => apiFetch<CustomAgent[]>("/api/custom-agents"),
  });
}

/**
 * Create a new custom agent.
 * POST /api/custom-agents
 */
export function useCreateCustomAgent() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateCustomAgentInput) =>
      apiFetch<CustomAgent>("/api/custom-agents", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["custom-agents"] });
    },
  });
}

/**
 * Update an existing custom agent.
 * PUT /api/custom-agents/{id}
 */
export function useUpdateCustomAgent() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateCustomAgentInput }) =>
      apiFetch<CustomAgent>(`/api/custom-agents/${id}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["custom-agents"] });
    },
  });
}

/**
 * Delete a custom agent.
 * DELETE /api/custom-agents/{id}
 */
export function useDeleteCustomAgent() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/custom-agents/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["custom-agents"] });
    },
  });
}
