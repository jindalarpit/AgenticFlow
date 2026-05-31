import {
  useQuery,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { apiFetch } from "../lib/api";
import type { Skill } from "./useSkills";

// --- Types ---

export interface SkillTemplateSummary {
  id: string;
  slug: string;
  name: string;
  description: string;
  category: string;
  version: string;
  icon: string | null;
}

export interface SkillTemplate extends SkillTemplateSummary {
  content: string;
  created_at: string;
  updated_at: string;
}

export interface TemplateOrigin {
  template_slug: string;
  template_version: string;
  instantiated_at: string;
}

// --- Hooks ---

/**
 * Fetch all skill templates, optionally filtered by category.
 * GET /api/skill-templates
 * GET /api/skill-templates?category=:category
 */
export function useSkillTemplates(category?: string) {
  return useQuery<SkillTemplateSummary[]>({
    queryKey: category
      ? ["skill-templates", { category }]
      : ["skill-templates"],
    queryFn: () => {
      const params = category
        ? `?category=${encodeURIComponent(category)}`
        : "";
      return apiFetch<SkillTemplateSummary[]>(
        `/api/skill-templates${params}`
      );
    },
  });
}

/**
 * Fetch a single skill template by slug (includes full content).
 * GET /api/skill-templates/:slug
 */
export function useSkillTemplate(slug: string) {
  return useQuery<SkillTemplate>({
    queryKey: ["skill-templates", slug],
    queryFn: () => apiFetch<SkillTemplate>(`/api/skill-templates/${slug}`),
    enabled: !!slug,
  });
}

/**
 * Instantiate a skill template into the user's personal skill collection.
 * POST /api/skill-templates/:slug/instantiate
 */
export function useInstantiateTemplate() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (slug: string) =>
      apiFetch<Skill>(`/api/skill-templates/${slug}/instantiate`, {
        method: "POST",
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["skill-templates"] });
      void queryClient.invalidateQueries({ queryKey: ["skills"] });
    },
  });
}
