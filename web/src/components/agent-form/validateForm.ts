import type { AgentFormValues } from "./types";

/**
 * Pure validation function for AgentFormValues.
 *
 * Returns a Record<string, string> where keys are field names and values are
 * error messages. An empty object means the form is valid.
 */
export function validateForm(values: AgentFormValues): Record<string, string> {
  const errors: Record<string, string> = {};

  // Name: required, 1-64 chars
  if (!values.name.trim()) {
    errors.name = "Name is required";
  } else if (values.name.length > 64) {
    errors.name = "Name must be 64 characters or fewer";
  }

  // Description: max 255 chars
  if (values.description.length > 255) {
    errors.description = "Description must be 255 characters or fewer";
  }

  // Runtime mode specific validation
  if (values.runtime_mode === "local") {
    // Runtime: required for local mode
    if (!values.runtime_id) {
      errors.runtime_id = "Runtime is required";
    }
  } else if (values.runtime_mode === "online") {
    // Provider: required for online mode
    if (!values.provider_id) {
      errors.provider_id = "Provider is required";
    }
    // Model: required for online mode
    if (!values.model) {
      errors.model = "Model is required for online agents";
    } else if (values.model.length > 100) {
      errors.model = "Model must be 100 characters or fewer";
    }
  }

  // Max concurrent tasks: 1-20
  if (
    !Number.isInteger(values.max_concurrent_tasks) ||
    values.max_concurrent_tasks < 1 ||
    values.max_concurrent_tasks > 20
  ) {
    errors.max_concurrent_tasks = "Must be between 1 and 20";
  }

  // Custom env: max 20 pairs
  if (Object.keys(values.custom_env).length > 20) {
    errors.custom_env = "Maximum 20 environment variable pairs";
  }

  // MCP config: valid JSON object or null
  if (values.mcp_config !== null) {
    if (
      typeof values.mcp_config !== "object" ||
      Array.isArray(values.mcp_config)
    ) {
      errors.mcp_config = "MCP config must be a valid JSON object";
    }
  }

  return errors;
}

/**
 * Validate a single field. Returns an error string or undefined.
 */
export function validateField(
  field: keyof AgentFormValues,
  value: unknown,
  allValues?: AgentFormValues
): string | undefined {
  switch (field) {
    case "name": {
      const v = value as string;
      if (!v.trim()) return "Name is required";
      if (v.length > 64) return "Name must be 64 characters or fewer";
      return undefined;
    }
    case "description": {
      const v = value as string;
      if (v.length > 255) return "Description must be 255 characters or fewer";
      return undefined;
    }
    case "runtime_id": {
      const v = value as string;
      // Only required in local mode
      if (allValues && allValues.runtime_mode === "online") return undefined;
      if (!v) return "Runtime is required";
      return undefined;
    }
    case "provider_id": {
      const v = value as string;
      // Only required in online mode
      if (allValues && allValues.runtime_mode === "local") return undefined;
      if (!v) return "Provider is required";
      return undefined;
    }
    case "model": {
      const v = value as string;
      if (allValues && allValues.runtime_mode === "online") {
        if (!v) return "Model is required for online agents";
        if (v.length > 100) return "Model must be 100 characters or fewer";
      }
      return undefined;
    }
    case "max_concurrent_tasks": {
      const v = value as number;
      if (!Number.isInteger(v) || v < 1 || v > 20)
        return "Must be between 1 and 20";
      return undefined;
    }
    case "custom_env": {
      const v = value as Record<string, string>;
      if (Object.keys(v).length > 20)
        return "Maximum 20 environment variable pairs";
      return undefined;
    }
    case "mcp_config": {
      if (value !== null) {
        if (typeof value !== "object" || Array.isArray(value))
          return "MCP config must be a valid JSON object";
      }
      return undefined;
    }
    default:
      return undefined;
  }
}
