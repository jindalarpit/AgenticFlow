import type { Task } from "../../hooks/useTasks";

interface TaskResultHeaderProps {
  status: Task["status"];
  prompt: string;
  onDismiss: () => void;
}

/**
 * Header for the Task Result Panel.
 * Displays a status badge, the task prompt (truncated to one line),
 * an animated indicator for active states, and a dismiss button.
 */
export function TaskResultHeader({
  status,
  prompt,
  onDismiss,
}: TaskResultHeaderProps) {
  return (
    <div className="flex items-center justify-between gap-3 px-4 py-3 border-b border-gray-200">
      <div className="flex items-center gap-2 min-w-0">
        <TaskStatusBadge status={status} />
        {(status === "pending" || status === "running") && (
          <AnimatedIndicator status={status} />
        )}
        <p className="text-sm text-gray-700 truncate min-w-0">{prompt}</p>
      </div>
      <button
        onClick={onDismiss}
        className="flex-shrink-0 p-1 rounded hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors"
        aria-label="Dismiss result panel"
      >
        <XIcon />
      </button>
    </div>
  );
}

/* ─── Status Badge (reuses Dashboard pattern) ─── */

function TaskStatusBadge({ status }: { status: Task["status"] }) {
  const config: Record<
    Task["status"],
    { variant: "green" | "yellow" | "gray" | "red"; label: string }
  > = {
    pending: { variant: "yellow", label: "Pending" },
    running: { variant: "yellow", label: "Running" },
    completed: { variant: "green", label: "Completed" },
    failed: { variant: "red", label: "Failed" },
    cancelled: { variant: "gray", label: "Cancelled" },
    timeout: { variant: "red", label: "Timeout" },
  };

  const colors = {
    green: "bg-green-100 text-green-700",
    yellow: "bg-yellow-100 text-yellow-700",
    gray: "bg-gray-100 text-gray-600",
    red: "bg-red-100 text-red-700",
  };

  const { variant, label } = config[status];

  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium whitespace-nowrap ${colors[variant]}`}
    >
      {label}
    </span>
  );
}

/* ─── Animated Indicator ─── */

function AnimatedIndicator({ status }: { status: "pending" | "running" }) {
  if (status === "running") {
    // Pulsing dot for running state
    return (
      <span className="relative flex h-2.5 w-2.5 flex-shrink-0">
        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-yellow-400 opacity-75" />
        <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-yellow-500" />
      </span>
    );
  }

  // Spinner for pending state
  return (
    <svg
      className="animate-spin h-3.5 w-3.5 text-yellow-600 flex-shrink-0"
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

/* ─── X Icon ─── */

function XIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      className="h-4 w-4"
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden="true"
    >
      <path
        fillRule="evenodd"
        d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
        clipRule="evenodd"
      />
    </svg>
  );
}
