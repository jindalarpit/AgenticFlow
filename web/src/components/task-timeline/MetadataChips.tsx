import { useEffect, useState } from "react";

interface MetadataChipsProps {
  toolCallCount: number;
  totalCount: number;
  taskStatus: string;
  startedAt?: string;
  completedAt?: string;
}

/**
 * Format a duration in seconds into a human-readable string.
 * - Under 60s: "Xs"
 * - Under 1h: "Xm Ys"
 * - 1h+: "Xh Ym"
 */
function formatDuration(seconds: number): string {
  if (seconds < 0) seconds = 0;
  const s = Math.floor(seconds);

  if (s < 60) {
    return `${s}s`;
  }

  const minutes = Math.floor(s / 60);
  const remainingSeconds = s % 60;

  if (minutes < 60) {
    return `${minutes}m ${remainingSeconds}s`;
  }

  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return `${hours}h ${remainingMinutes}m`;
}

/**
 * Displays metadata chips: tool call count, total event count,
 * and a live elapsed duration counter (while running) or total duration (when completed).
 */
export function MetadataChips({
  toolCallCount,
  totalCount,
  taskStatus,
  startedAt,
  completedAt,
}: MetadataChipsProps) {
  const [elapsed, setElapsed] = useState<number>(0);

  // Live elapsed timer while task is running
  useEffect(() => {
    if (taskStatus !== "running" || !startedAt) {
      return;
    }

    const start = new Date(startedAt).getTime();

    const updateElapsed = () => {
      const now = Date.now();
      setElapsed(Math.floor((now - start) / 1000));
    };

    updateElapsed();
    const interval = setInterval(updateElapsed, 1000);

    return () => clearInterval(interval);
  }, [taskStatus, startedAt]);

  // Compute total duration for completed tasks
  const totalDuration =
    taskStatus === "completed" && startedAt && completedAt
      ? Math.floor(
          (new Date(completedAt).getTime() - new Date(startedAt).getTime()) /
            1000
        )
      : null;

  return (
    <div className="flex items-center gap-2 flex-wrap">
      {/* Tool call count chip */}
      <span className="inline-flex items-center rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-600">
        {toolCallCount} tool call{toolCallCount !== 1 ? "s" : ""}
      </span>

      {/* Total event count chip */}
      <span className="inline-flex items-center rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-600">
        {totalCount} event{totalCount !== 1 ? "s" : ""}
      </span>

      {/* Live elapsed duration (while running) */}
      {taskStatus === "running" && startedAt && (
        <span className="inline-flex items-center rounded-full bg-blue-50 px-2.5 py-0.5 text-xs font-medium text-blue-700">
          ⏱ {formatDuration(elapsed)}
        </span>
      )}

      {/* Total duration (when completed) */}
      {totalDuration !== null && (
        <span className="inline-flex items-center rounded-full bg-emerald-50 px-2.5 py-0.5 text-xs font-medium text-emerald-700">
          ⏱ {formatDuration(totalDuration)}
        </span>
      )}
    </div>
  );
}
