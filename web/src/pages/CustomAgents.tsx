import { useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  useCustomAgents,
  useCreateCustomAgent,
  useUpdateCustomAgent,
  useDeleteCustomAgent,
} from "../hooks/useCustomAgents";
import type { CustomAgent } from "../hooks/useCustomAgents";

interface EnvVar {
  key: string;
  value: string;
}

interface AgentFormData {
  name: string;
  command: string;
  args_template: string;
  model_override: string;
  env_vars: EnvVar[];
}

const EMPTY_FORM: AgentFormData = {
  name: "",
  command: "",
  args_template: "{{prompt}}",
  model_override: "",
  env_vars: [],
};

const NAME_PATTERN = /^[a-zA-Z0-9_-]{1,64}$/;

export default function CustomAgents() {
  const navigate = useNavigate();
  const { data: agents, isLoading } = useCustomAgents();
  const [editingAgent, setEditingAgent] = useState<CustomAgent | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);

  const handleCreate = () => {
    setEditingAgent(null);
    setShowForm(true);
  };

  const handleEdit = (agent: CustomAgent) => {
    setEditingAgent(agent);
    setShowForm(true);
  };

  const handleCloseForm = () => {
    setShowForm(false);
    setEditingAgent(null);
  };

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-200 px-6 py-4">
        <div className="max-w-7xl mx-auto flex items-center justify-between">
          <h1 className="text-xl font-semibold text-gray-900">
            Custom Agents
          </h1>
          <div className="flex items-center gap-4">
            <button
              onClick={() => navigate("/")}
              className="text-sm text-blue-600 hover:text-blue-800"
            >
              ← Back to Dashboard
            </button>
            <button
              onClick={handleCreate}
              className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
            >
              + New Agent
            </button>
          </div>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-6 py-8">
        {showForm && (
          <AgentForm
            agent={editingAgent}
            onClose={handleCloseForm}
          />
        )}

        {isLoading ? (
          <p className="text-sm text-gray-500">Loading agents...</p>
        ) : !agents || agents.length === 0 ? (
          <div className="text-center py-12">
            <p className="text-sm text-gray-500 mb-4">
              No custom agents defined yet.
            </p>
            <button
              onClick={handleCreate}
              className="text-sm text-blue-600 hover:text-blue-800"
            >
              Create your first custom agent
            </button>
          </div>
        ) : (
          <div className="space-y-4">
            {agents.map((agent) => (
              <AgentCard
                key={agent.id}
                agent={agent}
                onEdit={() => handleEdit(agent)}
                deleteConfirmId={deleteConfirmId}
                setDeleteConfirmId={setDeleteConfirmId}
              />
            ))}
          </div>
        )}
      </main>
    </div>
  );
}

/* ─── Agent Card ─── */

