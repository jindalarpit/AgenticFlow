import { FormEvent, useState, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { useMutation } from "@tanstack/react-query";
import { apiFetch } from "../lib/api";
import { useDaemons } from "../hooks/useDaemons";
import type { AgentRuntime } from "../hooks/useDaemons";
import { useToast } from "../components/Toast";

/* ─── Validation ─── */

const AGENT_NAME_REGEX = /^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$/;

function validateAgentName(name: string): string | null {
  if (!name) return "Name is required";
  if (name.length > 64) return "Name must not exceed 64 characters";
  if (!AGENT_NAME_REGEX.test(name))
    return "Name must start with a letter or number and contain only letters, numbers, hyphens, and underscores";
  return null;
}

function validateDescription(desc: string): string | null {
  if (desc.length > 255) return "Description must not exceed 255 characters";
  return null;
}

function validateInstructions(instructions: string): string | null {
  if (instructions.length > 50000)
    return "Instructions must not exceed 50,000 characters";
  return null;
}

function validateModel(model: string): string | null {
  if (model.length > 100) return "Model must not exceed 100 characters";
  return null;
}

function validateRuntimeId(runtimeId: string): string | null {
  if (!runtimeId) return "Runtime is required";
  return null;
}

function validateMaxConcurrentTasks(value: number): string | null {
  if (!Number.isInteger(value)) return "Must be a whole number";
  if (value < 1) return "Must be at least 1";
  if (value > 20) return "Must not exceed 20";
  return null;
}

function validateEnvKey(key: string): string | null {
  if (!key) return "Key is required";
  if (key.length > 64) return "Key must not exceed 64 characters";
  return null;
}

function validateEnvValue(value: string): string | null {
  if (!value) return "Value is required";
  if (value.length > 1024) return "Value must not exceed 1024 characters";
  return null;
}

function validateCustomArg(arg: string): string | null {
  if (!arg) return "Argument cannot be empty";
  if (arg.length > 256) return "Argument must not exceed 256 characters";
  return null;
}

/* ─── Types ─── */

interface EnvPair {
  key: string;
  value: string;
}

interface CreateAgentPayload {
  name: string;
  description: string;
  instructions: string;
  runtime_id: string;
  model: string;
  custom_env: Record<string, string>;
  custom_args: string[];
  max_concurrent_tasks: number;
  visibility: "private" | "shared";
}

/* ─── Component ─── */

export default function AgentForm() {
  const navigate = useNavigate();
  const { showToast } = useToast();
  const { data: daemons, isLoading: daemonsLoading } = useDaemons();

  // Form state
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [instructions, setInstructions] = useState("");
  const [runtimeId, setRuntimeId] = useState("");
  const [model, setModel] = useState("");
  const [envPairs, setEnvPairs] = useState<EnvPair[]>([]);
  const [customArgs, setCustomArgs] = useState<string[]>([]);
  const [maxConcurrentTasks, setMaxConcurrentTasks] = useState(1);
  const [visibility, setVisibility] = useState<"private" | "shared">("private");

  // Validation state
  const [fieldErrors, setFieldErrors] = useState<Record<string, string | null>>(
    {}
  );
  const [submitError, setSubmitError] = useState<string | null>(null);

  // Filter runtimes to online daemons only
  const onlineRuntimes: (AgentRuntime & { daemon_name: string })[] =
    useMemo(() => {
      if (!daemons) return [];
      return daemons
        .filter((d) => d.status === "online")
        .flatMap((d) =>
          d.agent_runtimes.map((r) => ({ ...r, daemon_name: d.device_name }))
        );
    }, [daemons]);

  // Mutation
  const createAgent = useMutation({
    mutationFn: (payload: CreateAgentPayload) =>
      apiFetch("/api/agents", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      showToast("Agent created successfully", "success");
      navigate("/agents");
    },
    onError: (err: Error) => {
      setSubmitError(err.message || "Failed to create agent");
    },
  });

  /* ─── Field validation on blur ─── */

  function onBlurField(field: string) {
    let err: string | null = null;
    switch (field) {
      case "name":
        err = validateAgentName(name);
        break;
      case "description":
        err = validateDescription(description);
        break;
      case "instructions":
        err = validateInstructions(instructions);
        break;
      case "model":
        err = validateModel(model);
        break;
      case "runtime_id":
        err = validateRuntimeId(runtimeId);
        break;
      case "max_concurrent_tasks":
        err = validateMaxConcurrentTasks(maxConcurrentTasks);
        break;
    }
    setFieldErrors((prev) => ({ ...prev, [field]: err }));
  }

  /* ─── Env Pairs ─── */

  function addEnvPair() {
    if (envPairs.length >= 20) return;
    setEnvPairs([...envPairs, { key: "", value: "" }]);
  }

  function removeEnvPair(index: number) {
    setEnvPairs(envPairs.filter((_, i) => i !== index));
    // Clear related errors
    setFieldErrors((prev) => {
      const next = { ...prev };
      delete next[`env_key_${index}`];
      delete next[`env_value_${index}`];
      return next;
    });
  }

  function updateEnvPair(index: number, field: "key" | "value", val: string) {
    setEnvPairs(
      envPairs.map((pair, i) =>
        i === index ? { ...pair, [field]: val } : pair
      )
    );
  }

  function validateEnvPairField(index: number, field: "key" | "value") {
    const pair = envPairs[index];
    if (!pair) return;
    const err =
      field === "key"
        ? validateEnvKey(pair.key)
        : validateEnvValue(pair.value);
    setFieldErrors((prev) => ({
      ...prev,
      [`env_${field}_${index}`]: err,
    }));
  }

  /* ─── Custom Args ─── */

  function addArg() {
    if (customArgs.length >= 20) return;
    setCustomArgs([...customArgs, ""]);
  }

  function removeArg(index: number) {
    setCustomArgs(customArgs.filter((_, i) => i !== index));
    setFieldErrors((prev) => {
      const next = { ...prev };
      delete next[`arg_${index}`];
      return next;
    });
  }

  function updateArg(index: number, val: string) {
    setCustomArgs(customArgs.map((a, i) => (i === index ? val : a)));
  }

  function validateArgField(index: number) {
    const err = validateCustomArg(customArgs[index] ?? "");
    setFieldErrors((prev) => ({ ...prev, [`arg_${index}`]: err }));
  }

  /* ─── Submit ─── */

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitError(null);

    // Validate all fields
    const errors: Record<string, string | null> = {};
    errors.name = validateAgentName(name);
    errors.description = validateDescription(description);
    errors.instructions = validateInstructions(instructions);
    errors.model = validateModel(model);
    errors.runtime_id = validateRuntimeId(runtimeId);
    errors.max_concurrent_tasks = validateMaxConcurrentTasks(maxConcurrentTasks);

    // Validate env pairs
    envPairs.forEach((pair, i) => {
      errors[`env_key_${i}`] = validateEnvKey(pair.key);
      errors[`env_value_${i}`] = validateEnvValue(pair.value);
    });

    // Validate custom args
    customArgs.forEach((arg, i) => {
      errors[`arg_${i}`] = validateCustomArg(arg);
    });

    setFieldErrors(errors);

    // Check if any errors exist
    const hasErrors = Object.values(errors).some((e) => e !== null);
    if (hasErrors) return;

    // Build payload
    const customEnv: Record<string, string> = {};
    envPairs.forEach((pair) => {
      customEnv[pair.key] = pair.value;
    });

    const payload: CreateAgentPayload = {
      name,
      description,
      instructions,
      runtime_id: runtimeId,
      model,
      custom_env: customEnv,
      custom_args: customArgs,
      max_concurrent_tasks: maxConcurrentTasks,
      visibility,
    };

    createAgent.mutate(payload);
  }

  return (
    <div className="max-w-3xl mx-auto px-6 py-8">
      <div className="mb-6">
        <button
          type="button"
          onClick={() => navigate("/agents")}
          className="text-sm text-gray-500 hover:text-gray-700 mb-2 inline-flex items-center gap-1"
        >
          ← Back to Agents
        </button>
        <h1 className="text-2xl font-semibold text-gray-900">Create Agent</h1>
        <p className="mt-1 text-sm text-gray-500">
          Define a new agent with its configuration, runtime binding, and
          execution parameters.
        </p>
      </div>

      {submitError && (
        <div className="rounded-md bg-red-50 border border-red-200 p-3 text-sm text-red-700 mb-6">
          {submitError}
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-6">
        {/* Name */}
        <div>
          <label
            htmlFor="agent-name"
            className="block text-sm font-medium text-gray-700"
          >
            Name <span className="text-red-500">*</span>
          </label>
          <input
            id="agent-name"
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            onBlur={() => onBlurField("name")}
            maxLength={64}
            placeholder="my-agent"
            className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
          <p className="mt-1 text-xs text-gray-500">
            1–64 characters. Letters, numbers, hyphens, and underscores. Must
            start with a letter or number.
          </p>
          {fieldErrors.name && (
            <p className="mt-1 text-xs text-red-600">{fieldErrors.name}</p>
          )}
        </div>

        {/* Description */}
        <div>
          <label
            htmlFor="agent-description"
            className="block text-sm font-medium text-gray-700"
          >
            Description
          </label>
          <textarea
            id="agent-description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            onBlur={() => onBlurField("description")}
            maxLength={255}
            rows={2}
            placeholder="A brief description of what this agent does"
            className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 resize-none"
          />
          <p className="mt-1 text-xs text-gray-500">
            {description.length}/255 characters
          </p>
          {fieldErrors.description && (
            <p className="mt-1 text-xs text-red-600">
              {fieldErrors.description}
            </p>
          )}
        </div>

        {/* Instructions */}
        <div>
          <label
            htmlFor="agent-instructions"
            className="block text-sm font-medium text-gray-700"
          >
            Instructions
          </label>
          <textarea
            id="agent-instructions"
            value={instructions}
            onChange={(e) => setInstructions(e.target.value)}
            onBlur={() => onBlurField("instructions")}
            rows={8}
            placeholder="System prompt for the agent. Define its persona, capabilities, and behavior..."
            className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 resize-y font-mono"
          />
          <p className="mt-1 text-xs text-gray-500">
            {instructions.length.toLocaleString()}/50,000 characters
          </p>
          {fieldErrors.instructions && (
            <p className="mt-1 text-xs text-red-600">
              {fieldErrors.instructions}
            </p>
          )}
        </div>

        {/* Runtime */}
        <div>
          <label
            htmlFor="agent-runtime"
            className="block text-sm font-medium text-gray-700"
          >
            Runtime <span className="text-red-500">*</span>
          </label>
          <select
            id="agent-runtime"
            value={runtimeId}
            onChange={(e) => setRuntimeId(e.target.value)}
            onBlur={() => onBlurField("runtime_id")}
            className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 bg-white"
          >
            <option value="">Select a runtime...</option>
            {onlineRuntimes.map((rt) => (
              <option key={rt.id} value={rt.id}>
                {rt.name} ({rt.provider}) — {rt.daemon_name}
              </option>
            ))}
          </select>
          {daemonsLoading && (
            <p className="mt-1 text-xs text-gray-500">Loading runtimes...</p>
          )}
          {!daemonsLoading && onlineRuntimes.length === 0 && (
            <p className="mt-1 text-xs text-amber-600">
              No online runtimes available. Ensure a daemon is connected.
            </p>
          )}
          {fieldErrors.runtime_id && (
            <p className="mt-1 text-xs text-red-600">
              {fieldErrors.runtime_id}
            </p>
          )}
        </div>

        {/* Model */}
        <div>
          <label
            htmlFor="agent-model"
            className="block text-sm font-medium text-gray-700"
          >
            Model
          </label>
          <input
            id="agent-model"
            type="text"
            value={model}
            onChange={(e) => setModel(e.target.value)}
            onBlur={() => onBlurField("model")}
            maxLength={100}
            placeholder="e.g., claude-sonnet-4-20250514"
            className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
          <p className="mt-1 text-xs text-gray-500">
            Optional. Overrides the runtime's default model.
          </p>
          {fieldErrors.model && (
            <p className="mt-1 text-xs text-red-600">{fieldErrors.model}</p>
          )}
        </div>

        {/* Custom Environment Variables */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <label className="block text-sm font-medium text-gray-700">
              Custom Environment Variables
            </label>
            <button
              type="button"
              onClick={addEnvPair}
              disabled={envPairs.length >= 20}
              className="text-xs text-blue-600 hover:text-blue-700 disabled:text-gray-400 disabled:cursor-not-allowed"
            >
              + Add Variable
            </button>
          </div>
          {envPairs.length === 0 && (
            <p className="text-xs text-gray-500">
              No environment variables configured. Click "Add Variable" to add
              key-value pairs.
            </p>
          )}
          <div className="space-y-2">
            {envPairs.map((pair, index) => (
              <div key={index} className="flex items-start gap-2">
                <div className="flex-1">
                  <input
                    type="text"
                    value={pair.key}
                    onChange={(e) =>
                      updateEnvPair(index, "key", e.target.value)
                    }
                    onBlur={() => validateEnvPairField(index, "key")}
                    placeholder="KEY"
                    className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 font-mono"
                  />
                  {fieldErrors[`env_key_${index}`] && (
                    <p className="mt-0.5 text-xs text-red-600">
                      {fieldErrors[`env_key_${index}`]}
                    </p>
                  )}
                </div>
                <div className="flex-1">
                  <input
                    type="text"
                    value={pair.value}
                    onChange={(e) =>
                      updateEnvPair(index, "value", e.target.value)
                    }
                    onBlur={() => validateEnvPairField(index, "value")}
                    placeholder="value"
                    className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 font-mono"
                  />
                  {fieldErrors[`env_value_${index}`] && (
                    <p className="mt-0.5 text-xs text-red-600">
                      {fieldErrors[`env_value_${index}`]}
                    </p>
                  )}
                </div>
                <button
                  type="button"
                  onClick={() => removeEnvPair(index)}
                  className="mt-1 text-gray-400 hover:text-red-500 text-sm"
                  aria-label={`Remove variable ${pair.key || index + 1}`}
                >
                  ✕
                </button>
              </div>
            ))}
          </div>
          <p className="mt-1 text-xs text-gray-500">
            Up to 20 key-value pairs. Keys: 1–64 chars. Values: 1–1024 chars.
          </p>
        </div>

        {/* Custom Arguments */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <label className="block text-sm font-medium text-gray-700">
              Custom Arguments
            </label>
            <button
              type="button"
              onClick={addArg}
              disabled={customArgs.length >= 20}
              className="text-xs text-blue-600 hover:text-blue-700 disabled:text-gray-400 disabled:cursor-not-allowed"
            >
              + Add Argument
            </button>
          </div>
          {customArgs.length === 0 && (
            <p className="text-xs text-gray-500">
              No custom arguments configured. Click "Add Argument" to add CLI
              flags or options.
            </p>
          )}
          <div className="space-y-2">
            {customArgs.map((arg, index) => (
              <div key={index} className="flex items-start gap-2">
                <div className="flex-1">
                  <input
                    type="text"
                    value={arg}
                    onChange={(e) => updateArg(index, e.target.value)}
                    onBlur={() => validateArgField(index)}
                    placeholder="--flag or value"
                    className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 font-mono"
                  />
                  {fieldErrors[`arg_${index}`] && (
                    <p className="mt-0.5 text-xs text-red-600">
                      {fieldErrors[`arg_${index}`]}
                    </p>
                  )}
                </div>
                <button
                  type="button"
                  onClick={() => removeArg(index)}
                  className="mt-1 text-gray-400 hover:text-red-500 text-sm"
                  aria-label={`Remove argument ${index + 1}`}
                >
                  ✕
                </button>
              </div>
            ))}
          </div>
          <p className="mt-1 text-xs text-gray-500">
            Up to 20 arguments. Each max 256 characters.
          </p>
        </div>

        {/* Max Concurrent Tasks */}
        <div>
          <label
            htmlFor="agent-max-tasks"
            className="block text-sm font-medium text-gray-700"
          >
            Max Concurrent Tasks
          </label>
          <input
            id="agent-max-tasks"
            type="number"
            min={1}
            max={20}
            value={maxConcurrentTasks}
            onChange={(e) =>
              setMaxConcurrentTasks(parseInt(e.target.value, 10) || 1)
            }
            onBlur={() => onBlurField("max_concurrent_tasks")}
            className="mt-1 block w-32 rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
          <p className="mt-1 text-xs text-gray-500">
            How many tasks this agent can run simultaneously (1–20).
          </p>
          {fieldErrors.max_concurrent_tasks && (
            <p className="mt-1 text-xs text-red-600">
              {fieldErrors.max_concurrent_tasks}
            </p>
          )}
        </div>

        {/* Visibility */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">
            Visibility
          </label>
          <div className="flex items-center gap-4">
            <label className="inline-flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                name="visibility"
                value="private"
                checked={visibility === "private"}
                onChange={() => setVisibility("private")}
                className="h-4 w-4 text-blue-600 border-gray-300 focus:ring-blue-500"
              />
              <span className="text-sm text-gray-700">Private</span>
            </label>
            <label className="inline-flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                name="visibility"
                value="shared"
                checked={visibility === "shared"}
                onChange={() => setVisibility("shared")}
                className="h-4 w-4 text-blue-600 border-gray-300 focus:ring-blue-500"
              />
              <span className="text-sm text-gray-700">Shared</span>
            </label>
          </div>
          <p className="mt-1 text-xs text-gray-500">
            Private agents are only visible to you. Shared agents are visible to
            all users on this instance.
          </p>
        </div>

        {/* Submit */}
        <div className="flex items-center gap-3 pt-4 border-t border-gray-200">
          <button
            type="submit"
            disabled={createAgent.isPending}
            className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50"
          >
            {createAgent.isPending ? "Creating…" : "Create Agent"}
          </button>
          <button
            type="button"
            onClick={() => navigate("/agents")}
            className="inline-flex items-center px-4 py-2 border border-gray-300 text-gray-700 text-sm font-medium rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
