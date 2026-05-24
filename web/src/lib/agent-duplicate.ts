/**
 * Duplicate pre-population logic for the Create Agent Dialog.
 *
 * Builds a CreateAgentPayload from an existing agent, appending " copy"
 * to the name and forwarding all relevant configuration fields.
 * Optional fields are only included when they have meaningful values.
 */

/* ─── Types ─── */

/**
 * Source agent shape needed for duplication.
 * Matches the AgentListItem fields relevant to creating a copy.
 */
export interface AgentListItem {
  name: string;
  description: string;
  instructions: string;
  runtime_id: string;
  model: string;
  visibility: "private" | "shared";
  avatar_url: string | null;
  custom_env: Record<string, string>;
  custom_args: string[];
  max_concurrent_tasks: number;
}

/**
 * Payload sent to POST /api/agents when creating a new agent.
 * Optional fields are omitted from the payload when they have no meaningful value.
 */
export interface CreateAgentPayload {
  name: string;
  description: string;
  instructions?: string;
  runtime_id: string;
  model?: string;
  visibility: "private" | "shared";
  avatar_url?: string;
  custom_env?: Record<string, string>;
  custom_args?: string[];
  max_concurrent_tasks?: number;
}

/* ─── Functions ─── */

/**
 * Builds a CreateAgentPayload from a source agent for duplication.
 *
 * - Sets name to source.name + " copy"
 * - Copies description, runtime_id, and visibility directly
 * - Includes optional fields (instructions, model, avatar_url) only if non-empty strings
 * - Includes custom_env only if it has at least one key
 * - Includes custom_args only if the array is non-empty
 * - Includes max_concurrent_tasks only if it is a positive number
 *
 * Property 7: payload.name === source.name + " copy"; all config fields forwarded correctly.
 *
 * Validates: Requirements 7.2, 9.12
 */
export function buildDuplicatePayload(source: AgentListItem): CreateAgentPayload {
  const payload: CreateAgentPayload = {
    name: source.name + " copy",
    description: source.description,
    runtime_id: source.runtime_id,
    visibility: source.visibility,
  };

  if (source.instructions && source.instructions.length > 0) {
    payload.instructions = source.instructions;
  }

  if (source.model && source.model.length > 0) {
    payload.model = source.model;
  }

  if (source.avatar_url && source.avatar_url.length > 0) {
    payload.avatar_url = source.avatar_url;
  }

  if (source.custom_env && Object.keys(source.custom_env).length > 0) {
    payload.custom_env = source.custom_env;
  }

  if (source.custom_args && source.custom_args.length > 0) {
    payload.custom_args = source.custom_args;
  }

  if (source.max_concurrent_tasks && source.max_concurrent_tasks > 0) {
    payload.max_concurrent_tasks = source.max_concurrent_tasks;
  }

  return payload;
}
