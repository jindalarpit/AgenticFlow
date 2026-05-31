import { useReducer, useCallback, useRef, useState, useMemo } from "react";

import type { AgentFormProps, AgentFormValues, RuntimeMode } from "./types";
import { createFormReducer, createInitialState } from "./formReducer";
import { validateForm, validateField } from "./validateForm";
import { RuntimeSelector } from "./RuntimeSelector";
import { ModelDropdown } from "./ModelDropdown";
import { KeyValueEditor } from "./KeyValueEditor";
import { ArrayEditor } from "./ArrayEditor";
import { McpConfigEditor } from "./McpConfigEditor";
import { SkillsPicker } from "./SkillsPicker";
import { OnlineProviderSection } from "./OnlineProviderSection";
import { DeliverableTypeSelector } from "./DeliverableTypeSelector";

import { useCreateAgent } from "../../hooks/useCreateAgent";
import { useUpdateAgent } from "../../hooks/useAgentDetail";
import { useSetAgentSkills } from "../../hooks/useAgentSkills";
import { useToast } from "../Toast";

/**
 * AgentForm — shared form component for create and edit modes.
 *
 * Uses useReducer for state management. Renders all sub-components.
 * Implements on-blur field validation, on-submit full validation,
 * submission logic with error handling, and dirty-state navigation guard.
 */
