import { useCallback, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useDaemons } from "../hooks/useDaemons";
import { useAgents } from "../hooks/useAgents";
import { useManagedAgents } from "../hooks/useManagedAgents";
import { useTasks, useCreateTask } from "../hooks/useTasks";
import { useTaskResultPanel } from "../hooks/useTaskResultPanel";
import { TaskResultPanel } from "../components/TaskResultPanel";
import type { Daemon } from "../hooks/useDaemons";
import type { Agent } from "../hooks/useAgents";
import type { ManagedAgent } from "../hooks/useManagedAgents";
import type { Task, CreateTaskInput } from "../hooks/useTasks";

const TASKS_PER_PAGE = 50;

export default function Dashboard() {
  const [taskOffset, setTaskOffset] = useState(0);

  // Data fetching
  const { data: daemons, isLoading: daemonsLoading } = useDaemons();
  const { data: agents, isLoading: agentsLoading } = useAgents();
  const { data: managedAgents, isLoading: managedAgentsLoading } = useManagedAgents();
  const { data: tasksData, isLoading: tasksLoading } = useTasks(
    TASKS_PER_PAGE,
    taskOffset
  );

  // Task Result Panel state
  const {
    panelTaskId,
    isVisible: isResultPanelVisible,
    setTask: setResultPanelTask,
    dismiss: dismissResultPanel,
    task: resultPanelTask,
    messages: resultPanelMessages,
    wsConnected: resultPanelWsConnected,
  } = useTaskResultPanel();

  const handleTaskCreated = useCallback(
    (taskId: string) => {
      setResultPanelTask(taskId);
    },
    [setResultPanelTask]
  );

  const handlePrevPage = useCallback(() => {
    setTaskOffset((prev) => Math.max(0, prev - TASKS_PER_PAGE));
  }, []);

  const handleNextPage = useCallback(() => {
    if (tasksData && taskOffset + TASKS_PER_PAGE < tasksData.total) {
      setTaskOffset((prev) => prev + TASKS_PER_PAGE);
    }
  }, [tasksData, taskOffset]);

  return (
    <div className="max-w-7xl mx-auto px-6 py-8 space-y-8">
      {/* Connected Daemons Section */}
      <DaemonsSection daemons={daemons ?? []} isLoading={daemonsLoading} />

        {/* Agent Runtimes Section */}
        <AgentsSection agents={agents ?? []} isLoading={agentsLoading} />

        {/* Task Submission Form */}
        <TaskSubmissionForm
          agents={agents ?? []}
          managedAgents={managedAgents ?? []}
          managedAgentsLoading={managedAgentsLoading}
          onTaskCreated={handleTaskCreated}
        />

        {/* Task Result Panel — shown between form and queue when a task is active */}
        {isResultPanelVisible && panelTaskId && (
          <TaskResultPanel
            taskId={panelTaskId}
            onDismiss={dismissResultPanel}
            task={resultPanelTask}
            messages={resultPanelMessages}
            wsConnected={resultPanelWsConnected}
          />
        )}

        {/* Task Queue Section */}
        <TaskQueueSection
          tasks={tasksData?.tasks ?? []}
          total={tasksData?.total ?? 0}
          offset={taskOffset}
          isLoading={tasksLoading}
          onPrevPage={handlePrevPage}
          onNextPage={handleNextPage}
        />
    </div>
  );
}

/* ─── Connected Daemons ─── */

function DaemonsSection({
  daemons,
  isLoading,
}: {
  daemons: Daemon[];
  isLoading: boolean;
}) {
  return (
    <section>
      <h2 className="text-lg font-medium text-gray-900 mb-4">
        Connected Daemons
      </h2>
      {isLoading ? (
        <p className="text-sm text-gray-500">Loading daemons...</p>
      ) : daemons.length === 0 ? (
        <p className="text-sm text-gray-500">No daemons connected.</p>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {daemons.map((daemon) => (
            <DaemonCard key={daemon.id} daemon={daemon} />
          ))}
        </div>
      )}
    </section>
  );
}

