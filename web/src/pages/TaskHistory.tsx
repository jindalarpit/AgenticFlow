import { useCallback, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTasks } from "../hooks/useTasks";
import type { Task } from "../hooks/useTasks";

const TASKS_PER_PAGE = 25;

export default function TaskHistory() {
  const navigate = useNavigate();
  const [offset, setOffset] = useState(0);
  const { data, isLoading } = useTasks(TASKS_PER_PAGE, offset);

  const tasks = data?.tasks ?? [];
  const total = data?.total ?? 0;
  const currentPage = Math.floor(offset / TASKS_PER_PAGE) + 1;
  const totalPages = Math.max(1, Math.ceil(total / TASKS_PER_PAGE));

  const handlePrev = useCallback(() => {
    setOffset((prev) => Math.max(0, prev - TASKS_PER_PAGE));
  }, []);

  const handleNext = useCallback(() => {
    if (offset + TASKS_PER_PAGE < total) {
      setOffset((prev) => prev + TASKS_PER_PAGE);
    }
  }, [offset, total]);

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-200 px-6 py-4">
        <div className="max-w-7xl mx-auto flex items-center justify-between">
          <h1 className="text-xl font-semibold text-gray-900">Task History</h1>
          <button
            onClick={() => navigate("/")}
            className="text-sm text-blue-600 hover:text-blue-800"
          >
            ← Back to Dashboard
          </button>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-6 py-8">
        {isLoading ? (
          <p className="text-sm text-gray-500">Loading tasks...</p>
        ) : tasks.length === 0 ? (
          <p className="text-sm text-gray-500">No tasks found.</p>
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
                      Duration
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                      Created
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200">
                  {tasks.map((task) => (
                    <TaskHistoryRow
                      key={task.id}
                      task={task}
                      onClick={() => navigate(`/tasks/${task.id}`)}
                    />
                  ))}
                </tbody>
              </table>
            </div>

            {/* Pagination Controls */}
            <div className="flex items-center justify-between mt-4">
              <p className="text-sm text-gray-500">
                Showing {offset + 1}–{Math.min(offset + TASKS_PER_PAGE, total)}{" "}
                of {total} tasks
              </p>
              <div className="flex items-center gap-2">
                <button
                  onClick={handlePrev}
                  disabled={offset === 0}
                  className="px-3 py-1.5 text-sm border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Previous
                </button>
                <span className="text-sm text-gray-600">
                  Page {currentPage} of {totalPages}
                </span>
                <button
                  onClick={handleNext}
                  disabled={offset + TASKS_PER_PAGE >= total}
                  className="px-3 py-1.5 text-sm border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Next
                </button>
              </div>
            </div>
          </>
        )}
      </main>
    </div>
  );
}

/* ─── Task History Row ─── */

function TaskHistoryRow({
  task,
  onClick,
}: {
  task: Task;
  onClick: () => void;
}) {
  const promptPreview =
    task.prompt.length > 200 ? task.prompt.slice(0, 200) + "…" : task.prompt;

  return (
    <tr
      onClick={onClick}
      className="hover:bg-gray-50 cursor-pointer"
    >
      <td className="px-4 py-3">
        <TaskStatusBadge status={task.status} />
      </td>
      <td className="px-4 py-3 text-sm text-gray-900">{task.agent_type}</td>
      <td className="px-4 py-3 text-sm text-gray-600 max-w-md truncate">
        {promptPreview}
      </td>
      <td className="px-4 py-3 text-sm text-gray-500 whitespace-nowrap">
        {formatDuration(task.started_at, task.completed_at)}
      </td>
      <td className="px-4 py-3 text-sm text-gray-500 whitespace-nowrap">
        {new Date(task.created_at).toLocaleString()}
      </td>
    </tr>
  );
}

/* ─── Helpers ─── */

function formatDuration(
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

function TaskStatusBadge({ status }: { status: Task["status"] }) {
  const config: Record<
    Task["status"],
    { color: string; label: string }
  > = {
    pending: { color: "bg-yellow-100 text-yellow-700", label: "Pending" },
    running: { color: "bg-blue-100 text-blue-700", label: "Running" },
    completed: { color: "bg-green-100 text-green-700", label: "Completed" },
    failed: { color: "bg-red-100 text-red-700", label: "Failed" },
    cancelled: { color: "bg-gray-100 text-gray-600", label: "Cancelled" },
    timeout: { color: "bg-red-100 text-red-700", label: "Timeout" },
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
