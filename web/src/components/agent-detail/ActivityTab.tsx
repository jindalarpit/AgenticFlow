import { useState } from "react";
import { useNavigate } from "react-router-dom";
import type { Agent, AgentTask, TaskStatus } from "../../lib/agent-detail-types";
import { useAgentStats, useAgentTasks } from "../../hooks/useAgentDetail";
import {
  truncatePrompt,
  sortActiveTasks,
  formatDuration,
  formatRelativeTime,
  computeSuccessRate,
} from "../../lib/agent-utils";

/* ─── Constants ─── */

const INITIAL_RECENT_LIMIT = 5;
const LOAD_MORE_BATCH = 20;

const ACTIVE_STATUSES: TaskStatus[] = ["running", "dispatched", "pending"];
const TERMINAL_STATUSES: TaskStatus[] = ["completed", "failed", "cancelled", "timeout"];

/* ─── Props ─── */

interface ActivityTabProps {
  agent: Agent;
}

/* ─── Status Icon Component ─── */

function StatusIcon({ status }: { status: TaskStatus }) {
  switch (status) {
    case "running":
      return (
        <span
          className="inline-block h-2.5 w-2.5 rounded-full bg-blue-500 animate-pulse"
          aria-label="Running"
        />
      );
    case "dispatched":
      return (
        <span
          className="inline-block h-2.5 w-2.5 rounded-full bg-amber-500"
          aria-label="Dispatched"
        />
      );
    case "pending":
      return (
        <span
          className="inline-block h-2.5 w-2.5 rounded-full bg-gray-400"
          aria-label="Pending"
        />
      );
    case "completed":
      return (
        <span
          className="inline-block h-2.5 w-2.5 rounded-full bg-emerald-500"
          aria-label="Completed"
        />
      );
    case "failed":
      return (
        <span
          className="inline-block h-2.5 w-2.5 rounded-full bg-red-500"
          aria-label="Failed"
        />
      );
    case "cancelled":
      return (
        <span
          className="inline-block h-2.5 w-2.5 rounded-full bg-gray-500"
          aria-label="Cancelled"
        />
      );
    case "timeout":
      return (
        <span
          className="inline-block h-2.5 w-2.5 rounded-full bg-orange-500"
          aria-label="Timeout"
        />
      );
    default:
      return null;
  }
}

/* ─── Elapsed Time Helper ─── */

function getElapsedTime(task: AgentTask): string {
  const startTime = task.status === "running" && task.started_at
    ? task.started_at
    : task.created_at;

  if (!startTime) return "";

  const start = new Date(startTime).getTime();
  const now = Date.now();
  const elapsed = now - start;

  if (elapsed < 0) return "<1s";
  return formatDuration(elapsed);
}

/* ─── NOW Section ─── */

