import { useMemo } from "react";
import { Link } from "react-router-dom";
import { useProviders, PROVIDER_TYPE_LABELS } from "../../hooks/useProviders";
import { useDeliverableTypes } from "../../hooks/useDeliverableTypes";
import type { Provider } from "../../hooks/useProviders";
import type { DeliverableType } from "../../hooks/useDeliverableTypes";

interface OnlineProviderSectionProps {
  providerId: string;
  model: string;
  deliverableTypeId: string;
  onProviderChange: (providerId: string) => void;
  onModelChange: (model: string) => void;
  onDeliverableTypeChange: (deliverableTypeId: string) => void;
  errors: Partial<Record<string, string>>;
}

/**
 * OnlineProviderSection — renders provider, model, and deliverable type
 * dropdowns for agents in "online" runtime mode.
 *
 * - Provider dropdown: filtered to active providers only
 * - Model dropdown: populated from the selected provider's models array
 * - Deliverable type dropdown: excludes "Code Execution"
 * - Shows a message with link to /providers if no active providers exist
 */
export function OnlineProviderSection({
  providerId,
  model,
  deliverableTypeId,
  onProviderChange,
  onModelChange,
  onDeliverableTypeChange,
  errors,
}: OnlineProviderSectionProps) {
  const { data: providers, isLoading: providersLoading } = useProviders("active");
  const { data: deliverableTypes, isLoading: dtLoading } = useDeliverableTypes();

  // Get models from the selected provider
  const selectedProvider: Provider | undefined = useMemo(() => {
    if (!providers || !providerId) return undefined;
    return providers.find((p) => p.id === providerId);
  }, [providers, providerId]);

  const providerModels: string[] = useMemo(() => {
    return selectedProvider?.models ?? [];
  }, [selectedProvider]);

  // Filter deliverable types: exclude "Code Execution" for online agents
  const filteredDeliverableTypes: DeliverableType[] = useMemo(() => {
    if (!deliverableTypes) return [];
    return deliverableTypes.filter((dt) => dt.name !== "Code Execution");
  }, [deliverableTypes]);

  const activeProviders = providers ?? [];

  return (
    <div className="space-y-4">
      {/* Provider Dropdown */}
      <div className="space-y-1">
        <label
          htmlFor="provider-select"
          className="block text-sm font-medium text-gray-700"
        >
          Provider <span className="text-red-500">*</span>
        </label>

        {providersLoading ? (
          <div className="h-10 bg-gray-100 rounded-md animate-pulse" />
        ) : activeProviders.length === 0 ? (
          <div className="rounded-md bg-amber-50 border border-amber-200 p-3">
            <p className="text-sm text-amber-700">
              No active providers available.{" "}
              <Link
                to="/providers"
                className="font-medium text-amber-800 underline hover:text-amber-900"
              >
                Register a provider
              </Link>{" "}
              to use online mode.
            </p>
          </div>
        ) : (
          <select
            id="provider-select"
            value={providerId}
            onChange={(e) => onProviderChange(e.target.value)}
            className={`block w-full rounded-md border px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 ${
              errors.provider_id
                ? "border-red-300 focus:ring-red-500"
                : "border-gray-300"
            }`}
            aria-invalid={!!errors.provider_id}
            aria-describedby={errors.provider_id ? "provider-error" : undefined}
          >
            <option value="">Select a provider…</option>
            {activeProviders.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} ({PROVIDER_TYPE_LABELS[p.provider_type]})
              </option>
            ))}
          </select>
        )}
        {errors.provider_id && (
          <p id="provider-error" className="text-sm text-red-600">
            {errors.provider_id}
          </p>
        )}
      </div>

      {/* Model Dropdown */}
      <div className="space-y-1">
        <label
          htmlFor="online-model-select"
          className="block text-sm font-medium text-gray-700"
        >
          Model <span className="text-red-500">*</span>
        </label>

        {!providerId ? (
          <input
            type="text"
            disabled
            placeholder="Select a provider first"
            className="block w-full rounded-md border border-gray-300 bg-gray-50 px-3 py-2 text-sm text-gray-400"
          />
        ) : providerModels.length > 0 ? (
          <select
            id="online-model-select"
            value={model}
            onChange={(e) => onModelChange(e.target.value)}
            className={`block w-full rounded-md border px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 ${
              errors.model
                ? "border-red-300 focus:ring-red-500"
                : "border-gray-300"
            }`}
            aria-invalid={!!errors.model}
            aria-describedby={errors.model ? "online-model-error" : undefined}
          >
            <option value="">Select a model…</option>
            {providerModels.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
        ) : (
          <input
            id="online-model-select"
            type="text"
            value={model}
            onChange={(e) => onModelChange(e.target.value)}
            placeholder="e.g., gpt-4o, claude-sonnet-4-20250514"
            maxLength={100}
            className={`block w-full rounded-md border px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 ${
              errors.model
                ? "border-red-300 focus:ring-red-500"
                : "border-gray-300"
            }`}
            aria-invalid={!!errors.model}
            aria-describedby={errors.model ? "online-model-error" : undefined}
          />
        )}
        {!providerId ? null : providerModels.length === 0 ? (
          <p className="text-xs text-amber-600">
            No models discovered for this provider. Type a model name manually.
          </p>
        ) : null}
        {errors.model && (
          <p id="online-model-error" className="text-sm text-red-600">
            {errors.model}
          </p>
        )}
      </div>

      {/* Deliverable Type Dropdown */}
      <div className="space-y-1">
        <label
          htmlFor="deliverable-type-select"
          className="block text-sm font-medium text-gray-700"
        >
          Deliverable Type
        </label>

        {dtLoading ? (
          <div className="h-10 bg-gray-100 rounded-md animate-pulse" />
        ) : (
          <select
            id="deliverable-type-select"
            value={deliverableTypeId}
            onChange={(e) => onDeliverableTypeChange(e.target.value)}
            className="block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="">Default (Chat Completion)</option>
            {filteredDeliverableTypes.map((dt) => (
              <option key={dt.id} value={dt.id}>
                {dt.name}
                {dt.is_system ? " (System)" : ""}
              </option>
            ))}
          </select>
        )}
        <p className="text-xs text-gray-500">
          Defines the output format for this agent. Code Execution is not
          available for online agents.
        </p>
      </div>
    </div>
  );
}