function AgentCard({
  agent,
  onEdit,
  deleteConfirmId,
  setDeleteConfirmId,
}: {
  agent: CustomAgent;
  onEdit: () => void;
  deleteConfirmId: string | null;
  setDeleteConfirmId: (id: string | null) => void;
}) {
  const deleteAgent = useDeleteCustomAgent();
  const isConfirming = deleteConfirmId === agent.id;

  const handleDelete = () => {
    if (isConfirming) {
      deleteAgent.mutate(agent.id, {
        onSuccess: () => setDeleteConfirmId(null),
      });
    } else {
      setDeleteConfirmId(agent.id);
    }
  };

  const envCount = Object.keys(agent.env_vars ?? {}).length;

  return (
    <div className="bg-white rounded-lg border border-gray-200 p-4 shadow-sm">
      <div className="flex items-start justify-between">
        <div className="flex-1 min-w-0">
          <h3 className="font-medium text-gray-900">{agent.name}</h3>
          <p className="text-sm text-gray-500 mt-1">
            Command: <code className="text-xs bg-gray-100 px-1 py-0.5 rounded">{agent.command}</code>
          </p>
          <div className="flex items-center gap-4 mt-2 text-xs text-gray-400">
            <span>Args: {agent.args_template}</span>
            {agent.model_override && (
              <span>Model: {agent.model_override}</span>
            )}
            {envCount > 0 && (
              <span>{envCount} env var{envCount > 1 ? "s" : ""}</span>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2 ml-4">
          <button
            onClick={onEdit}
            className="px-3 py-1.5 text-sm border border-gray-300 rounded-md hover:bg-gray-50"
          >
            Edit
          </button>
          <button
            onClick={handleDelete}
            disabled={deleteAgent.isPending}
            className={`px-3 py-1.5 text-sm rounded-md ${
              isConfirming
                ? "bg-red-600 text-white hover:bg-red-700"
                : "border border-red-300 text-red-600 hover:bg-red-50"
            } disabled:opacity-50`}
          >
            {deleteAgent.isPending
              ? "Deleting..."
              : isConfirming
                ? "Confirm Delete"
                : "Delete"}
          </button>
          {isConfirming && (
            <button
              onClick={() => setDeleteConfirmId(null)}
              className="px-3 py-1.5 text-sm border border-gray-300 rounded-md hover:bg-gray-50"
            >
              Cancel
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

/* ─── Agent Form ─── */

function AgentForm({
  agent,
  onClose,
}: {
  agent: CustomAgent | null;
  onClose: () => void;
}) {
  const createAgent = useCreateCustomAgent();
  const updateAgent = useUpdateCustomAgent();

  const initialForm: AgentFormData = agent
    ? {
        name: agent.name,
        command: agent.command,
        args_template: agent.args_template,
        model_override: agent.model_override ?? "",
        env_vars: Object.entries(agent.env_vars ?? {}).map(([key, value]) => ({
          key,
          value,
        })),
      }
    : EMPTY_FORM;

  const [form, setForm] = useState<AgentFormData>(initialForm);
  const [error, setError] = useState("");

  const isEditing = agent !== null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    // Validate name
    if (!NAME_PATTERN.test(form.name)) {
      setError(
        "Name must be 1-64 characters, alphanumeric, hyphens, and underscores only."
      );
      return;
    }

    if (!form.command.trim()) {
      setError("Command is required.");
      return;
    }

    // Build env_vars object from key-value pairs
    const envVars: Record<string, string> = {};
    for (const { key, value } of form.env_vars) {
      if (key.trim()) {
        envVars[key.trim()] = value;
      }
    }

    const payload = {
      name: form.name,
      command: form.command.trim(),
      args_template: form.args_template || "{{prompt}}",
      model_override: form.model_override || undefined,
      env_vars: envVars,
    };

    if (isEditing) {
      updateAgent.mutate(
        { id: agent.id, data: payload },
        {
          onSuccess: () => onClose(),
          onError: (err: Error) => setError(err.message),
        }
      );
    } else {
      createAgent.mutate(payload, {
        onSuccess: () => onClose(),
        onError: (err: Error) => setError(err.message),
      });
    }
  };

  const addEnvVar = () => {
    setForm((prev) => ({
      ...prev,
      env_vars: [...prev.env_vars, { key: "", value: "" }],
    }));
  };

  const removeEnvVar = (index: number) => {
    setForm((prev) => ({
      ...prev,
      env_vars: prev.env_vars.filter((_, i) => i !== index),
    }));
  };

  const updateEnvVar = (
    index: number,
    field: "key" | "value",
    val: string
  ) => {
    setForm((prev) => ({
      ...prev,
      env_vars: prev.env_vars.map((ev, i) =>
        i === index ? { ...ev, [field]: val } : ev
      ),
    }));
  };

  const isPending = createAgent.isPending || updateAgent.isPending;

  return (
    <div className="bg-white rounded-lg border border-gray-200 p-6 shadow-sm mb-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-medium text-gray-900">
          {isEditing ? "Edit Agent" : "Create Agent"}
        </h2>
        <button
          onClick={onClose}
          className="text-sm text-gray-500 hover:text-gray-700"
        >
          ✕ Close
        </button>
      </div>

      <form onSubmit={handleSubmit} className="space-y-4">
        {/* Name */}
        <div>
          <label
            htmlFor="agent-name"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            Name
          </label>
          <input
            id="agent-name"
            type="text"
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            maxLength={64}
            placeholder="my-agent"
            className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          />
          <p className="text-xs text-gray-400 mt-1">
            1-64 chars, alphanumeric, hyphens, underscores
          </p>
        </div>

        {/* Command */}
        <div>
          <label
            htmlFor="agent-command"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            Command
          </label>
          <input
            id="agent-command"
            type="text"
            value={form.command}
            onChange={(e) =>
              setForm((f) => ({ ...f, command: e.target.value }))
            }
            placeholder="/usr/local/bin/my-cli"
            className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          />
        </div>

        {/* Args Template */}
        <div>
          <label
            htmlFor="agent-args"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            Arguments Template
          </label>
          <input
            id="agent-args"
            type="text"
            value={form.args_template}
            onChange={(e) =>
              setForm((f) => ({ ...f, args_template: e.target.value }))
            }
            placeholder="{{prompt}}"
            className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          />
          <p className="text-xs text-gray-400 mt-1">
            Supports: {"{{prompt}}"}, {"{{workspace}}"}, {"{{model}}"}
          </p>
        </div>

        {/* Model Override */}
        <div>
          <label
            htmlFor="agent-model"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            Model Override (optional)
          </label>
          <input
            id="agent-model"
            type="text"
            value={form.model_override}
            onChange={(e) =>
              setForm((f) => ({ ...f, model_override: e.target.value }))
            }
            placeholder="e.g., claude-sonnet-4-20250514"
            className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          />
        </div>

        {/* Environment Variables */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <label className="block text-sm font-medium text-gray-700">
              Environment Variables
            </label>
            <button
              type="button"
              onClick={addEnvVar}
              disabled={form.env_vars.length >= 20}
              className="text-xs text-blue-600 hover:text-blue-800 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              + Add Variable
            </button>
          </div>
          {form.env_vars.length === 0 ? (
            <p className="text-xs text-gray-400">No environment variables.</p>
          ) : (
            <div className="space-y-2">
              {form.env_vars.map((ev, index) => (
                <div key={index} className="flex items-center gap-2">
                  <input
                    type="text"
                    value={ev.key}
                    onChange={(e) =>
                      updateEnvVar(index, "key", e.target.value)
                    }
                    placeholder="KEY"
                    className="flex-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                  />
                  <span className="text-gray-400">=</span>
                  <input
                    type="text"
                    value={ev.value}
                    onChange={(e) =>
                      updateEnvVar(index, "value", e.target.value)
                    }
                    placeholder="value"
                    className="flex-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                  />
                  <button
                    type="button"
                    onClick={() => removeEnvVar(index)}
                    className="text-red-500 hover:text-red-700 text-sm px-2"
                  >
                    ✕
                  </button>
                </div>
              ))}
            </div>
          )}
          <p className="text-xs text-gray-400 mt-1">
            Up to 20 key-value pairs
          </p>
        </div>

        {error && <p className="text-sm text-red-600">{error}</p>}

        <div className="flex items-center gap-3 pt-2">
          <button
            type="submit"
            disabled={isPending}
            className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isPending
              ? "Saving..."
              : isEditing
                ? "Update Agent"
                : "Create Agent"}
          </button>
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-sm border border-gray-300 rounded-md hover:bg-gray-50"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