function NowSection({ tasks }: { tasks: AgentTask[] }) {
  const activeTasks = tasks.filter((t) =>
    ACTIVE_STATUSES.includes(t.status)
  );

  const sorted = sortActiveTasks(
    activeTasks.map((t) => ({ id: t.id, status: t.status, created_at: t.created_at }))
  );

  // Map sorted IDs back to full tasks
  const sortedTasks = sorted
    .map((s) => activeTasks.find((t) => t.id === s.id))
    .filter((t): t is AgentTask => t !== undefined);

  return (
    <section aria-labelledby="now-heading">
      <h3
        id="now-heading"
        className="text-xs font-medium uppercase tracking-wider text-muted-foreground mb-3"
      >
        Now
      </h3>
      {sortedTasks.length === 0 ? (
        <p className="text-sm text-gray-500">Not running</p>
      ) : (
        <ul className="flex flex-col gap-2" role="list">
          {sortedTasks.map((task) => (
            <li
              key={task.id}
              className="flex items-center gap-3 rounded-md border px-3 py-2"
            >
              <StatusIcon status={task.status} />
              <span className="flex-1 truncate text-sm">
                {truncatePrompt(task.prompt)}
              </span>
              <span className="text-xs text-gray-500 whitespace-nowrap">
                {getElapsedTime(task)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

/* ─── LAST 30 DAYS Section ─── */

function Last30DaysSection({ agentId }: { agentId: string }) {
  const { data: stats } = useAgentStats(agentId);

  if (!stats) {
    return (
      <section aria-labelledby="stats-heading">
        <h3
          id="stats-heading"
          className="text-xs font-medium uppercase tracking-wider text-muted-foreground mb-3"
        >
          Last 30 Days
        </h3>
        <p className="text-sm text-gray-500">Loading...</p>
      </section>
    );
  }

  const hasTerminalTasks = stats.total_terminal > 0;
  const hasCompletedTasks = stats.total_runs > 0;

  return (
    <section aria-labelledby="stats-heading">
      <h3
        id="stats-heading"
        className="text-xs font-medium uppercase tracking-wider text-muted-foreground mb-3"
      >
        Last 30 Days
      </h3>
      {!hasTerminalTasks ? (
        <p className="text-sm text-gray-500">No activity in the last 30 days</p>
      ) : (
        <div className="grid grid-cols-3 gap-4">
          <div className="flex flex-col">
            <span className="text-lg font-semibold">{stats.total_runs}</span>
            <span className="text-xs text-gray-500">Total Runs</span>
          </div>
          <div className="flex flex-col">
            <span className="text-lg font-semibold">
              {hasCompletedTasks
                ? `${computeSuccessRate(stats.total_runs, stats.total_terminal)}%`
                : "0%"}
            </span>
            <span className="text-xs text-gray-500">Success Rate</span>
          </div>
          <div className="flex flex-col">
            <span className="text-lg font-semibold">
              {hasCompletedTasks
                ? formatDuration(stats.avg_duration_ms)
                : "\u2014"}
            </span>
            <span className="text-xs text-gray-500">Avg Duration</span>
          </div>
        </div>
      )}
    </section>
  );
}

/* ─── RECENT WORK Section ─── */

function RecentWorkSection({ agentId }: { agentId: string }) {
  const navigate = useNavigate();
  const [limit, setLimit] = useState(INITIAL_RECENT_LIMIT);

  const { data } = useAgentTasks(agentId, { limit, offset: 0 });

  const terminalTasks = (data?.tasks ?? []).filter((t) =>
    TERMINAL_STATUSES.includes(t.status)
  );

  const hasMore = data ? data.total > limit : false;

  const handleShowMore = () => {
    setLimit((prev) => prev + LOAD_MORE_BATCH);
  };

  const handleTaskClick = (taskId: string) => {
    navigate(`/tasks/${taskId}`);
  };

  return (
    <section aria-labelledby="recent-work-heading">
      <h3
        id="recent-work-heading"
        className="text-xs font-medium uppercase tracking-wider text-muted-foreground mb-3"
      >
        Recent Work
      </h3>
      {terminalTasks.length === 0 ? (
        <p className="text-sm text-gray-500">No recent work</p>
      ) : (
        <>
          <ul className="flex flex-col gap-2" role="list">
            {terminalTasks.map((task) => (
              <li key={task.id}>
                <button
                  type="button"
                  onClick={() => handleTaskClick(task.id)}
                  className="w-full flex flex-col gap-1 rounded-md border px-3 py-2 text-left hover:bg-gray-50 transition-colors cursor-pointer"
                >
                  <div className="flex items-center gap-3">
                    <StatusIcon status={task.status} />
                    <span className="flex-1 truncate text-sm">
                      {truncatePrompt(task.prompt)}
                    </span>
                    <span className="text-xs text-gray-500 whitespace-nowrap">
                      {task.completed_at
                        ? formatRelativeTime(task.completed_at)
                        : ""}
                    </span>
                    {task.duration_ms !== undefined && (
                      <span className="text-xs text-gray-400 whitespace-nowrap">
                        {formatDuration(task.duration_ms)}
                      </span>
                    )}
                  </div>
                  {task.status === "failed" && task.failure_reason && (
                    <p className="text-xs text-red-600 ml-5 mt-1">
                      {task.failure_reason}
                    </p>
                  )}
                </button>
              </li>
            ))}
          </ul>
          {hasMore && (
            <button
              type="button"
              onClick={handleShowMore}
              className="mt-3 text-sm text-blue-600 hover:text-blue-800 font-medium"
            >
              Show more
            </button>
          )}
        </>
      )}
    </section>
  );
}

/* ─── ActivityTab Component ─── */

/**
 * Activity tab in the Overview Pane.
 * Displays three sections: NOW (active tasks), LAST 30 DAYS (stats), RECENT WORK (terminal tasks).
 *
 * Validates: Requirements 8.1, 8.2, 8.3, 8.4, 8.5, 9.1, 9.2, 9.3, 9.4, 9.5, 9.6, 10.1, 10.2, 10.3, 10.4, 10.5
 */
export function ActivityTab({ agent }: ActivityTabProps) {
  // Fetch all tasks to derive active tasks for the NOW section
  const { data: tasksData } = useAgentTasks(agent.id, { limit: 50, offset: 0 });
  const allTasks = tasksData?.tasks ?? [];

  return (
    <div className="flex flex-col gap-6">
      <NowSection tasks={allTasks} />
      <Last30DaysSection agentId={agent.id} />
      <RecentWorkSection agentId={agent.id} />
    </div>
  );
}
