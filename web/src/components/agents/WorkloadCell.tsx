import type { Workload } from "../../lib/agent-availability";

/* ─── Props ─── */

interface WorkloadCellProps {
  workload: Workload;
  runningCount: number;
  queuedCount: number;
  capacity: number;
}

/**
 * Table cell for the Workload column.
 * Renders the agent's current task execution state:
 * - idle: "Idle" in muted text
 * - working: "Working" with running/capacity counts
 * - queued: "Queued" with queue count
 *
 * Requirements: 6.4
 */
export function WorkloadCell({
  workload,
  runningCount,
  queuedCount,
  capacity,
}: WorkloadCellProps) {
  switch (workload) {
    case "idle":
      return (
        <span className="text-sm text-gray-500">Idle</span>
      );

    case "working":
      return (
        <span className="text-sm text-gray-700">
          <span className="font-medium text-blue-600">Working</span>
          <span className="ml-1 text-xs text-gray-500">
            {runningCount}/{capacity}
          </span>
        </span>
      );

    case "queued":
      return (
        <span className="text-sm text-gray-700">
          <span className="font-medium text-amber-600">Queued</span>
          {queuedCount > 0 && (
            <span className="ml-1 text-xs text-gray-500">
              +{queuedCount}
            </span>
          )}
        </span>
      );
  }
}
