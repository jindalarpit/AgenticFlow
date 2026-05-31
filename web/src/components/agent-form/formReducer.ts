import type { AgentFormValues, FormState, FormAction } from "./types";

/**
 * Deep equality check for AgentFormValues to determine dirty state.
 */
function isEqual(a: AgentFormValues, b: AgentFormValues): boolean {
  if (a.name !== b.name) return false;
  if (a.description !== b.description) return false;
  if (a.instructions !== b.instructions) return false;
  if (a.runtime_mode !== b.runtime_mode) return false;
  if (a.runtime_id !== b.runtime_id) return false;
  if (a.provider_id !== b.provider_id) return false;
  if (a.deliverable_type_id !== b.deliverable_type_id) return false;
  if (a.model !== b.model) return false;
  if (a.max_concurrent_tasks !== b.max_concurrent_tasks) return false;
  if (a.visibility !== b.visibility) return false;

  // Compare custom_env
  const aEnvKeys = Object.keys(a.custom_env).sort();
  const bEnvKeys = Object.keys(b.custom_env).sort();
  if (aEnvKeys.length !== bEnvKeys.length) return false;
  for (let i = 0; i < aEnvKeys.length; i++) {
    const aKey = aEnvKeys[i]!;
    const bKey = bEnvKeys[i]!;
    if (aKey !== bKey) return false;
    if (a.custom_env[aKey] !== b.custom_env[bKey]) return false;
  }

  // Compare custom_args
  if (a.custom_args.length !== b.custom_args.length) return false;
  for (let i = 0; i < a.custom_args.length; i++) {
    if (a.custom_args[i] !== b.custom_args[i]) return false;
  }

  // Compare skill_ids
  const aSkills = [...a.skill_ids].sort();
  const bSkills = [...b.skill_ids].sort();
  if (aSkills.length !== bSkills.length) return false;
  for (let i = 0; i < aSkills.length; i++) {
    if (aSkills[i] !== bSkills[i]) return false;
  }

  // Compare mcp_config
  if (a.mcp_config === null && b.mcp_config === null) return true;
  if (a.mcp_config === null || b.mcp_config === null) return false;
  return JSON.stringify(a.mcp_config) === JSON.stringify(b.mcp_config);
}

/**
 * Create the initial form state from initial values.
 */
export function createInitialState(initialValues: AgentFormValues): FormState {
  return {
    values: { ...initialValues },
    errors: {},
    isDirty: false,
    isSubmitting: false,
  };
}

/**
 * Form reducer for the AgentForm component.
 *
 * Handles all form state transitions and tracks dirty state by comparing
 * current values against the initial values stored in closure.
 */
export function createFormReducer(initialValues: AgentFormValues) {
  return function formReducer(state: FormState, action: FormAction): FormState {
    switch (action.type) {
      case "SET_FIELD": {
        const newValues = { ...state.values, [action.field]: action.value };
        return {
          ...state,
          values: newValues,
          isDirty: !isEqual(newValues, initialValues),
        };
      }

      case "SET_ERRORS": {
        return { ...state, errors: action.errors };
      }

      case "CLEAR_ERROR": {
        const { [action.field]: _, ...rest } = state.errors;
        return { ...state, errors: rest };
      }

      case "RESET": {
        return {
          values: { ...action.values },
          errors: {},
          isDirty: false,
          isSubmitting: false,
        };
      }

      case "ADD_ENV_PAIR": {
        const newEnv = { ...state.values.custom_env, [action.key]: action.value };
        const newValues = { ...state.values, custom_env: newEnv };
        return {
          ...state,
          values: newValues,
          isDirty: !isEqual(newValues, initialValues),
        };
      }

      case "REMOVE_ENV_PAIR": {
        const { [action.key]: _, ...restEnv } = state.values.custom_env;
        const newValues = { ...state.values, custom_env: restEnv };
        return {
          ...state,
          values: newValues,
          isDirty: !isEqual(newValues, initialValues),
        };
      }

      case "ADD_ARG": {
        const newArgs = [...state.values.custom_args, action.value];
        const newValues = { ...state.values, custom_args: newArgs };
        return {
          ...state,
          values: newValues,
          isDirty: !isEqual(newValues, initialValues),
        };
      }

      case "REMOVE_ARG": {
        const newArgs = state.values.custom_args.filter(
          (_, i) => i !== action.index
        );
        const newValues = { ...state.values, custom_args: newArgs };
        return {
          ...state,
          values: newValues,
          isDirty: !isEqual(newValues, initialValues),
        };
      }

      case "ADD_SKILL": {
        if (state.values.skill_ids.includes(action.skillId)) return state;
        const newSkills = [...state.values.skill_ids, action.skillId];
        const newValues = { ...state.values, skill_ids: newSkills };
        return {
          ...state,
          values: newValues,
          isDirty: !isEqual(newValues, initialValues),
        };
      }

      case "REMOVE_SKILL": {
        const newSkills = state.values.skill_ids.filter(
          (id) => id !== action.skillId
        );
        const newValues = { ...state.values, skill_ids: newSkills };
        return {
          ...state,
          values: newValues,
          isDirty: !isEqual(newValues, initialValues),
        };
      }

      default:
        return state;
    }
  };
}