function DaemonCard({ daemon }: { daemon: Daemon }) {
  return (
    <div className="bg-white rounded-lg border border-gray-200 p-4 shadow-sm">
      <div className="flex items-center justify-between mb-3">
        <h3 className="font-medium text-gray-900 truncate">
          {daemon.device_name}
        </h3>
        <StatusBadge
          status={daemon.status}
          variant={daemon.status === "online" ? "green" : "gray"}
        />
      </div>

      {daemon.agent_runtimes && daemon.agent_runtimes.length > 0 && (
        <div className="mb-3">
          <p className="text-xs font-medium text-gray-500 uppercase mb-1">
            Runtimes
          </p>
          <div className="flex flex-wrap gap-1">
            {daemon.agent_runtimes.map((rt) => (
              <span
                key={rt.id}
                className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-50 text-blue-700"
              >
                {rt.provider}
              </span>
            ))}
          </div>
        </div>
      )}

      {daemon.last_heartbeat_at && (
        <p className="text-xs text-gray-400">
          Last heartbeat:{" "}
          {new Date(daemon.last_heartbeat_at).toLocaleTimeString()}
        </p>
      )}
    </div>
  );
}

/* ─── Agent Runtimes ─── */

function AgentsSection({
  agents,
  isLoading,
}: {
  agents: Agent[];
  isLoading: boolean;
}) {
  return (
    <section>
      <h2 className="text-lg font-medium text-gray-900 mb-4">
        Agent Runtimes
      </h2>
      {isLoading ? (
        <p className="text-sm text-gray-500">Loading agents...</p>
      ) : agents.length === 0 ? (
        <p className="text-sm text-gray-500">No agent runtimes available.</p>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3">
          {agents.map((agent) => (
            <div
              key={agent.id}
              className="bg-white rounded-lg border border-gray-200 p-3 shadow-sm"
            >
              <div className="flex items-center justify-between">
                <span className="font-medium text-gray-900 text-sm">
                  {agent.name}
                </span>
                <StatusBadge
                  status={agent.status}
                  variant={
                    agent.status === "available"
                      ? "green"
                      : agent.status === "busy"
                        ? "yellow"
                        : "gray"
                  }
                />
              </div>
              {agent.version && (
                <p className="text-xs text-gray-400 mt-1">v{agent.version}</p>
              )}
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

/* ─── Task Submission Form ─── */

function TaskSubmissionForm({
  agents,
  managedAgents,
  managedAgentsLoading,
  onTaskCreated,
}: {
  agents: Agent[];
  managedAgents: ManagedAgent[];
  managedAgentsLoading: boolean;
  onTaskCreated?: (taskId: string) => void;
}) {
  const [agentType, setAgentType] = useState("");
  const [selectedAgentId, setSelectedAgentId] = useState("");
  const [prompt, setPrompt] = useState("");
  const [error, setError] = useState("");
  const [deliverableType, setDeliverableType] = useState("");
  const [gitRepoUrl, setGitRepoUrl] = useState("");
  const [localDirectoryPath, setLocalDirectoryPath] = useState("");
  const createTask = useCreateTask();

  // Filter managed agents to show only those that are available for task delegation:
  // - "idle" agents (no running tasks)
  // - "working" agents (have running tasks but below their concurrency limit)
  const availableAgents = managedAgents.filter(
    (a) => a.status === "idle" || a.status === "working"
  );

  // Find the currently selected managed agent for confirmation display
  const selectedAgent = managedAgents.find((a) => a.id === selectedAgentId);

  // Validation for execution workspace path
  const pathError =
    deliverableType === "execution" && localDirectoryPath.trim() && !localDirectoryPath.startsWith("/")
      ? "Path must be absolute (start with /)"
      : null;

  const isSubmitDisabled = createTask.isPending || !!pathError;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    if (!agentType && !selectedAgentId) {
      setError("Please select an agent type or a managed agent.");
      return;
    }
    if (!prompt.trim()) {
      setError("Prompt cannot be empty.");
      return;
    }
    if (prompt.length > 10000) {
      setError("Prompt must be 10,000 characters or fewer.");
      return;
    }

    // Validate execution workspace config
    if (deliverableType === "execution") {
      if (!localDirectoryPath.trim()) {
        setError("Local directory path is required for execution tasks.");
        return;
      }
      if (!localDirectoryPath.startsWith("/")) {
        setError("Local directory path must be an absolute path (start with /).");
        return;
      }
    }

    // Build the task creation payload
    const payload: CreateTaskInput = {
      agent_type: agentType,
      prompt: prompt.trim(),
    };

    // If a managed agent is selected, include agent_id and derive agent_type from runtime
    if (selectedAgentId) {
      payload.agent_id = selectedAgentId;
      // Use the agent's runtime name as agent_type if no explicit type selected
      if (!agentType && selectedAgent?.runtime_name) {
        payload.agent_type = selectedAgent.runtime_name;
      }
    }

    // Include conversational task fields when deliverable_type is selected
    if (deliverableType) {
      payload.deliverable_type = deliverableType;

      if (deliverableType === "execution") {
        payload.local_directory_path = localDirectoryPath.trim();
        if (gitRepoUrl.trim()) {
          payload.git_repo_url = gitRepoUrl.trim();
        }
      }
    }

    createTask.mutate(payload, {
      onSuccess: (createdTask) => {
        setPrompt("");
        setSelectedAgentId("");
        setAgentType("");
        setError("");
        setDeliverableType("");
        setGitRepoUrl("");
        setLocalDirectoryPath("");
        onTaskCreated?.(createdTask.id);
      },
      onError: (err: Error) => {
        setError(err.message || "Failed to create task");
      },
    });
  };

  // Deduplicate agent providers for the fallback dropdown
  const uniqueProviders = Array.from(
    new Map(agents.map((a) => [a.provider, a])).values()
  );

  return (
    <section>
      <h2 className="text-lg font-medium text-gray-900 mb-4">Submit Task</h2>
      <form
        onSubmit={handleSubmit}
        className="bg-white rounded-lg border border-gray-200 p-4 shadow-sm space-y-4"
      >
        {/* Managed Agent Selector */}
        <div>
          <label
            htmlFor="managed-agent"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            Delegate to Agent
          </label>
          <select
            id="managed-agent"
            value={selectedAgentId}
            onChange={(e) => {
              setSelectedAgentId(e.target.value);
              // Clear the legacy agent type when a managed agent is selected
              if (e.target.value) {
                setAgentType("");
              }
            }}
            className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          >
            <option value="">Select an agent...</option>
            {managedAgentsLoading ? (
              <option disabled>Loading agents...</option>
            ) : availableAgents.length === 0 ? (
              <option disabled>No available agents</option>
            ) : (
              availableAgents.map((agent) => (
                <option key={agent.id} value={agent.id}>
                  {agent.name}
                  {agent.status === "working" ? " (working)" : ""}
                  {agent.model ? ` — ${agent.model}` : ""}
                </option>
              ))
            )}
          </select>
          <p className="text-xs text-gray-400 mt-1">
            Showing agents that are idle or below their concurrency limit.
          </p>
        </div>

        {/* Selected Agent Confirmation */}
        {selectedAgent && (
          <div className="rounded-md bg-blue-50 border border-blue-200 p-3">
            <p className="text-sm font-medium text-blue-900 mb-1">
              Selected Agent
            </p>
            <div className="flex flex-wrap items-center gap-3 text-sm text-blue-700">
              <span className="inline-flex items-center gap-1">
                <span className="font-medium">Name:</span> {selectedAgent.name}
              </span>
              {selectedAgent.runtime_name && (
                <span className="inline-flex items-center gap-1">
                  <span className="font-medium">Runtime:</span>{" "}
                  {selectedAgent.runtime_name}
                </span>
              )}
              {selectedAgent.model && (
                <span className="inline-flex items-center gap-1">
                  <span className="font-medium">Model:</span>{" "}
                  {selectedAgent.model}
                </span>
              )}
            </div>
          </div>
        )}

        {/* Legacy Agent Type (fallback when no managed agent selected) */}
        {!selectedAgentId && (
          <div>
            <label
              htmlFor="agent-type"
              className="block text-sm font-medium text-gray-700 mb-1"
            >
              Agent Type
            </label>
            <select
              id="agent-type"
              value={agentType}
              onChange={(e) => setAgentType(e.target.value)}
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            >
              <option value="">Select an agent type...</option>
              {uniqueProviders.map((agent) => (
                <option key={agent.provider} value={agent.provider}>
                  {agent.name} ({agent.provider})
                </option>
              ))}
            </select>
          </div>
        )}

        <div>
          <label
            htmlFor="prompt"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            Prompt
          </label>
          <textarea
            id="prompt"
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            rows={4}
            maxLength={10000}
            placeholder="Enter your task prompt..."
            className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 resize-y"
          />
          <p className="text-xs text-gray-400 mt-1">
            {prompt.length}/10,000 characters
          </p>
        </div>

        {/* Deliverable Type Selector (Conversational Mode) */}
        <div>
          <label
            htmlFor="deliverable-type"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            Deliverable Type
          </label>
          <select
            id="deliverable-type"
            value={deliverableType}
            onChange={(e) => setDeliverableType(e.target.value)}
            className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          >
            <option value="">None (single-pass execution)</option>
            <option value="plan">Plan</option>
            <option value="design">Design</option>
            <option value="tasks">Tasks</option>
            <option value="execution">Execution</option>
          </select>
          <p className="text-xs text-gray-400 mt-1">
            Select a deliverable type for conversational workflow, or leave as "None" for a single-pass task.
          </p>
        </div>

        {/* Execution Workspace Config (shown when execution is selected) */}
        {deliverableType === "execution" && (
          <div className="space-y-3 rounded-md border border-gray-200 bg-gray-50 p-3">
            <p className="text-sm font-medium text-gray-700">
              Workspace Configuration
            </p>

            <div>
              <label
                htmlFor="git-repo-url"
                className="block text-sm font-medium text-gray-700 mb-1"
              >
                Git Repository URL{" "}
                <span className="text-xs text-gray-400 font-normal">(optional)</span>
              </label>
              <input
                id="git-repo-url"
                type="text"
                value={gitRepoUrl}
                onChange={(e) => setGitRepoUrl(e.target.value)}
                placeholder="https://github.com/user/repo.git"
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
              />
              <p className="text-xs text-gray-400 mt-1">
                If the local directory doesn't exist, the repo will be cloned there.
              </p>
            </div>

            <div>
              <label
                htmlFor="local-directory-path"
                className="block text-sm font-medium text-gray-700 mb-1"
              >
                Local Directory Path <span className="text-red-500">*</span>
              </label>
              <input
                id="local-directory-path"
                type="text"
                value={localDirectoryPath}
                onChange={(e) => setLocalDirectoryPath(e.target.value)}
                placeholder="/home/user/projects/my-app"
                className={`w-full rounded-md border px-3 py-2 text-sm focus:outline-none focus:ring-2 ${
                  pathError
                    ? "border-red-300 focus:border-red-300 focus:ring-red-100"
                    : "border-gray-300 focus:border-blue-500 focus:ring-blue-500"
                }`}
                aria-required="true"
                aria-invalid={!!pathError}
                aria-describedby={pathError ? "local-dir-path-error" : undefined}
              />
              {pathError && (
                <p
                  id="local-dir-path-error"
                  className="mt-1 text-xs text-red-600"
                  role="alert"
                >
                  {pathError}
                </p>
              )}
              <p className="text-xs text-gray-400 mt-1">
                Absolute path where the agent will execute. Required for execution tasks.
              </p>
            </div>
          </div>
        )}

        {error && <p className="text-sm text-red-600">{error}</p>}

        <button
          type="submit"
          disabled={isSubmitDisabled}
          className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {createTask.isPending ? "Submitting..." : "Submit Task"}
        </button>
      </form>
    </section>
  );
}

/* ─── Task Queue ─── */

function TaskQueueSection({
  tasks,
  total,
  offset,
  isLoading,
  onPrevPage,
  onNextPage,
}: {
  tasks: Task[];
  total: number;
  offset: number;
  isLoading: boolean;
  onPrevPage: () => void;
  onNextPage: () => void;
}) {
  const currentPage = Math.floor(offset / TASKS_PER_PAGE) + 1;
  const totalPages = Math.max(1, Math.ceil(total / TASKS_PER_PAGE));

  return (
    <section>
      <h2 className="text-lg font-medium text-gray-900 mb-4">Task Queue</h2>
      {isLoading ? (
        <p className="text-sm text-gray-500">Loading tasks...</p>
      ) : tasks.length === 0 ? (
        <p className="text-sm text-gray-500">No tasks in queue.</p>
      ) : (
        <>
          <div className="bg-white rounded-lg border border-gray-200 shadow-sm overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                    Status
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                    Agent
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                    Prompt
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                    Created
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {tasks.map((task) => (
                  <TaskRow key={task.id} task={task} />
                ))}
              </tbody>
            </table>
          </div>

          {/* Pagination Controls */}
          <div className="flex items-center justify-between mt-4">
            <p className="text-sm text-gray-500">
              Showing {offset + 1}–{Math.min(offset + TASKS_PER_PAGE, total)} of{" "}
              {total} tasks
            </p>
            <div className="flex items-center gap-2">
              <button
                onClick={onPrevPage}
                disabled={offset === 0}
                className="px-3 py-1.5 text-sm border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Previous
              </button>
              <span className="text-sm text-gray-600">
                Page {currentPage} of {totalPages}
              </span>
              <button
                onClick={onNextPage}
                disabled={offset + TASKS_PER_PAGE >= total}
                className="px-3 py-1.5 text-sm border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Next
              </button>
            </div>
          </div>
        </>
      )}
    </section>
  );
}

function TaskRow({ task }: { task: Task }) {
  const navigate = useNavigate();
  const promptPreview =
    task.prompt.length > 100 ? task.prompt.slice(0, 100) + "…" : task.prompt;

  return (
    <tr className="hover:bg-gray-50 cursor-pointer" onClick={() => navigate(`/tasks/${task.id}`)}>
      <td className="px-4 py-3">
        <TaskStatusBadge status={task.status} />
      </td>
      <td className="px-4 py-3 text-sm text-gray-900">{task.agent_type}</td>
      <td className="px-4 py-3 text-sm text-gray-600 max-w-xs truncate">
        {promptPreview}
      </td>
      <td className="px-4 py-3 text-sm text-gray-500 whitespace-nowrap">
        {new Date(task.created_at).toLocaleString()}
      </td>
    </tr>
  );
}

/* ─── Shared Components ─── */

function StatusBadge({
  status,
  variant,
}: {
  status: string;
  variant: "green" | "yellow" | "gray" | "red";
}) {
  const colors = {
    green: "bg-green-100 text-green-700",
    yellow: "bg-yellow-100 text-yellow-700",
    gray: "bg-gray-100 text-gray-600",
    red: "bg-red-100 text-red-700",
  };

  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${colors[variant]}`}
    >
      {status}
    </span>
  );
}

function TaskStatusBadge({
  status,
}: {
  status: Task["status"];
}) {
  const config: Record<Task["status"], { variant: "green" | "yellow" | "gray" | "red"; label: string }> = {
    pending: { variant: "yellow", label: "Pending" },
    running: { variant: "yellow", label: "Running" },
    completed: { variant: "green", label: "Completed" },
    failed: { variant: "red", label: "Failed" },
    cancelled: { variant: "gray", label: "Cancelled" },
    timeout: { variant: "red", label: "Timeout" },
  };

  const { variant, label } = config[status];
  return <StatusBadge status={label} variant={variant} />;
}
