import { useState } from "react";
import type { TimelineItem } from "../../lib/tool-chain-parser";
import { extractFinalResult } from "../../lib/tool-chain-parser";

interface FinalResultPanelProps {
  items: TimelineItem[];
  taskStatus: string;
}

const TRUNCATE_THRESHOLD = 2000;

/**
 * Highlighted panel displaying the final result of a task.
 * - Completed tasks: green/success border with last text-type item
 * - Failed tasks: red/error border with last error-type item
 * - Running/pending tasks: not rendered
 * Content is truncated at 2000 chars with a "Show more" expand button.
 */
export function FinalResultPanel({ items, taskStatus }: FinalResultPanelProps) {
  const [expanded, setExpanded] = useState(false);

  const result = extractFinalResult(items, taskStatus);

  if (!result) {
    return null;
  }

  const content = result.content || result.output || "";
  const isLong = content.length > TRUNCATE_THRESHOLD;
  const displayContent = !expanded && isLong
    ? content.slice(0, TRUNCATE_THRESHOLD)
    : content;

  const borderColor =
    taskStatus === "completed"
      ? "border-emerald-500"
      : "border-red-500";

  const headingColor =
    taskStatus === "completed"
      ? "text-emerald-700"
      : "text-red-700";

  return (
    <div
      className={`rounded-lg border-2 ${borderColor} bg-white p-4`}
      role="region"
      aria-label="Final result"
    >
      <h3 className={`text-sm font-semibold ${headingColor} mb-2`}>
        Result
      </h3>
      <pre className="text-sm text-gray-800 whitespace-pre-wrap break-words font-mono">
        {displayContent}
        {!expanded && isLong && "…"}
      </pre>
      {isLong && (
        <button
          type="button"
          className="mt-2 text-xs text-blue-600 hover:text-blue-800 font-medium cursor-pointer"
          onClick={() => setExpanded((prev) => !prev)}
        >
          {expanded ? "Show less" : "Show more"}
        </button>
      )}
    </div>
  );
}
