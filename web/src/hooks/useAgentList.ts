import { useQuery, UseQueryResult } from "@tanstack/react-query";
import { apiFetch } from "../lib/api";
import { ManagedAgent } from "./useManagedAgents";

/**
 * Extended agent model that includes archive status.
 * owner_id is already present in ManagedAgent.
 * Used by the Agent List Page for filtering by archived state and scope.
 */
export interface AgentListItem extends ManagedAgent {
  archived_at: string | null;
}

/**
 * Fetch all agents visible to the current user, including archived_at and owner_id fields.
 * Used by the Agent Management UI data table.
 *
 * Query key: ["agents"]
 * Endpoint: GET /api/agents
 */
export function useAgentList(): UseQueryResult<AgentListItem[]> {
  return useQuery<AgentListItem[]>({
    queryKey: ["agents"],
    queryFn: () => apiFetch<AgentListItem[]>("/api/agents"),
  });
}
