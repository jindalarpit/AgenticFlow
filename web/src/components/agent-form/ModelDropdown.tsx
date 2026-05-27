import { useRuntimeModels } from "../../hooks/useRuntimeModels";
import type { ModelDropdownProps } from "./types";

/**
 * ModelDropdown — fetches available models for the selected runtime.
 *
 * States:
 * - No runtime selected: disabled placeholder
 * - Loading: spinner
 * - Populated: <select> with model options
 * - Empty/Error: free-text <input> fallback with warning
 */
export function ModelDropdown({ runtimeId, value, onChange, error }: ModelDropdownProps) {
  const {
    data: models,
    isLoading,
    isError,
  } = useRuntimeModels(runtimeId);

  // No runtime selected
  if (!runtimeId) {
    return (
      <div className="space-y-1">
        <label className="block text-sm font-medium text-gray-700">Model</label>
        <input
          type="text"
          disabled
          placeholder="Select a runtime first"
          className="block w-full rounded-md border border-gray-300 bg-gray-50 px-3 py-2 text-sm text-gray-400"
        />
      </div>
    );
  }

  // Loading state
  if (isLoading) {
    return (
      <div className="space-y-1">
        <label className="block text-sm font-medium text-gray-700">Model</label>
        <div className="flex items-center gap-2">
          <div className="h-10 flex-1 bg-gray-100 rounded-md animate-pulse" />
          <svg
            className="h-4 w-4 animate-spin text-gray-400"
            viewBox="0 0 24 24"
            fill="none"
          >
            <circle
              className="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              strokeWidth="4"
            />
            <path
              className="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
            />
          </svg>
        </div>
      </div>
    );
  }

  // Error or empty models — fallback to free-text input
  if (isError || !models || models.length === 0) {
    return (
      <div className="space-y-1">
        <label htmlFor="model-input" className="block text-sm font-medium text-gray-700">
          Model
        </label>
        <input
          id="model-input"
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder="e.g., claude-sonnet-4-20250514"
          className={`block w-full rounded-md border px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 ${
            error ? "border-red-300 focus:ring-red-500" : "border-gray-300"
          }`}
          aria-invalid={!!error}
          aria-describedby={error ? "model-error" : undefined}
        />
        {isError && (
          <p className="text-xs text-amber-600">
            Could not fetch models. You can type a model name manually.
          </p>
        )}
        {error && (
          <p id="model-error" className="text-sm text-red-600">
            {error}
          </p>
        )}
      </div>
    );
  }

  // Populated — show select dropdown
  return (
    <div className="space-y-1">
      <label htmlFor="model-select" className="block text-sm font-medium text-gray-700">
        Model
      </label>
      <select
        id="model-select"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className={`block w-full rounded-md border px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 ${
          error ? "border-red-300 focus:ring-red-500" : "border-gray-300"
        }`}
        aria-invalid={!!error}
        aria-describedby={error ? "model-error" : undefined}
      >
        <option value="">Select a model…</option>
        {models.map((model) => (
          <option key={model} value={model}>
            {model}
          </option>
        ))}
      </select>
      {error && (
        <p id="model-error" className="text-sm text-red-600">
          {error}
        </p>
      )}
    </div>
  );
}
