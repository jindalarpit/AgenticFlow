/**
 * DeliverableNav — Tab/step navigation showing deliverable states.
 *
 * Displays a horizontal tab bar for the 4 deliverable types (plan, design, tasks, execution).
 * Each tab shows the deliverable name and a visual indicator for its current state:
 * - completed: green check
 * - running: spinner
 * - pending: gray dot
 * - skipped: strikethrough text
 *
 * Clicking a tab calls onSelect with the deliverable type, allowing the parent
 * to show that deliverable's ConversationThread and DeliverablePanel.
 *
 * Validates: Requirements 7.3
 */

export type DeliverableStatus = "pending" | "running" | "completed" | "skipped";

export interface DeliverableInfo {
  /** Deliverable type identifier. */
  type: string;
  /** Current status of the deliverable. */
  status: DeliverableStatus;
}

export interface DeliverableNavProps {
  /** Array of deliverables with their current states. */
  deliverables: DeliverableInfo[];
  /** The currently active/selected deliverable type. */
  activeType: string;
  /** Callback when a tab is clicked. Receives the deliverable type. */
  onSelect: (type: string) => void;
}

/** Human-readable labels for deliverable types. */
const DELIVERABLE_LABELS: Record<string, string> = {
  plan: "Plan",
  design: "Design",
  tasks: "Tasks",
  execution: "Execution",
};

export function DeliverableNav({ deliverables, activeType, onSelect }: DeliverableNavProps) {
  if (deliverables.length === 0) return null;

  return (
    <nav aria-label="Deliverable navigation" className="w-full">
      <div className="flex border-b border-gray-200" role="tablist">
        {deliverables.map((deliverable) => {
          const isActive = deliverable.type === activeType;
          return (
            <button
              key={deliverable.type}
              role="tab"
              aria-selected={isActive}
              aria-label={`${DELIVERABLE_LABELS[deliverable.type] ?? deliverable.type} — ${deliverable.status}`}
              onClick={() => onSelect(deliverable.type)}
              className={`
                relative flex items-center gap-2 px-4 py-2.5 text-sm font-medium
                transition-colors focus:outline-none focus:ring-2 focus:ring-blue-200 focus:ring-inset
                ${isActive
                  ? "text-blue-600 border-b-2 border-blue-600 -mb-px"
                  : "text-gray-500 hover:text-gray-700 hover:bg-gray-50"
                }
              `}
            >
              <StatusIndicator status={deliverable.status} />
              <span className={deliverable.status === "skipped" ? "line-through" : ""}>
                {DELIVERABLE_LABELS[deliverable.type] ?? deliverable.type}
              </span>
            </button>
          );
        })}
      </div>
    </nav>
  );
}

/* ─── Internal Components ─── */

function StatusIndicator({ status }: { status: DeliverableStatus }) {
  switch (status) {
    case "completed":
      return (
        <svg
          className="h-4 w-4 text-green-500"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2.5}
          aria-hidden="true"
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
        </svg>
      );

    case "running":
      return (
        <span
          className="inline-block h-4 w-4 animate-spin rounded-full border-2 border-gray-300 border-t-blue-500"
          aria-hidden="true"
        />
      );

    case "pending":
      return (
        <span
          className="inline-block h-2.5 w-2.5 rounded-full bg-gray-300"
          aria-hidden="true"
        />
      );

    case "skipped":
      return (
        <svg
          className="h-4 w-4 text-gray-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
          aria-hidden="true"
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M20 12H4" />
        </svg>
      );

    default:
      return (
        <span
          className="inline-block h-2.5 w-2.5 rounded-full bg-gray-300"
          aria-hidden="true"
        />
      );
  }
}
