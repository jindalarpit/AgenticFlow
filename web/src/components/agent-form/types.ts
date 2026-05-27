/**
 * Type definitions for the shared AgentForm component.
 *
 * Used by both the Create (/agents/new) and Edit (/agents/:id/edit) pages.
 */

/* ─── Form Values ─── */

/** All editable fields for an agent. */
export interface AgentFormValues {
  name: string;
  description: string;
  instructions: string;
  runtime_id: string;
  model: string;
  custom_env: Record<string, string>;
  custom_args: string[];
  max_concurrent_tasks: number;
  visibility: "private" | "shared";
  mcp_config: Record<string, unknown> | null;
  skill_ids: string[];
}

/** Returns default form values for create mode. */
export function defaultFormValues(): AgentFormValues {
  return {
    name: "",
    description: "",
    instructions: "",
    runtime_id: "",
    model: "",
    custom_env: {},
    custom_args: [],
    max_concurrent_tasks: 1,
    visibility: "private",
    mcp_config: null,
    skill_ids: [],
  };
}

/* ─── Form State & Actions ─── */

/** Reducer state for the agent form. */
export interface FormState {
  values: AgentFormValues;
  errors: Partial<Record<string, string>>;
  isDirty: boolean;
  isSubmitting: boolean;
}

/** Discriminated union of all actions the form reducer handles. */
export type FormAction =
  | { type: "SET_FIELD"; field: keyof AgentFormValues; value: unknown }
  | { type: "SET_ERRORS"; errors: Partial<Record<string, string>> }
  | { type: "CLEAR_ERROR"; field: string }
  | { type: "RESET"; values: AgentFormValues }
  | { type: "ADD_ENV_PAIR"; key: string; value: string }
  | { type: "REMOVE_ENV_PAIR"; key: string }
  | { type: "ADD_ARG"; value: string }
  | { type: "REMOVE_ARG"; index: number }
  | { type: "ADD_SKILL"; skillId: string }
  | { type: "REMOVE_SKILL"; skillId: string };

/* ─── Component Props ─── */

/** Props for the shared AgentForm component. */
export interface AgentFormProps {
  mode: "create" | "edit";
  initialValues: AgentFormValues;
  agentId?: string;
  onSuccess: (agent: any) => void;
  onCancel: () => void;
}

/** Props for the RuntimeSelector sub-component. */
export interface RuntimeSelectorProps {
  value: string;
  onChange: (runtimeId: string) => void;
  error?: string;
}

/** Props for the ModelDropdown sub-component. */
export interface ModelDropdownProps {
  runtimeId: string;
  value: string;
  onChange: (model: string) => void;
  error?: string;
}

/* ─── Data Models ─── */

/** A runtime flattened from the nested daemon → agent_runtimes structure. */
export interface FlatRuntime {
  id: string;
  name: string;
  provider: string;
  status: string;
  daemon_id: string;
  daemon_status: "online" | "offline";
  daemon_device_name: string;
}
