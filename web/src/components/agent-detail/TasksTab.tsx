import { useState } from "react";
import { useNavigate } from "react-router-dom";
import type { Agent, TaskStatus } from "../../lib/agent-detail-types";
import { useAgentTasks } from "../../hooks/useAgentDetail";
import { truncatePrompt, formatDuration, formatRelativeTime } from "../../lib/agent-utils";

/* ─── Props ─── */

interface TasksTabProps {
  agent: Agent;
}

/* ─── Status Badge ─── */

const statusConfig: Record<TaskStatus, { color: string; label: string }> = {
  pending: { color: "bg-yellow-100 text-yellow-700", label: "Pending" },
  dispatched: { color: "bg-purple-100 text-purple-700", label: "Dispatched" },
  running: { color: "bg-blue-100 text-blue-700", label: "Running" },
  completed: { color: "bg-green-100 text-green-700", label: "Completed" },
  failed: { color: "bg-red-100 text-red-700", label: "Failed" },
  cancelled: { color: "bg-gray-100 text-gray-600", label: "Cancelled" },
  timeout: { color: "bg-orange-100 text-orange-700", label: "Timeout" },
};

function TaskStatusBadge({ status }: { status: TaskStatus }) {
  const { color, label } = statusConfig[status] ?? {
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

/* ─── Component ─── */

/**
 * Tasks tab in the Overview Pane.
 * Displays all tasks assigned to the agent, ordered by creation date (newest first).
 * Each task shows: status badge, prompt preview, creation time, and duration.
 * Clicking a task row navigates to /tasks/:taskId.
 * Shows an empty state message when no tasks exist.
 *
 * Validates: Requirements 11.1, 11.2, 11.3, 11.4
 */
export function TasksTab({ agent }: TasksTabProps) {
  const navigate = useNavigate();
  const [offset, setOffset] = useState(0);
  const limit = 20;

  const { data, isLoading } = useAgentTasks(agent.id, { limit, offset });

  /* ─── Loading State ─── */
  if (isLoading) {
    return (
      <div className="flex flex-col gap-3">
        {Array.from({ length: 5 }).map((_, i) => (
          <div
            key={i}
            className="h-14 animate-pulse rounded-md bg-gray-100"
            aria-hidden="true"
          />
        ))}
      </div>
    );
  }

  const tasks = data?.tasks ?? [];
  const total = data?.total ?? 0;

  /* ─── Empty State ─── */
  if (tasks.length === 0 && offset === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <p className="text-sm text-muted-foreground">No tasks yet</p>
      </div>
    );
  }

  const hasMore = offset + limit < total;
  const hasPrev = offset > 0;

  return (
    <div className="flex flex-col gap-3">
      {/* Task list */}
      <ul className="flex flex-col gap-1" role="list">
        {tasks.map((task) => (
          <li
            key={task.id}
            role="button"
            tabIndex={0}
            className="flex items-center gap-3 rounded-md border px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 transition-colors"
            onClick={() => navigate(`/tasks/${task.id}`)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                navigate(`/tasks/${task.id}`);
              }
            }}
            aria-label={`Task: ${truncatePrompt(task.prompt, 60)} — ${statusConfig[task.status]?.label ?? task.status}`}
          >
            {/* Status badge */}
            <TaskStatusBadge status={task.status} />

            {/* Prompt preview */}
            <span className="flex-1 truncate text-gray-900">
              {truncatePrompt(task.prompt)}
            </span>

            {/* Duration */}
            <span className="shrink-0 text-xs text-gray-500">
              {task.duration_ms != null ? formatDuration(task.duration_ms) : "—"}
            </span>

            {/* Creation time */}
            <span className="shrink-0 text-xs text-gray-400">
              {formatRelativeTime(task.created_at)}
            </span>
          </li>
        ))}
      </ul>

      {/* Pagination controls */}
      {(hasPrev || hasMore) && (
        <div className="flex items-center justify-between pt-2">
          <button
            type="button"
            disabled={!hasPrev}
            onClick={() => setOffset((prev) => Math.max(0, prev - limit))}
            className="rounded px-3 py-1 text-xs font-medium text-gray-700 hover:bg-gray-100 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            ← Previous
          </button>
          <span className="text-xs text-gray-500">
            {offset + 1}–{Math.min(offset + limit, total)} of {total}
          </span>
          <button
            type="button"
            disabled={!hasMore}
            onClick={() => setOffset((prev) => prev + limit)}
            className="rounded px-3 py-1 text-xs font-medium text-gray-700 hover:bg-gray-100 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            Next →
          </button>
        </div>
      )}
    </div>
  );
}
