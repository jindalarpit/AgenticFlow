import {
  useQuery,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { apiFetch } from "../lib/api";

// --- Types ---

export interface Provider {
  id: string;
  name: string;
  provider_type: ProviderType;
  status: ProviderStatus;
  status_message: string | null;
  models: string[];
  created_at: string;
  updated_at: string;
}

export type ProviderType =
  | "openai"
  | "azure_openai"
  | "aws_bedrock"
  | "anthropic"
  | "litellm";

export type ProviderStatus = "active" | "error" | "validating" | "inactive";

export interface CreateProviderRequest {
  name: string;
  provider_type: ProviderType;
  credentials: Record<string, string>;
  models?: string[];
}

export interface UpdateProviderRequest {
  name?: string;
  credentials?: Record<string, string>;
}

// --- Credential field definitions per provider type ---

export interface CredentialFieldDef {
  key: string;
  label: string;
  required: boolean;
  placeholder: string;
  type?: "text" | "password" | "url";
}

export const PROVIDER_TYPE_LABELS: Record<ProviderType, string> = {
  openai: "OpenAI",
  azure_openai: "Azure OpenAI",
  aws_bedrock: "AWS Bedrock",
  anthropic: "Anthropic",
  litellm: "LiteLLM",
};

export const CREDENTIAL_FIELDS: Record<ProviderType, CredentialFieldDef[]> = {
  openai: [
    { key: "api_key", label: "API Key", required: true, placeholder: "sk-...", type: "password" },
    { key: "base_url", label: "Base URL", required: false, placeholder: "https://api.openai.com/v1", type: "url" },
    { key: "organization", label: "Organization", required: false, placeholder: "org-..." },
  ],
  azure_openai: [
    { key: "api_key", label: "API Key", required: true, placeholder: "Your Azure API key", type: "password" },
    { key: "endpoint", label: "Endpoint", required: true, placeholder: "https://myresource.openai.azure.com", type: "url" },
    { key: "api_version", label: "API Version", required: true, placeholder: "2024-02-01" },
    { key: "deployment_name", label: "Deployment Name", required: true, placeholder: "gpt-4" },
  ],
  aws_bedrock: [
    { key: "access_key_id", label: "Access Key ID", required: true, placeholder: "AKIA..." },
    { key: "secret_access_key", label: "Secret Access Key", required: true, placeholder: "Your secret key", type: "password" },
    { key: "region", label: "Region", required: true, placeholder: "us-east-1" },
    { key: "session_token", label: "Session Token", required: false, placeholder: "Optional session token", type: "password" },
  ],
  anthropic: [
    { key: "api_key", label: "API Key", required: true, placeholder: "sk-ant-...", type: "password" },
    { key: "base_url", label: "Base URL", required: false, placeholder: "https://api.anthropic.com/v1", type: "url" },
  ],
  litellm: [
    { key: "api_key", label: "API Key", required: true, placeholder: "Your API key", type: "password" },
    { key: "api_base", label: "API Base URL", required: true, placeholder: "https://my-litellm.example.com", type: "url" },
    { key: "custom_llm_provider", label: "Custom LLM Provider", required: false, placeholder: "Optional provider name" },
  ],
};

// --- Hooks ---

/**
 * Fetch all providers for the current user.
 * GET /api/providers
 */
export function useProviders(status?: ProviderStatus) {
  const params = status ? `?status=${status}` : "";
  return useQuery<Provider[]>({
    queryKey: ["providers", status ?? "all"],
    queryFn: () => apiFetch<Provider[]>(`/api/providers${params}`),
  });
}

/**
 * Fetch a single provider by ID.
 * GET /api/providers/:id
 */
export function useProvider(id: string) {
  return useQuery<Provider>({
    queryKey: ["providers", id],
    queryFn: () => apiFetch<Provider>(`/api/providers/${id}`),
    enabled: !!id,
  });
}

/**
 * Create a new provider.
 * POST /api/providers
 */
export function useCreateProvider() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateProviderRequest) =>
      apiFetch<Provider>("/api/providers", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["providers"] });
    },
  });
}

/**
 * Update an existing provider.
 * PUT /api/providers/:id
 */
export function useUpdateProvider() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateProviderRequest }) =>
      apiFetch<Provider>(`/api/providers/${id}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["providers"] });
    },
  });
}

/**
 * Delete a provider.
 * DELETE /api/providers/:id
 */
export function useDeleteProvider() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/providers/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["providers"] });
    },
  });
}

/**
 * Re-validate a provider's credentials.
 * POST /api/providers/:id/validate
 */
export function useValidateProvider() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<Provider>(`/api/providers/${id}/validate`, { method: "POST" }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["providers"] });
    },
  });
}