export function AgentForm({
  mode,
  initialValues,
  agentId,
  onSuccess,
  onCancel,
}: AgentFormProps) {
  const { showToast } = useToast();
  const formRef = useRef<HTMLFormElement>(null);

  // Create reducer bound to initial values for dirty tracking
  const reducer = useMemo(() => createFormReducer(initialValues), [initialValues]);
  const [state, dispatch] = useReducer(reducer, createInitialState(initialValues));

  const [isSubmitting, setIsSubmitting] = useState(false);

  // Mutations
  const createAgent = useCreateAgent();
  const updateAgent = useUpdateAgent(agentId || "");
  const setAgentSkills = useSetAgentSkills(agentId || "");

  // ─── Field Handlers ───

  const handleFieldChange = useCallback(
    (field: keyof AgentFormValues, value: unknown) => {
      dispatch({ type: "SET_FIELD", field, value });
      dispatch({ type: "CLEAR_ERROR", field });
    },
    []
  );

  const handleFieldBlur = useCallback(
    (field: keyof AgentFormValues) => {
      const error = validateField(field, state.values[field], state.values);
      if (error) {
        dispatch({ type: "SET_ERRORS", errors: { ...state.errors, [field]: error } });
      } else {
        dispatch({ type: "CLEAR_ERROR", field });
      }
    },
    [state.values, state.errors]
  );

  // ─── Runtime Mode Toggle ───

  const handleRuntimeModeChange = useCallback(
    (newMode: RuntimeMode) => {
      handleFieldChange("runtime_mode", newMode);
      // Clear mode-specific fields when switching
      if (newMode === "online") {
        handleFieldChange("runtime_id", "");
        // Clear runtime_id error
        dispatch({ type: "CLEAR_ERROR", field: "runtime_id" });
      } else {
        handleFieldChange("provider_id", "");
        handleFieldChange("model", "");
        // Clear online-specific errors
        dispatch({ type: "CLEAR_ERROR", field: "provider_id" });
        dispatch({ type: "CLEAR_ERROR", field: "model" });
      }
      // Reset deliverable type when switching modes
      handleFieldChange("deliverable_type_id", "");
    },
    [handleFieldChange]
  );

  // ─── Online Provider Handlers ───

  const handleProviderChange = useCallback(
    (providerId: string) => {
      handleFieldChange("provider_id", providerId);
      // Clear model when provider changes (models list changes)
      handleFieldChange("model", "");
    },
    [handleFieldChange]
  );

  // ─── Submission ───

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();

      // Full validation
      const errors = validateForm(state.values);
      if (Object.keys(errors).length > 0) {
        dispatch({ type: "SET_ERRORS", errors });
        // Scroll to first error
        const firstErrorField = Object.keys(errors)[0];
        const el = formRef.current?.querySelector(`[name="${firstErrorField}"]`);
        el?.scrollIntoView({ behavior: "smooth", block: "center" });
        return;
      }

      setIsSubmitting(true);

      try {
        // Prepare agent data (exclude skill_ids — separate endpoint)
        const { skill_ids, ...agentData } = state.values;

        // Build the payload: only include relevant fields based on runtime_mode
        const payload: Record<string, unknown> = {
          name: agentData.name,
          description: agentData.description,
          instructions: agentData.instructions,
          runtime_mode: agentData.runtime_mode,
          model: agentData.model,
          custom_env: agentData.custom_env,
          custom_args: agentData.custom_args,
          max_concurrent_tasks: agentData.max_concurrent_tasks,
          visibility: agentData.visibility,
          mcp_config: agentData.mcp_config,
        };

        if (agentData.runtime_mode === "local") {
          payload.runtime_id = agentData.runtime_id;
          if (agentData.deliverable_type_id) {
            payload.deliverable_type_id = agentData.deliverable_type_id;
          }
        } else {
          payload.provider_id = agentData.provider_id;
          if (agentData.deliverable_type_id) {
            payload.deliverable_type_id = agentData.deliverable_type_id;
          }
        }

        let savedAgentId = agentId;

        if (mode === "edit" && agentId) {
          // Edit mode: PUT /api/agents/:id
          const updated = await updateAgent.mutateAsync(payload as any);
          savedAgentId = updated.id;
        } else {
          // Create mode: POST /api/agents
          const created = await createAgent.mutateAsync(payload as any);
          savedAgentId = created.id;
        }

        // Set skills if they changed (compare with initial)
        const initialSkillsSorted = [...initialValues.skill_ids].sort().join(",");
        const currentSkillsSorted = [...skill_ids].sort().join(",");

        if (initialSkillsSorted !== currentSkillsSorted && savedAgentId) {
          try {
            // For create mode, we need a fresh mutation with the new ID
            if (mode === "create") {
              await fetch(`/api/agents/${savedAgentId}/skills`, {
                method: "PUT",
                headers: {
                  "Content-Type": "application/json",
                  Authorization: `Bearer ${localStorage.getItem("af_token")}`,
                },
                body: JSON.stringify({ skill_ids }),
              });
            } else {
              await setAgentSkills.mutateAsync({ skill_ids });
            }
          } catch {
            showToast(
              "Agent saved but skills update failed. Please try again.",
              "info"
            );
          }
        }

        // Clear dirty state and notify parent
        dispatch({ type: "RESET", values: state.values });
        showToast(
          mode === "edit" ? "Agent updated successfully" : "Agent created successfully",
          "success"
        );
        onSuccess({ id: savedAgentId });
      } catch (err: unknown) {
        // Handle specific error codes
        const message = err instanceof Error ? err.message : String(err);

        if (message.includes("409") || message.toLowerCase().includes("conflict")) {
          dispatch({
            type: "SET_ERRORS",
            errors: { ...state.errors, name: "Name already taken" },
          });
        } else if (message.includes("422") && message.toLowerCase().includes("code execution")) {
          dispatch({
            type: "SET_ERRORS",
            errors: {
              ...state.errors,
              deliverable_type_id:
                "Online agents cannot use the Code Execution deliverable type",
            },
          });
        } else {
          showToast(
            message || "An error occurred while saving. Please try again.",
            "error"
          );
        }
      } finally {
        setIsSubmitting(false);
      }
    },
    [
      state.values,
      state.errors,
      mode,
      agentId,
      initialValues.skill_ids,
      updateAgent,
      createAgent,
      setAgentSkills,
      showToast,
      onSuccess,
    ]
  );

  // ─── Render ───

  const isOnline = state.values.runtime_mode === "online";

  return (
    <form
      ref={formRef}
      onSubmit={(e) => void handleSubmit(e)}
        className="space-y-6 max-w-2xl"
        noValidate
      >
        {/* Name */}
        <div className="space-y-1">
          <label htmlFor="agent-name" className="block text-sm font-medium text-gray-700">
            Name <span className="text-red-500">*</span>
          </label>
          <input
            id="agent-name"
            name="name"
            type="text"
            value={state.values.name}
            onChange={(e) => handleFieldChange("name", e.target.value)}
            onBlur={() => handleFieldBlur("name")}
            maxLength={64}
            className={`block w-full rounded-md border px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 ${
              state.errors.name ? "border-red-300 focus:ring-red-500" : "border-gray-300"
            }`}
            aria-invalid={!!state.errors.name}
            aria-describedby={state.errors.name ? "name-error" : undefined}
          />
          {state.errors.name && (
            <p id="name-error" className="text-sm text-red-600">{state.errors.name}</p>
          )}
        </div>

        {/* Description */}
        <div className="space-y-1">
          <label htmlFor="agent-description" className="block text-sm font-medium text-gray-700">
            Description
          </label>
          <textarea
            id="agent-description"
            name="description"
            value={state.values.description}
            onChange={(e) => handleFieldChange("description", e.target.value)}
            onBlur={() => handleFieldBlur("description")}
            maxLength={255}
            rows={2}
            className={`block w-full rounded-md border px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 ${
              state.errors.description ? "border-red-300 focus:ring-red-500" : "border-gray-300"
            }`}
            aria-invalid={!!state.errors.description}
            aria-describedby={state.errors.description ? "description-error" : undefined}
          />
          <div className="flex justify-between">
            {state.errors.description && (
              <p id="description-error" className="text-sm text-red-600">{state.errors.description}</p>
            )}
            <span className="text-xs text-gray-400 ml-auto">
              {state.values.description.length}/255
            </span>
          </div>
        </div>

        {/* Instructions */}
        <div className="space-y-1">
          <label htmlFor="agent-instructions" className="block text-sm font-medium text-gray-700">
            Instructions
          </label>
          <textarea
            id="agent-instructions"
            name="instructions"
            value={state.values.instructions}
            onChange={(e) => handleFieldChange("instructions", e.target.value)}
            rows={5}
            placeholder="System prompt for the agent..."
            className="block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>

        {/* Runtime Mode Toggle */}
        <div className="space-y-2">
          <label className="block text-sm font-medium text-gray-700">
            Runtime Mode <span className="text-red-500">*</span>
          </label>
          <div className="flex rounded-md border border-gray-300 overflow-hidden w-fit">
            <button
              type="button"
              onClick={() => handleRuntimeModeChange("local")}
              className={`px-4 py-2 text-sm font-medium transition-colors ${
                !isOnline
                  ? "bg-blue-600 text-white"
                  : "bg-white text-gray-700 hover:bg-gray-50"
              }`}
              aria-pressed={!isOnline}
            >
              Local
            </button>
            <button
              type="button"
              onClick={() => handleRuntimeModeChange("online")}
              className={`px-4 py-2 text-sm font-medium border-l border-gray-300 transition-colors ${
                isOnline
                  ? "bg-blue-600 text-white"
                  : "bg-white text-gray-700 hover:bg-gray-50"
              }`}
              aria-pressed={isOnline}
            >
              Online
            </button>
          </div>
          <p className="text-xs text-gray-500">
            {isOnline
              ? "Online agents use cloud AI providers (OpenAI, Anthropic, etc.) without requiring a local daemon."
              : "Local agents execute tasks via a connected daemon runtime on your machine."}
          </p>
        </div>

        {/* Conditional: Local Mode Fields */}
        {!isOnline && (
          <>
            {/* Runtime */}
            <RuntimeSelector
              value={state.values.runtime_id}
              onChange={(runtimeId) => {
                handleFieldChange("runtime_id", runtimeId);
                // Clear model when runtime changes
                if (runtimeId !== state.values.runtime_id) {
                  handleFieldChange("model", "");
                }
              }}
              error={state.errors.runtime_id}
            />

            {/* Model */}
            <ModelDropdown
              runtimeId={state.values.runtime_id}
              value={state.values.model}
              onChange={(model) => handleFieldChange("model", model)}
              error={state.errors.model}
            />

            {/* Deliverable Type (all types including Code Execution) */}
            <DeliverableTypeSelector
              value={state.values.deliverable_type_id}
              onChange={(dtId) => handleFieldChange("deliverable_type_id", dtId)}
            />
          </>
        )}

        {/* Conditional: Online Mode Fields */}
        {isOnline && (
          <OnlineProviderSection
            providerId={state.values.provider_id}
            model={state.values.model}
            deliverableTypeId={state.values.deliverable_type_id}
            onProviderChange={handleProviderChange}
            onModelChange={(model) => handleFieldChange("model", model)}
            onDeliverableTypeChange={(dtId) =>
              handleFieldChange("deliverable_type_id", dtId)
            }
            errors={state.errors}
          />
        )}

        {/* Max Concurrent Tasks */}
        <div className="space-y-1">
          <label htmlFor="max-concurrent-tasks" className="block text-sm font-medium text-gray-700">
            Max Concurrent Tasks
          </label>
          <input
            id="max-concurrent-tasks"
            name="max_concurrent_tasks"
            type="number"
            min={1}
            max={20}
            value={state.values.max_concurrent_tasks}
            onChange={(e) =>
              handleFieldChange("max_concurrent_tasks", parseInt(e.target.value, 10) || 1)
            }
            onBlur={() => handleFieldBlur("max_concurrent_tasks")}
            className={`block w-32 rounded-md border px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 ${
              state.errors.max_concurrent_tasks
                ? "border-red-300 focus:ring-red-500"
                : "border-gray-300"
            }`}
            aria-invalid={!!state.errors.max_concurrent_tasks}
            aria-describedby={
              state.errors.max_concurrent_tasks ? "max-tasks-error" : undefined
            }
          />
          {state.errors.max_concurrent_tasks && (
            <p id="max-tasks-error" className="text-sm text-red-600">
              {state.errors.max_concurrent_tasks}
            </p>
          )}
        </div>

        {/* Visibility */}
        <div className="space-y-1">
          <label className="block text-sm font-medium text-gray-700">Visibility</label>
          <div className="flex items-center gap-4">
            <label className="inline-flex items-center gap-2 text-sm">
              <input
                type="radio"
                name="visibility"
                value="private"
                checked={state.values.visibility === "private"}
                onChange={() => handleFieldChange("visibility", "private")}
                className="text-blue-600 focus:ring-blue-500"
              />
              Private
            </label>
            <label className="inline-flex items-center gap-2 text-sm">
              <input
                type="radio"
                name="visibility"
                value="shared"
                checked={state.values.visibility === "shared"}
                onChange={() => handleFieldChange("visibility", "shared")}
                className="text-blue-600 focus:ring-blue-500"
              />
              Shared
            </label>
          </div>
        </div>

        {/* Environment Variables */}
        <KeyValueEditor
          label="Environment Variables"
          value={state.values.custom_env}
          onChange={(env) => handleFieldChange("custom_env", env)}
          maxPairs={20}
          error={state.errors.custom_env}
        />

        {/* Custom Arguments */}
        <ArrayEditor
          label="Custom Arguments"
          value={state.values.custom_args}
          onChange={(args) => handleFieldChange("custom_args", args)}
          error={state.errors.custom_args}
        />

        {/* MCP Config */}
        <McpConfigEditor
          value={state.values.mcp_config}
          onChange={(config) => handleFieldChange("mcp_config", config)}
          error={state.errors.mcp_config}
        />

        {/* Skills */}
        <SkillsPicker
          value={state.values.skill_ids}
          onAdd={(skillId) => dispatch({ type: "ADD_SKILL", skillId })}
          onRemove={(skillId) => dispatch({ type: "REMOVE_SKILL", skillId })}
        />

        {/* Actions */}
        <div className="flex items-center gap-3 pt-4 border-t border-gray-200">
          <button
            type="submit"
            disabled={isSubmitting}
            className="inline-flex items-center rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
          >
            {isSubmitting ? (
              <>
                <svg
                  className="mr-2 h-4 w-4 animate-spin"
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
                Saving…
              </>
            ) : mode === "edit" ? (
              "Save Changes"
            ) : (
              "Create Agent"
            )}
          </button>
          <button
            type="button"
            onClick={onCancel}
            className="inline-flex items-center rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
          >
            Cancel
          </button>
        </div>
      </form>
  );
}
