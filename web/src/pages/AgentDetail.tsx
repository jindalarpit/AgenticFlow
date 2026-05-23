import { useState, useMemo } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useManagedAgent } from "../hooks/useManagedAgents";
import type { ManagedAgent, AgentStatus } from "../hooks/useManagedAgents";
import { useDaemons } from "../hooks/useDaemons";
import type { AgentRuntime } from "../hooks/useDaemons";
import { apiFetch } from "../lib/api";
import { useToast } from "../components/Toast";

/* ─── Types ─── */

interface AgentTask {
  id: string;
  status: "pending" | "running" | "completed" | "failed" | "cancelled" | "timeout";
  prompt: string;
  started_at: string | null;
  completed_at: string | null;
  created_at: string;
}

interface AgentTasksResponse {
  tasks: AgentTask[];
  total: number;
}

interface UpdateAgentPayload {
  name?: string;
  description?: string;
  instructions?: string;
  runtime_id?: string;
  model?: string;
  custom_env?: Record<string, string>;
  custom_args?: string[];
  max_concurrent_tasks?: number;
  visibility?: "private" | "shared";
}

/* ─── Validation (reused from AgentForm) ─── */

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

function validateMaxConcurrentTasks(value: number): string | null {
  if (!Number.isInteger(value)) return "Must be a whole number";
  if (value < 1) return "Must be at least 1";
  if (value > 20) return "Must not exceed 20";
  return null;
}

/* ─── Helper: get current user ID from token ─── */

function getCurrentUserId(): string | null {
  try {
    const token = localStorage.getItem("af_token");
    if (!token) return null;
    const parts = token.split(".");
    const part = parts[1];
    if (!part) return null;
    const payload = JSON.parse(atob(part));
    return payload.sub || payload.user_id || null;
  } catch {
    return null;
  }
}

/* ─── Hook: fetch agent tasks ─── */

function useAgentTasks(agentId: string) {
  return useQuery<AgentTasksResponse>({
    queryKey: ["tasks", { agent_id: agentId, limit: 10 }],
    queryFn: () =>
      apiFetch<AgentTasksResponse>(
        `/api/tasks?agent_id=${agentId}&limit=10`
      ),
    enabled: !!agentId,
  });
}

/* ─── Component ─── */

