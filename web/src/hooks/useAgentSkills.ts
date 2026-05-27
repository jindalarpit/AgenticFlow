import {
  useQuery,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { apiFetch } from "../lib/api";
import type { Skill } from "./useSkills";

// --- Types ---

/** Skill as returned by the agent-skills endpoint (same shape as Skill). */
export type AgentSkill = Skill;

export interface SetAgentSkillsRequest {
  skill_ids: string[];
}

// --- Hooks ---

/**
 * Fetch skills assigned to an agent.
 * GET /api/agents/:id/skills
 */
export function useAgentSkills(agentId: string) {
  return useQuery<AgentSkill[]>({
    queryKey: ["agents", agentId, "skills"],
    queryFn: () => apiFetch<AgentSkill[]>(`/api/agents/${agentId}/skills`),
    enabled: !!agentId,
  });
}

/**
 * Replace all skill associations for an agent.
 * PUT /api/agents/:id/skills
 */
export function useSetAgentSkills(agentId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: SetAgentSkillsRequest) =>
      apiFetch<void>(`/api/agents/${agentId}/skills`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ["agents", agentId, "skills"],
      });
      void queryClient.invalidateQueries({ queryKey: ["agents"] });
    },
  });
}
