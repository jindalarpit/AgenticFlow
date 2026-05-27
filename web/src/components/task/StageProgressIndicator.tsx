/**
 * StageProgressIndicator — Horizontal stepper showing workflow stage statuses.
 *
 * Displays each stage in execution order with visual indicators:
 * - pending: gray/neutral circle
 * - running: blue with pulse animation
 * - awaiting_approval: amber/yellow attention state
 * - approved/completed: green checkmark
 * - rejected: red/warning state
 *
 * Validates: Requirements 10.1, 10.2, 10.3, 10.4, 10.5, 10.6
 */

export type StageStatus =
  | "pending"
  | "running"
  | "awaiting_approval"
  | "approved"
  | "completed"
  | "rejected"
  | "failed";

export interface StageInfo {
  /** Stage name (e.g., "plan", "design", "tasks", "execution") */
  name: string;
  /** Current status of the stage */
  status: StageStatus;
}

export interface StageProgressIndicatorProps {
  /** Ordered array of stages to display */
  stages: StageInfo[];
}

/** Human-readable labels for stage names. */
const STAGE_LABELS: Record<string, string> = {
  plan: "Plan",
  design: "Design",
  tasks: "Tasks",
  execution: "Execution",
};

export function StageProgressIndicator({ stages }: StageProgressIndicatorProps) {
  if (stages.length === 0) return null;

  return (
    <nav aria-label="Workflow stage progress" className="w-full">
      <ol className="flex items-center">
        {stages.map((stage, index) => (
          <li
            key={stage.name}
            className={`flex items-center ${index < stages.length - 1 ? "flex-1" : ""}`}
          >
            {/* Stage indicator + label */}
            <div className="flex flex-col items-center gap-1.5">
              <StageCircle status={stage.status} />
              <span
                className={`text-xs font-medium whitespace-nowrap ${getTextColor(stage.status)}`}
              >
                {STAGE_LABELS[stage.name] ?? stage.name}
              </span>
            </div>

            {/* Connector line between stages */}
            {index < stages.length - 1 && (
              <div
                className={`flex-1 h-0.5 mx-2 mt-[-1.125rem] ${getConnectorColor(stage.status)}`}
                aria-hidden="true"
                data-testid={`connector-${stage.name}`}
              />
            )}
          </li>
        ))}
      </ol>
    </nav>
  );
}

/* ─── Internal Components ─── */

function StageCircle({ status }: { status: StageStatus }) {
  const base = "flex items-center justify-center w-8 h-8 rounded-full border-2 transition-colors";

  switch (status) {
    case "pending":
      return (
        <div
          className={`${base} border-gray-300 bg-gray-100`}
          aria-label="Pending"
        >
          <span className="w-2 h-2 rounded-full bg-gray-400" />
        </div>
      );

    case "running":
      return (
        <div
          className={`${base} border-blue-400 bg-blue-50 animate-pulse`}
          aria-label="Running"
        >
          <span className="w-2.5 h-2.5 rounded-full bg-blue-500" />
        </div>
      );

    case "awaiting_approval":
      return (
        <div
          className={`${base} border-amber-400 bg-amber-50`}
          aria-label="Awaiting approval"
        >
          <svg
            className="w-4 h-4 text-amber-500"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M12 9v2m0 4h.01M12 3a9 9 0 100 18 9 9 0 000-18z"
            />
          </svg>
        </div>
      );

    case "approved":
    case "completed":
      return (
        <div
          className={`${base} border-green-400 bg-green-50`}
          aria-label="Completed"
        >
          <svg
            className="w-4 h-4 text-green-600"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2.5}
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M5 13l4 4L19 7"
            />
          </svg>
        </div>
      );

    case "rejected":
    case "failed":
      return (
        <div
          className={`${base} border-red-400 bg-red-50`}
          aria-label="Rejected"
        >
          <svg
            className="w-4 h-4 text-red-500"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2.5}
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </div>
      );

    default:
      return (
        <div
          className={`${base} border-gray-300 bg-gray-100`}
          aria-label="Unknown"
        >
          <span className="w-2 h-2 rounded-full bg-gray-400" />
        </div>
      );
  }
}

/* ─── Helpers ─── */

function getTextColor(status: StageStatus): string {
  switch (status) {
    case "pending":
      return "text-gray-500";
    case "running":
      return "text-blue-600";
    case "awaiting_approval":
      return "text-amber-600";
    case "approved":
    case "completed":
      return "text-green-600";
    case "rejected":
    case "failed":
      return "text-red-600";
    default:
      return "text-gray-500";
  }
}

function getConnectorColor(status: StageStatus): string {
  switch (status) {
    case "approved":
    case "completed":
      return "bg-green-400";
    case "running":
      return "bg-blue-300";
    case "rejected":
    case "failed":
      return "bg-red-300";
    default:
      return "bg-gray-200";
  }
}