export default function AgentDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { showToast } = useToast();
  const { data: agent, isLoading, error } = useManagedAgent(id || "");
  const { data: tasksData } = useAgentTasks(id || "");
  const { data: daemons } = useDaemons();

  // Determine if current user owns this agent
  const currentUserId = getCurrentUserId();
  const isOwner = agent ? agent.owner_id === currentUserId : true;
  const isReadOnly = !isOwner;

  // Editing state
  const [editingField, setEditingField] = useState<string | null>(null);
  const [editValues, setEditValues] = useState<Record<string, any>>({});
  const [fieldError, setFieldError] = useState<string | null>(null);

  // Online runtimes for runtime dropdown
  const onlineRuntimes: (AgentRuntime & { daemon_name: string })[] =
    useMemo(() => {
      if (!daemons) return [];
      return daemons
        .filter((d) => d.status === "online")
        .flatMap((d) =>
          d.agent_runtimes.map((r) => ({ ...r, daemon_name: d.device_name }))
        );
    }, [daemons]);

  // Update mutation
  const updateAgent = useMutation({
    mutationFn: (payload: UpdateAgentPayload) =>
      apiFetch<ManagedAgent>(`/api/agents/${id}`, {
        method: "PUT",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      showToast("Agent updated successfully", "success");
      void queryClient.invalidateQueries({ queryKey: ["agents", id] });
      void queryClient.invalidateQueries({ queryKey: ["agents"] });
      setEditingField(null);
      setEditValues({});
      setFieldError(null);
    },
    onError: (err: Error) => {
      showToast(err.message || "Failed to update agent", "error");
    },
  });

  /* ─── Inline edit helpers ─── */

  function startEdit(field: string, currentValue: any) {
    if (isReadOnly) return;
    setEditingField(field);
    setEditValues({ [field]: currentValue });
    setFieldError(null);
  }

  function cancelEdit() {
    setEditingField(null);
    setEditValues({});
    setFieldError(null);
  }

  function saveField(field: string) {
    const value = editValues[field];
    let error: string | null = null;

    switch (field) {
      case "name":
        error = validateAgentName(value);
        break;
      case "description":
        error = validateDescription(value);
        break;
      case "instructions":
        error = validateInstructions(value);
        break;
      case "model":
        error = validateModel(value);
        break;
      case "max_concurrent_tasks":
        error = validateMaxConcurrentTasks(value);
        break;
    }

    if (error) {
      setFieldError(error);
      return;
    }

    updateAgent.mutate({ [field]: value });
  }

  function saveVisibility(value: "private" | "shared") {
    updateAgent.mutate({ visibility: value });
  }

  function saveRuntimeId(value: string) {
    if (!value) return;
    updateAgent.mutate({ runtime_id: value });
  }

  /* ─── Loading / Error states ─── */

  if (isLoading) {
    return (
      <div className="max-w-4xl mx-auto px-6 py-8">
        <p className="text-sm text-gray-500">Loading agent...</p>
      </div>
    );
  }

  if (error || !agent) {
    return (
      <div className="max-w-4xl mx-auto px-6 py-8">
        <p className="text-sm text-red-600">
          {error instanceof Error ? error.message : "Agent not found"}
        </p>
        <button
          onClick={() => navigate("/agents")}
          className="mt-4 text-sm text-blue-600 hover:text-blue-700"
        >
          ← Back to Agents
        </button>
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto px-6 py-8">
      {/* Header */}
      <div className="mb-6">
        <button
          type="button"
          onClick={() => navigate("/agents")}
          className="text-sm text-gray-500 hover:text-gray-700 mb-2 inline-flex items-center gap-1"
        >
          ← Back to Agents
        </button>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-semibold text-gray-900">
              {agent.name}
            </h1>
            <AgentStatusBadge status={agent.status} />
          </div>
          {isReadOnly && (
            <span className="text-xs text-gray-500 bg-gray-100 px-2 py-1 rounded">
              Read-only (shared agent)
            </span>
          )}
        </div>
      </div>

      {/* Agent Fields */}
      <div className="bg-white rounded-lg border border-gray-200 shadow-sm divide-y divide-gray-100">
        {/* Name */}
        <InlineField
          label="Name"
          value={agent.name}
          isEditing={editingField === "name"}
          isReadOnly={isReadOnly}
          isSaving={updateAgent.isPending}
          error={editingField === "name" ? fieldError : null}
          onStartEdit={() => startEdit("name", agent.name)}
          onCancel={cancelEdit}
          onSave={() => saveField("name")}
          renderEdit={() => (
            <input
              type="text"
              value={editValues.name ?? ""}
              onChange={(e) =>
                setEditValues({ name: e.target.value })
              }
              maxLength={64}
              className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              autoFocus
            />
          )}
        />

        {/* Description */}
        <InlineField
          label="Description"
          value={agent.description || "—"}
          isEditing={editingField === "description"}
          isReadOnly={isReadOnly}
          isSaving={updateAgent.isPending}
          error={editingField === "description" ? fieldError : null}
          onStartEdit={() => startEdit("description", agent.description)}
          onCancel={cancelEdit}
          onSave={() => saveField("description")}
          renderEdit={() => (
            <textarea
              value={editValues.description ?? ""}
              onChange={(e) =>
                setEditValues({ description: e.target.value })
              }
              maxLength={255}
              rows={2}
              className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 resize-none"
              autoFocus
            />
          )}
        />

        {/* Instructions */}
        <InlineField
          label="Instructions"
          value={
            agent.instructions
              ? agent.instructions.length > 120
                ? agent.instructions.slice(0, 120) + "…"
                : agent.instructions
              : "—"
          }
          isEditing={editingField === "instructions"}
          isReadOnly={isReadOnly}
          isSaving={updateAgent.isPending}
          error={editingField === "instructions" ? fieldError : null}
          onStartEdit={() => startEdit("instructions", agent.instructions)}
          onCancel={cancelEdit}
          onSave={() => saveField("instructions")}
          renderEdit={() => (
            <textarea
              value={editValues.instructions ?? ""}
              onChange={(e) =>
                setEditValues({ instructions: e.target.value })
              }
              rows={8}
              className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 resize-y font-mono"
              autoFocus
            />
          )}
        />

        {/* Runtime */}
        <InlineField
          label="Runtime"
          value={agent.runtime_name || agent.runtime_id}
          isEditing={editingField === "runtime_id"}
          isReadOnly={isReadOnly}
          isSaving={updateAgent.isPending}
          error={editingField === "runtime_id" ? fieldError : null}
          onStartEdit={() => startEdit("runtime_id", agent.runtime_id)}
          onCancel={cancelEdit}
          onSave={() => saveRuntimeId(editValues.runtime_id)}
          renderEdit={() => (
            <select
              value={editValues.runtime_id ?? ""}
              onChange={(e) =>
                setEditValues({ runtime_id: e.target.value })
              }
              className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 bg-white"
            >
              <option value="">Select a runtime...</option>
              {onlineRuntimes.map((rt) => (
                <option key={rt.id} value={rt.id}>
                  {rt.name} ({rt.provider}) — {rt.daemon_name}
                </option>
              ))}
            </select>
          )}
        />

        {/* Model */}
        <InlineField
          label="Model"
          value={agent.model || "—"}
          isEditing={editingField === "model"}
          isReadOnly={isReadOnly}
          isSaving={updateAgent.isPending}
          error={editingField === "model" ? fieldError : null}
          onStartEdit={() => startEdit("model", agent.model)}
          onCancel={cancelEdit}
          onSave={() => saveField("model")}
          renderEdit={() => (
            <input
              type="text"
              value={editValues.model ?? ""}
              onChange={(e) =>
                setEditValues({ model: e.target.value })
              }
              maxLength={100}
              placeholder="e.g., claude-sonnet-4-20250514"
              className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              autoFocus
            />
          )}
        />

        {/* Max Concurrent Tasks */}
        <InlineField
          label="Max Concurrent Tasks"
          value={String(agent.max_concurrent_tasks)}
          isEditing={editingField === "max_concurrent_tasks"}
          isReadOnly={isReadOnly}
          isSaving={updateAgent.isPending}
          error={editingField === "max_concurrent_tasks" ? fieldError : null}
          onStartEdit={() =>
            startEdit("max_concurrent_tasks", agent.max_concurrent_tasks)
          }
          onCancel={cancelEdit}
          onSave={() => saveField("max_concurrent_tasks")}
          renderEdit={() => (
            <input
              type="number"
              min={1}
              max={20}
              value={editValues.max_concurrent_tasks ?? 1}
              onChange={(e) =>
                setEditValues({
                  max_concurrent_tasks: parseInt(e.target.value, 10) || 1,
                })
              }
              className="block w-32 rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              autoFocus
            />
          )}
        />

        {/* Visibility */}
        <div className="px-4 py-3 flex items-center justify-between">
          <div>
            <span className="text-sm font-medium text-gray-700">
              Visibility
            </span>
            <p className="text-sm text-gray-900 mt-0.5">
              {agent.visibility === "shared" ? "Shared" : "Private"}
            </p>
          </div>
          {!isReadOnly && (
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() =>
                  saveVisibility(
                    agent.visibility === "private" ? "shared" : "private"
                  )
                }
                disabled={updateAgent.isPending}
                className="text-xs text-blue-600 hover:text-blue-700 disabled:text-gray-400"
              >
                Switch to{" "}
                {agent.visibility === "private" ? "Shared" : "Private"}
              </button>
            </div>
          )}
        </div>

        {/* Custom Env (display only, simplified) */}
        <div className="px-4 py-3">
          <span className="text-sm font-medium text-gray-700">
            Environment Variables
          </span>
          {Object.keys(agent.custom_env).length === 0 ? (
            <p className="text-sm text-gray-500 mt-0.5">None configured</p>
          ) : (
            <div className="mt-1 space-y-1">
              {Object.entries(agent.custom_env).map(([key, value]) => (
                <div key={key} className="flex items-center gap-2 text-xs font-mono">
                  <span className="text-gray-700">{key}</span>
                  <span className="text-gray-400">=</span>
                  <span className="text-gray-500 truncate max-w-xs">
                    {value}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Custom Args (display only, simplified) */}
        <div className="px-4 py-3">
          <span className="text-sm font-medium text-gray-700">
            Custom Arguments
          </span>
          {agent.custom_args.length === 0 ? (
            <p className="text-sm text-gray-500 mt-0.5">None configured</p>
          ) : (
            <div className="mt-1 flex flex-wrap gap-1">
              {agent.custom_args.map((arg, i) => (
                <span
                  key={i}
                  className="inline-flex items-center px-2 py-0.5 rounded bg-gray-100 text-xs font-mono text-gray-700"
                >
                  {arg}
                </span>
              ))}
            </div>
          )}
        </div>

        {/* Metadata */}
        <div className="px-4 py-3 flex items-center gap-6 text-xs text-gray-500">
          <span>Created: {new Date(agent.created_at).toLocaleDateString()}</span>
          <span>Updated: {new Date(agent.updated_at).toLocaleDateString()}</span>
        </div>
      </div>

      {/* Task History */}
      <div className="mt-8">
        <h2 className="text-lg font-semibold text-gray-900 mb-4">
          Recent Task History
        </h2>
        <TaskHistoryTable tasks={tasksData?.tasks || []} />
      </div>
    </div>
  );
}

/* ─── InlineField Component ─── */

interface InlineFieldProps {
  label: string;
  value: string;
  isEditing: boolean;
  isReadOnly: boolean;
  isSaving: boolean;
  error: string | null;
  onStartEdit: () => void;
  onCancel: () => void;
  onSave: () => void;
  renderEdit: () => React.ReactNode;
}

function InlineField({
  label,
  value,
  isEditing,
  isReadOnly,
  isSaving,
  error,
  onStartEdit,
  onCancel,
  onSave,
  renderEdit,
}: InlineFieldProps) {
  if (isEditing) {
    return (
      <div className="px-4 py-3">
        <span className="text-sm font-medium text-gray-700 mb-1 block">
          {label}
        </span>
        {renderEdit()}
        {error && <p className="mt-1 text-xs text-red-600">{error}</p>}
        <div className="mt-2 flex items-center gap-2">
          <button
            type="button"
            onClick={onSave}
            disabled={isSaving}
            className="inline-flex items-center px-3 py-1 bg-blue-600 text-white text-xs font-medium rounded hover:bg-blue-700 disabled:opacity-50"
          >
            {isSaving ? "Saving…" : "Save"}
          </button>
          <button
            type="button"
            onClick={onCancel}
            disabled={isSaving}
            className="inline-flex items-center px-3 py-1 border border-gray-300 text-gray-700 text-xs font-medium rounded hover:bg-gray-50 disabled:opacity-50"
          >
            Cancel
          </button>
        </div>
      </div>
    );
  }

  return (
    <div
      className={`px-4 py-3 flex items-center justify-between ${
        !isReadOnly ? "group cursor-pointer hover:bg-gray-50" : ""
      }`}
      onClick={!isReadOnly ? onStartEdit : undefined}
    >
      <div className="min-w-0 flex-1">
        <span className="text-sm font-medium text-gray-700">{label}</span>
        <p className="text-sm text-gray-900 mt-0.5 whitespace-pre-wrap break-words">
          {value}
        </p>
      </div>
      {!isReadOnly && (
        <span className="text-xs text-gray-400 opacity-0 group-hover:opacity-100 transition-opacity ml-2 shrink-0">
          Edit
        </span>
      )}
    </div>
  );
}

/* ─── Status Badge ─── */

function AgentStatusBadge({ status }: { status: AgentStatus }) {
  const config: Record<AgentStatus, { color: string; label: string }> = {
    idle: { color: "bg-green-100 text-green-700", label: "Idle" },
    working: { color: "bg-amber-100 text-amber-700", label: "Working" },
    offline: { color: "bg-gray-100 text-gray-600", label: "Offline" },
  };

  const { color, label } = config[status];

  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${color}`}
    >
      {label}
    </span>
  );
}

/* ─── Task History Table ─── */

function TaskHistoryTable({ tasks }: { tasks: AgentTask[] }) {
  if (tasks.length === 0) {
    return (
      <div className="text-center py-8 bg-white rounded-lg border border-gray-200">
        <p className="text-sm text-gray-500">No tasks have been run yet.</p>
      </div>
    );
  }

  return (
    <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
      <table className="min-w-full divide-y divide-gray-200">
        <thead className="bg-gray-50">
          <tr>
            <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">
              Status
            </th>
            <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">
              Prompt
            </th>
            <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">
              Duration
            </th>
            <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">
              Completed
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100">
          {tasks.map((task) => (
            <TaskRow key={task.id} task={task} />
          ))}
        </tbody>
      </table>
    </div>
  );
}

/* ─── Task Row ─── */

function TaskRow({ task }: { task: AgentTask }) {
  const promptPreview =
    task.prompt.length > 60 ? task.prompt.slice(0, 60) + "…" : task.prompt;

  const duration = computeDuration(task.started_at, task.completed_at);
  const completedTime = task.completed_at
    ? new Date(task.completed_at).toLocaleString()
    : "—";

  return (
    <tr className="hover:bg-gray-50">
      <td className="px-4 py-2">
        <TaskStatusBadge status={task.status} />
      </td>
      <td className="px-4 py-2 text-sm text-gray-700 max-w-xs truncate">
        {promptPreview}
      </td>
      <td className="px-4 py-2 text-sm text-gray-500">{duration}</td>
      <td className="px-4 py-2 text-sm text-gray-500">{completedTime}</td>
    </tr>
  );
}

/* ─── Task Status Badge ─── */

type TaskStatus =
  | "pending"
  | "running"
  | "completed"
  | "failed"
  | "cancelled"
  | "timeout";

function TaskStatusBadge({ status }: { status: TaskStatus }) {
  const config: Record<TaskStatus, { color: string; label: string }> = {
    pending: { color: "bg-yellow-100 text-yellow-700", label: "Pending" },
    running: { color: "bg-blue-100 text-blue-700", label: "Running" },
    completed: { color: "bg-green-100 text-green-700", label: "Completed" },
    failed: { color: "bg-red-100 text-red-700", label: "Failed" },
    cancelled: { color: "bg-gray-100 text-gray-600", label: "Cancelled" },
    timeout: { color: "bg-orange-100 text-orange-700", label: "Timeout" },
  };

  const { color, label } = config[status] || {
    color: "bg-gray-100 text-gray-600",
    label: status,
  };

  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${color}`}
    >
      {label}
    </span>
  );
}

/* ─── Duration Helper ─── */

function computeDuration(
  startedAt: string | null,
  completedAt: string | null
): string {
  if (!startedAt) return "—";
  const start = new Date(startedAt).getTime();
  const end = completedAt ? new Date(completedAt).getTime() : Date.now();
  const diffMs = end - start;

  if (diffMs < 1000) return "<1s";
  const seconds = Math.floor(diffMs / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  if (minutes < 60) return `${minutes}m ${remainingSeconds}s`;
  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return `${hours}h ${remainingMinutes}m`;
}
