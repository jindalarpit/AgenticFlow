import {
  useQuery,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { apiFetch } from "../lib/api";

// --- Types ---

export interface SkillFileInput {
  path: string;
  content: string;
}

export interface SkillFileResponse {
  id: string;
  path: string;
  content?: string;
}

export interface Skill {
  id: string;
  name: string;
  description: string;
  content: string;
  config?: Record<string, unknown> | null;
  files: SkillFileResponse[];
  agent_count: number;
  created_at: string;
  updated_at: string;
}

export interface CreateSkillRequest {
  name: string;
  description: string;
  content: string;
  config?: Record<string, unknown> | null;
  files?: SkillFileInput[];
}

export interface UpdateSkillRequest {
  name?: string;
  description?: string;
  content?: string;
  config?: Record<string, unknown> | null;
  files?: SkillFileInput[];
}

export interface ImportSkillRequest {
  url: string;
}

// --- Hooks ---

/**
 * Fetch all skills for the current user.
 * GET /api/skills
 */
export function useSkills() {
  return useQuery<Skill[]>({
    queryKey: ["skills"],
    queryFn: () => apiFetch<Skill[]>("/api/skills"),
  });
}

/**
 * Fetch a single skill by ID (includes full file content).
 * GET /api/skills/:id
 */
export function useSkill(id: string) {
  return useQuery<Skill>({
    queryKey: ["skills", id],
    queryFn: () => apiFetch<Skill>(`/api/skills/${id}`),
    enabled: !!id,
  });
}

/**
 * Create a new skill.
 * POST /api/skills
 */
export function useCreateSkill() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateSkillRequest) =>
      apiFetch<Skill>("/api/skills", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["skills"] });
    },
  });
}

/**
 * Update an existing skill.
 * PUT /api/skills/:id
 */
export function useUpdateSkill(id: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: UpdateSkillRequest) =>
      apiFetch<Skill>(`/api/skills/${id}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["skills"] });
      void queryClient.invalidateQueries({ queryKey: ["skills", id] });
    },
  });
}

/**
 * Delete a skill.
 * DELETE /api/skills/:id
 */
export function useDeleteSkill() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/skills/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["skills"] });
    },
  });
}

/**
 * Import a skill from a URL.
 * POST /api/skills/import
 */
export function useImportSkill() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: ImportSkillRequest) =>
      apiFetch<Skill>("/api/skills/import", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["skills"] });
    },
  });
}
