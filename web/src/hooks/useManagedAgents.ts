import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../lib/api";

export type AgentStatus = "idle" | "working" | "offline";

export interface ManagedAgent {
  id: string;
  name: string;
  description: string;
  instructions: string;
  avatar_url: string | null;
  runtime_id: string;
  runtime_name?: string;
  custom_env: Record<string, string>;
  custom_args: string[];
  model: string;
  visibility: "private" | "shared";
  status: AgentStatus;
  max_concurrent_tasks: number;
  owner_id: string;
  created_at: string;
  updated_at: string;
}

/**
 * Fetch managed agents (rich agent model from the `agent` table).
 * GET /api/agents
 */
export function useManagedAgents() {
  return useQuery<ManagedAgent[]>({
    queryKey: ["agents"],
    queryFn: () => apiFetch<ManagedAgent[]>("/api/agents"),
  });
}

/**
 * Fetch a single managed agent by ID.
 * GET /api/agents/:id
 */
export function useManagedAgent(id: string) {
  return useQuery<ManagedAgent>({
    queryKey: ["agents", id],
    queryFn: () => apiFetch<ManagedAgent>(`/api/agents/${id}`),
    enabled: !!id,
  });
}
