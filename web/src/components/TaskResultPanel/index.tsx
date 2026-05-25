import type { Task, TaskMessage } from "../../hooks/useTasks";
import { extractDashboardResult } from "../../lib/taskResultUtils";
import { TaskResultHeader } from "./TaskResultHeader";
import { TaskResultBody } from "./TaskResultBody";
import { TaskResultActions } from "./TaskResultActions";

export interface TaskResultPanelProps {
  taskId: string;
  onDismiss: () => void;
  /** Task data from React Query (provided by useTaskResultPanel at Dashboard level) */
  task?: Task;
  /** Streaming messages (provided by useTaskResultPanel at Dashboard level) */
  messages?: TaskMessage[];
  /** Whether WebSocket is connected */
  wsConnected?: boolean;
}

/**
 * Composed Task Result Panel that displays the outcome of a submitted task
 * directly on the Dashboard. Composes TaskResultHeader, TaskResultBody,
 * and TaskResultActions into a single styled container.
 *
 * The parent Dashboard calls `useTaskResultPanel` and passes down the
 * task data, messages, and connection status as props.
 */
export function TaskResultPanel({
  taskId,
  onDismiss,
  task,
  messages = [],
  wsConnected = true,
}: TaskResultPanelProps) {
  // Loading state — task data hasn't arrived yet
  if (!task) {
    return (
      <div className="bg-white rounded-lg border border-gray-200 shadow-sm overflow-hidden">
        <div className="flex items-center justify-center px-4 py-8">
          <div className="flex items-center gap-2 text-sm text-gray-500">
            <LoadingSpinner />
            <span>Loading task...</span>
          </div>
        </div>
      </div>
    );
  }

  // Compute full content for the actions bar (copy button needs non-truncated content)
  const fullContent =
    task.status === "completed"
      ? extractDashboardResult(messages, task.output_preview ?? null) ?? ""
      : messages.map((m) => m.content).join("");

  // Determine if streaming is active (WS connected and task is non-terminal)
  const isStreaming =
    wsConnected && (task.status === "pending" || task.status === "running");

  return (
    <div className="bg-white rounded-lg border border-gray-200 shadow-sm overflow-hidden">
      <TaskResultHeader
        status={task.status}
        prompt={task.prompt}
        onDismiss={onDismiss}
      />
      <TaskResultBody
        task={task}
        messages={messages}
        isStreaming={isStreaming}
      />
      <TaskResultActions
        taskId={taskId}
        status={task.status}
        fullContent={fullContent}
        onCancelSuccess={onDismiss}
      />
    </div>
  );
}

/* ─── Loading Spinner ─── */

function LoadingSpinner() {
  return (
    <svg
      className="animate-spin h-4 w-4 text-gray-400"
      xmlns="http://www.w3.org/2000/svg"
      fill="none"
      viewBox="0 0 24 24"
      aria-hidden="true"
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
  );
}
