import {
  useQuery,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { apiFetch } from "../lib/api";

// --- Types ---

export interface DeliverableType {
  id: string;
  name: string;
  description: string;
  output_format: string;
  is_system: boolean;
  created_at: string;
}

export interface CreateDeliverableTypeRequest {
  name: string;
  description: string;
  output_format: string;
}

export interface UpdateDeliverableTypeRequest {
  name?: string;
  description?: string;
  output_format?: string;
}

// --- Hooks ---

/**
 * Fetch all deliverable types (system + user-created).
 * GET /api/deliverable-types
 */
export function useDeliverableTypes() {
  return useQuery<DeliverableType[]>({
    queryKey: ["deliverable-types"],
    queryFn: () => apiFetch<DeliverableType[]>("/api/deliverable-types"),
  });
}

/**
 * Fetch a single deliverable type by ID.
 * GET /api/deliverable-types/:id
 */
export function useDeliverableType(id: string) {
  return useQuery<DeliverableType>({
    queryKey: ["deliverable-types", id],
    queryFn: () => apiFetch<DeliverableType>(`/api/deliverable-types/${id}`),
    enabled: !!id,
  });
}

/**
 * Create a new deliverable type.
 * POST /api/deliverable-types
 */
export function useCreateDeliverableType() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateDeliverableTypeRequest) =>
      apiFetch<DeliverableType>("/api/deliverable-types", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["deliverable-types"] });
    },
  });
}

/**
 * Update an existing deliverable type.
 * PUT /api/deliverable-types/:id
 */
export function useUpdateDeliverableType(id: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: UpdateDeliverableTypeRequest) =>
      apiFetch<DeliverableType>(`/api/deliverable-types/${id}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["deliverable-types"] });
      void queryClient.invalidateQueries({
        queryKey: ["deliverable-types", id],
      });
    },
  });
}

/**
 * Delete a deliverable type.
 * DELETE /api/deliverable-types/:id
 */
export function useDeleteDeliverableType() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/deliverable-types/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["deliverable-types"] });
    },
  });
}
