import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../lib/api";

export interface Agent {
  id: string;
  daemon_id: string;
  provider: string;
  name: string;
  version: string;
  binary_path: string;
  status: "available" | "busy" | "unavailable";
  created_at: string;
  updated_at: string;
}

/**
 * Fetch available agent runtimes.
 * GET /api/agents
 */
export function useAgents() {
  return useQuery<Agent[]>({
    queryKey: ["agents"],
    queryFn: () => apiFetch<Agent[]>("/api/agents"),
  });
}
