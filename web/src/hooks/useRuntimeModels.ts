import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../lib/api";

interface RuntimeModelsResponse {
  models: string[];
}

/**
 * Fetch available models for a given runtime.
 * GET /api/runtimes/:id/models
 */
export function useRuntimeModels(runtimeId: string) {
  return useQuery<string[]>({
    queryKey: ["runtimes", runtimeId, "models"],
    queryFn: async () => {
      const res = await apiFetch<RuntimeModelsResponse>(
        `/api/runtimes/${runtimeId}/models`
      );
      return res.models ?? [];
    },
    enabled: !!runtimeId,
    staleTime: 5 * 60 * 1000, // Models rarely change; cache 5 min
  });
}
